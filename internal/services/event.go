package services

import (
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"
	"alertHub/internal/ctx"
	"alertHub/internal/models"
	"alertHub/internal/types"
	"alertHub/pkg/quickaction"
	"alertHub/pkg/tools"
)

type eventService struct {
	ctx                 *ctx.Context
	processTraceService InterProcessTraceService // 处理流程追踪服务，用于自动创建处理记录
}

type InterEventService interface {
	ListCurrentEvent(req interface{}) (interface{}, interface{})
	ListHistoryEvent(req interface{}) (interface{}, interface{})
	ProcessAlertEvent(req interface{}) (interface{}, interface{})
	ListComments(req interface{}) (interface{}, interface{})
	AddComment(req interface{}) (interface{}, interface{})
	DeleteComment(req interface{}) (interface{}, interface{})
}

func newInterEventService(ctx *ctx.Context) InterEventService {
	return &eventService{
		ctx:                 ctx,
		processTraceService: ProcessTraceService, // 注入已经初始化的ProcessTraceService实例
	}
}

func (e eventService) ProcessAlertEvent(req interface{}) (interface{}, interface{}) {
	r := req.(*types.RequestProcessAlertEvent)

	var wg sync.WaitGroup
	wg.Add(len(r.Fingerprints))
	for _, fingerprint := range r.Fingerprints {
		go func(fingerprint string) {
			defer wg.Done()
			cache, err := e.ctx.Redis.Alert().GetEventFromCache(r.TenantId, r.FaultCenterId, fingerprint)
			if err != nil {
				return
			}

			if cache.ConfirmState.IsOk {
				return
			}

			cache.ConfirmState.IsOk = true
			cache.ConfirmState.ConfirmUsername = r.Username
			cache.ConfirmState.ConfirmActionTime = r.Time

			e.ctx.Redis.Alert().PushAlertEvent(&cache)

			// 自动创建处理流程追踪记录（异步执行，失败不影响主流程）
			go e.autoCreateProcessTrace(r.TenantId, fingerprint, r.Username, &cache)

			// 发送确认消息到群聊(异步，失败不影响主流程)
			go func() {
				if err := quickaction.SendConfirmationMessage(e.ctx, &cache, "claim", r.Username); err != nil {
					fmt.Printf("发送确认消息失败: %v\n", err)
				}
			}()
		}(fingerprint)
	}

	wg.Wait()
	return nil, nil
}

