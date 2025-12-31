package models

import (
	"fmt"
	"time"
)

// ProcessTraceStatus 处理流程状态枚举
type ProcessTraceStatus string

const (
	ProcessStatusDetected   ProcessTraceStatus = "detected"   // 告警检测发现
	ProcessStatusAnalyzing  ProcessTraceStatus = "analyzing"  // AI智能分析中
	ProcessStatusCorrelated ProcessTraceStatus = "correlated" // 多维度关联分析
	ProcessStatusProcessing ProcessTraceStatus = "processing" // 问题排查处理中
	ProcessStatusValidated  ProcessTraceStatus = "validated"  // 效果验证
	ProcessStatusCompleted  ProcessTraceStatus = "completed"  // 处理完成
)

// ProcessTrace 处理流程追踪记录
type ProcessTrace struct {
	ID             string             `json:"id" gorm:"primaryKey"`
	TenantId       string             `json:"tenantId" gorm:"index"`
	EventId        string             `json:"eventId" gorm:"index"` // 关联的告警事件ID
	FaultCenterId  string             `json:"faultCenterId"`        // 关联的故障中心ID
	RuleId         string             `json:"ruleId"`               // 关联的告警规则ID（持久化存储）
	RuleName       string             `json:"ruleName"`             // 告警规则名称（持久化存储，确保历史数据可读）
	ProcessSteps   []ProcessStep      `json:"processSteps" gorm:"processSteps;serializer:json"`
	CurrentStatus  ProcessTraceStatus `json:"currentStatus"`          // 当前处理状态
	StartTime      int64              `json:"startTime"`              // 开始处理时间
	EndTime        int64              `json:"endTime"`                // 结束处理时间
	TotalDuration  int64              `json:"totalDuration" gorm:"-"` // 总处理时长(秒)
	AssignedUser   string             `json:"assignedUser"`           // 分配处理人
	AIAnalysisTime int64              `json:"aiAnalysisTime"`         // AI分析耗时(毫秒)
	CreatedAt      int64              `json:"createdAt"`
	UpdatedAt      int64              `json:"updatedAt"`
}

// ProcessStep 处理步骤
type ProcessStep struct {
	StepName       string             `json:"stepName"`       // 步骤名称
	Status         ProcessTraceStatus `json:"status"`         // 步骤状态
	StartTime      int64              `json:"startTime"`      // 步骤开始时间
	EndTime        int64              `json:"endTime"`        // 步骤结束时间
	Duration       int64              `json:"duration"`       // 步骤耗时(秒)
	Description    string             `json:"description"`    // 步骤描述
	AssignedUser   string             `json:"assignedUser"`   // 执行人
	Notes          string             `json:"notes"`          // 备注信息
	IsCompleted    bool               `json:"isCompleted"`    // 是否完成
	AIAnalysisData *AIAnalysisData    `json:"aiAnalysisData"` // AI分析数据
}

// AIAnalysisData AI分析数据
type AIAnalysisData struct {
	AnalysisType   string                 `json:"analysisType"`   // 分析类型
	AnalysisResult map[string]interface{} `json:"analysisResult"` // 分析结果
	Confidence     float64                `json:"confidence"`     // 置信度
	Suggestions    []string               `json:"suggestions"`    // 处理建议
	AnalysisTime   int64                  `json:"analysisTime"`   // 分析时间戳
}

// StatusTransitionRule 状态转换规则
type StatusTransitionRule struct {
	From    ProcessTraceStatus
	To      ProcessTraceStatus
	IsValid bool
	Warning string // 需要特殊警告的转换
}

