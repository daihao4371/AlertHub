package repo

import (
	"context"
	"alertHub/internal/models"

	"github.com/zeromicro/go-zero/core/logc"
	"gorm.io/gorm"
)

type (
	TenantRepo struct {
		entryRepo
	}

	InterTenantRepo interface {
		Create(t models.Tenant) error
		Update(t models.Tenant) error
		Delete(tenantId string) error
		List(userId string) (data []models.Tenant, err error)
		GetAll() (data []models.Tenant, err error)
		Get(tenantId string) (data models.Tenant, err error)
		CreateTenantLinkedUserRecord(t models.TenantLinkedUsers) error
		AddTenantLinkedUsers(tenantId string, users []models.TenantUser, userRole string) error
		RemoveTenantLinkedUsers(tenantId, userId string) error
		GetTenantLinkedUsers(tenantId string) (models.TenantLinkedUsers, error)
		DelTenantLinkedUserRecord(tenantId string) error
		GetTenantLinkedUserInfo(tenantId, userId string) (models.TenantUser, error)
		ChangeTenantUserRole(tenantId, userId, userRole string) error
	}
)

func newTenantInterface(db *gorm.DB, g InterGormDBCli) InterTenantRepo {
	return &TenantRepo{
		entryRepo{
			g:  g,
			db: db,
		},
	}
}

func (tr TenantRepo) Create(t models.Tenant) error {
	err := tr.g.Create(&models.Tenant{}, t)
	if err != nil {
		return err
	}

	// 尝试关联admin用户到新租户（如果admin用户存在）
	adminUser, exists, err := tr.User().Get("admin", "", "")
	if err == nil && exists {
		err = tr.Tenant().CreateTenantLinkedUserRecord(
			models.TenantLinkedUsers{
				ID: t.ID,
				Users: []models.TenantUser{
					{
						UserID:   adminUser.UserId,
						UserName: adminUser.UserName,
						UserRole: "admin",
					},
				}})
		if err != nil {
			logc.Errorf(context.Background(), "创建租户关联admin用户失败: %s", err.Error())
			// 不返回错误，继续创建租户
		}

		// 更新admin用户的租户列表
		if !tr.containsTenant(adminUser.Tenants, t.ID) {
			adminUser.Tenants = append(adminUser.Tenants, t.ID)
			_ = tr.g.Updates(Updates{
				Table: models.Member{},
				Where: map[string]interface{}{
					"user_id = ?": adminUser.UserId,
				},
				Updates: adminUser,
			})
		}
	}

	return nil
}

// containsTenant 检查租户是否已在列表中
func (tr TenantRepo) containsTenant(tenants []string, tenantId string) bool {
	for _, t := range tenants {
		if t == tenantId {
			return true
		}
	}
	return false
}

func (tr TenantRepo) Update(t models.Tenant) error {
	u := Updates{
		Table: &models.Tenant{},
		Where: map[string]interface{}{
			"id = ?": t.ID,
		},
		Updates: t,
	}
	err := tr.g.Updates(u)
	if err != nil {
		logc.Error(context.Background(), err)
		return err
	}
	return nil
}

func (tr TenantRepo) Delete(tenantId string) error {
	getTenant, err := tr.Tenant().GetTenantLinkedUsers(tenantId)
	if err != nil {
		return err
	}

	for _, u := range getTenant.Users {
		err := tr.Tenant().RemoveTenantLinkedUsers(tenantId, u.UserID)
		if err != nil {
			return err
		}
	}

	err = tr.Tenant().DelTenantLinkedUserRecord(tenantId)
	if err != nil {
		return err
	}

	err = tr.g.Delete(Delete{
		Table: &models.Tenant{},
		Where: map[string]interface{}{
			"id = ?": tenantId,
		},
	})
	if err != nil {
		logc.Error(context.Background(), err)
		return err
	}
	return nil
}

