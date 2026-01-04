package services

import (
	"alertHub/internal/models"
	"alertHub/internal/types"
	"alertHub/pkg/executor"
	"alertHub/pkg/idutil"
	"alertHub/pkg/metadata"
	"alertHub/pkg/provider"
	"fmt"
	"time"
)

// UniversalDataCollector 通用数据采集器 - 最简化设计
type UniversalDataCollector struct {
	PrometheusProvider provider.MetricsFactoryProvider
	MetadataExtractor  *metadata.MetadataExtractor
	ParallelExecutor   *executor.ParallelQueryExecutor
}

// CollectUniversalContext 采集通用分析上下文 - 核心方法
func (udc *UniversalDataCollector) CollectUniversalContext(
	alertInfo *models.AlertBasicInfo,
	ruleInfo *models.RuleBasicInfo,
	collectionConfig *types.DataCollectionConfig,
) (*models.UniversalAnalysisContext, error) {

	startTime := time.Now()
	collectionId := idutil.GenerateCollectionId()

	// 1. 提取主要指标的元数据
	metricMetadata, err := udc.MetadataExtractor.ExtractFromPromQL(ruleInfo.PromQL)
	if err != nil {
		return nil, fmt.Errorf("提取指标元数据失败: %w", err)
	}

	// 2. 构建基础查询计划
	queryPlan := &types.QueryExecutionPlan{
		PrimaryQuery: &types.QueryTask{
			TaskId:     idutil.GenerateTaskId("primary"),
			MetricName: metricMetadata.MetricName,
			Query:      ruleInfo.PromQL,
			TimeRange:  getDefaultTimeRange(),
			Priority:   1.0,
		},
		ParallelQueries:  []*types.QueryTask{},
		DependentQueries: []*types.QueryTask{},
		ExecutionOrder:   []string{},
		EstimatedTime:    1000, // 1秒估计
	}

	// 3. 执行查询
	collectionResult, err := udc.ParallelExecutor.ExecuteParallel(queryPlan)
	if err != nil {
		return nil, fmt.Errorf("查询执行失败: %w", err)
	}

	// 4. 构建简化的分析上下文
	universalContext := &models.UniversalAnalysisContext{
		ContextId:      collectionId,
		TenantId:       alertInfo.RuleId,
		CreatedAt:      time.Now().Unix(),
		AlertInfo:      alertInfo,
		RuleInfo:       ruleInfo,
		PrimaryMetric:  collectionResult.PrimaryMetric,
		RelatedMetrics: collectionResult.RelatedMetrics,
		MetricFeatures: nil,
		SystemContext:  buildSimpleSystemContext(alertInfo),
		TimeContext:    buildSimpleTimeContext(alertInfo),
		AnalysisConfig: buildSimpleAnalysisConfig(),
		Extensions: map[string]interface{}{
			"collectionMetadata": &types.DataCollectionMetadata{
				CollectionId: collectionId,
				StartTime:    startTime.UnixMilli(),
				EndTime:      time.Now().UnixMilli(),
				Duration:     time.Since(startTime).Milliseconds(),
				SourceConfig: collectionConfig,
			},
		},
	}

	return universalContext, nil
}

// NewUniversalDataCollector 创建最简化的数据采集器实例
func NewUniversalDataCollector(prometheusProvider provider.MetricsFactoryProvider) *UniversalDataCollector {
	promqlParser := metadata.NewPrometheusQLParser()
	metricRegistry := metadata.NewMetricRegistry()
	metadataExtractor := metadata.NewMetadataExtractor(promqlParser, metricRegistry)
	parallelExecutor := executor.NewParallelQueryExecutor(prometheusProvider)

	return &UniversalDataCollector{
		PrometheusProvider: prometheusProvider,
		MetadataExtractor:  metadataExtractor,
		ParallelExecutor:   parallelExecutor,
	}
}

// 简化的辅助函数
func buildSimpleSystemContext(alertInfo *models.AlertBasicInfo) *models.SystemContextInfo {
	return &models.SystemContextInfo{
		Environment: "production",
		Region:      "default",
		ServiceName: "unknown-service",
		ServiceType: "service",
	}
}

func buildSimpleTimeContext(alertInfo *models.AlertBasicInfo) *models.TimeContextInfo {
	now := time.Now()
	timeContext := &models.TimeContextInfo{
		AnalysisTime: now.Unix(),
		TimeZone:     now.Location().String(),
	}

	if alertInfo != nil && alertInfo.TriggerTime > 0 {
		timeContext.EventTime = alertInfo.TriggerTime
		alertTime := time.Unix(alertInfo.TriggerTime, 0)
		timeContext.IsBusinessHours = isWorkingHours(alertTime)
	}

	return timeContext
}

func buildSimpleAnalysisConfig() *models.AnalysisConfiguration {
	return &models.AnalysisConfiguration{
		AnalysisType:  "auto",
		AnalysisMode:  "basic",
		AnalysisScope: "current",
		DataCollectionConfig: &models.DataCollectionConfig{
			MaxRelatedMetrics:  10,
			QueryTimeout:       "30s",
			ParallelQueryLimit: 5,
		},
		AiAnalysisConfig: &models.AiAnalysisConfig{
			AiModel:            "claude-3-sonnet",
			MaxTokens:          2000,
			Temperature:        0.1,
			AnalysisDepth:      "basic",
			PromptTemplate:     "simple_analysis",
			ResponseFormat:     "json",
		},
	}
}

func getDefaultTimeRange() *types.TimeRangeInfo {
	now := time.Now()
	return &types.TimeRangeInfo{
		StartTime: now.Add(-1 * time.Hour).Unix(),
		EndTime:   now.Unix(),
		Step:      60, // 1分钟步长
	}
}

func isWorkingHours(t time.Time) bool {
	weekday := t.Weekday()
	hour := t.Hour()
	return weekday >= time.Monday && weekday <= time.Friday && hour >= 9 && hour < 18
}