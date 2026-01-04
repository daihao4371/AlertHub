package idutil

import (
	"fmt"
	"time"
	"crypto/rand"
	"encoding/hex"
)

// GeneratePlanId 生成查询计划ID
func GeneratePlanId() string {
	return fmt.Sprintf("plan_%d", time.Now().UnixNano())
}

// GenerateTaskId 生成任务ID
func GenerateTaskId(prefix string) string {
	return fmt.Sprintf("%s_%d", prefix, time.Now().UnixNano())
}

// GenerateCollectionId 生成采集ID
func GenerateCollectionId() string {
	return fmt.Sprintf("collection_%d", time.Now().UnixNano())
}

// GenerateExecutionId 生成执行ID
func GenerateExecutionId() string {
	return fmt.Sprintf("execution_%d", time.Now().UnixNano())
}

// GenerateAnalysisId 生成分析ID
func GenerateAnalysisId() string {
	return fmt.Sprintf("analysis_%d", time.Now().UnixNano())
}

// GenerateUUID 生成简单的UUID（基于时间戳和随机数）
func GenerateUUID() string {
	// 生成16字节的随机数
	randomBytes := make([]byte, 16)
	_, err := rand.Read(randomBytes)
	if err != nil {
		// 如果随机数生成失败，使用时间戳作为后备
		timestamp := time.Now().UnixNano()
		return fmt.Sprintf("uuid_%x", timestamp)
	}
	
	return hex.EncodeToString(randomBytes)
}

// GenerateShortId 生成短ID（8位）
func GenerateShortId() string {
	randomBytes := make([]byte, 4)
	_, err := rand.Read(randomBytes)
	if err != nil {
		// 后备方案：使用时间戳的后8位
		timestamp := time.Now().UnixNano()
		return fmt.Sprintf("%08x", timestamp&0xFFFFFFFF)
	}
	
	return hex.EncodeToString(randomBytes)
}

// GenerateContextId 生成上下文ID
func GenerateContextId(prefix string) string {
	if prefix == "" {
		prefix = "ctx"
	}
	return fmt.Sprintf("%s_%s_%d", prefix, GenerateShortId(), time.Now().Unix())
}