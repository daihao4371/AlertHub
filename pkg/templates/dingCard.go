package templates

import (
	"fmt"
	"regexp"
	"strings"
	models2 "alertHub/internal/models"
	"alertHub/pkg/tools"
	"alertHub/pkg/utils"
)

// quickActionConfig 快捷操作配置缓存（避免频繁查询数据库）
var quickActionConfig *models2.QuickActionConfig

// SetQuickActionConfig 设置快捷操作配置（由初始化程序调用）
func SetQuickActionConfig(config models2.QuickActionConfig) {
	quickActionConfig = &config
}

// getQuickActionConfig 获取快捷操作配置
func getQuickActionConfig() models2.QuickActionConfig {
	if quickActionConfig == nil {
		// 返回默认配置（禁用状态）
		disabled := false
		return models2.QuickActionConfig{
			Enabled: &disabled,
		}
	}
	return *quickActionConfig
}

// dingdingTemplate 钉钉消息模板
// 支持两种模式：
// 1. Markdown 模式（默认）- 传统文本消息
// 2. ActionCard 模式 - 带快捷操作按钮的卡片消息
func dingdingTemplate(alert models2.AlertCurEvent, noticeTmpl models2.NoticeTemplateExample, notice models2.AlertNotice) string {
	// 检查模板级别的快捷操作开关（如果未设置，默认启用以保持向后兼容）
	enableQuickAction := noticeTmpl.EnableQuickAction == nil || *noticeTmpl.EnableQuickAction
	if !enableQuickAction {
		// 如果模板级别禁用了快捷操作，直接使用 Markdown 模式
		return buildDingdingMarkdown(alert, noticeTmpl, notice)
	}

	// 获取快捷操作配置
	quickConfig := getQuickActionConfig()

	// 如果启用快捷操作且配置了 BaseUrl 和 SecretKey，使用 ActionCard 模式
	if quickConfig.GetEnable() && quickConfig.BaseUrl != "" && quickConfig.SecretKey != "" {
		return buildDingdingActionCard(alert, noticeTmpl, quickConfig, notice)
	}

	// 否则使用传统 Markdown 模式
	return buildDingdingMarkdown(alert, noticeTmpl, notice)
}

