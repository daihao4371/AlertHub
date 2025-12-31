package services

import (
	"alertHub/internal/ctx"
	"alertHub/internal/models"
	"alertHub/internal/repo"
	"alertHub/internal/types"
	"alertHub/pkg/tools"
	"errors"
	"fmt"
	"time"

	"gorm.io/gorm"
)

type (
	processTraceService struct {
		db      *gorm.DB
		ctx     *ctx.Context
		repo    repo.ProcessTraceRepo
		logRepo repo.ProcessOperationLogRepo
	}

	InterProcessTraceService interface {
		// 创建处理流程追踪记录
		CreateProcessTrace(tenantId, eventId, faultCenterId, assignedUser string) (*models.ProcessTrace, error)

		// 获取处理流程追踪记录
		GetProcessTrace(tenantId, eventId string) (*models.ProcessTrace, error)

		// 根据指纹获取处理流程追踪记录
		GetProcessTraceByFingerprint(tenantId, fingerprint string) (*models.ProcessTrace, error)

		// 获取处理流程追踪记录列表
		GetProcessTraceList(tenantId, eventId, faultCenterId string, page, pageSize int) (*types.ProcessTraceListResponse, error)

		// 更新处理状态
		UpdateProcessStatus(tenantId, eventId, operator string, status models.ProcessTraceStatus) error

		// 添加处理步骤
		AddProcessStep(tenantId, eventId, stepName, description, assignedUser, operator string) error

		// 完成处理步骤
		CompleteProcessStep(tenantId, eventId, stepName, notes, operator string) error

		// 更新AI分析结果
		UpdateAIAnalysis(tenantId, eventId, stepName string, analysisData *models.AIAnalysisData) error

		// 记录操作日志
		LogOperation(tenantId, eventId, processId, operationType, operationDesc, operator string, beforeData, afterData map[string]interface{}, ipAddress, userAgent string) error

		// 获取操作日志列表
		GetOperationLogs(tenantId, eventId string, page, pageSize int) ([]models.ProcessOperationLog, int64, error)

		// 根据指纹获取操作日志列表
		GetOperationLogsByFingerprint(tenantId, fingerprint string, page, pageSize int) ([]models.ProcessOperationLog, int64, error)

		// 获取流程统计数据
		GetProcessStatistics(tenantId string, startTime, endTime int64) (map[string]interface{}, error)
	}
)

func NewInterProcessTraceService(ctx *ctx.Context) InterProcessTraceService {
	return &processTraceService{
		db:      ctx.DB.DB(),
		ctx:     ctx,
		repo:    repo.NewProcessTraceRepo(ctx.DB.DB()),
		logRepo: repo.NewProcessOperationLogRepo(ctx.DB.DB()),
	}
}

// getRuleInfoFromEvent 从事件获取规则信息
func (pts *processTraceService) getRuleInfoFromEvent(tenantId, eventId string) (ruleId string, ruleName string) {
	// 方法1: 首先尝试通过eventId直接从历史事件表查询
	var historyEvent models.AlertHisEvent
	err := pts.db.Table("alert_his_events").Where("tenant_id = ? AND event_id = ?", tenantId, eventId).
		Select("rule_id, rule_name").First(&historyEvent).Error

	if err == nil && historyEvent.RuleName != "" {
		return historyEvent.RuleId, historyEvent.RuleName
	}

	// 方法2: 尝试从当前事件表查询
	var currentEvent models.AlertCurEvent
	err = pts.db.Table("alert_cur_events").Where("tenant_id = ? AND event_id = ?", tenantId, eventId).
		Select("rule_id, rule_name").First(&currentEvent).Error

	if err == nil && currentEvent.RuleName != "" {
		return currentEvent.RuleId, currentEvent.RuleName
	}

	// 方法3: 从Redis缓存中查找（主要数据源）
	var faultCenters []models.FaultCenter
	err = pts.db.Where("tenant_id = ?", tenantId).Find(&faultCenters).Error
	if err == nil {
		for _, fc := range faultCenters {
			events, err := pts.ctx.Redis.Alert().GetAllEvents(models.BuildAlertEventCacheKey(tenantId, fc.ID))
			if err != nil {
				continue
			}

			// 遍历事件找到匹配的eventId
			for _, event := range events {
				if event.EventId == eventId && event.RuleName != "" {
					return event.RuleId, event.RuleName
				}
			}
		}
	}

	// 如果都找不到，返回空值
	return "", ""
}

