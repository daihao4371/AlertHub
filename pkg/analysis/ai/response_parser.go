package ai

import (
	"alertHub/internal/ctx"
	"alertHub/pkg/analysis/interfaces"
	"encoding/json"
	"strings"

	"github.com/zeromicro/go-zero/core/logc"
)

// UniversalResponseParser 通用响应解析器
type UniversalResponseParser struct{}

// NewUniversalResponseParser 创建通用响应解析器
func NewUniversalResponseParser() *UniversalResponseParser {
	return &UniversalResponseParser{}
}

// Parse 解析AI响应
func (urp *UniversalResponseParser) Parse(ctx *ctx.Context, rawResponse string, analysisID string) (*AnalysisResult, error) {
	// 尝试解析JSON格式的响应
	var result AnalysisResult
	result.AnalysisID = analysisID

	// 清理响应字符串（移除可能的markdown代码块标记）
	cleanedResponse := urp.cleanResponse(rawResponse)

	// 尝试解析为JSON
	if err := json.Unmarshal([]byte(cleanedResponse), &result); err != nil {
		// 如果不是JSON格式，尝试提取JSON部分
		jsonPart := urp.extractJSON(cleanedResponse)
		if jsonPart != "" {
			if err := json.Unmarshal([]byte(jsonPart), &result); err != nil {
				logc.Infof(ctx.Ctx, "[响应解析] JSON解析失败，使用文本解析: %v", err)
				return urp.parseAsText(ctx, cleanedResponse, analysisID)
			}
		} else {
			// 完全无法解析为JSON，使用文本解析
			return urp.parseAsText(ctx, cleanedResponse, analysisID)
		}
	}

	return &result, nil
}

// cleanResponse 清理响应字符串
func (urp *UniversalResponseParser) cleanResponse(response string) string {
	// 移除markdown代码块标记
	response = strings.TrimSpace(response)
	if strings.HasPrefix(response, "```json") {
		response = strings.TrimPrefix(response, "```json")
		response = strings.TrimSuffix(response, "```")
	} else if strings.HasPrefix(response, "```") {
		response = strings.TrimPrefix(response, "```")
		response = strings.TrimSuffix(response, "```")
	}
	return strings.TrimSpace(response)
}

// extractJSON 从文本中提取JSON部分
func (urp *UniversalResponseParser) extractJSON(text string) string {
	// 查找第一个 { 和最后一个 }
	start := strings.Index(text, "{")
	end := strings.LastIndex(text, "}")
	if start != -1 && end != -1 && end > start {
		return text[start : end+1]
	}
	return ""
}

// parseAsText 将响应解析为文本格式
func (urp *UniversalResponseParser) parseAsText(ctx *ctx.Context, text string, analysisID string) (*AnalysisResult, error) {
	// 简单的文本解析，将整个响应作为摘要
	result := &AnalysisResult{
		AnalysisID: analysisID,
		Summary: &AnalysisSummary{
			Title:       "AI分析结果",
			Description: text,
			Severity:    "medium",
			Category:    "general",
			Confidence:  0.5,
		},
		ConfidenceScore: 0.5,
	}

	return result, nil
}

