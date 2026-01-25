package ai

import (
	"context"
)

type (
	// AiClient is the interface for AI chatbot clients.
	AiClient interface {
		// ChatCompletion returns the completion of the given input text.
		ChatCompletion(context.Context, string) (string, error)
		// StreamCompletion returns a channel that streams the completion of the given input text.
		StreamCompletion(context.Context, string) (<-chan string, error)
		// Check checks the health of the AI chatbot client.
		Check(context.Context) error
	}

	AiConfig struct {
		Url       string
		ApiKey    string
		Model     string
		Timeout   int
		Stream    bool
		MaxTokens int
	}

	// OpenAI 格式请求
	Request struct {
		Model       string     `json:"model"`
		Messages    []*Message `json:"messages"`
		Stream      bool       `json:"stream,omitempty"`
		MaxTokens   int        `json:"max_tokens,omitempty"`
		Temperature float64    `json:"temperature,omitempty"`
	}

	// Dify 格式请求
	DifyRequest struct {
		Inputs         map[string]interface{} `json:"inputs"`
		Query          string                 `json:"query"`
		ResponseMode   string                 `json:"response_mode"`
		ConversationId string                 `json:"conversation_id"`
		User           string                 `json:"user"`
		Files          []interface{}          `json:"files,omitempty"`
		// 最大生成 token 数，确保 Dify 有足够的令牌生成完整回复
		MaxTokens      int                    `json:"max_tokens,omitempty"`
	}

	// Message is a message
	Message struct {
		Role    string `json:"role"` // system/user/assistant
		Content string `json:"content"`
	}

	// StreamChunk 流式响应结构
	StreamChunk struct {
		Choices []struct {
			Delta struct {
				Content string `json:"content"`
			} `json:"delta"`
		} `json:"choices"`
	}

	// Response OpenAI 格式响应结构
	Response struct {
		ID      string `json:"id"`
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
		Error struct {
			Message string `json:"message"`
		} `json:"error"`
	}

	// DifyResponse Dify 格式响应结构
	DifyResponse struct {
		Event          string `json:"event"`
		TaskId         string `json:"task_id"`
		Id             string `json:"id"`
		MessageId      string `json:"message_id"`
		ConversationId string `json:"conversation_id"`
		Mode           string `json:"mode"`
		Answer         string `json:"answer"`
		Metadata       struct {
			Usage struct {
				PromptTokens      int `json:"prompt_tokens"`
				CompletionTokens  int `json:"completion_tokens"`
				TotalTokens       int `json:"total_tokens"`
			} `json:"usage"`
		} `json:"metadata"`
	}
)
