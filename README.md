


<p align="center">
  <b>🌐 WatchAlert —— 云原生环境下的轻量级智能监控告警引擎</b>
</p>


## 💎 WatchAlert 是什么？
🎯 **专注可观测性与稳定性，为运维提效降本**

WatchAlert 是一款专为云原生环境设计 的轻量级监控告警引擎，聚焦于可观测性（Metrics、Logs、Traces）与系统稳定性保障，提供从采集、分析到告警的全链路解决方案 。

🔍 **AI 智能加持，让告警更有“洞察力”**

通过 AI 技术深度分析 Metrics、Logs 和 Traces 中的异常信号，精准定位根因，智能生成排查建议与修复方案，显著提升故障响应效率。

![img.png](assets/architecture.png)

## 🧩 全面兼容主流可观测技术栈

|  监控类型   | 支持的数据源                                                                                    |
|:-------:|-------------------------------------------------------------------------------------------|
| Metrics | Prometheus、VictoriaMetrics                                                                |
|  Logs   | Loki、ElasticSearch、VictoriaLogs、ClickHouse、SLS（阿里云日志服务）、TLS（火山云日志服务，开发中）、CLS（腾讯云日志服务，开发中） |
| Traces  | Jaeger                                                                                    |
| Events  | Kubernetes 事件监控                                                                           |
|  网络拨测   | HTTP、ICMP、TCP、SSL                                                                         |
|  通知渠道   | 飞书、钉钉、企业微信、邮件、自定义 Webhook、Slack                                                           |


## 🔍 核心亮点

🧠 **AI 智能分析**

- 基于 AI 技术对告警内容进行深度语义解析，自动识别异常模式
- 提供根因推测、排查建议与修复思路，让每一次告警都“言之有物”

🕰️ **完善的值班机制**
- 支持轮班排班、节假日调整、值班交接等场景
- 告警通知精准匹配责任人，确保第一时间响应

⚡ **告警升级机制**
- 多级告警策略配置：从首次触发到升级通知，层层保障不漏报
- 支持超时重试、通知升级、负责人转接等功能，保障告警闭环处理

📊 **Namespace 级告警分类**
- 支持以命名空间（Namespace）为单位进行告警分组管理
- 清晰分类，快速定位，大幅提升故障处理效率

## 🚀 技术栈
- 后端环境要求
  - Go >= `1.23`

  - `Go`、`Gin`、`Viper` 、`Gorm`、`JWT`、`Go-zero`...

- 前端环境要求
  - Node.js >= `v18.20.3`
  - Yarn >= `1.22.22`
  - `React`、`JSX`、`Ant-design`、`Codemirror`...

## 🐳 Docker 镜像

### 官方镜像地址
- **后端服务**: `registry.cn-hangzhou.aliyuncs.com/devops-dh/watchalert:beta-v1`
- **前端服务**: `registry.cn-hangzhou.aliyuncs.com/devops-dh/watchalert-web:beta-v1`

> 💡 **提示**: 使用上述镜像可直接替换 Helm Chart 中的默认镜像,无需自行构建。

## 📦 部署方式

WatchAlert 提供多种部署方式,可根据实际环境灵活选择:

### 方式一: Helm Chart 标准版 (推荐新用户)
**适用场景**: 快速体验、测试环境、无现有 MySQL/Redis 的场景

```bash
# 使用标准版 Chart,自动部署 MySQL + Redis + WatchAlert 完整环境
cd deploy/helmchart
helm install watchalert . -n monitoring --create-namespace
```

**特点**:
- ✅ 一键部署,包含 MySQL 8.0、Redis 及 WatchAlert 全套服务
- ✅ 适合快速体验和测试环境
- ⚠️ 默认未启用持久化存储,生产环境需修改 `values.yaml` 配置

### 方式二: Helm Chart 定制版 (推荐生产环境)
**适用场景**: 已有 MySQL/Redis 基础设施,需对接现有数据库的场景

```bash
# 使用定制版 Chart,对接已有的 MySQL 和 Redis
tar -xzf deploy/watchalert-custom-1.0-beta.tar.gz
cd watchalert

# 编辑 values.yaml,配置现有 MySQL/Redis 连接信息
vim values.yaml

# 部署(关闭内置 MySQL/Redis)
helm install watchalert . -n monitoring \
  --set mysql.enabled=false \
  --set redis.enabled=false \
  --create-namespace
```

**特点**:
- ✅ 复用现有数据库,避免资源浪费
- ✅ 适合生产环境,数据持久化由现有基础设施保障
- ⚙️ 需手动配置数据库连接信息

### 镜像替换说明

如需使用官方最新镜像,可在 `values.yaml` 中修改:

```yaml
service:
  image:
    repository: registry.cn-hangzhou.aliyuncs.com/devops-dh/watchalert
    tag: "beta-v1"

web:
  image:
    repository: registry.cn-hangzhou.aliyuncs.com/devops-dh/watchalert-web
    tag: "beta-v1"
```

或通过 `--set` 参数动态指定:

```bash
helm install watchalert . -n monitoring \
  --set service.image.repository=registry.cn-hangzhou.aliyuncs.com/devops-dh/watchalert \
  --set service.image.tag=beta-v1 \
  --set web.image.repository=registry.cn-hangzhou.aliyuncs.com/devops-dh/watchalert-web \
  --set web.image.tag=beta-v1
```

## 📚 项目文档



## 🎉 项目预览


| ![Login](assets/login.png) | ![Home](assets/home.png)            |
|:--------------------------:|------------------------------|
|    ![rules](assets/rules.png)     | ![img.png](assets/faultcenter.png)  |
|   ![notice](assets/notice.png)    | ![duty](assets/duty.png)            |
|  ![probing](assets/probing.png)   | ![datasource](assets/datasource.png) |
|     ![user](assets/user.png)      | ![log](assets/log.png)              |

