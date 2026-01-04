package metadata

import (
	"fmt"

	"github.com/prometheus/prometheus/promql/parser"
)

// PrometheusQLParser Prometheus官方解析器封装
// 负责解析PromQL查询语句，确保与Prometheus完全一致的解析行为
type PrometheusQLParser struct{}

// NewPrometheusQLParser 创建新的PromQL解析器实例
func NewPrometheusQLParser() *PrometheusQLParser {
	return &PrometheusQLParser{}
}

// ParsePromQL 使用Prometheus官方解析器解析PromQL（确保生产级别准确性）
// 这个方法是整个元数据提取的入口点，任何PromQL解析错误都会在这里被捕获
func (p *PrometheusQLParser) ParsePromQL(promQL string) (parser.Expr, error) {
	// 使用Prometheus官方解析器 - 确保与Prometheus完全一致
	expr, err := parser.ParseExpr(promQL)
	if err != nil {
		return nil, fmt.Errorf("Prometheus官方解析器解析失败: %w", err)
	}

	return expr, nil
}

// ValidatePromQL 验证PromQL语句的语法正确性
// 这是一个轻量级的验证方法，不进行完整解析，只检查语法
func (p *PrometheusQLParser) ValidatePromQL(promQL string) error {
	_, err := parser.ParseExpr(promQL)
	if err != nil {
		return fmt.Errorf("PromQL语法验证失败: %w", err)
	}
	return nil
}

// GetQueryType 分析PromQL查询的类型（即时查询 vs 范围查询）
// 基于解析后的AST判断查询特征
func (p *PrometheusQLParser) GetQueryType(promQL string) (string, error) {
	expr, err := p.ParsePromQL(promQL)
	if err != nil {
		return "", err
	}

	// 检查是否包含时间范围选择器
	hasRangeSelector := false
	parser.Inspect(expr, func(node parser.Node, path []parser.Node) error {
		if _, ok := node.(*parser.MatrixSelector); ok {
			hasRangeSelector = true
		}
		return nil
	})

	if hasRangeSelector {
		return "range_query", nil
	}
	return "instant_query", nil
}

// ExtractMetricNames 从PromQL中提取所有涉及的指标名称
// 返回去重后的指标名称列表，用于后续的关联分析
func (p *PrometheusQLParser) ExtractMetricNames(promQL string) ([]string, error) {
	expr, err := p.ParsePromQL(promQL)
	if err != nil {
		return nil, err
	}

	metricNames := make([]string, 0)
	seen := make(map[string]bool)

	parser.Inspect(expr, func(node parser.Node, path []parser.Node) error {
		if vs, ok := node.(*parser.VectorSelector); ok {
			if vs.Name != "" && !seen[vs.Name] {
				metricNames = append(metricNames, vs.Name)
				seen[vs.Name] = true
			}
		}
		return nil
	})

	return metricNames, nil
}