package repo

import (
	"time"
	"alertHub/internal/models"

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
		UpdateAutoRefresh(tenantId string, autoRefresh bool) error

		// Schedule 相关
		GetSchedule(tenantId string) (models.ExporterReportSchedule, error)
		SaveSchedule(schedule models.ExporterReportSchedule) error

		// Inspection 相关
		CreateInspection(inspection models.ExporterInspection) error
		CreateInspectionDetails(details []models.ExporterInspectionDetail) error
		GetLatestInspection(tenantId, datasourceId string) (*models.ExporterInspection, error)
		GetInspectionsByTimeRange(tenantId, datasourceId string, startTime, endTime time.Time) ([]models.ExporterInspection, error)
		GetInspectionDetails(inspectionId string, status, job, keyword string) ([]models.ExporterInspectionDetail, error)
		DeleteExpiredInspections(tenantId string, retentionDays int) error
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
			return models.ExporterMonitorConfig{
				TenantId:         tenantId,
				Enabled:          &enabled,
				DatasourceIds:    []string{},
				InspectionTimes:  []string{"09:00", "21:00"},
				HistoryRetention: 90,
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

// UpdateAutoRefresh 更新自动刷新开关状态
func (r exporterMonitorRepo) UpdateAutoRefresh(tenantId string, autoRefresh bool) error {
	// 尝试查询是否已存在配置
	var existing models.ExporterMonitorConfig
	err := r.db.Model(&models.ExporterMonitorConfig{}).
		Where("tenant_id = ?", tenantId).
		First(&existing).Error

	if err == gorm.ErrRecordNotFound {
		// 不存在配置,创建默认配置并设置 autoRefresh
		enabled := true
		config := models.ExporterMonitorConfig{
			TenantId:         tenantId,
			Enabled:          &enabled,
			DatasourceIds:    []string{},
			InspectionTimes:  []string{"09:00", "21:00"},
			HistoryRetention: 90,
			AutoRefresh:      &autoRefresh,
			CreatedAt:        time.Now(),
			UpdatedAt:        time.Now(),
		}
		return r.g.Create(&models.ExporterMonitorConfig{}, &config)
	}

	if err != nil {
		return err
	}

	// 已存在,只更新 auto_refresh 字段
	return r.db.Model(&models.ExporterMonitorConfig{}).
		Where("tenant_id = ?", tenantId).
		Update("auto_refresh", autoRefresh).Error
}

// CreateInspection 创建巡检主记录
func (r exporterMonitorRepo) CreateInspection(inspection models.ExporterInspection) error {
	inspection.CreatedAt = time.Now()
	return r.g.Create(&models.ExporterInspection{}, &inspection)
}

// CreateInspectionDetails 批量创建巡检明细
func (r exporterMonitorRepo) CreateInspectionDetails(details []models.ExporterInspectionDetail) error {
	if len(details) == 0 {
		return nil
	}

	// 批量插入,每批 500 条
	batchSize := 500
	for i := 0; i < len(details); i += batchSize {
		end := i + batchSize
		if end > len(details) {
			end = len(details)
		}

		batch := details[i:end]
		if err := r.db.Create(&batch).Error; err != nil {
			return err
		}
	}

	return nil
}

// GetLatestInspection 获取最新的巡检记录
func (r exporterMonitorRepo) GetLatestInspection(tenantId, datasourceId string) (*models.ExporterInspection, error) {
	var inspection models.ExporterInspection

	query := r.db.Model(&models.ExporterInspection{}).
		Where("tenant_id = ?", tenantId)

	// 如果指定数据源,添加过滤条件
	if datasourceId != "" {
		query = query.Where("datasource_id = ?", datasourceId)
	}

	// 按巡检时间倒序,获取第一条
	err := query.Order("inspection_time DESC").First(&inspection).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}

	return &inspection, nil
}

// GetInspectionsByTimeRange 查询历史巡检记录列表
func (r exporterMonitorRepo) GetInspectionsByTimeRange(tenantId, datasourceId string, startTime, endTime time.Time) ([]models.ExporterInspection, error) {
	var inspections []models.ExporterInspection

	query := r.db.Model(&models.ExporterInspection{}).
		Where("tenant_id = ?", tenantId)

	// 如果指定数据源,添加过滤条件
	if datasourceId != "" {
		query = query.Where("datasource_id = ?", datasourceId)
	}

	// 时间范围过滤
	query = query.Where("inspection_time BETWEEN ? AND ?", startTime, endTime)

	// 按时间倒序
	query = query.Order("inspection_time DESC")

	err := query.Find(&inspections).Error
	if err != nil {
		return nil, err
	}

	return inspections, nil
}

// GetInspectionDetails 查询巡检明细列表 (支持按状态、Job、关键词过滤)
func (r exporterMonitorRepo) GetInspectionDetails(inspectionId string, status, job, keyword string) ([]models.ExporterInspectionDetail, error) {
	var details []models.ExporterInspectionDetail

	query := r.db.Model(&models.ExporterInspectionDetail{}).
		Where("inspection_id = ?", inspectionId)

	// 状态筛选
	if status != "" && status != "all" {
		query = query.Where("status = ?", status)
	}

	// Job 筛选
	if job != "" {
		query = query.Where("job = ?", job)
	}

	// 关键词筛选 (支持 instance 模糊匹配)
	if keyword != "" {
		query = query.Where("instance LIKE ?", "%"+keyword+"%")
	}

	err := query.Find(&details).Error
	if err != nil {
		return nil, err
	}

	return details, nil
}

// DeleteExpiredInspections 删除过期的巡检数据 (主表和明细表级联删除)
func (r exporterMonitorRepo) DeleteExpiredInspections(tenantId string, retentionDays int) error {
	// 计算过期时间点
	expiredTime := time.Now().AddDate(0, 0, -retentionDays)

	// 查询过期的巡检ID列表
	var expiredIds []string
	err := r.db.Model(&models.ExporterInspection{}).
		Select("inspection_id").
		Where("tenant_id = ? AND inspection_time < ?", tenantId, expiredTime).
		Scan(&expiredIds).Error

	if err != nil {
		return err
	}

	if len(expiredIds) == 0 {
		return nil
	}

	// 删除明细表
	err = r.db.Where("inspection_id IN ?", expiredIds).
		Delete(&models.ExporterInspectionDetail{}).Error
	if err != nil {
		return err
	}

	// 删除主表
	err = r.db.Where("tenant_id = ? AND inspection_time < ?", tenantId, expiredTime).
		Delete(&models.ExporterInspection{}).Error

	return err
}
