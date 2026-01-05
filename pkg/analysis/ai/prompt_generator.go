package ai

import (
	"alertHub/internal/ctx"
	"alertHub/internal/models"
	"alertHub/pkg/analysis/interfaces"
	"encoding/json"
	"fmt"
	"strings"
	"text/template"
	"time"

	"github.com/zeromicro/go-zero/core/logc"
)

// DynamicPromptGenerator 动态提示生成器 - 基于上下文自适应生成提示
type DynamicPromptGenerator struct {
	templates       map[string]*template.Template
	contextAnalyzer *ContextAnalyzer
}

// ContextAnalyzer 上下文分析器
type ContextAnalyzer struct{}

// NewDynamicPromptGenerator 创建动态提示生成器
func NewDynamicPromptGenerator() *DynamicPromptGenerator {
	generator := &DynamicPromptGenerator{
		templates:       make(map[string]*template.Template),
		contextAnalyzer: &ContextAnalyzer{},
	}

	// 初始化模板
	generator.initializeTemplates()
	return generator
}

// initializeTemplates 初始化提示模板
func (dpg *DynamicPromptGenerator) initializeTemplates() {
	// 通用分析模板
	universalTemplate := `
你是一位资深的监控分析专家，请对以下结构化监控数据进行深度分析。

## 分析上下文
{{.SerializedContext}}

## 分析要求
**分析原则**:
1. **完全基于数据**: 所有结论必须有数据支撑，避免基于指标名称的先入为主判断
2. **关系分析**: 重点分析指标间的数量关系、时序关系和因果关系
3. **模式识别**: 识别数据中的趋势、周期性、异常模式
4. **系统性思维**: 从系统整体角度分析问题，考虑依赖关系和影响链路
5. **操作导向**: 提供具体可执行的分析和处理步骤

**分析深度**: {{.AnalysisDepth}}
**关注领域**: {{.FocusAreas}}

请以JSON格式返回分析结果：

{
  "summary": {
    "title": "基于数据特征的问题描述",
    "description": "详细分析总结",
    "severity": "critical|high|medium|low",
    "category": "performance|resource|network|application|configuration|infrastructure",
    "confidence": 0.85,
    "keyFindings": ["关键发现1", "关键发现2"]
  },
  "dataAnalysis": {
    "primaryMetricAnalysis": {
      "metricName": "{{.PrimaryMetricName}}",
      "dataPoints": {{.DataPoints}},
      "statisticalFeatures": "统计特征分析",
      "trendAnalysis": "趋势分析",
      "anomalyAnalysis": "异常分析",
      "baselineComparison": "基线对比"
    },
    "relationshipAnalysis": {
      "correlatedMetrics": [{"metric": "指标名", "correlation": 0.8, "type": "positive"}],
      "causalChain": "可能的因果链",
      "impactScope": "影响范围分析"
    },
    "systemAnalysis": {
      "topologyImpact": "拓扑影响分析",
      "dependencyAnalysis": "依赖关系分析",
      "cascadingEffects": "级联效应分析"
    }
  },
  "rootCauseAnalysis": {
    "primaryHypothesis": "基于数据的主要假设",
    "supportingEvidence": [
      {
        "type": "statistical|pattern|correlation|anomaly",
        "description": "证据描述",
        "strength": 0.8,
        "data": "支撑数据"
      }
    ],
    "alternativeHypotheses": ["其他可能性"],
    "confidence": 0.85
  },
  "actionRecommendations": [
    {
      "priority": 1,
      "type": "immediate|investigation|optimization|prevention",
      "title": "建议标题",
      "rationale": "基于分析的理由",
      "steps": [
        {
          "order": 1,
          "action": "具体操作",
          "verification": "如何验证结果",
          "expectedOutcome": "预期结果"
        }
      ],
      "riskAssessment": "风险评估",
      "successMetrics": ["成功判断指标"]
    }
  ],
  "monitoringRecommendations": {
    "additionalMetrics": ["建议增加监控的指标"],
    "alertOptimization": "告警优化建议",
    "dashboardImprovements": "监控面板改进建议"
  },
  "confidence": 0.85,
  "limitations": ["分析局限性说明"],
  "followUpAnalysis": ["建议的后续分析方向"]
}

重要：请确保分析完全基于提供的数据特征，而不是基于指标名称的经验假设。
`

	tmpl, err := template.New("universal").Parse(universalTemplate)
	if err != nil {
		logc.Errorf(nil, "初始化通用分析模板失败: %v", err)
		return
	}
	dpg.templates["universal"] = tmpl
}

