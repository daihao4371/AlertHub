package anomaly

import (
	"alertHub/internal/models"
	"math"
	"sort"

	"github.com/montanaflynn/stats"
)

// AnomalyPoint 异常点信息
type AnomalyPoint struct {
	Index      int                    `json:"index"`      // 在时间序列中的位置
	Timestamp  int64                  `json:"timestamp"`  // 时间戳
	Value      float64                `json:"value"`      // 异常值
	Expected   float64                `json:"expected"`   // 期望值
	Deviation  float64                `json:"deviation"`  // 偏差程度
	Severity   string                 `json:"severity"`   // 严重程度: low/medium/high/critical
	Method     string                 `json:"method"`     // 检测方法
	Confidence float64                `json:"confidence"` // 置信度 [0,1]
	Metadata   map[string]interface{} `json:"metadata"`   // 额外元数据
}

// AnomalyDetectionResult 异常检测结果
type AnomalyDetectionResult struct {
	Method          string          `json:"method"`          // 检测方法名称
	AnomalyPoints   []*AnomalyPoint `json:"anomalyPoints"`   // 检测到的异常点
	AnomalyCount    int             `json:"anomalyCount"`    // 异常点数量
	AnomalyRate     float64         `json:"anomalyRate"`     // 异常率
	OverallSeverity string          `json:"overallSeverity"` // 整体严重程度
	ConfidenceScore float64         `json:"confidenceScore"` // 整体置信度
	ProcessingTime  int64           `json:"processingTime"`  // 处理时间(毫秒)
}

// CombinedAnomalyResult 多算法组合结果
type CombinedAnomalyResult struct {
	Methods          []string                  `json:"methods"`          // 使用的方法
	Results          []*AnomalyDetectionResult `json:"results"`          // 各方法结果
	ConsensusPoints  []*AnomalyPoint           `json:"consensusPoints"`  // 多方法一致的异常点
	FinalAnomalies   []*AnomalyPoint           `json:"finalAnomalies"`   // 最终异常点列表
	AggregatedScore  float64                   `json:"aggregatedScore"`  // 聚合评分
	ReliabilityScore float64                   `json:"reliabilityScore"` // 可靠性评分
}

// AnomalyDetector 异常检测器接口
type AnomalyDetector interface {
	Name() string
	DetectAnomalies(dataPoints []*models.DataPoint) (*AnomalyDetectionResult, error)
	GetConfidence() float64
	GetDescription() string
}

// MultiMethodDetector 多算法异常检测器
type MultiMethodDetector struct {
	Detectors       []AnomalyDetector `json:"-"`
	ConsensusWeight float64           `json:"consensusWeight"` // 一致性权重
	MinConsensus    int               `json:"minConsensus"`    // 最小一致数
}

// NewMultiMethodDetector 创建多算法检测器
func NewMultiMethodDetector() *MultiMethodDetector {
	return &MultiMethodDetector{
		Detectors: []AnomalyDetector{
			NewStatisticalDetector(),
			NewIQRDetector(),
			NewZScoreDetector(),
			NewMADDetector(),
			NewIsolationForestDetector(),
		},
		ConsensusWeight: 0.7,
		MinConsensus:    2,
	}
}

// DetectAnomalies 使用多种算法检测异常
func (mmd *MultiMethodDetector) DetectAnomalies(dataPoints []*models.DataPoint) (*CombinedAnomalyResult, error) {
	if len(dataPoints) < 3 {
		return &CombinedAnomalyResult{}, nil
	}

	result := &CombinedAnomalyResult{
		Methods: make([]string, 0),
		Results: make([]*AnomalyDetectionResult, 0),
	}

	// 运行所有检测器
	for _, detector := range mmd.Detectors {
		detectionResult, err := detector.DetectAnomalies(dataPoints)
		if err != nil {
			continue
		}

		result.Methods = append(result.Methods, detector.Name())
		result.Results = append(result.Results, detectionResult)
	}

	// 计算一致性异常点
	result.ConsensusPoints = mmd.findConsensusAnomalies(result.Results)

	// 生成最终异常列表
	result.FinalAnomalies = mmd.generateFinalAnomalies(result.Results, result.ConsensusPoints)

	// 计算聚合评分
	result.AggregatedScore = mmd.calculateAggregatedScore(result.Results)
	result.ReliabilityScore = mmd.calculateReliabilityScore(result.Results, result.ConsensusPoints)

	return result, nil
}

