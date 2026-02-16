package rca

import (
	"alertHub/internal/models"
	"alertHub/internal/repo"
	"alertHub/internal/types"
	"alertHub/pkg/ai"
	"alertHub/pkg/tools"
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/zeromicro/go-zero/core/logc"
)

// RCASuggestionBl AI 聚类建议业务逻辑类
// 支持多提供商（Dify、OpenAI）的 AI 聚类分析
type RCASuggestionBl struct {
	tenantId string
	rcaRepo  repo.InterRCARepo
	aiClient ai.AiClient // 改为使用 pkg/ai 的统一接口
	config   AIClusteringConfig
}

// truncateString 截断字符串到指定长度，用于日志记录
func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "...(omitted)"
}

// normalizeAlerts 规范化 alerts 字段，处理 alerts/alarms 之间的转换
// JSON 中整数会被 Go 解析为 float64，需要正确处理
func normalizeAlerts(incident map[string]interface{}) []int {
	// 尝试获取 alerts 字段
	if alertsVal, exists := incident["alerts"]; exists {
		if alertsArr, ok := alertsVal.([]interface{}); ok && len(alertsArr) > 0 {
			// 检查是否为对象数组，需要转换为索引
			if _, isObj := alertsArr[0].(map[string]interface{}); isObj {
				indices := make([]int, len(alertsArr))
				for j := range alertsArr {
					indices[j] = j + 1
				}
				return indices
			}
			// 检查是否为数字数组（JSON 整数在 Go 中解析为 float64）
			if _, isNum := alertsArr[0].(float64); isNum {
				indices := make([]int, 0, len(alertsArr))
				for _, v := range alertsArr {
					if num, ok := v.(float64); ok {
						indices = append(indices, int(num))
					}
				}
				return indices
			}
			return []int{}
		}
	}

	// 尝试从 alarms 字段转换
	if alarmsVal, exists := incident["alarms"]; exists {
		if alarmsArr, ok := alarmsVal.([]interface{}); ok && len(alarmsArr) > 0 {
			indices := make([]int, len(alarmsArr))
			for j := range alarmsArr {
				indices[j] = j + 1
			}
			delete(incident, "alarms")
			return indices
		}
	}

	return []int{}
}

// NewRCASuggestionBl 创建 RCASuggestionBl 实例
// 依赖注入：租户ID、Repository、AI 客户端
func NewRCASuggestionBl(
	tenantId string,
	rcaRepo repo.InterRCARepo,
	aiClient ai.AiClient,
	config AIClusteringConfig,
) *RCASuggestionBl {
	if config.Model == "" {
		config = DefaultAIClusteringConfig()
	}

	return &RCASuggestionBl{
		tenantId: tenantId,
		rcaRepo:  rcaRepo,
		aiClient: aiClient,
		config:   config,
	}
}

// mapFieldValue 根据优先级尝试从多个源字段映射值到目标字段
// 如果目标字段已有有效值，则跳过映射；否则按优先级尝试源字段
// sources 参数顺序表示优先级（第一个最高）
func mapFieldValue(incident map[string]interface{}, target string, sources []string, typeCheck func(v interface{}) bool) {
	// 检查目标字段是否已有有效值
	if val, exists := incident[target]; exists && typeCheck(val) {
		return
	}

	// 按优先级尝试源字段
	for _, source := range sources {
		if val, exists := incident[source]; exists && typeCheck(val) {
			incident[target] = val
			delete(incident, source)
			return
		}
	}
}

// deleteFields 删除指定的字段
func deleteFields(incident map[string]interface{}, fields []string) {
	for _, field := range fields {
		delete(incident, field)
	}
}

// SuggestIncidents 主方法：对告警进行 AI 聚类
// 对标 Keep: keep/api/bl/ai_suggestion_bl.py:232-295
// forceRefresh=true 时跳过缓存，重新调用 AI 分析
func (bl *RCASuggestionBl) SuggestIncidents(
	ctx context.Context,
	alerts []models.AlertCurEvent,
	userId string,
	topologyData string, // 可选：拓扑上下文
	forceRefresh bool,
) (*types.ClusteringSuggestion, error) {
	// 验证输入
	if err := bl._validateAlerts(alerts); err != nil {
		return nil, err
	}

	// 计算输入哈希并检查缓存
	inputHash := bl._hashAlertFingerprints(alerts)
	logc.Infof(ctx, "RCA 聚类请求: 租户=%s, 告警数=%d, 强制刷新=%v", bl.tenantId, len(alerts), forceRefresh)

	// 非强制刷新时检查缓存
	if !forceRefresh {
		existingSuggestion, err := bl.rcaRepo.GetSuggestionByHash(ctx, bl.tenantId, inputHash)
		if existingSuggestion != nil {
			logc.Infof(ctx, "命中缓存: 返回已存在的聚类建议 %s", existingSuggestion.ID)
			return bl._suggestionToDto(ctx, existingSuggestion, true), nil
		}
		if err != nil && !strings.Contains(err.Error(), "record not found") {
			logc.Errorf(ctx, "查询缓存时出错: %v", err)
		}
	} else {
		// 强制刷新时删除旧的缓存记录，避免 unique 约束冲突
		bl._deleteExistingSuggestion(ctx, inputHash)
	}

	// 调用 AI 聚类
	response, err := bl._performAIClustering(ctx, alerts, topologyData)
	if err != nil {
		return nil, err
	}

	// 转换数据并持久化
	return bl._saveSuggestionAndIncidents(ctx, inputHash, userId, alerts, response)
}

