package api

import (
	"alertHub/internal/middleware"
	"alertHub/internal/models"
	"alertHub/internal/services"
	"alertHub/internal/types"
	"alertHub/pkg/response"
	"time"

	"github.com/gin-gonic/gin"
)

type exporterMonitorController struct{}

var ExporterMonitorController = new(exporterMonitorController)

// API 注册路由
func (exporterMonitorController exporterMonitorController) API(gin *gin.RouterGroup) {
	// 需要认证 + 权限 + 审计日志
	a := gin.Group("exporter/monitor")
	a.Use(
		middleware.Auth(),
		middleware.CasbinPermission(),
		middleware.AuditingLog(),
	)
	{
		// 配置管理
		a.POST("config", exporterMonitorController.SaveConfig)
		a.POST("schedule", exporterMonitorController.SaveSchedule)
		a.POST("autoRefresh", exporterMonitorController.UpdateAutoRefresh)

		// 手动触发巡检
		a.POST("inspect", exporterMonitorController.TriggerInspection)

		// 手动触发推送
		a.POST("report/send", exporterMonitorController.SendReport)
	}

	// 需要认证 + 权限
	b := gin.Group("exporter/monitor")
	b.Use(
		middleware.Auth(),
		middleware.CasbinPermission(),
	)
	{
		// 查询接口
		b.GET("status", exporterMonitorController.GetStatus)
		b.GET("history", exporterMonitorController.GetHistory)
		b.GET("config", exporterMonitorController.GetConfig)
		b.GET("schedule", exporterMonitorController.GetSchedule)
	}
}

// GetStatus 获取实时 Exporter 状态
// GET /api/w8t/exporter/monitor/status?datasourceId=xxx&status=down&job=node_exporter&keyword=192.168
func (exporterMonitorController exporterMonitorController) GetStatus(ctx *gin.Context) {
	r := new(types.RequestExporterMonitorStatus)
	BindQuery(ctx, r)

	Service(ctx, func() (interface{}, interface{}) {
		tenantId := ctx.GetString("TenantID")
		if tenantId == "" {
			tenantId = "default"
		}

		return services.ExporterMonitorService.GetRealtimeStatus(
			tenantId,
			r.DatasourceId,
			r.Status,
			r.Job,
			r.Keyword,
		)
	})
}

// GetHistory 获取历史趋势数据
// GET /api/w8t/exporter/monitor/history?datasourceId=xxx&startTime=2024-01-09T00:00:00Z&endTime=2024-01-15T23:59:59Z
func (exporterMonitorController exporterMonitorController) GetHistory(ctx *gin.Context) {
	r := new(types.RequestExporterMonitorHistory)
	BindQuery(ctx, r)

	// 解析时间
	startTime, err := time.Parse(time.RFC3339, r.StartTime)
	if err != nil {
		response.Fail(ctx, "startTime 格式错误,应为 RFC3339 格式", "failed")
		return
	}

	endTime, err := time.Parse(time.RFC3339, r.EndTime)
	if err != nil {
		response.Fail(ctx, "endTime 格式错误,应为 RFC3339 格式", "failed")
		return
	}

	Service(ctx, func() (interface{}, interface{}) {
		tenantId := ctx.GetString("TenantID")
		if tenantId == "" {
			tenantId = "default"
		}
		return services.ExporterMonitorService.GetHistory(
			tenantId,
			r.DatasourceId,
			startTime,
			endTime,
		)
	})
}

// GetConfig 获取监控配置
// GET /api/w8t/exporter/monitor/config
func (exporterMonitorController exporterMonitorController) GetConfig(ctx *gin.Context) {
	Service(ctx, func() (interface{}, interface{}) {
		tenantId := ctx.GetString("TenantID")
		if tenantId == "" {
			tenantId = "default"
		}
		return services.ExporterMonitorService.GetConfig(tenantId)
	})
}

