package ai

import (
	"alertHub/internal/ctx"
	"alertHub/internal/models"
	"alertHub/pkg/analysis/interfaces"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/zeromicro/go-zero/core/logc"
)

// AIAnalysisEngine AI分析引擎 - 纯推理驱动
// 实现interfaces.UniversalAIEngine接口
type AIAnalysisEngine struct {
	client            *http.Client
	contextSerializer *ContextSerializer
	promptGenerator   *DynamicPromptGenerator
	responseParser    *UniversalResponseParser
	strategyEngine    interfaces.ConfigurableStrategyEngine
	config           *AIEngineConfig
}

// 确保实现了接口
var _ interfaces.UniversalAIEngine = (*AIAnalysisEngine)(nil)

// AIEngineConfig AI引擎配置
type AIEngineConfig struct {
	APIEndpoint     string            `json:"apiEndpoint"`     // AI API端点
	APIKey          string            `json:"apiKey"`          // API密钥
	Model           string            `json:"model"`           // 模型名称
	MaxTokens       int               `json:"maxTokens"`       // 最大Token数
	Temperature     float64           `json:"temperature"`     // 温度参数
	Timeout         time.Duration     `json:"timeout"`         // 请求超时
	RetryAttempts   int               `json:"retryAttempts"`   // 重试次数
	RetryDelay      time.Duration     `json:"retryDelay"`      // 重试延迟
	CustomHeaders   map[string]string `json:"customHeaders"`   // 自定义请求头
}

// AnalysisRequest 分析请求
type AnalysisRequest struct {
	AnalysisType   string                 `json:"analysisType"`   // 分析类型
	AnalysisMode   string                 `json:"analysisMode"`   // 分析模式
	AnalysisDepth  string                 `json:"analysisDepth"`  // 分析深度
	FocusAreas     []string               `json:"focusAreas"`     // 关注领域
	CustomPrompts  map[string]interface{} `json:"customPrompts"`  // 自定义提示参数
}

// AnalysisResult AI分析结果
type AnalysisResult struct {
	AnalysisID       string                 `json:"analysisId"`       // 分析ID
	Summary          *AnalysisSummary       `json:"summary"`          // 分析摘要
	DataAnalysis     *DataAnalysisResult    `json:"dataAnalysis"`     // 数据分析结果
	RootCauseAnalysis *RootCauseAnalysis    `json:"rootCauseAnalysis"` // 根因分析
	ActionRecommendations []*ActionRecommendation `json:"actionRecommendations"` // 行动建议
	MonitoringRecommendations *MonitoringRecommendation `json:"monitoringRecommendations"` // 监控建议
	ConfidenceScore  float64                `json:"confidenceScore"`  // 置信度评分
	Limitations      []string               `json:"limitations"`      // 分析局限性
	FollowUpAnalysis []string               `json:"followUpAnalysis"` // 后续分析建议
	ProcessingTime   int64                  `json:"processingTime"`   // 处理时间(毫秒)
}

// AnalysisSummary 分析摘要
type AnalysisSummary struct {
	Title        string   `json:"title"`        // 问题标题
	Description  string   `json:"description"`  // 详细描述
	Severity     string   `json:"severity"`     // 严重程度
	Category     string   `json:"category"`     // 问题类别
	Confidence   float64  `json:"confidence"`   // 置信度
	KeyFindings  []string `json:"keyFindings"`  // 关键发现
}

// DataAnalysisResult 数据分析结果
type DataAnalysisResult struct {
	PrimaryMetricAnalysis *MetricAnalysis       `json:"primaryMetricAnalysis"` // 主要指标分析
	RelationshipAnalysis  *RelationshipAnalysis `json:"relationshipAnalysis"`  // 关系分析
	SystemAnalysis        *SystemAnalysis       `json:"systemAnalysis"`        // 系统分析
	TrendAnalysis         *TrendAnalysis        `json:"trendAnalysis"`         // 趋势分析
	AnomalyAnalysis       *AnomalyAnalysisResult `json:"anomalyAnalysis"`      // 异常分析
}

// RootCauseAnalysis 根因分析
type RootCauseAnalysis struct {
	PrimaryHypothesis      string              `json:"primaryHypothesis"`      // 主要假设
	SupportingEvidence     []*Evidence         `json:"supportingEvidence"`     // 支撑证据
	AlternativeHypotheses  []string            `json:"alternativeHypotheses"`  // 替代假设
	CausalChain           []*CausalLink       `json:"causalChain"`            // 因果链
	Confidence            float64             `json:"confidence"`             // 置信度
}