// findConsensusAnomalies 查找多个方法一致认为的异常点
func (mmd *MultiMethodDetector) findConsensusAnomalies(results []*AnomalyDetectionResult) []*AnomalyPoint {
	if len(results) == 0 {
		return []*AnomalyPoint{}
	}

	// 统计每个时间点被标记为异常的次数
	anomalyCounter := make(map[int64]map[int]*AnomalyPoint) // timestamp -> index -> AnomalyPoint

	for _, result := range results {
		for _, anomaly := range result.AnomalyPoints {
			if _, exists := anomalyCounter[anomaly.Timestamp]; !exists {
				anomalyCounter[anomaly.Timestamp] = make(map[int]*AnomalyPoint)
			}
			anomalyCounter[anomaly.Timestamp][anomaly.Index] = anomaly
		}
	}

	// 找出达到最小一致数的异常点
	consensusPoints := make([]*AnomalyPoint, 0)
	for timestamp, indexMap := range anomalyCounter {
		for index, anomaly := range indexMap {
			count := 0
			totalConfidence := 0.0
			totalSeverity := 0.0

			// 计算该点被多少种方法标记为异常
			for _, result := range results {
				for _, resultAnomaly := range result.AnomalyPoints {
					if resultAnomaly.Timestamp == timestamp && resultAnomaly.Index == index {
						count++
						totalConfidence += resultAnomaly.Confidence
						totalSeverity += mmd.severityToScore(resultAnomaly.Severity)
						break
					}
				}
			}

			if count >= mmd.MinConsensus {
				consensusAnomaly := &AnomalyPoint{
					Index:      anomaly.Index,
					Timestamp:  anomaly.Timestamp,
					Value:      anomaly.Value,
					Expected:   anomaly.Expected,
					Deviation:  anomaly.Deviation,
					Severity:   mmd.scoreToSeverity(totalSeverity / float64(count)),
					Method:     "consensus",
					Confidence: totalConfidence / float64(count),
					Metadata: map[string]interface{}{
						"consensusCount": count,
						"totalMethods":   len(results),
					},
				}
				consensusPoints = append(consensusPoints, consensusAnomaly)
			}
		}
	}

	return consensusPoints
}

// generateFinalAnomalies 生成最终异常列表
func (mmd *MultiMethodDetector) generateFinalAnomalies(
	results []*AnomalyDetectionResult,
	consensusPoints []*AnomalyPoint,
) []*AnomalyPoint {
	finalAnomalies := make([]*AnomalyPoint, 0)

	// 首先添加一致性异常点（高权重）
	for _, anomaly := range consensusPoints {
		finalAnomaly := *anomaly
		finalAnomaly.Confidence = anomaly.Confidence * mmd.ConsensusWeight
		finalAnomalies = append(finalAnomalies, &finalAnomaly)
	}

	// 添加其他高置信度异常点
	addedTimestamps := make(map[int64]bool)
	for _, anomaly := range consensusPoints {
		addedTimestamps[anomaly.Timestamp] = true
	}

	for _, result := range results {
		for _, anomaly := range result.AnomalyPoints {
			if !addedTimestamps[anomaly.Timestamp] && anomaly.Confidence > 0.8 {
				finalAnomaly := *anomaly
				finalAnomaly.Confidence = anomaly.Confidence * (1 - mmd.ConsensusWeight)
				finalAnomalies = append(finalAnomalies, &finalAnomaly)
				addedTimestamps[anomaly.Timestamp] = true
			}
		}
	}

	// 按置信度排序
	sort.Slice(finalAnomalies, func(i, j int) bool {
		return finalAnomalies[i].Confidence > finalAnomalies[j].Confidence
	})

	return finalAnomalies
}

