package provider

// SmsProvider 短信服务商接口
type SmsProvider interface {
	// SendSms 发送短信
	SendSms(config SmsProviderConfig, phoneNumbers []string, content string, isRecovered bool) error
}

// SmsProviderConfig 短信服务商配置接口
type SmsProviderConfig interface {
	// GetProvider 获取服务商名称
	GetProvider() string
	// GetAccessKeyId 获取访问密钥ID
	GetAccessKeyId() string
	// GetAccessKeySecret 获取访问密钥Secret
	GetAccessKeySecret() string
	// GetSignName 获取短信签名
	GetSignName() string
	// GetRetryConfig 获取重试配置
	GetRetryConfig() *RetryConfig
}

// SmsProviderFactory 短信服务商工厂
type SmsProviderFactory struct{}

// NewSmsProviderFactory 创建服务商工厂
func NewSmsProviderFactory() *SmsProviderFactory {
	return &SmsProviderFactory{}
}

// CreateProvider 创建服务商实例
func (f *SmsProviderFactory) CreateProvider(providerName string) SmsProvider {
	switch providerName {
	case "tencent":
		return NewTencentSmsProvider()
	case "aliyun":
		return NewAliyunSmsProvider()
	default:
		return nil
	}
}