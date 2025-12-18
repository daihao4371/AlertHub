package mute

import (
	"github.com/zeromicro/go-zero/core/logc"
	"regexp"
	"time"
	"alertHub/internal/ctx"
	models "alertHub/internal/models"
	"alertHub/pkg/tools"
)

type MuteParams struct {
	EffectiveTime models.EffectiveTime
	RecoverNotify *bool
	IsRecovered   bool
	TenantId      string
	Labels        map[string]interface{}
	FaultCenterId string
	Fingerprint   string // 添加 Fingerprint 字段用于匹配快捷静默规则
}

func IsMuted(mute MuteParams) bool {
	if IsSilence(mute) {
		return true
	}

	if NotInTheEffectiveTime(mute) {
		return true
	}

	if RecoverNotify(mute) {
		return true
	}

	return false
}

// NotInTheEffectiveTime 判断生效时间
func NotInTheEffectiveTime(mp MuteParams) bool {
	if len(mp.EffectiveTime.Week) <= 0 {
		return false
	}

	// 获取当前日期
	currentTime := time.Now()
	currentWeekday := tools.TimeTransformToWeek(currentTime)
	for _, weekday := range mp.EffectiveTime.Week {
		if currentWeekday == weekday {
			currentTimeSeconds := tools.TimeTransformToSeconds(currentTime)
			return currentTimeSeconds < mp.EffectiveTime.StartTime || currentTimeSeconds > mp.EffectiveTime.EndTime
		}
	}

	return true
}

// RecoverNotify 判断是否推送恢复通知
func RecoverNotify(mp MuteParams) bool {
	return mp.IsRecovered && !*mp.RecoverNotify
}

// IsSilence 判断是否静默
func IsSilence(mute MuteParams) bool {
	return GetMatchedSilenceRule(mute) != nil
}

// GetMatchedSilenceRule 获取匹配的静默规则（如果有）
// 返回匹配的静默规则详情，如果没有匹配则返回nil
func GetMatchedSilenceRule(mute MuteParams) *models.AlertSilences {
	silenceCtx := ctx.Redis.Silence()
	// 获取静默列表中所有的id
	ids, err := silenceCtx.GetAlertMutes(mute.TenantId, mute.FaultCenterId)
	if err != nil {
		logc.Errorf(ctx.Ctx, err.Error())
		return nil
	}

	// 根据ID获取到详细的静默规则
	for _, id := range ids {
		muteRule, err := silenceCtx.WithIdGetMuteFromCache(mute.TenantId, mute.FaultCenterId, id)
		if err != nil {
			logc.Errorf(ctx.Ctx, err.Error())
			continue
		}

		if muteRule.Status != 1 {
			continue
		}

		if evalCondition(mute.Labels, mute.Fingerprint, muteRule.Labels) {
			return muteRule
		}
	}

	return nil
}

func evalCondition(metrics map[string]interface{}, fingerprint string, muteLabels []models.SilenceLabel) bool {
	for _, muteLabel := range muteLabels {
		var val string
		var exists bool

		// 特殊处理 fingerprint 标签：直接使用参数中的 fingerprint
		if muteLabel.Key == "fingerprint" {
			val = fingerprint
			exists = fingerprint != ""
		} else {
			// 其他标签从 metrics 中获取
			value, ok := metrics[muteLabel.Key]
			if !ok {
				return false
			}
			val, exists = value.(string)
			if !exists {
				continue
			}
		}

		if !exists {
			return false
		}

		var matched bool
		switch muteLabel.Operator {
		case "==", "=":
			matched = regexp.MustCompile(muteLabel.Value).MatchString(val)
		case "!=":
			matched = !regexp.MustCompile(muteLabel.Value).MatchString(val)
		default:
			matched = false
		}

		if !matched {
			return false // 只要有一个不匹配，就不静默
		}
	}

	return true
}
