package eval

import (
	"alertHub/internal/ctx"
	"alertHub/internal/models"
	"alertHub/pkg/provider"
	"alertHub/pkg/tools"
	"context"
	"fmt"
	"runtime/debug"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/go-redis/redis"
	"github.com/zeromicro/go-zero/core/logc"
)

const (
	// 数据源类型
	DatasourceTypePrometheus      = "Prometheus"
	DatasourceTypeVictoriaMetrics = "VictoriaMetrics"
	DatasourceTypeAliCloudSLS     = "AliCloudSLS"
	DatasourceTypeLoki            = "Loki"
	DatasourceTypeElasticSearch   = "ElasticSearch"
	DatasourceTypeVictoriaLogs    = "VictoriaLogs"
	DatasourceTypeClickHouse      = "ClickHouse"
	DatasourceTypeJaeger          = "Jaeger"
	DatasourceTypeCloudWatch      = "CloudWatch"
	DatasourceTypeKubernetesEvent = "KubernetesEvent"

	// 时间类型
	TimeTypeMillisecond = "millisecond"
	TimeTypeSecond      = "second"

	// 默认恢复等待时间
	DefaultRecoverWaitTime = 1

	// 任务通道缓冲区大小
	TaskChannelBufferSize = 1
)

// EvalResult 评估结果，包含当前活跃告警的指纹列表和所有指标的当前值
// 用于在恢复告警时更新指标值（如磁盘使用率）为当前实际值
type EvalResult struct {
	// Fingerprints 当前活跃告警的指纹列表
	Fingerprints []string
	// CurrentValues 所有指标的当前值（fingerprint -> value）
	// 用于恢复时更新 Labels["value"] 为当前实际值
	CurrentValues map[string]float64
}

// 数据源处理器映射
// 返回 EvalResult 包含指纹列表和当前指标值，用于恢复时更新值
var datasourceHandlers = map[string]func(*ctx.Context, string, string, models.AlertRule) EvalResult{
	DatasourceTypePrometheus:      metrics,
	DatasourceTypeVictoriaMetrics: metrics,
	DatasourceTypeAliCloudSLS:     logs,
	DatasourceTypeLoki:            logs,
	DatasourceTypeElasticSearch:   logs,
	DatasourceTypeVictoriaLogs:    logs,
	DatasourceTypeClickHouse:      logs,
	DatasourceTypeJaeger:          traces,
	DatasourceTypeCloudWatch:      cloudWatch,
	DatasourceTypeKubernetesEvent: kubernetesEvent,
}

type (
	// AlertRuleEval 告警规则评估
	AlertRuleEval interface {
		Submit(rule models.AlertRule)
		Stop(ruleId string)
		Eval(ctx context.Context, rule models.AlertRule)
		Recover(tenantId, ruleId string, eventCacheKey models.AlertEventCacheKey, faultCenterInfoKey models.FaultCenterInfoCacheKey, curFingerprints []string, currentValues map[string]float64, annotationTemplate string)
		RestartAllEvals()
		StopAllEvals()
	}

	// AlertRule 告警规则
	AlertRule struct {
		ctx *ctx.Context
	}
)

func NewAlertRuleEval(ctx *ctx.Context) AlertRuleEval {
	return &AlertRule{
		ctx: ctx,
	}
}

func (t *AlertRule) Submit(rule models.AlertRule) {
	t.ctx.Mux.Lock()
	defer t.ctx.Mux.Unlock()

	c, cancel := context.WithCancel(context.Background())
	t.ctx.ContextMap[rule.RuleId] = cancel
	go t.Eval(c, rule)
}

func (t *AlertRule) Stop(ruleId string) {
	t.ctx.Mux.Lock()
	defer t.ctx.Mux.Unlock()

	if cancel, exists := t.ctx.ContextMap[ruleId]; exists {
		cancel()
		delete(t.ctx.ContextMap, ruleId)
	}
}

func (t *AlertRule) Restart(rule models.AlertRule) {
	t.Stop(rule.RuleId)
	t.Submit(rule)
}

func (t *AlertRule) Eval(ctx context.Context, rule models.AlertRule) {
	taskChan := make(chan struct{}, TaskChannelBufferSize)
	timer := time.NewTicker(t.getEvalTimeDuration(rule.EvalTimeType, rule.EvalInterval))
	defer func() {
		timer.Stop()
		if r := recover(); r != nil {
			// 获取调用栈信息
			stack := debug.Stack()
			logc.Error(t.ctx.Ctx, fmt.Sprintf("Recovered from rule eval goroutine panic: %s, RuleName: %s, RuleId: %s\n%s", r, rule.RuleName, rule.RuleId, stack))
			t.Restart(rule)
		}
	}()

	for {
		select {
		case <-timer.C:
			// 处理任务信号量
			taskChan <- struct{}{}
			t.executeTask(rule, taskChan)
		case <-ctx.Done():
			logc.Infof(t.ctx.Ctx, fmt.Sprintf("停止 RuleId: %v, RuleName: %s 的 Watch 协程", rule.RuleId, rule.RuleName))
			return
		}
		timer.Reset(t.getEvalTimeDuration(rule.EvalTimeType, rule.EvalInterval))
	}
}

