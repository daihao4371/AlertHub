package services

import (
	"context"
	"fmt"
	"time"
	"watchAlert/internal/ctx"
	"watchAlert/internal/models"
	"watchAlert/pkg/provider"
	"watchAlert/pkg/sender"

	"github.com/bytedance/sonic"
	"github.com/zeromicro/go-zero/core/logc"
)

type exporterMonitorService struct {
	ctx *ctx.Context
}

type InterExporterMonitorService interface {
	// 采集相关
	CollectAll() error
	CollectFromDatasource(datasourceId string) error

	// 快照相关
	RecordSnapshot() error

	// 查询相关
	GetRealtimeStatus(tenantId, datasourceId, status, job, keyword string) (interface{}, error)
	GetHistory(tenantId, datasourceId string, startTime, endTime time.Time) (interface{}, error)

	// 配置相关
	GetConfig(tenantId string) (models.ExporterMonitorConfig, error)
	SaveConfig(config models.ExporterMonitorConfig) error
	UpdateAutoRefresh(tenantId string, autoRefresh bool) error

	GetSchedule(tenantId string) (models.ExporterReportSchedule, error)
	SaveSchedule(schedule models.ExporterReportSchedule) error

	// 报告相关
	SendReport(tenantId string, noticeGroups []string, reportFormat string) error
}

func newInterExporterMonitorService(ctx *ctx.Context) InterExporterMonitorService {
	return &exporterMonitorService{
		ctx: ctx,
	}
}

// CollectAll 采集所有已启用的 Prometheus 数据源
func (s *exporterMonitorService) CollectAll() error {
	// 获取所有 Prometheus 类型的数据源
	datasources, err := s.ctx.DB.Datasource().List("", "", "prometheus", "")
	if err != nil {
		logc.Errorf(s.ctx.Ctx, "获取数据源列表失败: %v", err)
		return err
	}

	// 遍历采集 (忽略单个数据源的失败,继续采集其他数据源)
	for _, ds := range datasources {
		if !*ds.GetEnabled() {
			continue
		}

		err := s.CollectFromDatasource(ds.ID)
		if err != nil {
			logc.Errorf(s.ctx.Ctx, "采集数据源失败: datasource=%s, err=%v", ds.Name, err)
			continue
		}
	}

	return nil
}

// CollectFromDatasource 从单个 Prometheus 数据源采集 Exporter 状态
// 复用现有的 PrometheusProvider,避免重复代码
func (s *exporterMonitorService) CollectFromDatasource(datasourceId string) error {
	// 从 ProviderPools 获取已创建的 Provider (复用现有连接池)
	providerInterface, err := s.ctx.Redis.ProviderPools().GetClient(datasourceId)
	if err != nil {
		return fmt.Errorf("获取数据源 Provider 失败: %w", err)
	}

	// 类型断言为 PrometheusProvider
	prometheusProvider, ok := providerInterface.(provider.PrometheusProvider)
	if !ok {
		return fmt.Errorf("数据源类型不是 Prometheus")
	}

	// 调用 PrometheusProvider.GetTargets() 获取所有 Targets
	targets, err := prometheusProvider.GetTargets()
	if err != nil {
		return fmt.Errorf("获取 Targets 失败: %w", err)
	}

	// 转换为 ExporterStatus 列表
	exporters := make([]models.ExporterStatus, 0, len(targets))
	upCount := 0
	downCount := 0
	unknownCount := 0

	for _, target := range targets {
		// 解析时间
		lastScrapeTime, _ := time.Parse(time.RFC3339, target.LastScrape)

		// 判断状态 (复用 mapHealthStatus 逻辑)
		status := mapHealthStatus(target.Health, target.LastError)

		// 转换 Labels 为 map[string]interface{} (适配模型)
		labels := make(map[string]interface{})
		for k, v := range target.Labels {
			labels[k] = v
		}

		exporter := models.ExporterStatus{
			DatasourceId:   datasourceId,
			Job:            target.Job,
			Instance:       target.Instance,
			Labels:         labels,
			ScrapeUrl:      target.ScrapeUrl,
			Status:         status,
			LastScrapeTime: lastScrapeTime,
			LastError:      target.LastError,
		}

		exporters = append(exporters, exporter)

		// 统计数量
		switch status {
		case "up":
			upCount++
		case "down":
			downCount++
		case "unknown":
			unknownCount++
		}
	}

	// 计算可用率
	totalCount := len(exporters)
	availabilityRate := 0.0
	if totalCount > 0 {
		availabilityRate = float64(upCount) / float64(totalCount) * 100
	}

	// 构造统计摘要
	summary := models.ExporterStatusSummary{
		TotalCount:       totalCount,
		UpCount:          upCount,
		DownCount:        downCount,
		UnknownCount:     unknownCount,
		AvailabilityRate: availabilityRate,
		LastUpdateTime:   time.Now(),
	}

	// 写入 Redis 缓存 (TTL 设置为 5 分钟)
	ttl := 5 * time.Minute
	err = s.ctx.Redis.ExporterMonitor().SetExporters(datasourceId, exporters, ttl)
	if err != nil {
		logc.Errorf(s.ctx.Ctx, "写入 Exporter 列表缓存失败: %v", err)
	}

	err = s.ctx.Redis.ExporterMonitor().SetSummary(datasourceId, summary, ttl)
	if err != nil {
		logc.Errorf(s.ctx.Ctx, "写入统计摘要缓存失败: %v", err)
	}

	logc.Infof(s.ctx.Ctx, "采集完成: datasourceId=%s, total=%d, up=%d, down=%d, unknown=%d",
		datasourceId, totalCount, upCount, downCount, unknownCount)

	return nil
}

