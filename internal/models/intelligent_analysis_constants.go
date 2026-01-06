package models

// =============================================================================
// 智能分析常量定义 - 用于数据库配置和业务逻辑
// =============================================================================

// 配置类型常量
const (
	ConfigScopeGlobal = "global" // 全局配置
	ConfigScopeTenant = "tenant" // 租户配置  
	ConfigScopeRule   = "rule"   // 规则级配置
)

// =============================================================================
// 全局配置键
// =============================================================================

const (
	// 系统级配置
	ConfigKeyGlobalEnabled         = "global.enabled"          // 智能分析全局开关
	ConfigKeyGlobalAIEngine        = "global.ai_engine"        // AI引擎类型
	ConfigKeyGlobalWorkerPoolSize  = "global.worker_pool_size" // 工作池大小
	ConfigKeyGlobalMaxAnalysisTime = "global.max_analysis_time" // 最大分析时间(秒)
)

// =============================================================================
// 数据采集配置键
// =============================================================================

const (
	ConfigKeyDataCollectionEnabled      = "data_collection.enabled"        // 数据采集开关
	ConfigKeyDataCollectionTimeout      = "data_collection.timeout"        // 数据采集超时时间(秒)
	ConfigKeyDataCollectionMaxMetrics   = "data_collection.max_metrics"    // 最大相关指标数量
	ConfigKeyDataCollectionOptimization = "data_collection.optimization"   // 查询优化开关
	ConfigKeyDataCollectionDatasourceId = "data_collection.datasource_id"  // 默认数据源ID(引用现有数据源)
)

// =============================================================================
// AI引擎配置键
// =============================================================================

const (
	ConfigKeyAIEngineType        = "ai_engine.type"                 // AI引擎类型
	ConfigKeyAIEngineOpenAIModel = "ai_engine.openai_model"         // OpenAI模型名称
	ConfigKeyAIEngineOpenAIKey   = "ai_engine.openai_key"           // OpenAI API密钥
	ConfigKeyAIEngineConfidence  = "ai_engine.confidence_threshold" // 置信度阈值
	ConfigKeyAIEngineTimeout     = "ai_engine.timeout"              // AI分析超时时间(秒)
)

// =============================================================================
// 分析功能配置键
// =============================================================================

const (
	ConfigKeyFeaturesAnomalyDetection    = "features.anomaly_detection"    // 异常检测功能
	ConfigKeyFeaturesTrendAnalysis       = "features.trend_analysis"       // 趋势分析功能
	ConfigKeyFeaturesPatternRecognition  = "features.pattern_recognition"  // 模式识别功能
	ConfigKeyFeaturesCorrelationAnalysis = "features.correlation_analysis" // 关联分析功能
)

// =============================================================================
// 触发条件配置键
// =============================================================================

const (
	ConfigKeyTriggerSeverityLevels = "trigger.severity_levels" // 触发的告警级别(JSON数组字符串)
	ConfigKeyTriggerDataSources    = "trigger.data_sources"    // 支持的数据源类型(JSON数组字符串)
	ConfigKeyTriggerCooldownPeriod = "trigger.cooldown_period" // 冷却时间(秒)
	ConfigKeyTriggerMaxRetries     = "trigger.max_retries"     // 最大重试次数
)

// =============================================================================
// AI引擎类型枚举
// =============================================================================

const (
	AIEngineRuleBased = "rule_based" // 基于规则的分析引擎(默认)
	AIEngineOpenAI    = "openai"     // OpenAI GPT分析引擎
	AIEngineLocal     = "local"      // 本地部署的AI模型
)

// =============================================================================
// 分析状态枚举
// =============================================================================

const (
	AnalysisStatusPending    = "pending"    // 等待分析
	AnalysisStatusAnalyzing  = "analyzing"  // 分析中
	AnalysisStatusCompleted  = "completed"  // 分析完成
	AnalysisStatusFailed     = "failed"     // 分析失败
	AnalysisStatusCancelled  = "cancelled"  // 分析取消
)

