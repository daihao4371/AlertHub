package services

import (
	"alertHub/internal/ctx"
	"alertHub/internal/models"
	"alertHub/pkg/exporter"
	"fmt"
	"time"

	"github.com/zeromicro/go-zero/core/logc"
)

type exporterMonitorService struct {
	ctx *ctx.Context
}

type InterExporterMonitorService interface {
	// 巡检相关
	InspectAll() error
	InspectDatasource(datasourceId string) error
	TriggerManualInspection(tenantId, datasourceId string) error // 手动触发巡检

	// 查询相关
	GetRealtimeStatus(tenantId, datasourceId, status, job, keyword string) (interface{}, error)
	GetHistory(tenantId, datasourceId string, startTime, endTime time.Time) (interface{}, error)

	// 配置相关
	GetConfig(tenantId string) (models.ExporterMonitorConfig, error)
	SaveConfig(config models.ExporterMonitorConfig) error
	UpdateAutoRefresh(tenantId string, autoRefresh bool) error

	GetSchedule(tenantId string) (models.ExporterReportSchedule, error)
	SaveSchedule(schedule models.ExporterReportSchedule) error

	// 调度器配置重载
	ReloadSchedulerConfig(tenantId string) error

	// 报告相关
	SendReport(tenantId string, noticeGroups []string, reportFormat string) error
}

func newInterExporterMonitorService(ctx *ctx.Context) InterExporterMonitorService {
	return &exporterMonitorService{
		ctx: ctx,
	}
}

// InspectAll 巡检所有已启用的 Prometheus 数据源
// 委托给 pkg/exporter.Inspector 处理业务逻辑
func (s *exporterMonitorService) InspectAll() error {
	inspector := exporter.NewInspector(s.ctx)
	return inspector.InspectAll(true)
}

// InspectDatasource 巡检单个 Prometheus 数据源并写入数据库
// 委托给 pkg/exporter.Inspector 处理业务逻辑
func (s *exporterMonitorService) InspectDatasource(datasourceId string) error {
	inspector := exporter.NewInspector(s.ctx)
	return inspector.InspectDatasource(datasourceId)
}

// TriggerManualInspection 手动触发巡检
// 支持全量巡检或指定数据源巡检
// - datasourceId 为空时: 巡检当前租户配置的所有数据源
// - datasourceId 非空时: 只巡检指定的数据源
func (s *exporterMonitorService) TriggerManualInspection(tenantId, datasourceId string) error {
	inspector := exporter.NewInspector(s.ctx)

	// 如果指定了数据源ID,只巡检该数据源
	if datasourceId != "" {
		// Check if datasource exists and belongs to tenant
		datasource, err := s.ctx.DB.Datasource().Get(datasourceId)
		if err != nil {
			return fmt.Errorf("failed to get datasource %s: %w", datasourceId, err)
		}

		if datasource.TenantId != tenantId {
			return fmt.Errorf("datasource %s does not belong to tenant %s", datasourceId, tenantId)
		}

		err = inspector.InspectDatasource(datasourceId)
		if err != nil {
			return fmt.Errorf("inspection failed for datasource %s: %w", datasourceId, err)
		}

		return nil
	}

	// 否则巡检所有已启用的数据源 (强制执行,忽略时间检查)
	// For manual inspection, query all available Prometheus datasources directly from database
	// instead of relying on ExporterMonitorConfig
	var availableDS []models.AlertDataSource
	err := s.ctx.DB.DB().Where("tenant_id = ? AND type = ? AND enabled = ?", tenantId, "Prometheus", true).
		Find(&availableDS).Error
	if err != nil {
		return fmt.Errorf("failed to query available datasources for tenant %s: %w", tenantId, err)
	}
	
	if len(availableDS) == 0 {
		return fmt.Errorf("no Prometheus datasources found for tenant %s", tenantId)
	}
	
	// Manually inspect each datasource
	successCount := 0
	for _, ds := range availableDS {
		err = inspector.InspectDatasource(ds.ID)
		if err != nil {
			continue // Continue with other datasources even if one fails
		}
		successCount++
	}
	
	if successCount == 0 {
		return fmt.Errorf("all datasource inspections failed for tenant %s", tenantId)
	}
	
	return nil
}