// mapHealthStatus 映射 Prometheus Health 状态
func mapHealthStatus(health string, lastError string) string {
	if health == "up" && lastError == "" {
		return "up"
	}
	if health == "down" || lastError != "" {
		return "down"
	}
	return "unknown"
}

// RecordSnapshot 记录历史快照 (定时任务调用)
func (s *exporterMonitorService) RecordSnapshot() error {
	// 获取所有租户的配置
	tenantIds := s.getAllTenantIds()

	for _, tenantId := range tenantIds {
		// 获取配置
		config, err := s.GetConfig(tenantId)
		if err != nil || !config.GetEnabled() {
			continue
		}

		// 遍历配置的数据源
		for _, dsId := range config.DatasourceIds {
			// 从缓存读取统计摘要
			summary, err := s.ctx.Redis.ExporterMonitor().GetSummary(dsId)
			if err != nil || summary == nil {
				logc.Errorf(s.ctx.Ctx, "读取缓存失败: dsId=%s, err=%v", dsId, err)
				continue
			}

			// 读取 Exporter 列表,提取 DOWN 状态
			exporters, err := s.ctx.Redis.ExporterMonitor().GetExporters(dsId)
			if err != nil {
				logc.Errorf(s.ctx.Ctx, "读取 Exporter 列表失败: dsId=%s, err=%v", dsId, err)
				continue
			}

			downList := extractDownList(exporters)

			// 创建快照记录
			snapshot := models.ExporterMonitorSnapshot{
				TenantId:         tenantId,
				DatasourceId:     dsId,
				SnapshotTime:     time.Now(),
				TotalCount:       summary.TotalCount,
				UpCount:          summary.UpCount,
				DownCount:        summary.DownCount,
				UnknownCount:     summary.UnknownCount,
				AvailabilityRate: summary.AvailabilityRate,
				DownList:         downList,
			}

			err = s.ctx.DB.ExporterMonitor().CreateSnapshot(snapshot)
			if err != nil {
				logc.Errorf(s.ctx.Ctx, "创建快照失败: %v", err)
			}
		}
	}

	// 清理过期数据
	for _, tenantId := range tenantIds {
		config, err := s.GetConfig(tenantId)
		if err != nil {
			continue
		}

		err = s.ctx.DB.ExporterMonitor().DeleteExpiredSnapshots(tenantId, config.HistoryRetention)
		if err != nil {
			logc.Errorf(s.ctx.Ctx, "清理过期快照失败: tenantId=%s, err=%v", tenantId, err)
		}
	}

	return nil
}

// extractDownList 提取 DOWN 状态的实例列表
func extractDownList(exporters []models.ExporterStatus) []map[string]interface{} {
	downList := make([]map[string]interface{}, 0)
	for _, exp := range exporters {
		if exp.Status == "down" {
			downList = append(downList, map[string]interface{}{
				"instance":  exp.Instance,
				"job":       exp.Job,
				"labels":    exp.Labels,
				"lastError": exp.LastError,
				"downSince": exp.DownSince,
			})
		}
	}
	return downList
}

// getAllTenantIds 获取所有已启用的租户ID
func (s *exporterMonitorService) getAllTenantIds() []string {
	var tenantIds []string
	err := s.ctx.DB.DB().Model(&models.ExporterMonitorConfig{}).
		Select("tenant_id").
		Where("enabled = ?", 1).
		Scan(&tenantIds).Error

	if err != nil {
		logc.Errorf(context.Background(), "获取租户列表失败: %v", err)
		return []string{}
	}

	return tenantIds
}