// _deleteExistingSuggestion 删除旧的缓存建议和关联事件
// 强制刷新时调用，避免 unique 约束冲突
func (bl *RCASuggestionBl) _deleteExistingSuggestion(ctx context.Context, inputHash string) {
	existing, err := bl.rcaRepo.GetSuggestionByHash(ctx, bl.tenantId, inputHash)
	if err != nil || existing == nil {
		return
	}

	logc.Infof(ctx, "强制刷新: 删除旧建议 %s", existing.ID)
	if err := bl.rcaRepo.DeleteSuggestionAndIncidents(ctx, bl.tenantId, existing.ID); err != nil {
		logc.Errorf(ctx, "删除旧建议失败: %v", err)
	}
}

// _validateAlerts 验证告警输入
func (bl *RCASuggestionBl) _validateAlerts(alerts []models.AlertCurEvent) error {
	if len(alerts) == 0 {
		return models.NewRCAError("聚类", "至少需要一条告警数据")
	}
	if len(alerts) > 50 {
		return models.NewRCAError("聚类", "告警数量不能超过 50 条")
	}
	return nil
}

// _performAIClustering 调用 AI 进行聚类分析并解析响应
func (bl *RCASuggestionBl) _performAIClustering(
	ctx context.Context,
	alerts []models.AlertCurEvent,
	topologyData string,
) (string, error) {
	// 构建 Prompt
	alertDescriptions := bl._buildAlertDescriptions(alerts)
	systemPrompt := SystemPrompt
	userPrompt := BuildUserPrompt(alertDescriptions, topologyData)

	// 调用 AI API
	response, err := bl._callAIClusteringAPI(ctx, systemPrompt, userPrompt)
	if err != nil {
		logc.Errorf(ctx, "AI 聚类失败: %v", err)
		return "", models.NewRCAError("聚类", fmt.Sprintf("AI 服务调用失败: %v", err))
	}

	logc.Infof(ctx, "AI 返回成功，响应长度=%d 字符", len(response))
	return response, nil
}

// _saveSuggestionAndIncidents 将建议和事件保存到数据库
func (bl *RCASuggestionBl) _saveSuggestionAndIncidents(
	ctx context.Context,
	inputHash string,
	userId string,
	alerts []models.AlertCurEvent,
	response string,
) (*types.ClusteringSuggestion, error) {
	// 解析和转换 AI 响应
	clusteringResp, err := bl._parseAIResponse(response)
	if err != nil {
		return nil, err
	}

	// 转换为事件模型
	incidents, tokenUsed, err := bl._transformToIncidents(ctx, alerts, clusteringResp)
	if err != nil {
		logc.Errorf(ctx, "数据转换失败: %v", err)
		return nil, err
	}

	// 创建建议记录
	suggestion := &models.RCASuggestion{
		ID:                tools.RandId(),
		TenantId:          bl.tenantId,
		UserId:            userId,
		InputHash:         inputHash,
		AlertCount:        len(alerts),
		AlertFingerprints: bl._serializeFingerprints(alerts),
		Model:             bl.config.Model,
		SuggestionContent: response,
		IncidentCount:     len(incidents),
		TotalTokens:       tokenUsed,
		Status:            "pending",
		CreatedAt:         time.Now().Unix(),
	}

	// 保存建议
	if err := bl.rcaRepo.CreateSuggestion(ctx, suggestion); err != nil {
		logc.Errorf(ctx, "保存聚类建议失败: %v", err)
		return nil, models.NewRCAError("聚类", fmt.Sprintf("保存数据失败: %v", err))
	}

	// 保存事件
	for _, incident := range incidents {
		incident.SuggestionId = suggestion.ID
	}
	if err := bl.rcaRepo.BatchCreateIncidents(ctx, incidents); err != nil {
		logc.Errorf(ctx, "保存聚类事件失败: %v", err)
		return nil, models.NewRCAError("聚类", fmt.Sprintf("保存事件失败: %v", err))
	}

	logc.Infof(ctx, "聚类完成: suggestionId=%s, 事件数=%d", suggestion.ID, len(incidents))

	// 直接从已构建的 incidents 生成 DTO（数据完整，避免重新解析 AI 响应丢失字段）
	return &types.ClusteringSuggestion{
		SuggestionId: suggestion.ID,
		Incidents:    bl._incidentsToDto(incidents),
		FromCache:    false,
		Model:        suggestion.Model,
		TotalTokens:  suggestion.TotalTokens,
		CreatedAt:    suggestion.CreatedAt,
	}, nil
}

