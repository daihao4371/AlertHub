package analysis

import (
	"alertHub/internal/ctx"
	"alertHub/internal/models"
	"alertHub/pkg/analysis/ai"
	"alertHub/pkg/analysis/interfaces"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/zeromicro/go-zero/core/logc"
)

// ResultProcessor 完全配置驱动的结果处理器
// 实现interfaces.UniversalResultProcessor接口
type ResultProcessor struct {
	config          *ResultProcessorConfig
	qualityChecker  *ResultQualityChecker
	enricher        *ResultEnricher
	formatter       *ResultFormatter
	validator       *ResultValidator
}

// 确保实现了接口
var _ interfaces.UniversalResultProcessor = (*ResultProcessor)(nil)

// 内部辅助类型定义
type ResultQualityChecker struct {
	rules map[string]interface{}
}

type ResultEnricher struct {
	rules map[string]interface{}
}

type ResultFormatter struct {
	rules map[string]interface{}
}

type ResultValidator struct{}

// 构造函数
func NewResultQualityChecker(rules map[string]interface{}) *ResultQualityChecker {
	if rules == nil {
		rules = make(map[string]interface{})
	}
	return &ResultQualityChecker{rules: rules}
}

func NewResultEnricher(rules map[string]interface{}) *ResultEnricher {
	if rules == nil {
		rules = make(map[string]interface{})
	}
	return &ResultEnricher{rules: rules}
}

func NewResultFormatter(rules map[string]interface{}) *ResultFormatter {
	if rules == nil {
		rules = make(map[string]interface{})
	}
	return &ResultFormatter{rules: rules}
}

func NewResultValidator() *ResultValidator {
	return &ResultValidator{}
}

// ResultProcessorConfig 结果处理器配置
type ResultProcessorConfig struct {
	// 质量控制配置
	MinConfidenceThreshold   float64   `json:"minConfidenceThreshold"`   // 最小置信度阈值
	RequiredSections         []string  `json:"requiredSections"`         // 必需的结果章节
	MaxProcessingTime        int64     `json:"maxProcessingTime"`        // 最大处理时间(毫秒)
	EnableQualityValidation  bool      `json:"enableQualityValidation"`  // 启用质量验证
	EnableResultEnrichment   bool      `json:"enableResultEnrichment"`   // 启用结果增强
	EnableFormatOptimization bool      `json:"enableFormatOptimization"` // 启用格式优化
	
	// 增强配置
	EnrichmentRules          map[string]interface{} `json:"enrichmentRules"`          // 增强规则
	FormattingRules          map[string]interface{} `json:"formattingRules"`          // 格式化规则
	ValidationRules          map[string]interface{} `json:"validationRules"`          // 验证规则
	
	// 输出配置
	OutputFormats            []string               `json:"outputFormats"`            // 输出格式
	IncludeRawData          bool                   `json:"includeRawData"`           // 包含原始数据
	IncludeMetadata         bool                   `json:"includeMetadata"`          // 包含元数据
	IncludeProcessingStats  bool                   `json:"includeProcessingStats"`   // 包含处理统计
}

// ProcessingResult 处理结果
type ProcessingResult struct {
	// 基础结果
	OriginalResult    *ai.AnalysisResult         `json:"originalResult"`    // 原始AI结果
	ProcessedResult   *ai.AnalysisResult         `json:"processedResult"`   // 处理后结果
	
	// 质量评估
	QualityAssessment *QualityAssessment         `json:"qualityAssessment"` // 质量评估
	
	// 处理元数据
	ProcessingMetadata *ProcessingMetadata       `json:"processingMetadata"` // 处理元数据
	
	// 格式化输出
	FormattedOutputs   map[string]interface{}    `json:"formattedOutputs"`   // 格式化输出
	
	// 处理状态
	ProcessingStatus   string                    `json:"processingStatus"`   // 处理状态
	ProcessingErrors   []string                  `json:"processingErrors"`   // 处理错误
	ProcessingWarnings []string                  `json:"processingWarnings"` // 处理警告
}