// GetRealtimeStatus 获取实时 Exporter 状态 (从数据库读取最新巡检结果)
// 委托给 pkg/exporter.Aggregator 处理业务逻辑
func (s *exporterMonitorService) GetRealtimeStatus(tenantId, datasourceId, status, job, keyword string) (interface{}, error) {
	aggregator := exporter.NewAggregator(s.ctx)
	return aggregator.GetRealtimeStatus(tenantId, datasourceId, status, job, keyword)
}

// GetHistory 获取历史趋势数据
// 委托给 pkg/exporter.Aggregator 处理业务逻辑
func (s *exporterMonitorService) GetHistory(tenantId, datasourceId string, startTime, endTime time.Time) (interface{}, error) {
	aggregator := exporter.NewAggregator(s.ctx)
	return aggregator.GetHistory(tenantId, datasourceId, startTime, endTime)
}

// GetConfig 获取 Exporter 监控配置 (纯 DB 操作)
func (s *exporterMonitorService) GetConfig(tenantId string) (models.ExporterMonitorConfig, error) {
	return s.ctx.DB.ExporterMonitor().GetConfig(tenantId)
}

// SaveConfig 保存 Exporter 监控配置 (纯 DB 操作)
func (s *exporterMonitorService) SaveConfig(config models.ExporterMonitorConfig) error {
	return s.ctx.DB.ExporterMonitor().SaveConfig(config)
}

// UpdateAutoRefresh 更新自动刷新开关状态 (纯 DB 操作)
func (s *exporterMonitorService) UpdateAutoRefresh(tenantId string, autoRefresh bool) error {
	return s.ctx.DB.ExporterMonitor().UpdateAutoRefresh(tenantId, autoRefresh)
}

// GetSchedule 获取报告推送配置 (纯 DB 操作)
func (s *exporterMonitorService) GetSchedule(tenantId string) (models.ExporterReportSchedule, error) {
	return s.ctx.DB.ExporterMonitor().GetSchedule(tenantId)
}

// SaveSchedule 保存报告推送配置 (纯 DB 操作)
func (s *exporterMonitorService) SaveSchedule(schedule models.ExporterReportSchedule) error {
	return s.ctx.DB.ExporterMonitor().SaveSchedule(schedule)
}

// SendReport 发送巡检报告 (手动触发或定时任务调用)
// 协调多个 pkg 层组件完成报告发送流程
func (s *exporterMonitorService) SendReport(tenantId string, noticeGroups []string, reportFormat string) error {
	// 1. 获取当前状态 (委托给 Aggregator)
	aggregator := exporter.NewAggregator(s.ctx)
	realtimeData, err := aggregator.GetRealtimeStatus(tenantId, "", "", "", "")
	if err != nil {
		return fmt.Errorf("获取实时状态失败: %w", err)
	}

	realtimeMap, ok := realtimeData.(map[string]interface{})
	if !ok {
		return fmt.Errorf("实时状态数据格式错误")
	}

	summary, ok := realtimeMap["summary"].(models.ExporterStatusSummary)
	if !ok {
		return fmt.Errorf("统计摘要数据格式错误")
	}

	exporters, ok := realtimeMap["exporters"].([]models.ExporterStatus)
	if !ok {
		return fmt.Errorf("Exporter列表数据格式错误")
	}

	// 2. 获取近 7 日趋势 (委托给 Aggregator)
	endTime := time.Now()
	startTime := endTime.AddDate(0, 0, -7)
	historyData, err := aggregator.GetHistory(tenantId, "", startTime, endTime)
	if err != nil {
		logc.Errorf(s.ctx.Ctx, "获取历史趋势失败: %v", err)
		// 历史数据获取失败不阻塞报告发送
	}

	// 3. 生成报告内容 (委托给 Reporter)
	reporter := exporter.NewReporter()
	content := reporter.GenerateReportContent(summary, exporters, historyData, reportFormat)

	// 4. 调用 Notifier 推送通知
	notifier := exporter.NewNotifier(s.ctx)
	return notifier.SendToNoticeGroups(tenantId, noticeGroups, content)
}

// ReloadSchedulerConfig 重载调度器配置
// 保存配置后调用,热更新调度器任务
func (s *exporterMonitorService) ReloadSchedulerConfig(tenantId string) error {
	return exporter.ReloadGlobalSchedulerConfig(tenantId)
}
