package models

// 智能分析相关的扩展模型

// IntelligentAnalysisRecord 智能分析记录 - 扩展现有AI功能
type IntelligentAnalysisRecord struct {
	AiContentRecord // 继承现有AI内容记录
	
	// 新增智能分析字段
	AnalysisId       string                 `json:"analysisId" gorm:"column:analysis_id"`        // 分析唯一ID
	AnalysisType     string                 `json:"analysisType" gorm:"column:analysis_type"`    // 分析类型：auto/manual/scheduled
	AnalysisMode     string                 `json:"analysisMode" gorm:"column:analysis_mode"`    // 分析模式：basic/deep/comprehensive
	AnalysisStatus   string                 `json:"analysisStatus" gorm:"column:analysis_status"` // 分析状态：pending/analyzing/completed/failed
	AnalyzedAt       int64                  `json:"analyzedAt" gorm:"column:analyzed_at"`        // 分析时间戳
	AnalysisTime     int64                  `json:"analysisTime" gorm:"column:analysis_time"`    // 分析耗时(毫秒)
	
	// 分析配置和上下文（JSON存储）
	AnalysisConfig   map[string]interface{} `json:"analysisConfig" gorm:"column:analysis_config;serializer:json"` // 分析配置
	ContextMetadata  map[string]interface{} `json:"contextMetadata" gorm:"column:context_metadata;serializer:json"` // 上下文元数据
	
	// 质量和反馈
	ConfidenceScore  float64                `json:"confidenceScore" gorm:"column:confidence_score"` // 置信度评分 0-1
	FeedbackScore    float64                `json:"feedbackScore" gorm:"column:feedback_score"`     // 用户反馈评分 0-5
	UserFeedback     string                 `json:"userFeedback" gorm:"column:user_feedback"`       // 用户反馈内容
	
	// 扩展字段
	Tags             []string               `json:"tags" gorm:"column:tags;serializer:json"`         // 分析标签
	Metadata         map[string]interface{} `json:"metadata" gorm:"column:metadata;serializer:json"` // 扩展元数据
	
	// 审计字段
	CreatedAt        int64                  `json:"createdAt" gorm:"column:created_at"`
	UpdatedAt        int64                  `json:"updatedAt" gorm:"column:updated_at"`
}

func (iar IntelligentAnalysisRecord) TableName() string {
	return "w8t_intelligent_analysis_record"
}

// AlertEventAnalysis 告警事件分析状态 - 作为AlertCurEvent的扩展
type AlertEventAnalysis struct {
	// 关联告警事件
	TenantId    string `json:"tenantId"`    // 租户ID
	EventId     string `json:"eventId"`     // 事件ID  
	Fingerprint string `json:"fingerprint"` // 告警指纹
	
	// 智能分析状态
	AnalysisEnabled    bool   `json:"analysisEnabled"`    // 是否启用智能分析
	AnalysisStatus     string `json:"analysisStatus"`     // 当前分析状态
	AnalysisResult     string `json:"analysisResult"`     // 分析结果缓存
	AnalysisId         string `json:"analysisId"`         // 关联的分析记录ID
	LastAnalysisTime   int64  `json:"lastAnalysisTime"`   // 最后分析时间
	AnalysisScore      float64 `json:"analysisScore"`     // 分析评分
	
	// 这些字段不存储到数据库，仅作为运行时缓存
	AnalysisContext    *UniversalAnalysisContext `json:"analysisContext,omitempty" gorm:"-"`    // 分析上下文
	AnalysisMetrics    *AnalysisQualityMetrics   `json:"analysisMetrics,omitempty" gorm:"-"`    // 分析质量指标
}

