package metadata

import (
	"alertHub/internal/types"
	"fmt"
	"strings"
	"time"

	promLabels "github.com/prometheus/prometheus/model/labels"
	"github.com/prometheus/prometheus/promql/parser"
)

// MetadataExtractor 指标元数据提取器
// 负责从PromQL查询中提取完整的指标元数据信息
type MetadataExtractor struct {
	// 使用Prometheus官方解析器确保生产级别的准确性
	PrometheusParser *PrometheusQLParser
	MetricRegistry   *MetricRegistry
}

// NewMetadataExtractor 创建新的元数据提取器实例
func NewMetadataExtractor(parser *PrometheusQLParser, registry *MetricRegistry) *MetadataExtractor {
	return &MetadataExtractor{
		PrometheusParser: parser,
		MetricRegistry:   registry,
	}
}

// ExtractFromPromQL 从PromQL中提取指标元数据（使用Prometheus官方解析器）
// 这是主要的元数据提取接口，返回完整的指标元数据信息
func (me *MetadataExtractor) ExtractFromPromQL(promQL string) (*types.MetricMetadata, error) {
	// 1. 使用Prometheus官方解析器解析PromQL（生产级别）
	parsed, err := me.PrometheusParser.ParsePromQL(promQL)
	if err != nil {
		return nil, fmt.Errorf("解析PromQL失败: %w", err)
	}

	// 2. 提取核心指标名称
	metricName := me.extractPrimaryMetricName(parsed)

	// 3. 推断指标类型
	metricType := me.inferMetricType(metricName)

	// 4. 提取标签和选择器（使用正确的标签解析）
	labels := me.extractLabelsFromAST(parsed)

	// 5. 分析聚合和函数
	aggregation := me.analyzeAggregation(parsed)
	functions := me.extractFunctions(parsed)

	// 6. 分析依赖关系
	dependencies := me.extractDependencies(parsed)

	metadata := &types.MetricMetadata{
		MetricName:   metricName,
		MetricType:   metricType,
		PromQL:       promQL,
		ParsedLabels: labels,
		Aggregation:  aggregation,
		Functions:    functions,
		Dependencies: dependencies,
		Metadata: map[string]interface{}{
			"complexity": me.calculateComplexity(parsed),
			"parseTime":  time.Now().UnixMilli(),
		},
	}

	return metadata, nil
}

// extractPrimaryMetricName 从解析树中提取主要指标名称
// 遍历AST找到第一个向量选择器中的指标名称
func (me *MetadataExtractor) extractPrimaryMetricName(expr parser.Expr) string {
	var metricName string

	// 遍历解析树找到第一个指标选择器
	parser.Inspect(expr, func(node parser.Node, path []parser.Node) error {
		if vs, ok := node.(*parser.VectorSelector); ok && metricName == "" {
			metricName = vs.Name
		}
		return nil
	})

	return metricName
}

// extractLabelsFromAST 从AST中提取标签（生产级别准确性）
// 提取所有标签选择器中的等值匹配条件
func (me *MetadataExtractor) extractLabelsFromAST(expr parser.Expr) map[string]string {
	extractedLabels := make(map[string]string)

	// 遍历解析树提取所有标签选择器
	parser.Inspect(expr, func(node parser.Node, path []parser.Node) error {
		if vs, ok := node.(*parser.VectorSelector); ok {
			for _, matcher := range vs.LabelMatchers {
				if matcher.Type == promLabels.MatchEqual {
					extractedLabels[matcher.Name] = matcher.Value
				}
			}
		}
		return nil
	})

	return extractedLabels
}

// analyzeAggregation 分析聚合函数
// 提取聚合操作的详细信息，包括聚合函数、分组方式等
func (me *MetadataExtractor) analyzeAggregation(expr parser.Expr) *types.AggregationInfo {
	var aggregation *types.AggregationInfo

	parser.Inspect(expr, func(node parser.Node, path []parser.Node) error {
		if agg, ok := node.(*parser.AggregateExpr); ok {
			groupBy := make([]string, 0)
			without := make([]string, 0)

			if agg.Grouping != nil {
				for _, label := range agg.Grouping {
					if agg.Without {
						without = append(without, label)
					} else {
						groupBy = append(groupBy, label)
					}
				}
			}

			aggregation = &types.AggregationInfo{
				Function: agg.Op.String(),
				GroupBy:  groupBy,
				Without:  without,
			}
		}
		return nil
	})

	return aggregation
}

