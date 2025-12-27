package provider

import (
	"fmt"
	"time"
	"alertHub/internal/ctx"
	"github.com/zeromicro/go-zero/core/logc"
)

// Retryer 重试器接口
type Retryer interface {
	// Execute 执行操作，带重试机制
	Execute(operation func() error) error
	// ExecuteWithContext 执行操作，带上下文和重试机制
	ExecuteWithContext(operationName string, operation func() error) error
}

// SmsRetryer SMS重试器实现
type SmsRetryer struct {
	config  *RetryConfig
	metrics *SmsMetricsManager
}

// NewSmsRetryer 创建SMS重试器
func NewSmsRetryer(config *RetryConfig) *SmsRetryer {
	if config == nil {
		config = NewDefaultRetryConfig()
	}
	return &SmsRetryer{
		config:  config,
		metrics: GetSmsMetricsManager(),
	}
}

// Execute 执行操作，带重试机制
func (r *SmsRetryer) Execute(operation func() error) error {
	if !r.config.EnableRetry {
		// 未启用重试，直接执行
		return operation()
	}
	
	var lastErr error
	
	// 第一次尝试
	lastErr = operation()
	if lastErr == nil {
		return nil // 成功，无需重试
	}
	
	// 检查是否可重试
	if !IsRetriableError(lastErr) {
		logc.Info(ctx.Ctx, fmt.Sprintf("错误不可重试，直接失败: %v", lastErr))
		return lastErr
	}
	
	// 开始重试循环
	for attempt := 1; attempt <= r.config.MaxRetries; attempt++ {
		// 计算延迟时间
		delay := r.config.GetDelay(attempt)
		
		logc.Info(ctx.Ctx, fmt.Sprintf("第%d次重试，延迟%v后执行", attempt, delay))
		
		// 等待退避时间
		if delay > 0 {
			time.Sleep(delay)
		}
		
		// 执行重试
		err := operation()
		if err == nil {
			logc.Info(ctx.Ctx, fmt.Sprintf("第%d次重试成功", attempt))
			return nil // 重试成功
		}
		
		lastErr = err
		
		// 检查是否继续重试
		if !r.config.ShouldRetry(attempt+1, err) {
			if attempt >= r.config.MaxRetries {
				logc.Error(ctx.Ctx, fmt.Sprintf("达到最大重试次数%d，最终失败: %v", 
					r.config.MaxRetries, err))
			} else {
				logc.Info(ctx.Ctx, fmt.Sprintf("错误不可重试，停止重试: %v", err))
			}
			break
		}
		
		// 记录重试指标
		r.metrics.RecordRetry("unknown", attempt, err.Error())
		logc.Error(ctx.Ctx, fmt.Sprintf("第%d次重试失败: %v", attempt, err))
	}
	
	return fmt.Errorf("重试%d次后仍然失败，最后错误: %v", r.config.MaxRetries, lastErr)
}

// ExecuteWithContext 执行操作，带上下文和重试机制
func (r *SmsRetryer) ExecuteWithContext(operationName string, operation func() error) error {
	logc.Info(ctx.Ctx, fmt.Sprintf("开始执行操作: %s", operationName))
	
	startTime := time.Now()
	err := r.Execute(operation)
	duration := time.Since(startTime)
	
	if err != nil {
		logc.Error(ctx.Ctx, fmt.Sprintf("操作%s执行失败，耗时%v: %v", 
			operationName, duration, err))
	} else {
		logc.Info(ctx.Ctx, fmt.Sprintf("操作%s执行成功，耗时%v", 
			operationName, duration))
	}
	
	return err
}