package exporter

import (
	"alertHub/internal/ctx"
	"alertHub/internal/models"
	"fmt"
	"time"

	"github.com/zeromicro/go-zero/core/logc"
)

// Aggregator 数据聚合器 - 负责从数据库聚合和转换 Exporter 数据
// 将数据检索与业务逻辑分离，提高代码可维护性
type Aggregator struct {
	ctx *ctx.Context
}

// NewAggregator 创建数据聚合器实例
func NewAggregator(c *ctx.Context) *Aggregator {
	return &Aggregator{ctx: c}
}

// GetRealtimeStatus 获取实时 Exporter 状态，通过聚合最新巡检结果
// 处理数据源过滤、状态聚合和结果格式化
func (agg *Aggregator) GetRealtimeStatus(tenantId, datasourceId, status, job, keyword string) (interface{}, error) {
	datasourceIds, err := agg.resolveDatasourceIds(tenantId, datasourceId)
	if err != nil {
		return nil, err
	}

	summary, exporters, err := agg.aggregateInspectionResults(tenantId, datasourceIds, status, job, keyword)
	if err != nil {
		return nil, err
	}

	agg.calculateAvailabilityRate(&summary)

	// 输出聚合汇总日志
	datasourceInfo := ""
	if datasourceId != "" {
		datasourceInfo = fmt.Sprintf("(datasource=%s)", datasourceId)
	} else {
		datasourceInfo = fmt.Sprintf("(datasources=%d)", len(datasourceIds))
	}
	logc.Infof(agg.ctx.Ctx, "聚合完成: tenantId=%s %s, total=%d, up=%d, down=%d, unknown=%d, availability=%.2f%%",
		tenantId, datasourceInfo, summary.TotalCount, summary.UpCount,
		summary.DownCount, summary.UnknownCount, summary.AvailabilityRate)

	return map[string]interface{}{
		"summary":   summary,
		"exporters": exporters,
	}, nil
}

// resolveDatasourceIds 根据配置和过滤条件确定要查询的数据源列表
// 返回配置的数据源，如果配置为空则返回所有启用的 Prometheus 数据源
func (agg *Aggregator) resolveDatasourceIds(tenantId, filterDatasourceId string) ([]string, error) {
	config, err := agg.ctx.DB.ExporterMonitor().GetConfig(tenantId)
	if err != nil {
		return nil, fmt.Errorf("获取配置失败: %w", err)
	}

	// 如果指定了数据源，只返回该数据源
	if filterDatasourceId != "" {
		return []string{filterDatasourceId}, nil
	}

	// 如果配置中有数据源，使用配置的数据源
	if len(config.DatasourceIds) > 0 {
		return config.DatasourceIds, nil
	}

	// 回退方案：获取所有启用的 Prometheus 数据源
	return agg.getAllEnabledPrometheusDatasources(tenantId)
}

// getAllEnabledPrometheusDatasources 获取租户下所有启用的 Prometheus 数据源
// 当没有配置特定数据源时，作为回退方案使用
func (agg *Aggregator) getAllEnabledPrometheusDatasources(tenantId string) ([]string, error) {
	datasources, err := agg.ctx.DB.Datasource().List(tenantId, "", "prometheus", "")
	if err != nil {
		return nil, fmt.Errorf("获取数据源列表失败: %w", err)
	}

	ids := make([]string, 0)
	for _, ds := range datasources {
		if *ds.GetEnabled() {
			ids = append(ids, ds.ID)
		}
	}

	return ids, nil
}

// aggregateInspectionResults 聚合多个数据源的巡检结果
// 收集 exporter 状态并累加统计摘要
func (agg *Aggregator) aggregateInspectionResults(
	tenantId string,
	datasourceIds []string,
	status, job, keyword string,
) (models.ExporterStatusSummary, []models.ExporterStatus, error) {
	summary := models.ExporterStatusSummary{
		LastUpdateTime: time.Now(),
	}
	exporters := make([]models.ExporterStatus, 0)

	for _, dsId := range datasourceIds {
		dsSummary, dsExporters, err := agg.aggregateSingleDatasource(tenantId, dsId, status, job, keyword)
		if err != nil {
			logc.Errorf(agg.ctx.Ctx, "聚合数据源 %s 失败: %v", dsId, err)
			continue
		}

		agg.mergeSummary(&summary, &dsSummary)
		exporters = append(exporters, dsExporters...)
	}

	return summary, exporters, nil
}

