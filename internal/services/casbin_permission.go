package services

import (
	"errors"
	"fmt"
	"sync"
	
	"github.com/casbin/casbin/v2"
	"github.com/casbin/casbin/v2/model"
	gormadapter "github.com/casbin/gorm-adapter/v3"
	"gorm.io/gorm"
	
	"alertHub/internal/ctx"
	"alertHub/internal/models"
)

type casbinService struct {
	ctx *ctx.Context
}

type InterCasbinService interface {
	// 核心权限验证方法
	CheckPermission(roleID, apiPath, method string) (bool, error)
	
	// 权限管理方法
	SetRolePermissions(roleID string, permissions []models.PermissionInfo) error
	GetRolePermissions(roleID string) ([]models.PermissionInfo, error)
	RemoveRolePermissions(roleID string) error
	
	// 用户权限方法
	CheckUserPermission(userID, tenantID, apiPath, method string) (bool, error)
	GetUserAllPermissions(userID, tenantID string) ([]models.PermissionInfo, error)
	
	// API权限管理
	GetAllApiPermissions() ([]models.SysApi, error)
	EnsureAllApisRegistered() error
	InitAdminPermissions() error
	
	// Casbin实例获取
	GetEnforcer() (*casbin.SyncedCachedEnforcer, error)
}

var (
	syncedCachedEnforcer *casbin.SyncedCachedEnforcer
	casbinOnce          sync.Once
)

func newInterCasbinService(ctx *ctx.Context) InterCasbinService {
	return &casbinService{
		ctx: ctx,
	}
}

// GetEnforcer 获取Casbin执行器(单例模式)
func (c *casbinService) GetEnforcer() (*casbin.SyncedCachedEnforcer, error) {
	var err error
	casbinOnce.Do(func() {
		// 1. 创建GORM适配器
		adapter, adapterErr := gormadapter.NewAdapterByDB(c.ctx.DB.DB())
		if adapterErr != nil {
			err = fmt.Errorf("创建Casbin适配器失败: %v", adapterErr)
			return
		}

		// 2. 定义RBAC模型
		modelText := `
		[request_definition]
		r = sub, obj, act

		[policy_definition]  
		p = sub, obj, act

		[role_definition]
		g = _, _

		[policy_effect]
		e = some(where (p.eft == allow))

		[matchers]
		m = r.sub == p.sub && keyMatch2(r.obj, p.obj) && r.act == p.act
		`

		// 3. 创建模型
		m, modelErr := model.NewModelFromString(modelText)
		if modelErr != nil {
			err = fmt.Errorf("创建Casbin模型失败: %v", modelErr)
			return
		}

		// 4. 创建同步缓存执行器
		enforcer, enforcerErr := casbin.NewSyncedCachedEnforcer(m, adapter)
		if enforcerErr != nil {
			err = fmt.Errorf("创建Casbin执行器失败: %v", enforcerErr)
			return
		}

		// 5. 配置执行器
		enforcer.SetExpireTime(60 * 60) // 缓存1小时
		if loadErr := enforcer.LoadPolicy(); loadErr != nil {
			err = fmt.Errorf("加载Casbin策略失败: %v", loadErr)
			return
		}

		syncedCachedEnforcer = enforcer
	})

	return syncedCachedEnforcer, err
}

// CheckPermission 检查角色是否有指定API权限
func (c *casbinService) CheckPermission(roleID, apiPath, method string) (bool, error) {
	enforcer, err := c.GetEnforcer()
	if err != nil {
		return false, err
	}

	// 执行权限检查
	result, err := enforcer.Enforce(roleID, apiPath, method)
	return result, err
}

// SetRolePermissions 为角色设置API权限(覆盖式)
func (c *casbinService) SetRolePermissions(roleID string, permissions []models.PermissionInfo) error {
	enforcer, err := c.GetEnforcer()
	if err != nil {
		return err
	}

	// 1. 清除角色的所有现有权限
	_, err = enforcer.RemoveFilteredPolicy(0, roleID)
	if err != nil {
		return fmt.Errorf("清除角色权限失败: %v", err)
	}

	// 2. 权限去重和批量添加
	deduplicateMap := make(map[string]bool)
	var rules [][]string

	for _, perm := range permissions {
		// 构建唯一键用于去重
		key := roleID + "|" + perm.Path + "|" + perm.Method
		if _, exists := deduplicateMap[key]; !exists {
			deduplicateMap[key] = true
			rules = append(rules, []string{roleID, perm.Path, perm.Method})
		}
	}

	// 3. 批量添加权限规则
	if len(rules) > 0 {
		_, err = enforcer.AddPolicies(rules)
		if err != nil {
			return fmt.Errorf("添加角色权限失败: %v", err)
		}
	}

	return nil
}

