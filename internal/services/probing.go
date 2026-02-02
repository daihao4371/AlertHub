package services

import (
	"fmt"
	"time"
	"alertHub/alert"
	"alertHub/alert/probing"
	"alertHub/internal/ctx"
	"alertHub/internal/models"
	"alertHub/internal/types"
	"alertHub/pkg/client"
	"alertHub/pkg/provider"
	"alertHub/pkg/tools"
)

type (
	probingService struct {
		ctx *ctx.Context
	}

	InterProbingService interface {
		Create(req interface{}) (interface{}, interface{})
		Update(req interface{}) (interface{}, interface{})
		Delete(req interface{}) (interface{}, interface{})
		List(req interface{}) (interface{}, interface{})
		Search(req interface{}) (interface{}, interface{})
		Once(req interface{}) (interface{}, interface{})
		GetHistory(req interface{}) (interface{}, interface{})
		ChangeState(req interface{}) (interface{}, interface{})
	}
)

func newInterProbingService(ctx *ctx.Context) InterProbingService {
	return &probingService{
		ctx: ctx,
	}
}

func (m probingService) Create(req interface{}) (interface{}, interface{}) {
	r := req.(*types.RequestProbingRuleCreate)

	// 验证故障中心是否存在
	if r.FaultCenterId != "" {
		_, err := m.ctx.DB.FaultCenter().Get(r.TenantId, r.FaultCenterId, "")
		if err != nil {
			return nil, fmt.Errorf("故障中心不存在或已删除")
		}
	}

	data := models.ProbingRule{
		TenantId:              r.TenantId,
		RuleName:              r.RuleName,
		RuleId:                "r-" + tools.RandId(),
		RuleType:              r.RuleType,
		RepeatNoticeInterval:  r.RepeatNoticeInterval,
		ProbingEndpointConfig: r.ProbingEndpointConfig,
		ProbingEndpointValues: r.ProbingEndpointValues,
		NoticeId:              r.NoticeId,
		Annotations:           r.Annotations,
		RecoverNotify:         r.RecoverNotify,
		Severity:              r.Severity,
		FaultCenterId:         r.FaultCenterId,
		UpdateAt:              time.Now().Unix(),
		UpdateBy:              r.UpdateBy,
		Enabled:               r.Enabled,
	}

	err := m.ctx.DB.Probing().Create(data)
	if err != nil {
		return nil, err
	}

	// 判断当前节点角色
	if *r.GetEnabled() {
		if alert.IsLeader() {
			// Leader: 直接启动拨测协程
			alert.ProductProbing.Add(data)
			alert.ConsumeProbing.Add(data)
		} else {
			// Follower: 发布消息通知 Leader
			tools.PublishReloadMessage(m.ctx.Ctx, client.Redis, tools.ChannelProbingReload, tools.ReloadMessage{
				Action:   tools.ActionCreate,
				ID:       data.RuleId,
				TenantID: data.TenantId,
				Name:     data.RuleName,
			})
		}
	}

	return nil, nil
}

func (m probingService) Update(req interface{}) (interface{}, interface{}) {
	r := req.(*types.RequestProbingRuleUpdate)

	// 验证故障中心是否存在
	if r.FaultCenterId != "" {
		_, err := m.ctx.DB.FaultCenter().Get(r.TenantId, r.FaultCenterId, "")
		if err != nil {
			return nil, fmt.Errorf("故障中心不存在或已删除")
		}
	}

	data := models.ProbingRule{
		TenantId:              r.TenantId,
		RuleName:              r.RuleName,
		RuleId:                r.RuleId,
		RuleType:              r.RuleType,
		RepeatNoticeInterval:  r.RepeatNoticeInterval,
		ProbingEndpointConfig: r.ProbingEndpointConfig,
		ProbingEndpointValues: r.ProbingEndpointValues,
		NoticeId:              r.NoticeId,
		Annotations:           r.Annotations,
		RecoverNotify:         r.RecoverNotify,
		Severity:              r.Severity,
		FaultCenterId:         r.FaultCenterId,
		UpdateAt:              time.Now().Unix(),
		UpdateBy:              r.UpdateBy,
		Enabled:               r.Enabled,
	}

	_, err := m.ctx.DB.Probing().Search(r.TenantId, r.RuleId)
	if err != nil {
		return nil, err
	}

	err = m.ctx.DB.Probing().Update(data)
	if err != nil {
		return nil, err
	}

	// 判断当前节点角色
	if alert.LeaderElector != nil && alert.LeaderElector.IsLeader() {
		// Leader: 直接重启拨测协程
		alert.ProductProbing.Stop(r.RuleId)
		alert.ConsumeProbing.Stop(r.RuleId)
		if *r.GetEnabled() {
			alert.ProductProbing.Add(data)
			alert.ConsumeProbing.Add(data)
		}
	} else {
		// Follower: 发布消息通知 Leader
		tools.PublishReloadMessage(m.ctx.Ctx, client.Redis, tools.ChannelProbingReload, tools.ReloadMessage{
			Action:   tools.ActionUpdate,
			ID:       r.RuleId,
			TenantID: r.TenantId,
			Name:     r.RuleName,
		})
	}

	return nil, nil
}

