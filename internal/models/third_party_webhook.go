package models

// WebhookStatus Webhook状态类型
type WebhookStatus string

const (
	WebhookStatusActive   WebhookStatus = "active"   // 启用状态
	WebhookStatusDisabled WebhookStatus = "disabled" // 禁用状态
)

// ThirdPartyWebhook 第三方Webhook配置模型
type ThirdPartyWebhook struct {
	TenantId string `json:"tenantId" gorm:"size:64;index"` // 租户ID
	ID       string `json:"id" gorm:"size:64"`             // 主键ID

	// 基本信息
	Name        string `json:"name" gorm:"size:255;not null"` // Webhook名称
	Description string `json:"description" gorm:"type:text"`  // 描述说明
	Source      string `json:"source" gorm:"size:50"`         // 来源系统（用户自定义，如：zabbix、nagios、custom-system等）

	// Webhook配置
	WebhookId  string `json:"webhookId" gorm:"size:64;uniqueIndex"` // 唯一随机ID（如：wh_7d8f9e2c4b1a...）
	WebhookUrl string `json:"webhookUrl" gorm:"size:512"`           // 完整的Webhook URL

	// 数据映射配置
	DataMapping string `json:"dataMapping" gorm:"type:text"` // 数据映射规则（JSON格式）
	Transform   string `json:"transform" gorm:"type:text"`   // 转换脚本（JavaScript，可选）

	// 状态和统计
	Status     string `json:"status" gorm:"size:20;default:'active'"` // 状态：active/disabled
	CallCount  int64  `json:"callCount" gorm:"default:0"`             // 调用次数统计
	LastCallAt int64  `json:"lastCallAt"`                             // 最后调用时间（Unix时间戳）

	// 配置选项
	EnableLog bool `json:"enableLog" gorm:"default:true"` // 是否记录详细日志

	// 通知配置
	NoticeIds []string `json:"noticeIds" gorm:"column:noticeIds;serializer:json"` // 关联的通知对象ID列表

	// 审计字段
	CreateAt int64  `json:"createAt"`                // 创建时间（Unix时间戳）
	UpdateAt int64  `json:"updateAt"`                // 更新时间（Unix时间戳）
	CreateBy string `json:"createBy" gorm:"size:64"` // 创建人用户ID
	UpdateBy string `json:"updateBy" gorm:"size:64"` // 更新人用户ID
}

// TableName 指定表名
func (ThirdPartyWebhook) TableName() string {
	return "third_party_webhooks"
}

// IsActive 判断Webhook是否处于启用状态
func (w *ThirdPartyWebhook) IsActive() bool {
	return w.Status == string(WebhookStatusActive)
}

// IncrementCallCount 增加调用次数
func (w *ThirdPartyWebhook) IncrementCallCount() {
	w.CallCount++
}
