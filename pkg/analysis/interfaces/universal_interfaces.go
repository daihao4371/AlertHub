package interfaces

import (
	"alertHub/internal/ctx"
	"alertHub/internal/models"
	"alertHub/internal/types"
)

// ================================
// 通用化接口定义 - 消除硬编码依赖
// ================================

// UniversalDataCollector 通用数据采集器接口
// 完全数据驱动，无任何指标类型假设
type UniversalDataCollector interface {
	// CollectContext 采集通用分析上下文
	// 基于配置动态发现相关指标，无硬编码业务逻辑
	CollectContext(ctx *ctx.Context, request *types.RequestDataCollection) (*types.ResponseDataCollection, error)
	
	// DiscoverRelatedMetrics 动态发现相关指标
	// 基于标签相似度和拓扑关系，完全配置驱动
	DiscoverRelatedMetrics(ctx *ctx.Context, primaryMetric *MetricDescriptor, config *DiscoveryConfig) ([]*MetricDescriptor, error)
	
	// ValidateConfig 验证采集器配置
	ValidateConfig(config interface{}) error
}

// UniversalDataStandardizer 通用数据标准化器接口
// 通用特征提取，适用于任何指标类型
type UniversalDataStandardizer interface {
	// StandardizeContext 标准化分析上下文
	// 纯数学/统计特征提取，无业务假设
	StandardizeContext(ctx *ctx.Context, context *models.UniversalAnalysisContext) (*models.UniversalAnalysisContext, error)
	
	// ExtractFeatures 提取通用特征
	// 基于配置选择特征类型，支持动态扩展
	ExtractFeatures(ctx *ctx.Context, metricData *models.MetricDataSet, config *FeatureExtractionConfig) (map[string]interface{}, error)
	
	// ValidateConfig 验证标准化器配置
	ValidateConfig(config interface{}) error
}

// UniversalAIEngine AI分析引擎接口
// 纯推理驱动，让AI完全自主分析
type UniversalAIEngine interface {
	// Analyze 执行AI分析
	// 动态生成提示，支持多轮对话增强分析
	Analyze(ctx *ctx.Context, context *models.UniversalAnalysisContext, request *AIAnalysisRequest) (*AIAnalysisResult, error)
	
	// GeneratePrompt 动态生成分析提示
	// 基于上下文自适应生成，无固定模板
	GeneratePrompt(ctx *ctx.Context, context *models.UniversalAnalysisContext, request *AIAnalysisRequest) (string, error)
	
	// ParseResponse 解析AI响应
	// 通用解析器，支持各种响应格式
	ParseResponse(ctx *ctx.Context, response string, context *models.UniversalAnalysisContext) (*AIAnalysisResult, error)
	
	// ValidateConfig 验证AI引擎配置
	ValidateConfig(config interface{}) error
}

// UniversalResultProcessor 通用结果处理器接口
// 标准化结果处理，无业务逻辑硬编码
type UniversalResultProcessor interface {
	// Process 处理分析结果
	// 通用后处理，支持配置化的质量验证和格式化
	Process(ctx *ctx.Context, analysisResult *AIAnalysisResult, context *models.UniversalAnalysisContext) (*ProcessingResult, error)
	
	// ValidateQuality 验证结果质量
	// 基于配置的质量阈值，动态验证
	ValidateQuality(ctx *ctx.Context, result *AIAnalysisResult, config *QualityConfig) (*QualityAssessment, error)
	
	// FormatResult 格式化结果
	// 支持多种输出格式，配置驱动
	FormatResult(ctx *ctx.Context, result *AIAnalysisResult, format string, config *FormattingConfig) (interface{}, error)
	
	// ValidateConfig 验证结果处理器配置
	ValidateConfig(config interface{}) error
}

// ConfigurableStrategyEngine 可配置策略引擎接口
// 所有策略都通过配置文件管理，支持运行时动态加载
type ConfigurableStrategyEngine interface {
	// LoadStrategy 加载策略
	// 从配置文件动态加载策略实现
	LoadStrategy(ctx *ctx.Context, strategyType string, config map[string]interface{}) (Strategy, error)
	
	// ExecuteStrategy 执行策略
	// 通用策略执行框架
	ExecuteStrategy(ctx *ctx.Context, strategy Strategy, input interface{}) (interface{}, error)
	
	// RegisterStrategy 注册策略
	// 支持运行时策略注册和更新
	RegisterStrategy(strategyType string, builder StrategyBuilder) error
	
	// ValidateStrategyConfig 验证策略配置
	ValidateStrategyConfig(strategyType string, config map[string]interface{}) error
}

