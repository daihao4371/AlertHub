package sender

import (
	"alertHub/pkg/tools"
	"bytes"
	"fmt"
	"io"

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

func NewDingSender() SendInter {
	return &DingDingSender{}
}

func (d *DingDingSender) Send(params SendParams) error {
	return d.post(params.Hook, params.Content)
}

// getDingdingTestKeyword gets appropriate keyword for DingDing robot based on notice name
func getDingdingTestKeyword(noticeName string) string {
	// 统一使用 AlertHub 作为钉钉测试消息的关键词
	return "AlertHub"
}

func (d *DingDingSender) Test(params SendParams) error {
	// 使用统一的测试消息常量，构建钉钉 text 类型消息
	testContent := fmt.Sprintf(`{
		"msgtype": "text",
		"text": {
			"content": "%s"
		}
	}`, RobotTestContent)
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
