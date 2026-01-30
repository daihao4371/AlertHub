package repo

import (
	"alertHub/internal/models"
	"encoding/json"
	"fmt"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type (
	consulRepo struct {
		entryRepo
	}

	// InterConsulRepo Consul数据访问层接口
	InterConsulRepo interface {
		// Consul 目标相关操作
		CreateTarget(target models.ConsulTarget) error
		UpdateTarget(target models.ConsulTarget) error
		DeleteTarget(id int64) error
		GetTargetById(id int64) (models.ConsulTarget, error)
		GetTargets(tenantId string, filters map[string]interface{}, page, pageSize int) ([]models.ConsulTarget, int64, error)
		GetTargetsByInstance(tenantId string, instance string) (models.ConsulTarget, error)
		BatchDeleteTargets(tenantId string, instances []string) error
		GetAllTargetsByTenant(tenantId string) ([]models.ConsulTarget, error)
		// 批量操作（用于性能优化）
		BatchCreateTargets(targets []models.ConsulTarget) error
		BatchUpdateTargets(targets []models.ConsulTarget) error
		BatchUpdateDeletedTargets(tenantId string, serviceIDs []string) error

		// 标签相关操作
		GetTargetsByTag(tenantId string, tag string, page, pageSize int) ([]models.ConsulTarget, int64, error)
		GetTargetsByJobAndTag(tenantId string, job, tag string, page, pageSize int) ([]models.ConsulTarget, int64, error)

		// Consul 注销历史相关操作
		CreateOfflineLog(log models.ConsulTargetOfflineLog) error
		GetOfflineLogs(tenantId string, page, pageSize int) ([]models.ConsulTargetOfflineLog, int64, error)
		GetOfflineLogsByInstance(tenantId string, instance string) ([]models.ConsulTargetOfflineLog, error)

		// 数据清理相关操作（在同步时自动调用）
		CleanupDuplicateTargets(tenantId string) (int64, error)
	}
)

func newConsulRepoInterface(db *gorm.DB, g InterGormDBCli) InterConsulRepo {
	return &consulRepo{
		entryRepo{
			g:  g,
			db: db,
		},
	}
}

// CreateTarget 创建Consul目标
func (c consulRepo) CreateTarget(target models.ConsulTarget) error {
	// 设置创建时间
	target.CreatedAt = time.Now()
	target.UpdatedAt = time.Now()

	err := c.g.Create(models.ConsulTarget{}, target)
	return err
}

// UpdateTarget 更新Consul目标
func (c consulRepo) UpdateTarget(target models.ConsulTarget) error {
	target.UpdatedAt = time.Now()

	err := c.g.Updates(
		Updates{
			Table: models.ConsulTarget{},
			Where: map[string]interface{}{
				"id = ?": target.ID,
			},
			Updates: target,
		},
	)
	return err
}

// DeleteTarget 删除单个目标
func (c consulRepo) DeleteTarget(id int64) error {
	err := c.db.Model(&models.ConsulTarget{}).
		Where("id = ?", id).
		Delete(&models.ConsulTarget{}).Error

	return err
}

// GetTargetById 按ID获取目标
func (c consulRepo) GetTargetById(id int64) (models.ConsulTarget, error) {
	var target models.ConsulTarget
	err := c.db.Model(&models.ConsulTarget{}).
		Where("id = ?", id).
		First(&target).Error

	return target, err
}

// GetTargets 获取目标列表，支持过滤和分页
func (c consulRepo) GetTargets(tenantId string, filters map[string]interface{}, page, pageSize int) ([]models.ConsulTarget, int64, error) {
	var targets []models.ConsulTarget
	var count int64

	db := c.db.Model(&models.ConsulTarget{}).
		Where("tenant_id = ?", tenantId)

	// 应用过滤条件
	if job, ok := filters["job"]; ok && job != "" {
		db = db.Where("job = ?", job)
	}
	if status, ok := filters["status"]; ok && status != "" {
		db = db.Where("status = ?", status)
	}
	if keyword, ok := filters["keyword"]; ok && keyword != "" {
		db = db.Where("instance LIKE ?", "%"+keyword.(string)+"%")
	}

	// 获取总数
	if err := db.Count(&count).Error; err != nil {
		return nil, 0, err
	}

	// 分页查询，按 ID 升序排序
	offset := (page - 1) * pageSize
	err := db.Offset(offset).Limit(pageSize).
		Order("id ASC").
		Find(&targets).Error

	return targets, count, err
}

// GetTargetsByInstance 按实例ID获取目标
func (c consulRepo) GetTargetsByInstance(tenantId string, instance string) (models.ConsulTarget, error) {
	var target models.ConsulTarget
	err := c.db.Model(&models.ConsulTarget{}).
		Where("tenant_id = ? AND instance = ?", tenantId, instance).
		First(&target).Error

	return target, err
}

// BatchDeleteTargets 批量删除指定实例的目标
func (c consulRepo) BatchDeleteTargets(tenantId string, instances []string) error {
	err := c.db.Model(&models.ConsulTarget{}).
		Where("tenant_id = ? AND instance IN ?", tenantId, instances).
		Delete(&models.ConsulTarget{}).Error

	return err
}

// GetAllTargetsByTenant 获取租户下所有未注销的目标
func (c consulRepo) GetAllTargetsByTenant(tenantId string) ([]models.ConsulTarget, error) {
	var targets []models.ConsulTarget
	err := c.db.Model(&models.ConsulTarget{}).
		Where("tenant_id = ? AND status != ? AND consul_deregistered = ?", tenantId, "no checks", false).
		Find(&targets).Error

	return targets, err
}

// 批量操作常量
const (
	// consulBatchSize 定义批量操作的最优批次大小
	// 选择500是为了在内存使用和数据库连接开销之间取得平衡
	consulBatchSize = 500
)

// BatchCreateTargets 批量创建 Consul 目标，使用 UPSERT 处理重复记录
// 如果记录已存在（基于 tenant_id 和 service_id），则自动更新而不是插入失败
// 大数据集被拆分为较小的批次，以避免内存问题和连接超时
func (c consulRepo) BatchCreateTargets(targets []models.ConsulTarget) error {
	if len(targets) == 0 {
		return nil
	}

	// 设置创建和更新时间
	now := time.Now()
	for i := range targets {
		targets[i].CreatedAt = now
		targets[i].UpdatedAt = now
	}

	// 分批处理目标记录以优化数据库性能
	for i := 0; i < len(targets); i += consulBatchSize {
		end := i + consulBatchSize
		if end > len(targets) {
			end = len(targets)
		}

		batch := targets[i:end]
		// 使用 INSERT ... ON DUPLICATE KEY UPDATE 处理重复记录
		// 在冲突时更新关键字段，使用蛇形命名法与数据库列名对应
		err := c.db.Model(&models.ConsulTarget{}).
			Clauses(clause.OnConflict{
				Columns:   []clause.Column{{Name: "tenant_id"}, {Name: "service_id"}},
				DoUpdates: clause.AssignmentColumns([]string{"instance", "job", "labels", "status", "service_name", "consul_deregistered", "updated_at"}),
			}).
			Create(&batch).Error

		if err != nil {
			return fmt.Errorf("批量创建/更新 Consul 目标失败 (批次 %d-%d): %w", i, end-1, err)
		}
	}

	return nil
}

// BatchUpdateTargets 批量更新 Consul 目标，用于优化同步性能
// 在单个事务中更新多条记录，确保原子性并减少提交开销
func (c consulRepo) BatchUpdateTargets(targets []models.ConsulTarget) error {
	if len(targets) == 0 {
		return nil
	}

	// 设置更新时间
	now := time.Now()
	for i := range targets {
		targets[i].UpdatedAt = now
	}

	// 使用事务批量执行所有更新以提升性能
	tx := c.db.Begin()
	if tx.Error != nil {
		return fmt.Errorf("启动事务失败: %w", tx.Error)
	}

	// 在事务中执行所有更新
	for _, target := range targets {
		updateFields := map[string]interface{}{
			"instance":   target.Instance,
			"status":     target.Status,
			"updated_at": now,
		}

		// 更新 Labels 字段（即使为空也要更新，用于清空之前的数据）
		// 使用 Updates(map) 时 GORM 不会自动应用 serializer:json，需要手动序列化为 JSON 字符串
		labelsToUpdate := target.Labels
		if labelsToUpdate == nil {
			labelsToUpdate = map[string]interface{}{}
		}
		labelsJSON, err := json.Marshal(labelsToUpdate)
		if err != nil {
			tx.Rollback()
			return fmt.Errorf("序列化 Labels 失败 (serviceId=%s): %w", target.ServiceID, err)
		}
		updateFields["labels"] = string(labelsJSON)

		err = tx.Model(&models.ConsulTarget{}).
			Where("service_id = ? AND tenant_id = ?", target.ServiceID, target.TenantId).
			Updates(updateFields).Error
		if err != nil {
			tx.Rollback()
			return fmt.Errorf("批量更新 Consul 目标失败 (serviceId=%s): %w", target.ServiceID, err)
		}
	}

	// 提交事务或在出错时回滚
	if err := tx.Commit().Error; err != nil {
		tx.Rollback()
		return fmt.Errorf("提交事务失败: %w", err)
	}

	return nil
}

// BatchUpdateDeletedTargets 批量更新已删除的目标状态，用于优化同步性能
// 将 Consul 中不存在的服务标记为 "no checks" 状态
// 注意：自动删除不会设置 consul_deregistered 和 deregistration_time，以区分手动注销和自动删除
func (c consulRepo) BatchUpdateDeletedTargets(tenantId string, serviceIDs []string) error {
	if len(serviceIDs) == 0 {
		return nil
	}

	// 分批处理以避免 SQL 语句过长
	now := time.Now()
	for i := 0; i < len(serviceIDs); i += consulBatchSize {
		end := i + consulBatchSize
		if end > len(serviceIDs) {
			end = len(serviceIDs)
		}

		batch := serviceIDs[i:end]
		// 自动删除只设置状态，不设置 consul_deregistered 和 deregistration_time
		// 这样可以通过 deregistration_time 是否为 nil 来区分手动注销和自动删除
		err := c.db.Model(&models.ConsulTarget{}).
			Where("tenant_id = ? AND service_id IN ?", tenantId, batch).
			Updates(map[string]interface{}{
				"status":     "no checks",
				"updated_at": now,
			}).Error
		if err != nil {
			return fmt.Errorf("批量更新删除状态失败 (批次 %d-%d): %w", i, end-1, err)
		}
	}

	return nil
}

// CreateOfflineLog 记录注销历史
func (c consulRepo) CreateOfflineLog(log models.ConsulTargetOfflineLog) error {
	log.CreatedAt = time.Now()

	err := c.g.Create(models.ConsulTargetOfflineLog{}, log)
	return err
}

// GetOfflineLogs 获取注销历史列表，只返回对应目标仍处于注销状态的日志
// 自动过滤掉已重新上线的记录（通过比对目标状态判断）
func (c consulRepo) GetOfflineLogs(tenantId string, page, pageSize int) ([]models.ConsulTargetOfflineLog, int64, error) {
	var logs []models.ConsulTargetOfflineLog
	var count int64

	// 使用子查询：获取已重新上线的目标 instance 列表（consul_deregistered = false）
	// 这样我们可以排除那些已经重新上线的注销日志
	alreadyReonlineSubQuery := c.db.Model(&models.ConsulTarget{}).
		Select("instance").
		Where("tenant_id = ? AND consul_deregistered = ?", tenantId, false)

	db := c.db.Model(&models.ConsulTargetOfflineLog{}).
		Where("tenant_id = ?", tenantId).
		Where("instance NOT IN (?)", alreadyReonlineSubQuery)

	// 获取总数
	if err := db.Count(&count).Error; err != nil {
		return nil, 0, err
	}

	// 分页查询
	offset := (page - 1) * pageSize
	err := db.Offset(offset).Limit(pageSize).
		Order("created_at DESC").
		Find(&logs).Error

	return logs, count, err
}

// GetOfflineLogsByInstance 按实例获取注销历史
func (c consulRepo) GetOfflineLogsByInstance(tenantId string, instance string) ([]models.ConsulTargetOfflineLog, error) {
	var logs []models.ConsulTargetOfflineLog
	err := c.db.Model(&models.ConsulTargetOfflineLog{}).
		Where("tenant_id = ? AND instance = ?", tenantId, instance).
		Order("created_at DESC").
		Find(&logs).Error

	return logs, err
}

// GetTargetsByTag 按标签获取目标，支持分页
// tag 参数是标签的键（key），如 "business"、"department" 等
func (c consulRepo) GetTargetsByTag(tenantId string, tag string, page, pageSize int) ([]models.ConsulTarget, int64, error) {
	// 先查询所有目标（不分页），因为需要在内存中按标签过滤
	// 注意：Labels 是 JSON 类型，MySQL JSON 查询在某些情况下可能效率不高，所以使用客户端过滤
	var allTargets []models.ConsulTarget
	err := c.db.Model(&models.ConsulTarget{}).
		Where("tenant_id = ?", tenantId).
		Where("status != ?", "no checks").
		Order("created_at DESC").
		Find(&allTargets).Error

	if err != nil {
		return nil, 0, err
	}

	// 在内存中按标签过滤
	// tag 参数是标签的键（key），检查标签中是否存在这个键
	var filtered []models.ConsulTarget
	for _, target := range allTargets {
		if target.Labels != nil {
			// 检查标签中是否存在这个键（key）
			if _, exists := target.Labels[tag]; exists {
				filtered = append(filtered, target)
			}
		}
	}

	// 计算过滤后的总数
	total := int64(len(filtered))

	// 对过滤后的结果进行分页
	offset := (page - 1) * pageSize
	end := offset + pageSize
	if end > len(filtered) {
		end = len(filtered)
	}

	var paginatedTargets []models.ConsulTarget
	if offset < len(filtered) {
		paginatedTargets = filtered[offset:end]
	}

	return paginatedTargets, total, nil
}

// GetTargetsByJobAndTag 按 Job 和标签结合查询目标
// tag 参数是标签的键（key），如 "business"、"department" 等
func (c consulRepo) GetTargetsByJobAndTag(tenantId string, job, tag string, page, pageSize int) ([]models.ConsulTarget, int64, error) {
	// 先查询所有匹配 Job 的目标（不分页），因为需要在内存中按标签过滤
	var allTargets []models.ConsulTarget
	err := c.db.Model(&models.ConsulTarget{}).
		Where("tenant_id = ?", tenantId).
		Where("job = ?", job).
		Where("status != ?", "no checks").
		Order("created_at DESC").
		Find(&allTargets).Error

	if err != nil {
		return nil, 0, err
	}

	// 在内存中按标签过滤
	// tag 参数是标签的键（key），检查标签中是否存在这个键
	var filtered []models.ConsulTarget
	for _, target := range allTargets {
		if target.Labels != nil {
			// 检查标签中是否存在这个键（key）
			if _, exists := target.Labels[tag]; exists {
				filtered = append(filtered, target)
			}
		}
	}

	// 计算过滤后的总数
	total := int64(len(filtered))

	// 对过滤后的结果进行分页
	offset := (page - 1) * pageSize
	end := offset + pageSize
	if end > len(filtered) {
		end = len(filtered)
	}

	var paginatedTargets []models.ConsulTarget
	if offset < len(filtered) {
		paginatedTargets = filtered[offset:end]
	}

	return paginatedTargets, total, nil
}

// CleanupDuplicateTargets 清理重复的目标记录（保留 id 最大的，删除其他的）
// 返回删除的重复记录数
// 该方法在同步前调用，确保数据库满足唯一约束
func (c consulRepo) CleanupDuplicateTargets(tenantId string) (int64, error) {
	// 构建 SQL 查询：找出每个 (tenant_id, service_id) 组合中除了 id 最大的记录外的所有记录
	// 使用多层子查询避免 MySQL "can't specify target table for update in FROM clause" 错误
	query := `
		DELETE FROM consul_target
		WHERE id IN (
			SELECT id FROM (
				SELECT ct.id
				FROM consul_target ct
				WHERE ct.tenant_id = ?
				AND ct.id < (
					SELECT MAX(id)
					FROM consul_target ct2
					WHERE ct2.tenant_id = ct.tenant_id
					AND ct2.service_id = ct.service_id
				)
			) AS to_delete
		)
	`

	result := c.db.Exec(query, tenantId)
	return result.RowsAffected, result.Error
}
