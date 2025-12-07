package types

// RequestExporterMonitorStatus 请求查询 Exporter 监控状态
type RequestExporterMonitorStatus struct {
	DatasourceId string `form:"datasourceId"`
	Status       string `form:"status"`  // up/down/unknown/all
	Job          string `form:"job"`     // Job 筛选
	Keyword      string `form:"keyword"` // 关键词搜索
}

// RequestExporterMonitorHistory 请求查询 Exporter 监控历史
type RequestExporterMonitorHistory struct {
	DatasourceId string `form:"datasourceId"`
	StartTime    string `form:"startTime" binding:"required"` // RFC3339 格式
	EndTime      string `form:"endTime" binding:"required"`   // RFC3339 格式
}

// RequestExporterMonitorSendReport 请求手动触发报告推送
type RequestExporterMonitorSendReport struct {
	NoticeGroups []string `json:"noticeGroups" binding:"required"` // 通知组 UUID 列表
	ReportFormat string   `json:"reportFormat"`                    // simple/detailed
}

// RequestExporterMonitorInspect 请求手动触发巡检
type RequestExporterMonitorInspect struct {
	DatasourceId string `json:"datasourceId"` // 可选,为空则巡检所有数据源
}