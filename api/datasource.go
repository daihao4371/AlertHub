package api

import (
	ctx2 "alertHub/internal/ctx"
	"alertHub/internal/middleware"
	"alertHub/internal/models"
	"alertHub/internal/services"
	"alertHub/internal/types"
	"alertHub/pkg/provider"
	"alertHub/pkg/tools"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"

	"regexp"

	"github.com/gin-gonic/gin"
)

type datasourceController struct{}

var DatasourceController = new(datasourceController)

// parseVariablesFromQuery 从查询参数中解析变量
// 支持多种格式：
// 1. variables[instance]=value1&variables[ifName]=value2
// 2. variables=JSON字符串
// 3. 直接传递 instance=value1&ifName=value2 (兼容Grafana风格)
func parseVariablesFromQuery(ctx *gin.Context) map[string]string {
	variables := make(map[string]string)
	queryParams := ctx.Request.URL.Query()

	// 方式1: 从查询参数中解析 variables[key]=value 格式
	for key, values := range queryParams {
		if strings.HasPrefix(key, "variables[") && strings.HasSuffix(key, "]") {
			// 提取变量名，例如 variables[instance] -> instance
			varName := key[11 : len(key)-1] // 去掉 "variables[" 和 "]"
			if len(values) > 0 && values[0] != "" {
				variables[varName] = values[0]
			}
		}
	}

	// 方式2: 如果存在 variables JSON字符串参数，尝试解析
	if jsonStr := ctx.Query("variables"); jsonStr != "" {
		var jsonVars map[string]string
		if err := json.Unmarshal([]byte(jsonStr), &jsonVars); err == nil {
			for k, v := range jsonVars {
				variables[k] = v
			}
		}
	}

	// 方式3: 直接传递 instance 和 ifName 参数（兼容Grafana风格）
	// 如果查询语句中包含 $instance 或 $ifName，且参数中有对应的值，则使用
	if instance := ctx.Query("instance"); instance != "" {
		if _, exists := variables["instance"]; !exists {
			variables["instance"] = instance
		}
	}
	if ifName := ctx.Query("ifName"); ifName != "" {
		if _, exists := variables["ifName"]; !exists {
			variables["ifName"] = ifName
		}
	}

	return variables
}

// autoFillMissingVariables 自动填充缺失的变量
// 如果查询语句中包含 $instance 或 $ifName 但没有提供值，尝试从 Prometheus 获取
func autoFillMissingVariables(ctx *gin.Context, query string, datasourceId string, variables map[string]string) map[string]string {
	result := make(map[string]string)
	for k, v := range variables {
		result[k] = v
	}

	// 检查查询语句中是否包含 $instance 或 $ifName
	hasInstanceVar := strings.Contains(query, "$instance")
	hasIfNameVar := strings.Contains(query, "$ifName")

	// 如果查询包含变量但没有提供值，尝试从 Prometheus 获取
	if hasInstanceVar && result["instance"] == "" {
		if instance := tryGetLabelValue(ctx, datasourceId, "instance", query); instance != "" {
			result["instance"] = instance
		}
	}

	if hasIfNameVar && result["ifName"] == "" {
		if ifName := tryGetLabelValue(ctx, datasourceId, "ifName", query); ifName != "" {
			result["ifName"] = ifName
		}
	}

	return result
}

// tryGetLabelValue 尝试从 Prometheus 获取 label 的第一个可用值
// 通过查询包含该 label 的 metric 来获取值
func tryGetLabelValue(ctx *gin.Context, datasourceId, labelName, originalQuery string) string {
	source, err := ctx2.DO().DB.Datasource().Get(datasourceId)
	if err != nil {
		return ""
	}

	// 从原始查询中提取 metric 名称（例如：ifHCInMulticastPkts 或 ifInMulticastPkts）
	// 使用正则表达式匹配 metric 名称
	metricRe := regexp.MustCompile(`(ifHCIn\w+|ifIn\w+|ifOut\w+|ifHCOut\w+)`)
	matches := metricRe.FindStringSubmatch(originalQuery)
	if len(matches) == 0 {
		// 如果没有找到，尝试查询 up metric
		matches = []string{"up"}
	}

	metricName := matches[0]
	// 构建查询：查询该 metric 的所有时间序列，限制返回1个结果
	query := fmt.Sprintf("%s{%s=~\".+\"}", metricName, labelName)
	fullURL := fmt.Sprintf("%s/api/v1/query?query=%s&time=%d",
		source.HTTP.URL, url.QueryEscape(query), time.Now().Unix())

	get, err := tools.Get(tools.CreateBasicAuthHeader(source.Auth.User, source.Auth.Pass), fullURL, 5)
	if err != nil {
		return ""
	}
	defer get.Body.Close()

	if get.StatusCode != 200 {
		return ""
	}

	var res provider.QueryResponse
	if err := tools.ParseReaderBody(get.Body, &res); err != nil {
		return ""
	}

	if res.Status != "success" || len(res.VMData.VMResult) == 0 {
		return ""
	}

	// 从第一个结果的 metric 标签中提取值
	if len(res.VMData.VMResult) > 0 {
		metricMap := res.VMData.VMResult[0].Metric
		if value, exists := metricMap[labelName]; exists {
			if valueStr, ok := value.(string); ok {
				return valueStr
			}
		}
	}

	return ""
}

