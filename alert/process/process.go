package process

import (
	"fmt"
	"time"
	"alertHub/alert/mute"
	"alertHub/internal/ctx"
	"alertHub/internal/models"
	"alertHub/pkg/analysis"

	"github.com/zeromicro/go-zero/core/logc"
)

func BuildEvent(rule models.AlertRule, labels func() map[string]interface{}) models.AlertCurEvent {
	return models.AlertCurEvent{
		TenantId:             rule.TenantId,
		DatasourceType:       rule.DatasourceType,
		RuleGroupId:          rule.RuleGroupId,
		RuleId:               rule.RuleId,
		RuleName:             rule.RuleName,
		Labels:               labels(),
		EvalInterval:         rule.EvalInterval,
		IsRecovered:          false,
		RepeatNoticeInterval: rule.RepeatNoticeInterval,
		Severity:             rule.Severity,
		EffectiveTime:        rule.EffectiveTime,
		FaultCenterId:        rule.FaultCenterId,
	}
}

func PushEventToFaultCenter(ctx *ctx.Context, event *models.AlertCurEvent) {
	if event == nil {
		return
	}

	ctx.Mux.Lock()
	defer ctx.Mux.Unlock()
	if len(event.TenantId) <= 0 || len(event.Fingerprint) <= 0 {
		return
	}

	cache := ctx.Redis
	cacheEvent, err := cache.Alert().GetEventFromCache(event.TenantId, event.FaultCenterId, event.Fingerprint)

	// 如果是恢复事件但找不到缓存事件，记录警告日志
	if event.IsRecovered && (err != nil || cacheEvent.RuleId == "") {
		logc.Errorf(ctx.Ctx, "[恢复事件警告] 找不到缓存事件: ruleId=%s, fingerprint=%s, ruleName=%s, error=%v",
			event.RuleId, event.Fingerprint, event.RuleName, err)
		// 如果找不到缓存事件，尝试通过 ruleId 查找（兼容旧指纹）
		if event.RuleId != "" {
			fingerprints := cache.Alert().GetFingerprintsByRuleId(event.TenantId, event.FaultCenterId, event.RuleId)
			if len(fingerprints) > 0 {
				// 使用第一个找到的指纹（通常是旧的基于 address 的指纹）
				cacheEvent, _ = cache.Alert().GetEventFromCache(event.TenantId, event.FaultCenterId, fingerprints[0])
				// 更新 event 的指纹为找到的旧指纹，确保能正确更新缓存
				event.Fingerprint = fingerprints[0]
				logc.Infof(ctx.Ctx, "[恢复事件] 通过 ruleId 找到旧指纹: ruleId=%s, oldFingerprint=%s, newFingerprint=%s",
					event.RuleId, fingerprints[0], event.Fingerprint)
			}
		}
	}

	// 获取基础信息
	event.FirstTriggerTime = cacheEvent.GetFirstTime()
	event.LastEvalTime = cacheEvent.GetLastEvalTime()
	event.LastSendTime = cacheEvent.GetLastSendTime()
	event.ConfirmState = cacheEvent.GetLastConfirmState()
	event.EventId = cacheEvent.GetEventId()
	event.FaultCenter = cache.FaultCenter().GetFaultCenterInfo(models.BuildFaultCenterInfoCacheKey(event.TenantId, event.FaultCenterId))

	// 如果是恢复事件，重置 LastSendTime 为 0，确保恢复通知能够发送
	// 因为 consumer 中恢复事件只有在 LastSendTime == 0 时才会发送
	if event.IsRecovered {
		event.LastSendTime = 0
	}

	// 获取当前缓存中的状态
	currentStatus := cacheEvent.GetEventStatus()

	// 如果是新的告警事件，设置为 StatePreAlert
	if currentStatus == "" {
		event.Status = models.StatePreAlert
	} else {
		event.Status = currentStatus
	}

	// 检查是否处于静默状态，并获取匹配的静默规则
	matchedSilence := GetMatchedSilenceRule(event)
	isSilenced := matchedSilence != nil

	// 如果匹配到静默规则，设置静默信息
	if isSilenced {
		now := time.Now().Unix()
		event.SilenceInfo = &models.SilenceInfo{
			SilenceId:     matchedSilence.ID,
			StartsAt:      matchedSilence.StartsAt,
			EndsAt:        matchedSilence.EndsAt,
			RemainingTime: matchedSilence.EndsAt - now,
			Comment:       matchedSilence.Comment,
		}
	} else {
		// 清除静默信息
		event.SilenceInfo = nil
	}

	// 根据不同情况处理状态转换
	switch event.Status {
	case models.StatePreAlert:
		// 如果需要静默
		if isSilenced {
			event.TransitionStatus(models.StateSilenced)
		} else if event.IsRecovered {
			// 如果已恢复，但当前处于预告警状态，允许直接转换到已恢复状态
			// 这种情况通常发生在拨测告警还未达到持续时间就恢复了（快速恢复场景）
			event.TransitionStatus(models.StateRecovered)
		} else if event.IsArriveForDuration() {
			// 如果达到持续时间，转为告警状态
			event.TransitionStatus(models.StateAlerting)
		}
	case models.StateAlerting:
		// 优先检查是否恢复
		if event.IsRecovered {
			// 告警恢复：告警中 → 已恢复
			if err := event.TransitionStatus(models.StateRecovered); err != nil {
				logc.Errorf(ctx.Ctx, "[状态转换失败] 告警中→已恢复: ruleId=%s, fingerprint=%s, error=%v",
					event.RuleId, event.Fingerprint, err)
			} else {
				logc.Infof(ctx.Ctx, "[状态转换成功] 告警中→已恢复: ruleId=%s, fingerprint=%s, ruleName=%s",
					event.RuleId, event.Fingerprint, event.RuleName)
			}
		} else if isSilenced {
			// 如果需要静默
			event.TransitionStatus(models.StateSilenced)
		}
	case models.StatePendingRecovery:
		// 待恢复状态的处理
		if event.IsRecovered {
			// 待恢复 → 已恢复
			event.TransitionStatus(models.StateRecovered)
		} else {
			// 如果又出现告警（恢复失败），转回告警状态
			event.TransitionStatus(models.StateAlerting)
		}
	case models.StateSilenced:
		// 优先检查是否恢复
		if event.IsRecovered {
			// 静默中恢复：静默中 → 已恢复
			event.TransitionStatus(models.StateRecovered)
		} else if !isSilenced {
			// 如果不再静默，转换回预告警状态
			event.TransitionStatus(models.StatePreAlert)
		}
	case models.StateRecovered:
		// 已恢复状态下，如果再次触发告警（非恢复事件），转回预告警状态
		if !event.IsRecovered {
			event.TransitionStatus(models.StatePreAlert)
		}
	}

	// 最终再次校验 fingerprint 非空，避免 push 时使用空 key
	if event.Fingerprint == "" {
		logc.Errorf(ctx.Ctx, "PushEventToFaultCenter: fingerprint became empty before PushAlertEvent, tenant=%s, rule=%s(%s)", event.TenantId, event.RuleName, event.RuleId)
		return
	}

	// 更新缓存
	cache.Alert().PushAlertEvent(event)
	
	// 🧠 新增：触发智能分析
	// 只有在新告警或状态变化时才触发智能分析
	if !event.IsRecovered && (event.Status == models.StateAlerting || event.Status == models.StatePreAlert) {
		analysis.TriggerIntelligentAnalysis(ctx, event)
	}
}