// executeTask 执行评估任务
func (t *AlertRule) executeTask(rule models.AlertRule, taskChan chan struct{}) {
	defer func() {
		// 释放任务信号量
		<-taskChan
	}()

	// 在规则评估前检查是否仍然启用
	if !t.isRuleEnabled(rule.RuleId) {
		return
	}

	// 并发处理数据源
	evalResult := t.processDatasources(rule)

	// 处理恢复逻辑，传入当前指标值和注解模板用于恢复时更新
	t.Recover(rule.TenantId, rule.RuleId,
		models.BuildAlertEventCacheKey(rule.TenantId, rule.FaultCenterId),
		models.BuildFaultCenterInfoCacheKey(rule.TenantId, rule.FaultCenterId),
		evalResult.Fingerprints,
		evalResult.CurrentValues,
		rule.PrometheusConfig.Annotations)
}

// processDatasources 处理数据源
func (t *AlertRule) processDatasources(rule models.AlertRule) EvalResult {
	var (
		curFingerprints []string
		currentValues   = make(map[string]float64)
		resultChan      = make(chan EvalResult, len(rule.DatasourceIdList))
		wg              sync.WaitGroup
	)

	// 启动工作协程
	for _, dsId := range rule.DatasourceIdList {
		wg.Add(1)
		go func(dsId string) {
			defer wg.Done()
			result := t.processSingleDatasource(dsId, rule)
			resultChan <- result
		}(dsId)
	}

	go func() {
		wg.Wait()
		close(resultChan)
	}()

	// 聚合所有数据源的结果
	for result := range resultChan {
		curFingerprints = append(curFingerprints, result.Fingerprints...)
		for fp, val := range result.CurrentValues {
			currentValues[fp] = val
		}
	}

	return EvalResult{
		Fingerprints:  curFingerprints,
		CurrentValues: currentValues,
	}
}

// processSingleDatasource 处理单个数据源
func (t *AlertRule) processSingleDatasource(dsId string, rule models.AlertRule) EvalResult {
	instance, err := t.ctx.DB.Datasource().GetInstance(dsId)
	if err != nil {
		logc.Errorf(t.ctx.Ctx, fmt.Sprintf("Failed to get datasource instance %s: %v", dsId, err))
		return EvalResult{}
	}

	// 检查数据源健康状态
	if ok, _ := provider.CheckDatasourceHealth(instance); !ok {
		logc.Errorf(t.ctx.Ctx, "Datasource %s is unhealthy", dsId)
		return EvalResult{}
	}

	// 检查数据源是否启用
	if !*instance.Enabled {
		logc.Errorf(t.ctx.Ctx, "Datasource %s is disabled", dsId)
		return EvalResult{}
	}

	// 调用处理器
	handler, exists := datasourceHandlers[rule.DatasourceType]
	if !exists {
		logc.Errorf(t.ctx.Ctx, "Unsupported datasource type: %s", rule.DatasourceType)
		return EvalResult{}
	}

	return handler(t.ctx, dsId, instance.Type, rule)
}

// getEvalTimeDuration 获取评估时间间隔
func (t *AlertRule) getEvalTimeDuration(evalTimeType string, evalInterval int64) time.Duration {
	switch evalTimeType {
	case TimeTypeMillisecond:
		return time.Duration(evalInterval) * time.Millisecond
	default:
		return time.Duration(evalInterval) * time.Second
	}
}

