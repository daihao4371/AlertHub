package features

import (
	"alertHub/internal/models"
	"math"
	"sort"

	"github.com/montanaflynn/stats"
)

// StatisticalFeatures 完整的统计特征
type StatisticalFeatures struct {
	// 基础统计量
	Mean     float64 `json:"mean"`     // 平均值
	Median   float64 `json:"median"`   // 中位数
	Mode     float64 `json:"mode"`     // 众数
	StdDev   float64 `json:"stdDev"`   // 标准差
	Variance float64 `json:"variance"` // 方差

	// 分布特征
	Min          float64 `json:"min"`          // 最小值
	Max          float64 `json:"max"`          // 最大值
	Range        float64 `json:"range"`        // 极差
	Q25          float64 `json:"q25"`          // 第一四分位数
	Q75          float64 `json:"q75"`          // 第三四分位数
	IQR          float64 `json:"iqr"`          // 四分位距
	Percentile90 float64 `json:"percentile90"` // 90分位数
	Percentile95 float64 `json:"percentile95"` // 95分位数
	Percentile99 float64 `json:"percentile99"` // 99分位数

	// 形状特征
	Skewness float64 `json:"skewness"` // 偏度
	Kurtosis float64 `json:"kurtosis"` // 峰度

	// 变异性指标
	CoefficientOfVariation float64 `json:"coefficientOfVariation"` // 变异系数
	MeanAbsoluteDeviation  float64 `json:"meanAbsoluteDeviation"`  // 平均绝对偏差

	// 稳健统计量
	RobustMean     float64 `json:"robustMean"`     // 稳健均值(去除异常值)
	TrimmedMean    float64 `json:"trimmedMean"`    // 修剪均值
	WinsorizedMean float64 `json:"winsorizedMean"` // Winsorized均值

	// 数据质量指标
	ZeroCount    int     `json:"zeroCount"`    // 零值个数
	NullCount    int     `json:"nullCount"`    // 空值个数
	UniqueCount  int     `json:"uniqueCount"`  // 唯一值个数
	Completeness float64 `json:"completeness"` // 完整性比例
}

// StatisticalAnalyzer 统计特征分析器
type StatisticalAnalyzer struct{}

// NewStatisticalAnalyzer 创建统计分析器
func NewStatisticalAnalyzer() *StatisticalAnalyzer {
	return &StatisticalAnalyzer{}
}

// ExtractFeatures 提取完整统计特征
func (sa *StatisticalAnalyzer) ExtractFeatures(dataPoints []*models.DataPoint) (*StatisticalFeatures, error) {
	if len(dataPoints) == 0 {
		return &StatisticalFeatures{}, nil
	}

	// 提取数值序列
	values := make([]float64, 0, len(dataPoints))
	zeroCount := 0
	nullCount := 0

	for _, point := range dataPoints {
		if point == nil {
			nullCount++
			continue
		}
		if point.Value == 0 {
			zeroCount++
		}
		values = append(values, point.Value)
	}

	if len(values) == 0 {
		return &StatisticalFeatures{
			NullCount:    nullCount,
			Completeness: 0.0,
		}, nil
	}

	features := &StatisticalFeatures{}

	// 基础统计量
	mean, _ := stats.Mean(values)
	features.Mean = mean

	median, _ := stats.Median(values)
	features.Median = median

	mode, _ := stats.Mode(values)
	if len(mode) > 0 {
		features.Mode = mode[0]
	}

	stdDev, _ := stats.StandardDeviation(values)
	features.StdDev = stdDev

	variance, _ := stats.Variance(values)
	features.Variance = variance

	// 分布特征
	min, _ := stats.Min(values)
	features.Min = min

	max, _ := stats.Max(values)
	features.Max = max

	features.Range = max - min

	q25, _ := stats.Percentile(values, 25)
	features.Q25 = q25

	q75, _ := stats.Percentile(values, 75)
	features.Q75 = q75

	features.IQR = q75 - q25

	p90, _ := stats.Percentile(values, 90)
	features.Percentile90 = p90

	p95, _ := stats.Percentile(values, 95)
	features.Percentile95 = p95

	p99, _ := stats.Percentile(values, 99)
	features.Percentile99 = p99

	// 形状特征
	features.Skewness = calculateSkewness(values, mean, stdDev)
	features.Kurtosis = calculateKurtosis(values, mean, stdDev)

	// 变异性指标
	if mean != 0 {
		features.CoefficientOfVariation = stdDev / math.Abs(mean)
	}

	features.MeanAbsoluteDeviation = calculateMAD(values, mean)

	// 稳健统计量
	features.RobustMean = calculateRobustMean(values)
	features.TrimmedMean = calculateTrimmedMean(values, 0.1)        // 去除10%的极端值
	features.WinsorizedMean = calculateWinsorizedMean(values, 0.05) // 5%的Winsorization

	// 数据质量指标
	features.ZeroCount = zeroCount
	features.NullCount = nullCount
	features.UniqueCount = calculateUniqueCount(values)
	features.Completeness = float64(len(values)) / float64(len(dataPoints))

	return features, nil
}