// ================================
// 通用数据结构
// ================================

// MetricDescriptor 指标描述符
// 通用的指标描述，无特定业务假设
type MetricDescriptor struct {
	Name          string                 `json:"name"`          // 指标名称
	Type          string                 `json:"type"`          // 指标类型
	Labels        map[string]string      `json:"labels"`        // 标签
	Query         string                 `json:"query"`         // 查询语句
	Importance    float64                `json:"importance"`    // 重要性权重
	Metadata      map[string]interface{} `json:"metadata"`      // 动态元数据
}

// DiscoveryConfig 发现配置
// 完全配置化的发现策略
type DiscoveryConfig struct {
	Strategies          []StrategyConfig       `json:"strategies"`          // 发现策略列表
	MaxRelatedMetrics   int                    `json:"maxRelatedMetrics"`   // 最大相关指标数
	SimilarityThreshold float64                `json:"similarityThreshold"` // 相似度阈值
	TimeWindow          string                 `json:"timeWindow"`          // 时间窗口
	ParallelQueries     int                    `json:"parallelQueries"`     // 并行查询数
	CacheConfig         *CacheConfig           `json:"cacheConfig"`         // 缓存配置
}

// FeatureExtractionConfig 特征提取配置
// 配置化的特征提取策略
type FeatureExtractionConfig struct {
	EnabledFeatures    []string               `json:"enabledFeatures"`    // 启用的特征类型
	StatisticalConfig  map[string]interface{} `json:"statisticalConfig"`  // 统计特征配置
	TimeSeriesConfig   map[string]interface{} `json:"timeSeriesConfig"`   // 时序特征配置
	AnomalyConfig      map[string]interface{} `json:"anomalyConfig"`      // 异常检测配置
	PatternConfig      map[string]interface{} `json:"patternConfig"`      // 模式识别配置
	CorrelationConfig  map[string]interface{} `json:"correlationConfig"`  // 相关性分析配置
	CustomFeatures     map[string]interface{} `json:"customFeatures"`     // 自定义特征配置
}

// AIAnalysisRequest AI分析请求
// 配置化的AI分析参数
type AIAnalysisRequest struct {
	AnalysisType     string                 `json:"analysisType"`     // 分析类型
	AnalysisMode     string                 `json:"analysisMode"`     // 分析模式
	AnalysisDepth    string                 `json:"analysisDepth"`    // 分析深度
	FocusAreas       []string               `json:"focusAreas"`       // 关注领域
	PromptTemplate   string                 `json:"promptTemplate"`   // 提示模板
	PromptParams     map[string]interface{} `json:"promptParams"`     // 提示参数
	ModelConfig      *ModelConfig           `json:"modelConfig"`      // 模型配置
	QualityRequirements *QualityConfig      `json:"qualityRequirements"` // 质量要求
}

// AIAnalysisResult AI分析结果
// 标准化的AI分析输出
type AIAnalysisResult struct {
	AnalysisID        string                 `json:"analysisId"`        // 分析唯一ID
	AnalysisType      string                 `json:"analysisType"`      // 分析类型
	Timestamp         int64                  `json:"timestamp"`         // 分析时间戳
	ConfidenceScore   float64                `json:"confidenceScore"`   // 置信度评分
	
	// 分析内容
	Summary           *AnalysisSummary       `json:"summary"`           // 分析摘要
	DataAnalysis      *DataAnalysisResult    `json:"dataAnalysis"`      // 数据分析
	RootCauseAnalysis *RootCauseAnalysis     `json:"rootCauseAnalysis"` // 根因分析
	Recommendations   []*Recommendation      `json:"recommendations"`   // 建议
	
	// 元数据
	ProcessingTime    int64                  `json:"processingTime"`    // 处理耗时(ms)
	TokenUsage        *TokenUsage            `json:"tokenUsage"`        // Token使用情况
	ModelInfo         *ModelInfo             `json:"modelInfo"`         // 模型信息
	Metadata          map[string]interface{} `json:"metadata"`          // 扩展元数据
}