// highlightAlertFields 高亮告警消息中的关键字段
// 使用钉钉Markdown支持的HTML标签来实现颜色高亮
// 注意：钉钉Markdown支持 <font color="red"> 标签，但需要确保格式正确
func highlightAlertFields(text string, alert models2.AlertCurEvent) string {
	// 高亮报警等级（P0, P1, P2等）- 红色
	// 匹配格式：**🚨 报警等级:** P0 或 **报警等级:** P0（支持emoji）
	if alert.Severity != "" {
		// 匹配带emoji和不带emoji的格式
		severityPatterns := []*regexp.Regexp{
			// 匹配 **🚨 报警等级:** P0 格式（带emoji）
			regexp.MustCompile(`(\*\*[^\*]*报警等级[^\*]*:\*\*)\s*` + regexp.QuoteMeta(alert.Severity)),
			// 匹配 **报警等级:** P0 格式（不带emoji）
			regexp.MustCompile(`(\*\*[^\*]*报警等级[^\*]*:\*\*)\s*` + regexp.QuoteMeta(alert.Severity)),
			// 匹配表格格式
			regexp.MustCompile(`(报警等级[^|]*\|[^|]*)\s*` + regexp.QuoteMeta(alert.Severity)),
		}
		for _, pattern := range severityPatterns {
			// 检查是否已经高亮过，避免重复处理
			if !strings.Contains(text, fmt.Sprintf(`<font color="red">%s</font>`, alert.Severity)) {
				text = pattern.ReplaceAllString(text, fmt.Sprintf("$1 <font color=\"red\">%s</font>", alert.Severity))
			}
		}
	}

	// 高亮报警状态（报警中）- 红色
	if !alert.IsRecovered {
		statusPatterns := []*regexp.Regexp{
			regexp.MustCompile(`(\*\*[^\*]*报警状态[^\*]*:\*\*)\s*报警中`),
			regexp.MustCompile(`(报警状态[^|]*\|[^|]*)\s*报警中`),
		}
		for _, pattern := range statusPatterns {
			if !strings.Contains(text, `<font color="red">报警中</font>`) {
				text = pattern.ReplaceAllString(text, "$1 <font color=\"red\">报警中</font>")
			}
		}
	}

	// 高亮当前延迟值（如果存在）- 红色
	// 匹配 "当前延迟" 或 "延迟告警当前值" 后面的数字
	// 注意：必须确保匹配完整的数字，不能截断
	delayPatterns := []*regexp.Regexp{
		// 匹配格式：**延迟告警当前值:** 111941 或 **延迟告警当前值:** 111941条消息
		// 使用非贪婪匹配，确保只匹配到数字部分
		regexp.MustCompile(`(\*\*[^\*]*延迟告警当前值[^\*]*:\*\*)\s*(\d+)([^\d<]*)?`),
		regexp.MustCompile(`(\*\*[^\*]*当前延迟[^\*]*:\*\*)\s*(\d+)([^\d<]*)?`),
		regexp.MustCompile(`(延迟告警当前值[^|]*\|[^|]*)\s*(\d+)([^\d<]*)?`),
		regexp.MustCompile(`(当前延迟[^|]*\|[^|]*)\s*(\d+)([^\d<]*)?`),
	}
	for _, pattern := range delayPatterns {
		text = pattern.ReplaceAllStringFunc(text, func(match string) string {
			parts := pattern.FindStringSubmatch(match)
			if len(parts) >= 3 {
				// 提取完整的数字部分并高亮，保留后面的文字（如果有，如"条消息"）
				number := parts[2]
				suffix := ""
				if len(parts) >= 4 && parts[3] != "" {
					suffix = parts[3]
				}
				// 直接高亮完整数字，不进行二次处理
				highlightedNumber := `<font color="red">` + number + `</font>`
				return parts[1] + " " + highlightedNumber + suffix
			}
			return match
		})
	}

	// 高亮报警主机和消费组等链接字段 - 蓝色
	if alert.Labels != nil {
		// 报警主机
		if instanceVal, ok := alert.Labels["instance"]; ok {
			if instance, ok := instanceVal.(string); ok && instance != "" {
				instancePatterns := []*regexp.Regexp{
					regexp.MustCompile(`(\*\*[^\*]*报警主机[^\*]*:\*\*)\s*` + regexp.QuoteMeta(instance)),
					regexp.MustCompile(`(报警主机[^|]*\|[^|]*)\s*` + regexp.QuoteMeta(instance)),
				}
				for _, pattern := range instancePatterns {
					if !strings.Contains(text, fmt.Sprintf(`<font color="blue">%s</font>`, instance)) {
						text = pattern.ReplaceAllString(text, fmt.Sprintf("$1 <font color=\"blue\">%s</font>", instance))
					}
				}
			}
		}
		// 消费组（consumer group）
		if consumerGroupVal, ok := alert.Labels["consumer_group"]; ok {
			if consumerGroup, ok := consumerGroupVal.(string); ok && consumerGroup != "" {
				consumerPatterns := []*regexp.Regexp{
					regexp.MustCompile(`(\*\*[^\*]*消费组[^\*]*:\*\*)\s*` + regexp.QuoteMeta(consumerGroup)),
					regexp.MustCompile(`(消费组[^|]*\|[^|]*)\s*` + regexp.QuoteMeta(consumerGroup)),
				}
				for _, pattern := range consumerPatterns {
					if !strings.Contains(text, fmt.Sprintf(`<font color="blue">%s</font>`, consumerGroup)) {
						text = pattern.ReplaceAllString(text, fmt.Sprintf("$1 <font color=\"blue\">%s</font>", consumerGroup))
					}
				}
			}
		}
	}

	// 高亮值班人员 - 蓝色（@提及通常是可点击的）
	// 匹配格式：**🧑‍💻 值班人员:** @valjnf @qiwehbf
	if alert.DutyUser != "" {
		// 匹配值班人员字段，高亮@用户名（支持emoji）
		dutyUserPatterns := []*regexp.Regexp{
			regexp.MustCompile(`(\*\*[^\*]*值班人员[^\*]*:\*\*)\s*((?:@[^\s<]+\s*)+)`),
			regexp.MustCompile(`(值班人员[^|]*\|[^|]*)\s*((?:@[^\s<]+\s*)+)`),
		}
		for _, pattern := range dutyUserPatterns {
			text = pattern.ReplaceAllStringFunc(text, func(match string) string {
				parts := pattern.FindStringSubmatch(match)
				if len(parts) >= 3 {
					// 高亮所有@用户名（支持中文用户名）
					userPattern := regexp.MustCompile(`(@[^\s<]+)`)
					highlightedUsers := userPattern.ReplaceAllString(parts[2], `<font color="blue">$1</font>`)
					return parts[1] + " " + highlightedUsers
				}
				return match
			})
		}
	}

	// 高亮报警事件中的延迟值 - 红色
	// 匹配格式：**📝 报警事件:** group-rt-480998-mzkjz-consumer消费延迟告警当前值: 111941
	eventDelayPatterns := []*regexp.Regexp{
		// 匹配 "延迟告警当前值: 数字" 格式（在报警事件字段中）
		// 使用更精确的匹配，确保匹配完整的数字
		regexp.MustCompile(`(\*\*[^\*]*报警事件[^\*]*:\*\*[^<]*延迟告警当前值[^:]*:\s*)(\d+)([^\d<]*)?`),
		regexp.MustCompile(`(报警事件[^|]*\|[^|]*延迟告警当前值[^:]*:\s*)(\d+)([^\d<]*)?`),
		// 匹配报警事件字段末尾的大数字（可能是延迟值，但优先级较低）
		// 注意：这个规则可能会误匹配，所以放在最后，并且只在没有匹配到"延迟告警当前值"时才使用
		regexp.MustCompile(`(\*\*[^\*]*报警事件[^\*]*:\*\*[^<]*[^延迟告警当前值])(\d{6,})`), // 匹配6位以上的数字（更精确，避免误匹配）
	}
	for _, pattern := range eventDelayPatterns {
		text = pattern.ReplaceAllStringFunc(text, func(match string) string {
			parts := pattern.FindStringSubmatch(match)
			if len(parts) >= 3 {
				// 提取完整的数字部分并高亮
				number := parts[2]
				suffix := ""
				if len(parts) >= 4 && parts[3] != "" {
					suffix = parts[3]
				}
				// 检查是否已经高亮过，避免重复处理
				if !strings.Contains(match, `<font color="red">`+number+`</font>`) {
					return parts[1] + `<font color="red">` + number + `</font>` + suffix
				}
			}
			return match
		})
	}

	return text
}

