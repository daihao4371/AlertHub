package registry

import (
	"alertHub/internal/ctx"
	"alertHub/internal/models"
	"alertHub/pkg/tools"

	"github.com/zeromicro/go-zero/core/logc"
)

// ApiEndpoint 表示单个API接口
type ApiEndpoint struct {
	Path        string `json:"path"`        // API路径
	Method      string `json:"method"`      // HTTP方法
	Description string `json:"description"` // 接口描述
	Group       string `json:"group"`       // 接口分组
}

// ApiRegistry API注册表，用于管理接口发现和注册
type ApiRegistry struct {
	ctx *ctx.Context
}

// NewApiRegistry 创建新的API注册表实例
func NewApiRegistry(ctx *ctx.Context) *ApiRegistry {
	return &ApiRegistry{
		ctx: ctx,
	}
}

// GetAllApiEndpoints 获取系统中所有API接口的完整列表
func (r *ApiRegistry) GetAllApiEndpoints() []ApiEndpoint {
	return []ApiEndpoint{
		// AI接口
		{"/api/w8t/ai/chat", "POST", "AI聊天", "AI助手"},

		// 审计日志
		{"/api/w8t/auditLog/listAuditLog", "GET", "获取审计日志列表", "审计日志"},
		{"/api/w8t/auditLog/searchAuditLog", "GET", "搜索审计日志", "审计日志"},

		// AWS CloudWatch
		{"/api/w8t/community/cloudwatch/metricTypes", "GET", "获取CloudWatch指标类型", "AWS监控"},
		{"/api/w8t/community/cloudwatch/metricNames", "GET", "获取CloudWatch指标名称", "AWS监控"},
		{"/api/w8t/community/cloudwatch/statistics", "GET", "获取CloudWatch统计信息", "AWS监控"},
		{"/api/w8t/community/cloudwatch/dimensions", "GET", "获取CloudWatch维度信息", "AWS监控"},

		// AWS CloudWatch RDS
		{"/api/w8t/community/rds/instances", "GET", "获取RDS实例", "AWS监控"},
		{"/api/w8t/community/rds/clusters", "GET", "获取RDS集群", "AWS监控"},

		// Casbin权限管理
		{"/api/w8t/casbin/setRolePermissions", "POST", "设置角色权限", "权限管理"},
		{"/api/w8t/casbin/getRolePermissions", "GET", "获取角色权限", "权限管理"},
		{"/api/w8t/casbin/removeRolePermissions", "DELETE", "移除角色权限", "权限管理"},
		{"/api/w8t/casbin/getUserPermissions", "GET", "获取用户权限", "权限管理"},
		{"/api/w8t/casbin/checkPermission", "POST", "检查权限", "权限管理"},
		{"/api/w8t/casbin/initDefaultPermissions", "POST", "初始化默认权限", "权限管理"},
		{"/api/w8t/casbin/getApiPermissions", "GET", "获取API权限列表", "权限管理"},

		// 客户端管理
		{"/api/w8t/c/getJaegerService", "GET", "获取Jaeger服务", "客户端管理"},

		// 仪表板
		{"/api/w8t/dashboard/createFolder", "POST", "创建仪表板文件夹", "仪表板"},
		{"/api/w8t/dashboard/updateFolder", "POST", "更新仪表板文件夹", "仪表板"},
		{"/api/w8t/dashboard/deleteFolder", "POST", "删除仪表板文件夹", "仪表板"},
		{"/api/w8t/dashboard/listFolder", "GET", "获取仪表板文件夹列表", "仪表板"},
		{"/api/w8t/dashboard/getFolder", "GET", "获取仪表板文件夹", "仪表板"},
		{"/api/w8t/dashboard/listGrafanaDashboards", "GET", "获取Grafana仪表板列表", "仪表板"},
		{"/api/w8t/dashboard/getDashboardFullUrl", "GET", "获取仪表板完整URL", "仪表板"},

		// 仪表板信息
		{"/api/system/getDashboardInfo", "GET", "获取仪表板信息", "仪表板"},
		{"/api/system/getDashboardStatistics", "GET", "获取首页统计数据", "仪表板"}, // 新增首页统计接口

		// 数据源
		{"/api/w8t/datasource/dataSourceCreate", "POST", "创建数据源", "数据源管理"},
		{"/api/w8t/datasource/dataSourceUpdate", "POST", "更新数据源", "数据源管理"},
		{"/api/w8t/datasource/dataSourceDelete", "POST", "删除数据源", "数据源管理"},
		{"/api/w8t/datasource/dataSourceList", "GET", "获取数据源列表", "数据源管理"},
		{"/api/w8t/datasource/dataSourceGet", "GET", "获取数据源详情", "数据源管理"},
		{"/api/w8t/datasource/promQuery", "GET", "Prometheus查询", "数据源管理"},
		{"/api/w8t/datasource/promQueryRange", "GET", "Prometheus范围查询", "数据源管理"},
		{"/api/w8t/datasource/promLabelValues", "GET", "获取Prometheus标签值", "数据源管理"},
		{"/api/w8t/datasource/dataSourcePing", "POST", "测试数据源连接", "数据源管理"},
		{"/api/w8t/datasource/searchViewLogsContent", "POST", "搜索日志内容", "数据源管理"},

		// 值班管理
		{"/api/w8t/dutyManage/dutyManageCreate", "POST", "创建值班计划", "值班管理"},
		{"/api/w8t/dutyManage/dutyManageUpdate", "POST", "更新值班计划", "值班管理"},
		{"/api/w8t/dutyManage/dutyManageDelete", "POST", "删除值班计划", "值班管理"},
		{"/api/w8t/dutyManage/dutyManageList", "GET", "获取值班计划列表", "值班管理"},

		// 值班日历
		{"/api/w8t/calendar/calendarCreate", "POST", "创建值班表", "值班管理"},
		{"/api/w8t/calendar/calendarUpdate", "POST", "更新值班表", "值班管理"},
		{"/api/w8t/calendar/calendarSearch", "GET", "搜索值班表", "值班管理"},
		{"/api/w8t/calendar/getCalendarUsers", "GET", "获取值班表用户", "值班管理"},

		// 告警事件
		{"/api/w8t/event/processAlertEvent", "POST", "处理告警事件", "告警事件"},
		{"/api/w8t/event/addComment", "POST", "添加评论", "告警事件"},
		{"/api/w8t/event/listComments", "GET", "获取评论列表", "告警事件"},
		{"/api/w8t/event/deleteComment", "POST", "删除评论", "告警事件"},
		{"/api/w8t/event/curEvent", "GET", "获取当前事件", "告警事件"},
		{"/api/w8t/event/hisEvent", "GET", "获取历史事件", "告警事件"},

		// 导出监控
		{"/api/w8t/exporter/monitor/config", "POST", "保存监控配置", "导出监控"},
		{"/api/w8t/exporter/monitor/schedule", "POST", "保存调度配置", "导出监控"},
		{"/api/w8t/exporter/monitor/autoRefresh", "POST", "更新自动刷新", "导出监控"},
		{"/api/w8t/exporter/monitor/inspect", "POST", "手动触发巡检", "导出监控"},
		{"/api/w8t/exporter/monitor/report/send", "POST", "发送报告", "导出监控"},
		{"/api/w8t/exporter/monitor/status", "GET", "获取状态", "导出监控"},
		{"/api/w8t/exporter/monitor/history", "GET", "获取历史记录", "导出监控"},
		{"/api/w8t/exporter/monitor/config", "GET", "获取监控配置", "导出监控"},
		{"/api/w8t/exporter/monitor/schedule", "GET", "获取调度配置", "导出监控"},

		// 故障中心
		{"/api/w8t/faultCenter/faultCenterCreate", "POST", "创建故障", "故障中心"},
		{"/api/w8t/faultCenter/faultCenterUpdate", "POST", "更新故障", "故障中心"},
		{"/api/w8t/faultCenter/faultCenterDelete", "POST", "删除故障", "故障中心"},
		{"/api/w8t/faultCenter/faultCenterReset", "POST", "重置故障", "故障中心"},
		{"/api/w8t/faultCenter/faultCenterList", "GET", "获取故障列表", "故障中心"},
		{"/api/w8t/faultCenter/faultCenterSearch", "GET", "搜索故障", "故障中心"},
		{"/api/w8t/faultCenter/slo", "GET", "获取SLO", "故障中心"},

		// Kubernetes事件类型
		{"/api/w8t/kubernetes/getResourceList", "GET", "获取Kubernetes资源列表", "Kubernetes"},
		{"/api/w8t/kubernetes/getReasonList", "GET", "获取Kubernetes原因列表", "Kubernetes"},

		// 指标浏览器
		{"/api/w8t/metrics-explorer/metrics", "GET", "获取指标列表", "监控查询"},
		{"/api/w8t/metrics-explorer/categories", "GET", "获取指标分类", "监控查询"},
		{"/api/w8t/metrics-explorer/query_range", "POST", "增强查询范围", "监控查询"},

		// 通知
		{"/api/w8t/notice/noticeCreate", "POST", "创建通知", "告警通知"},
		{"/api/w8t/notice/noticeUpdate", "POST", "更新通知", "告警通知"},
		{"/api/w8t/notice/noticeDelete", "POST", "删除通知", "告警通知"},
		{"/api/w8t/notice/noticeList", "GET", "获取通知列表", "告警通知"},
		{"/api/w8t/notice/noticeRecordList", "GET", "获取通知记录列表", "告警通知"},
		{"/api/w8t/notice/noticeRecordMetric", "GET", "获取通知记录指标", "告警通知"},
		{"/api/w8t/notice/noticeTest", "POST", "测试通知", "告警通知"},

		// 通知模板
		{"/api/w8t/noticeTemplate/noticeTemplateCreate", "POST", "创建通知模板", "告警通知"},
		{"/api/w8t/noticeTemplate/noticeTemplateUpdate", "POST", "更新通知模板", "告警通知"},
		{"/api/w8t/noticeTemplate/noticeTemplateDelete", "POST", "删除通知模板", "告警通知"},
		{"/api/w8t/noticeTemplate/noticeTemplateList", "GET", "获取通知模板列表", "告警通知"},

		// 拨测管理
		{"/api/w8t/probing/createProbing", "POST", "创建拨测", "拨测管理"},
		{"/api/w8t/probing/updateProbing", "POST", "更新拨测", "拨测管理"},
		{"/api/w8t/probing/deleteProbing", "POST", "删除拨测", "拨测管理"},
		{"/api/w8t/probing/listProbing", "GET", "获取拨测列表", "拨测管理"},
		{"/api/w8t/probing/searchProbing", "GET", "搜索拨测", "拨测管理"},
		{"/api/w8t/probing/getProbingHistory", "GET", "获取拨测历史", "拨测管理"},
		{"/api/w8t/probing/onceProbing", "POST", "单次拨测", "拨测管理"},
		{"/api/w8t/probing/changeState", "POST", "改变状态", "拨测管理"},

		// Prometheus代理
		{"/api/w8t/prometheus/labels", "GET", "获取标签名称列表", "监控查询"},
		{"/api/w8t/prometheus/label_values", "GET", "获取标签值列表", "监控查询"},
		{"/api/w8t/prometheus/metrics", "GET", "获取指标名称列表", "监控查询"},
		{"/api/w8t/prometheus/series", "POST", "获取时间序列元数据", "监控查询"},

		// 快捷操作
		{"/api/w8t/alert/quick-login", "GET", "显示登录页面", "快捷操作"},
		{"/api/w8t/alert/quick-login", "POST", "处理登录请求", "快捷操作"},
		{"/api/w8t/alert/quick-action", "GET", "快捷操作", "快捷操作"},
		{"/api/w8t/alert/quick-silence", "GET", "自定义静默表单", "快捷操作"},
		{"/api/w8t/alert/quick-silence", "POST", "提交自定义静默", "快捷操作"},

		// 告警规则
		{"/api/w8t/rule/ruleCreate", "POST", "创建告警规则", "告警规则"},
		{"/api/w8t/rule/ruleUpdate", "POST", "更新告警规则", "告警规则"},
		{"/api/w8t/rule/ruleDelete", "POST", "删除告警规则", "告警规则"},
		{"/api/w8t/rule/ruleList", "GET", "获取告警规则列表", "告警规则"},
		{"/api/w8t/rule/ruleSearch", "GET", "搜索告警规则", "告警规则"},
		{"/api/w8t/rule/import", "POST", "导入告警规则", "告警规则"},
		{"/api/w8t/rule/ruleChangeStatus", "POST", "改变规则状态", "告警规则"},

		// 告警规则组
		{"/api/w8t/ruleGroup/ruleGroupCreate", "POST", "创建告警规则组", "告警规则"},
		{"/api/w8t/ruleGroup/ruleGroupUpdate", "POST", "更新告警规则组", "告警规则"},
		{"/api/w8t/ruleGroup/ruleGroupDelete", "POST", "删除告警规则组", "告警规则"},
		{"/api/w8t/ruleGroup/ruleGroupList", "GET", "获取告警规则组列表", "告警规则"},

		// 告警规则模板
		{"/api/w8t/ruleTmpl/ruleTmplCreate", "POST", "创建告警规则模板", "告警规则"},
		{"/api/w8t/ruleTmpl/ruleTmplUpdate", "POST", "更新告警规则模板", "告警规则"},
		{"/api/w8t/ruleTmpl/ruleTmplDelete", "POST", "删除告警规则模板", "告警规则"},
		{"/api/w8t/ruleTmpl/ruleTmplList", "GET", "获取告警规则模板列表", "告警规则"},

		// 告警规则模板组
		{"/api/w8t/ruleTmplGroup/ruleTmplGroupCreate", "POST", "创建告警规则模板组", "告警规则"},
		{"/api/w8t/ruleTmplGroup/ruleTmplGroupUpdate", "POST", "更新告警规则模板组", "告警规则"},
		{"/api/w8t/ruleTmplGroup/ruleTmplGroupDelete", "POST", "删除告警规则模板组", "告警规则"},
		{"/api/w8t/ruleTmplGroup/ruleTmplGroupList", "GET", "获取告警规则模板组列表", "告警规则"},

		// 系统设置
		{"/api/w8t/setting/saveSystemSetting", "POST", "保存系统设置", "系统设置"},
		{"/api/w8t/setting/getSystemSetting", "GET", "获取系统设置", "系统设置"},

		// 静默管理
		{"/api/w8t/silence/silenceCreate", "POST", "创建静默规则", "告警管理"},
		{"/api/w8t/silence/silenceUpdate", "POST", "更新静默规则", "告警管理"},
		{"/api/w8t/silence/silenceDelete", "POST", "删除静默规则", "告警管理"},
		{"/api/w8t/silence/silenceList", "GET", "获取静默规则列表", "告警管理"},

		// 订阅管理
		{"/api/w8t/subscribe/createSubscribe", "POST", "创建订阅", "订阅管理"},
		{"/api/w8t/subscribe/deleteSubscribe", "POST", "删除订阅", "订阅管理"},
		{"/api/w8t/subscribe/listSubscribe", "GET", "获取订阅列表", "订阅管理"},
		{"/api/w8t/subscribe/getSubscribe", "GET", "获取订阅详情", "订阅管理"},

		// 租户管理
		{"/api/w8t/tenant/createTenant", "POST", "创建租户", "租户管理"},
		{"/api/w8t/tenant/updateTenant", "POST", "更新租户", "租户管理"},
		{"/api/w8t/tenant/deleteTenant", "POST", "删除租户", "租户管理"},
		{"/api/w8t/tenant/addUsersToTenant", "POST", "添加用户到租户", "租户管理"},
		{"/api/w8t/tenant/delUsersOfTenant", "POST", "从租户删除用户", "租户管理"},
		{"/api/w8t/tenant/changeTenantUserRole", "POST", "修改租户用户角色", "租户管理"},
		{"/api/w8t/tenant/getTenantList", "GET", "获取租户列表", "租户管理"},
		{"/api/w8t/tenant/getTenant", "GET", "获取租户详情", "租户管理"},
		{"/api/w8t/tenant/getUsersForTenant", "GET", "获取租户用户列表", "租户管理"},

		// 用户管理
		{"/api/w8t/user/userUpdate", "POST", "更新用户", "用户管理"},
		{"/api/w8t/user/userDelete", "POST", "删除用户", "用户管理"},
		{"/api/w8t/user/userChangePass", "POST", "修改密码", "用户管理"},
		{"/api/w8t/user/userList", "GET", "获取用户列表", "用户管理"},

		// 角色管理
		{"/api/w8t/role/roleCreate", "POST", "创建角色", "角色管理"},
		{"/api/w8t/role/roleUpdate", "POST", "更新角色", "角色管理"},
		{"/api/w8t/role/roleDelete", "POST", "删除角色", "角色管理"},
		{"/api/w8t/role/setRolePermissions", "POST", "设置角色权限", "角色管理"},
		{"/api/w8t/role/getRolePermissions", "GET", "获取角色权限", "角色管理"},
		{"/api/w8t/role/checkUserPermission", "POST", "检查用户权限", "角色管理"},
		{"/api/w8t/role/getUserRoles", "GET", "获取用户角色", "角色管理"},
		{"/api/w8t/role/roleList", "GET", "获取角色列表", "角色管理"},

		// 处理流程追踪
		{"/api/w8t/process-trace", "GET", "获取处理流程追踪记录", "处理流程追踪"},
		{"/api/w8t/process-trace/list", "GET", "获取处理流程追踪记录列表", "处理流程追踪"},
		{"/api/w8t/process-trace/status", "PUT", "更新处理状态", "处理流程追踪"},
		{"/api/w8t/process-trace/step", "POST", "添加处理步骤", "处理流程追踪"},
		{"/api/w8t/process-trace/step/complete", "PUT", "完成处理步骤", "处理流程追踪"},
		{"/api/w8t/process-trace/ai-analysis", "PUT", "更新AI分析结果", "处理流程追踪"},
		{"/api/w8t/process-trace/operation-logs", "GET", "获取操作日志列表", "处理流程追踪"},
		{"/api/w8t/process-trace/statistics", "GET", "获取流程统计数据", "处理流程追踪"},
	}
}