// CreateProcessTrace 创建处理流程追踪记录
func (pts *processTraceService) CreateProcessTrace(tenantId, eventId, faultCenterId, assignedUser string) (*models.ProcessTrace, error) {
	// 检查是否已存在处理流程记录
	var existing models.ProcessTrace
	err := pts.db.Where("tenant_id = ? AND event_id = ?", tenantId, eventId).First(&existing).Error
	if err == nil {
		return &existing, nil // 已存在，直接返回
	}

	// 获取规则信息
	ruleId, ruleName := pts.getRuleInfoFromEvent(tenantId, eventId)

	// 如果没有找到规则名称，使用eventId作为备用显示名称
	if ruleName == "" {
		ruleName = eventId
	}

	now := time.Now().Unix()
	processTrace := &models.ProcessTrace{
		ID:            tools.RandId(),
		TenantId:      tenantId,
		EventId:       eventId,
		FaultCenterId: faultCenterId,
		RuleId:        ruleId,   // 存储规则ID
		RuleName:      ruleName, // 存储规则名称，确保历史数据可读
		CurrentStatus: models.ProcessStatusDetected,
		StartTime:     now,
		AssignedUser:  assignedUser,
		CreatedAt:     now,
		UpdatedAt:     now,
		ProcessSteps: []models.ProcessStep{
			{
				StepName:     "告警发现检测",
				Status:       models.ProcessStatusDetected,
				StartTime:    now,
				Description:  "系统自动发现告警，开始处理流程",
				AssignedUser: assignedUser, // 使用实际的认领用户而不是"system"
				IsCompleted:  true,
				EndTime:      now,
				Duration:     0,
			},
		},
	}

	err = pts.db.Create(processTrace).Error
	if err != nil {
		return nil, fmt.Errorf("创建处理流程追踪记录失败: %v", err)
	}

	// 记录操作日志
	_ = pts.LogOperation(tenantId, eventId, processTrace.ID, "create_process",
		"创建处理流程追踪记录", assignedUser, nil, // 使用实际用户而不是"system"
		map[string]interface{}{"processId": processTrace.ID}, "", "")

	return processTrace, nil
}

// GetProcessTrace 获取处理流程追踪记录
func (pts *processTraceService) GetProcessTrace(tenantId, eventId string) (*models.ProcessTrace, error) {
	processTrace, err := pts.repo.GetByEventId(tenantId, eventId)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("未找到事件ID为 %s 的处理流程追踪记录", eventId)
		}
		return nil, fmt.Errorf("获取处理流程追踪记录失败: %v", err)
	}

	// 计算总处理时长
	processTrace.TotalDuration = processTrace.GetTotalDuration()

	return processTrace, nil
}

// GetProcessTraceByFingerprint 根据指纹获取处理流程追踪记录
func (pts *processTraceService) GetProcessTraceByFingerprint(tenantId, fingerprint string) (*models.ProcessTrace, error) {
	// 方法1: 直接通过eventId查询（因为在某些情况下eventId就是fingerprint）
	processTrace, err := pts.GetProcessTrace(tenantId, fingerprint)
	if err == nil {
		return processTrace, nil
	}

	// 方法2: 从Redis缓存中查找fingerprint对应的eventId
	var targetEventId string

	var faultCenters []models.FaultCenter
	err = pts.db.Where("tenant_id = ?", tenantId).Find(&faultCenters).Error
	if err == nil {
		for _, fc := range faultCenters {
			// 尝试从缓存中获取事件
			event, err := pts.ctx.Redis.Alert().GetEventFromCache(tenantId, fc.ID, fingerprint)
			if err == nil && event.EventId != "" && event.EventId != fingerprint {
				targetEventId = event.EventId
				break
			}
		}
	}

	// 方法3: 如果找到了不同的eventId，用它查询
	if targetEventId != "" {
		return pts.GetProcessTrace(tenantId, targetEventId)
	}

	// 方法4: 数据库查找
	var alertEvent models.AlertCurEvent
	err = pts.db.Table("alert_cur_events").Where("tenant_id = ? AND fingerprint = ?", tenantId, fingerprint).First(&alertEvent).Error
	if err == nil && alertEvent.EventId != fingerprint {
		return pts.GetProcessTrace(tenantId, alertEvent.EventId)
	}

	// 方法5: 遍历ProcessTrace表，寻找可能的匹配
	var processTraces []models.ProcessTrace
	err = pts.db.Where("tenant_id = ?", tenantId).Find(&processTraces).Error
	if err == nil {
		for _, pt := range processTraces {
			// 如果eventId就是fingerprint，直接返回
			if pt.EventId == fingerprint {
				pt.TotalDuration = pt.GetTotalDuration()
				return &pt, nil
			}

			// 尝试通过Redis匹配
			if pts.isEventMatchFingerprint(tenantId, pt.EventId, fingerprint) {
				pt.TotalDuration = pt.GetTotalDuration()
				return &pt, nil
			}
		}
	}

	return nil, fmt.Errorf("未找到指纹为 %s 的告警事件", fingerprint)
}

