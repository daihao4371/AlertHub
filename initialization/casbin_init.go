package initialization

import (
	"alertHub/internal/ctx"
	"alertHub/internal/models"
	"alertHub/internal/registry"
	"alertHub/internal/services"
	"alertHub/pkg/tools"
	"fmt"
	"time"

	"github.com/zeromicro/go-zero/core/logc"
	"gorm.io/gorm"
)

// InitCasbinSQL 初始化Casbin相关表和数据
func InitCasbinSQL(ctx *ctx.Context) {
	// 1. 确保Casbin规则表存在
	if err := createCasbinTables(ctx); err != nil {
		logc.Errorf(ctx.Ctx, "创建Casbin表失败: %s", err.Error())
		panic(err)
	}

	// 2. 初始化API注册表
	if err := initApiRegistry(ctx); err != nil {
		logc.Errorf(ctx.Ctx, "初始化API注册表失败: %s", err.Error())
		panic(err)
	}

	// 3. 检查是否需要初始化权限数据
	needsInit, err := needsCasbinInitialization(ctx)
	if err != nil {
		logc.Errorf(ctx.Ctx, "检查Casbin初始化状态失败: %s", err.Error())
		panic(err)
	}

	if needsInit {
		// 4. 初始化默认权限数据
		if err := initDefaultCasbinPermissions(ctx); err != nil {
			logc.Errorf(ctx.Ctx, "初始化默认Casbin权限失败: %s", err.Error())
			panic(err)
		}
		logc.Infof(ctx.Ctx, "Casbin权限数据初始化完成")
	}

	logc.Infof(ctx.Ctx, "Casbin初始化完成")
}

// createCasbinTables 创建Casbin相关表
func createCasbinTables(ctx *ctx.Context) error {
	db := ctx.DB.DB()

	// 自动迁移Casbin规则表（如果表不存在会创建，存在则保持现有数据）
	if err := db.AutoMigrate(&models.CasbinRule{}); err != nil {
		return fmt.Errorf("迁移CasbinRule表失败: %v", err)
	}

	// 创建必要的索引，使用较短的字段长度避免超出限制
	if err := createCasbinIndexes(ctx, db); err != nil {
		logc.Errorf(ctx.Ctx, "创建Casbin索引失败: %s", err.Error())
		// 索引创建失败不阻止系统启动，只记录警告
		logc.Infof(ctx.Ctx, "Casbin表创建完成，但索引创建失败，系统仍可正常运行")
	} else {
		logc.Infof(ctx.Ctx, "Casbin表和索引创建完成")
	}

	return nil
}

// initDefaultCasbinPermissions 初始化默认的Casbin权限规则
func initDefaultCasbinPermissions(ctx *ctx.Context) error {
	// 获取Casbin服务
	casbinService := services.CasbinPermissionService
	if casbinService == nil {
		return fmt.Errorf("CasbinPermissionService未初始化")
	}

	// 定义默认角色权限映射
	defaultRolePermissions := map[string][]models.PermissionInfo{
		"admin": models.GetAllApiPermissions(),   // 超级管理员拥有所有权限
		"user":  models.GetBasicApiPermissions(), // 普通用户基础权限
	}

	// 设置默认角色权限
	for roleID, permissions := range defaultRolePermissions {
		err := casbinService.SetRolePermissions(roleID, permissions)
		if err != nil {
			logc.Errorf(ctx.Ctx, "为角色 %s 设置权限失败: %s", roleID, err.Error())
			continue
		}
		logc.Infof(ctx.Ctx, "为角色 %s 设置了 %d 个权限", roleID, len(permissions))
	}

	return nil
}

// needsCasbinInitialization 检查是否需要初始化Casbin权限
func needsCasbinInitialization(ctx *ctx.Context) (bool, error) {
	db := ctx.DB.DB()

	// 检查casbin_rule表是否有数据
	var count int64
	if err := db.Model(&models.CasbinRule{}).Count(&count).Error; err != nil {
		return false, fmt.Errorf("检查casbin_rule表数据失败: %v", err)
	}

	// 如果没有数据，需要初始化
	if count == 0 {
		return true, nil
	}

	// 检查admin角色是否有权限
	var adminPermissionCount int64
	if err := db.Model(&models.CasbinRule{}).Where("v0 = ?", "admin").Count(&adminPermissionCount).Error; err != nil {
		return false, fmt.Errorf("检查admin角色权限失败: %v", err)
	}

	// 如果admin角色没有权限，需要初始化
	if adminPermissionCount == 0 {
		return true, nil
	}

	// 检查是否有新的API需要为admin分配权限
	// 获取数据库中所有API的数量
	var totalApiCount int64
	if err := db.Model(&models.SysApi{}).Where("enabled = ?", true).Count(&totalApiCount).Error; err != nil {
		return false, fmt.Errorf("检查API总数失败: %v", err)
	}

	// 如果admin权限数量少于API总数，说明有新API需要分配权限
	if adminPermissionCount < totalApiCount {
		logc.Infof(ctx.Ctx, "检测到admin权限不完整: 权限数=%d, API总数=%d，将重新初始化", adminPermissionCount, totalApiCount)
		return true, nil
	}

	return false, nil
}

