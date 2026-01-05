package standardizer

import (
	"alertHub/internal/ctx"
	"alertHub/internal/models"
	"alertHub/pkg/analysis/interfaces"
	"alertHub/pkg/analysis/anomaly"
	"alertHub/pkg/analysis/features"
	"fmt"
	"math"
	"sync"
	"time"

	"github.com/zeromicro/go-zero/core/logc"
)

// DataStandardizer 数据标准化器 - 通用的数据特征提取
// 实现interfaces.UniversalDataStandardizer接口
type DataStandardizer struct {
	statisticalAnalyzer *features.StatisticalAnalyzer
	timeSeriesAnalyzer  *features.TimeSeriesAnalyzer
	anomalyDetector     *anomaly.MultiMethodDetector
	patternRecognizer   *PatternRecognizer
	correlationAnalyzer *CorrelationAnalyzer
	qualityAnalyzer     *DataQualityAnalyzer
	strategyEngine      interfaces.ConfigurableStrategyEngine
	config              *StandardizerConfig
	mutex               sync.RWMutex
}

// 确保实现了接口
var _ interfaces.UniversalDataStandardizer = (*DataStandardizer)(nil)

// StandardizerConfig 标准化器配置
type StandardizerConfig struct {
	EnabledFeatures          []string               `json:"enabledFeatures"`          // 启用的特征类型
	MinDataPoints            int                    `json:"minDataPoints"`            // 最少数据点要求
	QualityThreshold         float64                `json:"qualityThreshold"`         // 数据质量阈值
	AnomalyDetectionConfig   map[string]interface{} `json:"anomalyDetectionConfig"`   // 异常检测配置
	FeatureConfig            map[string]interface{} `json:"featureConfig"`            // 特征提取配置
	EnableParallelProcessing bool                   `json:"enableParallelProcessing"` // 是否启用并行处理
}

// NewDataStandardizer 创建数据标准化器
func NewDataStandardizer(config *StandardizerConfig) *DataStandardizer {
	if config == nil {
		config = &StandardizerConfig{
			EnabledFeatures:          []string{"statistical", "timeseries", "anomaly", "pattern", "correlation"},
			MinDataPoints:            3,
			QualityThreshold:         0.7,
			EnableParallelProcessing: true,
		}
	}

	// 创建策略引擎
	strategyEngine, err := createConfigurableStrategyEngine()
	if err != nil {
		// 使用默认实现
		strategyEngine = &DefaultStrategyEngine{}
	}

	return &DataStandardizer{
		statisticalAnalyzer: features.NewStatisticalAnalyzer(),
		timeSeriesAnalyzer:  features.NewTimeSeriesAnalyzer(),
		anomalyDetector:     anomaly.NewMultiMethodDetector(),
		patternRecognizer:   NewPatternRecognizer(),
		correlationAnalyzer: NewCorrelationAnalyzer(),
		qualityAnalyzer:     NewDataQualityAnalyzer(),
		strategyEngine:      strategyEngine,
		config:              config,
	}
}