// buildDingdingMarkdown 构建钉钉 Markdown 消息（传统模式）
func buildDingdingMarkdown(alert models2.AlertCurEvent, noticeTmpl models2.NoticeTemplateExample, notice models2.AlertNotice) string {
	Title := ParserTemplate("Title", alert, noticeTmpl.Template)
	Footer := ParserTemplate("Footer", alert, noticeTmpl.Template)
	EventText := ParserTemplate("Event", alert, noticeTmpl.Template)

	// 对告警详情进行高亮处理
	EventText = highlightAlertFields(EventText, alert)

	// 解析值班用户，支持 @提及
	dutyUser := alert.DutyUser
	var dutyUsers []string
	for _, user := range strings.Split(dutyUser, " ") {
		u := strings.Trim(user, "@")
		dutyUsers = append(dutyUsers, u)
	}

	// 钉钉关键词处理：在消息内容中添加关键词以满足钉钉机器人的关键词验证要求
	// 注意：钉钉的关键词验证要求关键词必须是独立的词，不能只是包含在其他词中
	// 例如："告警中"包含"告警"，但钉钉可能要求独立的"告警"这个词
	// 因此，无论标题和文本中是否包含关键词，都在消息开头添加关键词，确保通过验证
	dingdingKeyword := getDingdingKeyword(notice)
	keywordPrefix := ""
	if dingdingKeyword != "" {
		// 始终在消息开头添加关键词，确保钉钉能识别到独立的关键词
		keywordPrefix = dingdingKeyword + " "
	}

	t := models2.DingMsg{
		Msgtype: "markdown",
		Markdown: &models2.Markdown{
			Title: keywordPrefix + Title,
			Text: keywordPrefix + "**" + Title + "**" +
				"\n" + "\n" +
				EventText +
				"\n" +
				Footer,
		},
		At: &models2.At{
			AtUserIds: dutyUsers,
			AtMobiles: dutyUsers,
			IsAtAll:   false,
		},
	}

	// 如果是 @all，则@所有人
	if strings.Trim(alert.DutyUser, " ") == "all" {
		t.At = &models2.At{
			AtUserIds: []string{},
			AtMobiles: []string{},
			IsAtAll:   true,
		}
	}

	return tools.JsonMarshalToString(t)
}

