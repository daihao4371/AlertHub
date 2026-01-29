package probing

import (
	"alertHub/alert/process"
	"alertHub/internal/ctx"
	"alertHub/internal/models"
	"alertHub/pkg/provider"
	"alertHub/pkg/tools"
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/go-redis/redis"
	"github.com/zeromicro/go-zero/core/logc"
	"golang.org/x/sync/errgroup"
)

// safeConvertToFloat64 安全地将 interface{} 转换为 float64
// 如果转换失败，返回 0 和错误信息
func safeConvertToFloat64(value interface{}, fieldName string) (float64, error) {
	if value == nil {
		return 0, fmt.Errorf("field %s is nil", fieldName)
	}

	switch v := value.(type) {
	case float64:
		return v, nil
	case float32:
		return float64(v), nil
	case int:
		return float64(v), nil
	case int64:
		return float64(v), nil
	case int32:
		return float64(v), nil
	case string:
		// 尝试解析字符串为 float64
		result, err := strconv.ParseFloat(v, 64)
		if err != nil {
			return 0, fmt.Errorf("field %s cannot be converted to float64: %v", fieldName, err)
		}
		return result, nil
	default:
		return 0, fmt.Errorf("field %s has unsupported type: %T", fieldName, value)
	}
}

type ProductProbing struct {
	ctx           *ctx.Context
	WatchCtxMap   map[string]context.CancelFunc
	FailFrequency map[string]int
	OkFrequency   map[string]int
}

func NewProbingTask(ctx *ctx.Context) ProductProbing {
	return ProductProbing{
		ctx:           ctx,
		FailFrequency: make(map[string]int),
		OkFrequency:   make(map[string]int),
		WatchCtxMap:   make(map[string]context.CancelFunc),
	}
}

func (t *ProductProbing) Add(rule models.ProbingRule) {
	t.ctx.Mux.Lock()
	defer t.ctx.Mux.Unlock()

	c, cancel := context.WithCancel(t.ctx.Ctx)
	t.WatchCtxMap[rule.RuleId] = cancel
	go t.Eval(c, rule)
}

func (t *ProductProbing) Stop(id string) {
	t.ctx.Mux.Lock()
	defer t.ctx.Mux.Unlock()

	if cancel, exists := t.WatchCtxMap[id]; exists {
		cancel()
		delete(t.WatchCtxMap, id)
	}
}

func (t *ProductProbing) Eval(ctx context.Context, rule models.ProbingRule) {
	timer := time.NewTicker(time.Second * time.Duration(rule.ProbingEndpointConfig.Strategy.EvalInterval))
	defer timer.Stop()
	t.worker(rule)

	for {
		select {
		case <-timer.C:
			//logc.Infof(t.ctx.Ctx, fmt.Sprintf("网络监控: %s", tools.JsonMarshal(rule)))
			t.worker(rule)
		case <-ctx.Done():
			return
		}
	}
}