// calculateSkewness 计算偏度 - 数据分布的不对称程度
func calculateSkewness(values []float64, mean, stdDev float64) float64 {
	if stdDev == 0 || len(values) < 3 {
		return 0
	}

	n := float64(len(values))
	sum := 0.0

	for _, v := range values {
		normalized := (v - mean) / stdDev
		sum += math.Pow(normalized, 3)
	}

	// 使用样本偏度公式
	return (n / ((n - 1) * (n - 2))) * sum
}

// calculateKurtosis 计算峰度 - 数据分布的尖锐程度
func calculateKurtosis(values []float64, mean, stdDev float64) float64 {
	if stdDev == 0 || len(values) < 4 {
		return 0
	}

	n := float64(len(values))
	sum := 0.0

	for _, v := range values {
		normalized := (v - mean) / stdDev
		sum += math.Pow(normalized, 4)
	}

	// 使用样本峰度公式(超额峰度)
	kurtosis := ((n*(n+1))/((n-1)*(n-2)*(n-3)))*sum - (3*(n-1)*(n-1))/((n-2)*(n-3))
	return kurtosis
}

// calculateMAD 计算平均绝对偏差
func calculateMAD(values []float64, mean float64) float64 {
	if len(values) == 0 {
		return 0
	}

	sum := 0.0
	for _, v := range values {
		sum += math.Abs(v - mean)
	}

	return sum / float64(len(values))
}

// calculateRobustMean 计算稳健均值(使用MAD去除异常值)
func calculateRobustMean(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}

	median, _ := stats.Median(values)

	// 计算绝对中位差(MAD)
	deviations := make([]float64, len(values))
	for i, v := range values {
		deviations[i] = math.Abs(v - median)
	}

	mad, _ := stats.Median(deviations)
	threshold := median + 3*mad // 使用3倍MAD作为阈值

	// 过滤异常值后计算均值
	filteredValues := make([]float64, 0)
	for _, v := range values {
		if math.Abs(v-median) <= threshold {
			filteredValues = append(filteredValues, v)
		}
	}

	if len(filteredValues) == 0 {
		return median
	}

	robustMean, _ := stats.Mean(filteredValues)
	return robustMean
}

// calculateTrimmedMean 计算修剪均值
func calculateTrimmedMean(values []float64, trimPercent float64) float64 {
	if len(values) == 0 {
		return 0
	}

	// 复制并排序
	sorted := make([]float64, len(values))
	copy(sorted, values)
	sort.Float64s(sorted)

	// 计算要修剪的元素个数
	trimCount := int(float64(len(sorted)) * trimPercent)

	if trimCount*2 >= len(sorted) {
		median, _ := stats.Median(sorted)
		return median
	}

	// 去除首尾极值
	trimmed := sorted[trimCount : len(sorted)-trimCount]

	trimmedMean, _ := stats.Mean(trimmed)
	return trimmedMean
}

// calculateWinsorizedMean 计算Winsorized均值
func calculateWinsorizedMean(values []float64, winsorizePercent float64) float64 {
	if len(values) == 0 {
		return 0
	}

	// 复制并排序
	winsorized := make([]float64, len(values))
	copy(winsorized, values)
	sort.Float64s(winsorized)

	// 计算Winsorization的边界
	lowerIndex := int(float64(len(winsorized)) * winsorizePercent)
	upperIndex := len(winsorized) - 1 - lowerIndex

	if lowerIndex >= upperIndex {
		median, _ := stats.Median(winsorized)
		return median
	}

	lowerBound := winsorized[lowerIndex]
	upperBound := winsorized[upperIndex]

	// 应用Winsorization
	for i := range winsorized {
		if winsorized[i] < lowerBound {
			winsorized[i] = lowerBound
		} else if winsorized[i] > upperBound {
			winsorized[i] = upperBound
		}
	}

	winsorizedMean, _ := stats.Mean(winsorized)
	return winsorizedMean
}

// calculateUniqueCount 计算唯一值个数
func calculateUniqueCount(values []float64) int {
	uniqueMap := make(map[float64]bool)
	for _, v := range values {
		uniqueMap[v] = true
	}
	return len(uniqueMap)
}
