package sender

import (
	"alertHub/pkg/cmdb"
	"alertHub/pkg/tools"
	"bytes"
	"context"
	"fmt"
	"io"
	"strings"

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

func NewDingSender() SendInter {
	return &DingDingSender{}
}

func (d *DingDingSender) Send(params SendParams) error {
	// 检查是否使用企业内部API
	useEnterpriseApi := params.EnterpriseApiConfig != nil && params.EnterpriseApiConfig.EnablePersonalNotification

	var enterpriseApiErr error
	var groupMessageErr error

	// 如果配置了企业内部API，先尝试发送个人消息
	if useEnterpriseApi {
		logc.Infof(context.Background(), "开始发送钉钉个人消息, EventId: %s, 接收者数量: %d", params.EventId, len(params.ReceiverAccounts))
		enterpriseApiErr = d.postToEnterpriseApi(params)
	}

	// 同时发送群消息（标准Webhook），确保消息能够送达
	if params.Hook != "" {
		groupMessageErr = d.post(params.Hook, params.Content)
	}

	// 如果两种方式都失败了，返回错误
	if useEnterpriseApi && enterpriseApiErr != nil && groupMessageErr != nil {
		return fmt.Errorf("个人消息和群消息都发送失败: 个人消息错误=%v, 群消息错误=%v", enterpriseApiErr, groupMessageErr)
	}

	// 如果只有一种方式失败，不返回错误（至少有一种方式成功了）
	if groupMessageErr != nil && !useEnterpriseApi {
		// 如果没有配置企业内部API，群消息失败就是完全失败
		return groupMessageErr
	}

	return nil
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

// isEnterpriseApiURL 判断URL是否为企业内部API
func isEnterpriseApiURL(url string) bool {
	return strings.Contains(url, "open-gateway") && strings.Contains(url, "enterpriseRobot/receiverSingle")
}

// postToEnterpriseApi 通过企业内部API发送消息
func (d *DingDingSender) postToEnterpriseApi(params SendParams) error {
	if params.EnterpriseApiConfig == nil {
		return fmt.Errorf("企业内部API配置为空")
	}

	config := params.EnterpriseApiConfig

	// 1. 获取接收者账号列表（钉钉ID）
	receiverAccounts := d.getReceiverAccounts(params)
	if len(receiverAccounts) == 0 {
		logc.Infof(context.Background(), "钉钉个人消息发送跳过, EventId: %s, 原因: 未找到接收者账号", params.EventId)
		return fmt.Errorf("未找到接收者账号")
	}

	// 2. 转换内容格式（从标准钉钉格式转换为企业内部API格式）
	title, text := d.convertContentFormat(params.Content, params.RuleName)

	// 3. 构建请求体
	payload := cmdb.BuildEnterpriseApiRequestPayload(
		config.SecretKey,
		config.BusinessCode,
		config.RobotCode,
		receiverAccounts,
		title,
		text,
	)

	// 4. 构建认证请求头
	authHeaders, _, err := cmdb.BuildEnterpriseApiAuthHeaders(
		config.ApiUrl,
		config.ClientId,
		config.ClientSecret,
		"", // accessToken为空，会自动获取
	)
	if err != nil {
		return fmt.Errorf("构建认证请求头失败: %w", err)
	}

	// 5. 序列化请求体
	payloadBytes, err := sonic.Marshal(payload)
	if err != nil {
		return fmt.Errorf("序列化请求体失败: %w", err)
	}

	// 6. 构建请求头map
	headers := map[string]string{
		"signature":       authHeaders.Signature,
		"clientId":        authHeaders.ClientId,
		"timestamp":       authHeaders.Timestamp,
		"requestId":       authHeaders.RequestId,
		"signatureMethod": authHeaders.SignatureMethod,
		"accessToken":     authHeaders.AccessToken,
		"Content-Type":    "application/json",
	}

	// 7. 使用配置中的完整URL
	if config.ApiUrl == "" {
		return fmt.Errorf("企业内部API URL为空")
	}

	// 8. 发送POST请求
	bodyReader := bytes.NewReader(payloadBytes)
	res, err := tools.Post(headers, config.ApiUrl, bodyReader, 10)
	if err != nil {
		return fmt.Errorf("企业内部API请求失败: %w", err)
	}
	defer res.Body.Close()

	// 9. 读取响应
	bodyBytes, err := io.ReadAll(res.Body)
	if err != nil {
		return fmt.Errorf("读取响应失败: %w", err)
	}

	// 10. 检查响应状态码
	if res.StatusCode != 200 {
		return fmt.Errorf("企业内部API返回错误状态码: %d, 响应内容: %s", res.StatusCode, string(bodyBytes))
	}

	// 11. 解析响应（如果有JSON格式的响应）
	var apiResp map[string]interface{}
	if err := sonic.Unmarshal(bodyBytes, &apiResp); err == nil {
		// 检查响应中的错误码
		if code, ok := apiResp["code"].(float64); ok && code != 200 {
			msg := ""
			if m, ok := apiResp["msg"].(string); ok {
				msg = m
			}
			return fmt.Errorf("企业内部API返回错误: Code=%.0f, Msg=%s", code, msg)
		}
	}

	// 12. 记录个人消息发送成功日志
	logc.Infof(context.Background(), "钉钉个人消息发送成功, EventId: %s, 接收者数量: %d, 接收者: %v", params.EventId, len(receiverAccounts), receiverAccounts)

	return nil
}

// getReceiverAccounts 获取接收者账号列表（钉钉ID）
// 从SendParams中获取已提取的接收者账号列表
func (d *DingDingSender) getReceiverAccounts(params SendParams) []string {
	return params.ReceiverAccounts
}

// convertContentFormat 将标准钉钉格式转换为企业内部API格式
// 支持多种格式：
// 1. actionCard: {"msgtype":"actionCard","actionCard":{"title":"...","text":"...","btns":[...]}}
// 2. markdown: {"msgtype":"markdown","markdown":{"title":"...","text":"..."}}
// 3. text: {"msgtype":"text","text":{"content":"..."}}
// 企业内部API格式: {title: "...", text: "..."}
// 注意：个人消息只包含告警内容，不包含按钮等交互元素
func (d *DingDingSender) convertContentFormat(standardContent, ruleName string) (title, text string) {
	// 默认标题使用规则名称
	title = ruleName
	text = ""

	// 尝试解析标准格式
	var contentMap map[string]interface{}
	if err := sonic.Unmarshal([]byte(standardContent), &contentMap); err != nil {
		// 如果解析失败，直接使用原始内容作为text
		text = standardContent
		return title, text
	}

	// 检查消息类型
	msgType, _ := contentMap["msgtype"].(string)

	// 1. 处理 actionCard 类型（最常见的告警卡片格式）
	if msgType == "actionCard" {
		if actionCard, ok := contentMap["actionCard"].(map[string]interface{}); ok {
			// 提取标题
			if t, ok := actionCard["title"].(string); ok && t != "" {
				title = t
			}
			// 提取文本内容（忽略按钮 btns）
			if txt, ok := actionCard["text"].(string); ok && txt != "" {
				text = txt
			}
			return title, text
		}
	}

	// 2. 处理 markdown 类型
	if msgType == "markdown" {
		if markdown, ok := contentMap["markdown"].(map[string]interface{}); ok {
			if t, ok := markdown["title"].(string); ok && t != "" {
				title = t
			}
			if txt, ok := markdown["text"].(string); ok && txt != "" {
				text = txt
			}
			return title, text
		}
	}

	// 3. 处理 text 类型
	if msgType == "text" {
		if txt, ok := contentMap["text"].(map[string]interface{}); ok {
			if content, ok := txt["content"].(string); ok {
				text = content
			}
		}
		return title, text
	}

	// 4. 如果无法识别类型，尝试通用提取
	// 先尝试 markdown
	if markdown, ok := contentMap["markdown"].(map[string]interface{}); ok {
		if t, ok := markdown["title"].(string); ok && t != "" {
			title = t
		}
		if txt, ok := markdown["text"].(string); ok && txt != "" {
			text = txt
		}
		return title, text
	}

	// 再尝试 actionCard
	if actionCard, ok := contentMap["actionCard"].(map[string]interface{}); ok {
		if t, ok := actionCard["title"].(string); ok && t != "" {
			title = t
		}
		if txt, ok := actionCard["text"].(string); ok && txt != "" {
			text = txt
		}
		return title, text
	}

	// 最后尝试 text
	if txt, ok := contentMap["text"].(map[string]interface{}); ok {
		if content, ok := txt["content"].(string); ok {
			text = content
		}
		return title, text
	}

	// 如果都无法解析，使用原始内容
	text = standardContent
	return title, text
}