// GenerateAnalysisPrompt 生成分析提示
func (dpg *DynamicPromptGenerator) GenerateAnalysisPrompt(
	ctx *ctx.Context,
	analysisContext *models.UniversalAnalysisContext,
	request *AnalysisRequest,
) (string, error) {

	// 1. 分析上下文特征
	contextFeatures := dpg.contextAnalyzer.AnalyzeContext(ctx, analysisContext)

	// 2. 基于特征选择最适合的提示模板
	templateName := dpg.selectOptimalTemplate(contextFeatures)
	template, exists := dpg.templates[templateName]
	if !exists {
		return "", fmt.Errorf("未找到模板: %s", templateName)
	}

	// 3. 序列化上下文数据
	serializedContext, err := dpg.serializeContext(ctx, analysisContext)
	if err != nil {
		return "", fmt.Errorf("序列化上下文失败: %w", err)
	}

	// 4. 准备模板参数
	templateData := dpg.buildTemplateData(analysisContext, request, serializedContext)

	// 5. 生成提示
	var promptBuffer strings.Builder
	if err := template.Execute(&promptBuffer, templateData); err != nil {
		return "", fmt.Errorf("执行模板失败: %w", err)
	}

	prompt := promptBuffer.String()

	// 6. 优化提示（长度、清晰度等）
	optimizedPrompt := dpg.optimizePrompt(ctx, prompt)

	logc.Infof(ctx.Ctx, "[提示生成] 生成完成: 模板=%s, 长度=%d", templateName, len(optimizedPrompt))

	return optimizedPrompt, nil
}

// AnalyzeContext 分析上下文特征
func (ca *ContextAnalyzer) AnalyzeContext(
	ctx *ctx.Context,
	analysisContext *models.UniversalAnalysisContext,
) map[string]interface{} {

	features := make(map[string]interface{})

	// 数据规模特征
	if analysisContext.PrimaryMetric != nil {
		features["data_points"] = len(analysisContext.PrimaryMetric.TimeSeries)
		features["has_primary_metric"] = true

		if analysisContext.PrimaryMetric.DataQuality != nil {
			features["data_quality"] = analysisContext.PrimaryMetric.DataQuality.QualityScore
		}
	}

	// 相关指标特征
	features["related_metrics_count"] = len(analysisContext.RelatedMetrics)
	features["has_related_metrics"] = len(analysisContext.RelatedMetrics) > 0

	// 系统上下文特征
	if analysisContext.SystemContext != nil {
		features["has_system_context"] = true
		features["environment"] = analysisContext.SystemContext.Environment
		features["service_name"] = analysisContext.SystemContext.ServiceName
	}

	// 时间上下文特征
	if analysisContext.TimeContext != nil {
		features["has_time_context"] = true
		features["is_business_hours"] = analysisContext.TimeContext.IsBusinessHours

		// 计算时间跨度
		if timeRange, exists := analysisContext.TimeContext.TimeRanges["current"]; exists {
			features["time_span_hours"] = float64(timeRange.Duration) / 3600.0
		}
	}

	// 特征完整性
	if analysisContext.MetricFeatures != nil {
		features["has_statistical_features"] = analysisContext.MetricFeatures.StatisticalFeatures != nil
		features["has_timeseries_features"] = analysisContext.MetricFeatures.TimeSeriesFeatures != nil
		features["has_anomaly_features"] = analysisContext.MetricFeatures.AnomalyFeatures != nil
		features["has_pattern_features"] = analysisContext.MetricFeatures.PatternFeatures != nil
		features["has_correlation_features"] = analysisContext.MetricFeatures.CorrelationFeatures != nil
	}

	return features
}

// selectOptimalTemplate 选择最优模板
func (dpg *DynamicPromptGenerator) selectOptimalTemplate(features map[string]interface{}) string {
	// 简化的模板选择逻辑，后续可以基于机器学习优化

	// 检查数据完整性
	dataQuality, _ := features["data_quality"].(float64)
	hasRelatedMetrics, _ := features["has_related_metrics"].(bool)

	// 当前只有一个通用模板，后续可以扩展更多专业模板
	if dataQuality > 0.8 && hasRelatedMetrics {
		return "universal" // 高质量数据使用通用模板
	}

	return "universal" // 默认使用通用模板
}

