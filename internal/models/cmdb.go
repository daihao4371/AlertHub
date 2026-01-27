package models

import "time"

// CmdbHost CMDB主机表模型
// 表名: cmdb_hosts (GORM默认规则会自动转换为复数形式)
type CmdbHost struct {
	ID        int64     `gorm:"column:id;primary_key;AUTO_INCREMENT" json:"id"`
	IP        string    `gorm:"column:ip;type:varchar(50);not null;uniqueIndex:uk_ip" json:"ip"`
	CreatedAt time.Time `gorm:"column:created_at;type:datetime;default:CURRENT_TIMESTAMP" json:"createdAt"`
	UpdatedAt time.Time `gorm:"column:updated_at;type:datetime;default:CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP" json:"updatedAt"`
}

// CmdbHostApplication CMDB主机应用关联表模型
// 表名: cmdb_host_applications (GORM默认规则会自动转换为复数形式)
type CmdbHostApplication struct {
	ID         int64     `gorm:"column:id;primary_key;AUTO_INCREMENT" json:"id"`
	HostID     int64     `gorm:"column:host_id;type:bigint;not null;uniqueIndex:uk_host_app" json:"hostId"`
	AppName    string    `gorm:"column:app_name;type:varchar(200);not null;uniqueIndex:uk_host_app" json:"appName"`
	OpsOwner   *string   `gorm:"column:ops_owner;type:varchar(100);index:idx_ops_owner" json:"opsOwner"`
	DevOwner   *string   `gorm:"column:dev_owner;type:varchar(100);index:idx_dev_owner" json:"devOwner"`
	DingDingId *string   `gorm:"column:dingding_id;type:varchar(100);index:idx_dingding_id" json:"dingDingId"` // 钉钉用户ID
	CreatedAt  time.Time `gorm:"column:created_at;type:datetime;default:CURRENT_TIMESTAMP" json:"createdAt"`
	UpdatedAt  time.Time `gorm:"column:updated_at;type:datetime;default:CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP" json:"updatedAt"`
}

// CmdbHostInfo CMDB主机信息（包含应用信息）
// 用于返回查询结果，包含主机及其关联的应用信息
type CmdbHostInfo struct {
	HostID              int64    `json:"hostId"`              // 主机ID
	IP                  string   `json:"ip"`                  // 主机IP
	AppNames            []string `json:"appNames"`            // 应用名称列表
	OpsOwners           []string `json:"opsOwners"`           // 运维负责人列表（去重）
	DevOwners           []string `json:"devOwners"`           // 开发负责人列表（去重）
	OpsOwnerDingDingIds []string `json:"opsOwnerDingDingIds"` // 运维负责人钉钉ID列表（去重）
	DevOwnerDingDingIds []string `json:"devOwnerDingDingIds"` // 开发负责人钉钉ID列表（去重）
}