// _parseAIResponse 解析 AI 响应的 JSON
func (bl *RCASuggestionBl) _parseAIResponse(response string) (*types.ClusteringResponse, error) {
	// 规范化字段名
	normalizedResponse := bl._normalizeAIResponseJSON(response)

	var clusteringResp types.ClusteringResponse
	if err := json.Unmarshal([]byte(normalizedResponse), &clusteringResp); err != nil {
		logc.Errorf(context.Background(), "直接解析失败: %v", err)
		// 尝试清理响应并重新解析
		cleanedResponse := bl._cleanAIResponse(normalizedResponse)
		if err := json.Unmarshal([]byte(cleanedResponse), &clusteringResp); err != nil {
			logc.Errorf(context.Background(), "清理后仍解析失败: %v", err)
			return nil, models.NewRCAError("聚类", fmt.Sprintf("AI 响应格式错误: %v", err))
		}
	}

	// 适配不同模型格式
	clusteringResp = bl._adaptAIResponse(&clusteringResp, response)
	return &clusteringResp, nil
}

// CommitIncidents 用户确认或拒绝聚类建议
// 对标 Keep 的反馈流程
func (bl *RCASuggestionBl) CommitIncidents(
	ctx context.Context,
	suggestionId string,
	status string, // accepted/rejected
	feedbackScore int,
	feedbackComment string,
) error {
	if status != "accepted" && status != "rejected" {
		return models.NewRCAError("提交反馈", "状态必须为 accepted 或 rejected")
	}

	if feedbackScore < 1 || feedbackScore > 5 {
		return models.NewRCAError("提交反馈", "评分必须在 1-5 之间")
	}

	// 更新建议状态和反馈
	err := bl.rcaRepo.UpdateSuggestionStatus(
		ctx,
		bl.tenantId,
		suggestionId,
		status,
		feedbackScore,
		feedbackComment,
	)

	if err != nil {
		logc.Errorf(ctx, "更新反馈失败: %v", err)
		return models.NewRCAError("提交反馈", fmt.Sprintf("更新失败: %v", err))
	}

	logc.Infof(ctx, "反馈已保存: suggestionId=%s, status=%s, score=%d", suggestionId, status, feedbackScore)
	return nil
}

// _hashAlertFingerprints 计算输入哈希（SHA-256）
// 对标 Keep: keep/api/bl/ai_suggestion_bl.py:67-79
// 用于缓存去重：同租户的同输入只保存一次
func (bl *RCASuggestionBl) _hashAlertFingerprints(alerts []models.AlertCurEvent) string {
	fingerprints := make([]string, len(alerts))
	for i, alert := range alerts {
		fingerprints[i] = alert.Fingerprint
	}

	// 排序后拼接
	sort.Strings(fingerprints)
	data := strings.Join(fingerprints, ",")

	// SHA-256 哈希
	hash := sha256.Sum256([]byte(data))
	return fmt.Sprintf("%x", hash)
}

