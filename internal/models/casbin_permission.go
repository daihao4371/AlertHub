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
func GetAllApiPermissions() []PermissionInfo {
	var permissions []PermissionInfo

	// 基础系统权限
	permissions = append(permissions, DefaultPermissions()...)

	// API管理权限模块定义
	apiModules := map[string][]string{
		"用户管理": {"/api/w8t/user/*"},
		"角色管理": {"/api/w8t/role/*"},
		"数据源管理": {"/api/w8t/datasource/*"},
		"告警规则": {"/api/w8t/rule/*", "/api/w8t/ruleGroup/*", "/api/w8t/ruleTmpl/*", "/api/w8t/ruleTmplGroup/*"},
		"仪表板": {"/api/w8t/dashboard/*"},
		"告警通知": {"/api/w8t/notice/*", "/api/w8t/noticeTmpl/*", "/api/w8t/noticeTemplate/*"},
		"告警管理": {"/api/w8t/silence/*"},
		"值班管理": {"/api/w8t/duty/*", "/api/w8t/dutyManage/*"},
		"拨测管理": {"/api/w8t/probing/*"},
		"故障中心": {"/api/w8t/faultCenter/*"},
		"租户管理": {"/api/w8t/tenant/*"},
		"系统设置": {"/api/w8t/setting/*"},
		"监控查询": {"/api/w8t/prometheus/*", "/api/w8t/metrics/*"},
		"权限管理": {"/api/w8t/casbin/*"},
		"导出监控": {"/api/w8t/exporter/*"},
		"订阅管理": {"/api/w8t/subscribe/*"},
	}

	// 标准HTTP方法
	methods := []string{"GET", "POST", "PUT", "DELETE"}

	// 生成所有权限组合
	for group, paths := range apiModules {
		for _, path := range paths {
			for _, method := range methods {
				// 对于监控查询模块，只需要GET和POST方法
				if group == "监控查询" && (method == "PUT" || method == "DELETE") {
					continue
				}
				permissions = append(permissions, PermissionInfo{
					Path:   path,
					Method: method,
					Group:  group,
				})
			}
		}
	}

	return permissions
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