// serializeContext 序列化上下文
func (dpg *DynamicPromptGenerator) serializeContext(
	ctx *ctx.Context,
	analysisContext *models.UniversalAnalysisContext,
) (string, error) {

	// 创建序列化友好的上下文结构
	serializable := map[string]interface{}{
		"contextId": analysisContext.ContextId,
		"createdAt": time.Unix(analysisContext.CreatedAt, 0).Format(time.RFC3339),
	}

	// 告警信息
	if analysisContext.AlertInfo != nil {
		serializable["alertInfo"] = map[string]interface{}{
			"ruleId":      analysisContext.AlertInfo.RuleId,
			"ruleName":    analysisContext.AlertInfo.RuleName,
			"severity":    analysisContext.AlertInfo.Severity,
			"triggerTime": time.Unix(analysisContext.AlertInfo.TriggerTime, 0).Format(time.RFC3339),
			"duration":    fmt.Sprintf("%d秒", analysisContext.AlertInfo.Duration),
			"labels":      analysisContext.AlertInfo.Labels,
		}
	}

	// 主要指标信息
	if analysisContext.PrimaryMetric != nil {
		metricInfo := map[string]interface{}{
			"metricName":  analysisContext.PrimaryMetric.MetricName,
			"metricType":  analysisContext.PrimaryMetric.MetricType,
			"unit":        analysisContext.PrimaryMetric.Unit,
			"description": analysisContext.PrimaryMetric.Description,
			"dataPoints":  len(analysisContext.PrimaryMetric.TimeSeries),
		}

		// 添加数据质量信息
		if analysisContext.PrimaryMetric.DataQuality != nil {
			metricInfo["dataQuality"] = map[string]interface{}{
				"completeness": analysisContext.PrimaryMetric.DataQuality.Completeness,
				"accuracy":     analysisContext.PrimaryMetric.DataQuality.Accuracy,
				"qualityScore": analysisContext.PrimaryMetric.DataQuality.QualityScore,
			}
		}

		serializable["primaryMetric"] = metricInfo
	}

	// 相关指标信息
	if len(analysisContext.RelatedMetrics) > 0 {
		relatedInfo := make([]map[string]interface{}, 0)
		for id, metric := range analysisContext.RelatedMetrics {
			relatedInfo = append(relatedInfo, map[string]interface{}{
				"id":         id,
				"metricName": metric.MetricName,
				"dataPoints": len(metric.TimeSeries),
			})
		}
		serializable["relatedMetrics"] = relatedInfo
	}

	// 系统上下文
	if analysisContext.SystemContext != nil {
		serializable["systemContext"] = map[string]interface{}{
			"environment": analysisContext.SystemContext.Environment,
			"region":      analysisContext.SystemContext.Region,
			"cluster":     analysisContext.SystemContext.Cluster,
			"serviceName": analysisContext.SystemContext.ServiceName,
			"serviceType": analysisContext.SystemContext.ServiceType,
			"labels":      analysisContext.SystemContext.Labels,
		}
	}

	// 时间上下文
	if analysisContext.TimeContext != nil {
		timeInfo := map[string]interface{}{
			"analysisTime":    time.Unix(analysisContext.TimeContext.AnalysisTime, 0).Format(time.RFC3339),
			"eventTime":       time.Unix(analysisContext.TimeContext.EventTime, 0).Format(time.RFC3339),
			"timeZone":        analysisContext.TimeContext.TimeZone,
			"isBusinessHours": analysisContext.TimeContext.IsBusinessHours,
		}

		// 添加时间范围信息
		if timeRange, exists := analysisContext.TimeContext.TimeRanges["current"]; exists {
			timeInfo["analysisWindow"] = map[string]interface{}{
				"startTime": time.Unix(timeRange.StartTime, 0).Format(time.RFC3339),
				"endTime":   time.Unix(timeRange.EndTime, 0).Format(time.RFC3339),
				"duration":  fmt.Sprintf("%.1f小时", float64(timeRange.Duration)/3600.0),
			}
		}

		serializable["timeContext"] = timeInfo
	}

	// 特征信息（如果存在）
	if analysisContext.MetricFeatures != nil {
		featuresInfo := make(map[string]interface{})

		// 统计特征摘要
		if analysisContext.MetricFeatures.StatisticalFeatures != nil {
			stats := analysisContext.MetricFeatures.StatisticalFeatures
			featuresInfo["statistical"] = map[string]interface{}{
				"mean":   fmt.Sprintf("%.2f", stats.Mean),
				"stdDev": fmt.Sprintf("%.2f", stats.StdDev),
				"min":    fmt.Sprintf("%.2f", stats.Min),
				"max":    fmt.Sprintf("%.2f", stats.Max),
				"p95":    fmt.Sprintf("%.2f", stats.P95),
				"p99":    fmt.Sprintf("%.2f", stats.P99),
			}
		}

		// 时序特征摘要
		if analysisContext.MetricFeatures.TimeSeriesFeatures != nil {
			timeSeries := analysisContext.MetricFeatures.TimeSeriesFeatures
			featuresInfo["timeseries"] = map[string]interface{}{
				"trendType":        timeSeries.Trend,
				"trendStrength":    fmt.Sprintf("%.2f", timeSeries.TrendStrength),
				"seasonality":      timeSeries.Seasonality,
				"changePointCount": len(timeSeries.ChangePoint),
				"volatility":       fmt.Sprintf("%.2f", timeSeries.Volatility),
			}
		}

		// 异常特征摘要
		if analysisContext.MetricFeatures.AnomalyFeatures != nil {
			anomaly := analysisContext.MetricFeatures.AnomalyFeatures
			featuresInfo["anomaly"] = map[string]interface{}{
				"hasAnomalies": anomaly.HasAnomalies,
				"anomalyCount": anomaly.AnomalyCount,
				"anomalyRatio": fmt.Sprintf("%.2f", anomaly.AnomalyRatio),
				"anomalyScore": fmt.Sprintf("%.2f", anomaly.AnomalyScore),
				"anomalyTypes": anomaly.AnomalyTypes,
			}
		}

		serializable["features"] = featuresInfo
	}

	// 序列化为JSON
	jsonData, err := json.MarshalIndent(serializable, "", "  ")
	if err != nil {
		return "", fmt.Errorf("JSON序列化失败: %w", err)
	}

	return string(jsonData), nil
}