// GetConfig 获取 Exporter 监控配置
func (s *exporterMonitorService) GetConfig(tenantId string) (models.ExporterMonitorConfig, error) {
	return s.ctx.DB.ExporterMonitor().GetConfig(tenantId)
}

// SaveConfig 保存 Exporter 监控配置
func (s *exporterMonitorService) SaveConfig(config models.ExporterMonitorConfig) error {
	return s.ctx.DB.ExporterMonitor().SaveConfig(config)
}

// GetSchedule 获取报告推送配置
func (s *exporterMonitorService) GetSchedule(tenantId string) (models.ExporterReportSchedule, error) {
	return s.ctx.DB.ExporterMonitor().GetSchedule(tenantId)
}

// SaveSchedule 保存报告推送配置
func (s *exporterMonitorService) SaveSchedule(schedule models.ExporterReportSchedule) error {
	return s.ctx.DB.ExporterMonitor().SaveSchedule(schedule)
}

// UpdateAutoRefresh 更新自动刷新状态
func (s *exporterMonitorService) UpdateAutoRefresh(tenantId string, autoRefresh bool) error {
	// 获取当前配置
	config, err := s.GetConfig(tenantId)
	if err != nil {
		return err
	}

	// 更新自动刷新状态
	config.AutoRefresh = &autoRefresh
	config.TenantId = tenantId

	// 保存配置
	return s.SaveConfig(config)
}

// GetRealtimeStatus 获取实时 Exporter 状态 (供 API 调用)
func (s *exporterMonitorService) GetRealtimeStatus(tenantId, datasourceId, status, job, keyword string) (interface{}, error) {
	// 获取配置
	config, err := s.GetConfig(tenantId)
	if err != nil {
		return nil, fmt.Errorf("获取配置失败: %w", err)
	}

	// 确定要查询的数据源列表
	datasourceIds := config.DatasourceIds

	// 如果配置为空,则获取所有 Prometheus 类型的数据源
	if len(datasourceIds) == 0 {
		allDatasources, err := s.ctx.DB.Datasource().List("", "", "prometheus", "")
		if err != nil {
			return nil, fmt.Errorf("获取数据源列表失败: %w", err)
		}
		for _, ds := range allDatasources {
			if *ds.GetEnabled() {
				datasourceIds = append(datasourceIds, ds.ID)
			}
		}
	}

	// 聚合所有数据源的结果
	allExporters := make([]models.ExporterStatus, 0)
	totalSummary := models.ExporterStatusSummary{
		LastUpdateTime: time.Now(),
	}

	for _, dsId := range datasourceIds {
		// 筛选条件: 如果指定了 datasourceId,则只查询该数据源
		if datasourceId != "" && dsId != datasourceId {
			continue
		}

		// 从缓存读取
		exporters, err := s.ctx.Redis.ExporterMonitor().GetExporters(dsId)
		if err != nil {
			logc.Errorf(s.ctx.Ctx, "读取缓存失败: dsId=%s, err=%v", dsId, err)
			continue
		}

		summary, _ := s.ctx.Redis.ExporterMonitor().GetSummary(dsId)
		if summary != nil {
			totalSummary.TotalCount += summary.TotalCount
			totalSummary.UpCount += summary.UpCount
			totalSummary.DownCount += summary.DownCount
			totalSummary.UnknownCount += summary.UnknownCount
		}

		// 过滤条件
		for _, exp := range exporters {
			// 状态筛选
			if status != "" && exp.Status != status {
				continue
			}

			// Job 筛选
			if job != "" && exp.Job != job {
				continue
			}

			// 关键词筛选 (支持 IP/实例名/标签)
			if keyword != "" && !matchKeyword(exp, keyword) {
				continue
			}

			allExporters = append(allExporters, exp)
		}
	}

	// 重新计算可用率
	if totalSummary.TotalCount > 0 {
		totalSummary.AvailabilityRate = float64(totalSummary.UpCount) / float64(totalSummary.TotalCount) * 100
	}

	return map[string]interface{}{
		"summary":   totalSummary,
		"exporters": allExporters,
	}, nil
}

// matchKeyword 关键词匹配 (简单实现,支持 instance/job/labels)
func matchKeyword(exp models.ExporterStatus, keyword string) bool {
	// TODO: 更精确的匹配逻辑
	return false
}

