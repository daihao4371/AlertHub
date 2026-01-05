package metadata

import (
	"alertHub/internal/ctx"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"github.com/zeromicro/go-zero/core/logc"
)

// MetricMetadata 指标元数据
type MetricMetadata struct {
	MetricName  string            `json:"metricName"`
	MetricType  string            `json:"metricType"`
	Unit        string            `json:"unit"`
	Description string            `json:"description"`
	ServiceType string            `json:"serviceType"`
	Labels      map[string]string `json:"labels"`
	Extensions  map[string]interface{} `json:"extensions"`
}

// ExtractionConfig 提取配置
type ExtractionConfig struct {
	// 指标名称提取规则
	MetricNamePatterns []PatternRule `json:"metricNamePatterns"`
	
	// 指标类型推断规则
	TypeInferenceRules []TypeRule `json:"typeInferenceRules"`
	
	// 单位推断规则
	UnitInferenceRules []UnitRule `json:"unitInferenceRules"`
	
	// 描述生成规则
	DescriptionRules []DescriptionRule `json:"descriptionRules"`
	
	// 服务类型推断规则
	ServiceTypeRules []ServiceTypeRule `json:"serviceTypeRules"`
	
	// 默认值配置
	DefaultValues DefaultMetadata `json:"defaultValues"`
}

// PatternRule 模式规则
type PatternRule struct {
	Name     string   `json:"name"`
	Patterns []string `json:"patterns"`
	Priority int      `json:"priority"`
	Enabled  bool     `json:"enabled"`
}

// TypeRule 类型推断规则
type TypeRule struct {
	Name        string            `json:"name"`
	Conditions  []ConditionRule   `json:"conditions"`
	MetricType  string            `json:"metricType"`
	Priority    int               `json:"priority"`
	Enabled     bool              `json:"enabled"`
	Metadata    map[string]interface{} `json:"metadata"`
}

// UnitRule 单位推断规则
type UnitRule struct {
	Name       string          `json:"name"`
	Conditions []ConditionRule `json:"conditions"`
	Unit       string          `json:"unit"`
	Priority   int             `json:"priority"`
	Enabled    bool            `json:"enabled"`
}

// DescriptionRule 描述生成规则
type DescriptionRule struct {
	Name        string          `json:"name"`
	Conditions  []ConditionRule `json:"conditions"`
	Template    string          `json:"template"`
	Priority    int             `json:"priority"`
	Enabled     bool            `json:"enabled"`
}

// ServiceTypeRule 服务类型推断规则
type ServiceTypeRule struct {
	Name        string          `json:"name"`
	Conditions  []ConditionRule `json:"conditions"`
	ServiceType string          `json:"serviceType"`
	Priority    int             `json:"priority"`
	Enabled     bool            `json:"enabled"`
}

// ConditionRule 条件规则
type ConditionRule struct {
	Type     string      `json:"type"`     // metric_name, label, regex, contains
	Field    string      `json:"field"`    // 字段名
	Operator string      `json:"operator"` // equals, contains, matches, starts_with, ends_with
	Value    interface{} `json:"value"`    // 匹配值
}

// DefaultMetadata 默认元数据
type DefaultMetadata struct {
	MetricType  string `json:"metricType"`
	Unit        string `json:"unit"`
	Description string `json:"description"`
	ServiceType string `json:"serviceType"`
}

// MetadataExtractor 完全配置驱动的元数据提取器
type MetadataExtractor struct {
	config *ExtractionConfig
}

// NewMetadataExtractor 创建元数据提取器
func NewMetadataExtractor() *MetadataExtractor {
	return &MetadataExtractor{
		config: getDefaultConfig(),
	}
}

// NewMetadataExtractorWithConfig 创建带配置的元数据提取器
func NewMetadataExtractorWithConfig(config *ExtractionConfig) *MetadataExtractor {
	if config == nil {
		config = getDefaultConfig()
	}
	return &MetadataExtractor{
		config: config,
	}
}

// SetConfig 设置配置
func (me *MetadataExtractor) SetConfig(config *ExtractionConfig) {
	if config != nil {
		me.config = config
	}
}

// LoadConfigFromJSON 从JSON加载配置
func (me *MetadataExtractor) LoadConfigFromJSON(jsonData []byte) error {
	config := &ExtractionConfig{}
	if err := json.Unmarshal(jsonData, config); err != nil {
		return fmt.Errorf("解析配置JSON失败: %w", err)
	}
	me.config = config
	return nil
}

