package templates

import (
	"strings"
	"time"
)

// SmsTemplateConfig 短信模板配置
type SmsTemplateConfig struct {
	AlertTemplate    string `json:"alertTemplate,omitempty"`    // 告警模板
	RecoveryTemplate string `json:"recoveryTemplate,omitempty"` // 恢复模板  
	TestTemplate     string `json:"testTemplate,omitempty"`     // 测试模板
}

// TemplateVariables 模板变量结构
type TemplateVariables struct {
	Status      string `json:"status"`      // 告警状态: "告警中"/"已恢复"
	RuleName    string `json:"ruleName"`    // 规则名称
	Content     string `json:"content"`     // 告警内容
	Description string `json:"description"` // 告警描述
	Severity    string `json:"severity"`    // 告警级别
	NoticeName  string `json:"noticeName"`  // 通知对象名称
	Time        string `json:"time"`        // 当前时间
	Date        string `json:"date"`        // 当前日期
	EventId     string `json:"eventId"`     // 事件ID
	TenantId    string `json:"tenantId"`    // 租户ID
}

// 默认模板常量
const (
	// DefaultAlertTemplate 默认告警模板
	DefaultAlertTemplate = "【{status}】{ruleName} - {content}\n时间: {time}\n级别: {severity}\n通知对象: {noticeName}"
	
	// DefaultRecoveryTemplate 默认恢复模板  
	DefaultRecoveryTemplate = "【{status}】{ruleName} - {content}\n恢复时间: {time}\n通知对象: {noticeName}"
	
	// DefaultTestTemplate 默认测试模板
	DefaultTestTemplate = "【测试】这是一条来自 AlertHub 的测试短信\n发送时间: {time}"
)

// SmsTemplateRenderer 短信模板渲染器
type SmsTemplateRenderer struct{}

// NewSmsTemplateRenderer 创建短信模板渲染器实例
func NewSmsTemplateRenderer() *SmsTemplateRenderer {
	return &SmsTemplateRenderer{}
}

// GetTemplate 获取消息模板
func (r *SmsTemplateRenderer) GetTemplate(config SmsTemplateConfig, isRecovered, isTest bool) string {
	if isTest {
		// 测试模板
		if config.TestTemplate != "" {
			return config.TestTemplate
		}
		return DefaultTestTemplate
	}
	
	if isRecovered {
		// 恢复模板
		if config.RecoveryTemplate != "" {
			return config.RecoveryTemplate
		}
		return DefaultRecoveryTemplate
	}
	
	// 告警模板
	if config.AlertTemplate != "" {
		return config.AlertTemplate
	}
	return DefaultAlertTemplate
}

// RenderTemplate 渲染模板变量
func (r *SmsTemplateRenderer) RenderTemplate(template string, variables TemplateVariables) string {
	message := template
	
	// 构建替换映射
	replacements := map[string]string{
		"{status}":      variables.Status,
		"{ruleName}":    variables.RuleName,
		"{content}":     variables.Content,
		"{description}": variables.Description,
		"{severity}":    variables.Severity,
		"{noticeName}":  variables.NoticeName,
		"{time}":        variables.Time,
		"{date}":        variables.Date,
		"{eventId}":     variables.EventId,
		"{tenantId}":    variables.TenantId,
	}
	
	// 执行变量替换
	for placeholder, value := range replacements {
		message = strings.ReplaceAll(message, placeholder, value)
	}
	
	return message
}

// LimitMessageLength 限制消息长度
func (r *SmsTemplateRenderer) LimitMessageLength(message string) string {
	// 限制消息长度为300字符
	if len([]rune(message)) > 297 {
		return string([]rune(message)[:297]) + "..."
	}
	return message
}

// BuildVariables 构建模板变量
func (r *SmsTemplateRenderer) BuildVariables(status, ruleName, content, description, severity, noticeName, eventId, tenantId string) TemplateVariables {
	now := time.Now()
	
	return TemplateVariables{
		Status:      status,
		RuleName:    ruleName,
		Content:     content,
		Description: description,
		Severity:    severity,
		NoticeName:  noticeName,
		Time:        now.Format("2006-01-02 15:04:05"),
		Date:        now.Format("2006-01-02"),
		EventId:     eventId,
		TenantId:    tenantId,
	}
}

// BuildTestVariables 构建测试模板变量
func (r *SmsTemplateRenderer) BuildTestVariables() TemplateVariables {
	now := time.Now()
	
	return TemplateVariables{
		Status: "测试",
		Time:   now.Format("2006-01-02 15:04:05"),
		Date:   now.Format("2006-01-02"),
	}
}