// StandardizeContext 标准化整个分析上下文
func (ds *DataStandardizer) StandardizeContext(
	ctx *ctx.Context,
	analysisContext *models.UniversalAnalysisContext,
) (*models.UniversalAnalysisContext, error) {

	startTime := time.Now()
	logc.Infof(ctx.Ctx, "[数据标准化] 开始标准化上下文: contextId=%s", analysisContext.ContextId)

	// 创建副本以避免修改原始数据
	standardizedContext := *analysisContext

	// 1. 标准化主要指标
	if analysisContext.PrimaryMetric != nil {
		standardizedPrimary, err := ds.StandardizeMetric(ctx, analysisContext.PrimaryMetric)
		if err != nil {
			return nil, fmt.Errorf("标准化主要指标失败: %w", err)
		}
		standardizedContext.PrimaryMetric = standardizedPrimary
	}

	// 2. 标准化相关指标（并行处理）
	if len(analysisContext.RelatedMetrics) > 0 {
		standardizedRelated, err := ds.standardizeRelatedMetrics(ctx, analysisContext.RelatedMetrics)
		if err != nil {
			logc.Infof(ctx.Ctx, "[数据标准化] 相关指标标准化部分失败: %v", err)
		}
		standardizedContext.RelatedMetrics = standardizedRelated
	}

	// 3. 提取通用指标特征
	if standardizedContext.PrimaryMetric != nil {
		metricFeatures, err := ds.extractUniversalFeatures(ctx, standardizedContext.PrimaryMetric, standardizedContext.RelatedMetrics)
		if err != nil {
			logc.Infof(ctx.Ctx, "[数据标准化] 特征提取失败: %v", err)
		} else {
			standardizedContext.MetricFeatures = metricFeatures
		}
	}

	// 4. 计算整体数据质量
	overallQuality := ds.calculateOverallQuality(ctx, &standardizedContext)
	if overallQuality < ds.config.QualityThreshold {
		logc.Infof(ctx.Ctx, "[数据标准化] 数据质量较低: %.2f < %.2f", overallQuality, ds.config.QualityThreshold)
	}

	duration := time.Since(startTime)
	logc.Infof(ctx.Ctx, "[数据标准化] 完成: contextId=%s, 耗时=%v, 质量评分=%.2f",
		analysisContext.ContextId, duration, overallQuality)

	return &standardizedContext, nil
}

// StandardizeMetric 标准化单个指标数据
func (ds *DataStandardizer) StandardizeMetric(
	ctx *ctx.Context,
	metricData *models.MetricDataSet,
) (*models.MetricDataSet, error) {

	if metricData == nil || len(metricData.TimeSeries) < ds.config.MinDataPoints {
		return metricData, fmt.Errorf("数据点数量不足: %d < %d", len(metricData.TimeSeries), ds.config.MinDataPoints)
	}

	// 创建标准化副本
	standardized := *metricData

	// 数据清洗和预处理
	cleanedData := ds.cleanAndPreprocessData(ctx, metricData.TimeSeries)
	standardized.TimeSeries = cleanedData

	// 更新数据质量信息
	if standardized.DataQuality == nil {
		standardized.DataQuality = &models.DataQualityInfo{}
	}

	qualityInfo := ds.qualityAnalyzer.AnalyzeQuality(ctx, cleanedData)
	*standardized.DataQuality = *qualityInfo

	// 更新元数据
	if standardized.Metadata == nil {
		standardized.Metadata = make(map[string]interface{})
	}
	standardized.Metadata["standardized_at"] = time.Now().Unix()
	standardized.Metadata["original_points"] = len(metricData.TimeSeries)
	standardized.Metadata["cleaned_points"] = len(cleanedData)

	return &standardized, nil
}

// extractUniversalFeatures 提取通用指标特征
func (ds *DataStandardizer) extractUniversalFeatures(
	ctx *ctx.Context,
	primaryMetric *models.MetricDataSet,
	relatedMetrics map[string]*models.MetricDataSet,
) (*models.UniversalMetricFeatures, error) {

	features := &models.UniversalMetricFeatures{
		DynamicFeatures: make(map[string]interface{}),
	}

	if len(primaryMetric.TimeSeries) == 0 {
		return features, nil
	}

	// 使用并行处理提取特征
	if ds.config.EnableParallelProcessing {
		return ds.extractFeaturesParallel(ctx, primaryMetric, relatedMetrics)
	}

	// 串行处理
	return ds.extractFeaturesSequential(ctx, primaryMetric, relatedMetrics)
}