// IsSilencedEvent 静默检查
func IsSilencedEvent(event *models.AlertCurEvent) bool {
	return mute.IsSilence(mute.MuteParams{
		EffectiveTime: event.EffectiveTime,
		IsRecovered:   event.IsRecovered,
		TenantId:      event.TenantId,
		Labels:        event.Labels,
		FaultCenterId: event.FaultCenterId,
		Fingerprint:   event.Fingerprint,
	})
}

// GetMatchedSilenceRule 获取匹配的静默规则
// 返回匹配的静默规则详情，如果没有匹配则返回nil
func GetMatchedSilenceRule(event *models.AlertCurEvent) *models.AlertSilences {
	return mute.GetMatchedSilenceRule(mute.MuteParams{
		EffectiveTime: event.EffectiveTime,
		IsRecovered:   event.IsRecovered,
		TenantId:      event.TenantId,
		Labels:        event.Labels,
		FaultCenterId: event.FaultCenterId,
		Fingerprint:   event.Fingerprint,
	})
}

func GetDutyUsers(ctx *ctx.Context, noticeData models.AlertNotice) []string {
	var us []string
	users, ok := ctx.DB.DutyCalendar().GetDutyUserInfo(*noticeData.GetDutyId(), time.Now().Format("2006-1-2"))
	if ok {
		switch noticeData.NoticeType {
		case "FeiShu":
			for _, user := range users {
				us = append(us, fmt.Sprintf("<at id=%s></at>", user.DutyUserId))
			}
			return us
		case "DingDing":
			for _, user := range users {
				us = append(us, fmt.Sprintf("@%s", user.DutyUserId))
			}
			return us
		case "Email", "WeChat", "CustomHook":
			for _, user := range users {
				us = append(us, fmt.Sprintf("@%s", user.UserName))
			}
			return us
		case "Slack":
			for _, user := range users {
				us = append(us, fmt.Sprintf("<@%s>", user.DutyUserId))
			}
			return us
		}
	}

	return []string{"暂无"}
}

// GetDutyUserPhoneNumber 获取当班人员手机号
func GetDutyUserPhoneNumber(ctx *ctx.Context, noticeData models.AlertNotice) []string {
	//user, ok := ctx.DB.DutyCalendar().GetDutyUserInfo(*noticeData.GetDutyId(), time.Now().Format("2006-1-2"))
	//if ok {
	//	switch noticeData.NoticeType {
	//	case "PhoneCall":
	//		if len(user.DutyUserId) > 1 {
	//			return []string{user.Phone}
	//		}
	//	}
	//}
	return []string{}
}

// RecordAlertHisEvent 记录历史告警
func RecordAlertHisEvent(ctx *ctx.Context, alert models.AlertCurEvent) error {
	hisData := models.AlertHisEvent{
		TenantId:         alert.TenantId,
		EventId:          alert.EventId,
		DatasourceType:   alert.DatasourceType,
		DatasourceId:     alert.DatasourceId,
		Fingerprint:      alert.Fingerprint,
		RuleId:           alert.RuleId,
		RuleName:         alert.RuleName,
		Severity:         alert.Severity,
		Labels:           alert.Labels,
		EvalInterval:     alert.EvalInterval,
		Annotations:      alert.Annotations,
		FirstTriggerTime: alert.FirstTriggerTime,
		LastEvalTime:     alert.LastEvalTime,
		LastSendTime:     alert.LastSendTime,
		RecoverTime:      alert.RecoverTime,
		FaultCenterId:    alert.FaultCenterId,
		ConfirmState:     alert.ConfirmState,
		AlarmDuration:    alert.RecoverTime - alert.FirstTriggerTime,
		SearchQL:         alert.SearchQL,
	}

	err := ctx.DB.Event().CreateHistoryEvent(hisData)
	if err != nil {
		return fmt.Errorf("RecordAlertHisEvent, 恢复告警记录失败, err: %s", err)
	}

	return nil
}
