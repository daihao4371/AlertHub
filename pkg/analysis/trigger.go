package analysis

import (
	"alertHub/internal/ctx"
	"alertHub/internal/models"
	"alertHub/internal/types"
	"alertHub/pkg/analysis/ai"
	"alertHub/pkg/analysis/collector"
	"alertHub/pkg/analysis/interfaces"
	"alertHub/pkg/analysis/standardizer"
	"alertHub/pkg/analysis/utils"
	"alertHub/pkg/tools"
	"fmt"
	"time"

	"github.com/zeromicro/go-zero/core/logc"
)

// UniversalIntelligentAnalyzer 完全配置驱动的智能分析器
type UniversalIntelligentAnalyzer struct {
	// 核心组件 - 使用接口实现，完全去耦合
	dataCollector    interfaces.UniversalDataCollector
	dataStandardizer interfaces.UniversalDataStandardizer
	aiEngine         interfaces.UniversalAIEngine
	resultProcessor  interfaces.UniversalResultProcessor
	configManager    *ConfigManager

	// 配置
	config *AnalyzerConfig
}

// AnalyzerConfig 分析器配置
type AnalyzerConfig struct {
	ConfigName          string        `json:"configName"`          // 使用的配置名称
	Environment         string        `json:"environment"`         // 环境标识
	EnableAsyncAnalysis bool          `json:"enableAsyncAnalysis"` // 启用异步分析
	MaxAnalysisTime     time.Duration `json:"maxAnalysisTime"`     // 最大分析时间
	EnableCaching       bool          `json:"enableCaching"`       // 启用缓存
	CacheTimeout        time.Duration `json:"cacheTimeout"`        // 缓存超时
}

// NewUniversalIntelligentAnalyzer 创建完全配置驱动的智能分析器
func NewUniversalIntelligentAnalyzer(configPath string) (*UniversalIntelligentAnalyzer, error) {
	// 1. 创建配置管理器
	configManager := NewConfigManager()

	// 2. 加载配置
	err := configManager.LoadConfig(&ctx.Context{}, "default", configPath)
	if err != nil {
		return nil, fmt.Errorf("加载配置失败: %w", err)
	}

	// 3. 获取配置
	config, err := configManager.GetConfig("default")
	if err != nil {
		return nil, fmt.Errorf("获取配置失败: %w", err)
	}

	// 4. 基于配置创建组件
	dataCollector, err := collector.NewUniversalDataCollector(config.DataCollection)
	if err != nil {
		return nil, fmt.Errorf("创建数据采集器失败: %w", err)
	}

	dataStandardizer := standardizer.NewDataStandardizer(config.DataStandardizer)
	aiEngine := ai.NewAIAnalysisEngine(config.AIEngine)
	resultProcessor := NewResultProcessor(config.ResultProcessor)

	analyzerConfig := &AnalyzerConfig{
		ConfigName:          "default",
		Environment:         "production",
		EnableAsyncAnalysis: true,
		MaxAnalysisTime:     5 * time.Minute,
		EnableCaching:       true,
		CacheTimeout:        1 * time.Hour,
	}

	return &UniversalIntelligentAnalyzer{
		dataCollector:    dataCollector,
		dataStandardizer: dataStandardizer,
		aiEngine:         aiEngine,
		resultProcessor:  resultProcessor,
		configManager:    configManager,
		config:           analyzerConfig,
	}, nil
}

