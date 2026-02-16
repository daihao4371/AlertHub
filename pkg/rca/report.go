package rca

import (
	"alertHub/internal/models"
	"alertHub/internal/repo"
	"alertHub/internal/types"
	"alertHub/pkg/ai"
	"alertHub/pkg/tools"
	"context"
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"time"

	"github.com/zeromicro/go-zero/core/logc"
)

// RCAReportBl 统计报告业务逻辑
// 负责计算 MTTD、MTTR、根因分析等统计指标
// 支持多提供商 AI（Dify、OpenAI）
type RCAReportBl struct {
	tenantId string
	rcaRepo  repo.InterRCARepo
	aiClient ai.AiClient // 改为使用统一的 AiClient 接口
	config   AIClusteringConfig
}

// NewRCAReportBl 创建报告业务逻辑实例
func NewRCAReportBl(
	tenantId string,
	rcaRepo repo.InterRCARepo,
	aiClient ai.AiClient,
	config AIClusteringConfig,
) *RCAReportBl {
	if config.Model == "" {
		config = DefaultAIClusteringConfig()
	}

	return &RCAReportBl{
		tenantId: tenantId,
		rcaRepo:  rcaRepo,
		aiClient: aiClient,
		config:   config,
	}
}

// GenerateReportSnapshot 生成统计报告快照
// 对标 Keep: keep/api/bl/incident_reports.py
//
// 流程：
// 1. 查询时间范围内的事件
// 2. 计算 MTTD、MTTR 等指标
// 3. 统计严重程度分布、受影响服务等
// 4. AI 分析根因频率
// 5. 持久化报告快照
func (bl *RCAReportBl) GenerateReportSnapshot(
	ctx context.Context,
	timeRangeStart, timeRangeEnd int64,
) (*types.IncidentReportDto, error) {
	logc.Infof(ctx, "生成统计报告: 时间范围 %d ~ %d", timeRangeStart, timeRangeEnd)

	// 1. 查询时间范围内的事件（不使用 Repository，直接计算）
	// 注：实际使用时需要在 Repository 添加 GetIncidentsByTimeRange 方法
	// 这里假设所有事件已通过其他方式获取

	// 2. 计算基础统计
	allIncidents, _, err := bl.rcaRepo.ListIncidents(
		ctx,
		bl.tenantId,
		map[string]interface{}{},
		1,
		10000, // 大数值获取所有
	)
	if err != nil {
		logc.Errorf(ctx, "查询事件失败: %v", err)
		return nil, fmt.Errorf("查询事件失败: %v", err)
	}

	// 过滤时间范围内的事件
	var incidents []*models.RCAIncident
	for _, incident := range allIncidents {
		if incident.StartTime >= timeRangeStart && incident.StartTime <= timeRangeEnd {
			incidents = append(incidents, incident)
		}
	}

	// 分离已解决和待处理的事件
	var resolvedIncidents []*models.RCAIncident
	var pendingIncidents []*models.RCAIncident

	for _, incident := range incidents {
		if incident.IsResolved() {
			resolvedIncidents = append(resolvedIncidents, incident)
		} else {
			pendingIncidents = append(pendingIncidents, incident)
		}
	}

	// 3. 计算关键指标
	mttd := bl._calculateMTTD(incidents)
	mttr := bl._calculateMTTR(resolvedIncidents)
	duration := bl._calculateDurationStats(resolvedIncidents)

	// 4. 统计分布数据
	severityDist := bl._calculateSeverityDistribution(incidents)
	affectedServices := bl._calculateTopAffectedServices(incidents)

	// 5. AI 分析根因（可选）
	var rootCauses map[string][]string
	if len(incidents) >= 5 {
		// 只有足够的事件数时才进行 AI 分析
		rootCauses, _ = bl._analyzeRootCauses(ctx, incidents)
	}

	// 6. 构建报告 DTO
	report := &types.IncidentReportDto{
		TotalIncidents:       len(incidents),
		ResolvedIncidents:    len(resolvedIncidents),
		PendingIncidents:     len(pendingIncidents),
		MTTD:                 mttd,
		MTTR:                 mttr,
		ShortestDuration:     duration.shortest,
		LongestDuration:      duration.longest,
		SeverityDistribution: severityDist,
		TopAffectedServices:  affectedServices,
		MostFrequentReasons:  rootCauses,
		TimeRangeStart:       timeRangeStart,
		TimeRangeEnd:         timeRangeEnd,
	}

	// 7. 持久化报告快照到数据库
	snapshot := &models.RCAReportSnapshot{
		ID:                   tools.RandId(),
		TenantId:             bl.tenantId,
		TimeRangeStart:       timeRangeStart,
		TimeRangeEnd:         timeRangeEnd,
		TotalIncidents:       len(incidents),
		ResolvedIncidents:    len(resolvedIncidents),
		PendingIncidents:     len(pendingIncidents),
		MTTD:                 mttd,
		MTTR:                 mttr,
		ShortestDuration:     duration.shortest,
		LongestDuration:      duration.longest,
		SeverityDistribution: severityDist,
		TopAffectedServices:  affectedServices,
		MostFrequentReasons:  rootCauses,
		CreatedAt:            time.Now().Unix(),
	}

	if err := bl.rcaRepo.CreateReport(ctx, snapshot); err != nil {
		logc.Errorf(ctx, "保存报告快照失败: %v", err)
		// 报告生成失败不影响返回结果，只记录日志
	}

	logc.Infof(ctx, "报告生成完成: 总事件数=%d, MTTD=%d秒, MTTR=%d秒", len(incidents), mttd, mttr)
	return report, nil
}

