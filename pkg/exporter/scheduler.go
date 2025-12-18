package exporter

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"
	ctx2 "alertHub/internal/ctx"
	"alertHub/internal/models"

	"github.com/robfig/cron/v3"
	"github.com/zeromicro/go-zero/core/logc"
)

// globalScheduler 全局 Scheduler 实例
// 由 initialization 包初始化后设置
var (
	globalScheduler *Scheduler
	globalMu        sync.RWMutex
)

// Scheduler Exporter 巡检调度器
// 负责三类定时任务:
// 1. 定时巡检任务 (根据 InspectionTimes 配置)
// 2. 定时报告推送任务 (根据 CronExpression 配置)
// 3. 历史数据清理任务 (每天凌晨执行)
type Scheduler struct {
	ctx            *ctx2.Context
	cron           *cron.Cron
	mu             sync.RWMutex
	inspectionJobs map[string]cron.EntryID   // 巡检任务ID映射 (key: tenantId)
	reportJobs     map[string][]cron.EntryID // 报告推送任务ID映射 (key: tenantId, value: 多个任务ID)
	cleanupJobID   cron.EntryID              // 清理任务ID
	cancelFunc     context.CancelFunc        // 用于优雅停止
}

// NewScheduler 创建调度器实例
func NewScheduler(ctx *ctx2.Context) *Scheduler {
	// 创建支持秒级精度的 cron (可选)
	// 这里使用标准的分钟级精度: cron.New()
	c := cron.New(cron.WithSeconds()) // 支持秒级,格式: "秒 分 时 日 月 周"

	return &Scheduler{
		ctx:            ctx,
		cron:           c,
		inspectionJobs: make(map[string]cron.EntryID),
		reportJobs:     make(map[string][]cron.EntryID),
	}
}

// Start 启动调度器
// 1. 加载所有租户的巡检配置并注册定时巡检任务
// 2. 加载所有租户的推送配置并注册定时推送任务
// 3. 注册历史数据清理任务 (每天凌晨 2:00 执行)
func (s *Scheduler) Start() error {
	logc.Info(s.ctx.Ctx, "[ExporterScheduler] 正在启动 Exporter 巡检调度器...")

	// 1. 注册历史数据清理任务 (每天凌晨 2:00)
	if err := s.registerCleanupJob(); err != nil {
		logc.Errorf(s.ctx.Ctx, "[ExporterScheduler] 注册清理任务失败: %v", err)
		return err
	}

	// 2. 加载并注册所有租户的巡检任务
	if err := s.loadAndRegisterInspectionJobs(); err != nil {
		logc.Errorf(s.ctx.Ctx, "[ExporterScheduler] 加载巡检任务失败: %v", err)
		return err
	}

	// 3. 加载并注册所有租户的报告推送任务
	if err := s.loadAndRegisterReportJobs(); err != nil {
		logc.Errorf(s.ctx.Ctx, "[ExporterScheduler] 加载推送任务失败: %v", err)
		return err
	}

	// 4. 启动 cron 调度器
	s.cron.Start()

	logc.Info(s.ctx.Ctx, "[ExporterScheduler] Exporter 巡检调度器启动成功")
	logc.Infof(s.ctx.Ctx, "[ExporterScheduler] 已注册 %d 个巡检任务, %d 个租户的推送任务",
		len(s.inspectionJobs), len(s.reportJobs))
	logc.Info(s.ctx.Ctx, "[ExporterScheduler] 提示: 可使用'立即巡检'按钮手动触发数据采集")

	return nil
}

// Stop 停止调度器
func (s *Scheduler) Stop() {
	logc.Info(s.ctx.Ctx, "[ExporterScheduler] 正在停止 Exporter 巡检调度器...")
	stopCtx := s.cron.Stop()
	<-stopCtx.Done() // 等待所有任务完成
	logc.Info(s.ctx.Ctx, "[ExporterScheduler] Exporter 巡检调度器已停止")
}