// extractFunctions 提取函数列表
// 识别PromQL中使用的所有函数和聚合操作
func (me *MetadataExtractor) extractFunctions(expr parser.Expr) []string {
	functions := make([]string, 0)

	parser.Inspect(expr, func(node parser.Node, path []parser.Node) error {
		switch n := node.(type) {
		case *parser.Call:
			functions = append(functions, n.Func.Name)
		case *parser.AggregateExpr:
			functions = append(functions, n.Op.String())
		}
		return nil
	})

	return functions
}

// extractDependencies 提取依赖的其他指标
// 识别查询中引用的所有指标名称，用于依赖分析
func (me *MetadataExtractor) extractDependencies(expr parser.Expr) []string {
	dependencies := make([]string, 0)
	seen := make(map[string]bool)

	parser.Inspect(expr, func(node parser.Node, path []parser.Node) error {
		if vs, ok := node.(*parser.VectorSelector); ok {
			if vs.Name != "" && !seen[vs.Name] {
				dependencies = append(dependencies, vs.Name)
				seen[vs.Name] = true
			}
		}
		return nil
	})

	return dependencies
}

// calculateComplexity 计算PromQL复杂度
// 基于AST中不同节点类型的权重计算查询复杂度
func (me *MetadataExtractor) calculateComplexity(expr parser.Expr) int {
	complexity := 0

	parser.Inspect(expr, func(node parser.Node, path []parser.Node) error {
		switch node.(type) {
		case *parser.Call:
			complexity += 2
		case *parser.AggregateExpr:
			complexity += 3
		case *parser.BinaryExpr:
			complexity += 1
		case *parser.VectorSelector:
			complexity += 1
		}
		return nil
	})

	return complexity
}

// inferMetricType 推断指标类型
// 基于Prometheus指标命名最佳实践推断指标类型
func (me *MetadataExtractor) inferMetricType(metricName string) string {
	// 基于Prometheus指标命名最佳实践推断类型
	if strings.HasSuffix(metricName, "_total") || strings.HasSuffix(metricName, "_count") {
		return "counter"
	}
	if strings.Contains(metricName, "_bucket") || strings.Contains(metricName, "_histogram") {
		return "histogram"
	}
	if strings.Contains(metricName, "_summary") {
		return "summary"
	}
	return "gauge" // 默认类型
}

// ExtractQueryPattern 提取查询模式
// 分析PromQL的查询模式，用于优化和缓存
func (me *MetadataExtractor) ExtractQueryPattern(promQL string) (*types.QueryPattern, error) {
	parsed, err := me.PrometheusParser.ParsePromQL(promQL)
	if err != nil {
		return nil, err
	}

	pattern := &types.QueryPattern{
		HasAggregation: false,
		HasFunctions:   false,
		HasBinaryOps:   false,
		HasSubqueries:  false,
		Complexity:     me.calculateComplexity(parsed),
	}

	parser.Inspect(parsed, func(node parser.Node, path []parser.Node) error {
		switch node.(type) {
		case *parser.AggregateExpr:
			pattern.HasAggregation = true
		case *parser.Call:
			pattern.HasFunctions = true
		case *parser.BinaryExpr:
			pattern.HasBinaryOps = true
		case *parser.SubqueryExpr:
			pattern.HasSubqueries = true
		}
		return nil
	})

	return pattern, nil
}

// ValidateMetadata 验证提取的元数据完整性
// 确保提取的元数据包含必要的信息
func (me *MetadataExtractor) ValidateMetadata(metadata *types.MetricMetadata) []string {
	issues := make([]string, 0)

	if metadata.MetricName == "" {
		issues = append(issues, "缺少指标名称")
	}

	if metadata.PromQL == "" {
		issues = append(issues, "缺少PromQL查询")
	}

	if len(metadata.Dependencies) == 0 && metadata.MetricName != "" {
		issues = append(issues, "未检测到任何指标依赖")
	}

	if complexity, ok := metadata.Metadata["complexity"].(int); ok && complexity > 10 {
		issues = append(issues, fmt.Sprintf("查询复杂度过高: %d", complexity))
	}

	return issues
}