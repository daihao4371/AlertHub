package collector

import (
	"alertHub/internal/ctx"
	"alertHub/internal/models"
	"alertHub/internal/types"
	"alertHub/pkg/analysis/interfaces"
	"alertHub/pkg/analysis/metadata"
	"alertHub/pkg/analysis/query"
	"alertHub/pkg/analysis/relationship"
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/prometheus/client_golang/api"
	v1 "github.com/prometheus/client_golang/api/prometheus/v1"
	"github.com/prometheus/common/model"
	"github.com/zeromicro/go-zero/core/logc"
)

// UniversalDataCollector 完全通用的数据采集器 - 零硬编码设计
// 实现interfaces.UniversalDataCollector接口
type UniversalDataCollector struct {
	prometheusClient   api.Client
	prometheusAPI      v1.API
	metadataExtractor  *metadata.MetadataExtractor
	relationshipEngine *relationship.DynamicRelationshipEngine
	queryOptimizer     *query.QueryOptimizer
	parallelExecutor   *query.ParallelExecutor
	strategyEngine     interfaces.ConfigurableStrategyEngine
	config             *CollectorConfig
	mutex              sync.RWMutex
}

// 确保实现了接口
var _ interfaces.UniversalDataCollector = (*UniversalDataCollector)(nil)

// CollectorConfig 采集器配置
type CollectorConfig struct {
	PrometheusURL           string        `json:"prometheusURL"`
	QueryTimeout            time.Duration `json:"queryTimeout"`
	MaxConcurrentQueries    int           `json:"maxConcurrentQueries"`
	MaxRelatedMetrics       int           `json:"maxRelatedMetrics"`
	CacheTTL                time.Duration `json:"cacheTTL"`
	RetryAttempts           int           `json:"retryAttempts"`
	RetryDelay              time.Duration `json:"retryDelay"`
	EnableQueryOptimization bool          `json:"enableQueryOptimization"`
	EnableMetricDiscovery   bool          `json:"enableMetricDiscovery"`
	// 业务时间配置
	BusinessHoursStart int            `json:"businessHoursStart"` // 业务时间开始小时（0-23），默认9
	BusinessHoursEnd   int            `json:"businessHoursEnd"`   // 业务时间结束小时（0-23），默认18
	BusinessDays       []time.Weekday `json:"businessDays"`       // 业务日列表，默认周一到周五
	BusinessTimeZone   string         `json:"businessTimeZone"`   // 业务时区，默认UTC
}

// NewUniversalDataCollector 创建通用数据采集器
func NewUniversalDataCollector(config *CollectorConfig) (*UniversalDataCollector, error) {
	// 设置业务时间配置默认值（在验证之前设置）
	if config.BusinessHoursStart == 0 {
		config.BusinessHoursStart = 9
	}
	if config.BusinessHoursEnd == 0 {
		config.BusinessHoursEnd = 18
	}
	if len(config.BusinessDays) == 0 {
		config.BusinessDays = []time.Weekday{
			time.Monday,
			time.Tuesday,
			time.Wednesday,
			time.Thursday,
			time.Friday,
		}
	}
	if config.BusinessTimeZone == "" {
		config.BusinessTimeZone = "UTC"
	}

	// 验证配置
	if err := validateCollectorConfig(config); err != nil {
		return nil, fmt.Errorf("配置验证失败: %w", err)
	}

	// 创建Prometheus客户端
	client, err := api.NewClient(api.Config{
		Address: config.PrometheusURL,
	})
	if err != nil {
		return nil, fmt.Errorf("创建Prometheus客户端失败: %w", err)
	}

	prometheusAPI := v1.NewAPI(client)

	// 创建策略引擎
	strategyEngine, err := NewConfigurableStrategyEngine()
	if err != nil {
		return nil, fmt.Errorf("创建策略引擎失败: %w", err)
	}

	return &UniversalDataCollector{
		prometheusClient:   client,
		prometheusAPI:      prometheusAPI,
		metadataExtractor:  metadata.NewMetadataExtractor(),
		relationshipEngine: relationship.NewDynamicRelationshipEngine(),
		queryOptimizer:     query.NewQueryOptimizer(),
		parallelExecutor:   query.NewParallelExecutor(config.MaxConcurrentQueries),
		strategyEngine:     strategyEngine,
		config:             config,
	}, nil
}

