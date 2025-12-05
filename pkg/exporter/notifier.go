package exporter

import (
	"fmt"
	"strings"
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
			// 使用飞书原生卡片元素，不使用 Markdown 表格
			cardElements, hasDown := n.buildFeiShuCardElements(content)

			// 根据是否有异常决定卡片颜色
			cardTemplate := "blue" // 默认蓝色
			if hasDown {
				cardTemplate = "red" // 有异常使用红色
			}

			msgContent := map[string]interface{}{
				"msg_type": "interactive",
				"card": map[string]interface{}{
					"config": map[string]interface{}{
						"wide_screen_mode": true, // 启用宽屏模式，提升视觉效果
						"enable_forward":   true, // 允许转发
					},
					"header": map[string]interface{}{
						"template": cardTemplate,
						"title": map[string]interface{}{
							"tag":     "plain_text",
							"content": "📊 Exporter 健康巡检报告",
						},
					},
					"elements": cardElements,
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

// buildFeiShuCardElements 构建飞书卡片元素，将 Markdown 内容转换为飞书原生卡片格式
// 返回卡片元素列表和是否有异常状态
func (n *Notifier) buildFeiShuCardElements(content string) ([]map[string]interface{}, bool) {
	elements := []map[string]interface{}{}
	hasDown := false
	lines := splitLines(content)

	// 解析内容，构建卡片元素
	i := 0
	for i < len(lines) {
		line := strings.TrimSpace(lines[i])

		// 跳过标题和空行
		if line == "" || line == "## 📊 Exporter 健康巡检报告" || strings.HasPrefix(line, "**巡检时间**:") {
			i++
			continue
		}

		// 处理总体统计部分
		if strings.Contains(line, "📈 总体统计") {
			// 解析统计信息
			stats := n.parseStatistics(lines, &i)
			if stats != nil {
				elements = append(elements, n.buildStatisticsSection(stats)...)
				if stats["downCount"].(int) > 0 {
					hasDown = true
				}
			}
			continue
		}

		// 处理异常列表
		if strings.Contains(line, "⚠️ 异常 Exporter 列表") {
			downList := n.parseDownList(lines, &i)
			if len(downList) > 0 {
				elements = append(elements, n.buildDownListSection(downList)...)
				hasDown = true
			}
			continue
		}

		// 处理正常运行提示
		if strings.Contains(line, "✅ 所有 Exporter 运行正常") {
			elements = append(elements, map[string]interface{}{
				"tag": "div",
				"text": map[string]interface{}{
					"tag":     "lark_md",
					"content": "✅ 所有 Exporter 运行正常\n\n🎉 本次巡检未发现任何异常，所有 Exporter 均正常运行。",
				},
			})
			i++
			continue
		}

		// 处理详细列表
		if strings.Contains(line, "📋 所有 Exporter 状态") {
			detailed := n.parseDetailedList(lines, &i)
			if len(detailed) > 0 {
				elements = append(elements, detailed...)
			}
			continue
		}

		// 处理历史趋势
		if strings.Contains(line, "📉 近 7 日趋势") {
			trends := n.parseTrends(lines, &i)
			if len(trends) > 0 {
				elements = append(elements, trends...)
			}
			continue
		}

		// 处理其他文本内容（使用 lark_md）
		if strings.HasPrefix(line, "####") || strings.HasPrefix(line, "**") {
			textContent := line
			for i+1 < len(lines) && lines[i+1] != "" && !strings.HasPrefix(lines[i+1], "###") {
				i++
				textContent += "\n" + lines[i]
			}
			elements = append(elements, map[string]interface{}{
				"tag": "div",
				"text": map[string]interface{}{
					"tag":     "lark_md",
					"content": textContent,
				},
			})
		}

		i++
	}

	// 添加分隔线和底部信息
	elements = append(elements, map[string]interface{}{
		"tag": "hr",
	})
	elements = append(elements, map[string]interface{}{
		"tag": "note",
		"elements": []map[string]interface{}{
			{
				"tag":     "lark_md",
				"content": fmt.Sprintf("⏰ **报告时间**: %s\n\n*本报告由 WatchAlert Exporter 健康巡检系统自动生成*", time.Now().Format("2006-01-02 15:04:05")),
			},
		},
	})

	return elements, hasDown
}

// cleanValueForParsing 清理值中的 Markdown 和 HTML 标记，便于数值提取
// 移除: ** (粗体), <font> (颜色), ` (代码), 等格式标记
func cleanValueForParsing(value string) string {
	// 移除 HTML 标签（如 <font color='green'>...</font>）
	value = removeHTMLTags(value)
	// 移除 Markdown 粗体标记 **
	value = strings.ReplaceAll(value, "**", "")
	// 移除代码标记 `
	value = strings.ReplaceAll(value, "`", "")
	// 移除多余空格
	return strings.TrimSpace(value)
}

// removeHTMLTags 移除字符串中的所有 HTML 标签
func removeHTMLTags(s string) string {
	result := ""
	inTag := false
	for _, ch := range s {
		if ch == '<' {
			inTag = true
		} else if ch == '>' {
			inTag = false
		} else if !inTag {
			result += string(ch)
		}
	}
	return result
}

// parseStatistics 解析统计信息
func (n *Notifier) parseStatistics(lines []string, i *int) map[string]interface{} {
	stats := make(map[string]interface{})
	*i++ // 跳过标题行

	var totalCount, upCount, downCount, unknownCount int
	var availabilityRate float64

	// 查找状态行
	for *i < len(lines) {
		line := strings.TrimSpace(lines[*i])
		if line == "" {
			*i++
			continue
		}
		if strings.HasPrefix(line, "|") {
			// 解析表格行
			parts := strings.Split(line, "|")
			if len(parts) >= 3 {
				key := strings.TrimSpace(parts[1])
				rawValue := strings.TrimSpace(parts[2])
				// 清理值，移除 Markdown/HTML 标记
				value := cleanValueForParsing(rawValue)

				// 提取数值
				if strings.Contains(key, "总数") {
					if _, err := fmt.Sscanf(value, "%d", &totalCount); err == nil {
						stats["totalCount"] = totalCount
					}
				} else if strings.Contains(key, "正常") {
					if _, err := fmt.Sscanf(value, "%d", &upCount); err == nil {
						stats["upCount"] = upCount
					}
				} else if strings.Contains(key, "异常") {
					if _, err := fmt.Sscanf(value, "%d", &downCount); err == nil {
						stats["downCount"] = downCount
					}
				} else if strings.Contains(key, "未知") {
					if _, err := fmt.Sscanf(value, "%d", &unknownCount); err == nil {
						stats["unknownCount"] = unknownCount
					}
				} else if strings.Contains(key, "可用率") {
					if _, err := fmt.Sscanf(value, "%f%%", &availabilityRate); err == nil {
						stats["availabilityRate"] = availabilityRate
					}
				}
			}
		} else if strings.Contains(line, "**状态**:") {
			// 状态行
			if strings.Contains(line, "全部正常") {
				stats["status"] = "全部正常"
			} else if strings.Contains(line, "异常") {
				stats["status"] = "有异常"
			} else {
				stats["status"] = "有未知状态"
			}
		} else if strings.HasPrefix(line, "###") {
			// 遇到下一个章节，停止解析
			break
		}
		*i++
	}

	if len(stats) == 0 {
		return nil
	}

	// 设置默认值
	if stats["downCount"] == nil {
		stats["downCount"] = 0
	}
	if stats["unknownCount"] == nil {
		stats["unknownCount"] = 0
	}
	if stats["status"] == nil {
		stats["status"] = "全部正常"
	}

	return stats
}

// buildStatisticsSection 构建统计信息卡片元素
func (n *Notifier) buildStatisticsSection(stats map[string]interface{}) []map[string]interface{} {
	elements := []map[string]interface{}{}

	// 状态标题
	statusIcon := "✅"
	statusText := stats["status"].(string)
	if strings.Contains(statusText, "异常") {
		statusIcon = "⚠️"
	} else if strings.Contains(statusText, "未知") {
		statusIcon = "❓"
	}

	elements = append(elements, map[string]interface{}{
		"tag": "div",
		"text": map[string]interface{}{
			"tag":     "lark_md",
			"content": fmt.Sprintf("**📈 总体统计**\n\n%s **状态**: %s", statusIcon, statusText),
		},
	})

	// 使用 columns 展示统计信息（飞书原生格式）
	columns := []map[string]interface{}{}

	// 总数
	columns = append(columns, map[string]interface{}{
		"tag":    "column",
		"width":  "weighted",
		"weight": 1,
		"elements": []map[string]interface{}{
			{
				"tag": "div",
				"text": map[string]interface{}{
					"tag":     "lark_md",
					"content": fmt.Sprintf("**📊 总数**\n\n**%v**", stats["totalCount"]),
				},
			},
		},
	})

	// 正常
	columns = append(columns, map[string]interface{}{
		"tag":    "column",
		"width":  "weighted",
		"weight": 1,
		"elements": []map[string]interface{}{
			{
				"tag": "div",
				"text": map[string]interface{}{
					"tag":     "lark_md",
					"content": fmt.Sprintf("**✅ 正常**\n\n<font color='green'>**%v**</font>", stats["upCount"]),
				},
			},
		},
	})

	// 异常
	columns = append(columns, map[string]interface{}{
		"tag":    "column",
		"width":  "weighted",
		"weight": 1,
		"elements": []map[string]interface{}{
			{
				"tag": "div",
				"text": map[string]interface{}{
					"tag":     "lark_md",
					"content": fmt.Sprintf("**❌ 异常**\n\n<font color='red'>**%v**</font>", stats["downCount"]),
				},
			},
		},
	})

	// 可用率
	rateColor := "blue"
	if stats["availabilityRate"] != nil {
		rate := stats["availabilityRate"].(float64)
		if rate < 80 {
			rateColor = "red"
		} else if rate < 95 {
			rateColor = "orange"
		}
		columns = append(columns, map[string]interface{}{
			"tag":    "column",
			"width":  "weighted",
			"weight": 1,
			"elements": []map[string]interface{}{
				{
					"tag": "div",
					"text": map[string]interface{}{
						"tag":     "lark_md",
						"content": fmt.Sprintf("**📈 可用率**\n\n<font color='%s'>**%.2f%%**</font>", rateColor, rate),
					},
				},
			},
		})
	}

	elements = append(elements, map[string]interface{}{
		"tag":              "column_set",
		"flex_mode":        "none",
		"background_style": "default",
		"columns":          columns,
	})

	return elements
}

// parseDownList 解析异常列表
func (n *Notifier) parseDownList(lines []string, i *int) []map[string]interface{} {
	downList := []map[string]interface{}{}
	*i++ // 跳过标题行

	// 跳过表格标题行
	for *i < len(lines) {
		line := strings.TrimSpace(lines[*i])
		if strings.HasPrefix(line, "|") && strings.Contains(line, "实例名称") {
			*i++
			break
		}
		*i++
	}

	// 解析数据行
	for *i < len(lines) {
		line := strings.TrimSpace(lines[*i])
		if line == "" {
			*i++
			continue
		}
		if strings.HasPrefix(line, "|") && !strings.Contains(line, "---") {
			parts := strings.Split(line, "|")
			if len(parts) >= 6 {
				item := map[string]interface{}{
					"index":      strings.TrimSpace(parts[1]),
					"instance":   strings.TrimSpace(parts[2]),
					"job":        strings.TrimSpace(parts[3]),
					"datasource": strings.TrimSpace(parts[4]),
					"url":        strings.TrimSpace(parts[5]),
					"time":       strings.TrimSpace(parts[6]),
				}
				downList = append(downList, item)
			}
		} else if strings.HasPrefix(line, "###") || strings.HasPrefix(line, "####") {
			break
		}
		*i++
	}

	return downList
}

// buildDownListSection 构建异常列表卡片元素
func (n *Notifier) buildDownListSection(downList []map[string]interface{}) []map[string]interface{} {
	elements := []map[string]interface{}{}

	// 标题
	elements = append(elements, map[string]interface{}{
		"tag": "div",
		"text": map[string]interface{}{
			"tag":     "lark_md",
			"content": fmt.Sprintf("**⚠️ 异常 Exporter 列表 (%d)**", len(downList)),
		},
	})

	// 使用 div + note 展示每个异常项，增强视觉效果
	for idx, item := range downList {
		content := fmt.Sprintf("**%s. %s**\n\n", item["index"], item["instance"])
		content += fmt.Sprintf("🏷️ **Job**: `%s`\n\n", item["job"])
		content += fmt.Sprintf("📦 **数据源**: %s\n\n", item["datasource"])
		content += fmt.Sprintf("🔗 **采集地址**: `%s`\n\n", item["url"])
		content += fmt.Sprintf("⏰ **最后采集**: %s", item["time"])

		elements = append(elements, map[string]interface{}{
			"tag": "note",
			"elements": []map[string]interface{}{
				{
					"tag":     "lark_md",
					"content": content,
				},
			},
		})

		// 在异常项之间添加分隔线（除最后一项）
		if idx < len(downList)-1 {
			elements = append(elements, map[string]interface{}{
				"tag": "hr",
			})
		}
	}

	return elements
}

// parseDetailedList 解析详细列表
func (n *Notifier) parseDetailedList(lines []string, i *int) []map[string]interface{} {
	elements := []map[string]interface{}{}
	*i++ // 跳过标题行

	for *i < len(lines) {
		line := strings.TrimSpace(lines[*i])
		if strings.HasPrefix(line, "####") {
			// 子标题
			elements = append(elements, map[string]interface{}{
				"tag": "div",
				"text": map[string]interface{}{
					"tag":     "lark_md",
					"content": line,
				},
			})
		} else if strings.HasPrefix(line, "|") && !strings.Contains(line, "---") {
			// 数据行，转换为文本格式
			parts := strings.Split(line, "|")
			if len(parts) >= 4 {
				content := fmt.Sprintf("%s. %s (`%s`) - %s",
					strings.TrimSpace(parts[1]),
					strings.TrimSpace(parts[2]),
					strings.TrimSpace(parts[3]),
					strings.TrimSpace(parts[4]))
				elements = append(elements, map[string]interface{}{
					"tag": "div",
					"text": map[string]interface{}{
						"tag":     "lark_md",
						"content": content,
					},
				})
			}
		} else if strings.HasPrefix(line, "###") {
			break
		}
		*i++
	}

	return elements
}

// parseTrends 解析历史趋势
func (n *Notifier) parseTrends(lines []string, i *int) []map[string]interface{} {
	elements := []map[string]interface{}{}
	*i++ // 跳过标题行

	// 跳过表格标题行
	for *i < len(lines) {
		line := strings.TrimSpace(lines[*i])
		if strings.HasPrefix(line, "|") && strings.Contains(line, "时间") {
			*i++
			break
		}
		*i++
	}

	// 解析数据行
	count := 0
	for *i < len(lines) && count < 10 {
		line := strings.TrimSpace(lines[*i])
		if line == "" {
			*i++
			continue
		}
		if strings.HasPrefix(line, "|") && !strings.Contains(line, "---") {
			parts := strings.Split(line, "|")
			if len(parts) >= 6 {
				content := fmt.Sprintf("**%s** | %s | 总数: %s | 正常: <font color='green'>%s</font> | 异常: <font color='red'>%s</font> | 可用率: <font color='blue'>%s</font>",
					strings.TrimSpace(parts[1]),
					strings.TrimSpace(parts[2]),
					strings.TrimSpace(parts[3]),
					strings.TrimSpace(parts[4]),
					strings.TrimSpace(parts[5]),
					strings.TrimSpace(parts[6]))
				elements = append(elements, map[string]interface{}{
					"tag": "div",
					"text": map[string]interface{}{
						"tag":     "lark_md",
						"content": content,
					},
				})
				count++
			}
		} else if strings.HasPrefix(line, "###") {
			break
		}
		*i++
	}

	if len(elements) > 0 {
		// 添加标题
		title := map[string]interface{}{
			"tag": "div",
			"text": map[string]interface{}{
				"tag":     "lark_md",
				"content": "**📉 近 7 日趋势**",
			},
		}
		elements = append([]map[string]interface{}{title}, elements...)
	}

	return elements
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
