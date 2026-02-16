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

	"github.com/zeromicro/go-zero/core/logc"
)

type openaiClient struct {
	config *AiConfig
}

// ChatCompletion 调用 OpenAI API（非流式）
func (c *openaiClient) ChatCompletion(ctx context.Context, prompt string) (string, error) {
	// 构建 OpenAI 请求体
	req := Request{
		Model: c.config.Model,
		Messages: []*Message{
			{Role: "user", Content: prompt},
		},
		Stream:    false,
		MaxTokens: c.config.MaxTokens,
	}

	body, err := json.Marshal(req)
	if err != nil {
		return "", fmt.Errorf("序列化请求体失败: %w", err)
	}

	// 直接使用用户配置的完整 URL（不做拼接）
	// 用户应在系统设置中填写完整的 API 端点地址
	// 例如：https://api.openai.com/v1/chat/completions
	url := c.config.Url

	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("创建请求失败: %w", err)
	}

	httpReq.Header.Set("Authorization", fmt.Sprintf("Bearer %s", c.config.ApiKey))
	httpReq.Header.Set("Content-Type", "application/json")

	// 创建 HTTP 客户端并设置超时
	httpClient := &http.Client{}
	if c.config.Timeout > 0 {
		httpClient.Timeout = time.Duration(c.config.Timeout*3) * time.Second
	}

	resp, err := httpClient.Do(httpReq)
	if err != nil {
		return "", fmt.Errorf("OpenAI API 调用失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		// 尝试解析错误响应
		var errResp struct {
			Error struct {
				Message string `json:"message"`
			} `json:"error"`
		}
		_ = json.NewDecoder(resp.Body).Decode(&errResp)
		if errResp.Error.Message != "" {
			return "", fmt.Errorf("OpenAI API 返回错误 (%d): %s", resp.StatusCode, errResp.Error.Message)
		}
		return "", fmt.Errorf("OpenAI API 返回状态码 %d", resp.StatusCode)
	}

	// 解析响应
	var respData Response
	if err := json.NewDecoder(resp.Body).Decode(&respData); err != nil {
		return "", fmt.Errorf("解析响应失败: %w", err)
	}

	if len(respData.Choices) > 0 {
		return respData.Choices[0].Message.Content, nil
	}

	return "", fmt.Errorf("OpenAI API 返回空内容")
}

// StreamCompletion 调用 OpenAI API（流式）
func (c *openaiClient) StreamCompletion(ctx context.Context, prompt string) (<-chan string, error) {
	// 构建 OpenAI 请求体（流式）
	// 注意：请求体必须在这里构建，不能在 goroutine 内部
	req := Request{
		Model: c.config.Model,
		Messages: []*Message{
			{Role: "user", Content: prompt},
		},
		Stream:    true, // 启用流式
		MaxTokens: c.config.MaxTokens,
	}

	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("序列化请求体失败: %w", err)
	}

	// 直接使用用户配置的完整 URL（不做拼接）
	// 用户应在系统设置中填写完整的 API 端点地址
	// 例如：https://api.openai.com/v1/chat/completions
	url := c.config.Url

	// 使用后台context而不是外部context，避免请求过期
	bgCtx := context.Background()
	httpReq, err := http.NewRequestWithContext(bgCtx, "POST", url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("创建请求失败: %w", err)
	}

	httpReq.Header.Set("Authorization", fmt.Sprintf("Bearer %s", c.config.ApiKey))
	httpReq.Header.Set("Content-Type", "application/json")

	resultChan := make(chan string, 10)

	go func() {
		defer close(resultChan)

		// 创建 HTTP 客户端并设置超时
		httpClient := &http.Client{}
		if c.config.Timeout > 0 {
			httpClient.Timeout = time.Duration(c.config.Timeout*3) * time.Second
		}

		resp, err := httpClient.Do(httpReq)
		if err != nil {
			return
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			var errResp struct {
				Error struct {
					Message string `json:"message"`
				} `json:"error"`
			}
			_ = json.NewDecoder(resp.Body).Decode(&errResp)
			return
		}

		// 处理 SSE 流式响应
		scanner := bufio.NewScanner(resp.Body)
		for scanner.Scan() {
			line := scanner.Text()

			// SSE 格式：data: {...}
			if !strings.HasPrefix(line, "data: ") {
				continue
			}

			jsonData := strings.TrimPrefix(line, "data: ")

			// [DONE] 标志表示流结束
			if jsonData == "[DONE]" {
				break
			}

			// 解析 StreamChunk
			var chunk StreamChunk
			if err := json.Unmarshal([]byte(jsonData), &chunk); err != nil {
				continue
			}

			// 提取内容：choices[0].delta.content
			if len(chunk.Choices) > 0 {
				content := chunk.Choices[0].Delta.Content
				if content != "" {
					select {
					case <-ctx.Done():
						return
					case resultChan <- content:
					}
				}
			}
		}
	}()

	return resultChan, nil
}