// GetHistory 获取历史趋势数据
func (s *exporterMonitorService) GetHistory(tenantId, datasourceId string, startTime, endTime time.Time) (interface{}, error) {
	snapshots, err := s.ctx.DB.ExporterMonitor().GetSnapshotsByTimeRange(tenantId, datasourceId, startTime, endTime)
	if err != nil {
		return nil, fmt.Errorf("查询历史快照失败: %w", err)
	}

	// 转换为时间线格式
	timeline := make([]map[string]interface{}, 0, len(snapshots))
	for _, snapshot := range snapshots {
		timeline = append(timeline, map[string]interface{}{
			"time":             snapshot.SnapshotTime,
			"totalCount":       snapshot.TotalCount,
			"upCount":          snapshot.UpCount,
			"downCount":        snapshot.DownCount,
			"availabilityRate": snapshot.AvailabilityRate,
		})
	}

	return map[string]interface{}{
		"timeline": timeline,
	}, nil
}

// SendReport 发送巡检报告 (手动触发或定时任务调用)
func (s *exporterMonitorService) SendReport(tenantId string, noticeGroups []string, reportFormat string) error {
	// 1. 获取当前状态
	realtimeData, err := s.GetRealtimeStatus(tenantId, "", "", "", "")
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

	// 2. 获取近 7 日趋势
	endTime := time.Now()
	startTime := endTime.AddDate(0, 0, -7)
	historyData, err := s.GetHistory(tenantId, "", startTime, endTime)
	if err != nil {
		logc.Errorf(s.ctx.Ctx, "获取历史趋势失败: %v", err)
		// 历史数据获取失败不阻塞报告发送
	}

	// 3. 生成报告内容
	content := s.generateReportContent(summary, exporters, historyData, reportFormat)

	// 4. 调用 Sender 推送通知
	return s.sendToNoticeGroups(tenantId, noticeGroups, content)
}

// generateReportContent 生成报告内容 (支持 Markdown 和飞书卡片格式)
func (s *exporterMonitorService) generateReportContent(
	summary models.ExporterStatusSummary,
	exporters []models.ExporterStatus,
	historyData interface{},
	reportFormat string,
) string {
	// 当前时间
	now := time.Now().Format("2006-01-02 15:04:05")

	// 构建报告标题
	content := fmt.Sprintf("## 📊 Exporter 健康巡检报告\n\n")
	content += fmt.Sprintf("**巡检时间**: %s\n\n", now)

	// 统计摘要
	content += fmt.Sprintf("### 📈 总体统计\n\n")
	content += fmt.Sprintf("- **总数**: %d\n", summary.TotalCount)
	content += fmt.Sprintf("- **✅ 正常**: %d\n", summary.UpCount)
	content += fmt.Sprintf("- **❌ 异常**: %d\n", summary.DownCount)
	content += fmt.Sprintf("- **❓ 未知**: %d\n", summary.UnknownCount)
	content += fmt.Sprintf("- **可用率**: %.2f%%\n\n", summary.AvailabilityRate)

	// DOWN 列表 (简洁版和详细版都显示)
	downCount := 0
	downList := make([]models.ExporterStatus, 0)
	for _, exp := range exporters {
		if exp.Status == "down" {
			downCount++
			downList = append(downList, exp)
		}
	}

	if downCount > 0 {
		content += fmt.Sprintf("### ⚠️ 异常 Exporter 列表 (%d)\n\n", downCount)
		for i, exp := range downList {
			content += fmt.Sprintf("%d. **%s** (%s)\n", i+1, exp.Instance, exp.Job)
			content += fmt.Sprintf("   - 采集地址: %s\n", exp.ScrapeUrl)
			if exp.LastError != "" {
				content += fmt.Sprintf("   - 错误信息: %s\n", exp.LastError)
			}
			content += fmt.Sprintf("   - 最后采集时间: %s\n\n", exp.LastScrapeTime.Format("2006-01-02 15:04:05"))
		}
	} else {
		content += fmt.Sprintf("### ✅ 所有 Exporter 运行正常\n\n")
	}

	// 详细版: 显示所有 Exporter 状态
	if reportFormat == "detailed" {
		content += fmt.Sprintf("### 📋 所有 Exporter 状态\n\n")
		upList := make([]models.ExporterStatus, 0)
		unknownList := make([]models.ExporterStatus, 0)

		for _, exp := range exporters {
			if exp.Status == "up" {
				upList = append(upList, exp)
			} else if exp.Status == "unknown" {
				unknownList = append(unknownList, exp)
			}
		}

		// 正常列表
		if len(upList) > 0 {
			content += fmt.Sprintf("#### ✅ 正常 (%d)\n\n", len(upList))
			for i, exp := range upList {
				content += fmt.Sprintf("%d. %s (%s)\n", i+1, exp.Instance, exp.Job)
			}
			content += "\n"
		}

		// 未知状态列表
		if len(unknownList) > 0 {
			content += fmt.Sprintf("#### ❓ 未知状态 (%d)\n\n", len(unknownList))
			for i, exp := range unknownList {
				content += fmt.Sprintf("%d. %s (%s)\n", i+1, exp.Instance, exp.Job)
			}
			content += "\n"
		}
	}

	// 历史趋势 (如果有数据)
	if historyData != nil {
		historyMap, ok := historyData.(map[string]interface{})
		if ok {
			timeline, ok := historyMap["timeline"].([]map[string]interface{})
			if ok && len(timeline) > 0 {
				content += fmt.Sprintf("### 📉 近 7 日趋势\n\n")
				content += "| 时间 | 总数 | 正常 | 异常 | 可用率 |\n"
				content += "|------|------|------|------|--------|\n"

				// 只显示最近几天的数据 (最多5条)
				displayCount := len(timeline)
				if displayCount > 5 {
					displayCount = 5
				}

				for i := 0; i < displayCount; i++ {
					record := timeline[i]
					timeStr, _ := record["time"].(time.Time)
					totalCount, _ := record["totalCount"].(int)
					upCount, _ := record["upCount"].(int)
					downCount, _ := record["downCount"].(int)
					availabilityRate, _ := record["availabilityRate"].(float64)

					content += fmt.Sprintf("| %s | %d | %d | %d | %.2f%% |\n",
						timeStr.Format("01-02 15:04"),
						totalCount,
						upCount,
						downCount,
						availabilityRate,
					)
				}
				content += "\n"
			}
		}
	}

	content += fmt.Sprintf("---\n\n")
	content += fmt.Sprintf("*本报告由 WatchAlert Exporter 健康巡检系统自动生成*\n")

	return content
}

