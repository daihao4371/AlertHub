package services

import (
	"alertHub/internal/ctx"
	"alertHub/internal/models"
	"alertHub/internal/repo"
	"alertHub/internal/types"
	"alertHub/pkg/ai"
	"alertHub/pkg/rca"
	"context"
	"fmt"

	"github.com/zeromicro/go-zero/core/logc"
)

// InterRCAService RCA 业务服务接口
type InterRCAService interface {
	// SuggestIncidents 执行告警聚类
	SuggestIncidents(
		ctx context.Context,
		alertEventIds []string,
		topologyData string,
		forceRefresh bool,
		modelName string,
	) (*types.ClusteringSuggestion, error)

	// CommitFeedback 提交用户反馈
	CommitFeedback(
		ctx context.Context,
		suggestionId string,
		status string, // accepted/rejected
		feedbackScore int,
		feedbackComment string,
	) error

	// GetIncidents 列表查询事件
	GetIncidents(
		ctx context.Context,
		filters map[string]interface{},
		page, pageSize int,
	) (*types.ListIncidentsResponseDto, error)

	// GetReport 获取统计报告
	GetReport(
		ctx context.Context,
		timeRangeStart, timeRangeEnd int64,
	) (*types.IncidentReportDto, error)
}

// rcaService RCA 业务服务实现
type rcaService struct {
	ctx          *ctx.Context
	rcaRepo      repo.InterRCARepo
	aiClient     ai.AiClient
	clusteringBl *rca.RCASuggestionBl
	reportBl     *rca.RCAReportBl
}

// NewRCAService 创建 RCA 服务实例
// 从 AiConfig 创建多提供商 AI 客户端（Dify 或 OpenAI）
func NewRCAService(context *ctx.Context, aiConfig *ai.AiConfig) InterRCAService {
	// 根据配置创建 AI 客户端（支持 Dify 和 OpenAI）
	aiClient, err := ai.NewAiClient(aiConfig)
	if err != nil {
		logc.Errorf(context.Ctx, "创建 AI 客户端失败: %v", err)
		// 如果创建失败，使用 nil 客户端，后续调用时会返回错误
		// 这样可以让启动不会因为 AI 配置问题而中断
	}

	// 初始化缓存管理器
	// 注：当前版本暂不启用 Redis 缓存（可选组件）
	// 如果需要启用，可以在这里添加 Redis 初始化逻辑

	// 初始化 Repository - 从 InterEntryRepo 获取 *gorm.DB
	var rcaRepo repo.InterRCARepo
	if context.DB != nil {
		db := context.DB.DB() // 获取 GORM 实例
		rcaRepo = repo.NewRCARepository(db)
	}

	// 获取租户 ID（从上下文或使用默认值）
	tenantId := "default"
	if context.Ctx != nil {
		if tid := context.Ctx.Value("TenantID"); tid != nil {
			if tidStr, ok := tid.(string); ok && tidStr != "" {
				tenantId = tidStr
			}
		}
	}

	// 从 AI 配置中提取模型配置
	clusteringConfig := rca.DefaultAIClusteringConfig()
	if aiConfig.Model != "" {
		clusteringConfig.Model = aiConfig.Model
	}
	if aiConfig.Timeout > 0 {
		clusteringConfig.Timeout = aiConfig.Timeout
	}
	if aiConfig.MaxTokens > 0 {
		clusteringConfig.MaxTokens = aiConfig.MaxTokens
	}

	// 初始化业务逻辑层 - 传递 AiClient 接口而不是 OpenAI 实例
	clusteringBl := rca.NewRCASuggestionBl(
		tenantId,
		rcaRepo,
		aiClient,
		clusteringConfig,
	)

	reportBl := rca.NewRCAReportBl(
		tenantId,
		rcaRepo,
		aiClient,
		clusteringConfig,
	)

	return &rcaService{
		ctx:          context,
		rcaRepo:      rcaRepo,
		aiClient:     aiClient,
		clusteringBl: clusteringBl,
		reportBl:     reportBl,
	}
}