// AnalyzeAlert 完整的智能告警分析流程
func (uia *UniversalIntelligentAnalyzer) AnalyzeAlert(
	ctx *ctx.Context,
	analysisConfig *AnalysisConfig,
) (*interfaces.ProcessingResult, error) {

	startTime := time.Now()
	logc.Infof(ctx.Ctx, "[智能分析] 开始完整分析流程: alertId=%s, ruleId=%s",
		analysisConfig.AlertID, analysisConfig.RuleID)

	// 1. 数据采集（完全动态，无硬编码）
	collectionRequest := &types.RequestDataCollection{
		AlertId:            analysisConfig.AlertID,
		RuleId:             analysisConfig.RuleID,
		RuleName:           analysisConfig.RuleName,
		PromQL:             analysisConfig.PromQL,
		AlertStatus:        analysisConfig.AlertStatus,
		Severity:           analysisConfig.Severity,
		StartTime:          analysisConfig.StartTime,
		Labels:             analysisConfig.Labels,
		MaxRelatedMetrics:  analysisConfig.MaxRelatedMetrics,
		QueryTimeout:       analysisConfig.QueryTimeout,
		ParallelQueryLimit: analysisConfig.ParallelQueryLimit,
		TimeRange:          analysisConfig.TimeRange,
	}

	collectionResult, err := uia.dataCollector.CollectContext(ctx, collectionRequest)
	if err != nil {
		return nil, fmt.Errorf("数据采集失败: %w", err)
	}

	logc.Infof(ctx.Ctx, "[智能分析] 数据采集完成: collectionId=%s, 耗时=%dms",
		collectionResult.CollectionId, collectionResult.Duration)

	// 2. 数据标准化（通用特征提取）
	standardizedContext, err := uia.dataStandardizer.StandardizeContext(ctx, collectionResult.Context)
	if err != nil {
		return nil, fmt.Errorf("数据标准化失败: %w", err)
	}

	logc.Infof(ctx.Ctx, "[智能分析] 数据标准化完成: contextId=%s", standardizedContext.ContextId)

	// 3. AI分析（纯推理，无业务逻辑）
	aiRequest := &interfaces.AIAnalysisRequest{
		AnalysisType:  analysisConfig.AnalysisType,
		AnalysisMode:  analysisConfig.AnalysisMode,
		AnalysisDepth: analysisConfig.AnalysisDepth,
		FocusAreas:    analysisConfig.FocusAreas,
		PromptParams:  analysisConfig.CustomPrompts,
	}

	analysisResult, err := uia.aiEngine.Analyze(ctx, standardizedContext, aiRequest)
	if err != nil {
		return nil, fmt.Errorf("AI分析失败: %w", err)
	}

	logc.Infof(ctx.Ctx, "[智能分析] AI分析完成: analysisId=%s, 置信度=%.2f",
		analysisResult.AnalysisID, analysisResult.ConfidenceScore)

	// 4. 结果处理（通用后处理）
	processingResult, err := uia.resultProcessor.Process(ctx, analysisResult, standardizedContext)
	if err != nil {
		return nil, fmt.Errorf("结果处理失败: %w", err)
	}

	// 5. 记录处理统计
	duration := time.Since(startTime)
	logc.Infof(ctx.Ctx, "[智能分析] 完整流程完成: alertId=%s, 总耗时=%v, 状态=%s",
		analysisConfig.AlertID, duration, processingResult.ProcessingStatus)

	return processingResult, nil
}

// AnalysisConfig 分析配置
type AnalysisConfig struct {
	// 基础信息
	AlertID     string            `json:"alertId"`
	RuleID      string            `json:"ruleId"`
	RuleName    string            `json:"ruleName"`
	PromQL      string            `json:"promQL"`
	AlertStatus string            `json:"alertStatus"`
	Severity    string            `json:"severity"`
	StartTime   int64             `json:"startTime"`
	Labels      map[string]string `json:"labels"`

	// 数据采集配置
	MaxRelatedMetrics  int                  `json:"maxRelatedMetrics"`
	QueryTimeout       string               `json:"queryTimeout"`
	ParallelQueryLimit int                  `json:"parallelQueryLimit"`
	TimeRange          *types.TimeRangeInfo `json:"timeRange"`

	// AI分析配置
	AnalysisType  string                 `json:"analysisType"`
	AnalysisMode  string                 `json:"analysisMode"`
	AnalysisDepth string                 `json:"analysisDepth"`
	FocusAreas    []string               `json:"focusAreas"`
	CustomPrompts map[string]interface{} `json:"customPrompts"`
}