// CollectContext 采集完整分析上下文 - 纯通用逻辑
func (udc *UniversalDataCollector) CollectContext(
	ctx *ctx.Context,
	request *types.RequestDataCollection,
) (*types.ResponseDataCollection, error) {
	startTime := time.Now()
	collectionID := fmt.Sprintf("collection_%d_%s", startTime.Unix(), request.AlertId)

	logc.Infof(ctx.Ctx, "[数据采集] 开始采集: collectionId=%s, alertId=%s, promQL=%s",
		collectionID, request.AlertId, request.PromQL)

	// 1. 提取指标元数据（通过PromQL解析，无硬编码）
	metadata, err := udc.metadataExtractor.ExtractFromPromQL(ctx, request.PromQL, request.Labels)
	if err != nil {
		return nil, fmt.Errorf("提取元数据失败: %w", err)
	}

	// 2. 动态发现相关指标（基于标签相似度和拓扑关系）
	var relatedMetrics []*relationship.RelatedMetricDescriptor
	if udc.config.EnableMetricDiscovery {
		relatedMetrics, err = udc.relationshipEngine.DiscoverRelated(ctx, metadata)
		if err != nil {
			logc.Infof(ctx.Ctx, "[数据采集] 相关指标发现失败: %v", err)
			// 不影响主流程，继续执行
		}
	}

	// 3. 构建最优查询计划（基于数据量和重要性）
	queryPlan, err := udc.queryOptimizer.BuildPlan(ctx, &query.PlanRequest{
		PrimaryMetric:   metadata,
		RelatedMetrics:  relatedMetrics,
		TimeRange:       request.TimeRange,
		MaxMetrics:      request.MaxRelatedMetrics,
		OptimizeQueries: udc.config.EnableQueryOptimization,
	})
	if err != nil {
		return nil, fmt.Errorf("构建查询计划失败: %w", err)
	}

	// 4. 并行执行查询（通用查询执行引擎）
	rawData, err := udc.executeQueries(ctx, queryPlan)
	if err != nil {
		return nil, fmt.Errorf("执行查询失败: %w", err)
	}

	// 5. 构建标准化上下文（纯数据结构，无业务逻辑）
	universalContext, err := udc.buildUniversalContext(ctx, request, metadata, rawData)
	if err != nil {
		return nil, fmt.Errorf("构建通用上下文失败: %w", err)
	}

	duration := time.Since(startTime)
	logc.Infof(ctx.Ctx, "[数据采集] 完成: collectionId=%s, 耗时=%v, 指标数=%d",
		collectionID, duration, len(rawData))

	return &types.ResponseDataCollection{
		CollectionId: collectionID,
		Context:      universalContext,
		ProcessedAt:  time.Now().Unix(),
		Duration:     duration.Milliseconds(),
		Status:       "success",
	}, nil
}