// ActionRecommendation 行动建议
type ActionRecommendation struct {
	Priority        int                    `json:"priority"`        // 优先级
	Type           string                 `json:"type"`            // 类型
	Title          string                 `json:"title"`           // 标题
	Rationale      string                 `json:"rationale"`       // 理由
	Steps          []*ActionStep          `json:"steps"`           // 执行步骤
	RiskAssessment string                 `json:"riskAssessment"`  // 风险评估
	SuccessMetrics []string               `json:"successMetrics"`  // 成功指标
	Urgency        string                 `json:"urgency"`         // 紧急程度
}

// NewAIAnalysisEngine 创建AI分析引擎
func NewAIAnalysisEngine(config *AIEngineConfig) *AIAnalysisEngine {
	if config == nil {
		config = &AIEngineConfig{
			Model:         "gpt-4",
			MaxTokens:     4096,
			Temperature:   0.7,
			Timeout:       30 * time.Second,
			RetryAttempts: 3,
			RetryDelay:    2 * time.Second,
		}
	}

	// 创建策略引擎
	strategyEngine, err := createAIStrategyEngine()
	if err != nil {
		// 使用默认实现
		strategyEngine = &DefaultAIStrategyEngine{}
	}

	return &AIAnalysisEngine{
		client: &http.Client{
			Timeout: config.Timeout,
		},
		contextSerializer: NewContextSerializer(),
		promptGenerator:   NewDynamicPromptGenerator(),
		responseParser:    NewUniversalResponseParser(),
		strategyEngine:    strategyEngine,
		config:           config,
	}
}

// Analyze 执行AI分析 - 纯推理，无业务逻辑（实现接口方法）
func (aae *AIAnalysisEngine) Analyze(
	ctx *ctx.Context,
	analysisContext *models.UniversalAnalysisContext,
	request *interfaces.AIAnalysisRequest,
) (*interfaces.AIAnalysisResult, error) {
	
	startTime := time.Now()
	analysisID := fmt.Sprintf("ai_analysis_%d_%s", startTime.Unix(), analysisContext.ContextId)
	
	logc.Infof(ctx.Ctx, "[AI分析] 开始分析: analysisId=%s, contextId=%s, mode=%s", 
		analysisID, analysisContext.ContextId, request.AnalysisMode)

	// 1. 动态生成分析提示（基于上下文自适应）
	prompt, err := aae.GeneratePrompt(ctx, analysisContext, request)
	if err != nil {
		return nil, fmt.Errorf("生成分析提示失败: %w", err)
	}

	// 2. 执行AI推理（支持重试机制）
	rawResponse, err := aae.performAnalysisWithRetry(ctx, prompt)
	if err != nil {
		return nil, fmt.Errorf("AI推理失败: %w", err)
	}

	// 3. 解析AI响应（通用解析器，支持各种响应格式）
	result, err := aae.ParseResponse(ctx, rawResponse, analysisContext)
	if err != nil {
		return nil, fmt.Errorf("解析AI响应失败: %w", err)
	}

	// 4. 结果增强（添加置信度、数据链接等）
	enhancedResult := aae.enhanceAIResult(ctx, result, analysisContext)
	enhancedResult.ProcessingTime = time.Since(startTime).Milliseconds()

	logc.Infof(ctx.Ctx, "[AI分析] 完成: analysisId=%s, 耗时=%v, 置信度=%.2f", 
		analysisID, time.Since(startTime), enhancedResult.ConfidenceScore)

	return enhancedResult, nil
}

// performAnalysisWithRetry 执行AI分析（带重试）
func (aae *AIAnalysisEngine) performAnalysisWithRetry(
	ctx *ctx.Context,
	prompt string,
) (string, error) {
	
	var lastError error
	
	for attempt := 1; attempt <= aae.config.RetryAttempts; attempt++ {
		response, err := aae.performSingleAnalysis(ctx, prompt)
		if err == nil {
			return response, nil
		}
		
		lastError = err
		logc.Errorf(ctx.Ctx, "[AI分析] 第%d次尝试失败: %v", attempt, err)
		
		if attempt < aae.config.RetryAttempts {
			time.Sleep(aae.config.RetryDelay)
		}
	}
	
	return "", fmt.Errorf("AI分析重试%d次后仍失败: %w", aae.config.RetryAttempts, lastError)
}

