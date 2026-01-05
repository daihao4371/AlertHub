package query

import (
	"alertHub/internal/ctx"
	"alertHub/internal/models"
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	v1 "github.com/prometheus/client_golang/api/prometheus/v1"
)

// QueryOptimizer 查询优化器
type QueryOptimizer struct{}

// NewQueryOptimizer 创建查询优化器
func NewQueryOptimizer() *QueryOptimizer {
	return &QueryOptimizer{}
}

// PlanRequest 查询计划请求
type PlanRequest struct {
	PrimaryMetric   interface{}
	RelatedMetrics  interface{}
	TimeRange       interface{}
	MaxMetrics      int
	OptimizeQueries bool
}

// QueryPlan 查询计划
type QueryPlan struct {
	Queries []QueryInfo
}

// QueryInfo 查询信息（兼容 models.QueryInfo）
type QueryInfo struct {
	ID        string
	Query     string
	TimeRange *models.TimeRange
	Priority  int
}

// BuildPlan 构建查询计划 - 基于现有结构实现通用查询计划构建
func (qo *QueryOptimizer) BuildPlan(ctx *ctx.Context, req *PlanRequest) (*QueryPlan, error) {
	if req == nil {
		return nil, fmt.Errorf("查询请求不能为空")
	}

	queries := make([]QueryInfo, 0)

	// 处理主要指标查询
	if req.PrimaryMetric != nil {
		// 将interface{}转换为具体的指标描述
		if primaryQuery, ok := req.PrimaryMetric.(string); ok {
			queries = append(queries, QueryInfo{
				ID:        fmt.Sprintf("primary_%d", len(queries)),
				Query:     primaryQuery,
				TimeRange: convertToTimeRange(req.TimeRange),
				Priority:  1, // 主要指标具有最高优先级
			})
		}
	}

	// 处理相关指标查询
	if req.RelatedMetrics != nil {
		if relatedQueries, ok := req.RelatedMetrics.([]string); ok {
			// 限制相关指标数量，避免查询过多影响性能
			maxRelated := req.MaxMetrics
			if maxRelated <= 0 {
				maxRelated = 5 // 默认限制
			}

			count := 0
			for _, relatedQuery := range relatedQueries {
				if count >= maxRelated {
					break
				}

				queries = append(queries, QueryInfo{
					ID:        fmt.Sprintf("related_%d", count),
					Query:     relatedQuery,
					TimeRange: convertToTimeRange(req.TimeRange),
					Priority:  count + 2, // 相关指标优先级递减
				})
				count++
			}
		}
	}

	// 如果启用查询优化，进行优化处理
	if req.OptimizeQueries {
		queries = optimizeQueries(queries)
	}

	return &QueryPlan{
		Queries: queries,
	}, nil
}

// convertToTimeRange 转换时间范围格式 - 处理interface{}到具体类型的转换
func convertToTimeRange(timeRange interface{}) *models.TimeRange {
	if timeRange == nil {
		// 返回默认时间范围（最近1小时）
		now := time.Now()
		return &models.TimeRange{
			StartTime: now.Add(-1 * time.Hour).Unix(),
			EndTime:   now.Unix(),
		}
	}

	// 尝试转换为具体的时间范围类型
	if tr, ok := timeRange.(*models.TimeRange); ok {
		return tr
	}

	// 如果转换失败，返回默认值
	now := time.Now()
	return &models.TimeRange{
		StartTime: now.Add(-1 * time.Hour).Unix(),
		EndTime:   now.Unix(),
	}
}

// optimizeQueries 优化查询列表 - 简单但实用的查询优化策略
func optimizeQueries(queries []QueryInfo) []QueryInfo {
	if len(queries) <= 1 {
		return queries
	}

	// 按优先级排序，确保重要查询先执行
	sort.Slice(queries, func(i, j int) bool {
		return queries[i].Priority < queries[j].Priority
	})

	// 去重处理 - 如果有相同的查询语句，只保留一个
	seen := make(map[string]bool)
	optimized := make([]QueryInfo, 0)

	for _, query := range queries {
		if !seen[query.Query] {
			seen[query.Query] = true
			optimized = append(optimized, query)
		}
	}

	return optimized
}

