package services

import (
	"alertHub/internal/ctx"
	"alertHub/internal/models"
	"alertHub/internal/types"
	"time"

	"github.com/zeromicro/go-zero/core/logc"
)

// dashboardStatisticsService 首页统计数据服务
type dashboardStatisticsService struct {
	ctx *ctx.Context
}

// InterDashboardStatisticsService 首页统计服务接口
type InterDashboardStatisticsService interface {
	GetDashboardStatistics(req interface{}) (interface{}, interface{})
}

// newDashboardStatisticsService 创建新的首页统计服务实例
func newDashboardStatisticsService(ctx *ctx.Context) InterDashboardStatisticsService {
	return &dashboardStatisticsService{
		ctx: ctx,
	}
}

// GetDashboardStatistics 获取首页统计数据
// 包含今日主告警、新增告警数量以及过去7天的MTTA、MTTR等关键指标
func (ds dashboardStatisticsService) GetDashboardStatistics(req interface{}) (interface{}, interface{}) {
	r := req.(*types.RequestDashboardStatistics)

	// 计算时间范围 - 今日、昨日、过去7天、前7天
	now := time.Now()
	todayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	yesterdayStart := todayStart.Add(-24 * time.Hour)
	weekAgo := now.Add(-7 * 24 * time.Hour)
	twoWeeksAgo := now.Add(-14 * 24 * time.Hour)

	stats := &types.ResponseDashboardStatistics{}

	// 获取今日与昨日告警统计，用于计算环比
	todayStats := ds.getTodayAlertStats(r.TenantId, r.FaultCenterId, todayStart.Unix(), now.Unix())
	yesterdayStats := ds.getTodayAlertStats(r.TenantId, r.FaultCenterId, yesterdayStart.Unix(), todayStart.Unix())

	stats.TodayMainAlerts = todayStats.MainAlerts
	stats.TodayNewAlerts = todayStats.NewAlerts

	// 获取过去7天与前7天统计数据，用于计算环比
	// 注意：MTTA/MTTR 使用全局数据（所有故障中心），其他指标使用指定故障中心数据
	weekStats := ds.getWeekAlertStats(r.TenantId, r.FaultCenterId, weekAgo.Unix(), now.Unix())
	prevWeekStats := ds.getWeekAlertStats(r.TenantId, r.FaultCenterId, twoWeeksAgo.Unix(), weekAgo.Unix())
	
	// 获取全局MTTA/MTTR统计（所有故障中心）
	globalWeekStats := ds.getGlobalWeekAlertStats(r.TenantId, weekAgo.Unix(), now.Unix())
	globalPrevWeekStats := ds.getGlobalWeekAlertStats(r.TenantId, twoWeeksAgo.Unix(), weekAgo.Unix())

	stats.Past7DaysAllEvents = weekStats.AllEvents
	stats.Past7DaysMainAlerts = weekStats.MainAlerts
	stats.Past7DaysMTTA = globalWeekStats.MTTA      // 使用全局MTTA
	stats.Past7DaysMTTR = globalWeekStats.MTTR      // 使用全局MTTR

	// 计算各项指标的环比增长率
	stats.TodayMainAlertsCompareRatio = ds.calculateCompareRatio(float64(todayStats.MainAlerts), float64(yesterdayStats.MainAlerts))
	stats.TodayNewAlertsCompareRatio = ds.calculateCompareRatio(float64(todayStats.NewAlerts), float64(yesterdayStats.NewAlerts))
	stats.Past7DaysAllEventsCompareRatio = ds.calculateCompareRatio(float64(weekStats.AllEvents), float64(prevWeekStats.AllEvents))
	stats.Past7DaysMainAlertsCompareRatio = ds.calculateCompareRatio(float64(weekStats.MainAlerts), float64(prevWeekStats.MainAlerts))
	stats.Past7DaysMTTACompareRatio = ds.calculateCompareRatio(globalWeekStats.MTTA, globalPrevWeekStats.MTTA)   // 使用全局MTTA环比
	stats.Past7DaysMTTRCompareRatio = ds.calculateCompareRatio(globalWeekStats.MTTR, globalPrevWeekStats.MTTR)   // 使用全局MTTR环比

	return stats, nil
}

