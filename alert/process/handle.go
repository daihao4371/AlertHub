package process

import (
	"alertHub/internal/ctx"
	"alertHub/internal/models"
	"alertHub/pkg/sender"
	"alertHub/pkg/templates"
	"alertHub/pkg/tools"
	"fmt"
	"strings"
	"time"

	"github.com/zeromicro/go-zero/core/logc"
	"golang.org/x/sync/errgroup"
)

// HandleAlert 处理告警逻辑
func HandleAlert(ctx *ctx.Context, processType string, faultCenter models.FaultCenter, noticeId string, alerts []*models.AlertCurEvent) error {
	curTime := time.Now().Unix()
	g := new(errgroup.Group)

	// 获取通知对象详细信息
	noticeData, err := getNoticeData(ctx, faultCenter.TenantId, noticeId)
	if err != nil {
		logc.Error(ctx.Ctx, fmt.Sprintf("Failed to get notice data: %v", err))
		return err
	}

	// 按告警等级分组
	severityGroups := make(map[string][]*models.AlertCurEvent)
	for _, alert := range alerts {
		severityGroups[alert.Severity] = append(severityGroups[alert.Severity], alert)
	}

	// 告警聚合
	var aggregationEvents map[string][]*models.AlertCurEvent
	if processType == "alarm" {
		aggregationEvents = alarmAggregation(ctx, processType, faultCenter, severityGroups)
	} else {
		aggregationEvents = severityGroups
	}

	for severity, events := range aggregationEvents {
		g.Go(func() error {
			if events == nil {
				return nil
			}

			// 获取当前事件等级对应的 Hook 和 Sign
			Hook, Sign := getNoticeHookUrlAndSign(noticeData, severity)

			for _, event := range events {
				// 对于告警事件，更新 LastSendTime
				if processType == "alarm" && !event.IsRecovered {
					event.LastSendTime = curTime
					ctx.Redis.Alert().PushAlertEvent(event)
				}

				phoneNumber := func() []string {
					if len(event.DutyUserPhoneNumber) > 0 {
						return event.DutyUserPhoneNumber
					}
					if len(noticeData.PhoneNumber) > 0 {
						return noticeData.PhoneNumber
					}
					return []string{}
				}()

				// 在生成告警内容前，先填充CMDB信息
				// 从告警主机的IP查询CMDB，获取关联应用和负责人信息
				if err := enrichAlertWithCmdbInfo(ctx, event); err != nil {
					logc.Errorf(ctx.Ctx, "填充CMDB信息失败, eventId: %s, 错误: %v", event.EventId, err)
					// 即使CMDB查询失败，也继续发送告警，不影响现有功能
				}

				// 设置值班人员：优先使用CMDB的负责人信息，如果没有则使用值班日历
				event.DutyUser = getDutyUserFromCmdbOrCalendar(ctx, event, noticeData)
				event.DutyUserPhoneNumber = GetDutyUserPhoneNumber(ctx, noticeData)

				content := generateAlertContent(ctx, event, noticeData)
				err := sender.Sender(ctx, sender.SendParams{
					TenantId:    event.TenantId,
					EventId:     event.EventId,
					RuleName:    event.RuleName,
					Severity:    event.Severity,
					NoticeType:  noticeData.NoticeType,
					NoticeId:    noticeId,
					NoticeName:  noticeData.Name,
					IsRecovered: event.IsRecovered,
					Hook:        Hook,
					Email:       getNoticeEmail(noticeData, severity),
					Content:     content,
					PhoneNumber: phoneNumber,
					Sign:        Sign,
				})
				if err != nil {
					logc.Error(ctx.Ctx, fmt.Sprintf("Failed to send alert: %v", err))
				} else {
					// 恢复通知发送成功后，更新 LastSendTime，避免重复发送
					if processType == "alarm" && event.IsRecovered && event.LastSendTime == 0 {
						event.LastSendTime = curTime
						ctx.Redis.Alert().PushAlertEvent(event)
					}
				}
			}
			return nil
		})
	}

	return g.Wait()
}

// alarmAggregation 告警聚合
func alarmAggregation(ctx *ctx.Context, processType string, faultCenter models.FaultCenter, alertGroups map[string][]*models.AlertCurEvent) map[string][]*models.AlertCurEvent {
	// 仅当 processType 为 "alarm" 时执行聚合
	if processType != "alarm" {
		return alertGroups
	}

	curTime := time.Now().Unix()
	newAlertGroups := alertGroups
	switch faultCenter.GetAlarmAggregationType() {
	case "Rule":
		for severity, events := range alertGroups {
			newAlertGroups[severity] = withRuleGroupByAlerts(ctx, curTime, events)
		}
	default:
		return alertGroups
	}

	return newAlertGroups
}