// UniversalAnalysisContext 通用分析上下文 - 核心数据结构
type UniversalAnalysisContext struct {
	// 基础元信息
	ContextId   string                 `json:"contextId"`   // 上下文唯一ID
	TenantId    string                 `json:"tenantId"`    // 租户ID
	CreatedAt   int64                  `json:"createdAt"`   // 创建时间
	
	// 告警基础信息
	AlertInfo   *AlertBasicInfo        `json:"alertInfo"`   // 告警基本信息
	RuleInfo    *RuleBasicInfo         `json:"ruleInfo"`    // 规则基本信息
	
	// 指标数据（核心）
	PrimaryMetric    *MetricDataSet             `json:"primaryMetric"`    // 主要指标数据
	RelatedMetrics   map[string]*MetricDataSet  `json:"relatedMetrics"`   // 相关指标数据
	MetricFeatures   *UniversalMetricFeatures   `json:"metricFeatures"`   // 通用指标特征
	
	// 系统上下文
	SystemContext    *SystemContextInfo         `json:"systemContext"`    // 系统上下文信息
	TimeContext      *TimeContextInfo           `json:"timeContext"`      // 时间上下文信息
	
	// 分析配置
	AnalysisConfig   *AnalysisConfiguration     `json:"analysisConfig"`   // 分析配置
	
	// 扩展属性
	Extensions       map[string]interface{}     `json:"extensions"`       // 动态扩展字段
}

// AlertBasicInfo 告警基本信息
type AlertBasicInfo struct {
	RuleId          string                 `json:"ruleId"`
	RuleName        string                 `json:"ruleName"`
	Severity        string                 `json:"severity"`
	Fingerprint     string                 `json:"fingerprint"`
	Labels          map[string]interface{} `json:"labels"`
	Annotations     string                 `json:"annotations"`
	TriggerTime     int64                  `json:"triggerTime"`
	Duration        int64                  `json:"duration"`        // 持续时间(秒)
	IsRecovered     bool                   `json:"isRecovered"`
}

// RuleBasicInfo 规则基本信息
type RuleBasicInfo struct {
	RuleId           string                 `json:"ruleId"`
	RuleName         string                 `json:"ruleName"`
	DatasourceType   string                 `json:"datasourceType"`
	PromQL           string                 `json:"promQL"`
	ForDuration      int64                  `json:"forDuration"`
	EvalInterval     int64                  `json:"evalInterval"`
	Threshold        map[string]interface{} `json:"threshold"`       // 阈值配置
	Labels           map[string]string      `json:"labels"`          // 规则标签
}

// MetricDataSet 指标数据集
type MetricDataSet struct {
	MetricName    string                 `json:"metricName"`    // 指标名称
	MetricType    string                 `json:"metricType"`    // 指标类型: counter/gauge/histogram
	Unit          string                 `json:"unit"`          // 单位
	Description   string                 `json:"description"`   // 描述
	
	// 时序数据
	CurrentValue  *DataPoint             `json:"currentValue"`  // 当前值
	TimeSeries    []*DataPoint           `json:"timeSeries"`    // 时序数据点
	
	// 数据质量
	DataQuality   *DataQualityInfo       `json:"dataQuality"`   // 数据质量信息
	
	// 查询信息
	QueryInfo     *QueryInfo             `json:"queryInfo"`     // 查询相关信息
	
	// 元数据
	Metadata      map[string]interface{} `json:"metadata"`      // 动态元数据
}

// DataPoint 数据点
type DataPoint struct {
	Timestamp int64                    `json:"timestamp"` // 时间戳
	Value     float64                  `json:"value"`     // 数值
	Labels    map[string]interface{}   `json:"labels,omitempty"` // 可选标签
	Quality   *DataPointQuality        `json:"quality,omitempty"` // 数据质量
}

// DataPointQuality 数据点质量信息
type DataPointQuality struct {
	IsValid      bool    `json:"isValid"`      // 是否有效
	Confidence   float64 `json:"confidence"`   // 置信度
	Source       string  `json:"source"`       // 数据源
	Anomaly      bool    `json:"anomaly"`      // 是否异常
	AnomalyScore float64 `json:"anomalyScore"` // 异常评分
}

// DataQualityInfo 数据质量信息
type DataQualityInfo struct {
	Completeness     float64               `json:"completeness"`     // 完整性 0-1
	Accuracy         float64               `json:"accuracy"`         // 准确性 0-1
	Timeliness       float64               `json:"timeliness"`       // 及时性 0-1
	TotalPoints      int                   `json:"totalPoints"`      // 总数据点数
	ValidPoints      int                   `json:"validPoints"`      // 有效数据点数
	MissingPoints    int                   `json:"missingPoints"`    // 缺失数据点数
	AnomalyPoints    int                   `json:"anomalyPoints"`    // 异常数据点数
	QualityScore     float64               `json:"qualityScore"`     // 综合质量评分
	Issues           []string              `json:"issues"`           // 质量问题列表
	Metadata         map[string]interface{} `json:"metadata"`        // 质量元数据
}

