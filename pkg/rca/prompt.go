package rca

// SystemPrompt AI 告警聚类的系统提示词
// 对标 Keep: keep/api/bl/ai_suggestion_bl.py:369-388
// 中文场景优化，定义 AI 的角色和分析维度
// 包含明确的 JSON 输出示例，确保不支持 strict JSON schema 的模型也能返回正确格式
const SystemPrompt = `你是一个专业的 IT 运维智能分析系统，擅长告警根因分析和事件聚类。

你的任务是分析提供的告警数据和基础设施上下文，将相关告警聚类为有意义的事件。

【重要规则】
- 即使只有一个告警，也要分析并生成一个事件候选
- 每个告警都代表一个可能的问题，需要进行根因分析
- 不能因为数据量少就拒绝分析

分析时请考虑以下维度：
1. 告警描述和内容的相似性
2. 告警触发的时间临近性
3. 影响的系统、服务或主机
4. IT 问题的类型（性能退化、服务中断、资源耗尽、网络故障等）
5. 潜在的根本原因
6. 服务间的依赖关系和拓扑关系（如果提供了 CMDB 数据）

对每个聚类出的事件，你需要：
1. 评估事件的严重程度
2. 给出置信度评分（0.0 到 1.0），并解释评分依据
3. 推荐运维团队的初步处理动作
4. 分析推理过程：为什么这些告警被聚类到一起
5. 总结可能的根因

【输出格式】
你必须严格输出以下 JSON 格式，不要添加任何额外字段，不要使用 Markdown 代码块包裹：

{
  "incidents": [
    {
      "incident_name": "事件的简洁名称",
      "alerts": [1, 2],
      "reasoning": "详细解释为什么这些告警被聚类到一起，分析推理过程",
      "severity": "critical",
      "recommended_actions": ["操作1", "操作2"],
      "confidence_score": 0.85,
      "confidence_explanation": "置信度评分的依据说明",
      "root_cause_summary": "对这个事件潜在根因的简要总结"
    }
  ]
}

【字段说明】
- incident_name: 字符串，事件的简洁名称，清晰描述问题
- alerts: 整数数组，关联的告警序号列表（从 1 开始计数，对应告警列表中的序号）
- reasoning: 字符串，【必填】详细的聚类推理分析，解释这些告警的关联性、共同特征和因果关系
- severity: 字符串，取值范围：critical/high/warning/info/low
- recommended_actions: 字符串数组，推荐运维团队的具体处理步骤
- confidence_score: 数字，0.0 到 1.0 之间
- confidence_explanation: 字符串，置信度评分的详细依据
- root_cause_summary: 字符串，【必填】根因摘要，分析潜在的根本原因并给出总结

【重要】
- reasoning 和 root_cause_summary 字段不能为空字符串
- 每个字段都必须填写有意义的内容`

// BuildUserPrompt 构建用户提示词
// 将告警数据和可选的拓扑数据组织成人类可读的提示
func BuildUserPrompt(alertDescriptions string, topologyData string) string {
	prompt := "分析以下 IT 运维告警数据，进行智能聚类：\n\n"

	// 添加告警部分
	prompt += "【告警数据】\n"
	prompt += alertDescriptions + "\n\n"

	// 如果提供了拓扑数据，添加到 prompt 中
	if topologyData != "" {
		prompt += "【服务拓扑数据（用于增强聚类准确性）】\n"
		prompt += topologyData + "\n\n"
	}

	prompt += "请根据上述数据进行告警聚类分析，输出遵循指定的 JSON Schema 格式。"

	return prompt
}

// GetClusteringResponseSchema 返回聚类响应的 JSON Schema
// 对标 Keep: keep/api/bl/ai_suggestion_bl.py:412-462
// 用于约束 OpenAI API 的结构化输出
// 返回格式：{"type": "object", "properties": {...}, "required": [...]}
func GetClusteringResponseSchema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"incidents": map[string]interface{}{
				"type":        "array",
				"description": "聚类出的事件列表",
				"items": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"incident_name": map[string]interface{}{
							"type":        "string",
							"description": "事件的简洁名称，应该清晰描述问题",
						},
						"alerts": map[string]interface{}{
							"type": "array",
							"items": map[string]interface{}{
								"type": "integer",
							},
							"description": "本事件关联的告警序号列表（从 1 开始计数）",
						},
						"reasoning": map[string]interface{}{
							"type":        "string",
							"description": "为什么这些告警被聚类到一起的推理过程",
						},
						"severity": map[string]interface{}{
							"type":        "string",
							"enum":        []string{"critical", "high", "warning", "info", "low"},
							"description": "事件的严重程度等级",
						},
						"recommended_actions": map[string]interface{}{
							"type": "array",
							"items": map[string]interface{}{
								"type": "string",
							},
							"description": "运维团队的推荐处理动作列表",
						},
						"confidence_score": map[string]interface{}{
							"type":        "number",
							"description": "聚类的置信度评分（0.0 ~ 1.0）",
						},
						"confidence_explanation": map[string]interface{}{
							"type":        "string",
							"description": "置信度评分的解释，说明哪些因素支持或降低了这个分数",
						},
						"root_cause_summary": map[string]interface{}{
							"type":        "string",
							"description": "对这个事件潜在根因的简要总结",
						},
					},
					"required": []string{
						"incident_name",
						"alerts",
						"reasoning",
						"severity",
						"recommended_actions",
						"confidence_score",
						"confidence_explanation",
						"root_cause_summary",
					},
					// OpenAI strict 模式要求每个 object 层级都有 additionalProperties: false
					"additionalProperties": false,
				},
			},
		},
		"required":             []string{"incidents"},
		"additionalProperties": false,
	}
}

// RootCauseAnalysisPrompt 根因频率分析提示词
// 对标 Keep: keep/api/bl/incident_reports.py:54-74
// 用于生成统计报告中的根因分析
func RootCauseAnalysisPrompt(incidentsData string) string {
	return `基于提供的事件数据集生成根因分析报告。

【分析要求】
1. 分析以下字段以识别最常见根因：事件名称、事件摘要、严重程度
2. 尝试发现数据中未明确提及的深层根因
3. 根因描述需简洁但准确
4. 合并相似根因，避免重复
5. 仅输出 Top 6 根因

【事件数据】
` + incidentsData + `

【输出格式】
返回 JSON 对象，每个 key 为根因描述（字符串），value 为关联的事件 ID 列表。
示例格式：
{
    "数据库连接池耗尽": ["incident_1", "incident_2"],
    "Kubernetes Pod 内存不足被 OOM Kill": ["incident_3"],
    "上游服务超时导致级联失败": ["incident_4", "incident_5"]
}

请仅输出 JSON 对象，不要包含其他说明文字。`
}

// AIClusteringConfig AI 聚类的配置参数
type AIClusteringConfig struct {
	Model       string  // 使用的模型（默认 gpt-4o-2024-08-06）
	Temperature float64 // 温度参数（默认 0.2，低随机性）
	MaxTokens   int     // 最大 token 数（默认 4000）
	Timeout     int     // 超时时间（秒）
}

// DefaultAIClusteringConfig 返回默认配置
// 对标 Keep 的配置
func DefaultAIClusteringConfig() AIClusteringConfig {
	return AIClusteringConfig{
		Model:       "gpt-4o-2024-08-06",
		Temperature: 0.2, // 低随机性确保结果一致性
		MaxTokens:   4000,
		Timeout:     30, // 30 秒超时
	}
}