// Check 验证 OpenAI 配置是否有效
func (c *openaiClient) Check(ctx context.Context) error {
	if c.config.Url == "" || c.config.ApiKey == "" {
		return fmt.Errorf("OpenAI API 配置错误：URL 和 ApiKey 不能为空")
	}

	if c.config.Model == "" {
		return fmt.Errorf("OpenAI API 配置错误：Model 不能为空")
	}

	if c.config.Timeout == 0 {
		c.config.Timeout = 30 // 默认 30 秒超时
	}

	return nil
}

// ChatCompletionWithSchema 返回 OpenAI 的结构化 JSON 输出
// 兼容 OpenAI 和 OpenAI 兼容 API（如 vLLM、text-generation-inference）
// 先尝试 json_schema strict 模式，失败时回退到 json_object 模式
func (c *openaiClient) ChatCompletionWithSchema(ctx context.Context, prompt string, schema map[string]interface{}) (string, error) {
	// 先尝试 json_schema strict 模式（OpenAI 原生支持）
	response, err := c._chatWithJsonSchema(ctx, prompt, schema)
	if err == nil && response != "" {
		return response, nil
	}

	// json_schema 模式失败，回退到 json_object 模式
	// 非 OpenAI 模型（如 Qwen、LLaMA）可能不支持 json_schema 但支持 json_object
	if err != nil {
		logc.Infof(context.Background(), "json_schema 模式失败 (%v)，回退到 json_object 模式", err)
	}

	return c._chatWithJsonObject(ctx, prompt, schema)
}

// _chatWithJsonSchema 使用 OpenAI json_schema strict 模式
func (c *openaiClient) _chatWithJsonSchema(ctx context.Context, prompt string, schema map[string]interface{}) (string, error) {
	req := map[string]interface{}{
		"model": c.config.Model,
		"messages": []map[string]string{
			{"role": "user", "content": prompt},
		},
		"temperature": 0.2,
		"max_tokens":  c.config.MaxTokens,
		"response_format": map[string]interface{}{
			"type": "json_schema",
			"json_schema": map[string]interface{}{
				"name":   "structured_output",
				"strict": true,
				"schema": schema,
			},
		},
	}

	return c._executeRequest(ctx, req)
}

// _chatWithJsonObject 使用 json_object 模式（更广泛兼容）
// 将 schema 信息嵌入到 prompt 中引导模型输出
func (c *openaiClient) _chatWithJsonObject(ctx context.Context, prompt string, schema map[string]interface{}) (string, error) {
	// 将 schema 嵌入到 prompt 中
	enhancedPrompt := prompt
	if schema != nil {
		schemaJSON, err := json.MarshalIndent(schema, "", "  ")
		if err == nil {
			enhancedPrompt = prompt + "\n\n【JSON Schema 约束】\n请严格按照以下 JSON Schema 格式输出：\n" + string(schemaJSON)
		}
	}

	req := map[string]interface{}{
		"model": c.config.Model,
		"messages": []map[string]string{
			{"role": "user", "content": enhancedPrompt},
		},
		"temperature": 0.2,
		"max_tokens":  c.config.MaxTokens,
		"response_format": map[string]interface{}{
			"type": "json_object",
		},
	}

	return c._executeRequest(ctx, req)
}

// _executeRequest 执行 HTTP 请求并返回 AI 响应内容
func (c *openaiClient) _executeRequest(ctx context.Context, req map[string]interface{}) (string, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return "", fmt.Errorf("序列化请求体失败: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", c.config.Url, bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("创建请求失败: %w", err)
	}

	httpReq.Header.Set("Authorization", fmt.Sprintf("Bearer %s", c.config.ApiKey))
	httpReq.Header.Set("Content-Type", "application/json")

	httpClient := &http.Client{}
	if c.config.Timeout > 0 {
		httpClient.Timeout = time.Duration(c.config.Timeout*3) * time.Second
	}

	resp, err := httpClient.Do(httpReq)
	if err != nil {
		return "", fmt.Errorf("OpenAI API 调用失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		var errResp struct {
			Error struct {
				Message string `json:"message"`
			} `json:"error"`
		}
		_ = json.NewDecoder(resp.Body).Decode(&errResp)
		if errResp.Error.Message != "" {
			return "", fmt.Errorf("OpenAI API 返回错误 (%d): %s", resp.StatusCode, errResp.Error.Message)
		}
		return "", fmt.Errorf("OpenAI API 返回状态码 %d", resp.StatusCode)
	}

	var respData Response
	if err := json.NewDecoder(resp.Body).Decode(&respData); err != nil {
		return "", fmt.Errorf("解析响应失败: %w", err)
	}

	if len(respData.Choices) > 0 {
		return respData.Choices[0].Message.Content, nil
	}

	return "", fmt.Errorf("OpenAI API 返回空内容")
}
