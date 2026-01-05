package standardizer

import (
	"alertHub/internal/ctx"
	"alertHub/internal/models"
	"alertHub/pkg/analysis/utils"
	"math"
	"sort"
)

// PatternRecognizer 模式识别器
type PatternRecognizer struct {
	config *PatternRecognizerConfig
}

// PatternRecognizerConfig 模式识别器配置
type PatternRecognizerConfig struct {
	MinPatternLength    int     `json:"minPatternLength"`    // 最小模式长度
	MaxPatternLength    int     `json:"maxPatternLength"`    // 最大模式长度
	SimilarityThreshold float64 `json:"similarityThreshold"` // 相似度阈值
	EnableSeasonality   bool    `json:"enableSeasonality"`   // 是否启用季节性检测
	EnableTrend         bool    `json:"enableTrend"`         // 是否启用趋势检测
	EnableCyclical      bool    `json:"enableCyclical"`      // 是否启用周期性检测
}

// NewPatternRecognizer 创建模式识别器
func NewPatternRecognizer() *PatternRecognizer {
	return &PatternRecognizer{
		config: &PatternRecognizerConfig{
			MinPatternLength:    3,
			MaxPatternLength:    24,
			SimilarityThreshold: 0.8,
			EnableSeasonality:   true,
			EnableTrend:         true,
			EnableCyclical:      true,
		},
	}
}

// RecognizePatterns 识别数据中的模式
func (pr *PatternRecognizer) RecognizePatterns(
	ctx *ctx.Context,
	dataPoints []*models.DataPoint,
) (*models.PatternFeatures, error) {
	if len(dataPoints) < pr.config.MinPatternLength {
		return &models.PatternFeatures{}, nil
	}

	// 提取数值序列
	values := make([]float64, len(dataPoints))
	for i, point := range dataPoints {
		values[i] = point.Value
	}

	features := &models.PatternFeatures{
		PatternTypes: make([]string, 0),
		Confidence:   0.0,
	}

	// 1. 趋势检测
	if pr.config.EnableTrend {
		trendType, trendStrength := pr.detectTrend(values)
		features.TrendType = trendType
		features.TrendStrength = trendStrength
		if trendType != "none" {
			features.PatternTypes = append(features.PatternTypes, "trend")
		}
	}

	// 2. 季节性检测
	if pr.config.EnableSeasonality {
		seasonalityType, seasonalPeriod := pr.detectSeasonality(values)
		features.SeasonalityType = seasonalityType
		features.SeasonalPeriod = seasonalPeriod
		if seasonalityType != "none" {
			features.PatternTypes = append(features.PatternTypes, "seasonal")
		}
	}

	// 3. 周期性检测
	if pr.config.EnableCyclical {
		cyclicalType, cyclePeriod := pr.detectCyclical(values)
		features.CyclicalType = cyclicalType
		features.CyclePeriod = int64(cyclePeriod)
		if cyclicalType != "none" {
			features.PatternTypes = append(features.PatternTypes, "cyclical")
		}
	}

	// 计算整体置信度
	features.Confidence = pr.calculateOverallConfidence(features)

	return features, nil
}

// detectTrend 检测趋势
func (pr *PatternRecognizer) detectTrend(values []float64) (string, float64) {
	if len(values) < 3 {
		return "none", 0.0
	}

	// 使用统一的线性回归函数
	slope, correlation := utils.GlobalMathUtils.CalculateLinearRegression(values)

	// 判断趋势类型
	strength := math.Abs(correlation)
	if strength < 0.3 {
		return "none", strength
	} else if slope > 0 {
		return "increasing", strength
	} else {
		return "decreasing", strength
	}
}

// detectSeasonality 检测季节性
func (pr *PatternRecognizer) detectSeasonality(values []float64) (string, int) {
	if len(values) < pr.config.MinPatternLength*2 {
		return "none", 0
	}

	// 检测常见的季节性周期
	commonPeriods := []int{24, 168, 720} // 24小时、7天、30天

	bestPeriod := 0
	bestScore := 0.0

	for _, period := range commonPeriods {
		if period > len(values)/2 {
			continue
		}

		score := pr.calculateSeasonalityScore(values, period)
		if score > bestScore {
			bestScore = score
			bestPeriod = period
		}
	}

	if bestScore > pr.config.SimilarityThreshold {
		return "seasonal", bestPeriod
	}

	return "none", 0
}

