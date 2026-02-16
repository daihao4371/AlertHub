package models

import (
	"fmt"
	"time"
)

// RCAIncidentStatus RCA 事件状态
type RCAIncidentStatus string

const (
	RCAStatusCandidate RCAIncidentStatus = "candidate" // 待确认候选
	RCAStatusConfirmed RCAIncidentStatus = "confirmed" // 已确认
	RCAStatusRejected  RCAIncidentStatus = "rejected"  // 已拒绝
	RCAStatusResolved  RCAIncidentStatus = "resolved"  // 已解决
)

// RCASuggestionStatus 聚类建议状态
type RCASuggestionStatus string

const (
	RCASuggestionPending  RCASuggestionStatus = "pending"  // 待用户反馈
	RCASuggestionAccepted RCASuggestionStatus = "accepted" // 用户已接受
	RCASuggestionRejected RCASuggestionStatus = "rejected" // 用户已拒绝
)

// RCASuggestion AI 聚类建议记录
// 说明：存储每次 AI 分析的输入摘要和输出结果，支持缓存和用户反馈
// 对标 Keep: keep/api/models/db/ai_suggestion.py
type RCASuggestion struct {
	// 基础字段
	ID       string `json:"id" gorm:"primaryKey;size:64"`
	TenantId string `json:"tenantId" gorm:"size:64;index:idx_tenant_created;not null"`
	UserId   string `json:"userId" gorm:"size:64"`

	// 输入摘要（用于缓存命中判断）
	// InputHash = SHA-256(排序后的告警指纹拼接)，支持去重
	InputHash         string `json:"inputHash" gorm:"size:64;uniqueIndex:uk_tenant_hash;not null"`
	AlertCount        int    `json:"alertCount"`                             // 输入告警数量
	AlertFingerprints string `json:"alertFingerprints" gorm:"type:longtext"` // 告警指纹列表，JSON 格式

	// AI 输出
	Model             string `json:"model" gorm:"size:64"`                   // 使用的 AI 模型 (gpt-4o-2024-08-06)
	SuggestionContent string `json:"suggestionContent" gorm:"type:longtext"` // AI 返回的完整 JSON (ClusteringResponse)
	IncidentCount     int    `json:"incidentCount"`                          // 建议的事件数量
	TotalTokens       int    `json:"totalTokens"`                            // 消耗的 Token 数

	// 用户反馈
	Status          string `json:"status" gorm:"size:20;default:'pending'"` // pending/accepted/rejected
	FeedbackScore   int    `json:"feedbackScore"`                           // 用户评分 1-5 分
	FeedbackComment string `json:"feedbackComment" gorm:"type:text"`        // 用户反馈

	// 时间戳（Unix 秒，与 AlertHub 保持一致）
	CreatedAt int64 `json:"createdAt" gorm:"index:idx_tenant_created;autoCreateTime:second"`
	UpdatedAt int64 `json:"updatedAt" gorm:"autoUpdateTime:second"`
}

// TableName 返回表名
func (RCASuggestion) TableName() string {
	return "rca_suggestions"
}

// IsPending 判断是否待反馈
func (s *RCASuggestion) IsPending() bool {
	return s.Status == string(RCASuggestionPending)
}

// IsAccepted 判断是否已接受
func (s *RCASuggestion) IsAccepted() bool {
	return s.Status == string(RCASuggestionAccepted)
}