// QueryInfo 查询信息
type QueryInfo struct {
	OriginalQuery string                 `json:"originalQuery"` // 原始查询语句
	ExecutedQuery string                 `json:"executedQuery"` // 实际执行的查询
	QueryTime     int64                  `json:"queryTime"`     // 查询时间戳
	Duration      int64                  `json:"duration"`      // 查询耗时(ms)
	DataSource    string                 `json:"dataSource"`    // 数据源标识
	CacheHit      bool                   `json:"cacheHit"`      // 是否命中缓存
	ResultSize    int                    `json:"resultSize"`    // 结果大小
}

// UniversalMetricFeatures 通用指标特征
type UniversalMetricFeatures struct {
	// 统计特征
	StatisticalFeatures  *StatisticalFeatures  `json:"statisticalFeatures"`  // 统计特征
	TimeSeriesFeatures   *TimeSeriesFeatures   `json:"timeSeriesFeatures"`   // 时序特征
	DistributionFeatures *DistributionFeatures `json:"distributionFeatures"` // 分布特征
	AnomalyFeatures      *AnomalyFeatures      `json:"anomalyFeatures"`      // 异常特征
	PatternFeatures      *PatternFeatures      `json:"patternFeatures"`      // 模式特征
	
	// 关系特征
	CorrelationFeatures  *CorrelationFeatures  `json:"correlationFeatures"`  // 关联特征
	
	// 动态特征
	DynamicFeatures      map[string]interface{} `json:"dynamicFeatures"`     // 动态计算的特征
}

// StatisticalFeatures 统计特征
type StatisticalFeatures struct {
	Count         int     `json:"count"`         // 数据点个数
	Mean          float64 `json:"mean"`          // 平均值
	Median        float64 `json:"median"`        // 中位数
	StdDev        float64 `json:"stdDev"`        // 标准差
	Variance      float64 `json:"variance"`      // 方差
	Skewness      float64 `json:"skewness"`      // 偏度
	Kurtosis      float64 `json:"kurtosis"`      // 峰度
	Min           float64 `json:"min"`           // 最小值
	Max           float64 `json:"max"`           // 最大值
	Range         float64 `json:"range"`         // 范围
	Q1            float64 `json:"q1"`            // 第一四分位数
	Q3            float64 `json:"q3"`            // 第三四分位数
	IQR           float64 `json:"iqr"`           // 四分位距
	P95           float64 `json:"p95"`           // 95分位数
	P99           float64 `json:"p99"`           // 99分位数
}

// TimeSeriesFeatures 时序特征
type TimeSeriesFeatures struct {
	Trend           string  `json:"trend"`           // 趋势: increasing/decreasing/stable/volatile
	TrendStrength   float64 `json:"trendStrength"`   // 趋势强度 0-1
	Seasonality     bool    `json:"seasonality"`     // 是否有季节性
	SeasonPeriod    int     `json:"seasonPeriod"`    // 季节周期
	Volatility      float64 `json:"volatility"`      // 波动性
	Stationarity    bool    `json:"stationarity"`    // 是否平稳
	AutoCorrelation float64 `json:"autoCorrelation"` // 自相关性
	ChangePoint     []int64 `json:"changePoint"`     // 变点时间戳
	ChangeRate      float64 `json:"changeRate"`      // 变化率
}

// DistributionFeatures 分布特征
type DistributionFeatures struct {
	DistributionType   string    `json:"distributionType"`   // 分布类型: normal/uniform/exponential/etc
	IsNormal           bool      `json:"isNormal"`           // 是否正态分布
	NormalityTest      float64   `json:"normalityTest"`      // 正态性检验p值
	Outliers           []float64 `json:"outliers"`           // 离群值
	OutlierCount       int       `json:"outlierCount"`       // 离群值数量
	OutlierRatio       float64   `json:"outlierRatio"`       // 离群值比例
	DistributionParams map[string]float64 `json:"distributionParams"` // 分布参数
}

// AnomalyFeatures 异常特征
type AnomalyFeatures struct {
	HasAnomalies     bool              `json:"hasAnomalies"`     // 是否存在异常
	AnomalyCount     int               `json:"anomalyCount"`     // 异常点数量
	AnomalyRatio     float64           `json:"anomalyRatio"`     // 异常比例
	AnomalyScore     float64           `json:"anomalyScore"`     // 综合异常评分
	AnomalyTypes     []string          `json:"anomalyTypes"`     // 异常类型列表
	AnomalyTimestamps []int64          `json:"anomalyTimestamps"` // 异常时间戳
	AnomalyDetails   []AnomalyDetail   `json:"anomalyDetails"`   // 异常详情
}

