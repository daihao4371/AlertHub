package models

import "time"

// UserRole 用户角色表
type UserRole struct {
	ID          string    `gorm:"column:id;primary_key;type:varchar(50)" json:"id"`                      // 角色ID
	Name        string    `gorm:"column:name;type:varchar(100);not null" json:"name"`                   // 角色名称
	Description string    `gorm:"column:description;type:varchar(255)" json:"description"`              // 角色描述
	Enabled     *bool     `gorm:"column:enabled;type:tinyint(1);default:1" json:"enabled"`              // 是否启用
	UpdateAt    int64     `gorm:"column:update_at" json:"updateAt"`                                      // 更新时间戳
	CreatedAt   time.Time `gorm:"column:created_at;type:datetime;default:CURRENT_TIMESTAMP" json:"createdAt"` // 创建时间
	UpdatedAt   time.Time `gorm:"column:updated_at;type:datetime;default:CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP" json:"updatedAt"` // 更新时间

	// 关联关系 - 角色拥有的API权限
	APIs        []SysApi          `json:"apis" gorm:"many2many:user_role_apis;"`
}

// TableName 指定表名
func (UserRole) TableName() string {
	return "user_roles"
}

// GetEnabled 安全获取启用状态(防止nil指针)
func (r UserRole) GetEnabled() bool {
	if r.Enabled == nil {
		return true // 默认启用
	}
	return *r.Enabled
}

// IsEnabled 检查角色是否启用
func (r UserRole) IsEnabled() bool {
	return r.GetEnabled()
}