// RCAIncident AI 聚类产生的事件候选
// 说明：一个 RCASuggestion 可以产生多个 RCAIncident，存储聚类结果的详细信息
// 对标 Keep: keep/api/models/db/incident.py
type RCAIncident struct {
	// 基础字段
	ID           string `json:"id" gorm:"primaryKey;size:64"`
	TenantId     string `json:"tenantId" gorm:"size:64;index:idx_tenant;not null"`
	SuggestionId string `json:"suggestionId" gorm:"size:64;index:idx_suggestion;not null"` // FK to RCASuggestion

	// 事件基本信息
	Name     string `json:"name" gorm:"size:255;not null"`
	Severity string `json:"severity" gorm:"size:20;not null"`                                  // critical/high/warning/info/low
	Status   string `json:"status" gorm:"size:20;default:'candidate';index:idx_status_tenant"` // 候选/已确认/已拒绝/已解决

	// AI 分析内容
	Reasoning        string `json:"reasoning" gorm:"type:text"`        // 聚类推理说明
	RootCauseSummary string `json:"rootCauseSummary" gorm:"type:text"` // 根因摘要

	// 置信度评分（AI 生成，非算法计算）
	// 范围 0.0 ~ 1.0，由 AI 模型评估
	ConfidenceScore       float64 `json:"confidenceScore"`                        // 置信度分值
	ConfidenceExplanation string  `json:"confidenceExplanation" gorm:"type:text"` // 置信度评分说明

	// 关联告警
	// 使用 serializer:json 存储数组，AlertHub 原生支持
	AlertEventIds     []string `json:"alertEventIds" gorm:"column:alertEventIds;serializer:json"`
	AlertFingerprints []string `json:"alertFingerprints" gorm:"column:alertFingerprints;serializer:json"`
	AlertCount        int      `json:"alertCount"`
	AlertSources      []string `json:"alertSources" gorm:"column:alertSources;serializer:json"`
	AffectedServices  []string `json:"affectedServices" gorm:"column:affectedServices;serializer:json"`

	// AI 建议
	RecommendedActions []string `json:"recommendedActions" gorm:"column:recommendedActions;serializer:json"`

	// 时间信息（Unix 秒）
	StartTime    int64 `json:"startTime"`    // 最早告警时间
	LastSeenTime int64 `json:"lastSeenTime"` // 最晚告警时间
	ResolvedTime int64 `json:"resolvedTime"` // 解决时间

	// 时间戳
	CreatedAt int64 `json:"createdAt" gorm:"index:idx_tenant;autoCreateTime:second"`
	UpdatedAt int64 `json:"updatedAt" gorm:"autoUpdateTime:second"`
}

// TableName 返回表名
func (RCAIncident) TableName() string {
	return "rca_incidents"
}

// IsPending 判断是否待确认
func (i *RCAIncident) IsPending() bool {
	return i.Status == string(RCAStatusCandidate)
}

// IsResolved 判断是否已解决
func (i *RCAIncident) IsResolved() bool {
	return i.Status == string(RCAStatusResolved)
}

// Duration 计算事件持续时间（秒）
func (i *RCAIncident) Duration() int64 {
	if i.ResolvedTime == 0 {
		return 0
	}
	return i.ResolvedTime - i.StartTime
}