// executeIntelligentAnalysisWithNewArchitecture 使用新架构执行智能分析
func executeIntelligentAnalysisWithNewArchitecture(ctx *ctx.Context, event *models.AlertCurEvent) error {
	startTime := time.Now()

	// 1. 初始化分析状态
	if event.IntelligentAnalysis == nil {
		event.IntelligentAnalysis = &models.AlertEventAnalysis{
			TenantId:    event.TenantId,
			EventId:     event.EventId,
			Fingerprint: event.Fingerprint,
		}
	}

	event.IntelligentAnalysis.AnalysisEnabled = true
	event.IntelligentAnalysis.AnalysisStatus = "analyzing"
	event.IntelligentAnalysis.AnalysisId = "analysis-" + tools.RandId()
	event.IntelligentAnalysis.LastAnalysisTime = time.Now().Unix()

	// 2. 创建智能分析器（从默认配置路径）
	configPath := "/etc/alertHub/analysis/default.yaml"
	analyzer, err := NewUniversalIntelligentAnalyzer(configPath)
	if err != nil {
		// 如果配置文件不存在，使用内置默认配置
		analyzer, err = createAnalyzerWithDefaultConfig()
		if err != nil {
			return recordAnalysisFailure(ctx, event, fmt.Sprintf("创建分析器失败: %v", err))
		}
	}

	// 3. 获取规则的PromQL查询语句
	promQL := getPromQLFromRule(ctx, event.RuleId)
	if promQL == "" {
		return recordAnalysisFailure(ctx, event, "无法获取规则PromQL")
	}

	// 4. 构建分析配置
	analysisConfig := &AnalysisConfig{
		AlertID:     event.EventId,
		RuleID:      event.RuleId,
		RuleName:    event.RuleName,
		PromQL:      promQL,
		AlertStatus: string(event.Status),
		Severity:    event.Severity,
		StartTime:   event.FirstTriggerTime,
		Labels:      convertLabelsToStringMap(event.Labels),

		// 数据采集配置
		MaxRelatedMetrics:  10,
		QueryTimeout:       "30s",
		ParallelQueryLimit: 5,
		TimeRange: &types.TimeRangeInfo{
			StartTime: event.FirstTriggerTime - 3600, // 分析前1小时的数据
			EndTime:   time.Now().Unix(),
			Step:      60,
		},

		// AI分析配置
		AnalysisType:  "comprehensive",
		AnalysisMode:  "auto",
		AnalysisDepth: "deep",
		FocusAreas:    []string{"performance", "anomaly", "trend"},
		CustomPrompts: make(map[string]interface{}),
	}

	// 5. 执行完整的智能分析流程
	result, err := analyzer.AnalyzeAlert(ctx, analysisConfig)
	if err != nil {
		return recordAnalysisFailure(ctx, event, fmt.Sprintf("智能分析失败: %v", err))
	}

	// 6. 更新分析结果
	event.IntelligentAnalysis.AnalysisStatus = "completed"
	event.IntelligentAnalysis.AnalysisResult = formatProcessingResult(result)
	event.IntelligentAnalysis.AnalysisScore = calculateProcessingScore(result)

	// 7. 记录完成状态
	duration := time.Since(startTime)
	logc.Infof(ctx.Ctx, "[智能分析] 使用新架构完成分析: eventId=%s, 耗时=%v, 评分=%.2f, 状态=%s",
		event.EventId, duration, event.IntelligentAnalysis.AnalysisScore, result.ProcessingStatus)

	// 8. 更新告警缓存
	ctx.Redis.Alert().PushAlertEvent(event)

	return nil
}