func (t *ProductProbing) worker(rule models.ProbingRule) {
	var (
		eValue     provider.EndpointValue
		err        error
		ruleConfig = rule.ProbingEndpointConfig
	)

	eValue, err = t.runProbing(rule)
	if err != nil {
		logc.Errorf(t.ctx.Ctx, err.Error())
		return
	}
	err = t.ctx.DB.Probing().AddRecord(models.ProbingHistory{
		Timestamp: time.Now().Unix(),
		RuleId:    rule.RuleId,
		Value:     eValue,
	})
	if err != nil {
		logc.Errorf(t.ctx.Ctx, err.Error())
		return
	}

	event := t.buildEvent(rule)
	// 拨测告警的指纹基于 ruleId，而不是 address
	// 这样即使域名变化，也能正确匹配到同一个规则下的告警事件
	event.Fingerprint = tools.Md5Hash([]byte(event.RuleId))
	event.Labels = eValue.GetLabels()
	var isValue float64
	var queryValue float64

	if rule.RuleType != provider.TCPEndpointProvider {
		// 安全地获取字段值并转换为 float64
		fieldValue, exists := eValue[ruleConfig.Strategy.Field]
		if !exists {
			logc.Errorf(t.ctx.Ctx, "field %s not found in endpoint value for rule %s", ruleConfig.Strategy.Field, rule.RuleId)
			return
		}

		var convertErr error
		queryValue, convertErr = safeConvertToFloat64(fieldValue, ruleConfig.Strategy.Field)
		if convertErr != nil {
			logc.Errorf(t.ctx.Ctx, "failed to convert field %s to float64 for rule %s: %v", ruleConfig.Strategy.Field, rule.RuleId, convertErr)
			return
		}
		event.Labels["value"] = queryValue
	} else {
		if eValue["IsSuccessful"] == true {
			isValue = 1
		}
		event.Labels["value"] = isValue
	}
	event.Annotations = tools.ParserVariables(rule.Annotations, event.Labels)

	var option models.EvalCondition
	switch rule.RuleType {
	// 如果拨测类型是 TCP ，直接定义好计算条件 == 0 则表示异常
	case provider.TCPEndpointProvider:
		option = models.EvalCondition{
			Operator:      "==",
			QueryValue:    isValue,
			ExpectedValue: 0,
		}
	default:
		option = models.EvalCondition{
			Operator:      ruleConfig.Strategy.Operator,
			QueryValue:    queryValue,
			ExpectedValue: ruleConfig.Strategy.ExpectedValue,
		}
	}

	err = SetProbingValueMap(models.BuildProbingValueCacheKey(event.TenantId, event.RuleId), eValue)
	if err != nil {
		return
	}

	// 判断是否接入故障中心
	if rule.FaultCenterId != "" {
		// 转换为告警事件并推送到故障中心
		alertEvent := ConvertProbingEventToAlertEvent(event, rule)

		// 评估条件，决定是否触发告警或恢复
		if process.EvalCondition(option) {
			// 控制失败频次
			t.setFrequency(t.FailFrequency, event.RuleId)
			// 达到失败次数后推送告警事件
			if t.getFrequency(t.FailFrequency, event.RuleId) >= event.ProbingEndpointConfig.Strategy.Failure {
				defer func() {
					t.cleanFrequency(t.FailFrequency, event.RuleId)
					t.cleanFrequency(t.OkFrequency, event.RuleId)
				}()
				alertEvent.IsRecovered = false
				process.PushEventToFaultCenter(t.ctx, alertEvent)
			}
		} else {
			// 检查缓存中的告警是否已经恢复，避免重复推送恢复事件
			cacheEvent, err := t.ctx.Redis.Alert().GetEventFromCache(event.TenantId, rule.FaultCenterId, event.Fingerprint)
			// 如果通过新指纹找不到告警事件，尝试通过 ruleId 查找（兼容旧指纹）
			if err != nil || cacheEvent.RuleId == "" {
				fingerprints := t.ctx.Redis.Alert().GetFingerprintsByRuleId(event.TenantId, rule.FaultCenterId, event.RuleId)
				if len(fingerprints) > 0 {
					// 使用第一个找到的指纹（通常是旧的基于 address 的指纹）
					cacheEvent, _ = t.ctx.Redis.Alert().GetEventFromCache(event.TenantId, rule.FaultCenterId, fingerprints[0])
					// 更新 alertEvent 的指纹为找到的旧指纹，确保能正确更新缓存
					alertEvent.Fingerprint = fingerprints[0]
				} else {
					// 如果通过 ruleId 也找不到任何告警事件，说明告警还没有触发过
					// 或者告警已经被清理了，此时不应该推送恢复事件，直接返回
					t.cleanFrequency(t.FailFrequency, event.RuleId)
					t.cleanFrequency(t.OkFrequency, event.RuleId)
					return
				}
			}

			// 如果缓存中的告警事件为空（RuleId 为空），说明没有找到有效的告警事件
			if cacheEvent.RuleId == "" {
				// 没有找到告警事件，不应该推送恢复事件
				t.cleanFrequency(t.FailFrequency, event.RuleId)
				t.cleanFrequency(t.OkFrequency, event.RuleId)
				return
			}

			if cacheEvent.IsRecovered {
				// 已经恢复过了，不再重复推送
				// 但需要清理内存中的失败频次计数器
				t.cleanFrequency(t.FailFrequency, event.RuleId)
				return
			}

			// 控制成功频次
			t.setFrequency(t.OkFrequency, event.RuleId)
			if t.getFrequency(t.OkFrequency, event.RuleId) >= 3 {
				defer func() {
					t.cleanFrequency(t.FailFrequency, event.RuleId)
					t.cleanFrequency(t.OkFrequency, event.RuleId)
				}()
				// 标记为已恢复
				alertEvent.IsRecovered = true
				alertEvent.RecoverTime = time.Now().Unix()
				// 重置 LastSendTime 为 0，确保恢复通知能够发送
				// 因为 consumer 中恢复事件只有在 LastSendTime == 0 时才会发送
				alertEvent.LastSendTime = 0
				// 记录恢复事件推送日志
				logc.Infof(t.ctx.Ctx, "[拨测恢复] 推送恢复事件: ruleId=%s, fingerprint=%s, ruleName=%s",
					event.RuleId, alertEvent.Fingerprint, event.RuleName)
				process.PushEventToFaultCenter(t.ctx, alertEvent)
			}
		}
	} else {
		// 未配置故障中心，使用原有逻辑
		t.Evaluation(event, option)
	}
}

