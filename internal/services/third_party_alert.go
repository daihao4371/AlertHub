package services

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"alertHub/internal/ctx"
	"alertHub/internal/models"
	"alertHub/internal/types"
	"alertHub/pkg/sender"
	"alertHub/pkg/templates"
	"alertHub/pkg/tools"
)

type (
	thirdPartyAlertService struct {
		ctx *ctx.Context
	}

	InterThirdPartyAlertService interface {
		ReceiveAlert(req interface{}) (interface{}, interface{})
		List(req interface{}) (interface{}, interface{})
	}
)

func newInterThirdPartyAlertService(ctx *ctx.Context) InterThirdPartyAlertService {
	return &thirdPartyAlertService{
		ctx: ctx,
	}
}

// RequestReceiveAlert 接收告警请求
type RequestReceiveAlert struct {
	WebhookId string                 `json:"-"` // Webhook ID（从URL路径获取）
	RawData   map[string]interface{} `json:"-"` // 原始JSON数据
	Headers   map[string]string      `json:"-"` // 请求头
}

// ResponseReceiveAlert 接收告警响应
type ResponseReceiveAlert struct {
	Success   bool   `json:"success"`   // 是否成功
	Message   string `json:"message"`   // 消息
	AlertId   string `json:"alertId"`   // 告警记录ID
	Timestamp int64  `json:"timestamp"` // 时间戳
}

// ReceiveAlert 接收第三方告警
func (s thirdPartyAlertService) ReceiveAlert(req interface{}) (interface{}, interface{}) {
	r := req.(*RequestReceiveAlert)

	// 1. 根据WebhookId查找配置
	webhook, err := s.ctx.DB.ThirdPartyWebhook().GetByWebhookId(r.WebhookId)
	if err != nil {
		return ResponseReceiveAlert{
			Success:   false,
			Message:   "Webhook配置不存在",
			Timestamp: time.Now().Unix(),
		}, fmt.Errorf("webhook not found: %v", err)
	}

	// 2. 验证Webhook状态
	if !webhook.IsActive() {
		return ResponseReceiveAlert{
			Success:   false,
			Message:   "Webhook已禁用",
			Timestamp: time.Now().Unix(),
		}, fmt.Errorf("webhook disabled")
	}

	// 3. 提取告警数据（简单字段提取）
	extractedData := s.extractAlertData(r.RawData, webhook.Source)

	// 4. 生成告警指纹（用于去重）
	fingerprint := tools.GenerateAlertFingerprint(
		extractedData.Source,
		extractedData.Host,
		extractedData.Title,
	)

	// 5. 序列化原始数据和请求头
	rawDataStr := tools.JsonMarshalToString(r.RawData)
	headersStr := tools.JsonMarshalToString(r.Headers)

	// 6. 创建告警记录
	now := time.Now().Unix()
	alert := &models.ThirdPartyAlert{
		TenantId:      webhook.TenantId,
		ID:            tools.RandId(),
		WebhookId:     webhook.ID,
		RawData:       rawDataStr,
		Headers:       headersStr,
		AlertId:       extractedData.AlertId,
		Fingerprint:   fingerprint,
		Title:         extractedData.Title,
		Content:       extractedData.Content,
		Severity:      extractedData.Severity,
		Status:        extractedData.Status,
		Source:        extractedData.Source,
		Host:          extractedData.Host,
		Service:       extractedData.Service,
		Tags:          tools.JsonMarshalToString(extractedData.Tags),
		SourceTime:    extractedData.SourceTime,
		ProcessTime:   now,
		ProcessStatus: string(models.ProcessStatusSuccess),
		ErrorMessage:  "",
		FaultCenterId: "",
		EventId:       "",
		CreateAt:      now,
	}

	// 7. 保存告警记录
	err = s.ctx.DB.ThirdPartyAlert().Create(alert)
	if err != nil {
		return ResponseReceiveAlert{
			Success:   false,
			Message:   "保存告警失败",
			Timestamp: time.Now().Unix(),
		}, fmt.Errorf("save alert failed: %v", err)
	}

	// 8. 更新Webhook统计
	s.updateWebhookStats(webhook.ID, webhook.CallCount+1, now)

	// 9. 发送通知（如果配置了通知对象）
	go s.sendNotifications(&webhook, alert, extractedData)

	// 10. 返回成功响应
	return ResponseReceiveAlert{
		Success:   true,
		Message:   "告警接收成功",
		AlertId:   alert.ID,
		Timestamp: now,
	}, nil
}