// replaceQueryVariables 替换查询语句中的变量
// 支持 $variable 格式的变量替换
// 例如: $instance -> variables["instance"] 的值
func replaceQueryVariables(query string, variables map[string]string) string {
	return tools.ReplacePromQLVariables(query, variables, false)
}

// injectInstanceFilter 向 PromQL 查询中注入 instance 标签过滤条件
// 用于限制查询只返回指定主机的数据，减少返回数据量
// 参数:
//   - query: 原始 PromQL 查询语句
//   - instances: 要过滤的主机列表
//
// 返回: 注入过滤条件后的查询语句
func injectInstanceFilter(query string, instances []string) string {
	if len(instances) == 0 {
		return query
	}

	// 构建 instance 正则匹配表达式: instance=~"host1|host2|host3"
	instanceRegex := strings.Join(instances, "|")
	instanceFilter := fmt.Sprintf(`instance=~"%s"`, instanceRegex)

	// 检查查询是否已包含大括号（标签选择器）
	// 情况1: metric_name{existing_labels} -> metric_name{existing_labels, instance=~"..."}
	// 情况2: metric_name -> metric_name{instance=~"..."}
	if strings.Contains(query, "{") {
		// 已有标签选择器，在最后一个 } 前插入新的过滤条件
		lastBraceIdx := strings.LastIndex(query, "}")
		if lastBraceIdx > 0 {
			// 检查大括号内是否已有内容
			firstBraceIdx := strings.LastIndex(query[:lastBraceIdx], "{")
			if firstBraceIdx >= 0 {
				insideBraces := strings.TrimSpace(query[firstBraceIdx+1 : lastBraceIdx])
				if insideBraces == "" {
					// 空大括号: metric{} -> metric{instance=~"..."}
					return query[:firstBraceIdx+1] + instanceFilter + query[lastBraceIdx:]
				}
				// 非空大括号: metric{label="value"} -> metric{label="value", instance=~"..."}
				return query[:lastBraceIdx] + ", " + instanceFilter + query[lastBraceIdx:]
			}
		}
	}

	// 没有标签选择器，需要找到指标名称的结束位置并添加
	// 处理可能的函数调用: rate(metric[5m]) -> rate(metric{instance=~"..."}[5m])
	// 简单情况: metric_name -> metric_name{instance=~"..."}
	metricEndPattern := regexp.MustCompile(`^([a-zA-Z_:][a-zA-Z0-9_:]*)`)
	if match := metricEndPattern.FindStringIndex(query); match != nil {
		metricEnd := match[1]
		return query[:metricEnd] + "{" + instanceFilter + "}" + query[metricEnd:]
	}

	// 如果无法识别格式，返回原查询
	return query
}

// applyPagination 对查询结果应用分页
// 在服务端对时间序列进行截取，减少返回给前端的数据量
// 参数:
//   - results: Prometheus 返回的时间序列列表
//   - limit: 最大返回数量，0 表示不限制
//   - offset: 跳过的数量
//
// 返回: (分页后的结果, 原始总数)
func applyPagination(results []provider.VMResult, limit, offset int) ([]provider.VMResult, int) {
	total := len(results)

	// 如果没有设置分页参数，返回全部结果
	if limit <= 0 && offset <= 0 {
		return results, total
	}

	// 应用 offset
	if offset >= total {
		return []provider.VMResult{}, total
	}
	results = results[offset:]

	// 应用 limit
	if limit > 0 && limit < len(results) {
		results = results[:limit]
	}

	return results, total
}