func (t *ProductProbing) runProbing(rule models.ProbingRule) (provider.EndpointValue, error) {
	var ruleConfig = rule.ProbingEndpointConfig
	switch rule.RuleType {
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
	return provider.EndpointValue{}, fmt.Errorf("unsupported rule type: %s", rule.RuleType)
}

func (t *ProductProbing) Evaluation(event *models.ProbingEvent, option models.EvalCondition) {
	if event == nil {
		return
	}

	key := models.BuildProbingEventCacheKey(event.TenantId, event.RuleId)
	c := ctx.Redis.Probing()
	event.FirstTriggerTime = c.GetProbingEventFirstTime(key)

	if process.EvalCondition(option) {
		// 控制失败频次
		t.setFrequency(t.FailFrequency, event.RuleId)
		// 如果失败频次达到设定次数后记录事件
		if t.getFrequency(t.FailFrequency, event.RuleId) >= event.ProbingEndpointConfig.Strategy.Failure {
			defer func() {
				t.cleanFrequency(t.FailFrequency, event.RuleId)
				t.cleanFrequency(t.OkFrequency, event.RuleId)
			}()

			event.LastEvalTime = c.GetProbingEventLastEvalTime(key)
			event.LastSendTime = c.GetProbingEventLastSendTime(key)
			c.SetProbingEventCache(*event, 0)
		}

	} else {
		neCache, err := c.GetProbingEventCache(key)
		if err != nil && err == redis.Nil {
			return
		}

		if neCache.IsRecovered {
			return
		}

		// 控制成功频次
		t.setFrequency(t.OkFrequency, event.RuleId)
		if t.getFrequency(t.OkFrequency, event.RuleId) >= 3 {
			defer func() {
				t.cleanFrequency(t.FailFrequency, event.RuleId)
				t.cleanFrequency(t.OkFrequency, event.RuleId)
			}()

			// 触发恢复后，需要更新缓存中的 label、annotations
			neCache.Labels = event.Labels
			neCache.Annotations = event.Annotations

			// 标记恢复状态
			neCache.IsRecovered = true
			neCache.RecoverTime = time.Now().Unix()
			neCache.LastSendTime = 0
			c.SetProbingEventCache(*neCache, 0)
		}
	}
}

func (t *ProductProbing) RePushRule(consumer *ConsumeProbing) {
	var ruleList []models.ProbingRule
	if err := t.ctx.DB.DB().Where("enabled = ?", true).Find(&ruleList).Error; err != nil {
		logc.Errorf(t.ctx.Ctx, err.Error())
		return
	}

	g := new(errgroup.Group)
	for _, rule := range ruleList {
		rule := rule
		g.Go(func() error {
			t.Add(rule)
			consumer.Add(rule)
			return nil
		})
	}

	if err := g.Wait(); err != nil {
		logc.Errorf(t.ctx.Ctx, err.Error())
	}
}

// StopAllTasks 停止所有拨测任务
func (t *ProductProbing) StopAllTasks() {
	t.ctx.Mux.Lock()
	defer t.ctx.Mux.Unlock()

	count := len(t.WatchCtxMap)
	if count == 0 {
		return
	}

	logc.Infof(t.ctx.Ctx, "停止 %d 个拨测生产任务...", count)

	// 取消所有拨测任务
	for ruleId, cancel := range t.WatchCtxMap {
		cancel()
		delete(t.WatchCtxMap, ruleId)
	}

	logc.Infof(t.ctx.Ctx, "所有拨测生产任务已停止")
}

func (t *ProductProbing) setFrequency(frequencyStorage map[string]int, ruleId string) {
	t.ctx.Mux.Lock()
	defer t.ctx.Mux.Unlock()

	frequencyStorage[ruleId]++
}

func (t *ProductProbing) getFrequency(frequencyStorage map[string]int, ruleId string) int {
	t.ctx.Mux.RLock()
	defer t.ctx.Mux.RUnlock()

	return frequencyStorage[ruleId]
}

func (t *ProductProbing) cleanFrequency(frequencyStorage map[string]int, ruleId string) {
	delete(frequencyStorage, ruleId)
}
