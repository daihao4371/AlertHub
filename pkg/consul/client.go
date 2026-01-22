package consul

import (
	"context"
	"fmt"
	"time"

	consulapi "github.com/hashicorp/consul/api"
)

// Client Consul 客户端封装
type Client struct {
	client *consulapi.Client
	config *ClientConfig
}

// ClientConfig Consul 客户端配置
type ClientConfig struct {
	Address string        // Consul 服务器地址（完整 URL，例：http://10.10.217.225:8500）
	Token   string        // 认证令牌（可选）
	Timeout time.Duration // 连接超时时间（默认：3s）
}

// NewClient 创建新的 Consul 客户端
func NewClient(config ClientConfig) (*Client, error) {
	// 设置超时的默认值
	if config.Timeout == 0 {
		config.Timeout = 3 * time.Second
	}

	// 创建 Consul 客户端配置
	consulConfig := consulapi.DefaultConfig()
	consulConfig.Address = config.Address

	// 如果提供了令牌，设置认证
	if config.Token != "" {
		consulConfig.Token = config.Token
	}

	// 创建客户端
	client, err := consulapi.NewClient(consulConfig)
	if err != nil {
		return nil, fmt.Errorf("创建 Consul 客户端失败: %w", err)
	}

	return &Client{
		client: client,
		config: &config,
	}, nil
}

// HealthCheck 健康检查，验证连接是否可用
func (c *Client) HealthCheck(ctx context.Context) error {
	// 调用 Consul agent 的自身健康检查
	_, err := c.client.Agent().Self()
	if err != nil {
		return fmt.Errorf("Consul 连接失败: %w", err)
	}

	return nil
}

// GetServices 获取所有已注册的服务
func (c *Client) GetServices(ctx context.Context) (map[string]*consulapi.AgentService, error) {
	services, err := c.client.Agent().Services()
	if err != nil {
		return nil, fmt.Errorf("获取 Consul 服务列表失败: %w", err)
	}

	return services, nil
}

// ServiceInstance 服务实例信息
type ServiceInstance struct {
	ServiceID   string
	ServiceName string
	Address     string
	Port        int
	Tags        []string
	Meta        map[string]string
}

// GetServiceInstances 获取指定服务的所有实例
func (c *Client) GetServiceInstances(ctx context.Context, serviceName string) ([]ServiceInstance, error) {
	// 使用 Health 接口获取健康的实例
	entries, _, err := c.client.Health().Service(serviceName, "", true, &consulapi.QueryOptions{})
	if err != nil {
		return nil, fmt.Errorf("获取服务 %s 的实例失败: %w", serviceName, err)
	}

	var instances []ServiceInstance
	for _, entry := range entries {
		instance := ServiceInstance{
			ServiceID:   entry.Service.ID,
			ServiceName: entry.Service.Service,
			Address:     entry.Service.Address,
			Port:        entry.Service.Port,
			Tags:        entry.Service.Tags,
			Meta:        entry.Service.Meta,
		}
		instances = append(instances, instance)
	}

	return instances, nil
}

// DeregisterService 注销指定的服务实例
func (c *Client) DeregisterService(ctx context.Context, serviceID string) error {
	err := c.client.Agent().ServiceDeregister(serviceID)
	if err != nil {
		return fmt.Errorf("从 Consul 注销服务 %s 失败: %w", serviceID, err)
	}

	return nil
}

// Close 关闭客户端连接
func (c *Client) Close() error {
	// Consul 客户端不需要显式关闭
	return nil
}

// GetInfo 获取 Consul 服务器信息 (用于测试连接)
func (c *Client) GetInfo(ctx context.Context) (map[string]interface{}, error) {
	// 简单地检查连接是否正常即可
	// 通过调用 Agent().Self() 来验证连接
	_, err := c.client.Agent().Self()
	if err != nil {
		return nil, fmt.Errorf("获取 Consul 服务器信息失败: %w", err)
	}

	info := make(map[string]interface{})
	info["status"] = "connected"
	info["timestamp"] = time.Now().Unix()

	return info, nil
}

