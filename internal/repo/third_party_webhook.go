package repo

import (
	"alertHub/internal/models"
	"gorm.io/gorm"
)

type (
	thirdPartyWebhookRepo struct {
		entryRepo
	}

	InterThirdPartyWebhookRepo interface {
		// 基本CRUD操作
		Create(webhook *models.ThirdPartyWebhook) error
		Update(webhook models.ThirdPartyWebhook) error
		Delete(tenantId, id string) error
		Get(tenantId, id string) (models.ThirdPartyWebhook, error)
		GetByWebhookId(webhookId string) (models.ThirdPartyWebhook, error)

		// 查询操作
		List(tenantId, source, status, query string, page models.Page) ([]models.ThirdPartyWebhook, int64, error)
		ExistsByWebhookId(tenantId, webhookId string) (bool, error)

		// 统计更新
		UpdateStats(id string, callCount int64, lastCallAt int64) error
	}
)

func newThirdPartyWebhookRepo(db *gorm.DB, g InterGormDBCli) InterThirdPartyWebhookRepo {
	return &thirdPartyWebhookRepo{
		entryRepo{
			g:  g,
			db: db,
		},
	}
}

// Create 创建Webhook配置
func (r thirdPartyWebhookRepo) Create(webhook *models.ThirdPartyWebhook) error {
	return r.g.Create(&models.ThirdPartyWebhook{}, webhook)
}

// Update 更新Webhook配置
func (r thirdPartyWebhookRepo) Update(webhook models.ThirdPartyWebhook) error {
	u := Updates{
		Table: &models.ThirdPartyWebhook{},
		Where: map[string]interface{}{
			"tenant_id = ?": webhook.TenantId,
			"id = ?":        webhook.ID,
		},
		Updates: webhook,
	}
	return r.g.Updates(u)
}

// Delete 删除Webhook配置
func (r thirdPartyWebhookRepo) Delete(tenantId, id string) error {
	d := Delete{
		Table: &models.ThirdPartyWebhook{},
		Where: map[string]interface{}{
			"tenant_id = ?": tenantId,
			"id = ?":        id,
		},
	}
	return r.g.Delete(d)
}

// Get 根据ID获取Webhook配置
func (r thirdPartyWebhookRepo) Get(tenantId, id string) (models.ThirdPartyWebhook, error) {
	var webhook models.ThirdPartyWebhook
	db := r.db.Model(&models.ThirdPartyWebhook{}).
		Where("tenant_id = ? AND id = ?", tenantId, id)

	err := db.First(&webhook).Error
	if err != nil {
		return webhook, err
	}

	return webhook, nil
}

// GetByWebhookId 根据WebhookId获取配置（用于接收告警时查找配置）
func (r thirdPartyWebhookRepo) GetByWebhookId(webhookId string) (models.ThirdPartyWebhook, error) {
	var webhook models.ThirdPartyWebhook
	db := r.db.Model(&models.ThirdPartyWebhook{}).
		Where("webhook_id = ?", webhookId)

	err := db.First(&webhook).Error
	if err != nil {
		return webhook, err
	}

	return webhook, nil
}

// List 分页查询Webhook列表
// 参数：
//   - tenantId: 租户ID（必填）
//   - source: 来源系统过滤（可选）
//   - status: 状态过滤（可选）
//   - query: 关键词搜索（可选，搜索名称、描述、来源）
//   - page: 分页参数
// 返回：Webhook列表、总数、错误
func (r thirdPartyWebhookRepo) List(tenantId, source, status, query string, page models.Page) ([]models.ThirdPartyWebhook, int64, error) {
	var (
		webhooks []models.ThirdPartyWebhook
		count    int64
		db       = r.db.Model(&models.ThirdPartyWebhook{})
	)

	// 必须指定租户ID
	db = db.Where("tenant_id = ?", tenantId)

	// 可选过滤条件
	if source != "" {
		db = db.Where("source = ?", source)
	}
	if status != "" {
		db = db.Where("status = ?", status)
	}
	if query != "" {
		db = db.Where("name LIKE ? OR description LIKE ? OR source LIKE ?",
			"%"+query+"%", "%"+query+"%", "%"+query+"%")
	}

	// 获取总数
	if err := db.Count(&count).Error; err != nil {
		return nil, 0, err
	}

	// 分页查询
	err := db.Limit(int(page.Size)).
		Offset(int((page.Index - 1) * page.Size)).
		Order("create_at DESC").
		Find(&webhooks).Error

	if err != nil {
		return nil, 0, err
	}

	return webhooks, count, nil
}

// ExistsByWebhookId 检查WebhookId是否已存在
// 用于生成唯一ID时检查冲突
func (r thirdPartyWebhookRepo) ExistsByWebhookId(tenantId, webhookId string) (bool, error) {
	var count int64
	err := r.db.Model(&models.ThirdPartyWebhook{}).
		Where("tenant_id = ? AND webhook_id = ?", tenantId, webhookId).
		Count(&count).Error

	if err != nil {
		return false, err
	}

	return count > 0, nil
}

// UpdateStats 更新Webhook调用统计
// 参数：
//   - id: Webhook配置ID
//   - callCount: 最新的调用次数
//   - lastCallAt: 最后调用时间（Unix时间戳）
func (r thirdPartyWebhookRepo) UpdateStats(id string, callCount int64, lastCallAt int64) error {
	u := Updates{
		Table: &models.ThirdPartyWebhook{},
		Where: map[string]interface{}{
			"id = ?": id,
		},
		Updates: map[string]interface{}{
			"call_count":   callCount,
			"last_call_at": lastCallAt,
		},
	}
	return r.g.Updates(u)
}
