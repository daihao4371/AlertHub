package api

import (
	"alertHub/internal/middleware"
	"alertHub/internal/services"
	"alertHub/internal/types"
	"alertHub/pkg/response"

	"github.com/gin-gonic/gin"
)

// rcaController RCA API 控制器
type rcaController struct{}

var RCAController = new(rcaController)

// API 注册 RCA 路由
func (rcaController rcaController) API(gin *gin.RouterGroup) {
	// RCA 聚类分析 API
	rca := gin.Group("rca")
	rca.Use(middleware.Auth(), middleware.CasbinPermission(), middleware.ParseTenant(), middleware.AuditingLog())
	{
		// 告警聚类请求
		rca.POST("cluster", rcaController.Cluster)
		// 用户反馈提交
		rca.POST("cluster/:id/commit", rcaController.CommitFeedback)
		// 事件查询列表
		rca.GET("incidents", rcaController.GetIncidents)
		// 统计报告查询
		rca.GET("report", rcaController.GetReport)
	}
}

// Cluster 执行告警聚类分析
// POST /api/w8t/rca/cluster
// 请求体: { "alertIds": ["id1", "id2", ...], "topologyData": "..." }
// 响应: 聚类建议（事件列表、置信度、根因推理）
func (rcaController rcaController) Cluster(ctx *gin.Context) {
	r := new(types.RequestRCACluster)
	BindJson(ctx, r)

	// 调用 RCA 业务服务（租户ID从中间件context获取）
	Service(ctx, func() (interface{}, interface{}) {
		return services.RCAService.SuggestIncidents(ctx, r.AlertIds, r.TopologyData, r.ForceRefresh, r.ModelName)
	})
}

// CommitFeedback 提交用户反馈
// POST /api/w8t/rca/cluster/:id/commit
// 请求体: { "status": "accepted|rejected", "score": 1-5, "comment": "..." }
// 响应: 反馈确认
func (rcaController rcaController) CommitFeedback(ctx *gin.Context) {
	r := new(types.RequestRCACommitFeedback)
	BindJson(ctx, r)

	// 获取路径参数中的建议 ID
	suggestionId := ctx.Param("id")
	if suggestionId == "" {
		response.Fail(ctx, "建议 ID 不能为空", "failed")
		return
	}

	// 调用 RCA 业务服务
	Service(ctx, func() (interface{}, interface{}) {
		err := services.RCAService.CommitFeedback(
			ctx,
			suggestionId,
			r.Status,
			r.Score,
			r.Comment,
		)
		if err != nil {
			return nil, err
		}
		return gin.H{"status": r.Status, "id": suggestionId}, nil
	})
}

// GetIncidents 列表查询 RCA 事件
// GET /api/w8t/rca/incidents?status=candidate&severity=high&page=1&pageSize=20
// 支持过滤：status, severity, startTime, endTime
// 响应: 分页事件列表
func (rcaController rcaController) GetIncidents(ctx *gin.Context) {
	r := new(types.RequestRCAListIncidents)
	BindQuery(ctx, r)

	if r.Page < 1 {
		r.Page = 1
	}
	if r.PageSize < 1 || r.PageSize > 100 {
		r.PageSize = 20
	}

	// 构建过滤条件
	filters := make(map[string]interface{})
	if r.Status != "" {
		filters["status"] = r.Status
	}
	if r.Severity != "" {
		filters["severity"] = r.Severity
	}
	if r.StartTime > 0 {
		filters["start_time"] = r.StartTime
	}
	if r.EndTime > 0 {
		filters["end_time"] = r.EndTime
	}

	// 调用 RCA 业务服务
	Service(ctx, func() (interface{}, interface{}) {
		return services.RCAService.GetIncidents(ctx, filters, r.Page, r.PageSize)
	})
}

// GetReport 获取 RCA 统计报告
// GET /api/w8t/rca/report?startTime=1707000000&endTime=1707086400
// 响应: 报告数据（MTTD、MTTR、根因频率、服务分布）
func (rcaController rcaController) GetReport(ctx *gin.Context) {
	r := new(types.RequestRCAReport)
	BindQuery(ctx, r)

	// 时间范围验证
	if r.StartTime <= 0 || r.EndTime <= 0 {
		response.Fail(ctx, "时间范围不能为空，需要提供 startTime 和 endTime（Unix 时间戳）", "failed")
		return
	}
	if r.StartTime >= r.EndTime {
		response.Fail(ctx, "开始时间必须小于结束时间", "failed")
		return
	}

	// 调用 RCA 业务服务
	Service(ctx, func() (interface{}, interface{}) {
		return services.RCAService.GetReport(ctx, r.StartTime, r.EndTime)
	})
}
