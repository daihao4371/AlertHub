package tools

import (
	"fmt"
	"math"
	"sort"
	"strings"
	"time"
)

// MetricCategory 指标分类结构
type MetricCategory struct {
	Name    string   `json:"category"` // 分类名称
	Count   int      `json:"count"`    // 该分类下指标数量
	Sample  []string `json:"metrics"`  // 分类下的指标样例
}

// FilterMetricsByKeyword 根据关键词过滤指标列表
func FilterMetricsByKeyword(metrics []string, keyword string) []string {
	if keyword == "" {
		return metrics
	}
	
	keyword = strings.ToLower(keyword)
	var filtered []string
	
	for _, metric := range metrics {
		if strings.Contains(strings.ToLower(metric), keyword) {
			filtered = append(filtered, metric)
		}
	}
	
	return filtered
}

// PaginateSlice 对字符串切片进行分页处理，包含边界检查
func PaginateSlice(items []string, page, size int) ([]string, int) {
	total := len(items)
	
	// 参数校验和默认值设置
	if page <= 0 {
		page = 1
	}
	if size <= 0 {
		size = 20
	}
	if size > 100 {
		size = 100 // 限制最大页面大小
	}
	
	// 计算分页索引
	start := (page - 1) * size
	end := start + size
	
	// 边界检查
	if start >= total {
		return []string{}, total
	}
	
	if end > total {
		end = total
	}
	
	return items[start:end], total
}

// CategorizeMetrics 根据指标前缀对指标进行分类
func CategorizeMetrics(metrics []string) []MetricCategory {
	categoryMap := make(map[string][]string)
	
	// 按前缀分组指标
	for _, metric := range metrics {
		category := extractPrefix(metric)
		categoryMap[category] = append(categoryMap[category], metric)
	}
	
	// 转换为分类列表
	categories := make([]MetricCategory, 0, len(categoryMap))
	for name, metricList := range categoryMap {
		// 每个分类最多返回5个指标作为样例
		sample := metricList
		if len(sample) > 5 {
			sample = sample[:5]
		}
		
		categories = append(categories, MetricCategory{
			Name:   name,
			Count:  len(metricList),
			Sample: sample,
		})
	}
	
	// 按指标数量降序排列
	sort.Slice(categories, func(i, j int) bool {
		return categories[i].Count > categories[j].Count
	})
	
	return categories
}

// extractPrefix 从指标名称中提取分类前缀
func extractPrefix(metric string) string {
	// 查找第一个下划线位置
	underscoreIdx := strings.Index(metric, "_")
	if underscoreIdx > 0 {
		return metric[:underscoreIdx+1] // 包含下划线
	}
	
	// 如果没有下划线，使用首字母
	if len(metric) > 0 {
		return strings.ToUpper(string(metric[0])) + "*"
	}
	
	return "其他"
}

// CalculateOptimalStep 计算时间范围查询的最佳步长
func CalculateOptimalStep(start, end int64, customStep string) (time.Duration, error) {
	duration := time.Unix(end, 0).Sub(time.Unix(start, 0))
	
	// 如果没有自定义步长，使用自动计算
	if customStep == "" || customStep == "auto" {
		return calculateAutoStep(duration), nil
	}
	
	// 解析自定义步长
	step, err := time.ParseDuration(customStep)
	if err != nil {
		return 0, err
	}
	
	// 验证步长边界
	minStep := time.Second
	maxStep := duration / 10 // 最大步长不超过时间范围的1/10
	
	if step < minStep {
		step = minStep
	} else if step > maxStep {
		step = maxStep
	}
	
	return step, nil
}

// DataPoint 时间序列数据点
type DataPoint struct {
	Timestamp int64   `json:"timestamp"`
	Value     float64 `json:"value"`
}

// DownsamplingConfig 下采样配置
type DownsamplingConfig struct {
	TargetPoints int     // 目标数据点数量（默认800）
	Algorithm    string  // 下采样算法（lttb, average, max, min）
	Threshold    float64 // 算法特定阈值
}

// DefaultDownsamplingConfig 默认下采样配置
func DefaultDownsamplingConfig() DownsamplingConfig {
	return DownsamplingConfig{
		TargetPoints: 800,
		Algorithm:    "lttb",
		Threshold:    0.0,
	}
}

