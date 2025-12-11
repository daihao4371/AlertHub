package services

import (
	"fmt"
	"github.com/zeromicro/go-zero/core/logc"
	"sync"
	"time"
	"alertHub/internal/ctx"
	"alertHub/internal/models"
	"alertHub/internal/types"
	"alertHub/pkg/tools"
)

type dutyCalendarService struct {
	ctx *ctx.Context
}

type InterDutyCalendarService interface {
	CreateAndUpdate(req interface{}) (interface{}, interface{})
	Update(req interface{}) (interface{}, interface{})
	Search(req interface{}) (interface{}, interface{})
	GetCalendarUsers(req interface{}) (interface{}, interface{})
	AutoGenerateNextYearSchedule() error
}

func newInterDutyCalendarService(ctx *ctx.Context) InterDutyCalendarService {
	return &dutyCalendarService{
		ctx: ctx,
	}
}

// CreateAndUpdate 创建和更新值班表
func (dms dutyCalendarService) CreateAndUpdate(req interface{}) (interface{}, interface{}) {
	r := req.(*types.RequestDutyCalendarCreate)
	dutyScheduleList, err := dms.generateDutySchedule(*r)
	if err != nil {
		return nil, fmt.Errorf("生成值班表失败: %w", err)
	}

	if err := dms.updateDutyScheduleInDB(dutyScheduleList, r.TenantId); err != nil {
		logc.Errorf(dms.ctx.Ctx, err.Error())
	}
	return nil, nil
}

// Update 更新值班表
func (dms dutyCalendarService) Update(req interface{}) (interface{}, interface{}) {
	r := req.(*types.RequestDutyCalendarUpdate)
	err := dms.ctx.DB.DutyCalendar().Update(models.DutySchedule{
		TenantId: r.TenantId,
		DutyId:   r.DutyId,
		Time:     r.Time,
		Status:   r.Status,
		Users:    r.Users,
	})
	if err != nil {
		return nil, err
	}

	return nil, nil
}

// Search 查询值班表
func (dms dutyCalendarService) Search(req interface{}) (interface{}, interface{}) {
	r := req.(*types.RequestDutyCalendarQuery)
	data, err := dms.ctx.DB.DutyCalendar().Search(r.TenantId, r.DutyId, r.Time)
	if err != nil {
		return nil, err
	}

	return data, nil
}

func (dms dutyCalendarService) GetCalendarUsers(req interface{}) (interface{}, interface{}) {
	r := req.(*types.RequestDutyCalendarQuery)
	data, err := dms.ctx.DB.DutyCalendar().GetCalendarUsers(r.TenantId, r.DutyId)
	if err != nil {
		return nil, err
	}

	return data, nil
}

// AutoGenerateNextYearSchedule 自动生成次年值班表
// 每年12月1日自动触发，为所有值班组生成次年全年的值班表
func (dms dutyCalendarService) AutoGenerateNextYearSchedule() error {
	logc.Info(dms.ctx.Ctx, "开始自动生成次年值班表...")

	// 获取所有租户的值班组列表
	// 使用空字符串获取所有租户（系统管理员权限）
	tenants, err := dms.ctx.DB.Tenant().List("")
	if err != nil {
		logc.Errorf(dms.ctx.Ctx, "获取租户列表失败: %s", err.Error())
		return fmt.Errorf("获取租户列表失败: %w", err)
	}

	successCount := 0
	failCount := 0
	skipCount := 0

	for _, tenant := range tenants {
		dutyList, err := dms.ctx.DB.Duty().List(tenant.ID)
		if err != nil {
			logc.Errorf(dms.ctx.Ctx, "获取租户 %s 的值班组列表失败: %s", tenant.ID, err.Error())
			continue
		}

		for _, duty := range dutyList {
			if err := dms.generateNextYearScheduleForDuty(tenant.ID, duty.ID); err != nil {
				logc.Errorf(dms.ctx.Ctx, "为值班组 %s (%s) 生成次年值班表失败: %s", duty.Name, duty.ID, err.Error())
				failCount++
			} else {
				successCount++
			}
		}
	}

	logc.Infof(dms.ctx.Ctx, "自动生成次年值班表完成: 成功 %d 个, 失败 %d 个, 跳过 %d 个", successCount, failCount, skipCount)
	return nil
}

