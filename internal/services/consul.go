package services

import (
	"alertHub/internal/ctx"
	"alertHub/internal/models"
	consulclient "alertHub/pkg/consul"
	"context"
	"fmt"
	"net"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/robfig/cron/v3"
	"github.com/zeromicro/go-zero/core/logc"
)

type (
	consulService struct {
		ctx *ctx.Context
	}

	// InterConsulService Consul 服务接口
	InterConsulService interface {
		// 目标管理
		GetAllTargets(tenantId string, filters map[string]interface{}, page, pageSize int) (interface{}, interface{})
		GetTargetById(id int64) (interface{}, interface{})
		DeregisterTarget(tenantId string, targetId int64, reason string, userId string) (interface{}, interface{})
		ReRegisterTarget(tenantId string, targetId int64, userId string) (interface{}, interface{})
		RegisterTarget(tenantId string, serviceID, serviceName, address string, port int, job string, tags []string, labels map[string]string) (interface{}, interface{})

		// 标签管理
		GetTargetsByTag(tenantId string, tag string, page, pageSize int) (interface{}, interface{})
		GetTargetsByJobAndTag(tenantId string, job, tag string, page, pageSize int) (interface{}, interface{})
		UpdateTargetTags(tenantId string, targetId int64, labels map[string]interface{}) (interface{}, interface{})

		// 同步管理
		SyncTargets(tenantId string) (interface{}, interface{})
		SyncTargetsCronjob(ctx context.Context)

		// 注销记录管理
		GetOfflineLogs(tenantId string, page, pageSize int) (interface{}, interface{})
	}
)

func newInterConsulService(ctx *ctx.Context) InterConsulService {
	return &consulService{
		ctx: ctx,
	}
}

// normalizePagination 标准化分页参数，确保页码和每页数量的有效性
// 返回标准化后的页码和每页数量
func (c *consulService) normalizePagination(page, pageSize int) (int, int) {
	// 页码最小为1
	if page < 1 {
		page = 1
	}
	// 每页数量范围：1-100，默认20
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}
	return page, pageSize
}

// labelsEqual 比较两个 Labels map 是否相等
// 用于判断 Tags 和 Meta 是否发生变化
func (c *consulService) labelsEqual(labels1, labels2 map[string]interface{}) bool {
	// 如果两个都为 nil 或空，认为相等
	if (labels1 == nil || len(labels1) == 0) && (labels2 == nil || len(labels2) == 0) {
		return true
	}

	// 如果其中一个为 nil 或空，另一个不为空，则不相等
	if (labels1 == nil || len(labels1) == 0) || (labels2 == nil || len(labels2) == 0) {
		return false
	}

	// 比较长度
	if len(labels1) != len(labels2) {
		return false
	}

	// 比较每个键值对
	for key, val1 := range labels1 {
		val2, exists := labels2[key]
		if !exists {
			return false
		}

		// 使用 fmt.Sprintf 进行简单的值比较（适用于基本类型和数组）
		if fmt.Sprintf("%v", val1) != fmt.Sprintf("%v", val2) {
			return false
		}
	}

	return true
}

// getConsulConfigFromDataSource 从数据源系统中获取 Consul 配置
// 返回从 AlertDataSource 中类型为 "consul" 的数据源配置
func (c *consulService) getConsulConfigFromDataSource(tenantId string) (*models.DsConsulConfig, error) {
	// 从数据源管理中查询类型为 "consul" 的数据源
	dataSources, err := c.ctx.DB.Datasource().List(tenantId, "", "consul", "")
	if err != nil {
		return nil, fmt.Errorf("查询 Consul 数据源失败: %w", err)
	}

	// 检查是否找到 Consul 数据源
	if len(dataSources) == 0 {
		return nil, fmt.Errorf("请先在数据源管理中配置 Consul 数据源")
	}

	// 获取第一个 Consul 数据源的配置
	dataSource := dataSources[0]
	if dataSource.ConsulConfig.Address == "" {
		return nil, fmt.Errorf("Consul 数据源配置不完整，缺少服务器地址")
	}

	return &dataSource.ConsulConfig, nil
}

