package services

import (
	"alertHub/internal/ctx"
	"alertHub/internal/global"
	"alertHub/internal/models"
	"alertHub/pkg/templates"
	"context"
	"fmt"
)

type (
	settingService struct {
		ctx *ctx.Context
	}

	InterSettingService interface {
		Save(req interface{}) (interface{}, interface{})
		Get() (interface{}, interface{})
	}
)

func newInterSettingService(ctx *ctx.Context) InterSettingService {
	return settingService{
		ctx: ctx,
	}
}

func (a settingService) Save(req interface{}) (interface{}, interface{}) {
	r := req.(*models.Settings)
	dbConf, err := a.ctx.DB.Setting().Get()
	if err != nil {
		return nil, err
	}

	if a.ctx.DB.Setting().Check() {
		err := a.ctx.DB.Setting().Update(*r)
		if err != nil {
			return nil, err
		}
	} else {
		err := a.ctx.DB.Setting().Create(*r)
		if err != nil {
			return nil, err
		}
	}

	const mark = "SyncLdapUserJob"
	if r.AuthType != nil && *r.AuthType == models.SettingLdapAuth && *dbConf.AuthType != models.SettingLdapAuth {
		if cancel, exists := a.ctx.ContextMap[mark]; exists {
			cancel()
			delete(a.ctx.ContextMap, mark)
		}
		c, cancel := context.WithCancel(context.Background())
		a.ctx.ContextMap[mark] = cancel
		// 定时同步LDAP用户任务
		go LdapService.SyncUsersCronjob(c)
	} else {
		if cancel, exists := a.ctx.ContextMap[mark]; exists {
			cancel()
			delete(a.ctx.ContextMap, mark)
		}
	}

	if r.AiConfig.GetEnable() {
		// AI 功能已启用，验证是否有配置
		providers := r.AiConfig.GetAllProviders()
		if len(providers) == 0 {
			return nil, fmt.Errorf("AI 已启用但未配置任何 Provider")
		}
	}

	// 重新加载快捷操作配置到内存缓存（保存后立即生效）
	templates.SetQuickActionConfig(r.QuickActionConfig)

	return nil, nil
}

func (a settingService) Get() (interface{}, interface{}) {
	get, err := a.ctx.DB.Setting().Get()
	if err != nil {
		return nil, err
	}
	get.AppVersion = global.Version

	return get, nil
}
