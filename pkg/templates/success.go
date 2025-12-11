package templates

import "fmt"

// RenderSuccessPage 渲染操作成功页面(移动端友好)
// actionName: 操作名称(如"认领"、"静默"、"标记已处理")
// 返回完整的 HTML 页面字符串
func RenderSuccessPage(actionName string) string {
	return fmt.Sprintf(`
<!DOCTYPE html>
<html>
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0, maximum-scale=1.0, user-scalable=no">
    <title>操作成功</title>
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
            background: linear-gradient(135deg, #a8edea 0%%, #fed6e3 100%%);
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
            animation: slideUp 0.4s ease-out;
        }
        @keyframes slideUp {
            from {
                opacity: 0;
                transform: translateY(20px);
            }
            to {
                opacity: 1;
                transform: translateY(0);
            }
        }
        .icon {
            font-size: 64px;
            margin-bottom: 20px;
            animation: scaleIn 0.5s ease-out 0.2s both;
        }
        @keyframes scaleIn {
            from {
                transform: scale(0);
            }
            to {
                transform: scale(1);
            }
        }
        h1 {
            color: #52c41a;
            margin: 0 0 15px 0;
            font-size: 24px;
            font-weight: 600;
        }
        p {
            color: #666;
            font-size: 14px;
            line-height: 1.6;
        }
        .divider {
            height: 1px;
            background: #f0f0f0;
            margin: 20px 0;
        }
        .tip {
            color: #999;
            font-size: 12px;
        }
    </style>
</head>
<body>
    <div class="container">
        <div class="icon">✅</div>
        <h1>%s成功</h1>
        <p>操作已成功完成</p>
        <div class="divider"></div>
        <p class="tip">您可以关闭此页面</p>
    </div>
    <script>
        // 3秒后自动尝试关闭页面(部分浏览器支持)
        setTimeout(function() {
            window.close();
        }, 3000);
    </script>
</body>
</html>
    `, actionName)
}
