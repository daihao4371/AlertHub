package types

import "alertHub/internal/models"

// RequestSetRolePermissions 设置角色权限请求
type RequestSetRolePermissions struct {
	RoleID      string                    `json:"roleId" binding:"required"`
	Permissions []models.PermissionInfo   `json:"permissions" binding:"required"`
}

// RequestSetRoleApiPermissions 设置角色API权限请求（兼容旧接口）
type RequestSetRoleApiPermissions struct {
	RoleID string  `json:"roleId" binding:"required"`
	ApiIDs []int64 `json:"apiIds" binding:"required"`
}

// RequestGetRolePermissions 获取角色权限请求
type RequestGetRolePermissions struct {
	RoleID string `json:"roleId" form:"roleId" binding:"required"`
}

// RequestRemoveRolePermissions 移除角色权限请求
type RequestRemoveRolePermissions struct {
	RoleID string `json:"roleId" binding:"required"`
}

// RequestGetUserPermissions 获取用户权限请求
type RequestGetUserPermissions struct {
	UserID string `json:"userId" form:"userId" binding:"required"`
}

// RequestCheckPermission 检查权限请求
type RequestCheckPermission struct {
	UserID  string `json:"userId" binding:"required"`
	ApiPath string `json:"apiPath" binding:"required"`
	Method  string `json:"method" binding:"required"`
}

// RequestCheckUserPermission 检查用户权限请求（兼容旧接口）
type RequestCheckUserPermission struct {
	UserID  string `json:"userId" binding:"required"`
	ApiPath string `json:"apiPath" binding:"required"`
	Method  string `json:"method" binding:"required"`
}

// RequestInitDefaultPermissions 初始化默认权限请求
type RequestInitDefaultPermissions struct {
	Force bool `json:"force"` // 是否强制重新初始化
}

// ResponseRolePermissions 角色权限响应
type ResponseRolePermissions struct {
	RoleID      string                  `json:"roleId"`
	Permissions []models.PermissionInfo `json:"permissions"`
}

// ResponseUserPermissions 用户权限响应
type ResponseUserPermissions struct {
	UserID      string                  `json:"userId"`
	Permissions []models.PermissionInfo `json:"permissions"`
}

// ResponseCheckPermission 权限检查响应
type ResponseCheckPermission struct {
	HasPermission bool   `json:"hasPermission"`
	UserID        string `json:"userId"`
	ApiPath       string `json:"apiPath"`
	Method        string `json:"method"`
}