// executeQueries 并行执行查询计划
func (udc *UniversalDataCollector) executeQueries(
	ctx *ctx.Context,
	plan *query.QueryPlan,
) (map[string]*query.RawMetricData, error) {
	queryCtx, cancel := context.WithTimeout(ctx.Ctx, udc.config.QueryTimeout)
	defer cancel()

	// 构建查询任务
	tasks := make([]*query.QueryTask, 0, len(plan.Queries))
	for _, queryInfo := range plan.Queries {
		task := &query.QueryTask{
			ID:         queryInfo.ID,
			Query:      queryInfo.Query,
			TimeRange:  queryInfo.TimeRange,
			Priority:   queryInfo.Priority,
			RetryCount: udc.config.RetryAttempts,
		}
		tasks = append(tasks, task)
	}

	// 并行执行查询
	results, err := udc.parallelExecutor.ExecuteQueries(queryCtx, udc.prometheusAPI, tasks)
	if err != nil {
		return nil, fmt.Errorf("并行查询执行失败: %w", err)
	}

	// 转换结果格式
	rawData := make(map[string]*query.RawMetricData)
	for id, result := range results {
		if result.Error != nil {
			logc.Infof(ctx.Ctx, "[数据采集] 查询失败: id=%s, error=%v", id, result.Error)
			continue
		}

		// 转换QueryInfo
		var queryInfo *models.QueryInfo
		if result.QueryInfo != nil {
			queryInfo = &models.QueryInfo{
				OriginalQuery: result.QueryInfo.Query,
				ExecutedQuery: result.QueryInfo.Query,
				QueryTime:     time.Now().Unix(),
				Duration:      0,
				DataSource:    "Prometheus",
				CacheHit:      false,
				ResultSize:    0,
			}
		}

		// 转换数据
		var timeSeries []*models.DataPoint
		if result.Data != nil {
			if promData, ok := result.Data.(model.Value); ok {
				timeSeries = udc.convertPrometheusData(promData)
			}
		}

		rawData[id] = &query.RawMetricData{
			MetricName: result.MetricName,
			Labels:     result.Labels,
			TimeSeries: timeSeries,
			QueryInfo:  queryInfo,
		}
	}

	return rawData, nil
}

// convertPrometheusData 转换Prometheus数据为内部格式
func (udc *UniversalDataCollector) convertPrometheusData(data model.Value) []*models.DataPoint {
	var dataPoints []*models.DataPoint

	switch v := data.(type) {
	case model.Matrix:
		for _, sampleStream := range v {
			for _, sample := range sampleStream.Values {
				point := &models.DataPoint{
					Timestamp: int64(sample.Timestamp) / 1000, // 转换为秒
					Value:     float64(sample.Value),
					Labels:    convertModelLabels(sampleStream.Metric),
					Quality: &models.DataPointQuality{
						IsValid:    true,
						Confidence: 1.0,
						Source:     "prometheus",
					},
				}
				dataPoints = append(dataPoints, point)
			}
		}

	case model.Vector:
		for _, sample := range v {
			point := &models.DataPoint{
				Timestamp: int64(sample.Timestamp) / 1000,
				Value:     float64(sample.Value),
				Labels:    convertModelLabels(sample.Metric),
				Quality: &models.DataPointQuality{
					IsValid:    true,
					Confidence: 1.0,
					Source:     "prometheus",
				},
			}
			dataPoints = append(dataPoints, point)
		}

	case *model.Scalar:
		point := &models.DataPoint{
			Timestamp: int64(v.Timestamp) / 1000,
			Value:     float64(v.Value),
			Quality: &models.DataPointQuality{
				IsValid:    true,
				Confidence: 1.0,
				Source:     "prometheus",
			},
		}
		dataPoints = append(dataPoints, point)
	}

	return dataPoints
}