// LTTBDownsample LTTB下采样算法实现
// Largest Triangle Three Buckets - 保留数据形状的下采样
func LTTBDownsample(data []DataPoint, targetCount int) []DataPoint {
	if len(data) <= targetCount {
		return data
	}

	bucketSize := float64(len(data)) / float64(targetCount)
	sampled := make([]DataPoint, 0, targetCount)

	// 保留第一个和最后一个点
	sampled = append(sampled, data[0])

	for i := 1; i < targetCount-1; i++ {
		// 当前bucket的范围
		avgRangeStart := int(float64(i-1)*bucketSize) + 1
		avgRangeEnd := int(float64(i)*bucketSize) + 1
		if avgRangeEnd >= len(data) {
			avgRangeEnd = len(data) - 1
		}

		// 下一个bucket的第一个点
		nextBucketStart := int(float64(i)*bucketSize) + 1
		nextBucketEnd := int(float64(i+1)*bucketSize) + 1
		if nextBucketStart >= len(data) {
			break
		}
		if nextBucketEnd > len(data) {
			nextBucketEnd = len(data)
		}

		// 计算下一个bucket的平均点
		var avgNext DataPoint
		if nextBucketEnd > nextBucketStart {
			avgTime := int64(0)
			avgValue := 0.0
			count := 0
			for j := nextBucketStart; j < nextBucketEnd; j++ {
				avgTime += data[j].Timestamp
				avgValue += data[j].Value
				count++
			}
			if count > 0 {
				avgNext = DataPoint{
					Timestamp: avgTime / int64(count),
					Value:     avgValue / float64(count),
				}
			}
		}

		// 在当前bucket中找到形成最大三角形面积的点
		maxArea := -1.0
		maxAreaIdx := avgRangeStart

		for j := avgRangeStart; j < avgRangeEnd && j < len(data); j++ {
			area := calculateTriangleArea(
				sampled[len(sampled)-1],
				data[j],
				avgNext,
			)
			if area > maxArea {
				maxArea = area
				maxAreaIdx = j
			}
		}

		if maxAreaIdx < len(data) {
			sampled = append(sampled, data[maxAreaIdx])
		}
	}

	// 保留最后一个点
	sampled = append(sampled, data[len(data)-1])
	return sampled
}

// calculateTriangleArea 计算三个数据点构成的三角形面积
func calculateTriangleArea(a, b, c DataPoint) float64 {
	// 使用向量叉积计算面积
	// Area = |((b.x - a.x) * (c.y - a.y) - (c.x - a.x) * (b.y - a.y))| / 2
	return math.Abs(float64(b.Timestamp-a.Timestamp)*
		(c.Value-a.Value) -
		float64(c.Timestamp-a.Timestamp)*
		(b.Value-a.Value)) / 2
}

// AverageDownsample 平均值下采样
func AverageDownsample(data []DataPoint, targetCount int) []DataPoint {
	if len(data) <= targetCount {
		return data
	}

	bucketSize := float64(len(data)) / float64(targetCount)
	sampled := make([]DataPoint, 0, targetCount)

	for i := 0; i < targetCount; i++ {
		start := int(float64(i) * bucketSize)
		end := int(float64(i+1) * bucketSize)
		if end > len(data) {
			end = len(data)
		}
		if start >= end {
			continue
		}

		// 计算bucket内的平均值
		var sumTimestamp int64
		var sumValue float64
		count := end - start

		for j := start; j < end; j++ {
			sumTimestamp += data[j].Timestamp
			sumValue += data[j].Value
		}

		sampled = append(sampled, DataPoint{
			Timestamp: sumTimestamp / int64(count),
			Value:     sumValue / float64(count),
		})
	}

	return sampled
}

// calculateAutoStep 基于时间范围智能计算步长
func calculateAutoStep(duration time.Duration) time.Duration {
	// 更精细的步长计算，确保查询性能
	switch {
	case duration <= 15*time.Minute:
		return 15 * time.Second  // 15分钟内，15秒步长
	case duration <= time.Hour:
		return 30 * time.Second  // 1小时内，30秒步长
	case duration <= 6*time.Hour:
		return 2 * time.Minute   // 6小时内，2分钟步长
	case duration <= 24*time.Hour:
		return 5 * time.Minute   // 24小时内，5分钟步长
	case duration <= 7*24*time.Hour:
		return 30 * time.Minute  // 7天内，30分钟步长
	case duration <= 30*24*time.Hour:
		return 2 * time.Hour     // 30天内，2小时步长
	default:
		return 6 * time.Hour     // 超过30天，6小时步长
	}
}

// EstimateDataPoints 估算查询将返回的数据点数量
func EstimateDataPoints(start, end int64, step time.Duration) int {
	duration := time.Unix(end, 0).Sub(time.Unix(start, 0))
	return int(duration / step)
}

// OptimizeQueryParameters 优化查询参数以避免过载
func OptimizeQueryParameters(start, end int64, requestedStep string, maxPoints int) (time.Duration, error) {
	if maxPoints <= 0 {
		maxPoints = 800 // 默认最大点数
	}

	duration := time.Unix(end, 0).Sub(time.Unix(start, 0))
	minStep := duration / time.Duration(maxPoints)

	// 如果没有指定步长，使用自动计算
	if requestedStep == "" || requestedStep == "auto" {
		autoStep := calculateAutoStep(duration)
		if autoStep < minStep {
			return minStep, nil
		}
		return autoStep, nil
	}

	// 解析请求的步长
	parsedStep, err := time.ParseDuration(requestedStep)
	if err != nil {
		return 0, fmt.Errorf("invalid step format: %s", requestedStep)
	}

	// 确保步长不会产生过多数据点
	if parsedStep < minStep {
		return minStep, nil
	}

	return parsedStep, nil
}