// calculateAggregatedScore 计算聚合评分
func (mmd *MultiMethodDetector) calculateAggregatedScore(results []*AnomalyDetectionResult) float64 {
	if len(results) == 0 {
		return 0.0
	}

	totalScore := 0.0
	totalWeight := 0.0

	for _, result := range results {
		weight := result.ConfidenceScore
		score := 1.0 - result.AnomalyRate // 异常率越低，分数越高

		totalScore += score * weight
		totalWeight += weight
	}

	if totalWeight == 0 {
		return 0.0
	}

	return totalScore / totalWeight
}

// calculateReliabilityScore 计算可靠性评分
func (mmd *MultiMethodDetector) calculateReliabilityScore(
	results []*AnomalyDetectionResult,
	consensusPoints []*AnomalyPoint,
) float64 {
	if len(results) == 0 {
		return 0.0
	}

	// 计算方法间一致性
	totalAnomalies := 0
	for _, result := range results {
		totalAnomalies += len(result.AnomalyPoints)
	}

	if totalAnomalies == 0 {
		return 1.0 // 所有方法都认为没有异常，一致性很高
	}

	consensusRatio := float64(len(consensusPoints)*len(results)) / float64(totalAnomalies)

	// 计算置信度一致性
	confidenceVariance := 0.0
	meanConfidence := 0.0
	confidenceCount := 0

	for _, result := range results {
		meanConfidence += result.ConfidenceScore
		confidenceCount++
	}
	meanConfidence /= float64(confidenceCount)

	for _, result := range results {
		diff := result.ConfidenceScore - meanConfidence
		confidenceVariance += diff * diff
	}
	confidenceVariance /= float64(confidenceCount)

	confidenceConsistency := 1.0 / (1.0 + confidenceVariance)

	return (consensusRatio + confidenceConsistency) / 2.0
}

// severityToScore 严重程度转换为数值
func (mmd *MultiMethodDetector) severityToScore(severity string) float64 {
	switch severity {
	case "low":
		return 1.0
	case "medium":
		return 2.0
	case "high":
		return 3.0
	case "critical":
		return 4.0
	default:
		return 1.0
	}
}

// scoreToSeverity 数值转换为严重程度
func (mmd *MultiMethodDetector) scoreToSeverity(score float64) string {
	switch {
	case score >= 3.5:
		return "critical"
	case score >= 2.5:
		return "high"
	case score >= 1.5:
		return "medium"
	default:
		return "low"
	}
}

// --- 具体检测算法实现 ---

// StatisticalDetector 统计学异常检测器
type StatisticalDetector struct{}

func NewStatisticalDetector() *StatisticalDetector {
	return &StatisticalDetector{}
}

func (sd *StatisticalDetector) Name() string {
	return "statistical"
}

func (sd *StatisticalDetector) GetConfidence() float64 {
	return 0.8
}

func (sd *StatisticalDetector) GetDescription() string {
	return "基于3-sigma规则的统计异常检测"
}

func (sd *StatisticalDetector) DetectAnomalies(dataPoints []*models.DataPoint) (*AnomalyDetectionResult, error) {
	values := extractValues(dataPoints)
	if len(values) < 3 {
		return &AnomalyDetectionResult{Method: sd.Name()}, nil
	}

	mean, _ := stats.Mean(values)
	stdDev, _ := stats.StandardDeviation(values)
	threshold := 3.0 * stdDev

	anomalies := make([]*AnomalyPoint, 0)

	for i, point := range dataPoints {
		deviation := math.Abs(point.Value - mean)
		if deviation > threshold {
			var severity string
			switch {
			case deviation > 4*stdDev:
				severity = "critical"
			case deviation > 3.5*stdDev:
				severity = "high"
			default:
				severity = "medium"
			}

			anomaly := &AnomalyPoint{
				Index:      i,
				Timestamp:  point.Timestamp,
				Value:      point.Value,
				Expected:   mean,
				Deviation:  deviation,
				Severity:   severity,
				Method:     sd.Name(),
				Confidence: math.Min(0.95, deviation/(4*stdDev)),
			}
			anomalies = append(anomalies, anomaly)
		}
	}

	return &AnomalyDetectionResult{
		Method:          sd.Name(),
		AnomalyPoints:   anomalies,
		AnomalyCount:    len(anomalies),
		AnomalyRate:     float64(len(anomalies)) / float64(len(dataPoints)),
		OverallSeverity: calculateOverallSeverity(anomalies),
		ConfidenceScore: sd.GetConfidence(),
	}, nil
}