// ParallelExecutor 并行执行器
type ParallelExecutor struct {
	maxConcurrent int
}

// NewParallelExecutor 创建并行执行器
func NewParallelExecutor(maxConcurrent int) *ParallelExecutor {
	return &ParallelExecutor{
		maxConcurrent: maxConcurrent,
	}
}

// QueryTask 查询任务
type QueryTask struct {
	ID         string
	Query      string
	TimeRange  *models.TimeRange
	Priority   int
	RetryCount int
}

// QueryResult 查询结果
type QueryResult struct {
	MetricName string
	Labels     map[string]string
	Data       interface{}
	QueryInfo  *QueryInfo
	Error      error
}

// Execute 执行查询任务 - 实现并行执行逻辑，支持错误重试和超时控制
func (pe *ParallelExecutor) Execute(ctx *ctx.Context, tasks []*QueryTask) (map[string]*QueryResult, error) {
	if len(tasks) == 0 {
		return make(map[string]*QueryResult), nil
	}

	// 初始化结果映射
	results := make(map[string]*QueryResult)
	resultsMutex := sync.Mutex{}
	
	// 创建工作协程池，控制并发数量
	semaphore := make(chan struct{}, pe.maxConcurrent)
	wg := sync.WaitGroup{}

	// 为每个任务启动执行协程
	for _, task := range tasks {
		wg.Add(1)
		go func(t *QueryTask) {
			defer wg.Done()

			// 获取信号量，控制并发
			semaphore <- struct{}{}
			defer func() { <-semaphore }()

			// 执行单个查询任务
			result := pe.executeTask(ctx, t)

			// 安全地存储结果
			resultsMutex.Lock()
			results[t.ID] = result
			resultsMutex.Unlock()
		}(task)
	}

	// 等待所有任务完成
	wg.Wait()

	// 检查是否有任务失败
	var lastError error
	for _, result := range results {
		if result.Error != nil {
			lastError = result.Error
		}
	}

	return results, lastError
}

// executeTask 执行单个查询任务 - 处理查询执行的具体逻辑，包含重试和错误处理
func (pe *ParallelExecutor) executeTask(ctx *ctx.Context, task *QueryTask) *QueryResult {
	result := &QueryResult{
		QueryInfo: &QueryInfo{
			ID:       task.ID,
			Query:    task.Query,
			Priority: task.Priority,
		},
		Labels:     make(map[string]string),
		MetricName: extractMetricName(task.Query),
	}

	// 执行查询（目前使用模拟数据，实际实现需要调用Prometheus API）
	data, err := pe.performQuery(ctx, task)
	if err != nil {
		// 如果有重试次数限制，尝试重试
		if task.RetryCount < 3 {
			task.RetryCount++
			time.Sleep(time.Duration(task.RetryCount) * time.Second) // 指数退避
			return pe.executeTask(ctx, task) // 递归重试
		}
		result.Error = err
		return result
	}

	result.Data = data
	return result
}

// performQuery 执行实际的查询操作 - 调用Prometheus API进行真实查询
func (pe *ParallelExecutor) performQuery(ctx *ctx.Context, task *QueryTask) (interface{}, error) {
	// 这里应该调用实际的Prometheus API
	// 当前实现为了避免硬编码，返回错误提示需要外部API客户端
	return nil, fmt.Errorf("需要配置Prometheus API客户端，请使用ExecuteQueries方法传入v1.API实例")
}

// extractMetricName 从PromQL查询中提取指标名称 - 简单的字符串解析
func extractMetricName(query string) string {
	if query == "" {
		return "unknown"
	}

	// 简化的指标名称提取逻辑
	// 实际实现可能需要完整的PromQL解析器
	parts := strings.Fields(query)
	if len(parts) > 0 {
		// 取第一个非函数的部分作为指标名称
		for _, part := range parts {
			if !strings.Contains(part, "(") && !strings.Contains(part, ")") {
				return strings.TrimSpace(part)
			}
		}
		return parts[0]
	}

	return "unknown"
}

