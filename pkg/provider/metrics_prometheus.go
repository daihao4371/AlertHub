package provider

import (
	"alertHub/internal/models"
	"context"
	"fmt"
	"math"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/prometheus/client_golang/api"
	v1 "github.com/prometheus/client_golang/api/prometheus/v1"
	"github.com/prometheus/common/model"
)

type PrometheusProvider struct {
	ExternalLabels map[string]interface{}
	apiV1          v1.API
}

// BasicAuthTransport 实现带认证的HTTP传输层
type BasicAuthTransport struct {
	Username string
	Password string
	Base     http.RoundTripper
}

func (t *BasicAuthTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if t.Username != "" || t.Password != "" {
		req.SetBasicAuth(t.Username, t.Password)
	}
	return t.Base.RoundTrip(req)
}

func NewPrometheusClient(source models.AlertDataSource) (MetricsFactoryProvider, error) {
	// 创建基础传输层
	baseTransport := http.DefaultTransport

	// 配置认证传输层
	authTransport := &BasicAuthTransport{
		Username: source.Auth.User,
		Password: source.Auth.Pass,
		Base:     baseTransport,
	}

	// 创建客户端配置
	clientConfig := api.Config{
		Address:      source.HTTP.URL,
		RoundTripper: authTransport,
	}

	// 创建带认证的客户端
	client, err := api.NewClient(clientConfig)
	if err != nil {
		return nil, err
	}

	return PrometheusProvider{
		apiV1:          v1.NewAPI(client),
		ExternalLabels: source.Labels,
	}, nil
}

func (p PrometheusProvider) Query(promQL string) ([]Metrics, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	result, _, err := p.apiV1.Query(ctx, promQL, time.Now(), v1.WithTimeout(5*time.Second))
	if err != nil {
		return nil, err
	}

	return ConvertVectors(result), nil
}

func (p PrometheusProvider) QueryRange(promQL string, start, end time.Time, step time.Duration) ([]Metrics, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	r := v1.Range{
		Start: start,
		End:   end,
		Step:  step,
	}

	result, _, err := p.apiV1.QueryRange(ctx, promQL, r, v1.WithTimeout(20*time.Second))
	if err != nil {
		return nil, err
	}

	return ConvertMatrix(result), nil
}

func ConvertVectors(value model.Value) (lst []Metrics) {
	items, ok := value.(model.Vector)
	if !ok {
		return
	}

	for _, item := range items {
		if math.IsNaN(float64(item.Value)) {
			continue
		}

		var metric = make(map[string]interface{})
		for k, v := range item.Metric {
			metric[string(k)] = string(v)
		}

		lst = append(lst, Metrics{
			Timestamp: float64(item.Timestamp),
			Value:     float64(item.Value),
			Metric:    metric,
		})
	}
	return
}

// ConvertMatrix 将 Prometheus QueryRange 结果转换为 Metrics 列表
func ConvertMatrix(value model.Value) (lst []Metrics) {
	matrix, ok := value.(model.Matrix)
	if !ok {
		return
	}

	for _, stream := range matrix {
		var metric = make(map[string]interface{})
		for k, v := range stream.Metric {
			metric[string(k)] = string(v)
		}

		// 将每个时间点的数据转换为单独的 Metrics
		for _, value := range stream.Values {
			if math.IsNaN(float64(value.Value)) {
				continue
			}

			lst = append(lst, Metrics{
				Timestamp: float64(value.Timestamp),
				Value:     float64(value.Value),
				Metric:    metric,
			})
		}
	}
	return
}

func (p PrometheusProvider) Check() (bool, error) {
	_, err := p.apiV1.Config(context.Background())
	if err != nil {
		return false, err
	}

	return true, nil
}

func (p PrometheusProvider) GetExternalLabels() map[string]interface{} {
	return p.ExternalLabels
}