// performSingleAnalysis 执行单次AI分析
func (aae *AIAnalysisEngine) performSingleAnalysis(
	ctx *ctx.Context,
	prompt string,
) (string, error) {
	
	// 构建请求体
	requestBody := map[string]interface{}{
		"model":       aae.config.Model,
		"messages": []map[string]interface{}{
			{
				"role":    "system",
				"content": "你是一位资深的监控分析专家，专门进行告警数据的深度分析。",
			},
			{
				"role":    "user", 
				"content": prompt,
			},
		},
		"max_tokens":  aae.config.MaxTokens,
		"temperature": aae.config.Temperature,
	}

	jsonData, err := json.Marshal(requestBody)
	if err != nil {
		return "", fmt.Errorf("序列化请求失败: %w", err)
	}

	// 创建HTTP请求
	req, err := http.NewRequestWithContext(ctx.Ctx, "POST", aae.config.APIEndpoint, bytes.NewBuffer(jsonData))
	if err != nil {
		return "", fmt.Errorf("创建HTTP请求失败: %w", err)
	}

	// 设置请求头
	req.Header.Set("Content-Type", "application/json")
	if aae.config.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+aae.config.APIKey)
	}
	
	// 设置自定义请求头
	for key, value := range aae.config.CustomHeaders {
		req.Header.Set(key, value)
	}

	// 执行请求
	resp, err := aae.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("HTTP请求失败: %w", err)
	}
	defer resp.Body.Close()

	// 读取响应
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("读取响应失败: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("API请求失败: status=%d, body=%s", resp.StatusCode, string(body))
	}

	// 解析响应
	var response map[string]interface{}
	if err := json.Unmarshal(body, &response); err != nil {
		return "", fmt.Errorf("解析响应JSON失败: %w", err)
	}

	// 提取内容
	choices, ok := response["choices"].([]interface{})
	if !ok || len(choices) == 0 {
		return "", fmt.Errorf("响应中没有找到choices字段")
	}

	firstChoice, ok := choices[0].(map[string]interface{})
	if !ok {
		return "", fmt.Errorf("无效的choice格式")
	}

	message, ok := firstChoice["message"].(map[string]interface{})
	if !ok {
		return "", fmt.Errorf("无效的message格式")
	}

	content, ok := message["content"].(string)
	if !ok {
		return "", fmt.Errorf("无效的content格式")
	}

	return strings.TrimSpace(content), nil
}

// enhanceResult 增强分析结果
func (aae *AIAnalysisEngine) enhanceResult(
	ctx *ctx.Context,
	result *AnalysisResult,
	analysisContext *models.UniversalAnalysisContext,
) *AnalysisResult {
	
	// 1. 计算整体置信度
	result.ConfidenceScore = aae.calculateOverallConfidence(result)

	// 2. 添加数据质量评估
	if result.Summary != nil && analysisContext.PrimaryMetric != nil {
		if analysisContext.PrimaryMetric.DataQuality != nil {
			dataQuality := analysisContext.PrimaryMetric.DataQuality.QualityScore
			// 根据数据质量调整置信度
			result.ConfidenceScore = result.ConfidenceScore * dataQuality
		}
	}

	// 3. 添加上下文链接
	if result.DataAnalysis != nil && result.DataAnalysis.PrimaryMetricAnalysis != nil {
		result.DataAnalysis.PrimaryMetricAnalysis.MetricName = analysisContext.PrimaryMetric.MetricName
		result.DataAnalysis.PrimaryMetricAnalysis.DataPoints = len(analysisContext.PrimaryMetric.TimeSeries)
	}

	// 4. 添加局限性说明
	result.Limitations = aae.generateLimitations(analysisContext)

	// 5. 添加后续分析建议
	result.FollowUpAnalysis = aae.generateFollowUpSuggestions(result, analysisContext)

	return result
}

// calculateOverallConfidence 计算整体置信度
func (aae *AIAnalysisEngine) calculateOverallConfidence(result *AnalysisResult) float64 {
	weights := []struct {
		confidence float64
		weight     float64
	}{
		{result.Summary.Confidence, 0.3},
	}

	if result.RootCauseAnalysis != nil {
		weights = append(weights, struct {
			confidence float64
			weight     float64
		}{result.RootCauseAnalysis.Confidence, 0.4})
	}

	totalWeight := 0.0
	weightedSum := 0.0

	for _, w := range weights {
		weightedSum += w.confidence * w.weight
		totalWeight += w.weight
	}

	if totalWeight == 0 {
		return 0.5 // 默认置信度
	}

	return weightedSum / totalWeight
}

