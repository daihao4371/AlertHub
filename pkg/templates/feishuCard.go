package templates

import (
	"fmt"
	"strings"
	"alertHub/internal/models"
	"alertHub/pkg/tools"
	"alertHub/pkg/utils"

	"github.com/bytedance/sonic"
)

// Template 飞书消息卡片模版
func feishuTemplate(alert models.AlertCurEvent, noticeTmpl models.NoticeTemplateExample) string {

	var cardContentString string
	if *noticeTmpl.EnableFeiShuJsonCard {
		// 使用自定义JSON卡片模板
		defaultTemplate := models.FeiShuJsonCardMsg{
			MsgType: "interactive",
		}
		var tmplC models.JsonCards
		switch alert.IsRecovered {
		case false:
			cardContentString = noticeTmpl.TemplateFiring
		case true:
			cardContentString = noticeTmpl.TemplateRecover
		}
		cardContentString = ParserTemplate("Card", alert, cardContentString)
		_ = sonic.Unmarshal([]byte(cardContentString), &tmplC)
		defaultTemplate.Card = tmplC

		// 为自定义JSON卡片也添加快捷操作按钮（如果启用）
		// 检查模板级别的快捷操作开关（如果未设置，默认启用以保持向后兼容）
		enableQuickAction := noticeTmpl.EnableQuickAction == nil || *noticeTmpl.EnableQuickAction
		if enableQuickAction {
			actionButtonsMap := buildFeishuActionButtonsMap(alert)
			if actionButtonsMap != nil {
				// 确保Elements字段已初始化
				if defaultTemplate.Card.Elements == nil {
					defaultTemplate.Card.Elements = []map[string]interface{}{}
				}
				defaultTemplate.Card.Elements = append(defaultTemplate.Card.Elements, actionButtonsMap)
			}
		}

		cardContentString = tools.JsonMarshalToString(defaultTemplate)

	} else {
		defaultTemplate := models.FeiShuJsonCardMsg{
			MsgType: "interactive",
			Card: models.JsonCards{
				Config: tools.ConvertStructToMap(models.Configs{
					EnableForward: true,
					WidthMode:     models.WidthModeDefault,
				}),
			},
		}
		cardHeader := models.Headers{
			Template: ParserTemplate("TitleColor", alert, noticeTmpl.Template),
			Title: models.Titles{
				Content: ParserTemplate("Title", alert, noticeTmpl.Template),
				Tag:     "plain_text",
			},
		}
		cardElements := []models.Elements{
			{
				Tag:            "column_set",
				FlexMode:       "none",
				BackgroupStyle: "default",
				Columns: []models.Columns{
					{
						Tag:           "column",
						Width:         "weighted",
						Weight:        1,
						VerticalAlign: "top",
						Elements: []models.ColumnsElements{
							{
								Tag: "div",
								Text: models.Texts{
									Content: ParserTemplate("Event", alert, noticeTmpl.Template),
									Tag:     "lark_md",
								},
							},
						},
					},
				},
			},
			{
				Tag: "hr",
			},
			{
				Tag: "note",
				Elements: []models.ElementsElements{
					{
						Tag:     "plain_text",
						Content: ParserTemplate("Footer", alert, noticeTmpl.Template),
					},
				},
			},
		}

		// 转换cardElements为map列表
		defaultTemplate.Card.Elements = tools.ConvertSliceToMapList(cardElements)

		// 添加快捷操作按钮（如果启用）
		// 检查模板级别的快捷操作开关（如果未设置，默认启用以保持向后兼容）
		enableQuickAction := noticeTmpl.EnableQuickAction == nil || *noticeTmpl.EnableQuickAction
		if enableQuickAction {
			actionButtonsMap := buildFeishuActionButtonsMap(alert)
			if actionButtonsMap != nil {
				defaultTemplate.Card.Elements = append(defaultTemplate.Card.Elements, actionButtonsMap)
			}
		}

		defaultTemplate.Card.Header = tools.ConvertStructToMap(cardHeader)
		cardContentString = tools.JsonMarshalToString(defaultTemplate)

	}

	// 需要将所有换行符进行转义
	cardContentString = strings.Replace(cardContentString, "\n", "\\n", -1)

	return cardContentString

}