// extractAlertData 提取告警数据（简化版实现）
// 从原始JSON中提取常见字段，支持多种命名风格
func (s thirdPartyAlertService) extractAlertData(rawData map[string]interface{}, source string) ExtractedAlertData {
	data := ExtractedAlertData{
		Source:     source,
		SourceTime: time.Now().Unix(),
		Status:     string(models.ThirdPartyAlertFiring), // 默认为触发状态
		Severity:   "P2",                                  // 默认P2级别
		Tags:       make(map[string]string),
	}

	// 提取告警ID（支持多种字段名）
	data.AlertId = s.extractString(rawData, []string{"id", "alert_id", "alertId", "eventId", "event_id"})

	// 提取标题（支持多种字段名）
	data.Title = s.extractString(rawData, []string{"title", "subject", "summary", "name", "alert_name", "alertName"})
	if data.Title == "" {
		data.Title = "未命名告警" // 默认标题
	}

	// 提取内容（支持多种字段名）
	data.Content = s.extractString(rawData, []string{"content", "message", "description", "text", "body"})

	// 提取主机信息
	data.Host = s.extractString(rawData, []string{"host", "hostname", "server", "instance"})

	// 提取服务信息
	data.Service = s.extractString(rawData, []string{"service", "service_name", "serviceName", "app", "application"})

	// 提取严重级别
	severityStr := s.extractString(rawData, []string{"severity", "level", "priority", "criticality"})
	if severityStr != "" {
		data.Severity = s.mapSeverity(severityStr)
	}

	// 提取状态
	statusStr := s.extractString(rawData, []string{"status", "state"})
	if statusStr != "" {
		data.Status = s.mapStatus(statusStr)
	}

	// 提取时间戳
	if timestamp, ok := rawData["timestamp"]; ok {
		switch v := timestamp.(type) {
		case float64:
			data.SourceTime = int64(v)
		case int64:
			data.SourceTime = v
		case string:
			// 尝试解析时间字符串
			if t, err := time.Parse(time.RFC3339, v); err == nil {
				data.SourceTime = t.Unix()
			}
		}
	}

	return data
}

// extractString 从map中提取字符串字段（支持多个候选字段名）
func (s thirdPartyAlertService) extractString(data map[string]interface{}, fieldNames []string) string {
	for _, fieldName := range fieldNames {
		if value, ok := data[fieldName]; ok {
			switch v := value.(type) {
			case string:
				return v
			case float64, int, int64:
				return fmt.Sprintf("%v", v)
			}
		}
	}
	return ""
}

// mapSeverity 映射严重级别到AlertHub标准级别（P0/P1/P2）
// P0：最高级别（灾难/紧急/致命）
// P1：高级别（严重/高/重要）
// P2：中级别（警告/一般/低级）
func (s thirdPartyAlertService) mapSeverity(severity string) string {
	// 标准化：转小写并去除空格
	severity = strings.TrimSpace(strings.ToLower(fmt.Sprintf("%v", severity)))

	// P0级别映射：最高优先级
	p0Keywords := []string{"0", "5", "critical", "crit", "disaster", "emergency", "fatal"}
	for _, keyword := range p0Keywords {
		if severity == keyword {
			return "P0"
		}
	}

	// P1级别映射：高优先级
	p1Keywords := []string{"1", "4", "high", "severe", "serious", "major", "error"}
	for _, keyword := range p1Keywords {
		if severity == keyword {
			return "P1"
		}
	}

	// P2级别映射：中等及以下优先级（默认）
	// 包括：warning, warn, average, medium, normal, low, info, information, minor, 2, 3 等
	return "P2"
}

// mapStatus 映射告警状态到AlertHub标准状态
func (s thirdPartyAlertService) mapStatus(status string) string {
	status = strings.TrimSpace(strings.ToLower(fmt.Sprintf("%v", status)))

	// 恢复状态映射
	resolvedKeywords := []string{"ok", "resolved", "recovery", "normal", "cleared"}
	for _, keyword := range resolvedKeywords {
		if status == keyword {
			return string(models.ThirdPartyAlertResolved)
		}
	}

	// 默认为触发状态
	return string(models.ThirdPartyAlertFiring)
}

