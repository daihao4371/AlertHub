package services

import (
	"fmt"
	"regexp"
	"time"

	"alertHub/internal/ctx"
	models "alertHub/internal/models"
	"alertHub/internal/types"
	"alertHub/pkg/quickaction"
	"alertHub/pkg/tools"
)

type alertSilenceService struct {
	alertEvent models.AlertCurEvent
	ctx        *ctx.Context
}

type InterSilenceService interface {
	Create(req interface{}) (interface{}, interface{})
	Update(req interface{}) (interface{}, interface{})
	Delete(req interface{}) (interface{}, interface{})
	List(req interface{}) (interface{}, interface{})
}

func newInterSilenceService(ctx *ctx.Context) InterSilenceService {
	return &alertSilenceService{
		ctx: ctx,
	}
}

// validateSilenceLabels 验证静默规则中所有标签的正则表达式是否合法
func validateSilenceLabels(labels []models.SilenceLabel) error {
	for _, label := range labels {
		if _, err := regexp.Compile(label.Value); err != nil {
			return fmt.Errorf("标签 '%s' 的正则表达式无效: %s, 错误: %v", label.Key, label.Value, err)
		}
	}
	return nil
}

func (ass alertSilenceService) Create(req interface{}) (interface{}, interface{}) {
	r := req.(*types.RequestSilenceCreate)
	updateAt := time.Now().Unix()

	// 验证正则表达式的有效性，防止无效正则导致运行时 panic
	if err := validateSilenceLabels(r.Labels); err != nil {
		return nil, err
	}

	// 根据开始时间判断状态：如果开始时间在未来，则设为未生效(0)，否则为生效中(1)
	status := 1
	if r.StartsAt > updateAt {
		status = 0
	}

	silence := models.AlertSilences{
		TenantId:      r.TenantId,
		Name:          r.Name,
		ID:            "s-" + tools.RandId(),
		StartsAt:      r.StartsAt,
		EndsAt:        r.EndsAt,
		UpdateAt:      updateAt,
		UpdateBy:      r.UpdateBy,
		FaultCenterId: r.FaultCenterId,
		Labels:        r.Labels,
		Comment:       r.Comment,
		Status:        status,
	}

	ass.ctx.Redis.Silence().PushAlertMute(silence)
	err := ass.ctx.DB.Silence().Create(silence)
	if err != nil {
		return nil, err
	}

	// 如果静默规则是针对特定告警的（通过fingerprint标签），发送确认消息到群聊
	// 查找fingerprint标签
	var fingerprint string
	var duration string
	for _, label := range r.Labels {
		if label.Key == "fingerprint" {
			fingerprint = label.Value
			break
		}
	}

	// 如果找到了fingerprint，说明是针对特定告警的静默，需要更新认领状态并发送消息推送
	if fingerprint != "" {
		// 计算静默时长（用于消息显示）
		if r.EndsAt > r.StartsAt {
			durationSeconds := r.EndsAt - r.StartsAt
			dur := time.Duration(durationSeconds) * time.Second
			duration = quickaction.FormatDurationChinese(dur.String())
		}

		// 异步获取告警信息，更新认领状态并发送确认消息
		go func() {
			// 尝试从缓存中获取告警信息
			if r.FaultCenterId != "" {
				alert, err := ass.ctx.Redis.Alert().GetEventFromCache(r.TenantId, r.FaultCenterId, fingerprint)
				if err == nil {
					// 如果告警未被认领，静默操作时自动认领（静默的人就是认领人）
					// 业务逻辑：执行静默操作的用户通常意味着正在处理该告警，应该自动认领
					needUpdate := false
					if !alert.ConfirmState.IsOk {
						alert.ConfirmState.IsOk = true
						alert.ConfirmState.ConfirmUsername = r.UpdateBy
						alert.ConfirmState.ConfirmActionTime = updateAt
						needUpdate = true
					}

					// 如果更新了认领状态，需要推送到Redis
					if needUpdate {
						ass.ctx.Redis.Alert().PushAlertEvent(&alert)
					}

					// 发送确认消息到群聊(异步，失败不影响主流程)
					if err := quickaction.SendConfirmationMessage(ass.ctx, &alert, "silence", r.UpdateBy, duration); err != nil {
						fmt.Printf("发送确认消息失败: %v\n", err)
					}
				}
			}
		}()
	}

	return nil, nil
}

func (ass alertSilenceService) Update(req interface{}) (interface{}, interface{}) {
	r := req.(*types.RequestSilenceUpdate)
	updateAt := time.Now().Unix()

	// 验证正则表达式的有效性，防止无效正则导致运行时 panic
	if err := validateSilenceLabels(r.Labels); err != nil {
		return nil, err
	}

	// 根据开始时间判断状态：如果开始时间在未来，则设为未生效(0)，否则为生效中(1)
	status := 1
	if r.StartsAt > updateAt {
		status = 0
	}

	silence := models.AlertSilences{
		TenantId:      r.TenantId,
		Name:          r.Name,
		ID:            r.ID,
		StartsAt:      r.StartsAt,
		EndsAt:        r.EndsAt,
		UpdateAt:      updateAt,
		UpdateBy:      r.UpdateBy,
		FaultCenterId: r.FaultCenterId,
		Labels:        r.Labels,
		Comment:       r.Comment,
		Status:        status,
	}

	ass.ctx.Redis.Silence().PushAlertMute(silence)
	err := ass.ctx.DB.Silence().Update(silence)
	if err != nil {
		return nil, err
	}

	return nil, nil
}

func (ass alertSilenceService) Delete(req interface{}) (interface{}, interface{}) {
	r := req.(*types.RequestSilenceQuery)
	ass.ctx.Redis.Silence().RemoveAlertMute(r.TenantId, r.FaultCenterId, r.ID)
	err := ass.ctx.DB.Silence().Delete(r.TenantId, r.ID)
	if err != nil {
		return nil, err
	}

	return nil, nil
}

func (ass alertSilenceService) List(req interface{}) (interface{}, interface{}) {
	r := req.(*types.RequestSilenceQuery)
	data, count, err := ass.ctx.DB.Silence().List(r.TenantId, r.FaultCenterId, r.Query, r.Page)
	if err != nil {
		return nil, err
	}

	return types.ResponseSilenceList{
		List: data,
		Page: models.Page{
			Total: count,
			Index: r.Page.Index,
			Size:  r.Page.Size,
		},
	}, nil
}