// registerCleanupJob 注册历史数据清理任务
// 每天凌晨 2:00 执行一次
func (s *Scheduler) registerCleanupJob() error {
	// Cron 表达式: "秒 分 时 日 月 周"
	// "0 0 2 * * *" = 每天 02:00:00 执行
	cronExpr := "0 0 2 * * *"

	entryID, err := s.cron.AddFunc(cronExpr, func() {
		logc.Info(s.ctx.Ctx, "[ExporterScheduler] 开始执行历史数据清理任务...")
		if err := s.cleanupHistoryData(); err != nil {
			logc.Errorf(s.ctx.Ctx, "[ExporterScheduler] 历史数据清理失败: %v", err)
		} else {
			logc.Info(s.ctx.Ctx, "[ExporterScheduler] 历史数据清理完成")
		}
	})

	if err != nil {
		return fmt.Errorf("注册清理任务失败: %w", err)
	}

	s.cleanupJobID = entryID
	logc.Infof(s.ctx.Ctx, "[ExporterScheduler] 已注册清理任务 (每天 02:00 执行), EntryID: %d", entryID)
	return nil
}

// loadAndRegisterInspectionJobs 加载并注册所有启用的巡检任务
func (s *Scheduler) loadAndRegisterInspectionJobs() error {
	// 从数据库查询所有租户
	tenants, err := s.ctx.DB.Tenant().GetAll()
	if err != nil {
		logc.Errorf(s.ctx.Ctx, "[ExporterScheduler] 查询租户列表失败: %v", err)
		return err
	}

	// 如果没有租户，使用默认租户
	if len(tenants) == 0 {
		logc.Infof(s.ctx.Ctx, "[ExporterScheduler] 未找到任何租户，使用默认租户 'default'")
		tenants = append(tenants, models.Tenant{ID: "default"})
	}

	for _, tenant := range tenants {
		tenantId := tenant.ID
		config, err := s.ctx.DB.ExporterMonitor().GetConfig(tenantId)
		if err != nil {
			logc.Errorf(s.ctx.Ctx, "[ExporterScheduler] 获取租户 %s 的巡检配置失败: %v", tenantId, err)
			continue
		}

		// 检查是否启用
		if !config.GetEnabled() {
			logc.Infof(s.ctx.Ctx, "[ExporterScheduler] 租户 %s 的巡检功能未启用,跳过", tenantId)
			continue
		}

		// 检查是否配置了巡检时间
		if len(config.InspectionTimes) == 0 {
			logc.Infof(s.ctx.Ctx, "[ExporterScheduler] 租户 %s 未配置巡检时间,跳过", tenantId)
			continue
		}

		// 为每个巡检时间注册定时任务
		if err := s.registerInspectionJob(tenantId, config.InspectionTimes); err != nil {
			logc.Errorf(s.ctx.Ctx, "[ExporterScheduler] 注册租户 %s 的巡检任务失败: %v", tenantId, err)
			continue
		}
	}

	return nil
}

// registerInspectionJob 为指定租户注册巡检任务
// inspectionTimes: ["09:00", "21:00"] 格式
func (s *Scheduler) registerInspectionJob(tenantId string, inspectionTimes []string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// 先移除旧任务
	if oldEntryID, exists := s.inspectionJobs[tenantId]; exists {
		s.cron.Remove(oldEntryID)
		logc.Infof(s.ctx.Ctx, "[ExporterScheduler] 已移除租户 %s 的旧巡检任务", tenantId)
	}

	// 将多个巡检时间合并为一个 cron 表达式
	// 例如: ["09:00", "21:00"] -> "0 0 9,21 * * *"
	cronExpr, err := s.convertInspectionTimesToCron(inspectionTimes)
	if err != nil {
		return fmt.Errorf("转换巡检时间失败: %w", err)
	}

	// 注册新任务
	entryID, err := s.cron.AddFunc(cronExpr, func() {
		logc.Infof(s.ctx.Ctx, "[ExporterScheduler] 触发租户 %s 的定时巡检任务...", tenantId)
		inspector := NewInspector(s.ctx)
		// 定时任务按配置的时间执行，不强制
		if err := inspector.InspectAll(false); err != nil {
			logc.Errorf(s.ctx.Ctx, "[ExporterScheduler] 租户 %s 巡检失败: %v", tenantId, err)
		} else {
			logc.Infof(s.ctx.Ctx, "[ExporterScheduler] 租户 %s 巡检完成", tenantId)
		}
	})

	if err != nil {
		return fmt.Errorf("注册巡检任务失败: %w", err)
	}

	s.inspectionJobs[tenantId] = entryID
	logc.Infof(s.ctx.Ctx, "[ExporterScheduler] 已注册租户 %s 的巡检任务, 巡检时间: %v, Cron: %s, EntryID: %d",
		tenantId, inspectionTimes, cronExpr, entryID)

	return nil
}