// GetAllTargets 获取所有目标机器
func (c *consulService) GetAllTargets(tenantId string, filters map[string]interface{}, page, pageSize int) (interface{}, interface{}) {
	// 标准化分页参数
	page, pageSize = c.normalizePagination(page, pageSize)

	targets, total, err := c.ctx.DB.Consul().GetTargets(tenantId, filters, page, pageSize)
	if err != nil {
		return nil, fmt.Errorf("获取目标列表失败: %w", err)
	}

	// 获取统计信息
	allTargets, err := c.ctx.DB.Consul().GetAllTargetsByTenant(tenantId)
	if err != nil {
		return nil, fmt.Errorf("获取统计信息失败: %w", err)
	}

	var passingCount, warningCount, criticalCount, noChecksCount int
	// 同时提取所有不重复的 Job 名称，用于前端下拉筛选（避免只从当前页提取导致遗漏）
	uniqueJobsMap := make(map[string]bool)
	for _, t := range allTargets {
		switch t.Status {
		case "passing":
			passingCount++
		case "warning":
			warningCount++
		case "critical":
			criticalCount++
		case "no checks":
			noChecksCount++
		}
		if t.Job != "" {
			uniqueJobsMap[t.Job] = true
		}
	}

	uniqueJobs := make([]string, 0, len(uniqueJobsMap))
	for job := range uniqueJobsMap {
		uniqueJobs = append(uniqueJobs, job)
	}

	// 计算可用率（基于 passing 状态）
	availabilityRate := 0.0
	if len(allTargets) > 0 {
		availabilityRate = float64(passingCount) / float64(len(allTargets)) * 100
	}

	var targetList []map[string]interface{}
	for _, t := range targets {
		targetList = append(targetList, map[string]interface{}{
			"id":                 t.ID,
			"instance":           t.Instance,
			"job":                t.Job,
			"status":             t.Status,
			"labels":             t.Labels,
			"serviceId":          t.ServiceID,
			"serviceName":        t.ServiceName,
			"consulDeregistered": t.ConsulDeregistered,
			"createdAt":          t.CreatedAt,
		})
	}

	return map[string]interface{}{
		"total":    total,
		"page":     page,
		"pageSize": pageSize,
		"list":     targetList,
		"summary": map[string]interface{}{
			"totalCount":       len(allTargets),
			"upCount":          passingCount,
			"downCount":        criticalCount,
			"offlineCount":     noChecksCount,
			"availabilityRate": availabilityRate,
			"uniqueJobs":       uniqueJobs,
		},
	}, nil
}

// GetTargetById 按ID获取目标详情
func (c *consulService) GetTargetById(id int64) (interface{}, interface{}) {
	target, err := c.ctx.DB.Consul().GetTargetById(id)
	if err != nil {
		return nil, fmt.Errorf("获取目标详情失败: %w", err)
	}

	return map[string]interface{}{
		"id":                 target.ID,
		"instance":           target.Instance,
		"job":                target.Job,
		"status":             target.Status,
		"labels":             target.Labels,
		"serviceId":          target.ServiceID,
		"serviceName":        target.ServiceName,
		"consulDeregistered": target.ConsulDeregistered,
		"createdAt":          target.CreatedAt,
	}, nil
}

// DeregisterTarget 注销目标机器并清理告警
func (c *consulService) DeregisterTarget(tenantId string, targetId int64, reason string, userId string) (interface{}, interface{}) {
	// 获取目标信息
	target, err := c.ctx.DB.Consul().GetTargetById(targetId)
	if err != nil {
		return nil, fmt.Errorf("获取目标信息失败: %w", err)
	}

	if target.ID == 0 {
		return nil, fmt.Errorf("目标不存在")
	}

	// 从数据源系统中获取 Consul 配置
	config, err := c.getConsulConfigFromDataSource(tenantId)
	if err != nil {
		// 未配置 Consul 数据源，仍允许在本地数据库中标记为已注销
		// 但无法在 Consul 中进行注销操作
	} else {
		// 创建 Consul 客户端并注销服务
		consulConfig := consulclient.ClientConfig{
			Address: config.Address,
			Token:   config.Token,
		}
		client, err := consulclient.NewClient(consulConfig)
		if err != nil {
			// 记录错误但继续，允许本地数据库更新
			fmt.Printf("创建 Consul 客户端失败: %v\n", err)
		} else {
			// 注销 Consul 中的服务
			if target.ServiceID != "" {
				_ = client.DeregisterService(context.Background(), target.ServiceID)
			}
		}
	}

	// 清理相关的告警事件
	alertEventsCleared := 0
	// TODO: 根据 target.Instance 清理 AlertCurEvent 中相关的告警

	// 更新目标状态为已注销（使用 Consul 健康检查状态）
	target.Status = "no checks"
	target.ConsulDeregistered = true
	now := time.Now()
	target.DeregistrationTime = &now
	_ = c.ctx.DB.Consul().UpdateTarget(target)

	// 记录注销历史
	log := models.ConsulTargetOfflineLog{
		TenantId:           tenantId,
		Instance:           target.Instance,
		Job:                target.Job,
		Labels:             target.Labels,
		Reason:             reason,
		DeregisteredBy:     userId,
		AlertEventsCleared: alertEventsCleared,
	}
	_ = c.ctx.DB.Consul().CreateOfflineLog(log)

	return map[string]interface{}{
		"instance":           target.Instance,
		"alertEventsCleared": alertEventsCleared,
		"deregistrationTime": now,
		"message":            "机器已从 Consul 中注销",
	}, nil
}