// QualityAssessment 质量评估
type QualityAssessment struct {
	OverallQuality     float64            `json:"overallQuality"`     // 整体质量分数
	CompletenessScore  float64            `json:"completenessScore"`  // 完整性分数
	ConfidenceScore    float64            `json:"confidenceScore"`    // 置信度分数
	ConsistencyScore   float64            `json:"consistencyScore"`   // 一致性分数
	UsabilityScore     float64            `json:"usabilityScore"`     // 可用性分数
	
	QualityIssues      []QualityIssue     `json:"qualityIssues"`      // 质量问题
	QualityMetrics     map[string]float64 `json:"qualityMetrics"`     // 质量指标
}

// QualityIssue 质量问题
type QualityIssue struct {
	Type        string  `json:"type"`        // 问题类型
	Severity    string  `json:"severity"`    // 严重程度
	Description string  `json:"description"` // 问题描述
	Impact      float64 `json:"impact"`      // 影响分数
	Suggestion  string  `json:"suggestion"`  // 改进建议
}

// ProcessingMetadata 处理元数据
type ProcessingMetadata struct {
	ProcessorVersion   string    `json:"processorVersion"`   // 处理器版本
	ProcessingTime     int64     `json:"processingTime"`     // 处理时间(毫秒)
	ProcessedAt        time.Time `json:"processedAt"`        // 处理时间戳
	ProcessingSteps    []string  `json:"processingSteps"`    // 处理步骤
	ConfigurationUsed  string    `json:"configurationUsed"`  // 使用的配置
	EnhancementsApplied []string `json:"enhancementsApplied"` // 应用的增强
}

// NewResultProcessor 创建结果处理器
func NewResultProcessor(config *ResultProcessorConfig) *ResultProcessor {
	if config == nil {
		config = getDefaultResultProcessorConfig()
	}

	return &ResultProcessor{
		config:         config,
		qualityChecker: NewResultQualityChecker(config.ValidationRules),
		enricher:       NewResultEnricher(config.EnrichmentRules),
		formatter:      NewResultFormatter(config.FormattingRules),
		validator:      NewResultValidator(),
	}
}

// getDefaultResultProcessorConfig 获取默认配置
func getDefaultResultProcessorConfig() *ResultProcessorConfig {
	return &ResultProcessorConfig{
		MinConfidenceThreshold:   0.6,
		RequiredSections:         []string{"summary", "dataAnalysis", "actionRecommendations"},
		MaxProcessingTime:        5000, // 5秒
		EnableQualityValidation:  true,
		EnableResultEnrichment:   true,
		EnableFormatOptimization: true,
		OutputFormats:            []string{"json", "markdown"},
		IncludeRawData:          false,
		IncludeMetadata:         true,
		IncludeProcessingStats:  true,
		EnrichmentRules:         make(map[string]interface{}),
		FormattingRules:         make(map[string]interface{}),
		ValidationRules:         make(map[string]interface{}),
	}
}

