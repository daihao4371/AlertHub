package models

import "time"

// ExporterMonitorConfig Exporter 监控配置表
type ExporterMonitorConfig struct {
	ID               int64     `gorm:"column:id;primary_key;AUTO_INCREMENT" json:"id"`
	TenantId         string    `gorm:"column:tenant_id;type:varchar(64);not null;uniqueIndex:uk_tenant" json:"tenantId"`
	Enabled          *bool     `gorm:"column:enabled;type:tinyint(1);default:1" json:"enabled"`                       // 是否启用巡检
	DatasourceIds    []string  `gorm:"column:datasource_ids;serializer:json" json:"datasourceIds"`                    // 监控的数据源ID列表
	InspectionTimes  []string  `gorm:"column:inspection_times;serializer:json" json:"inspectionTimes"`                // 巡检时间配置 (如: ["09:00", "21:00"])
	HistoryRetention int       `gorm:"column:history_retention;type:int;default:90" json:"historyRetention"`          // 历史保留天数,默认90天
	AutoRefresh      *bool     `gorm:"column:auto_refresh;type:tinyint(1);default:0" json:"autoRefresh"`              // 前端自动刷新开关
	CreatedAt        time.Time `gorm:"column:created_at;type:datetime;default:CURRENT_TIMESTAMP" json:"createdAt"`
	UpdatedAt        time.Time `gorm:"column:updated_at;type:datetime;default:CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP" json:"updatedAt"`
}

// TableName 指定表名
func (ExporterMonitorConfig) TableName() string {
	return "w8t_exporter_monitor_config"
}

// GetEnabled 安全获取 Enabled 字段 (防止空指针)
func (c *ExporterMonitorConfig) GetEnabled() bool {
	if c.Enabled == nil {
		return false
	}
	return *c.Enabled
}

// GetAutoRefresh 安全获取 AutoRefresh 字段 (防止空指针)
func (c *ExporterMonitorConfig) GetAutoRefresh() bool {
	if c.AutoRefresh == nil {
		return false
	}
	return *c.AutoRefresh
}

// ExporterReportSchedule Exporter 报告推送配置表
type ExporterReportSchedule struct {
	ID             int64     `gorm:"column:id;primary_key;AUTO_INCREMENT" json:"id"`
	TenantId       string    `gorm:"column:tenant_id;type:varchar(64);not null;uniqueIndex:uk_tenant" json:"tenantId"`
	Enabled        *bool     `gorm:"column:enabled;type:tinyint(1);default:1" json:"enabled"`                    // 是否启用
	CronExpression []string  `gorm:"column:cron_expression;serializer:json" json:"cronExpression"`               // Cron表达式数组
	NoticeGroups   []string  `gorm:"column:notice_groups;serializer:json" json:"noticeGroups"`                   // 通知组ID数组
	ReportFormat   string    `gorm:"column:report_format;type:varchar(20);default:'simple'" json:"reportFormat"` // 报告格式: simple/detailed
	CreatedAt      time.Time `gorm:"column:created_at;type:datetime;default:CURRENT_TIMESTAMP" json:"createdAt"`
	UpdatedAt      time.Time `gorm:"column:updated_at;type:datetime;default:CURRENT_TIMESTAMP" json:"updatedAt"`
}

// TableName 指定表名
func (ExporterReportSchedule) TableName() string {
	return "w8t_exporter_report_schedule"
}

// GetEnabled 安全获取 Enabled 字段
func (s *ExporterReportSchedule) GetEnabled() bool {
	if s.Enabled == nil {
		return false
	}
	return *s.Enabled
}

// ExporterInspection Exporter 巡检记录主表
type ExporterInspection struct {
	ID               int64                    `gorm:"column:id;primary_key;AUTO_INCREMENT" json:"id"`
	InspectionId     string                   `gorm:"column:inspection_id;type:varchar(64);not null;uniqueIndex:uk_inspection" json:"inspectionId"` // 巡检批次ID (UUID)
	TenantId         string                   `gorm:"column:tenant_id;type:varchar(64);not null;index:idx_tenant_time" json:"tenantId"`
	DatasourceId     string                   `gorm:"column:datasource_id;type:varchar(64);not null;index:idx_datasource_time" json:"datasourceId"`
	DatasourceName   string                   `gorm:"column:datasource_name;type:varchar(128)" json:"datasourceName"`                                 // 数据源名称
	InspectionTime   time.Time                `gorm:"column:inspection_time;type:datetime;not null;index:idx_inspection_time" json:"inspectionTime"` // 巡检时间
	TotalCount       int                      `gorm:"column:total_count;type:int;not null;default:0" json:"totalCount"`                              // Exporter 总数
	UpCount          int                      `gorm:"column:up_count;type:int;not null;default:0" json:"upCount"`                                    // UP 状态数量
	DownCount        int                      `gorm:"column:down_count;type:int;not null;default:0" json:"downCount"`                                // DOWN 状态数量
	UnknownCount     int                      `gorm:"column:unknown_count;type:int;not null;default:0" json:"unknownCount"`                          // UNKNOWN 状态数量
	AvailabilityRate float64                  `gorm:"column:availability_rate;type:decimal(5,2);not null;default:0.00" json:"availabilityRate"`      // 可用率(%)
	DownListSummary  []map[string]interface{} `gorm:"column:down_list_summary;serializer:json" json:"downListSummary"`                               // DOWN 状态的实例摘要 (前10个)
	CreatedAt        time.Time                `gorm:"column:created_at;type:datetime;default:CURRENT_TIMESTAMP" json:"createdAt"`
}

