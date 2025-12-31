package types

import "alertHub/internal/models"

// ProcessTrace 相关请求和响应类型

// UpdateProcessStatusRequest 更新处理状态请求
type UpdateProcessStatusRequest struct {
	EventId string `json:"eventId" binding:"required"` // 告警事件ID
	Status  string `json:"status" binding:"required"`  // 新状态
}

// AddProcessStepRequest 添加处理步骤请求
type AddProcessStepRequest struct {
	EventId      string `json:"eventId" binding:"required"`      // 告警事件ID
	StepName     string `json:"stepName" binding:"required"`     // 步骤名称
	Description  string `json:"description" binding:"required"`  // 步骤描述
	AssignedUser string `json:"assignedUser"`                    // 分配处理人
}

// CompleteProcessStepRequest 完成处理步骤请求
type CompleteProcessStepRequest struct {
	EventId  string `json:"eventId" binding:"required"`  // 告警事件ID
	StepName string `json:"stepName" binding:"required"` // 步骤名称
	Notes    string `json:"notes"`                       // 备注信息
}

// UpdateAIAnalysisRequest 更新AI分析结果请求
type UpdateAIAnalysisRequest struct {
	EventId        string                 `json:"eventId" binding:"required"`        // 告警事件ID
	StepName       string                 `json:"stepName" binding:"required"`       // 步骤名称
	AnalysisType   string                 `json:"analysisType" binding:"required"`   // 分析类型
	AnalysisResult map[string]interface{} `json:"analysisResult" binding:"required"` // 分析结果
	Confidence     float64                `json:"confidence"`                        // 置信度
	Suggestions    []string               `json:"suggestions"`                       // 处理建议
}

// OperationLogListResponse 操作日志列表响应
type OperationLogListResponse struct {
	List     []models.ProcessOperationLog `json:"list"`     // 日志列表
	Total    int64                        `json:"total"`    // 总数
	Page     int                          `json:"page"`     // 当前页码
	PageSize int                          `json:"pageSize"` // 每页数量
}

// ProcessTraceListResponse 处理流程列表响应
type ProcessTraceListResponse struct {
	List     []models.ProcessTrace `json:"list"`     // 流程列表
	Total    int64                 `json:"total"`    // 总数
	Page     int                   `json:"page"`     // 当前页码
	PageSize int                   `json:"pageSize"` // 每页数量
}

// ProcessStatisticsResponse 处理流程统计响应
type ProcessStatisticsResponse struct {
	TotalCount          int64                        `json:"totalCount"`          // 总处理流程数
	CompletedCount      int64                        `json:"completedCount"`      // 已完成流程数
	AvgDuration         float64                      `json:"avgDuration"`         // 平均处理时长(秒)
	StatusDistribution  []map[string]interface{}     `json:"statusDistribution"`  // 状态分布
	CompletionRate      float64                      `json:"completionRate"`      // 完成率
}

// ProcessTraceDetailResponse 处理流程详情响应
type ProcessTraceDetailResponse struct {
	ProcessTrace   *models.ProcessTrace           `json:"processTrace"`   // 流程追踪记录
	OperationLogs  []models.ProcessOperationLog   `json:"operationLogs"`  // 操作日志
	RelatedEvents  []models.AlertCurEvent         `json:"relatedEvents"`  // 相关告警事件
}