// buildUniversalContext 构建通用分析上下文
func (udc *UniversalDataCollector) buildUniversalContext(
	ctx *ctx.Context,
	request *types.RequestDataCollection,
	metadata *metadata.MetricMetadata,
	rawData map[string]*query.RawMetricData,
) (*models.UniversalAnalysisContext, error) {

	contextID := fmt.Sprintf("context_%s_%d", request.AlertId, time.Now().Unix())

	// 构建主要指标数据
	var primaryMetric *models.MetricDataSet
	if primaryData, exists := rawData["primary"]; exists {
		primaryMetric = &models.MetricDataSet{
			MetricName:  metadata.MetricName,
			MetricType:  metadata.MetricType,
			Unit:        metadata.Unit,
			Description: metadata.Description,
			TimeSeries:  primaryData.TimeSeries,
			DataQuality: udc.calculateDataQuality(primaryData.TimeSeries),
			QueryInfo:   primaryData.QueryInfo,
			Metadata:    metadata.ToMap(),
		}
	}

	// 构建相关指标数据
	relatedMetrics := make(map[string]*models.MetricDataSet)
	for id, data := range rawData {
		if id == "primary" {
			continue
		}

		relatedMetrics[id] = &models.MetricDataSet{
			MetricName:  data.MetricName,
			MetricType:  "gauge", // 默认类型
			TimeSeries:  data.TimeSeries,
			DataQuality: udc.calculateDataQuality(data.TimeSeries),
			QueryInfo:   data.QueryInfo,
		}
	}

	// 构建系统上下文
	systemContext := udc.buildSystemContext(ctx, request, metadata)

	// 构建时间上下文
	timeContext := udc.buildTimeContext(ctx, request)

	// 获取租户ID：优先从context中获取，其次从request的Labels中获取
	tenantId := udc.extractTenantId(ctx, request)

	return &models.UniversalAnalysisContext{
		ContextId:      contextID,
		TenantId:       tenantId,
		CreatedAt:      time.Now().Unix(),
		AlertInfo:      udc.buildAlertInfo(request),
		RuleInfo:       udc.buildRuleInfo(request),
		PrimaryMetric:  primaryMetric,
		RelatedMetrics: relatedMetrics,
		SystemContext:  systemContext,
		TimeContext:    timeContext,
		AnalysisConfig: udc.buildAnalysisConfig(request),
		Extensions:     make(map[string]interface{}),
	}, nil
}

// calculateDataQuality 计算数据质量
func (udc *UniversalDataCollector) calculateDataQuality(dataPoints []*models.DataPoint) *models.DataQualityInfo {
	if len(dataPoints) == 0 {
		return &models.DataQualityInfo{
			Completeness: 0.0,
			Accuracy:     0.0,
			TotalPoints:  0,
			QualityScore: 0.0,
		}
	}

	validPoints := 0
	anomalyPoints := 0

	for _, point := range dataPoints {
		if point.Quality != nil && point.Quality.IsValid {
			validPoints++
		}
		if point.Quality != nil && point.Quality.Anomaly {
			anomalyPoints++
		}
	}

	completeness := float64(validPoints) / float64(len(dataPoints))
	accuracy := float64(len(dataPoints)-anomalyPoints) / float64(len(dataPoints))

	return &models.DataQualityInfo{
		Completeness:  completeness,
		Accuracy:      accuracy,
		Timeliness:    1.0, // 实时数据，时效性满分
		TotalPoints:   len(dataPoints),
		ValidPoints:   validPoints,
		AnomalyPoints: anomalyPoints,
		QualityScore:  (completeness + accuracy + 1.0) / 3.0,
	}
}

// buildSystemContext 构建系统上下文
func (udc *UniversalDataCollector) buildSystemContext(
	ctx *ctx.Context,
	request *types.RequestDataCollection,
	metadata *metadata.MetricMetadata,
) *models.SystemContextInfo {

	// 从标签中提取系统信息
	environment := extractLabelValue(request.Labels, []string{"environment", "env"}, "unknown")
	region := extractLabelValue(request.Labels, []string{"region", "zone"}, "unknown")
	cluster := extractLabelValue(request.Labels, []string{"cluster", "cluster_name"}, "unknown")
	serviceName := extractLabelValue(request.Labels, []string{"service", "service_name", "job"}, "unknown")

	return &models.SystemContextInfo{
		Environment:    environment,
		Region:         region,
		Cluster:        cluster,
		ServiceName:    serviceName,
		ServiceType:    metadata.ServiceType,
		Labels:         request.Labels,
		EnrichedLabels: udc.enrichLabels(request.Labels, metadata),
	}
}

