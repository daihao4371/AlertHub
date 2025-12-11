package repo

import (
	"alertHub/internal/models"

	"gorm.io/gorm"
)

// EnrichUpdateByRealName enriches UpdateByRealName field for a slice of items
// This function performs batch query to avoid N+1 query problem
// Supported types: *[]models.AlertNotice, *[]models.NoticeTemplateExample, *[]models.AlertDataSource, *[]models.ProbingRule, *[]models.AlertSilences, *[]models.AlertRule
func EnrichUpdateByRealName(db *gorm.DB, items interface{}) {
	if items == nil {
		return
	}

	// Collect unique usernames that need realName enrichment
	usernamesMap := make(map[string]bool)

	// Handle different slice pointer types
	switch v := items.(type) {
	case *[]models.AlertNotice:
		if v == nil || len(*v) == 0 {
			return
		}
		for _, item := range *v {
			if item.UpdateBy != "" {
				usernamesMap[item.UpdateBy] = true
			}
		}
		// Batch query and enrich
		usernameToRealNameMap := batchQueryRealNames(db, usernamesMap)
		for i := range *v {
			if (*v)[i].UpdateBy != "" {
				if realName, exists := usernameToRealNameMap[(*v)[i].UpdateBy]; exists {
					(*v)[i].UpdateByRealName = realName
				}
			}
		}
	case *[]models.NoticeTemplateExample:
		if v == nil || len(*v) == 0 {
			return
		}
		for _, item := range *v {
			if item.UpdateBy != "" {
				usernamesMap[item.UpdateBy] = true
			}
		}
		usernameToRealNameMap := batchQueryRealNames(db, usernamesMap)
		for i := range *v {
			if (*v)[i].UpdateBy != "" {
				if realName, exists := usernameToRealNameMap[(*v)[i].UpdateBy]; exists {
					(*v)[i].UpdateByRealName = realName
				}
			}
		}
	case *[]models.AlertDataSource:
		if v == nil || len(*v) == 0 {
			return
		}
		for _, item := range *v {
			if item.UpdateBy != "" {
				usernamesMap[item.UpdateBy] = true
			}
		}
		usernameToRealNameMap := batchQueryRealNames(db, usernamesMap)
		for i := range *v {
			if (*v)[i].UpdateBy != "" {
				if realName, exists := usernameToRealNameMap[(*v)[i].UpdateBy]; exists {
					(*v)[i].UpdateByRealName = realName
				}
			}
		}
	case *[]models.ProbingRule:
		if v == nil || len(*v) == 0 {
			return
		}
		for _, item := range *v {
			if item.UpdateBy != "" {
				usernamesMap[item.UpdateBy] = true
			}
		}
		usernameToRealNameMap := batchQueryRealNames(db, usernamesMap)
		for i := range *v {
			if (*v)[i].UpdateBy != "" {
				if realName, exists := usernameToRealNameMap[(*v)[i].UpdateBy]; exists {
					(*v)[i].UpdateByRealName = realName
				}
			}
		}
	case *[]models.AlertSilences:
		if v == nil || len(*v) == 0 {
			return
		}
		for _, item := range *v {
			if item.UpdateBy != "" {
				usernamesMap[item.UpdateBy] = true
			}
		}
		usernameToRealNameMap := batchQueryRealNames(db, usernamesMap)
		for i := range *v {
			if (*v)[i].UpdateBy != "" {
				if realName, exists := usernameToRealNameMap[(*v)[i].UpdateBy]; exists {
					(*v)[i].UpdateByRealName = realName
				}
			}
		}
	case *[]models.AlertRule:
		if v == nil || len(*v) == 0 {
			return
		}
		for _, item := range *v {
			if item.UpdateBy != "" {
				usernamesMap[item.UpdateBy] = true
			}
		}
		usernameToRealNameMap := batchQueryRealNames(db, usernamesMap)
		for i := range *v {
			if (*v)[i].UpdateBy != "" {
				if realName, exists := usernameToRealNameMap[(*v)[i].UpdateBy]; exists {
					(*v)[i].UpdateByRealName = realName
				}
			}
		}
	default:
		// Unsupported type, silently return
		return
	}
}

// batchQueryRealNames performs batch query to get realName by usernames
func batchQueryRealNames(db *gorm.DB, usernamesMap map[string]bool) map[string]string {
	usernameToRealNameMap := make(map[string]string)
	if len(usernamesMap) == 0 {
		return usernameToRealNameMap
	}

	usernames := make([]string, 0, len(usernamesMap))
	for username := range usernamesMap {
		usernames = append(usernames, username)
	}

	var users []models.Member
	db.Model(&models.Member{}).Where("user_name IN ?", usernames).Find(&users)
	for _, user := range users {
		usernameToRealNameMap[user.UserName] = user.RealName
	}

	return usernameToRealNameMap
}

// EnrichUsernameRealName enriches UsernameRealName field for a slice of AuditLog items
// This function performs batch query to avoid N+1 query problem
func EnrichUsernameRealName(db *gorm.DB, items *[]models.AuditLog) {
	if items == nil || len(*items) == 0 {
		return
	}

	// Collect unique usernames that need realName enrichment
	usernamesMap := make(map[string]bool)
	for _, item := range *items {
		if item.Username != "" {
			usernamesMap[item.Username] = true
		}
	}

	// Batch query and enrich
	usernameToRealNameMap := batchQueryRealNames(db, usernamesMap)
	for i := range *items {
		if (*items)[i].Username != "" {
			if realName, exists := usernameToRealNameMap[(*items)[i].Username]; exists {
				(*items)[i].UsernameRealName = realName
			}
		}
	}
}

// EnrichManagerRealName enriches ManagerRealName field for a slice of Tenant items
// This function performs batch query to avoid N+1 query problem
func EnrichManagerRealName(db *gorm.DB, items *[]models.Tenant) {
	if items == nil || len(*items) == 0 {
		return
	}

	// Collect unique usernames that need realName enrichment
	usernamesMap := make(map[string]bool)
	for _, item := range *items {
		if item.Manager != "" {
			usernamesMap[item.Manager] = true
		}
	}

	// Batch query and enrich
	usernameToRealNameMap := batchQueryRealNames(db, usernamesMap)
	for i := range *items {
		if (*items)[i].Manager != "" {
			if realName, exists := usernameToRealNameMap[(*items)[i].Manager]; exists {
				(*items)[i].ManagerRealName = realName
			}
		}
	}
}