/*
数据源 API
/api/w8t/datasource
*/
func (datasourceController datasourceController) API(gin *gin.RouterGroup) {
	a := gin.Group("datasource")
	a.Use(
		middleware.Auth(),
		middleware.CasbinPermission(),
		middleware.ParseTenant(),
		middleware.AuditingLog(),
	)
	{
		a.POST("dataSourceCreate", datasourceController.Create)
		a.POST("dataSourceUpdate", datasourceController.Update)
		a.POST("dataSourceDelete", datasourceController.Delete)
	}

	b := gin.Group("datasource")
	b.Use(
		middleware.Auth(),
		middleware.CasbinPermission(),
		middleware.ParseTenant(),
	)
	{
		b.GET("dataSourceList", datasourceController.List)
		b.GET("dataSourceGet", datasourceController.Get)
	}

	c := gin.Group("datasource")
	c.Use(
		middleware.Auth(),
		middleware.ParseTenant(),
	)
	{
		c.GET("promQuery", datasourceController.PromQuery)
		c.GET("promQueryRange", datasourceController.PromQueryRange)
		c.GET("promLabelValues", datasourceController.PromLabelValues)
		c.POST("dataSourcePing", datasourceController.Ping)
		c.POST("searchViewLogsContent", datasourceController.SearchViewLogsContent)
	}

}

func (datasourceController datasourceController) Create(ctx *gin.Context) {
	r := new(types.RequestDatasourceCreate)
	BindJson(ctx, r)

	Service(ctx, func() (interface{}, interface{}) {
		userName := tools.GetUser(ctx.Request.Header.Get("Authorization"))
		r.UpdateBy = userName

		tid, _ := ctx.Get("TenantID")
		r.TenantId = tid.(string)

		return services.DatasourceService.Create(r)
	})
}

func (datasourceController datasourceController) List(ctx *gin.Context) {
	r := new(types.RequestDatasourceQuery)
	BindQuery(ctx, r)

	tid, _ := ctx.Get("TenantID")
	r.TenantId = tid.(string)

	Service(ctx, func() (interface{}, interface{}) {
		return services.DatasourceService.List(r)
	})
}

func (datasourceController datasourceController) Get(ctx *gin.Context) {
	r := new(types.RequestDatasourceQuery)
	BindQuery(ctx, r)

	tid, _ := ctx.Get("TenantID")
	r.TenantId = tid.(string)

	Service(ctx, func() (interface{}, interface{}) {
		return services.DatasourceService.Get(r)
	})
}

func (datasourceController datasourceController) Update(ctx *gin.Context) {
	r := new(types.RequestDatasourceUpdate)
	BindJson(ctx, r)

	Service(ctx, func() (interface{}, interface{}) {
		userName := tools.GetUser(ctx.Request.Header.Get("Authorization"))
		r.UpdateBy = userName

		tid, _ := ctx.Get("TenantID")
		r.TenantId = tid.(string)

		return services.DatasourceService.Update(r)
	})
}

func (datasourceController datasourceController) Delete(ctx *gin.Context) {
	r := new(types.RequestDatasourceQuery)
	BindJson(ctx, r)

	tid, _ := ctx.Get("TenantID")
	r.TenantId = tid.(string)

	Service(ctx, func() (interface{}, interface{}) {
		return services.DatasourceService.Delete(r)
	})
}