// updateWebhookStats 更新Webhook统计信息
func (s thirdPartyAlertService) updateWebhookStats(webhookId string, callCount int64, lastCallAt int64) {
	err := s.ctx.DB.ThirdPartyWebhook().UpdateStats(webhookId, callCount, lastCallAt)
	if err != nil {
		// 统计更新失败不影响主流程，只记录日志
		// 这里可以添加日志记录
	}
}

// List 查询第三方告警列表
func (s thirdPartyAlertService) List(req interface{}) (interface{}, interface{}) {
	r := req.(*types.RequestAlertQuery)

	// 查询数据库
	alerts, total, err := s.ctx.DB.ThirdPartyAlert().List(
		r.TenantId,
		r.WebhookId,
		r.ProcessStatus,
		r.Status,
		r.Page,
	)
	if err != nil {
		return nil, fmt.Errorf("查询告警列表失败: %v", err)
	}

	// 转换为响应格式
	list := make([]types.ResponseAlert, 0, len(alerts))
	for _, alert := range alerts {
		list = append(list, types.ResponseAlert{
			ID:            alert.ID,
			WebhookId:     alert.WebhookId,
			AlertId:       alert.AlertId,
			Fingerprint:   alert.Fingerprint,
			Title:         alert.Title,
			Content:       alert.Content,
			Severity:      alert.Severity,
			Status:        alert.Status,
			Source:        alert.Source,
			Host:          alert.Host,
			Service:       alert.Service,
			SourceTime:    alert.SourceTime,
			ProcessTime:   alert.ProcessTime,
			ProcessStatus: alert.ProcessStatus,
			ErrorMessage:  alert.ErrorMessage,
			FaultCenterId: alert.FaultCenterId,
			EventId:       alert.EventId,
		})
	}

	return types.ResponseAlertList{
		List:  list,
		Total: total,
		Page:  r.Page,
	}, nil
}

// ExtractedAlertData 提取的告警数据
type ExtractedAlertData struct {
	AlertId    string            // 告警ID
	Title      string            // 标题
	Content    string            // 内容
	Severity   string            // 严重级别（P0/P1/P2）
	Status     string            // 状态（firing/resolved）
	Source     string            // 来源系统
	Host       string            // 主机
	Service    string            // 服务
	Tags       map[string]string // 标签
	SourceTime int64             // 时间戳
}

// parseJSON 解析JSON字符串
func parseJSON(jsonStr string) (map[string]interface{}, error) {
	var data map[string]interface{}
	err := json.Unmarshal([]byte(jsonStr), &data)
	return data, err
}

// sendNotifications 发送通知到IM（支持告警触发和恢复）
// 异步执行，不影响Webhook接收主流程
func (s thirdPartyAlertService) sendNotifications(webhook *models.ThirdPartyWebhook, alert *models.ThirdPartyAlert, extractedData ExtractedAlertData) {
	// 1. 检查是否配置了通知对象
	if len(webhook.NoticeIds) == 0 {
		return
	}

	// 2. 判断是否为恢复通知
	isRecovered := alert.Status == string(models.ThirdPartyAlertResolved)

	// 3. 构造AlertCurEvent对象（用于复用现有的模板生成逻辑）
	// 注意：第三方告警没有RuleId、DatasourceType等字段，使用Source代替
	alertEvent := &models.AlertCurEvent{
		TenantId:         alert.TenantId,
		DatasourceType:   "Third-Party", // 标识为第三方告警
		RuleId:           alert.WebhookId,
		RuleName:         fmt.Sprintf("[%s] %s", alert.Source, alert.Title),
		Labels:           s.buildLabelsFromAlert(alert, extractedData),
		Annotations:      s.buildAnnotationsFromAlert(alert, extractedData),
		Severity:         alert.Severity,
		IsRecovered:      isRecovered,
		FirstTriggerTime: alert.SourceTime,
		LastEvalTime:     alert.ProcessTime,
		EventId:          alert.ID, // 使用告警记录ID作为EventId
	}

	// 4. 遍历所有通知对象，发送通知
	for _, noticeId := range webhook.NoticeIds {
		// 获取通知对象详情
		noticeData, err := s.ctx.DB.Notice().Get(alert.TenantId, noticeId)
		if err != nil {
			fmt.Printf("[Webhook通知] 获取通知对象失败: noticeId=%s, err=%v\n", noticeId, err)
			continue
		}

		// 生成告警内容
		content := s.generateThirdPartyAlertContent(alertEvent, noticeData)

		// 获取通知Hook和Sign（根据告警等级）
		hook, sign := s.getNoticeHookAndSign(noticeData, alert.Severity)

		// 发送通知
		err = sender.Sender(s.ctx, sender.SendParams{
			TenantId:    alert.TenantId,
			EventId:     alert.ID,
			RuleName:    alertEvent.RuleName,
			Severity:    alert.Severity,
			NoticeType:  noticeData.NoticeType,
			NoticeId:    noticeId,
			NoticeName:  noticeData.Name,
			IsRecovered: isRecovered,
			Hook:        hook,
			Email:       s.getNoticeEmail(noticeData, alert.Severity),
			Content:     content,
			PhoneNumber: noticeData.PhoneNumber,
			Sign:        sign,
		})

		if err != nil {
			fmt.Printf("[Webhook通知] 发送失败: noticeId=%s, noticeName=%s, err=%v\n",
				noticeId, noticeData.Name, err)
		} else {
			fmt.Printf("[Webhook通知] 发送成功: noticeId=%s, noticeName=%s, alertId=%s, status=%s\n",
				noticeId, noticeData.Name, alert.ID, alert.Status)
		}
	}
}