// getTodayAlertStats 获取指定时间范围内的告警统计
// 统计主告警(P0/P1级别)和新增告警的数量
func (ds dashboardStatisticsService) getTodayAlertStats(tenantId, faultCenterId string, startTime, endTime int64) *todayAlertStats {
	stats := &todayAlertStats{}

	// 从Redis缓存中获取当前活跃的告警事件
	events, err := ds.ctx.Redis.Alert().GetAllEvents(models.BuildAlertEventCacheKey(tenantId, faultCenterId))
	if err != nil {
		logc.Errorf(ds.ctx.Ctx, "获取告警事件失败: %s", err.Error())
		return stats
	}

	// 遍历告警事件，统计指定时间范围内的数据
	for _, event := range events {
		// 检查事件首次触发时间是否在统计范围内
		if event.FirstTriggerTime >= startTime && event.FirstTriggerTime <= endTime {
			stats.NewAlerts++

			// 判断是否为主告警（P0、P1为高优先级告警）
			if event.Severity == "P0" || event.Severity == "P1" {
				stats.MainAlerts++
			}
		}
	}

	return stats
}

// getWeekAlertStats 获取周统计数据，包含MTTA和MTTR计算
// MTTA: 平均告警认领时间，MTTR: 平均告警恢复时间
func (ds dashboardStatisticsService) getWeekAlertStats(tenantId, faultCenterId string, startTime, endTime int64) *weekAlertStats {
	stats := &weekAlertStats{}

	// 用于计算平均值的累计变量
	var totalAcknowledgeTime int64 // 总认领时间
	var totalRecoverTime int64     // 总恢复时间
	var acknowledgedCount int64    // 已认领告警数量
	var recoveredCount int64       // 已恢复告警数量

	// 从Redis获取当前告警事件进行统计
	currentEvents, err := ds.ctx.Redis.Alert().GetAllEvents(models.BuildAlertEventCacheKey(tenantId, faultCenterId))
	if err != nil {
		logc.Errorf(ds.ctx.Ctx, "获取告警事件失败: %s", err.Error())
		return stats
	}
	
	for _, event := range currentEvents {
		// 检查事件是否在统计时间范围内
		if event.FirstTriggerTime >= startTime && event.FirstTriggerTime <= endTime {
			stats.AllEvents++
			
			// 统计主告警数量
			if event.Severity == "P0" || event.Severity == "P1" {
				stats.MainAlerts++
			}
			
			// 计算MTTA（平均认领时间）
			// 只统计已认领且认领时间有效的告警
			if event.ConfirmState.IsOk && event.ConfirmState.ConfirmActionTime > 0 {
				acknowledgeTime := event.ConfirmState.ConfirmActionTime - event.FirstTriggerTime
				if acknowledgeTime > 0 {
					totalAcknowledgeTime += acknowledgeTime
					acknowledgedCount++
				}
			}
			
			// 计算MTTR（平均恢复时间）
			// 只统计已恢复且恢复时间有效的告警
			if event.Status == models.StateRecovered && event.RecoverTime > 0 {
				recoverTime := event.RecoverTime - event.FirstTriggerTime
				if recoverTime > 0 {
					totalRecoverTime += recoverTime
					recoveredCount++
				}
			}
		}
	}
	
	// 从历史数据库获取已归档的告警事件
	// 这些事件可能已经从Redis缓存中清除，但包含完整的MTTA/MTTR数据
	historyStats := ds.getHistoryAlertStats(tenantId, faultCenterId, startTime, endTime)
	
	// 合并Redis和历史数据库的统计结果
	stats.AllEvents += historyStats.AllEvents
	stats.MainAlerts += historyStats.MainAlerts
	
	// 重新计算MTTA/MTTR，合并Redis和历史数据
	if historyStats.AcknowledgedCount > 0 {
		totalAcknowledgeTime += historyStats.TotalAcknowledgeTime
		acknowledgedCount += historyStats.AcknowledgedCount
	}
	
	if historyStats.RecoveredCount > 0 {
		totalRecoverTime += historyStats.TotalRecoverTime
		recoveredCount += historyStats.RecoveredCount
	}

	// 计算平均认领时间（转换为分钟）
	if acknowledgedCount > 0 {
		stats.MTTA = float64(totalAcknowledgeTime) / float64(acknowledgedCount) / 60
	}

	// 计算平均恢复时间（转换为分钟）
	if recoveredCount > 0 {
		stats.MTTR = float64(totalRecoverTime) / float64(recoveredCount) / 60
	}

	return stats
}