// _buildAlertDescriptions 构建告警描述文本
// 返回格式化的告警信息，用于 Prompt
// 包含尽可能多的上下文信息（标签、指标值、注释等），帮助 AI 做出准确分析
func (bl *RCASuggestionBl) _buildAlertDescriptions(alerts []models.AlertCurEvent) string {
	var descriptions strings.Builder
	descriptions.WriteString("【告警列表】\n")

	for i, alert := range alerts {
		descriptions.WriteString(fmt.Sprintf("%d. 名称: %s\n", i+1, alert.RuleName))
		descriptions.WriteString(fmt.Sprintf("   严重程度: %s\n", alert.Severity))
		descriptions.WriteString(fmt.Sprintf("   数据源类型: %s\n", alert.DatasourceType))

		// 触发时间
		if alert.FirstTriggerTime > 0 {
			t := time.Unix(alert.FirstTriggerTime, 0)
			descriptions.WriteString(fmt.Sprintf("   首次触发时间: %s\n", t.Format("2006-01-02 15:04:05")))
		}

		// 从标签中提取关键信息（指标值、实例、服务等）
		if alert.Labels != nil {
			for key, value := range alert.Labels {
				descriptions.WriteString(fmt.Sprintf("   标签[%s]: %v\n", key, value))
			}
		}

		// 注释信息（通常包含告警描述和当前值）
		if alert.Annotations != "" {
			descriptions.WriteString(fmt.Sprintf("   注释: %s\n", alert.Annotations))
		}

		// CMDB 富化信息
		if alert.CmdbAppNames != "" {
			descriptions.WriteString(fmt.Sprintf("   应用: %s\n", alert.CmdbAppNames))
		}
		if alert.CmdbOpsOwners != "" {
			descriptions.WriteString(fmt.Sprintf("   运维负责人: %s\n", alert.CmdbOpsOwners))
		}

		// 告警持续时间
		if alert.ForDuration > 0 {
			descriptions.WriteString(fmt.Sprintf("   持续阈值: %d秒\n", alert.ForDuration))
		}

		descriptions.WriteString("\n")
	}

	return descriptions.String()
}

// _callAIClusteringAPI 调用 AI API 进行聚类分析（支持 Dify 和 OpenAI）
// 使用结构化输出（JSON Schema）确保响应符合要求
func (bl *RCASuggestionBl) _callAIClusteringAPI(
	ctx context.Context,
	systemPrompt, userPrompt string,
) (string, error) {
	// 获取聚类结果的 JSON Schema 定义
	schemaMap := GetClusteringResponseSchema()

	// 组合 System Prompt 和 User Prompt
	fullPrompt := systemPrompt + "\n\n" + userPrompt

	// 设置超时
	ctx, cancel := context.WithTimeout(ctx, time.Duration(bl.config.Timeout)*time.Second)
	defer cancel()

	// 调用 AI 客户端的结构化输出方法（支持 Dify 和 OpenAI）
	response, err := bl.aiClient.ChatCompletionWithSchema(ctx, fullPrompt, schemaMap)
	if err != nil {
		return "", fmt.Errorf("AI 聚类 API 调用失败: %w", err)
	}

	if response == "" {
		return "", fmt.Errorf("AI 响应为空")
	}

	return response, nil
}

// _transformToIncidents 将 AI 响应转换为 RCAIncident 模型列表
func (bl *RCASuggestionBl) _transformToIncidents(
	ctx context.Context,
	alerts []models.AlertCurEvent,
	clustering *types.ClusteringResponse,
) ([]*models.RCAIncident, int, error) {
	incidents := make([]*models.RCAIncident, 0, len(clustering.Incidents))

	for _, candidate := range clustering.Incidents {
		incident, err := bl._buildIncidentFromCandidate(ctx, alerts, &candidate)
		if err != nil {
			return nil, 0, err
		}
		incidents = append(incidents, incident)
	}

	return incidents, clustering.TokensUsed, nil
}

// _buildIncidentFromCandidate 从 AI 候选事件构建 RCAIncident
func (bl *RCASuggestionBl) _buildIncidentFromCandidate(
	ctx context.Context,
	alerts []models.AlertCurEvent,
	candidate *types.IncidentCandidate,
) (*models.RCAIncident, error) {
	// 验证和提取告警数据
	alertData, err := bl._extractAlertData(ctx, alerts, candidate.Alerts)
	if err != nil {
		return nil, err
	}

	// 提取受影响的服务和来源
	affectedServices := bl._extractAffectedServices(alerts, candidate.Alerts)
	alertSources := bl._extractAlertSources(alerts, candidate.Alerts)

	return &models.RCAIncident{
		ID:                    tools.RandId(),
		TenantId:              bl.tenantId,
		Name:                  candidate.IncidentName,
		Severity:              candidate.Severity,
		Status:                string(models.RCAStatusCandidate),
		Reasoning:             candidate.Reasoning,
		RootCauseSummary:      candidate.RootCauseSummary,
		ConfidenceScore:       candidate.ConfidenceScore,
		ConfidenceExplanation: candidate.ConfidenceExplanation,
		AlertEventIds:         alertData.EventIds,
		AlertFingerprints:     alertData.Fingerprints,
		AlertCount:            len(alertData.EventIds),
		AlertSources:          alertSources,
		AffectedServices:      affectedServices,
		RecommendedActions:    candidate.RecommendedActions,
		StartTime:             alertData.StartTime,
		LastSeenTime:          alertData.LastSeenTime,
		CreatedAt:             time.Now().Unix(),
	}, nil
}

// alertData 告警数据容器
type alertData struct {
	EventIds     []string
	Fingerprints []string
	StartTime    int64
	LastSeenTime int64
}

