package sender

import (
	"bytes"
	"fmt"
	"io"
	"strings"
	"alertHub/pkg/tools"

	"github.com/bytedance/sonic"
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

// getDingdingTestKeyword gets appropriate keyword for DingDing robot based on notice name
// Uses a keyword mapping table for better maintainability and readability
func getDingdingTestKeyword(noticeName string) string {
	// Default keyword for most common DingDing robot configurations
	defaultKeyword := "告警"

	// Return default keyword if notice name is empty
	if noticeName == "" {
		return defaultKeyword
	}

	// Keyword mapping table - more maintainable than multiple if statements
	// Priority: first match wins, so order matters for overlapping keywords
	keywordMappings := []struct {
		contains string
		keyword  string
	}{
		{"报警", "报警"},     // Alert-related notifications
		{"Alert", "Alert"}, // English alert notifications
		{"监控", "监控"},     // Monitoring notifications
		{"巡检", "巡检"},     // Inspection/health check notifications  
		{"健康", "健康"},     // Health check notifications
		{"报告", "报告"},     // Report notifications
		{"测试", "测试"},     // Test notifications
	}

	// Check each keyword mapping in order
	for _, mapping := range keywordMappings {
		if strings.Contains(noticeName, mapping.contains) {
			return mapping.keyword
		}
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
	cardContentByte := bytes.NewReader([]byte(content))
	res, err := tools.Post(nil, hook, cardContentByte, 10)
	if err != nil {
		return fmt.Errorf("钉钉消息发送失败: %w", err)
	}
	defer res.Body.Close()

	bodyBytes, err := io.ReadAll(res.Body)
	if err != nil {
		return fmt.Errorf("读取钉钉响应失败: %w", err)
	}

	var response DingResponse
	if err := sonic.Unmarshal(bodyBytes, &response); err != nil {
		return fmt.Errorf("解析钉钉响应失败: %w, 响应内容: %s", err, string(bodyBytes))
	}

	if response.Code != 0 {
		return fmt.Errorf("钉钉API返回错误: Code=%d, Msg=%s", response.Code, response.Msg)
	}

	return nil
}