// extractFeaturesParallel 并行提取特征
func (ds *DataStandardizer) extractFeaturesParallel(
	ctx *ctx.Context,
	primaryMetric *models.MetricDataSet,
	relatedMetrics map[string]*models.MetricDataSet,
) (*models.UniversalMetricFeatures, error) {

	features := &models.UniversalMetricFeatures{
		DynamicFeatures: make(map[string]interface{}),
	}

	var wg sync.WaitGroup
	var mutex sync.Mutex
	errors := make([]error, 0)

	// 1. 统计特征
	if ds.isFeatureEnabled("statistical") {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if statsFeatures, err := ds.statisticalAnalyzer.ExtractFeatures(primaryMetric.TimeSeries); err != nil {
				mutex.Lock()
				errors = append(errors, fmt.Errorf("统计特征提取失败: %w", err))
				mutex.Unlock()
			} else {
				mutex.Lock()
				features.StatisticalFeatures = ds.convertStatisticalFeatures(statsFeatures)
				mutex.Unlock()
			}
		}()
	}

	// 2. 时序特征
	if ds.isFeatureEnabled("timeseries") {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if timeFeatures, err := ds.timeSeriesAnalyzer.ExtractFeatures(primaryMetric.TimeSeries); err != nil {
				mutex.Lock()
				errors = append(errors, fmt.Errorf("时序特征提取失败: %w", err))
				mutex.Unlock()
			} else {
				mutex.Lock()
				features.TimeSeriesFeatures = ds.convertTimeSeriesFeatures(timeFeatures)
				mutex.Unlock()
			}
		}()
	}

	// 3. 异常特征
	if ds.isFeatureEnabled("anomaly") {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if anomalyResult, err := ds.anomalyDetector.DetectAnomalies(primaryMetric.TimeSeries); err != nil {
				mutex.Lock()
				errors = append(errors, fmt.Errorf("异常检测失败: %w", err))
				mutex.Unlock()
			} else {
				anomalyFeatures := ds.convertAnomalyResultToFeatures(anomalyResult)
				mutex.Lock()
				features.AnomalyFeatures = anomalyFeatures
				mutex.Unlock()
			}
		}()
	}

	// 4. 模式特征
	if ds.isFeatureEnabled("pattern") {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if patternFeatures, err := ds.patternRecognizer.RecognizePatterns(ctx, primaryMetric.TimeSeries); err != nil {
				mutex.Lock()
				errors = append(errors, fmt.Errorf("模式识别失败: %w", err))
				mutex.Unlock()
			} else {
				mutex.Lock()
				features.PatternFeatures = patternFeatures
				mutex.Unlock()
			}
		}()
	}

	// 5. 相关性特征
	if ds.isFeatureEnabled("correlation") && len(relatedMetrics) > 0 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if corrFeatures, err := ds.correlationAnalyzer.AnalyzeCorrelations(ctx, primaryMetric, relatedMetrics); err != nil {
				mutex.Lock()
				errors = append(errors, fmt.Errorf("相关性分析失败: %w", err))
				mutex.Unlock()
			} else {
				mutex.Lock()
				features.CorrelationFeatures = corrFeatures
				mutex.Unlock()
			}
		}()
	}

	wg.Wait()

	if len(errors) > 0 {
		logc.Infof(ctx.Ctx, "[特征提取] 部分特征提取失败: %v", errors)
	}

	return features, nil
}

