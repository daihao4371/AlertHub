package cmdb

import (
	"alertHub/internal/models"
	"strings"
)

// ExtractIPFromInstance 从instance字符串中提取IP地址
// 支持格式: "10.10.217.225:9100" -> "10.10.217.225"
// 如果已经是IP格式，直接返回
// 如果输入为空，返回空字符串
func ExtractIPFromInstance(instance string) string {
	if instance == "" {
		return ""
	}

	// 如果包含冒号，提取IP部分
	if strings.Contains(instance, ":") {
		parts := strings.Split(instance, ":")
		if len(parts) > 0 {
			return strings.TrimSpace(parts[0])
		}
	}

	// 直接返回（已经是IP格式）
	return strings.TrimSpace(instance)
}

// ExtractDingDingId 从应用记录中提取并验证钉钉ID
// 如果钉钉ID不存在或为空，返回空字符串
func ExtractDingDingId(app models.CmdbHostApplication) string {
	if app.DingDingId == nil || *app.DingDingId == "" {
		return ""
	}
	dingDingId := strings.TrimSpace(*app.DingDingId)
	if dingDingId == "" {
		return ""
	}
	return dingDingId
}

// HasOwner 检查负责人字段是否存在且不为空
// 用于判断ops_owner或dev_owner字段是否有有效值
func HasOwner(owner *string) bool {
	return owner != nil && *owner != "" && strings.TrimSpace(*owner) != ""
}

// AddDingDingIdIfNotExists 将钉钉ID添加到列表中（去重）
// 如果ID已存在，跳过添加
// idMap: 用于去重的map，key为钉钉ID，value为是否已存在
// idList: 钉钉ID列表的指针，用于追加新ID
func AddDingDingIdIfNotExists(dingDingId string, idMap map[string]bool, idList *[]string) {
	if idMap[dingDingId] {
		return
	}
	idMap[dingDingId] = true
	*idList = append(*idList, dingDingId)
}