// buildLabelsFromAlert 从第三方告警构建Labels（用于模板渲染）
func (s thirdPartyAlertService) buildLabelsFromAlert(alert *models.ThirdPartyAlert, extractedData ExtractedAlertData) map[string]interface{} {
	labels := make(map[string]interface{})

	// 添加基础标签
	if alert.Host != "" {
		labels["host"] = alert.Host
	}
	if alert.Service != "" {
		labels["service"] = alert.Service
	}
	labels["source"] = alert.Source
	labels["alertId"] = alert.AlertId

	// 添加自定义标签
	for k, v := range extractedData.Tags {
		labels[k] = v
	}

	return labels
}

// buildAnnotationsFromAlert 从第三方告警构建Annotations（告警详细信息）
func (s thirdPartyAlertService) buildAnnotationsFromAlert(alert *models.ThirdPartyAlert, extractedData ExtractedAlertData) string {
	var annotations []string

	// 添加告警内容
	if alert.Content != "" {
		annotations = append(annotations, alert.Content)
	}

	// 添加主机信息
	if alert.Host != "" {
		annotations = append(annotations, fmt.Sprintf("主机: %s", alert.Host))
	}

	// 添加服务信息
	if alert.Service != "" {
		annotations = append(annotations, fmt.Sprintf("服务: %s", alert.Service))
	}

	// 添加来源系统
	annotations = append(annotations, fmt.Sprintf("来源: %s", alert.Source))

	// 添加告警ID
	if alert.AlertId != "" {
		annotations = append(annotations, fmt.Sprintf("告警ID: %s", alert.AlertId))
	}

	return strings.Join(annotations, "\n")
}

// generateThirdPartyAlertContent 生成第三方告警的通知内容
func (s thirdPartyAlertService) generateThirdPartyAlertContent(alert *models.AlertCurEvent, noticeData models.AlertNotice) string {
	// 复用现有的模板生成逻辑
	return templates.NewTemplate(s.ctx, *alert, noticeData).CardContentMsg
}

// getNoticeHookAndSign 获取通知对象的Hook和Sign（根据告警等级）
func (s thirdPartyAlertService) getNoticeHookAndSign(notice models.AlertNotice, severity string) (string, string) {
	if notice.Routes != nil {
		for _, route := range notice.Routes {
			if route.Severity == severity {
				return route.Hook, route.Sign
			}
		}
	}
	return notice.DefaultHook, notice.DefaultSign
}

// getNoticeEmail 获取通知对象的Email配置（根据告警等级）
func (s thirdPartyAlertService) getNoticeEmail(notice models.AlertNotice, severity string) models.Email {
	if notice.Routes != nil {
		for _, route := range notice.Routes {
			if route.Severity == severity {
				return models.Email{
					Subject: notice.Email.Subject,
					To:      route.To,
					CC:      route.CC,
				}
			}
		}
	}
	return notice.Email
}
