package quickaction

import (
	"fmt"
	"time"
	"alertHub/internal/ctx"
	"alertHub/internal/models"
	"alertHub/pkg/sender"
	"alertHub/pkg/tools"
)

// BuildDingTalkConfirmationMessage 构建钉钉确认消息（Markdown格式）
// 用于告知群成员快捷操作的执行结果
// 使用 Markdown 格式,提供更美观的卡片样式展示
func BuildDingTalkConfirmationMessage(
	alert *models.AlertCurEvent,
	actionType, username string,
	duration ...string, // 可选参数，用于静默时传递时长
) string {
	// 根据操作类型生成操作描述、图标和标题
	var actionDesc, actionIcon, title string
	switch actionType {
	case "claim":
		actionDesc = "认领"
		actionIcon = "🔔"
		title = "告警快捷操作通知"
	case "silence":
		// 如果提供了duration参数,显示具体静默时长
		if len(duration) > 0 && duration[0] != "" {
			actionDesc = fmt.Sprintf("静默 %s", FormatDurationChinese(duration[0]))
		} else {
			actionDesc = "静默"
		}
		actionIcon = "🔕"
		title = "告警快捷操作通知"
	case "resolve":
		actionDesc = "标记已处理"
		actionIcon = "✅"
		title = "告警快捷操作通知"
	default:
		actionDesc = actionType
		actionIcon = "ℹ️"
		title = "告警快捷操作通知"
	}

	// 构建 Markdown 格式的消息内容
	// 参考钉钉官方文档的 Markdown 语法
	markdownText := fmt.Sprintf(
		"#### %s %s\n\n"+
			"**📋 告警名称**: %s\n\n"+
			"**🎯 操作类型**: %s\n\n"+
			"**👤 操作人**: %s\n\n"+
			"**⏰ 操作时间**: %s\n\n"+
			"---\n\n"+
			"💡 此消息由 WatchAlert 告警系统自动发送，原告警按钮已失效",
		actionIcon,
		title,
		alert.RuleName,
		actionDesc,
		username,
		time.Now().Format("2006-01-02 15:04:05"),
	)

	// 构建钉钉 Markdown 消息格式
	msg := map[string]interface{}{
		"msgtype": "markdown",
		"markdown": map[string]interface{}{
			"title": fmt.Sprintf("%s %s", actionIcon, title),
			"text":  markdownText,
		},
	}

	return tools.JsonMarshalToString(msg)
}

