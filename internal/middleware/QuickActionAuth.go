package middleware

import (
	"fmt"
	"github.com/bytedance/sonic"
	"github.com/gin-gonic/gin"
	"net/url"
	"time"
	"alertHub/internal/ctx"
	"alertHub/internal/models"
	"alertHub/pkg/response"
	"alertHub/pkg/tools"
	"alertHub/pkg/utils"
)

// QuickActionAuth 快捷操作Token验证中间件
// 用于验证快捷操作链接中的Token，无需传统的登录Token
func QuickActionAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		// 获取Token（支持query参数和form参数）
		token := c.Query("token")
		if token == "" {
			token = c.PostForm("token")
		}

		// Token为空，返回错误
		if token == "" {
			response.Fail(c, "缺少Token参数", "")
			c.Abort()
			return
		}

		// 获取系统配置中的SecretKey
		settings, err := ctx.DO().DB.Setting().Get()
		if err != nil {
			response.Fail(c, "获取系统配置失败", "")
			c.Abort()
			return
		}

		secretKey := settings.QuickActionConfig.SecretKey
		if secretKey == "" {
			response.Fail(c, "快捷操作未配置密钥", "")
			c.Abort()
			return
		}

		// 验证Token
		payload, err := utils.VerifyQuickToken(token, secretKey)
		if err != nil {
			response.Fail(c, "Token验证失败: "+err.Error(), "")
			c.Abort()
			return
		}

		// 将Token信息设置到上下文中(不包含Username,需要登录后获取)
		c.Set("TenantID", payload.TenantId)
		c.Set("Fingerprint", payload.Fingerprint)

		c.Next()
	}
}

// QuickActionLoginAuth 快捷操作登录验证中间件
// 专门用于快捷操作场景：检查用户是否已登录，未登录则重定向到登录页面
// 区别于标准Auth中间件（返回JSON 401），本中间件返回HTML重定向响应
func QuickActionLoginAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		// 获取 JWT Token（优先从 Header，其次从 Cookie）
		tokenStr := c.Request.Header.Get("Authorization")
		if tokenStr == "" {
			// 尝试从 Cookie 获取（适配浏览器环境）
			tokenStr, _ = c.Cookie("Authorization")
			if tokenStr != "" {
				// Cookie中的token可能没有Bearer前缀，需要添加
				if len(tokenStr) > 0 && tokenStr[:len(tools.TokenType)] != tools.TokenType {
					tokenStr = tools.TokenType + " " + tokenStr
				}
			}
		}

		// 未登录：重定向到登录页面
		if tokenStr == "" {
			redirectToLogin(c)
			return
		}

		// 校验 Token 是否有效
		code, ok := isQuickActionTokenValid(c, tokenStr)
		if !ok {
			// Token无效：重定向到登录页面
			redirectToLogin(c)
			return
		}

		// Token有效但可能过期
		if code == 401 {
			redirectToLogin(c)
			return
		}

		// Token验证成功，继续执行后续中间件
		c.Next()
	}
}

// redirectToLogin 重定向到登录页面
// 保留原始URL参数（包括快捷操作token和操作参数），登录成功后跳转回来
func redirectToLogin(c *gin.Context) {
	// 获取快捷操作配置（获取ApiUrl）
	settings, err := ctx.DO().DB.Setting().Get()
	if err != nil {
		// 配置获取失败，渲染错误页面
		renderLoginErrorPage(c, "系统配置错误，无法跳转到登录页面")
		return
	}

	// 构建当前完整URL（需要使用绝对路径，因为从外部访问）
	apiUrl := settings.QuickActionConfig.ApiUrl
	if apiUrl == "" {
		apiUrl = settings.QuickActionConfig.BaseUrl
	}
	currentURL := apiUrl + c.Request.URL.String()

	// 构建后端登录页面URL（快捷操作专用登录页）
	loginURL := fmt.Sprintf("/api/v1/alert/quick-login?redirect=%s", url.QueryEscape(currentURL))

	// 返回HTML重定向响应
	c.Redirect(302, loginURL)
	c.Abort()
}

// renderLoginErrorPage 渲染登录错误页面
func renderLoginErrorPage(c *gin.Context, errorMsg string) {
	html := fmt.Sprintf(`
<!DOCTYPE html>
<html>
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>登录失败</title>
    <style>
        * { margin: 0; padding: 0; box-sizing: border-box; }
        body {
            font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif;
            display: flex;
            justify-content: center;
            align-items: center;
            min-height: 100vh;
            background: linear-gradient(135deg, #f5f7fa 0%%, #c3cfe2 100%%);
            padding: 20px;
        }
        .container {
            text-align: center;
            background: white;
            padding: 40px 30px;
            border-radius: 16px;
            box-shadow: 0 10px 40px rgba(0,0,0,0.15);
            max-width: 400px;
        }
        .icon { font-size: 64px; margin-bottom: 20px; }
        h1 { color: #ff4d4f; margin: 0 0 15px 0; font-size: 24px; }
        .error-msg {
            color: #666;
            font-size: 14px;
            line-height: 1.6;
            background: #fff2f0;
            padding: 12px;
            border-radius: 8px;
            border-left: 3px solid #ff4d4f;
        }
    </style>
</head>
<body>
    <div class="container">
        <div class="icon">⚠️</div>
        <h1>登录失败</h1>
        <div class="error-msg">%s</div>
    </div>
</body>
</html>
    `, errorMsg)

	c.Header("Content-Type", "text/html; charset=utf-8")
	c.String(400, html)
	c.Abort()
}

// isQuickActionTokenValid 验证快捷操作场景下的JWT Token有效性
// 复用标准Auth中间件的验证逻辑，但适配快捷操作场景
func isQuickActionTokenValid(c *gin.Context, tokenStr string) (int64, bool) {
	// Bearer Token, 获取 Token 值
	if len(tokenStr) <= len(tools.TokenType)+1 {
		return 400, false
	}
	tokenStr = tokenStr[len(tools.TokenType)+1:]

	token, err := tools.ParseToken(tokenStr)
	if err != nil {
		return 400, false
	}

	// 发布者校验
	if token.StandardClaims.Issuer != tools.AppGuardName {
		return 400, false
	}

	// 密码校验, 当修改密码后其他已登陆的终端会被下线
	var user models.Member
	result, err := ctx.DO().Redis.Redis().Get("uid-" + token.ID).Result()
	if err != nil {
		return 400, false
	}
	_ = sonic.Unmarshal([]byte(result), &user)

	if token.Pass != user.Password {
		return 401, false
	}

	// 校验过期时间
	ok := token.StandardClaims.VerifyExpiresAt(time.Now().Unix(), false)
	if !ok {
		return 401, false
	}

	// Token验证成功，将用户信息设置到上下文中（供后续handler使用）
	c.Set("username", user.UserName)
	c.Set("userId", user.UserId)
	// 注意：Member模型使用Tenants数组，但快捷操作已从QuickActionAuth中获取TenantID
	// 这里不设置tenantId，避免覆盖QuickActionAuth设置的值

	return 200, true
}