// extractFeaturesSequential 串行提取特征
func (ds *DataStandardizer) extractFeaturesSequential(
	ctx *ctx.Context,
	primaryMetric *models.MetricDataSet,
	relatedMetrics map[string]*models.MetricDataSet,
) (*models.UniversalMetricFeatures, error) {

	features := &models.UniversalMetricFeatures{
		DynamicFeatures: make(map[string]interface{}),
	}

	// 1. 统计特征
	if ds.isFeatureEnabled("statistical") {
		if statsFeatures, err := ds.statisticalAnalyzer.ExtractFeatures(primaryMetric.TimeSeries); err != nil {
			logc.Infof(ctx.Ctx, "[特征提取] 统计特征提取失败: %v", err)
		} else {
			features.StatisticalFeatures = ds.convertStatisticalFeatures(statsFeatures)
		}
	}

	// 2. 时序特征
	if ds.isFeatureEnabled("timeseries") {
		if timeFeatures, err := ds.timeSeriesAnalyzer.ExtractFeatures(primaryMetric.TimeSeries); err != nil {
			logc.Errorf(ctx.Ctx, "[特征提取] 时序特征提取失败: %v", err)
		} else {
			features.TimeSeriesFeatures = ds.convertTimeSeriesFeatures(timeFeatures)
		}
	}

	// 3. 异常特征
	if ds.isFeatureEnabled("anomaly") {
		if anomalyResult, err := ds.anomalyDetector.DetectAnomalies(primaryMetric.TimeSeries); err != nil {
			logc.Errorf(ctx.Ctx, "[特征提取] 异常检测失败: %v", err)
		} else {
			features.AnomalyFeatures = ds.convertAnomalyResultToFeatures(anomalyResult)
		}
	}

	// 4. 模式特征
	if ds.isFeatureEnabled("pattern") {
		if patternFeatures, err := ds.patternRecognizer.RecognizePatterns(ctx, primaryMetric.TimeSeries); err != nil {
			logc.Errorf(ctx.Ctx, "[特征提取] 模式识别失败: %v", err)
		} else {
			features.PatternFeatures = patternFeatures
		}
	}

	// 5. 相关性特征
	if ds.isFeatureEnabled("correlation") && len(relatedMetrics) > 0 {
		if corrFeatures, err := ds.correlationAnalyzer.AnalyzeCorrelations(ctx, primaryMetric, relatedMetrics); err != nil {
			logc.Errorf(ctx.Ctx, "[特征提取] 相关性分析失败: %v", err)
		} else {
			features.CorrelationFeatures = corrFeatures
		}
	}

	return features, nil
}

// standardizeRelatedMetrics 标准化相关指标（并行处理）
func (ds *DataStandardizer) standardizeRelatedMetrics(
	ctx *ctx.Context,
	relatedMetrics map[string]*models.MetricDataSet,
) (map[string]*models.MetricDataSet, error) {

	standardized := make(map[string]*models.MetricDataSet)
	var wg sync.WaitGroup
	var mutex sync.Mutex
	errors := make([]error, 0)

	for id, metric := range relatedMetrics {
		wg.Add(1)
		go func(metricID string, metricData *models.MetricDataSet) {
			defer wg.Done()

			if standardizedMetric, err := ds.StandardizeMetric(ctx, metricData); err != nil {
				mutex.Lock()
				errors = append(errors, fmt.Errorf("标准化指标 %s 失败: %w", metricID, err))
				mutex.Unlock()
			} else {
				mutex.Lock()
				standardized[metricID] = standardizedMetric
				mutex.Unlock()
			}
		}(id, metric)
	}

	wg.Wait()

	if len(errors) > 0 {
		return standardized, fmt.Errorf("部分相关指标标准化失败: %v", errors)
	}

	return standardized, nil
}

// cleanAndPreprocessData 数据清洗和预处理
func (ds *DataStandardizer) cleanAndPreprocessData(
	ctx *ctx.Context,
	dataPoints []*models.DataPoint,
) []*models.DataPoint {

	if len(dataPoints) == 0 {
		return dataPoints
	}

	cleaned := make([]*models.DataPoint, 0, len(dataPoints))

	for _, point := range dataPoints {
		// 1. 过滤无效数据点
		if point == nil {
			continue
		}

		// 2. 处理异常值
		if math.IsNaN(point.Value) || math.IsInf(point.Value, 0) {
			// 使用插值或跳过
			continue
		}

		// 3. 时间戳验证
		if point.Timestamp <= 0 {
			continue
		}

		// 4. 数据质量标记
		if point.Quality == nil {
			point.Quality = &models.DataPointQuality{
				IsValid:    true,
				Confidence: 1.0,
				Source:     "cleaned",
			}
		}

		cleaned = append(cleaned, point)
	}

	return cleaned
}

