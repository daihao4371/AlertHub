package exporter

import (
	"fmt"
	"time"
	"watchAlert/internal/models"
)

// Reporter 报告生成器 - 负责生成 Exporter 健康巡检报告
type Reporter struct{}

// NewReporter 创建报告生成器实例
func NewReporter() *Reporter {
	return &Reporter{}
}

// GenerateReportContent 生成报告内容 (支持 Markdown 格式)
func (r *Reporter) GenerateReportContent(
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

	// DOWN 列表
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
			content += fmt.Sprintf("   - 数据源: %s\n", exp.DatasourceName)
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
				content += fmt.Sprintf("%d. %s (%s) - %s\n", i+1, exp.Instance, exp.Job, exp.DatasourceName)
			}
			content += "\n"
		}

		// 未知状态列表
		if len(unknownList) > 0 {
			content += fmt.Sprintf("#### ❓ 未知状态 (%d)\n\n", len(unknownList))
			for i, exp := range unknownList {
				content += fmt.Sprintf("%d. %s (%s) - %s\n", i+1, exp.Instance, exp.Job, exp.DatasourceName)
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
				content += "| 时间 | 数据源 | 总数 | 正常 | 异常 | 可用率 |\n"
				content += "|------|--------|------|------|------|--------|\n"

				// 只显示最近几天的数据 (最多10条)
				displayCount := len(timeline)
				if displayCount > 10 {
					displayCount = 10
				}

				for i := 0; i < displayCount; i++ {
					record := timeline[i]
					timeStr, _ := record["time"].(time.Time)
					datasourceName, _ := record["datasourceName"].(string)
					totalCount, _ := record["totalCount"].(int)
					upCount, _ := record["upCount"].(int)
					downCount, _ := record["downCount"].(int)
					availabilityRate, _ := record["availabilityRate"].(float64)

					content += fmt.Sprintf("| %s | %s | %d | %d | %d | %.2f%% |\n",
						timeStr.Format("01-02 15:04"),
						datasourceName,
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