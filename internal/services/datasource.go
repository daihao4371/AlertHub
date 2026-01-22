package services

import (
	"alertHub/internal/ctx"
	"alertHub/internal/models"
	"alertHub/internal/types"
	"alertHub/pkg/provider"
	"alertHub/pkg/tools"
	"fmt"
	"time"
)

type datasourceService struct {
	ctx *ctx.Context
}

type InterDatasourceService interface {
	Create(req interface{}) (interface{}, interface{})
	Update(req interface{}) (interface{}, interface{})
	Delete(req interface{}) (interface{}, interface{})
	List(req interface{}) (interface{}, interface{})
	Get(req interface{}) (interface{}, interface{})
	WithAddClientToProviderPools(datasource models.AlertDataSource) error
	WithRemoveClientForProviderPools(datasourceId string)
}

func newInterDatasourceService(ctx *ctx.Context) InterDatasourceService {
	return &datasourceService{
		ctx: ctx,
	}
}

// normalizeConsulConfig 标准化 Consul 配置，兼容旧的 host:port 格式和新的 address 格式
func normalizeConsulConfig(config models.DsConsulConfig) models.DsConsulConfig {
	// 如果已有完整的 Address，直接返回
	if config.Address != "" {
		return config
	}

	// 如果 Address 为空但有 Host 和 Port（兼容旧的格式），自动组合成 Address
	if config.Host != "" && config.Port > 0 {
		config.Address = fmt.Sprintf("%s:%d", config.Host, config.Port)
		return config
	}

	// 如果 Address 和 Host 都为空，返回原配置（会由健康检查器捕获错误）
	return config
}

func (ds datasourceService) Create(req interface{}) (interface{}, interface{}) {
	dataSource := req.(*types.RequestDatasourceCreate)

	// 标准化 Consul 配置
	consulConfig := normalizeConsulConfig(dataSource.ConsulConfig)

	data := models.AlertDataSource{
		TenantId:         dataSource.TenantId,
		ID:               "ds-" + tools.RandId(),
		Name:             dataSource.Name,
		Labels:           dataSource.Labels,
		Type:             dataSource.Type,
		HTTP:             dataSource.HTTP,
		Auth:             dataSource.Auth,
		DsAliCloudConfig: dataSource.DsAliCloudConfig,
		AWSCloudWatch:    dataSource.AWSCloudWatch,
		ClickHouseConfig: dataSource.ClickHouseConfig,
		ConsulConfig:     consulConfig,
		Description:      dataSource.Description,
		KubeConfig:       dataSource.KubeConfig,
		UpdateBy:         dataSource.UpdateBy,
		UpdateAt:         time.Now().Unix(),
		Enabled:          dataSource.Enabled,
	}

	err := ds.ctx.DB.Datasource().Create(data)
	if err != nil {
		return nil, err
	}

	err = ds.WithAddClientToProviderPools(data)
	if err != nil {
		return nil, err
	}

	return nil, nil
}

func (ds datasourceService) Update(req interface{}) (interface{}, interface{}) {
	dataSource := req.(*types.RequestDatasourceUpdate)

	// 标准化 Consul 配置
	consulConfig := normalizeConsulConfig(dataSource.ConsulConfig)

	data := models.AlertDataSource{
		TenantId:         dataSource.TenantId,
		ID:               dataSource.ID,
		Name:             dataSource.Name,
		Labels:           dataSource.Labels,
		Type:             dataSource.Type,
		HTTP:             dataSource.HTTP,
		Auth:             dataSource.Auth,
		DsAliCloudConfig: dataSource.DsAliCloudConfig,
		AWSCloudWatch:    dataSource.AWSCloudWatch,
		ClickHouseConfig: dataSource.ClickHouseConfig,
		ConsulConfig:     consulConfig,
		Description:      dataSource.Description,
		KubeConfig:       dataSource.KubeConfig,
		UpdateBy:         dataSource.UpdateBy,
		UpdateAt:         time.Now().Unix(),
		Enabled:          dataSource.Enabled,
	}

	err := ds.ctx.DB.Datasource().Update(data)
	if err != nil {
		return nil, err
	}

	err = ds.WithAddClientToProviderPools(data)
	if err != nil {
		return nil, err
	}

	return nil, nil
}

func (ds datasourceService) Delete(req interface{}) (interface{}, interface{}) {
	dataSource := req.(*types.RequestDatasourceQuery)
	err := ds.ctx.DB.Datasource().Delete(dataSource.TenantId, dataSource.ID)
	if err != nil {
		return nil, err
	}

	ds.WithRemoveClientForProviderPools(dataSource.ID)

	return nil, nil
}

func (ds datasourceService) Get(req interface{}) (interface{}, interface{}) {
	dataSource := req.(*types.RequestDatasourceQuery)
	data, err := ds.ctx.DB.Datasource().Get(dataSource.ID)
	if err != nil {
		return nil, err
	}

	return data, nil
}

func (ds datasourceService) List(req interface{}) (interface{}, interface{}) {
	var newData []models.AlertDataSource
	dataSource := req.(*types.RequestDatasourceQuery)
	data, err := ds.ctx.DB.Datasource().List(dataSource.TenantId, dataSource.ID, dataSource.Type, dataSource.Query)
	if err != nil {
		return nil, err
	}
	newData = data

	return newData, nil
}

func (ds datasourceService) WithAddClientToProviderPools(datasource models.AlertDataSource) error {
	var (
		cli interface{}
		err error
	)
	pools := ds.ctx.Redis.ProviderPools()
	switch datasource.Type {
	case provider.PrometheusDsProvider:
		cli, err = provider.NewPrometheusClient(datasource)
	case provider.VictoriaMetricsDsProvider:
		cli, err = provider.NewVictoriaMetricsClient(datasource)
	case provider.LokiDsProviderName:
		cli, err = provider.NewLokiClient(datasource)
	case provider.AliCloudSLSDsProviderName:
		cli, err = provider.NewAliCloudSlsClient(datasource)
	case provider.ElasticSearchDsProviderName:
		cli, err = provider.NewElasticSearchClient(ctx.Ctx, datasource)
	case provider.VictoriaLogsDsProviderName:
		cli, err = provider.NewVictoriaLogsClient(ctx.Ctx, datasource)
	case provider.JaegerDsProviderName:
		cli, err = provider.NewJaegerClient(datasource)
	case "Kubernetes":
		cli, err = provider.NewKubernetesClient(ds.ctx.Ctx, datasource.KubeConfig, datasource.Labels)
	case "CloudWatch":
		cli, err = provider.NewAWSCredentialCfg(datasource.AWSCloudWatch.Region, datasource.AWSCloudWatch.AccessKey, datasource.AWSCloudWatch.SecretKey, datasource.Labels)
	case "ClickHouse":
		cli, err = provider.NewClickHouseClient(ctx.Ctx, datasource)
	}

	if err != nil {
		return fmt.Errorf("New %s client failed, err: %s", datasource.Type, err.Error())
	}

	pools.SetClient(datasource.ID, cli)
	return nil
}

func (ds datasourceService) WithRemoveClientForProviderPools(datasourceId string) {
	pools := ds.ctx.Redis.ProviderPools()
	pools.RemoveClient(datasourceId)
}
