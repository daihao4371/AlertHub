package initialization

import (
	"alertHub/alert"
	"alertHub/config"
	"alertHub/internal/cache"
	"alertHub/internal/ctx"
	"alertHub/internal/global"
	"alertHub/internal/models"
	"alertHub/internal/repo"
	"alertHub/internal/services"
	"alertHub/pkg/ai"
	"alertHub/pkg/exporter"
	"alertHub/pkg/templates"
	"alertHub/pkg/tools"
	"context"
	"fmt"
	"sync"

	"github.com/zeromicro/go-zero/core/logc"
	"golang.org/x/sync/errgroup"
)

func InitBasic() {

	// 初始化配置
	global.Config = config.InitConfig()

	dbRepo := repo.NewRepoEntry()
	rCache := cache.NewEntryCache()
	ctx := ctx.NewContext(context.Background(), dbRepo, rCache)

	services.NewServices(ctx)

	// 启用告警评估携程
	alert.Initialize(ctx)

	// 初始化Casbin权限系统
	InitCasbinSQL(ctx)

	// 初始化SysApi权限数据
	InitSysApiPermissions(ctx)

	// 为现有角色初始化Casbin权限
	if err := InitCasbinPermissionsForExistingRoles(ctx); err != nil {
		logc.Errorf(ctx.Ctx, "为现有角色初始化Casbin权限失败: %s", err.Error())
	}

	// 导入数据源 Client 到存储池
	importClientPools(ctx)

	// 定时任务，清理历史通知记录和历史拨测数据
	go gcHistoryData(ctx)

	// 定时任务，每年12月1日自动生成次年值班表
	go autoGenerateNextYearDutySchedule(ctx)

	// 定时任务，Exporter 健康巡检
	go exporterMonitorScheduler(ctx)

	// 加载静默规则
	go pushMuteRuleToRedis()

	r, err := ctx.DB.Setting().Get()
	if err != nil {
		logc.Error(ctx.Ctx, fmt.Sprintf("加载系统设置失败: %s", err.Error()))
		return
	}

	// 初始化快捷操作配置缓存（供通知模板使用）
	// 配置来源：系统设置页面 → MySQL数据库 → settings表 → quick_action_config字段
	initQuickActionConfig(r.QuickActionConfig)

	if r.AuthType != nil && *r.AuthType == models.SettingLdapAuth {
		const mark = "SyncLdapUserJob"
		c, cancel := context.WithCancel(context.Background())
		ctx.ContextMap[mark] = cancel
		go services.LdapService.SyncUsersCronjob(c)
	}

	if r.AiConfig.GetEnable() {
		// Convert models.AiConfig to
		// ai.AiConfig for use with NewAiClient
		aiConfig := &ai.AiConfig{
			Url:       r.AiConfig.Url,
			ApiKey:    r.AiConfig.AppKey,
			Model:     r.AiConfig.Model,
			Timeout:   r.AiConfig.Timeout,
			MaxTokens: r.AiConfig.MaxTokens,
		}
		client, err := ai.NewAiClient(aiConfig)
		if err != nil {
			logc.Error(ctx.Ctx, fmt.Sprintf("创建 Ai 客户端失败: %s", err.Error()))
			return
		}
		ctx.Redis.ProviderPools().SetClient("AiClient", client)
	}
}

func importClientPools(ctx *ctx.Context) {
	list, err := ctx.DB.Datasource().List("", "", "", "")
	if err != nil {
		logc.Error(ctx.Ctx, err.Error())
		return
	}

	g := new(errgroup.Group)
	for _, datasource := range list {
		ds := datasource
		if !*ds.GetEnabled() {
			continue
		}
		g.Go(func() error {
			err := services.DatasourceService.WithAddClientToProviderPools(ds)
			if err != nil {
				logc.Error(ctx.Ctx, fmt.Sprintf("添加到 Client 存储池失败, err: %s", err.Error()))
				return err
			}
			return nil
		})
	}
}

func gcHistoryData(ctx *ctx.Context) {
	// gc probe history data and notice history record
	tools.NewCronjob("00 00 */1 * *", func() {
		err := ctx.DB.Probing().DeleteRecord()
		if err != nil {
			logc.Errorf(ctx.Ctx, "fail to delete probe history data, %s", err.Error())
		}

		err = ctx.DB.Notice().DeleteRecord()
		if err != nil {
			logc.Errorf(ctx.Ctx, "fail to delete notice history record, %s", err.Error())
		}
	})
}

// autoGenerateNextYearDutySchedule 自动生成次年值班表
// 定时任务：每年12月1日凌晨00:00触发
func autoGenerateNextYearDutySchedule(ctx *ctx.Context) {
	// Cron 表达式: 0 0 1 12 *
	// 分 时 日 月 星期 (robfig/cron 格式)
	tools.NewCronjob("0 0 1 12 *", func() {
		logc.Info(ctx.Ctx, "定时任务触发: 开始自动生成次年值班表")

		// 调用 DutyCalendarService 的自动生成方法
		err := services.DutyCalendarService.AutoGenerateNextYearSchedule()
		if err != nil {
			logc.Errorf(ctx.Ctx, "自动生成次年值班表失败: %s", err.Error())
		} else {
			logc.Info(ctx.Ctx, "自动生成次年值班表任务完成")
		}
	})
}

func pushMuteRuleToRedis() {
	list, _, err := ctx.DB.Silence().List("", "", "", models.Page{
		Index: 0,
		Size:  1000,
	})
	if err != nil {
		logc.Errorf(ctx.Ctx, "获取静默规则列表失败, err: %s", err.Error())
		return
	}

	if len(list) == 0 {
		return
	}

	logc.Infof(ctx.Ctx, "获取到 %d 个静默规则", len(list))

	var wg sync.WaitGroup
	wg.Add(len(list))
	for _, silence := range list {
		go func(silence models.AlertSilences) {
			defer func() {
				wg.Done()
			}()

			ctx.Redis.Silence().PushAlertMute(silence)
		}(silence)
	}

	wg.Wait()
	logc.Infof(ctx.Ctx, "所有静默规则加载完毕！")
}

// initQuickActionConfig 初始化快捷操作配置缓存
// 配置来源：系统设置页面 → MySQL数据库 → settings表 → quick_action_config字段（JSON格式）
// 该配置供通知模板层使用，避免模板层直接调用 repo 层
// 配置可通过系统设置页面实时修改，修改后立即生效（无需重启服务）
func initQuickActionConfig(config models.QuickActionConfig) {
	templates.SetQuickActionConfig(config)
	logc.Info(context.Background(), "快捷操作配置已从数据库加载并缓存到内存")
}

// exporterMonitorScheduler Exporter 健康巡检调度器
// 使用 pkg/exporter/Scheduler 管理:
// 1. 定时巡检任务 (根据 InspectionTimes 配置)
// 2. 定时报告推送任务 (根据 CronExpression 配置)
// 3. 历史数据清理任务 (每天凌晨2:00)
func exporterMonitorScheduler(ctx *ctx.Context) {
	scheduler := exporter.NewScheduler(ctx)

	if err := scheduler.Start(); err != nil {
		logc.Errorf(ctx.Ctx, "Exporter 巡检调度器启动失败: %s", err.Error())
		return
	}

	// 设置为全局实例,供其他包使用
	exporter.SetGlobalScheduler(scheduler)

	logc.Info(ctx.Ctx, "Exporter 巡检调度器启动成功")
}
