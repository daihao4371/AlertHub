package repo

import (
	"alertHub/internal/models"
	"context"
	"time"

	"gorm.io/gorm"
)

// InterRCARepo RCA 数据访问接口
// 模式：接口定义分离，便于单元测试和 mock
type InterRCARepo interface {
	// ============== RCASuggestion 相关 ==============

	// GetSuggestionById 按 ID 获取聚类建议
	GetSuggestionById(ctx context.Context, tenantId, suggestionId string) (*models.RCASuggestion, error)

	// GetSuggestionByHash 按输入哈希获取建议（缓存查询）
	// 返回最新的已反馈建议（status = accepted 或 rejected）
	GetSuggestionByHash(ctx context.Context, tenantId, inputHash string) (*models.RCASuggestion, error)

	// CreateSuggestion 创建新的聚类建议
	CreateSuggestion(ctx context.Context, suggestion *models.RCASuggestion) error

	// UpdateSuggestionStatus 更新建议状态和反馈
	UpdateSuggestionStatus(ctx context.Context, tenantId, suggestionId, status string, feedbackScore int, feedbackComment string) error

	// GetIncidentById 按 ID 获取事件
	GetIncidentById(ctx context.Context, tenantId, incidentId string) (*models.RCAIncident, error)

	// GetIncidentsBySuggestion 获取某个建议产生的所有事件
	GetIncidentsBySuggestion(ctx context.Context, tenantId, suggestionId string) ([]*models.RCAIncident, error)

	// ListIncidents 列表查询事件（支持分页和过滤）
	// filters: status, severity, minConfidence
	ListIncidents(ctx context.Context, tenantId string, filters map[string]interface{}, page, pageSize int) ([]*models.RCAIncident, int64, error)

	// CreateIncident 创建单个事件
	CreateIncident(ctx context.Context, incident *models.RCAIncident) error

	// BatchCreateIncidents 批量创建事件
	BatchCreateIncidents(ctx context.Context, incidents []*models.RCAIncident) error

	// UpdateIncidentStatus 更新事件状态
	UpdateIncidentStatus(ctx context.Context, tenantId, incidentId, status string) error

	// DeleteSuggestionAndIncidents 删除建议及其关联的所有事件
	// 用于强制刷新时清理旧数据
	DeleteSuggestionAndIncidents(ctx context.Context, tenantId, suggestionId string) error

	// GetLatestReport 获取最新报告快照
	GetLatestReport(ctx context.Context, tenantId string) (*models.RCAReportSnapshot, error)

	// GetReportByTimeRange 按时间范围查询报告
	GetReportByTimeRange(ctx context.Context, tenantId string, startTime, endTime int64) (*models.RCAReportSnapshot, error)

	// CreateReport 创建报告快照
	CreateReport(ctx context.Context, report *models.RCAReportSnapshot) error
}

// rcaRepository GORM 实现
type rcaRepository struct {
	db *gorm.DB
}

// NewRCARepository 创建 Repository 实例
func NewRCARepository(db *gorm.DB) InterRCARepo {
	return &rcaRepository{db: db}
}

// GetSuggestionById 按 ID 获取建议
func (r *rcaRepository) GetSuggestionById(
	ctx context.Context,
	tenantId, suggestionId string,
) (*models.RCASuggestion, error) {
	var suggestion models.RCASuggestion
	// 对标 AlertHub 模式：第一个条件总是 tenant_id
	err := r.db.WithContext(ctx).
		Where("tenant_id = ? AND id = ?", tenantId, suggestionId).
		First(&suggestion).Error

	if err != nil {
		return nil, err
	}
	return &suggestion, nil
}

// GetSuggestionByHash 按输入哈希获取建议（用于去重和缓存）
// 注意：此方法返回任何状态的建议，包括 pending、accepted、rejected
// 这确保了相同输入的告警不会重复调用 AI，避免数据库唯一约束冲突
func (r *rcaRepository) GetSuggestionByHash(
	ctx context.Context,
	tenantId, inputHash string,
) (*models.RCASuggestion, error) {
	var suggestion models.RCASuggestion
	err := r.db.WithContext(ctx).
		Where("tenant_id = ? AND input_hash = ?",
			tenantId,
			inputHash).
		Order("created_at DESC").
		First(&suggestion).Error

	if err != nil {
		return nil, err
	}
	return &suggestion, nil
}

// CreateSuggestion 创建聚类建议
func (r *rcaRepository) CreateSuggestion(
	ctx context.Context,
	suggestion *models.RCASuggestion,
) error {
	return r.db.WithContext(ctx).Create(suggestion).Error
}

// UpdateSuggestionStatus 更新建议状态
func (r *rcaRepository) UpdateSuggestionStatus(
	ctx context.Context,
	tenantId, suggestionId, status string,
	feedbackScore int,
	feedbackComment string,
) error {
	return r.db.WithContext(ctx).
		Model(&models.RCASuggestion{}).
		Where("tenant_id = ? AND id = ?", tenantId, suggestionId).
		Updates(map[string]interface{}{
			"status":           status,
			"feedback_score":   feedbackScore,
			"feedback_comment": feedbackComment,
			"updated_at":       time.Now().Unix(),
		}).Error
}

