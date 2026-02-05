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

// NewAiClient 创建 AI 客户端工厂方法
// 根据 config.Provider 返回对应的实现（dify 或 openai）
func NewAiClient(config *AiConfig) (AiClient, error) {
	// 参数校验
	if config.Provider == "" {
		config.Provider = "dify" // 默认 Dify（向后兼容）
	}

	// 根据 Provider 返回对应客户端
	switch config.Provider {
	case "dify":
		client := &difyClient{config: config}
		if err := client.Check(context.Background()); err != nil {
			return nil, err
		}
		return client, nil

	case "openai":
		client := &openaiClient{config: config}
		if err := client.Check(context.Background()); err != nil {
			return nil, err
		}
		return client, nil

	default:
		return nil, fmt.Errorf("不支持的 AI Provider: %s", config.Provider)
	}
}

// ============ Dify 客户端实现 ============

type difyClient struct {
	config *AiConfig
}

// ChatCompletion 调用 Dify 底层 API 获取完整分析结果
func (c *difyClient) ChatCompletion(ctx context.Context, prompt string) (string, error) {
	requestBody := map[string]interface{}{
		"inputs":           make(map[string]interface{}),
		"query":            prompt,
		"response_mode":    "streaming",
		"conversation_id":  "",
		"user":             "alertHub-system",
		"files":            []interface{}{},
	}

	body, err := json.Marshal(requestBody)
	if err != nil {
		return "", fmt.Errorf("序列化请求体失败: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", c.config.Url, bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("创建请求失败: %w", err)
	}

	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", c.config.ApiKey))
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	if c.config.Timeout > 0 {
		client.Timeout = time.Duration(c.config.Timeout*3) * time.Second
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

// StreamCompletion 返回 Dify 流式分析结果通道
func (c *difyClient) StreamCompletion(ctx context.Context, prompt string) (<-chan string, error) {
	resultChan := make(chan string, 10)

	go func() {
		defer close(resultChan)

		// 构建请求体
		// 注意：请求体必须在 goroutine 内部构建，避免 Body 被多次读取
		requestBody := map[string]interface{}{
			"inputs":           make(map[string]interface{}),
			"query":            prompt,
			"response_mode":    "streaming",
			"conversation_id":  "",
			"user":             "alertHub-system",
			"files":            []interface{}{},
		}

		body, err := json.Marshal(requestBody)
		if err != nil {
			return
		}

		req, err := http.NewRequestWithContext(ctx, "POST", c.config.Url, bytes.NewReader(body))
		if err != nil {
			return
		}

		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", c.config.ApiKey))
		req.Header.Set("Content-Type", "application/json")

		client := &http.Client{}
		if c.config.Timeout > 0 {
			client.Timeout = time.Duration(c.config.Timeout*3) * time.Second
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

// Check 验证 Dify 配置是否有效
func (c *difyClient) Check(ctx context.Context) error {
	if c.config.Url == "" || c.config.ApiKey == "" {
		return fmt.Errorf("Dify API 配置错误：URL 和 ApiKey 不能为空")
	}

	if c.config.Timeout == 0 {
		c.config.Timeout = 30 // 默认 30 秒超时
	}

	return nil
}
