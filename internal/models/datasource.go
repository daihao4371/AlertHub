package models

type AlertDataSource struct {
	TenantId         string                 `json:"tenantId"`
	ID               string                 `json:"id"`
	Name             string                 `json:"name"`
	Labels           map[string]interface{} `json:"labels" gorm:"labels;serializer:json"` // 额外标签，会添加到事件Metric中，可用于区分数据来源；
	Type             string                 `json:"type"`
	HTTP             HTTP                   `json:"http" gorm:"http;serializer:json"`
	Auth             Auth                   `json:"Auth" gorm:"auth;serializer:json"`
	DsAliCloudConfig DsAliCloudConfig       `json:"dsAliCloudConfig" gorm:"dsAliCloudConfig;serializer:json"`
	AWSCloudWatch    AWSCloudWatch          `json:"awsCloudwatch" gorm:"awsCloudwatch;serializer:json"`
	ClickHouseConfig DsClickHouseConfig     `json:"clickhouseConfig" gorm:"clickhouseConfig;serializer:json"`
	ConsulConfig     DsConsulConfig         `json:"consulConfig" gorm:"consulConfig;serializer:json"` // Consul 服务发现配置
	Description      string                 `json:"description"`
	KubeConfig       string                 `json:"kubeConfig"`
	UpdateBy         string                 `json:"updateBy"`
	UpdateByRealName string                 `json:"updateByRealName" gorm:"-"`
	UpdateAt         int64                  `json:"updateAt"`
	Enabled          *bool                  `json:"enabled" `
}

type HTTP struct {
	URL     string `json:"url"`
	Timeout int64  `json:"timeout"`
}

type Auth struct {
	User string `json:"user"`
	Pass string `json:"pass"`
}

type DsClickHouseConfig struct {
	Addr    string
	Timeout int64
}

type DsAliCloudConfig struct {
	AliCloudEndpoint string `json:"alicloudEndpoint"`
	AliCloudAk       string `json:"alicloudAk"`
	AliCloudSk       string `json:"alicloudSk"`
}

type AWSCloudWatch struct {
	//Endpoint  string `json:"endpoint"`
	Region    string `json:"region"`
	AccessKey string `json:"accessKey"`
	SecretKey string `json:"secretKey"`
}

// DsConsulConfig Consul 数据源配置
type DsConsulConfig struct {
	Address      string `json:"address"`       // Consul 服务器地址（完整 URL，例：http://10.10.218.45:8500）
	Host         string `json:"host"`          // Consul 主机地址（兼容旧格式，如果没有 Address 则自动组合）
	Port         int    `json:"port"`          // Consul 端口（兼容旧格式，如果没有 Address 则自动组合）
	Token        string `json:"token"`         // Consul 认证令牌（可选）
	SyncInterval int    `json:"syncInterval"`  // 同步间隔（秒），范围 10-3600，默认 60
}

func (d *AlertDataSource) GetEnabled() *bool {
	if d.Enabled == nil {
		isOk := false
		return &isOk
	}
	return d.Enabled
}
