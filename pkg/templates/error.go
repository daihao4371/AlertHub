package templates

import (
	"fmt"
	"strings"
)

// RenderErrorPage 渲染操作失败页面
// errorMsg: 错误消息文本
// 根据错误类型自动选择对应的图标、标题和提示信息
func RenderErrorPage(errorMsg string) string {
	// 根据错误消息判断错误类型,选择对应的显示内容
	var icon, title, tip string
	if containsAny(errorMsg, []string{"未找到指纹", "告警不存在"}) {
		icon = "⏰"
		title = "告警已失效"
		tip = "此告警可能已被处理或链接已过期(有效期24小时)"
	} else if containsAny(errorMsg, []string{"Token已过期", "Token验证失败"}) {
		icon = "🔒"
		title = "链接已过期"
		tip = "快捷操作链接有效期为24小时,请从最新的告警通知中重新访问"
	} else {
		icon = "❌"
		title = "操作失败"
		tip = "请稍后重试或联系管理员"
	}

	return fmt.Sprintf(`
<!DOCTYPE html>
<html>
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0, maximum-scale=1.0, user-scalable=no">
    <title>%s</title>
    <style>
        * {
            margin: 0;
            padding: 0;
            box-sizing: border-box;
        }
        body {
            font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, "Helvetica Neue", Arial, sans-serif;
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
            width: 100%%;
        }
        .icon { font-size: 64px; margin-bottom: 20px; }
        h1 { color: #ff9800; margin: 0 0 15px 0; font-size: 24px; font-weight: 600; }
        .error-msg {
            color: #666;
            font-size: 14px;
            line-height: 1.6;
            background: #fff3e0;
            padding: 12px;
            border-radius: 8px;
            border-left: 3px solid #ff9800;
            text-align: left;
            word-break: break-word;
        }
        .divider { height: 1px; background: #f0f0f0; margin: 20px 0; }
        .tip { color: #999; font-size: 12px; line-height: 1.5; }
    </style>
</head>
<body>
    <div class="container">
        <div class="icon">%s</div>
        <h1>%s</h1>
        <div class="error-msg">%s</div>
        <div class="divider"></div>
        <p class="tip">%s</p>
    </div>
</body>
</html>
    `, title, icon, title, errorMsg, tip)
}

// containsAny 检查字符串是否包含任意一个子串(辅助函数)
// 用于错误类型判断,比逐个 strings.Contains 更简洁
func containsAny(s string, substrs []string) bool {
	for _, substr := range substrs {
		if strings.Contains(s, substr) {
			return true
		}
	}
	return false
}
