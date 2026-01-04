package types

import (
	"alertHub/internal/models"
	"time"
)

// ================================
// 数据采集相关类型定义
// ================================

// DataCollectionResult 数据采集结果
type DataCollectionResult struct {
	PrimaryMetric       *models.MetricDataSet         `json:"primaryMetric"`       // 主要指标数据
	RelatedMetrics      map[string]*models.MetricDataSet `json:"relatedMetrics"`      // 相关指标数据
	MetricRelationships *MetricRelationshipGraph      `json:"metricRelationships"` // 指标关系图
	CollectionMetadata  *DataCollectionMetadata       `json:"collectionMetadata"`  // 采集元数据
	QualityReport       *DataCollectionQualityReport  `json:"qualityReport"`       // 数据质量报告
}

// DataCollectionMetadata 数据采集元数据
type DataCollectionMetadata struct {
	CollectionId     string                 `json:"collectionId"`     // 采集ID
	StartTime        int64                  `json:"startTime"`        // 开始时间
	EndTime          int64                  `json:"endTime"`          // 结束时间
	Duration         int64                  `json:"duration"`         // 采集耗时(ms)
	SourceConfig     *DataCollectionConfig  `json:"sourceConfig"`     // 源配置
	DiscoveryResults *DiscoveryResults      `json:"discoveryResults"` // 发现结果统计
	QueryPlan        *QueryExecutionPlan    `json:"queryPlan"`        // 查询计划
	Extensions       map[string]interface{} `json:"extensions"`       // 扩展信息
}

// DiscoveryResults 发现结果统计
type DiscoveryResults struct {
	RelatedMetricsFound  int                    `json:"relatedMetricsFound"`  // 发现的相关指标数量
	RelationshipsFound   int                    `json:"relationshipsFound"`   // 发现的关系数量
	DiscoveryStrategies  []string               `json:"discoveryStrategies"`  // 使用的发现策略
	DiscoveryDuration    int64                  `json:"discoveryDuration"`    // 发现耗时(ms)
	DiscoveryDetails     map[string]interface{} `json:"discoveryDetails"`     // 详细发现信息
}

// DataCollectionQualityReport 数据质量报告
type DataCollectionQualityReport struct {
	OverallQuality         float64                            `json:"overallQuality"`         // 整体质量评分 0-1
	MetricQualities        map[string]*models.DataQualityInfo `json:"metricQualities"`        // 各指标质量信息
	CollectionIssues       []string                           `json:"collectionIssues"`       // 采集问题列表
	QualityRecommendations []string                           `json:"qualityRecommendations"` // 质量改进建议
}

// ================================
// 指标关系图相关类型
// ================================

// MetricRelationshipGraph 指标关系图
type MetricRelationshipGraph struct {
	Nodes []*MetricNode      `json:"nodes"`
	Edges []*RelationshipEdge `json:"edges"`
}

// MetricNode 指标节点
type MetricNode struct {
	ID         string                 `json:"id"`
	Name       string                 `json:"name"`
	Type       string                 `json:"type"`
	Importance float64                `json:"importance"`
	Properties map[string]interface{} `json:"properties"`
}

// RelationshipEdge 关系边
type RelationshipEdge struct {
	From       string                 `json:"from"`
	To         string                 `json:"to"`
	Type       string                 `json:"type"`
	Strength   float64                `json:"strength"`
	Direction  string                 `json:"direction"`
	Properties map[string]interface{} `json:"properties"`
}

// ================================
// 配置相关类型
// ================================

// DataCollectionConfig 数据采集配置
type DataCollectionConfig struct {
	MaxRelatedMetrics    int                       `json:"maxRelatedMetrics"`
	ParallelQueryLimit   int                       `json:"parallelQueryLimit"`
	QueryTimeout         time.Duration             `json:"queryTimeout"`
	TimeRanges           map[string]*TimeRangeInfo `json:"timeRanges"`
	DiscoveryStrategies  []string                  `json:"discoveryStrategies"`
}

// TimeRangeInfo 时间范围信息
type TimeRangeInfo struct {
	StartTime int64 `json:"startTime"`
	EndTime   int64 `json:"endTime"`
	Step      int64 `json:"step"`
}

// IsInstantQuery 判断是否为即时查询
func (tr *TimeRangeInfo) IsInstantQuery() bool {
	return tr.StartTime == tr.EndTime
}

// GetTimeRange 获取时间范围配置
func (dc *DataCollectionConfig) GetTimeRange(rangeType string) *TimeRangeInfo {
	if tr, exists := dc.TimeRanges[rangeType]; exists {
		return tr
	}
	// 默认时间范围：最近1小时
	now := time.Now()
	return &TimeRangeInfo{
		StartTime: now.Add(-1 * time.Hour).Unix(),
		EndTime:   now.Unix(),
		Step:      60, // 1分钟步长
	}
}

