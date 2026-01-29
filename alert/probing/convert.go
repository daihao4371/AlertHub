package probing

import (
	"alertHub/internal/models"
)

// ConvertProbingEventToAlertEvent 将拨测事件转换为告警事件
// 用于将拨测告警接入故障中心统一管理
func ConvertProbingEventToAlertEvent(probingEvent *models.ProbingEvent, rule models.ProbingRule) *models.AlertCurEvent {
	return &models.AlertCurEvent{
		TenantId:             probingEvent.TenantId,
		RuleId:               probingEvent.RuleId,
		RuleName:             probingEvent.RuleName,
		DatasourceType:       rule.RuleType,
		DatasourceId:         "probing",
		Fingerprint:          probingEvent.Fingerprint,
		Severity:             rule.Severity,
		Labels:               probingEvent.Labels,
		Annotations:          probingEvent.Annotations,
		IsRecovered:          probingEvent.IsRecovered,
		FirstTriggerTime:     probingEvent.FirstTriggerTime,
		RepeatNoticeInterval: probingEvent.RepeatNoticeInterval,
		LastEvalTime:         probingEvent.LastEvalTime,
		LastSendTime:         probingEvent.LastSendTime,
		RecoverTime:          probingEvent.RecoverTime,
		FaultCenterId:        rule.FaultCenterId,
		EvalInterval:         rule.ProbingEndpointConfig.Strategy.EvalInterval,
		ForDuration:          0,
		EffectiveTime:        models.EffectiveTime{},
	}
}