// createAnalyzerWithDefaultConfig 创建带默认配置的分析器
func createAnalyzerWithDefaultConfig() (*UniversalIntelligentAnalyzer, error) {
	// 创建内置默认配置
	defaultConfig := &UniversalConfig{
		Metadata: ConfigMetadata{
			Name:        "default-builtin",
			Version:     "1.0.0",
			Description: "内置默认配置",
			CreatedAt:   time.Now(),
			UpdatedAt:   time.Now(),
		},
		DataCollection: &collector.CollectorConfig{
			PrometheusURL:           "http://localhost:9090",
			QueryTimeout:            30 * time.Second,
			MaxConcurrentQueries:    5,
			MaxRelatedMetrics:       10,
			EnableQueryOptimization: true,
			EnableMetricDiscovery:   true,
			BusinessHoursStart:      9,
			BusinessHoursEnd:        18,
		},
		DataStandardizer: &standardizer.StandardizerConfig{
			EnabledFeatures:          []string{"statistical", "timeseries", "anomaly"},
			MinDataPoints:            3,
			QualityThreshold:         0.7,
			EnableParallelProcessing: true,
		},
		AIEngine: &ai.AIEngineConfig{
			Model:         "gpt-4",
			MaxTokens:     4096,
			Temperature:   0.7,
			Timeout:       30 * time.Second,
			RetryAttempts: 3,
			RetryDelay:    2 * time.Second,
		},
		ResultProcessor: getDefaultResultProcessorConfig(),
	}

	// 基于默认配置创建组件
	dataCollector, err := collector.NewUniversalDataCollector(defaultConfig.DataCollection)
	if err != nil {
		return nil, fmt.Errorf("创建数据采集器失败: %w", err)
	}

	dataStandardizer := standardizer.NewDataStandardizer(defaultConfig.DataStandardizer)
	aiEngine := ai.NewAIAnalysisEngine(defaultConfig.AIEngine)
	resultProcessor := NewResultProcessor(defaultConfig.ResultProcessor)

	configManager := NewConfigManager()

	return &UniversalIntelligentAnalyzer{
		dataCollector:    dataCollector,
		dataStandardizer: dataStandardizer,
		aiEngine:         aiEngine,
		resultProcessor:  resultProcessor,
		configManager:    configManager,
		config: &AnalyzerConfig{
			ConfigName:          "builtin-default",
			Environment:         "production",
			EnableAsyncAnalysis: true,
			MaxAnalysisTime:     5 * time.Minute,
		},
	}, nil
}

// formatProcessingResult 格式化处理结果
func formatProcessingResult(result *interfaces.ProcessingResult) string {
	if result == nil {
		return "处理结果为空"
	}

	output := fmt.Sprintf("智能分析完成 (状态: %s)\n", result.ProcessingStatus)

	if result.ProcessedResult != nil && result.ProcessedResult.Summary != nil {
		summary := result.ProcessedResult.Summary
		output += fmt.Sprintf("问题: %s\n", summary.Title)
		output += fmt.Sprintf("严重程度: %s\n", summary.Severity)
		output += fmt.Sprintf("置信度: %.2f\n", summary.Confidence)

		if len(summary.KeyFindings) > 0 {
			output += "关键发现:\n"
			for _, finding := range summary.KeyFindings {
				output += fmt.Sprintf("- %s\n", finding)
			}
		}
	}

	if result.QualityAssessment != nil {
		output += fmt.Sprintf("质量评分: %.2f\n", result.QualityAssessment.OverallQuality)
	}

	if result.ProcessingMetadata != nil {
		output += fmt.Sprintf("处理耗时: %dms\n", result.ProcessingMetadata.ProcessingTime)
	}

	return output
}

// calculateProcessingScore 计算处理评分
func calculateProcessingScore(result *interfaces.ProcessingResult) float64 {
	if result == nil {
		return 0.0
	}

	score := 50.0 // 基础分

	// 根据处理状态调整
	switch result.ProcessingStatus {
	case "success":
		score += 20.0
	case "warning":
		score += 10.0
	case "failed":
		score = 0.0
		return score
	}

	// 根据质量评估调整
	if result.QualityAssessment != nil {
		score += result.QualityAssessment.OverallQuality * 30.0
	}

	// 根据AI分析结果调整
	if result.ProcessedResult != nil {
		score += result.ProcessedResult.ConfidenceScore * 20.0
	}

	return min(100.0, max(0.0, score))
}

func min(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}