// SuggestIncidents 执行告警聚类分析
// 获取告警详情、参数校验、调用 BL 层进行聚类
// modelName 为空时使用初始化时的默认客户端，非空时动态创建对应模型的客户端
func (s *rcaService) SuggestIncidents(
	ctx context.Context,
	alertEventIds []string,
	topologyData string,
	forceRefresh bool,
	modelName string,
) (*types.ClusteringSuggestion, error) {
	// 参数校验
	if len(alertEventIds) == 0 {
		return nil, models.NewRCAError("聚类", "告警列表不能为空")
	}
	if len(alertEventIds) > 50 {
		return nil, models.NewRCAError("聚类", "告警数量不能超过 50 条")
	}

	// 从上下文获取租户 ID
	tenantId := ctx.Value("TenantID")
	if tenantId == nil {
		return nil, models.NewRCAError("聚类", "缺少租户信息")
	}
	tenantIdStr, ok := tenantId.(string)
	if !ok || tenantIdStr == "" {
		return nil, models.NewRCAError("聚类", "无效的租户ID")
	}

	// 批量获取告警详情
	// 策略1：从数据库历史表获取完整告警信息
	// 策略2：从 Redis 缓存获取活跃告警
	alerts := make([]models.AlertCurEvent, 0, len(alertEventIds))

	// 注意：eventId 与 fingerprint 的对应关系
	// 在 AlertHub 中，需要根据 eventId 查询告警
	// 但历史表可能没有直接的 eventId 字段，需要通过其他方式查询

	// 当前实现方案：
	// 由于无法直接从已知的 eventId 获取完整告警数据，建议：
	// 1. 在 API 层提供 FaultCenterId（这样可以从缓存直接查询）
	// 2. 或者修改数据结构支持 eventId 到告警信息的映射
	// 3. 对于现有实现，优先使用 Redis 缓存获取活跃告警

	s._loadAlertsFromCache(ctx, tenantIdStr, alertEventIds, &alerts)

	// 调用 BL 层进行 AI 聚类
	logc.Infof(ctx, "RCA 聚类请求: 租户=%s, 告警数=%d, 强制刷新=%v, 指定模型=%s", tenantIdStr, len(alerts), forceRefresh, modelName)

	// 根据是否指定模型选择聚类执行器
	clusteringBl, err := s.getClusteringBl(ctx, tenantIdStr, modelName)
	if err != nil {
		return nil, err
	}

	return clusteringBl.SuggestIncidents(ctx, alerts, "", topologyData, forceRefresh)
}

// CommitFeedback 提交用户反馈
func (s *rcaService) CommitFeedback(
	ctx context.Context,
	suggestionId string,
	status string,
	feedbackScore int,
	feedbackComment string,
) error {
	return s.clusteringBl.CommitIncidents(ctx, suggestionId, status, feedbackScore, feedbackComment)
}

// GetIncidents 列表查询事件
// 支持按照租户、状态、严重程度等条件过滤
func (s *rcaService) GetIncidents(
	ctx context.Context,
	filters map[string]interface{},
	page, pageSize int,
) (*types.ListIncidentsResponseDto, error) {
	// 严格提取租户 ID，缺失则返回错误
	tenantId := ctx.Value("TenantID")
	if tenantId == nil {
		return nil, models.NewRCAError("查询事件", "缺少租户信息")
	}
	tenantIdStr, ok := tenantId.(string)
	if !ok || tenantIdStr == "" {
		return nil, models.NewRCAError("查询事件", "无效的租户ID")
	}

	// 查询 RCA 事件
	incidents, total, err := s.rcaRepo.ListIncidents(ctx, tenantIdStr, filters, page, pageSize)
	if err != nil {
		logc.Errorf(ctx, "查询事件失败: %v", err)
		return nil, err
	}

	// 转换为响应 DTO
	dtos := make([]types.IncidentCandidateDto, len(incidents))
	for i, incident := range incidents {
		dtos[i] = types.IncidentCandidateDto{
			ID:                    incident.ID,
			Name:                  incident.Name,
			Severity:              incident.Severity,
			ConfidenceScore:       incident.ConfidenceScore,
			ConfidenceExplanation: incident.ConfidenceExplanation,
			Reasoning:             incident.Reasoning,
			AlertCount:            incident.AlertCount,
			AffectedServices:      incident.AffectedServices,
			AlertSources:          incident.AlertSources,
			RecommendedActions:    incident.RecommendedActions,
			RootCauseSummary:      incident.RootCauseSummary,
			StartTime:             incident.StartTime,
			LastSeenTime:          incident.LastSeenTime,
			Status:                incident.Status,
		}
	}

	return &types.ListIncidentsResponseDto{
		Incidents: dtos,
		Total:     total,
		Page:      page,
		PageSize:  pageSize,
	}, nil
}

