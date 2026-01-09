package services

import (
	"alertHub/internal/ctx"
	"alertHub/internal/models"
	"strings"

	"github.com/zeromicro/go-zero/core/logc"
)

type cmdbService struct {
	ctx *ctx.Context
}

type InterCmdbService interface {
	// GetHostInfoByIP 根据IP地址获取主机信息（包含应用信息）
	// ip: 主机IP地址（支持带端口，会自动提取IP部分）
	// 返回主机信息，如果未找到返回nil
	GetHostInfoByIP(ip string) (*models.CmdbHostInfo, error)

	// EnrichAlertWithCmdbInfo 为告警事件填充CMDB信息
	// 从告警的Labels中提取instance或ip字段，查询CMDB并填充到告警对象中
	EnrichAlertWithCmdbInfo(alert *models.AlertCurEvent) error
}

func newInterCmdbService(ctx *ctx.Context) InterCmdbService {
	return &cmdbService{
		ctx: ctx,
	}
}

// GetHostInfoByIP 根据IP地址获取主机信息（包含应用信息）
func (s *cmdbService) GetHostInfoByIP(ip string) (*models.CmdbHostInfo, error) {
	return s.ctx.DB.Cmdb().GetHostInfoByIP(ip)
}

// EnrichAlertWithCmdbInfo 为告警事件填充CMDB信息
// 从告警的Labels中提取instance或ip字段，查询CMDB并填充到告警对象中
func (s *cmdbService) EnrichAlertWithCmdbInfo(alert *models.AlertCurEvent) error {
	if alert == nil || alert.Labels == nil {
		return nil
	}

	// 优先从Labels中提取IP信息
	var hostIP string

	// 1. 尝试从ip字段获取
	if ipVal, exists := alert.Labels["ip"]; exists {
		if ipStr, ok := ipVal.(string); ok && ipStr != "" {
			hostIP = ipStr
		}
	}

	// 2. 如果没有ip字段，尝试从instance字段提取
	if hostIP == "" {
		if instanceVal, exists := alert.Labels["instance"]; exists {
			if instanceStr, ok := instanceVal.(string); ok && instanceStr != "" {
				// 从 "10.10.217.225:9100" 格式中提取IP
				hostIP = extractIPFromInstance(instanceStr)
			}
		}
	}

	// 如果没有找到IP，直接返回
	if hostIP == "" {
		return nil
	}

	// 查询CMDB信息
	hostInfo, err := s.GetHostInfoByIP(hostIP)
	if err != nil {
		logc.Errorf(s.ctx.Ctx, "查询CMDB信息失败, IP: %s, 错误: %v", hostIP, err)
		return err
	}

	// 如果未找到主机信息，直接返回
	if hostInfo == nil {
		return nil
	}

	// 填充CMDB信息到告警对象
	// 将应用名称、运维负责人、开发负责人添加到Labels中，供模板使用
	if len(hostInfo.AppNames) > 0 {
		// 关联应用：多个应用用逗号分隔
		alert.Labels["cmdb_app_names"] = strings.Join(hostInfo.AppNames, ", ")
	}

	// 合并运维负责人和开发负责人作为值班人员
	allOwners := []string{}
	allOwners = append(allOwners, hostInfo.OpsOwners...)
	allOwners = append(allOwners, hostInfo.DevOwners...)

	// 去重
	ownerMap := make(map[string]bool)
	uniqueOwners := []string{}
	for _, owner := range allOwners {
		owner = strings.TrimSpace(owner)
		if owner != "" && !ownerMap[owner] {
			ownerMap[owner] = true
			uniqueOwners = append(uniqueOwners, owner)
		}
	}

	if len(uniqueOwners) > 0 {
		// 值班人员：多个负责人用逗号分隔
		alert.Labels["cmdb_owners"] = strings.Join(uniqueOwners, ", ")
	}

	return nil
}

// extractIPFromInstance 从instance字符串中提取IP地址
// 支持格式: "10.10.217.225:9100" -> "10.10.217.225"
// 如果已经是IP格式，直接返回
func extractIPFromInstance(instance string) string {
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