// TargetHealth Prometheus Target 健康状态
type TargetHealth struct {
	Instance   string            `json:"instance"`   // 实例地址 (如 192.168.1.100:9100)
	Job        string            `json:"job"`        // Job 名称
	Labels     map[string]string `json:"labels"`     // 标签
	ScrapeUrl  string            `json:"scrapeUrl"`  // 采集 URL
	Health     string            `json:"health"`     // up/down/unknown
	LastScrape string            `json:"lastScrape"` // 最后采集时间 (RFC3339 格式)
	LastError  string            `json:"lastError"`  // 错误信息
}

// extractPortFromScrapeURL 从 ScrapeURL 中提取端口信息
func extractPortFromScrapeURL(scrapeURL string) string {
	parsedURL, err := url.Parse(scrapeURL)
	if err != nil {
		return ""
	}

	// 如果 URL 有端口，返回端口号
	if parsedURL.Port() != "" {
		return parsedURL.Port()
	}

	// 根据协议返回默认端口
	switch parsedURL.Scheme {
	case "https":
		return "443"
	case "http":
		return "80"
	default:
		return ""
	}
}

// ensureInstanceWithPort 确保 instance 包含端口信息
func ensureInstanceWithPort(instance, scrapeURL string) string {
	// 如果 instance 已经包含端口（有冒号），直接返回
	if strings.Contains(instance, ":") {
		return instance
	}

	// 从 ScrapeURL 中提取端口
	port := extractPortFromScrapeURL(scrapeURL)
	if port != "" {
		return fmt.Sprintf("%s:%s", instance, port)
	}

	// 如果无法提取端口，返回原始 instance
	return instance
}

// GetTargets 获取 Prometheus 所有 Targets 的健康状态
// 直接调用 Prometheus Targets() API
func (p PrometheusProvider) GetTargets() ([]TargetHealth, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// 调用 Prometheus Targets API
	result, err := p.apiV1.Targets(ctx)
	if err != nil {
		return nil, err
	}

	// 转换为 TargetHealth 列表
	targets := make([]TargetHealth, 0, len(result.Active))
	for _, target := range result.Active {
		// 提取 instance 和 job (从 Labels 中)
		instance := string(target.Labels["instance"])
		job := string(target.Labels["job"])

		// 确保 instance 包含端口信息（如果缺少端口，从 ScrapeURL 中提取）
		instance = ensureInstanceWithPort(instance, target.ScrapeURL)

		// 转换 Labels 为 map[string]string
		labels := make(map[string]string)
		for k, v := range target.Labels {
			labels[string(k)] = string(v)
		}

		targets = append(targets, TargetHealth{
			Instance:   instance,
			Job:        job,
			Labels:     labels,
			ScrapeUrl:  target.ScrapeURL,
			Health:     string(target.Health),
			LastScrape: target.LastScrape.Format(time.RFC3339),
			LastError:  target.LastError,
		})
	}

	return targets, nil
}

// ========== PromQL 编辑器支持方法 ==========

// GetLabelNames 获取所有标签名称列表
// 用于 PromQL 编辑器的标签名称自动补全
// 参数:
//   - metricName: 可选,指定指标名称时只返回该指标的标签
//   - startTime: 可选,查询时间范围起点
//   - endTime: 可选,查询时间范围终点
func (p PrometheusProvider) GetLabelNames(metricName string, startTime, endTime time.Time) ([]string, error) {
	// 增加超时时间到 30 秒，避免因网络延迟或 Prometheus 响应慢导致超时
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// 构建查询选择器
	var matchers []string
	if metricName != "" {
		matchers = []string{fmt.Sprintf("{__name__=\"%s\"}", metricName)}
	}

	// 调用 Prometheus LabelNames API
	var labels []string
	var err error

	// 如果指定了时间范围,调用带时间参数的 API
	if !startTime.IsZero() && !endTime.IsZero() {
		labels, _, err = p.apiV1.LabelNames(ctx, matchers, startTime, endTime)
	} else {
		labels, _, err = p.apiV1.LabelNames(ctx, matchers, time.Time{}, time.Time{})
	}

	if err != nil {
		return nil, fmt.Errorf("prometheus LabelNames API 调用失败: %w", err)
	}

	return labels, nil
}