// ParseWithInterface 解析AI响应并返回接口类型
// 这是为了兼容新的接口设计而添加的重载方法
func (urp *UniversalResponseParser) ParseWithInterface(ctx *ctx.Context, rawResponse string, analysisID string) (*interfaces.AIAnalysisResult, error) {
	// 先调用原有的Parse方法
	internalResult, err := urp.Parse(ctx, rawResponse, analysisID)
	if err != nil {
		return nil, err
	}

	// 转换为接口类型
	interfaceResult := &interfaces.AIAnalysisResult{
		AnalysisID:      internalResult.AnalysisID,
		AnalysisType:    "auto",
		Timestamp:       internalResult.ProcessingTime,
		ConfidenceScore: internalResult.ConfidenceScore,

		// 转换Summary
		Summary: &interfaces.AnalysisSummary{
			Title:       internalResult.Summary.Title,
			Description: internalResult.Summary.Description,
			Severity:    internalResult.Summary.Severity,
			Category:    internalResult.Summary.Category,
			Confidence:  internalResult.Summary.Confidence,
			KeyFindings: internalResult.Summary.KeyFindings,
		},

		// 转换DataAnalysis
		DataAnalysis: &interfaces.DataAnalysisResult{
			PrimaryMetricAnalysis: make(map[string]interface{}),
			RelationshipAnalysis:  make(map[string]interface{}),
			SystemAnalysis:        make(map[string]interface{}),
			TrendAnalysis:         make(map[string]interface{}),
			AnomalyAnalysis:       make(map[string]interface{}),
		},

		// 转换RootCauseAnalysis
		RootCauseAnalysis: &interfaces.RootCauseAnalysis{
			PrimaryHypothesis:     internalResult.RootCauseAnalysis.PrimaryHypothesis,
			AlternativeHypotheses: internalResult.RootCauseAnalysis.AlternativeHypotheses,
			Confidence:            internalResult.RootCauseAnalysis.Confidence,
		},

		// 转换Recommendations
		Recommendations: make([]*interfaces.Recommendation, 0),

		// 基础字段
		ProcessingTime: internalResult.ProcessingTime,
		TokenUsage:     &interfaces.TokenUsage{},
		ModelInfo:      &interfaces.ModelInfo{},
		Metadata:       make(map[string]interface{}),
	}

	// 转换DataAnalysis
	if internalResult.DataAnalysis != nil {
		if internalResult.DataAnalysis.PrimaryMetricAnalysis != nil {
			interfaceResult.DataAnalysis.PrimaryMetricAnalysis["metricName"] = internalResult.DataAnalysis.PrimaryMetricAnalysis.MetricName
			interfaceResult.DataAnalysis.PrimaryMetricAnalysis["dataPoints"] = internalResult.DataAnalysis.PrimaryMetricAnalysis.DataPoints
			interfaceResult.DataAnalysis.PrimaryMetricAnalysis["statisticalFeatures"] = internalResult.DataAnalysis.PrimaryMetricAnalysis.StatisticalFeatures
			interfaceResult.DataAnalysis.PrimaryMetricAnalysis["trendAnalysis"] = internalResult.DataAnalysis.PrimaryMetricAnalysis.TrendAnalysis
		}
		if internalResult.DataAnalysis.RelationshipAnalysis != nil {
			interfaceResult.DataAnalysis.RelationshipAnalysis["causalChain"] = internalResult.DataAnalysis.RelationshipAnalysis.CausalChain
			interfaceResult.DataAnalysis.RelationshipAnalysis["impactScope"] = internalResult.DataAnalysis.RelationshipAnalysis.ImpactScope
		}
	}

	// 转换Recommendations
	for _, rec := range internalResult.ActionRecommendations {
		interfaceRec := &interfaces.Recommendation{
			Priority:        rec.Priority,
			Type:            rec.Type,
			Title:           rec.Title,
			Description:     "", // 使用Rationale作为Description
			Rationale:       rec.Rationale,
			Steps:           make([]*interfaces.ActionStep, 0),
			ExpectedOutcome: "",
			RiskAssessment:  rec.RiskAssessment,
			SuccessMetrics:  rec.SuccessMetrics,
		}

		// 转换Steps
		for _, step := range rec.Steps {
			interfaceStep := &interfaces.ActionStep{
				Order:             step.Order,
				Action:            step.Action,
				Verification:      step.Verification,
				ExpectedOutcome:   step.ExpectedOutcome,
				EstimatedDuration: "", // 内部类型没有这个字段
			}
			interfaceRec.Steps = append(interfaceRec.Steps, interfaceStep)
		}

		interfaceResult.Recommendations = append(interfaceResult.Recommendations, interfaceRec)
	}

	return interfaceResult, nil
}