// =============================================================================
// 告警级别枚举
// =============================================================================

const (
	SeverityInfo     = "info"     // 信息级别
	SeverityWarning  = "warning"  // 警告级别
	SeverityCritical = "critical" // 严重级别
)

// =============================================================================
// 数据源类型枚举 (引用现有数据源类型)
// =============================================================================

const (
	DatasourcePrometheus = "Prometheus" // Prometheus数据源
	DatasourceInfluxDB   = "InfluxDB"   // InfluxDB数据源
	DatasourceCloudWatch = "CloudWatch" // AWS CloudWatch数据源
)

// =============================================================================
// 默认配置值映射表
// =============================================================================

// GetDefaultConfigValue 获取配置项的默认值
func GetDefaultConfigValue(configKey string) string {
	defaultValues := map[string]string{
		// 全局配置默认值
		ConfigKeyGlobalEnabled:         "false",
		ConfigKeyGlobalAIEngine:        AIEngineRuleBased,
		ConfigKeyGlobalWorkerPoolSize:  "5",
		ConfigKeyGlobalMaxAnalysisTime: "300",
		
		// 数据采集默认值
		ConfigKeyDataCollectionEnabled:      "true",
		ConfigKeyDataCollectionTimeout:      "30",
		ConfigKeyDataCollectionMaxMetrics:   "15",
		ConfigKeyDataCollectionOptimization: "true",
		ConfigKeyDataCollectionDatasourceId: "", // 空值表示使用告警事件关联的数据源
		
		// AI引擎默认值
		ConfigKeyAIEngineType:        AIEngineRuleBased,
		ConfigKeyAIEngineOpenAIModel: "gpt-4",
		ConfigKeyAIEngineConfidence:  "0.6",
		ConfigKeyAIEngineTimeout:     "60",
		
		// 分析功能默认值
		ConfigKeyFeaturesAnomalyDetection:    "true",
		ConfigKeyFeaturesTrendAnalysis:       "true",
		ConfigKeyFeaturesPatternRecognition:  "false",
		ConfigKeyFeaturesCorrelationAnalysis: "false",
		
		// 触发条件默认值
		ConfigKeyTriggerSeverityLevels: `["warning","critical"]`,
		ConfigKeyTriggerDataSources:    `["Prometheus"]`,
		ConfigKeyTriggerCooldownPeriod: "3600",
		ConfigKeyTriggerMaxRetries:     "2",
	}
	
	if value, exists := defaultValues[configKey]; exists {
		return value
	}
	return ""
}

// =============================================================================
// 配置验证规则
// =============================================================================

// IsValidAIEngineType 验证AI引擎类型是否有效
func IsValidAIEngineType(engineType string) bool {
	validTypes := []string{AIEngineRuleBased, AIEngineOpenAI, AIEngineLocal}
	for _, validType := range validTypes {
		if engineType == validType {
			return true
		}
	}
	return false
}

// IsValidAnalysisStatus 验证分析状态是否有效
func IsValidAnalysisStatus(status string) bool {
	validStatuses := []string{
		AnalysisStatusPending,
		AnalysisStatusAnalyzing,
		AnalysisStatusCompleted,
		AnalysisStatusFailed,
		AnalysisStatusCancelled,
	}
	for _, validStatus := range validStatuses {
		if status == validStatus {
			return true
		}
	}
	return false
}

// IsValidSeverity 验证告警级别是否有效
func IsValidSeverity(severity string) bool {
	validSeverities := []string{SeverityInfo, SeverityWarning, SeverityCritical}
	for _, validSeverity := range validSeverities {
		if severity == validSeverity {
			return true
		}
	}
	return false
}

// IsValidConfigScope 验证配置作用域是否有效
func IsValidConfigScope(scope string) bool {
	validScopes := []string{ConfigScopeGlobal, ConfigScopeTenant, ConfigScopeRule}
	for _, validScope := range validScopes {
		if scope == validScope {
			return true
		}
	}
	return false
}