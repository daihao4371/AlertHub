package repo

import (
	"fmt"
	"time"
	models "alertHub/internal/models"

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

	// Collect all user IDs and usernames that need realName enrichment
	userIdsMap := make(map[string]bool)
	usernamesMap := make(map[string]bool)
	for _, value := range data {
		// Collect manager userid
		if value.Manager.UserId != "" {
			userIdsMap[value.Manager.UserId] = true
		}
		// Collect updateBy username
		if value.UpdateBy != "" {
			usernamesMap[value.UpdateBy] = true
		}
	}

	// Get today's duty schedule and collect user IDs
	for index, value := range data {
		var dutySchedule models.DutySchedule
		d.DB().Model(models.DutySchedule{}).Where("duty_id = ? and time = ?", value.ID, time.Now().Format("2006-1-2")).Find(&dutySchedule)
		data[index].CurDutyUser = dutySchedule.Users

		// Collect curDutyUser userids
		for _, user := range dutySchedule.Users {
			if user.UserId != "" {
				userIdsMap[user.UserId] = true
			}
		}
	}

	// Batch query user information to enrich realName
	userMap := make(map[string]string)               // userid -> realName
	usernameToRealNameMap := make(map[string]string) // username -> realName
	if len(userIdsMap) > 0 {
		userIds := make([]string, 0, len(userIdsMap))
		for userid := range userIdsMap {
			userIds = append(userIds, userid)
		}

		// Query users by userids
		var users []models.Member
		d.DB().Model(&models.Member{}).Where("user_id IN ?", userIds).Find(&users)
		for _, user := range users {
			userMap[user.UserId] = user.RealName
		}
	}

	// Query users by usernames for updateBy field
	if len(usernamesMap) > 0 {
		usernames := make([]string, 0, len(usernamesMap))
		for username := range usernamesMap {
			usernames = append(usernames, username)
		}

		// Query users by usernames
		var users []models.Member
		d.DB().Model(&models.Member{}).Where("user_name IN ?", usernames).Find(&users)
		for _, user := range users {
			usernameToRealNameMap[user.UserName] = user.RealName
		}
	}

	// Enrich realName for Manager, CurDutyUser, and UpdateBy
	for index := range data {
		// Enrich manager realName
		if data[index].Manager.UserId != "" {
			if realName, exists := userMap[data[index].Manager.UserId]; exists {
				data[index].Manager.RealName = realName
			}
		}

		// Enrich curDutyUser realName
		for i := range data[index].CurDutyUser {
			if data[index].CurDutyUser[i].UserId != "" {
				if realName, exists := userMap[data[index].CurDutyUser[i].UserId]; exists {
					data[index].CurDutyUser[i].RealName = realName
				}
			}
		}

		// Enrich updateBy realName
		if data[index].UpdateBy != "" {
			if realName, exists := usernameToRealNameMap[data[index].UpdateBy]; exists {
				data[index].UpdateByRealName = realName
			}
		}
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
