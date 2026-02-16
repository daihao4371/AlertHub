package types

// ClusterRequestDto RCA 聚类 API 请求参数
type ClusterRequestDto struct {
	AlertEventIds       []string `json:"alertEventIds" binding:"required,min=1,max=50"` // 告警事件 ID 列表，最多 50 条
	IncludeCmdbContext  bool     `json:"includeCmdbContext"`                            // 是否包含 CMDB 上下文
	IncludeTopologyData bool     `json:"includeTopologyData"`                           // 是否包含拓扑依赖数据
}

// ClusteringSuggestion 完整聚类建议响应
type ClusteringSuggestion struct {
	SuggestionId string                 `json:"suggestionId"`
	Incidents    []IncidentCandidateDto `json:"incidents"`
	FromCache    bool                   `json:"fromCache"`   // 是否来自缓存
	Model        string                 `json:"model"`       // 使用的模型
	TotalTokens  int                    `json:"totalTokens"` // Token 消耗
	CreatedAt    int64                  `json:"createdAt"`
}

// IncidentCandidateDto 事件候选 DTO
type IncidentCandidateDto struct {
	ID                    string   `json:"id"`
	Name                  string   `json:"name"`
	Severity              string   `json:"severity"`
	ConfidenceScore       float64  `json:"confidenceScore"`
	ConfidenceExplanation string   `json:"confidenceExplanation"`
	Reasoning             string   `json:"reasoning"`
	AlertCount            int      `json:"alertCount"`
	AffectedServices      []string `json:"affectedServices"`
	AlertSources          []string `json:"alertSources"`
	RecommendedActions    []string `json:"recommendedActions"`
	RootCauseSummary      string   `json:"rootCauseSummary"`
	StartTime             int64    `json:"startTime"`
	LastSeenTime          int64    `json:"lastSeenTime"`
	Status                string   `json:"status"`
}

// CommitRequestDto 用户提交反馈请求
type CommitRequestDto struct {
	Incidents    []IncidentFeedback `json:"incidents"`
	OverallScore int                `json:"overallScore" binding:"min=1,max=5"` // 1-5 评分
}

// IncidentFeedback 单个事件的反馈
type IncidentFeedback struct {
	IncidentId      string `json:"incidentId" binding:"required"`
	Accepted        bool   `json:"accepted"`
	FeedbackComment string `json:"feedbackComment"`
}

// ClusteringResponse AI 返回的聚类结果
// 对标 Keep: keep/api/models/incident.py#IncidentClustering
type ClusteringResponse struct {
	Incidents  []IncidentCandidate `json:"incidents"`
	TokensUsed int                 `json:"tokens_used,omitempty"` // 可选
}

// IncidentCandidate AI 建议的事件候选
// 对标 Keep: keep/api/models/incident.py#IncidentCandidate
type IncidentCandidate struct {
	IncidentName          string   `json:"incident_name"`
	Alerts                []int    `json:"alerts"` // 1-based index
	Reasoning             string   `json:"reasoning"`
	Severity              string   `json:"severity"`
	RecommendedActions    []string `json:"recommended_actions"`
	ConfidenceScore       float64  `json:"confidence_score"`
	ConfidenceExplanation string   `json:"confidence_explanation"`
	RootCauseSummary      string   `json:"root_cause_summary"`
}

// IncidentReportDto 事件统计报告
type IncidentReportDto struct {
	TotalIncidents       int                 `json:"totalIncidents"`
	ResolvedIncidents    int                 `json:"resolvedIncidents"`
	PendingIncidents     int                 `json:"pendingIncidents"`
	MTTD                 int64               `json:"mttd"` // 平均检测时间（秒）
	MTTR                 int64               `json:"mttr"` // 平均恢复时间（秒）
	ShortestDuration     int64               `json:"shortestDuration"`
	LongestDuration      int64               `json:"longestDuration"`
	SeverityDistribution map[string]int      `json:"severityDistribution"`
	TopAffectedServices  map[string]int      `json:"topAffectedServices"`
	MostFrequentReasons  map[string][]string `json:"mostFrequentReasons"`
	TimeRangeStart       int64               `json:"timeRangeStart"`
	TimeRangeEnd         int64               `json:"timeRangeEnd"`
}

// ListIncidentsQueryDto 事件列表查询参数
type ListIncidentsQueryDto struct {
	Status        string  `form:"status"`        // candidate/confirmed/rejected/resolved
	Severity      string  `form:"severity"`      // critical/high/warning/info/low
	MinConfidence float64 `form:"minConfidence"` // 最低置信度
	Page          int     `form:"page" binding:"min=1"`
	PageSize      int     `form:"pageSize" binding:"min=1,max=100"`
}

// ListIncidentsResponseDto 事件列表响应
type ListIncidentsResponseDto struct {
	Incidents []IncidentCandidateDto `json:"incidents"`
	Total     int64                  `json:"total"`
	Page      int                    `json:"page"`
	PageSize  int                    `json:"pageSize"`
}

// ErrorResponseDto 错误响应
type ErrorResponseDto struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Details string `json:"details,omitempty"`
}

// SuccessResponseDto 成功响应
type SuccessResponseDto struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// RequestRCACluster 告警聚类请求
type RequestRCACluster struct {
	AlertIds     []string `json:"alertIds" binding:"required,min=1,max=50"` // 告警 ID 列表
	TopologyData string   `json:"topologyData"`                             // 可选的拓扑数据
	ForceRefresh bool     `json:"forceRefresh"`                             // 强制刷新，跳过缓存重新分析
	ModelName    string   `json:"modelName"`                                // 指定 AI 模型名称，为空则使用默认模型
}

// RequestRCACommitFeedback 用户反馈提交请求
type RequestRCACommitFeedback struct {
	Status  string `json:"status" binding:"required,oneof=accepted rejected"` // accepted 或 rejected
	Score   int    `json:"score" binding:"required,min=1,max=5"`              // 评分 1-5
	Comment string `json:"comment"`                                           // 可选评论
}

// RequestRCAListIncidents 事件列表查询请求
type RequestRCAListIncidents struct {
	Status    string `form:"status"`    // 事件状态：candidate/confirmed/rejected/resolved
	Severity  string `form:"severity"`  // 严重程度：critical/high/warning/info/low
	StartTime int64  `form:"startTime"` // 开始时间戳（秒）
	EndTime   int64  `form:"endTime"`   // 结束时间戳（秒）
	Page      int    `form:"page" binding:"min=1"`
	PageSize  int    `form:"pageSize" binding:"min=1,max=100"`
}

// RequestRCAReport 统计报告查询请求
type RequestRCAReport struct {
	StartTime int64 `form:"startTime" binding:"required"` // 开始时间戳（秒）
	EndTime   int64 `form:"endTime" binding:"required"`   // 结束时间戳（秒）
}