// needsIndexCreation 检查是否需要创建索引
func needsIndexCreation(db *gorm.DB) bool {
	// 检查索引是否已经存在
	checkIndexSQL := `SHOW INDEX FROM casbin_rule WHERE Key_name = 'idx_casbin_rule_unique'`
	var result []interface{}
	if err := db.Raw(checkIndexSQL).Scan(&result).Error; err != nil {
		// 查询出错，假设需要创建索引
		return true
	}

	// 如果没有查询到索引，需要创建
	return len(result) == 0
}

// clearExistingCasbinRules 清除现有的Casbin规则（保留但不使用）
func clearExistingCasbinRules(ctx *ctx.Context) error {
	db := ctx.DB.DB()

	// 清空casbin_rule表
	if err := db.Session(&gorm.Session{AllowGlobalUpdate: true}).Delete(&models.CasbinRule{}).Error; err != nil {
		return fmt.Errorf("清除Casbin规则失败: %v", err)
	}

	logc.Infof(ctx.Ctx, "已清除所有现有Casbin规则")
	return nil
}

// InitCasbinPermissionsForExistingRoles 为现有角色初始化Casbin权限
func InitCasbinPermissionsForExistingRoles(ctx *ctx.Context) error {
	db := ctx.DB.DB()
	casbinService := services.CasbinPermissionService

	// 获取所有现有角色
	var roles []models.UserRole
	if err := db.Find(&roles).Error; err != nil {
		return fmt.Errorf("获取现有角色失败: %v", err)
	}

	// 如果没有角色，创建默认的admin和user角色
	if len(roles) == 0 {
		if err := createDefaultRoles(ctx); err != nil {
			return fmt.Errorf("创建默认角色失败: %v", err)
		}
		// 重新获取角色列表
		if err := db.Find(&roles).Error; err != nil {
			return fmt.Errorf("重新获取角色列表失败: %v", err)
		}
	}

	// 为每个角色检查并分配权限
	for _, role := range roles {
		// 检查角色是否已有权限
		var permissionCount int64
		if err := db.Model(&models.CasbinRule{}).Where("v0 = ?", role.ID).Count(&permissionCount).Error; err != nil {
			logc.Errorf(ctx.Ctx, "检查角色 %s 权限失败: %s", role.ID, err.Error())
			continue
		}

		// 对于admin角色，检查权限是否完整
		needsReinit := false
		if role.Name == "admin" {
			// 获取API总数
			var totalApiCount int64
			if err := db.Model(&models.SysApi{}).Where("enabled = ?", true).Count(&totalApiCount).Error; err != nil {
				logc.Errorf(ctx.Ctx, "获取API总数失败: %s", err.Error())
				continue
			}
			
			// 如果admin权限数量少于API总数，说明权限不完整，需要重新初始化
			if permissionCount < totalApiCount {
				needsReinit = true
				logc.Infof(ctx.Ctx, "检测到角色 %s (%s) 权限不完整: 现有%d个, 应有%d个, 将重新初始化", 
					role.ID, role.Name, permissionCount, totalApiCount)
			}
		}

		// 如果角色已有完整权限，跳过
		if permissionCount > 0 && !needsReinit {
			logc.Infof(ctx.Ctx, "角色 %s (%s) 已有 %d 个权限，跳过初始化", role.ID, role.Name, permissionCount)
			continue
		}

		// 如果需要重新初始化，先清除现有权限
		if needsReinit {
			if err := casbinService.RemoveRolePermissions(role.ID); err != nil {
				logc.Errorf(ctx.Ctx, "清除角色 %s 现有权限失败: %s", role.ID, err.Error())
				continue
			}
			logc.Infof(ctx.Ctx, "已清除角色 %s (%s) 的现有权限", role.ID, role.Name)
		}

		var permissions []models.PermissionInfo

		// 根据角色类型分配不同的权限集
		switch {
		case role.Name == "admin":
			// 从数据库获取所有API权限
			apis, err := casbinService.GetAllApiPermissions()
			if err != nil {
				logc.Errorf(ctx.Ctx, "获取所有API权限失败: %s", err.Error())
				continue
			}
			// 转换为PermissionInfo格式
			for _, api := range apis {
				if api.GetEnabled() {
					permissions = append(permissions, models.PermissionInfo{
						Path:   api.Path,
						Method: api.Method,
						Group:  api.ApiGroup,
					})
				}
			}
		case role.Name == "user":
			permissions = models.GetBasicApiPermissions()
		default:
			// 其他角色默认给予基础权限
			permissions = models.GetBasicApiPermissions()
		}

		// 设置角色权限
		if err := casbinService.SetRolePermissions(role.ID, permissions); err != nil {
			logc.Errorf(ctx.Ctx, "为现有角色 %s 设置Casbin权限失败: %s", role.ID, err.Error())
			continue
		}

		logc.Infof(ctx.Ctx, "为现有角色 %s (%s) 设置了 %d 个Casbin权限", role.ID, role.Name, len(permissions))
	}

	return nil
}