// buildInstanceFromAddressAndPort 从 address 和 port 构建 Instance 字符串
// 格式： "192.168.1.100:9100" 或 "192.168.1.100"（如果 port 为 0）
func (c *consulService) buildInstanceFromAddressAndPort(address string, port int) string {
	if port > 0 {
		return fmt.Sprintf("%s:%d", address, port)
	}
	return address
}

// parseInstanceAddressAndPort 从 Instance 字符串中解析出 address 和 port
// Instance 格式： "192.168.1.100:9100" 或 "192.168.1.100"
func (c *consulService) parseInstanceAddressAndPort(instance string) (string, int, error) {
	// 如果包含冒号，说明有端口
	if strings.Contains(instance, ":") {
		parts := strings.Split(instance, ":")
		if len(parts) != 2 {
			return "", 0, fmt.Errorf("实例地址格式不正确: %s", instance)
		}
		address := parts[0]
		port, err := strconv.Atoi(parts[1])
		if err != nil {
			return "", 0, fmt.Errorf("端口格式不正确: %s", parts[1])
		}
		return address, port, nil
	}

	// 如果没有端口，返回默认端口 0（Consul 会使用服务默认端口）
	return instance, 0, nil
}

// ReRegisterTarget 重新上线已注销的目标
func (c *consulService) ReRegisterTarget(tenantId string, targetId int64, userId string) (interface{}, interface{}) {
	// 获取目标信息
	target, err := c.ctx.DB.Consul().GetTargetById(targetId)
	if err != nil {
		return nil, fmt.Errorf("获取目标信息失败: %w", err)
	}

	if target.ID == 0 {
		return nil, fmt.Errorf("目标不存在")
	}

	// 检查目标是否属于当前租户
	if target.TenantId != tenantId {
		return nil, fmt.Errorf("目标不属于当前租户")
	}

	// 检查目标是否已注销
	if !target.ConsulDeregistered {
		return nil, fmt.Errorf("目标未注销，无需重新上线")
	}

	// 验证必要字段是否存在
	if target.ServiceID == "" || target.ServiceName == "" {
		return nil, fmt.Errorf("目标 ServiceID 或 ServiceName 为空，无法重新注册到 Consul")
	}

	// 从数据源系统中获取 Consul 配置
	config, err := c.getConsulConfigFromDataSource(tenantId)
	if err != nil {
		return nil, fmt.Errorf("获取 Consul 配置失败: %w", err)
	}

	// 从 Labels 中提取 Tags 和 Meta，使用 pkg 层的辅助函数
	tags, meta := consulclient.ExtractTagsAndMetaFromLabels(target.Labels)

	// 创建 Consul 客户端
	consulConfig := consulclient.ClientConfig{
		Address: config.Address,
		Token:   config.Token,
	}
	client, err := consulclient.NewClient(consulConfig)
	if err != nil {
		return nil, fmt.Errorf("创建 Consul 客户端失败: %w", err)
	}

	// 从 Instance 中解析 address 和 port
	address, port, err := c.parseInstanceAddressAndPort(target.Instance)
	if err != nil {
		return nil, fmt.Errorf("解析实例地址失败: %w", err)
	}

	// 如果端口为 0，尝试从 Consul 获取服务的原始信息
	// 注意：如果服务已被注销，GetServiceByID 会失败，这是正常的
	if port == 0 {
		serviceInfo, err := client.GetServiceByID(context.Background(), target.ServiceID)
		if err != nil || serviceInfo == nil || serviceInfo.Port == 0 {
			return nil, fmt.Errorf("实例地址 '%s' 中缺少端口信息，且无法从 Consul 获取（服务可能已被注销）。请先同步 Consul 目标，确保 Instance 格式为 'address:port'", target.Instance)
		}
		port = serviceInfo.Port
		if address == "" {
			address = serviceInfo.Address
		}
	}

	// 重新注册服务到 Consul
	if err := client.RegisterService(
		context.Background(),
		target.ServiceID,
		target.ServiceName,
		address,
		port,
		tags,
		meta,
	); err != nil {
		return nil, fmt.Errorf("重新注册服务到 Consul 失败: %w", err)
	}

	// 恢复目标状态
	target.ConsulDeregistered = false
	target.Status = "passing"       // 设置为正常状态，等待下次同步时根据实际健康检查状态更新
	target.DeregistrationTime = nil // 清空注销时间

	// 更新数据库
	if err := c.ctx.DB.Consul().UpdateTarget(target); err != nil {
		return nil, fmt.Errorf("更新目标状态失败: %w", err)
	}

	return map[string]interface{}{
		"instance": target.Instance,
		"status":   target.Status,
		"message":  "目标已重新注册到 Consul 并上线",
	}, nil
}

