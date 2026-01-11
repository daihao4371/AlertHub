package api

import (
	"alertHub/internal/middleware"
	"alertHub/internal/services"
	"alertHub/internal/types"
	"alertHub/pkg/response"

	"github.com/gin-gonic/gin"
)

// thirdPartyAlertController 第三方告警接收控制器
type thirdPartyAlertController struct{}

// ThirdPartyAlertController 控制器单例
var ThirdPartyAlertController = new(thirdPartyAlertController)

// API 注册路由和中间件
// 包含两个路由组：
// 1. 公开的Webhook接收接口（无需认证，单独注册在 /api/webhook/:webhookId）
// 2. 告警记录查询接口（需要认证，注册在 /api/w8t/thirdPartyAlert）
func (thirdPartyAlertController thirdPartyAlertController) API(gin *gin.RouterGroup) {
	// 读操作路由组（查询告警列表，需要认证）
	alertB := gin.Group("thirdPartyAlert")
	alertB.Use(
		middleware.Auth(),
		middleware.CasbinPermission(),
		middleware.ParseTenant(),
	)
	{
		alertB.GET("list", thirdPartyAlertController.List)
	}
}

// PublicAPI 注册公开的Webhook接收接口（无需认证）
// 这个方法会在路由注册时单独调用，将接口注册到 /api 根路径下
func (thirdPartyAlertController thirdPartyAlertController) PublicAPI(gin *gin.RouterGroup) {
	// 公开的Webhook接收接口（无需任何中间件）
	gin.POST("webhook/:webhookId", thirdPartyAlertController.ReceiveAlert)
}

// ReceiveAlert 接收第三方告警（公开接口，无需认证）
// 路由：POST /api/webhook/:webhookId
// 请求体：任意JSON格式
// 响应：{"success": true, "message": "告警接收成功", "alertId": "xxx", "timestamp": 1234567890}
func (thirdPartyAlertController thirdPartyAlertController) ReceiveAlert(ctx *gin.Context) {
	// 从路径参数获取webhookId
	webhookId := ctx.Param("webhookId")
	if webhookId == "" {
		response.Fail(ctx, nil, "Webhook ID不能为空")
		return
	}

	// 读取原始JSON数据
	var rawData map[string]interface{}
	if err := ctx.ShouldBindJSON(&rawData); err != nil {
		response.Fail(ctx, nil, "无效的JSON格式")
		return
	}

	// 读取请求头（用于日志和调试）
	headers := make(map[string]string)
	for key, values := range ctx.Request.Header {
		if len(values) > 0 {
			headers[key] = values[0]
		}
	}

	// 构建请求对象
	r := &services.RequestReceiveAlert{
		WebhookId: webhookId,
		RawData:   rawData,
		Headers:   headers,
	}

	// 调用服务层
	data, err := services.ThirdPartyAlertService.ReceiveAlert(r)
	if err != nil {
		response.Fail(ctx, data, err.(error).Error())
		return
	}

	response.Success(ctx, data, "success")
}

// List 分页查询告警记录列表
// 路由：GET /api/w8t/thirdPartyAlert/list
// 需要认证和租户隔离
func (thirdPartyAlertController thirdPartyAlertController) List(ctx *gin.Context) {
	r := new(types.RequestAlertQuery)
	BindQuery(ctx, r)

	// 从中间件获取租户ID
	tid, _ := ctx.Get("TenantID")
	r.TenantId = tid.(string)

	// 调用服务层
	Service(ctx, func() (interface{}, interface{}) {
		return services.ThirdPartyAlertService.List(r)
	})
}
