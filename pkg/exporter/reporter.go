package exporter

import (
	"alertHub/internal/models"
	"fmt"
	"time"
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
	content := "## 📊 Exporter 健康巡检报告\n\n"
	content += fmt.Sprintf("**巡检时间**: %s\n\n", now)

	// 统计摘要 - 使用更清晰的格式
	content += "### 📈 总体统计\n\n"

	// 根据状态使用不同的表情符号和格式
	statusIcon := "✅"
	if summary.DownCount > 0 {
		statusIcon = "⚠️"
	}
	if summary.UnknownCount > 0 && summary.DownCount == 0 {
		statusIcon = "❓"
	}

	content += fmt.Sprintf("%s **状态**: ", statusIcon)
	if summary.DownCount == 0 && summary.UnknownCount == 0 {
		content += "全部正常"
	} else if summary.DownCount > 0 {
		content += fmt.Sprintf("发现 %d 个异常", summary.DownCount)
	} else {
		content += fmt.Sprintf("发现 %d 个未知状态", summary.UnknownCount)
	}
	content += "\n\n"

	// 使用表格格式展示统计信息，更清晰
	content += "| 指标 | 数值 |\n"
	content += "|------|------|\n"
	content += fmt.Sprintf("| 📊 总数 | **%d** |\n", summary.TotalCount)
	content += fmt.Sprintf("| ✅ 正常 | <font color='green'>**%d**</font> |\n", summary.UpCount)
	content += fmt.Sprintf("| ❌ 异常 | <font color='red'>**%d**</font> |\n", summary.DownCount)
	if summary.UnknownCount > 0 {
		content += fmt.Sprintf("| ❓ 未知 | <font color='orange'>**%d**</font> |\n", summary.UnknownCount)
	}
	content += fmt.Sprintf("| 📈 可用率 | <font color='blue'>**%.2f%%**</font> |\n\n", summary.AvailabilityRate)

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

		// 使用表格格式展示异常列表，更清晰易读
		content += "| # | 实例名称 | Job | 数据源 | 采集地址 | 最后采集时间 |\n"
		content += "|---|---------|-----|--------|----------|-------------|\n"

		for i, exp := range downList {
			instanceName := exp.Instance
			if len(instanceName) > 20 {
				instanceName = instanceName[:17] + "..."
			}
			scrapeUrl := exp.ScrapeUrl
			if len(scrapeUrl) > 30 {
				scrapeUrl = scrapeUrl[:27] + "..."
			}

			content += fmt.Sprintf("| %d | **%s** | `%s` | %s | `%s` | %s |\n",
				i+1,
				instanceName,
				exp.Job,
				exp.DatasourceName,
				scrapeUrl,
				exp.LastScrapeTime.Format("01-02 15:04"),
			)
		}
		content += "\n"

		// 详细错误信息单独列出
		hasErrors := false
		for _, exp := range downList {
			if exp.LastError != "" {
				if !hasErrors {
					content += "#### 🔍 错误详情\n\n"
					hasErrors = true
				}
				content += fmt.Sprintf("**%s** (`%s`):\n", exp.Instance, exp.Job)
				content += fmt.Sprintf("```\n%s\n```\n\n", exp.LastError)
			}
		}
	} else {
		content += "### ✅ 所有 Exporter 运行正常\n\n"
		content += "🎉 本次巡检未发现任何异常，所有 Exporter 均正常运行。\n\n"
	}

	// 详细版: 显示异常详情（只显示有问题的 Exporter）
	if reportFormat == "detailed" {
		unknownList := make([]models.ExporterStatus, 0)

		// 收集未知状态的 Exporter
		for _, exp := range exporters {
			if exp.Status == "unknown" {
				unknownList = append(unknownList, exp)
			}
		}

		// 只在有未知状态时显示该段落
		if len(unknownList) > 0 {
			content += "### 📋 异常详情\n\n"
			content += fmt.Sprintf("#### ❓ 未知状态 (%d)\n\n", len(unknownList))
			content += "| # | 实例名称 | Job | 数据源 | 采集地址 | 最后采集时间 |\n"
			content += "|---|---------|-----|--------|----------|-------------|\n"
			for i, exp := range unknownList {
				instanceName := exp.Instance
				if len(instanceName) > 20 {
					instanceName = instanceName[:17] + "..."
				}
				scrapeUrl := exp.ScrapeUrl
				if len(scrapeUrl) > 30 {
					scrapeUrl = scrapeUrl[:27] + "..."
				}
				content += fmt.Sprintf("| %d | %s | `%s` | %s | `%s` | %s |\n",
					i+1, instanceName, exp.Job, exp.DatasourceName, scrapeUrl, exp.LastScrapeTime.Format("01-02 15:04"))
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
				content += "### 📉 近 7 日趋势\n\n"
				content += "| 时间 | 数据源 | 总数 | 正常 | 异常 | 可用率 |\n"
				content += "|:----:|:------:|:----:|:----:|:----:|:------:|\n"

				// 只显示最近几天的数据 (最多10条)
				displayCount := len(timeline)
				if displayCount > 10 {
					displayCount = 10
				}

				for i := 0; i < displayCount; i++ {
					record := timeline[i]
					timeStr, ok := record["time"].(time.Time)
					if !ok {
						// 尝试解析字符串格式的时间
						if timeStrStr, ok := record["time"].(string); ok {
							if parsedTime, err := time.Parse("2006-01-02 15:04:05", timeStrStr); err == nil {
								timeStr = parsedTime
							}
						}
					}
					datasourceName, _ := record["datasourceName"].(string)
					totalCount, _ := record["totalCount"].(int)
					upCount, _ := record["upCount"].(int)
					downCount, _ := record["downCount"].(int)
					availabilityRate, _ := record["availabilityRate"].(float64)

					// 根据可用率设置颜色
					rateColor := "green"
					if availabilityRate < 80 {
						rateColor = "red"
					} else if availabilityRate < 95 {
						rateColor = "orange"
					}

					content += fmt.Sprintf("| %s | %s | %d | <font color='green'>%d</font> | <font color='red'>%d</font> | <font color='%s'>**%.2f%%**</font> |\n",
						timeStr.Format("01-02 15:04"),
						datasourceName,
						totalCount,
						upCount,
						downCount,
						rateColor,
						availabilityRate,
					)
				}
				content += "\n"
			}
		}
	}

	content += "---\n\n"
	content += "*本报告由 AlertHub Exporter 健康巡检系统自动生成*\n"

	return content
}
