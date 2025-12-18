package models

const (
	SettingSystemAuth = 0
	SettingLdapAuth   = 1
)

type Settings struct {
	IsInit int `json:"isInit"`
	// 0 = 系统认证，1 = LDAP 认证
	AuthType            *int                `json:"authType"`
	EmailConfig         emailConfig         `json:"emailConfig" gorm:"emailConfig;serializer:json"`
	AppVersion          string              `json:"appVersion" gorm:"-"`
	PhoneCallConfig     phoneCallConfig     `json:"phoneCallConfig" gorm:"phoneCallConfig;serializer:json"`
	AiConfig            AiConfig            `json:"aiConfig" gorm:"aiConfig;serializer:json"`
	LdapConfig          LdapConfig          `json:"ldapConfig" gorm:"ldapConfig;serializer:json"`
	OidcConfig          OidcConfig          `json:"oidcConfig" gorm:"oidcConfig;serializer:json"`
	QuickActionConfig   QuickActionConfig   `json:"quickActionConfig" gorm:"quickActionConfig;serializer:json"`
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

// AiConfig ai config
type AiConfig struct {
	Enable *bool `json:"enable"`
	//Type      string `json:"type"` // OpenAi, DeepSeek
	Url       string `json:"url"`
	AppKey    string `json:"appKey"`
	Model     string `json:"model"`
	Timeout   int    `json:"timeout"`
	MaxTokens int    `json:"maxTokens"`
	Prompt    string `json:"prompt"`
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

func (q QuickActionConfig) GetEnable() bool {
	if q.Enabled == nil {
		return false
	}

	return *q.Enabled
}