// GetServicesByTag 按标签获取所有服务
func (c *Client) GetServicesByTag(ctx context.Context, tag string) (map[string]*consulapi.AgentService, error) {
	// 使用过滤器查询具有特定标签的服务
	filter := fmt.Sprintf("Tags contains %q", tag)
	services, err := c.client.Agent().ServicesWithFilter(filter)
	if err != nil {
		return nil, fmt.Errorf("按标签 %s 查询服务失败: %w", tag, err)
	}

	return services, nil
}

// GetAllServiceInstances 获取服务的所有实例，包括不健康的
func (c *Client) GetAllServiceInstances(ctx context.Context, serviceName string) ([]ServiceInstance, error) {
	// 获取所有实例，包括不健康的
	entries, _, err := c.client.Health().Service(serviceName, "", false, &consulapi.QueryOptions{})
	if err != nil {
		return nil, fmt.Errorf("获取服务 %s 的所有实例失败: %w", serviceName, err)
	}

	var instances []ServiceInstance
	for _, entry := range entries {
		instance := ServiceInstance{
			ServiceID:   entry.Service.ID,
			ServiceName: entry.Service.Service,
			Address:     entry.Service.Address,
			Port:        entry.Service.Port,
			Tags:        entry.Service.Tags,
			Meta:        entry.Service.Meta,
		}
		instances = append(instances, instance)
	}

	return instances, nil
}

// UpdateServiceTagsAndMeta 更新服务的标签和元数据
// 通过重新注册服务来更新标签和元数据
func (c *Client) UpdateServiceTagsAndMeta(ctx context.Context, serviceID string, tags []string, meta map[string]string) error {
	// 获取现有服务信息
	service, _, err := c.client.Agent().Service(serviceID, nil)
	if err != nil {
		return fmt.Errorf("获取服务信息失败: %w", err)
	}

	// 构建注册请求，保留原有信息，更新标签和元数据
	reg := &consulapi.AgentServiceRegistration{
		ID:      service.ID,
		Name:    service.Service,
		Address: service.Address,
		Port:    service.Port,
		Tags:    tags,
		Meta:    meta,
	}

	// 重新注册服务以更新标签和元数据
	err = c.client.Agent().ServiceRegister(reg)
	if err != nil {
		return fmt.Errorf("更新服务标签和元数据失败: %w", err)
	}

	return nil
}

// UpdateServiceTags 只更新服务的标签，保留元数据
func (c *Client) UpdateServiceTags(ctx context.Context, serviceID string, tags []string) error {
	// 获取现有服务信息
	service, _, err := c.client.Agent().Service(serviceID, nil)
	if err != nil {
		return fmt.Errorf("获取服务信息失败: %w", err)
	}

	// 构建注册请求，保留元数据，只更新标签
	reg := &consulapi.AgentServiceRegistration{
		ID:      service.ID,
		Name:    service.Service,
		Address: service.Address,
		Port:    service.Port,
		Tags:    tags,
		Meta:    service.Meta,
	}

	// 重新注册服务以更新标签
	err = c.client.Agent().ServiceRegister(reg)
	if err != nil {
		return fmt.Errorf("更新服务标签失败: %w", err)
	}

	return nil
}

// UpdateServiceMeta 只更新服务的元数据，保留标签
func (c *Client) UpdateServiceMeta(ctx context.Context, serviceID string, meta map[string]string) error {
	// 获取现有服务信息
	service, _, err := c.client.Agent().Service(serviceID, nil)
	if err != nil {
		return fmt.Errorf("获取服务信息失败: %w", err)
	}

	// 构建注册请求，保留标签，只更新元数据
	reg := &consulapi.AgentServiceRegistration{
		ID:      service.ID,
		Name:    service.Service,
		Address: service.Address,
		Port:    service.Port,
		Tags:    service.Tags,
		Meta:    meta,
	}

	// 重新注册服务以更新元数据
	err = c.client.Agent().ServiceRegister(reg)
	if err != nil {
		return fmt.Errorf("更新服务元数据失败: %w", err)
	}

	return nil
}

