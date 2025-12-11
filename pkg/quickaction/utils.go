package quickaction

import (
	"fmt"
	"time"
	"alertHub/internal/ctx"
	"alertHub/internal/models"
)

// FindActiveSilenceByFingerprint 查找指定指纹的激活静默规则
// 用于防止重复静默同一个告警
func FindActiveSilenceByFingerprint(ctx *ctx.Context, tenantId, fingerprint string) (*models.AlertSilences, error) {
	// 查询数据库中的所有静默规则
	var silences []models.AlertSilences
	err := ctx.DB.DB().
		Where("tenant_id = ? AND status = ?", tenantId, 1). // status=1 表示启用状态
		Find(&silences).Error
	if err != nil {
		return nil, err
	}

	// 当前时间戳
	now := time.Now().Unix()

	// 遍历静默规则,查找匹配指纹且仍在有效期内的规则
	for _, silence := range silences {
		// 检查静默规则是否已过期
		if silence.EndsAt <= now {
			continue
		}

		// 检查静默规则的标签是否匹配该指纹
		for _, label := range silence.Labels {
			if label.Key == "fingerprint" && label.Value == fingerprint && label.Operator == "=" {
				return &silence, nil
			}
		}
	}

	return nil, fmt.Errorf("未找到激活的静默规则")
}

// FormatDurationChinese 将Go的duration格式(如"1h"、"6h"、"24h")转换为中文友好格式
// 支持的输入格式: "1h" -> "1小时", "30m" -> "30分钟", "24h" -> "24小时"
func FormatDurationChinese(durationStr string) string {
	// 解析duration字符串
	dur, err := time.ParseDuration(durationStr)
	if err != nil {
		return durationStr // 解析失败,返回原始字符串
	}

	// 转换为秒数
	totalSeconds := int64(dur.Seconds())

	// 计算各个时间单位
	days := totalSeconds / 86400
	hours := (totalSeconds % 86400) / 3600
	minutes := (totalSeconds % 3600) / 60

	// 构建中文格式
	var result string
	if days > 0 {
		result = fmt.Sprintf("%d天", days)
		if hours > 0 {
			result += fmt.Sprintf("%d小时", hours)
		}
	} else if hours > 0 {
		result = fmt.Sprintf("%d小时", hours)
		if minutes > 0 {
			result += fmt.Sprintf("%d分钟", minutes)
		}
	} else if minutes > 0 {
		result = fmt.Sprintf("%d分钟", minutes)
	} else {
		result = fmt.Sprintf("%d秒", totalSeconds)
	}

	return result
}