// RegisterToDatabase 将所有API接口注册到数据库
func (r *ApiRegistry) RegisterToDatabase() error {
	db := r.ctx.DB.DB()
	endpoints := r.GetAllApiEndpoints()
	
	logc.Infof(r.ctx.Ctx, "开始注册 %d 个API接口到数据库", len(endpoints))

	for _, endpoint := range endpoints {
		// 检查API是否已存在
		var existingApi models.SysApi
		err := db.Where("path = ? AND method = ?", endpoint.Path, endpoint.Method).First(&existingApi).Error
		
		if err != nil {
			// API不存在，创建新记录
			newApi := models.SysApi{
				Path:        endpoint.Path,
				Method:      endpoint.Method,
				Description: endpoint.Description,
				ApiGroup:    endpoint.Group,
				Enabled:     tools.BoolPtr(true),
			}
			
			if err := db.Create(&newApi).Error; err != nil {
				logc.Errorf(r.ctx.Ctx, "创建API记录失败: %s [%s] %s - %v", 
					endpoint.Method, endpoint.Path, endpoint.Description, err)
				continue
			}
			
			logc.Infof(r.ctx.Ctx, "新增API: %s [%s] %s", 
				endpoint.Method, endpoint.Path, endpoint.Description)
		} else {
			// API已存在，根据需要更新描述和分组
			updated := false
			if existingApi.Description != endpoint.Description {
				existingApi.Description = endpoint.Description
				updated = true
			}
			if existingApi.ApiGroup != endpoint.Group {
				existingApi.ApiGroup = endpoint.Group
				updated = true
			}
			
			if updated {
				if err := db.Save(&existingApi).Error; err != nil {
					logc.Errorf(r.ctx.Ctx, "更新API记录失败: %s [%s] - %v", 
						endpoint.Method, endpoint.Path, err)
					continue
				}
				logc.Infof(r.ctx.Ctx, "更新API: %s [%s] %s", 
					endpoint.Method, endpoint.Path, endpoint.Description)
			}
		}
	}

	logc.Infof(r.ctx.Ctx, "API注册完成")
	return nil
}