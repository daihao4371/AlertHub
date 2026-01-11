package repo

import (
	"alertHub/internal/models"
	"gorm.io/gorm"
)

type (
	thirdPartyAlertRepo struct {
		entryRepo
	}

	InterThirdPartyAlertRepo interface {
		// 基本操作
		Create(alert *models.ThirdPartyAlert) error
		Get(id string) (models.ThirdPartyAlert, error)
		GetByFingerprint(tenantId, fingerprint string) (models.ThirdPartyAlert, error)

		// 查询操作
		List(tenantId, webhookId, processStatus, status string, page models.Page) ([]models.ThirdPartyAlert, int64, error)

		// 更新操作
		UpdateProcessStatus(id, processStatus, errorMessage string) error
		UpdateFaultCenterId(id, faultCenterId, eventId string) error
	}
)

func newThirdPartyAlertRepo(db *gorm.DB, g InterGormDBCli) InterThirdPartyAlertRepo {
	return &thirdPartyAlertRepo{
		entryRepo{
			g:  g,
			db: db,
		},
	}
}

// Create 创建第三方告警记录
func (r thirdPartyAlertRepo) Create(alert *models.ThirdPartyAlert) error {
	return r.g.Create(&models.ThirdPartyAlert{}, alert)
}

// Get 根据ID获取告警记录
func (r thirdPartyAlertRepo) Get(id string) (models.ThirdPartyAlert, error) {
	var alert models.ThirdPartyAlert
	err := r.db.Model(&models.ThirdPartyAlert{}).
		Where("id = ?", id).
		First(&alert).Error

	if err != nil {
		return alert, err
	}

	return alert, nil
}

// GetByFingerprint 根据指纹获取告警记录
// 用于告警去重：查找相同指纹的告警
// 参数：
//   - tenantId: 租户ID
//   - fingerprint: 告警指纹
// 返回：最新的一条匹配记录
func (r thirdPartyAlertRepo) GetByFingerprint(tenantId, fingerprint string) (models.ThirdPartyAlert, error) {
	var alert models.ThirdPartyAlert
	err := r.db.Model(&models.ThirdPartyAlert{}).
		Where("tenant_id = ? AND fingerprint = ?", tenantId, fingerprint).
		Order("create_at DESC"). // 获取最新的一条
		First(&alert).Error

	if err != nil {
		return alert, err
	}

	return alert, nil
}

// List 分页查询告警记录列表
// 参数：
//   - tenantId: 租户ID（必填）
//   - webhookId: Webhook配置ID过滤（可选）
//   - processStatus: 处理状态过滤（可选）：success/failed/pending
//   - status: 告警状态过滤（可选）：firing/resolved
//   - page: 分页参数
// 返回：告警列表、总数、错误
func (r thirdPartyAlertRepo) List(tenantId, webhookId, processStatus, status string, page models.Page) ([]models.ThirdPartyAlert, int64, error) {
	var (
		alerts []models.ThirdPartyAlert
		count  int64
		db     = r.db.Model(&models.ThirdPartyAlert{})
	)

	// 必须指定租户ID
	db = db.Where("tenant_id = ?", tenantId)

	// 可选过滤条件
	if webhookId != "" {
		db = db.Where("webhook_id = ?", webhookId)
	}
	if processStatus != "" {
		db = db.Where("process_status = ?", processStatus)
	}
	if status != "" {
		db = db.Where("status = ?", status)
	}

	// 获取总数
	if err := db.Count(&count).Error; err != nil {
		return nil, 0, err
	}

	// 分页查询，按创建时间倒序
	err := db.Limit(int(page.Size)).
		Offset(int((page.Index - 1) * page.Size)).
		Order("create_at DESC").
		Find(&alerts).Error

	if err != nil {
		return nil, 0, err
	}

	return alerts, count, nil
}

// UpdateProcessStatus 更新告警处理状态
// 参数：
//   - id: 告警记录ID
//   - processStatus: 处理状态（success/failed/pending）
//   - errorMessage: 错误信息（处理失败时填写）
func (r thirdPartyAlertRepo) UpdateProcessStatus(id, processStatus, errorMessage string) error {
	u := Updates{
		Table: &models.ThirdPartyAlert{},
		Where: map[string]interface{}{
			"id = ?": id,
		},
		Updates: map[string]interface{}{
			"process_status": processStatus,
			"error_message":  errorMessage,
		},
	}
	return r.g.Updates(u)
}

// UpdateFaultCenterId 更新关联的故障中心ID和告警事件ID
// 当第三方告警成功转换并创建了AlertHub内部事件后调用
// 参数：
//   - id: 告警记录ID
//   - faultCenterId: 故障中心ID
//   - eventId: 告警事件ID
func (r thirdPartyAlertRepo) UpdateFaultCenterId(id, faultCenterId, eventId string) error {
	u := Updates{
		Table: &models.ThirdPartyAlert{},
		Where: map[string]interface{}{
			"id = ?": id,
		},
		Updates: map[string]interface{}{
			"fault_center_id": faultCenterId,
			"event_id":        eventId,
		},
	}
	return r.g.Updates(u)
}
