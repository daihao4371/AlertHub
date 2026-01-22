package types

import "alertHub/internal/models"

// RequestConsulTargetsQuery Consul 目标查询请求
type RequestConsulTargetsQuery struct {
	Job     string `json:"job" form:"job"`         // Job 名称过滤
	Status  string `json:"status" form:"status"`   // 状态过滤 (up/down/offline)
	Keyword string `json:"keyword" form:"keyword"` // 实例名称模糊搜索
	models.Page
}

// RequestRegisterTarget 注册服务到 Consul 的请求
type RequestRegisterTarget struct {
	ServiceID   string            `json:"serviceId" binding:"required"`   // 服务ID（必填，唯一标识）
	ServiceName string            `json:"serviceName" binding:"required"` // 服务名称（必填）
	Address     string            `json:"address" binding:"required"`     // 服务地址（必填）
	Port        int               `json:"port" binding:"required"`        // 服务端口（必填）
	Job         string            `json:"job"`                            // Job 名称（可选）
	Tags        []string          `json:"tags"`                           // 标签列表（可选）
	Labels      map[string]string `json:"labels"`                         // 标签键值对（可选，会转换为 Meta）
}
