package api

import (
	"alertHub/internal/middleware"
	"alertHub/internal/services"
	"alertHub/internal/types"
	"alertHub/pkg/response"
	"strconv"

	"github.com/gin-gonic/gin"
)

type consulController struct{}

var ConsulController = new(consulController)

// API 注册 Consul 相关的 API 路由
func (consulController consulController) API(gin *gin.RouterGroup) {
	// Consul 目标管理 - 需要认证
	targets := gin.Group("consul")
	targets.Use(
		middleware.Auth(),
		middleware.CasbinPermission(),
		middleware.ParseTenant(),
	)
	{
		targets.GET("targets", consulController.GetTargets)
		targets.GET("targets/:id", consulController.GetTargetById)
		targets.POST("targets/register", consulController.RegisterTarget)
		targets.POST("targets/:id/deregister", consulController.DeregisterTarget)
		targets.POST("targets/:id/reregister", consulController.ReRegisterTarget)
		targets.POST("sync", consulController.SyncTargets)

		// 标签相关的API端点
		targets.GET("targets/by-tag", consulController.GetTargetsByTag)
		targets.GET("targets/by-job-tag", consulController.GetTargetsByJobAndTag)
		targets.POST("targets/:id/tags", consulController.UpdateTargetTags)

		// 注销记录相关API端点
		targets.GET("offline-logs", consulController.GetOfflineLogs)
	}
}

// GetTargets 获取目标机器列表
func (consulController consulController) GetTargets(ctx *gin.Context) {
	tenantId := ctx.GetString("TenantID")
	if tenantId == "" {
		response.Fail(ctx, nil, "租户ID不能为空")
		return
	}

	// 使用标准的分页查询请求
	req := new(types.RequestConsulTargetsQuery)
	BindQuery(ctx, req)
	if ctx.IsAborted() {
		return
	}

	// 构建过滤条件
	filters := make(map[string]interface{})
	if req.Job != "" {
		filters["job"] = req.Job
	}
	if req.Status != "" {
		filters["status"] = req.Status
	}
	if req.Keyword != "" {
		filters["keyword"] = req.Keyword
	}

	Service(ctx, func() (interface{}, interface{}) {
		return services.ConsulService.GetAllTargets(tenantId, filters, int(req.Index), int(req.Size))
	})
}

// RegisterTarget 手动注册服务到 Consul
func (consulController consulController) RegisterTarget(ctx *gin.Context) {
	tenantId := ctx.GetString("TenantID")
	if tenantId == "" {
		response.Fail(ctx, nil, "租户ID不能为空")
		return
	}

	// 解析请求体
	req := new(types.RequestRegisterTarget)
	if err := ctx.ShouldBindJSON(req); err != nil {
		response.Fail(ctx, nil, "解析请求失败: "+err.Error())
		return
	}

	Service(ctx, func() (interface{}, interface{}) {
		return services.ConsulService.RegisterTarget(
			tenantId,
			req.ServiceID,
			req.ServiceName,
			req.Address,
			req.Port,
			req.Job,
			req.Tags,
			req.Labels,
		)
	})
}

// GetTargetById 获取目标详情
func (consulController consulController) GetTargetById(ctx *gin.Context) {
	idStr := ctx.Param("id")
	if idStr == "" {
		response.Fail(ctx, nil, "目标ID不能为空")
		return
	}

	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		response.Fail(ctx, nil, "目标ID格式不正确")
		return
	}

	Service(ctx, func() (interface{}, interface{}) {
		return services.ConsulService.GetTargetById(id)
	})
}

// DeregisterTarget 注销目标
func (consulController consulController) DeregisterTarget(ctx *gin.Context) {
	tenantId := ctx.GetString("TenantID")
	userId := ctx.GetString("UserId")
	if tenantId == "" {
		response.Fail(ctx, nil, "租户ID不能为空")
		return
	}

	idStr := ctx.Param("id")
	if idStr == "" {
		response.Fail(ctx, nil, "目标ID不能为空")
		return
	}

	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		response.Fail(ctx, nil, "目标ID格式不正确")
		return
	}

	// 解析注销原因
	req := struct {
		Reason string `json:"reason"`
	}{}
	if err := ctx.ShouldBindJSON(&req); err != nil {
		// 注销原因是可选的，忽略解析错误
	}

	Service(ctx, func() (interface{}, interface{}) {
		return services.ConsulService.DeregisterTarget(tenantId, id, req.Reason, userId)
	})
}

