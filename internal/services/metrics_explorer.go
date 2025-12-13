package services

import (
	"fmt"
	"strconv"
	"time"
	"alertHub/internal/ctx"
	"alertHub/internal/types"
	"alertHub/pkg/provider"
	"alertHub/pkg/tools"

	"github.com/zeromicro/go-zero/core/logc"
)

// metricsExplorerService 指标浏览器服务
type metricsExplorerService struct {
	ctx *ctx.Context
}

// InterMetricsExplorerService 指标浏览器服务接口
type InterMetricsExplorerService interface {
	// GetMetricsWithPagination 分页获取指标列表
	GetMetricsWithPagination(req *types.MetricsExplorerMetricsRequest) (*types.MetricsExplorerMetricsResponse, error)

	// GetMetricsCategories 获取指标分类统计
	GetMetricsCategories(req *types.MetricsExplorerCategoriesRequest) ([]types.MetricsCategory, error)

	// QueryRangeEnhanced 增强的查询范围接口
	QueryRangeEnhanced(req *types.MetricsExplorerQueryRangeRequest) (*types.MetricsExplorerQueryRangeResponse, error)
}

// newInterMetricsExplorerService 创建指标浏览器服务实例
func newInterMetricsExplorerService(ctx *ctx.Context) InterMetricsExplorerService {
	return &metricsExplorerService{
		ctx: ctx,
	}
}

// GetMetricsWithPagination 分页获取指标列表
func (s *metricsExplorerService) GetMetricsWithPagination(req *types.MetricsExplorerMetricsRequest) (*types.MetricsExplorerMetricsResponse, error) {
	client, err := s.getPrometheusClient(req.DatasourceID)
	if err != nil {
		return nil, err
	}

	allMetrics, err := client.GetMetricNames()
	if err != nil {
		logc.Errorf(s.ctx.Ctx, "获取指标名称失败: datasourceId=%s, err=%v", req.DatasourceID, err)
		return nil, fmt.Errorf("获取指标名称失败: %w", err)
	}

	// 复用工具层的过滤和分页功能
	filteredMetrics := tools.FilterMetricsByKeyword(allMetrics, req.Search)
	pageMetrics, total := tools.PaginateSlice(filteredMetrics, req.Page, req.Size)

	page := req.Page
	if page <= 0 {
		page = 1
	}
	size := req.Size
	if size <= 0 {
		size = 20
	}

	return &types.MetricsExplorerMetricsResponse{
		Metrics: pageMetrics,
		Total:   total,
		Page:    page,
		Size:    size,
	}, nil
}

// GetMetricsCategories 获取指标分类统计
func (s *metricsExplorerService) GetMetricsCategories(req *types.MetricsExplorerCategoriesRequest) ([]types.MetricsCategory, error) {
	client, err := s.getPrometheusClient(req.DatasourceID)
	if err != nil {
		return nil, err
	}

	allMetrics, err := client.GetMetricNames()
	if err != nil {
		return nil, fmt.Errorf("获取指标名称失败: %w", err)
	}

	// 复用工具层的分类功能
	toolCategories := tools.CategorizeMetrics(allMetrics)

	categories := make([]types.MetricsCategory, len(toolCategories))
	for i, toolCat := range toolCategories {
		categories[i] = types.MetricsCategory{
			Category: toolCat.Name,
			Count:    toolCat.Count,
			Metrics:  toolCat.Sample,
		}
	}

	return categories, nil
}

