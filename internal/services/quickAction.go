package services

import (
	"encoding/json"
	"fmt"
	"time"
	"alertHub/internal/ctx"
	"alertHub/internal/models"
	"alertHub/pkg/quickaction"
	"alertHub/pkg/tools"
)

type quickActionService struct {
	ctx                 *ctx.Context
	processTraceService InterProcessTraceService // 处理流程追踪服务，用于自动创建处理记录
}

type InterQuickActionService interface {
	// ClaimAlert 认领告警
	ClaimAlert(tenantId, fingerprint, username, clientIP string) error
	// SilenceAlert 静默告警
	SilenceAlert(tenantId, fingerprint, duration, username, clientIP string) error
	// SilenceAlertWithReason 静默告警(带原因)
	SilenceAlertWithReason(tenantId, fingerprint, duration, username, reason, clientIP string) error
	// ResolveAlert 标记告警已处理
	ResolveAlert(tenantId, fingerprint, username, clientIP string) error
	// GetAlertByFingerprint 根据指纹获取告警
	GetAlertByFingerprint(tenantId, fingerprint string) (*models.AlertCurEvent, error)
}

func newInterQuickActionService(ctx *ctx.Context) InterQuickActionService {
	return &quickActionService{
		ctx:                 ctx,
		processTraceService: ProcessTraceService, // 注入已经初始化的ProcessTraceService实例
	}
}

// ClaimAlert 认领告警
// 更新告警的认领状态，标记为已认领
// 支持普通告警和拨测告警
func (q *quickActionService) ClaimAlert(tenantId, fingerprint, username, clientIP string) error {
	// 获取目标告警
	targetAlert, err := q.GetAlertByFingerprint(tenantId, fingerprint)
	if err != nil {
		return err
	}

	// 检查是否已经被认领
	if targetAlert.ConfirmState.IsOk {
		return fmt.Errorf("告警已被 %s 认领", targetAlert.ConfirmState.ConfirmUsername)
	}

	// 更新认领状态
	targetAlert.ConfirmState.IsOk = true
	targetAlert.ConfirmState.ConfirmUsername = username
	targetAlert.ConfirmState.ConfirmActionTime = time.Now().Unix()

	// 推送更新后的告警到缓存
	// 注意: 只有接入故障中心的告警(有FaultCenterId)才会保存认领状态
	// 未接入故障中心的拨测告警暂不支持认领功能
	if targetAlert.FaultCenterId != "" {
		q.ctx.Redis.Alert().PushAlertEvent(targetAlert)
		
		// 自动创建处理流程追踪记录（异步执行，失败不影响主流程）
		go q.autoCreateProcessTrace(tenantId, fingerprint, username, targetAlert)
	} else {
		// 未接入故障中心的告警不支持认领
		return fmt.Errorf("该告警未接入故障中心，暂不支持认领功能")
	}

	// 记录审计日志
	q.createAuditLog(tenantId, username, clientIP, "快捷操作-认领告警", map[string]interface{}{
		"fingerprint": fingerprint,
		"ruleName":    targetAlert.RuleName,
		"operator":    username,
		"timestamp":   time.Now().Unix(),
	})

	// 发送确认消息到群聊(异步，失败不影响主流程)
	go func() {
		if err := quickaction.SendConfirmationMessage(q.ctx, targetAlert, "claim", username); err != nil {
			fmt.Printf("发送确认消息失败: %v\n", err)
		}
	}()

	return nil
}

// SilenceAlert 静默告警
// 创建静默规则，在指定时间内抑制该告警
func (q *quickActionService) SilenceAlert(tenantId, fingerprint, duration, username, clientIP string) error {
	return q.silenceAlert(tenantId, fingerprint, duration, username, "", clientIP)
}