// SaveConfig 保存监控配置
// POST /api/w8t/exporter/monitor/config
func (exporterMonitorController exporterMonitorController) SaveConfig(ctx *gin.Context) {
	r := new(models.ExporterMonitorConfig)
	BindJson(ctx, r)

	// 自动填充 TenantId
	tenantId := ctx.GetString("TenantID")
	if tenantId == "" {
		tenantId = "default"
	}
	r.TenantId = tenantId

	Service(ctx, func() (interface{}, interface{}) {
		err := services.ExporterMonitorService.SaveConfig(*r)
		if err != nil {
			return nil, err
		}

		// 配置保存成功后,重新加载调度器配置
		_ = services.ExporterMonitorService.ReloadSchedulerConfig(tenantId)

		return nil, nil
	})
}

// UpdateAutoRefresh 更新自动刷新开关
// POST /api/w8t/exporter/monitor/autoRefresh
func (exporterMonitorController exporterMonitorController) UpdateAutoRefresh(ctx *gin.Context) {
	r := new(struct {
		AutoRefresh bool `json:"autoRefresh"`
	})
	BindJson(ctx, r)

	Service(ctx, func() (interface{}, interface{}) {
		tenantId := ctx.GetString("TenantID")
		if tenantId == "" {
			tenantId = "default"
		}
		return nil, services.ExporterMonitorService.UpdateAutoRefresh(tenantId, r.AutoRefresh)
	})
}

// GetSchedule 获取报告推送配置
// GET /api/w8t/exporter/monitor/schedule
func (exporterMonitorController exporterMonitorController) GetSchedule(ctx *gin.Context) {
	Service(ctx, func() (interface{}, interface{}) {
		tenantId := ctx.GetString("TenantID")
		if tenantId == "" {
			tenantId = "default"
		}
		return services.ExporterMonitorService.GetSchedule(tenantId)
	})
}

// SaveSchedule 保存报告推送配置
// POST /api/w8t/exporter/monitor/schedule
func (exporterMonitorController exporterMonitorController) SaveSchedule(ctx *gin.Context) {
	r := new(models.ExporterReportSchedule)
	BindJson(ctx, r)

	// 自动填充 TenantId
	tenantId := ctx.GetString("TenantID")
	if tenantId == "" {
		tenantId = "default"
	}
	r.TenantId = tenantId

	Service(ctx, func() (interface{}, interface{}) {
		err := services.ExporterMonitorService.SaveSchedule(*r)
		if err != nil {
			return nil, err
		}

		// 配置保存成功后,重新加载调度器配置
		_ = services.ExporterMonitorService.ReloadSchedulerConfig(tenantId)

		return nil, nil
	})
}

// SendReport 手动触发报告推送
// POST /api/w8t/exporter/monitor/report/send
func (exporterMonitorController exporterMonitorController) SendReport(ctx *gin.Context) {
	r := new(types.RequestExporterMonitorSendReport)
	BindJson(ctx, r)

	// 默认简洁版
	if r.ReportFormat == "" {
		r.ReportFormat = "simple"
	}

	Service(ctx, func() (interface{}, interface{}) {
		tenantId := ctx.GetString("TenantID")
		if tenantId == "" {
			tenantId = "default"
		}
		return nil, services.ExporterMonitorService.SendReport(
			tenantId,
			r.NoticeGroups,
			r.ReportFormat,
		)
	})
}

// TriggerInspection 手动触发巡检
// POST /api/w8t/exporter/monitor/inspect
// 请求体: { "datasourceId": "xxx" } (可选,为空则巡检所有数据源)
func (exporterMonitorController exporterMonitorController) TriggerInspection(ctx *gin.Context) {
	r := new(types.RequestExporterMonitorInspect)
	BindJson(ctx, r)

	Service(ctx, func() (interface{}, interface{}) {
		tenantId := ctx.GetString("TenantID")
		if tenantId == "" {
			tenantId = "default"
		}
		return nil, services.ExporterMonitorService.TriggerManualInspection(
			tenantId,
			r.DatasourceId,
		)
	})
}