func (datasourceController datasourceController) PromQuery(ctx *gin.Context) {
	r := new(types.RequestQueryMetricsValue)
	BindQuery(ctx, r)

	// 手动解析变量参数（Gin的ShouldBindQuery不支持map类型）
	variables := parseVariablesFromQuery(ctx)

	Service(ctx, func() (interface{}, interface{}) {
		// 自动填充缺失的变量（如果查询包含变量但没有提供值）
		if len(variables) == 0 && (strings.Contains(r.Query, "$instance") || strings.Contains(r.Query, "$ifName")) {
			// 尝试从第一个数据源获取变量值
			if len(strings.Split(r.DatasourceIds, ",")) > 0 {
				firstDatasourceId := strings.Split(r.DatasourceIds, ",")[0]
				variables = autoFillMissingVariables(ctx, r.Query, firstDatasourceId, variables)
			}
		}

		// 替换查询语句中的变量
		query := replaceQueryVariables(r.Query, variables)

		// 性能优化：如果指定了 instances 参数，注入到查询中
		instanceList := r.GetInstanceList()
		if len(instanceList) > 0 {
			query = injectInstanceFilter(query, instanceList)
		}

		var ress []provider.QueryResponse
		path := "/api/v1/query"
		params := url.Values{}
		params.Add("query", query)
		params.Add("time", strconv.FormatInt(time.Now().Unix(), 10))

		var ids = []string{}
		ids = strings.Split(r.DatasourceIds, ",")
		for _, id := range ids {
			var res provider.QueryResponse
			source, err := ctx2.DO().DB.Datasource().Get(id)
			if err != nil {
				return nil, err
			}
			fullURL := fmt.Sprintf("%s%s?%s", source.HTTP.URL, path, params.Encode())

			get, err := tools.Get(tools.CreateBasicAuthHeader(source.Auth.User, source.Auth.Pass), fullURL, 10)
			if err != nil {
				return nil, fmt.Errorf("请求Prometheus失败: %w", err)
			}
			defer get.Body.Close()

			// 检查HTTP状态码
			if get.StatusCode != 200 {
				return nil, fmt.Errorf("Prometheus返回非200状态码: %d, URL: %s", get.StatusCode, fullURL)
			}

			if err := tools.ParseReaderBody(get.Body, &res); err != nil {
				return nil, fmt.Errorf("解析Prometheus响应失败: %w, URL: %s", err, fullURL)
			}

			// 检查Prometheus响应的status字段
			if res.Status != "success" {
				// Prometheus返回错误状态，即使HTTP状态码是200
				errorMsg := fmt.Sprintf("Prometheus查询返回错误状态: %s, Query: %s", res.Status, query)
				return nil, fmt.Errorf("%s, URL: %s", errorMsg, fullURL)
			}

			// 性能优化：应用服务端分页
			if r.HasPagination() {
				paginatedResults, total := applyPagination(res.VMData.VMResult, r.Limit, r.Offset)
				res.VMData.VMResult = paginatedResults
				// 将分页信息附加到响应中
				ress = append(ress, res)
				// 如果启用了分页，返回带分页元数据的响应
				if len(ids) == 1 {
					return types.PromQueryPaginatedResponse{
						Data:   ress,
						Total:  total,
						Limit:  r.Limit,
						Offset: r.Offset,
					}, nil
				}
			} else {
				ress = append(ress, res)
			}
		}

		// 如果启用了分页且有多个数据源，合并计算总数
		if r.HasPagination() && len(ids) > 1 {
			totalCount := 0
			for _, res := range ress {
				totalCount += len(res.VMData.VMResult)
			}
			return types.PromQueryPaginatedResponse{
				Data:   ress,
				Total:  totalCount,
				Limit:  r.Limit,
				Offset: r.Offset,
			}, nil
		}

		return ress, nil
	})
}