// ProcessingResult 处理结果
// 标准化的处理输出
type ProcessingResult struct {
	ProcessingID       string                 `json:"processingId"`       // 处理唯一ID
	ProcessingStatus   string                 `json:"processingStatus"`   // 处理状态
	ProcessedResult    *AIAnalysisResult      `json:"processedResult"`    // 处理后的分析结果
	QualityAssessment  *QualityAssessment     `json:"qualityAssessment"`  // 质量评估
	FormattedOutput    map[string]interface{} `json:"formattedOutput"`    // 格式化输出
	ProcessingMetadata *ProcessingMetadata    `json:"processingMetadata"` // 处理元数据
}

// StrategyConfig 策略配置
// 通用策略配置结构
type StrategyConfig struct {
	Name       string                 `json:"name"`       // 策略名称
	Type       string                 `json:"type"`       // 策略类型
	Enabled    bool                   `json:"enabled"`    // 是否启用
	Weight     float64                `json:"weight"`     // 权重
	Config     map[string]interface{} `json:"config"`     // 策略具体配置
	Metadata   map[string]interface{} `json:"metadata"`   // 策略元数据
}

// Strategy 通用策略接口
type Strategy interface {
	// Execute 执行策略
	Execute(ctx *ctx.Context, input interface{}) (interface{}, error)
	
	// GetName 获取策略名称
	GetName() string
	
	// GetType 获取策略类型
	GetType() string
	
	// Validate 验证输入参数
	Validate(input interface{}) error
}

// StrategyBuilder 策略构建器接口
type StrategyBuilder interface {
	// Build 构建策略实例
	Build(config map[string]interface{}) (Strategy, error)
	
	// ValidateConfig 验证配置
	ValidateConfig(config map[string]interface{}) error
	
	// GetStrategyType 获取策略类型
	GetStrategyType() string
}

// ================================
// 配置结构定义
// ================================

// CacheConfig 缓存配置
type CacheConfig struct {
	Enabled    bool   `json:"enabled"`    // 是否启用缓存
	TTL        string `json:"ttl"`        // 缓存TTL
	MaxSize    int    `json:"maxSize"`    // 最大缓存大小
	Strategy   string `json:"strategy"`   // 缓存策略
}

// ModelConfig AI模型配置
type ModelConfig struct {
	Name         string  `json:"name"`         // 模型名称
	Version      string  `json:"version"`      // 模型版本
	MaxTokens    int     `json:"maxTokens"`    // 最大Token数
	Temperature  float64 `json:"temperature"`  // 温度参数
	TopP         float64 `json:"topP"`         // TopP参数
	FrequencyPenalty float64 `json:"frequencyPenalty"` // 频率惩罚
	PresencePenalty  float64 `json:"presencePenalty"`  // 存在惩罚
}

// QualityConfig 质量配置
type QualityConfig struct {
	MinConfidence      float64  `json:"minConfidence"`      // 最小置信度
	MaxProcessingTime  int64    `json:"maxProcessingTime"`  // 最大处理时间
	RequiredEvidence   int      `json:"requiredEvidence"`   // 要求的证据数量
	ValidationRules    []string `json:"validationRules"`    // 验证规则
	QualityThresholds  map[string]float64 `json:"qualityThresholds"` // 质量阈值
}

// FormattingConfig 格式化配置
type FormattingConfig struct {
	OutputFormat    string                 `json:"outputFormat"`    // 输出格式
	IncludeMetadata bool                   `json:"includeMetadata"` // 是否包含元数据
	Compression     bool                   `json:"compression"`     // 是否压缩
	CustomFields    map[string]interface{} `json:"customFields"`    // 自定义字段
}

// ================================
// 结果数据结构
// ================================

// AnalysisSummary 分析摘要
type AnalysisSummary struct {
	Title         string   `json:"title"`         // 分析标题
	Description   string   `json:"description"`   // 分析描述
	Severity      string   `json:"severity"`      // 严重程度
	Category      string   `json:"category"`      // 分析类别
	Confidence    float64  `json:"confidence"`    // 置信度
	KeyFindings   []string `json:"keyFindings"`   // 关键发现
}

// DataAnalysisResult 数据分析结果
type DataAnalysisResult struct {
	PrimaryMetricAnalysis   map[string]interface{} `json:"primaryMetricAnalysis"`   // 主要指标分析
	RelationshipAnalysis    map[string]interface{} `json:"relationshipAnalysis"`    // 关系分析
	SystemAnalysis          map[string]interface{} `json:"systemAnalysis"`          // 系统分析
	TrendAnalysis           map[string]interface{} `json:"trendAnalysis"`           // 趋势分析
	AnomalyAnalysis         map[string]interface{} `json:"anomalyAnalysis"`         // 异常分析
}

