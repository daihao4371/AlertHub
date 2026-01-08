package api

import (
	"alertHub/internal/middleware"
	"alertHub/internal/services"
	"alertHub/internal/types"
	"alertHub/pkg/response"
	"alertHub/pkg/templates"
	"fmt"

	"github.com/gin-gonic/gin"
)

type quickActionController struct{}

type InterQuickActionController interface {
	API(gin *gin.RouterGroup)
	QuickAction(ctx *gin.Context)
}

// QuickActionController 全局控制器实例（与其他控制器保持一致的命名规范）
var QuickActionController = &quickActionController{}

// API 注册快捷操作路由
// 所有快捷操作都需要登录验证,确保安全性和审计准确性
// 使用专用的QuickActionLoginAuth中间件,未登录时重定向到登录页面(而非返回JSON 401)
func (q quickActionController) API(gin *gin.RouterGroup) {
	alert := gin.Group("alert")

	// 登录相关路由（无需中间件）
	alert.GET("quick-login", q.QuickLogin)    // 显示登录页面
	alert.POST("quick-login", q.DoQuickLogin) // 处理登录请求

	// 快捷操作路由（需要登录验证）
	authGroup := alert.Group("")
	authGroup.Use(
		middleware.QuickActionAuth(), // Token验证(验证操作合法性)
		middleware.ParseTenant(),
		middleware.QuickActionLoginAuth(), // 登录验证(获取真实操作人,未登录则重定向)
		middleware.CasbinPermission(),
	)
	{
		authGroup.GET("quick-action", q.QuickAction)       // 快捷操作
		authGroup.GET("quick-silence", q.QuickSilenceForm) // 自定义静默表单
		authGroup.POST("quick-silence", q.QuickSilence)    // 提交自定义静默
	}
}

// QuickAction 快捷操作接口
// 支持的操作类型：claim（认领）、silence（静默）、resolve（标记已处理）
// 必须登录后才能操作,从JWT Token中获取真实操作人
func (q quickActionController) QuickAction(ctx *gin.Context) {
	// 解析参数
	action := ctx.Query("action")
	fingerprint := ctx.Query("fingerprint")
	duration := ctx.DefaultQuery("duration", "1h")

	// 从上下文获取租户ID(由QuickActionAuth中间件设置)
	tenantIdVal, exists := ctx.Get("TenantID")
	if !exists {
		ctx.Header("Content-Type", "text/html; charset=utf-8")
		ctx.String(400, templates.RenderErrorPage("缺少租户信息"))
		return
	}
	tenantId := tenantIdVal.(string)

	// 从JWT Token获取真实操作人(由Auth中间件设置)
	usernameVal, exists := ctx.Get("username")
	if !exists {
		ctx.Header("Content-Type", "text/html; charset=utf-8")
		ctx.String(400, templates.RenderErrorPage("用户未登录"))
		return
	}
	username := usernameVal.(string)
	clientIP := ctx.ClientIP()

	// 校验操作类型
	if action == "" {
		ctx.Header("Content-Type", "text/html; charset=utf-8")
		ctx.String(400, templates.RenderErrorPage("操作类型不能为空"))
		return
	}

	// 执行对应的操作
	var err error
	var actionName string

	switch action {
	case "claim":
		err = services.QuickActionService.ClaimAlert(tenantId, fingerprint, username, clientIP)
		actionName = "认领"

	case "silence":
		err = services.QuickActionService.SilenceAlert(tenantId, fingerprint, duration, username, clientIP)
		actionName = "静默"

	case "resolve":
		err = services.QuickActionService.ResolveAlert(tenantId, fingerprint, username, clientIP)
		actionName = "标记已处理"

	default:
		ctx.Header("Content-Type", "text/html; charset=utf-8")
		ctx.String(400, templates.RenderErrorPage("不支持的操作类型: "+action))
		return
	}

	// 处理操作结果
	if err != nil {
		ctx.Header("Content-Type", "text/html; charset=utf-8")
		ctx.String(400, templates.RenderErrorPage(err.Error()))
		return
	}

	// 渲染成功页面
	ctx.Header("Content-Type", "text/html; charset=utf-8")
	ctx.String(200, templates.RenderSuccessPage(actionName))
}