// ProcessOperationLog 处理操作日志
type ProcessOperationLog struct {
	ID            string                 `json:"id" gorm:"primaryKey"`
	TenantId      string                 `json:"tenantId" gorm:"index"`
	EventId       string                 `json:"eventId" gorm:"index"`                         // 关联的告警事件ID
	ProcessId     string                 `json:"processId" gorm:"index"`                       // 关联的流程追踪ID
	OperationType string                 `json:"operationType"`                                // 操作类型
	OperationDesc string                 `json:"operationDesc"`                                // 操作描述
	Operator      string                 `json:"operator"`                                     // 操作人
	OperatorName  string                 `json:"operatorName" gorm:"-"`                        // 操作人姓名(不持久化)
	OperationTime int64                  `json:"operationTime"`                                // 操作时间
	BeforeData    map[string]interface{} `json:"beforeData" gorm:"beforeData;serializer:json"` // 操作前数据
	AfterData     map[string]interface{} `json:"afterData" gorm:"afterData;serializer:json"`   // 操作后数据
	IPAddress     string                 `json:"ipAddress"`                                    // 操作IP
	UserAgent     string                 `json:"userAgent"`                                    // 用户代理
}

// TableName 指定表名
func (pt *ProcessTrace) TableName() string {
	return "process_trace"
}

func (pol *ProcessOperationLog) TableName() string {
	return "process_operation_log"
}

// GetTotalDuration 计算总处理时长
func (pt *ProcessTrace) GetTotalDuration() int64 {
	if pt.EndTime > 0 && pt.StartTime > 0 {
		return pt.EndTime - pt.StartTime
	}
	if pt.StartTime > 0 {
		return time.Now().Unix() - pt.StartTime
	}
	return 0
}





// UpdateAIAnalysis 更新AI分析结果
func (pt *ProcessTrace) UpdateAIAnalysis(stepName string, analysisData *AIAnalysisData) error {
	for i := range pt.ProcessSteps {
		if pt.ProcessSteps[i].StepName == stepName {
			pt.ProcessSteps[i].AIAnalysisData = analysisData
			pt.UpdatedAt = time.Now().Unix()
			return nil
		}
	}
	return fmt.Errorf("未找到名为 %s 的步骤", stepName)
}

