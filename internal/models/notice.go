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
