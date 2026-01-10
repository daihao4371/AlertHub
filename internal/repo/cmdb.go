package repo

import (
	"alertHub/internal/models"
	"alertHub/pkg/cmdb"
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

		// GetHostInfoWithDingDingIdByIP 根据IP地址获取主机信息（包含应用信息和钉钉ID）
		// ip: 主机IP地址（支持带端口，会自动提取IP部分）
		// 返回主机信息（包含运维和开发负责人的钉钉ID），如果未找到返回nil
		GetHostInfoWithDingDingIdByIP(ip string) (*models.CmdbHostInfo, error)
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
	hostIP := cmdb.ExtractIPFromInstance(ip)
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
		HostID:              host.ID,
		IP:                  host.IP,
		AppNames:            []string{},
		OpsOwners:           []string{},
		DevOwners:           []string{},
		OpsOwnerDingDingIds: []string{},
		DevOwnerDingDingIds: []string{},
	}

	// 使用map去重
	opsOwnerMap := make(map[string]bool)
	devOwnerMap := make(map[string]bool)
	opsOwnerDingDingIdMap := make(map[string]bool)
	devOwnerDingDingIdMap := make(map[string]bool)

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

		// 收集钉钉ID
		// 注意：当前实现将dingding_id同时添加到ops和dev的钉钉ID列表中
		// 实际业务中，可能需要根据ops_owner和dev_owner分别查询对应的dingding_id
		dingDingId := cmdb.ExtractDingDingId(app)
		if dingDingId != "" {
			// 将钉钉ID添加到运维负责人列表（如果存在）
			cmdb.AddDingDingIdIfNotExists(dingDingId, opsOwnerDingDingIdMap, &info.OpsOwnerDingDingIds)
			// 将钉钉ID添加到开发负责人列表（如果存在）
			cmdb.AddDingDingIdIfNotExists(dingDingId, devOwnerDingDingIdMap, &info.DevOwnerDingDingIds)
		}
	}

	return info, nil
}

// GetHostInfoWithDingDingIdByIP 根据IP地址获取主机信息（包含应用信息和钉钉ID）
// 支持从 "10.10.217.225:9100" 格式中提取IP
// 注意：此方法会查询并返回运维和开发负责人的钉钉ID
func (r *cmdbRepo) GetHostInfoWithDingDingIdByIP(ip string) (*models.CmdbHostInfo, error) {
	// 提取IP地址（去除端口部分）
	hostIP := cmdb.ExtractIPFromInstance(ip)
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

	// 查询该主机关联的所有应用（包含dingding_id字段）
	var applications []models.CmdbHostApplication
	err = r.db.Where("host_id = ?", host.ID).Find(&applications).Error
	if err != nil {
		return nil, err
	}

	// 构建返回结果
	info := &models.CmdbHostInfo{
		HostID:              host.ID,
		IP:                  host.IP,
		AppNames:            []string{},
		OpsOwners:           []string{},
		DevOwners:           []string{},
		OpsOwnerDingDingIds: []string{},
		DevOwnerDingDingIds: []string{},
	}

	// 使用map去重
	opsOwnerMap := make(map[string]bool)
	devOwnerMap := make(map[string]bool)
	opsOwnerDingDingIdMap := make(map[string]bool)
	devOwnerDingDingIdMap := make(map[string]bool)

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

		// 收集钉钉ID
		// 如果应用有钉钉ID，根据ops_owner和dev_owner分别添加到对应列表
		dingDingId := cmdb.ExtractDingDingId(app)
		if dingDingId == "" {
			continue
		}

		// 如果该应用有运维负责人，将dingding_id添加到运维负责人钉钉ID列表
		if cmdb.HasOwner(app.OpsOwner) {
			cmdb.AddDingDingIdIfNotExists(dingDingId, opsOwnerDingDingIdMap, &info.OpsOwnerDingDingIds)
		}

		// 如果该应用有开发负责人，将dingding_id添加到开发负责人钉钉ID列表
		if cmdb.HasOwner(app.DevOwner) {
			cmdb.AddDingDingIdIfNotExists(dingDingId, devOwnerDingDingIdMap, &info.DevOwnerDingDingIds)
		}
	}

	return info, nil
}