// BuildFeishuConfirmationMessage 构建飞书确认消息（交互式卡片格式）
// 用于告知群成员快捷操作的执行结果
// 注意: 确认消息不包含操作按钮,避免用户重复操作
// duration是可选参数,用于静默操作时显示具体时长
func BuildFeishuConfirmationMessage(
	alert *models.AlertCurEvent,
	actionType, username string,
	duration ...string, // 可选参数，用于静默时传递时长
) string {
	// 根据操作类型生成操作描述和图标
	var actionDesc, actionIcon, headerColor, noteText string
	switch actionType {
	case "claim":
		actionDesc = "认领"
		actionIcon = "🔔"
		headerColor = "blue"
		noteText = "该告警已被认领,后续操作将由认领人负责"
	case "silence":
		// 如果提供了duration参数,显示具体静默时长
		if len(duration) > 0 && duration[0] != "" {
			actionDesc = fmt.Sprintf("静默 %s", FormatDurationChinese(duration[0]))
		} else {
			actionDesc = "静默"
		}
		actionIcon = "🔕"
		headerColor = "orange"
		noteText = "告警已静默,在静默期间不会再次发送通知"
	case "resolve":
		actionDesc = "标记已处理"
		actionIcon = "✅"
		headerColor = "green"
		noteText = "该告警已标记为已处理状态"
	default:
		actionDesc = actionType
		actionIcon = "ℹ️"
		headerColor = "grey"
		noteText = "操作已完成"
	}

	// 构建飞书交互式卡片
	card := map[string]interface{}{
		"msg_type": "interactive",
		"card": map[string]interface{}{
			"header": map[string]interface{}{
				"template": headerColor,
				"title": map[string]interface{}{
					"tag":     "plain_text",
					"content": fmt.Sprintf("%s 告警快捷操作通知", actionIcon),
				},
			},
			"elements": []map[string]interface{}{
				{
					"tag": "div",
					"fields": []map[string]interface{}{
						{
							"is_short": true,
							"text": map[string]interface{}{
								"tag":     "lark_md",
								"content": fmt.Sprintf("**告警名称**\n%s", alert.RuleName),
							},
						},
						{
							"is_short": true,
							"text": map[string]interface{}{
								"tag":     "lark_md",
								"content": fmt.Sprintf("**操作类型**\n%s", actionDesc),
							},
						},
					},
				},
				{
					"tag": "div",
					"fields": []map[string]interface{}{
						{
							"is_short": true,
							"text": map[string]interface{}{
								"tag":     "lark_md",
								"content": fmt.Sprintf("**操作人**\n%s", username),
							},
						},
						{
							"is_short": true,
							"text": map[string]interface{}{
								"tag":     "lark_md",
								"content": fmt.Sprintf("**操作时间**\n%s", time.Now().Format("2006-01-02 15:04:05")),
							},
						},
					},
				},
				{
					"tag": "hr",
				},
				{
					"tag": "div",
					"text": map[string]interface{}{
						"tag":     "lark_md",
						"content": fmt.Sprintf("💡 **提示**: %s", noteText),
					},
				},
				{
					"tag": "note",
					"elements": []map[string]interface{}{
						{
							"tag":     "plain_text",
							"content": "此消息由 WatchAlert 告警系统自动发送 | 原告警按钮已失效",
						},
					},
				},
			},
		},
	}

	return tools.JsonMarshalToString(card)
}

// SendConfirmationMessage 发送确认消息到群聊
// 操作成功后自动发送一条新消息，告知群成员操作结果
// 支持飞书和钉钉两种通知类型
// duration参数是可选的，仅在静默操作时需要传递
func SendConfirmationMessage(
	ctx *ctx.Context,
	alert *models.AlertCurEvent,
	actionType, username string,
	duration ...string, // 可选参数，用于静默时传递时长
) error {
	// 1. 获取Webhook信息
	hook, sign, noticeType, err := GetWebhookFromAlert(ctx, alert)
	if err != nil {
		return fmt.Errorf("无法发送确认消息: %w", err)
	}

	// 2. 查询用户真实姓名（如果查询失败，则使用用户名）
	displayName := username
	if username != "" {
		var member models.Member
		if err := ctx.DB.DB().Model(&models.Member{}).Where("user_name = ?", username).First(&member).Error; err == nil {
			// 如果查询到真实姓名且不为空，则使用真实姓名；否则使用用户名
			if member.RealName != "" {
				displayName = member.RealName
			}
		}
		// 如果查询失败，继续使用用户名（displayName 已经是 username）
	}

	// 3. 根据通知类型构建不同的消息内容
	var message string
	switch noticeType {
	case "feishu":
		message = BuildFeishuConfirmationMessage(alert, actionType, displayName, duration...)
	case "dingtalk":
		message = BuildDingTalkConfirmationMessage(alert, actionType, displayName, duration...)
	default:
		return fmt.Errorf("不支持的通知类型: %s", noticeType)
	}

	// 4. 发送消息
	return SendMessage(hook, sign, noticeType, message)
}

// SendMessage 发送消息到飞书或钉钉(通用方法，避免代码重复)
// 根据通知类型选择对应的发送器
func SendMessage(hook, sign, noticeType, message string) error {
	params := sender.SendParams{
		Hook:    hook,
		Sign:    sign,
		Content: message,
	}

	switch noticeType {
	case "feishu":
		return sender.NewFeiShuSender().Send(params)
	case "dingtalk":
		return sender.NewDingSender().Send(params)
	default:
		return fmt.Errorf("不支持的通知类型: %s", noticeType)
	}
}