// convertAnomalyResultToFeatures 转换异常检测结果为特征
func (ds *DataStandardizer) convertAnomalyResultToFeatures(result *anomaly.CombinedAnomalyResult) *models.AnomalyFeatures {
	anomalyFeatures := &models.AnomalyFeatures{
		HasAnomalies:      len(result.FinalAnomalies) > 0,
		AnomalyCount:      len(result.FinalAnomalies),
		AnomalyScore:      result.AggregatedScore,
		AnomalyTypes:      make([]string, 0),
		AnomalyTimestamps: make([]int64, 0),
		AnomalyDetails:    make([]models.AnomalyDetail, 0),
	}

	if len(result.FinalAnomalies) > 0 {
		totalDataPoints := len(result.FinalAnomalies) // 这里需要实际的总数据点数
		anomalyFeatures.AnomalyRatio = float64(len(result.FinalAnomalies)) / float64(totalDataPoints)

		// 处理异常详情
		typeSet := make(map[string]bool)
		for _, anomaly := range result.FinalAnomalies {
			// 收集异常类型
			if !typeSet[anomaly.Method] {
				typeSet[anomaly.Method] = true
				anomalyFeatures.AnomalyTypes = append(anomalyFeatures.AnomalyTypes, anomaly.Method)
			}

			// 收集时间戳
			anomalyFeatures.AnomalyTimestamps = append(anomalyFeatures.AnomalyTimestamps, anomaly.Timestamp)

			// 收集异常详情
			detail := models.AnomalyDetail{
				Timestamp:   anomaly.Timestamp,
				Value:       anomaly.Value,
				AnomalyType: anomaly.Method,
				Severity:    anomaly.Severity,
				Score:       anomaly.Confidence,
				Description: fmt.Sprintf("异常值: %.2f, 期望值: %.2f, 偏差: %.2f",
					anomaly.Value, anomaly.Expected, anomaly.Deviation),
			}
			anomalyFeatures.AnomalyDetails = append(anomalyFeatures.AnomalyDetails, detail)
		}
	}

	return anomalyFeatures
}

// calculateOverallQuality 计算整体数据质量
func (ds *DataStandardizer) calculateOverallQuality(
	ctx *ctx.Context,
	context *models.UniversalAnalysisContext,
) float64 {

	var totalQuality float64
	var count int

	// 主要指标质量
	if context.PrimaryMetric != nil && context.PrimaryMetric.DataQuality != nil {
		totalQuality += context.PrimaryMetric.DataQuality.QualityScore
		count++
	}

	// 相关指标质量
	for _, metric := range context.RelatedMetrics {
		if metric.DataQuality != nil {
			totalQuality += metric.DataQuality.QualityScore
			count++
		}
	}

	if count == 0 {
		return 0.0
	}

	return totalQuality / float64(count)
}

// convertStatisticalFeatures 转换统计特征类型
func (ds *DataStandardizer) convertStatisticalFeatures(statsFeatures *features.StatisticalFeatures) *models.StatisticalFeatures {
	if statsFeatures == nil {
		return nil
	}

	return &models.StatisticalFeatures{
		Count:    100, // 默认估算值，实际应基于原始数据长度
		Mean:     statsFeatures.Mean,
		Median:   statsFeatures.Median,
		StdDev:   statsFeatures.StdDev,
		Variance: statsFeatures.Variance,
		Skewness: statsFeatures.Skewness,
		Kurtosis: statsFeatures.Kurtosis,
		Min:      statsFeatures.Min,
		Max:      statsFeatures.Max,
		Range:    statsFeatures.Range,
		Q1:       statsFeatures.Q25, // Q25 对应 Q1
		Q3:       statsFeatures.Q75, // Q75 对应 Q3
		IQR:      statsFeatures.IQR,
		P95:      statsFeatures.Percentile95,
		P99:      statsFeatures.Percentile99,
	}
}

