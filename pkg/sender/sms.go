package sender

import (
	"errors"
	"fmt"
	"regexp"
	"strings"

	"alertHub/pkg/provider"
	"alertHub/pkg/templates"
	"github.com/bytedance/sonic"
	"github.com/zeromicro/go-zero/core/logc"
)

// SmsConfig 短信服务配置
type SmsConfig struct {
	Provider        string `json:"provider"`        // 服务商: "tencent", "aliyun"
	AccessKeyId     string `json:"accessKeyId"`     // 访问密钥ID
	AccessKeySecret string `json:"accessKeySecret"` // 访问密钥Secret
	SdkAppId        string `json:"sdkAppId"`        // 腾讯云专用: 应用ID
	SignName        string `json:"signName"`        // 短信签名
	TemplateId      string `json:"templateId"`      // 腾讯云模板ID
	TemplateCode    string `json:"templateCode"`    // 阿里云模板代码
	
	// 消息模板配置
	MessageTemplates templates.SmsTemplateConfig `json:"messageTemplates,omitempty"` // 消息模板配置
	
	// 重试配置
	RetryConfig *provider.RetryConfig `json:"retryConfig,omitempty"` // 重试配置，可选
}

// 实现 provider.SmsProviderConfig 接口
func (c *SmsConfig) GetProvider() string {
	return c.Provider
}

func (c *SmsConfig) GetAccessKeyId() string {
	return c.AccessKeyId
}

func (c *SmsConfig) GetAccessKeySecret() string {
	return c.AccessKeySecret
}

func (c *SmsConfig) GetSignName() string {
	return c.SignName
}

// GetRetryConfig 获取重试配置
func (c *SmsConfig) GetRetryConfig() *provider.RetryConfig {
	if c.RetryConfig != nil {
		return c.RetryConfig
	}
	// 返回默认重试配置
	return provider.NewDefaultRetryConfig()
}

// 实现 provider.TencentSmsConfig 接口
func (c *SmsConfig) GetSdkAppId() string {
	return c.SdkAppId
}

func (c *SmsConfig) GetTemplateId() string {
	return c.TemplateId
}

// 实现 provider.AliyunSmsConfig 接口
func (c *SmsConfig) GetTemplateCode() string {
	return c.TemplateCode
}

// SmsResult 短信发送结果
type SmsResult struct {
	Success    bool     `json:"success"`
	Message    string   `json:"message"`
	FailedNums []string `json:"failedNums,omitempty"` // 发送失败的手机号
}

// SmsSender 短信发送器
type SmsSender struct{
	renderer        *templates.SmsTemplateRenderer // 模板渲染器
	providerFactory *provider.SmsProviderFactory   // 服务商工厂
}

// NewSmsSender 创建短信发送器实例
func NewSmsSender() SendInter {
	return &SmsSender{
		renderer:        templates.NewSmsTemplateRenderer(),
		providerFactory: provider.NewSmsProviderFactory(),
	}
}

// Send 发送告警短信
func (s *SmsSender) Send(params SendParams) error {
	// 验证手机号
	if len(params.PhoneNumber) == 0 {
		return errors.New("未配置手机号")
	}

	// 获取配置
	config, err := s.getConfig(params.Hook)
	if err != nil {
		return fmt.Errorf("获取短信配置失败: %v", err)
	}

	// 验证配置
	if err := s.validateConfig(config); err != nil {
		return fmt.Errorf("配置验证失败: %v", err)
	}

	// 验证手机号格式
	validNums := s.validatePhoneNumbers(params.PhoneNumber)
	if len(validNums) == 0 {
		return errors.New("未找到有效的手机号")
	}

	// 构建消息内容
	content := s.buildMessageWithTemplate(params, config)

	// 使用provider factory发送短信
	providerInstance := s.providerFactory.CreateProvider(config.Provider)
	if providerInstance == nil {
		return fmt.Errorf("不支持的短信服务商: %s", config.Provider)
	}

	return providerInstance.SendSms(config, validNums, content, params.IsRecovered)
}

