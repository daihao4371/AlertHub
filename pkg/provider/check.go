package provider

import (
	"alertHub/internal/models"
	consulclient "alertHub/pkg/consul"
	"context"
	"fmt"

	"github.com/zeromicro/go-zero/core/logc"
)

// HealthChecker 统一健康检查接口
type HealthChecker interface {
	Check() (bool, error)
}

// ClientFactory 客户端工厂函数类型
type ClientFactory func(models.AlertDataSource) (HealthChecker, error)

// 注册所有数据源类型的工厂方法
var datasourceFactories = map[string]ClientFactory{
	"Prometheus": func(ds models.AlertDataSource) (HealthChecker, error) {
		return NewPrometheusClient(ds)
	},
	"VictoriaMetrics": func(ds models.AlertDataSource) (HealthChecker, error) {
		return NewVictoriaMetricsClient(ds)
	},
	"Kubernetes": func(ds models.AlertDataSource) (HealthChecker, error) {
		return NewKubernetesClient(context.Background(), ds.KubeConfig, ds.Labels)
	},
	"ElasticSearch": func(ds models.AlertDataSource) (HealthChecker, error) {
		return NewElasticSearchClient(context.Background(), ds)
	},
	"AliCloudSLS": func(ds models.AlertDataSource) (HealthChecker, error) {
		return NewAliCloudSlsClient(ds)
	},
	"Loki": func(ds models.AlertDataSource) (HealthChecker, error) {
		return NewLokiClient(ds)
	},
	"Jaeger": func(ds models.AlertDataSource) (HealthChecker, error) {
		return NewJaegerClient(ds)
	},
	"CloudWatch": func(ds models.AlertDataSource) (HealthChecker, error) {
		return &CloudWatchDummyChecker{}, nil
	},
	"VictoriaLogs": func(ds models.AlertDataSource) (HealthChecker, error) {
		return NewVictoriaLogsClient(context.Background(), ds)
	},
	"ClickHouse": func(ds models.AlertDataSource) (HealthChecker, error) {
		return NewClickHouseClient(context.Background(), ds)
	},
	"consul": func(ds models.AlertDataSource) (HealthChecker, error) {
		return NewConsulHealthChecker(ds)
	},
}

// CloudWatchDummyChecker 云监控哑检查器
type CloudWatchDummyChecker struct{}

func (c *CloudWatchDummyChecker) Check() (bool, error) {
	return true, nil
}

// ConsulHealthChecker Consul 健康检查器
type ConsulHealthChecker struct {
	client *consulclient.Client
}

// NewConsulHealthChecker 创建 Consul 健康检查器
func NewConsulHealthChecker(ds models.AlertDataSource) (*ConsulHealthChecker, error) {
	// 获取 Consul 配置
	consulConfig := ds.ConsulConfig

	// 验证并获取 Address
	if consulConfig.Address == "" {
		return nil, fmt.Errorf("Consul 地址不能为空，请检查配置中的 address 字段")
	}

	address := consulConfig.Address

	// 使用组合后的地址创建 Consul 客户端
	config := consulclient.ClientConfig{
		Address: address,
		Token:   consulConfig.Token,
	}

	client, err := consulclient.NewClient(config)
	if err != nil {
		return nil, fmt.Errorf("创建 Consul 客户端失败: %w", err)
	}

	return &ConsulHealthChecker{
		client: client,
	}, nil
}

// Check 执行 Consul 健康检查
func (c *ConsulHealthChecker) Check() (bool, error) {
	err := c.client.HealthCheck(context.Background())
	if err != nil {
		return false, err
	}
	return true, nil
}

// CheckDatasourceHealth 统一健康检查入口
func CheckDatasourceHealth(datasource models.AlertDataSource) (bool, error) {
	// 获取对应的工厂方法
	factory, ok := datasourceFactories[datasource.Type]
	if !ok {
		err := fmt.Errorf("unsupported datasource type: %s", datasource.Type)
		logDatasourceError(datasource, err)
		return false, err
	}

	// 创建客户端
	client, err := factory(datasource)
	if err != nil {
		logDatasourceError(datasource, fmt.Errorf("client creation failed: %w", err))
		return false, err
	}

	// 执行健康检查
	healthy, err := client.Check()
	if err != nil || !healthy {
		logDatasourceError(datasource, fmt.Errorf("health check failed: %w", err))
		return false, err
	}

	return true, nil
}

// 统一日志记录方法
func logDatasourceError(ds models.AlertDataSource, err error) {
	logc.Errorf(context.Background(), "Datasource error",
		map[string]interface{}{
			"id":   ds.ID,
			"name": ds.Name,
			"type": ds.Type,
			"err":  err.Error(),
		})
}
