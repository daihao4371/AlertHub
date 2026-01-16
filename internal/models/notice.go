package models

type AlertNotice struct {
	TenantId            string                       `json:"tenantId"`
	Uuid                string                       `json:"uuid"`
	Name                string                       `json:"name"`
	DutyId              *string                      `json:"dutyId"`
	NoticeType          string                       `json:"noticeType"`
	NoticeTmplId        string                       `json:"noticeTmplId"`
	DefaultHook         string                       `json:"hook" gorm:"column:hook"`
	DefaultSign         string                       `json:"sign" gorm:"column:sign"`
	Routes              []Route                      `json:"routes" gorm:"column:routes;serializer:json"`
	Email               Email                        `json:"email" gorm:"email;serializer:json"`
	PhoneNumber         []string                     `json:"phoneNumber" gorm:"phoneNumber;serializer:json"`
	EnterpriseApiConfig *DingDingEnterpriseApiConfig `json:"enterpriseApiConfig,omitempty" gorm:"column:enterprise_api_config;serializer:json"`
	InternalSmsConfig   *InternalSmsConfig           `json:"internalSmsConfig,omitempty" gorm:"column:internal_sms_config;serializer:json"`
	UpdateAt            int64                        `json:"updateAt"`
	UpdateBy            string                       `json:"updateBy"`
	UpdateByRealName    string                       `json:"updateByRealName" gorm:"-"` // Not persisted, for display only
}

func (alertNotice *AlertNotice) GetDutyId() *string {
	if alertNotice.DutyId == nil {
		return new(string)
	}
	return alertNotice.DutyId
}

type Route struct {
	// 告警等级
	Severity string `json:"severity"`
	// WebHook
	Hook string `json:"hook"`
	// 签名
	Sign string `json:"sign"`
	// 收件人
	To []string `json:"to" gorm:"column:to;serializer:json"`
	// 抄送人
	CC []string `json:"cc" gorm:"column:cc;serializer:json"`
	// 企业内部API配置（仅钉钉使用，可选）
	EnterpriseApiConfig *DingDingEnterpriseApiConfig `json:"enterpriseApiConfig,omitempty" gorm:"column:enterprise_api_config;serializer:json"`
	// 内部短信网关配置（可选，如果未配置则使用默认配置）
	InternalSmsConfig *InternalSmsConfig `json:"internalSmsConfig,omitempty" gorm:"column:internal_sms_config;serializer:json"`
}

// DingDingEnterpriseApiConfig 钉钉企业内部API配置
// 用于配置通过企业内部API发送个人通知
type DingDingEnterpriseApiConfig struct {
	// 是否启用个人通知（true=使用企业内部API，false=使用标准Webhook）
	EnablePersonalNotification bool `json:"enablePersonalNotification"`

	// 企业内部API完整URL（用户配置的完整URL）
	ApiUrl string `json:"apiUrl"` // 例如: http://xxxxxx/dmc-service/dmc/api/msg/enterpriseRobot/receiverSingle

	// 认证配置
	ClientId     string `json:"clientId"`     // loonflow
	ClientSecret string `json:"clientSecret"` // 加密存储（TODO: 后续实现加密）

	// 业务配置
	SecretKey    string `json:"secretKey"`    // 加密存储（TODO: 后续实现加密）
	BusinessCode string `json:"businessCode"` // devops01
	RobotCode    string `json:"robotCode"`    // 钉钉机器人Code

	// 接收者类型配置（固定为5=钉钉用户ID）
	ReceiverType int `json:"receiverType"` // 固定值：5=钉钉用户ID
}

// InternalSmsConfig 内部短信网关配置
type InternalSmsConfig struct {
	// 网关基础配置
	GatewayUrl     string `json:"gatewayUrl"`     // 网关地址，例如：http://smsinner.01.prd.bjm6v.belle.lan/o2o-ms/sendSms
	Priority       string `json:"priority"`       // 优先级，例如：20
	TimeoutSeconds int    `json:"timeoutSeconds"` // 请求超时时间（秒），默认5秒

	// 重试配置
	RetryConfig *RetryConfig `json:"retryConfig,omitempty"`

	// 速率限制配置
	RateLimitConfig *RateLimitConfig `json:"rateLimitConfig,omitempty"`
}

// RetryConfig 重试配置
type RetryConfig struct {
	MaxRetries            int     `json:"maxRetries"`           // 最大重试次数，默认3
	BackoffMultiplier     float64 `json:"backoffMultiplier"`    // 退避倍数，默认2.0
	InitialBackoffSeconds int     `json:"initialBackoffSeconds"` // 初始退避时间（秒），默认1
}

// RateLimitConfig 速率限制配置
type RateLimitConfig struct {
	MaxPerSecond int `json:"maxPerSecond"`   // 每秒最大发送数，默认10
	MaxPerMinute int `json:"maxPerMinute"`   // 每分钟最大发送数，默认500
}

type Email struct {
	Subject string   `json:"subject"`
	To      []string `json:"to" gorm:"column:to;serializer:json"`
	CC      []string `json:"cc" gorm:"column:cc;serializer:json"`
}

type NoticeRecord struct {
	EventId  string `json:"eventId"`  // 事件ID
	Date     string `json:"date"`     // 记录日期
	CreateAt int64  `json:"createAt"` // 记录时间
	TenantId string `json:"tenantId"` // 租户
	RuleName string `json:"ruleName"` // 规则名称
	NType    string `json:"nType"`    // 通知类型
	NObj     string `json:"nObj"`     // 通知对象
	Severity string `json:"severity"` // 告警等级
	Status   int    `json:"status"`   // 通知状态 0 成功 1 失败
	AlarmMsg string `json:"alarmMsg"` // 告警信息
	ErrMsg   string `json:"errMsg"`   // 错误信息
}

type CountRecord struct {
	Date     string `json:"date"`     // 记录日期
	TenantId string `json:"tenantId"` // 租户
	Severity string `json:"severity"` // 告警等级
}

type ResponseNoticeRecords struct {
	List []NoticeRecord `json:"list"`
	Page
}