// QueryRangeEnhanced 增强的查询范围接口
func (s *metricsExplorerService) QueryRangeEnhanced(req *types.MetricsExplorerQueryRangeRequest) (*types.MetricsExplorerQueryRangeResponse, error) {
	client, err := s.getPrometheusClient(req.DatasourceID)
	if err != nil {
		return nil, err
	}

	// 使用优化后的查询参数计算功能
	step, err := tools.OptimizeQueryParameters(req.Start, req.End, req.Step, 800)
	if err != nil {
		return nil, fmt.Errorf("计算查询步长失败: %w", err)
	}

	startTime := time.Unix(req.Start, 0)
	endTime := time.Unix(req.End, 0)

	// 预估数据点数量
	estimatedPoints := tools.EstimateDataPoints(req.Start, req.End, step)
	logc.Infof(s.ctx.Ctx, "预估数据点数: %d, step: %v", estimatedPoints, step)

	result, err := client.QueryRange(req.Query, startTime, endTime, step)
	if err != nil {
		return nil, fmt.Errorf("查询范围失败: %w", err)
	}

	// 应用下采样优化（如果数据点过多）
	optimizedResult := s.applyDownsampling(result, req.MaxPoints)

	return &types.MetricsExplorerQueryRangeResponse{
		Status: "success",
		Data:   optimizedResult,
		Metadata: &types.QueryMetadata{
			EstimatedPoints: estimatedPoints,
			ActualPoints:    s.countDataPoints(optimizedResult),
			Step:            step.String(),
			DownsamplingApplied: estimatedPoints > 800,
		},
	}, nil
}

// applyDownsampling 应用数据下采样优化
func (s *metricsExplorerService) applyDownsampling(data interface{}, maxPoints int) interface{} {
	if maxPoints <= 0 {
		maxPoints = 800
	}

	// 这里需要根据Prometheus返回的具体数据格式进行处理
	// 通常是 map[string]interface{} 格式，包含 resultType 和 result 字段
	dataMap, ok := data.(map[string]interface{})
	if !ok {
		return data
	}

	resultArray, ok := dataMap["result"].([]interface{})
	if !ok {
		return data
	}

	// 对每个时间序列应用下采样
	for i, series := range resultArray {
		seriesMap, ok := series.(map[string]interface{})
		if !ok {
			continue
		}

		values, ok := seriesMap["values"].([]interface{})
		if !ok {
			continue
		}

		// 转换为 DataPoint 结构
		dataPoints := make([]tools.DataPoint, len(values))
		for j, value := range values {
			valueArray, ok := value.([]interface{})
			if !ok || len(valueArray) < 2 {
				continue
			}
			
			timestamp, _ := valueArray[0].(float64)
			val, _ := valueArray[1].(string)
			
			if parsedVal, err := strconv.ParseFloat(val, 64); err == nil {
				dataPoints[j] = tools.DataPoint{
					Timestamp: int64(timestamp),
					Value:     parsedVal,
				}
			}
		}

		// 应用LTTB下采样
		if len(dataPoints) > maxPoints {
			downsampledPoints := tools.LTTBDownsample(dataPoints, maxPoints)
			
			// 转换回原格式
			newValues := make([]interface{}, len(downsampledPoints))
			for j, point := range downsampledPoints {
				newValues[j] = []interface{}{
					float64(point.Timestamp),
					fmt.Sprintf("%.6f", point.Value),
				}
			}
			
			seriesMap["values"] = newValues
			resultArray[i] = seriesMap
		}
	}

	dataMap["result"] = resultArray
	return dataMap
}

// countDataPoints 统计实际数据点数量
func (s *metricsExplorerService) countDataPoints(data interface{}) int {
	dataMap, ok := data.(map[string]interface{})
	if !ok {
		return 0
	}

	resultArray, ok := dataMap["result"].([]interface{})
	if !ok {
		return 0
	}

	totalPoints := 0
	for _, series := range resultArray {
		seriesMap, ok := series.(map[string]interface{})
		if !ok {
			continue
		}

		values, ok := seriesMap["values"].([]interface{})
		if ok {
			totalPoints += len(values)
		}
	}

	return totalPoints
}

// getPrometheusClient 获取Prometheus客户端
func (s *metricsExplorerService) getPrometheusClient(datasourceID string) (provider.PrometheusProvider, error) {
	pools := s.ctx.Redis.ProviderPools()
	cli, err := pools.GetClient(datasourceID)
	if err != nil {
		return provider.PrometheusProvider{}, fmt.Errorf("获取数据源客户端失败: %w", err)
	}
	if cli == nil {
		return provider.PrometheusProvider{}, fmt.Errorf("数据源不存在或未初始化: %s", datasourceID)
	}

	promClient, ok := cli.(provider.PrometheusProvider)
	if !ok {
		return provider.PrometheusProvider{}, fmt.Errorf("数据源类型不是 Prometheus: %s", datasourceID)
	}

	return promClient, nil
}