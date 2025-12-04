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

	// 构造 JSON 格式的消息内容
	msgContent := map[string]interface{}{
		"msg_type": "text",
		"content": map[string]interface{}{
			"text": content,
		},
	}

	msgBytes, err := sonic.Marshal(msgContent)
	if err != nil {
		return fmt.Errorf("序列化消息内容失败: %w", err)
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