package exporter

import (
	"fmt"
	"time"
	"alertHub/internal/ctx"
	"alertHub/internal/models"

	"github.com/zeromicro/go-zero/core/logc"
)

// Aggregator 数据聚合器 - 负责从数据库聚合和转换 Exporter 数据
type Aggregator struct {
	ctx *ctx.Context
}

// NewAggregator 创建数据聚合器实例
func NewAggregator(c *ctx.Context) *Aggregator {
	return &Aggregator{ctx: c}
}

// GetRealtimeStatus 获取实时 Exporter 状态 (从数据库读取最新巡检结果)
func (agg *Aggregator) GetRealtimeStatus(tenantId, datasourceId, status, job, keyword string) (interface{}, error) {
	// 1. 获取配置
	config, err := agg.ctx.DB.ExporterMonitor().GetConfig(tenantId)
	if err != nil {
		return nil, fmt.Errorf("获取配置失败: %w", err)
	}

	// 2. 确定要查询的数据源列表
	datasourceIds := config.DatasourceIds

	// 如果配置为空,则获取所有 Prometheus 类型的数据源
	if len(datasourceIds) == 0 {
		allDatasources, err := agg.ctx.DB.Datasource().List(tenantId, "", "prometheus", "")
		if err != nil {
			return nil, fmt.Errorf("获取数据源列表失败: %w", err)
		}
		for _, ds := range allDatasources {
			if *ds.GetEnabled() {
				datasourceIds = append(datasourceIds, ds.ID)
			}
		}
	}

	// 3. 筛选条件: 如果指定了 datasourceId,则只查询该数据源
	if datasourceId != "" {
		datasourceIds = []string{datasourceId}
	}

	// 4. 聚合所有数据源的最新巡检结果
	allExporters := make([]models.ExporterStatus, 0)
	totalSummary := models.ExporterStatusSummary{
		LastUpdateTime: time.Now(),
	}

	for _, dsId := range datasourceIds {
		// 获取最新的巡检记录
		inspection, err := agg.ctx.DB.ExporterMonitor().GetLatestInspection(tenantId, dsId)
		if err != nil {
			logc.Errorf(agg.ctx.Ctx, "获取最新巡检记录失败: dsId=%s, err=%v", dsId, err)
			continue
		}

		if inspection == nil {
			// 如果没有巡检记录,跳过
			continue
		}

		// 累加统计数据
		totalSummary.TotalCount += inspection.TotalCount
		totalSummary.UpCount += inspection.UpCount
		totalSummary.DownCount += inspection.DownCount
		totalSummary.UnknownCount += inspection.UnknownCount

		// 获取巡检明细 (支持过滤)
		details, err := agg.ctx.DB.ExporterMonitor().GetInspectionDetails(inspection.InspectionId, status, job, keyword)
		if err != nil {
			logc.Errorf(agg.ctx.Ctx, "获取巡检明细失败: inspectionId=%s, err=%v", inspection.InspectionId, err)
			continue
		}

		// 转换为 ExporterStatus
		for _, detail := range details {
			exporter := models.ExporterStatus{
				DatasourceId:   detail.DatasourceId,
				DatasourceName: detail.DatasourceName,
				Job:            detail.Job,
				Instance:       detail.Instance,
				Labels:         detail.Labels,
				ScrapeUrl:      detail.ScrapeUrl,
				Status:         detail.Status,
				LastScrapeTime: detail.LastScrapeTime,
				LastError:      detail.LastError,
				DownDuration:   detail.DownDuration,
				DownSince:      detail.DownSince,
			}
			allExporters = append(allExporters, exporter)
		}
	}

	// 5. 重新计算可用率
	if totalSummary.TotalCount > 0 {
		totalSummary.AvailabilityRate = float64(totalSummary.UpCount) / float64(totalSummary.TotalCount) * 100
	}

	return map[string]interface{}{
		"summary":   totalSummary,
		"exporters": allExporters,
	}, nil
}

// GetHistory 获取历史趋势数据
func (agg *Aggregator) GetHistory(tenantId, datasourceId string, startTime, endTime time.Time) (interface{}, error) {
	inspections, err := agg.ctx.DB.ExporterMonitor().GetInspectionsByTimeRange(tenantId, datasourceId, startTime, endTime)
	if err != nil {
		return nil, fmt.Errorf("查询历史巡检失败: %w", err)
	}

	// 转换为时间线格式
	timeline := make([]map[string]interface{}, 0, len(inspections))
	for _, inspection := range inspections {
		timeline = append(timeline, map[string]interface{}{
			"time":             inspection.InspectionTime,
			"datasourceName":   inspection.DatasourceName,
			"totalCount":       inspection.TotalCount,
			"upCount":          inspection.UpCount,
			"downCount":        inspection.DownCount,
			"unknownCount":     inspection.UnknownCount,
			"availabilityRate": inspection.AvailabilityRate,
			"downListSummary":  inspection.DownListSummary,
		})
	}

	return map[string]interface{}{
		"timeline": timeline,
	}, nil
}