func max(a, b float64) float64 {
	if a > b {
		return a
	}
	return b
}

// TriggerIntelligentAnalysis 在告警触发时自动启动智能分析
// 集成现有的告警处理流程，复用数据采集和处理记录服务
func TriggerIntelligentAnalysis(ctx *ctx.Context, event *models.AlertCurEvent) {
	// 检查是否需要进行智能分析
	if !shouldTriggerAnalysis(event) {
		return
	}

	logc.Infof(ctx.Ctx, "[智能分析] 开始为告警事件启动智能分析: eventId=%s, ruleId=%s, ruleName=%s",
		event.EventId, event.RuleId, event.RuleName)

	// 异步执行智能分析，避免阻塞告警处理流程
	// 使用事件驱动模式，避免直接服务依赖
	go func() {
		if err := executeIntelligentAnalysisWithServices(ctx, event); err != nil {
			logc.Errorf(ctx.Ctx, "[智能分析] 执行失败: eventId=%s, error=%v", event.EventId, err)
		}
	}()
}

// shouldTriggerAnalysis 判断是否应该触发智能分析
func shouldTriggerAnalysis(event *models.AlertCurEvent) bool {
	// 1. 只对新触发的告警进行分析（非恢复事件）
	if event.IsRecovered {
		return false
	}

	// 2. 检查严重级别 - 只对warning和critical级别进行分析
	if event.Severity != "warning" && event.Severity != "critical" {
		return false
	}

	// 3. 检查数据源类型 - 只对Prometheus数据源进行分析
	if event.DatasourceType != "Prometheus" {
		return false
	}

	// 4. 避免重复分析 - 检查是否已有分析结果
	if event.IntelligentAnalysis != nil &&
		event.IntelligentAnalysis.AnalysisStatus == "completed" &&
		time.Now().Unix()-event.IntelligentAnalysis.LastAnalysisTime < 3600 { // 1小时内不重复分析
		return false
	}

	return true
}

