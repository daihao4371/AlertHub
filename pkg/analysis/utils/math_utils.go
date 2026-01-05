package utils

import (
	"alertHub/internal/models"
	"math"

	"github.com/montanaflynn/stats"
)

// MathUtils 通用数学计算工具
type MathUtils struct{}

// NewMathUtils 创建数学工具实例
func NewMathUtils() *MathUtils {
	return &MathUtils{}
}

// ExtractValues 从DataPoint数组提取数值
func (mu *MathUtils) ExtractValues(dataPoints []*models.DataPoint) []float64 {
	values := make([]float64, 0, len(dataPoints))
	for _, point := range dataPoints {
		if point != nil && !math.IsNaN(point.Value) && !math.IsInf(point.Value, 0) {
			values = append(values, point.Value)
		}
	}
	return values
}

// ExtractTimestamps 从DataPoint数组提取时间戳
func (mu *MathUtils) ExtractTimestamps(dataPoints []*models.DataPoint) []int64 {
	timestamps := make([]int64, 0, len(dataPoints))
	for _, point := range dataPoints {
		if point != nil {
			timestamps = append(timestamps, point.Timestamp)
		}
	}
	return timestamps
}

// CalculateMean 计算均值
func (mu *MathUtils) CalculateMean(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}
	sum := 0.0
	for _, v := range values {
		sum += v
	}
	return sum / float64(len(values))
}

// CalculateStdDev 计算标准差
func (mu *MathUtils) CalculateStdDev(values []float64) float64 {
	if len(values) <= 1 {
		return 0
	}
	mean := mu.CalculateMean(values)
	sumSquaredDiff := 0.0
	for _, v := range values {
		diff := v - mean
		sumSquaredDiff += diff * diff
	}
	return math.Sqrt(sumSquaredDiff / float64(len(values)-1))
}

// CalculatePearsonCorrelation 计算皮尔逊相关系数
func (mu *MathUtils) CalculatePearsonCorrelation(x, y []float64) float64 {
	if len(x) != len(y) || len(x) < 2 {
		return 0.0
	}

	n := float64(len(x))
	sumX := 0.0
	sumY := 0.0
	sumXY := 0.0
	sumX2 := 0.0
	sumY2 := 0.0

	for i := range x {
		sumX += x[i]
		sumY += y[i]
		sumXY += x[i] * y[i]
		sumX2 += x[i] * x[i]
		sumY2 += y[i] * y[i]
	}

	numerator := n*sumXY - sumX*sumY
	denominator := math.Sqrt((n*sumX2 - sumX*sumX) * (n*sumY2 - sumY*sumY))

	if denominator == 0 {
		return 0.0
	}

	return numerator / denominator
}

// CalculateAutocorrelation 计算自相关系数
func (mu *MathUtils) CalculateAutocorrelation(values []float64, lag int) float64 {
	n := len(values)
	if lag >= n {
		return 0
	}

	mean, _ := stats.Mean(values)

	numerator := 0.0
	denominator := 0.0

	for i := 0; i < n-lag; i++ {
		numerator += (values[i] - mean) * (values[i+lag] - mean)
	}

	for i := 0; i < n; i++ {
		denominator += (values[i] - mean) * (values[i] - mean)
	}

	if denominator == 0 {
		return 0
	}

	return numerator / denominator
}

// AlignTimeSeries 对齐两个时间序列数据
func (mu *MathUtils) AlignTimeSeries(primary, related []float64) ([]float64, []float64) {
	minLen := len(primary)
	if len(related) < minLen {
		minLen = len(related)
	}
	return primary[:minLen], related[:minLen]
}

// Min 返回两个整数的最小值
func (mu *MathUtils) Min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// Max 返回两个整数的最大值
func (mu *MathUtils) Max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// MinFloat64 返回两个浮点数的最小值
func (mu *MathUtils) MinFloat64(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}

// MaxFloat64 返回两个浮点数的最大值
func (mu *MathUtils) MaxFloat64(a, b float64) float64 {
	if a > b {
		return a
	}
	return b
}

// CalculateLinearRegression 计算线性回归的斜率和相关系数
func (mu *MathUtils) CalculateLinearRegression(values []float64) (slope float64, correlation float64) {
	n := len(values)
	if n < 2 {
		return 0.0, 0.0
	}

	// 创建x序列 (索引)
	x := make([]float64, n)
	for i := range x {
		x[i] = float64(i)
	}

	// 使用现有的皮尔逊相关系数函数
	correlation = mu.CalculatePearsonCorrelation(x, values)

	// 计算斜率
	nf := float64(n)
	sumX := 0.0
	sumY := 0.0
	sumXY := 0.0
	sumX2 := 0.0

	for i, y := range values {
		xVal := float64(i)
		sumX += xVal
		sumY += y
		sumXY += xVal * y
		sumX2 += xVal * xVal
	}

	denominator := nf*sumX2 - sumX*sumX
	if denominator == 0 {
		slope = 0.0
	} else {
		slope = (nf*sumXY - sumX*sumY) / denominator
	}

	return slope, correlation
}

// 全局实例，供其他包使用
var GlobalMathUtils = NewMathUtils()