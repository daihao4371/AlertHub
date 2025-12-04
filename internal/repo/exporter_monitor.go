package repo

import (
	"time"
	"watchAlert/internal/models"

	"gorm.io/gorm"
)

type (
	exporterMonitorRepo struct {
		entryRepo
	}

	InterExporterMonitorRepo interface {
		// Config 相关
		GetConfig(tenantId string) (models.ExporterMonitorConfig, error)
		SaveConfig(config models.ExporterMonitorConfig) error

		// Schedule 相关
		GetSchedule(tenantId string) (models.ExporterReportSchedule, error)
		SaveSchedule(schedule models.ExporterReportSchedule) error

		// Snapshot 相关
		CreateSnapshot(snapshot models.ExporterMonitorSnapshot) error
		GetSnapshotsByTimeRange(tenantId, datasourceId string, startTime, endTime time.Time) ([]models.ExporterMonitorSnapshot, error)
		DeleteExpiredSnapshots(tenantId string, retentionDays int) error
	}
)

func newExporterMonitorInterface(db *gorm.DB, g InterGormDBCli) InterExporterMonitorRepo {
	return exporterMonitorRepo{
		entryRepo{
			db: db,
			g:  g,
		},
	}
}

// GetConfig 获取租户的监控配置
func (r exporterMonitorRepo) GetConfig(tenantId string) (models.ExporterMonitorConfig, error) {
	var config models.ExporterMonitorConfig
	err := r.db.Model(&models.ExporterMonitorConfig{}).
		Where("tenant_id = ?", tenantId).
		First(&config).Error

	if err != nil {
		// 如果记录不存在,返回默认配置
		if err == gorm.ErrRecordNotFound {
			enabled := true
			autoRefresh := true
			return models.ExporterMonitorConfig{
				TenantId:         tenantId,
				Enabled:          &enabled,
				AutoRefresh:      &autoRefresh,
				DatasourceIds:    []string{},
				RefreshInterval:  30,
				SnapshotInterval: 5,
				HistoryRetention: 30,
			}, nil
		}
		return config, err
	}
	return config, nil
}

// SaveConfig 保存监控配置 (创建或更新)
func (r exporterMonitorRepo) SaveConfig(config models.ExporterMonitorConfig) error {
	// 尝试查询是否已存在
	var existing models.ExporterMonitorConfig
	err := r.db.Model(&models.ExporterMonitorConfig{}).
		Where("tenant_id = ?", config.TenantId).
		First(&existing).Error

	if err == gorm.ErrRecordNotFound {
		// 不存在,创建
		config.CreatedAt = time.Now()
		config.UpdatedAt = time.Now()
		return r.g.Create(&models.ExporterMonitorConfig{}, &config)
	}

	// 已存在,更新
	config.UpdatedAt = time.Now()
	return r.g.Updates(Updates{
		Table: models.ExporterMonitorConfig{},
		Where: map[string]interface{}{
			"tenant_id = ?": config.TenantId,
		},
		Updates: config,
	})
}

// GetSchedule 获取租户的推送配置
func (r exporterMonitorRepo) GetSchedule(tenantId string) (models.ExporterReportSchedule, error) {
	var schedule models.ExporterReportSchedule
	err := r.db.Model(&models.ExporterReportSchedule{}).
		Where("tenant_id = ?", tenantId).
		First(&schedule).Error

	if err != nil {
		// 如果记录不存在,返回默认配置
		if err == gorm.ErrRecordNotFound {
			enabled := false
			return models.ExporterReportSchedule{
				TenantId:       tenantId,
				Enabled:        &enabled,
				CronExpression: []string{},
				NoticeGroups:   []string{},
				ReportFormat:   "simple",
			}, nil
		}
		return schedule, err
	}
	return schedule, nil
}

// SaveSchedule 保存推送配置 (创建或更新)
func (r exporterMonitorRepo) SaveSchedule(schedule models.ExporterReportSchedule) error {
	// 尝试查询是否已存在
	var existing models.ExporterReportSchedule
	err := r.db.Model(&models.ExporterReportSchedule{}).
		Where("tenant_id = ?", schedule.TenantId).
		First(&existing).Error

	if err == gorm.ErrRecordNotFound {
		// 不存在,创建
		schedule.CreatedAt = time.Now()
		schedule.UpdatedAt = time.Now()
		return r.g.Create(&models.ExporterReportSchedule{}, &schedule)
	}

	// 已存在,更新
	schedule.UpdatedAt = time.Now()
	return r.g.Updates(Updates{
		Table: models.ExporterReportSchedule{},
		Where: map[string]interface{}{
			"tenant_id = ?": schedule.TenantId,
		},
		Updates: schedule,
	})
}

// CreateSnapshot 创建历史快照
func (r exporterMonitorRepo) CreateSnapshot(snapshot models.ExporterMonitorSnapshot) error {
	snapshot.CreatedAt = time.Now()
	return r.g.Create(&models.ExporterMonitorSnapshot{}, &snapshot)
}

// GetSnapshotsByTimeRange 查询历史快照列表 (别名方法,与接口保持一致)
func (r exporterMonitorRepo) GetSnapshotsByTimeRange(tenantId, datasourceId string, startTime, endTime time.Time) ([]models.ExporterMonitorSnapshot, error) {
	var snapshots []models.ExporterMonitorSnapshot

	query := r.db.Model(&models.ExporterMonitorSnapshot{}).
		Where("tenant_id = ?", tenantId)

	// 如果指定数据源,添加过滤条件
	if datasourceId != "" {
		query = query.Where("datasource_id = ?", datasourceId)
	}

	// 时间范围过滤
	query = query.Where("snapshot_time BETWEEN ? AND ?", startTime, endTime)

	// 按时间倒序
	query = query.Order("snapshot_time DESC")

	err := query.Find(&snapshots).Error
	if err != nil {
		return nil, err
	}

	return snapshots, nil
}

// DeleteExpiredSnapshots 删除过期的快照数据
func (r exporterMonitorRepo) DeleteExpiredSnapshots(tenantId string, retentionDays int) error {
	// 计算过期时间点
	expiredTime := time.Now().AddDate(0, 0, -retentionDays)

	err := r.db.Where("tenant_id = ? AND snapshot_time < ?", tenantId, expiredTime).
		Delete(&models.ExporterMonitorSnapshot{}).Error

	return err
}
