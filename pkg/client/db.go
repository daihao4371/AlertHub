package client

import (
	"alertHub/internal/global"
	"alertHub/internal/models"
	"context"
	"fmt"

	"github.com/zeromicro/go-zero/core/logc"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

type DBConfig struct {
	Host    string
	Port    string
	User    string
	Pass    string
	DBName  string
	Timeout string
}

func NewDBClient(config DBConfig) *gorm.DB {
	// 初始化本地 test.db 数据库文件
	//db, err := gorm.Open(sqlite.Open("data/sql.db"), &gorm.Config{})

	dsn := fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?charset=utf8mb4&collation=utf8mb4_general_ci&parseTime=True&loc=Local&timeout=%s",
		config.User,
		config.Pass,
		config.Host,
		config.Port,
		config.DBName,
		config.Timeout)
	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{})

	if err != nil {
		logc.Errorf(context.Background(), "failed to connect database: %s", err.Error())
		return nil
	}

	// 检查 Product 结构是否变化，变化则进行迁移
	err = db.AutoMigrate(
		&models.DutySchedule{},
		&models.DutyManagement{},
		&models.AlertNotice{},
		&models.AlertDataSource{},
		&models.AlertRule{},
		&models.AlertCurEvent{},
		&models.AlertHisEvent{},
		&models.AlertSilences{},
		&models.Member{},
		&models.UserRole{},
		&models.SysApi{},      // API权限表
		&models.UserRoleApi{}, // 角色API关联表
		&models.NoticeTemplateExample{},
		&models.RuleGroups{},
		&models.RuleTemplateGroup{},
		&models.RuleTemplate{},
		&models.Tenant{},
		&models.Dashboard{},
		&models.AuditLog{},
		&models.Settings{},
		&models.TenantLinkedUsers{},
		&models.DashboardFolders{},
		&models.AlertSubscribe{},
		&models.NoticeRecord{},
		&models.ProbingRule{},
		&models.FaultCenter{},
		&models.AiContentRecord{},
		&models.ProbingHistory{},
		&models.Comment{},
		&models.ExporterMonitorConfig{},
		&models.ExporterReportSchedule{},
		&models.ExporterInspection{},       // 新增: 巡检记录主表
		&models.ExporterInspectionDetail{}, // 新增: 巡检明细表
		&models.ProcessTrace{},             // 新增: 处理流程追踪表
		&models.ProcessOperationLog{},      // 新增: 处理操作日志表
		&models.ThirdPartyWebhook{},        // 新增: 第三方Webhook配置表
		&models.ThirdPartyAlert{},          // 新增: 第三方告警记录表
		&models.ConsulTarget{},             // 新增: Consul目标追踪表
		&models.ConsulTargetOfflineLog{},   // 新增: Consul注销历史记录表
		&models.CmdbHost{},                 // 新增: CMDB主机表
		&models.CmdbHostApplication{},      // 新增: CMDB主机应用关联表
	)
	if err != nil {
		logc.Error(context.Background(), err.Error())
		return nil
	}

	if global.Config.Server.Mode == "debug" {
		db.Debug()
	} else {
		db.Logger = logger.Default.LogMode(logger.Silent)
	}

	return db
}