// createDefaultRoles 创建默认的admin和user角色
func createDefaultRoles(ctx *ctx.Context) error {
	db := ctx.DB.DB()

	defaultRoles := []models.UserRole{
		{
			ID:          "admin",
			Name:        "admin",
			Description: "系统管理员，拥有所有权限",
			Enabled:     tools.BoolPtr(true),
			UpdateAt:    time.Now().Unix(),
		},
		{
			ID:          "user",
			Name:        "user",
			Description: "普通用户，拥有基础权限",
			Enabled:     tools.BoolPtr(true),
			UpdateAt:    time.Now().Unix(),
		},
	}

	for _, role := range defaultRoles {
		if err := db.FirstOrCreate(&role, "id = ?", role.ID).Error; err != nil {
			return fmt.Errorf("创建默认角色 %s 失败: %v", role.Name, err)
		}
		logc.Infof(ctx.Ctx, "创建/更新默认角色: %s", role.Name)
	}

	return nil
}

// InitSysApiPermissions 初始化SysApi权限数据到数据库
func InitSysApiPermissions(ctx *ctx.Context) {
	db := ctx.DB.DB()

	// 检查SysApi表是否已有数据
	var count int64
	if err := db.Model(&models.SysApi{}).Count(&count).Error; err != nil {
		logc.Errorf(ctx.Ctx, "检查SysApi表数据失败: %s", err.Error())
		return
	}

	// 如果已有数据，跳过初始化
	if count > 0 {
		logc.Infof(ctx.Ctx, "SysApi表已有 %d 条权限数据，跳过初始化", count)
		return
	}

	// 直接使用Casbin模型中的权限定义
	allPermissions := models.GetAllApiPermissions()

	// 转换为SysApi格式
	var apiList []models.SysApi
	for _, perm := range allPermissions {
		apiList = append(apiList, models.SysApi{
			Path:        perm.Path,
			Description: perm.Group + "-" + perm.Method + ": " + perm.Path,
			ApiGroup:    perm.Group,
			Method:      perm.Method,
			Enabled:     tools.BoolPtr(true),
		})
	}

	// 批量创建API权限数据
	if err := db.CreateInBatches(&apiList, 100).Error; err != nil {
		logc.Errorf(ctx.Ctx, "创建SysApi数据失败: %s", err.Error())
		return
	}

	logc.Infof(ctx.Ctx, "成功初始化 %d 个API权限数据到SysApi表", len(apiList))
}

// createCasbinIndexes 创建Casbin表索引，避免MySQL key长度限制
func createCasbinIndexes(ctx *ctx.Context, db *gorm.DB) error {
	// 检查索引是否已经存在，避免重复创建
	if !needsIndexCreation(db) {
		logc.Infof(ctx.Ctx, "Casbin索引已存在，跳过创建")
		return nil
	}

	// 创建复合唯一索引，避免重复权限规则
	// 使用较短的字段长度: ptype(50) + v0(50) + v1(200) + v2(10) = 310字符
	// 按UTF-8计算约930字节，远低于MySQL 3072字节限制
	indexSQL := `
		CREATE UNIQUE INDEX idx_casbin_rule_unique 
		ON casbin_rule (ptype, v0, v1(200), v2)
	`
	
	if err := db.Exec(indexSQL).Error; err != nil {
		// 如果索引已存在，忽略错误
		if tools.ContainsAny(err.Error(), []string{"Duplicate key", "already exists", "重复"}) {
			logc.Infof(ctx.Ctx, "Casbin索引已存在")
			return nil
		}
		return fmt.Errorf("创建Casbin唯一索引失败: %v", err)
	}

	logc.Infof(ctx.Ctx, "成功创建Casbin唯一索引")
	return nil
}

// initApiRegistry 初始化API注册表
func initApiRegistry(ctx *ctx.Context) error {
	logc.Infof(ctx.Ctx, "开始初始化API注册表")
	
	// 创建API注册表实例
	apiRegistry := registry.NewApiRegistry(ctx)
	
	// 注册所有API到数据库
	if err := apiRegistry.RegisterToDatabase(); err != nil {
		return fmt.Errorf("API注册表注册失败: %v", err)
	}
	
	logc.Infof(ctx.Ctx, "API注册表初始化完成")
	return nil
}
