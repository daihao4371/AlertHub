package sender

import (
	"alertHub/internal/models"
	"alertHub/pkg/tools"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/zeromicro/go-zero/core/logc"
)

// InternalSmsSender 内部短信网关发送器
type InternalSmsSender struct{}

// NewInternalSmsSender 创建内部短信发送器实例
func NewInternalSmsSender() SendInter {
	return &InternalSmsSender{}
}

// Send 发送告警短信
func (s *InternalSmsSender) Send(params SendParams) error {
	// 验证手机号
	if len(params.PhoneNumber) == 0 {
		return errors.New("未配置手机号")
	}

	// 验证手机号格式
	validNums := tools.ValidatePhoneNumbers(params.PhoneNumber)
	if len(validNums) == 0 {
		return errors.New("未找到有效的手机号")
	}

	// 获取内部短信配置
	config := params.InternalSmsConfig
	if config == nil {
		return errors.New("未配置内部短信网关")
	}

	// 验证配置
	if err := s.validateConfig(config); err != nil {
		return fmt.Errorf("配置验证失败: %v", err)
	}

	// 构建短信内容
	content := s.buildMessage(params)

	// 发送短信（包含重试）
	return s.sendWithRetry(validNums, content, config)
}

// Test 发送测试短信
func (s *InternalSmsSender) Test(params SendParams) error {
	if len(params.PhoneNumber) == 0 {
		return errors.New("未配置手机号")
	}

	validNums := tools.ValidatePhoneNumbers(params.PhoneNumber)
	if len(validNums) == 0 {
		return errors.New("未找到有效的手机号")
	}

	config := params.InternalSmsConfig
	if config == nil {
		return errors.New("未配置内部短信网关")
	}

	if err := s.validateConfig(config); err != nil {
		return fmt.Errorf("配置验证失败: %v", err)
	}

	// 使用公共的测试内容常量
	return s.sendWithRetry(validNums, RobotTestContent, config)
}

// validateConfig 验证内部短信配置
func (s *InternalSmsSender) validateConfig(config *models.InternalSmsConfig) error {
	if config.GatewayUrl == "" {
		return errors.New("网关地址不能为空")
	}

	if config.Priority == "" {
		return errors.New("优先级不能为空")
	}

	// 验证URL格式
	if _, err := url.Parse(config.GatewayUrl); err != nil {
		return fmt.Errorf("网关地址格式错误: %v", err)
	}

	return nil
}

// buildMessage 构建短信内容
// 直接使用 params.Content，该内容已通过通知模板生成，包含完整的告警信息
func (s *InternalSmsSender) buildMessage(params SendParams) string {
	// params.Content 已经通过 generateAlertContent 函数使用通知模板生成
	// 这里直接使用模板生成的内容，而不是硬编码格式
	message := params.Content

	// 如果内容为空，使用默认格式作为后备方案
	if message == "" {
		status := "告警中"
		if params.IsRecovered {
			status = "已恢复"
		}
		message = fmt.Sprintf("[%s] %s - 等级:%s - %s",
			status,
			params.RuleName,
			params.Severity,
			params.NoticeName,
		)
	}

	// 限制短信长度（考虑SMS长度限制，一般为300个字符）
	// 如果内容过长，进行截断处理
	const maxSmsLength = 300
	if len([]rune(message)) > maxSmsLength {
		message = string([]rune(message)[:maxSmsLength-3]) + "..."
	}

	return message
}

// sendWithRetry 带重试的发送短信
func (s *InternalSmsSender) sendWithRetry(phoneNumbers []string, content string, config *models.InternalSmsConfig) error {
	retryConfig := s.getRetryConfig(config)

	var lastErr error
	for attempt := 0; attempt <= retryConfig.MaxRetries; attempt++ {
		err := s.sendSms(phoneNumbers, content, config)
		if err == nil {
			return nil
		}

		lastErr = err

		// 判断是否应该重试
		if attempt < retryConfig.MaxRetries && s.isRetriableError(err) {
			backoffTime := s.calculateBackoff(attempt, retryConfig)
			logc.Infof(nil, "短信发送失败，准备重试（第%d次），等待%v ms", attempt+1, backoffTime.Milliseconds())
			time.Sleep(backoffTime)
		}
	}

	return fmt.Errorf("短信发送失败: %v", lastErr)
}

// sendSms 发送短信到网关
func (s *InternalSmsSender) sendSms(phoneNumbers []string, content string, config *models.InternalSmsConfig) error {
	// 创建HTTP客户端，设置超时
	timeout := time.Duration(config.TimeoutSeconds) * time.Second
	if timeout == 0 {
		timeout = 5 * time.Second
	}
	client := &http.Client{Timeout: timeout}

	// 构建请求参数
	data := url.Values{}
	data.Set("receivePhones", strings.Join(phoneNumbers, ","))
	data.Set("content", content)
	data.Set("priority", config.Priority)

	// 发送POST请求
	resp, err := client.PostForm(config.GatewayUrl, data)
	if err != nil {
		return fmt.Errorf("HTTP请求失败: %v", err)
	}
	defer resp.Body.Close()

	// 读取响应
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("读取响应失败: %v", err)
	}

	// 检查HTTP状态码
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("网关返回错误: HTTP %d, 响应: %s", resp.StatusCode, string(body))
	}

	logc.Info(nil, "短信发送成功")

	return nil
}

// getRetryConfig 获取重试配置，使用默认值如果未配置
func (s *InternalSmsSender) getRetryConfig(config *models.InternalSmsConfig) *models.RetryConfig {
	if config.RetryConfig != nil {
		return config.RetryConfig
	}

	return &models.RetryConfig{
		MaxRetries:            3,
		BackoffMultiplier:     2.0,
		InitialBackoffSeconds: 1,
	}
}

// calculateBackoff 计算指数退避的等待时间
func (s *InternalSmsSender) calculateBackoff(attempt int, retryConfig *models.RetryConfig) time.Duration {
	initialBackoff := time.Duration(retryConfig.InitialBackoffSeconds) * time.Second
	backoffTime := time.Duration(float64(initialBackoff) * math.Pow(retryConfig.BackoffMultiplier, float64(attempt)))
	return backoffTime
}

// isRetriableError 判断错误是否可重试
func (s *InternalSmsSender) isRetriableError(err error) bool {
	if err == nil {
		return false
	}

	errMsg := strings.ToLower(err.Error())
	retriableKeywords := []string{
		"timeout",
		"connection",
		"network",
		"500",
		"502",
		"503",
	}

	return tools.ContainsAny(errMsg, retriableKeywords)
}