// GetReport 获取统计报告
func (s *rcaService) GetReport(
	ctx context.Context,
	timeRangeStart, timeRangeEnd int64,
) (*types.IncidentReportDto, error) {
	logc.Infof(ctx, "生成统计报告: 时间范围 %d ~ %d", timeRangeStart, timeRangeEnd)
	return s.reportBl.GenerateReportSnapshot(ctx, timeRangeStart, timeRangeEnd)
}

// getClusteringBl 获取聚类执行器
// modelName 为空时返回默认执行器，非空时从数据库加载配置并动态创建
func (s *rcaService) getClusteringBl(ctx context.Context, tenantId string, modelName string) (*rca.RCASuggestionBl, error) {
	// 未指定模型 → 使用初始化时的默认执行器（向后兼容）
	if modelName == "" {
		if s.clusteringBl == nil {
			return nil, fmt.Errorf("默认 AI 客户端未初始化，请检查 AI 配置")
		}
		return s.clusteringBl, nil
	}

	// 从数据库加载最新的 AI 配置
	setting, err := s.ctx.DB.Setting().Get()
	if err != nil {
		return nil, fmt.Errorf("获取系统配置失败: %w", err)
	}
	if !setting.AiConfig.GetEnable() {
		return nil, fmt.Errorf("未开启 AI 分析能力")
	}

	// 根据模型名称找到对应的 Provider（复用 models.AiConfig 的方法）
	_, providerConfig := setting.AiConfig.GetProviderByModel(modelName)
	if providerConfig == nil {
		return nil, fmt.Errorf("未找到模型 %s 对应的 Provider 配置", modelName)
	}

	// 创建临时 AI 客户端
	aiConfig := &ai.AiConfig{
		Provider:  providerConfig.Type,
		Url:       providerConfig.Url,
		ApiKey:    providerConfig.AppKey,
		Model:     modelName,
		Timeout:   setting.AiConfig.Timeout,
		MaxTokens: setting.AiConfig.MaxTokens,
	}

	aiClient, err := ai.NewAiClient(aiConfig)
	if err != nil {
		return nil, fmt.Errorf("创建 AI 客户端失败: %w", err)
	}

	// 构建聚类配置
	clusteringConfig := rca.DefaultAIClusteringConfig()
	clusteringConfig.Model = modelName
	if aiConfig.Timeout > 0 {
		clusteringConfig.Timeout = aiConfig.Timeout
	}
	if aiConfig.MaxTokens > 0 {
		clusteringConfig.MaxTokens = aiConfig.MaxTokens
	}

	logc.Infof(ctx, "动态创建 AI 客户端: 模型=%s, Provider=%s", modelName, providerConfig.Type)
	return rca.NewRCASuggestionBl(tenantId, s.rcaRepo, aiClient, clusteringConfig), nil
}

// _loadAlertsFromCache 加载告警信息 - 优先使用 Redis 缓存，回退数据库查询
// 策略：先从缓存查询活跃告警，如果不存在则从数据库查询历史告警
func (s *rcaService) _loadAlertsFromCache(ctx context.Context, tenantId string, alertEventIds []string, alerts *[]models.AlertCurEvent) {
	if len(alertEventIds) == 0 {
		return
	}

	// 第一步：尝试从 Redis 缓存加载告警（快速路径）
	notFoundIds := s._loadAlertsFromRedis(ctx, tenantId, alertEventIds, alerts)

	// 第二步：对于缓存中找不到的告警，从数据库查询（回退机制）
	if len(notFoundIds) > 0 {
		s._loadAlertsFromDatabase(ctx, tenantId, notFoundIds, alerts)
	}
}

