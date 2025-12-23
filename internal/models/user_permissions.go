package models

import "time"

// SysApi represents system API permissions in database
type SysApi struct {
	ID          int64     `gorm:"column:id;primary_key;AUTO_INCREMENT" json:"id"`
	Path        string    `gorm:"column:path;type:varchar(255);not null;uniqueIndex:uk_path_method" json:"path"`             // API path
	Description string    `gorm:"column:description;type:varchar(255);comment:API description" json:"description"`          // API description
	ApiGroup    string    `gorm:"column:api_group;type:varchar(100);comment:API group;index:idx_api_group" json:"apiGroup"` // API group for categorization
	Method      string    `gorm:"column:method;type:varchar(10);not null;default:POST;comment:HTTP method;uniqueIndex:uk_path_method" json:"method"` // HTTP method: GET, POST, PUT, DELETE
	Enabled     *bool     `gorm:"column:enabled;type:tinyint(1);default:1;comment:Permission enabled status" json:"enabled"`  // Enable/disable permission
	CreatedAt   time.Time `gorm:"column:created_at;type:datetime;default:CURRENT_TIMESTAMP" json:"createdAt"`
	UpdatedAt   time.Time `gorm:"column:updated_at;type:datetime;default:CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP" json:"updatedAt"`
}

// TableName specifies the table name
func (SysApi) TableName() string {
	return "sys_apis"
}

// GetEnabled safely gets the Enabled field (prevents nil pointer)
func (s SysApi) GetEnabled() bool {
	if s.Enabled == nil {
		return true // Default enabled
	}
	return *s.Enabled
}

// UserRoleApi 用户角色与API权限关联表
type UserRoleApi struct {
	RoleID string `gorm:"column:user_role_id;type:varchar(50);not null;index" json:"roleId"` // 角色ID
	ApiID  int64  `gorm:"column:sys_api_id;not null;index" json:"apiId"`                     // API权限ID
}

// TableName 指定关联表名
func (UserRoleApi) TableName() string {
	return "user_role_apis"
}
