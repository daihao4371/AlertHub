package services

import (
	"alertHub/internal/ctx"
	models "alertHub/internal/models"
	"alertHub/internal/types"
	"alertHub/pkg/tools"
	"fmt"
	"time"
)

type userRoleService struct {
	ctx *ctx.Context
}

type InterUserRoleService interface {
	List(req interface{}) (interface{}, interface{})
	Create(req interface{}) (interface{}, interface{})
	Update(req interface{}) (interface{}, interface{})
	Delete(req interface{}) (interface{}, interface{})

	// 角色权限管理接口
	SetRolePermissions(roleID string, apiIDs []int64) error                  // 为角色分配权限
	GetRolePermissions(roleID string) ([]models.SysApi, error)               // 获取角色权限
	CheckUserPermission(userID string, apiPath, method string) (bool, error) // 检查用户权限
	GetUserRoles(userID string) ([]models.UserRole, error)                   // 获取用户角色列表
}

func newInterUserRoleService(ctx *ctx.Context) InterUserRoleService {
	return &userRoleService{
		ctx: ctx,
	}
}

func (ur userRoleService) List(req interface{}) (interface{}, interface{}) {
	data, err := ur.ctx.DB.UserRole().List()
	if err != nil {
		return nil, err
	}

	return data, nil
}

func (ur userRoleService) Create(req interface{}) (interface{}, interface{}) {
	r := req.(*types.RequestUserRoleCreate)
	
	// Initialize enabled as true by default
	enabled := true

	err := ur.ctx.DB.UserRole().Create(models.UserRole{
		ID:          "ur-" + tools.RandId(),
		Name:        r.Name,
		Description: r.Description,
		Enabled:     &enabled, // 显式初始化指针字段
		UpdateAt:    time.Now().Unix(),
	})
	if err != nil {
		return nil, err
	}

	return nil, nil
}

func (ur userRoleService) Update(req interface{}) (interface{}, interface{}) {
	r := req.(*types.RequestUserRoleUpdate)

	err := ur.ctx.DB.UserRole().Update(models.UserRole{
		ID:          r.ID,
		Name:        r.Name,
		Description: r.Description,
		UpdateAt:    time.Now().Unix(),
	})
	if err != nil {
		return nil, err
	}

	return nil, nil
}

func (ur userRoleService) Delete(req interface{}) (interface{}, interface{}) {
	r := req.(*types.RequestUserRoleQuery)
	err := ur.ctx.DB.UserRole().Delete(r.ID)
	if err != nil {
		return nil, err
	}

	return nil, nil
}

// SetRolePermissions 为角色分配API权限
func (ur userRoleService) SetRolePermissions(roleID string, apiIDs []int64) error {
	return ur.ctx.DB.UserRole().SetRoleApis(roleID, apiIDs)
}

// GetRolePermissions 获取角色的API权限
func (ur userRoleService) GetRolePermissions(roleID string) ([]models.SysApi, error) {
	return ur.ctx.DB.UserRole().GetRoleApis(roleID)
}

// CheckUserPermission 检查用户是否拥有指定API权限
func (ur userRoleService) CheckUserPermission(userID string, apiPath, method string) (bool, error) {
	// 1. 获取用户角色列表
	userRoles, err := ur.GetUserRoles(userID)
	if err != nil {
		return false, err
	}

	if len(userRoles) == 0 {
		return false, nil
	}

	// 2. 检查用户的各个角色是否有该API权限
	for _, role := range userRoles {
		// 跳过禁用的角色
		if !role.IsEnabled() {
			continue
		}

		hasPermission, err := ur.ctx.DB.UserRole().HasRoleApi(role.ID, apiPath, method)
		if err != nil {
			return false, err
		}

		// 只要有一个角色拥有权限就返回true
		if hasPermission {
			return true, nil
		}
	}

	return false, nil
}

// GetUserRoles 获取用户的角色列表
func (ur userRoleService) GetUserRoles(userID string) ([]models.UserRole, error) {
	// 1. 获取用户信息
	user, exists, err := ur.ctx.DB.User().Get(userID, "", "")
	if err != nil {
		return nil, err
	}

	if !exists {
		return nil, fmt.Errorf("用户不存在: %s", userID)
	}

	// 2. 如果用户没有分配角色，返回空列表
	if user.Role == "" {
		return []models.UserRole{}, nil
	}

	// 3. 根据用户的角色ID查询角色信息
	// 需要通过repo层来访问数据库，而不是直接使用gorm
	allRoles, err := ur.ctx.DB.UserRole().List()
	if err != nil {
		return nil, err
	}

	// 4. 筛选出用户的角色并过滤启用状态
	var userRoles []models.UserRole
	for _, role := range allRoles {
		if role.ID == user.Role && role.IsEnabled() {
			userRoles = append(userRoles, role)
			break // 用户只能有一个角色，找到后退出循环
		}
	}

	return userRoles, nil
}
