package provider

import (
	"time"
	"sync"
	"alertHub/internal/ctx"
	"github.com/zeromicro/go-zero/core/logc"
)

// SmsMetrics SMS发送指标
type SmsMetrics struct {
	// 基础统计
	TotalSent     int64 `json:"totalSent"`     // 总发送数
	TotalSuccess  int64 `json:"totalSuccess"`  // 成功数
	TotalFailure  int64 `json:"totalFailure"`  // 失败数
	TotalRetries  int64 `json:"totalRetries"`  // 重试次数
	
	// 按服务商统计
	TencentSent   int64 `json:"tencentSent"`   // 腾讯云发送数
	TencentFailed int64 `json:"tencentFailed"` // 腾讯云失败数
	AliyunSent    int64 `json:"aliyunSent"`    // 阿里云发送数
	AliyunFailed  int64 `json:"aliyunFailed"`  // 阿里云失败数
	
	// 时间统计
	LastSentTime     time.Time     `json:"lastSentTime"`     // 最后发送时间
	AverageLatency   time.Duration `json:"averageLatency"`   // 平均延迟
	TotalLatency     time.Duration `json:"-"`                // 总延迟(内部使用)
	SuccessCount     int64         `json:"-"`                // 成功次数计数(内部使用)
	
	mutex sync.RWMutex `json:"-"` // 并发保护
}

// SmsMetricsManager 指标管理器
type SmsMetricsManager struct {
	metrics *SmsMetrics
}

// 全局指标管理器实例
var globalMetricsManager *SmsMetricsManager
var metricsOnce sync.Once

// GetSmsMetricsManager 获取全局指标管理器实例
func GetSmsMetricsManager() *SmsMetricsManager {
	metricsOnce.Do(func() {
		globalMetricsManager = &SmsMetricsManager{
			metrics: &SmsMetrics{
				LastSentTime: time.Now(),
			},
		}
	})
	return globalMetricsManager
}

// RecordSent 记录发送尝试
func (m *SmsMetricsManager) RecordSent(provider string, phoneCount int) {
	m.metrics.mutex.Lock()
	defer m.metrics.mutex.Unlock()
	
	m.metrics.TotalSent += int64(phoneCount)
	m.metrics.LastSentTime = time.Now()
	
	// 按服务商记录
	switch provider {
	case "tencent":
		m.metrics.TencentSent += int64(phoneCount)
	case "aliyun":
		m.metrics.AliyunSent += int64(phoneCount)
	}
	
	logc.Info(ctx.Ctx, "SMS发送指标更新", 
		"provider", provider, 
		"phoneCount", phoneCount, 
		"totalSent", m.metrics.TotalSent)
}

// RecordSuccess 记录发送成功
func (m *SmsMetricsManager) RecordSuccess(provider string, phoneCount int, latency time.Duration) {
	m.metrics.mutex.Lock()
	defer m.metrics.mutex.Unlock()
	
	m.metrics.TotalSuccess += int64(phoneCount)
	
	// 更新平均延迟
	m.metrics.TotalLatency += latency
	m.metrics.SuccessCount++
	m.metrics.AverageLatency = m.metrics.TotalLatency / time.Duration(m.metrics.SuccessCount)
	
	logc.Info(ctx.Ctx, "SMS发送成功",
		"provider", provider,
		"phoneCount", phoneCount,
		"latency", latency,
		"totalSuccess", m.metrics.TotalSuccess)
}

// RecordFailure 记录发送失败
func (m *SmsMetricsManager) RecordFailure(provider string, phoneCount int, errorMsg string) {
	m.metrics.mutex.Lock()
	defer m.metrics.mutex.Unlock()
	
	m.metrics.TotalFailure += int64(phoneCount)
	
	// 按服务商记录失败
	switch provider {
	case "tencent":
		m.metrics.TencentFailed += int64(phoneCount)
	case "aliyun":
		m.metrics.AliyunFailed += int64(phoneCount)
	}
	
	logc.Error(ctx.Ctx, "SMS发送失败",
		"provider", provider,
		"phoneCount", phoneCount,
		"error", errorMsg,
		"totalFailure", m.metrics.TotalFailure)
}

// RecordRetry 记录重试
func (m *SmsMetricsManager) RecordRetry(provider string, attempt int, errorMsg string) {
	m.metrics.mutex.Lock()
	defer m.metrics.mutex.Unlock()
	
	m.metrics.TotalRetries++
	
	logc.Info(ctx.Ctx, "SMS重试记录",
		"provider", provider,
		"attempt", attempt,
		"error", errorMsg,
		"totalRetries", m.metrics.TotalRetries)
}

// GetMetrics 获取当前指标数据
func (m *SmsMetricsManager) GetMetrics() SmsMetrics {
	m.metrics.mutex.RLock()
	defer m.metrics.mutex.RUnlock()
	
	// 创建副本返回，避免外部修改
	return SmsMetrics{
		TotalSent:        m.metrics.TotalSent,
		TotalSuccess:     m.metrics.TotalSuccess,
		TotalFailure:     m.metrics.TotalFailure,
		TotalRetries:     m.metrics.TotalRetries,
		TencentSent:      m.metrics.TencentSent,
		TencentFailed:    m.metrics.TencentFailed,
		AliyunSent:       m.metrics.AliyunSent,
		AliyunFailed:     m.metrics.AliyunFailed,
		LastSentTime:     m.metrics.LastSentTime,
		AverageLatency:   m.metrics.AverageLatency,
	}
}

// GetSuccessRate 计算成功率
func (m *SmsMetricsManager) GetSuccessRate() float64 {
	m.metrics.mutex.RLock()
	defer m.metrics.mutex.RUnlock()
	
	if m.metrics.TotalSent == 0 {
		return 0.0
	}
	
	return float64(m.metrics.TotalSuccess) / float64(m.metrics.TotalSent) * 100.0
}

// GetProviderSuccessRate 获取指定服务商成功率
func (m *SmsMetricsManager) GetProviderSuccessRate(provider string) float64 {
	m.metrics.mutex.RLock()
	defer m.metrics.mutex.RUnlock()
	
	var sent, failed int64
	switch provider {
	case "tencent":
		sent, failed = m.metrics.TencentSent, m.metrics.TencentFailed
	case "aliyun":
		sent, failed = m.metrics.AliyunSent, m.metrics.AliyunFailed
	default:
		return 0.0
	}
	
	if sent == 0 {
		return 0.0
	}
	
	success := sent - failed
	return float64(success) / float64(sent) * 100.0
}

// ResetMetrics 重置指标数据（用于测试或定期清理）
func (m *SmsMetricsManager) ResetMetrics() {
	m.metrics.mutex.Lock()
	defer m.metrics.mutex.Unlock()
	
	*m.metrics = SmsMetrics{
		LastSentTime: time.Now(),
	}
	
	logc.Info(ctx.Ctx, "SMS指标已重置")
}