func (datasourceController datasourceController) PromQueryRange(ctx *gin.Context) {
	r := new(types.RequestQueryMetricsValue)
	BindQuery(ctx, r)

	// 手动解析变量参数（Gin的ShouldBindQuery不支持map类型）
	variables := parseVariablesFromQuery(ctx)

	Service(ctx, func() (interface{}, interface{}) {
		err := r.Validate()
		if err != nil {
			return nil, err
		}

		// 自动填充缺失的变量（如果查询包含变量但没有提供值）
		if len(variables) == 0 && (strings.Contains(r.Query, "$instance") || strings.Contains(r.Query, "$ifName")) {
			// 尝试从第一个数据源获取变量值
			if len(strings.Split(r.DatasourceIds, ",")) > 0 {
				firstDatasourceId := strings.Split(r.DatasourceIds, ",")[0]
				variables = autoFillMissingVariables(ctx, r.Query, firstDatasourceId, variables)
			}
		}

		// 替换查询语句中的变量
		query := replaceQueryVariables(r.Query, variables)

		// 性能优化：如果指定了 instances 参数，注入到查询中
		instanceList := r.GetInstanceList()
		if len(instanceList) > 0 {
			query = injectInstanceFilter(query, instanceList)
		}

		var ress []provider.QueryResponse
		path := "/api/v1/query_range"
		params := url.Values{}
		params.Add("query", query)
		params.Add("start", strconv.FormatInt(r.GetStartTime().Unix(), 10))
		params.Add("end", strconv.FormatInt(r.GetEndTime().Unix(), 10))
		params.Add("step", fmt.Sprintf("%.0fs", r.GetStep().Seconds()))

		var ids = []string{}
		ids = strings.Split(r.DatasourceIds, ",")

		for _, id := range ids {
			var res provider.QueryResponse
			source, err := ctx2.DO().DB.Datasource().Get(id)
			if err != nil {
				return nil, err
			}
			fullURL := fmt.Sprintf("%s%s?%s", source.HTTP.URL, path, params.Encode())

			get, err := tools.Get(tools.CreateBasicAuthHeader(source.Auth.User, source.Auth.Pass), fullURL, 10)
			if err != nil {
				return nil, fmt.Errorf("请求Prometheus失败: %w", err)
			}
			defer get.Body.Close()

			// 检查HTTP状态码
			if get.StatusCode != 200 {
				return nil, fmt.Errorf("Prometheus返回非200状态码: %d, URL: %s", get.StatusCode, fullURL)
			}

			if err := tools.ParseReaderBody(get.Body, &res); err != nil {
				return nil, fmt.Errorf("解析Prometheus响应失败: %w, URL: %s", err, fullURL)
			}

			// 检查Prometheus响应的status字段
			if res.Status != "success" {
				// Prometheus返回错误状态，即使HTTP状态码是200
				errorMsg := fmt.Sprintf("Prometheus查询返回错误状态: %s, Query: %s", res.Status, query)
				return nil, fmt.Errorf("%s, URL: %s", errorMsg, fullURL)
			}

			// 性能优化：应用服务端分页
			if r.HasPagination() {
				paginatedResults, total := applyPagination(res.VMData.VMResult, r.Limit, r.Offset)
				res.VMData.VMResult = paginatedResults
				ress = append(ress, res)
				// 如果启用了分页且只有单个数据源，返回带分页元数据的响应
				if len(ids) == 1 {
					return types.PromQueryPaginatedResponse{
						Data:   ress,
						Total:  total,
						Limit:  r.Limit,
						Offset: r.Offset,
					}, nil
				}
			} else {
				ress = append(ress, res)
			}
		}

		// 如果启用了分页且有多个数据源，合并计算总数
		if r.HasPagination() && len(ids) > 1 {
			totalCount := 0
			for _, res := range ress {
				totalCount += len(res.VMData.VMResult)
			}
			return types.PromQueryPaginatedResponse{
				Data:   ress,
				Total:  totalCount,
				Limit:  r.Limit,
				Offset: r.Offset,
			}, nil
		}

		return ress, nil
	})
}

// PromLabelValues 获取 Prometheus label 的所有可用值
// 用于前端生成下拉选择器
func (datasourceController datasourceController) PromLabelValues(ctx *gin.Context) {
	r := new(struct {
		DatasourceId string `form:"datasourceId"`
		LabelName    string `form:"labelName"`
		MetricName   string `form:"metricName"` // 可选的 metric 名称，用于过滤
	})
	BindQuery(ctx, r)

	Service(ctx, func() (interface{}, interface{}) {
		if r.DatasourceId == "" || r.LabelName == "" {
			return nil, fmt.Errorf("datasourceId 和 labelName 参数不能为空")
		}

		source, err := ctx2.DO().DB.Datasource().Get(r.DatasourceId)
		if err != nil {
			return nil, fmt.Errorf("获取数据源失败: %w", err)
		}

		// 构建查询：查询包含该 label 的所有时间序列
		var query string
		if r.MetricName != "" {
			// 如果提供了 metric 名称，查询该 metric 的所有时间序列
			query = fmt.Sprintf("%s{%s=~\".+\"}", r.MetricName, r.LabelName)
		} else {
			// 否则查询所有包含该 label 的时间序列（使用 up metric 作为基础）
			query = fmt.Sprintf("up{%s=~\".+\"}", r.LabelName)
		}

		fullURL := fmt.Sprintf("%s/api/v1/query?query=%s&time=%d",
			source.HTTP.URL, url.QueryEscape(query), time.Now().Unix())

		get, err := tools.Get(tools.CreateBasicAuthHeader(source.Auth.User, source.Auth.Pass), fullURL, 10)
		if err != nil {
			return nil, fmt.Errorf("请求Prometheus失败: %w", err)
		}
		defer get.Body.Close()

		if get.StatusCode != 200 {
			return nil, fmt.Errorf("Prometheus返回非200状态码: %d", get.StatusCode)
		}

		var res provider.QueryResponse
		if err := tools.ParseReaderBody(get.Body, &res); err != nil {
			return nil, fmt.Errorf("解析Prometheus响应失败: %w", err)
		}

		if res.Status != "success" {
			return nil, fmt.Errorf("prometheus查询返回错误状态: %s", res.Status)
		}

		// 提取所有唯一的 label 值
		values := make(map[string]bool)
		for _, result := range res.VMData.VMResult {
			if metricMap := result.Metric; metricMap != nil {
				if value, exists := metricMap[r.LabelName]; exists {
					if valueStr, ok := value.(string); ok && valueStr != "" {
						values[valueStr] = true
					}
				}
			}
		}

		// 转换为排序后的字符串数组
		valueList := make([]string, 0, len(values))
		for value := range values {
			valueList = append(valueList, value)
		}

		// 简单排序
		for i := 0; i < len(valueList)-1; i++ {
			for j := i + 1; j < len(valueList); j++ {
				if valueList[i] > valueList[j] {
					valueList[i], valueList[j] = valueList[j], valueList[i]
				}
			}
		}

		return valueList, nil
	})
}

