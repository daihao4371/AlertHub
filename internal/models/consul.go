package models

import "time"

// ConsulTarget 从 Consul 发现的目标机器追踪表
type ConsulTarget struct {
	ID                 int64                  `gorm:"primaryKey" json:"id"`
	TenantId           string                 `gorm:"uniqueIndex:idx_tenant_service_id;index;type:varchar(128)" json:"tenantId"`                // 租户ID (同时是唯一索引的一部分)
	Instance           string                 `gorm:"index;type:varchar(255)" json:"instance"`              // 实例标识，如 "192.168.1.100:9100"
	Job                string                 `gorm:"type:varchar(128)" json:"job"`                        // Job 名称，如 "node"、"mysql"
	Labels             map[string]interface{} `gorm:"serializer:json" json:"labels"`                        // 标签信息 (Prometheus Labels)
	ServiceID          string                 `gorm:"uniqueIndex:idx_tenant_service_id;type:varchar(255)" json:"serviceId"` // Consul ServiceID (租户内唯一标识，指定长度避免索引错误)
	ServiceName        string                 `gorm:"type:varchar(255)" json:"serviceName"`               // Consul Service Name
	Status             string                 `gorm:"type:varchar(64)" json:"status"`                      // 状态: "passing" (正常) / "warning" (警告) / "critical" (严重) / "no checks" (无检查)
	ConsulDeregistered bool                   `gorm:"column:consul_deregistered" json:"consulDeregistered"` // 是否已从 Consul 中删除
	DeregistrationTime *time.Time             `json:"deregistrationTime"`                                   // 注销时间戳
	CreatedAt          time.Time              `json:"createdAt"`
	UpdatedAt          time.Time              `json:"updatedAt"`
}

// TableName 指定表名
func (ConsulTarget) TableName() string {
	return "consul_target"
}

// ConsulTargetOfflineLog 机器注销历史记录
type ConsulTargetOfflineLog struct {
	ID                 int64                  `gorm:"primaryKey" json:"id"`
	TenantId           string                 `gorm:"index;type:varchar(128)" json:"tenantId"`              // 租户ID
	Instance           string                 `gorm:"index;type:varchar(255)" json:"instance"`             // 实例标识
	Job                string                 `gorm:"type:varchar(128)" json:"job"`                        // Job 名称
	Labels             map[string]interface{} `gorm:"serializer:json" json:"labels"`                       // 标签信息
	Reason             string                 `gorm:"type:varchar(500)" json:"reason"`                     // 注销原因 (如 "主机宕机"、"下线维护")
	DeregisteredBy     string                 `gorm:"type:varchar(128)" json:"deregisteredBy"`             // 操作人用户ID
	AlertEventsCleared int                    `json:"alertEventsCleared"`                                   // 同时清理的告警事件数量
	CreatedAt          time.Time              `json:"createdAt"`
}

// TableName 指定表名
func (ConsulTargetOfflineLog) TableName() string {
	return "consul_target_offline_log"
}