// _extractAlertData 从告警索引中提取告警数据并验证
func (bl *RCASuggestionBl) _extractAlertData(
	ctx context.Context,
	alerts []models.AlertCurEvent,
	indices []int,
) (*alertData, error) {
	data := &alertData{
		EventIds:     make([]string, 0),
		Fingerprints: make([]string, 0),
	}

	for _, idx := range indices {
		if idx < 1 || idx > len(alerts) {
			logc.Errorf(ctx, "无效的告警索引: %d（范围: 1-%d）", idx, len(alerts))
			return nil, models.NewRCAError("数据转换", fmt.Sprintf("无效的告警索引: %d（范围: 1-%d）", idx, len(alerts)))
		}

		alert := alerts[idx-1]
		data.EventIds = append(data.EventIds, alert.EventId)
		data.Fingerprints = append(data.Fingerprints, alert.Fingerprint)

		// 计算时间范围
		if data.StartTime == 0 || alert.FirstTriggerTime < data.StartTime {
			data.StartTime = alert.FirstTriggerTime
		}
		if alert.LastEvalTime > data.LastSeenTime {
			data.LastSeenTime = alert.LastEvalTime
		}
	}

	return data, nil
}

// _extractAffectedServices 从告警中提取受影响的服务
// 尝试从多种来源提取：labels 中的 service/job/instance、CMDB 应用名
func (bl *RCASuggestionBl) _extractAffectedServices(
	alerts []models.AlertCurEvent,
	indices []int,
) []string {
	serviceMap := make(map[string]bool)

	for _, idx := range indices {
		alert := alerts[idx-1]

		// 从标签中提取服务相关信息
		if alert.Labels != nil {
			// 常见的服务标识标签
			for _, key := range []string{"service", "job", "app", "application", "namespace"} {
				if val, ok := alert.Labels[key].(string); ok && val != "" {
					serviceMap[val] = true
				}
			}
			// 提取 instance 作为受影响的主机
			if instance, ok := alert.Labels["instance"].(string); ok && instance != "" {
				serviceMap[instance] = true
			}
		}

		// 从 CMDB 应用名中提取
		if alert.CmdbAppNames != "" {
			serviceMap[alert.CmdbAppNames] = true
		}
	}

	services := make([]string, 0, len(serviceMap))
	for service := range serviceMap {
		services = append(services, service)
	}

	sort.Strings(services)
	return services
}

// _extractAlertSources 从告警中提取来源
func (bl *RCASuggestionBl) _extractAlertSources(
	alerts []models.AlertCurEvent,
	indices []int,
) []string {
	sourceMap := make(map[string]bool)

	for _, idx := range indices {
		alert := alerts[idx-1]
		if alert.DatasourceType != "" {
			sourceMap[alert.DatasourceType] = true
		}
	}

	sources := make([]string, 0, len(sourceMap))
	for source := range sourceMap {
		sources = append(sources, source)
	}

	sort.Strings(sources)
	return sources
}

// _serializeFingerprints 序列化告警指纹列表为 JSON 字符串
func (bl *RCASuggestionBl) _serializeFingerprints(alerts []models.AlertCurEvent) string {
	fingerprints := make([]string, len(alerts))
	for i, alert := range alerts {
		fingerprints[i] = alert.Fingerprint
	}

	data, _ := json.Marshal(fingerprints)
	return string(data)
}

// _suggestionToDto 将 RCASuggestion 转换为响应 DTO
// 优先从数据库查询已保存的 incidents（数据完整），回退到解析原始 AI 响应
func (bl *RCASuggestionBl) _suggestionToDto(
	ctx context.Context,
	suggestion *models.RCASuggestion,
	fromCache bool,
) *types.ClusteringSuggestion {
	// 优先从数据库查询已保存的事件（包含完整字段：reasoning、rootCauseSummary、affectedServices 等）
	if bl.rcaRepo != nil {
		savedIncidents, err := bl.rcaRepo.GetIncidentsBySuggestion(ctx, bl.tenantId, suggestion.ID)
		if err == nil && len(savedIncidents) > 0 {
			return &types.ClusteringSuggestion{
				SuggestionId: suggestion.ID,
				Incidents:    bl._incidentsToDto(savedIncidents),
				FromCache:    fromCache,
				Model:        suggestion.Model,
				TotalTokens:  suggestion.TotalTokens,
				CreatedAt:    suggestion.CreatedAt,
			}
		}
	}

	// 回退：从原始 AI 响应解析（数据可能不完整）
	normalizedContent := bl._normalizeAIResponseJSON(suggestion.SuggestionContent)
	var clustering types.ClusteringResponse
	if err := json.Unmarshal([]byte(normalizedContent), &clustering); err != nil {
		cleanedContent := bl._cleanAIResponse(normalizedContent)
		_ = json.Unmarshal([]byte(cleanedContent), &clustering)
	}
	clustering = bl._adaptAIResponse(&clustering, suggestion.SuggestionContent)

	incidents := make([]types.IncidentCandidateDto, len(clustering.Incidents))
	for i, candidate := range clustering.Incidents {
		incidents[i] = types.IncidentCandidateDto{
			Name:                  candidate.IncidentName,
			Severity:              candidate.Severity,
			ConfidenceScore:       candidate.ConfidenceScore,
			ConfidenceExplanation: candidate.ConfidenceExplanation,
			Reasoning:             candidate.Reasoning,
			AlertCount:            len(candidate.Alerts),
			RootCauseSummary:      candidate.RootCauseSummary,
			RecommendedActions:    candidate.RecommendedActions,
		}
	}

	return &types.ClusteringSuggestion{
		SuggestionId: suggestion.ID,
		Incidents:    incidents,
		FromCache:    fromCache,
		Model:        suggestion.Model,
		TotalTokens:  suggestion.TotalTokens,
		CreatedAt:    suggestion.CreatedAt,
	}
}