// RegisterTarget 手动注册服务到 Consul
func (c *consulService) RegisterTarget(tenantId string, serviceID, serviceName, address string, port int, job string, tags []string, labels map[string]string) (interface{}, interface{}) {
	// 验证必填字段
	if serviceID == "" {
		return nil, fmt.Errorf("ServiceID 不能为空")
	}
	if serviceName == "" {
		return nil, fmt.Errorf("ServiceName 不能为空")
	}
	if address == "" {
		return nil, fmt.Errorf("服务地址不能为空")
	}
	if port <= 0 || port > 65535 {
		return nil, fmt.Errorf("服务端口必须在 1-65535 之间")
	}

	// 检查数据库中是否已存在相同的 ServiceID（同一租户内）
	existingTarget, err := c.ctx.DB.Consul().GetTargetsByInstance(tenantId, c.buildInstanceFromAddressAndPort(address, port))
	if err == nil && existingTarget.ID > 0 {
		// 检查是否是同一个 ServiceID
		if existingTarget.ServiceID == serviceID {
			return nil, fmt.Errorf("ServiceID '%s' 已存在，无法重复注册", serviceID)
		}
	}

	// 从数据源系统中获取 Consul 配置
	config, err := c.getConsulConfigFromDataSource(tenantId)
	if err != nil {
		return nil, fmt.Errorf("获取 Consul 配置失败: %w", err)
	}

	// 创建 Consul 客户端
	consulConfig := consulclient.ClientConfig{
		Address: config.Address,
		Token:   config.Token,
	}
	client, err := consulclient.NewClient(consulConfig)
	if err != nil {
		return nil, fmt.Errorf("创建 Consul 客户端失败: %w", err)
	}

	// 检查 Consul 中是否已存在该 ServiceID
	existingService, err := client.GetServiceByID(context.Background(), serviceID)
	if err == nil && existingService != nil {
		return nil, fmt.Errorf("ServiceID '%s' 在 Consul 中已存在，无法重复注册", serviceID)
	}

	// 合并 Tags 和 Labels 到 Meta
	// Labels 中的键值对会作为 Meta 存储
	meta := make(map[string]string)
	if labels != nil {
		for k, v := range labels {
			meta[k] = v
		}
	}

	// 注册服务到 Consul
	if err := client.RegisterService(
		context.Background(),
		serviceID,
		serviceName,
		address,
		port,
		tags,
		meta,
	); err != nil {
		return nil, fmt.Errorf("注册服务到 Consul 失败: %w", err)
	}

	// 构建 Labels（用于数据库存储）
	// 将 Tags 和 Meta 合并到 Labels 中
	dbLabels := consulclient.BuildLabelsFromTagsAndMeta(tags, meta)

	// 构建 Instance 字段（address:port 格式）
	instance := c.buildInstanceFromAddressAndPort(address, port)

	// 创建目标记录并保存到数据库
	newTarget := models.ConsulTarget{
		TenantId:           tenantId,
		Instance:           instance,
		Job:                job,
		Labels:             dbLabels,
		ServiceID:          serviceID,
		ServiceName:        serviceName,
		Status:             "passing", // 新注册的服务默认为 passing 状态
		ConsulDeregistered: false,
		DeregistrationTime: nil,
		CreatedAt:          time.Now(),
		UpdatedAt:          time.Now(),
	}

	if err := c.ctx.DB.Consul().CreateTarget(newTarget); err != nil {
		// 如果数据库保存失败，尝试从 Consul 注销服务（回滚）
		_ = client.DeregisterService(context.Background(), serviceID)
		return nil, fmt.Errorf("保存目标到数据库失败: %w", err)
	}

	return map[string]interface{}{
		"id":        newTarget.ID,
		"instance":  instance,
		"serviceId": serviceID,
		"status":    newTarget.Status,
		"message":   "服务已成功注册到 Consul 并保存到数据库",
	}, nil
}

// probeTargetHealth 探测目标实例是否可达（TCP 连接检测）
// 用于同步时判断缺失的服务是否需要重新注册到 Consul
// instance 格式为 "address:port"，如 "10.0.42.9:9100"
func (c *consulService) probeTargetHealth(instance string, timeout time.Duration) bool {
	conn, err := net.DialTimeout("tcp", instance, timeout)
	if err != nil {
		return false
	}
	conn.Close()
	return true
}

