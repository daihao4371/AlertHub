package api

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"alertHub/internal/middleware"
	"alertHub/internal/models"
	"alertHub/internal/services"
	"alertHub/internal/types"
	"alertHub/pkg/response"
	"alertHub/pkg/tools"
)

type processTraceController struct{}

var ProcessTraceController = new(processTraceController)

// API 注册路由
func (processTraceController processTraceController) API(gin *gin.RouterGroup) {
	processTrace := gin.Group("process-trace")
	processTrace.Use(
		middleware.Auth(),
		middleware.CasbinPermission(),
		middleware.ParseTenant(),
		middleware.AuditingLog(),
	)
	{
		processTrace.GET("", processTraceController.GetProcessTrace)
		processTrace.GET("list", processTraceController.GetProcessTraceList)
		processTrace.PUT("status", processTraceController.UpdateProcessStatus)
		processTrace.PUT("ai-analysis", processTraceController.UpdateAIAnalysis)
		processTrace.GET("operation-logs", processTraceController.GetOperationLogs)
		processTrace.GET("statistics", processTraceController.GetProcessStatistics)
	}
}

// GetProcessTrace 获取处理流程追踪记录
// @Summary 获取处理流程追踪记录
// @Tags ProcessTrace
// @Accept json
// @Produce json
// @Param fingerprint query string true "告警指纹"
// @Success 200 {object} response.Response{data=models.ProcessTrace}
// @Router /api/w8t/process-trace [get]
func (processTraceController processTraceController) GetProcessTrace(ctx *gin.Context) {
	fingerprint := ctx.Query("fingerprint")

	if fingerprint == "" {
		response.Fail(ctx, nil, "fingerprint不能为空")
		return
	}

	tid, _ := ctx.Get("TenantID")
	tenantId := tid.(string)

	Service(ctx, func() (interface{}, interface{}) {
		processTrace, err := services.ProcessTraceService.GetProcessTraceByFingerprint(tenantId, fingerprint)
		if err != nil {
			// 如果是记录不存在的错误，返回更友好的提示
			if strings.Contains(err.Error(), "未找到指纹") {
				return nil, fmt.Errorf("该告警还没有创建处理流程追踪记录，请先创建")
			}
			return nil, err
		}
		return processTrace, nil
	})
}

