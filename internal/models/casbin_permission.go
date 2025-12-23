package models

// CasbinRule Casbin权限规则表(由Casbin自动管理)
type CasbinRule struct {
	ID    uint   `gorm:"primarykey" json:"id"`
	Ptype string `gorm:"size:50" json:"ptype"`                        // 策略类型
	V0    string `gorm:"size:50" json:"v0"`                           // 角色ID
	V1    string `gorm:"size:200" json:"v1"`                          // API路径
	V2    string `gorm:"size:10" json:"v2"`                           // HTTP方法
	V3    string `gorm:"size:50" json:"v3"`                           // 保留字段
	V4    string `gorm:"size:50" json:"v4"`                           // 保留字段
	V5    string `gorm:"size:50" json:"v5"`                           // 保留字段
}

// TableName 指定表名
func (CasbinRule) TableName() string {
	return "casbin_rule"
}

// PermissionRequest 权限请求结构
type PermissionRequest struct {
	RoleID string           `json:"roleId" binding:"required"` // 角色ID
	Apis   []PermissionInfo `json:"apis" binding:"required"`   // API权限列表
}

// PermissionInfo API权限信息
type PermissionInfo struct {
	Path   string `json:"path" binding:"required"`   // API路径
	Method string `json:"method" binding:"required"` // HTTP方法
	Group  string `json:"group"`                     // API分组(可选)
}

// DefaultPermissions 获取默认权限配置
func DefaultPermissions() []PermissionInfo {
	return []PermissionInfo{
		// 系统基础权限
		{Path: "/api/system/login", Method: "POST", Group: "认证"},
		{Path: "/api/system/logout", Method: "POST", Group: "认证"},
		{Path: "/api/system/refresh", Method: "POST", Group: "认证"},

		// 用户基础权限
		{Path: "/api/w8t/user/profile", Method: "GET", Group: "用户"},
		{Path: "/api/w8t/user/changePassword", Method: "PUT", Group: "用户"},

		// 健康检查等公共接口
		{Path: "/api/health", Method: "GET", Group: "系统"},
		{Path: "/api/version", Method: "GET", Group: "系统"},
	}
}

// GetAllApiPermissions 获取所有API权限配置(超级管理员)
// 现在从数据库动态获取，而不是硬编码
func GetAllApiPermissions() []PermissionInfo {
	// 这个函数保留是为了兼容性，但实际上应该从数据库获取
	// 在实际使用中，应该调用 CasbinPermissionService.GetAllApiPermissions()
	return []PermissionInfo{}
}

// GetBasicApiPermissions 获取基础权限配置(普通用户)
func GetBasicApiPermissions() []PermissionInfo {
	// 基础权限
	permissions := DefaultPermissions()

	// 只读权限
	basicApiPermissions := []PermissionInfo{
		{Path: "/api/w8t/user/profile", Method: "GET", Group: "用户管理"},
		{Path: "/api/w8t/user/changePassword", Method: "PUT", Group: "用户管理"},
		
		{Path: "/api/w8t/dashboard/dashboardList", Method: "GET", Group: "仪表板"},
		{Path: "/api/w8t/datasource/datasourceList", Method: "GET", Group: "数据源管理"},
		{Path: "/api/w8t/rule/ruleList", Method: "GET", Group: "告警规则"},
		
		// 告警事件查看权限
		{Path: "/api/w8t/event/*", Method: "GET", Group: "告警事件"},
	}

	permissions = append(permissions, basicApiPermissions...)
	return permissions
}