// SyncTargets 同步 Consul 中的目标
// 使用 Catalog + Health API 获取集群全局视图，包括不健康节点上的服务
// 服务从 Consul 消失后标记为 "critical"（而非 "no checks"），确保机器恢复后自动恢复监控
func (c *consulService) SyncTargets(tenantId string) (interface{}, interface{}) {
	// 第一步：自动清理重复的目标记录（保留最新的那条）
	deletedCount, err := c.ctx.DB.Consul().CleanupDuplicateTargets(tenantId)
	if err != nil {
		logc.Errorf(context.Background(), "清理重复记录失败: %v", err)
	} else if deletedCount > 0 {
		logc.Infof(context.Background(), "同步前自动清理了 %d 条重复记录", deletedCount)
	}

	// 第二步：从数据源系统中获取 Consul 配置
	config, err := c.getConsulConfigFromDataSource(tenantId)
	if err != nil {
		return nil, err
	}

	consulConfig := consulclient.ClientConfig{
		Address: config.Address,
		Token:   config.Token,
	}
	client, err := consulclient.NewClient(consulConfig)
	if err != nil {
		return nil, fmt.Errorf("创建 Consul 客户端失败: %w", err)
	}

	// 第三步：通过 Catalog API 获取集群中所有服务名称
	// Catalog API 能看到整个集群的服务，包括不健康节点上的
	serviceNames, err := client.GetCatalogServiceNames(context.Background())
	if err != nil {
		return nil, fmt.Errorf("获取 Catalog 服务列表失败: %w", err)
	}

	// 第四步：遍历每个服务，通过 Health API 获取实例及健康状态
	// consulServiceMap: serviceID → ServiceHealthEntry
	consulServiceMap := make(map[string]consulclient.ServiceHealthEntry)
	for _, serviceName := range serviceNames {
		entries, err := client.GetServiceHealthEntries(context.Background(), serviceName)
		if err != nil {
			logc.Errorf(context.Background(), "获取服务 %s 的健康信息失败: %v", serviceName, err)
			continue
		}
		for _, entry := range entries {
			consulServiceMap[entry.ServiceID] = entry
		}
	}

	// 第五步：获取数据库中该租户的所有非手动注销的目标
	dbTargets, err := c.ctx.DB.Consul().GetAllTargetsByTenant(tenantId)
	if err != nil {
		return nil, fmt.Errorf("获取数据库目标列表失败: %w", err)
	}

	// 构建 Map 用于快速查找，重复记录保留最新的
	dbTargetMap := make(map[string]models.ConsulTarget)
	for _, target := range dbTargets {
		if existing, exists := dbTargetMap[target.ServiceID]; exists {
			if target.UpdatedAt.After(existing.UpdatedAt) {
				dbTargetMap[target.ServiceID] = target
			}
		} else {
			dbTargetMap[target.ServiceID] = target
		}
	}

	// 收集需要批量操作的目标
	toCreate := make([]models.ConsulTarget, 0)
	toUpdate := make([]models.ConsulTarget, 0)

	// 第六步：对账 — 处理 Consul 中存在的服务
	for serviceID, entry := range consulServiceMap {
		labels := consulclient.BuildLabelsFromTagsAndMeta(entry.Tags, entry.Meta)
		expectedInstance := c.buildInstanceFromAddressAndPort(entry.Address, entry.Port)

		if dbTarget, exists := dbTargetMap[serviceID]; exists {
			// 服务在 DB 中已存在，检查是否需要更新
			needUpdate := false

			if dbTarget.Instance != expectedInstance {
				dbTarget.Instance = expectedInstance
				needUpdate = true
			}
			// 用 Consul 健康检查的真实状态更新 DB 状态
			if dbTarget.Status != entry.Status {
				dbTarget.Status = entry.Status
				needUpdate = true
			}
			if !c.labelsEqual(dbTarget.Labels, labels) {
				dbTarget.Labels = labels
				needUpdate = true
			}

			if needUpdate {
				toUpdate = append(toUpdate, dbTarget)
			}
		} else {
			// 新服务，创建新目标记录
			newTarget := models.ConsulTarget{
				TenantId:    tenantId,
				Instance:    expectedInstance,
				Job:         entry.ServiceName,
				ServiceID:   serviceID,
				ServiceName: entry.ServiceName,
				Status:      entry.Status,
				Labels:      labels,
			}
			toCreate = append(toCreate, newTarget)
		}
	}

	// 第七步：对账 — 处理 Consul 中不存在的服务
	// 对于非手动注销的缺失目标：
	//   1. TCP 探测机器是否可达
	//   2. 可达 → 自动重新注册到 Consul → 标记为 passing
	//   3. 不可达 → 标记为 critical
	absentTargets := make([]models.ConsulTarget, 0)
	for _, dbTarget := range dbTargets {
		if _, inConsul := consulServiceMap[dbTarget.ServiceID]; !inConsul {
			absentTargets = append(absentTargets, dbTarget)
		}
	}

	toAbsentServiceIDs := make([]string, 0)
	toReRegisteredIDs := make([]string, 0)

	if len(absentTargets) > 0 {
		var (
			mu  sync.Mutex
			wg  sync.WaitGroup
			sem = make(chan struct{}, 20) // 并发限制：最多 20 个并发探测
		)

		for _, target := range absentTargets {
			wg.Add(1)
			sem <- struct{}{}
			go func(t models.ConsulTarget) {
				defer wg.Done()
				defer func() { <-sem }()

				// TCP 探测目标是否可达
				if !c.probeTargetHealth(t.Instance, 2*time.Second) {
					mu.Lock()
					if t.Status != "critical" {
						toAbsentServiceIDs = append(toAbsentServiceIDs, t.ServiceID)
					}
					mu.Unlock()
					return
				}

				// 目标可达，尝试重新注册到 Consul
				address, port, err := c.parseInstanceAddressAndPort(t.Instance)
				if err != nil {
					mu.Lock()
					if t.Status != "critical" {
						toAbsentServiceIDs = append(toAbsentServiceIDs, t.ServiceID)
					}
					mu.Unlock()
					return
				}

				tags, meta := consulclient.ExtractTagsAndMetaFromLabels(t.Labels)
				if err := client.RegisterService(
					context.Background(),
					t.ServiceID,
					t.ServiceName,
					address,
					port,
					tags,
					meta,
				); err != nil {
					logc.Errorf(context.Background(), "自动重新注册服务 %s (%s) 失败: %v", t.ServiceID, t.Instance, err)
					mu.Lock()
					if t.Status != "critical" {
						toAbsentServiceIDs = append(toAbsentServiceIDs, t.ServiceID)
					}
					mu.Unlock()
					return
				}

				// 注册成功
				mu.Lock()
				if t.Status != "passing" {
					toReRegisteredIDs = append(toReRegisteredIDs, t.ServiceID)
				}
				mu.Unlock()
			}(target)
		}

		wg.Wait()

		if len(toReRegisteredIDs) > 0 {
			logc.Infof(context.Background(), "自动重新注册了 %d 个缺失的服务到 Consul", len(toReRegisteredIDs))
		}
	}

	// 第八步：批量执行数据库操作
	if len(toCreate) > 0 {
		if err := c.ctx.DB.Consul().BatchCreateTargets(toCreate); err != nil {
			return nil, fmt.Errorf("批量创建目标失败: %w", err)
		}
	}
	if len(toUpdate) > 0 {
		if err := c.ctx.DB.Consul().BatchUpdateTargets(toUpdate); err != nil {
			return nil, fmt.Errorf("批量更新目标失败: %w", err)
		}
	}
	if len(toAbsentServiceIDs) > 0 {
		if err := c.ctx.DB.Consul().BatchUpdateTargetStatusByServiceIDs(tenantId, toAbsentServiceIDs, "critical"); err != nil {
			return nil, fmt.Errorf("批量更新离线状态失败: %w", err)
		}
	}
	if len(toReRegisteredIDs) > 0 {
		if err := c.ctx.DB.Consul().BatchUpdateTargetStatusByServiceIDs(tenantId, toReRegisteredIDs, "passing"); err != nil {
			return nil, fmt.Errorf("批量更新重新注册状态失败: %w", err)
		}
	}

	return map[string]interface{}{
		"syncTime":              time.Now(),
		"cleanedDuplicateCount": deletedCount,
		"newTargetsCount":       len(toCreate),
		"updatedTargetsCount":   len(toUpdate),
		"absentTargetsCount":    len(toAbsentServiceIDs),
		"reRegisteredCount":     len(toReRegisteredIDs),
		"totalTargetsCount":     len(consulServiceMap),
	}, nil
}

