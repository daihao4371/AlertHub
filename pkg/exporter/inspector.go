package exporter

import (
	"alertHub/internal/ctx"
	"alertHub/internal/models"
	"alertHub/pkg/provider"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// Inspector 巡检器 - 负责执行 Exporter 健康巡检
type Inspector struct {
	ctx *ctx.Context
}

// NewInspector 创建巡检器实例
func NewInspector(c *ctx.Context) *Inspector {
	return &Inspector{ctx: c}
}

// InspectAll 巡检所有已启用的 Prometheus 数据源
// forceInspect: 是否强制执行巡检(忽略时间检查)
// - false: 只在当前时间匹配租户配置的巡检时间时才执行
// - true: 忽略时间检查,立即执行巡检(用于应用启动时刷新状态)
// 特殊情况: 如果租户没有任何历史巡检记录,则立即执行一次初始化巡检
func (ins *Inspector) InspectAll(forceInspect bool) error {
	// 获取当前时间 (HH:MM 格式)
	currentTime := time.Now().Format("15:04")

	// 获取所有租户的配置
	tenantIds := ins.getAllTenantIds()

	executedCount := 0
	for _, tenantId := range tenantIds {
		// 获取配置
		config, err := ins.ctx.DB.ExporterMonitor().GetConfig(tenantId)
		if err != nil || !config.GetEnabled() {
			continue
		}

		// 检查是否需要执行巡检
		shouldInspect := false
		isFirstInspection := true // 默认为首次巡检

		// 1. 如果强制执行,直接巡检
		if forceInspect {
			shouldInspect = true
		} else {
			// 2. 检查是否有历史巡检记录 (任意数据源有记录就不是首次)
			for _, dsId := range config.DatasourceIds {
				latestInspection, _ := ins.ctx.DB.ExporterMonitor().GetLatestInspection(tenantId, dsId)
				if latestInspection != nil {
					// 发现有历史记录,不是首次巡检
					isFirstInspection = false
					break
				}
			}

			// 3. 如果是首次巡检,立即执行
			if isFirstInspection {
				shouldInspect = true
			} else {
				// 4. 否则检查当前时间是否在配置的巡检时间点中
				for _, inspectionTime := range config.InspectionTimes {
					if inspectionTime == currentTime {
						shouldInspect = true
						break
					}
				}
			}
		}

		// 如果不需要巡检,跳过
		if !shouldInspect {
			continue
		}

		// 遍历配置的数据源
		for _, dsId := range config.DatasourceIds {
			err := ins.InspectDatasource(dsId)
			if err != nil {
				continue
			}
		}

		// 清理过期数据
		err = ins.ctx.DB.ExporterMonitor().DeleteExpiredInspections(tenantId, config.HistoryRetention)
		if err != nil {
			// 忽略清理错误
		}

		executedCount++
	}

	return nil
}

// InspectDatasource 巡检单个 Prometheus 数据源并写入数据库
func (ins *Inspector) InspectDatasource(datasourceId string) error {
	// 1. 获取数据源信息
	datasource, err := ins.ctx.DB.Datasource().Get(datasourceId)
	if err != nil {
		return fmt.Errorf("failed to get datasource %s: %w", datasourceId, err)
	}

	// 2. 从 ProviderPools 获取 Provider
	providerInterface, err := ins.ctx.Redis.ProviderPools().GetClient(datasourceId)
	if err != nil {
		return fmt.Errorf("failed to get provider for datasource %s: %w", datasourceId, err)
	}

	// 3. 类型断言为 PrometheusProvider
	prometheusProvider, ok := providerInterface.(provider.PrometheusProvider)
	if !ok {
		return fmt.Errorf("datasource %s is not prometheus type, actual type: %T", datasourceId, providerInterface)
	}

	// 4. 获取所有 Targets
	targets, err := prometheusProvider.GetTargets()
	if err != nil {
		return fmt.Errorf("failed to get targets from datasource %s: %w", datasourceId, err)
	}

	// 5. 生成巡检批次ID
	inspectionId := uuid.New().String()
	inspectionTime := time.Now()

	// 6. 统计数据
	upCount := 0
	downCount := 0
	unknownCount := 0
	downListSummary := make([]map[string]interface{}, 0)
	details := make([]models.ExporterInspectionDetail, 0, len(targets))

	for _, target := range targets {
		// 解析时间
		lastScrapeTime, _ := time.Parse(time.RFC3339, target.LastScrape)

		// 判断状态
		status := MapHealthStatus(target.Health, target.LastError)

		// 转换 Labels
		labels := make(map[string]interface{})
		for k, v := range target.Labels {
			labels[k] = v
		}

		// 创建明细记录
		detail := models.ExporterInspectionDetail{
			InspectionId:   inspectionId,
			TenantId:       datasource.TenantId,
			DatasourceId:   datasourceId,
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

		// 统计数量
		switch status {
		case "up":
			upCount++
		case "down":
			downCount++
			// 收集前10个DOWN实例到摘要
			if len(downListSummary) < 10 {
				downListSummary = append(downListSummary, map[string]interface{}{
					"instance":  target.Instance,
					"job":       target.Job,
					"lastError": target.LastError,
				})
			}
		case "unknown":
			unknownCount++
		}

		details = append(details, detail)
	}

	// 7. 计算可用率
	totalCount := len(targets)
	availabilityRate := 0.0
	if totalCount > 0 {
		availabilityRate = float64(upCount) / float64(totalCount) * 100
	}

	// 8. 创建主表记录
	inspection := models.ExporterInspection{
		InspectionId:     inspectionId,
		TenantId:         datasource.TenantId,
		DatasourceId:     datasourceId,
		DatasourceName:   datasource.Name,
		InspectionTime:   inspectionTime,
		TotalCount:       totalCount,
		UpCount:          upCount,
		DownCount:        downCount,
		UnknownCount:     unknownCount,
		AvailabilityRate: availabilityRate,
		DownListSummary:  downListSummary,
	}

	err = ins.ctx.DB.ExporterMonitor().CreateInspection(inspection)
	if err != nil {
		return fmt.Errorf("failed to create inspection record for datasource %s: %w", datasourceId, err)
	}

	// 9. 批量插入明细记录
	err = ins.ctx.DB.ExporterMonitor().CreateInspectionDetails(details)
	if err != nil {
		return fmt.Errorf("failed to create inspection details for datasource %s: %w", datasourceId, err)
	}

	return nil
}

// MapHealthStatus 映射 Prometheus Health 状态
func MapHealthStatus(health string, lastError string) string {
	if health == "up" && lastError == "" {
		return "up"
	}
	if health == "down" || lastError != "" {
		return "down"
	}
	return "unknown"
}

// getAllTenantIds 获取所有已启用的租户ID
func (ins *Inspector) getAllTenantIds() []string {
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