// AnomalyDetail 异常详情
type AnomalyDetail struct {
	Timestamp    int64   `json:"timestamp"`    // 异常时间戳
	Value        float64 `json:"value"`        // 异常值
	AnomalyType  string  `json:"anomalyType"`  // 异常类型
	Severity     string  `json:"severity"`     // 严重程度: low/medium/high/critical
	Score        float64 `json:"score"`        // 异常评分
	Description  string  `json:"description"`  // 异常描述
}

// PatternFeatures 模式特征
type PatternFeatures struct {
	HasPatterns      bool                   `json:"hasPatterns"`      // 是否存在模式
	PatternTypes     []string               `json:"patternTypes"`     // 模式类型
	PatternCount     int                    `json:"patternCount"`     // 模式数量
	DominantPattern  string                 `json:"dominantPattern"`  // 主要模式
	PatternStrength  float64                `json:"patternStrength"`  // 模式强度
	PatternDetails   []PatternDetail        `json:"patternDetails"`   // 模式详情
	CyclicalPatterns []CyclicalPattern      `json:"cyclicalPatterns"` // 周期性模式
	
	// 趋势特征
	TrendType        string                 `json:"trendType"`        // 趋势类型: increasing/decreasing/stable
	TrendStrength    float64                `json:"trendStrength"`    // 趋势强度
	
	// 季节性特征
	SeasonalityType  string                 `json:"seasonalityType"`  // 季节性类型
	SeasonalPeriod   int                    `json:"seasonalPeriod"`   // 季节性周期
	
	// 周期性特征
	CyclicalType     string                 `json:"cyclicalType"`     // 周期类型
	CyclePeriod      int64                  `json:"cyclePeriod"`      // 周期长度
	
	// 置信度
	Confidence       float64                `json:"confidence"`       // 模式识别置信度
}

// PatternDetail 模式详情
type PatternDetail struct {
	PatternType   string  `json:"patternType"`   // 模式类型
	StartTime     int64   `json:"startTime"`     // 开始时间
	EndTime       int64   `json:"endTime"`       // 结束时间
	Duration      int64   `json:"duration"`      // 持续时间
	Strength      float64 `json:"strength"`      // 模式强度
	Description   string  `json:"description"`   // 模式描述
}

// CyclicalPattern 周期性模式
type CyclicalPattern struct {
	Period       int64   `json:"period"`       // 周期长度(秒)
	Amplitude    float64 `json:"amplitude"`    // 振幅
	Phase        float64 `json:"phase"`        // 相位
	Confidence   float64 `json:"confidence"`   // 置信度
	Description  string  `json:"description"`  // 周期描述
}

// CorrelationFeatures 关联特征
type CorrelationFeatures struct {
	SelfCorrelation     float64                        `json:"selfCorrelation"`     // 自相关性
	CrossCorrelations   map[string]*CorrelationInfo    `json:"crossCorrelations"`   // 与其他指标的相关性
	LeadLagRelations    map[string]*LeadLagRelation    `json:"leadLagRelations"`    // 领先滞后关系
	CausalRelations     map[string]*CausalRelation     `json:"causalRelations"`     // 因果关系
	LagAnalysis         map[string]*LagAnalysisResult  `json:"lagAnalysis"`         // 滞后分析结果
}

// CorrelationInfo 相关性信息
type CorrelationInfo struct {
	TargetMetric    string  `json:"targetMetric"`    // 目标指标
	Correlation     float64 `json:"correlation"`     // 相关系数
	PValue          float64 `json:"pValue"`          // p值
	Significance    string  `json:"significance"`    // 显著性级别
	Method          string  `json:"method"`          // 计算方法
	DataPoints      int     `json:"dataPoints"`      // 数据点数
	Confidence      float64 `json:"confidence"`      // 置信度
}

// LeadLagRelation 领先滞后关系
type LeadLagRelation struct {
	TargetMetric    string  `json:"targetMetric"`    // 目标指标
	Relationship    string  `json:"relationship"`    // 关系类型: lead/lag
	TimeOffset      int64   `json:"timeOffset"`      // 时间偏移(秒)
	Correlation     float64 `json:"correlation"`     // 相关系数
	Confidence      float64 `json:"confidence"`      // 置信度
}

