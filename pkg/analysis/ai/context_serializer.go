package ai

import (
	"alertHub/internal/ctx"
	"alertHub/internal/models"
	"encoding/json"
)

// ContextSerializer 上下文序列化器
type ContextSerializer struct{}

// NewContextSerializer 创建上下文序列化器
func NewContextSerializer() *ContextSerializer {
	return &ContextSerializer{}
}

// Serialize 序列化分析上下文为字符串
func (cs *ContextSerializer) Serialize(ctx *ctx.Context, analysisContext *models.UniversalAnalysisContext) (string, error) {
	// 提取关键信息，避免序列化过多数据
	summary := map[string]interface{}{
		"contextId":     analysisContext.ContextId,
		"tenantId":      analysisContext.TenantId,
		"alertInfo":     analysisContext.AlertInfo,
		"ruleInfo":      analysisContext.RuleInfo,
		"timeContext":   analysisContext.TimeContext,
		"systemContext": analysisContext.SystemContext,
	}

	// 添加主要指标信息
	if analysisContext.PrimaryMetric != nil {
		summary["primaryMetric"] = map[string]interface{}{
			"metricName":  analysisContext.PrimaryMetric.MetricName,
			"metricType":  analysisContext.PrimaryMetric.MetricType,
			"unit":        analysisContext.PrimaryMetric.Unit,
			"description": analysisContext.PrimaryMetric.Description,
			"dataQuality": analysisContext.PrimaryMetric.DataQuality,
			"dataPoints":  len(analysisContext.PrimaryMetric.TimeSeries),
		}
	}

	// 添加相关指标数量
	if analysisContext.RelatedMetrics != nil {
		summary["relatedMetricsCount"] = len(analysisContext.RelatedMetrics)
	}

	// 序列化为JSON
	jsonData, err := json.Marshal(summary)
	if err != nil {
		return "", err
	}

	return string(jsonData), nil
}