// buildTemplateData 构建模板数据
func (dpg *DynamicPromptGenerator) buildTemplateData(
	analysisContext *models.UniversalAnalysisContext,
	request *AnalysisRequest,
	serializedContext string,
) map[string]interface{} {

	data := map[string]interface{}{
		"SerializedContext": serializedContext,
		"AnalysisDepth":     request.AnalysisDepth,
		"FocusAreas":        strings.Join(request.FocusAreas, ", "),
	}

	// 添加主要指标信息
	if analysisContext.PrimaryMetric != nil {
		data["PrimaryMetricName"] = analysisContext.PrimaryMetric.MetricName
		data["DataPoints"] = len(analysisContext.PrimaryMetric.TimeSeries)
	} else {
		data["PrimaryMetricName"] = "未知"
		data["DataPoints"] = 0
	}

	// 添加自定义参数
	for key, value := range request.CustomPrompts {
		data[key] = value
	}

	return data
}

// optimizePrompt 优化提示
func (dpg *DynamicPromptGenerator) optimizePrompt(ctx *ctx.Context, prompt string) string {
	// 简化的提示优化，主要是清理多余的空白字符
	lines := strings.Split(prompt, "\n")
	optimized := make([]string, 0)

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed != "" {
			optimized = append(optimized, trimmed)
		}
	}

	result := strings.Join(optimized, "\n")

	// 长度控制（简单的截断保护）
	maxLength := 8000 // 根据AI模型的上下文长度限制调整
	if len(result) > maxLength {
		logc.Infof(ctx.Ctx, "[提示优化] 提示长度超限，进行截断: %d > %d", len(result), maxLength)
		result = result[:maxLength-100] + "\n\n[提示已截断，请基于已有信息进行分析]"
	}

	return result
}

// GenerateAnalysisPromptWithInterface 支持接口类型的分析提示生成
// 这是为了兼容新的接口设计而添加的重载方法
func (dpg *DynamicPromptGenerator) GenerateAnalysisPromptWithInterface(
	ctx *ctx.Context,
	analysisContext *models.UniversalAnalysisContext,
	request *interfaces.AIAnalysisRequest,
) (string, error) {
	// 转换接口类型为内部类型
	internalRequest := &AnalysisRequest{
		AnalysisType:  request.AnalysisType,
		AnalysisMode:  request.AnalysisMode,
		AnalysisDepth: request.AnalysisDepth,
		FocusAreas:    request.FocusAreas,
		CustomPrompts: request.PromptParams,
	}

	// 调用原有方法
	return dpg.GenerateAnalysisPrompt(ctx, analysisContext, internalRequest)
}
