package repo

import (
	"alertHub/internal/models"

	"gorm.io/gorm"
)

type (
	ProcessTraceRepo interface {
		// 创建处理流程追踪记录
		Create(processTrace *models.ProcessTrace) error

		// 根据事件ID获取处理流程追踪记录
		GetByEventId(tenantId, eventId string) (*models.ProcessTrace, error)

		// 更新处理流程追踪记录
		Update(processTrace *models.ProcessTrace) error

		// 获取处理流程列表（支持多种筛选条件）
		GetList(tenantId string, page, pageSize int, status string) ([]models.ProcessTrace, int64, error)

		// 获取处理流程列表（支持 eventId 和 faultCenterId 筛选）
		GetListWithFilters(tenantId, eventId, faultCenterId string, page, pageSize int) ([]models.ProcessTrace, int64, error)

		// 获取处理流程列表（包含规则名称，优化版本）
		GetListWithRuleNames(tenantId, eventId, faultCenterId string, page, pageSize int) ([]models.ProcessTrace, int64, error)

		// 删除处理流程记录
		Delete(tenantId, processId string) error
	}

	ProcessOperationLogRepo interface {
		// 创建操作日志
		Create(log *models.ProcessOperationLog) error

		// 获取操作日志列表
		GetList(tenantId, eventId string, page, pageSize int) ([]models.ProcessOperationLog, int64, error)

		// 根据流程ID获取操作日志
		GetByProcessId(tenantId, processId string, page, pageSize int) ([]models.ProcessOperationLog, int64, error)
	}

	processTraceRepo struct {
		db *gorm.DB
	}

	processOperationLogRepo struct {
		db *gorm.DB
	}
)

func NewProcessTraceRepo(db *gorm.DB) ProcessTraceRepo {
	return &processTraceRepo{
		db: db,
	}
}

func NewProcessOperationLogRepo(db *gorm.DB) ProcessOperationLogRepo {
	return &processOperationLogRepo{
		db: db,
	}
}

// ProcessTraceRepo 实现

func (r *processTraceRepo) Create(processTrace *models.ProcessTrace) error {
	return r.db.Create(processTrace).Error
}

func (r *processTraceRepo) GetByEventId(tenantId, eventId string) (*models.ProcessTrace, error) {
	var processTrace models.ProcessTrace
	err := r.db.Where("tenant_id = ? AND event_id = ?", tenantId, eventId).First(&processTrace).Error
	return &processTrace, err
}

func (r *processTraceRepo) Update(processTrace *models.ProcessTrace) error {
	return r.db.Save(processTrace).Error
}

func (r *processTraceRepo) GetList(tenantId string, page, pageSize int, status string) ([]models.ProcessTrace, int64, error) {
	var processes []models.ProcessTrace
	var total int64

	db := r.db.Model(&models.ProcessTrace{}).Where("tenant_id = ?", tenantId)

	if status != "" {
		db = db.Where("current_status = ?", status)
	}

	// 获取总数
	err := db.Count(&total).Error
	if err != nil {
		return nil, 0, err
	}

	// 分页查询
	offset := (page - 1) * pageSize
	err = db.Order("created_at DESC").Offset(offset).Limit(pageSize).Find(&processes).Error
	if err != nil {
		return nil, 0, err
	}

	return processes, total, nil
}

func (r *processTraceRepo) GetListWithFilters(tenantId, eventId, faultCenterId string, page, pageSize int) ([]models.ProcessTrace, int64, error) {
	var processes []models.ProcessTrace
	var total int64

	db := r.db.Model(&models.ProcessTrace{}).Where("tenant_id = ?", tenantId)

	// 添加可选的筛选条件
	if eventId != "" {
		db = db.Where("event_id = ?", eventId)
	}
	if faultCenterId != "" {
		db = db.Where("fault_center_id = ?", faultCenterId)
	}

	// 获取总数
	err := db.Count(&total).Error
	if err != nil {
		return nil, 0, err
	}

	// 分页查询
	offset := (page - 1) * pageSize
	err = db.Order("created_at DESC").Offset(offset).Limit(pageSize).Find(&processes).Error
	if err != nil {
		return nil, 0, err
	}

	return processes, total, nil
}