// GetLabelValues 获取指定标签的所有值列表
// 用于 PromQL 编辑器的标签值自动补全
// 参数:
//   - labelName: 标签名称 (如 "job", "instance", "env")
//   - metricName: 可选,限定指标范围
//   - startTime: 可选,查询时间范围起点
//   - endTime: 可选,查询时间范围终点
func (p PrometheusProvider) GetLabelValues(labelName, metricName string, startTime, endTime time.Time) ([]string, error) {
	// 增加超时时间到 30 秒，避免因网络延迟或 Prometheus 响应慢导致超时
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// 构建查询选择器
	var matchers []string
	if metricName != "" {
		// 重要: 使用 __name__ 标签来限制指标范围
		// 格式: {__name__="metricName"}
		matcher := fmt.Sprintf("{__name__=\"%s\"}", metricName)
		matchers = []string{matcher}
		// 调试日志: 记录构建的查询选择器
		// 注意: 在生产环境中应该使用日志级别控制
		// logc.Infof(ctx, "[PromQL 补全] Provider 层构建查询选择器: labelName=%s, matcher=%s", labelName, matcher)
	}

	// 调用 Prometheus LabelValues API
	var labelValues model.LabelValues
	var err error

	// 如果指定了时间范围,调用带时间参数的 API
	if !startTime.IsZero() && !endTime.IsZero() {
		labelValues, _, err = p.apiV1.LabelValues(ctx, labelName, matchers, startTime, endTime)
	} else {
		// 如果没有指定时间范围,使用零值表示不限制时间范围
		labelValues, _, err = p.apiV1.LabelValues(ctx, labelName, matchers, time.Time{}, time.Time{})
	}

	if err != nil {
		// 记录错误详情,包括查询参数
		// logc.Errorf(ctx, "[PromQL 补全] Prometheus API 调用失败: labelName=%s, metricName=%s, matchers=%v, err=%v",
		// 	labelName, metricName, matchers, err)
		return nil, fmt.Errorf("prometheus LabelValues API 调用失败: labelName=%s, metricName=%s, err=%w",
			labelName, metricName, err)
	}

	// 将 model.LabelValues 转换为 []string
	values := make([]string, len(labelValues))
	for i, lv := range labelValues {
		values[i] = string(lv)
	}

	// 调试日志: 记录返回结果
	// logc.Infof(ctx, "[PromQL 补全] Provider 层返回结果: labelName=%s, metricName=%s, count=%d",
	// 	labelName, metricName, len(values))

	return values, nil
}

// GetMetricNames 获取所有指标名称列表
// 用于 PromQL 编辑器的指标名称自动补全
// 本质上是获取 __name__ 标签的所有值
func (p PrometheusProvider) GetMetricNames() ([]string, error) {
	return p.GetLabelValues("__name__", "", time.Time{}, time.Time{})
}

// GetSeries 获取时间序列元数据
// 用于 PromQL 编辑器的序列信息查询
// 参数:
//   - matchers: 标签选择器数组 (如 []string{"up", "node_cpu_seconds_total{job=\"node\"}"})
//   - startTime: 查询时间范围起点
//   - endTime: 查询时间范围终点
//
// 返回:
//   - 匹配的时间序列标签组合列表
func (p PrometheusProvider) GetSeries(matchers []string, startTime, endTime time.Time) ([]map[string]string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	// 调用 Prometheus Series API
	var result []model.LabelSet
	var err error

	// 如果指定了时间范围,调用带时间参数的 API
	if !startTime.IsZero() && !endTime.IsZero() {
		result, _, err = p.apiV1.Series(ctx, matchers, startTime, endTime)
	} else {
		result, _, err = p.apiV1.Series(ctx, matchers, time.Time{}, time.Time{})
	}

	if err != nil {
		return nil, err
	}

	// 转换 model.LabelSet 为 map[string]string
	series := make([]map[string]string, 0, len(result))
	for _, labelSet := range result {
		labels := make(map[string]string)
		for k, v := range labelSet {
			labels[string(k)] = string(v)
		}
		series = append(series, labels)
	}

	return series, nil
}
