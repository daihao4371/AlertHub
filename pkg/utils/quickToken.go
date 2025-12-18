package utils

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// QuickActionToken 快捷操作Token载荷
// 注意: 不包含username字段,需要用户登录后才能获取真实操作人
type QuickActionToken struct {
	TenantId    string `json:"tenantId"`    // 租户ID
	Fingerprint string `json:"fingerprint"` // 告警指纹
	ExpireAt    int64  `json:"expireAt"`    // 过期时间戳
}

const (
	// TokenTTL Token默认有效期（24小时）
	TokenTTL = 24 * time.Hour
)

// GenerateQuickToken 生成快捷操作Token
// username参数已废弃,仅为兼容性保留,实际不使用
func GenerateQuickToken(tenantId, fingerprint, username, secretKey string) (string, error) {
	// 构建Token载荷(不包含username,需要用户登录后获取)
	payload := QuickActionToken{
		TenantId:    tenantId,
		Fingerprint: fingerprint,
		ExpireAt:    time.Now().Add(TokenTTL).Unix(),
	}

	// 序列化为JSON
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("序列化Token载荷失败: %w", err)
	}

	// Base64编码载荷
	payloadEncoded := base64.URLEncoding.EncodeToString(payloadBytes)

	// 生成签名
	signature := generateSignature(payloadEncoded, secretKey)

	// 拼接Token: payload.signature
	token := fmt.Sprintf("%s.%s", payloadEncoded, signature)

	return token, nil
}

// VerifyQuickToken 验证快捷操作Token
// 返回Token载荷和错误信息
func VerifyQuickToken(token, secretKey string) (*QuickActionToken, error) {
	// 分割Token
	parts := strings.Split(token, ".")
	if len(parts) != 2 {
		return nil, fmt.Errorf("Token格式错误")
	}

	payloadEncoded := parts[0]
	signature := parts[1]

	// 验证签名
	expectedSignature := generateSignature(payloadEncoded, secretKey)
	if !hmac.Equal([]byte(signature), []byte(expectedSignature)) {
		return nil, fmt.Errorf("Token签名无效")
	}

	// 解码载荷
	payloadBytes, err := base64.URLEncoding.DecodeString(payloadEncoded)
	if err != nil {
		return nil, fmt.Errorf("Token载荷解码失败: %w", err)
	}

	// 反序列化载荷
	var payload QuickActionToken
	if err := json.Unmarshal(payloadBytes, &payload); err != nil {
		return nil, fmt.Errorf("Token载荷解析失败: %w", err)
	}

	// 验证过期时间
	if time.Now().Unix() > payload.ExpireAt {
		return nil, fmt.Errorf("Token已过期")
	}

	return &payload, nil
}

// generateSignature 生成HMAC-SHA256签名
func generateSignature(data, secretKey string) string {
	h := hmac.New(sha256.New, []byte(secretKey))
	h.Write([]byte(data))
	return base64.URLEncoding.EncodeToString(h.Sum(nil))
}