// GetTargetsByTag 按标签获取目标列表
func (c *consulService) GetTargetsByTag(tenantId string, tag string, page, pageSize int) (interface{}, interface{}) {
	// 标准化分页参数
	page, pageSize = c.normalizePagination(page, pageSize)

	// 从数据库查询按标签过滤的目标
	targets, total, err := c.ctx.DB.Consul().GetTargetsByTag(tenantId, tag, page, pageSize)
	if err != nil {
		return nil, fmt.Errorf("查询目标失败: %w", err)
	}

	// 构建返回数据
	var targetList []map[string]interface{}
	for _, t := range targets {
		targetList = append(targetList, map[string]interface{}{
			"id":        t.ID,
			"instance":  t.Instance,
			"job":       t.Job,
			"status":    t.Status,
			"labels":    t.Labels,
			"serviceId": t.ServiceID,
			"createdAt": t.CreatedAt,
		})
	}

	return map[string]interface{}{
		"total":    total,
		"page":     page,
		"pageSize": pageSize,
		"tag":      tag,
		"list":     targetList,
	}, nil
}

// GetTargetsByJobAndTag 按 Job 和标签结合查询目标
func (c *consulService) GetTargetsByJobAndTag(tenantId string, job, tag string, page, pageSize int) (interface{}, interface{}) {
	// 标准化分页参数
	page, pageSize = c.normalizePagination(page, pageSize)

	// 从数据库查询按 Job 和标签结合过滤的目标
	targets, total, err := c.ctx.DB.Consul().GetTargetsByJobAndTag(tenantId, job, tag, page, pageSize)
	if err != nil {
		return nil, fmt.Errorf("查询目标失败: %w", err)
	}

	// 构建返回数据
	var targetList []map[string]interface{}
	for _, t := range targets {
		targetList = append(targetList, map[string]interface{}{
			"id":        t.ID,
			"instance":  t.Instance,
			"job":       t.Job,
			"status":    t.Status,
			"labels":    t.Labels,
			"serviceId": t.ServiceID,
			"createdAt": t.CreatedAt,
		})
	}

	return map[string]interface{}{
		"total":    total,
		"page":     page,
		"pageSize": pageSize,
		"job":      job,
		"tag":      tag,
		"list":     targetList,
	}, nil
}

