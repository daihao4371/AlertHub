package exporter

import (
	"alertHub/internal/ctx"
	"alertHub/internal/models"
	"alertHub/pkg/provider"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/zeromicro/go-zero/core/logc"
)

// TargetHealth represents the health status of a monitoring target
type TargetHealth = provider.TargetHealth

// Inspector handles Exporter health inspections
type Inspector struct {
	ctx *ctx.Context
}

// NewInspector creates a new Inspector instance
func NewInspector(c *ctx.Context) *Inspector {
	return &Inspector{ctx: c}
}

// InspectAll performs health checks on all enabled Prometheus datasources
// forceInspect: whether to force execution ignoring time checks
func (ins *Inspector) InspectAll(forceInspect bool) error {
	currentTime := time.Now().Format("15:04")
	tenantIds := ins.getAllActiveTenantIds()

	for _, tenantId := range tenantIds {
		config, err := ins.ctx.DB.ExporterMonitor().GetConfig(tenantId)
		if err != nil || !config.GetEnabled() {
			continue
		}

		if ins.shouldExecuteInspection(config, currentTime, forceInspect) {
			ins.executeInspectionForTenant(tenantId, config)
		}
	}

	return nil
}

// shouldExecuteInspection determines if inspection should run for this tenant
func (ins *Inspector) shouldExecuteInspection(config models.ExporterMonitorConfig, currentTime string, forceInspect bool) bool {
	if forceInspect {
		return true
	}

	if ins.isFirstTimeInspection(config) {
		return true
	}

	return ins.isScheduledTime(config.InspectionTimes, currentTime)
}

// isFirstTimeInspection checks if this is the first inspection for any datasource
func (ins *Inspector) isFirstTimeInspection(config models.ExporterMonitorConfig) bool {
	for _, dsId := range config.DatasourceIds {
		latestInspection, _ := ins.ctx.DB.ExporterMonitor().GetLatestInspection(config.TenantId, dsId)
		if latestInspection != nil {
			return false
		}
	}
	return true
}

// isScheduledTime checks if current time matches any scheduled inspection time
func (ins *Inspector) isScheduledTime(inspectionTimes []string, currentTime string) bool {
	for _, inspectionTime := range inspectionTimes {
		if inspectionTime == currentTime {
			return true
		}
	}
	return false
}

// executeInspectionForTenant runs inspection for all datasources of a tenant
func (ins *Inspector) executeInspectionForTenant(tenantId string, config models.ExporterMonitorConfig) {
	for _, dsId := range config.DatasourceIds {
		_ = ins.InspectDatasource(dsId) // Ignore individual errors to continue with other datasources
	}

	// Clean up expired data
	_ = ins.ctx.DB.ExporterMonitor().DeleteExpiredInspections(tenantId, config.HistoryRetention)
}

// InspectDatasource performs health check on a single Prometheus datasource
func (ins *Inspector) InspectDatasource(datasourceId string) error {
	datasource, err := ins.getDatasourceInfo(datasourceId)
	if err != nil {
		return err
	}

	prometheusProvider, err := ins.getPrometheusProvider(datasourceId)
	if err != nil {
		return err
	}

	targets, err := prometheusProvider.GetTargets()
	if err != nil {
		return fmt.Errorf("failed to get targets from datasource %s: %w", datasourceId, err)
	}

	return ins.processAndStoreInspectionResults(datasource, targets)
}

// getDatasourceInfo retrieves datasource information from database
func (ins *Inspector) getDatasourceInfo(datasourceId string) (*models.AlertDataSource, error) {
	datasource, err := ins.ctx.DB.Datasource().Get(datasourceId)
	if err != nil {
		return nil, fmt.Errorf("failed to get datasource %s: %w", datasourceId, err)
	}
	return &datasource, nil
}

// getPrometheusProvider gets and validates Prometheus provider from pool
func (ins *Inspector) getPrometheusProvider(datasourceId string) (provider.PrometheusProvider, error) {
	providerInterface, err := ins.ctx.Redis.ProviderPools().GetClient(datasourceId)
	if err != nil {
		return provider.PrometheusProvider{}, fmt.Errorf("failed to get provider for datasource %s: %w", datasourceId, err)
	}

	prometheusProvider, ok := providerInterface.(provider.PrometheusProvider)
	if !ok {
		return provider.PrometheusProvider{}, fmt.Errorf("datasource %s is not prometheus type, actual type: %T", datasourceId, providerInterface)
	}

	return prometheusProvider, nil
}