func (t *AlertRule) Recover(tenantId, ruleId string, eventCacheKey models.AlertEventCacheKey, faultCenterInfoKey models.FaultCenterInfoCacheKey, curFingerprints []string, currentValues map[string]float64, annotationTemplate string) {
	// 过滤空指纹
	var filteredCurFingerprints []string
	for _, fp := range curFingerprints {
		if fp != "" {
			filteredCurFingerprints = append(filteredCurFingerprints, fp)
		}
	}
	curFingerprints = filteredCurFingerprints

	// 校验 key 非空
	if eventCacheKey == "" || faultCenterInfoKey == "" {
		logc.Errorf(t.ctx.Ctx, "AlertRule.Recover: eventCacheKey or faultCenterInfoKey is empty")
		return
	}

	// 获取所有的故障中心告警事件
	events, err := t.ctx.Redis.Alert().GetAllEvents(eventCacheKey)
	if err != nil {
		logc.Errorf(t.ctx.Ctx, "AlertRule.Recover: Failed to get all events: %v", err)
		return
	}

	// 存储当前规则下所有活动的指纹
	var activeRuleFingerprints []string

	// 筛选当前规则相关的指纹，并处理预告警状态
	for fingerprint, event := range events {
		if fingerprint == "" {
			continue
		}

		if !strings.Contains(event.RuleId, ruleId) {
			continue
		}

		// 移除状态为预告警且当前告警列表中不存在的事件
		// 注意：StateAlerting、StatePendingRecovery、StateRecovered 状态的事件不应该被移除
		// 它们需要进入恢复流程或保持当前状态
		if event.Status == models.StatePreAlert && !slices.Contains(curFingerprints, fingerprint) {
			t.ctx.Redis.Alert().RemoveAlertEvent(event.TenantId, event.FaultCenterId, event.Fingerprint)
			continue
		}

		// 将所有非预告警状态的事件（或预告警但在当前活动列表中的事件）添加到活动列表
		// 这样 StateAlerting 状态的事件可以进入恢复流程
		activeRuleFingerprints = append(activeRuleFingerprints, fingerprint)
	}

	/*
		从待恢复状态转换成告警状态（即在 Redis 中存在待恢复 且在 curFingerprints 存在告警的事件）
	*/

	// 获取当前待恢复的告警指纹列表
	pendingFingerprints := t.ctx.Redis.PendingRecover().List(tenantId, ruleId)
	if len(pendingFingerprints) != 0 {
		for _, fingerprint := range curFingerprints {
			if _, exists := pendingFingerprints[fingerprint]; !exists {
				continue
			}
			event, ok := events[fingerprint]
			if !ok {
				continue
			}

			newEvent := event
			// 转换成告警状态
			err := newEvent.TransitionStatus(models.StateAlerting)
			if err != nil {
				logc.Errorf(t.ctx.Ctx, "Failed to transition to「alerting」state for fingerprint %s: %v", fingerprint, err)
				continue
			}
			t.ctx.Redis.Alert().PushAlertEvent(newEvent)
			t.ctx.Redis.PendingRecover().Delete(tenantId, ruleId, fingerprint)
		}
	}

	/*
		从待恢复状态转换成已恢复状态
	*/

	// 计算需要恢复的指纹列表 (即在 Redis 中存在但在当前活动列表中不存在的指纹)
	recoverFingerprints := tools.GetSliceDifference(activeRuleFingerprints, curFingerprints)
	curTime := time.Now().Unix()
	recoverWaitTime := t.getRecoverWaitTime(faultCenterInfoKey)
	for _, fingerprint := range recoverFingerprints {
		event, ok := events[fingerprint]
		if !ok {
			continue
		}

		newEvent := event
		// 获取待恢复状态的时间戳
		wTime, err := t.ctx.Redis.PendingRecover().Get(tenantId, ruleId, fingerprint)
		if err == redis.Nil {
			// 检查当前状态：只有告警中或预告警状态的事件才能转换到待恢复状态
			// 已恢复状态的事件不应该再进入待恢复流程（根据状态转换规则，StateRecovered 只允许转换到 StatePreAlert）
			if newEvent.Status == models.StateRecovered {
				// 已恢复状态的事件从活动列表中消失，说明已经完成恢复流程，跳过处理
				logc.Debugf(t.ctx.Ctx, "Skipping transition to pending_recovery for fingerprint %s: event already in recovered state", fingerprint)
				continue
			}
			// 转换状态, 标记为待恢复（仅适用于 StateAlerting 或 StatePreAlert 状态）
			if err := newEvent.TransitionStatus(models.StatePendingRecovery); err != nil {
				logc.Errorf(t.ctx.Ctx, "Failed to transition to「pending_recovery」state for fingerprint %s: %v", fingerprint, err)
				continue
			}
			// 记录当前时间
			t.ctx.Redis.PendingRecover().Set(tenantId, ruleId, fingerprint, curTime)
			t.ctx.Redis.Alert().PushAlertEvent(newEvent)
			continue
		} else if err != nil {
			logc.Errorf(t.ctx.Ctx, "Failed to get「pending_recovery」time for fingerprint %s: %v", fingerprint, err)
			continue
		}

		// 判断是否在等待时间内
		recoverThreshold := wTime + recoverWaitTime
		// 当前时间超过预期等待时间，并且状态是 PendingRecovery 时才执行恢复逻辑
		if curTime >= recoverThreshold && newEvent.Status == models.StatePendingRecovery {
			// 已恢复状态
			if err := newEvent.TransitionStatus(models.StateRecovered); err != nil {
				logc.Errorf(t.ctx.Ctx, "Failed to transition to recovered state for fingerprint %s: %v", fingerprint, err)
				continue
			}
			// 确保恢复标志和发送时间正确设置（TransitionStatus 会自动设置，但为了保险再确认一次）
			// handleStateTransition 已经设置了 IsRecovered = true 和 LastSendTime = 0
			// 但为了确保恢复通知能发送，再次确认
			if !newEvent.IsRecovered {
				newEvent.IsRecovered = true
			}
			if newEvent.LastSendTime != 0 {
				newEvent.LastSendTime = 0
			}

			// 更新恢复时的指标值（如磁盘使用率从100%恢复到70%时，显示当前的70%而非告警时的100%）
			if currentValue, exists := currentValues[fingerprint]; exists {
				// 更新 Labels 中的 value 为当前实际值
				if newEvent.Labels != nil {
					newEvent.Labels["value"] = int(currentValue + 0.5) // 四舍五入取整
				}
				// 重新生成 Annotations，使用更新后的值
				if annotationTemplate != "" {
					newEvent.Annotations = tools.ParserVariables(annotationTemplate, tools.ConvertStructToMap(newEvent))
				}
				logc.Infof(t.ctx.Ctx, "[告警恢复] 更新指标值: fingerprint=%s, 恢复时值=%.2f",
					fingerprint, currentValue)
			}

			// 记录恢复事件推送日志
			logc.Infof(t.ctx.Ctx, "[普通告警恢复] 推送恢复事件: ruleId=%s, fingerprint=%s, ruleName=%s",
				newEvent.RuleId, newEvent.Fingerprint, newEvent.RuleName)
			// 更新告警事件
			t.ctx.Redis.Alert().PushAlertEvent(newEvent)
			// 恢复后继续处理下一个事件
			t.ctx.Redis.PendingRecover().Delete(tenantId, ruleId, fingerprint)
			continue
		}
	}
}

