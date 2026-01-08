package cmdb

import (
	"alertHub/internal/models"
	"fmt"
	"strings"
)

// CmdbAlertEvent CMDB告警事件格式
// 根据CMDB接入文档定义的标准格式
type CmdbAlertEvent struct {
	AlertId    string                 `json:"alertId"`    // 告警ID，相同的ID将会被视为同一告警事件
	AlertDims  map[string]interface{} `json:"alertDims"`  // 告警维度
	MetricName string                 `json:"metricName"` // 告警指标
	Value      string                 `json:"value"`      // 告警值
	MetricUnit string                 `json:"metricUnit"` // 值单位
	Subject    string                 `json:"subject"`    // 告警标题
	Content    string                 `json:"content"`    // 告警内容
	Time       int64                  `json:"time"`       // 告警时间
	IsRecover  bool                   `json:"isRecover"`  // 是否恢复
	ExtInfo    map[string]interface{} `json:"extInfo"`    // 扩展信息，会填充到告警事件中心的field字段里
	OriginInfo map[string]interface{} `json:"originInfo"` // 原始告警信息，即第三方告警转换为此结构体的数据
	Source     string                 `json:"source"`     // 告警源
}

// Converter 告警事件转换器
type Converter struct{}

// NewConverter 创建新的转换器实例
func NewConverter() *Converter {
	return &Converter{}
}

// ConvertToStandard 将AlertCurEvent转换为CMDB标准格式
// 这个函数将AlertHub的内部告警格式转换为CMDB要求的标准格式
func (c *Converter) ConvertToStandard(alert *models.AlertCurEvent) *CmdbAlertEvent {
	// 构建告警维度 - 从Labels中提取关键维度信息
	alertDims := c.buildAlertDims(alert)

	// 构建扩展信息 - 包含告警的详细元数据
	extInfo := c.buildExtInfo(alert)

	// 构建原始信息 - 保存AlertHub的原始告警数据
	originInfo := c.buildOriginInfo(alert)

	// 生成指标名称 - 基于规则名称或Labels中的指标名
	metricName := c.buildMetricName(alert)

	// 转换告警值 - 将严重级别转换为数值
	value := c.convertSeverityToValue(alert.Severity)

	return &CmdbAlertEvent{
		AlertId:    alert.GetEventId(),      // 使用事件ID作为告警ID
		AlertDims:  alertDims,               // 告警维度信息
		MetricName: metricName,              // 告警指标名称
		Value:      value,                   // 告警值（基于严重级别）
		MetricUnit: "",                      // 告警值单位（暂时为空）
		Subject:    alert.RuleName,          // 使用规则名称作为告警标题
		Content:    c.buildContent(alert),   // 构建告警内容描述
		Time:       alert.GetLastEvalTime(), // 使用最后评估时间
		IsRecover:  alert.IsRecovered,       // 告警恢复状态
		ExtInfo:    extInfo,                 // 扩展信息
		OriginInfo: originInfo,              // 原始告警信息
		Source:     "alerthub",              // 标记来源为AlertHub
	}
}

// buildAlertDims 构建告警维度信息
// 从Labels中提取IP、实例、任务等关键维度
func (c *Converter) buildAlertDims(alert *models.AlertCurEvent) map[string]interface{} {
	alertDims := make(map[string]interface{})

	if alert.Labels != nil {
		// 优先提取IP信息 - CMDB资源关联的关键字段
		if ip, exists := alert.Labels["ip"]; exists {
			alertDims["ip"] = ip
		}

		// 提取实例信息 - 通常包含主机:端口格式
		if instance, exists := alert.Labels["instance"]; exists {
			alertDims["instance"] = instance
			// 如果没有单独的IP，尝试从instance中提取
			if _, hasIP := alertDims["ip"]; !hasIP {
				if instanceStr, ok := instance.(string); ok {
					// 提取 host:port 中的 host 部分
					if parts := strings.Split(instanceStr, ":"); len(parts) > 0 {
						alertDims["ip"] = parts[0]
					}
				}
			}
		}

		// 提取任务信息 - 监控任务标识
		if job, exists := alert.Labels["job"]; exists {
			alertDims["job"] = job
		}

		// 提取主机名信息
		if hostname, exists := alert.Labels["hostname"]; exists {
			alertDims["hostname"] = hostname
		}
	}

	return alertDims
}

// buildExtInfo 构建扩展信息
// 包含AlertHub的核心元数据，用于丰富告警事件信息
func (c *Converter) buildExtInfo(alert *models.AlertCurEvent) map[string]interface{} {
	extInfo := map[string]interface{}{
		"tenantId":             alert.TenantId,             // 租户ID
		"ruleId":               alert.RuleId,               // 规则ID
		"ruleName":             alert.RuleName,             // 规则名称
		"datasourceId":         alert.DatasourceId,         // 数据源ID
		"datasourceType":       alert.DatasourceType,       // 数据源类型
		"fingerprint":          alert.Fingerprint,          // 告警指纹
		"severity":             alert.Severity,             // 严重级别
		"faultCenterId":        alert.FaultCenterId,        // 故障中心ID
		"evalInterval":         alert.EvalInterval,         // 评估间隔
		"forDuration":          alert.ForDuration,          // 持续时间
		"repeatNoticeInterval": alert.RepeatNoticeInterval, // 重复通知间隔
		"status":               string(alert.Status),       // 告警状态
	}

	// 如果有确认状态信息，添加到扩展信息中
	if alert.ConfirmState.IsOk {
		extInfo["confirmState"] = map[string]interface{}{
			"isConfirmed": alert.ConfirmState.IsOk,
			"confirmUser": alert.ConfirmState.ConfirmUsername,
			"confirmTime": alert.ConfirmState.ConfirmActionTime,
		}
	}

	return extInfo
}