func (e eventService) ListCurrentEvent(req interface{}) (interface{}, interface{}) {
	r, ok := req.(*types.RequestAlertCurEventQuery)
	if !ok {
		return nil, fmt.Errorf("invalid request type: expected *models.AlertCurEventQuery")
	}

	center, err := e.ctx.Redis.Alert().GetAllEvents(models.BuildAlertEventCacheKey(r.TenantId, r.FaultCenterId))
	if err != nil {
		return nil, err
	}

	var (
		allEvents      []models.AlertCurEvent
		filteredEvents []models.AlertCurEvent
		curTime        = time.Now()
	)
	for _, alert := range center {
		allEvents = append(allEvents, *alert)
	}

	var form int64
	var to int64
	if r.Scope > 0 {
		to = curTime.Unix()
		form = curTime.Add(-time.Duration(r.Scope) * 24 * time.Hour).Unix()
	}

	for _, event := range allEvents {
		if r.DatasourceType != "" && event.DatasourceType != r.DatasourceType {
			continue
		}

		if r.Severity != "" && event.Severity != r.Severity {
			continue
		}

		if r.Scope > 0 && (event.FirstTriggerTime < form || event.FirstTriggerTime > to) {
			continue
		}

		if r.Query != "" {
			queryMatch := false
			if strings.Contains(event.RuleName, r.Query) {
				queryMatch = true
			} else if strings.Contains(event.Annotations, r.Query) {
				queryMatch = true
			} else if event.Labels != nil && strings.Contains(tools.JsonMarshalToString(event.Labels), r.Query) {
				queryMatch = true
			}
			if !queryMatch {
				continue
			}
		}

		if r.FaultCenterId != "" && !strings.Contains(event.FaultCenterId, r.FaultCenterId) {
			continue
		}

		// 如果没有指定状态过滤，默认过滤掉已恢复的告警（活跃告警列表不应该显示已恢复的告警）
		if r.Status == "" {
			if event.Status == models.StateRecovered {
				continue
			}
		} else if string(event.Status) != r.Status {
			// 如果指定了状态过滤，则按指定状态过滤
			continue
		}

		filteredEvents = append(filteredEvents, event)
	}

	sort.Slice(filteredEvents, func(i, j int) bool {
		a, b := &filteredEvents[i], &filteredEvents[j]

		// 按持续时间降序
		durA := a.LastEvalTime - a.FirstTriggerTime
		durB := b.LastEvalTime - b.FirstTriggerTime
		switch r.SortOrder {
		case models.SortOrderASC:
			if durA != durB {
				return durA < durB // 升序
			}
		case models.SortOrderDesc:
			if durA != durB {
				return durA > durB // 降序
			}
		default:
			if a.FirstTriggerTime != b.FirstTriggerTime {
				return a.Fingerprint < b.Fingerprint
			}
		}

		// 默认按指纹升序
		return a.Fingerprint < b.Fingerprint
	})

	paginatedList := pageSlice(filteredEvents, int(r.Page.Index), int(r.Page.Size))

	// 批量查询并填充认领人真实姓名
	// 只处理已认领的告警（IsOk = true 且 ConfirmUsername 不为空）
	if len(paginatedList) > 0 {
		// 收集所有需要查询的用户名（只收集已认领的）
		usernamesMap := make(map[string]bool)
		for _, event := range paginatedList {
			// 只处理已认领的告警
			if event.ConfirmState.IsOk && event.ConfirmState.ConfirmUsername != "" {
				usernamesMap[event.ConfirmState.ConfirmUsername] = true
			}
		}

		// 批量查询真实姓名
		if len(usernamesMap) > 0 {
			usernames := make([]string, 0, len(usernamesMap))
			for username := range usernamesMap {
				usernames = append(usernames, username)
			}

			var members []models.Member
			e.ctx.DB.DB().Model(&models.Member{}).Where("user_name IN ?", usernames).Find(&members)

			// 创建用户名到真实姓名的映射
			usernameToRealNameMap := make(map[string]string)
			for _, member := range members {
				usernameToRealNameMap[member.UserName] = member.RealName
			}

			// 填充真实姓名（只填充已认领的告警）
			for i := range paginatedList {
				// 只处理已认领的告警
				if paginatedList[i].ConfirmState.IsOk && paginatedList[i].ConfirmState.ConfirmUsername != "" {
					if realName, exists := usernameToRealNameMap[paginatedList[i].ConfirmState.ConfirmUsername]; exists {
						paginatedList[i].ConfirmState.ConfirmUsernameRealName = realName
					}
				}
			}
		}
	}

	return types.ResponseAlertCurEventList{
		List: paginatedList,
		Page: models.Page{
			Total: int64(len(filteredEvents)),
			Index: r.Page.Index,
			Size:  r.Page.Size,
		},
	}, nil
}

func (e eventService) ListHistoryEvent(req interface{}) (interface{}, interface{}) {
	r := req.(*types.RequestAlertHisEventQuery)
	data, err := e.ctx.DB.Event().GetHistoryEvent(*r)
	if err != nil {
		return nil, err
	}

	return data, err

}

func pageSlice(data []models.AlertCurEvent, index, size int) []models.AlertCurEvent {
	if index <= 0 {
		index = 1
	}

	if size <= 0 {
		index = 10
	}

	total := len(data)
	if total == 0 {
		return []models.AlertCurEvent{}
	}

	offset := (index - 1) * size
	if offset >= total {
		return []models.AlertCurEvent{}
	}

	limit := index * size
	if limit > total {
		limit = total
	}

	return data[offset:limit]
}

func (e eventService) ListComments(req interface{}) (interface{}, interface{}) {
	r := req.(*types.RequestListEventComments)
	comment := e.ctx.DB.Comment()
	data, err := comment.List(*r)
	if err != nil {
		return nil, fmt.Errorf("获取评论失败, %s", err.Error())
	}

	return data, nil
}

func (e eventService) AddComment(req interface{}) (interface{}, interface{}) {
	r := req.(*types.RequestAddEventComment)
	comment := e.ctx.DB.Comment()
	err := comment.Add(*r)
	if err != nil {
		return nil, fmt.Errorf("评论失败, %s", err.Error())
	}

	return "评论成功", nil
}

func (e eventService) DeleteComment(req interface{}) (interface{}, interface{}) {
	r := req.(*types.RequestDeleteEventComment)
	comment := e.ctx.DB.Comment()
	err := comment.Delete(*r)
	if err != nil {
		return nil, fmt.Errorf("删除评论失败, %s", err.Error())
	}

	return "删除评论成功", nil
}

// autoCreateProcessTrace 自动创建处理流程追踪记录
// 当用户从故障中心认领告警时自动调用，确保认领操作与处理流程追踪同步创建
// 失败时仅记录错误，不影响主流程（认领操作）
func (e *eventService) autoCreateProcessTrace(tenantId, fingerprint, username string, targetAlert *models.AlertCurEvent) {
	// 防护性检查：确保processTraceService已正确初始化
	if e.processTraceService == nil {
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
	processTrace, err := e.processTraceService.CreateProcessTrace(
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