// ValidateStatusTransition 验证状态转换是否有效
func (pt *ProcessTrace) ValidateStatusTransition(newStatus ProcessTraceStatus) (bool, string) {
	// 如果状态没有变化，直接允许
	if pt.CurrentStatus == newStatus {
		return true, ""
	}

	// 定义状态转换规则矩阵
	transitionRules := map[ProcessTraceStatus]map[ProcessTraceStatus]StatusTransitionRule{
		ProcessStatusDetected: {
			ProcessStatusAnalyzing:  {From: ProcessStatusDetected, To: ProcessStatusAnalyzing, IsValid: true, Warning: ""},
			ProcessStatusCorrelated: {From: ProcessStatusDetected, To: ProcessStatusCorrelated, IsValid: true, Warning: ""},
			ProcessStatusProcessing: {From: ProcessStatusDetected, To: ProcessStatusProcessing, IsValid: true, Warning: ""},
			ProcessStatusValidated:  {From: ProcessStatusDetected, To: ProcessStatusValidated, IsValid: true, Warning: "跳过中间步骤，请确认是否需要详细分析"},
			ProcessStatusCompleted:  {From: ProcessStatusDetected, To: ProcessStatusCompleted, IsValid: true, Warning: "直接完成处理，请确认问题已彻底解决"},
		},
		ProcessStatusAnalyzing: {
			ProcessStatusDetected:   {From: ProcessStatusAnalyzing, To: ProcessStatusDetected, IsValid: true, Warning: "回退到检测状态，请确认是否需要重新评估"},
			ProcessStatusCorrelated: {From: ProcessStatusAnalyzing, To: ProcessStatusCorrelated, IsValid: true, Warning: ""},
			ProcessStatusProcessing: {From: ProcessStatusAnalyzing, To: ProcessStatusProcessing, IsValid: true, Warning: ""},
			ProcessStatusValidated:  {From: ProcessStatusAnalyzing, To: ProcessStatusValidated, IsValid: true, Warning: "跳过处理步骤，请确认问题已解决"},
			ProcessStatusCompleted:  {From: ProcessStatusAnalyzing, To: ProcessStatusCompleted, IsValid: true, Warning: "直接完成处理，请确认问题已彻底解决"},
		},
		ProcessStatusCorrelated: {
			ProcessStatusDetected:   {From: ProcessStatusCorrelated, To: ProcessStatusDetected, IsValid: true, Warning: "回退到检测状态，请确认是否需要重新评估"},
			ProcessStatusAnalyzing:  {From: ProcessStatusCorrelated, To: ProcessStatusAnalyzing, IsValid: true, Warning: "回退到分析阶段，请确认是否需要重新分析"},
			ProcessStatusProcessing: {From: ProcessStatusCorrelated, To: ProcessStatusProcessing, IsValid: true, Warning: ""},
			ProcessStatusValidated:  {From: ProcessStatusCorrelated, To: ProcessStatusValidated, IsValid: true, Warning: "跳过处理步骤，请确认问题已解决"},
			ProcessStatusCompleted:  {From: ProcessStatusCorrelated, To: ProcessStatusCompleted, IsValid: true, Warning: "直接完成处理，请确认问题已彻底解决"},
		},
		ProcessStatusProcessing: {
			ProcessStatusDetected:   {From: ProcessStatusProcessing, To: ProcessStatusDetected, IsValid: true, Warning: "回退到检测状态，请确认是否需要重新评估"},
			ProcessStatusAnalyzing:  {From: ProcessStatusProcessing, To: ProcessStatusAnalyzing, IsValid: true, Warning: "回退到分析阶段，请确认是否需要重新分析"},
			ProcessStatusCorrelated: {From: ProcessStatusProcessing, To: ProcessStatusCorrelated, IsValid: true, Warning: "回退到关联分析阶段，请确认是否需要重新关联"},
			ProcessStatusValidated:  {From: ProcessStatusProcessing, To: ProcessStatusValidated, IsValid: true, Warning: ""},
			ProcessStatusCompleted:  {From: ProcessStatusProcessing, To: ProcessStatusCompleted, IsValid: true, Warning: "直接完成处理，请确认效果验证已完成"},
		},
		ProcessStatusValidated: {
			ProcessStatusDetected:   {From: ProcessStatusValidated, To: ProcessStatusDetected, IsValid: true, Warning: "回退到检测状态，请确认是否发现新问题"},
			ProcessStatusAnalyzing:  {From: ProcessStatusValidated, To: ProcessStatusAnalyzing, IsValid: true, Warning: "回退到分析阶段，请确认是否需要重新分析"},
			ProcessStatusCorrelated: {From: ProcessStatusValidated, To: ProcessStatusCorrelated, IsValid: true, Warning: "回退到关联分析阶段，请确认是否需要重新关联"},
			ProcessStatusProcessing: {From: ProcessStatusValidated, To: ProcessStatusProcessing, IsValid: true, Warning: "回退到处理阶段，请确认验证结果不满足要求"},
			ProcessStatusCompleted:  {From: ProcessStatusValidated, To: ProcessStatusCompleted, IsValid: true, Warning: ""},
		},
		ProcessStatusCompleted: {
			ProcessStatusDetected:   {From: ProcessStatusCompleted, To: ProcessStatusDetected, IsValid: true, Warning: "重新开始处理流程，请确认问题复现或发现新问题"},
			ProcessStatusAnalyzing:  {From: ProcessStatusCompleted, To: ProcessStatusAnalyzing, IsValid: true, Warning: "从完成状态回退到分析阶段，请确认发现问题需要重新分析"},
			ProcessStatusCorrelated: {From: ProcessStatusCompleted, To: ProcessStatusCorrelated, IsValid: true, Warning: "从完成状态回退到关联分析阶段，请确认发现关联问题"},
			ProcessStatusProcessing: {From: ProcessStatusCompleted, To: ProcessStatusProcessing, IsValid: true, Warning: "从完成状态回退到处理阶段，请确认发现处理不彻底"},
			ProcessStatusValidated:  {From: ProcessStatusCompleted, To: ProcessStatusValidated, IsValid: true, Warning: "从完成状态回退到验证阶段，请确认需要重新验证"},
		},
	}

	// 查找转换规则
	if rules, exists := transitionRules[pt.CurrentStatus]; exists {
		if rule, ruleExists := rules[newStatus]; ruleExists {
			return rule.IsValid, rule.Warning
		}
	}

	// 如果没有定义的规则，认为是无效转换
	return false, fmt.Sprintf("不支持从 %s 状态转换到 %s 状态", pt.CurrentStatus, newStatus)
}