func (tr TenantRepo) List(userId string) (data []models.Tenant, err error) {
	getUser, _, err := tr.User().Get(userId, "", "")
	if err != nil {
		return nil, err
	}

	var ts = &[]models.Tenant{}
	for _, tid := range getUser.Tenants {
		getT, err := tr.Tenant().Get(tid)
		// 如果租户不存在（可能已被删除），跳过该租户，继续处理其他租户
		if err != nil {
			if err == gorm.ErrRecordNotFound {
				logc.Infof(context.Background(), "租户 %s 不存在，已跳过", tid)
				continue
			}
			// 其他错误直接返回
			return nil, err
		}
		*ts = append(*ts, getT)
	}

	// Enrich ManagerRealName using common function
	EnrichManagerRealName(tr.db, ts)

	return *ts, nil
}

func (tr TenantRepo) Get(tenantId string) (data models.Tenant, err error) {
	var d models.Tenant
	err = tr.db.Model(&models.Tenant{}).Where("id = ?", tenantId).First(&d).Error
	if err != nil {
		return d, err
	}

	// 查询并填充 ManagerRealName
	if d.Manager != "" {
		var member models.Member
		err = tr.db.Model(&models.Member{}).Where("user_name = ?", d.Manager).First(&member).Error
		if err == nil {
			d.ManagerRealName = member.RealName
		}
	}

	return d, nil
}

// GetAll 获取所有租户列表
// 供 Exporter 调度器等系统模块使用，查询所有租户执行全局任务
func (tr TenantRepo) GetAll() (data []models.Tenant, err error) {
	var tenants []models.Tenant
	err = tr.db.Model(&models.Tenant{}).Find(&tenants).Error
	if err != nil {
		return nil, err
	}

	return tenants, nil
}

// CreateTenantLinkedUserRecord 创建租户关联的用户记录
func (tr TenantRepo) CreateTenantLinkedUserRecord(t models.TenantLinkedUsers) error {
	err := tr.g.Create(&models.TenantLinkedUsers{}, t)
	if err != nil {
		logc.Error(context.Background(), err)
		return err
	}
	return nil
}

// AddTenantLinkedUsers 新增租户用户数据
func (tr TenantRepo) AddTenantLinkedUsers(tenantId string, users []models.TenantUser, userRole string) error {
	oldTenantUsers, err := tr.Tenant().GetTenantLinkedUsers(tenantId)
	if err != nil {
		return err
	}

	// 在新增成员时不会一并将角色写入，需要找到新增的成员，并且修改它的角色。
	for _, nUser := range users {
		found := false
		for _, oUser := range oldTenantUsers.Users {
			if oUser.UserID == nUser.UserID {
				found = true
				break
			}
		}
		if !found {
			oldTenantUsers.Users = append(oldTenantUsers.Users, models.TenantUser{
				UserID:   nUser.UserID,
				UserName: nUser.UserName,
				UserRole: userRole,
			})
		}
	}

	// 更新租户表
	err = tr.g.Updates(Updates{
		Table: models.TenantLinkedUsers{},
		Where: map[string]interface{}{
			"id = ?": tenantId,
		},
		Updates: oldTenantUsers,
	})
	if err != nil {
		return err
	}

	// 更新用户表，新增租户ID
	for _, u := range users {
		userData, _, err := tr.User().Get(u.UserID, "", "")
		if err != nil {
			return err
		}

		var exist bool
		for _, tid := range userData.Tenants {
			if tid == tenantId {
				exist = true
			}
		}

		if !exist {
			userData.Tenants = append(userData.Tenants, tenantId)
		}
		err = tr.g.Updates(Updates{
			Table: models.Member{},
			Where: map[string]interface{}{
				"user_id = ?": u.UserID,
			},
			Updates: userData,
		})
		if err != nil {
			return err
		}
	}

	return nil
}

