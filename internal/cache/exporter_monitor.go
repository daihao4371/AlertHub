package cache

import (
	"context"
	"encoding/json"
	"fmt"
	"time"
	"watchAlert/internal/models"

	"github.com/go-redis/redis"
	"github.com/zeromicro/go-zero/core/logc"
)

type (
	// ExporterMonitorCache 用于管理 Exporter 监控缓存操作
	ExporterMonitorCache struct {
		rc *redis.Client
	}

	// ExporterMonitorCacheInterface 定义了 Exporter 监控缓存的操作接口
	ExporterMonitorCacheInterface interface {
		// SetExporters 设置数据源的 Exporter 状态列表
		SetExporters(datasourceId string, exporters []models.ExporterStatus, ttl time.Duration) error
		// GetExporters 获取数据源的 Exporter 状态列表
		GetExporters(datasourceId string) ([]models.ExporterStatus, error)
		// SetSummary 设置数据源的统计摘要
		SetSummary(datasourceId string, summary models.ExporterStatusSummary, ttl time.Duration) error
		// GetSummary 获取数据源的统计摘要
		GetSummary(datasourceId string) (*models.ExporterStatusSummary, error)
		// DeleteByDatasource 删除指定数据源的所有缓存
		DeleteByDatasource(datasourceId string) error
	}
)

// newExporterMonitorCacheInterface 创建一个新的 ExporterMonitorCache 实例
func newExporterMonitorCacheInterface(r *redis.Client) ExporterMonitorCacheInterface {
	return &ExporterMonitorCache{
		rc: r,
	}
}

// buildExporterListKey 构建 Exporter 列表的 Redis Key
func buildExporterListKey(datasourceId string) string {
	return fmt.Sprintf("exporter:monitor:%s:list", datasourceId)
}

// buildExporterSummaryKey 构建 Exporter 摘要的 Redis Key
func buildExporterSummaryKey(datasourceId string) string {
	return fmt.Sprintf("exporter:monitor:%s:summary", datasourceId)
}

// SetExporters 设置数据源的 Exporter 状态列表
// 使用 JSON 序列化存储,设置过期时间避免脏数据
func (e *ExporterMonitorCache) SetExporters(datasourceId string, exporters []models.ExporterStatus, ttl time.Duration) error {
	key := buildExporterListKey(datasourceId)

	// 序列化为 JSON
	data, err := json.Marshal(exporters)
	if err != nil {
		logc.Errorf(context.Background(), "序列化 Exporter 列表失败: %v", err)
		return err
	}

	// 写入 Redis 并设置过期时间
	err = e.rc.Set(key, data, ttl).Err()
	if err != nil {
		logc.Errorf(context.Background(), "写入 Redis 失败: key=%s, err=%v", key, err)
		return err
	}

	return nil
}

// GetExporters 获取数据源的 Exporter 状态列表
func (e *ExporterMonitorCache) GetExporters(datasourceId string) ([]models.ExporterStatus, error) {
	key := buildExporterListKey(datasourceId)

	// 从 Redis 读取
	data, err := e.rc.Get(key).Result()
	if err != nil {
		if err == redis.Nil {
			// Key 不存在,返回空列表
			return []models.ExporterStatus{}, nil
		}
		logc.Errorf(context.Background(), "从 Redis 读取失败: key=%s, err=%v", key, err)
		return nil, err
	}

	// 反序列化
	var exporters []models.ExporterStatus
	err = json.Unmarshal([]byte(data), &exporters)
	if err != nil {
		logc.Errorf(context.Background(), "反序列化 Exporter 列表失败: %v", err)
		return nil, err
	}

	return exporters, nil
}

// SetSummary 设置数据源的统计摘要
func (e *ExporterMonitorCache) SetSummary(datasourceId string, summary models.ExporterStatusSummary, ttl time.Duration) error {
	key := buildExporterSummaryKey(datasourceId)

	// 序列化为 JSON
	data, err := json.Marshal(summary)
	if err != nil {
		logc.Errorf(context.Background(), "序列化统计摘要失败: %v", err)
		return err
	}

	// 写入 Redis 并设置过期时间
	err = e.rc.Set(key, data, ttl).Err()
	if err != nil {
		logc.Errorf(context.Background(), "写入 Redis 失败: key=%s, err=%v", key, err)
		return err
	}

	return nil
}

// GetSummary 获取数据源的统计摘要
func (e *ExporterMonitorCache) GetSummary(datasourceId string) (*models.ExporterStatusSummary, error) {
	key := buildExporterSummaryKey(datasourceId)

	// 从 Redis 读取
	data, err := e.rc.Get(key).Result()
	if err != nil {
		if err == redis.Nil {
			// Key 不存在,返回 nil
			return nil, nil
		}
		logc.Errorf(context.Background(), "从 Redis 读取失败: key=%s, err=%v", key, err)
		return nil, err
	}

	// 反序列化
	var summary models.ExporterStatusSummary
	err = json.Unmarshal([]byte(data), &summary)
	if err != nil {
		logc.Errorf(context.Background(), "反序列化统计摘要失败: %v", err)
		return nil, err
	}

	return &summary, nil
}

// DeleteByDatasource 删除指定数据源的所有缓存
func (e *ExporterMonitorCache) DeleteByDatasource(datasourceId string) error {
	listKey := buildExporterListKey(datasourceId)
	summaryKey := buildExporterSummaryKey(datasourceId)

	// 批量删除
	err := e.rc.Del(listKey, summaryKey).Err()
	if err != nil {
		logc.Errorf(context.Background(), "删除缓存失败: datasourceId=%s, err=%v", datasourceId, err)
		return err
	}

	return nil
}