// buildFeishuActionButtonsMap 构建飞书快捷操作按钮(返回map格式)
// 由于Elements模型不包含Actions字段,直接返回map结构
func buildFeishuActionButtonsMap(alert models.AlertCurEvent) map[string]interface{} {
	// 如果告警已恢复，不显示快捷操作按钮
	if alert.IsRecovered {
		return nil
	}

	// 获取快捷操作配置
	quickConfig := getQuickActionConfig()

	// 检查配置是否启用且必需字段齐全
	if !quickConfig.GetEnable() || quickConfig.BaseUrl == "" || quickConfig.SecretKey == "" {
		return nil
	}

	// 生成快捷操作Token(24小时有效期)
	token, err := utils.GenerateQuickToken(
		alert.TenantId,
		alert.Fingerprint,
		alert.DutyUser,
		quickConfig.SecretKey,
	)
	if err != nil {
		// Token生成失败,降级处理,不显示按钮
		return nil
	}

	// 确定API调用地址(优先使用ApiUrl,否则使用BaseUrl)
	apiUrl := quickConfig.ApiUrl
	if apiUrl == "" {
		apiUrl = quickConfig.BaseUrl
	}

	// 检查告警是否已被认领或已恢复
	isAlertClaimed := alert.ConfirmState.IsOk
	isAlertRecovered := alert.IsRecovered

	// 构建按钮数组
	buttons := []map[string]interface{}{}

	// 认领告警按钮 - 如果已认领或已恢复则禁用
	claimButton := map[string]interface{}{
		"tag":  "button",
		"type": "primary",
		"text": map[string]interface{}{
			"tag": "plain_text",
		},
	}
	if isAlertClaimed {
		claimButton["text"].(map[string]interface{})["content"] = fmt.Sprintf("✓ 已认领 (%s)", alert.ConfirmState.ConfirmUsername)
		claimButton["disabled"] = true
	} else if isAlertRecovered {
		claimButton["text"].(map[string]interface{})["content"] = "🔔 认领告警 (已恢复)"
		claimButton["disabled"] = true
	} else {
		claimButton["text"].(map[string]interface{})["content"] = "🔔 认领告警"
		claimButton["url"] = fmt.Sprintf("%s/api/v1/alert/quick-action?action=claim&fingerprint=%s&token=%s",
			apiUrl, alert.Fingerprint, token)
	}
	buttons = append(buttons, claimButton)

	// 静默按钮 - 如果已恢复则全部禁用
	silenceButtonsDisabled := isAlertRecovered || isAlertClaimed

	// 静默告警按钮(默认1小时,保持兼容)
	silenceDefaultButton := map[string]interface{}{
		"tag":  "button",
		"type": "default",
		"text": map[string]interface{}{
			"tag":     "plain_text",
			"content": "🔕 静默告警",
		},
	}
	if silenceButtonsDisabled {
		silenceDefaultButton["disabled"] = true
	} else {
		silenceDefaultButton["url"] = fmt.Sprintf("%s/api/v1/alert/quick-action?action=silence&fingerprint=%s&token=%s&duration=1h",
			apiUrl, alert.Fingerprint, token)
	}
	buttons = append(buttons, silenceDefaultButton)

	// 静默1小时
	silence1hButton := map[string]interface{}{
		"tag":  "button",
		"type": "default",
		"text": map[string]interface{}{
			"tag":     "plain_text",
			"content": "🕐 静默1小时",
		},
	}
	if silenceButtonsDisabled {
		silence1hButton["disabled"] = true
	} else {
		silence1hButton["url"] = fmt.Sprintf("%s/api/v1/alert/quick-action?action=silence&fingerprint=%s&token=%s&duration=1h",
			apiUrl, alert.Fingerprint, token)
	}
	buttons = append(buttons, silence1hButton)

	// 静默6小时
	silence6hButton := map[string]interface{}{
		"tag":  "button",
		"type": "default",
		"text": map[string]interface{}{
			"tag":     "plain_text",
			"content": "🕕 静默6小时",
		},
	}
	if silenceButtonsDisabled {
		silence6hButton["disabled"] = true
	} else {
		silence6hButton["url"] = fmt.Sprintf("%s/api/v1/alert/quick-action?action=silence&fingerprint=%s&token=%s&duration=6h",
			apiUrl, alert.Fingerprint, token)
	}
	buttons = append(buttons, silence6hButton)

	// 静默24小时
	silence24hButton := map[string]interface{}{
		"tag":  "button",
		"type": "default",
		"text": map[string]interface{}{
			"tag":     "plain_text",
			"content": "🕙 静默24小时",
		},
	}
	if silenceButtonsDisabled {
		silence24hButton["disabled"] = true
	} else {
		silence24hButton["url"] = fmt.Sprintf("%s/api/v1/alert/quick-action?action=silence&fingerprint=%s&token=%s&duration=24h",
			apiUrl, alert.Fingerprint, token)
	}
	buttons = append(buttons, silence24hButton)

	// 自定义静默(跳转到自定义页面)
	customSilenceButton := map[string]interface{}{
		"tag":  "button",
		"type": "default",
		"text": map[string]interface{}{
			"tag":     "plain_text",
			"content": "⚙️ 自定义静默",
		},
	}
	if silenceButtonsDisabled {
		customSilenceButton["disabled"] = true
	} else {
		customSilenceButton["url"] = fmt.Sprintf("%s/api/v1/alert/quick-silence?fingerprint=%s&token=%s",
			apiUrl, alert.Fingerprint, token)
	}
	buttons = append(buttons, customSilenceButton)

	// 查看详情按钮 - 始终可用
	detailButton := map[string]interface{}{
		"tag":  "button",
		"type": "default",
		"text": map[string]interface{}{
			"tag":     "plain_text",
			"content": "📊 查看详情",
		},
		"url": buildDetailUrl(alert, quickConfig.BaseUrl),
	}
	buttons = append(buttons, detailButton)

	// 返回action元素的map结构
	return map[string]interface{}{
		"tag":     "action",
		"actions": buttons,
	}
}

// buildDetailUrl 构建详情页URL
// 如果有FaultCenterId,跳转到故障中心详情页
// 否则跳转到对应的监控规则列表页
func buildDetailUrl(alert models.AlertCurEvent, baseUrl string) string {
	if alert.FaultCenterId != "" {
		return fmt.Sprintf("%s/faultCenter/detail/%s", baseUrl, alert.FaultCenterId)
	}
	// Probing事件没有FaultCenterId,跳转到拨测规则列表
	return fmt.Sprintf("%s/probing", baseUrl)
}
