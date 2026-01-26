package services

import (
	"alertHub/internal/ctx"
	"alertHub/internal/global"
	"alertHub/internal/models"
	"alertHub/internal/types"
	"alertHub/pkg/tools"
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/zeromicro/go-zero/core/logc"
	"gorm.io/gorm"
)

type userService struct {
	ctx *ctx.Context
}

type InterUserService interface {
	List(req interface{}) (interface{}, interface{})
	Get(req interface{}) (interface{}, interface{})
	Login(req interface{}) (interface{}, interface{})
	Update(req interface{}) (interface{}, interface{})
	Register(req interface{}) (interface{}, interface{})
	Delete(req interface{}) (interface{}, interface{})
	ChangePass(req interface{}) (interface{}, interface{})
}

func newInterUserService(ctx *ctx.Context) InterUserService {
	return &userService{
		ctx: ctx,
	}
}

func (us userService) List(req interface{}) (interface{}, interface{}) {
	r := req.(*types.RequestUserQuery)

	data, err := us.ctx.DB.User().List(r.Query, r.JoinDuty)
	if err != nil {
		return nil, err
	}

	return data, nil
}

func (us userService) Get(req interface{}) (interface{}, interface{}) {
	r := req.(*types.RequestUserQuery)

	data, _, err := us.ctx.DB.User().Get(r.UserId, r.UserName, r.Query)
	if err != nil {
		return nil, err
	}

	return data, nil
}

func (us userService) Login(req interface{}) (interface{}, interface{}) {
	r := req.(*types.RequestUserLogin)
	originalPassword := r.Password
	r.Password = tools.GenerateHashPassword(r.Password)

	data, _, err := us.ctx.DB.User().Get("", r.UserName, "")
	if err != nil {
		return nil, err
	}

	setting, err := us.ctx.DB.Setting().Get()
	if err != nil {
		return nil, err
	}
	switch data.CreateBy {
	case "LDAP":
		if *setting.AuthType == models.SettingLdapAuth {
			err := LdapService.Login(r.UserName, originalPassword)
			if err != nil {
				logc.Error(us.ctx.Ctx, fmt.Sprintf("LDAP 用户登陆失败, err: %s", err.Error()))
				return nil, fmt.Errorf("LDAP 用户登陆失败, err: %s", err.Error())
			}
		} else {
			logc.Error(us.ctx.Ctx, "请先开启 LDAP 功能!")
			return nil, fmt.Errorf("请先开启 LDAP 功能!")
		}
	case "OIDC":
		logc.Error(us.ctx.Ctx, "请使用 OIDC 登录!")
		return nil, fmt.Errorf("请使用 OIDC 登录!")
	default:
		if data.Password != r.Password {
			return nil, fmt.Errorf("密码错误")
		}
	}

	tokenData, err := tools.GenerateToken(data.UserId, r.UserName, r.Password)
	if err != nil {
		return nil, err
	}

	duration := time.Duration(global.Config.Jwt.Expire) * time.Second
	us.ctx.Redis.Redis().Set("uid-"+data.UserId, tools.JsonMarshalToString(r), duration)

	return models.ResponseLoginInfo{
		Token:    tokenData,
		Username: r.UserName,
		UserId:   data.UserId,
	}, nil
}

func (us userService) Register(req interface{}) (interface{}, interface{}) {
	r := req.(*types.RequestUserCreate)

	// 验证用户名唯一性
	if err := us.validateUserNotExists(r.UserName); err != nil {
		return nil, err
	}

	// 应用默认值
	us.applyDefaults(r)

	// 创建用户
	user := us.buildUserModel(r)
	if err := us.ctx.DB.User().Create(user); err != nil {
		return nil, fmt.Errorf("创建用户失败: %w", err)
	}

	// 关联租户
	if err := us.associateTenants(r); err != nil {
		logc.Errorf(us.ctx.Ctx, "关联租户失败: %s", err.Error())
	}

	return nil, nil
}

// applyDefaults 应用默认值
func (us userService) applyDefaults(r *types.RequestUserCreate) {
	// 如果 UserId 为空，使用用户名作为 UserId 以保证唯一性
	if r.UserId == "" {
		r.UserId = r.UserName
	}

	// 应用其他默认值
	if r.CreateBy == "" {
		r.CreateBy = "system"
	}

	if r.RealName == "" {
		r.RealName = r.UserName
	}

	if len(r.Tenants) == 0 {
		r.Tenants = []string{"default"}
	}
}

// validateUserNotExists 验证用户名是否已存在
func (us userService) validateUserNotExists(userName string) error {
	_, exists, _ := us.ctx.DB.User().Get("", userName, "")
	if exists {
		return fmt.Errorf("用户已存在")
	}
	return nil
}