// buildTimeContext 构建时间上下文
func (udc *UniversalDataCollector) buildTimeContext(
	ctx *ctx.Context,
	request *types.RequestDataCollection,
) *models.TimeContextInfo {
	now := time.Now()

	return &models.TimeContextInfo{
		AnalysisTime:    now.Unix(),
		EventTime:       request.StartTime,
		TimeZone:        "UTC",
		IsBusinessHours: udc.isBusinessHours(now),
		TimeRanges: map[string]*models.TimeRange{
			"current": {
				Name:      "current",
				StartTime: request.TimeRange.StartTime,
				EndTime:   request.TimeRange.EndTime,
				Duration:  request.TimeRange.EndTime - request.TimeRange.StartTime,
				Purpose:   "primary analysis window",
			},
		},
	}
}

// buildAlertInfo 构建告警信息
func (udc *UniversalDataCollector) buildAlertInfo(request *types.RequestDataCollection) *models.AlertBasicInfo {
	return &models.AlertBasicInfo{
		RuleId:      request.RuleId,
		RuleName:    request.RuleName,
		Severity:    request.Severity,
		TriggerTime: request.StartTime,
		Duration:    time.Now().Unix() - request.StartTime,
		Labels:      convertStringMapToInterface(request.Labels),
	}
}

// buildRuleInfo 构建规则信息
func (udc *UniversalDataCollector) buildRuleInfo(request *types.RequestDataCollection) *models.RuleBasicInfo {
	return &models.RuleBasicInfo{
		RuleId:         request.RuleId,
		RuleName:       request.RuleName,
		DatasourceType: "Prometheus",
		PromQL:         request.PromQL,
		Labels:         request.Labels,
	}
}

// buildAnalysisConfig 构建分析配置
func (udc *UniversalDataCollector) buildAnalysisConfig(request *types.RequestDataCollection) *models.AnalysisConfiguration {
	return &models.AnalysisConfiguration{
		AnalysisType:  "auto",
		AnalysisMode:  "comprehensive",
		AnalysisScope: "current",
	}
}

// 辅助函数

func convertModelLabels(metric model.Metric) map[string]interface{} {
	labels := make(map[string]interface{})
	for key, value := range metric {
		labels[string(key)] = string(value)
	}
	return labels
}

func convertStringMapToInterface(strMap map[string]string) map[string]interface{} {
	result := make(map[string]interface{})
	for k, v := range strMap {
		result[k] = v
	}
	return result
}

func extractLabelValue(labels map[string]string, keys []string, defaultValue string) string {
	for _, key := range keys {
		if value, exists := labels[key]; exists && value != "" {
			return value
		}
	}
	return defaultValue
}

func (udc *UniversalDataCollector) enrichLabels(
	originalLabels map[string]string,
	metadata *metadata.MetricMetadata,
) map[string]interface{} {
	enriched := make(map[string]interface{})

	// 复制原始标签
	for k, v := range originalLabels {
		enriched[k] = v
	}

	// 添加元数据信息
	enriched["metric_type"] = metadata.MetricType
	enriched["metric_unit"] = metadata.Unit
	enriched["collection_time"] = time.Now().Unix()

	return enriched
}

// extractTenantId 从context或request中提取租户ID
// 优先级：1. context中的TenantID值 2. request.Labels中的tenantId/tenant_id 3. 空字符串
func (udc *UniversalDataCollector) extractTenantId(ctx *ctx.Context, request *types.RequestDataCollection) string {
	// 1. 尝试从标准库context中获取TenantID（如果中间件已设置）
	if ctx.Ctx != nil {
		if tenantId, ok := ctx.Ctx.Value("TenantID").(string); ok && tenantId != "" {
			return tenantId
		}
		// 尝试其他可能的key
		if tenantId, ok := ctx.Ctx.Value("TenantId").(string); ok && tenantId != "" {
			return tenantId
		}
	}

	// 2. 从request的Labels中获取（如果存在）
	if request.Labels != nil {
		// 尝试多种可能的key名称
		if tenantId, exists := request.Labels["tenantId"]; exists && tenantId != "" {
			return tenantId
		}
		if tenantId, exists := request.Labels["tenant_id"]; exists && tenantId != "" {
			return tenantId
		}
		if tenantId, exists := request.Labels["TenantID"]; exists && tenantId != "" {
			return tenantId
		}
		if tenantId, exists := request.Labels["TenantId"]; exists && tenantId != "" {
			return tenantId
		}
	}

	// 3. 如果都获取不到，返回空字符串
	return ""
}