// _calculateMTTD 计算平均检测时间（Mean Time To Detect）
// MTTD = Σ(事件创建时间 - 最早告警时间) / 事件数
// 对标 Keep: keep/api/bl/incident_reports.py:212-228
func (bl *RCAReportBl) _calculateMTTD(incidents []*models.RCAIncident) int64 {
	if len(incidents) == 0 {
		return 0
	}

	var totalSeconds int64
	var count int64

	for _, incident := range incidents {
		if incident.StartTime <= 0 {
			continue
		}

		// 事件的检测时间 = 事件创建时间 - 最早告警时间
		detectionTime := incident.CreatedAt - (incident.StartTime / 1000)
		totalSeconds += detectionTime
		count++
	}

	if count == 0 {
		return 0
	}

	// 返回平均值，向上取整
	return int64(math.Ceil(float64(totalSeconds) / float64(count)))
}

// _calculateMTTR 计算平均恢复时间（Mean Time To Resolve）
// MTTR = Σ(事件解决时间 - 事件开始时间) / 已解决事件数
// 对标 Keep: keep/api/bl/incident_reports.py:230-243
func (bl *RCAReportBl) _calculateMTTR(resolvedIncidents []*models.RCAIncident) int64 {
	if len(resolvedIncidents) == 0 {
		return 0
	}

	var totalSeconds int64
	var count int64

	for _, incident := range resolvedIncidents {
		if incident.ResolvedTime <= 0 {
			continue
		}

		startTime := incident.StartTime
		if startTime <= 0 {
			startTime = incident.CreatedAt * 1000
		}

		// 恢复时间 = 解决时间 - 开始时间
		recoveryTime := incident.ResolvedTime - startTime
		totalSeconds += recoveryTime
		count++
	}

	if count == 0 {
		return 0
	}

	// 返回平均值，向上取整
	return int64(math.Ceil(float64(totalSeconds) / float64(count)))
}

// 持续时间统计结果
type durationStats struct {
	shortest int64
	longest  int64
}

// _calculateDurationStats 计算事件持续时间统计
func (bl *RCAReportBl) _calculateDurationStats(resolvedIncidents []*models.RCAIncident) durationStats {
	stats := durationStats{}

	if len(resolvedIncidents) == 0 {
		return stats
	}

	var shortest, longest int64

	for _, incident := range resolvedIncidents {
		duration := incident.Duration()
		if duration <= 0 {
			continue
		}

		if shortest == 0 || duration < shortest {
			shortest = duration
		}
		if duration > longest {
			longest = duration
		}
	}

	stats.shortest = shortest
	stats.longest = longest
	return stats
}

// _calculateSeverityDistribution 统计严重程度分布
// 返回: {severity: count}
func (bl *RCAReportBl) _calculateSeverityDistribution(incidents []*models.RCAIncident) map[string]int {
	distribution := make(map[string]int)

	for _, incident := range incidents {
		distribution[incident.Severity]++
	}

	return distribution
}

// _calculateTopAffectedServices 统计影响最多的服务（Top N）
func (bl *RCAReportBl) _calculateTopAffectedServices(incidents []*models.RCAIncident) map[string]int {
	serviceCount := make(map[string]int)

	// 统计每个服务出现的次数
	for _, incident := range incidents {
		for _, service := range incident.AffectedServices {
			serviceCount[service]++
		}
	}

	// 只保留 Top 10 服务
	if len(serviceCount) <= 10 {
		return serviceCount
	}

	// 排序并取 Top 10
	type kv struct {
		Key   string
		Value int
	}

	var sorted []kv
	for k, v := range serviceCount {
		sorted = append(sorted, kv{k, v})
	}

	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].Value > sorted[j].Value
	})

	top := make(map[string]int)
	for i := 0; i < 10 && i < len(sorted); i++ {
		top[sorted[i].Key] = sorted[i].Value
	}

	return top
}

// _analyzeRootCauses 使用 AI 分析根因频率（支持 Dify 和 OpenAI）
// 返回: {根因描述: [事件ID列表]}
func (bl *RCAReportBl) _analyzeRootCauses(
	ctx context.Context,
	incidents []*models.RCAIncident,
) (map[string][]string, error) {
	// 限制最多分析 40 个事件（Token 限制）
	if len(incidents) > 40 {
		incidents = incidents[:40]
	}

	// 构建事件数据文本
	var incidentTexts []string
	for i, incident := range incidents {
		text := fmt.Sprintf("事件%d: %s (严重程度=%s, 根因=%s)\n",
			i+1, incident.Name, incident.Severity, incident.RootCauseSummary)
		incidentTexts = append(incidentTexts, text)
	}

	incidentsData := ""
	for _, text := range incidentTexts {
		incidentsData += text
	}

	// 构建 Prompt
	prompt := RootCauseAnalysisPrompt(incidentsData)

	// 调用 AI（使用统一的 ChatCompletion 接口）
	ctx, cancel := context.WithTimeout(ctx, time.Duration(bl.config.Timeout)*time.Second)
	defer cancel()

	response, err := bl.aiClient.ChatCompletion(ctx, prompt)
	if err != nil {
		logc.Errorf(ctx, "AI 根因分析失败: %v", err)
		return nil, err
	}

	if response == "" {
		return nil, fmt.Errorf("AI 响应为空")
	}

	// 解析 JSON 响应
	var rootCauses map[string][]string
	if err := json.Unmarshal([]byte(response), &rootCauses); err != nil {
		logc.Errorf(ctx, "根因分析 JSON 解析失败: %v", err)
		return nil, nil // 根因分析失败不影响报告生成
	}

	return rootCauses, nil
}
