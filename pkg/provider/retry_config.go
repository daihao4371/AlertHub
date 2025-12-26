package provider

import (
	"math"
	"time"
	"strings"
	"alertHub/pkg/tools"
)

// RetryConfig 重试配置
type RetryConfig struct {
	MaxRetries    int           `json:"maxRetries"`    // 最大重试次数，默认3次
	InitialDelay  time.Duration `json:"initialDelay"`  // 初始延迟，默认1秒
	MaxDelay      time.Duration `json:"maxDelay"`      // 最大延迟，默认30秒
	BackoffFactor float64       `json:"backoffFactor"` // 退避因子，默认2.0
	EnableRetry   bool          `json:"enableRetry"`   // 是否启用重试，默认true
}

// NewDefaultRetryConfig 创建默认重试配置
func NewDefaultRetryConfig() *RetryConfig {
	return &RetryConfig{
		MaxRetries:    3,
		InitialDelay:  1 * time.Second,
		MaxDelay:      30 * time.Second,
		BackoffFactor: 2.0,
		EnableRetry:   true,
	}
}

// GetDelay 计算指定重试次数的延迟时间（指数退避）
func (r *RetryConfig) GetDelay(attempt int) time.Duration {
	if !r.EnableRetry || attempt <= 0 {
		return 0
	}
	
	// 指数退避: initialDelay * (backoffFactor ^ (attempt-1))
	delay := float64(r.InitialDelay) * math.Pow(r.BackoffFactor, float64(attempt-1))
	delayDuration := time.Duration(delay)
	
	// 限制最大延迟
	if delayDuration > r.MaxDelay {
		delayDuration = r.MaxDelay
	}
	
	return delayDuration
}

// ShouldRetry 判断是否应该重试
func (r *RetryConfig) ShouldRetry(attempt int, err error) bool {
	if !r.EnableRetry || attempt > r.MaxRetries {
		return false
	}
	
	// 判断错误是否可重试
	return IsRetriableError(err)
}

// IsRetriableError 判断错误是否可重试
func IsRetriableError(err error) bool {
	if err == nil {
		return false
	}
	
	errMsg := strings.ToLower(err.Error())
	
	// 可重试的错误类型
	retriableErrors := []string{
		"timeout",          // 超时错误
		"connection",       // 连接错误
		"network",          // 网络错误
		"temporary",        // 临时错误
		"rate limit",       // 限流错误
		"throttled",        // 限流错误
		"server error",     // 服务器错误
		"internal error",   // 内部错误
		"503",              // 服务不可用
		"502",              // 网关错误
		"500",              // 服务器内部错误
		"i/o timeout",      // IO超时
		"context deadline", // 上下文超时
	}
	
	// 使用pkg/tools中现有的ContainsAny函数
	return tools.ContainsAny(errMsg, retriableErrors)
}