// generateLimitations 生成局限性说明
func (aae *AIAnalysisEngine) generateLimitations(
	analysisContext *models.UniversalAnalysisContext,
) []string {
	
	limitations := make([]string, 0)

	// 检查数据完整性
	if analysisContext.PrimaryMetric != nil {
		dataPoints := len(analysisContext.PrimaryMetric.TimeSeries)
		if dataPoints < 10 {
			limitations = append(limitations, "数据点数量较少，可能影响分析准确性")
		}

		if analysisContext.PrimaryMetric.DataQuality != nil {
			quality := analysisContext.PrimaryMetric.DataQuality.QualityScore
			if quality < 0.8 {
				limitations = append(limitations, "数据质量存在问题，可能影响分析结果")
			}
		}
	}

	// 检查相关指标
	if len(analysisContext.RelatedMetrics) == 0 {
		limitations = append(limitations, "缺少相关指标数据，无法进行全面的关联分析")
	}

	// 检查时间范围
	if analysisContext.TimeContext != nil {
		timeSpan := analysisContext.TimeContext.AnalysisTime - analysisContext.TimeContext.EventTime
		if timeSpan < 3600 { // 少于1小时
			limitations = append(limitations, "分析时间窗口较短，可能无法捕获长期趋势")
		}
	}

	return limitations
}

// generateFollowUpSuggestions 生成后续分析建议
func (aae *AIAnalysisEngine) generateFollowUpSuggestions(
	result *AnalysisResult,
	analysisContext *models.UniversalAnalysisContext,
) []string {
	
	suggestions := make([]string, 0)

	// 根据置信度建议
	if result.ConfidenceScore < 0.7 {
		suggestions = append(suggestions, "建议收集更多数据后重新分析")
		suggestions = append(suggestions, "建议人工验证分析结果")
	}

	// 根据严重程度建议
	if result.Summary != nil && result.Summary.Severity == "critical" {
		suggestions = append(suggestions, "建议立即启动应急响应流程")
		suggestions = append(suggestions, "建议扩大监控范围，检查影响范围")
	}

	// 根据问题类别建议
	if result.Summary != nil {
		switch result.Summary.Category {
		case "performance":
			suggestions = append(suggestions, "建议进行性能基线分析")
		case "resource":
			suggestions = append(suggestions, "建议进行容量规划分析")
		case "network":
			suggestions = append(suggestions, "建议进行网络拓扑分析")
		}
	}

	return suggestions
}

// GeneratePrompt 动态生成分析提示（实现接口方法）
func (aae *AIAnalysisEngine) GeneratePrompt(
	ctx *ctx.Context,
	analysisContext *models.UniversalAnalysisContext,
	request *interfaces.AIAnalysisRequest,
) (string, error) {
	return aae.promptGenerator.GenerateAnalysisPromptWithInterface(ctx, analysisContext, request)
}

// ParseResponse 解析AI响应（实现接口方法）
func (aae *AIAnalysisEngine) ParseResponse(
	ctx *ctx.Context,
	response string,
	analysisContext *models.UniversalAnalysisContext,
) (*interfaces.AIAnalysisResult, error) {
	return aae.responseParser.ParseWithInterface(ctx, response, analysisContext.ContextId)
}

// ValidateConfig 验证AI引擎配置（实现接口方法）
func (aae *AIAnalysisEngine) ValidateConfig(config interface{}) error {
	aiConfig, ok := config.(*AIEngineConfig)
	if !ok {
		return fmt.Errorf("配置类型错误，期望 *AIEngineConfig，实际 %T", config)
	}
	return validateAIEngineConfig(aiConfig)
}

// validateAIEngineConfig 验证AI引擎配置
func validateAIEngineConfig(config *AIEngineConfig) error {
	if config == nil {
		return fmt.Errorf("配置不能为空")
	}

	if config.Model == "" {
		return fmt.Errorf("Model不能为空")
	}

	if config.MaxTokens <= 0 {
		return fmt.Errorf("MaxTokens必须大于0")
	}

	if config.Temperature < 0 || config.Temperature > 2 {
		return fmt.Errorf("Temperature必须在0-2范围内")
	}

	if config.Timeout <= 0 {
		return fmt.Errorf("Timeout必须大于0")
	}

	if config.RetryAttempts < 0 {
		return fmt.Errorf("RetryAttempts不能为负数")
	}

	if config.RetryDelay < 0 {
		return fmt.Errorf("RetryDelay不能为负数")
	}

	return nil
}

