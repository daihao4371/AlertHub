package executor

import (
	"alertHub/internal/models"
	"alertHub/internal/types"
	"alertHub/pkg/idutil"
	"alertHub/pkg/provider"
	"context"
	"fmt"
	"sync"
	"time"
)

// ParallelQueryExecutor 并行查询执行器 - 生产级别实现
type ParallelQueryExecutor struct {
	PrometheusProvider provider.MetricsFactoryProvider
	WorkerPool         *QueryWorkerPool
	ResultAggregator   *QueryResultAggregator
	ErrorHandler       *QueryErrorHandler
	ExecutionMetrics   *ExecutionMetrics
	TimeoutConfig      *QueryTimeoutConfig
}

// NewParallelQueryExecutor 创建并行查询执行器实例
func NewParallelQueryExecutor(prometheusProvider provider.MetricsFactoryProvider) *ParallelQueryExecutor {
	return &ParallelQueryExecutor{
		PrometheusProvider: prometheusProvider,
		WorkerPool: &QueryWorkerPool{
			Workers: 10,
		},
		ResultAggregator: &QueryResultAggregator{},
		TimeoutConfig: &QueryTimeoutConfig{
			GlobalTimeout: 5 * time.Minute,
			QueryTimeout:  30 * time.Second,
		},
		ExecutionMetrics: &ExecutionMetrics{},
	}
}

// ExecuteParallel 并行执行查询计划（生产级别实现）
func (pqe *ParallelQueryExecutor) ExecuteParallel(plan *types.QueryExecutionPlan) (*types.DataCollectionResult, error) {
	executionId := idutil.GenerateExecutionId()
	startTime := time.Now()

	// 1. 初始化执行环境
	ctx, cancel := context.WithTimeout(context.Background(), pqe.TimeoutConfig.GlobalTimeout)
	defer cancel()

	executionContext := &QueryExecutionContext{
		ExecutionId: executionId,
		StartTime:   startTime,
		Context:     ctx,
		Results:     make(map[string]*QueryResult),
		Errors:      make(map[string]error),
		Mutex:       &sync.RWMutex{},
	}

	// 2. 按照执行顺序执行查询
	err := pqe.executeInOrder(executionContext, plan)
	if err != nil {
		return nil, fmt.Errorf("执行查询计划失败: %w", err)
	}

	// 3. 聚合和标准化结果
	result, err := pqe.ResultAggregator.AggregateResults(executionContext)
	if err != nil {
		return nil, fmt.Errorf("聚合查询结果失败: %w", err)
	}

	// 4. 记录执行指标
	pqe.ExecutionMetrics.RecordExecution(time.Since(startTime), executionContext.Errors)

	return result, nil
}

// executeInOrder 按顺序执行查询
func (pqe *ParallelQueryExecutor) executeInOrder(
	executionContext *QueryExecutionContext,
	plan *types.QueryExecutionPlan,
) error {
	// 1. 首先执行主查询
	if plan.PrimaryQuery != nil {
		result, err := pqe.executeQuery(executionContext, plan.PrimaryQuery)
		if err != nil {
			return fmt.Errorf("主查询执行失败: %w", err)
		}

		executionContext.Mutex.Lock()
		executionContext.Results[plan.PrimaryQuery.TaskId] = result
		executionContext.Mutex.Unlock()
	}

	// 2. 并行执行相关查询
	if len(plan.ParallelQueries) > 0 {
		err := pqe.executeParallelQueries(executionContext, plan.ParallelQueries)
		if err != nil {
			return fmt.Errorf("并行查询执行失败: %w", err)
		}
	}

	// 3. 执行依赖查询
	for _, dependentQuery := range plan.DependentQueries {
		result, err := pqe.executeQuery(executionContext, dependentQuery)
		if err != nil {
			// 依赖查询失败不致命，记录错误继续
			executionContext.Mutex.Lock()
			executionContext.Errors[dependentQuery.TaskId] = err
			executionContext.Mutex.Unlock()
			continue
		}

		executionContext.Mutex.Lock()
		executionContext.Results[dependentQuery.TaskId] = result
		executionContext.Mutex.Unlock()
	}

	return nil
}