// buildUserModel 构建用户模型
func (us userService) buildUserModel(r *types.RequestUserCreate) models.Member {
	return models.Member{
		UserId:     r.UserId,
		UserName:   r.UserName,
		RealName:   r.RealName,
		Email:      r.Email,
		Phone:      r.Phone,
		Password:   tools.GenerateHashPassword(r.Password),
		Role:       r.Role,
		CreateBy:   r.CreateBy,
		CreateAt:   time.Now().Unix(),
		JoinDuty:   r.JoinDuty,
		DutyUserId: r.DutyUserId,
		Tenants:    r.Tenants,
	}
}

// associateTenants 关联租户
func (us userService) associateTenants(r *types.RequestUserCreate) error {
	if !us.containsTenant(r.Tenants, "default") {
		return nil
	}

	if err := us.ensureDefaultTenant(); err != nil {
		return fmt.Errorf("准备默认租户失败: %w", err)
	}

	userRole := us.normalizeRole(r.Role)
	return us.addUserToTenant("default", r.UserId, r.UserName, userRole)
}

// ensureDefaultTenant 确保默认租户存在（使用事务和行锁确保原子性）
func (us userService) ensureDefaultTenant() error {
	// 先清理可能存在的重复记录（只保留一条，按update_at排序保留最早的）
	us.cleanupDuplicateDefaultTenantsOnce()

	// 使用数据库事务确保操作的原子性
	return us.ctx.DB.DB().Transaction(func(tx *gorm.DB) error {
		// 使用First方法查询，FOR UPDATE加行锁防止并发创建
		var existingTenant models.Tenant
		err := tx.Model(&models.Tenant{}).
			Where("id = ?", "default").
			Set("gorm:query_option", "FOR UPDATE").
			First(&existingTenant).Error

		if err == nil {
			// 租户已存在，只需确保关联记录存在
			return us.ensureTenantLinkInTx(tx)
		}

		if err != gorm.ErrRecordNotFound {
			// 其他错误，返回
			return err
		}

		// 租户不存在，创建它
		removeProtection := true
		tenant := models.Tenant{
			ID:               "default",
			Name:             "default",
			Manager:          "admin",
			Description:      "default 租户",
			UserNumber:       999,
			RuleNumber:       999,
			DutyNumber:       999,
			NoticeNumber:     999,
			RemoveProtection: &removeProtection,
			UpdateAt:         time.Now().Unix(),
		}

		// 在事务中创建租户（主键约束会防止重复）
		if err := tx.Model(&models.Tenant{}).Create(&tenant).Error; err != nil {
			// 如果创建失败（可能是并发创建或主键冲突），再次检查
			var checkTenant models.Tenant
			if checkErr := tx.Model(&models.Tenant{}).
				Where("id = ?", "default").
				First(&checkTenant).Error; checkErr == nil {
				// 租户已存在（并发创建成功），继续
			} else {
				// 真正的错误，返回
				return fmt.Errorf("创建默认租户失败: %w", err)
			}
		}

		// 确保关联记录存在
		return us.ensureTenantLinkInTx(tx)
	})
}

// cleanupDuplicateDefaultTenantsOnce 清理重复的default租户（只保留一条，按update_at排序保留最早的）
func (us userService) cleanupDuplicateDefaultTenantsOnce() {
	var tenants []models.Tenant
	err := us.ctx.DB.DB().Model(&models.Tenant{}).
		Where("id = ?", "default").
		Order("update_at ASC").
		Find(&tenants).Error

	if err != nil || len(tenants) <= 1 {
		return
	}

	// 保留第一条（最早的），删除其他的
	// 由于ID都是"default"，我们使用update_at来区分，保留最早的
	keepUpdateAt := tenants[0].UpdateAt

	// 删除除第一条外的所有记录（使用update_at区分）
	us.ctx.DB.DB().Model(&models.Tenant{}).
		Where("id = ? AND update_at != ?", "default", keepUpdateAt).
		Delete(&models.Tenant{})
}

