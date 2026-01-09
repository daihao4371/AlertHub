package repo

import (
	"alertHub/internal/models"
	"strings"

	"gorm.io/gorm"
)

type (
	// InterCmdbRepo CMDB仓库接口
	InterCmdbRepo interface {
		// GetHostInfoByIP 根据IP地址获取主机信息（包含应用信息）
		// ip: 主机IP地址（支持带端口，会自动提取IP部分）
		// 返回主机信息，如果未找到返回nil
		GetHostInfoByIP(ip string) (*models.CmdbHostInfo, error)
	}

	cmdbRepo struct {
		db *gorm.DB
	}
)

// newCmdbInterface 创建CMDB仓库实例
func newCmdbInterface(db *gorm.DB, g InterGormDBCli) InterCmdbRepo {
	return &cmdbRepo{
		db: db,
	}
}

// GetHostInfoByIP 根据IP地址获取主机信息（包含应用信息）
// 支持从 "10.10.217.225:9100" 格式中提取IP
func (r *cmdbRepo) GetHostInfoByIP(ip string) (*models.CmdbHostInfo, error) {
	// 提取IP地址（去除端口部分）
	hostIP := extractIPFromInstance(ip)
	if hostIP == "" {
		return nil, nil
	}

	// 查询主机信息
	var host models.CmdbHost
	err := r.db.Where("ip = ?", hostIP).First(&host).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}

	// 查询该主机关联的所有应用
	var applications []models.CmdbHostApplication
	err = r.db.Where("host_id = ?", host.ID).Find(&applications).Error
	if err != nil {
		return nil, err
	}

	// 构建返回结果
	info := &models.CmdbHostInfo{
		HostID:    host.ID,
		IP:        host.IP,
		AppNames:  []string{},
		OpsOwners: []string{},
		DevOwners: []string{},
	}

	// 使用map去重
	opsOwnerMap := make(map[string]bool)
	devOwnerMap := make(map[string]bool)

	// 收集应用信息
	for _, app := range applications {
		// 收集应用名称
		if app.AppName != "" {
			info.AppNames = append(info.AppNames, app.AppName)
		}

		// 收集运维负责人（去重）
		if app.OpsOwner != nil && *app.OpsOwner != "" {
			opsOwner := strings.TrimSpace(*app.OpsOwner)
			if opsOwner != "" && !opsOwnerMap[opsOwner] {
				opsOwnerMap[opsOwner] = true
				info.OpsOwners = append(info.OpsOwners, opsOwner)
			}
		}

		// 收集开发负责人（去重）
		if app.DevOwner != nil && *app.DevOwner != "" {
			devOwner := strings.TrimSpace(*app.DevOwner)
			if devOwner != "" && !devOwnerMap[devOwner] {
				devOwnerMap[devOwner] = true
				info.DevOwners = append(info.DevOwners, devOwner)
			}
		}
	}

	return info, nil
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