// ReRegisterTarget 重新上线已注销的目标
func (consulController consulController) ReRegisterTarget(ctx *gin.Context) {
	tenantId := ctx.GetString("TenantID")
	userId := ctx.GetString("UserId")
	if tenantId == "" {
		response.Fail(ctx, nil, "租户ID不能为空")
		return
	}

	idStr := ctx.Param("id")
	if idStr == "" {
		response.Fail(ctx, nil, "目标ID不能为空")
		return
	}

	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		response.Fail(ctx, nil, "目标ID格式不正确")
		return
	}

	Service(ctx, func() (interface{}, interface{}) {
		return services.ConsulService.ReRegisterTarget(tenantId, id, userId)
	})
}

// SyncTargets 同步 Consul 目标
func (consulController consulController) SyncTargets(ctx *gin.Context) {
	tenantId := ctx.GetString("TenantID")
	if tenantId == "" {
		response.Fail(ctx, nil, "租户ID不能为空")
		return
	}

	Service(ctx, func() (interface{}, interface{}) {
		return services.ConsulService.SyncTargets(tenantId)
	})
}

// GetTargetsByTag 按标签获取目标列表
func (consulController consulController) GetTargetsByTag(ctx *gin.Context) {
	tenantId := ctx.GetString("TenantID")
	if tenantId == "" {
		response.Fail(ctx, nil, "租户ID不能为空")
		return
	}

	// 获取查询参数
	req := new(types.RequestConsulTargetsQuery)
	BindQuery(ctx, req)
	if ctx.IsAborted() {
		return
	}

	if req.Job == "" {
		response.Fail(ctx, nil, "标签不能为空")
		return
	}

	Service(ctx, func() (interface{}, interface{}) {
		return services.ConsulService.GetTargetsByTag(tenantId, req.Job, int(req.Index), int(req.Size))
	})
}

// GetTargetsByJobAndTag 按 Job 和标签结合查询目标
func (consulController consulController) GetTargetsByJobAndTag(ctx *gin.Context) {
	tenantId := ctx.GetString("TenantID")
	if tenantId == "" {
		response.Fail(ctx, nil, "租户ID不能为空")
		return
	}

	// 获取查询参数
	req := new(types.RequestConsulTargetsQuery)
	BindQuery(ctx, req)
	if ctx.IsAborted() {
		return
	}

	if req.Job == "" {
		response.Fail(ctx, nil, "Job 不能为空")
		return
	}
	if req.Keyword == "" {
		response.Fail(ctx, nil, "标签不能为空")
		return
	}

	Service(ctx, func() (interface{}, interface{}) {
		return services.ConsulService.GetTargetsByJobAndTag(tenantId, req.Job, req.Keyword, int(req.Index), int(req.Size))
	})
}

// UpdateTargetTags 更新单个目标的标签
func (consulController consulController) UpdateTargetTags(ctx *gin.Context) {
	tenantId := ctx.GetString("TenantID")
	if tenantId == "" {
		response.Fail(ctx, nil, "租户ID不能为空")
		return
	}

	idStr := ctx.Param("id")
	if idStr == "" {
		response.Fail(ctx, nil, "目标ID不能为空")
		return
	}

	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		response.Fail(ctx, nil, "目标ID格式不正确")
		return
	}

	// 解析请求体中的标签
	req := struct {
		Labels map[string]interface{} `json:"labels"`
	}{}
	if err := ctx.ShouldBindJSON(&req); err != nil {
		response.Fail(ctx, nil, "解析标签失败")
		return
	}

	if len(req.Labels) == 0 {
		response.Fail(ctx, nil, "标签不能为空")
		return
	}

	Service(ctx, func() (interface{}, interface{}) {
		return services.ConsulService.UpdateTargetTags(tenantId, id, req.Labels)
	})
}

// GetOfflineLogs 获取注销历史记录列表
func (consulController consulController) GetOfflineLogs(ctx *gin.Context) {
	tenantId := ctx.GetString("TenantID")
	if tenantId == "" {
		response.Fail(ctx, nil, "租户ID不能为空")
		return
	}

	// 使用标准的分页查询请求
	req := new(types.RequestConsulTargetsQuery)
	BindQuery(ctx, req)
	if ctx.IsAborted() {
		return
	}

	Service(ctx, func() (interface{}, interface{}) {
		return services.ConsulService.GetOfflineLogs(tenantId, int(req.Index), int(req.Size))
	})
}