func (datasourceController datasourceController) Ping(ctx *gin.Context) {
	r := new(types.RequestDatasourceCreate)
	BindJson(ctx, r)

	Service(ctx, func() (interface{}, interface{}) {
		ok, err := provider.CheckDatasourceHealth(models.AlertDataSource{
			TenantId:         r.TenantId,
			Name:             r.Name,
			Labels:           r.Labels,
			Type:             r.Type,
			HTTP:             r.HTTP,
			Auth:             r.Auth,
			DsAliCloudConfig: r.DsAliCloudConfig,
			AWSCloudWatch:    r.AWSCloudWatch,
			ClickHouseConfig: r.ClickHouseConfig,
			ConsulConfig:     r.ConsulConfig,
			Description:      r.Description,
			KubeConfig:       r.KubeConfig,
			Enabled:          r.Enabled,
		})
		if !ok {
			return "", fmt.Errorf("数据源不可达, err: %s", err.Error())
		}
		return "", nil
	})
}

// SearchViewLogsContent Logs 数据预览
func (datasourceController datasourceController) SearchViewLogsContent(ctx *gin.Context) {
	r := new(types.RequestSearchLogsContent)
	BindJson(ctx, r)

	Service(ctx, func() (interface{}, interface{}) {
		data, err := services.DatasourceService.Get(&types.RequestDatasourceQuery{ID: r.DatasourceId})
		if err != nil {
			return nil, err
		}

		datasource := data.(models.AlertDataSource)

		var (
			client  provider.LogsFactoryProvider
			options provider.LogQueryOptions
		)

		// 使用 base64.StdEncoding 进行解码
		decodedBytes, err := base64.StdEncoding.DecodeString(r.Query)
		if err != nil {
			return nil, fmt.Errorf("base64 解码失败: %s", err)
		}
		// 将解码后的字节转换为字符串
		QueryStr := string(decodedBytes)

		switch r.Type {
		case provider.VictoriaLogsDsProviderName:
			client, err = provider.NewVictoriaLogsClient(ctx, datasource)
			if err != nil {
				return nil, err
			}

			options = provider.LogQueryOptions{
				VictoriaLogs: provider.VictoriaLogs{
					Query: QueryStr,
				},
			}
		case provider.ElasticSearchDsProviderName:
			client, err = provider.NewElasticSearchClient(ctx, datasource)
			if err != nil {
				return nil, err
			}

			options = provider.LogQueryOptions{
				ElasticSearch: provider.Elasticsearch{
					Index:     r.GetElasticSearchIndexName(),
					QueryType: "RawJson",
					RawJson:   QueryStr,
				},
			}
		case provider.ClickHouseDsProviderName:
			client, err = provider.NewClickHouseClient(ctx, datasource)
			if err != nil {
				return nil, err
			}

			options = provider.LogQueryOptions{
				ClickHouse: provider.ClickHouse{
					Query: QueryStr,
				},
			}
		}

		query, _, err := client.Query(options)
		if err != nil {
			return nil, err
		}

		return query, nil
	})
}