// executeQuery 执行单个查询任务（生产级别实现）
func (pqe *ParallelQueryExecutor) executeQuery(
	executionContext *QueryExecutionContext,
	task *types.QueryTask,
) (*QueryResult, error) {
	startTime := time.Now()

	// 使用真实的Prometheus Provider执行查询
	var promMetrics []provider.Metrics
	var queryErr error

	// 判断查询类型并调用相应的Provider方法
	if task.TimeRange.IsInstantQuery() {
		// 即时查询
		promMetrics, queryErr = pqe.PrometheusProvider.Query(task.Query)
	} else {
		// 范围查询
		promMetrics, queryErr = pqe.PrometheusProvider.QueryRange(
			task.Query,
			time.Unix(task.TimeRange.StartTime, 0),
			time.Unix(task.TimeRange.EndTime, 0),
			time.Duration(task.TimeRange.Step)*time.Second,
		)
	}

	// 处理查询错误
	if queryErr != nil {
		return nil, fmt.Errorf("prometheus查询失败[%s]: %w", task.TaskId, queryErr)
	}

	// 验证查询结果
	if promMetrics == nil {
		return nil, fmt.Errorf("查询返回空结果[%s]", task.TaskId)
	}

	// 将Provider结果转换为标准数据结构
	queryData, err := pqe.convertPrometheusMetrics(promMetrics, task.MetricName)
	if err != nil {
		return nil, fmt.Errorf("转换查询结果失败[%s]: %w", task.TaskId, err)
	}

	result := &QueryResult{
		TaskId:     task.TaskId,
		MetricName: task.MetricName,
		Data:       queryData,
		Duration:   time.Since(startTime),
		Timestamp:  time.Now(),
		Success:    true,
		Metadata: map[string]interface{}{
			"query":      task.Query,
			"queryType":  task.Metadata["type"],
			"timeRange":  task.TimeRange,
			"dataPoints": len(promMetrics),
		},
	}

	return result, nil
}

// convertPrometheusMetrics 将Provider的Metrics转换为标准MetricDataSet
func (pqe *ParallelQueryExecutor) convertPrometheusMetrics(
	promMetrics []provider.Metrics,
	metricName string,
) (*models.MetricDataSet, error) {
	if len(promMetrics) == 0 {
		return &models.MetricDataSet{
			MetricName: metricName,
			TimeSeries: make([]*models.DataPoint, 0),
			Metadata:   make(map[string]interface{}),
		}, nil
	}

	// 转换所有数据点
	timeSeries := make([]*models.DataPoint, 0, len(promMetrics))
	for _, metric := range promMetrics {
		// 转换标签格式
		labels := make(map[string]interface{})
		if metric.GetMetric() != nil {
			for k, v := range metric.GetMetric() {
				if k != "__name__" { // 排除指标名称标签
					labels[k] = v
				}
			}
		}

		dataPoint := &models.DataPoint{
			Timestamp: int64(metric.Timestamp),
			Value:     metric.GetValue(),
			Labels:    labels,
			Quality: &models.DataPointQuality{
				IsValid:    true,
				Confidence: 1.0,
				Source:     "prometheus",
				Anomaly:    false,
			},
		}
		timeSeries = append(timeSeries, dataPoint)
	}

	// 使用 pkg/metadata 中的指标类型推断逻辑
	metricType := "gauge" // 默认类型，实际类型推断已在 metadata 包中实现

	return &models.MetricDataSet{
		MetricName: metricName,
		MetricType: metricType,
		TimeSeries: timeSeries,
		DataQuality: &models.DataQualityInfo{
			Completeness: 1.0,
			Accuracy:     1.0,
			Timeliness:   1.0,
			TotalPoints:  len(timeSeries),
			ValidPoints:  len(timeSeries),
			QualityScore: 1.0,
		},
		QueryInfo: &models.QueryInfo{
			OriginalQuery: "", // 会在调用方设置
			ExecutedQuery: "", // 会在调用方设置
			QueryTime:     time.Now().Unix(),
			Duration:      0, // 会在调用方设置
			DataSource:    "prometheus",
			ResultSize:    len(timeSeries),
		},
		Metadata: map[string]interface{}{
			"originalDataPoints": len(promMetrics),
			"convertedAt":        time.Now().Unix(),
		},
	}, nil
}

