package api

import (
	"alertHub/internal/middleware"
	"alertHub/internal/services"
	"alertHub/internal/types"
	"alertHub/pkg/response"
	"fmt"

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
	// 解析请求参数（支持JSON）
	r := new(types.RequestAiChatContent)
	err := ctx.ShouldBind(r)
	if err != nil {
		response.Fail(ctx, err.Error(), "failed")
		ctx.Abort()
		return
	}

	// 设置 SSE 响应头进行流式传输
	ctx.Header("Content-Type", "text/event-stream")
	ctx.Header("Cache-Control", "no-cache")
	ctx.Header("Connection", "keep-alive")
	ctx.Header("Access-Control-Allow-Origin", "*")

	// 调用流式 AI Service 获取数据通道
	streamChan, errInterface := services.AiService.StreamChat(r)

	// 处理错误
	if errInterface != nil {
		response.Fail(ctx, fmt.Sprint(errInterface), "failed")
		return
	}

	// 实时发送流式数据 - 这是真正的流式传输
	// 注意：使用 map 包装 chunk，确保 Gin 进行 JSON 序列化
	// 这样可以正确处理内容中的换行符等特殊字符
	for chunk := range streamChan {
		// 每收到一个数据块立即发送给客户端
		// 使用 map 包装，确保 JSON 序列化处理换行符
		ctx.SSEvent("message", map[string]string{"content": chunk})
		// 立即刷新缓冲区，确保数据实时发送到客户端
		ctx.Writer.Flush()
	}

	// 流式传输完成，无需发送额外结束信号
	// 前端通过 ReadableStream 的 done 状态即可检测流结束
}