// RegisterService 注册新服务
func (c *Client) RegisterService(ctx context.Context, serviceID, serviceName, address string, port int, tags []string, meta map[string]string) error {
	// 构建服务注册请求
	reg := &consulapi.AgentServiceRegistration{
		ID:      serviceID,
		Name:    serviceName,
		Address: address,
		Port:    port,
		Tags:    tags,
		Meta:    meta,
	}

	// 注册服务
	err := c.client.Agent().ServiceRegister(reg)
	if err != nil {
		return fmt.Errorf("注册服务 %s (ID: %s) 失败: %w", serviceName, serviceID, err)
	}

	return nil
}

// GetServiceByID 按 ServiceID 获取单个服务实例
func (c *Client) GetServiceByID(ctx context.Context, serviceID string) (*ServiceInstance, error) {
	// 获取单个服务信息
	service, _, err := c.client.Agent().Service(serviceID, nil)
	if err != nil {
		return nil, fmt.Errorf("获取服务 %s 失败: %w", serviceID, err)
	}

	if service == nil {
		return nil, fmt.Errorf("服务 %s 不存在", serviceID)
	}

	instance := &ServiceInstance{
		ServiceID:   service.ID,
		ServiceName: service.Service,
		Address:     service.Address,
		Port:        service.Port,
		Tags:        service.Tags,
		Meta:        service.Meta,
	}

	return instance, nil
}

// FilterServiceInstancesByTag 从服务实例列表中按标签过滤
func (c *Client) FilterServiceInstancesByTag(instances []ServiceInstance, tag string) []ServiceInstance {
	var filtered []ServiceInstance

	for _, instance := range instances {
		// 检查实例是否包含该标签
		for _, t := range instance.Tags {
			if t == tag {
				filtered = append(filtered, instance)
				break
			}
		}
	}

	return filtered
}

// BuildLabelsFromTagsAndMeta 将 Consul 服务的 Tags 和 Meta 合并到 Labels map 中
// Tags 会作为数组存储在 labels["tags"] 中
// Meta 的键值对会直接合并到 labels 中
// 这个函数用于将 Consul 的原始数据格式转换为数据库存储格式
func BuildLabelsFromTagsAndMeta(tags []string, meta map[string]string) map[string]interface{} {
	labels := make(map[string]interface{})

	// 将 Tags 存储到 labels["tags"] 中
	if len(tags) > 0 {
		labels["tags"] = tags
	}

	// 将 Meta 的键值对合并到 labels 中
	if meta != nil && len(meta) > 0 {
		for key, value := range meta {
			labels[key] = value
		}
	}

	return labels
}

// ExtractTagsAndMetaFromLabels 从 Labels 中提取 Tags 和 Meta
// Tags 存储在 labels["tags"] 中（数组类型）
// Meta 是 labels 中除了 "tags" 之外的所有键值对
// 这个函数用于将数据库存储格式转换为 Consul 的原始数据格式
func ExtractTagsAndMetaFromLabels(labels map[string]interface{}) ([]string, map[string]string) {
	var tags []string
	meta := make(map[string]string)

	if labels == nil {
		return tags, meta
	}

	// 提取 Tags
	if tagsValue, ok := labels["tags"]; ok {
		if tagsArray, ok := tagsValue.([]interface{}); ok {
			for _, tag := range tagsArray {
				if tagStr, ok := tag.(string); ok {
					tags = append(tags, tagStr)
				}
			}
		}
	}

	// 提取 Meta（除了 "tags" 之外的所有键值对）
	for key, value := range labels {
		if key != "tags" {
			// 将值转换为字符串
			meta[key] = fmt.Sprintf("%v", value)
		}
	}

	return tags, meta
}