// CausalRelation 因果关系
type CausalRelation struct {
	TargetMetric    string  `json:"targetMetric"`    // 目标指标
	CausalDirection string  `json:"causalDirection"` // 因果方向: causes/caused_by/bidirectional
	CausalStrength  float64 `json:"causalStrength"`  // 因果强度
	Confidence      float64 `json:"confidence"`      // 置信度
	Method          string  `json:"method"`          // 检测方法
}

// SystemContextInfo 系统上下文信息
type SystemContextInfo struct {
	// 基础信息
	Environment     string                 `json:"environment"`     // 环境: prod/staging/dev
	Region          string                 `json:"region"`          // 地区
	Cluster         string                 `json:"cluster"`         // 集群
	Namespace       string                 `json:"namespace"`       // 命名空间
	
	// 服务信息
	ServiceName     string                 `json:"serviceName"`     // 服务名称
	ServiceType     string                 `json:"serviceType"`     // 服务类型
	ServiceVersion  string                 `json:"serviceVersion"`  // 服务版本
	
	// 基础设施信息
	Infrastructure  *InfrastructureInfo    `json:"infrastructure"`  // 基础设施信息
	
	// 标签信息
	Labels          map[string]string      `json:"labels"`          // 原始标签
	EnrichedLabels  map[string]interface{} `json:"enrichedLabels"`  // 增强标签
}

// InfrastructureInfo 基础设施信息
type InfrastructureInfo struct {
	Provider       string                 `json:"provider"`       // 云服务商
	InstanceType   string                 `json:"instanceType"`   // 实例类型
	InstanceId     string                 `json:"instanceId"`     // 实例ID
	AvailabilityZone string               `json:"availabilityZone"` // 可用区
	CPU            string                 `json:"cpu"`            // CPU配置
	Memory         string                 `json:"memory"`         // 内存配置
	Storage        string                 `json:"storage"`        // 存储配置
	Network        string                 `json:"network"`        // 网络配置
	Metadata       map[string]interface{} `json:"metadata"`       // 基础设施元数据
}

// TimeContextInfo 时间上下文信息
type TimeContextInfo struct {
	AnalysisTime        int64                  `json:"analysisTime"`        // 分析时间
	EventTime           int64                  `json:"eventTime"`           // 事件时间
	TimeZone            string                 `json:"timeZone"`            // 时区
	IsBusinessHours     bool                   `json:"isBusinessHours"`     // 是否业务时间
	BusinessHoursInfo   *BusinessHoursInfo     `json:"businessHoursInfo"`   // 业务时间信息
	SeasonalContext     *SeasonalContext       `json:"seasonalContext"`     // 季节性上下文
	HistoricalContext   *HistoricalContext     `json:"historicalContext"`   // 历史上下文
	TimeRanges          map[string]*TimeRange  `json:"timeRanges"`          // 分析时间范围
}

// BusinessHoursInfo 业务时间信息
type BusinessHoursInfo struct {
	Period         string `json:"period"`         // 时段: business/off_hours/weekend
	Description    string `json:"description"`    // 时段描述
	IsHoliday      bool   `json:"isHoliday"`      // 是否节假日
	IsPeakHours    bool   `json:"isPeakHours"`    // 是否高峰期
	ExpectedLoad   string `json:"expectedLoad"`   // 预期负载: high/medium/low
}

// SeasonalContext 季节性上下文
type SeasonalContext struct {
	Season         string  `json:"season"`         // 季节
	Month          int     `json:"month"`          // 月份
	DayOfWeek      int     `json:"dayOfWeek"`      // 星期几
	HourOfDay      int     `json:"hourOfDay"`      // 小时
	IsWeekend      bool    `json:"isWeekend"`      // 是否周末
	SeasonalFactor float64 `json:"seasonalFactor"` // 季节性因子
}

// HistoricalContext 历史上下文
type HistoricalContext struct {
	SamePeriodLastWeek  *PeriodComparison `json:"samePeriodLastWeek"`  // 上周同期对比
	SamePeriodLastMonth *PeriodComparison `json:"samePeriodLastMonth"` // 上月同期对比
	SamePeriodLastYear  *PeriodComparison `json:"samePeriodLastYear"`  // 去年同期对比
	TrendComparison     *TrendComparison  `json:"trendComparison"`     // 趋势对比
}