// QuickSilenceForm 渲染自定义静默表单页面
func (q quickActionController) QuickSilenceForm(ctx *gin.Context) {
	fingerprint := ctx.Query("fingerprint")
	token := ctx.Query("token")

	// 获取告警信息用于显示
	tenantIdVal, _ := ctx.Get("TenantID")
	tenantId := tenantIdVal.(string)

	// 获取告警详情(用于显示告警名称)
	alert, err := services.QuickActionService.GetAlertByFingerprint(tenantId, fingerprint)
	alertTitle := "告警"
	if err == nil && alert != nil {
		alertTitle = alert.RuleName
	}

	ctx.Header("Content-Type", "text/html; charset=utf-8")
	ctx.String(200, templates.RenderSilenceForm(alertTitle, fingerprint, token))
}

// QuickSilence 处理自定义静默提交
// 必须登录后才能操作
func (q quickActionController) QuickSilence(ctx *gin.Context) {
	// 解析参数
	fingerprint := ctx.PostForm("fingerprint")
	duration := ctx.PostForm("duration")
	reason := ctx.PostForm("reason")

	// 从上下文获取租户信息
	tenantIdVal, exists := ctx.Get("TenantID")
	if !exists {
		ctx.Header("Content-Type", "text/html; charset=utf-8")
		ctx.String(400, templates.RenderErrorPage("缺少租户信息"))
		return
	}
	tenantId := tenantIdVal.(string)

	// 从JWT Token获取真实操作人(必须登录)
	usernameVal, exists := ctx.Get("username")
	if !exists {
		ctx.Header("Content-Type", "text/html; charset=utf-8")
		ctx.String(400, templates.RenderErrorPage("用户未登录"))
		return
	}
	username := usernameVal.(string)
	clientIP := ctx.ClientIP()

	// 校验必填参数
	if fingerprint == "" || duration == "" || reason == "" {
		ctx.Header("Content-Type", "text/html; charset=utf-8")
		ctx.String(400, templates.RenderErrorPage("参数不完整"))
		return
	}

	// 执行静默操作,传入reason和clientIP
	err := services.QuickActionService.SilenceAlertWithReason(tenantId, fingerprint, duration, username, reason, clientIP)
	if err != nil {
		ctx.Header("Content-Type", "text/html; charset=utf-8")
		ctx.String(400, templates.RenderErrorPage(err.Error()))
		return
	}

	// 渲染成功页面
	ctx.Header("Content-Type", "text/html; charset=utf-8")
	ctx.String(200, templates.RenderSuccessPage("静默"))
}

// QuickLogin 渲染快捷操作登录页面
// 用于快捷操作场景的专用登录页面,登录成功后自动跳转回原始操作URL
func (q quickActionController) QuickLogin(ctx *gin.Context) {
	// 获取redirect参数(原始快捷操作URL)
	redirectURL := ctx.Query("redirect")
	if redirectURL == "" {
		ctx.Header("Content-Type", "text/html; charset=utf-8")
		ctx.String(400, templates.RenderErrorPage("缺少redirect参数"))
		return
	}

	ctx.Header("Content-Type", "text/html; charset=utf-8")
	ctx.String(200, templates.RenderLoginPage(redirectURL))
}

// DoQuickLogin 处理快捷操作登录请求
// 调用用户登录服务,返回JWT token
func (q quickActionController) DoQuickLogin(ctx *gin.Context) {
	var loginReq struct {
		Username string `json:"username"`
		Password string `json:"password"`
		Redirect string `json:"redirect"`
	}

	if err := ctx.ShouldBindJSON(&loginReq); err != nil {
		response.Fail(ctx, nil, "参数错误")
		return
	}

	// 调用用户登录服务
	result, errMsg := services.UserService.Login(&types.RequestUserLogin{
		UserName: loginReq.Username,
		Password: loginReq.Password,
	})

	if errMsg != nil {
		// 登录失败,errMsg是interface{}类型,转换为字符串
		response.Response(ctx, 401, 401, nil, fmt.Sprintf("%v", errMsg))
		return
	}

	// 登录成功,返回token
	response.Success(ctx, result, "登录成功")
}