// IQRDetector IQR异常检测器
type IQRDetector struct{}

func NewIQRDetector() *IQRDetector {
	return &IQRDetector{}
}

func (iqr *IQRDetector) Name() string {
	return "iqr"
}

func (iqr *IQRDetector) GetConfidence() float64 {
	return 0.85
}

func (iqr *IQRDetector) GetDescription() string {
	return "基于四分位距(IQR)的异常检测"
}

func (iqr *IQRDetector) DetectAnomalies(dataPoints []*models.DataPoint) (*AnomalyDetectionResult, error) {
	values := extractValues(dataPoints)
	if len(values) < 5 {
		return &AnomalyDetectionResult{Method: iqr.Name()}, nil
	}

	q25, _ := stats.Percentile(values, 25)
	q75, _ := stats.Percentile(values, 75)
	iqrValue := q75 - q25

	lowerBound := q25 - 1.5*iqrValue
	upperBound := q75 + 1.5*iqrValue
	extremeLowerBound := q25 - 3*iqrValue
	extremeUpperBound := q75 + 3*iqrValue

	anomalies := make([]*AnomalyPoint, 0)

	for i, point := range dataPoints {
		if point.Value < lowerBound || point.Value > upperBound {
			severity := "medium"
			expected := q25
			if point.Value > upperBound {
				expected = q75
			}

			if point.Value < extremeLowerBound || point.Value > extremeUpperBound {
				severity = "critical"
			} else if point.Value < q25-2*iqrValue || point.Value > q75+2*iqrValue {
				severity = "high"
			}

			deviation := math.Min(math.Abs(point.Value-lowerBound), math.Abs(point.Value-upperBound))
			confidence := math.Min(0.95, deviation/iqrValue)

			anomaly := &AnomalyPoint{
				Index:      i,
				Timestamp:  point.Timestamp,
				Value:      point.Value,
				Expected:   expected,
				Deviation:  deviation,
				Severity:   severity,
				Method:     iqr.Name(),
				Confidence: confidence,
			}
			anomalies = append(anomalies, anomaly)
		}
	}

	return &AnomalyDetectionResult{
		Method:          iqr.Name(),
		AnomalyPoints:   anomalies,
		AnomalyCount:    len(anomalies),
		AnomalyRate:     float64(len(anomalies)) / float64(len(dataPoints)),
		OverallSeverity: calculateOverallSeverity(anomalies),
		ConfidenceScore: iqr.GetConfidence(),
	}, nil
}

// ZScoreDetector Z-Score异常检测器
type ZScoreDetector struct{}

func NewZScoreDetector() *ZScoreDetector {
	return &ZScoreDetector{}
}

func (zs *ZScoreDetector) Name() string {
	return "zscore"
}

func (zs *ZScoreDetector) GetConfidence() float64 {
	return 0.75
}

func (zs *ZScoreDetector) GetDescription() string {
	return "基于Z-Score的异常检测"
}