// PeriodComparison 时期对比
type PeriodComparison struct {
	PeriodName     string  `json:"periodName"`     // 时期名称
	BaselineValue  float64 `json:"baselineValue"`  // 基线值
	CurrentValue   float64 `json:"currentValue"`   // 当前值
	ChangeRatio    float64 `json:"changeRatio"`    // 变化比例
	ChangeDirection string `json:"changeDirection"` // 变化方向: increase/decrease/stable
	IsSignificant  bool    `json:"isSignificant"`  // 变化是否显著
}

// TrendComparison 趋势对比
type TrendComparison struct {
	ShortTermTrend  string  `json:"shortTermTrend"`  // 短期趋势
	MediumTermTrend string  `json:"mediumTermTrend"` // 中期趋势
	LongTermTrend   string  `json:"longTermTrend"`   // 长期趋势
	TrendStrength   float64 `json:"trendStrength"`   // 趋势强度
	TrendConsistency float64 `json:"trendConsistency"` // 趋势一致性
}

// TimeRange 时间范围
type TimeRange struct {
	Name      string `json:"name"`      // 范围名称
	StartTime int64  `json:"startTime"` // 开始时间
	EndTime   int64  `json:"endTime"`   // 结束时间
	Duration  int64  `json:"duration"`  // 持续时间(秒)
	Purpose   string `json:"purpose"`   // 用途说明
}

// AnalysisConfiguration 分析配置
type AnalysisConfiguration struct {
	// 分析类型和模式
	AnalysisType           string                 `json:"analysisType"`           // 分析类型: auto/manual/scheduled
	AnalysisMode           string                 `json:"analysisMode"`           // 分析模式: basic/deep/comprehensive
	AnalysisScope          string                 `json:"analysisScope"`          // 分析范围: current/historical/predictive
	
	// 数据收集配置
	DataCollectionConfig   *DataCollectionConfig  `json:"dataCollectionConfig"`   // 数据收集配置
	
	// AI分析配置
	AiAnalysisConfig       *AiAnalysisConfig      `json:"aiAnalysisConfig"`       // AI分析配置
	
	// 特征提取配置
	FeatureExtractionConfig *FeatureExtractionConfig `json:"featureExtractionConfig"` // 特征提取配置
	
	// 质量控制配置
	QualityControlConfig   *QualityControlConfig  `json:"qualityControlConfig"`   // 质量控制配置
	
	// 自定义配置
	CustomConfig           map[string]interface{} `json:"customConfig"`           // 自定义配置
}

// DataCollectionConfig 数据收集配置
type DataCollectionConfig struct {
	TimeRanges           map[string]string      `json:"timeRanges"`           // 时间范围配置
	MaxRelatedMetrics    int                    `json:"maxRelatedMetrics"`    // 最大相关指标数
	ParallelQueryLimit   int                    `json:"parallelQueryLimit"`   // 并行查询限制
	QueryTimeout         string                 `json:"queryTimeout"`         // 查询超时时间
	CacheTTL             string                 `json:"cacheTTL"`             // 缓存TTL
	DataQualityThreshold float64                `json:"dataQualityThreshold"` // 数据质量阈值
	DiscoveryStrategies  []string               `json:"discoveryStrategies"`  // 发现策略列表
	CustomQueries        map[string]string      `json:"customQueries"`        // 自定义查询
}

// AiAnalysisConfig AI分析配置
type AiAnalysisConfig struct {
	AiModel               string                 `json:"aiModel"`               // AI模型名称
	MaxTokens             int                    `json:"maxTokens"`             // 最大Token数
	Temperature           float64                `json:"temperature"`           // 温度参数
	AnalysisDepth         string                 `json:"analysisDepth"`         // 分析深度: shallow/medium/deep
	ContextOptimization   bool                   `json:"contextOptimization"`   // 是否优化上下文
	MultiRoundAnalysis    bool                   `json:"multiRoundAnalysis"`    // 是否多轮分析
	PromptTemplate        string                 `json:"promptTemplate"`        // 提示模板
	ResponseFormat        string                 `json:"responseFormat"`        // 响应格式
	QualityValidation     bool                   `json:"qualityValidation"`     // 是否质量验证
	FallbackStrategy      string                 `json:"fallbackStrategy"`      // 降级策略
	CustomPromptParams    map[string]interface{} `json:"customPromptParams"`    // 自定义提示参数
}