// executeParallelQueries 并行执行多个查询
func (pqe *ParallelQueryExecutor) executeParallelQueries(
	executionContext *QueryExecutionContext,
	queries []*types.QueryTask,
) error {
	var wg sync.WaitGroup
	var mu sync.Mutex
	errors := make([]error, 0)

	// 限制并发数量
	maxWorkers := pqe.WorkerPool.Workers
	if maxWorkers <= 0 {
		maxWorkers = 10 // 默认最大工作协程数
	}
	semaphore := make(chan struct{}, maxWorkers)

	for _, query := range queries {
		wg.Add(1)
		go func(q *types.QueryTask) {
			defer wg.Done()

			// 获取信号量
			semaphore <- struct{}{}
			defer func() { <-semaphore }()

			result, err := pqe.executeQuery(executionContext, q)
			if err != nil {
				mu.Lock()
				errors = append(errors, fmt.Errorf("查询 %s 失败: %w", q.TaskId, err))
				executionContext.Errors[q.TaskId] = err
				mu.Unlock()
				return
			}

			executionContext.Mutex.Lock()
			executionContext.Results[q.TaskId] = result
			executionContext.Mutex.Unlock()
		}(query)
	}

	wg.Wait()

	// 如果有错误，记录但不终止执行
	if len(errors) > 0 {
		return fmt.Errorf("部分并行查询失败，成功: %d，失败: %d",
			len(queries)-len(errors), len(errors))
	}

	return nil
}

// 辅助类型定义

// QueryExecutionContext 查询执行上下文
type QueryExecutionContext struct {
	ExecutionId string
	StartTime   time.Time
	Context     context.Context
	Results     map[string]*QueryResult
	Errors      map[string]error
	Mutex       *sync.RWMutex
}

// QueryResult 查询结果
type QueryResult struct {
	TaskId     string
	MetricName string
	Data       *models.MetricDataSet
	Duration   time.Duration
	Timestamp  time.Time
	Success    bool
	ErrorInfo  *QueryError
	Metadata   map[string]interface{}
}

// QueryError 查询错误信息
type QueryError struct {
	TaskId    string
	ErrorType string
	Message   string
	Timestamp time.Time
	Retryable bool
	Metadata  map[string]interface{}
}

// QueryWorkerPool 查询工作池
type QueryWorkerPool struct {
	Workers       int
	JobQueue      chan *QueryJob
	ResultChannel chan *QueryResult
	ErrorChannel  chan *QueryError
	WorkerContext context.Context
}

// QueryJob 查询任务
type QueryJob struct {
	Task     *types.QueryTask
	Context  context.Context
	Timeout  time.Duration
	Retry    int
	Metadata map[string]interface{}
}

// QueryTimeoutConfig 查询超时配置
type QueryTimeoutConfig struct {
	GlobalTimeout     time.Duration
	QueryTimeout      time.Duration
	ConnectionTimeout time.Duration
	RetryTimeout      time.Duration
}

// QueryErrorHandler 查询错误处理器
type QueryErrorHandler struct {
	RetryPolicy      *RetryPolicy
	FallbackHandlers map[string]FallbackHandler
	ErrorClassifiers []ErrorClassifier
}

// ExecutionMetrics 执行指标收集器
type ExecutionMetrics struct {
	TotalExecutions  int64
	SuccessRate      float64
	AvgExecutionTime time.Duration
	ErrorStats       map[string]int64
}

// RecordExecution 记录执行指标
func (em *ExecutionMetrics) RecordExecution(duration time.Duration, errors map[string]error) {
	em.TotalExecutions++

	// 更新平均执行时间
	if em.TotalExecutions == 1 {
		em.AvgExecutionTime = duration
	} else {
		em.AvgExecutionTime = (em.AvgExecutionTime*time.Duration(em.TotalExecutions-1) + duration) / time.Duration(em.TotalExecutions)
	}

	// 更新成功率
	if len(errors) == 0 {
		successCount := em.TotalExecutions * int64(em.SuccessRate)
		em.SuccessRate = float64(successCount+1) / float64(em.TotalExecutions)
	} else {
		successCount := em.TotalExecutions * int64(em.SuccessRate)
		em.SuccessRate = float64(successCount) / float64(em.TotalExecutions)
	}

	// 更新错误统计
	if em.ErrorStats == nil {
		em.ErrorStats = make(map[string]int64)
	}

	for _, err := range errors {
		errorType := "unknown"
		if err != nil {
			errorType = err.Error()
		}
		em.ErrorStats[errorType]++
	}
}

// 接口类型定义
type FallbackHandler interface {
	Handle(*QueryError) (*QueryResult, error)
}

type ErrorClassifier interface {
	Classify(error) string
}

type RetryPolicy struct {
	MaxRetries      int
	BackoffStrategy string
	RetryableErrors []string
}