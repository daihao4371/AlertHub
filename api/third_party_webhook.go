package api

import (
	"alertHub/internal/middleware"
	"alertHub/internal/services"
	"alertHub/internal/types"

	"github.com/gin-gonic/gin"
)

// thirdPartyWebhookController 第三方Webhook管理控制器
type thirdPartyWebhookController struct{}

// ThirdPartyWebhookController 控制器单例
var ThirdPartyWebhookController = new(thirdPartyWebhookController)

// API 注册路由和中间件
func (thirdPartyWebhookController thirdPartyWebhookController) API(gin *gin.RouterGroup) {
	// 写操作路由组（需要审计日志）
	webhookA := gin.Group("thirdPartyWebhook")
	webhookA.Use(
		middleware.Auth(),
		middleware.CasbinPermission(),
		middleware.ParseTenant(),
		middleware.AuditingLog(),
	)
	{
		webhookA.POST("create", thirdPartyWebhookController.Create)
		webhookA.POST("update", thirdPartyWebhookController.Update)
		webhookA.POST("delete", thirdPartyWebhookController.Delete)
	}

	// 读操作路由组（不需要审计日志）
	webhookB := gin.Group("thirdPartyWebhook")
	webhookB.Use(
		middleware.Auth(),
		middleware.CasbinPermission(),
		middleware.ParseTenant(),
	)
	{
		webhookB.GET("get", thirdPartyWebhookController.Get)
		webhookB.GET("list", thirdPartyWebhookController.List)
	}
}

// Create 创建Webhook配置
func (thirdPartyWebhookController thirdPartyWebhookController) Create(ctx *gin.Context) {
	r := new(types.RequestWebhookCreate)
	BindJson(ctx, r)

	// 从中间件获取租户ID
	tid, _ := ctx.Get("TenantID")
	r.TenantId = tid.(string)

	// 调用服务层
	Service(ctx, func() (interface{}, interface{}) {
		return services.ThirdPartyWebhookService.Create(r)
	})
}

// Update 更新Webhook配置
func (thirdPartyWebhookController thirdPartyWebhookController) Update(ctx *gin.Context) {
	r := new(types.RequestWebhookUpdate)
	BindJson(ctx, r)

	// 从中间件获取租户ID
	tid, _ := ctx.Get("TenantID")
	r.TenantId = tid.(string)

	// 调用服务层
	Service(ctx, func() (interface{}, interface{}) {
		return services.ThirdPartyWebhookService.Update(r)
	})
}

// Delete 删除Webhook配置
func (thirdPartyWebhookController thirdPartyWebhookController) Delete(ctx *gin.Context) {
	r := new(types.RequestWebhookDelete)
	BindJson(ctx, r)

	// 从中间件获取租户ID
	tid, _ := ctx.Get("TenantID")
	r.TenantId = tid.(string)

	// 调用服务层
	Service(ctx, func() (interface{}, interface{}) {
		return services.ThirdPartyWebhookService.Delete(r)
	})
}

// Get 获取单个Webhook配置详情
func (thirdPartyWebhookController thirdPartyWebhookController) Get(ctx *gin.Context) {
	r := new(types.RequestWebhookQuery)
	BindQuery(ctx, r)

	// 从中间件获取租户ID
	tid, _ := ctx.Get("TenantID")
	r.TenantId = tid.(string)

	// 调用服务层
	Service(ctx, func() (interface{}, interface{}) {
		return services.ThirdPartyWebhookService.Get(r)
	})
}

// List 分页查询Webhook配置列表
func (thirdPartyWebhookController thirdPartyWebhookController) List(ctx *gin.Context) {
	r := new(types.RequestWebhookQuery)
	BindQuery(ctx, r)

	// 从中间件获取租户ID
	tid, _ := ctx.Get("TenantID")
	r.TenantId = tid.(string)

	// 调用服务层
	Service(ctx, func() (interface{}, interface{}) {
		return services.ThirdPartyWebhookService.List(r)
	})
}
