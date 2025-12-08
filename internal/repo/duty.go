package repo

import (
	"fmt"
	"time"
	models "watchAlert/internal/models"

	"gorm.io/gorm"
)

type (
	DutyRepo struct {
		entryRepo
	}
	InterDutyRepo interface {
		GetQuota(id string) bool
		List(tenantId string) ([]models.DutyManagement, error)
		Create(r models.DutyManagement) error
		Update(r models.DutyManagement) error
		Delete(tenantId, id string) error
		Get(tenantId, id string) (models.DutyManagement, error)
	}
)

func newDutyInterface(db *gorm.DB, g InterGormDBCli) InterDutyRepo {
	return &DutyRepo{
		entryRepo{
			g:  g,
			db: db,
		},
	}
}

func (d DutyRepo) GetQuota(id string) bool {
	var (
		db     = d.DB().Model(&models.Tenant{})
		data   models.Tenant
		Number int64
	)

	db.Where("id = ?", id)
	db.Find(&data)

	d.DB().Model(&models.DutyManagement{}).Where("tenant_id = ?", id).Count(&Number)

	return Number < data.DutyNumber
}

func (d DutyRepo) List(tenantId string) ([]models.DutyManagement, error) {
	var data []models.DutyManagement

	db := d.db.Model(&models.DutyManagement{})
	db.Where("tenant_id = ?", tenantId)
	err := db.Find(&data).Error
	if err != nil {
		return nil, err
	}

	for index, value := range data {
		var dutySchedule models.DutySchedule
		d.DB().Model(models.DutySchedule{}).Where("duty_id = ? and time = ?", value.ID, time.Now().Format("2006-1-2")).Find(&dutySchedule)
		data[index].CurDutyUser = dutySchedule.Users
	}

	return data, nil
}

func (d DutyRepo) Create(r models.DutyManagement) error {
	err := d.g.Create(&models.DutyManagement{}, r)
	if err != nil {
		return err
	}
	return nil
}

func (d DutyRepo) Update(r models.DutyManagement) error {
	u := Updates{
		Table: models.DutyManagement{},
		Where: map[string]interface{}{
			"tenant_id = ?": r.TenantId,
			"id = ?":        r.ID,
		},
		Updates: r,
	}
	err := d.g.Updates(u)
	if err != nil {
		return err
	}
	return nil
}

func (d DutyRepo) Delete(tenantId, id string) error {
	// 1. 检查是否有通知对象绑定
	var notices []models.AlertNotice
	db := d.db.Model(&models.AlertNotice{})
	db.Where("tenant_id = ? AND duty_id = ?", tenantId, id).Find(&notices)
	if len(notices) > 0 {
		// 构建详细的错误信息，列出所有绑定的通知对象
		noticeNames := make([]string, 0, len(notices))
		for _, notice := range notices {
			noticeNames = append(noticeNames, notice.Name)
		}
		return fmt.Errorf("无法删除值班表 %s, 因为已有 %d 个通知对象绑定: %v", id, len(notices), noticeNames)
	}

	// 2. 删除值班管理记录
	delDuty := Delete{
		Table: models.DutyManagement{},
		Where: map[string]interface{}{
			"tenant_id = ?": tenantId,
			"id = ?":        id,
		},
	}
	err := d.g.Delete(delDuty)
	if err != nil {
		return fmt.Errorf("删除值班管理记录失败: %w", err)
	}

	// 3. 删除值班日程记录
	delCalendar := Delete{
		Table: models.DutySchedule{},
		Where: map[string]interface{}{
			"tenant_id = ?": tenantId,
			"duty_id = ?":   id,
		},
	}
	err = d.g.Delete(delCalendar)
	if err != nil {
		return fmt.Errorf("删除值班日程记录失败: %w", err)
	}

	return nil
}

func (d DutyRepo) Get(tenantId, id string) (models.DutyManagement, error) {
	var data models.DutyManagement
	db := d.db.Model(&models.DutyManagement{})
	db.Where("tenant_id = ? AND id = ?", tenantId, id)
	err := db.First(&data).Error
	if err != nil {
		return models.DutyManagement{}, err
	}

	return data, nil
}