// isEventMatchFingerprint 检查事件ID是否匹配给定指纹
func (pts *processTraceService) isEventMatchFingerprint(tenantId, eventId, targetFingerprint string) bool {
	// 从Redis缓存中查找事件
	var faultCenters []models.FaultCenter
	err := pts.db.Where("tenant_id = ?", tenantId).Find(&faultCenters).Error
	if err != nil {
		return false
	}

	for _, fc := range faultCenters {
		events, err := pts.ctx.Redis.Alert().GetAllEvents(models.BuildAlertEventCacheKey(tenantId, fc.ID))
		if err != nil {
			continue
		}

		for fingerprint, event := range events {
			if event.EventId == eventId && fingerprint == targetFingerprint {
				return true
			}
		}
	}

	return false
}

// calculateProcessDurations 计算每个处理流程记录的总处理时长
func (pts *processTraceService) calculateProcessDurations(processTraces []models.ProcessTrace) {
	for i := range processTraces {
		processTraces[i].TotalDuration = processTraces[i].GetTotalDuration()
	}
}

// GetProcessTraceList 获取处理流程追踪记录列表
func (pts *processTraceService) GetProcessTraceList(tenantId, eventId, faultCenterId string, page, pageSize int) (*types.ProcessTraceListResponse, error) {
	// 直接使用基础查询，规则名称已经存储在数据库中，无需复杂的关联查询
	processTraces, total, err := pts.repo.GetListWithFilters(tenantId, eventId, faultCenterId, page, pageSize)
	if err != nil {
		return nil, fmt.Errorf("获取处理流程记录列表失败: %v", err)
	}

	// 计算每个记录的总处理时长
	pts.calculateProcessDurations(processTraces)

	response := &types.ProcessTraceListResponse{
		List:     processTraces,
		Total:    total,
		Page:     page,
		PageSize: pageSize,
	}

	return response, nil
}

// getStatusChineseName 获取状态的中文名称
func (pts *processTraceService) getStatusChineseName(status models.ProcessTraceStatus) string {
	statusMap := map[models.ProcessTraceStatus]string{
		models.ProcessStatusDetected:   "已检测",
		models.ProcessStatusAnalyzing:  "分析中",
		models.ProcessStatusCorrelated: "关联分析",
		models.ProcessStatusProcessing: "处理中",
		models.ProcessStatusValidated:  "验证中",
		models.ProcessStatusCompleted:  "已完成",
	}
	if chineseName, ok := statusMap[status]; ok {
		return chineseName
	}
	return string(status) // 如果找不到映射，返回原值
}

// UpdateProcessStatus 更新处理状态
func (pts *processTraceService) UpdateProcessStatus(tenantId, eventId, operator string, status models.ProcessTraceStatus) error {
	var processTrace models.ProcessTrace
	err := pts.db.Where("tenant_id = ? AND event_id = ?", tenantId, eventId).First(&processTrace).Error
	if err != nil {
		return fmt.Errorf("未找到处理流程追踪记录: %v", err)
	}

	// 验证状态转换是否有效
	isValid, warning := processTrace.ValidateStatusTransition(status)
	if !isValid {
		return fmt.Errorf("状态转换验证失败: %s", warning)
	}

	oldStatus := processTrace.CurrentStatus
	processTrace.CurrentStatus = status
	processTrace.UpdatedAt = time.Now().Unix()

	// 如果转换到完成状态，设置结束时间
	if status == models.ProcessStatusCompleted && processTrace.EndTime == 0 {
		processTrace.EndTime = time.Now().Unix()
	}

	err = pts.db.Save(&processTrace).Error
	if err != nil {
		return fmt.Errorf("更新处理状态失败: %v", err)
	}

	// 记录操作日志 - 使用中文状态名称
	oldStatusCN := pts.getStatusChineseName(oldStatus)
	newStatusCN := pts.getStatusChineseName(status)
	
	// 构建操作描述，包含警告信息（如果有）
	operationDesc := fmt.Sprintf("更新处理状态从 %s 到 %s", oldStatusCN, newStatusCN)
	if warning != "" {
		operationDesc += fmt.Sprintf("。系统提醒: %s", warning)
	}
	
	_ = pts.LogOperation(tenantId, eventId, processTrace.ID, "update_status",
		operationDesc, operator, // 使用实际操作用户
		map[string]interface{}{"status": oldStatus},
		map[string]interface{}{"status": status}, "", "")

	return nil
}

