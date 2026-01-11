package tools

import (
	"strings"
)

// WebhookIdPrefix Webhook ID前缀
const WebhookIdPrefix = "wh_"

// GenerateWebhookId 生成唯一的Webhook ID
// 格式: wh_{随机字符串}
// 示例: wh_c9on6mt3e1fg00crhdag
func GenerateWebhookId() string {
	return WebhookIdPrefix + RandId()
}

// GenerateAlertFingerprint 生成告警指纹（用于告警去重）
// 基于关键字段生成MD5哈希值，确保相同告警能被识别
// 参数：
//   - source: 来源系统（如：zabbix、nagios等）
//   - host: 主机标识
//   - title: 告警标题
//   - additionalFields: 其他需要参与指纹计算的字段（可选）
//
// 返回：32位MD5哈希字符串
func GenerateAlertFingerprint(source, host, title string, additionalFields ...string) string {
	// 构建指纹字符串：source|host|title|field1|field2...
	// 使用管道符分隔，避免字段拼接导致的歧义
	// 例如：source="a", host="b|c" 和 source="a|b", host="c" 会产生不同的指纹
	var parts []string
	parts = append(parts, source, host, title)
	parts = append(parts, additionalFields...)

	// 将所有字段转为小写，避免大小写差异导致重复告警
	for i := range parts {
		parts[i] = strings.ToLower(strings.TrimSpace(parts[i]))
	}

	fingerprintString := strings.Join(parts, "|")
	return Md5Hash([]byte(fingerprintString))
}

// ValidateWebhookId 验证Webhook ID格式是否合法
// 合法的ID应该以"wh_"开头，且总长度在指定范围内
func ValidateWebhookId(webhookId string) bool {
	if !strings.HasPrefix(webhookId, WebhookIdPrefix) {
		return false
	}

	// Webhook ID长度应该在合理范围内
	// xid生成的ID长度为20个字符，加上前缀"wh_"（3个字符）= 23个字符
	// 允许一定的范围：20-40个字符
	idLen := len(webhookId)
	return idLen >= 20 && idLen <= 40
}
