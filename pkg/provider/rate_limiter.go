package provider

import (
	"time"
	"sync"
	"context"
	"alertHub/internal/ctx"
	"github.com/zeromicro/go-zero/core/logc"
)

// RateLimitConfig 速率限制配置
type RateLimitConfig struct {
	// 通用限制
	MaxPerSecond int `json:"maxPerSecond"` // 每秒最大请求数，默认10
	MaxPerMinute int `json:"maxPerMinute"` // 每分钟最大请求数，默认100
	MaxPerHour   int `json:"maxPerHour"`   // 每小时最大请求数，默认1000
	
	// 突发限制
	BurstSize int `json:"burstSize"` // 突发容量，默认20
	
	// 启用开关
	EnableRateLimit bool `json:"enableRateLimit"` // 是否启用速率限制，默认true
}

// NewDefaultRateLimitConfig 创建默认速率限制配置
func NewDefaultRateLimitConfig() *RateLimitConfig {
	return &RateLimitConfig{
		MaxPerSecond:    10,
		MaxPerMinute:    100,
		MaxPerHour:      1000,
		BurstSize:       20,
		EnableRateLimit: true,
	}
}

// RateLimiter 速率限制器
type RateLimiter struct {
	config *RateLimitConfig
	
	// 时间窗口计数器
	secondCounter *WindowCounter
	minuteCounter *WindowCounter
	hourCounter   *WindowCounter
	
	// 令牌桶
	tokenBucket chan struct{}
	lastRefill  time.Time
	mutex       sync.Mutex
}

// WindowCounter 时间窗口计数器
type WindowCounter struct {
	count     int
	window    time.Time
	duration  time.Duration
	mutex     sync.RWMutex
}

// NewWindowCounter 创建时间窗口计数器
func NewWindowCounter(duration time.Duration) *WindowCounter {
	return &WindowCounter{
		count:    0,
		window:   time.Now(),
		duration: duration,
	}
}

// TryIncrement 尝试增加计数，返回是否成功
func (wc *WindowCounter) TryIncrement(maxCount int) bool {
	wc.mutex.Lock()
	defer wc.mutex.Unlock()
	
	now := time.Now()
	// 检查是否需要重置窗口
	if now.Sub(wc.window) >= wc.duration {
		wc.count = 0
		wc.window = now
	}
	
	// 检查是否超过限制
	if wc.count >= maxCount {
		return false
	}
	
	wc.count++
	return true
}

// GetCount 获取当前窗口计数
func (wc *WindowCounter) GetCount() int {
	wc.mutex.RLock()
	defer wc.mutex.RUnlock()
	
	now := time.Now()
	// 检查是否需要重置窗口
	if now.Sub(wc.window) >= wc.duration {
		return 0
	}
	
	return wc.count
}

// NewRateLimiter 创建速率限制器
func NewRateLimiter(config *RateLimitConfig) *RateLimiter {
	if config == nil {
		config = NewDefaultRateLimitConfig()
	}
	
	rl := &RateLimiter{
		config:        config,
		secondCounter: NewWindowCounter(time.Second),
		minuteCounter: NewWindowCounter(time.Minute),
		hourCounter:   NewWindowCounter(time.Hour),
		tokenBucket:   make(chan struct{}, config.BurstSize),
		lastRefill:    time.Now(),
	}
	
	// 初始化令牌桶
	for i := 0; i < config.BurstSize; i++ {
		rl.tokenBucket <- struct{}{}
	}
	
	// 启动令牌补充器
	go rl.refillTokens()
	
	return rl
}

// TryAcquire 尝试获取许可，返回是否成功
func (rl *RateLimiter) TryAcquire(phoneCount int) bool {
	if !rl.config.EnableRateLimit {
		return true // 未启用限制，直接通过
	}
	
	// 检查时间窗口限制
	if !rl.secondCounter.TryIncrement(rl.config.MaxPerSecond) {
		logc.Info(ctx.Ctx, "SMS速率限制: 超过每秒限制", "limit", rl.config.MaxPerSecond)
		return false
	}
	
	if !rl.minuteCounter.TryIncrement(rl.config.MaxPerMinute) {
		logc.Info(ctx.Ctx, "SMS速率限制: 超过每分钟限制", "limit", rl.config.MaxPerMinute)
		return false
	}
	
	if !rl.hourCounter.TryIncrement(rl.config.MaxPerHour) {
		logc.Info(ctx.Ctx, "SMS速率限制: 超过每小时限制", "limit", rl.config.MaxPerHour)
		return false
	}
	
	// 尝试获取令牌
	for i := 0; i < phoneCount; i++ {
		select {
		case <-rl.tokenBucket:
			// 成功获取令牌
		default:
			// 令牌不足，需要等待
			logc.Info(ctx.Ctx, "SMS速率限制: 令牌桶容量不足", "required", phoneCount, "available", len(rl.tokenBucket))
			// 归还已获取的令牌
			for j := 0; j < i; j++ {
				select {
				case rl.tokenBucket <- struct{}{}:
				default:
					// 桶已满，忽略
				}
			}
			return false
		}
	}
	
	return true
}

// AcquireWithWait 获取许可，如果受限则等待
func (rl *RateLimiter) AcquireWithWait(phoneCount int, timeout time.Duration) error {
	if !rl.config.EnableRateLimit {
		return nil // 未启用限制，直接通过
	}
	
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	
	ticker := time.NewTicker(100 * time.Millisecond) // 每100ms检查一次
	defer ticker.Stop()
	
	for {
		if rl.TryAcquire(phoneCount) {
			return nil // 成功获取许可
		}
		
		select {
		case <-ctx.Done():
			return ctx.Err() // 超时
		case <-ticker.C:
			// 继续重试
		}
	}
}

// refillTokens 定期补充令牌
func (rl *RateLimiter) refillTokens() {
	ticker := time.NewTicker(100 * time.Millisecond) // 每100ms补充一次
	defer ticker.Stop()
	
	for range ticker.C {
		rl.mutex.Lock()
		now := time.Now()
		elapsed := now.Sub(rl.lastRefill)
		
		// 计算应该补充的令牌数量
		tokensToAdd := int(elapsed.Seconds() * float64(rl.config.MaxPerSecond) / 10) // 平滑补充
		
		for i := 0; i < tokensToAdd && len(rl.tokenBucket) < rl.config.BurstSize; i++ {
			select {
			case rl.tokenBucket <- struct{}{}:
			default:
				// 桶已满
				break
			}
		}
		
		rl.lastRefill = now
		rl.mutex.Unlock()
	}
}

// GetStats 获取速率限制统计信息
func (rl *RateLimiter) GetStats() map[string]interface{} {
	return map[string]interface{}{
		"secondCount":     rl.secondCounter.GetCount(),
		"minuteCount":     rl.minuteCounter.GetCount(),
		"hourCount":       rl.hourCounter.GetCount(),
		"availableTokens": len(rl.tokenBucket),
		"maxPerSecond":    rl.config.MaxPerSecond,
		"maxPerMinute":    rl.config.MaxPerMinute,
		"maxPerHour":      rl.config.MaxPerHour,
		"burstSize":       rl.config.BurstSize,
	}
}