// calculateSeasonalityScore 计算季节性分数
func (pr *PatternRecognizer) calculateSeasonalityScore(values []float64, period int) float64 {
	if period >= len(values) {
		return 0.0
	}

	cycles := len(values) / period
	if cycles < 2 {
		return 0.0
	}

	totalCorrelation := 0.0
	comparisonCount := 0

	for i := 0; i < cycles-1; i++ {
		start1 := i * period
		end1 := start1 + period
		start2 := (i + 1) * period
		end2 := start2 + period

		if end2 > len(values) {
			end2 = len(values)
			end1 = start1 + (end2 - start2)
		}

		cycle1 := values[start1:end1]
		cycle2 := values[start2:end2]

		corr := utils.GlobalMathUtils.CalculatePearsonCorrelation(cycle1, cycle2)
		if !math.IsNaN(corr) {
			totalCorrelation += corr
			comparisonCount++
		}
	}

	if comparisonCount == 0 {
		return 0.0
	}

	return totalCorrelation / float64(comparisonCount)
}

// detectCyclical 检测周期性
func (pr *PatternRecognizer) detectCyclical(values []float64) (string, int) {
	if len(values) < pr.config.MinPatternLength*2 {
		return "none", 0
	}

	bestPeriod := 0
	bestScore := 0.0

	for period := pr.config.MinPatternLength; period <= pr.config.MaxPatternLength && period <= len(values)/2; period++ {
		score := pr.calculateCyclicalScore(values, period)
		if score > bestScore {
			bestScore = score
			bestPeriod = period
		}
	}

	if bestScore > pr.config.SimilarityThreshold {
		return "cyclical", bestPeriod
	}

	return "none", 0
}

// calculateCyclicalScore 计算周期性分数
func (pr *PatternRecognizer) calculateCyclicalScore(values []float64, period int) float64 {
	if period >= len(values) {
		return 0.0
	}

	n := len(values)
	mean := utils.GlobalMathUtils.CalculateMean(values)

	numerator := 0.0
	denominator := 0.0

	for i := 0; i < n-period; i++ {
		numerator += (values[i] - mean) * (values[i+period] - mean)
	}

	for _, v := range values {
		denominator += (v - mean) * (v - mean)
	}

	if denominator == 0 {
		return 0.0
	}

	return numerator / denominator
}


// calculateOverallConfidence 计算整体置信度
func (pr *PatternRecognizer) calculateOverallConfidence(features *models.PatternFeatures) float64 {
	totalConfidence := 0.0
	count := 0

	if features.TrendStrength > 0 {
		totalConfidence += features.TrendStrength
		count++
	}

	if features.SeasonalPeriod > 0 {
		totalConfidence += 0.8
		count++
	}

	if features.CyclePeriod > 0 {
		totalConfidence += 0.7
		count++
	}

	if count == 0 {
		return 0.0
	}

	return totalConfidence / float64(count)
}

// CorrelationAnalyzer 相关性分析器
type CorrelationAnalyzer struct {
	config *CorrelationConfig
}

// CorrelationConfig 相关性分析配置
type CorrelationConfig struct {
	MinCorrelationThreshold float64 `json:"minCorrelationThreshold"` // 最小相关性阈值
	MaxLagPeriods           int     `json:"maxLagPeriods"`           // 最大滞后周期
	EnableLagAnalysis       bool    `json:"enableLagAnalysis"`       // 是否启用滞后分析
}

// NewCorrelationAnalyzer 创建相关性分析器
func NewCorrelationAnalyzer() *CorrelationAnalyzer {
	return &CorrelationAnalyzer{
		config: &CorrelationConfig{
			MinCorrelationThreshold: 0.3,
			MaxLagPeriods:           10,
			EnableLagAnalysis:       true,
		},
	}
}

