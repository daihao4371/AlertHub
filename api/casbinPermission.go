package api

import (
	"fmt"
	"github.com/gin-gonic/gin"
	middleware "alertHub/internal/middleware"
	"alertHub/internal/services"
	"alertHub/internal/types"
)

type casbinPermissionController struct{}

var CasbinPermissionController = new(casbinPermissionController)

/*
Casbin权限管理 API
/api/w8t/casbin
*/
func (casbinPermissionController casbinPermissionController) API(gin *gin.RouterGroup) {
	c := gin.Group("casbin")
	c.Use(
		middleware.Auth(),
		middleware.CasbinPermission(), // 使用Casbin权限中间件
		middleware.ParseTenant(),
		middleware.AuditingLog(),
	)
	{
		// 角色权限管理
		c.POST("setRolePermissions", casbinPermissionController.SetRolePermissions)
		c.GET("getRolePermissions", casbinPermissionController.GetRolePermissions)
		c.DELETE("removeRolePermissions", casbinPermissionController.RemoveRolePermissions)
		
		// 用户权限查询
		c.GET("getUserPermissions", casbinPermissionController.GetUserPermissions)
		c.POST("checkPermission", casbinPermissionController.CheckPermission)
		
		// 权限初始化
		c.POST("initDefaultPermissions", casbinPermissionController.InitDefaultPermissions)
		
		// API权限管理
		c.GET("getApiPermissions", casbinPermissionController.GetApiPermissions)
	}
}

// SetRolePermissions 设置角色权限
func (casbinPermissionController casbinPermissionController) SetRolePermissions(ctx *gin.Context) {
	r := new(types.RequestSetRolePermissions)
	BindJson(ctx, r)

	Service(ctx, func() (interface{}, interface{}) {
		err := services.CasbinPermissionService.SetRolePermissions(r.RoleID, r.Permissions)
		if err != nil {
			return nil, err
		}
		return "角色权限设置成功", nil
	})
}

// GetRolePermissions 获取角色权限
func (casbinPermissionController casbinPermissionController) GetRolePermissions(ctx *gin.Context) {
	r := new(types.RequestGetRolePermissions)
	BindQuery(ctx, r)

	Service(ctx, func() (interface{}, interface{}) {
		return services.CasbinPermissionService.GetRolePermissions(r.RoleID)
	})
}

// RemoveRolePermissions 移除角色权限
func (casbinPermissionController casbinPermissionController) RemoveRolePermissions(ctx *gin.Context) {
	r := new(types.RequestRemoveRolePermissions)
	BindJson(ctx, r)

	Service(ctx, func() (interface{}, interface{}) {
		err := services.CasbinPermissionService.RemoveRolePermissions(r.RoleID)
		if err != nil {
			return nil, err
		}
		return "权限移除成功", nil
	})
}

// GetUserPermissions 获取用户所有权限
func (casbinPermissionController casbinPermissionController) GetUserPermissions(ctx *gin.Context) {
	r := new(types.RequestGetUserPermissions)
	BindQuery(ctx, r)

	Service(ctx, func() (interface{}, interface{}) {
		// 获取租户ID
		tenantID := ctx.Request.Header.Get("TenantID")
		if tenantID == "" {
			return nil, fmt.Errorf("租户ID不能为空")
		}
		
		return services.CasbinPermissionService.GetUserAllPermissions(r.UserID, tenantID)
	})
}

// CheckPermission 检查权限
func (casbinPermissionController casbinPermissionController) CheckPermission(ctx *gin.Context) {
	r := new(types.RequestCheckPermission)
	BindJson(ctx, r)

	Service(ctx, func() (interface{}, interface{}) {
		// 获取租户ID
		tenantID := ctx.Request.Header.Get("TenantID")
		if tenantID == "" {
			return nil, fmt.Errorf("租户ID不能为空")
		}
		
		hasPermission, err := services.CasbinPermissionService.CheckUserPermission(r.UserID, tenantID, r.ApiPath, r.Method)
		if err != nil {
			return nil, err
		}
		return map[string]interface{}{
			"hasPermission": hasPermission,
			"userID":        r.UserID,
			"apiPath":       r.ApiPath,
			"method":        r.Method,
		}, nil
	})
}

// InitDefaultPermissions 初始化默认权限
func (casbinPermissionController casbinPermissionController) InitDefaultPermissions(ctx *gin.Context) {
	r := new(types.RequestInitDefaultPermissions)
	BindJson(ctx, r)

	Service(ctx, func() (interface{}, interface{}) {
		// 获取租户ID
		tenantID := ctx.Request.Header.Get("TenantID")
		if tenantID == "" {
			return nil, fmt.Errorf("租户ID不能为空")
		}
		
		// 1. 确保所有API已注册到SysApi表
		err := services.CasbinPermissionService.EnsureAllApisRegistered()
		if err != nil {
			return nil, fmt.Errorf("注册API失败: %v", err)
		}
		
		// 2. 为admin角色分配所有权限
		err = services.CasbinPermissionService.InitAdminPermissions()
		if err != nil {
			return nil, fmt.Errorf("初始化admin权限失败: %v", err)
		}
		
		return "默认权限初始化成功", nil
	})
}

// GetApiPermissions 获取所有API权限列表供前端分配使用
func (casbinPermissionController casbinPermissionController) GetApiPermissions(ctx *gin.Context) {
	Service(ctx, func() (interface{}, interface{}) {
		return services.CasbinPermissionService.GetAllApiPermissions()
	})
}