// UpdateTargetTags 更新单个目标的标签
func (c *consulService) UpdateTargetTags(tenantId string, targetId int64, labels map[string]interface{}) (interface{}, interface{}) {
	// 获取目标信息
	target, err := c.ctx.DB.Consul().GetTargetById(targetId)
	if err != nil {
		return nil, fmt.Errorf("获取目标信息失败: %w", err)
	}

	if target.ID == 0 {
		return nil, fmt.Errorf("目标不存在")
	}

	// 验证租户隔离
	if target.TenantId != tenantId {
		return nil, fmt.Errorf("无权限更新此目标")
	}

	// 更新数据库中的标签
	target.Labels = labels
	target.UpdatedAt = time.Now()
	err = c.ctx.DB.Consul().UpdateTarget(target)
	if err != nil {
		return nil, fmt.Errorf("更新目标标签失败: %w", err)
	}

	// 同步更新到 Consul（如果服务未注销且 ServiceID 存在）
	if !target.ConsulDeregistered && target.ServiceID != "" {
		c.syncTagsToConsul(tenantId, target.ServiceID, labels)
	}

	return map[string]interface{}{
		"id":        target.ID,
		"instance":  target.Instance,
		"labels":    labels,
		"updatedAt": target.UpdatedAt,
		"message":   "标签更新成功",
	}, nil
}

// syncTagsToConsul 将标签同步到 Consul（不返回错误，只记录警告）
func (c *consulService) syncTagsToConsul(tenantId, serviceID string, labels map[string]interface{}) {
	// 获取 Consul 配置
	config, err := c.getConsulConfigFromDataSource(tenantId)
	if err != nil {
		logc.Errorf(context.Background(), "无法获取 Consul 配置，标签已更新到数据库但未同步到 Consul: %v", err)
		return
	}

	// 创建 Consul 客户端
	consulConfig := consulclient.ClientConfig{
		Address: config.Address,
		Token:   config.Token,
	}
	client, err := consulclient.NewClient(consulConfig)
	if err != nil {
		logc.Errorf(context.Background(), "创建 Consul 客户端失败，标签已更新到数据库但未同步到 Consul: %v", err)
		return
	}

	// 从 Labels 中提取 Tags 和 Meta，并更新 Consul
	tags, meta := consulclient.ExtractTagsAndMetaFromLabels(labels)
	if err := client.UpdateServiceTagsAndMeta(context.Background(), serviceID, tags, meta); err != nil {
		logc.Errorf(context.Background(), "更新 Consul 服务标签失败，标签已更新到数据库但未同步到 Consul: %v", err)
	}
}