// AddProcessStep 添加处理步骤
func (pts *processTraceService) AddProcessStep(tenantId, eventId, stepName, description, assignedUser, operator string) error {
	var processTrace models.ProcessTrace
	err := pts.db.Where("tenant_id = ? AND event_id = ?", tenantId, eventId).First(&processTrace).Error
	if err != nil {
		return fmt.Errorf("未找到处理流程追踪记录: %v", err)
	}

	processTrace.AddProcessStep(stepName, description, assignedUser)

	err = pts.db.Save(&processTrace).Error
	if err != nil {
		return fmt.Errorf("添加处理步骤失败: %v", err)
	}

	// 记录操作日志 - operator是添加步骤的操作人，assignedUser是被分配的执行人
	_ = pts.LogOperation(tenantId, eventId, processTrace.ID, "add_step",
		fmt.Sprintf("添加处理步骤: %s，分配给: %s", stepName, assignedUser), operator, nil,
		map[string]interface{}{"stepName": stepName, "description": description, "assignedUser": assignedUser}, "", "")

	return nil
}

// CompleteProcessStep 完成处理步骤
func (pts *processTraceService) CompleteProcessStep(tenantId, eventId, stepName, notes, operator string) error {
	var processTrace models.ProcessTrace
	err := pts.db.Where("tenant_id = ? AND event_id = ?", tenantId, eventId).First(&processTrace).Error
	if err != nil {
		return fmt.Errorf("未找到处理流程追踪记录: %v", err)
	}

	err = processTrace.CompleteProcessStep(stepName, notes)
	if err != nil {
		return err
	}

	err = pts.db.Save(&processTrace).Error
	if err != nil {
		return fmt.Errorf("完成处理步骤失败: %v", err)
	}

	// 记录操作日志
	_ = pts.LogOperation(tenantId, eventId, processTrace.ID, "complete_step",
		fmt.Sprintf("完成处理步骤: %s", stepName), operator, nil, // 使用实际操作用户
		map[string]interface{}{"stepName": stepName, "notes": notes}, "", "")

	return nil
}

// UpdateAIAnalysis 更新AI分析结果
func (pts *processTraceService) UpdateAIAnalysis(tenantId, eventId, stepName string, analysisData *models.AIAnalysisData) error {
	var processTrace models.ProcessTrace
	err := pts.db.Where("tenant_id = ? AND event_id = ?", tenantId, eventId).First(&processTrace).Error
	if err != nil {
		return fmt.Errorf("未找到处理流程追踪记录: %v", err)
	}

	err = processTrace.UpdateAIAnalysis(stepName, analysisData)
	if err != nil {
		return err
	}

	err = pts.db.Save(&processTrace).Error
	if err != nil {
		return fmt.Errorf("更新AI分析结果失败: %v", err)
	}

	// 记录操作日志
	_ = pts.LogOperation(tenantId, eventId, processTrace.ID, "update_ai_analysis",
		fmt.Sprintf("更新AI分析结果: %s", stepName), "ai_system", nil,
		map[string]interface{}{"stepName": stepName, "analysisData": analysisData}, "", "")

	return nil
}

// LogOperation 记录操作日志
func (pts *processTraceService) LogOperation(tenantId, eventId, processId, operationType, operationDesc, operator string, beforeData, afterData map[string]interface{}, ipAddress, userAgent string) error {
	log := &models.ProcessOperationLog{
		ID:            tools.RandId(),
		TenantId:      tenantId,
		EventId:       eventId,
		ProcessId:     processId,
		OperationType: operationType,
		OperationDesc: operationDesc,
		Operator:      operator,
		OperationTime: time.Now().Unix(),
		BeforeData:    beforeData,
		AfterData:     afterData,
		IPAddress:     ipAddress,
		UserAgent:     userAgent,
	}

	err := pts.db.Create(log).Error
	if err != nil {
		return fmt.Errorf("记录操作日志失败: %v", err)
	}

	return nil
}