// buildDingdingActionCard 构建钉钉 ActionCard 消息（带快捷操作按钮）
func buildDingdingActionCard(alert models2.AlertCurEvent, noticeTmpl models2.NoticeTemplateExample, config models2.QuickActionConfig, notice models2.AlertNotice) string {
	// 如果告警已恢复，不显示快捷操作按钮，使用 Markdown 模式
	if alert.IsRecovered {
		return buildDingdingMarkdown(alert, noticeTmpl, notice)
	}

	Title := ParserTemplate("Title", alert, noticeTmpl.Template)
	EventText := ParserTemplate("Event", alert, noticeTmpl.Template)

	// 对告警详情进行高亮处理
	EventText = highlightAlertFields(EventText, alert)

	// 生成快捷操作 Token（24小时有效期）
	token, err := utils.GenerateQuickToken(
		alert.TenantId,
		alert.Fingerprint,
		alert.DutyUser,
		config.SecretKey,
	)
	if err != nil {
		// Token 生成失败，降级为 Markdown 模式
		return buildDingdingMarkdown(alert, noticeTmpl, notice)
	}

	// 确定 API 调用地址（优先使用 ApiUrl，否则使用 BaseUrl）
	apiUrl := config.ApiUrl
	if apiUrl == "" {
		apiUrl = config.BaseUrl // 向后兼容：如果没有配置 ApiUrl，使用 BaseUrl
	}

	// 钉钉关键词处理：在消息内容中添加关键词以满足钉钉机器人的关键词验证要求
	// 注意：钉钉的关键词验证要求关键词必须是独立的词，不能只是包含在其他词中
	// 例如："告警中"包含"告警"，但钉钉可能要求独立的"告警"这个词
	// 因此，无论标题和文本中是否包含关键词，都在消息开头添加关键词，确保通过验证
	dingdingKeyword := getDingdingKeyword(notice)
	keywordPrefix := ""
	if dingdingKeyword != "" {
		// 始终在消息开头添加关键词，确保钉钉能识别到独立的关键词
		keywordPrefix = dingdingKeyword + " "
	}

	// 构建 ActionCard 消息（使用钉钉官方字段名）
	// 注意：ActionCard 模式下不应包含 markdown 和 at 字段
	card := models2.DingMsg{
		Msgtype: "actionCard",
		ActionCard: &models2.ActionCard{
			Title:          keywordPrefix + Title,
			Text:           keywordPrefix + "#### " + Title + "\n\n" + EventText,
			BtnOrientation: "1", // 按钮纵向排列，移动端体验更好
			Btns: []models2.ActionCardBtn{
				// 认领告警按钮
				{
					Title:     "🔔 认领告警",
					ActionURL: fmt.Sprintf("%s/api/v1/alert/quick-action?action=claim&fingerprint=%s&token=%s", apiUrl, alert.Fingerprint, token),
				},
				// 静默告警按钮(默认87600小时=10年,模拟永久静默)
				{
					Title:     "🔕 静默告警",
					ActionURL: fmt.Sprintf("%s/api/v1/alert/quick-action?action=silence&fingerprint=%s&token=%s&duration=87600h", apiUrl, alert.Fingerprint, token),
				},
				// 静默1小时
				{
					Title:     "🕐 静默1小时",
					ActionURL: fmt.Sprintf("%s/api/v1/alert/quick-action?action=silence&fingerprint=%s&token=%s&duration=1h", apiUrl, alert.Fingerprint, token),
				},
				// 静默6小时
				{
					Title:     "🕕 静默6小时",
					ActionURL: fmt.Sprintf("%s/api/v1/alert/quick-action?action=silence&fingerprint=%s&token=%s&duration=6h", apiUrl, alert.Fingerprint, token),
				},
				// 静默24小时
				{
					Title:     "🕙 静默24小时",
					ActionURL: fmt.Sprintf("%s/api/v1/alert/quick-action?action=silence&fingerprint=%s&token=%s&duration=24h", apiUrl, alert.Fingerprint, token),
				},
				// 自定义静默(跳转到自定义页面)
				{
					Title:     "⚙️ 自定义静默",
					ActionURL: fmt.Sprintf("%s/api/v1/alert/quick-silence?fingerprint=%s&token=%s", apiUrl, alert.Fingerprint, token),
				},
				// 查看详情按钮
				{
					Title:     "📊 查看详情",
					ActionURL: fmt.Sprintf("%s/faultCenter/detail/%s", config.BaseUrl, alert.FaultCenterId),
				},
			},
		},
	}

	return tools.JsonMarshalToString(card)
}

// getDingdingKeyword 获取钉钉关键词
// 钉钉机器人如果配置了关键词验证，消息内容中必须包含关键词才能发送成功
// 返回空字符串，不再添加关键词前缀
func getDingdingKeyword(notice models2.AlertNotice) string {
	// 不再添加关键词前缀，让消息内容更简洁
	// 如果钉钉机器人需要关键词，建议在机器人设置中使用"自定义关键词"或"安全设置"的其他选项
	return ""
}
