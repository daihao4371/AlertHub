package services

import (
	"alertHub/internal/ctx"
	"alertHub/internal/types"
	"alertHub/pkg/ai"
	"fmt"
	"strings"
)

type (
	aiService struct {
		ctx *ctx.Context
	}

	InterAiService interface {
		// StreamChat 返回流式数据通道，用于真正的实时流式传输
		StreamChat(req interface{}) (<-chan string, interface{})
	}
)

func newInterAiService(ctx *ctx.Context) InterAiService {
	return &aiService{
		ctx: ctx,
	}
}

// buildPrompt 构建最终的提示词，支持动态字段替换
// 参数说明：
//   template: Prompt 模板字符串
//   r: 请求参数（包含标准字段如 RuleName, Content 等）
//   返回值: 替换后的完整 Prompt
func buildPrompt(template string, r *types.RequestAiChatContent) string {
	prompt := template

	// 第一步：替换标准告警字段
	prompt = strings.ReplaceAll(prompt, "{{ RuleName }}", r.RuleName)
	prompt = strings.ReplaceAll(prompt, "{{ Content }}", r.Content)
	prompt = strings.ReplaceAll(prompt, "{{ SearchQL }}", r.SearchQL)

	// 第二步：替换自定义字段（来自 Extra map）
	// 这样可以支持任意占位符，如 {{ CustomField }}, {{ UserId }}, {{ Language }} 等
	if r.Extra != nil && len(r.Extra) > 0 {
		for key, value := range r.Extra {
			// 构建占位符格式：{{ key }}
			placeholder := fmt.Sprintf("{{ %s }}", key)
			// 将 value 转换为字符串替换
			valueStr := fmt.Sprint(value)
			prompt = strings.ReplaceAll(prompt, placeholder, valueStr)
		}
	}

	// 第三步：深度分析标志处理
	if r.Deep == "true" {
		prompt = fmt.Sprintf("注意, 请深度思考下面的问题!\n%s", prompt)
	}

	return prompt
}

// StreamChat 流式聊天方法 - 返回通道支持实时流式传输
func (a aiService) StreamChat(req interface{}) (<-chan string, interface{}) {
	setting, err := a.ctx.DB.Setting().Get()
	if err != nil {
		return nil, err
	}

	if !setting.AiConfig.GetEnable() {
		return nil, fmt.Errorf("未开启 Ai 分析能力")
	}

	r := req.(*types.RequestAiChatContent)
	err = r.ValidateParams()
	if err != nil {
		return nil, err
	}

	client, err := a.ctx.Redis.ProviderPools().GetClient("AiClient")
	if err != nil {
		return nil, err
	}

	aiClient := client.(ai.AiClient)

	// 构建最终的提示词：优先使用用户提供的自定义 Prompt，否则使用配置中的 Prompt
	var prompt string
	if r.Prompt != "" {
		// 用户运行时提供了自定义 Prompt（支持通用机器人场景）
		prompt = buildPrompt(r.Prompt, r)
	} else {
		// 使用配置中的默认 Prompt（支持现有的告警分析场景）
		prompt = buildPrompt(setting.AiConfig.Prompt, r)
	}

	// 调用流式 API 获取通道
	streamChan, err := aiClient.StreamCompletion(a.ctx.Ctx, prompt)
	if err != nil {
		return nil, err
	}

	return streamChan, nil
}