// getRecoverWaitTime 获取恢复等待时间
func (t *AlertRule) getRecoverWaitTime(faultCenterInfoKey models.FaultCenterInfoCacheKey) int64 {
	faultCenter := t.ctx.Redis.FaultCenter().GetFaultCenterInfo(faultCenterInfoKey)
	if faultCenter.RecoverWaitTime == 0 {
		return DefaultRecoverWaitTime
	}
	return faultCenter.RecoverWaitTime
}

// RestartAllEvals 重启所有评估器
func (t *AlertRule) RestartAllEvals() {
	ruleList, err := t.getRuleList()
	if err != nil {
		logc.Error(t.ctx.Ctx, fmt.Sprintf("Failed to get rule list: %v", err))
		return
	}

	count := len(ruleList)
	if count == 0 {
		return
	}

	logc.Info(t.ctx.Ctx, fmt.Sprintf("获取到 %d 个状态为启用的规则", count))

	// 使用工作池限制并发数量
	const maxWorkers = 10
	wg := sync.WaitGroup{}
	semaphore := make(chan struct{}, maxWorkers)

	wg.Add(count)
	for _, rule := range ruleList {
		rule := rule
		go func() {
			semaphore <- struct{}{}
			defer func() {
				wg.Done()
				<-semaphore
			}()

			t.Submit(rule)
		}()
	}

	wg.Wait()
	close(semaphore)
	logc.Info(t.ctx.Ctx, "所有规则评估器启动成功！")
}

// isRuleEnabled 检查规则是否启用
func (t *AlertRule) isRuleEnabled(ruleId string) bool {
	// 直接检查数据库或缓存中的当前启用状态
	e := t.ctx.DB.Rule().GetRuleObject(ruleId).Enabled
	if e == nil {
		return false
	}

	return *e
}

// getRuleList 获取规则列表
func (t *AlertRule) getRuleList() ([]models.AlertRule, error) {
	var ruleList []models.AlertRule
	if err := t.ctx.DB.DB().Where("enabled = ?", "1").Find(&ruleList).Error; err != nil {
		return nil, fmt.Errorf("获取 Rule List 失败: %w", err)
	}
	return ruleList, nil
}

// StopAllEvals 停止所有评估器
func (t *AlertRule) StopAllEvals() {
	t.ctx.Mux.Lock()
	defer t.ctx.Mux.Unlock()

	count := len(t.ctx.ContextMap)
	if count == 0 {
		return
	}

	logc.Infof(t.ctx.Ctx, "停止 %d 个规则评估器...", count)

	// 取消所有评估任务
	for ruleId, cancel := range t.ctx.ContextMap {
		cancel()
		delete(t.ctx.ContextMap, ruleId)
	}

	logc.Infof(t.ctx.Ctx, "所有规则评估器已停止")
}