// isBusinessHours 判断指定时间是否在业务时间内
// 支持可配置的业务时间范围、业务日和时区
// 参数:
//   - t: 要判断的时间（UTC时间）
//
// 返回:
//   - bool: true表示在业务时间内，false表示不在业务时间内
func (udc *UniversalDataCollector) isBusinessHours(t time.Time) bool {
	udc.mutex.RLock()
	defer udc.mutex.RUnlock()

	// 加载配置，使用默认值保护
	startHour := udc.config.BusinessHoursStart
	endHour := udc.config.BusinessHoursEnd
	businessDays := udc.config.BusinessDays
	timeZone := udc.config.BusinessTimeZone

	// 参数验证和边界检查
	if startHour < 0 || startHour > 23 {
		startHour = 9 // 使用默认值
	}
	if endHour < 0 || endHour > 23 {
		endHour = 18 // 使用默认值
	}
	if startHour >= endHour {
		// 无效的时间范围，默认返回false
		return false
	}
	if len(businessDays) == 0 {
		// 没有配置业务日，默认返回false
		return false
	}

	// 转换时区
	var localTime time.Time
	if timeZone != "" && timeZone != "UTC" {
		loc, err := time.LoadLocation(timeZone)
		if err != nil {
			// 时区加载失败，使用UTC
			localTime = t.UTC()
		} else {
			localTime = t.In(loc)
		}
	} else {
		localTime = t.UTC()
	}

	// 检查是否在业务日内
	weekday := localTime.Weekday()
	isBusinessDay := false
	for _, day := range businessDays {
		if weekday == day {
			isBusinessDay = true
			break
		}
	}

	if !isBusinessDay {
		return false
	}

	// 检查是否在业务时间范围内
	hour := localTime.Hour()
	return hour >= startHour && hour < endHour
}

// DiscoverRelatedMetrics 动态发现相关指标（实现接口方法）
func (udc *UniversalDataCollector) DiscoverRelatedMetrics(
	ctx *ctx.Context,
	primaryMetric *interfaces.MetricDescriptor,
	config *interfaces.DiscoveryConfig,
) ([]*interfaces.MetricDescriptor, error) {
	logc.Infof(ctx.Ctx, "[数据采集] 开始发现相关指标: metric=%s", primaryMetric.Name)

	// 转换为内部格式
	metadata := &metadata.MetricMetadata{
		MetricName: primaryMetric.Name,
		MetricType: primaryMetric.Type,
		Labels:     primaryMetric.Labels,
	}

	// 执行发现
	relatedMetrics, err := udc.relationshipEngine.DiscoverRelated(ctx, metadata)
	if err != nil {
		return nil, fmt.Errorf("发现相关指标失败: %w", err)
	}

	// 转换为接口格式
	result := make([]*interfaces.MetricDescriptor, 0, len(relatedMetrics))
	for _, metric := range relatedMetrics {
		descriptor := &interfaces.MetricDescriptor{
			Name:       metric.MetricName,
			Type:       "gauge", // 默认类型，因为RelatedMetricDescriptor没有Type字段
			Labels:     metric.Labels,
			Query:      "", // RelatedMetricDescriptor没有Query字段
			Importance: float64(metric.Priority), // 转换Priority为Importance
			Metadata:   make(map[string]interface{}),
		}
		result = append(result, descriptor)
	}

	return result, nil
}

// ValidateConfig 验证采集器配置（实现接口方法）
func (udc *UniversalDataCollector) ValidateConfig(config interface{}) error {
	collectorConfig, ok := config.(*CollectorConfig)
	if !ok {
		return fmt.Errorf("配置类型错误，期望 *CollectorConfig，实际 %T", config)
	}
	return validateCollectorConfig(collectorConfig)
}