// ResolveAlert 标记告警已处理
// 手动标记告警为已恢复状态
// 支持普通告警和拨测告警
func (q *quickActionService) ResolveAlert(tenantId, fingerprint, username, clientIP string) error {
	// 获取目标告警
	targetAlert, err := q.GetAlertByFingerprint(tenantId, fingerprint)
	if err != nil {
		return err
	}

	// 检查告警是否已经恢复
	if targetAlert.IsRecovered {
		return fmt.Errorf("告警已经恢复")
	}

	// 标记为已恢复
	targetAlert.IsRecovered = true
	targetAlert.RecoverTime = time.Now().Unix()

	// 推送更新后的告警到缓存
	// 对于普通告警,更新AlertCache
	if targetAlert.FaultCenterId != "" {
		q.ctx.Redis.Alert().PushAlertEvent(targetAlert)
	} else {
		// 对于拨测告警,需要更新ProbingCache
		err := quickaction.UpdateProbingEventRecovery(q.ctx, tenantId, targetAlert.RuleId, fingerprint)
		if err != nil {
			return fmt.Errorf("更新拨测告警恢复状态失败: %w", err)
		}
	}

	// 记录审计日志
	q.createAuditLog(tenantId, username, clientIP, "快捷操作-标记已处理", map[string]interface{}{
		"fingerprint": fingerprint,
		"ruleName":    targetAlert.RuleName,
		"operator":    username,
		"timestamp":   time.Now().Unix(),
	})

	// 发送确认消息到群聊(异步，失败不影响主流程)
	go func() {
		if err := quickaction.SendConfirmationMessage(q.ctx, targetAlert, "resolve", username); err != nil {
			fmt.Printf("发送确认消息失败: %v\n", err)
		}
	}()

	return nil
}

// SilenceAlertWithReason 静默告警(带原因)
// 与SilenceAlert相比，此方法允许用户提供自定义的静默原因
func (q *quickActionService) SilenceAlertWithReason(tenantId, fingerprint, duration, username, reason, clientIP string) error {
	return q.silenceAlert(tenantId, fingerprint, duration, username, reason, clientIP)
}

// GetAlertByFingerprint 根据指纹获取告警
// 从Redis缓存中查找指定租户下匹配指纹的告警事件
// 支持查找普通告警(AlertCache)和拨测告警(ProbingCache)
func (q *quickActionService) GetAlertByFingerprint(tenantId, fingerprint string) (*models.AlertCurEvent, error) {
	// 1. 先在普通告警缓存(AlertCache)中查找
	faultCenters, err := q.ctx.DB.FaultCenter().List(tenantId, "")
	if err != nil {
		return nil, fmt.Errorf("获取故障中心列表失败: %w", err)
	}

	// 遍历所有故障中心，查找匹配的告警
	for _, fc := range faultCenters {
		// 从AlertCache中获取当前故障中心的告警事件
		events, err := q.ctx.Redis.Alert().GetAllEvents(models.BuildAlertEventCacheKey(tenantId, fc.ID))
		if err != nil {
			continue // 忽略错误，继续搜索下一个故障中心
		}

		// 查找匹配的告警
		for _, alert := range events {
			if alert.Fingerprint == fingerprint {
				return alert, nil
			}
		}
	}

	// 2. 如果在普通告警中没找到，尝试从拨测告警缓存(ProbingCache)中查找
	probingAlert, err := quickaction.FindProbingAlertByFingerprint(q.ctx, tenantId, fingerprint)
	if err == nil && probingAlert != nil {
		return probingAlert, nil
	}

	return nil, fmt.Errorf("未找到指纹为 %s 的告警 或者告警失效了", fingerprint)
}

// ------------------------ 私有辅助方法 ------------------------

