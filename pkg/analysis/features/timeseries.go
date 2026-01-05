package features

import (
	"alertHub/internal/models"
	"alertHub/pkg/analysis/utils"
	"math"
	"sort"

	"github.com/montanaflynn/stats"
	"gonum.org/v1/gonum/stat"
)

// TimeSeriesFeatures 时序特征
type TimeSeriesFeatures struct {
	// 趋势特征
	TrendType     string  `json:"trendType"`     // "increasing", "decreasing", "stable", "volatile"
	TrendStrength float64 `json:"trendStrength"` // 趋势强度 [0,1]
	TrendSlope    float64 `json:"trendSlope"`    // 线性趋势斜率
	TrendR2       float64 `json:"trendR2"`       // 趋势拟合优度

	// 周期性特征
	Seasonality         bool    `json:"seasonality"`         // 是否存在季节性
	SeasonalityStrength float64 `json:"seasonalityStrength"` // 季节性强度
	DominantPeriod      int     `json:"dominantPeriod"`      // 主要周期长度
	SeasonalAmplitude   float64 `json:"seasonalAmplitude"`   // 季节性幅度

	// 平稳性特征
	Stationarity       bool    `json:"stationarity"`       // 是否平稳
	StationarityPValue float64 `json:"stationarityPValue"` // ADF检验p值
	DifferenceOrder    int     `json:"differenceOrder"`    // 达到平稳需要的差分阶数

	// 自相关特征
	Autocorrelation  []float64 `json:"autocorrelation"`  // 自相关函数
	PartialAutocorr  []float64 `json:"partialAutocorr"`  // 偏自相关函数
	LagOfMaxAutocorr int       `json:"lagOfMaxAutocorr"` // 最大自相关的滞后期
	MaxAutocorrValue float64   `json:"maxAutocorrValue"` // 最大自相关值

	// 变异性特征
	Volatility       float64 `json:"volatility"`       // 波动率
	LocalVariability float64 `json:"localVariability"` // 局部变异性
	ChangePoints     []int   `json:"changePoints"`     // 变点位置
	ChangePointCount int     `json:"changePointCount"` // 变点数量

	// 分布演化特征
	MeanShift             bool    `json:"meanShift"`             // 均值漂移
	VarianceChange        bool    `json:"varianceChange"`        // 方差变化
	DistributionStability float64 `json:"distributionStability"` // 分布稳定性

	// 频率域特征
	SpectralEntropy float64   `json:"spectralEntropy"` // 谱熵
	PeakFrequencies []float64 `json:"peakFrequencies"` // 峰值频率
	BandwidthRatio  float64   `json:"bandwidthRatio"`  // 带宽比
}

// TimeSeriesAnalyzer 时序分析器
type TimeSeriesAnalyzer struct{}

// NewTimeSeriesAnalyzer 创建时序分析器
func NewTimeSeriesAnalyzer() *TimeSeriesAnalyzer {
	return &TimeSeriesAnalyzer{}
}

// ExtractFeatures 提取时序特征
func (tsa *TimeSeriesAnalyzer) ExtractFeatures(dataPoints []*models.DataPoint) (*TimeSeriesFeatures, error) {
	if len(dataPoints) < 3 {
		return &TimeSeriesFeatures{}, nil
	}

	// 按时间戳排序
	sortedPoints := make([]*models.DataPoint, len(dataPoints))
	copy(sortedPoints, dataPoints)
	sort.Slice(sortedPoints, func(i, j int) bool {
		return sortedPoints[i].Timestamp < sortedPoints[j].Timestamp
	})

	values := make([]float64, len(sortedPoints))
	timestamps := make([]int64, len(sortedPoints))

	for i, point := range sortedPoints {
		values[i] = point.Value
		timestamps[i] = point.Timestamp
	}

	features := &TimeSeriesFeatures{}

	// 提取各种时序特征
	tsa.extractTrendFeatures(values, timestamps, features)
	tsa.extractSeasonalityFeatures(values, features)
	tsa.extractStationarityFeatures(values, features)
	tsa.extractAutocorrelationFeatures(values, features)
	tsa.extractVariabilityFeatures(values, features)
	tsa.extractChangePointFeatures(values, features)
	tsa.extractDistributionFeatures(values, features)

	return features, nil
}