// _incidentsToDto 将数据库中的 RCAIncident 转换为前端 DTO
// 使用已保存的完整数据，避免重新解析 AI 响应丢失字段
func (bl *RCASuggestionBl) _incidentsToDto(incidents []*models.RCAIncident) []types.IncidentCandidateDto {
	dtos := make([]types.IncidentCandidateDto, len(incidents))
	for i, inc := range incidents {
		dtos[i] = types.IncidentCandidateDto{
			ID:                    inc.ID,
			Name:                  inc.Name,
			Severity:              inc.Severity,
			ConfidenceScore:       inc.ConfidenceScore,
			ConfidenceExplanation: inc.ConfidenceExplanation,
			Reasoning:             inc.Reasoning,
			AlertCount:            inc.AlertCount,
			AffectedServices:      inc.AffectedServices,
			AlertSources:          inc.AlertSources,
			RecommendedActions:    inc.RecommendedActions,
			RootCauseSummary:      inc.RootCauseSummary,
			StartTime:             inc.StartTime,
			LastSeenTime:          inc.LastSeenTime,
			Status:                inc.Status,
		}
	}
	return dtos
}

// _cleanAIResponse 清理 AI 响应文本，处理 Qwen 等模型的格式问题
// Qwen 模型有时会返回包含 markdown 代码块或其他格式的响应
func (bl *RCASuggestionBl) _cleanAIResponse(response string) string {
	// 移除 markdown 代码块标记（```json ... ```）
	if strings.Contains(response, "```json") {
		// 查找首次 ```json 和最后一次 ```
		start := strings.Index(response, "```json")
		end := strings.LastIndex(response, "```")

		if start != -1 && end != -1 && end > start {
			// 提取 json 标记后的内容
			jsonStart := start + len("```json")
			jsonContent := response[jsonStart:end]
			result := strings.TrimSpace(jsonContent)
			logc.Infof(context.Background(), "【_cleanAIResponse】移除 ```json 代码块，清理后长度=%d 字符", len(result))
			return result
		}
	}

	// 如果没有代码块，移除可能的 markdown 格式和空白
	if strings.Contains(response, "```") {
		start := strings.Index(response, "```") + 3
		end := strings.LastIndex(response, "```")
		if end > start {
			result := strings.TrimSpace(response[start:end])
			logc.Infof(context.Background(), "【_cleanAIResponse】移除普通代码块，清理后长度=%d 字符", len(result))
			return result
		}
	}

	// 移除反向单引号（`）通常是 Markdown 的代码格式
	cleaned := strings.ReplaceAll(response, "`", "")

	// 移除其他可能的格式化字符
	cleaned = strings.TrimSpace(cleaned)

	logc.Infof(context.Background(), "【_cleanAIResponse】完成清理，最终长度=%d 字符", len(cleaned))
	return cleaned
}