// autoCreateProcessTrace 自动创建处理流程追踪记录
// 当用户认领告警时自动调用，确保认领操作与处理流程追踪同步创建
// 失败时仅记录错误，不影响主流程（认领操作）
func (q *quickActionService) autoCreateProcessTrace(tenantId, fingerprint, username string, targetAlert *models.AlertCurEvent) {
	// 防护性检查：确保processTraceService已正确初始化
	if q.processTraceService == nil {
		fmt.Printf("自动创建ProcessTrace失败: processTraceService未初始化, fingerprint=%s\n", fingerprint)
		return
	}

	// 防护性检查：只有接入故障中心的告警才创建处理流程
	if targetAlert.FaultCenterId == "" {
		return
	}

	// 防护性检查：确保EventId存在
	if targetAlert.EventId == "" {
		fmt.Printf("自动创建ProcessTrace失败: 告警EventId为空, fingerprint=%s\n", fingerprint)
		return
	}

	// 调用ProcessTraceService创建处理流程追踪记录
	processTrace, err := q.processTraceService.CreateProcessTrace(
		tenantId,
		targetAlert.EventId,
		targetAlert.FaultCenterId,
		username, // 认领人作为处理负责人
	)

	if err != nil {
		// 记录错误但不中断主流程（认领操作已成功）
		fmt.Printf("自动创建ProcessTrace失败: %v, tenantId=%s, eventId=%s, fingerprint=%s\n", 
			err, tenantId, targetAlert.EventId, fingerprint)
		return
	}

	// 成功创建时记录日志
	fmt.Printf("自动创建ProcessTrace成功: processId=%s, eventId=%s, fingerprint=%s, assignedUser=%s\n",
		processTrace.ID, targetAlert.EventId, fingerprint, username)
}

