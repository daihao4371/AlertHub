package repo

import (
	"context"
	"errors"
	"fmt"
	"github.com/zeromicro/go-zero/core/logc"
	"gorm.io/gorm"
	"time"
	"alertHub/internal/models"
	"alertHub/pkg/tools"
)

type (
	DutyCalendarRepo struct {
		entryRepo
	}

	InterDutyCalendar interface {
		GetCalendarInfo(dutyId, time string) models.DutySchedule
		GetDutyUserInfo(dutyId, time string) ([]models.Member, bool)
		Create(r models.DutySchedule) error
		Update(r models.DutySchedule) error
		Search(tenantId, dutyId, time string) ([]models.DutySchedule, error)
		GetCalendarUsers(tenantId, dutyId string) ([][]models.DutyUser, error)
	}
)

func newDutyCalendarInterface(db *gorm.DB, g InterGormDBCli) InterDutyCalendar {
	return &DutyCalendarRepo{
		entryRepo{
			g:  g,
			db: db,
		},
	}
}

// GetCalendarInfo 获取值班表信息
func (dc DutyCalendarRepo) GetCalendarInfo(dutyId, time string) models.DutySchedule {
	var dutySchedule models.DutySchedule

	dc.db.Model(models.DutySchedule{}).
		Where("duty_id = ? AND time = ?", dutyId, time).
		First(&dutySchedule)

	return dutySchedule
}

// GetDutyUserInfo 获取值班用户信息
func (dc DutyCalendarRepo) GetDutyUserInfo(dutyId, time string) ([]models.Member, bool) {
	var users []models.Member
	schedule := dc.GetCalendarInfo(dutyId, time)
	for _, user := range schedule.Users {
		var userData models.Member
		db := dc.db.Model(models.Member{}).Where("user_id = ?", user.UserId)
		if err := db.First(&userData).Error; err != nil {
			logc.Error(context.Background(), "获取值班用户信息失败, msg: "+err.Error())
			continue
		}
		users = append(users, userData)
	}

	if users == nil {
		return users, false
	}

	return users, true
}

func (dc DutyCalendarRepo) Create(r models.DutySchedule) error {
	return dc.g.Create(models.DutySchedule{}, r)
}

func (dc DutyCalendarRepo) Update(r models.DutySchedule) error {
	u := Updates{
		Table: models.DutySchedule{},
		Where: map[string]interface{}{
			"tenant_id = ?": r.TenantId,
			"duty_id = ?":   r.DutyId,
			"time = ?":      r.Time,
		},
		Updates: r,
	}

	return dc.g.Updates(u)
}

func (dc DutyCalendarRepo) Search(tenantId, dutyId, time string) ([]models.DutySchedule, error) {
	var dutyScheduleList []models.DutySchedule
	db := dc.db.Model(&models.DutySchedule{})

	db.Where("tenant_id = ? AND duty_id = ? AND time LIKE ?", tenantId, dutyId, time+"%")
	err := db.Find(&dutyScheduleList).Error
	if err != nil {
		return dutyScheduleList, err
	}

	return dutyScheduleList, nil
}

// GetCalendarUsers 获取值班用户
// 获取当前月份（从今天到月底）正在值班的所有用户组，避免已移除的用户仍存在列表中
func (dc DutyCalendarRepo) GetCalendarUsers(tenantId, dutyId string) ([][]models.DutyUser, error) {
	var (
		entries      []models.DutySchedule
		groupedUsers [][]models.DutyUser
	)

	// 计算查询时间范围：今天 -> 当月最后一天
	now := time.Now().UTC()
	currentDate := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	endOfMonth := time.Date(now.Year(), now.Month()+1, 0, 0, 0, 0, 0, time.UTC)

	db := dc.db.Model(&models.DutySchedule{})
	db.Where("tenant_id = ? AND duty_id = ? AND status = ?", tenantId, dutyId, models.CalendarFormalStatus)
	db.Where("time >= ? AND time <= ?", currentDate, endOfMonth)

	if err := db.Find(&entries).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get calendar users: %w", err)
	}

	// 使用 map 去重用户组，避免重复
	user := make(map[string]struct{})
	for _, entry := range entries {
		key := tools.JsonMarshalToString(entry.Users)
		if _, ok := user[key]; ok {
			continue
		}

		groupedUsers = append(groupedUsers, entry.Users)
		user[key] = struct{}{}
	}

	return groupedUsers, nil
}