// validateCollectorConfig 验证采集器配置
func validateCollectorConfig(config *CollectorConfig) error {
	if config == nil {
		return fmt.Errorf("配置不能为空")
	}

	if config.PrometheusURL == "" {
		return fmt.Errorf("PrometheusURL不能为空")
	}

	if config.QueryTimeout <= 0 {
		return fmt.Errorf("QueryTimeout必须大于0")
	}

	if config.MaxConcurrentQueries <= 0 {
		return fmt.Errorf("MaxConcurrentQueries必须大于0")
	}

	if config.MaxRelatedMetrics < 0 {
		return fmt.Errorf("MaxRelatedMetrics不能为负数")
	}

	// 验证业务时间配置
	if config.BusinessHoursStart < 0 || config.BusinessHoursStart > 23 {
		return fmt.Errorf("BusinessHoursStart必须在0-23范围内")
	}

	if config.BusinessHoursEnd < 0 || config.BusinessHoursEnd > 23 {
		return fmt.Errorf("BusinessHoursEnd必须在0-23范围内")
	}

	if config.BusinessHoursStart >= config.BusinessHoursEnd {
		return fmt.Errorf("BusinessHoursStart必须小于BusinessHoursEnd")
	}

	return nil
}

// ConfigurableStrategyEngine 可配置策略引擎实现
type ConfigurableStrategyEngine struct {
	strategies map[string]interfaces.StrategyBuilder
	mutex      sync.RWMutex
}

// NewConfigurableStrategyEngine 创建可配置策略引擎
func NewConfigurableStrategyEngine() (*ConfigurableStrategyEngine, error) {
	engine := &ConfigurableStrategyEngine{
		strategies: make(map[string]interfaces.StrategyBuilder),
	}

	// 注册内置策略
	engine.registerBuiltinStrategies()

	return engine, nil
}

// LoadStrategy 加载策略（实现接口方法）
func (cse *ConfigurableStrategyEngine) LoadStrategy(
	ctx *ctx.Context,
	strategyType string,
	config map[string]interface{},
) (interfaces.Strategy, error) {
	cse.mutex.RLock()
	builder, exists := cse.strategies[strategyType]
	cse.mutex.RUnlock()

	if !exists {
		return nil, fmt.Errorf("未知的策略类型: %s", strategyType)
	}

	return builder.Build(config)
}

// ExecuteStrategy 执行策略（实现接口方法）
func (cse *ConfigurableStrategyEngine) ExecuteStrategy(
	ctx *ctx.Context,
	strategy interfaces.Strategy,
	input interface{},
) (interface{}, error) {
	if err := strategy.Validate(input); err != nil {
		return nil, fmt.Errorf("策略输入验证失败: %w", err)
	}

	return strategy.Execute(ctx, input)
}

// RegisterStrategy 注册策略（实现接口方法）
func (cse *ConfigurableStrategyEngine) RegisterStrategy(
	strategyType string,
	builder interfaces.StrategyBuilder,
) error {
	if strategyType == "" {
		return fmt.Errorf("策略类型不能为空")
	}

	if builder == nil {
		return fmt.Errorf("策略构建器不能为空")
	}

	cse.mutex.Lock()
	defer cse.mutex.Unlock()

	cse.strategies[strategyType] = builder
	return nil
}

// ValidateStrategyConfig 验证策略配置（实现接口方法）
func (cse *ConfigurableStrategyEngine) ValidateStrategyConfig(
	strategyType string,
	config map[string]interface{},
) error {
	cse.mutex.RLock()
	builder, exists := cse.strategies[strategyType]
	cse.mutex.RUnlock()

	if !exists {
		return fmt.Errorf("未知的策略类型: %s", strategyType)
	}

	return builder.ValidateConfig(config)
}