// Process 处理AI分析结果 - 实现接口方法
func (rp *ResultProcessor) Process(
	ctx *ctx.Context,
	analysisResult *interfaces.AIAnalysisResult,
	analysisContext *models.UniversalAnalysisContext,
) (*interfaces.ProcessingResult, error) {
	
	startTime := time.Now()
	processingID := fmt.Sprintf("processing_%d", startTime.Unix())
	
	logc.Infof(ctx.Ctx, "[结果处理] 开始处理: processingId=%s, analysisId=%s", 
		processingID, analysisResult.AnalysisID)

	result := &interfaces.ProcessingResult{
		ProcessingID:       processingID,
		ProcessingStatus:   "processing",
		ProcessedResult:    analysisResult,
		QualityAssessment:  &interfaces.QualityAssessment{},
		FormattedOutput:    make(map[string]interface{}),
		ProcessingMetadata: &interfaces.ProcessingMetadata{
			ProcessingTime: 0,
			ResourceUsage:  make(map[string]interface{}),
			PerformanceMetrics: make(map[string]interface{}),
			ConfigUsed:     make(map[string]interface{}),
			VersionInfo:    make(map[string]interface{}),
		},
	}

	// 1. 验证输入结果
	if err := rp.validateInterfaceInput(ctx, analysisResult); err != nil {
		result.ProcessingStatus = "failed"
		result.QualityAssessment.Warnings = append(result.QualityAssessment.Warnings, fmt.Sprintf("输入验证失败: %v", err))
		return result, err
	}

	// 2. 质量检查
	if rp.config.EnableQualityValidation {
		qualityConfig := &interfaces.QualityConfig{
			MinConfidence:    rp.config.MinConfidenceThreshold,
			ValidationRules: rp.config.RequiredSections,
			QualityThresholds: map[string]float64{"overall": 0.7},
		}
		
		qualityAssessment, err := rp.ValidateQuality(ctx, analysisResult, qualityConfig)
		if err != nil {
			result.QualityAssessment.Warnings = append(result.QualityAssessment.Warnings, fmt.Sprintf("质量检查失败: %v", err))
		} else {
			result.QualityAssessment = qualityAssessment
		}
	}

	// 3. 格式化输出
	if rp.config.EnableFormatOptimization {
		for _, format := range rp.config.OutputFormats {
			formattingConfig := &interfaces.FormattingConfig{
				OutputFormat:    format,
				IncludeMetadata: rp.config.IncludeMetadata,
				Compression:     false,
				CustomFields:    make(map[string]interface{}),
			}
			
			formatted, err := rp.FormatResult(ctx, analysisResult, format, formattingConfig)
			if err != nil {
				result.QualityAssessment.Warnings = append(result.QualityAssessment.Warnings, fmt.Sprintf("格式化失败 [%s]: %v", format, err))
			} else {
				result.FormattedOutput[format] = formatted
			}
		}
	}

	// 4. 处理时间和状态
	processingTime := time.Since(startTime)
	result.ProcessingMetadata.ProcessingTime = processingTime.Milliseconds()

	// 检查处理时间是否超限
	if result.ProcessingMetadata.ProcessingTime > rp.config.MaxProcessingTime {
		result.QualityAssessment.Warnings = append(result.QualityAssessment.Warnings, 
			fmt.Sprintf("处理时间超限: %d ms > %d ms", result.ProcessingMetadata.ProcessingTime, rp.config.MaxProcessingTime))
	}

	// 设置最终状态
	if len(result.QualityAssessment.QualityIssues) > 0 {
		result.ProcessingStatus = "failed"
	} else if len(result.QualityAssessment.Warnings) > 0 {
		result.ProcessingStatus = "warning"
	} else {
		result.ProcessingStatus = "success"
	}

	logc.Infof(ctx.Ctx, "[结果处理] 完成: processingId=%s, 状态=%s, 耗时=%v", 
		processingID, result.ProcessingStatus, processingTime)

	return result, nil
}

// validateInterfaceInput 验证接口输入
func (rp *ResultProcessor) validateInterfaceInput(ctx *ctx.Context, analysisResult *interfaces.AIAnalysisResult) error {
	if analysisResult == nil {
		return fmt.Errorf("分析结果为空")
	}

	if analysisResult.AnalysisID == "" {
		return fmt.Errorf("分析ID为空")
	}

	// 检查必需的章节
	for _, section := range rp.config.RequiredSections {
		switch section {
		case "summary":
			if analysisResult.Summary == nil {
				return fmt.Errorf("缺少必需章节: summary")
			}
		case "dataAnalysis":
			if analysisResult.DataAnalysis == nil {
				return fmt.Errorf("缺少必需章节: dataAnalysis")
			}
		case "actionRecommendations":
			if analysisResult.Recommendations == nil || len(analysisResult.Recommendations) == 0 {
				return fmt.Errorf("缺少必需章节: actionRecommendations")
			}
		}
	}

	// 检查置信度
	if analysisResult.ConfidenceScore < rp.config.MinConfidenceThreshold {
		return fmt.Errorf("置信度过低: %.2f < %.2f", analysisResult.ConfidenceScore, rp.config.MinConfidenceThreshold)
	}

	return nil
}

