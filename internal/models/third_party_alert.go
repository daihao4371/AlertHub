package models

// AlertProcessStatus 告警处理状态类型
type AlertProcessStatus string

const (
	ProcessStatusSuccess AlertProcessStatus = "success" // 处理成功
	ProcessStatusFailed  AlertProcessStatus = "failed"  // 处理失败
	ProcessStatusPending AlertProcessStatus = "pending" // 待处理
)

// ThirdPartyAlertStatus 第三方告警状态
type ThirdPartyAlertStatus string

const (
	ThirdPartyAlertFiring   ThirdPartyAlertStatus = "firing"   // 告警触发中
	ThirdPartyAlertResolved ThirdPartyAlertStatus = "resolved" // 告警已恢复
)

// ThirdPartyAlert 第三方告警记录模型
type ThirdPartyAlert struct {
	TenantId  string `json:"tenantId" gorm:"size:64;index"`  // 租户ID
	ID        string `json:"id" gorm:"size:64"`              // 主键ID
	WebhookId string `json:"webhookId" gorm:"size:64;index"` // 关联的Webhook配置ID

	// 原始数据（用于日志和问题排查）
	RawData string `json:"rawData" gorm:"type:text"` // 原始请求数据（JSON格式）
	Headers string `json:"headers" gorm:"type:text"` // 请求头信息（JSON格式）

	// 转换后的标准化数据
	AlertId     string `json:"alertId" gorm:"size:128"`           // 第三方系统的告警ID
	Fingerprint string `json:"fingerprint" gorm:"size:128;index"` // 告警指纹（用于去重）
	Title       string `json:"title" gorm:"size:255"`             // 告警标题
	Content     string `json:"content" gorm:"type:text"`          // 告警详细内容
	Severity    string `json:"severity" gorm:"size:20"`           // 严重级别（P0-P4）
	Status      string `json:"status" gorm:"size:20"`             // 告警状态：firing/resolved

	// 元数据
	Source  string `json:"source" gorm:"size:50"`   // 来源系统
	Host    string `json:"host" gorm:"size:255"`    // 主机信息
	Service string `json:"service" gorm:"size:255"` // 服务信息
	Tags    string `json:"tags" gorm:"type:text"`   // 标签（JSON格式）

	// 时间信息
	SourceTime  int64 `json:"sourceTime"`  // 第三方系统的告警时间（Unix时间戳）
	ProcessTime int64 `json:"processTime"` // AlertHub处理时间（Unix时间戳）

	// 处理状态
	ProcessStatus string `json:"processStatus" gorm:"size:20"`  // 处理状态：success/failed/pending
	ErrorMessage  string `json:"errorMessage" gorm:"type:text"` // 错误信息（处理失败时记录）

	// 关联信息
	FaultCenterId string `json:"faultCenterId" gorm:"size:64"` // 关联的故障中心事件ID
	EventId       string `json:"eventId" gorm:"size:64"`       // 关联的AlertHub告警事件ID

	// 审计字段
	CreateAt int64 `json:"createAt"` // 创建时间（Unix时间戳）
}

// TableName 指定表名
func (ThirdPartyAlert) TableName() string {
	return "third_party_alerts"
}

// IsProcessed 判断告警是否已处理
func (a *ThirdPartyAlert) IsProcessed() bool {
	return a.ProcessStatus == string(ProcessStatusSuccess)
}

// IsFiring 判断是否为触发状态的告警
func (a *ThirdPartyAlert) IsFiring() bool {
	return a.Status == string(ThirdPartyAlertFiring)
}

// IsResolved 判断是否为已恢复状态的告警
func (a *ThirdPartyAlert) IsResolved() bool {
	return a.Status == string(ThirdPartyAlertResolved)
}

// SetProcessSuccess 设置处理成功状态
func (a *ThirdPartyAlert) SetProcessSuccess() {
	a.ProcessStatus = string(ProcessStatusSuccess)
	a.ErrorMessage = ""
}

// SetProcessFailed 设置处理失败状态
func (a *ThirdPartyAlert) SetProcessFailed(errMsg string) {
	a.ProcessStatus = string(ProcessStatusFailed)
	a.ErrorMessage = errMsg
}