// GetRolePermissions 获取角色的所有权限
func (c *casbinService) GetRolePermissions(roleID string) ([]models.PermissionInfo, error) {
	enforcer, err := c.GetEnforcer()
	if err != nil {
		return nil, err
	}

	// 获取角色的所有权限策略
	policies, _ := enforcer.GetFilteredPolicy(0, roleID)
	
	var permissions []models.PermissionInfo
	for _, policy := range policies {
		if len(policy) >= 3 {
			permissions = append(permissions, models.PermissionInfo{
				Path:   policy[1],
				Method: policy[2],
			})
		}
	}

	return permissions, nil
}

// RemoveRolePermissions 移除角色的所有权限
func (c *casbinService) RemoveRolePermissions(roleID string) error {
	enforcer, err := c.GetEnforcer()
	if err != nil {
		return err
	}

	_, err = enforcer.RemoveFilteredPolicy(0, roleID)
	return err
}

// CheckUserPermission 检查用户是否有指定API权限
func (c *casbinService) CheckUserPermission(userID, tenantID, apiPath, method string) (bool, error) {
	// 参数验证
	if tenantID == "" {
		return false, fmt.Errorf("租户ID不能为空")
	}
	
	// 1. 获取用户在当前租户下的角色信息
	tenantUserInfo, err := c.ctx.DB.Tenant().GetTenantLinkedUserInfo(tenantID, userID)
	if err != nil {
		return false, err
	}

	// 2. 检查用户角色是否存在
	var role models.UserRole
	err = c.ctx.DB.DB().Model(&models.UserRole{}).Where("id = ?", tenantUserInfo.UserRole).First(&role).Error
	if err != nil {
		return false, err
	}

	// 3. 检查角色是否启用
	if !role.IsEnabled() {
		return false, nil
	}

	// 4. 使用角色ID检查权限
	return c.CheckPermission(role.ID, apiPath, method)
}

// GetUserAllPermissions 获取用户的所有权限(合并所有角色的权限)
func (c *casbinService) GetUserAllPermissions(userID, tenantID string) ([]models.PermissionInfo, error) {
	// 参数验证
	if tenantID == "" {
		return nil, fmt.Errorf("租户ID不能为空")
	}
	
	// 1. 获取用户在当前租户下的角色信息
	tenantUserInfo, err := c.ctx.DB.Tenant().GetTenantLinkedUserInfo(tenantID, userID)
	if err != nil {
		return nil, err
	}

	// 2. 检查用户角色是否存在
	var role models.UserRole
	err = c.ctx.DB.DB().Model(&models.UserRole{}).Where("id = ?", tenantUserInfo.UserRole).First(&role).Error
	if err != nil {
		return nil, err
	}

	// 3. 检查角色是否启用
	if !role.IsEnabled() {
		return []models.PermissionInfo{}, nil
	}

	// 4. 获取角色的所有权限
	return c.GetRolePermissions(role.ID)
}

// GetAllApiPermissions 获取所有API权限列表供前端选择
func (c *casbinService) GetAllApiPermissions() ([]models.SysApi, error) {
	db := c.ctx.DB.DB()
	
	var apiList []models.SysApi
	if err := db.Find(&apiList).Error; err != nil {
		return nil, fmt.Errorf("获取API权限列表失败: %v", err)
	}
	
	return apiList, nil
}

// EnsureAllApisRegistered 确保所有API已注册到SysApi表
func (c *casbinService) EnsureAllApisRegistered() error {
	db := c.ctx.DB.DB()
	
	// 获取预定义的API权限列表
	predefinedApis := models.GetAllApiPermissions()
	
	for _, api := range predefinedApis {
		var existingApi models.SysApi
		err := db.Where("path = ? AND method = ?", api.Path, api.Method).First(&existingApi).Error
		
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				// API不存在，创建新记录
				enabled := true
				newApi := models.SysApi{
					Path:        api.Path,
					Method:      api.Method,
					Description: fmt.Sprintf("%s [%s]", api.Group, api.Method),
					ApiGroup:    api.Group,
					Enabled:     &enabled,
				}
				
				if err := db.Create(&newApi).Error; err != nil {
					return fmt.Errorf("创建API记录失败: %v", err)
				}
			} else {
				return fmt.Errorf("查询API记录失败: %v", err)
			}
		}
	}
	
	return nil
}

// InitAdminPermissions 为admin角色初始化所有权限
func (c *casbinService) InitAdminPermissions() error {
	// 获取所有已注册的API
	apis, err := c.GetAllApiPermissions()
	if err != nil {
		return err
	}
	
	// 转换为PermissionInfo格式
	var permissions []models.PermissionInfo
	for _, api := range apis {
		if api.GetEnabled() {
			permissions = append(permissions, models.PermissionInfo{
				Path:   api.Path,
				Method: api.Method,
				Group:  api.ApiGroup,
			})
		}
	}
	
	// 为admin角色设置所有权限
	return c.SetRolePermissions("admin", permissions)
}