// registerBuiltinStrategies 注册内置策略
func (cse *ConfigurableStrategyEngine) registerBuiltinStrategies() {
	// 注册标签相似度策略
	cse.strategies["label_similarity"] = &LabelSimilarityStrategyBuilder{}
	// 注册拓扑关系策略
	cse.strategies["topology_relation"] = &TopologyRelationStrategyBuilder{}
	// 注册历史相关策略
	cse.strategies["historical_correlation"] = &HistoricalCorrelationStrategyBuilder{}
}

// 内置策略实现示例

// LabelSimilarityStrategyBuilder 标签相似度策略构建器
type LabelSimilarityStrategyBuilder struct{}

func (builder *LabelSimilarityStrategyBuilder) Build(config map[string]interface{}) (interfaces.Strategy, error) {
	return &LabelSimilarityStrategy{config: config}, nil
}

func (builder *LabelSimilarityStrategyBuilder) ValidateConfig(config map[string]interface{}) error {
	// 验证配置参数
	return nil
}

func (builder *LabelSimilarityStrategyBuilder) GetStrategyType() string {
	return "label_similarity"
}

// LabelSimilarityStrategy 标签相似度策略
type LabelSimilarityStrategy struct {
	config map[string]interface{}
}

func (s *LabelSimilarityStrategy) Execute(ctx *ctx.Context, input interface{}) (interface{}, error) {
	// 实现标签相似度计算逻辑
	return nil, nil
}

func (s *LabelSimilarityStrategy) GetName() string {
	return "LabelSimilarityStrategy"
}

func (s *LabelSimilarityStrategy) GetType() string {
	return "label_similarity"
}

func (s *LabelSimilarityStrategy) Validate(input interface{}) error {
	return nil
}

// TopologyRelationStrategyBuilder 拓扑关系策略构建器
type TopologyRelationStrategyBuilder struct{}

func (builder *TopologyRelationStrategyBuilder) Build(config map[string]interface{}) (interfaces.Strategy, error) {
	return &TopologyRelationStrategy{config: config}, nil
}

func (builder *TopologyRelationStrategyBuilder) ValidateConfig(config map[string]interface{}) error {
	return nil
}

func (builder *TopologyRelationStrategyBuilder) GetStrategyType() string {
	return "topology_relation"
}

// TopologyRelationStrategy 拓扑关系策略
type TopologyRelationStrategy struct {
	config map[string]interface{}
}

func (s *TopologyRelationStrategy) Execute(ctx *ctx.Context, input interface{}) (interface{}, error) {
	return nil, nil
}

func (s *TopologyRelationStrategy) GetName() string {
	return "TopologyRelationStrategy"
}

func (s *TopologyRelationStrategy) GetType() string {
	return "topology_relation"
}

func (s *TopologyRelationStrategy) Validate(input interface{}) error {
	return nil
}

// HistoricalCorrelationStrategyBuilder 历史相关策略构建器
type HistoricalCorrelationStrategyBuilder struct{}

func (builder *HistoricalCorrelationStrategyBuilder) Build(config map[string]interface{}) (interfaces.Strategy, error) {
	return &HistoricalCorrelationStrategy{config: config}, nil
}

func (builder *HistoricalCorrelationStrategyBuilder) ValidateConfig(config map[string]interface{}) error {
	return nil
}

func (builder *HistoricalCorrelationStrategyBuilder) GetStrategyType() string {
	return "historical_correlation"
}

// HistoricalCorrelationStrategy 历史相关策略
type HistoricalCorrelationStrategy struct {
	config map[string]interface{}
}

func (s *HistoricalCorrelationStrategy) Execute(ctx *ctx.Context, input interface{}) (interface{}, error) {
	return nil, nil
}

func (s *HistoricalCorrelationStrategy) GetName() string {
	return "HistoricalCorrelationStrategy"
}

func (s *HistoricalCorrelationStrategy) GetType() string {
	return "historical_correlation"
}

func (s *HistoricalCorrelationStrategy) Validate(input interface{}) error {
	return nil
}
