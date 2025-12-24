package api

import (
	"fmt"
	"github.com/gin-gonic/gin"
	middleware "alertHub/internal/middleware"
	"alertHub/internal/services"
	"alertHub/internal/types"
)

type userRoleController struct{}

var UserRoleController = new(userRoleController)

/*
用户角色 API
/api/w8t/role
*/
func (userRoleController userRoleController) API(gin *gin.RouterGroup) {
	a := gin.Group("role")
	a.Use(
		middleware.Auth(),
		middleware.CasbinPermission(), // 改为使用Casbin权限验证
		middleware.ParseTenant(),
		middleware.AuditingLog(),
	)
	{
		a.POST("roleCreate", userRoleController.Create)
		a.POST("roleUpdate", userRoleController.Update)
		a.POST("roleDelete", userRoleController.Delete)
		
		// 角色权限管理接口
		a.POST("setRolePermissions", userRoleController.SetRolePermissions)
		a.GET("getRolePermissions", userRoleController.GetRolePermissions)
		a.POST("checkUserPermission", userRoleController.CheckUserPermission)
		a.GET("getUserRoles", userRoleController.GetUserRoles)
	}

	b := gin.Group("role")
	b.Use(
		middleware.Auth(),
		middleware.CasbinPermission(), // 改为使用Casbin权限验证
		middleware.ParseTenant(),
	)
	{
		b.GET("roleList", userRoleController.List)
	}
}

func (userRoleController userRoleController) Create(ctx *gin.Context) {
	r := new(types.RequestUserRoleCreate)
	BindJson(ctx, r)

	Service(ctx, func() (interface{}, interface{}) {
		return services.UserRoleService.Create(r)
	})
}

func (userRoleController userRoleController) Update(ctx *gin.Context) {
	r := new(types.RequestUserRoleUpdate)
	BindJson(ctx, r)

	Service(ctx, func() (interface{}, interface{}) {
		return services.UserRoleService.Update(r)
	})
}

func (userRoleController userRoleController) Delete(ctx *gin.Context) {
	r := new(types.RequestUserRoleQuery)
	BindJson(ctx, r)

	Service(ctx, func() (interface{}, interface{}) {
		return services.UserRoleService.Delete(r)
	})
}

func (userRoleController userRoleController) List(ctx *gin.Context) {
	r := new(types.RequestUserRoleQuery)
	BindQuery(ctx, r)

	Service(ctx, func() (interface{}, interface{}) {
		return services.UserRoleService.List(r)
	})
}

// SetRolePermissions 设置角色权限
func (userRoleController userRoleController) SetRolePermissions(ctx *gin.Context) {
	r := new(types.RequestSetRoleApiPermissions)
	BindJson(ctx, r)

	Service(ctx, func() (interface{}, interface{}) {
		err := services.UserRoleService.SetRolePermissions(r.RoleID, r.ApiIDs)
		if err != nil {
			return nil, err
		}
		return "角色权限设置成功", nil
	})
}

// GetRolePermissions 获取角色权限
func (userRoleController userRoleController) GetRolePermissions(ctx *gin.Context) {
	r := new(types.RequestGetRolePermissions)
	BindQuery(ctx, r)

	Service(ctx, func() (interface{}, interface{}) {
		return services.UserRoleService.GetRolePermissions(r.RoleID)
	})
}

// CheckUserPermission 检查用户权限
func (userRoleController userRoleController) CheckUserPermission(ctx *gin.Context) {
	r := new(types.RequestCheckUserPermission)
	BindJson(ctx, r)

	Service(ctx, func() (interface{}, interface{}) {
		return services.UserRoleService.CheckUserPermission(r.UserID, r.ApiPath, r.Method)
	})
}

// GetUserRoles 获取用户角色列表
func (userRoleController userRoleController) GetUserRoles(ctx *gin.Context) {
	r := new(types.RequestGetUserRoles)
	BindQuery(ctx, r)

	Service(ctx, func() (interface{}, interface{}) {
		// 获取租户ID
		tenantID := ctx.Request.Header.Get("TenantID")
		if tenantID == "" {
			return nil, fmt.Errorf("租户ID不能为空")
		}
		
		return services.UserRoleService.GetUserRolesInTenant(r.UserID, tenantID)
	})
}