// withRuleGroupByAlerts 聚合告警
func withRuleGroupByAlerts(ctx *ctx.Context, timeInt int64, alerts []*models.AlertCurEvent) []*models.AlertCurEvent {
	if len(alerts) <= 1 {
		return alerts
	}

	var aggregatedAlert *models.AlertCurEvent
	for i := range alerts {
		alert := alerts[i]
		if !strings.Contains(alert.Annotations, "聚合") {
			alert.Annotations += fmt.Sprintf("\n聚合 %d 条告警\n", len(alerts))
		}
		aggregatedAlert = alert

		if !alert.IsRecovered {
			alert.LastSendTime = timeInt
			ctx.Redis.Alert().PushAlertEvent(alert)
		}
	}

	return []*models.AlertCurEvent{aggregatedAlert}
}

// getNoticeData 获取 Notice 数据
func getNoticeData(ctx *ctx.Context, tenantId, noticeId string) (models.AlertNotice, error) {
	return ctx.DB.Notice().Get(tenantId, noticeId)
}

// getNoticeHookUrlAndSign 获取事件等级对应的 Hook 和 Sign
func getNoticeHookUrlAndSign(notice models.AlertNotice, severity string) (string, string) {
	if notice.Routes != nil {
		for _, hook := range notice.Routes {
			if hook.Severity == severity {
				return hook.Hook, hook.Sign
			}
		}
	}
	return notice.DefaultHook, notice.DefaultSign
}

// getNoticeEmail 获取事件等级对应的 Email
func getNoticeEmail(notice models.AlertNotice, severity string) models.Email {
	if notice.Routes != nil {
		for _, route := range notice.Routes {
			if route.Severity == severity {
				return models.Email{
					Subject: notice.Email.Subject,
					To:      route.To,
					CC:      route.CC,
				}
			}
		}
	}
	return notice.Email
}

type WebhookContent struct {
	Alarm     *models.AlertCurEvent `json:"alarm"`
	DutyUsers []models.DutyUser     `json:"dutyUsers"`
}

// generateAlertContent 生成告警内容
func generateAlertContent(ctx *ctx.Context, alert *models.AlertCurEvent, noticeData models.AlertNotice) string {
	if noticeData.NoticeType == "CustomHook" || noticeData.NoticeType == "CMDB" {
		users, ok := ctx.DB.DutyCalendar().GetDutyUserInfo(*noticeData.GetDutyId(), time.Now().Format("2006-1-2"))
		if !ok || len(users) == 0 {
			logc.Error(ctx.Ctx, "Failed to get duty users, noticeName: ", noticeData.Name)
		}

		var dutyUsers = []models.DutyUser{}
		for _, user := range users {
			dutyUsers = append(dutyUsers, models.DutyUser{
				Email:    user.Email,
				Mobile:   user.Phone,
				UserId:   user.UserId,
				Username: user.UserName,
			})
		}
		content := WebhookContent{
			Alarm:     alert,
			DutyUsers: dutyUsers,
		}

		return tools.JsonMarshalToString(content)
	}
	return templates.NewTemplate(ctx, *alert, noticeData).CardContentMsg
}

