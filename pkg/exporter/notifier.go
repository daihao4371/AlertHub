package exporter

import (
	"fmt"
	"time"
	"watchAlert/internal/ctx"
	"watchAlert/pkg/sender"

	"github.com/bytedance/sonic"
	"github.com/zeromicro/go-zero/core/logc"
)

// Notifier 通知发送器 - 负责向通知组发送巡检报告
type Notifier struct {
	ctx *ctx.Context
}

// NewNotifier 创建通知发送器实例
func NewNotifier(c *ctx.Context) *Notifier {
	return &Notifier{ctx: c}
}

// SendToNoticeGroups 向通知组发送报告
func (n *Notifier) SendToNoticeGroups(tenantId string, noticeGroups []string, content string) error {
	if len(noticeGroups) == 0 {
		return fmt.Errorf("通知组列表为空")
	}

	// 记录发送失败的通知组
	var failedGroups []string
	successCount := 0

	// 遍历通知组并发送
	for _, groupId := range noticeGroups {
		// 获取通知对象详情
		notice, err := n.ctx.DB.Notice().Get(tenantId, groupId)
		if err != nil {
			logc.Errorf(n.ctx.Ctx, "获取通知对象失败: groupId=%s, err=%v", groupId, err)
			failedGroups = append(failedGroups, groupId)
			continue
		}

		// 根据通知类型构造不同格式的消息
		var msgBytes []byte
		switch notice.NoticeType {
		case "DingDing":
			// 钉钉使用 msgtype (无下划线) + markdown 格式
			// 优化内容: 去掉第一行的标题(避免与卡片标题重复)，优化格式
			optimizedContent := n.optimizeContentForDingDing(content)
			msgContent := map[string]interface{}{
				"msgtype": "markdown",
				"markdown": map[string]interface{}{
					"title": "📊 Exporter 健康巡检报告",
					"text":  optimizedContent,
				},
			}
			msgBytes, err = sonic.Marshal(msgContent)

		case "FeiShu":
			// 飞书使用 msg_type (有下划线) + 交互式卡片格式
			// 优化内容: 去掉第一行标题，调整格式以适配飞书 Markdown
			optimizedContent := n.optimizeContentForFeiShu(content)
			msgContent := map[string]interface{}{
				"msg_type": "interactive",
				"card": map[string]interface{}{
					"header": map[string]interface{}{
						"template": "blue",
						"title": map[string]interface{}{
							"tag":     "plain_text",
							"content": "📊 Exporter 健康巡检报告",
						},
					},
					"elements": []map[string]interface{}{
						{
							"tag": "div",
							"text": map[string]interface{}{
								"tag":     "lark_md",
								"content": optimizedContent,
							},
						},
						{
							"tag": "hr",
						},
						{
							"tag": "note",
							"elements": []map[string]interface{}{
								{
									"tag":     "plain_text",
									"content": fmt.Sprintf("报告时间: %s", time.Now().Format("2006-01-02 15:04:05")),
								},
							},
						},
					},
				},
			}
			msgBytes, err = sonic.Marshal(msgContent)

		default:
			// 其他类型默认使用通用 Markdown 格式（兼容企业微信等）
			msgContent := map[string]interface{}{
				"msgtype": "markdown",
				"markdown": map[string]interface{}{
					"content": fmt.Sprintf("# Exporter 健康巡检报告\n\n%s", content),
				},
			}
			msgBytes, err = sonic.Marshal(msgContent)
		}

		if err != nil {
			logc.Errorf(n.ctx.Ctx, "序列化消息内容失败: notice=%s, err=%v", notice.Name, err)
			failedGroups = append(failedGroups, groupId)
			continue
		}

		// 构造发送参数
		sendParams := sender.SendParams{
			TenantId:    tenantId,
			EventId:     "exporter-report-" + time.Now().Format("20060102150405"),
			RuleName:    "Exporter 健康巡检报告",
			Severity:    "info",
			NoticeType:  notice.NoticeType,
			NoticeId:    notice.Uuid,
			NoticeName:  notice.Name,
			IsRecovered: false,
			Hook:        notice.DefaultHook,
			Email:       notice.Email,
			Content:     string(msgBytes),
			PhoneNumber: notice.PhoneNumber,
			Sign:        notice.DefaultSign,
		}

		// 发送通知
		err = sender.Sender(n.ctx, sendParams)
		if err != nil {
			logc.Errorf(n.ctx.Ctx, "发送巡检报告失败: notice=%s, err=%v", notice.Name, err)
			failedGroups = append(failedGroups, groupId)
			continue
		}

		successCount++
		logc.Infof(n.ctx.Ctx, "巡检报告发送成功: notice=%s", notice.Name)
	}

	// 返回结果
	if len(failedGroups) > 0 {
		return fmt.Errorf("发送完成: 成功 %d/%d, 失败的通知组: %v", successCount, len(noticeGroups), failedGroups)
	}

	logc.Infof(n.ctx.Ctx, "巡检报告发送完成: 成功 %d/%d", successCount, len(noticeGroups))
	return nil
}

