package sender

import (
	"encoding/json"
	"fmt"
	"alertHub/pkg/cmdb"
	"alertHub/internal/models"
)

type (
	// CmdbSender CMDB告警发送器
	// 继承WebHookSender，专门处理CMDB格式的告警转换和推送
	CmdbSender struct {
		*WebHookSender // 继承WebHookSender的所有功能
	}
)

// NewCmdbSender 创建新的CMDB发送器实例
func NewCmdbSender() SendInter {
	return &CmdbSender{
		WebHookSender: &WebHookSender{},
	}
}

// Send 发送CMDB告警
// 将告警内容转换为CMDB格式后发送
func (c *CmdbSender) Send(params SendParams) error {
	// 转换为CMDB格式
	cmdbContent, err := c.convertToCmdbFormat(params.Content)
	if err != nil {
		return fmt.Errorf("转换CMDB格式失败: %w", err)
	}
	
	// 复用WebHookSender的post方法
	return c.post(params.Hook, cmdbContent)
}

// Test 测试CMDB连接 
// 复用WebHookSender的Test逻辑
func (c *CmdbSender) Test(params SendParams) error {
	return c.WebHookSender.Test(params)
}

// convertToCmdbFormat 将告警内容转换为CMDB格式
// 核心转换逻辑，将AlertHub的告警格式转换为CMDB要求的格式
func (c *CmdbSender) convertToCmdbFormat(content string) (string, error) {
	// 解析告警内容
	var msg map[string]interface{}
	if err := json.Unmarshal([]byte(content), &msg); err != nil {
		return "", fmt.Errorf("解析告警内容失败: %w", err)
	}
	
	// 提取AlertCurEvent数据
	var alert *models.AlertCurEvent
	if alarmData, exists := msg["alarm"]; exists {
		// WebhookContent格式：提取alarm字段
		alertBytes, err := json.Marshal(alarmData)
		if err != nil {
			return "", fmt.Errorf("序列化alarm字段失败: %w", err)
		}
		
		alert = &models.AlertCurEvent{}
		if err := json.Unmarshal(alertBytes, alert); err != nil {
			return "", fmt.Errorf("解析AlertCurEvent失败: %w", err)
		}
	} else {
		// 直接是AlertCurEvent格式
		alert = &models.AlertCurEvent{}
		if err := json.Unmarshal([]byte(content), alert); err != nil {
			return "", fmt.Errorf("无法解析为AlertCurEvent: %w", err)
		}
	}
	
	// 使用转换器转换为CMDB格式
	converter := cmdb.NewConverter()
	cmdbEvent := converter.ConvertToStandard(alert)
	
	// 序列化为JSON
	jsonBytes, err := json.Marshal(cmdbEvent)
	if err != nil {
		return "", fmt.Errorf("序列化CMDB事件失败: %w", err)
	}
	
	return string(jsonBytes), nil
}