// ensureTenantLinkInTx 在事务中确保租户关联记录存在
func (us userService) ensureTenantLinkInTx(tx *gorm.DB) error {
	var existingLink models.TenantLinkedUsers
	err := tx.Model(&models.TenantLinkedUsers{}).
		Where("id = ?", "default").
		First(&existingLink).Error

	if err == nil {
		// 关联记录已存在
		return nil
	}

	if err != gorm.ErrRecordNotFound {
		// 其他错误
		return err
	}

	// 关联记录不存在，创建它
	link := models.TenantLinkedUsers{
		ID:    "default",
		Users: []models.TenantUser{},
	}

	if err := tx.Model(&models.TenantLinkedUsers{}).Create(&link).Error; err != nil {
		// 如果创建失败（可能是并发创建），再次检查
		var checkLink models.TenantLinkedUsers
		if checkErr := tx.Model(&models.TenantLinkedUsers{}).
			Where("id = ?", "default").
			First(&checkLink).Error; checkErr == nil {
			// 关联记录已存在（并发创建成功），忽略错误
			return nil
		}
		return err
	}

	return nil
}

// ensureTenantLink 确保租户关联记录存在（兼容旧代码，内部调用ensureTenantLinkInTx）
func (us userService) ensureTenantLink() error {
	return us.ensureTenantLinkInTx(us.ctx.DB.DB())
}

// containsTenant 检查租户列表是否包含指定租户
func (us userService) containsTenant(tenants []string, target string) bool {
	for _, t := range tenants {
		if t == target {
			return true
		}
	}
	return false
}

// normalizeRole 规范化角色（空值使用默认角色）
func (us userService) normalizeRole(role string) string {
	if role == "" {
		return "admin"
	}
	return role
}

// addUserToTenant 添加用户到租户
func (us userService) addUserToTenant(tenantId, userId, userName, userRole string) error {
	// 先关联用户到租户
	err := us.ctx.DB.Tenant().AddTenantLinkedUsers(tenantId,
		[]models.TenantUser{{UserID: userId, UserName: userName}},
		userRole,
	)
	if err != nil {
		return err
	}

	// 然后确保该角色的权限在Casbin中已初始化
	if userRole == "admin" {
		// admin角色拥有所有权限
		permissions := models.GetAllApiPermissions()
		if err := services.CasbinPermissionService.SetRolePermissions(userRole, permissions); err != nil {
			logc.Errorf(us.ctx.Ctx, "为admin角色初始化Casbin权限失败: %s", err.Error())
		}
	} else if userRole == "user" {
		// user角色拥有基础权限
		permissions := models.GetBasicApiPermissions()
		if err := services.CasbinPermissionService.SetRolePermissions(userRole, permissions); err != nil {
			logc.Errorf(us.ctx.Ctx, "为user角色初始化Casbin权限失败: %s", err.Error())
		}
	} else {
		// 自定义角色时，给予基础权限
		permissions := models.GetBasicApiPermissions()
		if err := services.CasbinPermissionService.SetRolePermissions(userRole, permissions); err != nil {
			logc.Errorf(us.ctx.Ctx, "为自定义角色 %s 初始化Casbin权限失败: %s", userRole, err.Error())
		}
	}

	return nil
}

func (us userService) Update(req interface{}) (interface{}, interface{}) {
	r := req.(*types.RequestUserUpdate)
	var dbData models.Member

	db := us.ctx.DB.DB().Model(models.Member{})
	db.Where("user_id = ?", r.UserId).First(&dbData)

	if r.Password == "" {
		r.Password = dbData.Password
	} else {
		r.Password = tools.GenerateHashPassword(r.Password)
	}
	err := us.ctx.DB.User().Update(models.Member{
		UserId:     r.UserId,
		UserName:   r.UserName,
		RealName:   r.RealName,
		Email:      r.Email,
		Phone:      r.Phone,
		Password:   r.Password,
		Role:       r.Role,
		CreateBy:   r.CreateBy,
		CreateAt:   r.CreateAt,
		JoinDuty:   r.JoinDuty,
		DutyUserId: r.DutyUserId,
		Tenants:    r.Tenants,
	})
	if err != nil {
		return nil, err
	}

	us.ctx.DB.User().ChangeCache(r.UserId)

	return nil, nil
}

func (us userService) Delete(req interface{}) (interface{}, interface{}) {
	r := req.(*types.RequestUserQuery)
	err := us.ctx.DB.User().Delete(r.UserId)
	if err != nil {
		return nil, err
	}

	us.ctx.DB.User().ChangeCache(r.UserId)

	return nil, nil
}

func (us userService) ChangePass(req interface{}) (interface{}, interface{}) {
	r := req.(*types.RequestUserChangePassword)

	arr := md5.Sum([]byte(r.Password))
	hashPassword := hex.EncodeToString(arr[:])
	r.Password = hashPassword

	err := us.ctx.DB.User().ChangePass(r.UserId, r.Password)
	if err != nil {
		return nil, err
	}

	us.ctx.DB.User().ChangeCache(r.UserId)

	return nil, nil
}