// extractTrendFeatures 提取趋势特征
func (tsa *TimeSeriesAnalyzer) extractTrendFeatures(values []float64, timestamps []int64, features *TimeSeriesFeatures) {
	n := len(values)
	if n < 2 {
		return
	}

	// 计算线性趋势
	x := make([]float64, n)
	for i := range x {
		x[i] = float64(i)
	}

	// 使用最小二乘法拟合线性趋势
	alpha, beta := stat.LinearRegression(x, values, nil, false)
	features.TrendSlope = beta

	// 计算R²
	yMean, _ := stats.Mean(values)
	ss_tot := 0.0
	ss_res := 0.0

	for i, v := range values {
		predicted := alpha + beta*x[i]
		ss_tot += (v - yMean) * (v - yMean)
		ss_res += (v - predicted) * (v - predicted)
	}

	if ss_tot != 0 {
		features.TrendR2 = 1 - (ss_res / ss_tot)
	}

	// 判断趋势类型
	features.TrendStrength = math.Abs(beta) / (math.Abs(yMean) + 1) // 标准化趋势强度

	switch {
	case math.Abs(beta) < 0.01:
		features.TrendType = "stable"
	case beta > 0:
		features.TrendType = "increasing"
	default:
		features.TrendType = "decreasing"
	}

	// 检查波动性
	stdDev, _ := stats.StandardDeviation(values)
	if stdDev > math.Abs(yMean)*0.5 {
		features.TrendType = "volatile"
	}
}

// extractSeasonalityFeatures 提取季节性特征
func (tsa *TimeSeriesAnalyzer) extractSeasonalityFeatures(values []float64, features *TimeSeriesFeatures) {
	n := len(values)
	if n < 6 {
		return
	}

	// 简化的季节性检测 - 基于自相关
	maxLag := utils.GlobalMathUtils.Min(n/3, 50)
	autocorrs := make([]float64, maxLag)

	for lag := 1; lag < maxLag; lag++ {
		autocorrs[lag] = utils.GlobalMathUtils.CalculateAutocorrelation(values, lag)
	}

	// 查找周期性峰值
	maxAutocorr := 0.0
	dominantPeriod := 0

	for i := 2; i < len(autocorrs)-1; i++ {
		if autocorrs[i] > autocorrs[i-1] && autocorrs[i] > autocorrs[i+1] && autocorrs[i] > 0.3 {
			if autocorrs[i] > maxAutocorr {
				maxAutocorr = autocorrs[i]
				dominantPeriod = i
			}
		}
	}

	features.Seasonality = maxAutocorr > 0.3
	features.SeasonalityStrength = maxAutocorr
	features.DominantPeriod = dominantPeriod

	if features.Seasonality {
		// 计算季节性幅度
		if dominantPeriod > 0 && dominantPeriod < n/2 {
			seasonalValues := make([]float64, 0)
			for i := 0; i < n-dominantPeriod; i++ {
				seasonalValues = append(seasonalValues, math.Abs(values[i]-values[i+dominantPeriod]))
			}
			features.SeasonalAmplitude, _ = stats.Mean(seasonalValues)
		}
	}
}

// extractStationarityFeatures 提取平稳性特征
func (tsa *TimeSeriesAnalyzer) extractStationarityFeatures(values []float64, features *TimeSeriesFeatures) {
	// 简化的平稳性检测
	n := len(values)
	if n < 10 {
		return
	}

	// 检查均值稳定性
	half := n / 2
	firstHalf := values[:half]
	secondHalf := values[half:]

	mean1, _ := stats.Mean(firstHalf)
	mean2, _ := stats.Mean(secondHalf)
	var1, _ := stats.Variance(firstHalf)
	var2, _ := stats.Variance(secondHalf)

	meanDiff := math.Abs(mean1 - mean2)
	varDiff := math.Abs(var1 - var2)

	meanStability := meanDiff / (math.Abs(mean1) + math.Abs(mean2) + 1)
	varStability := varDiff / (var1 + var2 + 1)

	// 简单的平稳性判断
	features.Stationarity = meanStability < 0.1 && varStability < 0.1
	features.StationarityPValue = meanStability + varStability // 简化的p值替代

	// 差分阶数估计
	diffOrder := 0
	tempValues := make([]float64, len(values))
	copy(tempValues, values)

	for diffOrder < 3 {
		if isStationary(tempValues) {
			break
		}
		tempValues = calculateDifference(tempValues)
		diffOrder++
		if len(tempValues) < 5 {
			break
		}
	}

	features.DifferenceOrder = diffOrder
}

// extractAutocorrelationFeatures 提取自相关特征
func (tsa *TimeSeriesAnalyzer) extractAutocorrelationFeatures(values []float64, features *TimeSeriesFeatures) {
	n := len(values)
	maxLag := utils.GlobalMathUtils.Min(n/3, 20)

	autocorrs := make([]float64, maxLag)
	maxAutocorr := 0.0
	lagOfMax := 0

	for lag := 1; lag < maxLag; lag++ {
		autocorr := utils.GlobalMathUtils.CalculateAutocorrelation(values, lag)
		autocorrs[lag] = autocorr

		if math.Abs(autocorr) > math.Abs(maxAutocorr) {
			maxAutocorr = autocorr
			lagOfMax = lag
		}
	}

	features.Autocorrelation = autocorrs
	features.MaxAutocorrValue = maxAutocorr
	features.LagOfMaxAutocorr = lagOfMax

	// 简化的偏自相关计算
	features.PartialAutocorr = calculatePartialAutocorrelation(values, maxLag)
}

