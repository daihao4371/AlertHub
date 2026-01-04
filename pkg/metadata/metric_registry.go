package metadata

import (
	"alertHub/internal/types"
	"fmt"
	"strings"
	"sync"
	"time"
)

// MetricRegistry 指标注册表
// 负责管理和缓存指标元数据信息，提供快速查询能力
type MetricRegistry struct {
	// 指标元数据缓存，按指标名称索引
	metricCache map[string]*types.MetricMetadata
	// 指标类型映射，用于快速类型查询
	typeMapping map[string]string
	// 读写锁，保证并发安全
	mutex sync.RWMutex
	// 缓存过期时间
	cacheExpiry time.Duration
	// 最后更新时间记录
	lastUpdated map[string]time.Time
}

// NewMetricRegistry 创建新的指标注册表实例
func NewMetricRegistry() *MetricRegistry {
	return &MetricRegistry{
		metricCache: make(map[string]*types.MetricMetadata),
		typeMapping: make(map[string]string),
		lastUpdated: make(map[string]time.Time),
		cacheExpiry: 30 * time.Minute, // 默认30分钟过期
	}
}

// RegisterMetric 注册指标元数据
// 将新的指标元数据添加到注册表中
func (mr *MetricRegistry) RegisterMetric(metadata *types.MetricMetadata) error {
	if metadata == nil || metadata.MetricName == "" {
		return fmt.Errorf("无效的指标元数据")
	}

	mr.mutex.Lock()
	defer mr.mutex.Unlock()

	mr.metricCache[metadata.MetricName] = metadata
	mr.typeMapping[metadata.MetricName] = metadata.MetricType
	mr.lastUpdated[metadata.MetricName] = time.Now()

	return nil
}

// GetMetricMetadata 根据指标名称获取元数据
// 从注册表中查询指定指标的完整元数据
func (mr *MetricRegistry) GetMetricMetadata(metricName string) (*types.MetricMetadata, bool) {
	mr.mutex.RLock()
	defer mr.mutex.RUnlock()

	// 检查是否存在且未过期
	if lastUpdate, exists := mr.lastUpdated[metricName]; exists {
		if time.Since(lastUpdate) > mr.cacheExpiry {
			// 数据已过期，异步清理
			go mr.cleanExpiredMetric(metricName)
			return nil, false
		}
	}

	metadata, exists := mr.metricCache[metricName]
	if !exists {
		return nil, false
	}

	// 返回副本，避免外部修改
	return mr.cloneMetadata(metadata), true
}

// GetMetricType 快速获取指标类型
// 提供轻量级的指标类型查询接口
func (mr *MetricRegistry) GetMetricType(metricName string) (string, bool) {
	mr.mutex.RLock()
	defer mr.mutex.RUnlock()

	metricType, exists := mr.typeMapping[metricName]
	return metricType, exists
}

// ListMetricsByType 根据类型列出指标
// 返回指定类型的所有指标名称列表
func (mr *MetricRegistry) ListMetricsByType(metricType string) []string {
	mr.mutex.RLock()
	defer mr.mutex.RUnlock()

	metrics := make([]string, 0)
	for metricName, mType := range mr.typeMapping {
		if mType == metricType {
			metrics = append(metrics, metricName)
		}
	}

	return metrics
}

// SearchMetricsByPattern 根据模式搜索指标
// 支持通配符搜索指标名称
func (mr *MetricRegistry) SearchMetricsByPattern(pattern string) []string {
	mr.mutex.RLock()
	defer mr.mutex.RUnlock()

	metrics := make([]string, 0)
	
	// 简单的通配符匹配实现
	for metricName := range mr.metricCache {
		if mr.matchPattern(metricName, pattern) {
			metrics = append(metrics, metricName)
		}
	}

	return metrics
}

// GetRegistryStats 获取注册表统计信息
// 返回注册表的当前状态统计
func (mr *MetricRegistry) GetRegistryStats() *types.RegistryStats {
	mr.mutex.RLock()
	defer mr.mutex.RUnlock()

	stats := &types.RegistryStats{
		TotalMetrics: len(mr.metricCache),
		TypeCounts:   make(map[string]int),
		LastCleanup:  time.Now(),
	}

	// 统计各类型指标数量
	for _, metricType := range mr.typeMapping {
		stats.TypeCounts[metricType]++
	}

	return stats
}

// CleanExpiredMetrics 清理过期的指标缓存
// 定期清理过期的元数据，释放内存
func (mr *MetricRegistry) CleanExpiredMetrics() int {
	mr.mutex.Lock()
	defer mr.mutex.Unlock()

	cleaned := 0
	now := time.Now()

	for metricName, lastUpdate := range mr.lastUpdated {
		if now.Sub(lastUpdate) > mr.cacheExpiry {
			delete(mr.metricCache, metricName)
			delete(mr.typeMapping, metricName)
			delete(mr.lastUpdated, metricName)
			cleaned++
		}
	}

	return cleaned
}

