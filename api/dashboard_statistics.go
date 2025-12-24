package api

import (
	"alertHub/internal/middleware"
	"alertHub/internal/services"
	"alertHub/internal/types"

	"github.com/gin-gonic/gin"
)

// dashboardStatisticsController 首页统计数据控制器
type dashboardStatisticsController struct{}

var DashboardStatisticsController = new(dashboardStatisticsController)

// API 注册首页统计相关的路由
func (dashboardStatisticsController dashboardStatisticsController) API(gin *gin.RouterGroup) {
	system := gin.Group("system")
	system.Use(
		middleware.Auth(),              // 用户认证中间件
		middleware.CasbinPermission(),  // Casbin权限验证中间件
		middleware.ParseTenant(),       // 租户解析中间件
	)
	{
		// 获取首页统计数据的API端点
		system.GET("getDashboardStatistics", dashboardStatisticsController.GetDashboardStatistics)
	}
}

// GetDashboardStatistics 获取首页统计数据
// 包括今日主告警、新增告警、过去7天MTTA/MTTR等关键指标及其环比数据
func (dashboardStatisticsController dashboardStatisticsController) GetDashboardStatistics(ctx *gin.Context) {
	r := new(types.RequestDashboardStatistics)
	BindQuery(ctx, r)

	// 从上下文中获取租户ID
	tid, _ := ctx.Get("TenantID")
	r.TenantId = tid.(string)

	// 调用业务服务层获取统计数据
	Service(ctx, func() (interface{}, interface{}) {
		return services.DashboardStatisticsService.GetDashboardStatistics(r)
	})
}