func (zs *ZScoreDetector) DetectAnomalies(dataPoints []*models.DataPoint) (*AnomalyDetectionResult, error) {
	values := extractValues(dataPoints)
	if len(values) < 3 {
		return &AnomalyDetectionResult{Method: zs.Name()}, nil
	}

	mean, _ := stats.Mean(values)
	stdDev, _ := stats.StandardDeviation(values)

	if stdDev == 0 {
		return &AnomalyDetectionResult{Method: zs.Name()}, nil
	}

	anomalies := make([]*AnomalyPoint, 0)

	for i, point := range dataPoints {
		zScore := math.Abs(point.Value-mean) / stdDev

		if zScore > 2.0 {
			var severity string
			switch {
			case zScore > 4.0:
				severity = "critical"
			case zScore > 3.0:
				severity = "high"
			case zScore > 2.5:
				severity = "medium"
			default:
				severity = "low"
			}

			anomaly := &AnomalyPoint{
				Index:      i,
				Timestamp:  point.Timestamp,
				Value:      point.Value,
				Expected:   mean,
				Deviation:  math.Abs(point.Value - mean),
				Severity:   severity,
				Method:     zs.Name(),
				Confidence: math.Min(0.95, zScore/4.0),
				Metadata:   map[string]interface{}{"zscore": zScore},
			}
			anomalies = append(anomalies, anomaly)
		}
	}

	return &AnomalyDetectionResult{
		Method:          zs.Name(),
		AnomalyPoints:   anomalies,
		AnomalyCount:    len(anomalies),
		AnomalyRate:     float64(len(anomalies)) / float64(len(dataPoints)),
		OverallSeverity: calculateOverallSeverity(anomalies),
		ConfidenceScore: zs.GetConfidence(),
	}, nil
}

// MADDetector 基于MAD的异常检测器
type MADDetector struct{}

func NewMADDetector() *MADDetector {
	return &MADDetector{}
}

func (mad *MADDetector) Name() string {
	return "mad"
}

func (mad *MADDetector) GetConfidence() float64 {
	return 0.9
}

func (mad *MADDetector) GetDescription() string {
	return "基于中位数绝对偏差(MAD)的稳健异常检测"
}

func (mad *MADDetector) DetectAnomalies(dataPoints []*models.DataPoint) (*AnomalyDetectionResult, error) {
	values := extractValues(dataPoints)
	if len(values) < 3 {
		return &AnomalyDetectionResult{Method: mad.Name()}, nil
	}

	median, _ := stats.Median(values)

	// 计算绝对偏差
	deviations := make([]float64, len(values))
	for i, v := range values {
		deviations[i] = math.Abs(v - median)
	}

	medianDeviation, _ := stats.Median(deviations)

	// MAD常数，使其在正态分布下与标准差一致
	madConstant := 1.4826
	threshold := 3.0 * madConstant * medianDeviation

	anomalies := make([]*AnomalyPoint, 0)

	for i, point := range dataPoints {
		deviation := math.Abs(point.Value - median)

		if deviation > threshold {
			modifiedZScore := deviation / (madConstant * medianDeviation)
			var severity string
			switch {
			case modifiedZScore > 5.0:
				severity = "critical"
			case modifiedZScore > 4.0:
				severity = "high"
			default:
				severity = "medium"
			}

			anomaly := &AnomalyPoint{
				Index:      i,
				Timestamp:  point.Timestamp,
				Value:      point.Value,
				Expected:   median,
				Deviation:  deviation,
				Severity:   severity,
				Method:     mad.Name(),
				Confidence: math.Min(0.95, modifiedZScore/5.0),
				Metadata:   map[string]interface{}{"modifiedZScore": modifiedZScore},
			}
			anomalies = append(anomalies, anomaly)
		}
	}

	return &AnomalyDetectionResult{
		Method:          mad.Name(),
		AnomalyPoints:   anomalies,
		AnomalyCount:    len(anomalies),
		AnomalyRate:     float64(len(anomalies)) / float64(len(dataPoints)),
		OverallSeverity: calculateOverallSeverity(anomalies),
		ConfidenceScore: mad.GetConfidence(),
	}, nil
}

// IsolationForestDetector 简化的孤立森林检测器
type IsolationForestDetector struct{}

func NewIsolationForestDetector() *IsolationForestDetector {
	return &IsolationForestDetector{}
}

