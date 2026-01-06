package api

import (
	middleware "alertHub/internal/middleware"
	"alertHub/internal/services"
	"alertHub/internal/types"
	"github.com/gin-gonic/gin"
)

type clientController struct{}

var ClientController = new(clientController)

func (clientController clientController) API(gin *gin.RouterGroup) {
	a := gin.Group("c")
	a.Use(
		middleware.Auth(),
		middleware.CasbinPermission(), // 使用Casbin权限中间件
		middleware.ParseTenant(),
		middleware.AuditingLog(),
	)
	{
		a.GET("getJaegerService", clientController.GetJaegerService)
	}
}

func (clientController clientController) GetJaegerService(ctx *gin.Context) {
	r := new(types.RequestDatasourceQuery)
	BindQuery(ctx, r)

	Service(ctx, func() (interface{}, interface{}) {
		return services.ClientService.GetJaegerService(r)
	})
}