// GetOperationLogs 获取操作日志列表
func (pts *processTraceService) GetOperationLogs(tenantId, eventId string, page, pageSize int) ([]models.ProcessOperationLog, int64, error) {
	return pts.logRepo.GetList(tenantId, eventId, page, pageSize)
}

// GetOperationLogsByFingerprint 根据指纹获取操作日志列表
func (pts *processTraceService) GetOperationLogsByFingerprint(tenantId, fingerprint string, page, pageSize int) ([]models.ProcessOperationLog, int64, error) {
	// 方法1: 直接使用fingerprint作为eventId查询（处理eventId==fingerprint的情况）
	logs, count, err := pts.GetOperationLogs(tenantId, fingerprint, page, pageSize)
	if err == nil && count > 0 {
		return logs, count, nil
	}

	// 方法2: 从Redis缓存中查找fingerprint对应的eventId
	var targetEventId string

	var faultCenters []models.FaultCenter
	err = pts.db.Where("tenant_id = ?", tenantId).Find(&faultCenters).Error
	if err == nil {
		for _, fc := range faultCenters {
			// 尝试从缓存中获取事件
			event, err := pts.ctx.Redis.Alert().GetEventFromCache(tenantId, fc.ID, fingerprint)
			if err == nil && event.EventId != "" && event.EventId != fingerprint {
				targetEventId = event.EventId
				break
			}
		}
	}

	// 方法3: 如果找到了不同的eventId，用它查询
	if targetEventId != "" {
		return pts.GetOperationLogs(tenantId, targetEventId, page, pageSize)
	}

	// 方法4: 数据库查找作为兜底
	var alertEvent models.AlertCurEvent
	err = pts.db.Table("alert_cur_events").Where("tenant_id = ? AND fingerprint = ?", tenantId, fingerprint).First(&alertEvent).Error
	if err == nil && alertEvent.EventId != fingerprint {
		return pts.GetOperationLogs(tenantId, alertEvent.EventId, page, pageSize)
	}

	// 如果都失败了，返回原始错误或空结果
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, 0, fmt.Errorf("未找到指纹为 %s 的告警事件", fingerprint)
	}
	return nil, 0, fmt.Errorf("查找告警事件失败: %v", err)
}

// GetProcessStatistics 获取流程统计数据
func (pts *processTraceService) GetProcessStatistics(tenantId string, startTime, endTime int64) (map[string]interface{}, error) {
	var statistics map[string]interface{} = make(map[string]interface{})

	// 总处理流程数
	var totalCount int64
	err := pts.db.Model(&models.ProcessTrace{}).Where("tenant_id = ? AND created_at BETWEEN ? AND ?", tenantId, startTime, endTime).Count(&totalCount).Error
	if err != nil {
		return nil, fmt.Errorf("获取总处理流程数失败: %v", err)
	}
	statistics["totalCount"] = totalCount

	// 已完成流程数
	var completedCount int64
	err = pts.db.Model(&models.ProcessTrace{}).Where("tenant_id = ? AND current_status = ? AND created_at BETWEEN ? AND ?", tenantId, models.ProcessStatusCompleted, startTime, endTime).Count(&completedCount).Error
	if err != nil {
		return nil, fmt.Errorf("获取已完成流程数失败: %v", err)
	}
	statistics["completedCount"] = completedCount

	// 平均处理时长
	var avgDuration float64
	err = pts.db.Model(&models.ProcessTrace{}).Select("COALESCE(AVG(end_time - start_time), 0) as avg_duration").Where("tenant_id = ? AND current_status = ? AND end_time > 0 AND created_at BETWEEN ? AND ?", tenantId, models.ProcessStatusCompleted, startTime, endTime).Scan(&avgDuration).Error
	if err != nil {
		return nil, fmt.Errorf("获取平均处理时长失败: %v", err)
	}
	statistics["avgDuration"] = avgDuration

	// 各状态分布
	var statusDistribution []map[string]interface{}
	err = pts.db.Model(&models.ProcessTrace{}).Select("current_status, COUNT(*) as count").Where("tenant_id = ? AND created_at BETWEEN ? AND ?", tenantId, startTime, endTime).Group("current_status").Scan(&statusDistribution).Error
	if err != nil {
		return nil, fmt.Errorf("获取状态分布失败: %v", err)
	}
	statistics["statusDistribution"] = statusDistribution

	return statistics, nil
}