// GetProcessTraceList 获取处理流程追踪记录列表
// @Summary 获取处理流程追踪记录列表
// @Tags ProcessTrace
// @Accept json
// @Produce json
// @Param eventId query string false "告警事件ID"
// @Param faultCenterId query string false "故障中心ID"
// @Param page query int false "页码" default(1)
// @Param pageSize query int false "每页数量" default(10)
// @Success 200 {object} response.Response{data=types.ProcessTraceListResponse}
// @Router /api/w8t/process-trace/list [get]
func (processTraceController processTraceController) GetProcessTraceList(ctx *gin.Context) {
	tid, _ := ctx.Get("TenantID")
	tenantId := tid.(string)

	// 获取查询参数
	eventId := ctx.Query("eventId")
	faultCenterId := ctx.Query("faultCenterId")
	page, _ := strconv.Atoi(ctx.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(ctx.DefaultQuery("pageSize", "10"))

	Service(ctx, func() (interface{}, interface{}) {
		return services.ProcessTraceService.GetProcessTraceList(tenantId, eventId, faultCenterId, page, pageSize)
	})
}

// UpdateProcessStatus 更新处理状态
// @Summary 更新处理状态
// @Tags ProcessTrace
// @Accept json
// @Produce json
// @Param data body types.UpdateProcessStatusRequest true "请求参数"
// @Success 200 {object} response.Response
// @Router /api/w8t/process-trace/status [put]
func (processTraceController processTraceController) UpdateProcessStatus(ctx *gin.Context) {
	r := new(types.UpdateProcessStatusRequest)
	BindJson(ctx, r)

	tid, _ := ctx.Get("TenantID")
	tenantId := tid.(string)

	// 获取当前操作用户 - 使用tools.GetUser从token中获取用户名
	username := tools.GetUser(ctx.Request.Header.Get("Authorization"))

	Service(ctx, func() (interface{}, interface{}) {
		err := services.ProcessTraceService.UpdateProcessStatus(tenantId, r.EventId, username,
			models.ProcessTraceStatus(r.Status), r.AssignedUser, r.Description)
		if err != nil {
			// 检查是否是状态转换验证错误，提供更友好的错误信息
			if strings.Contains(err.Error(), "状态转换验证失败") {
				return nil, fmt.Errorf("状态转换不被允许: %s", err.Error()[8:]) // 去掉"状态转换验证失败: "前缀
			}
			return nil, err
		}
		return nil, nil
	})
}

// UpdateAIAnalysis 更新AI分析结果
// @Summary 更新AI分析结果
// @Tags ProcessTrace
// @Accept json
// @Produce json
// @Param data body types.UpdateAIAnalysisRequest true "请求参数"
// @Success 200 {object} response.Response
// @Router /api/v1/process-trace/ai-analysis [put]
func (processTraceController processTraceController) UpdateAIAnalysis(ctx *gin.Context) {
	r := new(types.UpdateAIAnalysisRequest)
	BindJson(ctx, r)

	tid, _ := ctx.Get("TenantID")
	tenantId := tid.(string)

	analysisData := &models.AIAnalysisData{
		AnalysisType:   r.AnalysisType,
		AnalysisResult: r.AnalysisResult,
		Confidence:     r.Confidence,
		Suggestions:    r.Suggestions,
		AnalysisTime:   time.Now().Unix(),
	}

	Service(ctx, func() (interface{}, interface{}) {
		err := services.ProcessTraceService.UpdateAIAnalysis(tenantId, r.EventId, r.StepName, analysisData)
		if err != nil {
			return nil, err
		}
		return nil, nil
	})
}

// GetOperationLogs 获取操作日志列表
// @Summary 获取操作日志列表
// @Tags ProcessTrace
// @Accept json
// @Produce json
// @Param fingerprint query string true "告警指纹"
// @Param page query int false "页码" default(1)
// @Param pageSize query int false "每页数量" default(10)
// @Success 200 {object} response.Response{data=types.OperationLogListResponse}
// @Router /api/w8t/process-trace/operation-logs [get]
func (processTraceController processTraceController) GetOperationLogs(ctx *gin.Context) {
	fingerprint := ctx.Query("fingerprint")

	if fingerprint == "" {
		response.Fail(ctx, nil, "fingerprint不能为空")
		return
	}

	tid, _ := ctx.Get("TenantID")
	tenantId := tid.(string)

	page, _ := strconv.Atoi(ctx.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(ctx.DefaultQuery("pageSize", "10"))

	Service(ctx, func() (interface{}, interface{}) {
		logs, total, err := services.ProcessTraceService.GetOperationLogsByFingerprint(tenantId, fingerprint, page, pageSize)
		if err != nil {
			return nil, err
		}

		resp := types.OperationLogListResponse{
			List:     logs,
			Total:    total,
			Page:     page,
			PageSize: pageSize,
		}
		return resp, nil
	})
}

// GetProcessStatistics 获取流程统计数据
// @Summary 获取流程统计数据
// @Tags ProcessTrace
// @Accept json
// @Produce json
// @Param startTime query int64 false "开始时间戳"
// @Param endTime query int64 false "结束时间戳"
// @Success 200 {object} response.Response{data=map[string]interface{}}
// @Router /api/v1/process-trace/statistics [get]
func (processTraceController processTraceController) GetProcessStatistics(ctx *gin.Context) {
	tid, _ := ctx.Get("TenantID")
	tenantId := tid.(string)

	startTime, _ := strconv.ParseInt(ctx.DefaultQuery("startTime", "0"), 10, 64)
	endTime, _ := strconv.ParseInt(ctx.DefaultQuery("endTime", strconv.FormatInt(time.Now().Unix(), 10)), 10, 64)

	// 如果未指定开始时间，默认为30天前
	if startTime == 0 {
		startTime = time.Now().AddDate(0, 0, -30).Unix()
	}

	Service(ctx, func() (interface{}, interface{}) {
		return services.ProcessTraceService.GetProcessStatistics(tenantId, startTime, endTime)
	})
}
