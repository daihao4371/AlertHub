package middleware

import (
	"alertHub/internal/ctx"
	"alertHub/internal/models"
	"alertHub/internal/services"
	"alertHub/pkg/response"
	utils2 "alertHub/pkg/tools"
	"fmt"

	"github.com/gin-gonic/gin"
	"github.com/zeromicro/go-zero/core/logc"
	"gorm.io/gorm"
)

// CasbinPermission Casbin权限验证中间件
func CasbinPermission() gin.HandlerFunc {
	return func(context *gin.Context) {
		// 检查租户ID
		tid := context.Request.Header.Get(TenantIDHeaderKey)
		if tid == "null" || tid == "" {
			return
		}

		// 获取Token
		tokenStr := context.Request.Header.Get("Authorization")
		if tokenStr == "" {
			response.TokenFail(context)
			context.Abort()
			return
		}

		userId := utils2.GetUserID(tokenStr)
		c := ctx.DO()

		// 获取当前用户
		var user models.Member
		err := c.DB.DB().Model(&models.Member{}).Where("user_id = ?", userId).First(&user).Error
		if gorm.ErrRecordNotFound == err {
			logc.Errorf(c.Ctx, fmt.Sprintf("用户不存在, uid: %s", userId))
		}
		if err != nil {
			response.PermissionFail(context)
			context.Abort()
			return
		}

		// 设置用户信息到上下文
		context.Set("UserId", user.UserId)
		context.Set("UserEmail", user.Email)

		// 获取租户用户角色
		tenantUserInfo, err := c.DB.Tenant().GetTenantLinkedUserInfo(tid, userId)
		if err != nil {
			logc.Errorf(c.Ctx, fmt.Sprintf("获取租户用户角色失败 %s", err.Error()))
			response.TokenFail(context)
			context.Abort()
			return
		}

		// 检查用户角色是否存在
		var role models.UserRole
		err = c.DB.DB().Model(&models.UserRole{}).Where("id = ?", tenantUserInfo.UserRole).First(&role).Error
		if err != nil {
			response.Fail(context, fmt.Sprintf("获取用户 %s 的角色失败, %s %s", user.UserName, tenantUserInfo.UserRole, err.Error()), "failed")
			logc.Errorf(c.Ctx, fmt.Sprintf("获取用户 %s 的角色失败 %s %s", user.UserName, tenantUserInfo.UserRole, err.Error()))
			context.Abort()
			return
		}

		// 获取请求信息
		apiPath := context.Request.URL.Path
		method := context.Request.Method

		// 使用Casbin进行权限验证
		hasPermission, err := services.CasbinPermissionService.CheckUserPermission(userId, tid, apiPath, method)
		if err != nil {
			logc.Errorf(c.Ctx, fmt.Sprintf("Casbin权限验证失败: %s", err.Error()))
			response.Fail(context, "权限验证失败", "failed")
			context.Abort()
			return
		}

		if !hasPermission {
			logc.Infof(c.Ctx, fmt.Sprintf("用户 %s 没有访问 %s %s 的权限", userId, method, apiPath))
			response.PermissionFail(context)
			context.Abort()
			return
		}

		// 权限验证通过，继续处理请求
		context.Next()
	}
}

// CasbinRolePermission 基于角色的权限验证中间件(简化版)
func CasbinRolePermission() gin.HandlerFunc {
	return func(context *gin.Context) {
		// 检查租户ID
		tid := context.Request.Header.Get(TenantIDHeaderKey)
		if tid == "null" || tid == "" {
			return
		}

		// 获取Token
		tokenStr := context.Request.Header.Get("Authorization")
		if tokenStr == "" {
			response.TokenFail(context)
			context.Abort()
			return
		}

		userId := utils2.GetUserID(tokenStr)
		c := ctx.DO()

		// 获取租户用户角色
		tenantUserInfo, err := c.DB.Tenant().GetTenantLinkedUserInfo(tid, userId)
		if err != nil {
			logc.Errorf(c.Ctx, fmt.Sprintf("获取租户用户角色失败 %s", err.Error()))
			response.TokenFail(context)
			context.Abort()
			return
		}

		// 获取请求信息
		apiPath := context.Request.URL.Path
		method := context.Request.Method

		// 直接使用角色ID进行权限验证
		hasPermission, err := services.CasbinPermissionService.CheckPermission(tenantUserInfo.UserRole, apiPath, method)
		if err != nil {
			logc.Errorf(c.Ctx, fmt.Sprintf("Casbin角色权限验证失败: %s", err.Error()))
			response.Fail(context, "权限验证失败", "failed")
			context.Abort()
			return
		}

		if !hasPermission {
			logc.Infof(c.Ctx, fmt.Sprintf("角色 %s 没有访问 %s %s 的权限", tenantUserInfo.UserRole, method, apiPath))
			response.PermissionFail(context)
			context.Abort()
			return
		}

		// 设置角色信息到上下文
		context.Set("RoleId", tenantUserInfo.UserRole)
		context.Set("UserId", userId)

		// 权限验证通过，继续处理请求
		context.Next()
	}
}
