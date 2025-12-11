package templates

import "fmt"

// RenderLoginPage 渲染快捷操作登录页面
// redirectURL: 登录成功后的跳转地址(原始快捷操作URL)
// 用于快捷操作场景的专用登录页面,登录成功后自动跳转回原始操作URL
func RenderLoginPage(redirectURL string) string {
	// 使用 fmt.Sprintf 包裹 redirectURL 为 JSON 字符串格式
	redirectURLJSON := fmt.Sprintf(`"%s"`, redirectURL)

	return fmt.Sprintf(`
<!DOCTYPE html>
<html>
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0, maximum-scale=1.0, user-scalable=no">
    <title>登录 - 快捷操作</title>
    <style>
        * { margin: 0; padding: 0; box-sizing: border-box; }
        body {
            font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif;
            background: linear-gradient(135deg, #e3f2fd 0%%, #bbdefb 100%%);
            display: flex;
            justify-content: center;
            align-items: center;
            min-height: 100vh;
            padding: 20px;
        }
        .login-container {
            background: white;
            border-radius: 16px;
            padding: 40px 30px;
            box-shadow: 0 10px 40px rgba(0,0,0,0.1);
            max-width: 400px;
            width: 100%%;
        }
        .logo {
            text-align: center;
            font-size: 48px;
            margin-bottom: 10px;
        }
        h1 {
            text-align: center;
            color: #333;
            margin-bottom: 10px;
            font-size: 24px;
        }
        .subtitle {
            text-align: center;
            color: #999;
            font-size: 14px;
            margin-bottom: 30px;
        }
        .form-group {
            margin-bottom: 20px;
        }
        label {
            display: block;
            margin-bottom: 8px;
            color: #333;
            font-weight: 600;
            font-size: 14px;
        }
        input {
            width: 100%%;
            padding: 12px;
            border: 1px solid #ddd;
            border-radius: 8px;
            font-size: 14px;
            transition: border-color 0.3s;
        }
        input:focus {
            outline: none;
            border-color: #1976d2;
        }
        .login-btn {
            width: 100%%;
            padding: 14px;
            background: linear-gradient(135deg, #1976d2 0%%, #1565c0 100%%);
            color: white;
            border: none;
            border-radius: 8px;
            font-size: 16px;
            font-weight: 600;
            cursor: pointer;
            transition: transform 0.2s, box-shadow 0.2s;
        }
        .login-btn:hover {
            transform: translateY(-2px);
            box-shadow: 0 4px 12px rgba(25, 118, 210, 0.3);
        }
        .login-btn:active {
            transform: translateY(0);
        }
        .login-btn:disabled {
            opacity: 0.6;
            cursor: not-allowed;
            transform: none;
        }
        .error-msg {
            color: #ff4d4f;
            font-size: 14px;
            margin-top: 10px;
            padding: 10px;
            background: #fff2f0;
            border-radius: 8px;
            display: none;
        }
        .tip {
            text-align: center;
            color: #999;
            font-size: 12px;
            margin-top: 20px;
        }
    </style>
</head>
<body>
    <div class="login-container">
        <div class="logo">🔐</div>
        <h1>登录验证</h1>
        <p class="subtitle">完成登录后自动执行快捷操作</p>

        <form id="loginForm">
            <div class="form-group">
                <label for="username">用户名</label>
                <input type="text" id="username" name="username" required autocomplete="username">
            </div>

            <div class="form-group">
                <label for="password">密码</label>
                <input type="password" id="password" name="password" required autocomplete="current-password">
            </div>

            <button type="submit" class="login-btn" id="loginBtn">登录</button>
            <div class="error-msg" id="errorMsg"></div>
        </form>

        <p class="tip">🔒 安全连接 · 操作将记录审计日志</p>
    </div>

    <script>
        const loginForm = document.getElementById('loginForm');
        const loginBtn = document.getElementById('loginBtn');
        const errorMsg = document.getElementById('errorMsg');
        const redirectURL = %s; // 原始快捷操作URL

        loginForm.onsubmit = async (e) => {
            e.preventDefault();

            const username = document.getElementById('username').value;
            const password = document.getElementById('password').value;

            // 禁用按钮
            loginBtn.disabled = true;
            loginBtn.textContent = '登录中...';
            errorMsg.style.display = 'none';

            try {
                const response = await fetch('/api/v1/alert/quick-login', {
                    method: 'POST',
                    headers: {
                        'Content-Type': 'application/json',
                    },
                    body: JSON.stringify({
                        username: username,
                        password: password,
                        redirect: redirectURL
                    })
                });

                const result = await response.json();

                if (response.ok && result.code === 200) {
                    // 登录成功,保存token到Cookie
                    document.cookie = 'Authorization=' + result.data.token + '; path=/; max-age=86400';

                    // 跳转回原始URL
                    window.location.href = redirectURL;
                } else {
                    // 登录失败
                    errorMsg.textContent = result.msg || '登录失败,请检查用户名和密码';
                    errorMsg.style.display = 'block';
                    loginBtn.disabled = false;
                    loginBtn.textContent = '登录';
                }
            } catch (error) {
                errorMsg.textContent = '网络错误: ' + error.message;
                errorMsg.style.display = 'block';
                loginBtn.disabled = false;
                loginBtn.textContent = '登录';
            }
        };
    </script>
</body>
</html>
    `, redirectURLJSON)
}
