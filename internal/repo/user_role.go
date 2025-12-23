package repo

import (
	"gorm.io/gorm"
	"alertHub/internal/models"
)

type (
	UserRoleRepo struct {
		entryRepo
	}

	InterUserRoleRepo interface {
		List() ([]models.UserRole, error)
		Create(r models.UserRole) error
		Update(r models.UserRole) error
		Delete(id string) error
		
		// 角色API权限管理方法
		SetRoleApis(roleID string, apiIDs []int64) error      // 为角色设置API权限
		GetRoleApis(roleID string) ([]models.SysApi, error)  // 获取角色的API权限
		RemoveRoleApis(roleID string) error                  // 移除角色的所有API权限
		HasRoleApi(roleID string, apiPath, method string) (bool, error) // 检查角色是否有指定API权限
	}
)

func newUserRoleInterface(db *gorm.DB, g InterGormDBCli) InterUserRoleRepo {
	return &UserRoleRepo{
		entryRepo{
			g:  g,
			db: db,
		},
	}
}

func (ur UserRoleRepo) List() ([]models.UserRole, error) {
	var (
		data []models.UserRole
		db   = ur.DB().Model(&models.UserRole{})
	)

	err := db.Where("id != ?", "admin").Find(&data).Error
	if err != nil {
		return data, err
	}

	return data, nil
}

func (ur UserRoleRepo) Create(r models.UserRole) error {
	err := ur.g.Create(models.UserRole{}, r)
	if err != nil {
		return err
	}

	return nil
}

func (ur UserRoleRepo) Update(r models.UserRole) error {
	u := Updates{
		Table: models.UserRole{},
		Where: map[string]interface{}{
			"id = ?": r.ID,
		},
		Updates: r,
	}

	err := ur.g.Updates(u)
	if err != nil {
		return err
	}

	return nil
}

func (ur UserRoleRepo) Delete(id string) error {
	// 使用事务确保数据一致性
	db := ur.DB().Begin()
	defer func() {
		if r := recover(); r != nil {
			db.Rollback()
		}
	}()

	// 1. 先删除角色的所有API权限关联
	if err := db.Where("user_role_id = ?", id).Delete(&models.UserRoleApi{}).Error; err != nil {
		db.Rollback()
		return err
	}

	// 2. 删除Casbin权限策略
	if err := db.Where("v0 = ?", id).Delete(&models.CasbinRule{}).Error; err != nil {
		db.Rollback()
		return err
	}

	// 3. 最后删除角色本身
	if err := db.Where("id = ?", id).Delete(&models.UserRole{}).Error; err != nil {
		db.Rollback()
		return err
	}

	return db.Commit().Error
}

// SetRoleApis 为角色设置API权限(覆盖式)
func (ur UserRoleRepo) SetRoleApis(roleID string, apiIDs []int64) error {
	// 开启事务
	db := ur.DB().Begin()
	defer func() {
		if r := recover(); r != nil {
			db.Rollback()
		}
	}()

	// 1. 删除原有角色API权限关联
	if err := db.Where("user_role_id = ?", roleID).Delete(&models.UserRoleApi{}).Error; err != nil {
		db.Rollback()
		return err
	}

	// 2. 批量插入新的权限关联
	if len(apiIDs) > 0 {
		var roleApis []models.UserRoleApi
		for _, apiID := range apiIDs {
			roleApis = append(roleApis, models.UserRoleApi{
				RoleID: roleID,
				ApiID:  apiID,
			})
		}
		if err := db.Create(&roleApis).Error; err != nil {
			db.Rollback()
			return err
		}
	}

	return db.Commit().Error
}

// GetRoleApis 获取角色的API权限
func (ur UserRoleRepo) GetRoleApis(roleID string) ([]models.SysApi, error) {
	var apis []models.SysApi
	err := ur.DB().Table("sys_apis").
		Joins("JOIN user_role_apis ON user_role_apis.sys_api_id = sys_apis.id").
		Where("user_role_apis.user_role_id = ? AND sys_apis.enabled = ?", roleID, true).
		Find(&apis).Error
	
	return apis, err
}

// RemoveRoleApis 移除角色的所有API权限
func (ur UserRoleRepo) RemoveRoleApis(roleID string) error {
	return ur.DB().Where("user_role_id = ?", roleID).Delete(&models.UserRoleApi{}).Error
}

// HasRoleApi 检查角色是否有指定API权限
func (ur UserRoleRepo) HasRoleApi(roleID string, apiPath, method string) (bool, error) {
	var count int64
	err := ur.DB().Table("sys_apis").
		Joins("JOIN user_role_apis ON user_role_apis.sys_api_id = sys_apis.id").
		Where("user_role_apis.user_role_id = ? AND sys_apis.path = ? AND sys_apis.method = ? AND sys_apis.enabled = ?", 
			roleID, apiPath, method, true).
		Count(&count).Error
	
	return count > 0, err
}
