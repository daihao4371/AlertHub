package services

import (
	"alertHub/internal/ctx"
	"alertHub/internal/models"
	"alertHub/internal/types"
	"alertHub/pkg/tools"
	"fmt"
	"sync"
	"time"

	"github.com/zeromicro/go-zero/core/logc"
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

// updateDutyScheduleInDB 使用批量操作将值班表数据持久化到数据库，以优化性能。
// 将新记录与已存在记录分离，并批量处理以减少数据库往返次数。
func (dms dutyCalendarService) updateDutyScheduleInDB(dutyScheduleList []models.DutySchedule, tenantId string) error {
	if len(dutyScheduleList) == 0 {
		return nil
	}

	// 在处理前为所有记录设置租户ID，确保数据一致性
	enrichedSchedules := dms.enrichSchedulesWithTenantId(dutyScheduleList, tenantId)

	// 根据已存在记录将值班表分为创建和更新批次
	toCreate, toUpdate, err := dms.partitionSchedulesByExistence(enrichedSchedules, tenantId)
	if err != nil {
		return err
	}

	// 批量创建新记录
	if err := dms.batchCreateSchedules(toCreate); err != nil {
		return err
	}

	// 在事务中批量更新已存在的记录
	if err := dms.batchUpdateSchedulesInTransaction(toUpdate); err != nil {
		return err
	}

	return nil
}

// enrichSchedulesWithTenantId 为所有值班表记录设置租户ID，确保数据一致性。
// 避免在数据库操作过程中逐个设置租户ID，提升性能。
func (dms dutyCalendarService) enrichSchedulesWithTenantId(
	schedules []models.DutySchedule,
	tenantId string,
) []models.DutySchedule {
	enriched := make([]models.DutySchedule, len(schedules))
	for i := range schedules {
		enriched[i] = schedules[i]
		enriched[i].TenantId = tenantId
	}
	return enriched
}

// extractScheduleTimes 从值班表记录中提取时间字符串，用于批量存在性检查。
// 预分配容量以提高大数据集的内存效率。
func (dms dutyCalendarService) extractScheduleTimes(schedules []models.DutySchedule) []string {
	times := make([]string, 0, len(schedules))
	for _, schedule := range schedules {
		times = append(times, schedule.Time)
	}
	return times
}

// partitionSchedulesByExistence 将值班表记录分离为创建和更新批次。
// 使用单次批量查询检查所有记录的存在性，然后根据结果进行分离。
func (dms dutyCalendarService) partitionSchedulesByExistence(
	schedules []models.DutySchedule,
	tenantId string,
) ([]models.DutySchedule, []models.DutySchedule, error) {
	if len(schedules) == 0 {
		return nil, nil, nil
	}

	// 提取所有时间值用于批量存在性检查
	times := dms.extractScheduleTimes(schedules)
	dutyId := schedules[0].DutyId

	// 单次批量查询检查所有记录的存在性
	existingSchedules, err := dms.ctx.DB.DutyCalendar().BatchGetExistingSchedules(tenantId, dutyId, times)
	if err != nil {
		return nil, nil, fmt.Errorf("批量查询已存在的值班表失败: %w", err)
	}

	// 根据存在性检查结果分离值班表记录
	toCreate := make([]models.DutySchedule, 0, len(schedules))
	toUpdate := make([]models.DutySchedule, 0, len(schedules))

	for _, schedule := range schedules {
		if _, exists := existingSchedules[schedule.Time]; exists {
			toUpdate = append(toUpdate, schedule)
		} else {
			toCreate = append(toCreate, schedule)
		}
	}

	return toCreate, toUpdate, nil
}

// batchCreateSchedules 批量创建新的值班表记录，以优化数据库性能。
// 记录创建的数量用于监控目的。
func (dms dutyCalendarService) batchCreateSchedules(schedules []models.DutySchedule) error {
	if len(schedules) == 0 {
		return nil
	}

	if err := dms.ctx.DB.DutyCalendar().BatchCreate(schedules); err != nil {
		return fmt.Errorf("批量创建值班表失败: %w", err)
	}

	logc.Infof(dms.ctx.Ctx, "批量创建了 %d 条值班表记录", len(schedules))
	return nil
}

// batchUpdateSchedulesInTransaction 使用仓库层的批量更新方法更新已存在的值班表记录。
// 将事务管理委托给仓库层，以实现更好的关注点分离。
func (dms dutyCalendarService) batchUpdateSchedulesInTransaction(schedules []models.DutySchedule) error {
	if len(schedules) == 0 {
		return nil
	}

	if err := dms.ctx.DB.DutyCalendar().BatchUpdate(schedules); err != nil {
		return fmt.Errorf("批量更新值班表失败: %w", err)
	}

	logc.Infof(dms.ctx.Ctx, "批量更新了 %d 条值班表记录", len(schedules))
	return nil
}
