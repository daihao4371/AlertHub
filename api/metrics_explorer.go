package api

import (
	"alertHub/internal/middleware"
	"alertHub/internal/services"
	"alertHub/internal/types"
	"alertHub/pkg/response"

	"github.com/gin-gonic/gin"
	"github.com/zeromicro/go-zero/core/logc"
)

type metricsExplorerController struct{}

var MetricsExplorerController = new(metricsExplorerController)

// API 注册路由
func (c *metricsExplorerController) API(gin *gin.RouterGroup) {
	group := gin.Group("metrics-explorer")
	group.Use(
		middleware.Auth(),
		middleware.CasbinPermission(), // 使用Casbin权限中间件
		middleware.ParseTenant(),
		middleware.AuditingLog(),
	)
	{
		// 指标浏览器核心接口
		group.GET("metrics", c.GetMetrics)              // 分页获取指标列表
		group.GET("categories", c.GetCategories)        // 获取指标分类
		group.POST("query_range", c.QueryRangeEnhanced) // 增强查询范围
	}
}

// GetMetrics 分页获取指标列表
// @Summary 分页获取指标列表
// @Description Metrics Explorer - 支持搜索和分页的指标列表查询
// @Tags Metrics Explorer
// @Accept json
// @Produce json
// @Param datasourceId query string true "数据源 ID"
// @Param page query int false "页码(从1开始,默认1)"
// @Param size query int false "页大小(默认20,最大100)"
// @Param search query string false "搜索关键词"
// @Success 200 {object} types.ApiResponse{data=types.MetricsExplorerMetricsResponse}
// @Router /api/w8t/metrics-explorer/metrics [get]
func (c *metricsExplorerController) GetMetrics(ctx *gin.Context) {
	var req types.MetricsExplorerMetricsRequest

	if err := ctx.ShouldBindQuery(&req); err != nil {
		response.Fail(ctx, err.Error(), "参数绑定失败")
		return
	}

	result, err := services.MetricsExplorerService.GetMetricsWithPagination(&req)
	if err != nil {
		logc.Errorf(ctx, "获取指标列表失败: %v", err)
		response.Fail(ctx, err.Error(), "获取指标列表失败")
		return
	}

	response.Success(ctx, result, "获取成功")
}

// GetCategories 获取指标分类
// @Summary 获取指标分类统计
// @Description Metrics Explorer - 按前缀对指标进行智能分类统计
// @Tags Metrics Explorer
// @Accept json
// @Produce json
// @Param datasourceId query string true "数据源 ID"
// @Success 200 {object} types.ApiResponse{data=[]types.MetricsCategory}
// @Router /api/w8t/metrics-explorer/categories [get]
func (c *metricsExplorerController) GetCategories(ctx *gin.Context) {
	var req types.MetricsExplorerCategoriesRequest

	if err := ctx.ShouldBindQuery(&req); err != nil {
		response.Fail(ctx, err.Error(), "参数绑定失败")
		return
	}

	categories, err := services.MetricsExplorerService.GetMetricsCategories(&req)
	if err != nil {
		logc.Errorf(ctx, "获取指标分类失败: %v", err)
		response.Fail(ctx, err.Error(), "获取指标分类失败")
		return
	}

	response.Success(ctx, categories, "获取成功")
}

// QueryRangeEnhanced 增强查询范围
// @Summary 增强的时间范围查询
// @Description Metrics Explorer - 支持自动步长计算的时间范围查询
// @Tags Metrics Explorer
// @Accept json
// @Produce json
// @Param request body types.MetricsExplorerQueryRangeRequest true "查询参数"
// @Success 200 {object} types.ApiResponse{data=types.MetricsExplorerQueryRangeResponse}
// @Router /api/w8t/metrics-explorer/query_range [post]
func (c *metricsExplorerController) QueryRangeEnhanced(ctx *gin.Context) {
	var req types.MetricsExplorerQueryRangeRequest

	if err := ctx.ShouldBindJSON(&req); err != nil {
		response.Fail(ctx, err.Error(), "参数绑定失败")
		return
	}

	result, err := services.MetricsExplorerService.QueryRangeEnhanced(&req)
	if err != nil {
		logc.Errorf(ctx, "查询范围失败: %v", err)
		response.Fail(ctx, err.Error(), "查询范围失败")
		return
	}

	response.Success(ctx, result, "查询成功")
}