// _loadAlertsFromRedis 从 Redis 缓存中加载告警
// 返回值：在缓存中找不到的告警 ID 列表
func (s *rcaService) _loadAlertsFromRedis(ctx context.Context, tenantId string, alertEventIds []string, alerts *[]models.AlertCurEvent) []string {
	if s.ctx == nil || s.ctx.Redis == nil || s.ctx.Redis.Alert() == nil {
		logc.Debugf(ctx, "Redis 缓存不可用，将使用数据库查询")
		return alertEventIds
	}

	// 从数据库获取租户的所有 FaultCenter
	faultCenters, err := s.ctx.DB.FaultCenter().List(tenantId, "")
	if err != nil || len(faultCenters) == 0 {
		logc.Debugf(ctx, "无法获取故障中心列表，将使用数据库查询: %v", err)
		return alertEventIds
	}

	notFoundIds := make([]string, 0)

	// 遍历所有待查询的告警 ID
	for _, eventId := range alertEventIds {
		found := false

		// 在所有故障中心的缓存中查找
		for _, faultCenter := range faultCenters {
			cacheKey := models.BuildAlertEventCacheKey(tenantId, faultCenter.ID)
			events, err := s.ctx.Redis.Alert().GetAllEvents(cacheKey)
			if err == nil && events != nil {
				// 在该故障中心缓存中查找匹配的事件
				for _, event := range events {
					if event.EventId == eventId {
						*alerts = append(*alerts, *event)
						found = true
						logc.Debugf(ctx, "从 Redis 缓存找到告警: eventId=%s", eventId)
						break
					}
				}
			}

			if found {
				break
			}
		}

		if !found {
			notFoundIds = append(notFoundIds, eventId)
		}
	}

	return notFoundIds
}

// _loadAlertsFromDatabase 从数据库中加载告警
// 用于 Redis 缓存中找不到的告警
func (s *rcaService) _loadAlertsFromDatabase(ctx context.Context, tenantId string, alertEventIds []string, alerts *[]models.AlertCurEvent) {
	if s.ctx == nil || s.ctx.DB == nil {
		logc.Errorf(ctx, "无法访问数据库")
		return
	}

	// 从 alert_cur_events 表查询告警
	var curEvents []models.AlertCurEvent
	err := s.ctx.DB.DB().
		Where("tenant_id = ? AND event_id IN ?", tenantId, alertEventIds).
		Find(&curEvents).Error

	if err != nil {
		logc.Errorf(ctx, "从数据库查询告警失败: %v", err)
		return
	}

	if len(curEvents) == 0 {
		logc.Debugf(ctx, "数据库中未找到任何告警，请求的 ID: %v", alertEventIds)
		return
	}

	logc.Infof(ctx, "从数据库加载了 %d 条告警", len(curEvents))
	*alerts = append(*alerts, curEvents...)
}

var RCAService InterRCAService

// InitRCAService 初始化 RCA Service（在应用启动时调用）
// 从系统配置中获取 AI 配置，然后初始化 RCA 服务
func InitRCAService(context *ctx.Context, aiConfig *ai.AiConfig) {
	if aiConfig == nil {
		logc.Error(context.Ctx, "RCA Service 初始化失败：AI 配置为空")
		return
	}
	RCAService = NewRCAService(context, aiConfig)
	logc.Info(context.Ctx, "RCA Service 已初始化")
}

// InitRCAServiceWithModelConfig 使用模型配置初始化 RCA Service
// 将 models.AiConfig 转换为 pkg/ai.AiConfig 后进行初始化
func InitRCAServiceWithModelConfig(context *ctx.Context, modelConfig *models.AiConfig) {
	if modelConfig == nil || !modelConfig.GetEnable() {
		logc.Error(context.Ctx, "RCA Service 初始化失败：模型配置无效或未启用")
		return
	}

	// 从模型配置中提取第一个可用的 Provider 配置
	providers := modelConfig.GetAllProviders()
	if len(providers) == 0 {
		logc.Error(context.Ctx, "RCA Service 初始化失败：没有可用的 Provider 配置")
		return
	}

	providerName := providers[0]
	providerConfig := modelConfig.GetProviderConfig(providerName)
	if providerConfig == nil {
		logc.Error(context.Ctx, "RCA Service 初始化失败：无法获取 Provider 配置")
		return
	}

	// 转换为 pkg/ai.AiConfig
	aiConfig := &ai.AiConfig{
		Provider:  providerConfig.Type,
		Url:       providerConfig.Url,
		ApiKey:    providerConfig.AppKey,
		Model:     providerConfig.GetFirstModel(),
		Timeout:   modelConfig.Timeout,
		MaxTokens: modelConfig.MaxTokens,
		Stream:    false,
	}

	InitRCAService(context, aiConfig)
}
