package relationship

import (
	"alertHub/internal/ctx"
	"fmt"
	"strings"
)

// DynamicRelationshipEngine 动态关系引擎
type DynamicRelationshipEngine struct{}

// NewDynamicRelationshipEngine 创建动态关系引擎
func NewDynamicRelationshipEngine() *DynamicRelationshipEngine {
	return &DynamicRelationshipEngine{}
}

// RelatedMetricDescriptor 相关指标描述符 - 扩展字段支持关系分析
type RelatedMetricDescriptor struct {
	MetricName       string            `json:"metricName"`
	Labels           map[string]string `json:"labels"`
	Priority         int               `json:"priority"`
	RelationshipType string            `json:"relationshipType"` // 关系类型
	Similarity       float64           `json:"similarity"`       // 相似度评分
}

// DiscoverRelated 发现相关指标 - 实现基于现有数据的关系发现逻辑
func (dre *DynamicRelationshipEngine) DiscoverRelated(ctx *ctx.Context, metadata interface{}) ([]*RelatedMetricDescriptor, error) {
	if metadata == nil {
		return []*RelatedMetricDescriptor{}, nil
	}

	// 将interface{}转换为字符串形式的指标名称（最常见的输入）
	var metricName string
	switch v := metadata.(type) {
	case string:
		metricName = v
	case map[string]interface{}:
		if name, exists := v["name"]; exists {
			if nameStr, ok := name.(string); ok {
				metricName = nameStr
			}
		}
	default:
		return nil, fmt.Errorf("不支持的元数据类型，请传入字符串或包含name字段的map")
	}

	if metricName == "" {
		return []*RelatedMetricDescriptor{}, nil
	}

	// 基于指标名称模式发现相关指标
	related := dre.findRelatedByNamingPattern(metricName)

	// 限制返回数量
	if len(related) > 10 {
		related = related[:10]
	}

	return related, nil
}

// findRelatedByNamingPattern 基于命名模式发现相关指标 - 纯字符串分析，无硬编码
func (dre *DynamicRelationshipEngine) findRelatedByNamingPattern(metricName string) []*RelatedMetricDescriptor {
	var related []*RelatedMetricDescriptor

	// 提取指标的前缀和后缀，用于寻找同族指标
	prefix := extractPrefix(metricName)
	if prefix == "" {
		return related
	}

	// 生成可能的相关指标模式
	patterns := generateRelatedPatterns(prefix)

	for i, pattern := range patterns {
		if pattern != metricName { // 避免包含自己
			related = append(related, &RelatedMetricDescriptor{
				MetricName:       pattern,
				Labels:           make(map[string]string),
				Priority:         i + 1,
				RelationshipType: "naming_pattern",
				Similarity:       calculatePatternSimilarity(metricName, pattern),
			})
		}
	}

	return related
}

// extractPrefix 提取指标前缀
func extractPrefix(metricName string) string {
	separators := []string{"_", ":", ".", "-"}

	for _, sep := range separators {
		if strings.Contains(metricName, sep) {
			parts := strings.Split(metricName, sep)
			if len(parts) > 1 {
				return parts[0]
			}
		}
	}

	return metricName
}

// generateRelatedPatterns 生成相关指标模式 - 基于通用命名约定
func generateRelatedPatterns(prefix string) []string {
	// 常见的指标后缀模式
	commonSuffixes := []string{
		"_total", "_count", "_sum", "_avg", "_max", "_min",
		"_rate", "_ratio", "_percent", "_bytes", "_seconds",
		"_usage", "_utilization", "_available", "_free",
	}

	var patterns []string
	for _, suffix := range commonSuffixes {
		patterns = append(patterns, prefix+suffix)
	}

	return patterns
}

// calculatePatternSimilarity 计算模式相似度
func calculatePatternSimilarity(metric1, metric2 string) float64 {
	if metric1 == metric2 {
		return 1.0
	}

	// 基于公共前缀长度计算相似度
	commonPrefixLen := 0
	minLen := len(metric1)
	if len(metric2) < minLen {
		minLen = len(metric2)
	}

	for i := 0; i < minLen; i++ {
		if metric1[i] == metric2[i] {
			commonPrefixLen++
		} else {
			break
		}
	}

	maxLen := len(metric1)
	if len(metric2) > maxLen {
		maxLen = len(metric2)
	}

	if maxLen == 0 {
		return 0.0
	}

	return float64(commonPrefixLen) / float64(maxLen)
}