// ExtractFromPromQL 完全配置驱动的元数据提取
func (me *MetadataExtractor) ExtractFromPromQL(ctx *ctx.Context, promQL string, labels map[string]string) (*MetricMetadata, error) {
	logc.Infof(ctx.Ctx, "[元数据提取] 开始配置驱动提取: promQL=%s", promQL)
	
	// 1. 提取指标名称（基于配置的模式规则）
	metricName := me.extractMetricNameByConfig(promQL)
	
	// 2. 基于配置推断指标类型
	metricType := me.inferTypeByConfig(metricName, labels)
	
	// 3. 基于配置推断单位
	unit := me.inferUnitByConfig(metricName, labels)
	
	// 4. 基于配置生成描述
	description := me.generateDescriptionByConfig(metricName, labels)
	
	// 5. 基于配置推断服务类型
	serviceType := me.inferServiceTypeByConfig(metricName, labels)
	
	metadata := &MetricMetadata{
		MetricName:  metricName,
		MetricType:  metricType,
		Unit:        unit,
		Description: description,
		ServiceType: serviceType,
		Labels:      labels,
		Extensions:  make(map[string]interface{}),
	}
	
	logc.Infof(ctx.Ctx, "[元数据提取] 完成: metric=%s, type=%s, unit=%s, serviceType=%s", 
		metricName, metricType, unit, serviceType)
	
	return metadata, nil
}

// extractMetricNameByConfig 基于配置提取指标名称
func (me *MetadataExtractor) extractMetricNameByConfig(promQL string) string {
	// 按优先级排序规则
	patterns := me.getSortedPatternRules(me.config.MetricNamePatterns)
	
	for _, rule := range patterns {
		if !rule.Enabled {
			continue
		}
		
		for _, pattern := range rule.Patterns {
			re, err := regexp.Compile(pattern)
			if err != nil {
				continue
			}
			
			matches := re.FindStringSubmatch(promQL)
			if len(matches) > 1 {
				return matches[1]
			}
		}
	}
	
	// 使用默认值
	return "unknown"
}

// inferTypeByConfig 基于配置推断指标类型
func (me *MetadataExtractor) inferTypeByConfig(metricName string, labels map[string]string) string {
	// 按优先级排序规则
	rules := me.getSortedTypeRules(me.config.TypeInferenceRules)
	
	for _, rule := range rules {
		if !rule.Enabled {
			continue
		}
		
		if me.evaluateConditions(rule.Conditions, metricName, labels) {
			return rule.MetricType
		}
	}
	
	// 使用默认值
	return me.config.DefaultValues.MetricType
}

// inferUnitByConfig 基于配置推断单位
func (me *MetadataExtractor) inferUnitByConfig(metricName string, labels map[string]string) string {
	rules := me.getSortedUnitRules(me.config.UnitInferenceRules)
	
	for _, rule := range rules {
		if !rule.Enabled {
			continue
		}
		
		if me.evaluateConditions(rule.Conditions, metricName, labels) {
			return rule.Unit
		}
	}
	
	return me.config.DefaultValues.Unit
}

// generateDescriptionByConfig 基于配置生成描述
func (me *MetadataExtractor) generateDescriptionByConfig(metricName string, labels map[string]string) string {
	rules := me.getSortedDescriptionRules(me.config.DescriptionRules)
	
	for _, rule := range rules {
		if !rule.Enabled {
			continue
		}
		
		if me.evaluateConditions(rule.Conditions, metricName, labels) {
			return me.renderTemplate(rule.Template, metricName, labels)
		}
	}
	
	return me.config.DefaultValues.Description
}

// inferServiceTypeByConfig 基于配置推断服务类型
func (me *MetadataExtractor) inferServiceTypeByConfig(metricName string, labels map[string]string) string {
	rules := me.getSortedServiceTypeRules(me.config.ServiceTypeRules)
	
	for _, rule := range rules {
		if !rule.Enabled {
			continue
		}
		
		if me.evaluateConditions(rule.Conditions, metricName, labels) {
			return rule.ServiceType
		}
	}
	
	return me.config.DefaultValues.ServiceType
}

// evaluateConditions 评估条件规则
func (me *MetadataExtractor) evaluateConditions(conditions []ConditionRule, metricName string, labels map[string]string) bool {
	for _, condition := range conditions {
		if !me.evaluateCondition(condition, metricName, labels) {
			return false
		}
	}
	return len(conditions) > 0
}

// evaluateCondition 评估单个条件
func (me *MetadataExtractor) evaluateCondition(condition ConditionRule, metricName string, labels map[string]string) bool {
	var target string
	
	// 获取目标值
	switch condition.Type {
	case "metric_name":
		target = metricName
	case "label":
		if labelValue, exists := labels[condition.Field]; exists {
			target = labelValue
		} else {
			return false
		}
	default:
		return false
	}
	
	// 评估条件
	valueStr := fmt.Sprintf("%v", condition.Value)
	
	switch condition.Operator {
	case "equals":
		return target == valueStr
	case "contains":
		return strings.Contains(strings.ToLower(target), strings.ToLower(valueStr))
	case "matches":
		if re, err := regexp.Compile(valueStr); err == nil {
			return re.MatchString(target)
		}
		return false
	case "starts_with":
		return strings.HasPrefix(strings.ToLower(target), strings.ToLower(valueStr))
	case "ends_with":
		return strings.HasSuffix(strings.ToLower(target), strings.ToLower(valueStr))
	default:
		return false
	}
}

