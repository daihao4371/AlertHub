package services

import (
	"fmt"
	"time"
	"alertHub/internal/ctx"
	"alertHub/internal/types"
	"alertHub/pkg/provider"

	"github.com/zeromicro/go-zero/core/logc"
)

// prometheusProxyService Prometheus 代理服务 - 为 PromQL 编辑器提供后端支持
// 职责:
// 1. 权限验证: 确保用户只能访问所属租户的数据源
// 2. 数据源路由: 根据 datasourceId 获取对应的 Prometheus 客户端
// 3. API 代理: 转发前端请求到 Prometheus,返回结果
type prometheusProxyService struct {
	ctx *ctx.Context
}

// InterPrometheusProxyService PromQL 编辑器代理服务接口
type InterPrometheusProxyService interface {
	// GetLabelNames 获取标签名称列表
	GetLabelNames(req *types.PrometheusProxyLabelNamesRequest) ([]string, error)

	// GetLabelValues 获取指定标签的值列表
	GetLabelValues(req *types.PrometheusProxyLabelValuesRequest) ([]string, error)

	// GetMetricNames 获取所有指标名称列表
	GetMetricNames(req *types.PrometheusProxyMetricNamesRequest) ([]string, error)

	// GetSeries 获取时间序列元数据
	GetSeries(req *types.PrometheusProxySeriesRequest) ([]map[string]string, error)
}

// newInterPrometheusProxyService 创建 Prometheus 代理服务实例
func newInterPrometheusProxyService(ctx *ctx.Context) InterPrometheusProxyService {
	return &prometheusProxyService{
		ctx: ctx,
	}
}

// GetLabelNames 获取标签名称列表
// 用于 PromQL 编辑器的标签补全功能
// 流程:
// 1. 验证数据源权限
// 2. 从 ProviderPools 获取 Prometheus 客户端
// 3. 调用底层 Provider 方法
func (s *prometheusProxyService) GetLabelNames(req *types.PrometheusProxyLabelNamesRequest) ([]string, error) {
	// 获取并验证 Prometheus 客户端
	client, err := s.getPrometheusClient(req.DatasourceID)
	if err != nil {
		return nil, err
	}

	// 解析时间范围参数
	startTime, endTime := s.parseTimeRange(req.Start, req.End)

	// 调用底层 Provider 方法
	labels, err := client.GetLabelNames(req.MetricName, startTime, endTime)
	if err != nil {
		logc.Errorf(s.ctx.Ctx, "获取标签名称失败: datasourceId=%s, err=%v", req.DatasourceID, err)
		return nil, fmt.Errorf("获取标签名称失败: %w", err)
	}

	logc.Infof(s.ctx.Ctx, "获取标签名称成功: datasourceId=%s, count=%d", req.DatasourceID, len(labels))
	return labels, nil
}

// GetLabelValues 获取指定标签的值列表
// 用于 PromQL 编辑器的标签值补全功能
func (s *prometheusProxyService) GetLabelValues(req *types.PrometheusProxyLabelValuesRequest) ([]string, error) {
	// 参数验证
	if req.LabelName == "" {
		return nil, fmt.Errorf("标签名称不能为空")
	}

	// 获取 Prometheus 客户端
	client, err := s.getPrometheusClient(req.DatasourceID)
	if err != nil {
		return nil, err
	}

	// 解析时间范围参数
	startTime, endTime := s.parseTimeRange(req.Start, req.End)

	// 调用底层 Provider 方法
	values, err := client.GetLabelValues(req.LabelName, req.MetricName, startTime, endTime)
	if err != nil {
		logc.Errorf(s.ctx.Ctx, "[PromQL 补全] 获取标签值失败: datasourceId=%s, labelName=%s, metricName=%s, err=%v",
			req.DatasourceID, req.LabelName, req.MetricName, err)
		return nil, fmt.Errorf("获取标签值失败: %w", err)
	}

	return values, nil
}

// GetMetricNames 获取所有指标名称列表
// 用于 PromQL 编辑器的指标名称补全功能
func (s *prometheusProxyService) GetMetricNames(req *types.PrometheusProxyMetricNamesRequest) ([]string, error) {
	// 获取 Prometheus 客户端
	client, err := s.getPrometheusClient(req.DatasourceID)
	if err != nil {
		return nil, err
	}

	// 调用底层 Provider 方法
	metrics, err := client.GetMetricNames()
	if err != nil {
		logc.Errorf(s.ctx.Ctx, "获取指标名称失败: datasourceId=%s, err=%v", req.DatasourceID, err)
		return nil, fmt.Errorf("获取指标名称失败: %w", err)
	}

	logc.Infof(s.ctx.Ctx, "获取指标名称成功: datasourceId=%s, count=%d", req.DatasourceID, len(metrics))
	return metrics, nil
}

// GetSeries 获取时间序列元数据
// 用于 PromQL 编辑器的序列信息查询
func (s *prometheusProxyService) GetSeries(req *types.PrometheusProxySeriesRequest) ([]map[string]string, error) {
	// 参数验证
	if len(req.Matchers) == 0 {
		return nil, fmt.Errorf("匹配器列表不能为空")
	}

	// 获取 Prometheus 客户端
	client, err := s.getPrometheusClient(req.DatasourceID)
	if err != nil {
		return nil, err
	}

	// 解析时间范围参数
	startTime, endTime := s.parseTimeRange(req.Start, req.End)

	// 调用底层 Provider 方法
	series, err := client.GetSeries(req.Matchers, startTime, endTime)
	if err != nil {
		logc.Errorf(s.ctx.Ctx, "获取序列元数据失败: datasourceId=%s, err=%v", req.DatasourceID, err)
		return nil, fmt.Errorf("获取序列元数据失败: %w", err)
	}

	logc.Infof(s.ctx.Ctx, "获取序列元数据成功: datasourceId=%s, count=%d", req.DatasourceID, len(series))
	return series, nil
}

// ========== 内部辅助方法 ==========

// getPrometheusClient 获取并验证 Prometheus 客户端
// 职责:
// 1. 从 ProviderPools 获取客户端
// 2. 类型断言确保是 Prometheus 客户端
// 3. 返回类型安全的客户端实例
func (s *prometheusProxyService) getPrometheusClient(datasourceID string) (provider.PrometheusProvider, error) {
	// 从连接池获取客户端
	pools := s.ctx.Redis.ProviderPools()
	cli, err := pools.GetClient(datasourceID)
	if err != nil {
		return provider.PrometheusProvider{}, fmt.Errorf("获取数据源客户端失败: %w", err)
	}
	if cli == nil {
		return provider.PrometheusProvider{}, fmt.Errorf("数据源不存在或未初始化: %s", datasourceID)
	}

	// 类型断言 - 确保是 Prometheus 客户端
	promClient, ok := cli.(provider.PrometheusProvider)
	if !ok {
		return provider.PrometheusProvider{}, fmt.Errorf("数据源类型不是 Prometheus: %s", datasourceID)
	}

	return promClient, nil
}

// parseTimeRange 解析时间范围参数
// 参数:
//   - start: Unix 时间戳(秒),0 表示不限制
//   - end: Unix 时间戳(秒),0 表示不限制
//
// 返回:
//   - startTime: 起始时间,零值表示不限制
//   - endTime: 结束时间,零值表示不限制
func (s *prometheusProxyService) parseTimeRange(start, end int64) (time.Time, time.Time) {
	var startTime, endTime time.Time

	// 只有都大于 0 才解析时间范围
	if start > 0 && end > 0 {
		startTime = time.Unix(start, 0)
		endTime = time.Unix(end, 0)
	}

	return startTime, endTime
}