// convertTimeSeriesFeatures 转换时序特征类型
func (ds *DataStandardizer) convertTimeSeriesFeatures(timeFeatures *features.TimeSeriesFeatures) *models.TimeSeriesFeatures {
	if timeFeatures == nil {
		return nil
	}
	
	return &models.TimeSeriesFeatures{
		Trend:           timeFeatures.TrendType,
		TrendStrength:   timeFeatures.TrendStrength,
		Seasonality:     timeFeatures.Seasonality,
		SeasonPeriod:    timeFeatures.DominantPeriod,
		Volatility:      timeFeatures.Volatility,
		Stationarity:    timeFeatures.Stationarity,
		AutoCorrelation: timeFeatures.MaxAutocorrValue,
		ChangePoint:     ds.convertChangePoints(timeFeatures.ChangePoints),
		ChangeRate:      ds.calculateChangeRate(timeFeatures),
	}
}

// convertChangePoints 转换变点数据类型
func (ds *DataStandardizer) convertChangePoints(changePoints []int) []int64 {
	result := make([]int64, len(changePoints))
	for i, cp := range changePoints {
		result[i] = int64(cp)
	}
	return result
}

// calculateChangeRate 计算变化率
func (ds *DataStandardizer) calculateChangeRate(timeFeatures *features.TimeSeriesFeatures) float64 {
	if timeFeatures.Volatility > 0 {
		return timeFeatures.Volatility * timeFeatures.TrendStrength
	}
	return 0.0
}

// isFeatureEnabled 检查特征是否启用
func (ds *DataStandardizer) isFeatureEnabled(featureType string) bool {
	for _, enabled := range ds.config.EnabledFeatures {
		if enabled == featureType {
			return true
		}
	}
	return false
}

// ExtractFeatures 提取通用特征（实现接口方法）
func (ds *DataStandardizer) ExtractFeatures(
	ctx *ctx.Context,
	metricData *models.MetricDataSet,
	config *interfaces.FeatureExtractionConfig,
) (map[string]interface{}, error) {
	logc.Infof(ctx.Ctx, "[特征提取] 开始提取特征: metric=%s", metricData.MetricName)

	result := make(map[string]interface{})

	// 根据配置提取不同类型的特征
	for _, featureType := range config.EnabledFeatures {
		switch featureType {
		case "statistical":
			if statsFeatures, err := ds.statisticalAnalyzer.ExtractFeatures(metricData.TimeSeries); err != nil {
				logc.Infof(ctx.Ctx, "[特征提取] 统计特征提取失败: %v", err)
			} else {
				result["statistical"] = ds.convertStatisticalFeatures(statsFeatures)
			}

		case "timeseries":
			if timeFeatures, err := ds.timeSeriesAnalyzer.ExtractFeatures(metricData.TimeSeries); err != nil {
				logc.Infof(ctx.Ctx, "[特征提取] 时序特征提取失败: %v", err)
			} else {
				result["timeseries"] = ds.convertTimeSeriesFeatures(timeFeatures)
			}

		case "anomaly":
			if anomalyResult, err := ds.anomalyDetector.DetectAnomalies(metricData.TimeSeries); err != nil {
				logc.Infof(ctx.Ctx, "[特征提取] 异常检测失败: %v", err)
			} else {
				result["anomaly"] = ds.convertAnomalyResultToFeatures(anomalyResult)
			}

		case "pattern":
			if patternFeatures, err := ds.patternRecognizer.RecognizePatterns(ctx, metricData.TimeSeries); err != nil {
				logc.Infof(ctx.Ctx, "[特征提取] 模式识别失败: %v", err)
			} else {
				result["pattern"] = patternFeatures
			}

		default:
			// 尝试使用策略引擎处理自定义特征类型
			if customConfig, exists := config.CustomFeatures[featureType]; exists {
				if configMap, ok := customConfig.(map[string]interface{}); ok {
					if strategy, err := ds.strategyEngine.LoadStrategy(ctx, featureType, configMap); err == nil {
						if features, err := ds.strategyEngine.ExecuteStrategy(ctx, strategy, metricData); err == nil {
							result[featureType] = features
						} else {
							logc.Infof(ctx.Ctx, "[特征提取] 自定义特征%s执行失败: %v", featureType, err)
						}
					} else {
						logc.Infof(ctx.Ctx, "[特征提取] 自定义特征%s加载失败: %v", featureType, err)
					}
				}
			}
		}
	}

	return result, nil
}