// _adaptAIResponse 适配不同 AI 模型的响应格式
// 统一处理 Qwen 等模型将 "events" 映射到 "incidents"，并规范化所有字段
// 使用 map 解析替代固定 struct，复用 _normalizeIncidentFields 统一字段规范化逻辑
func (bl *RCASuggestionBl) _adaptAIResponse(resp *types.ClusteringResponse, rawResponse string) types.ClusteringResponse {
	// 如果已经有 incidents，直接返回
	if len(resp.Incidents) > 0 {
		return *resp
	}

	logc.Infof(context.Background(), "【_adaptAIResponse】当前 incidents 为空，尝试解析非标准格式")

	// 使用 map 解析原始响应，避免固定 struct 导致字段丢失
	cleanedResp := bl._cleanAIResponse(rawResponse)
	var rawData map[string]interface{}
	if err := json.Unmarshal([]byte(cleanedResp), &rawData); err != nil {
		logc.Errorf(context.Background(), "【_adaptAIResponse】JSON 解析失败: %v", err)
		return *resp
	}

	// 查找事件数组：优先 incidents，其次 events
	var rawEvents []interface{}
	if arr, ok := rawData["incidents"].([]interface{}); ok && len(arr) > 0 {
		rawEvents = arr
	} else if arr, ok := rawData["events"].([]interface{}); ok && len(arr) > 0 {
		rawEvents = arr
	}

	if len(rawEvents) == 0 {
		logc.Infof(context.Background(), "【_adaptAIResponse】未找到 incidents 或 events 数组")
		return *resp
	}

	logc.Infof(context.Background(), "【_adaptAIResponse】成功识别 %d 个事件，开始字段规范化", len(rawEvents))

	// 将每个事件通过统一的字段规范化逻辑处理
	incidents := make([]types.IncidentCandidate, 0, len(rawEvents))
	for i, eventRaw := range rawEvents {
		eventMap, ok := eventRaw.(map[string]interface{})
		if !ok {
			continue
		}

		// 处理 root_cause_analysis 数组 → root_cause_summary 字符串
		bl._normalizeRootCauseField(eventMap)

		// 复用统一的字段规范化逻辑
		bl._normalizeIncidentFields(eventMap)

		// 从规范化后的 map 构建 IncidentCandidate
		incident := bl._mapToIncidentCandidate(eventMap)
		incidents = append(incidents, incident)

		logc.Infof(context.Background(), "【_adaptAIResponse】事件 %d: 名称=%s, reasoning长度=%d, rootCause长度=%d",
			i+1, incident.IncidentName, len(incident.Reasoning), len(incident.RootCauseSummary))
	}

	resp.Incidents = incidents
	logc.Infof(context.Background(), "【_adaptAIResponse】完成映射，最终事件数=%d", len(resp.Incidents))
	return *resp
}

// _normalizeRootCauseField 将 root_cause_analysis 数组类型转换为字符串
// Qwen 等模型可能返回数组格式的根因分析
func (bl *RCASuggestionBl) _normalizeRootCauseField(eventMap map[string]interface{}) {
	// 检查并转换 root_cause_analysis 数组
	for _, key := range []string{"root_cause_analysis", "root_causes", "rootCauseAnalysis"} {
		if val, exists := eventMap[key]; exists {
			if arr, ok := val.([]interface{}); ok && len(arr) > 0 {
				parts := make([]string, 0, len(arr))
				for _, item := range arr {
					if s, ok := item.(string); ok && s != "" {
						parts = append(parts, s)
					}
				}
				if len(parts) > 0 {
					// 设置为 root_cause_summary 供后续规范化使用
					if _, hasSummary := eventMap["root_cause_summary"]; !hasSummary {
						eventMap["root_cause_summary"] = strings.Join(parts, "; ")
					}
				}
			}
			delete(eventMap, key)
		}
	}
}

// _mapToIncidentCandidate 从规范化后的 map 构建 IncidentCandidate
func (bl *RCASuggestionBl) _mapToIncidentCandidate(m map[string]interface{}) types.IncidentCandidate {
	candidate := types.IncidentCandidate{}

	if v, ok := m["incident_name"].(string); ok {
		candidate.IncidentName = v
	}
	if v, ok := m["reasoning"].(string); ok {
		candidate.Reasoning = v
	}
	if v, ok := m["severity"].(string); ok {
		candidate.Severity = v
	}
	if v, ok := m["confidence_score"].(float64); ok {
		candidate.ConfidenceScore = v
	}
	if v, ok := m["confidence_explanation"].(string); ok {
		candidate.ConfidenceExplanation = v
	}
	if v, ok := m["root_cause_summary"].(string); ok {
		candidate.RootCauseSummary = v
	}

	// 提取 recommended_actions 字符串数组
	if v, ok := m["recommended_actions"].([]interface{}); ok {
		actions := make([]string, 0, len(v))
		for _, item := range v {
			if s, ok := item.(string); ok {
				actions = append(actions, s)
			}
		}
		candidate.RecommendedActions = actions
	}

	// 提取 alerts 索引数组
	candidate.Alerts = normalizeAlerts(m)

	return candidate
}

