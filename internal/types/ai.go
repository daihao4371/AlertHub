package types

import (
	"encoding/json"
	"fmt"
	"strings"
)

type RequestAiChatContent struct {
	// 规则名称，用来分析告警时，更明确当前是一个什么规则（可选，支持通用机器人）
	RuleName string `json:"ruleName" form:"ruleName"`
	RuleId   string `json:"ruleId" form:"ruleId"`
	SearchQL string `json:"searchQL" form:"searchQL"`
	// 用户内容（必填）
	Content string `json:"content" form:"content"`
	// 重新分析，不调用缓存
	Deep string `json:"deep" form:"deep"`
	// 从 form-data 接收的 alert_info（JSON 字符串）
	AlertInfo string `json:"alert_info" form:"alert_info"`
	// 新增：自定义参数，用于扩展 Prompt 占位符支持（如 {{ CustomField }}）
	Extra map[string]interface{} `json:"extra" form:"extra"`
	// 新增：运行时 Prompt 覆盖，支持完全自定义 Prompt（用于通用机器人场景）
	Prompt string `json:"prompt" form:"prompt"`
	// 新增：运行时指定 Provider，支持动态 Provider 切换（如 openai-gpt4, dify-production）
	Provider string `json:"provider" form:"provider"`
	// 新增：运行时指定模型，支持动态模型切换（如 gpt-4, gpt-3.5-turbo）
	Model string `json:"model" form:"model"`
}

func (a *RequestAiChatContent) ValidateParams() error {
	// 如果 alert_info 不为空，先从中解析 rule_name 和 rule_id
	if a.AlertInfo != "" {
		var alertData map[string]interface{}
		err := json.Unmarshal([]byte(a.AlertInfo), &alertData)
		if err == nil {
			// 从 alert_info 中提取字段
			if ruleName, ok := alertData["rule_name"].(string); ok && a.RuleName == "" {
				a.RuleName = ruleName
			}
			if ruleId, ok := alertData["rule_id"].(string); ok && a.RuleId == "" {
				a.RuleId = ruleId
			}
			// 如果 content 为空，使用 annotations 作为内容
			if a.Content == "" {
				if annotations, ok := alertData["annotations"].(string); ok {
					a.Content = annotations
				}
			}
		}
	}

	errors := make([]string, 0)

	// 核心字段：Content 是必填的
	if a.Content == "" {
		errors = append(errors, "content(告警事件详情)不可为空")
	}

	// 告警分析场景：RuleName 和 RuleId 使用情况
	// 1. 如果没有提供自定义 Prompt，说明使用默认的告警分析 Prompt，此时需要 RuleName 和 RuleId
	// 2. 如果提供了自定义 Prompt，则可以不需要这两个字段（支持通用机器人场景）
	if a.Prompt == "" {
		// 使用默认 Prompt，需要告警相关字段
		if a.RuleName == "" {
			errors = append(errors, "ruleName(规则名称)不可为空")
		}
		if a.RuleId == "" {
			errors = append(errors, "ruleId(规则 ID)不可为空")
		}
	}

	if len(errors) > 0 {
		return fmt.Errorf("参数验证失败: %s", strings.Join(errors, "; "))
	}
	return nil
}
