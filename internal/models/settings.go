package models

import "strings"

const (
	SettingSystemAuth = 0
	SettingLdapAuth   = 1
)

type Settings struct {
	IsInit int `json:"isInit"`
	// 0 = 系统认证，1 = LDAP 认证
	AuthType          *int              `json:"authType"`
	EmailConfig       emailConfig       `json:"emailConfig" gorm:"emailConfig;serializer:json"`
	AppVersion        string            `json:"appVersion" gorm:"-"`
	PhoneCallConfig   phoneCallConfig   `json:"phoneCallConfig" gorm:"phoneCallConfig;serializer:json"`
	AiConfig          AiConfig          `json:"aiConfig" gorm:"aiConfig;serializer:json"`
	LdapConfig        LdapConfig        `json:"ldapConfig" gorm:"ldapConfig;serializer:json"`
	OidcConfig        OidcConfig        `json:"oidcConfig" gorm:"oidcConfig;serializer:json"`
	QuickActionConfig QuickActionConfig `json:"quickActionConfig" gorm:"quickActionConfig;serializer:json"`
}

type emailConfig struct {
	ServerAddress string `json:"serverAddress"`
	Port          int    `json:"port"`
	Email         string `json:"email"`
	Token         string `json:"token"`
}

type phoneCallConfig struct {
	Provider        string `json:"provider"`
	Endpoint        string `json:"endpoint"`
	AccessKeyId     string `json:"accessKeyId"`
	AccessKeySecret string `json:"accessKeySecret"`
	TtsCode         string `json:"ttsCode"`
}

// AiConfig ai config - 支持多 Provider 配置管理
type AiConfig struct {
	Enable    *bool                     `json:"enable"`
	Providers map[string]ProviderConfig `json:"providers"` // 各个 provider 的配置
	Timeout   int                       `json:"timeout"`   // 全局超时设置（秒）
	MaxTokens int                       `json:"maxTokens"` // 全局最大 token 数
	Prompt    string                    `json:"prompt"`    // 全局默认提示词

	// 以下字段已废弃，仅用于向后兼容旧数据
	ActiveProvider string `json:"activeProvider,omitempty"` // 已废弃：不再使用激活概念
	Provider       string `json:"provider,omitempty"`       // 已废弃
	Url            string `json:"url,omitempty"`            // 已废弃
	AppKey         string `json:"appKey,omitempty"`         // 已废弃
	Model          string `json:"model,omitempty"`          // 已废弃
}

// ProviderConfig 单个 AI Provider 的配置
type ProviderConfig struct {
	Type   string   `json:"type"`   // Provider 类型：dify | openai
	Url    string   `json:"url"`    // API 端点地址
	AppKey string   `json:"appKey"` // API 密钥
	Models []string `json:"models"` // 该 Provider 支持的模型列表
}

type LdapConfig struct {
	Address         string `json:"address"`
	BaseDN          string `json:"baseDN"`
	AdminUser       string `json:"adminUser"`
	AdminPass       string `json:"adminPass"`
	UserDN          string `json:"userDN"`
	UserPrefix      string `json:"userPrefix"`
	DefaultUserRole string `json:"defaultUserRole"`
	Cronjob         string `json:"cronjob"`
}

type OidcConfig struct {
	ClientID    string `json:"clientID"`
	UpperURI    string `json:"upperURI"`
	RedirectURI string `json:"redirectURI"`
	Domain      string `json:"domain"`
}

// QuickActionConfig 快捷操作配置
type QuickActionConfig struct {
	Enabled   *bool  `json:"enabled"`   // 是否启用快捷操作
	BaseUrl   string `json:"baseUrl"`   // 前端页面地址（用于"查看详情"按钮跳转）
	ApiUrl    string `json:"apiUrl"`    // 后端API地址（用于快捷操作API调用）
	SecretKey string `json:"secretKey"` // Token签名密钥
}

func (a AiConfig) GetEnable() bool {
	if a.Enable == nil {
		return false
	}

	return *a.Enable
}

// GetProviderConfig 根据 Provider 名称获取配置
func (a *AiConfig) GetProviderConfig(providerName string) *ProviderConfig {
	if a.Providers == nil {
		return nil
	}

	if config, exists := a.Providers[providerName]; exists {
		return &config
	}

	return nil
}

// GetAllProviders 获取所有 Provider 名称列表
func (a *AiConfig) GetAllProviders() []string {
	providers := make([]string, 0, len(a.Providers))
	for name := range a.Providers {
		providers = append(providers, name)
	}
	return providers
}

// GetAllModels 获取所有可用模型列表
func (a *AiConfig) GetAllModels() []string {
	models := make([]string, 0)
	for _, config := range a.Providers {
		models = append(models, config.Models...)
	}
	return models
}

// GetProviderByModel 根据模型名称找到对应的 Provider 配置
func (a *AiConfig) GetProviderByModel(modelName string) (string, *ProviderConfig) {
	for providerName, config := range a.Providers {
		for _, model := range config.Models {
			if model == modelName {
				// 自动修复：如果 Type 为空，使用 providerName 的小写作为 Type
				// 这样对于规范命名（dify/openai）的 provider 可以自动工作
				if config.Type == "" {
					config.Type = strings.ToLower(providerName)
					// 更新 map 中的值
					a.Providers[providerName] = config
				}
				return providerName, &config
			}
		}
	}
	return "", nil
}

// SetProviderConfig 设置或更新指定 Provider 的配置
func (a *AiConfig) SetProviderConfig(providerName string, config ProviderConfig) {
	if a.Providers == nil {
		a.Providers = make(map[string]ProviderConfig)
	}
	a.Providers[providerName] = config
}

// DeleteProviderConfig 删除指定 Provider 的配置
func (a *AiConfig) DeleteProviderConfig(providerName string) bool {
	if a.Providers == nil {
		return false
	}
	if _, exists := a.Providers[providerName]; exists {
		delete(a.Providers, providerName)
		return true
	}
	return false
}

func (q QuickActionConfig) GetEnable() bool {
	if q.Enabled == nil {
		return false
	}

	return *q.Enabled
}