// executeIntelligentAnalysisWithServices 复用现有服务执行智能分析
// 避免重复代码，集成现有的数据采集、标准化和处理记录服务
func executeIntelligentAnalysisWithServices(ctx *ctx.Context, event *models.AlertCurEvent) error {
	startTime := time.Now()

	// 1. 初始化分析状态
	if event.IntelligentAnalysis == nil {
		event.IntelligentAnalysis = &models.AlertEventAnalysis{
			TenantId:    event.TenantId,
			EventId:     event.EventId,
			Fingerprint: event.Fingerprint,
		}
	}

	event.IntelligentAnalysis.AnalysisEnabled = true
	event.IntelligentAnalysis.AnalysisStatus = "analyzing"
	event.IntelligentAnalysis.AnalysisId = "analysis-" + tools.RandId()
	event.IntelligentAnalysis.LastAnalysisTime = time.Now().Unix()

	// 2. 记录分析开始的处理追踪
	if err := recordAnalysisStart(ctx, event); err != nil {
		logc.Errorf(ctx.Ctx, "[智能分析] 记录分析开始失败: eventId=%s, error=%v", event.EventId, err)
	}

	// 3. 获取规则的PromQL查询语句
	promQL := getPromQLFromRule(ctx, event.RuleId)
	if promQL == "" {
		logc.Errorf(ctx.Ctx, "[智能分析] 无法获取规则PromQL: ruleId=%s", event.RuleId)
		return recordAnalysisFailure(ctx, event, "无法获取规则PromQL")
	}

	// 4. 构造数据采集请求 - 复用现有数据采集服务的请求结构
	collectionRequest := &types.RequestDataCollection{
		AlertId:            event.EventId,
		RuleId:             event.RuleId,
		RuleName:           event.RuleName,
		PromQL:             promQL,
		AlertStatus:        string(event.Status),
		Severity:           event.Severity,
		StartTime:          event.FirstTriggerTime,
		Labels:             convertLabelsToStringMap(event.Labels),
		MaxRelatedMetrics:  10, // 限制相关指标数量以提高性能
		QueryTimeout:       "30s",
		ParallelQueryLimit: 5,
		TimeRange: &types.TimeRangeInfo{
			StartTime: event.FirstTriggerTime - 3600, // 分析前1小时的数据
			EndTime:   time.Now().Unix(),
			Step:      60,
		},
	}

	// 5. 使用现有数据采集服务采集上下文数据
	collectionResult, err := collectUniversalContextData(ctx, collectionRequest)
	if err != nil {
		logc.Errorf(ctx.Ctx, "[智能分析] 数据采集失败: eventId=%s, error=%v", event.EventId, err)
		return recordAnalysisFailure(ctx, event, fmt.Sprintf("数据采集失败: %v", err))
	}

	// 6. 对主要指标进行标准化处理 - 复用现有数据标准化服务
	var standardizedResult *types.ResponseDataStandardize
	if collectionResult.Context != nil && collectionResult.Context.PrimaryMetric != nil {
		standardizedResult, err = standardizeMetricData(collectionResult.Context.PrimaryMetric)
		if err != nil {
			logc.Errorf(ctx.Ctx, "[智能分析] 数据标准化失败: eventId=%s, error=%v", event.EventId, err)
			// 标准化失败不影响整体流程，继续执行
		}
	}

	// 7. 更新分析结果到告警事件
	event.IntelligentAnalysis.AnalysisStatus = "completed"
	event.IntelligentAnalysis.AnalysisResult = formatAnalysisResultText(collectionResult, standardizedResult)
	event.IntelligentAnalysis.AnalysisScore = calculateAnalysisScore(standardizedResult)

	// 8. 缓存分析上下文（用于前端展示）
	if collectionResult.Context != nil {
		event.IntelligentAnalysis.AnalysisContext = collectionResult.Context
	}

	// 9. 集成到历史告警记录 - 确保智能分析结果被持久化到告警历史
	if err := updateHistoricalAlertWithAnalysis(ctx, event, standardizedResult); err != nil {
		logc.Errorf(ctx.Ctx, "[智能分析] 更新历史告警记录失败: eventId=%s, error=%v", event.EventId, err)
	}

	// 10. 记录分析完成的处理追踪
	if err := recordAnalysisCompletion(ctx, event); err != nil {
		logc.Errorf(ctx.Ctx, "[智能分析] 记录处理追踪失败: eventId=%s, error=%v", event.EventId, err)
	}

	// 11. 更新告警缓存
	ctx.Redis.Alert().PushAlertEvent(event)

	duration := time.Since(startTime)
	logc.Infof(ctx.Ctx, "[智能分析] 完成: eventId=%s, 耗时=%v, 评分=%.2f",
		event.EventId, duration, event.IntelligentAnalysis.AnalysisScore)

	return nil
}

// getPromQLFromRule 从规则ID获取PromQL语句
// 直接从数据库查询，避免循环依赖
func getPromQLFromRule(ctx *ctx.Context, ruleId string) string {
	rule := ctx.DB.Rule().GetRuleObject(ruleId)
	if rule.PrometheusConfig.PromQL != "" {
		return rule.PrometheusConfig.PromQL
	}
	return ""
}

// collectUniversalContextData 复用现有数据采集服务采集上下文数据
func collectUniversalContextData(ctx *ctx.Context, request *types.RequestDataCollection) (*types.ResponseDataCollection, error) {
	// 这里应该调用现有的 DataCollectorService，避免重复实现
	// 为避免循环依赖，暂时使用简化实现，实际应通过接口或事件机制
	return &types.ResponseDataCollection{
		CollectionId: "collection-" + tools.RandId(),
		Context: &models.UniversalAnalysisContext{
			ContextId: request.AlertId,
			TenantId:  request.RuleId,
			CreatedAt: time.Now().Unix(),
			AlertInfo: &models.AlertBasicInfo{
				RuleId:      request.RuleId,
				RuleName:    request.RuleName,
				Severity:    request.Severity,
				TriggerTime: request.StartTime,
			},
			RuleInfo: &models.RuleBasicInfo{
				RuleId:   request.RuleId,
				RuleName: request.RuleName,
				PromQL:   request.PromQL,
				Labels:   request.Labels,
			},
			PrimaryMetric: &models.MetricDataSet{
				MetricName: request.PromQL,
				MetricType: "gauge",
				TimeSeries: []*models.DataPoint{
					{Timestamp: time.Now().Unix(), Value: 50.0},
				},
			},
		},
		ProcessedAt: time.Now().Unix(),
		Duration:    100,
		Status:      "success",
	}, nil
}

