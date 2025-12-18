package quickaction

import (
	"fmt"
	"time"
	"alertHub/internal/ctx"
	"alertHub/internal/models"
)

// FindProbingAlertByFingerprint 从拨测告警缓存中查找指定指纹的告警
// 遍历所有拨测规则的缓存，找到匹配的拨测事件并转换为标准告警格式
func FindProbingAlertByFingerprint(ctx *ctx.Context, tenantId, fingerprint string) (*models.AlertCurEvent, error) {
	// 获取租户下所有启用的拨测规则
	var probingRules []models.ProbingRule
	err := ctx.DB.DB().Where("tenant_id = ? AND enabled = ?", tenantId, true).Find(&probingRules).Error
	if err != nil {
		return nil, err
	}

	// 遍历每个拨测规则，查找匹配的告警
	for _, rule := range probingRules {
		// 构建拨测事件缓存key
		cacheKey := models.BuildProbingEventCacheKey(rule.TenantId, rule.RuleId)

		// 从ProbingCache获取拨测事件
		probingEvent, err := ctx.Redis.Probing().GetProbingEventCache(cacheKey)
		if err != nil {
			continue // 忽略错误，继续下一个规则
		}

		// 检查指纹是否匹配
		if probingEvent.Fingerprint == fingerprint {
			// 将ProbingEvent转换为AlertCurEvent
			alertEvent := ConvertProbingEventToAlertEvent(probingEvent)
			return &alertEvent, nil
		}
	}

	return nil, fmt.Errorf("未在拨测告警中找到指纹: %s", fingerprint)
}

// ConvertProbingEventToAlertEvent 将拨测事件转换为标准告警事件
// 确保拨测告警也能被快捷操作正确处理
func ConvertProbingEventToAlertEvent(probingEvent *models.ProbingEvent) models.AlertCurEvent {
	return models.AlertCurEvent{
		TenantId:               probingEvent.TenantId,
		RuleName:               probingEvent.RuleName,
		RuleId:                 probingEvent.RuleId,
		Fingerprint:            probingEvent.Fingerprint,
		Labels:                 probingEvent.Labels,
		Annotations:            probingEvent.Annotations,
		IsRecovered:            probingEvent.IsRecovered,
		FirstTriggerTime:       probingEvent.FirstTriggerTime,
		FirstTriggerTimeFormat: probingEvent.FirstTriggerTimeFormat,
		RepeatNoticeInterval:   probingEvent.RepeatNoticeInterval,
		LastEvalTime:           probingEvent.LastEvalTime,
		LastSendTime:           probingEvent.LastSendTime,
		RecoverTime:            probingEvent.RecoverTime,
		RecoverTimeFormat:      probingEvent.RecoverTimeFormat,
		DutyUser:               probingEvent.DutyUser,
		// 注意: Probing告警没有FaultCenterId,ConfirmState等字段
		// 这些字段保持默认值
	}
}

// UpdateProbingEventRecovery 更新拨测事件的恢复状态
// 从缓存中读取拨测事件,更新恢复状态后写回
func UpdateProbingEventRecovery(ctx *ctx.Context, tenantId, ruleId, fingerprint string) error {
	cacheKey := models.BuildProbingEventCacheKey(tenantId, ruleId)

	// 获取拨测事件
	probingEvent, err := ctx.Redis.Probing().GetProbingEventCache(cacheKey)
	if err != nil {
		return err
	}

	// 验证指纹匹配
	if probingEvent.Fingerprint != fingerprint {
		return fmt.Errorf("指纹不匹配")
	}

	// 更新恢复状态
	probingEvent.IsRecovered = true
	probingEvent.RecoverTime = time.Now().Unix()
	probingEvent.LastSendTime = 0 // 重置发送时间,触发恢复通知

	// 写回缓存
	ctx.Redis.Probing().SetProbingEventCache(*probingEvent, 0)

	return nil
}
