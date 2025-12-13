package types

// ========== Metrics Explorer 请求类型 ==========
// 用于指标浏览器功能的API接口请求

// MetricsExplorerMetricsRequest 指标列表分页查询请求
type MetricsExplorerMetricsRequest struct {
	DatasourceID string `json:"datasourceId" form:"datasourceId" binding:"required"` // 数据源 ID (必填)
	Page         int    `json:"page" form:"page"`                                     // 页码,从1开始 (默认1)
	Size         int    `json:"size" form:"size"`                                     // 页大小 (默认20,最大100)
	Search       string `json:"search" form:"search"`                                 // 搜索关键词 (可选)
}

// MetricsExplorerMetricsResponse 指标列表分页响应
type MetricsExplorerMetricsResponse struct {
	Metrics []string `json:"metrics"` // 指标名称列表
	Total   int      `json:"total"`   // 总数
	Page    int      `json:"page"`    // 当前页码
	Size    int      `json:"size"`    // 页大小
}

// MetricsExplorerCategoriesRequest 指标分类请求
type MetricsExplorerCategoriesRequest struct {
	DatasourceID string `json:"datasourceId" form:"datasourceId" binding:"required"` // 数据源 ID (必填)
}

// MetricsCategory 指标分类
type MetricsCategory struct {
	Category string   `json:"category"` // 分类名称 (如 "node_", "http_", "go_")
	Count    int      `json:"count"`    // 该分类下指标数量
	Metrics  []string `json:"metrics"`  // 分类下的指标列表 (仅返回前几个作为示例)
}

// MetricsExplorerQueryRangeRequest 增强查询范围请求
type MetricsExplorerQueryRangeRequest struct {
	DatasourceID string `json:"datasourceId" binding:"required"` // 数据源 ID (必填)
	Query        string `json:"query" binding:"required"`        // PromQL 查询语句 (必填)
	Start        int64  `json:"start" binding:"required"`        // 查询起始时间 (Unix 时间戳,必填)
	End          int64  `json:"end" binding:"required"`          // 查询结束时间 (Unix 时间戳,必填)
	Step         string `json:"step"`                            // 查询步长 (可选,支持 "auto", "30s", "1m" 等)
	MaxPoints    int    `json:"maxPoints"`                       // 最大数据点数量 (可选,默认800)
}

// QueryMetadata 查询元数据
type QueryMetadata struct {
	EstimatedPoints     int    `json:"estimatedPoints"`     // 预估数据点数
	ActualPoints        int    `json:"actualPoints"`        // 实际返回数据点数
	Step                string `json:"step"`                // 使用的步长
	DownsamplingApplied bool   `json:"downsamplingApplied"` // 是否应用了下采样
}

// MetricsExplorerQueryRangeResponse 增强查询范围响应
type MetricsExplorerQueryRangeResponse struct {
	Status   string         `json:"status"`   // 响应状态 ("success" 或 "error")
	Data     interface{}    `json:"data"`     // 查询结果数据
	Metadata *QueryMetadata `json:"metadata"` // 查询元数据
}