// generateNextYearScheduleForDuty 为单个值班组生成次年值班表
func (dms dutyCalendarService) generateNextYearScheduleForDuty(tenantId, dutyId string) error {
	// 获取当前年份和次年
	currentYear := time.Now().Year()
	nextYear := currentYear + 1

	// 检查次年是否已有数据，避免重复生成
	nextYearFirstDay := fmt.Sprintf("%d-1-1", nextYear)
	existingSchedule := dms.ctx.DB.DutyCalendar().GetCalendarInfo(dutyId, nextYearFirstDay)
	if existingSchedule.Time != "" {
		logc.Infof(dms.ctx.Ctx, "值班组 %s 的次年值班表已存在，跳过生成", dutyId)
		return nil
	}

	// 查询当前年度最后一个月的值班记录，提取值班规则
	currentYearLastMonth := fmt.Sprintf("%d-12", currentYear)
	schedules, err := dms.ctx.DB.DutyCalendar().Search(tenantId, dutyId, currentYearLastMonth)
	if err != nil || len(schedules) == 0 {
		return fmt.Errorf("未找到当前年度的值班记录，无法自动生成")
	}

	// 分析值班规则：提取用户组和值班周期
	userGroups, dateType, dutyPeriod := dms.analyzeSchedulePattern(schedules)
	if len(userGroups) == 0 {
		return fmt.Errorf("无法分析出有效的值班规则")
	}

	// 构造次年值班表生成请求
	request := types.RequestDutyCalendarCreate{
		TenantId:   tenantId,
		DutyId:     dutyId,
		Month:      fmt.Sprintf("%d-01", nextYear), // 次年1月
		DateType:   dateType,
		DutyPeriod: dutyPeriod,
		UserGroup:  userGroups,
		Status:     models.CalendarFormalStatus,
	}

	// 生成并保存次年值班表
	dutyScheduleList, err := dms.generateDutySchedule(request)
	if err != nil {
		return fmt.Errorf("生成值班表失败: %w", err)
	}

	if err := dms.updateDutyScheduleInDB(dutyScheduleList, tenantId); err != nil {
		return fmt.Errorf("保存值班表失败: %w", err)
	}

	logc.Infof(dms.ctx.Ctx, "成功为值班组 %s 生成次年值班表，共 %d 条记录", dutyId, len(dutyScheduleList))
	return nil
}

// analyzeSchedulePattern 分析值班表规律，提取用户组和值班周期
func (dms dutyCalendarService) analyzeSchedulePattern(schedules []models.DutySchedule) ([][]models.DutyUser, string, int) {
	if len(schedules) == 0 {
		return nil, "", 0
	}

	// 使用 map 去重用户组，保持顺序
	userGroupMap := make(map[string][]models.DutyUser)
	userGroupOrder := []string{}

	for _, schedule := range schedules {
		key := tools.JsonMarshalToString(schedule.Users)
		if _, exists := userGroupMap[key]; !exists {
			userGroupMap[key] = schedule.Users
			userGroupOrder = append(userGroupOrder, key)
		}
	}

	// 按照出现顺序构建用户组
	var userGroups [][]models.DutyUser
	for _, key := range userGroupOrder {
		userGroups = append(userGroups, userGroupMap[key])
	}

	// 推断值班类型和周期
	// 简化处理：假设按周值班，周期为1周
	// 可以根据实际数据模式进行更复杂的推断
	dateType := "week"
	dutyPeriod := 1

	// 尝试推断值班周期：检查同一组用户连续值班的天数
	if len(schedules) >= 7 && len(userGroups) > 0 {
		consecutiveDays := 1
		for i := 1; i < len(schedules) && i < 30; i++ {
			if tools.JsonMarshalToString(schedules[i].Users) == tools.JsonMarshalToString(schedules[0].Users) {
				consecutiveDays++
			} else {
				break
			}
		}

		// 判断是按天还是按周
		if consecutiveDays >= 7 {
			dateType = "week"
			dutyPeriod = consecutiveDays / 7
		} else {
			dateType = "day"
			dutyPeriod = consecutiveDays
		}
	}

	return userGroups, dateType, dutyPeriod
}