// FeatureExtractionConfig 特征提取配置
type FeatureExtractionConfig struct {
	EnabledFeatures       []string               `json:"enabledFeatures"`       // 启用的特征类型
	StatisticalConfig     map[string]interface{} `json:"statisticalConfig"`     // 统计特征配置
	TimeSeriesConfig      map[string]interface{} `json:"timeSeriesConfig"`      // 时序特征配置
	AnomalyDetectionConfig map[string]interface{} `json:"anomalyDetectionConfig"` // 异常检测配置
	PatternRecognitionConfig map[string]interface{} `json:"patternRecognitionConfig"` // 模式识别配置
	CorrelationConfig     map[string]interface{} `json:"correlationConfig"`     // 相关性分析配置
	CustomFeatureConfig   map[string]interface{} `json:"customFeatureConfig"`   // 自定义特征配置
}

// QualityControlConfig 质量控制配置
type QualityControlConfig struct {
	MinConfidenceScore    float64 `json:"minConfidenceScore"`    // 最小置信度要求
	MaxAnalysisTime       int64   `json:"maxAnalysisTime"`       // 最大分析时间(ms)
	RequiredEvidenceCount int     `json:"requiredEvidenceCount"` // 要求的证据数量
	DataCompletenessThreshold float64 `json:"dataCompletenessThreshold"` // 数据完整性阈值
	ValidationRules       []string `json:"validationRules"`      // 验证规则列表
	FallbackOnFailure     bool    `json:"fallbackOnFailure"`     // 失败时是否降级
}

// LagAnalysisResult 滞后分析结果
type LagAnalysisResult struct {
	SourceMetric       string             `json:"sourceMetric"`       // 源指标名称
	TargetMetric       string             `json:"targetMetric"`       // 目标指标名称
	OptimalLag         int64              `json:"optimalLag"`         // 最优滞后时间(秒)
	MaxCorrelation     float64            `json:"maxCorrelation"`     // 最大相关系数
	Confidence         float64            `json:"confidence"`         // 置信度
	LagType            string             `json:"lagType"`            // 滞后类型: positive/negative
	Significance       string             `json:"significance"`       // 显著性: high/medium/low
	Description        string             `json:"description"`        // 滞后关系描述
	
	// 兼容现有代码的字段
	BestLag            int64              `json:"bestLag"`            // 别名: OptimalLag
	BestCorrelation    float64            `json:"bestCorrelation"`    // 别名: MaxCorrelation
	LagCorrelations    map[int64]float64  `json:"lagCorrelations"`    // 不同滞后的相关性
}

// AnalysisQualityMetrics 分析质量指标
type AnalysisQualityMetrics struct {
	OverallScore         float64   `json:"overallScore"`         // 综合评分 0-1
	ConfidenceScore      float64   `json:"confidenceScore"`      // 置信度评分 0-1
	DataCompletenessScore float64  `json:"dataCompletenessScore"` // 数据完整性评分 0-1
	ConsistencyScore     float64   `json:"consistencyScore"`     // 一致性评分 0-1
	TimelinessScore      float64   `json:"timelinessScore"`      // 及时性评分 0-1
	CoverageScore        float64   `json:"coverageScore"`        // 覆盖度评分 0-1
	
	// 详细指标
	AnalysisTime         int64     `json:"analysisTime"`         // 分析耗时(ms)
	DataPointsAnalyzed   int       `json:"dataPointsAnalyzed"`   // 分析的数据点数
	MetricsAnalyzed      int       `json:"metricsAnalyzed"`      // 分析的指标数
	FeaturesExtracted    int       `json:"featuresExtracted"`    // 提取的特征数
	EvidenceFound        int       `json:"evidenceFound"`        // 找到的证据数
	AnomaliesDetected    int       `json:"anomaliesDetected"`    // 检测到的异常数
	PatternsIdentified   int       `json:"patternsIdentified"`   // 识别的模式数
	
	// 质量问题
	QualityIssues        []string  `json:"qualityIssues"`        // 质量问题列表
	Warnings             []string  `json:"warnings"`             // 警告信息
	Limitations          []string  `json:"limitations"`          // 局限性说明
}