// sendToNoticeGroups 向通知组发送报告
func (s *exporterMonitorService) sendToNoticeGroups(tenantId string, noticeGroups []string, content string) error {
	if len(noticeGroups) == 0 {
		return fmt.Errorf("通知组列表为空")
	}

	// 构造 JSON 格式的消息内容 (支持飞书/钉钉/企微等多种通知类型)
	msgContent := map[string]interface{}{
		"msg_type": "text",
		"content": map[string]interface{}{
			"text": content,
		},
	}

	msgBytes, err := sonic.Marshal(msgContent)
	if err != nil {
		return fmt.Errorf("序列化消息内容失败: %w", err)
	}

	// 记录发送失败的通知组
	var failedGroups []string
	successCount := 0

	// 遍历通知组并发送
	for _, groupId := range noticeGroups {
		// 获取通知对象详情
		notice, err := s.ctx.DB.Notice().Get(tenantId, groupId)
		if err != nil {
			logc.Errorf(s.ctx.Ctx, "获取通知对象失败: groupId=%s, err=%v", groupId, err)
			failedGroups = append(failedGroups, groupId)
			continue
		}

		// 构造发送参数
		sendParams := sender.SendParams{
			TenantId:    tenantId,
			EventId:     "exporter-report-" + time.Now().Format("20060102150405"),
			RuleName:    "Exporter 健康巡检报告",
			Severity:    "info",
			NoticeType:  notice.NoticeType,
			NoticeId:    notice.Uuid,
			NoticeName:  notice.Name,
			IsRecovered: false,
			Hook:        notice.DefaultHook,
			Email:       notice.Email,
			Content:     string(msgBytes),
			PhoneNumber: notice.PhoneNumber,
			Sign:        notice.DefaultSign,
		}

		// 发送通知
		err = sender.Sender(s.ctx, sendParams)
		if err != nil {
			logc.Errorf(s.ctx.Ctx, "发送巡检报告失败: notice=%s, err=%v", notice.Name, err)
			failedGroups = append(failedGroups, groupId)
			continue
		}

		successCount++
		logc.Infof(s.ctx.Ctx, "巡检报告发送成功: notice=%s", notice.Name)
	}

	// 返回结果
	if len(failedGroups) > 0 {
		return fmt.Errorf("发送完成: 成功 %d/%d, 失败的通知组: %v", successCount, len(noticeGroups), failedGroups)
	}

	logc.Infof(s.ctx.Ctx, "巡检报告发送完成: 成功 %d/%d", successCount, len(noticeGroups))
	return nil
}
