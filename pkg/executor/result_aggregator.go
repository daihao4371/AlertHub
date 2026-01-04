package executor

import (
	"alertHub/internal/models"
	"alertHub/internal/types"
	"fmt"
	"strings"
	"time"
)

// QueryResultAggregator 查询结果聚合器
type QueryResultAggregator struct {
	CollectionStrategies map[string]AggregationStrategy
	DataTransformers     []DataTransformer
	ValidationRules      []ValidationRule
}

// NewQueryResultAggregator 创建查询结果聚合器实例
func NewQueryResultAggregator() *QueryResultAggregator {
	return &QueryResultAggregator{
		CollectionStrategies: make(map[string]AggregationStrategy),
		DataTransformers:     make([]DataTransformer, 0),
		ValidationRules:      make([]ValidationRule, 0),
	}
}

// AggregateResults 聚合查询结果（生产级别实现）
func (qra *QueryResultAggregator) AggregateResults(
	executionContext *QueryExecutionContext,
) (*types.DataCollectionResult, error) {
	executionContext.Mutex.RLock()
	defer executionContext.Mutex.RUnlock()

	result := &types.DataCollectionResult{
		RelatedMetrics: make(map[string]*models.MetricDataSet),
		MetricRelationships: &types.MetricRelationshipGraph{
			Nodes: make([]*types.MetricNode, 0),
			Edges: make([]*types.RelationshipEdge, 0),
		},
		CollectionMetadata: &types.DataCollectionMetadata{
			CollectionId: executionContext.ExecutionId,
			StartTime:    executionContext.StartTime.UnixMilli(),
			EndTime:      time.Now().UnixMilli(),
			Duration:     time.Since(executionContext.StartTime).Milliseconds(),
		},
	}

	// 分离主要指标和相关指标
	primaryMetricFound := false
	for taskId, queryResult := range executionContext.Results {
		if queryResult.Success && queryResult.Data != nil {
			// 确定是否为主要指标
			if !primaryMetricFound && (strings.Contains(taskId, "primary") ||
				queryResult.Metadata["type"] == "primary") {
				result.PrimaryMetric = queryResult.Data
				primaryMetricFound = true
			} else {
				result.RelatedMetrics[queryResult.MetricName] = queryResult.Data
			}

			// 构建指标关系图节点
			node := &types.MetricNode{
				ID:         queryResult.MetricName,
				Name:       queryResult.MetricName,
				Type:       "metric",
				Importance: qra.calculateImportance(queryResult),
				Properties: map[string]interface{}{
					"metricType":    queryResult.Metadata["queryType"],
					"executionTime": queryResult.Duration.Milliseconds(),
					"dataPoints":    len(queryResult.Data.TimeSeries),
					"dataQuality":   queryResult.Data.DataQuality,
				},
			}
			result.MetricRelationships.Nodes = append(result.MetricRelationships.Nodes, node)
		}
	}

	// 构建指标间的关系边
	qra.buildMetricRelationships(result, executionContext.Results)

	return result, nil
}

// calculateImportance 计算指标重要性
func (qra *QueryResultAggregator) calculateImportance(queryResult *QueryResult) float64 {
	importance := 0.5 // 基础重要性

	// 根据查询类型调整重要性
	if queryResult.Metadata["type"] == "primary" {
		importance = 1.0
	} else if queryResult.Metadata["type"] == "parallel" {
		importance = 0.8
	} else if queryResult.Metadata["type"] == "dependent" {
		importance = 0.6
	}

	// 根据数据量调整重要性
	if queryResult.Data != nil && len(queryResult.Data.TimeSeries) > 0 {
		importance += 0.1 // 有数据的指标更重要
	}

	// 根据执行时间调整重要性（执行时间短的指标可能更稳定）
	if queryResult.Duration < 100*time.Millisecond {
		importance += 0.1
	}

	return importance
}

// buildMetricRelationships 构建指标间关系
func (qra *QueryResultAggregator) buildMetricRelationships(
	result *types.DataCollectionResult,
	queryResults map[string]*QueryResult,
) {
	// 构建指标间的关系边
	processedPairs := make(map[string]bool)

	for _, queryResult1 := range queryResults {
		if !queryResult1.Success || queryResult1.Data == nil {
			continue
		}

		for _, queryResult2 := range queryResults {
			if !queryResult2.Success || queryResult2.Data == nil ||
				queryResult1.MetricName == queryResult2.MetricName {
				continue
			}

			// 避免重复处理相同的指标对
			pairKey := fmt.Sprintf("%s-%s", queryResult1.MetricName, queryResult2.MetricName)
			reversePairKey := fmt.Sprintf("%s-%s", queryResult2.MetricName, queryResult1.MetricName)

			if processedPairs[pairKey] || processedPairs[reversePairKey] {
				continue
			}
			processedPairs[pairKey] = true

			// 计算指标间的关联强度
			strength := qra.calculateRelationshipStrength(queryResult1, queryResult2)
			if strength > 0.1 { // 只保留有意义的关系
				edge := &types.RelationshipEdge{
					From:      queryResult1.MetricName,
					To:        queryResult2.MetricName,
					Type:      "correlation",
					Strength:  strength,
					Direction: "bidirectional",
					Properties: map[string]interface{}{
						"discoveryMethod": "temporal_correlation",
						"confidence":      strength,
					},
				}
				result.MetricRelationships.Edges = append(result.MetricRelationships.Edges, edge)
			}
		}
	}
}

// calculateRelationshipStrength 计算指标间关系强度
func (qra *QueryResultAggregator) calculateRelationshipStrength(
	result1, result2 *QueryResult,
) float64 {
	// 基于标签重叠度和数据相关性计算关系强度
	if result1.Data == nil || result2.Data == nil {
		return 0.0
	}

	// 从第一个数据点获取标签进行比较
	var labels1, labels2 map[string]interface{}

	if len(result1.Data.TimeSeries) > 0 && result1.Data.TimeSeries[0].Labels != nil {
		labels1 = result1.Data.TimeSeries[0].Labels
	} else {
		labels1 = make(map[string]interface{})
	}

	if len(result2.Data.TimeSeries) > 0 && result2.Data.TimeSeries[0].Labels != nil {
		labels2 = result2.Data.TimeSeries[0].Labels
	} else {
		labels2 = make(map[string]interface{})
	}

	if len(labels1) == 0 && len(labels2) == 0 {
		return 0.3 // 基础关联度
	}

	// 计算标签重叠度
	intersection := 0
	union := 0

	allLabels := make(map[string]bool)
	for k, v1 := range labels1 {
		allLabels[k] = true
		if v2, exists := labels2[k]; exists && fmt.Sprintf("%v", v1) == fmt.Sprintf("%v", v2) {
			intersection++
		}
	}
	for k := range labels2 {
		allLabels[k] = true
	}
	union = len(allLabels)

	if union == 0 {
		return 0.3
	}

	// Jaccard相似度
	jaccard := float64(intersection) / float64(union)

	// 调整关系强度：0.1 到 0.9 之间
	strength := 0.1 + jaccard*0.8

	return strength
}

// 接口定义

// AggregationStrategy 聚合策略接口
type AggregationStrategy interface {
	Aggregate([]*QueryResult) (*models.MetricDataSet, error)
}

// DataTransformer 数据转换器接口
type DataTransformer interface {
	Transform(*models.MetricDataSet) (*models.MetricDataSet, error)
}

// ValidationRule 验证规则接口
type ValidationRule interface {
	Validate(*QueryResult) error
}