// RootCauseAnalysis 根因分析
type RootCauseAnalysis struct {
	PrimaryHypothesis       string              `json:"primaryHypothesis"`       // 主要假设
	SupportingEvidence      []*Evidence         `json:"supportingEvidence"`      // 支持证据
	AlternativeHypotheses   []string            `json:"alternativeHypotheses"`   // 其他假设
	CausalChain             []*CausalLink       `json:"causalChain"`             // 因果链
	Confidence              float64             `json:"confidence"`              // 置信度
}

// Evidence 证据
type Evidence struct {
	Type        string                 `json:"type"`        // 证据类型
	Description string                 `json:"description"` // 证据描述
	Strength    float64                `json:"strength"`    // 证据强度
	Source      string                 `json:"source"`      // 证据来源
	Data        map[string]interface{} `json:"data"`        // 证据数据
}

// CausalLink 因果链环节
type CausalLink struct {
	Cause       string  `json:"cause"`       // 原因
	Effect      string  `json:"effect"`      // 结果
	Strength    float64 `json:"strength"`    // 关系强度
	Confidence  float64 `json:"confidence"`  // 置信度
	Evidence    []*Evidence `json:"evidence"` // 支持证据
}

// Recommendation 建议
type Recommendation struct {
	Priority        int                    `json:"priority"`        // 优先级
	Type            string                 `json:"type"`            // 建议类型
	Title           string                 `json:"title"`           // 建议标题
	Description     string                 `json:"description"`     // 建议描述
	Rationale       string                 `json:"rationale"`       // 理由
	Steps           []*ActionStep          `json:"steps"`           // 执行步骤
	ExpectedOutcome string                 `json:"expectedOutcome"` // 预期结果
	RiskAssessment  string                 `json:"riskAssessment"`  // 风险评估
	SuccessMetrics  []string               `json:"successMetrics"`  // 成功指标
}

// ActionStep 行动步骤
type ActionStep struct {
	Order             int    `json:"order"`             // 步骤顺序
	Action            string `json:"action"`            // 具体行动
	Verification      string `json:"verification"`      // 验证方法
	ExpectedOutcome   string `json:"expectedOutcome"`   // 预期结果
	EstimatedDuration string `json:"estimatedDuration"` // 预计耗时
}

// QualityAssessment 质量评估
type QualityAssessment struct {
	OverallQuality     float64  `json:"overallQuality"`     // 综合质量评分
	DataCompleteness   float64  `json:"dataCompleteness"`   // 数据完整性
	AnalysisAccuracy   float64  `json:"analysisAccuracy"`   // 分析准确性
	RecommendationQuality float64 `json:"recommendationQuality"` // 建议质量
	Timeliness         float64  `json:"timeliness"`         // 及时性
	Consistency        float64  `json:"consistency"`        // 一致性
	QualityIssues      []string `json:"qualityIssues"`      // 质量问题
	Warnings           []string `json:"warnings"`           // 警告
	Limitations        []string `json:"limitations"`        // 局限性
}

// ProcessingMetadata 处理元数据
type ProcessingMetadata struct {
	ProcessingTime    int64                  `json:"processingTime"`    // 处理时间
	ResourceUsage     map[string]interface{} `json:"resourceUsage"`     // 资源使用情况
	PerformanceMetrics map[string]interface{} `json:"performanceMetrics"` // 性能指标
	ConfigUsed        map[string]interface{} `json:"configUsed"`        // 使用的配置
	VersionInfo       map[string]interface{} `json:"versionInfo"`       // 版本信息
}

// TokenUsage Token使用情况
type TokenUsage struct {
	PromptTokens     int `json:"promptTokens"`     // 提示Token数
	CompletionTokens int `json:"completionTokens"` // 完成Token数
	TotalTokens      int `json:"totalTokens"`      // 总Token数
}

// ModelInfo 模型信息
type ModelInfo struct {
	Name        string `json:"name"`        // 模型名称
	Version     string `json:"version"`     // 模型版本
	Provider    string `json:"provider"`    // 提供商
	ApiVersion  string `json:"apiVersion"`  // API版本
}