// convertInspectionTimesToCron 将巡检时间列表转换为 Cron 表达式
// 输入: ["09:00", "21:00"]
// 输出: "0 0 9,21 * * *" (每天 9:00 和 21:00 执行)
func (s *Scheduler) convertInspectionTimesToCron(times []string) (string, error) {
	hours := make([]string, 0, len(times))
	minutes := "0" // 假设所有时间都是整点,分钟统一为 0

	for _, t := range times {
		parts := strings.Split(t, ":")
		if len(parts) != 2 {
			return "", fmt.Errorf("无效的时间格式: %s, 应为 HH:MM", t)
		}

		hour := parts[0]
		minute := parts[1]

		// 如果分钟不是 0,需要特殊处理
		if minute != "00" && minute != "0" {
			// 当前简化实现: 暂不支持非整点时间
			return "", fmt.Errorf("当前仅支持整点巡检时间 (如 09:00), 不支持: %s", t)
		}

		hours = append(hours, hour)
	}

	// 合并小时: "9,21"
	hoursPart := strings.Join(hours, ",")

	// 构造 Cron 表达式: "秒 分 时 日 月 周"
	cronExpr := fmt.Sprintf("0 %s %s * * *", minutes, hoursPart)

	return cronExpr, nil
}

// loadAndRegisterReportJobs 加载并注册所有启用的报告推送任务
func (s *Scheduler) loadAndRegisterReportJobs() error {
	// 从数据库查询所有租户
	tenants, err := s.ctx.DB.Tenant().GetAll()
	if err != nil {
		logc.Errorf(s.ctx.Ctx, "[ExporterScheduler] 查询租户列表失败: %v", err)
		return err
	}

	// 如果没有租户，使用默认租户
	if len(tenants) == 0 {
		logc.Infof(s.ctx.Ctx, "[ExporterScheduler] 未找到任何租户，使用默认租户 'default'")
		tenants = append(tenants, models.Tenant{ID: "default"})
	}

	for _, tenant := range tenants {
		tenantId := tenant.ID
		schedule, err := s.ctx.DB.ExporterMonitor().GetSchedule(tenantId)
		if err != nil {
			logc.Errorf(s.ctx.Ctx, "[ExporterScheduler] 获取租户 %s 的推送配置失败: %v", tenantId, err)
			continue
		}

		// 检查是否启用
		if !schedule.GetEnabled() {
			logc.Infof(s.ctx.Ctx, "[ExporterScheduler] 租户 %s 的推送功能未启用,跳过", tenantId)
			continue
		}

		// 检查是否配置了 Cron 表达式
		if len(schedule.CronExpression) == 0 {
			logc.Infof(s.ctx.Ctx, "[ExporterScheduler] 租户 %s 未配置推送时间,跳过", tenantId)
			continue
		}

		// 注册推送任务
		if err := s.registerReportJobs(tenantId, schedule); err != nil {
			logc.Errorf(s.ctx.Ctx, "[ExporterScheduler] 注册租户 %s 的推送任务失败: %v", tenantId, err)
			continue
		}
	}

	return nil
}

