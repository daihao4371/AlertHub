package api

import (
	"alertHub/internal/middleware"
	"alertHub/internal/services"
	"alertHub/internal/types"
	"alertHub/pkg/response"

	"github.com/gin-gonic/gin"
	"github.com/zeromicro/go-zero/core/logc"
)

type prometheusProxyController struct{}

var PrometheusProxyController = new(prometheusProxyController)

// API 注册路由
// PromQL 编辑器代理接口 - 需要认证和权限
func (c *prometheusProxyController) API(gin *gin.RouterGroup) {
	group := gin.Group("prometheus")
	group.Use(
		middleware.Auth(),
		middleware.CasbinPermission(), // 使用Casbin权限中间件
		middleware.ParseTenant(),
		middleware.AuditingLog(),
	)
	{
		// PromQL 编辑器自动补全接口
		group.GET("labels", c.GetLabelNames)        // 获取标签名称列表
		group.GET("label_values", c.GetLabelValues) // 获取标签值列表
		group.GET("metrics", c.GetMetricNames)      // 获取指标名称列表
		group.POST("series", c.GetSeries)           // 获取时间序列元数据
	}
}

// GetLabelNames 获取 Prometheus 标签名称列表
// @Summary 获取标签名称列表
// @Description PromQL 编辑器 - 获取所有标签名称,用于自动补全
// @Tags PromQL 编辑器
// @Accept json
// @Produce json
// @Param datasourceId query string true "数据源 ID"
// @Param metricName query string false "可选,限定指标范围"
// @Param start query int64 false "可选,查询起始时间(Unix时间戳)"
// @Param end query int64 false "可选,查询结束时间(Unix时间戳)"
// @Success 200 {object} types.ApiResponse{data=[]string}
// @Router /api/w8t/prometheus/labels [get]
func (c *prometheusProxyController) GetLabelNames(ctx *gin.Context) {
	var req types.PrometheusProxyLabelNamesRequest

	// 绑定查询参数
	if err := ctx.ShouldBindQuery(&req); err != nil {
		response.Fail(ctx, err.Error(), "failed")
		return
	}

	// 调用 Service 层
	labels, err := services.PrometheusProxyService.GetLabelNames(&req)
	if err != nil {
		response.Fail(ctx, err.Error(), "failed")
		return
	}

	// 返回标准 Prometheus API 格式
	response.Success(ctx, map[string]interface{}{
		"status": "success",
		"data":   labels,
	}, "success")
}

// GetLabelValues 获取指定标签的值列表
// @Summary 获取标签值列表
// @Description PromQL 编辑器 - 获取指定标签的所有值,用于自动补全
// @Tags PromQL 编辑器
// @Accept json
// @Produce json
// @Param datasourceId query string true "数据源 ID"
// @Param labelName query string true "标签名称 (如 job, instance)"
// @Param metricName query string false "可选,限定指标范围"
// @Param start query int64 false "可选,查询起始时间(Unix时间戳)"
// @Param end query int64 false "可选,查询结束时间(Unix时间戳)"
// @Success 200 {object} types.ApiResponse{data=[]string}
// @Router /api/w8t/prometheus/label_values [get]
func (c *prometheusProxyController) GetLabelValues(ctx *gin.Context) {
	var req types.PrometheusProxyLabelValuesRequest

	// 绑定查询参数
	if err := ctx.ShouldBindQuery(&req); err != nil {
		response.Fail(ctx, err.Error(), "failed")
		return
	}

	// 调用 Service 层
	values, err := services.PrometheusProxyService.GetLabelValues(&req)
	if err != nil {
		logc.Errorf(ctx, "[PromQL 补全] 获取标签值失败: datasourceId=%s, labelName=%s, metricName=%s, err=%v",
			req.DatasourceID, req.LabelName, req.MetricName, err)
		response.Fail(ctx, err.Error(), "failed")
		return
	}

	// 返回标准 Prometheus API 格式
	response.Success(ctx, map[string]interface{}{
		"status": "success",
		"data":   values,
	}, "success")
}

// GetMetricNames 获取所有指标名称列表
// @Summary 获取指标名称列表
// @Description PromQL 编辑器 - 获取所有指标名称,用于自动补全
// @Tags PromQL 编辑器
// @Accept json
// @Produce json
// @Param datasourceId query string true "数据源 ID"
// @Success 200 {object} types.ApiResponse{data=[]string}
// @Router /api/w8t/prometheus/metrics [get]
func (c *prometheusProxyController) GetMetricNames(ctx *gin.Context) {
	var req types.PrometheusProxyMetricNamesRequest

	// 绑定查询参数
	if err := ctx.ShouldBindQuery(&req); err != nil {
		response.Fail(ctx, err.Error(), "failed")
		return
	}

	// 调用 Service 层
	metrics, err := services.PrometheusProxyService.GetMetricNames(&req)
	if err != nil {
		response.Fail(ctx, err.Error(), "failed")
		return
	}

	// 返回标准 Prometheus API 格式
	response.Success(ctx, map[string]interface{}{
		"status": "success",
		"data":   metrics,
	}, "success")
}

// GetSeries 获取时间序列元数据
// @Summary 获取时间序列元数据
// @Description PromQL 编辑器 - 获取匹配的时间序列标签组合
// @Tags PromQL 编辑器
// @Accept json
// @Produce json
// @Param request body types.PrometheusProxySeriesRequest true "请求参数"
// @Success 200 {object} types.ApiResponse{data=[]map[string]string}
// @Router /api/w8t/prometheus/series [post]
func (c *prometheusProxyController) GetSeries(ctx *gin.Context) {
	var req types.PrometheusProxySeriesRequest

	// 绑定 JSON 请求体
	if err := ctx.ShouldBindJSON(&req); err != nil {
		response.Fail(ctx, err.Error(), "failed")
		return
	}

	// 调用 Service 层
	series, err := services.PrometheusProxyService.GetSeries(&req)
	if err != nil {
		response.Fail(ctx, err.Error(), "failed")
		return
	}

	// 返回标准 Prometheus API 格式
	response.Success(ctx, map[string]interface{}{
		"status": "success",
		"data":   series,
	}, "success")
}