// enrichAlertWithCmdbInfo 为告警事件填充CMDB信息
// 从告警的Labels中提取instance或ip字段，查询CMDB并填充到告警对象中
// 避免循环依赖，直接调用repo层而不是service层
func enrichAlertWithCmdbInfo(ctx *ctx.Context, alert *models.AlertCurEvent) error {
	if alert == nil || alert.Labels == nil {
		return nil
	}

	// 优先从Labels中提取IP信息
	var hostIP string

	// 1. 尝试从ip字段获取
	if ipVal, exists := alert.Labels["ip"]; exists {
		if ipStr, ok := ipVal.(string); ok && ipStr != "" {
			hostIP = ipStr
		}
	}

	// 2. 如果没有ip字段，尝试从instance字段提取
	if hostIP == "" {
		if instanceVal, exists := alert.Labels["instance"]; exists {
			if instanceStr, ok := instanceVal.(string); ok && instanceStr != "" {
				// 从 "10.10.217.225:9100" 格式中提取IP
				hostIP = extractIPFromInstance(instanceStr)
			}
		}
	}

	// 如果没有找到IP，直接返回
	if hostIP == "" {
		return nil
	}

	// 查询CMDB信息
	hostInfo, err := ctx.DB.Cmdb().GetHostInfoByIP(hostIP)
	if err != nil {
		logc.Errorf(ctx.Ctx, "查询CMDB信息失败, IP: %s, 错误: %v", hostIP, err)
		return err
	}

	// 如果未找到主机信息，直接返回
	if hostInfo == nil {
		return nil
	}

	// 填充CMDB信息到告警对象
	// 将应用名称添加到Labels和运行时字段中，供模板使用
	if len(hostInfo.AppNames) > 0 {
		appNamesStr := strings.Join(hostInfo.AppNames, ", ")
		// 同时添加到Labels和运行时字段，方便模板访问
		alert.Labels["cmdb_app_names"] = appNamesStr
		alert.CmdbAppNames = appNamesStr
	}

	// 填充运维负责人信息
	if len(hostInfo.OpsOwners) > 0 {
		opsOwnersStr := strings.Join(hostInfo.OpsOwners, ", ")
		alert.Labels["cmdb_ops_owners"] = opsOwnersStr
		alert.CmdbOpsOwners = opsOwnersStr
	}

	// 填充开发负责人信息
	if len(hostInfo.DevOwners) > 0 {
		devOwnersStr := strings.Join(hostInfo.DevOwners, ", ")
		alert.Labels["cmdb_dev_owners"] = devOwnersStr
		alert.CmdbDevOwners = devOwnersStr
	}

	// 合并运维负责人和开发负责人作为值班人员（用于兼容现有逻辑）
	allOwners := []string{}
	allOwners = append(allOwners, hostInfo.OpsOwners...)
	allOwners = append(allOwners, hostInfo.DevOwners...)

	// 去重
	ownerMap := make(map[string]bool)
	uniqueOwners := []string{}
	for _, owner := range allOwners {
		owner = strings.TrimSpace(owner)
		if owner != "" && !ownerMap[owner] {
			ownerMap[owner] = true
			uniqueOwners = append(uniqueOwners, owner)
		}
	}

	if len(uniqueOwners) > 0 {
		// 值班人员：多个负责人用逗号分隔（保留此字段用于兼容）
		alert.Labels["cmdb_owners"] = strings.Join(uniqueOwners, ", ")
	}

	return nil
}

// getDutyUserFromCmdbOrCalendar 获取值班人员
// 合并CMDB负责人信息和值班日历的值班人员
func getDutyUserFromCmdbOrCalendar(ctx *ctx.Context, event *models.AlertCurEvent, noticeData models.AlertNotice) string {
	// 获取值班日历的值班人员
	dutyUsers := GetDutyUsers(ctx, noticeData)
	dutyUsersStr := strings.Join(dutyUsers, " ")

	// 尝试从CMDB获取负责人信息
	cmdbOwners := extractCmdbOwners(event)

	// 合并CMDB负责人和值班日历的值班人员
	if cmdbOwners != "" && dutyUsersStr != "" && dutyUsersStr != "暂无" {
		// 如果两者都有，合并显示（CMDB负责人在前，值班人员在后）
		return cmdbOwners + " " + dutyUsersStr
	}

	// 如果只有CMDB负责人，返回CMDB负责人
	if cmdbOwners != "" {
		return cmdbOwners
	}

	// 如果只有值班人员，返回值班人员
	return dutyUsersStr
}

// extractCmdbOwners 从告警事件中提取CMDB负责人信息
// 返回格式化后的负责人字符串（@用户名格式），如果没有则返回空字符串
func extractCmdbOwners(event *models.AlertCurEvent) string {
	if event.Labels == nil {
		return ""
	}

	cmdbOwners, exists := event.Labels["cmdb_owners"]
	if !exists {
		return ""
	}

	ownersStr, ok := cmdbOwners.(string)
	if !ok || ownersStr == "" {
		return ""
	}

	// 格式化负责人信息：添加@前缀，用空格分隔
	ownersList := strings.Split(ownersStr, ",")
	formattedOwners := make([]string, 0, len(ownersList))

	for _, owner := range ownersList {
		owner = strings.TrimSpace(owner)
		if owner == "" {
			continue
		}

		// 如果用户名不包含@，添加@前缀
		if !strings.HasPrefix(owner, "@") {
			owner = "@" + owner
		}
		formattedOwners = append(formattedOwners, owner)
	}

	if len(formattedOwners) == 0 {
		return ""
	}

	return strings.Join(formattedOwners, " ")
}

// extractIPFromInstance 从instance字符串中提取IP地址
// 支持格式: "10.10.217.225:9100" -> "10.10.217.225"
// 如果已经是IP格式，直接返回
func extractIPFromInstance(instance string) string {
	if instance == "" {
		return ""
	}

	// 如果包含冒号，提取IP部分
	if strings.Contains(instance, ":") {
		parts := strings.Split(instance, ":")
		if len(parts) > 0 {
			return strings.TrimSpace(parts[0])
		}
	}

	// 直接返回（已经是IP格式）
	return strings.TrimSpace(instance)
}