func (ifd *IsolationForestDetector) Name() string {
	return "isolation_forest"
}

func (ifd *IsolationForestDetector) GetConfidence() float64 {
	return 0.85
}

func (ifd *IsolationForestDetector) GetDescription() string {
	return "基于孤立森林的异常检测(简化版)"
}

func (ifd *IsolationForestDetector) DetectAnomalies(dataPoints []*models.DataPoint) (*AnomalyDetectionResult, error) {
	values := extractValues(dataPoints)
	if len(values) < 5 {
		return &AnomalyDetectionResult{Method: ifd.Name()}, nil
	}

	// 简化的孤立森林：基于局部密度
	anomalies := make([]*AnomalyPoint, 0)

	for i, point := range dataPoints {
		// 计算局部密度
		nearbyCount := 0
		radius := calculateLocalRadius(values, i, 0.1) // 10%的局部半径

		for j, otherValue := range values {
			if i != j && math.Abs(point.Value-otherValue) <= radius {
				nearbyCount++
			}
		}

		// 密度越低越可能是异常
		localDensity := float64(nearbyCount) / float64(len(values))

		if localDensity < 0.05 { // 局部密度阈值
			var severity string
			switch {
			case localDensity < 0.01:
				severity = "critical"
			case localDensity < 0.02:
				severity = "high"
			default:
				severity = "medium"
			}

			// 计算期望值（附近点的均值）
			nearbyValues := make([]float64, 0)
			for j, otherValue := range values {
				if i != j && math.Abs(point.Value-otherValue) <= radius*2 {
					nearbyValues = append(nearbyValues, otherValue)
				}
			}

			expected := point.Value
			if len(nearbyValues) > 0 {
				expected, _ = stats.Mean(nearbyValues)
			}

			anomaly := &AnomalyPoint{
				Index:      i,
				Timestamp:  point.Timestamp,
				Value:      point.Value,
				Expected:   expected,
				Deviation:  math.Abs(point.Value - expected),
				Severity:   severity,
				Method:     ifd.Name(),
				Confidence: math.Min(0.95, (0.05-localDensity)/0.05),
				Metadata:   map[string]interface{}{"localDensity": localDensity},
			}
			anomalies = append(anomalies, anomaly)
		}
	}

	return &AnomalyDetectionResult{
		Method:          ifd.Name(),
		AnomalyPoints:   anomalies,
		AnomalyCount:    len(anomalies),
		AnomalyRate:     float64(len(anomalies)) / float64(len(dataPoints)),
		OverallSeverity: calculateOverallSeverity(anomalies),
		ConfidenceScore: ifd.GetConfidence(),
	}, nil
}

// 辅助函数

func extractValues(dataPoints []*models.DataPoint) []float64 {
	values := make([]float64, len(dataPoints))
	for i, point := range dataPoints {
		values[i] = point.Value
	}
	return values
}

func calculateOverallSeverity(anomalies []*AnomalyPoint) string {
	if len(anomalies) == 0 {
		return "low"
	}

	criticalCount := 0
	highCount := 0

	for _, anomaly := range anomalies {
		switch anomaly.Severity {
		case "critical":
			criticalCount++
		case "high":
			highCount++
		}
	}

	switch {
	case criticalCount > 0:
		return "critical"
	case highCount > len(anomalies)/2:
		return "high"
	case len(anomalies) > 0:
		return "medium"
	default:
		return "low"
	}
}

func calculateLocalRadius(values []float64, index int, percentile float64) float64 {
	if len(values) < 2 {
		return 1.0
	}

	distances := make([]float64, 0, len(values)-1)
	for i, v := range values {
		if i != index {
			distances = append(distances, math.Abs(values[index]-v))
		}
	}

	sort.Float64s(distances)

	radiusIndex := int(float64(len(distances)) * percentile)
	if radiusIndex >= len(distances) {
		radiusIndex = len(distances) - 1
	}

	return distances[radiusIndex]
}