// registerReportJobs 为指定租户注册报告推送任务
func (s *Scheduler) registerReportJobs(tenantId string, schedule models.ExporterReportSchedule) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// 先移除旧任务
	if oldEntryIDs, exists := s.reportJobs[tenantId]; exists {
		for _, entryID := range oldEntryIDs {
			s.cron.Remove(entryID)
		}
		logc.Infof(s.ctx.Ctx, "[ExporterScheduler] 已移除租户 %s 的旧推送任务 (%d 个)", tenantId, len(oldEntryIDs))
	}

	// 为每个 Cron 表达式注册任务
	entryIDs := make([]cron.EntryID, 0, len(schedule.CronExpression))
	for _, cronExpr := range schedule.CronExpression {
		// 适配 cron 表达式格式
		// 前端传入的是标准 5 段式: "30 9 * * *" (分 时 日 月 周)
		// 需要转换为 6 段式: "0 30 9 * * *" (秒 分 时 日 月 周)
		fullCronExpr := "0 " + cronExpr

		// 捕获闭包变量
		capturedTenantId := tenantId
		capturedNoticeGroups := schedule.NoticeGroups
		capturedReportFormat := schedule.ReportFormat

		entryID, err := s.cron.AddFunc(fullCronExpr, func() {
			logc.Infof(s.ctx.Ctx, "[ExporterScheduler] 触发租户 %s 的定时推送任务...", capturedTenantId)

			// 1. 获取实时状态
			aggregator := NewAggregator(s.ctx)
			realtimeData, err := aggregator.GetRealtimeStatus(capturedTenantId, "", "", "", "")
			if err != nil {
				logc.Errorf(s.ctx.Ctx, "[ExporterScheduler] 获取实时状态失败: %v", err)
				return
			}

			realtimeMap, ok := realtimeData.(map[string]interface{})
			if !ok {
				logc.Errorf(s.ctx.Ctx, "[ExporterScheduler] 实时状态数据格式错误")
				return
			}

			summary, ok := realtimeMap["summary"].(models.ExporterStatusSummary)
			if !ok {
				logc.Errorf(s.ctx.Ctx, "[ExporterScheduler] 统计摘要数据格式错误")
				return
			}

			exporters, ok := realtimeMap["exporters"].([]models.ExporterStatus)
			if !ok {
				logc.Errorf(s.ctx.Ctx, "[ExporterScheduler] Exporter列表数据格式错误")
				return
			}

			// 2. 获取历史趋势 (可选)
			endTime := time.Now()
			startTime := endTime.AddDate(0, 0, -7)
			historyData, _ := aggregator.GetHistory(capturedTenantId, "", startTime, endTime)

			// 3. 生成报告内容
			reporter := NewReporter()
			content := reporter.GenerateReportContent(summary, exporters, historyData, capturedReportFormat)

			// 4. 发送到通知组
			notifier := NewNotifier(s.ctx)
			if err := notifier.SendToNoticeGroups(capturedTenantId, capturedNoticeGroups, content); err != nil {
				logc.Errorf(s.ctx.Ctx, "[ExporterScheduler] 推送报告失败: %v", err)
			} else {
				logc.Infof(s.ctx.Ctx, "[ExporterScheduler] 租户 %s 推送报告完成", capturedTenantId)
			}
		})

		if err != nil {
			logc.Errorf(s.ctx.Ctx, "[ExporterScheduler] 注册推送任务失败 (Cron: %s): %v", cronExpr, err)
			continue
		}

		entryIDs = append(entryIDs, entryID)
		logc.Infof(s.ctx.Ctx, "[ExporterScheduler] 已注册推送任务, Cron: %s, EntryID: %d", fullCronExpr, entryID)
	}

	s.reportJobs[tenantId] = entryIDs
	logc.Infof(s.ctx.Ctx, "[ExporterScheduler] 租户 %s 共注册 %d 个推送任务", tenantId, len(entryIDs))

	return nil
}

