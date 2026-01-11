package types

import "alertHub/internal/models"

// RequestWebhookCreate 创建Webhook请求
type RequestWebhookCreate struct {
	TenantId    string   `json:"tenantId"`    // 租户ID
	Name        string   `json:"name"`        // Webhook名称
	Description string   `json:"description"` // 描述说明
	Source      string   `json:"source"`      // 来源系统
	DataMapping string   `json:"dataMapping"` // 数据映射规则（JSON格式）
	Transform   string   `json:"transform"`   // 转换脚本（可选）
	EnableLog   bool     `json:"enableLog"`   // 是否记录详细日志
	NoticeIds   []string `json:"noticeIds"`   // 关联的通知对象ID列表
	CreateBy    string   `json:"createBy"`    // 创建人用户ID
}

// RequestWebhookUpdate 更新Webhook请求
type RequestWebhookUpdate struct {
	TenantId    string   `json:"tenantId"`    // 租户ID
	ID          string   `json:"id"`          // Webhook ID
	Name        string   `json:"name"`        // Webhook名称
	Description string   `json:"description"` // 描述说明
	Source      string   `json:"source"`      // 来源系统
	DataMapping string   `json:"dataMapping"` // 数据映射规则（JSON格式）
	Transform   string   `json:"transform"`   // 转换脚本（可选）
	Status      string   `json:"status"`      // 状态：active/disabled
	EnableLog   bool     `json:"enableLog"`   // 是否记录详细日志
	NoticeIds   []string `json:"noticeIds"`   // 关联的通知对象ID列表
	UpdateBy    string   `json:"updateBy"`    // 更新人用户ID
}

// RequestWebhookQuery 查询Webhook请求
type RequestWebhookQuery struct {
	TenantId  string `json:"tenantId" form:"tenantId"`   // 租户ID
	ID        string `json:"id" form:"id"`               // Webhook ID
	WebhookId string `json:"webhookId" form:"webhookId"` // Webhook随机ID
	Source    string `json:"source" form:"source"`       // 来源系统过滤
	Status    string `json:"status" form:"status"`       // 状态过滤
	Query     string `json:"query" form:"query"`         // 关键词搜索
	models.Page
}

// RequestWebhookDelete 删除Webhook请求
type RequestWebhookDelete struct {
	TenantId string `json:"tenantId"` // 租户ID
	ID       string `json:"id"`       // Webhook ID
}

// ResponseWebhook Webhook响应
type ResponseWebhook struct {
	ID          string   `json:"id"`          // 主键ID
	Name        string   `json:"name"`        // Webhook名称
	Description string   `json:"description"` // 描述说明
	Source      string   `json:"source"`      // 来源系统
	WebhookId   string   `json:"webhookId"`   // 唯一随机ID
	WebhookUrl  string   `json:"webhookUrl"`  // 完整的Webhook URL
	Status      string   `json:"status"`      // 状态
	CallCount   int64    `json:"callCount"`   // 调用次数
	LastCallAt  int64    `json:"lastCallAt"`  // 最后调用时间
	NoticeIds   []string `json:"noticeIds"`   // 关联的通知对象ID列表
	CreateAt    int64    `json:"createAt"`    // 创建时间
	UpdateAt    int64    `json:"updateAt"`    // 更新时间
}

// ResponseWebhookList Webhook列表响应
type ResponseWebhookList struct {
	List  []ResponseWebhook `json:"list"`  // Webhook列表
	Total int64             `json:"total"` // 总数
	models.Page
}

// RequestAlertQuery 第三方告警查询请求
type RequestAlertQuery struct {
	TenantId      string `json:"tenantId" form:"tenantId"`           // 租户ID
	WebhookId     string `json:"webhookId" form:"webhookId"`         // Webhook配置ID
	ProcessStatus string `json:"processStatus" form:"processStatus"` // 处理状态过滤
	Status        string `json:"status" form:"status"`               // 告警状态过滤
	models.Page
}

// ResponseAlert 第三方告警响应
type ResponseAlert struct {
	ID            string `json:"id"`            // 主键ID
	WebhookId     string `json:"webhookId"`     // 关联的Webhook配置ID
	AlertId       string `json:"alertId"`       // 第三方告警ID
	Fingerprint   string `json:"fingerprint"`   // 告警指纹
	Title         string `json:"title"`         // 告警标题
	Content       string `json:"content"`       // 告警内容
	Severity      string `json:"severity"`      // 严重级别
	Status        string `json:"status"`        // 告警状态
	Source        string `json:"source"`        // 来源系统
	Host          string `json:"host"`          // 主机信息
	Service       string `json:"service"`       // 服务信息
	SourceTime    int64  `json:"sourceTime"`    // 第三方系统时间
	ProcessTime   int64  `json:"processTime"`   // 处理时间
	ProcessStatus string `json:"processStatus"` // 处理状态
	ErrorMessage  string `json:"errorMessage"`  // 错误信息
	FaultCenterId string `json:"faultCenterId"` // 关联的故障中心ID
	EventId       string `json:"eventId"`       // 关联的告警事件ID
}

// ResponseAlertList 第三方告警列表响应
type ResponseAlertList struct {
	List  []ResponseAlert `json:"list"`  // 告警列表
	Total int64           `json:"total"` // 总数
	models.Page
}