// silenceAlert 静默告警的内部实现（避免代码重复）
// 参数reason为空时，使用默认注释；否则追加自定义原因
func (q *quickActionService) silenceAlert(tenantId, fingerprint, duration, username, reason, clientIP string) error {
	// 获取告警信息
	targetAlert, err := q.GetAlertByFingerprint(tenantId, fingerprint)
	if err != nil {
		return err
	}

	// 检查是否已经存在该指纹的激活静默规则(防止重复静默)
	existingSilence, err := quickaction.FindActiveSilenceByFingerprint(q.ctx, tenantId, fingerprint)
	if err == nil && existingSilence != nil {
		// 计算剩余静默时间
		remainingTime := existingSilence.EndsAt - time.Now().Unix()
		if remainingTime > 0 {
			remainingDuration := time.Duration(remainingTime) * time.Second
			return fmt.Errorf("该告警已处于静默状态,剩余时长: %s", quickaction.FormatDurationChinese(remainingDuration.String()))
		}
	}

	// 解析静默时长
	dur, err := time.ParseDuration(duration)
	if err != nil {
		return fmt.Errorf("无效的静默时长: %s", duration)
	}

	// 构建静默注释（根据是否有自定义原因）
	comment := fmt.Sprintf("[快捷操作] 由 %s 静默 %s", username, duration)
	if reason != "" {
		comment = fmt.Sprintf("%s\n原因: %s", comment, reason)
	}

	// 创建静默规则
	silence := models.AlertSilences{
		TenantId: tenantId,
		ID:       "s-" + tools.RandId(),
		Name:     fmt.Sprintf("快捷静默-%s", targetAlert.RuleName),
		Labels: []models.SilenceLabel{
			{
				Key:      "fingerprint",
				Value:    fingerprint,
				Operator: "=",
			},
		},
		Comment:       comment,
		StartsAt:      time.Now().Unix(),
		EndsAt:        time.Now().Add(dur).Unix(),
		UpdateAt:      time.Now().Unix(),
		UpdateBy:      username,
		FaultCenterId: targetAlert.FaultCenterId,
		Status:        1, // 状态设置为启用
	}

	// 先推送到Redis缓存，使静默规则立即生效
	q.ctx.Redis.Silence().PushAlertMute(silence)

	// 再保存到数据库进行持久化
	err = q.ctx.DB.Silence().Create(silence)
	if err != nil {
		return fmt.Errorf("创建静默规则失败: %w", err)
	}

	// 记录审计日志
	auditData := map[string]interface{}{
		"fingerprint": fingerprint,
		"ruleName":    targetAlert.RuleName,
		"duration":    duration,
		"operator":    username,
		"silenceId":   silence.ID,
		"timestamp":   time.Now().Unix(),
	}
	if reason != "" {
		auditData["reason"] = reason
	}
	q.createAuditLog(tenantId, username, clientIP, "快捷操作-静默告警", auditData)

	// 更新Redis中的告警状态为静默
	// 注意: 如果告警未被认领，静默操作时自动认领（静默的人就是认领人）
	if targetAlert.FaultCenterId != "" {
		// 计算剩余静默时间
		now := time.Now().Unix()
		remainingTime := silence.EndsAt - now

		// 设置静默信息
		targetAlert.SilenceInfo = &models.SilenceInfo{
			SilenceId:     silence.ID,
			StartsAt:      silence.StartsAt,
			EndsAt:        silence.EndsAt,
			RemainingTime: remainingTime,
			Comment:       silence.Comment,
		}

		// 如果告警未被认领，静默操作时自动认领
		// 业务逻辑：执行静默操作的用户通常意味着正在处理该告警，应该自动认领
		if !targetAlert.ConfirmState.IsOk {
			targetAlert.ConfirmState.IsOk = true
			targetAlert.ConfirmState.ConfirmUsername = username
			targetAlert.ConfirmState.ConfirmActionTime = now
		}
		// 如果已经被认领，保留原有的认领状态（不覆盖）

		// 更新状态为静默中
		// 注意: TransitionStatus 不会影响 ConfirmState，认领状态会被保留
		if err := targetAlert.TransitionStatus(models.StateSilenced); err != nil {
			// 状态转换失败,记录但不中断流程
			fmt.Printf("告警状态转换失败: %v\n", err)
		}

		// 推送到Redis（包含完整的告警信息，包括认领状态）
		// 注意: targetAlert 是从 GetAlertByFingerprint 获取的完整对象，包含所有字段
		q.ctx.Redis.Alert().PushAlertEvent(targetAlert)
	}
	// 注意: 对于未接入故障中心的拨测告警,它们的静默由拨测worker自己处理

	// 发送确认消息到群聊(异步，失败不影响主流程)
	go func() {
		if err := quickaction.SendConfirmationMessage(q.ctx, targetAlert, "silence", username, duration); err != nil {
			fmt.Printf("发送确认消息失败: %v\n", err)
		}
	}()

	return nil
}

// createAuditLog 创建审计日志（通用方法，避免代码重复）
// 将操作详情记录到审计日志表中，用于追踪和审计
func (q *quickActionService) createAuditLog(tenantId, username, clientIP, auditType string, data map[string]interface{}) {
	// 将数据序列化为JSON字符串
	bodyBytes, err := json.Marshal(data)
	if err != nil {
		// 序列化失败时，记录原始错误信息而非终止操作
		bodyBytes = []byte(fmt.Sprintf("{\"error\": \"序列化失败: %s\"}", err.Error()))
	}

	// 构建审计日志记录
	auditLog := models.AuditLog{
		TenantId:   tenantId,
		ID:         "Trace" + tools.RandId(),
		Username:   username,
		IPAddress:  clientIP,
		Method:     "QUICK_ACTION", // 标识为快捷操作
		Path:       "/api/v1/alert/quick-action",
		CreatedAt:  time.Now().Unix(),
		StatusCode: 200,
		Body:       string(bodyBytes),
		AuditType:  auditType,
	}

	// 异步写入审计日志（失败不影响主流程）
	go func() {
		if err := q.ctx.DB.AuditLog().Create(auditLog); err != nil {
			// 审计日志写入失败，仅打印错误，不中断业务流程
			fmt.Printf("审计日志写入失败: %v\n", err)
		}
	}()
}