func (dms dutyCalendarService) generateDutySchedule(dutyInfo types.RequestDutyCalendarCreate) ([]models.DutySchedule, error) {
	curYear, curMonth, _ := tools.ParseTime(dutyInfo.Month)
	dutyDays := dms.calculateDutyDays(dutyInfo.DateType, dutyInfo.DutyPeriod)
	timeC := dms.generateDutyDates(curYear, curMonth)
	dutyScheduleList := dms.createDutyScheduleList(dutyInfo, timeC, dutyDays)

	return dutyScheduleList, nil
}

// 计算值班天数
func (dms dutyCalendarService) calculateDutyDays(dateType string, dutyPeriod int) int {
	switch dateType {
	case "day":
		return dutyPeriod
	case "week":
		return 7 * dutyPeriod
	default:
		return 0
	}
}

// 生成值班日期 - 从指定月份开始生成未来12个月的日期（支持跨年）
func (dms dutyCalendarService) generateDutyDates(year int, startMonth time.Month) <-chan string {
	timeC := make(chan string, 370)
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer close(timeC)
		defer wg.Done()

		// 从指定月份的第一天开始
		currentDate := time.Date(year, startMonth, 1, 0, 0, 0, 0, time.UTC)
		// 计算结束日期：未来12个月后的最后一天
		endDate := currentDate.AddDate(1, 0, -1)

		// 逐日生成日期，直到结束日期
		for currentDate.Before(endDate) || currentDate.Equal(endDate) {
			timeC <- currentDate.Format("2006-1-2")
			currentDate = currentDate.AddDate(0, 0, 1) // 日期加1天
		}
	}()

	// 等待所有日期生产完成
	wg.Wait()
	return timeC
}

// 创建值班表
func (dms dutyCalendarService) createDutyScheduleList(dutyInfo types.RequestDutyCalendarCreate, timeC <-chan string, dutyDays int) []models.DutySchedule {
	var dutyScheduleList []models.DutySchedule
	var count int

	for {
		// 数据消费完成后退出
		if len(timeC) == 0 {
			break
		}

		for _, users := range dutyInfo.UserGroup {
			for day := 1; day <= dutyDays; day++ {
				date, ok := <-timeC
				if !ok {
					return dutyScheduleList
				}

				dutyScheduleList = append(dutyScheduleList, models.DutySchedule{
					DutyId: dutyInfo.DutyId,
					Time:   date,
					Users:  users,
					Status: dutyInfo.Status,
				})

				if dutyInfo.DateType == "week" && tools.IsEndOfWeek(date) {
					count++
					if count == dutyInfo.DutyPeriod {
						count = 0
						break
					}
				}
			}
		}
	}

	return dutyScheduleList
}

// 更新库表
func (dms dutyCalendarService) updateDutyScheduleInDB(dutyScheduleList []models.DutySchedule, tenantId string) error {
	for _, schedule := range dutyScheduleList {
		schedule.TenantId = tenantId
		dutyScheduleInfo := dms.ctx.DB.DutyCalendar().GetCalendarInfo(schedule.DutyId, schedule.Time)

		var err error
		if dutyScheduleInfo.Time != "" {
			err = dms.ctx.DB.DutyCalendar().Update(schedule)
		} else {
			err = dms.ctx.DB.DutyCalendar().Create(schedule)
		}

		if err != nil {
			return fmt.Errorf("更新/创建值班系统失败: %w", err)
		}
	}
	return nil
}
