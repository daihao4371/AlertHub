package templates

import "fmt"

// RenderSilenceForm 渲染自定义静默表单页面
// alertTitle: 告警名称(显示在表单顶部)
// fingerprint: 告警指纹(作为表单隐藏字段)
// token: 认证令牌(作为表单隐藏字段)
// 返回完整的 HTML 表单页面
func RenderSilenceForm(alertTitle, fingerprint, token string) string {
	return fmt.Sprintf(`
<!DOCTYPE html>
<html>
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0, maximum-scale=1.0, user-scalable=no">
    <title>自定义静默</title>
    <style>
        * {
            margin: 0;
            padding: 0;
            box-sizing: border-box;
        }
        body {
            font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, "Helvetica Neue", Arial, sans-serif;
            background: linear-gradient(135deg, #f5f7fa 0%%, #e4e9f2 100%%);
            padding: 20px;
            min-height: 100vh;
            display: flex;
            align-items: center;
            justify-content: center;
        }
        .container {
            max-width: 500px;
            width: 100%%;
            background: white;
            border-radius: 20px;
            padding: 32px;
            box-shadow: 0 20px 60px rgba(0, 0, 0, 0.08);
            animation: slideUp 0.4s cubic-bezier(0.16, 1, 0.3, 1);
        }
        @keyframes slideUp {
            from {
                opacity: 0;
                transform: translateY(30px);
            }
            to {
                opacity: 1;
                transform: translateY(0);
            }
        }
        .header {
            text-align: center;
            margin-bottom: 28px;
        }
        .icon {
            font-size: 48px;
            margin-bottom: 12px;
        }
        h2 {
            color: #1a1a1a;
            margin-bottom: 8px;
            font-size: 24px;
            font-weight: 700;
        }
        .subtitle {
            color: #666;
            font-size: 14px;
        }
        .alert-name {
            color: #555;
            font-size: 14px;
            margin-bottom: 28px;
            padding: 14px 16px;
            background: linear-gradient(135deg, #fff9f0 0%%, #fff5e6 100%%);
            border-radius: 10px;
            border-left: 4px solid #ff6b35;
            font-weight: 500;
            word-break: break-word;
        }
        .form-group {
            margin-bottom: 22px;
        }
        label {
            display: block;
            margin-bottom: 10px;
            font-weight: 600;
            color: #2c3e50;
            font-size: 14px;
        }
        select, textarea {
            width: 100%%;
            padding: 13px 16px;
            border: 2px solid #e8ecef;
            border-radius: 10px;
            font-size: 14px;
            font-family: inherit;
            transition: all 0.3s cubic-bezier(0.4, 0, 0.2, 1);
            background: #f8f9fa;
            color: #2c3e50;
        }
        select:hover, textarea:hover {
            border-color: #cbd5e0;
            background: #fff;
        }
        select:focus, textarea:focus {
            outline: none;
            border-color: #5b8def;
            background: white;
            box-shadow: 0 0 0 4px rgba(91, 141, 239, 0.1);
        }
        textarea {
            resize: vertical;
            min-height: 90px;
            line-height: 1.6;
        }
        .required {
            color: #ef4444;
            margin-left: 3px;
            font-weight: 700;
        }
        .submit-btn {
            width: 100%%;
            padding: 15px;
            background: linear-gradient(135deg, #5b8def 0%%, #4c7ce5 100%%);
            color: white;
            border: none;
            border-radius: 10px;
            font-size: 16px;
            font-weight: 600;
            cursor: pointer;
            transition: all 0.3s cubic-bezier(0.4, 0, 0.2, 1);
            box-shadow: 0 4px 14px rgba(91, 141, 239, 0.35);
            letter-spacing: 0.3px;
        }
        .submit-btn:hover {
            transform: translateY(-2px);
            box-shadow: 0 6px 20px rgba(91, 141, 239, 0.45);
            background: linear-gradient(135deg, #4c7ce5 0%%, #3d6dd6 100%%);
        }
        .submit-btn:active {
            transform: translateY(0);
            box-shadow: 0 4px 14px rgba(91, 141, 239, 0.35);
        }
        .submit-btn:disabled {
            opacity: 0.6;
            cursor: not-allowed;
            transform: none;
        }
        .tip {
            text-align: center;
            color: #94a3b8;
            font-size: 13px;
            margin-top: 20px;
            line-height: 1.5;
        }
        option {
            padding: 10px;
        }
    </style>
</head>
<body>
    <div class="container">
        <h2>🔕 自定义静默</h2>
        <div class="alert-name">告警: %s</div>

        <form id="silenceForm" action="/api/v1/alert/quick-silence" method="POST">
            <input type="hidden" name="fingerprint" value="%s">
            <input type="hidden" name="token" value="%s">

            <div class="form-group">
                <label>静默时长 <span class="required">*</span></label>
                <select name="duration" required>
                    <option value="1h">1小时 (临时问题)</option>
                    <option value="6h">6小时 (短期维护)</option>
                    <option value="24h">24小时 (已知问题,待修复)</option>
                    <option value="72h">3天 (计划维护)</option>
                    <option value="168h">7天 (长期维护)</option>
                    <option value="720h">30天 (规则误报,待优化)</option>
                </select>
            </div>

            <div class="form-group">
                <label>静默原因 <span class="required">*</span></label>
                <textarea
                    name="reason"
                    placeholder="请说明静默原因,如:服务器正在进行安全补丁升级"
                    required
                ></textarea>
            </div>

            <button type="submit" class="submit-btn" id="submitBtn">确认静默</button>
        </form>
    </div>

    <script>
        const form = document.getElementById('silenceForm');
        const submitBtn = document.getElementById('submitBtn');

        form.onsubmit = function(e) {
            const reason = document.querySelector('textarea[name="reason"]').value;

            if (!reason.trim()) {
                e.preventDefault();
                alert('请填写静默原因');
                return false;
            }

            // 禁用提交按钮,防止重复提交
            submitBtn.disabled = true;
            submitBtn.textContent = '提交中...';

            // 允许表单正常提交(传统POST方式,会导航到新页面)
            return true;
        };
    </script>
</body>
</html>
    `, alertTitle, fingerprint, token)
}