// processAndStoreInspectionResults processes targets and stores inspection results
func (ins *Inspector) processAndStoreInspectionResults(datasource *models.AlertDataSource, targets []TargetHealth) error {
	inspectionId := uuid.New().String()
	inspectionTime := time.Now()

	statistics := ins.calculateTargetStatistics(targets)
	details := ins.createInspectionDetails(datasource, targets, inspectionId, inspectionTime)

	inspection := ins.buildInspectionRecord(datasource, inspectionId, inspectionTime, statistics)

	if err := ins.ctx.DB.ExporterMonitor().CreateInspection(inspection); err != nil {
		return fmt.Errorf("failed to create inspection record for datasource %s: %w", datasource.ID, err)
	}

	if err := ins.ctx.DB.ExporterMonitor().CreateInspectionDetails(details); err != nil {
		return fmt.Errorf("failed to create inspection details for datasource %s: %w", datasource.ID, err)
	}

	// 输出巡检汇总日志
	logc.Infof(ins.ctx.Ctx, "巡检完成: datasource=%s(%s), total=%d, up=%d, down=%d, unknown=%d, availability=%.2f%%",
		datasource.Name, datasource.ID, statistics.TotalCount, statistics.UpCount, 
		statistics.DownCount, statistics.UnknownCount, statistics.AvailabilityRate)

	return nil
}

// TargetStatistics represents the statistics of target health status
type TargetStatistics struct {
	TotalCount       int
	UpCount          int
	DownCount        int
	UnknownCount     int
	AvailabilityRate float64
	DownListSummary  []map[string]interface{}
}

// calculateTargetStatistics analyzes targets and returns health statistics
func (ins *Inspector) calculateTargetStatistics(targets []TargetHealth) TargetStatistics {
	stats := TargetStatistics{
		TotalCount:      len(targets),
		DownListSummary: make([]map[string]interface{}, 0),
	}

	for _, target := range targets {
		status := MapHealthStatus(target.Health, target.LastError)

		switch status {
		case "up":
			stats.UpCount++
		case "down":
			stats.DownCount++
			ins.addToDownListSummary(&stats, target)
		case "unknown":
			stats.UnknownCount++
		}
	}

	if stats.TotalCount > 0 {
		stats.AvailabilityRate = float64(stats.UpCount) / float64(stats.TotalCount) * 100
	}

	return stats
}

// addToDownListSummary adds a down target to the summary (max 10 items)
func (ins *Inspector) addToDownListSummary(stats *TargetStatistics, target TargetHealth) {
	if len(stats.DownListSummary) < 10 {
		stats.DownListSummary = append(stats.DownListSummary, map[string]interface{}{
			"instance":  target.Instance,
			"job":       target.Job,
			"lastError": target.LastError,
		})
	}
}

// createInspectionDetails creates detailed inspection records for all targets
func (ins *Inspector) createInspectionDetails(datasource *models.AlertDataSource, targets []TargetHealth, inspectionId string, inspectionTime time.Time) []models.ExporterInspectionDetail {
	details := make([]models.ExporterInspectionDetail, 0, len(targets))

	for _, target := range targets {
		detail := ins.buildInspectionDetail(datasource, target, inspectionId, inspectionTime)
		details = append(details, detail)
	}

	return details
}

// buildInspectionDetail creates a single inspection detail record
func (ins *Inspector) buildInspectionDetail(datasource *models.AlertDataSource, target TargetHealth, inspectionId string, inspectionTime time.Time) models.ExporterInspectionDetail {
	lastScrapeTime, _ := time.Parse(time.RFC3339, target.LastScrape)
	status := MapHealthStatus(target.Health, target.LastError)

	labels := make(map[string]interface{})
	for k, v := range target.Labels {
		labels[k] = v
	}

	return models.ExporterInspectionDetail{
		InspectionId:   inspectionId,
		TenantId:       datasource.TenantId,
		DatasourceId:   datasource.ID,
		DatasourceName: datasource.Name,
		Job:            target.Job,
		Instance:       target.Instance,
		Labels:         labels,
		ScrapeUrl:      target.ScrapeUrl,
		Status:         status,
		LastScrapeTime: lastScrapeTime,
		LastError:      target.LastError,
		CreatedAt:      inspectionTime,
	}
}

// buildInspectionRecord creates the main inspection record
func (ins *Inspector) buildInspectionRecord(datasource *models.AlertDataSource, inspectionId string, inspectionTime time.Time, stats TargetStatistics) models.ExporterInspection {
	return models.ExporterInspection{
		InspectionId:     inspectionId,
		TenantId:         datasource.TenantId,
		DatasourceId:     datasource.ID,
		DatasourceName:   datasource.Name,
		InspectionTime:   inspectionTime,
		TotalCount:       stats.TotalCount,
		UpCount:          stats.UpCount,
		DownCount:        stats.DownCount,
		UnknownCount:     stats.UnknownCount,
		AvailabilityRate: stats.AvailabilityRate,
		DownListSummary:  stats.DownListSummary,
	}
}

// MapHealthStatus maps Prometheus health status to our internal status
func MapHealthStatus(health string, lastError string) string {
	if health == "up" && lastError == "" {
		return "up"
	}
	if health == "down" || lastError != "" {
		return "down"
	}
	return "unknown"
}

// getAllActiveTenantIds retrieves all tenant IDs with enabled exporter monitoring
func (ins *Inspector) getAllActiveTenantIds() []string {
	var tenantIds []string
	err := ins.ctx.DB.DB().Model(&models.ExporterMonitorConfig{}).
		Select("tenant_id").
		Where("enabled = ?", 1).
		Scan(&tenantIds).Error

	if err != nil {
		return []string{}
	}

	return tenantIds
}