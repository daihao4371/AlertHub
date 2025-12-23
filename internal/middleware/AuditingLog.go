package middleware

import (
	"bytes"
	"github.com/gin-gonic/gin"
	"github.com/zeromicro/go-zero/core/logc"
	"io"
	"io/ioutil"
	"strings"
	"time"
	"alertHub/internal/ctx"
	"alertHub/internal/models"
	"alertHub/pkg/response"
	"alertHub/pkg/tools"
)

func AuditingLog() gin.HandlerFunc {
	return func(context *gin.Context) {
		// Operation user
		var username string
		createBy := tools.GetUser(context.Request.Header.Get("Authorization"))
		if createBy != "" {
			username = createBy
		} else {
			username = "用户未登录"
		}

		// Response log
		body := context.Request.Body
		readBody, err := io.ReadAll(body)
		if err != nil {
			logc.Error(ctx.DO().Ctx, err)
			return
		}
		// 将 body 数据放回请求中
		context.Request.Body = ioutil.NopCloser(bytes.NewBuffer(readBody))

		tid := context.Request.Header.Get(TenantIDHeaderKey)
		if tid == "" {
			response.Fail(context, "租户ID不能为空", "failed")
			context.Abort()
			return
		}

		// 当请求处理完成后才会执行 Next() 后面的代码
		context.Next()

		// 生成操作描述，使用通用格式
		actionDesc := generateActionDescription(context.Request.Method, context.Request.URL.Path)
		
		auditLog := models.AuditLog{
			TenantId:   tid,
			ID:         "Trace" + tools.RandId(),
			Username:   username,
			IPAddress:  context.ClientIP(),
			Method:     context.Request.Method,
			Path:       context.Request.URL.Path,
			CreatedAt:  time.Now().Unix(),
			StatusCode: context.Writer.Status(),
			Body:       string(readBody),
			AuditType:  actionDesc,
		}

		c := ctx.DO()
		err = c.DB.AuditLog().Create(auditLog)
		if err != nil {
			response.Fail(context, "审计日志写入数据库失败, "+err.Error(), "failed")
			context.Abort()
			return
		}
	}
}

// generateActionDescription 生成操作描述
func generateActionDescription(method, path string) string {
	// 提取路径的最后一部分作为操作名称
	pathSegments := strings.Split(path, "/")
	var actionName string
	if len(pathSegments) > 0 {
		actionName = pathSegments[len(pathSegments)-1]
	}
	
	// 根据HTTP方法生成操作描述
	switch method {
	case "POST":
		return "创建" + actionName
	case "PUT":
		return "更新" + actionName  
	case "DELETE":
		return "删除" + actionName
	case "GET":
		return "查看" + actionName
	default:
		return method + " " + actionName
	}
}