// AnalyzeCorrelations 分析指标间相关性
func (ca *CorrelationAnalyzer) AnalyzeCorrelations(
	ctx *ctx.Context,
	primaryMetric *models.MetricDataSet,
	relatedMetrics map[string]*models.MetricDataSet,
) (*models.CorrelationFeatures, error) {
	features := &models.CorrelationFeatures{
		CrossCorrelations: make(map[string]*models.CorrelationInfo),
		LagAnalysis:       make(map[string]*models.LagAnalysisResult),
	}

	primaryValues := ca.extractValues(primaryMetric.TimeSeries)
	if len(primaryValues) == 0 {
		return features, nil
	}

	// 分析与每个相关指标的相关性
	for metricID, relatedMetric := range relatedMetrics {
		relatedValues := ca.extractValues(relatedMetric.TimeSeries)
		if len(relatedValues) == 0 {
			continue
		}

		// 对齐数据长度
		alignedPrimary, alignedRelated := ca.alignTimeSeries(primaryValues, relatedValues)

		// 计算相关性
		correlation := utils.GlobalMathUtils.CalculatePearsonCorrelation(alignedPrimary, alignedRelated)

		if math.Abs(correlation) >= ca.config.MinCorrelationThreshold {
			corrInfo := &models.CorrelationInfo{
				TargetMetric: relatedMetric.MetricName,
				Correlation:  correlation,
				Significance: ca.categorizeCorrelationStrength(correlation), // 转换为字符串
				Method:       "pearson",
				DataPoints:   len(alignedPrimary),
				Confidence:   ca.calculateCorrelationConfidence(correlation, len(alignedPrimary)),
			}

			features.CrossCorrelations[metricID] = corrInfo

			// 滞后分析
			if ca.config.EnableLagAnalysis {
				lagResult := ca.performLagAnalysis(alignedPrimary, alignedRelated)
				features.LagAnalysis[metricID] = lagResult
			}
		}
	}

	return features, nil
}

// extractValues 提取数值序列
func (ca *CorrelationAnalyzer) extractValues(dataPoints []*models.DataPoint) []float64 {
	values := make([]float64, 0, len(dataPoints))
	for _, point := range dataPoints {
		if point != nil && !math.IsNaN(point.Value) && !math.IsInf(point.Value, 0) {
			values = append(values, point.Value)
		}
	}
	return values
}

// alignTimeSeries 对齐时间序列
func (ca *CorrelationAnalyzer) alignTimeSeries(primary, related []float64) ([]float64, []float64) {
	minLen := len(primary)
	if len(related) < minLen {
		minLen = len(related)
	}
	return primary[:minLen], related[:minLen]
}


// categorizeCorrelationStrength 分类相关性强度
func (ca *CorrelationAnalyzer) categorizeCorrelationStrength(correlation float64) string {
	abs := math.Abs(correlation)
	if abs >= 0.8 {
		return "strong"
	} else if abs >= 0.5 {
		return "moderate"
	} else if abs >= 0.3 {
		return "weak"
	}
	return "negligible"
}

// getCorrelationDirection 获取相关性方向
func (ca *CorrelationAnalyzer) getCorrelationDirection(correlation float64) string {
	if correlation > 0 {
		return "positive"
	} else if correlation < 0 {
		return "negative"
	}
	return "none"
}

// calculateSignificance 计算显著性
func (ca *CorrelationAnalyzer) calculateSignificance(correlation float64, sampleSize int) float64 {
	if sampleSize < 3 {
		return 0.0
	}

	// 简化的显著性判断
	t := correlation * math.Sqrt(float64(sampleSize-2)) / math.Sqrt(1-correlation*correlation)

	if math.Abs(t) > 2.0 {
		return 0.95
	} else if math.Abs(t) > 1.645 {
		return 0.90
	}
	return 0.5
}

// performLagAnalysis 执行滞后分析
func (ca *CorrelationAnalyzer) performLagAnalysis(primary, related []float64) *models.LagAnalysisResult {
	result := &models.LagAnalysisResult{
		BestLag:         0,
		BestCorrelation: 0.0,
		LagCorrelations: make(map[int64]float64),
	}

	// 测试不同的滞后周期
	for lag := -ca.config.MaxLagPeriods; lag <= ca.config.MaxLagPeriods; lag++ {
		var shiftedPrimary, shiftedRelated []float64

		if lag > 0 {
			// 相关指标滞后
			if lag < len(related) {
				shiftedPrimary = primary[:len(primary)-lag]
				shiftedRelated = related[lag:]
			}
		} else if lag < 0 {
			// 主要指标滞后
			absLag := -lag
			if absLag < len(primary) {
				shiftedPrimary = primary[absLag:]
				shiftedRelated = related[:len(related)-absLag]
			}
		} else {
			// 无滞后
			shiftedPrimary = primary
			shiftedRelated = related
		}

		if len(shiftedPrimary) > 0 && len(shiftedRelated) > 0 {
			correlation := utils.GlobalMathUtils.CalculatePearsonCorrelation(shiftedPrimary, shiftedRelated)
			result.LagCorrelations[int64(lag)] = correlation

			if math.Abs(correlation) > math.Abs(result.BestCorrelation) {
				result.BestCorrelation = correlation
				result.BestLag = int64(lag)
			}
		}
	}

	return result
}