// GetIncidentById 按 ID 获取事件
func (r *rcaRepository) GetIncidentById(
	ctx context.Context,
	tenantId, incidentId string,
) (*models.RCAIncident, error) {
	var incident models.RCAIncident
	err := r.db.WithContext(ctx).
		Where("tenant_id = ? AND id = ?", tenantId, incidentId).
		First(&incident).Error

	if err != nil {
		return nil, err
	}
	return &incident, nil
}

// GetIncidentsBySuggestion 获取建议关联的所有事件
func (r *rcaRepository) GetIncidentsBySuggestion(
	ctx context.Context,
	tenantId, suggestionId string,
) ([]*models.RCAIncident, error) {
	var incidents []*models.RCAIncident
	err := r.db.WithContext(ctx).
		Where("tenant_id = ? AND suggestion_id = ?", tenantId, suggestionId).
		Order("created_at DESC").
		Find(&incidents).Error

	if err != nil {
		return nil, err
	}
	return incidents, nil
}

// ListIncidents 列表查询事件
// 对标 AlertHub 分页模式
func (r *rcaRepository) ListIncidents(
	ctx context.Context,
	tenantId string,
	filters map[string]interface{},
	page, pageSize int,
) ([]*models.RCAIncident, int64, error) {
	var incidents []*models.RCAIncident
	var total int64

	query := r.db.WithContext(ctx).Where("tenant_id = ?", tenantId)

	// 应用过滤条件
	if status, ok := filters["status"]; ok && status != "" {
		query = query.Where("status = ?", status)
	}
	if severity, ok := filters["severity"]; ok && severity != "" {
		query = query.Where("severity = ?", severity)
	}
	if minConfidence, ok := filters["minConfidence"]; ok {
		confidence := minConfidence.(float64)
		if confidence > 0 {
			query = query.Where("confidence_score >= ?", confidence)
		}
	}

	// 分别查询总数和分页数据（对标 AlertHub 模式）
	query.Model(&models.RCAIncident{}).Count(&total)

	offset := (page - 1) * pageSize
	err := query.
		Order("created_at DESC").
		Offset(offset).
		Limit(pageSize).
		Find(&incidents).Error

	if err != nil {
		return nil, 0, err
	}

	return incidents, total, nil
}

// CreateIncident 创建单个事件
func (r *rcaRepository) CreateIncident(
	ctx context.Context,
	incident *models.RCAIncident,
) error {
	return r.db.WithContext(ctx).Create(incident).Error
}

// BatchCreateIncidents 批量创建事件
func (r *rcaRepository) BatchCreateIncidents(
	ctx context.Context,
	incidents []*models.RCAIncident,
) error {
	if len(incidents) == 0 {
		return nil
	}
	// 按批次插入，每 100 条提交一次
	return r.db.WithContext(ctx).CreateInBatches(incidents, 100).Error
}

// UpdateIncidentStatus 更新事件状态
func (r *rcaRepository) UpdateIncidentStatus(
	ctx context.Context,
	tenantId, incidentId, status string,
) error {
	return r.db.WithContext(ctx).
		Model(&models.RCAIncident{}).
		Where("tenant_id = ? AND id = ?", tenantId, incidentId).
		Update("status", status).Error
}

// DeleteSuggestionAndIncidents 删除建议及其关联的所有事件
// 强制刷新时调用，清理旧的缓存数据
func (r *rcaRepository) DeleteSuggestionAndIncidents(
	ctx context.Context,
	tenantId, suggestionId string,
) error {
	// 先删除关联的事件
	if err := r.db.WithContext(ctx).
		Where("tenant_id = ? AND suggestion_id = ?", tenantId, suggestionId).
		Delete(&models.RCAIncident{}).Error; err != nil {
		return err
	}
	// 再删除建议
	return r.db.WithContext(ctx).
		Where("tenant_id = ? AND id = ?", tenantId, suggestionId).
		Delete(&models.RCASuggestion{}).Error
}

// GetLatestReport 获取最新报告快照
func (r *rcaRepository) GetLatestReport(
	ctx context.Context,
	tenantId string,
) (*models.RCAReportSnapshot, error) {
	var report models.RCAReportSnapshot
	err := r.db.WithContext(ctx).
		Where("tenant_id = ?", tenantId).
		Order("created_at DESC").
		First(&report).Error

	if err != nil {
		return nil, err
	}
	return &report, nil
}

// GetReportByTimeRange 按时间范围查询报告
func (r *rcaRepository) GetReportByTimeRange(
	ctx context.Context,
	tenantId string,
	startTime, endTime int64,
) (*models.RCAReportSnapshot, error) {
	var report models.RCAReportSnapshot
	err := r.db.WithContext(ctx).
		Where("tenant_id = ? AND time_range_start >= ? AND time_range_end <= ?",
			tenantId, startTime, endTime).
		Order("created_at DESC").
		First(&report).Error

	if err != nil {
		return nil, err
	}
	return &report, nil
}

// CreateReport 创建报告快照
func (r *rcaRepository) CreateReport(
	ctx context.Context,
	report *models.RCAReportSnapshot,
) error {
	return r.db.WithContext(ctx).Create(report).Error
}