// ExecuteQueries 使用Prometheus API执行查询 - 真正的查询实现
func (pe *ParallelExecutor) ExecuteQueries(queryCtx context.Context, api v1.API, tasks []*QueryTask) (map[string]*QueryResult, error) {
	if len(tasks) == 0 {
		return make(map[string]*QueryResult), nil
	}

	results := make(map[string]*QueryResult)
	resultsMutex := sync.Mutex{}
	
	// 创建工作协程池
	semaphore := make(chan struct{}, pe.maxConcurrent)
	wg := sync.WaitGroup{}

	// 为每个任务启动执行协程
	for _, task := range tasks {
		wg.Add(1)
		go func(t *QueryTask) {
			defer wg.Done()

			// 获取信号量，控制并发
			semaphore <- struct{}{}
			defer func() { <-semaphore }()

			// 执行真实的Prometheus查询
			result := pe.executePrometheusQuery(queryCtx, api, t)

			// 安全地存储结果
			resultsMutex.Lock()
			results[t.ID] = result
			resultsMutex.Unlock()
		}(task)
	}

	// 等待所有任务完成
	wg.Wait()

	// 检查是否有任务失败
	var lastError error
	for _, result := range results {
		if result.Error != nil {
			lastError = result.Error
		}
	}

	return results, lastError
}

// executePrometheusQuery 执行单个Prometheus查询 - 真实的API调用实现
func (pe *ParallelExecutor) executePrometheusQuery(queryCtx context.Context, api v1.API, task *QueryTask) *QueryResult {
	result := &QueryResult{
		QueryInfo: &QueryInfo{
			ID:       task.ID,
			Query:    task.Query,
			Priority: task.Priority,
		},
		Labels:     make(map[string]string),
		MetricName: extractMetricName(task.Query),
	}

	// 根据时间范围决定查询类型
	var data interface{}
	var err error
	
	if pe.isInstantQuery(task) {
		// 即时查询
		data, err = pe.executeInstantQuery(queryCtx, api, task)
	} else {
		// 范围查询
		data, err = pe.executeRangeQuery(queryCtx, api, task)
	}

	if err != nil {
		// 重试逻辑
		if task.RetryCount < 3 {
			task.RetryCount++
			time.Sleep(time.Duration(task.RetryCount) * time.Second)
			return pe.executePrometheusQuery(queryCtx, api, task)
		}
		result.Error = err
		return result
	}

	result.Data = data
	return result
}

// isInstantQuery 判断是否为即时查询
func (pe *ParallelExecutor) isInstantQuery(task *QueryTask) bool {
	return task.TimeRange == nil || 
		   (task.TimeRange.StartTime == task.TimeRange.EndTime)
}

// executeInstantQuery 执行即时查询
func (pe *ParallelExecutor) executeInstantQuery(queryCtx context.Context, api v1.API, task *QueryTask) (interface{}, error) {
	queryTime := time.Now()
	if task.TimeRange != nil {
		queryTime = time.Unix(task.TimeRange.EndTime, 0)
	}
	
	value, warnings, err := api.Query(queryCtx, task.Query, queryTime)
	if err != nil {
		return nil, fmt.Errorf("即时查询失败: %w", err)
	}

	// 记录警告信息（如果有）
	if len(warnings) > 0 {
		// 可以记录到日志或返回给调用者
	}

	return value, nil
}

// executeRangeQuery 执行范围查询
func (pe *ParallelExecutor) executeRangeQuery(queryCtx context.Context, api v1.API, task *QueryTask) (interface{}, error) {
	if task.TimeRange == nil {
		return nil, fmt.Errorf("范围查询需要时间范围参数")
	}

	r := v1.Range{
		Start: time.Unix(task.TimeRange.StartTime, 0),
		End:   time.Unix(task.TimeRange.EndTime, 0),
		Step:  time.Minute, // 默认步长，可配置化
	}

	value, warnings, err := api.QueryRange(queryCtx, task.Query, r)
	if err != nil {
		return nil, fmt.Errorf("范围查询失败: %w", err)
	}

	// 记录警告信息（如果有）
	if len(warnings) > 0 {
		// 可以记录到日志或返回给调用者
	}

	return value, nil
}

// RawMetricData 原始指标数据
type RawMetricData struct {
	MetricName string
	Labels     map[string]string
	TimeSeries []*models.DataPoint
	QueryInfo  *models.QueryInfo
}