// calculateCompareRatio 计算环比增长率
// 返回百分比形式的增长率，正值表示增长，负值表示下降
func (ds dashboardStatisticsService) calculateCompareRatio(current, previous float64) float64 {
	// 处理分母为0的情况
	if previous == 0 {
		if current == 0 {
			return 0 // 都为0时，增长率为0
		}
		return 100 // 之前为0，现在有值，设置为100%增长
	}

	// 标准环比计算公式：(当前值 - 上期值) / 上期值 * 100%
	return ((current - previous) / previous) * 100
}

// todayAlertStats 今日告警统计结构
type todayAlertStats struct {
	MainAlerts int64 // 主告警数量（P0、P1级别）
	NewAlerts  int64 // 新增告警总数
}

// weekAlertStats 周告警统计结构
type weekAlertStats struct {
	AllEvents  int64   // 所有告警事件总数
	MainAlerts int64   // 主告警数量（P0、P1级别）
	MTTA       float64 // 平均认领时间（分钟）
	MTTR       float64 // 平均恢复时间（分钟）
}

// historyAlertStats 历史告警统计结构 - 用于合并计算
type historyAlertStats struct {
	AllEvents            int64 // 所有告警事件总数
	MainAlerts           int64 // 主告警数量（P0、P1级别）
	TotalAcknowledgeTime int64 // 总认领时间（秒）
	AcknowledgedCount    int64 // 已认领告警数量
	TotalRecoverTime     int64 // 总恢复时间（秒）
	RecoveredCount       int64 // 已恢复告警数量
}

// getHistoryAlertStats 从历史数据库获取告警统计数据
// 补充Redis缓存中可能缺失的已恢复告警统计信息
func (ds dashboardStatisticsService) getHistoryAlertStats(tenantId, faultCenterId string, startTime, endTime int64) *historyAlertStats {
	stats := &historyAlertStats{}
	
	// 构建历史事件查询条件
	query := types.RequestAlertHisEventQuery{
		TenantId:      tenantId,
		FaultCenterId: faultCenterId,
		StartAt:       startTime,
		EndAt:         endTime,
		Page: models.Page{
			Index: 1,
			Size:  10000, // 设置较大的分页大小，确保获取所有数据
		},
	}
	
	// 从数据库查询历史告警事件
	historyEvents, err := ds.ctx.DB.Event().GetHistoryEvent(query)
	if err != nil {
		logc.Errorf(ds.ctx.Ctx, "查询历史告警事件失败: %s", err.Error())
		return stats
	}
	
	// 遍历历史事件进行统计
	for _, event := range historyEvents.List {
		// 检查事件是否在统计时间范围内（双重保险）
		if event.FirstTriggerTime >= startTime && event.FirstTriggerTime <= endTime {
			stats.AllEvents++
			
			// 统计主告警数量
			if event.Severity == "P0" || event.Severity == "P1" {
				stats.MainAlerts++
			}
			
			// 计算MTTA（平均认领时间）
			if event.ConfirmState.IsOk && event.ConfirmState.ConfirmActionTime > 0 {
				acknowledgeTime := event.ConfirmState.ConfirmActionTime - event.FirstTriggerTime
				if acknowledgeTime > 0 {
					stats.TotalAcknowledgeTime += acknowledgeTime
					stats.AcknowledgedCount++
				}
			}
			
			// 计算MTTR（平均恢复时间）
			if event.RecoverTime > 0 {
				recoverTime := event.RecoverTime - event.FirstTriggerTime
				if recoverTime > 0 {
					stats.TotalRecoverTime += recoverTime
					stats.RecoveredCount++
				}
			}
		}
	}
	
	return stats
}