func (r *processTraceRepo) GetListWithRuleNames(tenantId, eventId, faultCenterId string, page, pageSize int) ([]models.ProcessTrace, int64, error) {
	var processes []models.ProcessTrace
	var total int64

	// 构建基础查询，使用正确的表名，尝试从当前事件表和历史事件表获取规则名称
	baseQuery := r.db.Table("process_trace pt").
		Select("pt.*, COALESCE(ace.rule_name, ahe.rule_name) as rule_name").
		Joins("LEFT JOIN alert_cur_events ace ON pt.tenant_id = ace.tenant_id AND pt.event_id = ace.event_id").
		Joins("LEFT JOIN alert_his_events ahe ON pt.tenant_id = ahe.tenant_id AND pt.event_id = ahe.event_id").
		Where("pt.tenant_id = ?", tenantId)

	// 添加可选的筛选条件
	if eventId != "" {
		baseQuery = baseQuery.Where("pt.event_id = ?", eventId)
	}
	if faultCenterId != "" {
		baseQuery = baseQuery.Where("pt.fault_center_id = ?", faultCenterId)
	}

	// 获取总数（使用子查询避免 JOIN 影响 COUNT）
	countQuery := r.db.Model(&models.ProcessTrace{}).Where("tenant_id = ?", tenantId)
	if eventId != "" {
		countQuery = countQuery.Where("event_id = ?", eventId)
	}
	if faultCenterId != "" {
		countQuery = countQuery.Where("fault_center_id = ?", faultCenterId)
	}
	
	err := countQuery.Count(&total).Error
	if err != nil {
		return nil, 0, err
	}

	// 分页查询
	offset := (page - 1) * pageSize
	err = baseQuery.Order("pt.created_at DESC").Offset(offset).Limit(pageSize).Scan(&processes).Error
	if err != nil {
		return nil, 0, err
	}

	return processes, total, nil
}

func (r *processTraceRepo) Delete(tenantId, processId string) error {
	return r.db.Where("tenant_id = ? AND id = ?", tenantId, processId).Delete(&models.ProcessTrace{}).Error
}

// ProcessOperationLogRepo 实现

func (r *processOperationLogRepo) Create(log *models.ProcessOperationLog) error {
	return r.db.Create(log).Error
}

func (r *processOperationLogRepo) GetList(tenantId, eventId string, page, pageSize int) ([]models.ProcessOperationLog, int64, error) {
	var logs []models.ProcessOperationLog
	var total int64

	db := r.db.Model(&models.ProcessOperationLog{}).Where("tenant_id = ? AND event_id = ?", tenantId, eventId)

	// 获取总数
	err := db.Count(&total).Error
	if err != nil {
		return nil, 0, err
	}

	// 分页查询
	offset := (page - 1) * pageSize
	err = db.Order("operation_time DESC").Offset(offset).Limit(pageSize).Find(&logs).Error
	if err != nil {
		return nil, 0, err
	}

	return logs, total, nil
}

func (r *processOperationLogRepo) GetByProcessId(tenantId, processId string, page, pageSize int) ([]models.ProcessOperationLog, int64, error) {
	var logs []models.ProcessOperationLog
	var total int64

	db := r.db.Model(&models.ProcessOperationLog{}).Where("tenant_id = ? AND process_id = ?", tenantId, processId)

	// 获取总数
	err := db.Count(&total).Error
	if err != nil {
		return nil, 0, err
	}

	// 分页查询
	offset := (page - 1) * pageSize
	err = db.Order("operation_time DESC").Offset(offset).Limit(pageSize).Find(&logs).Error
	if err != nil {
		return nil, 0, err
	}

	return logs, total, nil
}
