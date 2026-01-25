package ai

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// isDifyAPI 检查是否为 Dify API（通过 URL 判断）
func (o *AiConfig) isDifyAPI() bool {
	return strings.Contains(o.Url, "/v1/chat-messages")
}

// NewAiClient 创建AI客户端工厂方法
func NewAiClient(config *AiConfig) (AiClient, error) {
	err := config.Check(context.Background())
	if err != nil {
		return nil, err
	}

	return config, nil
}

// ChatCompletion 调用Dify底层API获取完整分析结果
func (o *AiConfig) ChatCompletion(ctx context.Context, prompt string) (string, error) {
	if !o.isDifyAPI() {
		return "", fmt.Errorf("当前仅支持 Dify API")
	}

	requestBody := map[string]interface{}{
		"inputs":         make(map[string]interface{}),
		"query":          prompt,
		"response_mode":  "streaming",
		"conversation_id": "",
		"user":           "alertHub-system",
		"files":          []interface{}{},
	}

	body, err := json.Marshal(requestBody)
	if err != nil {
		return "", fmt.Errorf("序列化请求体失败: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", o.Url, bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("创建请求失败: %w", err)
	}

	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", o.ApiKey))
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	if o.Timeout > 0 {
		client.Timeout = time.Duration(o.Timeout*3) * time.Second
	}

	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("Dify API 调用失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("Dify API 返回状态码 %d", resp.StatusCode)
	}

	var fullAnswer string
	scanner := bufio.NewScanner(resp.Body)

	for scanner.Scan() {
		line := scanner.Text()

		if !strings.HasPrefix(line, "data: ") {
			continue
		}

		jsonData := strings.TrimPrefix(line, "data: ")

		var streamEvent struct {
			Event  string `json:"event"`
			Answer string `json:"answer"`
			Data   struct {
				Answer string `json:"answer"`
			} `json:"data"`
		}

		if err := json.Unmarshal([]byte(jsonData), &streamEvent); err != nil {
			continue
		}

		// 优先使用 message_end 事件的完整答案
		if streamEvent.Event == "message_end" && streamEvent.Data.Answer != "" {
			return streamEvent.Data.Answer, nil
		}

		// workflow_finished 事件的输出答案
		if streamEvent.Event == "workflow_finished" && streamEvent.Data.Answer != "" {
			return streamEvent.Data.Answer, nil
		}

		// 累积 message 事件的片段
		if streamEvent.Event == "message" && streamEvent.Answer != "" {
			fullAnswer += streamEvent.Answer
		}
	}

	if err := scanner.Err(); err != nil {
		return "", fmt.Errorf("读取响应错误: %w", err)
	}

	return fullAnswer, nil
}

// StreamCompletion 返回Dify流式分析结果通道
func (o *AiConfig) StreamCompletion(ctx context.Context, prompt string) (<-chan string, error) {
	if !o.isDifyAPI() {
		return nil, fmt.Errorf("当前仅支持 Dify API")
	}

	requestBody := map[string]interface{}{
		"inputs":         make(map[string]interface{}),
		"query":          prompt,
		"response_mode":  "streaming",
		"conversation_id": "",
		"user":           "alertHub-system",
		"files":          []interface{}{},
	}

	body, err := json.Marshal(requestBody)
	if err != nil {
		return nil, fmt.Errorf("序列化请求体失败: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", o.Url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("创建请求失败: %w", err)
	}

	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", o.ApiKey))
	req.Header.Set("Content-Type", "application/json")

	resultChan := make(chan string, 10)

	go func() {
		defer close(resultChan)

		client := &http.Client{}
		if o.Timeout > 0 {
			client.Timeout = time.Duration(o.Timeout*3) * time.Second
		}

		resp, err := client.Do(req)
		if err != nil {
			return
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			return
		}

		scanner := bufio.NewScanner(resp.Body)

		for scanner.Scan() {
			line := scanner.Text()

			if !strings.HasPrefix(line, "data: ") {
				continue
			}

			jsonData := strings.TrimPrefix(line, "data: ")

			var streamEvent struct {
				Event  string `json:"event"`
				Answer string `json:"answer"`
			}

			if err := json.Unmarshal([]byte(jsonData), &streamEvent); err != nil {
				continue
			}

			if streamEvent.Event == "message" && streamEvent.Answer != "" {
				select {
				case <-ctx.Done():
					return
				case resultChan <- streamEvent.Answer:
				}
			}
		}
	}()

	return resultChan, nil
}

// Check 验证 AI 配置是否有效
func (o *AiConfig) Check(ctx context.Context) error {
	if o.Url == "" || o.ApiKey == "" {
		return fmt.Errorf("Dify API 配置错误：URL 和 ApiKey 不能为空")
	}

	if o.Timeout == 0 {
		o.Timeout = 30 // 默认 30 秒超时
	}

	return nil
}

// extractHost 从 Dify URL 中提取主机地址
// 例如: http://10.252.10.12/v1/chat-messages -> http://10.252.10.12
func (o *AiConfig) extractHost() string {
	// 查找 /v1 的位置
	parts := strings.Split(o.Url, "/v1")
	if len(parts) > 0 {
		return strings.TrimSuffix(parts[0], "/")
	}
	return ""
}