// enhanceAIResult 增强AI分析结果（转换为接口格式）
func (aae *AIAnalysisEngine) enhanceAIResult(
	ctx *ctx.Context,
	result *interfaces.AIAnalysisResult,
	analysisContext *models.UniversalAnalysisContext,
) *interfaces.AIAnalysisResult {
	
	// 1. 计算整体置信度
	result.ConfidenceScore = aae.calculateConfidence(result)

	// 2. 添加数据质量评估
	if result.Summary != nil && analysisContext.PrimaryMetric != nil {
		if analysisContext.PrimaryMetric.DataQuality != nil {
			dataQuality := analysisContext.PrimaryMetric.DataQuality.QualityScore
			// 根据数据质量调整置信度
			result.ConfidenceScore = result.ConfidenceScore * dataQuality
		}
	}

	// 3. 添加上下文链接
	if result.DataAnalysis != nil {
		// 添加指标信息
		if analysisContext.PrimaryMetric != nil {
			if result.DataAnalysis.PrimaryMetricAnalysis == nil {
				result.DataAnalysis.PrimaryMetricAnalysis = make(map[string]interface{})
			}
			result.DataAnalysis.PrimaryMetricAnalysis["primaryMetricName"] = analysisContext.PrimaryMetric.MetricName
			result.DataAnalysis.PrimaryMetricAnalysis["dataPoints"] = len(analysisContext.PrimaryMetric.TimeSeries)
		}
	}

	// 4. 添加局限性说明
	if result.Metadata == nil {
		result.Metadata = make(map[string]interface{})
	}
	result.Metadata["limitations"] = aae.generateAILimitations(analysisContext)

	// 5. 添加后续分析建议
	result.Metadata["followUpSuggestions"] = aae.generateAIFollowUpSuggestions(result, analysisContext)

	return result
}

// calculateConfidence 计算整体置信度
func (aae *AIAnalysisEngine) calculateConfidence(result *interfaces.AIAnalysisResult) float64 {
	if result.Summary == nil {
		return 0.5 // 默认置信度
	}

	totalConfidence := result.Summary.Confidence
	count := 1

	// 根因分析置信度
	if result.RootCauseAnalysis != nil {
		totalConfidence += result.RootCauseAnalysis.Confidence
		count++
	}

	return totalConfidence / float64(count)
}

// generateAILimitations 生成AI分析局限性说明
func (aae *AIAnalysisEngine) generateAILimitations(
	analysisContext *models.UniversalAnalysisContext,
) []string {
	limitations := make([]string, 0)

	// 检查数据完整性
	if analysisContext.PrimaryMetric != nil {
		dataPoints := len(analysisContext.PrimaryMetric.TimeSeries)
		if dataPoints < 10 {
			limitations = append(limitations, "数据点数量较少，可能影响AI分析准确性")
		}

		if analysisContext.PrimaryMetric.DataQuality != nil {
			quality := analysisContext.PrimaryMetric.DataQuality.QualityScore
			if quality < 0.8 {
				limitations = append(limitations, "数据质量存在问题，可能影响AI分析结果")
			}
		}
	}

	// 检查相关指标
	if len(analysisContext.RelatedMetrics) == 0 {
		limitations = append(limitations, "缺少相关指标数据，无法进行全面的关联分析")
	}

	// AI特有局限性
	limitations = append(limitations, "AI分析基于统计模式，可能无法识别未知模式")
	limitations = append(limitations, "建议结合专家经验验证AI分析结果")

	return limitations
}

// generateAIFollowUpSuggestions 生成AI后续分析建议
func (aae *AIAnalysisEngine) generateAIFollowUpSuggestions(
	result *interfaces.AIAnalysisResult,
	analysisContext *models.UniversalAnalysisContext,
) []string {
	suggestions := make([]string, 0)

	// 根据置信度建议
	if result.ConfidenceScore < 0.7 {
		suggestions = append(suggestions, "建议收集更多数据后重新进行AI分析")
		suggestions = append(suggestions, "建议人工验证AI分析结果")
	}

	// 根据严重程度庭议
	if result.Summary != nil && result.Summary.Severity == "critical" {
		suggestions = append(suggestions, "建议立即启动应急响应流程")
		suggestions = append(suggestions, "建议扩大监控范围，检查影响范围")
	}

	// AI特有建议
	suggestions = append(suggestions, "建议定期重新训练AI模型以提高分析能力")
	suggestions = append(suggestions, "建议收集用户反馈优化AI分析算法")

	return suggestions
}

