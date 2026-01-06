package services

import (
	"alertHub/internal/ctx"
	service2 "alertHub/pkg/community/aws/cloudwatch/service"
	"alertHub/pkg/community/aws/service"
)

var (
	DatasourceService          InterDatasourceService
	AuditLogService            InterAuditLogService
	DashboardService           InterDashboardService
	DutyManageService          InterDutyManageService
	DutyCalendarService        InterDutyCalendarService
	EventService               InterEventService
	NoticeService              InterNoticeService
	NoticeTmplService          InterNoticeTmplService
	RuleService                InterRuleService
	RuleGroupService           InterRuleGroupService
	RuleTmplService            InterRuleTmplService
	SilenceService             InterSilenceService
	TenantService              InterTenantService
	UserService                InterUserService
	UserRoleService            InterUserRoleService
	AlertService               InterAlertService
	RuleTmplGroupService       InterRuleTmplGroupService
	AWSRegionService           service.InterAwsRegionService
	AWSCloudWatchService       service2.InterAwsCloudWatchService
	AWSCloudWatchRdsService    service2.InterAwsRdsService
	SettingService             InterSettingService
	ClientService              InterClientService
	LdapService                InterLdapService
	SubscribeService           InterAlertSubscribeService
	ProbingService             InterProbingService
	FaultCenterService         InterFaultCenterService
	AiService                  InterAiService
	OidcService                InterOidcService
	QuickActionService         InterQuickActionService
	ExporterMonitorService     InterExporterMonitorService
	PrometheusProxyService     InterPrometheusProxyService
	MetricsExplorerService     InterMetricsExplorerService
	CasbinPermissionService    InterCasbinService              // 新增Casbin权限服务
	DashboardStatisticsService InterDashboardStatisticsService // 首页统计服务
	ProcessTraceService        InterProcessTraceService        // 处理流程追踪服务
)

func NewServices(ctx *ctx.Context) {
	DatasourceService = newInterDatasourceService(ctx)
	AuditLogService = newInterAuditLogService(ctx)
	DashboardService = newInterDashboardService(ctx)
	DutyManageService = newInterDutyManageService(ctx)
	DutyCalendarService = newInterDutyCalendarService(ctx)
	NoticeService = newInterAlertNoticeService(ctx)
	NoticeTmplService = newInterNoticeTmplService(ctx)
	RuleService = newInterRuleService(ctx)
	RuleGroupService = newInterRuleGroupService(ctx)
	RuleTmplService = newInterRuleTmplService(ctx)
	RuleTmplGroupService = newInterRuleTmplGroupService(ctx)
	SilenceService = newInterSilenceService(ctx)
	TenantService = newInterTenantService(ctx)
	UserService = newInterUserService(ctx)
	UserRoleService = newInterUserRoleService(ctx)
	AlertService = newInterAlertService(ctx)
	AWSRegionService = service.NewInterAwsRegionService(ctx)
	AWSCloudWatchService = service2.NewInterAwsCloudWatchService(ctx)
	AWSCloudWatchRdsService = service2.NewInterAWSRdsService(ctx)
	SettingService = newInterSettingService(ctx)
	ClientService = newInterClientService(ctx)
	LdapService = newInterLdapService(ctx)
	SubscribeService = newInterAlertSubscribe(ctx)
	ProbingService = newInterProbingService(ctx)
	FaultCenterService = newInterFaultCenterService(ctx)
	AiService = newInterAiService(ctx)
	OidcService = newInterOidcService(ctx)
	ProcessTraceService = NewInterProcessTraceService(ctx) // 处理流程追踪服务需要先初始化
	QuickActionService = newInterQuickActionService(ctx)   // QuickActionService依赖ProcessTraceService
	EventService = newInterEventService(ctx)               // EventService依赖ProcessTraceService
	ExporterMonitorService = newInterExporterMonitorService(ctx)
	PrometheusProxyService = newInterPrometheusProxyService(ctx)
	MetricsExplorerService = newInterMetricsExplorerService(ctx)
	CasbinPermissionService = newInterCasbinService(ctx)            // 初始化Casbin权限服务
	DashboardStatisticsService = newDashboardStatisticsService(ctx) // 初始化首页统计服务
}
