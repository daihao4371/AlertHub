


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

### 登录与概览
<table>
  <tr>
    <td align="center" width="50%"><img src="assets/登录页面.png" alt="登录页面" width="100%"/><br/><b>登录页面</b></td>
    <td align="center" width="50%"><img src="assets/概览.png" alt="系统概览" width="100%"/><br/><b>系统概览</b></td>
  </tr>
  <tr>
    <td align="center"><img src="assets/概览1.png" alt="概览仪表盘" width="100%"/><br/><b>概览仪表盘</b></td>
    <td align="center"><img src="assets/仪表盘.png" alt="数据仪表盘" width="100%"/><br/><b>数据仪表盘</b></td>
  </tr>
</table>

### 告警规则管理
<table>
  <tr>
    <td align="center"><img src="assets/告警规则管理.png" alt="告警规则管理"/><br/><b>告警规则管理</b></td>
    <td align="center"><img src="assets/编辑告警规则.png" alt="编辑告警规则"/><br/><b>编辑告警规则</b></td>
  </tr>
  <tr>
    <td align="center"><img src="assets/编辑告警规则的数据预览.png" alt="数据预览"/><br/><b>数据预览</b></td>
    <td align="center"><img src="assets/规则模板.png" alt="规则模板"/><br/><b>规则模板</b></td>
  </tr>
  <tr>
    <td align="center"><img src="assets/告警订阅.png" alt="告警订阅"/><br/><b>告警订阅</b></td>
    <td align="center"><img src="assets/故障中心.png" alt="故障中心"/><br/><b>故障中心</b></td>
  </tr>
  <tr>
    <td colspan="2" align="center"><img src="assets/故障详情.png" alt="故障详情" width="50%"/><br/><b>故障详情</b></td>
  </tr>
</table>

### 通知管理
<table>
  <tr>
    <td align="center"><img src="assets/通知对象.png" alt="通知对象"/><br/><b>通知对象</b></td>
    <td align="center"><img src="assets/通知模板.png" alt="通知模板"/><br/><b>通知模板</b></td>
  </tr>
  <tr>
    <td align="center"><img src="assets/通知记录.png" alt="通知记录"/><br/><b>通知记录</b></td>
    <td align="center"><img src="assets/值班中心.png" alt="值班中心"/><br/><b>值班中心</b></td>
  </tr>
  <tr>
    <td align="center"><img src="assets/钉钉消息通知.png" alt="钉钉消息通知"/><br/><b>钉钉消息通知</b></td>
    <td align="center"><img src="assets/飞书通知.png" alt="飞书通知"/><br/><b>飞书通知</b></td>
  </tr>
</table>

### 拨测与巡检
<table>
  <tr>
    <td align="center"><img src="assets/拨测任务.png" alt="拨测任务"/><br/><b>拨测任务</b></td>
    <td align="center"><img src="assets/及时拨测.png" alt="及时拨测"/><br/><b>及时拨测</b></td>
  </tr>
  <tr>
    <td colspan="2" align="center"><img src="assets/Exporter巡检.png" alt="Exporter巡检" width="50%"/><br/><b>Exporter 巡检</b></td>
  </tr>
</table>

### 系统管理
<table>
  <tr>
    <td align="center"><img src="assets/数据源.png" alt="数据源管理"/><br/><b>数据源管理</b></td>
    <td align="center"><img src="assets/用户管理.png" alt="用户管理"/><br/><b>用户管理</b></td>
  </tr>
  <tr>
    <td align="center"><img src="assets/角色管理.png" alt="角色管理"/><br/><b>角色管理</b></td>
    <td align="center"><img src="assets/租户管理.png" alt="租户管理"/><br/><b>租户管理</b></td>
  </tr>
  <tr>
    <td align="center"><img src="assets/系统设置.png" alt="系统设置"/><br/><b>系统设置</b></td>
    <td align="center"><img src="assets/审计日志.png" alt="审计日志"/><br/><b>审计日志</b></td>
  </tr>
</table>