// cleanupHistoryData 清理过期的历史数据
func (s *Scheduler) cleanupHistoryData() error {
	// 从数据库查询所有租户
	tenants, err := s.ctx.DB.Tenant().GetAll()
	if err != nil {
		logc.Errorf(s.ctx.Ctx, "[ExporterScheduler] 查询租户列表失败: %v", err)
		return err
	}

	// 如果没有租户，使用默认租户
	if len(tenants) == 0 {
		logc.Infof(s.ctx.Ctx, "[ExporterScheduler] 未找到任何租户，使用默认租户 'default'")
		tenants = append(tenants, models.Tenant{ID: "default"})
	}

	for _, tenant := range tenants {
		tenantId := tenant.ID
		// 获取配置的保留天数
		config, err := s.ctx.DB.ExporterMonitor().GetConfig(tenantId)
		if err != nil {
			logc.Errorf(s.ctx.Ctx, "[ExporterScheduler] 获取租户 %s 配置失败: %v", tenantId, err)
			continue
		}

		retentionDays := config.HistoryRetention
		if retentionDays <= 0 {
			retentionDays = 90 // 默认保留 90 天
		}

		// 删除过期数据
		err = s.ctx.DB.ExporterMonitor().DeleteExpiredInspections(tenantId, retentionDays)
		if err != nil {
			logc.Errorf(s.ctx.Ctx, "[ExporterScheduler] 清理租户 %s 的历史数据失败: %v", tenantId, err)
			continue
		}

		logc.Infof(s.ctx.Ctx, "[ExporterScheduler] 租户 %s 历史数据清理完成 (保留 %d 天)",
			tenantId, retentionDays)
	}

	return nil
}

// ReloadConfig 重新加载配置并更新任务
// 当用户在前端修改配置后,需要调用此方法刷新调度器
func (s *Scheduler) ReloadConfig(tenantId string) error {
	logc.Infof(s.ctx.Ctx, "[ExporterScheduler] 正在重新加载租户 %s 的配置...", tenantId)

	// 1. 重新加载并注册巡检任务
	config, err := s.ctx.DB.ExporterMonitor().GetConfig(tenantId)
	if err == nil && config.GetEnabled() && len(config.InspectionTimes) > 0 {
		if err := s.registerInspectionJob(tenantId, config.InspectionTimes); err != nil {
			logc.Errorf(s.ctx.Ctx, "[ExporterScheduler] 重新加载巡检任务失败: %v", err)
		}
	} else {
		// 移除任务
		s.mu.Lock()
		if entryID, exists := s.inspectionJobs[tenantId]; exists {
			s.cron.Remove(entryID)
			delete(s.inspectionJobs, tenantId)
			logc.Infof(s.ctx.Ctx, "[ExporterScheduler] 已移除租户 %s 的巡检任务", tenantId)
		}
		s.mu.Unlock()
	}

	// 2. 重新加载并注册推送任务
	schedule, err := s.ctx.DB.ExporterMonitor().GetSchedule(tenantId)
	if err == nil && schedule.GetEnabled() && len(schedule.CronExpression) > 0 {
		if err := s.registerReportJobs(tenantId, schedule); err != nil {
			logc.Errorf(s.ctx.Ctx, "[ExporterScheduler] 重新加载推送任务失败: %v", err)
		}
	} else {
		// 移除任务
		s.mu.Lock()
		if entryIDs, exists := s.reportJobs[tenantId]; exists {
			for _, entryID := range entryIDs {
				s.cron.Remove(entryID)
			}
			delete(s.reportJobs, tenantId)
			logc.Infof(s.ctx.Ctx, "[ExporterScheduler] 已移除租户 %s 的推送任务", tenantId)
		}
		s.mu.Unlock()
	}

	logc.Infof(s.ctx.Ctx, "[ExporterScheduler] 租户 %s 配置重新加载完成", tenantId)
	return nil
}

// SetGlobalScheduler 设置全局 Scheduler 实例
// 由 initialization 包在启动时调用
func SetGlobalScheduler(s *Scheduler) {
	globalMu.Lock()
	defer globalMu.Unlock()
	globalScheduler = s
}

// GetGlobalScheduler 获取全局 Scheduler 实例
// 供其他包调用,返回 nil 表示未初始化
func GetGlobalScheduler() *Scheduler {
	globalMu.RLock()
	defer globalMu.RUnlock()
	return globalScheduler
}

// ReloadGlobalSchedulerConfig 重载全局 Scheduler 的配置
// 供 Service 层调用的便捷方法
func ReloadGlobalSchedulerConfig(tenantId string) error {
	globalMu.RLock()
	scheduler := globalScheduler
	globalMu.RUnlock()

	if scheduler == nil {
		return fmt.Errorf("调度器未初始化")
	}

	return scheduler.ReloadConfig(tenantId)
}