// extractVariabilityFeatures 提取变异性特征
func (tsa *TimeSeriesAnalyzer) extractVariabilityFeatures(values []float64, features *TimeSeriesFeatures) {
	n := len(values)
	if n < 2 {
		return
	}

	// 计算波动率 (相邻点变化率的标准差)
	changes := make([]float64, n-1)
	for i := 1; i < n; i++ {
		if values[i-1] != 0 {
			changes[i-1] = (values[i] - values[i-1]) / math.Abs(values[i-1])
		} else {
			changes[i-1] = values[i] - values[i-1]
		}
	}

	volatility, _ := stats.StandardDeviation(changes)
	features.Volatility = volatility

	// 计算局部变异性
	windowSize := utils.GlobalMathUtils.Min(10, n/3)
	if windowSize >= 3 {
		localVars := make([]float64, 0)
		for i := 0; i <= n-windowSize; i++ {
			window := values[i : i+windowSize]
			localVar, _ := stats.Variance(window)
			localVars = append(localVars, localVar)
		}
		features.LocalVariability, _ = stats.Mean(localVars)
	}
}

// extractChangePointFeatures 提取变点特征
func (tsa *TimeSeriesAnalyzer) extractChangePointFeatures(values []float64, features *TimeSeriesFeatures) {
	// 简化的变点检测
	changePoints := make([]int, 0)
	n := len(values)

	if n < 6 {
		features.ChangePoints = changePoints
		features.ChangePointCount = 0
		return
	}

	windowSize := utils.GlobalMathUtils.Max(3, n/10)

	for i := windowSize; i < n-windowSize; i++ {
		before := values[i-windowSize : i]
		after := values[i : i+windowSize]

		meanBefore, _ := stats.Mean(before)
		meanAfter, _ := stats.Mean(after)

		// 简单的变点检测：均值显著变化
		if math.Abs(meanAfter-meanBefore) > math.Abs(meanBefore)*0.3 {
			changePoints = append(changePoints, i)
		}
	}

	features.ChangePoints = changePoints
	features.ChangePointCount = len(changePoints)
}

// extractDistributionFeatures 提取分布演化特征
func (tsa *TimeSeriesAnalyzer) extractDistributionFeatures(values []float64, features *TimeSeriesFeatures) {
	n := len(values)
	if n < 10 {
		return
	}

	// 检查均值漂移
	segments := 3
	segmentSize := n / segments

	segmentMeans := make([]float64, segments)
	segmentVars := make([]float64, segments)

	for i := 0; i < segments; i++ {
		start := i * segmentSize
		end := start + segmentSize
		if i == segments-1 {
			end = n
		}

		segment := values[start:end]
		mean, _ := stats.Mean(segment)
		variance, _ := stats.Variance(segment)

		segmentMeans[i] = mean
		segmentVars[i] = variance
	}

	// 检查均值是否有显著变化
	meanRange := math.Abs(segmentMeans[segments-1] - segmentMeans[0])
	meanLevel := math.Abs(segmentMeans[0])
	features.MeanShift = meanRange > meanLevel*0.2

	// 检查方差变化
	varRange := math.Abs(segmentVars[segments-1] - segmentVars[0])
	varLevel := segmentVars[0]
	features.VarianceChange = varRange > varLevel*0.5

	// 计算分布稳定性
	meanStdDev, _ := stats.StandardDeviation(segmentMeans)
	varStdDev, _ := stats.StandardDeviation(segmentVars)
	features.DistributionStability = 1.0 / (1.0 + meanStdDev + varStdDev)
}

// 辅助函数

func calculatePartialAutocorrelation(values []float64, maxLag int) []float64 {
	// 简化的偏自相关计算
	partialAcf := make([]float64, maxLag)

	// 第一阶偏自相关等于自相关
	if maxLag > 0 {
		partialAcf[0] = utils.GlobalMathUtils.CalculateAutocorrelation(values, 1)
	}

	// 后续阶数使用Durbin-Levinson递归（简化版）
	for lag := 2; lag < maxLag; lag++ {
		partialAcf[lag-1] = utils.GlobalMathUtils.CalculateAutocorrelation(values, lag)
		// 这里应该实现完整的偏自相关计算，为简化起见使用自相关近似
	}

	return partialAcf
}

func calculateDifference(values []float64) []float64 {
	if len(values) < 2 {
		return []float64{}
	}

	diff := make([]float64, len(values)-1)
	for i := 1; i < len(values); i++ {
		diff[i-1] = values[i] - values[i-1]
	}
	return diff
}

func isStationary(values []float64) bool {
	// 简化的平稳性检验
	n := len(values)
	if n < 6 {
		return true
	}

	// 检查方差稳定性
	mid := n / 2
	first := values[:mid]
	second := values[mid:]

	var1, _ := stats.Variance(first)
	var2, _ := stats.Variance(second)

	return math.Abs(var1-var2) < (var1+var2)*0.3
}
