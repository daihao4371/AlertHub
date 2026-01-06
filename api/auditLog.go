package api

import (
	middleware "alertHub/internal/middleware"
	"alertHub/internal/services"
	"alertHub/internal/types"
	"github.com/gin-gonic/gin"
)

type auditLogController struct{}

var AuditLogController = new(auditLogController)

func (auditLogController auditLogController) API(gin *gin.RouterGroup) {
	a := gin.Group("auditLog")
	a.Use(
		middleware.Auth(),
		middleware.CasbinPermission(), // 使用Casbin权限中间件
		middleware.ParseTenant(),
		middleware.AuditingLog(),
	)
	{
		a.GET("listAuditLog", auditLogController.List)
		a.GET("searchAuditLog", auditLogController.Search)
	}
}

func (auditLogController auditLogController) List(ctx *gin.Context) {
	r := new(types.RequestAuditLogQuery)
	BindQuery(ctx, r)
	tid, _ := ctx.Get("TenantID")
	r.TenantId = tid.(string)
	Service(ctx, func() (interface{}, interface{}) {
		return services.AuditLogService.List(r)
	})
}

func (auditLogController auditLogController) Search(ctx *gin.Context) {
	r := new(types.RequestAuditLogQuery)
	BindQuery(ctx, r)

	tid, _ := ctx.Get("TenantID")
	r.TenantId = tid.(string)

	Service(ctx, func() (interface{}, interface{}) {
		return services.AuditLogService.Search(r)
	})
}