// _normalizeAIResponseJSON 在反序列化前将 AI 返回的 JSON 字段名标准化
// 处理不同 AI 模型的字段名差异（如 Qwen: event_id/title→incident_name, confidence→confidence_score）
// 参考 Keep 项目的做法，采用优先级映射策略
func (bl *RCASuggestionBl) _normalizeAIResponseJSON(rawJSON string) string {
	var raw interface{}
	if err := json.Unmarshal([]byte(rawJSON), &raw); err != nil {
		logc.Errorf(context.Background(), "【_normalizeAIResponseJSON】JSON 解析失败，返回原始内容: %v", err)
		return rawJSON
	}

	data, ok := raw.(map[string]interface{})
	if !ok {
		return rawJSON
	}

	incidents, ok := data["incidents"].([]interface{})
	if !ok {
		return rawJSON
	}

	// 逐个规范化事件字段
	for i, incidentRaw := range incidents {
		incident, ok := incidentRaw.(map[string]interface{})
		if !ok {
			continue
		}

		// 规范化所有字段名
		bl._normalizeIncidentFields(incident)

		incidents[i] = incident
	}

	data["incidents"] = incidents
	normalized, _ := json.Marshal(data)
	return string(normalized)
}

// _normalizeIncidentFields 规范化单个事件的所有字段
// 将各种可能的字段名映射到标准字段名
func (bl *RCASuggestionBl) _normalizeIncidentFields(incident map[string]interface{}) {
	// 1. 规范化 incident_name（优先级：incident_name > title > name > event_id > event_name）
	mapFieldValue(incident, "incident_name", []string{"title", "name", "event_id", "event_name"}, func(v interface{}) bool {
		s, ok := v.(string)
		return ok && s != ""
	})

	// 2. 规范化 confidence_score（优先级：confidence_score > confidence）
	mapFieldValue(incident, "confidence_score", []string{"confidence"}, func(v interface{}) bool {
		f, ok := v.(float64)
		return ok && f > 0
	})

	// 3. 规范化 root_cause_summary（先于 reasoning，因为 reasoning 可能需要从 root_cause_summary 回退）
	// Qwen 常返回 potential_root_cause 而非 root_cause_summary
	mapFieldValue(incident, "root_cause_summary", []string{
		"root_cause", "potential_root_cause", "cause_summary",
		"rootCause", "rootCauseSummary", "root_cause_analysis",
	}, func(v interface{}) bool {
		s, ok := v.(string)
		return ok && s != ""
	})

	// 4. 规范化 reasoning
	// Qwen 可能不返回 reasoning 字段，从其他语义相近的字段补充
	mapFieldValue(incident, "reasoning", []string{
		"analysis", "description", "rationale", "explanation",
		"reason", "impact_analysis",
	}, func(v interface{}) bool {
		s, ok := v.(string)
		return ok && s != ""
	})

	// 5. reasoning 仍为空时，从 root_cause_summary 或 confidence_explanation 生成
	if v, ok := incident["reasoning"].(string); !ok || v == "" {
		if rootCause, ok := incident["root_cause_summary"].(string); ok && rootCause != "" {
			incident["reasoning"] = rootCause
		} else if confExp, ok := incident["confidence_explanation"].(string); ok && confExp != "" {
			incident["reasoning"] = confExp
		}
	}

	// 6. 规范化 recommended_actions
	mapFieldValue(incident, "recommended_actions", []string{"recommendations", "actions", "suggested_actions"}, func(v interface{}) bool {
		arr, ok := v.([]interface{})
		return ok && len(arr) > 0
	})

	// 7. 规范化 confidence_explanation
	mapFieldValue(incident, "confidence_explanation", []string{"confidence_reasoning", "score_explanation"}, func(v interface{}) bool {
		s, ok := v.(string)
		return ok && s != ""
	})

	// 8. 规范化 alerts（处理 related_alerts、对象数组到索引数组的转换）
	// Qwen 常返回 related_alerts 而非 alerts
	if _, hasAlerts := incident["alerts"]; !hasAlerts {
		for _, altKey := range []string{"related_alerts", "alert_list", "associated_alerts"} {
			if val, exists := incident[altKey]; exists {
				incident["alerts"] = val
				delete(incident, altKey)
				break
			}
		}
	}
	incident["alerts"] = normalizeAlerts(incident)

	// 清理其他不标准的字段
	deleteFields(incident, []string{
		"event_id", "title", "name", "event_name", "confidence",
		"analysis", "description", "rationale", "explanation", "reason",
		"root_cause", "potential_root_cause", "cause_summary", "rootCause", "rootCauseSummary", "root_cause_analysis",
		"recommendations", "actions", "suggested_actions",
		"confidence_reasoning", "score_explanation",
		"related_alerts", "alert_list", "associated_alerts", "alarms",
		"impact_analysis", "event_type", "detection_method",
	})
}
