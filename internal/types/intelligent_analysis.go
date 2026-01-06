package types

import (
	"alertHub/internal/models"
	"fmt"
	"time"
)

// ================================
// 智能分析核心类型定义
// ================================

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

// DataCollectionConfig 数据采集配置
type DataCollectionConfig struct {
	MaxRelatedMetrics    int                       `json:"maxRelatedMetrics"`
	ParallelQueryLimit   int                       `json:"parallelQueryLimit"`
	QueryTimeout         time.Duration             `json:"queryTimeout"`
	TimeRanges           map[string]*TimeRangeInfo `json:"timeRanges"`
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
// API请求/响应类型定义
// ================================

// RequestDataCollection 数据采集请求
type RequestDataCollection struct {
	AlertId            string                 `json:"alertId" form:"alertId" binding:"required"`               // 告警ID
	RuleId             string                 `json:"ruleId" form:"ruleId" binding:"required"`                 // 规则ID
	RuleName           string                 `json:"ruleName" form:"ruleName" binding:"required"`             // 规则名称
	PromQL             string                 `json:"promQL" form:"promQL" binding:"required"`                 // PromQL查询语句
	AlertStatus        string                 `json:"alertStatus" form:"alertStatus"`                          // 告警状态
	Severity           string                 `json:"severity" form:"severity"`                                // 严重程度
	StartTime          int64                  `json:"startTime" form:"startTime"`                              // 开始时间
	Labels             map[string]string      `json:"labels" form:"labels"`                                    // 标签
	MaxRelatedMetrics  int                    `json:"maxRelatedMetrics" form:"maxRelatedMetrics"`              // 最大相关指标数
	QueryTimeout       string                 `json:"queryTimeout" form:"queryTimeout"`                       // 查询超时时间
	ParallelQueryLimit int                    `json:"parallelQueryLimit" form:"parallelQueryLimit"`           // 并行查询限制
	TimeRange          *TimeRangeInfo         `json:"timeRange" form:"timeRange"`                              // 时间范围
}

// Validate 验证数据采集请求参数
func (r *RequestDataCollection) Validate() error {
	if r.AlertId == "" {
		return fmt.Errorf("告警ID不能为空")
	}
	if r.RuleId == "" {
		return fmt.Errorf("规则ID不能为空")
	}
	if r.RuleName == "" {
		return fmt.Errorf("规则名称不能为空")
	}
	if r.PromQL == "" {
		return fmt.Errorf("PromQL不能为空")
	}
	return nil
}

// ResponseDataCollection 数据采集响应
type ResponseDataCollection struct {
	CollectionId string                           `json:"collectionId"` // 采集ID
	Context      *models.UniversalAnalysisContext `json:"context"`      // 采集的上下文
	ProcessedAt  int64                            `json:"processedAt"`  // 处理时间
	Duration     int64                            `json:"duration"`     // 耗时(ms)
	Status       string                           `json:"status"`       // 状态
}

// RequestDataStandardize 数据标准化请求
type RequestDataStandardize struct {
	MetricName  string                 `json:"metricName" form:"metricName" binding:"required"`   // 指标名称
	MetricType  string                 `json:"metricType" form:"metricType"`                      // 指标类型
	Values      []float64              `json:"values" form:"values" binding:"required"`           // 数值序列
	Timestamps  []int64                `json:"timestamps" form:"timestamps" binding:"required"`   // 时间戳序列
	Labels      map[string]string      `json:"labels" form:"labels"`                              // 标签
	Metadata    map[string]interface{} `json:"metadata" form:"metadata"`                          // 元数据
	Features    []string               `json:"features" form:"features"`                          // 需要提取的特征类型
}

// Validate 验证请求参数
func (r *RequestDataStandardize) Validate() error {
	if r.MetricName == "" {
		return fmt.Errorf("指标名称不能为空")
	}
	if len(r.Values) == 0 {
		return fmt.Errorf("数值序列不能为空")
	}
	if len(r.Timestamps) == 0 {
		return fmt.Errorf("时间戳序列不能为空")
	}
	if len(r.Values) != len(r.Timestamps) {
		return fmt.Errorf("数值序列和时间戳序列长度不匹配")
	}
	return nil
}

// ResponseDataStandardize 数据标准化响应
type ResponseDataStandardize struct {
	MetricName         string                      `json:"metricName"`         // 指标名称
	MetricType         string                      `json:"metricType"`         // 指标类型
	Features           map[string]interface{}      `json:"features"`           // 提取的特征
	Quality            *models.DataQualityInfo     `json:"quality"`            // 数据质量信息
	ProcessedAt        int64                       `json:"processedAt"`        // 处理时间
	ProcessingDuration int64                       `json:"processingDuration"` // 处理耗时(ms)
}