// renderTemplate 渲染模板
func (me *MetadataExtractor) renderTemplate(template string, metricName string, labels map[string]string) string {
	// 简单的模板替换
	result := template
	result = strings.ReplaceAll(result, "{{.MetricName}}", metricName)
	
	// 替换标签变量
	for key, value := range labels {
		placeholder := fmt.Sprintf("{{.Labels.%s}}", key)
		result = strings.ReplaceAll(result, placeholder, value)
	}
	
	return result
}

// 排序方法
func (me *MetadataExtractor) getSortedPatternRules(rules []PatternRule) []PatternRule {
	// 按优先级从高到低排序
	sorted := make([]PatternRule, len(rules))
	copy(sorted, rules)
	
	for i := 0; i < len(sorted)-1; i++ {
		for j := 0; j < len(sorted)-1-i; j++ {
			if sorted[j].Priority < sorted[j+1].Priority {
				sorted[j], sorted[j+1] = sorted[j+1], sorted[j]
			}
		}
	}
	
	return sorted
}

func (me *MetadataExtractor) getSortedTypeRules(rules []TypeRule) []TypeRule {
	sorted := make([]TypeRule, len(rules))
	copy(sorted, rules)
	
	for i := 0; i < len(sorted)-1; i++ {
		for j := 0; j < len(sorted)-1-i; j++ {
			if sorted[j].Priority < sorted[j+1].Priority {
				sorted[j], sorted[j+1] = sorted[j+1], sorted[j]
			}
		}
	}
	
	return sorted
}

func (me *MetadataExtractor) getSortedUnitRules(rules []UnitRule) []UnitRule {
	sorted := make([]UnitRule, len(rules))
	copy(sorted, rules)
	
	for i := 0; i < len(sorted)-1; i++ {
		for j := 0; j < len(sorted)-1-i; j++ {
			if sorted[j].Priority < sorted[j+1].Priority {
				sorted[j], sorted[j+1] = sorted[j+1], sorted[j]
			}
		}
	}
	
	return sorted
}

func (me *MetadataExtractor) getSortedDescriptionRules(rules []DescriptionRule) []DescriptionRule {
	sorted := make([]DescriptionRule, len(rules))
	copy(sorted, rules)
	
	for i := 0; i < len(sorted)-1; i++ {
		for j := 0; j < len(sorted)-1-i; j++ {
			if sorted[j].Priority < sorted[j+1].Priority {
				sorted[j], sorted[j+1] = sorted[j+1], sorted[j]
			}
		}
	}
	
	return sorted
}

func (me *MetadataExtractor) getSortedServiceTypeRules(rules []ServiceTypeRule) []ServiceTypeRule {
	sorted := make([]ServiceTypeRule, len(rules))
	copy(sorted, rules)
	
	for i := 0; i < len(sorted)-1; i++ {
		for j := 0; j < len(sorted)-1-i; j++ {
			if sorted[j].Priority < sorted[j+1].Priority {
				sorted[j], sorted[j+1] = sorted[j+1], sorted[j]
			}
		}
	}
	
	return sorted
}

// getDefaultConfig 获取默认配置（纯数据驱动，无硬编码逻辑）
func getDefaultConfig() *ExtractionConfig {
	return &ExtractionConfig{
		MetricNamePatterns: []PatternRule{
			{
				Name:     "standard_metric_pattern",
				Patterns: []string{`([a-zA-Z_:][a-zA-Z0-9_:]*)`},
				Priority: 100,
				Enabled:  true,
			},
		},
		TypeInferenceRules: []TypeRule{},
		UnitInferenceRules: []UnitRule{},
		DescriptionRules:   []DescriptionRule{},
		ServiceTypeRules:   []ServiceTypeRule{},
		DefaultValues: DefaultMetadata{
			MetricType:  "gauge",
			Unit:        "",
			Description: "自动生成的指标描述",
			ServiceType: "application",
		},
	}
}

// ToMap 转换为map
func (m *MetricMetadata) ToMap() map[string]interface{} {
	return map[string]interface{}{
		"metric_name":  m.MetricName,
		"metric_type":  m.MetricType,
		"unit":         m.Unit,
		"description":  m.Description,
		"service_type": m.ServiceType,
		"labels":       m.Labels,
		"extensions":   m.Extensions,
	}
}