// Test 发送测试短信
func (s *SmsSender) Test(params SendParams) error {
	if len(params.PhoneNumber) == 0 {
		return errors.New("未配置手机号")
	}

	config, err := s.getConfig(params.Hook)
	if err != nil {
		return fmt.Errorf("获取短信配置失败: %v", err)
	}

	if err := s.validateConfig(config); err != nil {
		return fmt.Errorf("配置验证失败: %v", err)
	}

	validNums := s.validatePhoneNumbers(params.PhoneNumber)
	if len(validNums) == 0 {
		return errors.New("未找到有效的手机号")
	}

	// 构建测试消息
	testContent := s.buildTestMessage(config)

	// 使用provider factory发送测试短信
	providerInstance := s.providerFactory.CreateProvider(config.Provider)
	if providerInstance == nil {
		return fmt.Errorf("不支持的短信服务商: %s", config.Provider)
	}

	return providerInstance.SendSms(config, validNums, testContent, false)
}

// getConfig 从Hook字段解析短信配置
func (s *SmsSender) getConfig(hook string) (*SmsConfig, error) {
	if hook == "" {
		return nil, errors.New("短信配置为空")
	}

	var config SmsConfig
	if err := sonic.UnmarshalString(hook, &config); err != nil {
		return nil, fmt.Errorf("解析短信配置失败: %v", err)
	}

	return &config, nil
}

// validateConfig 验证短信配置完整性
func (s *SmsSender) validateConfig(config *SmsConfig) error {
	if config.Provider == "" {
		return errors.New("未配置短信服务商")
	}

	if config.AccessKeyId == "" {
		return errors.New("未配置AccessKeyId")
	}

	if config.AccessKeySecret == "" {
		return errors.New("未配置AccessKeySecret")
	}

	if config.SignName == "" {
		return errors.New("未配置短信签名")
	}

	switch config.Provider {
	case "tencent":
		if config.SdkAppId == "" {
			return errors.New("腾讯云短信需要配置SdkAppId")
		}
		if config.TemplateId == "" {
			return errors.New("腾讯云短信需要配置TemplateId")
		}
	case "aliyun":
		if config.TemplateCode == "" {
			return errors.New("阿里云短信需要配置TemplateCode")
		}
	default:
		return fmt.Errorf("不支持的短信服务商: %s", config.Provider)
	}

	return nil
}

// validatePhoneNumbers 验证手机号格式 (中国手机号)
func (s *SmsSender) validatePhoneNumbers(numbers []string) []string {
	var validNumbers []string
	phoneRegex := regexp.MustCompile(`^1[3-9]\d{9}$`)

	for _, number := range numbers {
		// 清理号码格式
		cleanNumber := strings.ReplaceAll(number, " ", "")
		cleanNumber = strings.ReplaceAll(cleanNumber, "-", "")

		if phoneRegex.MatchString(cleanNumber) {
			validNumbers = append(validNumbers, cleanNumber)
		} else {
			logc.Error(nil, fmt.Sprintf("无效的手机号: %s", number))
		}
	}

	return validNumbers
}

// buildMessageWithTemplate 使用模板构建短信消息内容
func (s *SmsSender) buildMessageWithTemplate(params SendParams, config *SmsConfig) string {
	// 构建模板变量
	status := "告警中"
	if params.IsRecovered {
		status = "已恢复"
	}
	
	// 解析告警内容
	msgMap := params.GetSendMsg()
	content := params.Content
	description := content
	
	if desc, ok := msgMap["description"].(string); ok && desc != "" {
		description = desc
	}
	
	// 使用模板渲染器构建变量
	variables := s.renderer.BuildVariables(status, params.RuleName, content, description, 
		params.Severity, params.NoticeName, params.EventId, params.TenantId)
	
	// 获取对应的模板
	template := s.renderer.GetTemplate(config.MessageTemplates, params.IsRecovered, false)
	
	// 渲染模板
	message := s.renderer.RenderTemplate(template, variables)
	
	// 限制消息长度
	return s.renderer.LimitMessageLength(message)
}

// buildTestMessage 构建测试消息
func (s *SmsSender) buildTestMessage(config *SmsConfig) string {
	// 使用模板渲染器构建测试变量
	variables := s.renderer.BuildTestVariables()
	
	// 获取测试模板
	template := s.renderer.GetTemplate(config.MessageTemplates, false, true)
	
	// 渲染模板
	message := s.renderer.RenderTemplate(template, variables)
	
	// 限制消息长度
	return s.renderer.LimitMessageLength(message)
}