// UpdateCacheExpiry 更新缓存过期时间
// 动态调整缓存策略
func (mr *MetricRegistry) UpdateCacheExpiry(expiry time.Duration) {
	mr.mutex.Lock()
	defer mr.mutex.Unlock()

	mr.cacheExpiry = expiry
}

// BatchRegister 批量注册指标元数据
// 高效地批量添加多个指标元数据
func (mr *MetricRegistry) BatchRegister(metadataList []*types.MetricMetadata) error {
	mr.mutex.Lock()
	defer mr.mutex.Unlock()

	now := time.Now()
	for _, metadata := range metadataList {
		if metadata == nil || metadata.MetricName == "" {
			continue // 跳过无效的元数据
		}

		mr.metricCache[metadata.MetricName] = metadata
		mr.typeMapping[metadata.MetricName] = metadata.MetricType
		mr.lastUpdated[metadata.MetricName] = now
	}

	return nil
}

// GetMetricsByLabels 根据标签查找相关指标
// 返回包含指定标签的所有指标
func (mr *MetricRegistry) GetMetricsByLabels(labels map[string]string) []string {
	mr.mutex.RLock()
	defer mr.mutex.RUnlock()

	matchingMetrics := make([]string, 0)

	for metricName, metadata := range mr.metricCache {
		if mr.hasMatchingLabels(metadata, labels) {
			matchingMetrics = append(matchingMetrics, metricName)
		}
	}

	return matchingMetrics
}

// ========== 私有辅助方法 ==========

// cleanExpiredMetric 清理单个过期指标（异步调用）
func (mr *MetricRegistry) cleanExpiredMetric(metricName string) {
	mr.mutex.Lock()
	defer mr.mutex.Unlock()

	// 再次检查是否真的过期（避免竞态条件）
	if lastUpdate, exists := mr.lastUpdated[metricName]; exists {
		if time.Since(lastUpdate) > mr.cacheExpiry {
			delete(mr.metricCache, metricName)
			delete(mr.typeMapping, metricName)
			delete(mr.lastUpdated, metricName)
		}
	}
}

// cloneMetadata 克隆元数据对象，避免外部修改
func (mr *MetricRegistry) cloneMetadata(metadata *types.MetricMetadata) *types.MetricMetadata {
	if metadata == nil {
		return nil
	}

	// 创建深拷贝
	clone := &types.MetricMetadata{
		MetricName:   metadata.MetricName,
		MetricType:   metadata.MetricType,
		PromQL:       metadata.PromQL,
		ParsedLabels: make(map[string]string),
		Functions:    make([]string, len(metadata.Functions)),
		Dependencies: make([]string, len(metadata.Dependencies)),
		Metadata:     make(map[string]interface{}),
	}

	// 复制标签
	for k, v := range metadata.ParsedLabels {
		clone.ParsedLabels[k] = v
	}

	// 复制函数列表
	copy(clone.Functions, metadata.Functions)

	// 复制依赖列表
	copy(clone.Dependencies, metadata.Dependencies)

	// 复制元数据
	for k, v := range metadata.Metadata {
		clone.Metadata[k] = v
	}

	// 复制聚合信息
	if metadata.Aggregation != nil {
		clone.Aggregation = &types.AggregationInfo{
			Function: metadata.Aggregation.Function,
			GroupBy:  make([]string, len(metadata.Aggregation.GroupBy)),
			Without:  make([]string, len(metadata.Aggregation.Without)),
		}
		copy(clone.Aggregation.GroupBy, metadata.Aggregation.GroupBy)
		copy(clone.Aggregation.Without, metadata.Aggregation.Without)
	}

	return clone
}

// matchPattern 简单的通配符模式匹配
func (mr *MetricRegistry) matchPattern(text, pattern string) bool {
	// 支持 * 通配符的简单实现
	if pattern == "*" {
		return true
	}

	if !strings.Contains(pattern, "*") {
		return strings.Contains(text, pattern)
	}

	parts := strings.Split(pattern, "*")
	if len(parts) == 2 {
		prefix, suffix := parts[0], parts[1]
		return strings.HasPrefix(text, prefix) && strings.HasSuffix(text, suffix)
	}

	// 更复杂的通配符匹配可以在这里扩展
	return false
}

// hasMatchingLabels 检查元数据是否包含指定的标签
func (mr *MetricRegistry) hasMatchingLabels(metadata *types.MetricMetadata, targetLabels map[string]string) bool {
	if len(targetLabels) == 0 {
		return true // 空标签匹配所有指标
	}

	for key, value := range targetLabels {
		if metaValue, exists := metadata.ParsedLabels[key]; !exists || metaValue != value {
			return false
		}
	}

	return true
}