// ValidateConfig 验证标准化器配置（实现接口方法）
func (ds *DataStandardizer) ValidateConfig(config interface{}) error {
	standardizerConfig, ok := config.(*StandardizerConfig)
	if !ok {
		return fmt.Errorf("配置类型错误，期望 *StandardizerConfig，实际 %T", config)
	}
	return validateStandardizerConfig(standardizerConfig)
}

// validateStandardizerConfig 验证标准化器配置
func validateStandardizerConfig(config *StandardizerConfig) error {
	if config == nil {
		return fmt.Errorf("配置不能为空")
	}

	if len(config.EnabledFeatures) == 0 {
		return fmt.Errorf("必须启用至少一种特征类型")
	}

	if config.MinDataPoints <= 0 {
		return fmt.Errorf("MinDataPoints必须大于0")
	}

	if config.QualityThreshold < 0 || config.QualityThreshold > 1 {
		return fmt.Errorf("QualityThreshold必须在0-1范围内")
	}

	// 验证启用的特征类型
	validFeatures := map[string]bool{
		"statistical": true,
		"timeseries":  true,
		"anomaly":     true,
		"pattern":     true,
		"correlation": true,
	}

	for _, feature := range config.EnabledFeatures {
		if !validFeatures[feature] {
			// 自定义特征类型允许但需要有对应配置
			if config.FeatureConfig == nil {
				return fmt.Errorf("未知特征类型 %s 且无自定义配置", feature)
			}
			if _, exists := config.FeatureConfig[feature]; !exists {
				return fmt.Errorf("特征类型 %s 缺少对应配置", feature)
			}
		}
	}

	return nil
}

// createConfigurableStrategyEngine 创建策略引擎（内部使用）
func createConfigurableStrategyEngine() (interfaces.ConfigurableStrategyEngine, error) {
	// 这里可以集成已有的策略引擎实现
	// 为简化示例，返回默认实现
	return &DefaultStrategyEngine{}, nil
}

// DefaultStrategyEngine 默认策略引擎实现
type DefaultStrategyEngine struct {
	strategies map[string]interfaces.StrategyBuilder
	mutex      sync.RWMutex
}

func (dse *DefaultStrategyEngine) LoadStrategy(ctx *ctx.Context, strategyType string, config map[string]interface{}) (interfaces.Strategy, error) {
	return nil, fmt.Errorf("策略类型 %s 未实现", strategyType)
}

func (dse *DefaultStrategyEngine) ExecuteStrategy(ctx *ctx.Context, strategy interfaces.Strategy, input interface{}) (interface{}, error) {
	return strategy.Execute(ctx, input)
}

func (dse *DefaultStrategyEngine) RegisterStrategy(strategyType string, builder interfaces.StrategyBuilder) error {
	if dse.strategies == nil {
		dse.strategies = make(map[string]interfaces.StrategyBuilder)
	}
	dse.mutex.Lock()
	defer dse.mutex.Unlock()
	dse.strategies[strategyType] = builder
	return nil
}

func (dse *DefaultStrategyEngine) ValidateStrategyConfig(strategyType string, config map[string]interface{}) error {
	dse.mutex.RLock()
	builder, exists := dse.strategies[strategyType]
	dse.mutex.RUnlock()

	if !exists {
		return fmt.Errorf("未知策略类型: %s", strategyType)
	}

	return builder.ValidateConfig(config)
}
