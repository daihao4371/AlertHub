package models

import "time"

// ExporterMonitorConfig Exporter 监控配置表
type ExporterMonitorConfig struct {
	ID               int64     `gorm:"column:id;primary_key;AUTO_INCREMENT" json:"id"`
	TenantId         string    `gorm:"column:tenant_id;type:varchar(64);not null;uniqueIndex:uk_tenant" json:"tenantId"`
	Enabled          *bool     `gorm:"column:enabled;type:tinyint(1);default:1" json:"enabled"`              // 是否启用
	DatasourceIds    []string  `gorm:"column:datasource_ids;serializer:json" json:"datasourceIds"`           // 监控的数据源ID列表
	RefreshInterval  int       `gorm:"column:refresh_interval;type:int;default:30" json:"refreshInterval"`   // 页面刷新间隔(秒)
	SnapshotInterval int       `gorm:"column:snapshot_interval;type:int;default:5" json:"snapshotInterval"`  // 快照间隔(分钟)
	HistoryRetention int       `gorm:"column:history_retention;type:int;default:30" json:"historyRetention"` // 历史保留天数
	AutoRefresh      *bool     `gorm:"column:auto_refresh;type:tinyint(1);default:1" json:"autoRefresh"`     // 是否自动刷新
	CreatedAt        time.Time `gorm:"column:created_at;type:datetime;default:CURRENT_TIMESTAMP" json:"createdAt"`
	UpdatedAt        time.Time `gorm:"column:updated_at;type:datetime;default:CURRENT_TIMESTAMP" json:"updatedAt"`
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

// ExporterMonitorSnapshot Exporter 巡检历史快照表
type ExporterMonitorSnapshot struct {
	ID               int64                    `gorm:"column:id;primary_key;AUTO_INCREMENT" json:"id"`
	TenantId         string                   `gorm:"column:tenant_id;type:varchar(64);not null;index:idx_tenant_time" json:"tenantId"`
	DatasourceId     string                   `gorm:"column:datasource_id;type:varchar(64);not null;index:idx_datasource_time" json:"datasourceId"`
	SnapshotTime     time.Time                `gorm:"column:snapshot_time;type:datetime;not null" json:"snapshotTime"`             // 快照时间
	TotalCount       int                      `gorm:"column:total_count;type:int;not null" json:"totalCount"`                      // Exporter 总数
	UpCount          int                      `gorm:"column:up_count;type:int;not null" json:"upCount"`                            // UP 状态数量
	DownCount        int                      `gorm:"column:down_count;type:int;not null" json:"downCount"`                        // DOWN 状态数量
	UnknownCount     int                      `gorm:"column:unknown_count;type:int;not null" json:"unknownCount"`                  // UNKNOWN 状态数量
	AvailabilityRate float64                  `gorm:"column:availability_rate;type:decimal(5,2);not null" json:"availabilityRate"` // 可用率(%)
	DownList         []map[string]interface{} `gorm:"column:down_list;serializer:json" json:"downList"`                            // DOWN 状态的实例列表
	CreatedAt        time.Time                `gorm:"column:created_at;type:datetime;default:CURRENT_TIMESTAMP" json:"createdAt"`
}

// TableName 指定表名
func (ExporterMonitorSnapshot) TableName() string {
	return "w8t_exporter_monitor_snapshot"
}

// ExporterStatus Exporter 实时状态 (Redis 缓存结构,不存数据库)
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