// GetOfflineLogs 获取注销历史记录列表，支持分页
// 自动过滤掉已重新上线的记录（过滤逻辑在 repo 层通过 JOIN 查询完成）
func (c *consulService) GetOfflineLogs(tenantId string, page, pageSize int) (interface{}, interface{}) {
	// 标准化分页参数
	page, pageSize = c.normalizePagination(page, pageSize)

	// 从数据库查询注销历史记录（repo 层已自动过滤掉已重新上线的记录）
	logs, total, err := c.ctx.DB.Consul().GetOfflineLogs(tenantId, page, pageSize)
	if err != nil {
		return nil, fmt.Errorf("获取注销历史记录失败: %w", err)
	}

	// 批量查询用户信息，将用户ID转换为用户名或真实姓名
	userIdsMap := make(map[string]bool)
	for _, log := range logs {
		if log.DeregisteredBy != "" {
			userIdsMap[log.DeregisteredBy] = true
		}
	}

	// 批量查询用户信息，将用户ID转换为真实姓名
	userIdToRealNameMap := make(map[string]string)
	if len(userIdsMap) > 0 {
		userIds := make([]string, 0, len(userIdsMap))
		for userId := range userIdsMap {
			userIds = append(userIds, userId)
		}

		var users []models.Member
		if err := c.ctx.DB.DB().Model(&models.Member{}).Where("user_id IN ?", userIds).Find(&users).Error; err == nil {
			for _, user := range users {
				// 注册用户时必须提供真实姓名，所以这里直接使用真实姓名
				if user.RealName != "" {
					userIdToRealNameMap[user.UserId] = user.RealName
				}
			}
		}
	}

	// 构建返回数据
	var logList []map[string]interface{}
	for _, log := range logs {
		// 显示真实姓名，如果没有则显示用户ID（用户不存在或已删除的情况）
		deregisteredByDisplay := log.DeregisteredBy
		if log.DeregisteredBy != "" {
			if realName, exists := userIdToRealNameMap[log.DeregisteredBy]; exists {
				deregisteredByDisplay = realName
			}
		}

		logList = append(logList, map[string]interface{}{
			"id":                 log.ID,
			"instance":           log.Instance,
			"job":                log.Job,
			"labels":             log.Labels,
			"reason":             log.Reason,
			"deregisteredBy":     deregisteredByDisplay, // 显示用户名或真实姓名
			"deregisteredById":   log.DeregisteredBy,    // 保留原始用户ID，用于重新上线时查找
			"alertEventsCleared": log.AlertEventsCleared,
			"createdAt":          log.CreatedAt,
		})
	}

	return map[string]interface{}{
		"total":    total,
		"page":     page,
		"pageSize": pageSize,
		"list":     logList,
	}, nil
}

// SyncTargetsCronjob 定时同步 Consul 目标的后台任务
// 参考 LDAP SyncUsersCronjob 模式，支持 context 取消
// 自动查询所有配置了 Consul 数据源的租户，按 SyncInterval 周期执行同步
func (c *consulService) SyncTargetsCronjob(ctx context.Context) {
	// 查询所有 Consul 数据源，获取租户列表和同步间隔
	dataSources, err := c.ctx.DB.Datasource().List("", "", "consul", "")
	if err != nil || len(dataSources) == 0 {
		logc.Infof(ctx, "未找到 Consul 数据源配置，跳过定时同步")
		return
	}

	// 收集需要同步的租户 ID（去重）
	tenantIds := make([]string, 0)
	seen := make(map[string]bool)
	for _, ds := range dataSources {
		if ds.TenantId != "" && !seen[ds.TenantId] {
			seen[ds.TenantId] = true
			tenantIds = append(tenantIds, ds.TenantId)
		}
	}

	// 取第一个数据源的 SyncInterval 作为全局同步间隔，默认 60 秒
	syncInterval := dataSources[0].ConsulConfig.SyncInterval
	if syncInterval < 10 {
		syncInterval = 60
	}
	if syncInterval > 3600 {
		syncInterval = 3600
	}

	// 将秒数转换为 cron 表达式: "@every Xs"
	cronSpec := fmt.Sprintf("@every %ds", syncInterval)
	logc.Infof(ctx, "Consul 目标定时同步已启动，间隔: %d 秒，租户数: %d", syncInterval, len(tenantIds))

	cronJob := cron.New()
	_, err = cronJob.AddFunc(cronSpec, func() {
		for _, tenantId := range tenantIds {
			if _, syncErr := c.SyncTargets(tenantId); syncErr != nil {
				logc.Errorf(ctx, "Consul 定时同步失败 (租户: %s): %v", tenantId, syncErr)
			}
		}
	})
	if err != nil {
		logc.Errorf(ctx, "创建 Consul 定时同步任务失败: %v", err)
		return
	}

	cronJob.Start()
	defer cronJob.Stop()

	// 阻塞等待 context 取消信号
	select {
	case <-ctx.Done():
		logc.Infof(ctx, "Consul 目标定时同步已停止")
		return
	}
}
