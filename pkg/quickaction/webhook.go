package quickaction

import (
	"fmt"
	"alertHub/internal/ctx"
	"alertHub/internal/models"
)

// GetWebhookFromAlert 从告警事件中反查Webhook配置
// 通过告警的FaultCenterId获取故障中心，再从NoticeIds中获取通知对象的Webhook信息
// 返回值: hook(Webhook地址), sign(签名), noticeType(通知类型: feishu/dingtalk), error
func GetWebhookFromAlert(ctx *ctx.Context, alert *models.AlertCurEvent) (string, string, string, error) {
	// 1. 获取故障中心信息（包含通知对象ID列表）
	var faultCenter models.FaultCenter
	var err error

	if alert.FaultCenterId != "" {
		// 普通告警：通过FaultCenterId获取故障中心（name参数为空字符串）
		faultCenter, err = ctx.DB.FaultCenter().Get(alert.TenantId, alert.FaultCenterId, "")
		if err != nil {
			return "", "", "", fmt.Errorf("获取故障中心失败: %w", err)
		}
	} else {
		// 拨测告警：通过RuleId查找拨测规则
		return GetWebhookFromProbingRule(ctx, alert)
	}

	// 2. 遍历故障中心的通知对象ID，查找支持的通知类型(飞书或钉钉)
	for _, noticeId := range faultCenter.NoticeIds {
		noticeObj, err := ctx.DB.Notice().Get(alert.TenantId, noticeId)
		if err != nil {
			continue // 跳过获取失败的通知对象
		}

		// 检查是否为飞书或钉钉通知
		if noticeObj.NoticeType == "FeiShu" {
			hook, sign := ExtractWebhookFromNotice(&noticeObj, alert)
			if hook != "" {
				return hook, sign, "feishu", nil
			}
		} else if noticeObj.NoticeType == "DingDing" {
			hook, sign := ExtractWebhookFromNotice(&noticeObj, alert)
			if hook != "" {
				return hook, sign, "dingtalk", nil
			}
		}
	}

	return "", "", "", fmt.Errorf("未找到飞书或钉钉通知配置")
}

// GetWebhookFromProbingRule 从拨测规则中获取Webhook配置
// 拨测规则直接包含NoticeId字段
func GetWebhookFromProbingRule(ctx *ctx.Context, alert *models.AlertCurEvent) (string, string, string, error) {
	// 查询拨测规则
	var probingRule models.ProbingRule
	err := ctx.DB.DB().
		Where("tenant_id = ? AND rule_id = ?", alert.TenantId, alert.RuleId).
		First(&probingRule).Error
	if err != nil {
		return "", "", "", fmt.Errorf("获取拨测规则失败: %w", err)
	}

	// 获取通知对象
	noticeObj, err := ctx.DB.Notice().Get(alert.TenantId, probingRule.NoticeId)
	if err != nil {
		return "", "", "", fmt.Errorf("获取通知对象失败: %w", err)
	}

	// 提取Webhook配置
	hook, sign := ExtractWebhookFromNotice(&noticeObj, alert)
	if hook == "" {
		return "", "", "", fmt.Errorf("未找到有效的Webhook配置")
	}

	// 根据通知类型返回对应的noticeType
	var noticeType string
	if noticeObj.NoticeType == "FeiShu" {
		noticeType = "feishu"
	} else if noticeObj.NoticeType == "DingDing" {
		noticeType = "dingtalk"
	} else {
		return "", "", "", fmt.Errorf("不支持的通知类型: %s", noticeObj.NoticeType)
	}

	return hook, sign, noticeType, nil
}

// ExtractWebhookFromNotice 从通知对象中提取Webhook配置
// 优先使用DefaultHook，如果为空则根据告警等级从Routes中查找
func ExtractWebhookFromNotice(notice *models.AlertNotice, alert *models.AlertCurEvent) (string, string) {
	// 优先使用默认Webhook
	if notice.DefaultHook != "" {
		return notice.DefaultHook, notice.DefaultSign
	}

	// 如果没有默认Webhook，从Routes中根据告警等级查找
	for _, route := range notice.Routes {
		if route.Severity == alert.Severity {
			return route.Hook, route.Sign
		}
	}

	// 如果没有匹配的等级，尝试使用第一个Route
	if len(notice.Routes) > 0 {
		return notice.Routes[0].Hook, notice.Routes[0].Sign
	}

	return "", ""
}