// buildOriginInfo 构建原始信息
// 保存AlertHub的完整原始数据，并按照CMDB期望的格式组织
func (c *Converter) buildOriginInfo(alert *models.AlertCurEvent) map[string]interface{} {
	// 转换严重级别为CMDB格式
	cmdbSeverity := c.convertSeverityToValue(alert.Severity)
	
	originInfo := map[string]interface{}{
		"eventId":            alert.EventId,            // 事件ID
		"ruleGroupId":        alert.RuleGroupId,       // 规则组ID  
		"firstTriggerTime":   alert.FirstTriggerTime,  // 首次触发时间
		"lastEvalTime":       alert.LastEvalTime,      // 最后评估时间
		"lastSendTime":       alert.LastSendTime,      // 最后发送时间
		"recoverTime":        alert.RecoverTime,       // 恢复时间
		"labels":             alert.Labels,            // 完整的标签信息
		"annotations":        alert.Annotations,       // 注解信息
		"effectiveTime":      alert.EffectiveTime,     // 生效时间配置
		"severity":           cmdbSeverity,            // CMDB格式的严重级别（warn/error/critical）
		"original_severity":  alert.Severity,          // 原始严重级别（P0/P1/P2/P3）
	}
	
	// 从alertDims中提取IP信息，确保OriginInfo中也有IP
	if alert.Labels != nil {
		if ip, exists := alert.Labels["ip"]; exists {
			originInfo["ip"] = ip
		} else if instance, exists := alert.Labels["instance"]; exists {
			// 如果没有单独的IP，尝试从instance中提取
			if instanceStr, ok := instance.(string); ok {
				if parts := strings.Split(instanceStr, ":"); len(parts) > 0 {
					originInfo["ip"] = parts[0]
				}
			}
		}
	}
	
	// 如果有静默信息，添加到原始信息中
	if alert.SilenceInfo != nil {
		originInfo["silenceInfo"] = map[string]interface{}{
			"silenceId":     alert.SilenceInfo.SilenceId,
			"startsAt":      alert.SilenceInfo.StartsAt,
			"endsAt":        alert.SilenceInfo.EndsAt,
			"comment":       alert.SilenceInfo.Comment,
		}
	}
	
	return originInfo
}

// buildMetricName 构建指标名称
// 优先使用Labels中的__name__，否则基于规则名称生成
func (c *Converter) buildMetricName(alert *models.AlertCurEvent) string {
	// 首先尝试从Labels中获取Prometheus指标名称
	if alert.Labels != nil {
		if name, exists := alert.Labels["__name__"]; exists {
			if nameStr, ok := name.(string); ok && nameStr != "" {
				return nameStr
			}
		}
	}

	// 如果没有__name__，根据规则名称生成标准指标名
	// 将规则名称转换为snake_case格式
	ruleName := strings.ToLower(alert.RuleName)
	ruleName = strings.ReplaceAll(ruleName, " ", "_")
	ruleName = strings.ReplaceAll(ruleName, "-", "_")

	return fmt.Sprintf("custom.%s", ruleName)
}

// convertSeverityToValue 将严重级别转换为CMDB标准格式
// 将AlertHub的P0-P3级别映射为CMDB的warn/error/critical格式
func (c *Converter) convertSeverityToValue(severity string) string {
	switch strings.ToUpper(severity) {
	case "P0":
		return "critical" // 关键级别 -> critical
	case "P1":
		return "error" // 严重级别 -> error
	case "P2":
		return "warn" // 警告级别 -> warn
	default:
		return "warn" // 默认为warn级别
	}
}

// buildContent 构建告警内容描述
// 生成人类可读的告警描述信息
func (c *Converter) buildContent(alert *models.AlertCurEvent) string {
	var content strings.Builder

	// 基本告警信息
	content.WriteString(fmt.Sprintf("告警规则: %s\n", alert.RuleName))
	content.WriteString(fmt.Sprintf("严重级别: %s\n", alert.Severity))
	content.WriteString(fmt.Sprintf("数据源: %s\n", alert.DatasourceType))

	// 如果有注解信息，添加到内容中
	if alert.Annotations != "" {
		content.WriteString(fmt.Sprintf("详细信息: %s\n", alert.Annotations))
	}

	// 添加实例信息（如果有的话）
	if alert.Labels != nil {
		if instance, exists := alert.Labels["instance"]; exists {
			content.WriteString(fmt.Sprintf("实例: %v\n", instance))
		}
	}

	// 添加状态信息
	if alert.IsRecovered {
		content.WriteString("状态: 已恢复")
	} else {
		content.WriteString("状态: 告警中")
	}

	return content.String()
}