func (m probingService) Delete(req interface{}) (interface{}, interface{}) {
	r := req.(*types.RequestProbingRuleQuery)
	res, err := m.ctx.DB.Probing().Search(r.TenantId, r.RuleId)
	if err != nil {
		return nil, err
	}

	err = m.ctx.DB.Probing().Delete(r.TenantId, r.RuleId)
	if err != nil {
		return nil, err
	}

	// 判断当前节点角色
	if alert.LeaderElector != nil && alert.LeaderElector.IsLeader() {
		// Leader: 直接停止拨测协程
		alert.ProductProbing.Stop(r.RuleId)
		alert.ConsumeProbing.Stop(r.RuleId)
	} else {
		// Follower: 发布消息通知 Leader
		tools.PublishReloadMessage(m.ctx.Ctx, client.Redis, tools.ChannelProbingReload, tools.ReloadMessage{
			Action:   tools.ActionDelete,
			ID:       r.RuleId,
			TenantID: r.TenantId,
			Name:     res.RuleName,
		})
	}

	// 清理拨测事件缓存和值缓存
	err = m.ctx.Redis.Redis().Del(string(models.BuildProbingEventCacheKey(res.TenantId, res.RuleId)), string(models.BuildProbingValueCacheKey(res.TenantId, res.RuleId))).Err()
	if err != nil {
		return nil, err
	}

	// 清理故障中心的告警缓存，避免删除拨测任务后告警成为脏数据
	if res.FaultCenterId != "" {
		fingerprints := m.ctx.Redis.Alert().GetFingerprintsByRuleId(res.TenantId, res.FaultCenterId, res.RuleId)
		for _, fingerprint := range fingerprints {
			m.ctx.Redis.Alert().RemoveAlertEvent(res.TenantId, res.FaultCenterId, fingerprint)
		}
	}

	return nil, nil
}

func (m probingService) List(req interface{}) (interface{}, interface{}) {
	r := req.(*types.RequestProbingRuleQuery)
	data, err := m.ctx.DB.Probing().List(r.TenantId, r.RuleType, r.Query)
	if err != nil {
		return nil, err
	}

	for k, v := range data {
		value := &data[k].ProbingEndpointValues
		nv := probing.GetProbingValueMap(models.BuildProbingValueCacheKey(v.TenantId, v.RuleId))
		switch r.RuleType {
		case provider.HTTPEndpointProvider:
			value.PHTTP.Latency = nv["Latency"]
			value.PHTTP.StatusCode = nv["StatusCode"]
		case provider.ICMPEndpointProvider:
			value.PICMP.PacketLoss = nv["PacketLoss"]
			value.PICMP.MinRtt = nv["MinRtt"]
			value.PICMP.MaxRtt = nv["MaxRtt"]
			value.PICMP.AvgRtt = nv["AvgRtt"]
		case provider.TCPEndpointProvider:
			value.PTCP.ErrorMessage = nv["ErrorMessage"]
			value.PTCP.IsSuccessful = nv["IsSuccessful"]
		case provider.SSLEndpointProvider:
			value.PSSL.ExpireTime = nv["ExpireTime"]
			value.PSSL.StartTime = nv["StartTime"]
			value.PSSL.ResponseTime = nv["ResponseTime"]
			value.PSSL.TimeRemaining = nv["TimeRemaining"]
			value.PSSL.StartTimeFormatted = nv["StartTimeFormatted"]   // 中文友好格式的颁发日期
			value.PSSL.ExpireTimeFormatted = nv["ExpireTimeFormatted"] // 中文友好格式的截止日期
		}
	}

	return data, nil
}

