package api

import (
	"alertHub/internal/middleware"
	"alertHub/internal/services"
	"alertHub/internal/types"
	"github.com/gin-gonic/gin"
)

type aiController struct{}

var AiController = new(aiController)

func (aiController aiController) API(gin *gin.RouterGroup) {
	a := gin.Group("ai")
	a.Use(
		middleware.Auth(),
		middleware.CasbinPermission(), // 使用Casbin权限中间件
		middleware.ParseTenant(),
		middleware.AuditingLog(),
	)
	{
		a.POST("chat", aiController.Chat)
	}
}

func (aiController aiController) Chat(ctx *gin.Context) {
	r := new(types.RequestAiChatContent)
	r.Content = ctx.PostForm("content")
	r.RuleId = ctx.PostForm("rule_id")
	r.RuleName = ctx.PostForm("rule_name")
	r.Deep = ctx.PostForm("deep")
	r.SearchQL = ctx.PostForm("search_ql")

	Service(ctx, func() (interface{}, interface{}) {
		return services.AiService.Chat(r)
	})
}
