package sender

import (
	"bytes"
	"fmt"
	"io"
	"strings"
	"alertHub/pkg/tools"

	"github.com/bytedance/sonic"
	"github.com/zeromicro/go-zero/core/logc"
)

type (
	// DingDingSender 钉钉发送策略
	DingDingSender struct{}

	DingResponse struct {
		Code int    `json:"errcode"`
		Msg  string `json:"errmsg"`
	}
)

var DingTestContent = fmt.Sprintf(`{
	"msgtype": "text",
    "text": {
        "content": "%s"
    }
}`, RobotTestContent)

func NewDingSender() SendInter {
	return &DingDingSender{}
}

func (d *DingDingSender) Send(params SendParams) error {
	return d.post(params.Hook, params.Content)
}

// getDingdingTestKeyword 获取钉钉测试消息的关键词
// 根据通知对象名称智能识别关键词，如果没有则使用默认关键词"告警"
func getDingdingTestKeyword(noticeName string) string {
	// 默认关键词：告警（最常见的钉钉机器人关键词）
	defaultKeyword := "告警"

	// 如果通知对象名称为空，直接返回默认关键词
	if noticeName == "" {
		return defaultKeyword
	}

	// 检查通知对象名称中是否包含关键词提示
	// 例如：如果通知对象名称包含"报警"，则使用"报警"作为关键词
	if strings.Contains(noticeName, "报警") {
		return "报警"
	}
	if strings.Contains(noticeName, "Alert") {
		return "Alert"
	}
	if strings.Contains(noticeName, "监控") {
		return "监控"
	}

	return defaultKeyword
}

// buildDingdingTestContent 构建钉钉测试消息内容（包含关键词）
func buildDingdingTestContent(noticeName string) string {
	// 获取关键词
	keyword := getDingdingTestKeyword(noticeName)

	// 构建测试消息内容，确保包含关键词
	testContent := fmt.Sprintf("%s %s", keyword, RobotTestContent)

	// 构建JSON消息
	return fmt.Sprintf(`{
	"msgtype": "text",
    "text": {
        "content": "%s"
    }
}`, testContent)
}

func (d *DingDingSender) Test(params SendParams) error {
	// 动态生成测试消息内容，包含关键词
	testContent := buildDingdingTestContent(params.NoticeName)
	return d.post(params.Hook, testContent)
}

func (d *DingDingSender) post(hook, content string) error {
	// 记录发送内容（用于调试，仅记录前500字符）
	contentPreview := content
	if len(content) > 500 {
		contentPreview = content[:500] + "..."
	}
	logc.Info(nil, fmt.Sprintf("发送钉钉消息，Hook: %s, 内容预览: %s", hook, contentPreview))

	cardContentByte := bytes.NewReader([]byte(content))
	res, err := tools.Post(nil, hook, cardContentByte, 10)
	if err != nil {
		logc.Error(nil, fmt.Sprintf("钉钉消息发送失败（网络错误）: Hook=%s, 错误=%s", hook, err.Error()))
		return fmt.Errorf("钉钉消息发送失败: %w", err)
	}
	defer res.Body.Close()

	// 读取响应体（用于调试）
	bodyBytes, err := io.ReadAll(res.Body)
	if err != nil {
		logc.Error(nil, fmt.Sprintf("读取钉钉响应失败: Hook=%s, 错误=%s", hook, err.Error()))
		return fmt.Errorf("读取钉钉响应失败: %w", err)
	}

	var response DingResponse
	// 使用 sonic 解析 JSON（更高效）
	if err := sonic.Unmarshal(bodyBytes, &response); err != nil {
		logc.Error(nil, fmt.Sprintf("解析钉钉响应失败: Hook=%s, 响应内容=%s, 错误=%s", hook, string(bodyBytes), err.Error()))
		return fmt.Errorf("解析钉钉响应失败: %w, 响应内容: %s", err, string(bodyBytes))
	}

	if response.Code != 0 {
		logc.Error(nil, fmt.Sprintf("钉钉消息发送失败（API错误）: Hook=%s, Code=%d, Msg=%s, 发送内容预览=%s",
			hook, response.Code, response.Msg, contentPreview))
		return fmt.Errorf("钉钉API返回错误: Code=%d, Msg=%s", response.Code, response.Msg)
	}

	logc.Info(nil, fmt.Sprintf("钉钉消息发送成功: Hook=%s", hook))
	return nil
}
