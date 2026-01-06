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

			ress = append(ress, res)
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

			ress = append(ress, res)
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