// optimizeContentForDingDing 优化钉钉 Markdown 内容
// 去掉第一行标题(避免与卡片标题重复)，调整格式
func (n *Notifier) optimizeContentForDingDing(content string) string {
	lines := splitLines(content)
	if len(lines) == 0 {
		return content
	}

	result := []string{}
	started := false // 标记是否已经开始输出内容

	for _, line := range lines {
		// 跳过第一行标题 "## 📊 Exporter 健康巡检报告"
		if !started {
			if line == "## 📊 Exporter 健康巡检报告" {
				continue // 跳过标题行
			}
			// 跳过标题后的空行
			if line == "" {
				continue
			}
			// 遇到非空内容，开始处理
			started = true
		}

		// 转换标题格式（钉钉对多级标题支持不佳）
		if len(line) >= 4 && line[:4] == "### " {
			// 在标题前添加空行（如果前面有内容）
			if len(result) > 0 && result[len(result)-1] != "" {
				result = append(result, "")
			}
			result = append(result, "**"+line[4:]+"**")
		} else if len(line) >= 5 && line[:5] == "#### " {
			// 在标题前添加空行（如果前面有内容）
			if len(result) > 0 && result[len(result)-1] != "" {
				result = append(result, "")
			}
			result = append(result, "**"+line[5:]+"**")
		} else {
			result = append(result, line)
		}
	}

	return joinLines(result)
}

// optimizeContentForFeiShu 优化飞书 Markdown 内容
// 去掉第一行标题(避免与卡片标题重复)，保持 Markdown 格式
func (n *Notifier) optimizeContentForFeiShu(content string) string {
	lines := splitLines(content)
	if len(lines) == 0 {
		return content
	}

	result := []string{}
	started := false // 标记是否已经开始输出内容

	for _, line := range lines {
		// 跳过第一行标题 "## 📊 Exporter 健康巡检报告"
		if !started {
			if line == "## 📊 Exporter 健康巡检报告" {
				continue // 跳过标题行
			}
			// 跳过标题后的空行
			if line == "" {
				continue
			}
			// 遇到非空内容，开始处理
			started = true
		}

		// 飞书 lark_md 支持标准 Markdown，保持原格式
		result = append(result, line)
	}

	return joinLines(result)
}

// splitLines 分割字符串为行
func splitLines(s string) []string {
	lines := []string{}
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			lines = append(lines, s[start:i])
			start = i + 1
		}
	}
	if start < len(s) {
		lines = append(lines, s[start:])
	}
	return lines
}

// joinLines 合并行为字符串
func joinLines(lines []string) string {
	result := ""
	for i, line := range lines {
		if i > 0 {
			result += "\n"
		}
		result += line
	}
	return result
}