// aggregateSingleDatasource 聚合单个数据源的巡检结果
// 仅返回该数据源的摘要和 exporter 列表
func (agg *Aggregator) aggregateSingleDatasource(
	tenantId, datasourceId, status, job, keyword string,
) (models.ExporterStatusSummary, []models.ExporterStatus, error) {
	inspection, err := agg.ctx.DB.ExporterMonitor().GetLatestInspection(tenantId, datasourceId)
	if err != nil {
		return models.ExporterStatusSummary{}, nil, fmt.Errorf("获取最新巡检记录失败: %w", err)
	}

	if inspection == nil {
		// 未找到巡检记录，返回空结果
		return models.ExporterStatusSummary{}, []models.ExporterStatus{}, nil
	}

	details, err := agg.ctx.DB.ExporterMonitor().GetInspectionDetails(
		inspection.InspectionId, status, job, keyword,
	)
	if err != nil {
		return models.ExporterStatusSummary{}, nil, fmt.Errorf("获取巡检明细失败: %w", err)
	}

	summary := models.ExporterStatusSummary{
		TotalCount:     inspection.TotalCount,
		UpCount:        inspection.UpCount,
		DownCount:      inspection.DownCount,
		UnknownCount:   inspection.UnknownCount,
		LastUpdateTime: time.Now(),
	}

	exporters := agg.convertDetailsToStatuses(details)

	return summary, exporters, nil
}

// convertDetailsToStatuses 将巡检明细转换为 ExporterStatus 对象
// 此映射确保 API 响应的数据结构一致性
func (agg *Aggregator) convertDetailsToStatuses(details []models.ExporterInspectionDetail) []models.ExporterStatus {
	exporters := make([]models.ExporterStatus, 0, len(details))
	for _, detail := range details {
		exporters = append(exporters, models.ExporterStatus{
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
		})
	}
	return exporters
}

// mergeSummary 将数据源摘要合并到总摘要中
// 累加多个数据源的统计数据
func (agg *Aggregator) mergeSummary(total, source *models.ExporterStatusSummary) {
	total.TotalCount += source.TotalCount
	total.UpCount += source.UpCount
	total.DownCount += source.DownCount
	total.UnknownCount += source.UnknownCount
}

// calculateAvailabilityRate 计算可用率（百分比）
// 仅在有 exporter 时计算，避免除零错误
func (agg *Aggregator) calculateAvailabilityRate(summary *models.ExporterStatusSummary) {
	if summary.TotalCount == 0 {
		return
	}
	summary.AvailabilityRate = float64(summary.UpCount) / float64(summary.TotalCount) * 100
}

// GetHistory 获取指定时间范围的历史趋势数据
// 将巡检记录转换为时间线格式，用于图表展示
func (agg *Aggregator) GetHistory(tenantId, datasourceId string, startTime, endTime time.Time) (interface{}, error) {
	inspections, err := agg.ctx.DB.ExporterMonitor().GetInspectionsByTimeRange(
		tenantId, datasourceId, startTime, endTime,
	)
	if err != nil {
		return nil, fmt.Errorf("查询历史数据失败: %w", err)
	}

	timeline := agg.convertInspectionsToTimeline(inspections)

	return map[string]interface{}{
		"timeline": timeline,
	}, nil
}

// convertInspectionsToTimeline 将巡检记录转换为时间线格式
// 此格式针对前端图表库进行了优化
func (agg *Aggregator) convertInspectionsToTimeline(inspections []models.ExporterInspection) []map[string]interface{} {
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
	return timeline
}