// TableName 指定表名
func (ExporterInspection) TableName() string {
	return "w8t_exporter_inspection"
}

// ExporterInspectionDetail Exporter 巡检明细表
type ExporterInspectionDetail struct {
	ID             int64                  `gorm:"column:id;primary_key;AUTO_INCREMENT" json:"id"`
	InspectionId   string                 `gorm:"column:inspection_id;type:varchar(64);not null;index:idx_inspection" json:"inspectionId"` // 关联主表的巡检批次ID
	TenantId       string                 `gorm:"column:tenant_id;type:varchar(64);not null;index:idx_tenant_inspection" json:"tenantId"`
	DatasourceId   string                 `gorm:"column:datasource_id;type:varchar(64);not null" json:"datasourceId"`
	DatasourceName string                 `gorm:"column:datasource_name;type:varchar(128)" json:"datasourceName"` // 数据源名称
	Job            string                 `gorm:"column:job;type:varchar(128);not null;index:idx_job" json:"job"` // Job名称
	Instance       string                 `gorm:"column:instance;type:varchar(256);not null;index:idx_instance" json:"instance"`
	Labels         map[string]interface{} `gorm:"column:labels;serializer:json" json:"labels"`
	ScrapeUrl      string                 `gorm:"column:scrape_url;type:varchar(512)" json:"scrapeUrl"`
	Status         string                 `gorm:"column:status;type:varchar(20);not null;index:idx_status" json:"status"` // up/down/unknown
	LastScrapeTime time.Time              `gorm:"column:last_scrape_time;type:datetime" json:"lastScrapeTime"`
	LastError      string                 `gorm:"column:last_error;type:text" json:"lastError"`
	DownDuration   int64                  `gorm:"column:down_duration;type:bigint;default:0" json:"downDuration"` // DOWN持续时长(秒)
	DownSince      *time.Time             `gorm:"column:down_since;type:datetime" json:"downSince"`               // 首次DOWN的时间
	CreatedAt      time.Time              `gorm:"column:created_at;type:datetime;default:CURRENT_TIMESTAMP" json:"createdAt"`
}

// TableName 指定表名
func (ExporterInspectionDetail) TableName() string {
	return "w8t_exporter_inspection_detail"
}

// ExporterStatus Exporter 状态 (用于 API 返回,不直接存数据库)
type ExporterStatus struct {
	DatasourceId   string                 `json:"datasourceId"`   // 数据源ID
	DatasourceName string                 `json:"datasourceName"` // 数据源名称
	Job            string                 `json:"job"`            // Job名称
	Instance       string                 `json:"instance"`       // 实例名称 (如 192.168.1.100:9100)
	Labels         map[string]interface{} `json:"labels"`         // 标签
	ScrapeUrl      string                 `json:"scrapeUrl"`      // 采集URL
	Status         string                 `json:"status"`         // 状态: up/down/unknown
	LastScrapeTime time.Time              `json:"lastScrapeTime"` // 最后采集时间
	LastError      string                 `json:"lastError"`      // 错误信息
	DownDuration   int64                  `json:"downDuration"`   // DOWN持续时长(秒)
	DownSince      *time.Time             `json:"downSince"`      // 首次DOWN的时间
}

// ExporterStatusSummary Exporter 状态统计
type ExporterStatusSummary struct {
	TotalCount       int       `json:"totalCount"`       // 总数
	UpCount          int       `json:"upCount"`          // UP数量
	DownCount        int       `json:"downCount"`        // DOWN数量
	UnknownCount     int       `json:"unknownCount"`     // UNKNOWN数量
	AvailabilityRate float64   `json:"availabilityRate"` // 可用率(%)
	LastUpdateTime   time.Time `json:"lastUpdateTime"`   // 最后更新时间
}