// RCAReportSnapshot 事件统计报告快照
// 说明：定时生成（每天凌晨），存储统计指标避免重复计算
// 对标 Keep: keep/api/bl/incident_reports.py
type RCAReportSnapshot struct {
	// 基础字段
	ID       string `json:"id" gorm:"primaryKey;size:64"`
	TenantId string `json:"tenantId" gorm:"size:64;index:idx_tenant_time;not null"`

	// 时间范围
	// 定时快照按天生成，例如：2026-02-07 00:00:00 到 2026-02-08 00:00:00
	TimeRangeStart int64 `json:"timeRangeStart" gorm:"index:idx_tenant_time;not null"` // 报告周期起始时间
	TimeRangeEnd   int64 `json:"timeRangeEnd" gorm:"not null"`                         // 报告周期结束时间

	// 基础统计
	TotalIncidents    int `json:"totalIncidents"`    // 总事件数
	ResolvedIncidents int `json:"resolvedIncidents"` // 已解决事件数
	PendingIncidents  int `json:"pendingIncidents"`  // 待处理事件数

	// 关键指标（Unix秒）
	// 对标 Keep: incident_reports.py 的计算公式
	// MTTD = 平均检测时间 = (事件创建时间 - 最早告警时间) 平均值
	// MTTR = 平均恢复时间 = (事件解决时间 - 事件开始时间) 平均值 (仅对已解决事件)
	MTTD             int64 `json:"mttd"`             // 平均检测时间（秒）
	MTTR             int64 `json:"mttr"`             // 平均恢复时间（秒）
	ShortestDuration int64 `json:"shortestDuration"` // 最短事件持续时间
	LongestDuration  int64 `json:"longestDuration"`  // 最长事件持续时间

	// 分布数据（JSON 格式）
	// 使用 serializer:json 存储 map/slice，GORM 原生支持
	SeverityDistribution map[string]int      `json:"severityDistribution" gorm:"column:severityDistribution;serializer:json"` // {"critical": 5, "high": 12, ...}
	TopAffectedServices  map[string]int      `json:"topAffectedServices" gorm:"column:topAffectedServices;serializer:json"`   // {"order-service": 8, ...}
	MostFrequentReasons  map[string][]string `json:"mostFrequentReasons" gorm:"column:mostFrequentReasons;serializer:json"`   // {"根因1": ["inc_1", "inc_2"], ...}

	// 时间戳
	CreatedAt int64 `json:"createdAt" gorm:"index:idx_tenant_time;autoCreateTime:second"`
}

// TableName 返回表名
func (RCAReportSnapshot) TableName() string {
	return "rca_report_snapshots"
}

// IsLatest 判断是否比其他快照更新
func (s *RCAReportSnapshot) IsLatest(otherCreatedAt int64) bool {
	return s.CreatedAt > otherCreatedAt
}

// TimeRangeDays 计算报告覆盖的天数
func (s *RCAReportSnapshot) TimeRangeDays() int64 {
	return (s.TimeRangeEnd - s.TimeRangeStart) / (24 * 3600) // Unix 秒转换为天
}

// RCAError RCA 模块自定义错误
type RCAError struct {
	Operation string // 操作名称
	Reason    string // 错误原因
}

// Error 实现 error 接口
func (e RCAError) Error() string {
	return fmt.Sprintf("RCA %s 失败: %s", e.Operation, e.Reason)
}

// NewRCAError 创建 RCA 错误
func NewRCAError(operation, reason string) error {
	return RCAError{
		Operation: operation,
		Reason:    reason,
	}
}

// ==================== 辅助函数 ====================

// BuildRCACacheKey 构建 RCA 缓存键
// 格式: w8t:{tenant_id}:rca:suggestion:{input_hash}
func BuildRCACacheKey(tenantId, inputHash string) string {
	return fmt.Sprintf("w8t:%s:rca:suggestion:%s", tenantId, inputHash)
}

// BuildRCAIncidentsCacheKey 构建活跃事件缓存键
// 格式: w8t:{tenant_id}:rca:incidents:active
func BuildRCAIncidentsCacheKey(tenantId string) string {
	return fmt.Sprintf("w8t:%s:rca:incidents:active", tenantId)
}

// BuildRCAReportCacheKey 构建报告缓存键
// 格式: w8t:{tenant_id}:rca:report:{start_time}
func BuildRCAReportCacheKey(tenantId string, startTime int64) string {
	return fmt.Sprintf("w8t:%s:rca:report:%d", tenantId, startTime)
}

// GetCurrentDay 获取当天的起始和结束时间戳（Unix 秒）
// 用于定时生成日级报告快照
func GetCurrentDay(timestamp int64) (int64, int64) {
	t := time.Unix(timestamp, 0)
	// 当天 00:00:00 的时间戳
	startOfDay := time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, t.Location()).Unix()
	// 下一天 00:00:00 的时间戳
	endOfDay := startOfDay + 24*3600
	return startOfDay, endOfDay
}