// ValidateQuality 验证结果质量 - 实现接口方法
func (rp *ResultProcessor) ValidateQuality(
	ctx *ctx.Context,
	result *interfaces.AIAnalysisResult,
	config *interfaces.QualityConfig,
) (*interfaces.QualityAssessment, error) {
	
	assessment := &interfaces.QualityAssessment{
		OverallQuality:        0.0,
		DataCompleteness:      0.0,
		AnalysisAccuracy:      result.ConfidenceScore,
		RecommendationQuality: 0.0,
		Timeliness:           1.0, // 默认及时性满分
		Consistency:          0.0,
		QualityIssues:        make([]string, 0),
		Warnings:             make([]string, 0),
		Limitations:          make([]string, 0),
	}

	// 1. 完整性评估
	completeness := rp.assessInterfaceCompleteness(result)
	assessment.DataCompleteness = completeness

	// 2. 一致性评估
	consistency := rp.assessInterfaceConsistency(result)
	assessment.Consistency = consistency

	// 3. 可用性评估
	usability := rp.assessInterfaceUsability(result)
	assessment.RecommendationQuality = usability

	// 4. 计算整体质量
	assessment.OverallQuality = (completeness + result.ConfidenceScore + consistency + usability) / 4.0

	// 5. 识别质量问题
	issues := rp.identifyInterfaceQualityIssues(result, assessment, config)
	assessment.QualityIssues = issues

	return assessment, nil
}

// FormatResult 格式化结果 - 实现接口方法
func (rp *ResultProcessor) FormatResult(
	ctx *ctx.Context,
	result *interfaces.AIAnalysisResult,
	format string,
	config *interfaces.FormattingConfig,
) (interface{}, error) {
	
	switch format {
	case "json":
		return result, nil
	case "markdown":
		return rp.formatInterfaceAsMarkdown(result, config)
	case "summary":
		return rp.formatInterfaceAsSummary(result, config)
	default:
		return nil, fmt.Errorf("不支持的格式: %s", format)
	}
}

// ValidateConfig 验证结果处理器配置 - 实现接口方法
func (rp *ResultProcessor) ValidateConfig(config interface{}) error {
	resultProcessorConfig, ok := config.(*ResultProcessorConfig)
	if !ok {
		return fmt.Errorf("配置类型错误，期望 *ResultProcessorConfig，实际 %T", config)
	}
	return validateResultProcessorConfig(resultProcessorConfig)
}

// validateResultProcessorConfig 验证结果处理器配置
func validateResultProcessorConfig(config *ResultProcessorConfig) error {
	if config == nil {
		return fmt.Errorf("配置不能为空")
	}

	if config.MinConfidenceThreshold < 0 || config.MinConfidenceThreshold > 1 {
		return fmt.Errorf("MinConfidenceThreshold必须在0-1范围内")
	}

	if config.MaxProcessingTime <= 0 {
		return fmt.Errorf("MaxProcessingTime必须大于0")
	}

	return nil
}

// assessInterfaceCompleteness 评估接口完整性
func (rp *ResultProcessor) assessInterfaceCompleteness(result *interfaces.AIAnalysisResult) float64 {
	score := 0.0
	totalSections := 5.0 // summary, dataAnalysis, rootCause, recommendations, metadata

	if result.Summary != nil {
		score += 1.0
	}
	if result.DataAnalysis != nil {
		score += 1.0
	}
	if result.RootCauseAnalysis != nil {
		score += 1.0
	}
	if result.Recommendations != nil && len(result.Recommendations) > 0 {
		score += 1.0
	}
	if result.Metadata != nil && len(result.Metadata) > 0 {
		score += 1.0
	}

	return score / totalSections
}

// assessInterfaceConsistency 评估接口一致性
func (rp *ResultProcessor) assessInterfaceConsistency(result *interfaces.AIAnalysisResult) float64 {
	score := 1.0

	// 检查置信度一致性
	if result.Summary != nil && result.RootCauseAnalysis != nil {
		confidenceDiff := math.Abs(result.Summary.Confidence - result.RootCauseAnalysis.Confidence)
		if confidenceDiff > 0.3 {
			score -= 0.2
		}
	}

	// 检查严重程度与建议一致性
	if result.Summary != nil && result.Recommendations != nil {
		if result.Summary.Severity == "critical" && len(result.Recommendations) == 0 {
			score -= 0.3
		}
	}

	return math.Max(0.0, score)
}