// calculateCorrelationConfidence 计算相关性置信度
func (ca *CorrelationAnalyzer) calculateCorrelationConfidence(correlation float64, sampleSize int) float64 {
	if sampleSize < 3 {
		return 0.0
	}
	
	// 基于样本大小和相关性强度计算置信度
	if sampleSize > 30 && math.Abs(correlation) > 0.7 {
		return 0.95
	} else if sampleSize > 10 && math.Abs(correlation) > 0.5 {
		return 0.80
	} else if math.Abs(correlation) > 0.3 {
		return 0.60
	}
	
	return 0.40
}

// DataQualityAnalyzer 数据质量分析器
type DataQualityAnalyzer struct {
	config *DataQualityConfig
}

// DataQualityConfig 数据质量分析配置
type DataQualityConfig struct {
	OutlierThreshold     float64 `json:"outlierThreshold"`     // 异常值阈值
	MissingDataThreshold float64 `json:"missingDataThreshold"` // 缺失数据阈值
	VariabilityThreshold float64 `json:"variabilityThreshold"` // 变异性阈值
	MinDataPoints        int     `json:"minDataPoints"`        // 最少数据点
}

// NewDataQualityAnalyzer 创建数据质量分析器
func NewDataQualityAnalyzer() *DataQualityAnalyzer {
	return &DataQualityAnalyzer{
		config: &DataQualityConfig{
			OutlierThreshold:     3.0,
			MissingDataThreshold: 0.1,
			VariabilityThreshold: 0.05,
			MinDataPoints:        3,
		},
	}
}

// AnalyzeQuality 分析数据质量
func (dqa *DataQualityAnalyzer) AnalyzeQuality(
	ctx *ctx.Context,
	dataPoints []*models.DataPoint,
) *models.DataQualityInfo {
	qualityInfo := &models.DataQualityInfo{
		TotalPoints:   len(dataPoints),
		ValidPoints:   0,
		AnomalyPoints: 0,
		Completeness:  0.0,
		Accuracy:      0.0,
		Timeliness:    1.0,
		QualityScore:  0.0,
	}

	if len(dataPoints) == 0 {
		return qualityInfo
	}

	// 提取有效数据
	validValues := make([]float64, 0, len(dataPoints))
	nullCount := 0
	anomalyCount := 0

	for _, point := range dataPoints {
		if point == nil {
			nullCount++
			continue
		}

		if math.IsNaN(point.Value) || math.IsInf(point.Value, 0) {
			nullCount++
			continue
		}

		validValues = append(validValues, point.Value)

		// 检查异常标记
		if point.Quality != nil && point.Quality.Anomaly {
			anomalyCount++
		}
	}

	qualityInfo.ValidPoints = len(validValues)
	qualityInfo.AnomalyPoints = anomalyCount

	// 计算完整性
	qualityInfo.Completeness = float64(len(validValues)) / float64(len(dataPoints))

	// 计算准确性
	if len(validValues) > 0 {
		outlierCount := dqa.detectOutliers(validValues)
		totalAnomalies := anomalyCount + outlierCount
		qualityInfo.Accuracy = math.Max(0.0, 1.0-float64(totalAnomalies)/float64(len(validValues)))
	} else {
		qualityInfo.Accuracy = 0.0
	}

	// 计算整体质量分数
	qualityInfo.QualityScore = dqa.calculateOverallScore(qualityInfo)

	return qualityInfo
}

// detectOutliers 检测异常值
func (dqa *DataQualityAnalyzer) detectOutliers(values []float64) int {
	if len(values) < 3 {
		return 0
	}

	// 使用四分位数方法检测异常值
	sorted := make([]float64, len(values))
	copy(sorted, values)
	sort.Float64s(sorted)

	n := len(sorted)
	q1Index := n / 4
	q3Index := 3 * n / 4

	q1 := sorted[q1Index]
	q3 := sorted[q3Index]
	iqr := q3 - q1

	lowerBound := q1 - 1.5*iqr
	upperBound := q3 + 1.5*iqr

	outlierCount := 0
	for _, v := range values {
		if v < lowerBound || v > upperBound {
			outlierCount++
		}
	}

	return outlierCount
}

// calculateOverallScore 计算整体质量分数
func (dqa *DataQualityAnalyzer) calculateOverallScore(qualityInfo *models.DataQualityInfo) float64 {
	// 加权平均：完整性(40%) + 准确性(40%) + 时效性(20%)
	score := 0.4*qualityInfo.Completeness + 0.4*qualityInfo.Accuracy + 0.2*qualityInfo.Timeliness

	// 数据点数量的影响
	if qualityInfo.TotalPoints < dqa.config.MinDataPoints {
		score *= 0.5
	}

	return math.Max(0.0, math.Min(1.0, score))
}