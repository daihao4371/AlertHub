package types

// ========== PromQL 编辑器代理服务请求类型 ==========
// 用于前端 PromQL 编辑器调用后端代理接口
// 所有接口都需要指定 datasourceId 参数进行权限验证和路由

// PrometheusProxyLabelNamesRequest 获取标签名称列表请求
type PrometheusProxyLabelNamesRequest struct {
	DatasourceID string `json:"datasourceId" form:"datasourceId" binding:"required"` // 数据源 ID (必填)
	MetricName   string `json:"metricName" form:"metricName"`                         // 可选,限定指标范围
	Start        int64  `json:"start" form:"start"`                                   // 可选,查询起始时间 (Unix 时间戳)
	End          int64  `json:"end" form:"end"`                                       // 可选,查询结束时间 (Unix 时间戳)
}

// PrometheusProxyLabelValuesRequest 获取标签值列表请求
type PrometheusProxyLabelValuesRequest struct {
	DatasourceID string `json:"datasourceId" form:"datasourceId" binding:"required"` // 数据源 ID (必填)
	LabelName    string `json:"labelName" form:"labelName" binding:"required"`       // 标签名称 (必填,如 "job", "instance")
	MetricName   string `json:"metricName" form:"metricName"`                         // 可选,限定指标范围
	Start        int64  `json:"start" form:"start"`                                   // 可选,查询起始时间 (Unix 时间戳)
	End          int64  `json:"end" form:"end"`                                       // 可选,查询结束时间 (Unix 时间戳)
}

// PrometheusProxyMetricNamesRequest 获取指标名称列表请求
type PrometheusProxyMetricNamesRequest struct {
	DatasourceID string `json:"datasourceId" form:"datasourceId" binding:"required"` // 数据源 ID (必填)
}

// PrometheusProxySeriesRequest 获取时间序列元数据请求
type PrometheusProxySeriesRequest struct {
	DatasourceID string   `json:"datasourceId" binding:"required"` // 数据源 ID (必填)
	Matchers     []string `json:"match[]" binding:"required"`      // 匹配器数组 (必填,如 ["up", "node_cpu_seconds_total"])
	Start        int64    `json:"start"`                           // 可选,查询起始时间 (Unix 时间戳)
	End          int64    `json:"end"`                             // 可选,查询结束时间 (Unix 时间戳)
}