// getGlobalWeekAlertStats 获取全局周统计数据（所有故障中心的MTTA和MTTR）
// 用于计算跨故障中心的平均认领时间和恢复时间
func (ds dashboardStatisticsService) getGlobalWeekAlertStats(tenantId string, startTime, endTime int64) *weekAlertStats {
	stats := &weekAlertStats{}
	
	// 用于计算平均值的累计变量
	var totalAcknowledgeTime int64 // 总认领时间
	var totalRecoverTime int64     // 总恢复时间
	var acknowledgedCount int64    // 已认领告警数量
	var recoveredCount int64       // 已恢复告警数量

	// 获取租户下所有故障中心
	faultCenters, err := ds.ctx.DB.FaultCenter().List(tenantId, "")
	if err != nil {
		logc.Errorf(ds.ctx.Ctx, "获取故障中心列表失败: %s", err.Error())
		return stats
	}

	// 遍历所有故障中心，统计MTTA/MTTR
	for _, faultCenter := range faultCenters {
		// 从Redis获取当前故障中心的告警事件
		currentEvents, err := ds.ctx.Redis.Alert().GetAllEvents(models.BuildAlertEventCacheKey(tenantId, faultCenter.ID))
		if err != nil {
			continue // 忽略错误，继续处理下一个故障中心
		}
		
		for _, event := range currentEvents {
			// 检查事件是否在统计时间范围内
			if event.FirstTriggerTime >= startTime && event.FirstTriggerTime <= endTime {
				// 计算MTTA（平均认领时间）
				if event.ConfirmState.IsOk && event.ConfirmState.ConfirmActionTime > 0 {
					acknowledgeTime := event.ConfirmState.ConfirmActionTime - event.FirstTriggerTime
					if acknowledgeTime > 0 {
						totalAcknowledgeTime += acknowledgeTime
						acknowledgedCount++
					}
				}
				
				// 计算MTTR（平均恢复时间）
				if event.Status == models.StateRecovered && event.RecoverTime > 0 {
					recoverTime := event.RecoverTime - event.FirstTriggerTime
					if recoverTime > 0 {
						totalRecoverTime += recoverTime
						recoveredCount++
					}
				}
			}
		}
		
		// 从历史数据库获取该故障中心的已归档告警事件
		historyStats := ds.getHistoryAlertStats(tenantId, faultCenter.ID, startTime, endTime)
		
		// 合并历史数据的统计结果
		if historyStats.AcknowledgedCount > 0 {
			totalAcknowledgeTime += historyStats.TotalAcknowledgeTime
			acknowledgedCount += historyStats.AcknowledgedCount
		}
		
		if historyStats.RecoveredCount > 0 {
			totalRecoverTime += historyStats.TotalRecoverTime
			recoveredCount += historyStats.RecoveredCount
		}
	}

	// 计算全局平均认领时间（转换为分钟）
	if acknowledgedCount > 0 {
		stats.MTTA = float64(totalAcknowledgeTime) / float64(acknowledgedCount) / 60
	}

	// 计算全局平均恢复时间（转换为分钟）
	if recoveredCount > 0 {
		stats.MTTR = float64(totalRecoverTime) / float64(recoveredCount) / 60
	}

	return stats
}