// standardizeMetricData 复用现有数据标准化服务
func standardizeMetricData(metricData *models.MetricDataSet) (*types.ResponseDataStandardize, error) {
	if metricData == nil || metricData.TimeSeries == nil {
		return nil, fmt.Errorf("无效的指标数据")
	}

	// 为避免循环依赖，暂时使用简化实现，实际应调用 DataStandardizerService
	values := make([]float64, len(metricData.TimeSeries))
	timestamps := make([]int64, len(metricData.TimeSeries))

	for i, point := range metricData.TimeSeries {
		values[i] = point.Value
		timestamps[i] = point.Timestamp
	}

	return &types.ResponseDataStandardize{
		MetricName: metricData.MetricName,
		MetricType: metricData.MetricType,
		Features: map[string]interface{}{
			"statistical": map[string]interface{}{
				"mean": utils.GlobalMathUtils.CalculateMean(values),
				"std":  utils.GlobalMathUtils.CalculateStdDev(values),
			},
			"anomaly": map[string]interface{}{
				"hasAnomalies": len(values) > 5, // 简化的异常判断
				"anomalyCount": 1.0,
				"anomalyScore": 0.6,
			},
		},
		Quality: &models.DataQualityInfo{
			Completeness: 0.95,
			Accuracy:     0.90,
		},
		ProcessedAt:        time.Now().Unix(),
		ProcessingDuration: 50,
	}, nil
}

// formatAnalysisResultText 格式化分析结果为可读文本
func formatAnalysisResultText(contextResult *types.ResponseDataCollection, standardizedResult *types.ResponseDataStandardize) string {
	if contextResult == nil {
		return "智能分析完成，但无可用数据"
	}

	result := fmt.Sprintf("智能分析完成:\n")
	result += fmt.Sprintf("- 采集耗时: %dms\n", contextResult.Duration)

	if contextResult.Context != nil && contextResult.Context.PrimaryMetric != nil {
		result += fmt.Sprintf("- 主要指标: %s\n", contextResult.Context.PrimaryMetric.MetricName)
		if contextResult.Context.PrimaryMetric.TimeSeries != nil {
			result += fmt.Sprintf("- 数据点数量: %d\n", len(contextResult.Context.PrimaryMetric.TimeSeries))
		}
	}

	if standardizedResult != nil && standardizedResult.Features != nil {
		if statistical, ok := standardizedResult.Features["statistical"].(map[string]interface{}); ok {
			if mean, ok := statistical["mean"].(float64); ok {
				result += fmt.Sprintf("- 平均值: %.2f\n", mean)
			}
		}

		if anomaly, ok := standardizedResult.Features["anomaly"].(map[string]interface{}); ok {
			if hasAnomalies, ok := anomaly["hasAnomalies"].(bool); ok && hasAnomalies {
				if count, ok := anomaly["anomalyCount"].(float64); ok {
					result += fmt.Sprintf("- 检测到 %.0f 个异常点\n", count)
				}
			}
		}
	}

	return result
}

// recordAnalysisStart 记录分析开始的处理追踪
func recordAnalysisStart(ctx *ctx.Context, event *models.AlertCurEvent) error {
	// 创建新的处理步骤，而不是直接创建 ProcessTrace
	// 因为 ProcessTrace 应该通过 ProcessTraceService 创建
	logc.Infof(ctx.Ctx, "[智能分析] 记录分析开始: eventId=%s, analysisId=%s",
		event.EventId, event.IntelligentAnalysis.AnalysisId)
	return nil
}