// ================================
// 指标元数据相关类型
// ================================

// MetricMetadata 指标元数据
type MetricMetadata struct {
	MetricName    string                 `json:"metricName"`    // 指标名称
	MetricType    string                 `json:"metricType"`    // 指标类型(从名称推断)
	PromQL        string                 `json:"promQL"`        // 原始PromQL
	ParsedLabels  map[string]string      `json:"parsedLabels"`  // 解析出的标签
	TimeRange     *TimeRangeInfo         `json:"timeRange"`     // 时间范围信息
	Aggregation   *AggregationInfo       `json:"aggregation"`   // 聚合信息
	Functions     []string               `json:"functions"`     // 使用的函数列表
	Dependencies  []string               `json:"dependencies"`  // 依赖的其他指标
	Metadata      map[string]interface{} `json:"metadata"`      // 扩展元数据
}

// AggregationInfo 聚合信息
type AggregationInfo struct {
	Function string   `json:"function"` // sum, avg, max, min等
	GroupBy  []string `json:"groupBy"`  // 分组标签
	Without  []string `json:"without"`  // 排除标签
}

// ================================
// 相关指标发现类型
// ================================

// RelatedMetricDescriptor 相关指标描述符
type RelatedMetricDescriptor struct {
	MetricName      string                 `json:"metricName"`      // 指标名称
	RelationType    string                 `json:"relationType"`    // 关系类型
	Relevance       float64                `json:"relevance"`       // 相关性评分 0-1
	DiscoveryMethod string                 `json:"discoveryMethod"` // 发现方法
	QueryTemplate   string                 `json:"queryTemplate"`   // 查询模板
	Labels          map[string]string      `json:"labels"`          // 相关标签
	Metadata        map[string]interface{} `json:"metadata"`        // 扩展元数据
}

// ================================
// 查询执行相关类型
// ================================

// QueryExecutionPlan 查询执行计划
type QueryExecutionPlan struct {
	PrimaryQuery     *QueryTask             `json:"primaryQuery"`     // 主查询
	ParallelQueries  []*QueryTask           `json:"parallelQueries"`  // 并行查询列表
	DependentQueries []*QueryTask           `json:"dependentQueries"` // 依赖查询列表
	ExecutionOrder   []string               `json:"executionOrder"`   // 执行顺序
	EstimatedTime    int64                  `json:"estimatedTime"`    // 预估执行时间(ms)
	ResourceUsage    *ResourceUsageEstimate `json:"resourceUsage"`    // 资源使用估计
}

// QueryTask 查询任务
type QueryTask struct {
	TaskId       string                 `json:"taskId"`       // 任务ID
	MetricName   string                 `json:"metricName"`   // 指标名称
	Query        string                 `json:"query"`        // 查询语句
	TimeRange    *TimeRangeInfo         `json:"timeRange"`    // 时间范围
	Priority     float64                `json:"priority"`     // 优先级
	Dependencies []string               `json:"dependencies"` // 依赖任务
	Metadata     map[string]interface{} `json:"metadata"`     // 任务元数据
}

// ResourceUsageEstimate 资源使用估计
type ResourceUsageEstimate struct {
	EstimatedMemory    int64   `json:"estimatedMemory"`    // 预估内存使用(bytes)
	EstimatedCPU       float64 `json:"estimatedCpu"`       // 预估CPU使用
	EstimatedDuration  int64   `json:"estimatedDuration"`  // 预估执行时间(ms)
	ConcurrencyLevel   int     `json:"concurrencyLevel"`   // 并发级别
	DataPointsExpected int     `json:"dataPointsExpected"` // 预期数据点数
}

// ================================
// 元数据注册表相关类型
// ================================

// QueryPattern 查询模式分析结果
type QueryPattern struct {
	HasAggregation bool `json:"hasAggregation"` // 是否包含聚合函数
	HasFunctions   bool `json:"hasFunctions"`   // 是否包含函数调用
	HasBinaryOps   bool `json:"hasBinaryOps"`   // 是否包含二元运算
	HasSubqueries  bool `json:"hasSubqueries"`  // 是否包含子查询
	Complexity     int  `json:"complexity"`     // 查询复杂度评分
}

// RegistryStats 注册表统计信息
type RegistryStats struct {
	TotalMetrics int            `json:"totalMetrics"` // 总指标数量
	TypeCounts   map[string]int `json:"typeCounts"`   // 各类型指标数量统计
	LastCleanup  time.Time      `json:"lastCleanup"`  // 上次清理时间
}