// assessInterfaceUsability 评估接口可用性
func (rp *ResultProcessor) assessInterfaceUsability(result *interfaces.AIAnalysisResult) float64 {
	score := 0.0

	// 检查摘要质量
	if result.Summary != nil {
		if result.Summary.Title != "" && len(result.Summary.Title) > 5 {
			score += 0.2
		}
		if result.Summary.Description != "" && len(result.Summary.Description) > 20 {
			score += 0.3
		}
		if len(result.Summary.KeyFindings) > 0 {
			score += 0.2
		}
	}

	// 检查行动建议质量
	if result.Recommendations != nil {
		for _, recommendation := range result.Recommendations {
			if recommendation.Title != "" && recommendation.Rationale != "" {
				score += 0.1
				break
			}
		}
	}

	// 检查根因分析质量
	if result.RootCauseAnalysis != nil {
		if result.RootCauseAnalysis.PrimaryHypothesis != "" {
			score += 0.2
		}
	}

	return math.Min(1.0, score)
}

// identifyInterfaceQualityIssues 识别接口质量问题
func (rp *ResultProcessor) identifyInterfaceQualityIssues(
	result *interfaces.AIAnalysisResult,
	assessment *interfaces.QualityAssessment,
	config *interfaces.QualityConfig,
) []string {
	
	issues := make([]string, 0)

	// 置信度过低
	if assessment.AnalysisAccuracy < 0.7 {
		issues = append(issues, "分析结果置信度较低，建议收集更多数据或使用人工验证")
	}

	// 完整性不足
	if assessment.DataCompleteness < 0.8 {
		issues = append(issues, "分析结果不够完整，建议补充缺失的分析章节")
	}

	// 一致性问题
	if assessment.Consistency < 0.7 {
		issues = append(issues, "分析结果存在内部一致性问题，建议检查和修正不一致的分析结论")
	}

	return issues
}

// formatInterfaceAsMarkdown 格式化接口为Markdown
func (rp *ResultProcessor) formatInterfaceAsMarkdown(result *interfaces.AIAnalysisResult, config *interfaces.FormattingConfig) (string, error) {
	var md strings.Builder

	// 标题
	if result.Summary != nil {
		md.WriteString(fmt.Sprintf("# %s\n\n", result.Summary.Title))
		md.WriteString(fmt.Sprintf("**严重程度**: %s | **类别**: %s | **置信度**: %.2f\n\n",
			result.Summary.Severity, result.Summary.Category, result.Summary.Confidence))
		md.WriteString(fmt.Sprintf("%s\n\n", result.Summary.Description))
	}

	// 关键发现
	if result.Summary != nil && len(result.Summary.KeyFindings) > 0 {
		md.WriteString("## 关键发现\n\n")
		for _, finding := range result.Summary.KeyFindings {
			md.WriteString(fmt.Sprintf("- %s\n", finding))
		}
		md.WriteString("\n")
	}

	// 根因分析
	if result.RootCauseAnalysis != nil {
		md.WriteString("## 根因分析\n\n")
		md.WriteString(fmt.Sprintf("**主要假设**: %s\n\n", result.RootCauseAnalysis.PrimaryHypothesis))
	}

	// 行动建议
	if len(result.Recommendations) > 0 {
		md.WriteString("## 行动建议\n\n")
		for i, recommendation := range result.Recommendations {
			md.WriteString(fmt.Sprintf("### %d. %s\n\n", i+1, recommendation.Title))
			md.WriteString(fmt.Sprintf("**理由**: %s\n\n", recommendation.Rationale))
		}
	}

	return md.String(), nil
}

// formatInterfaceAsSummary 格式化接口为摘要
func (rp *ResultProcessor) formatInterfaceAsSummary(result *interfaces.AIAnalysisResult, config *interfaces.FormattingConfig) (map[string]interface{}, error) {
	summary := make(map[string]interface{})

	if result.Summary != nil {
		summary["title"] = result.Summary.Title
		summary["severity"] = result.Summary.Severity
		summary["category"] = result.Summary.Category
		summary["confidence"] = result.Summary.Confidence
	}

	summary["analysisId"] = result.AnalysisID
	summary["processingTime"] = result.ProcessingTime

	if config.IncludeMetadata {
		summary["metadata"] = result.Metadata
	}

	return summary, nil
}