package repo

import (
	"alertHub/internal/models"
	"alertHub/internal/types"

	"gorm.io/gorm"
)

type (
	EventRepo struct {
		entryRepo
	}

	InterEventRepo interface {
		GetHistoryEvent(r types.RequestAlertHisEventQuery) (types.ResponseHistoryEventList, error)
		CreateHistoryEvent(r models.AlertHisEvent) error
	}
)

func newEventInterface(db *gorm.DB, g InterGormDBCli) InterEventRepo {
	return &EventRepo{
		entryRepo{
			g:  g,
			db: db,
		},
	}
}

func (e EventRepo) GetHistoryEvent(r types.RequestAlertHisEventQuery) (types.ResponseHistoryEventList, error) {
	var data []models.AlertHisEvent
	var count int64

	db := e.DB().Model(&models.AlertHisEvent{})
	db.Where("tenant_id = ?", r.TenantId)
	db.Where("fault_center_id = ?", r.FaultCenterId)

	if r.Query != "" {
		db.Where("rule_name LIKE ? OR severity LIKE ? OR annotations LIKE ? OR fingerprint LIKE ?", "%"+r.Query+"%", "%"+r.Query+"%", "%"+r.Query+"%", "%"+r.Query+"%")
	}

	if r.DatasourceType != "" {
		db = db.Where("datasource_type = ?", r.DatasourceType)
	}

	if r.Severity != "" {
		db = db.Where("severity = ?", r.Severity)
	}

	if r.StartAt != 0 && r.EndAt != 0 {
		db = db.Where("first_trigger_time > ? and first_trigger_time < ?", r.StartAt, r.EndAt)
	}

	if err := db.Count(&count).Error; err != nil {
		return types.ResponseHistoryEventList{}, err
	}

	switch r.SortOrder {
	case models.SortOrderASC:
		db.Order("alarm_duration asc")
	case models.SortOrderDesc:
		db.Order("alarm_duration desc")
	default:
		db.Order("recover_time desc")
	}

	if err := db.Limit(int(r.Page.Size)).Offset(int((r.Page.Index - 1) * r.Page.Size)).Find(&data).Error; err != nil {
		return types.ResponseHistoryEventList{}, err
	}

	// 批量查询并填充认领人真实姓名
	// 只处理已认领的告警（IsOk = true 且 ConfirmUsername 不为空）
	if len(data) > 0 {
		// 收集所有需要查询的用户名（只收集已认领的）
		usernamesMap := make(map[string]bool)
		for _, event := range data {
			// 只处理已认领的告警
			if event.ConfirmState.IsOk && event.ConfirmState.ConfirmUsername != "" {
				usernamesMap[event.ConfirmState.ConfirmUsername] = true
			}
		}

		// 批量查询真实姓名
		if len(usernamesMap) > 0 {
			usernames := make([]string, 0, len(usernamesMap))
			for username := range usernamesMap {
				usernames = append(usernames, username)
			}

			var members []models.Member
			e.DB().Model(&models.Member{}).Where("user_name IN ?", usernames).Find(&members)

			// 创建用户名到真实姓名的映射
			usernameToRealNameMap := make(map[string]string)
			for _, member := range members {
				usernameToRealNameMap[member.UserName] = member.RealName
			}

			// 填充真实姓名（只填充已认领的告警）
			for i := range data {
				// 只处理已认领的告警
				if data[i].ConfirmState.IsOk && data[i].ConfirmState.ConfirmUsername != "" {
					if realName, exists := usernameToRealNameMap[data[i].ConfirmState.ConfirmUsername]; exists {
						data[i].ConfirmState.ConfirmUsernameRealName = realName
					}
				}
			}
		}
	}

	return types.ResponseHistoryEventList{
		List: data,
		Page: models.Page{
			Index: r.Page.Index,
			Size:  r.Page.Size,
			Total: count,
		},
	}, nil
}

func (e EventRepo) CreateHistoryEvent(r models.AlertHisEvent) error {
	err := e.g.Create(models.AlertHisEvent{}, r)
	if err != nil {
		return err
	}

	return nil
}
