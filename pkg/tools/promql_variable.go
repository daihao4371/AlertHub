package tools

import (
	"fmt"
	"regexp"
	"strings"
)

// ReplacePromQLVariables 替换 PromQL 查询语句中的变量
// 支持 $variable 格式的变量替换
// 例如: $instance -> variables["instance"] 的值
// 如果变量不存在，可以选择保留原样或替换为通配符
func ReplacePromQLVariables(query string, variables map[string]string, useWildcard bool) string {
	if len(variables) == 0 && !useWildcard {
		return query
	}

	// 使用正则表达式匹配 $variable 格式的变量
	re := regexp.MustCompile(`\$([a-zA-Z_][a-zA-Z0-9_]*)`)

	return re.ReplaceAllStringFunc(query, func(match string) string {
		// 提取变量名（去掉 $ 符号）
		varName := match[1:]
		if value, ok := variables[varName]; ok {
			return value
		}
		// 如果变量不存在且 useWildcard 为 true，替换为通配符（查询所有值）
		if useWildcard {
			// 在 PromQL 中，使用 =~".+" 来匹配所有值
			return `".+"`
		}
		// 如果变量不存在且 useWildcard 为 false，保留原样（不替换）
		return match
	})
}

// ExtractVariablesFromPromQL 从 PromQL 查询语句中提取变量名
// 返回所有 $variable 格式的变量名列表
func ExtractVariablesFromPromQL(query string) []string {
	re := regexp.MustCompile(`\$([a-zA-Z_][a-zA-Z0-9_]*)`)
	matches := re.FindAllStringSubmatch(query, -1)
	
	variables := make([]string, 0)
	seen := make(map[string]bool)
	
	for _, match := range matches {
		if len(match) >= 2 {
			varName := match[1]
			if !seen[varName] {
				variables = append(variables, varName)
				seen[varName] = true
			}
		}
	}
	
	return variables
}

// ReplacePromQLVariablesForAlert 为告警规则替换 PromQL 变量
// 告警规则执行时，如果 PromQL 包含 $instance 或 $ifName 等变量，应该查询所有匹配的指标
// 因此将变量替换为通配符模式：instance=~".+" 或 ifName=~".+"
func ReplacePromQLVariablesForAlert(query string, variables map[string]string) string {
	// 检查查询中是否包含变量
	hasVariables := strings.Contains(query, "$")
	if !hasVariables {
		return query
	}
	
	// 如果提供了变量值，使用变量值
	if len(variables) > 0 {
		return ReplacePromQLVariables(query, variables, false)
	}
	
	// 如果没有提供变量值，将变量替换为通配符模式
	// 例如: {instance="$instance",ifName="$ifName"} -> {instance=~".+",ifName=~".+"}
	re := regexp.MustCompile(`(\w+)=\\"\$(\w+)\\"`)
	return re.ReplaceAllStringFunc(query, func(match string) string {
		// 提取 label 名称和变量名
		parts := re.FindStringSubmatch(match)
		if len(parts) >= 3 {
			labelName := parts[1]
			// 替换为通配符模式
			return fmt.Sprintf(`%s=~".+"`, labelName)
		}
		return match
	})
}