// RemoveTenantLinkedUsers 移除租户关联的用户数据
func (tr TenantRepo) RemoveTenantLinkedUsers(tenantId, userId string) error {
	record, err := tr.GetTenantLinkedUsers(tenantId)
	if err != nil {
		return err
	}

	var newRecord []models.TenantUser
	// 移除租户中当前选择的用户，保留其他用户
	for _, u := range record.Users {
		if u.UserID == userId {
			continue
		}
		newRecord = append(newRecord, u)
	}
	record.Users = newRecord

	err = tr.g.Updates(Updates{
		Table: models.TenantLinkedUsers{},
		Where: map[string]interface{}{
			"id = ?": tenantId,
		},
		Updates: record,
	})
	if err != nil {
		return err
	}

	// 获取当前选择的用户详情
	userData, _, err := tr.User().Get(userId, "", "")
	if err != nil {
		return err
	}

	var newTenants = &[]string{}
	// 删除当前选择的租户，保留其他租户
	for _, tid := range userData.Tenants {
		if tid == tenantId {
			continue
		}
		*newTenants = append(*newTenants, tid)
	}

	userData.Tenants = *newTenants
	err = tr.g.Updates(Updates{
		Table: models.Member{},
		Where: map[string]interface{}{
			"user_id = ?": userId,
		},
		Updates: userData,
	})
	if err != nil {
		return err
	}

	return nil
}

// GetTenantLinkedUsers 获取租户关联的用户数据
func (tr TenantRepo) GetTenantLinkedUsers(tenantId string) (models.TenantLinkedUsers, error) {
	var d models.TenantLinkedUsers
	err := tr.db.Model(&models.TenantLinkedUsers{}).Where("id = ?", tenantId).First(&d).Error
	if err != nil {
		return d, err
	}

	// 批量查询用户真实姓名并填充
	if len(d.Users) > 0 {
		// 收集所有用户名
		userNamesMap := make(map[string]bool)
		for _, user := range d.Users {
			if user.UserName != "" {
				userNamesMap[user.UserName] = true
			}
		}

		// 批量查询真实姓名
		if len(userNamesMap) > 0 {
			userNames := make([]string, 0, len(userNamesMap))
			for userName := range userNamesMap {
				userNames = append(userNames, userName)
			}

			var members []models.Member
			tr.db.Model(&models.Member{}).Where("user_name IN ?", userNames).Find(&members)

			// 创建用户名到真实姓名的映射
			userNameToRealNameMap := make(map[string]string)
			for _, member := range members {
				userNameToRealNameMap[member.UserName] = member.RealName
			}

			// 填充真实姓名
			for i := range d.Users {
				if realName, exists := userNameToRealNameMap[d.Users[i].UserName]; exists {
					d.Users[i].RealName = realName
				}
			}
		}
	}

	return d, nil
}

// DelTenantLinkedUserRecord 删除租户关联表记录
func (tr TenantRepo) DelTenantLinkedUserRecord(tenantId string) error {
	err := tr.g.Delete(Delete{
		Table: &models.TenantLinkedUsers{},
		Where: map[string]interface{}{
			"id = ?": tenantId,
		},
	})
	if err != nil {
		logc.Error(context.Background(), err)
		return err
	}

	return nil
}

// GetTenantLinkedUserInfo 获取租户关联用户的详细信息
func (tr TenantRepo) GetTenantLinkedUserInfo(tenantId, userId string) (models.TenantUser, error) {
	var (
		tlu models.TenantLinkedUsers
		tu  models.TenantUser
	)

	err := tr.db.Model(&models.TenantLinkedUsers{}).Where("id = ?", tenantId).First(&tlu).Error
	if err != nil {
		return tu, err
	}

	for _, u := range tlu.Users {
		if u.UserID == userId {
			tu = u
			break
		}
	}

	return tu, nil
}

// ChangeTenantUserRole 修改用户角色
func (tr TenantRepo) ChangeTenantUserRole(tenantId, userId, userRole string) error {
	tenant, err := tr.GetTenantLinkedUsers(tenantId)
	if err != nil {
		return err
	}

	var users []models.TenantUser
	for _, u := range tenant.Users {
		if u.UserID != userId {
			users = append(users, u)
		} else {
			u.UserRole = userRole
			users = append(users, u)
		}
	}

	tenant.Users = users
	err = tr.g.Updates(Updates{
		Table: models.TenantLinkedUsers{},
		Where: map[string]interface{}{
			"id = ?": tenantId,
		},
		Updates: tenant,
	})
	if err != nil {
		return err
	}

	return nil
}