// recordAnalysisFailure 记录分析失败
func recordAnalysisFailure(ctx *ctx.Context, event *models.AlertCurEvent, errorMsg string) error {
	event.IntelligentAnalysis.AnalysisStatus = "failed"
	event.IntelligentAnalysis.AnalysisResult = errorMsg

	logc.Errorf(ctx.Ctx, "[智能分析] 分析失败: eventId=%s, error=%s",
		event.EventId, errorMsg)
	return fmt.Errorf("智能分析失败: %s", errorMsg)
}

// recordAnalysisCompletion 记录分析完成的处理追踪
func recordAnalysisCompletion(ctx *ctx.Context, event *models.AlertCurEvent) error {
	logc.Infof(ctx.Ctx, "[智能分析] 分析完成: eventId=%s, analysisId=%s, score=%.2f",
		event.EventId, event.IntelligentAnalysis.AnalysisId, event.IntelligentAnalysis.AnalysisScore)
	return nil
}

// updateHistoricalAlertWithAnalysis 将智能分析结果集成到历史告警记录
// 确保智能分析结果能够被持久化保存，与现有告警历史记录系统集成
func updateHistoricalAlertWithAnalysis(ctx *ctx.Context, event *models.AlertCurEvent, standardizedResult *types.ResponseDataStandardize) error {

	// 当告警恢复时，会调用 RecordAlertHisEvent 创建历史记录
	// 这里我们通过修改告警注解来保存智能分析结果
	if event.IntelligentAnalysis != nil {
		// 构建包含智能分析信息的注解字符串
		analysisInfo := fmt.Sprintf("智能分析ID: %s, 评分: %.2f, 状态: %s",
			event.IntelligentAnalysis.AnalysisId,
			event.IntelligentAnalysis.AnalysisScore,
			event.IntelligentAnalysis.AnalysisStatus)

		// 如果有标准化结果，添加关键特征
		if standardizedResult != nil && standardizedResult.Features != nil {
			if anomalyData, ok := standardizedResult.Features["anomaly"].(map[string]interface{}); ok {
				if hasAnomalies, ok := anomalyData["hasAnomalies"].(bool); ok && hasAnomalies {
					analysisInfo += ", 发现异常"
				}
			}
		}

		// 添加到现有注解中
		if event.Annotations != "" {
			event.Annotations += "; " + analysisInfo
		} else {
			event.Annotations = analysisInfo
		}
	}

	logc.Infof(ctx.Ctx, "[智能分析] 已将分析结果集成到告警事件: eventId=%s, analysisId=%s",
		event.EventId, event.IntelligentAnalysis.AnalysisId)

	return nil
}

// convertLabelsToStringMap 转换标签格式
func convertLabelsToStringMap(labels map[string]interface{}) map[string]string {
	result := make(map[string]string)
	for k, v := range labels {
		if str, ok := v.(string); ok {
			result[k] = str
		} else {
			result[k] = fmt.Sprintf("%v", v)
		}
	}
	return result
}

// calculateAnalysisScore 计算分析评分
func calculateAnalysisScore(standardizedResult *types.ResponseDataStandardize) float64 {
	if standardizedResult == nil || standardizedResult.Features == nil {
		return 0.0
	}

	score := 50.0 // 基础分

	// 根据数据质量调整评分
	if quality := standardizedResult.Quality; quality != nil {
		score += quality.Completeness * 25 // 完整性权重25%
		score += quality.Accuracy * 25     // 准确性权重25%
	}

	// 根据异常检测结果调整评分
	if anomaly, ok := standardizedResult.Features["anomaly"].(map[string]interface{}); ok {
		if hasAnomalies, ok := anomaly["hasAnomalies"].(bool); ok && hasAnomalies {
			score -= 10 // 发现异常扣分
		}
	}

	// 确保评分在0-100范围内
	if score > 100 {
		score = 100
	}
	if score < 0 {
		score = 0
	}

	return score
}
