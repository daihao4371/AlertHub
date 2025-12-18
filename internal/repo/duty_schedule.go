package repo

import (
	"alertHub/internal/models"
	"alertHub/pkg/tools"
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/zeromicro/go-zero/core/logc"
	"gorm.io/gorm"
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
		BatchCreate(schedules []models.DutySchedule) error
		BatchUpdate(schedules []models.DutySchedule) error
		BatchGetExistingSchedules(tenantId, dutyId string, times []string) (map[string]models.DutySchedule, error)
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

// BatchGetExistingSchedules 批量查询已存在的值班表记录
func (dc DutyCalendarRepo) BatchGetExistingSchedules(tenantId, dutyId string, times []string) (map[string]models.DutySchedule, error) {
	if len(times) == 0 {
		return make(map[string]models.DutySchedule), nil
	}

	var existingSchedules []models.DutySchedule
	err := dc.db.Model(&models.DutySchedule{}).
		Where("tenant_id = ? AND duty_id = ? AND time IN ?", tenantId, dutyId, times).
		Find(&existingSchedules).Error

	if err != nil {
		return nil, err
	}

	// 构建时间到记录的映射
	scheduleMap := make(map[string]models.DutySchedule)
	for _, schedule := range existingSchedules {
		scheduleMap[schedule.Time] = schedule
	}

	return scheduleMap, nil
}

const (
	// defaultBatchSize 定义批量插入的最优批次大小。
	// 选择500是为了在内存使用和数据库连接开销之间取得平衡。
	defaultBatchSize = 500
)

// BatchCreate 批量插入值班表记录以优化数据库性能。
// 大数据集被拆分为较小的批次，以避免内存问题和连接超时。
func (dc DutyCalendarRepo) BatchCreate(schedules []models.DutySchedule) error {
	if len(schedules) == 0 {
		return nil
	}

	// 分批处理值班表记录以优化数据库性能
	for i := 0; i < len(schedules); i += defaultBatchSize {
		end := i + defaultBatchSize
		if end > len(schedules) {
			end = len(schedules)
		}

		batch := schedules[i:end]
		if err := dc.db.Model(&models.DutySchedule{}).Create(&batch).Error; err != nil {
			return fmt.Errorf("批量创建值班表失败 (批次 %d-%d): %w", i, end-1, err)
		}
	}

	return nil
}

// BatchUpdate 在单个事务中更新多条值班表记录。
// 由于每条记录都有基于时间的唯一条件，需要单独更新，但包装在事务中
// 以确保原子性，并相比单独事务减少提交开销。
func (dc DutyCalendarRepo) BatchUpdate(schedules []models.DutySchedule) error {
	if len(schedules) == 0 {
		return nil
	}

	// 使用事务批量执行所有更新以提升性能
	tx := dc.db.Begin()
	if tx.Error != nil {
		return fmt.Errorf("启动事务失败: %w", tx.Error)
	}

	// 在事务中执行所有更新
	for _, schedule := range schedules {
		err := tx.Model(&models.DutySchedule{}).
			Where("tenant_id = ? AND duty_id = ? AND time = ?", schedule.TenantId, schedule.DutyId, schedule.Time).
			Updates(schedule).Error
		if err != nil {
			tx.Rollback()
			return fmt.Errorf("更新值班表失败 (dutyId=%s, time=%s): %w", schedule.DutyId, schedule.Time, err)
		}
	}

	// 提交事务或在出错时回滚
	if err := tx.Commit().Error; err != nil {
		tx.Rollback()
		return fmt.Errorf("提交事务失败: %w", err)
	}

	return nil
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