func (m probingService) Search(req interface{}) (interface{}, interface{}) {
	r := req.(*types.RequestProbingRuleQuery)
	data, err := m.ctx.DB.Probing().Search(r.TenantId, r.RuleId)
	if err != nil {
		return nil, err
	}

	return data, nil
}

func (m probingService) Once(req interface{}) (interface{}, interface{}) {
	r := req.(*types.RequestProbingOnce)
	var ruleConfig = r.ProbingEndpointConfig
	switch r.RuleType {
	case provider.ICMPEndpointProvider:
		return provider.NewEndpointPinger().Pilot(provider.EndpointOption{
			Endpoint: ruleConfig.Endpoint,
			Timeout:  ruleConfig.Strategy.Timeout,
			ICMP: provider.Eicmp{
				Interval: ruleConfig.ICMP.Interval,
				Count:    ruleConfig.ICMP.Count,
			},
		})
	case provider.HTTPEndpointProvider:
		return provider.NewEndpointHTTPer().Pilot(provider.EndpointOption{
			Endpoint: ruleConfig.Endpoint,
			Timeout:  ruleConfig.Strategy.Timeout,
			HTTP: provider.Ehttp{
				Method: ruleConfig.HTTP.Method,
				Header: ruleConfig.HTTP.Header,
				Body:   ruleConfig.HTTP.Body,
			},
		})
	case provider.TCPEndpointProvider:
		return provider.NewEndpointTcper().Pilot(provider.EndpointOption{
			Endpoint: ruleConfig.Endpoint,
			Timeout:  ruleConfig.Strategy.Timeout,
		})
	case provider.SSLEndpointProvider:
		return provider.NewEndpointSSLer().Pilot(provider.EndpointOption{
			Endpoint: ruleConfig.Endpoint,
			Timeout:  ruleConfig.Strategy.Timeout,
		})
	}
	return nil, nil
}

func (m probingService) GetHistory(req interface{}) (interface{}, interface{}) {
	r := req.(*types.RequestProbingHistoryRecord)
	data, err := m.ctx.DB.Probing().GetRecord(r.RuleId, r.DateRange)
	if err != nil {
		return nil, err
	}

	return data, nil
}

func (m probingService) ChangeState(req interface{}) (interface{}, interface{}) {
	r := req.(*types.RequestProbeChangeState)
	var action string
	switch *r.GetEnabled() {
	case true:
		action = tools.ActionEnable
	case false:
		action = tools.ActionDisable
		m.ctx.Redis.Probing().DelProbingEventCache(models.BuildProbingEventCacheKey(r.TenantId, r.RuleId))
	}

	err := m.ctx.DB.Probing().ChangeState(r.TenantId, r.RuleId, r.GetEnabled())
	if err != nil {
		return nil, err
	}

	// 判断当前节点角色
	rule, _ := m.ctx.DB.Probing().Search(r.TenantId, r.RuleId)
	if alert.LeaderElector != nil && alert.LeaderElector.IsLeader() {
		// Leader: 直接操作协程
		switch *r.GetEnabled() {
		case true:
			alert.ProductProbing.Add(rule)
			alert.ConsumeProbing.Add(rule)
		case false:
			alert.ProductProbing.Stop(r.RuleId)
			alert.ConsumeProbing.Stop(r.RuleId)
		}
	} else {
		// Follower: 发布消息通知 Leader
		tools.PublishReloadMessage(m.ctx.Ctx, client.Redis, tools.ChannelProbingReload, tools.ReloadMessage{
			Action:   action,
			ID:       r.RuleId,
			TenantID: r.TenantId,
			Name:     rule.RuleName,
		})
	}

	return nil, nil
}