// createAIStrategyEngine 创建AI策略引擎（内部使用）
func createAIStrategyEngine() (interfaces.ConfigurableStrategyEngine, error) {
	// 这里可以集成已有的策略引擎实现
	// 为简化示例，返回默认实现
	return &DefaultAIStrategyEngine{}, nil
}

// DefaultAIStrategyEngine AI默认策略引擎实现
type DefaultAIStrategyEngine struct {
	strategies map[string]interfaces.StrategyBuilder
	mutex      sync.RWMutex
}

func (dase *DefaultAIStrategyEngine) LoadStrategy(ctx *ctx.Context, strategyType string, config map[string]interface{}) (interfaces.Strategy, error) {
	return nil, fmt.Errorf("AI策略类型 %s 未实现", strategyType)
}

func (dase *DefaultAIStrategyEngine) ExecuteStrategy(ctx *ctx.Context, strategy interfaces.Strategy, input interface{}) (interface{}, error) {
	return strategy.Execute(ctx, input)
}

func (dase *DefaultAIStrategyEngine) RegisterStrategy(strategyType string, builder interfaces.StrategyBuilder) error {
	if dase.strategies == nil {
		dase.strategies = make(map[string]interfaces.StrategyBuilder)
	}
	dase.mutex.Lock()
	defer dase.mutex.Unlock()
	dase.strategies[strategyType] = builder
	return nil
}

func (dase *DefaultAIStrategyEngine) ValidateStrategyConfig(strategyType string, config map[string]interface{}) error {
	dase.mutex.RLock()
	builder, exists := dase.strategies[strategyType]
	dase.mutex.RUnlock()

	if !exists {
		return fmt.Errorf("未知AI策略类型: %s", strategyType)
	}

	return builder.ValidateConfig(config)
}

// 辅助数据结构

type MetricAnalysis struct {
	MetricName             string  `json:"metricName"`
	DataPoints             int     `json:"dataPoints"`
	StatisticalFeatures    string  `json:"statisticalFeatures"`
	TrendAnalysis          string  `json:"trendAnalysis"`
	AnomalyAnalysis        string  `json:"anomalyAnalysis"`
	BaselineComparison     string  `json:"baselineComparison"`
}

type RelationshipAnalysis struct {
	CorrelatedMetrics    []CorrelatedMetric `json:"correlatedMetrics"`
	CausalChain          string             `json:"causalChain"`
	ImpactScope          string             `json:"impactScope"`
}

type SystemAnalysis struct {
	TopologyImpact       string `json:"topologyImpact"`
	DependencyAnalysis   string `json:"dependencyAnalysis"`
	CascadingEffects     string `json:"cascadingEffects"`
}

type TrendAnalysis struct {
	ShortTermTrend       string  `json:"shortTermTrend"`
	MediumTermTrend      string  `json:"mediumTermTrend"`
	TrendStrength        float64 `json:"trendStrength"`
	SeasonalityDetected  bool    `json:"seasonalityDetected"`
}

type AnomalyAnalysisResult struct {
	HasAnomalies         bool    `json:"hasAnomalies"`
	AnomalyCount         int     `json:"anomalyCount"`
	AnomalySeverity      string  `json:"anomalySeverity"`
	AnomalyPattern       string  `json:"anomalyPattern"`
}

type Evidence struct {
	Type         string  `json:"type"`         // statistical|pattern|correlation|anomaly
	Description  string  `json:"description"`
	Strength     float64 `json:"strength"`
	Data         string  `json:"data"`
}

type CausalLink struct {
	From         string  `json:"from"`
	To           string  `json:"to"`
	Relationship string  `json:"relationship"`
	Confidence   float64 `json:"confidence"`
}

type ActionStep struct {
	Order            int    `json:"order"`
	Action           string `json:"action"`
	Verification     string `json:"verification"`
	ExpectedOutcome  string `json:"expectedOutcome"`
}

type MonitoringRecommendation struct {
	AdditionalMetrics    []string `json:"additionalMetrics"`
	AlertOptimization    string   `json:"alertOptimization"`
	DashboardImprovements string  `json:"dashboardImprovements"`
}

type CorrelatedMetric struct {
	Metric      string  `json:"metric"`
	Correlation float64 `json:"correlation"`
	Type        string  `json:"type"` // positive|negative|leading|lagging
}