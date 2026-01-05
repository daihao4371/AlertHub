package analysis

import (
	"alertHub/internal/ctx"
	"alertHub/pkg/analysis/ai"
	"alertHub/pkg/analysis/collector"
	"alertHub/pkg/analysis/standardizer"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/zeromicro/go-zero/core/logc"
	"gopkg.in/yaml.v3"
)

// ConfigManager 完全配置驱动的配置管理器
type ConfigManager struct {
	// 配置存储
	configs     map[string]*UniversalConfig
	configPaths map[string]string

	// 配置工厂
	strategyFactory *StrategyFactory

	// 验证引擎
	validationEngine *ConfigValidationEngine

	// 配置监控
	configWatcher *ConfigWatcher

	// 并发控制
	mutex sync.RWMutex

	// 配置缓存
	configCache  map[string]*CachedConfig
	cacheTimeout time.Duration
}

// UniversalConfig 通用配置结构
type UniversalConfig struct {
	// 配置元数据
	Metadata ConfigMetadata `json:"metadata" yaml:"metadata"`

	// 核心组件配置
	DataCollection   *collector.CollectorConfig       `json:"dataCollection,omitempty" yaml:"dataCollection,omitempty"`
	DataStandardizer *standardizer.StandardizerConfig `json:"dataStandardizer,omitempty" yaml:"dataStandardizer,omitempty"`
	AIEngine         *ai.AIEngineConfig               `json:"aiEngine,omitempty" yaml:"aiEngine,omitempty"`
	ResultProcessor  *ResultProcessorConfig           `json:"resultProcessor,omitempty" yaml:"resultProcessor,omitempty"`

	// 动态策略配置
	Strategies map[string]*StrategyConfig `json:"strategies,omitempty" yaml:"strategies,omitempty"`

	// 扩展配置
	Extensions map[string]interface{} `json:"extensions,omitempty" yaml:"extensions,omitempty"`

	// 环境特定配置
	Environments map[string]*EnvironmentConfig `json:"environments,omitempty" yaml:"environments,omitempty"`
}

// ConfigMetadata 配置元数据
type ConfigMetadata struct {
	Name         string            `json:"name" yaml:"name"`
	Version      string            `json:"version" yaml:"version"`
	Description  string            `json:"description" yaml:"description"`
	Author       string            `json:"author" yaml:"author"`
	CreatedAt    time.Time         `json:"createdAt" yaml:"createdAt"`
	UpdatedAt    time.Time         `json:"updatedAt" yaml:"updatedAt"`
	Tags         []string          `json:"tags,omitempty" yaml:"tags,omitempty"`
	Dependencies []string          `json:"dependencies,omitempty" yaml:"dependencies,omitempty"`
	Custom       map[string]string `json:"custom,omitempty" yaml:"custom,omitempty"`
}

// StrategyConfig 策略配置
type StrategyConfig struct {
	Name       string                 `json:"name" yaml:"name"`
	Type       string                 `json:"type" yaml:"type"`
	Enabled    bool                   `json:"enabled" yaml:"enabled"`
	Priority   int                    `json:"priority" yaml:"priority"`
	Conditions []ConditionConfig      `json:"conditions,omitempty" yaml:"conditions,omitempty"`
	Parameters map[string]interface{} `json:"parameters,omitempty" yaml:"parameters,omitempty"`
	Metadata   map[string]interface{} `json:"metadata,omitempty" yaml:"metadata,omitempty"`
}

// ConditionConfig 条件配置
type ConditionConfig struct {
	Field    string      `json:"field" yaml:"field"`
	Operator string      `json:"operator" yaml:"operator"`
	Value    interface{} `json:"value" yaml:"value"`
	Logic    string      `json:"logic,omitempty" yaml:"logic,omitempty"` // and, or, not
}

// EnvironmentConfig 环境配置
type EnvironmentConfig struct {
	Name      string                 `json:"name" yaml:"name"`
	Active    bool                   `json:"active" yaml:"active"`
	Overrides map[string]interface{} `json:"overrides,omitempty" yaml:"overrides,omitempty"`
	Resources map[string]interface{} `json:"resources,omitempty" yaml:"resources,omitempty"`
	Security  map[string]interface{} `json:"security,omitempty" yaml:"security,omitempty"`
}

// CachedConfig 缓存的配置
type CachedConfig struct {
	Config   *UniversalConfig
	LoadedAt time.Time
	Hash     string
	Used     int64
}

// StrategyFactory 策略工厂
type StrategyFactory struct {
	registeredStrategies map[string]StrategyBuilder
	mutex                sync.RWMutex
}

// StrategyBuilder 策略构建器接口
type StrategyBuilder interface {
	Build(config map[string]interface{}) (Strategy, error)
	ValidateConfig(config map[string]interface{}) error
	GetMetadata() StrategyMetadata
}

// Strategy 策略接口
type Strategy interface {
	Execute(context *ExecutionContext) (interface{}, error)
	Name() string
	Type() string
	IsEnabled() bool
	GetPriority() int
}

// StrategyMetadata 策略元数据
type StrategyMetadata struct {
	Name         string   `json:"name"`
	Type         string   `json:"type"`
	Description  string   `json:"description"`
	Version      string   `json:"version"`
	Author       string   `json:"author"`
	Dependencies []string `json:"dependencies,omitempty"`
	Tags         []string `json:"tags,omitempty"`
}

// ExecutionContext 执行上下文
type ExecutionContext struct {
	RequestID   string
	Environment string
	UserContext *ctx.Context
	Data        map[string]interface{}
	Metadata    map[string]interface{}
}

// ConfigValidationEngine 配置验证引擎
type ConfigValidationEngine struct {
	validators map[string]ConfigValidator
	rules      map[string]*ValidationRule
}

// ConfigValidator 配置验证器接口
type ConfigValidator interface {
	Validate(config interface{}) error
	GetValidationRules() []ValidationRule
}

// ValidationRule 验证规则
type ValidationRule struct {
	Name       string                 `json:"name"`
	Type       string                 `json:"type"` // required, format, range, custom
	Field      string                 `json:"field"`
	Parameters map[string]interface{} `json:"parameters,omitempty"`
	Message    string                 `json:"message"`
	Severity   string                 `json:"severity"` // error, warning, info
}

// ConfigWatcher 配置监控器
type ConfigWatcher struct {
	watchPaths   map[string]bool
	callbacks    map[string]func(string, *UniversalConfig)
	stopChannels map[string]chan bool
	mutex        sync.RWMutex
}

// NewConfigManager 创建配置管理器
func NewConfigManager() *ConfigManager {
	return &ConfigManager{
		configs:          make(map[string]*UniversalConfig),
		configPaths:      make(map[string]string),
		strategyFactory:  NewStrategyFactory(),
		validationEngine: NewConfigValidationEngine(),
		configWatcher:    NewConfigWatcher(),
		configCache:      make(map[string]*CachedConfig),
		cacheTimeout:     5 * time.Minute,
	}
}

// LoadConfig 加载配置
func (cm *ConfigManager) LoadConfig(
	ctx *ctx.Context,
	configName string,
	configPath string,
) error {
	cm.mutex.Lock()
	defer cm.mutex.Unlock()

	logc.Infof(ctx.Ctx, "[配置管理] 开始加载配置: name=%s, path=%s", configName, configPath)

	// 1. 读取配置文件
	configData, err := os.ReadFile(configPath)
	if err != nil {
		return fmt.Errorf("读取配置文件失败: %w", err)
	}

	// 2. 解析配置文件
	var config UniversalConfig
	ext := filepath.Ext(configPath)
	switch ext {
	case ".json":
		err = json.Unmarshal(configData, &config)
	case ".yaml", ".yml":
		err = yaml.Unmarshal(configData, &config)
	default:
		return fmt.Errorf("不支持的配置文件格式: %s", ext)
	}

	if err != nil {
		return fmt.Errorf("解析配置文件失败: %w", err)
	}

	// 3. 验证配置
	if err := cm.validationEngine.ValidateConfig(&config); err != nil {
		return fmt.Errorf("配置验证失败: %w", err)
	}

	// 4. 处理环境覆盖
	if err := cm.applyEnvironmentOverrides(&config); err != nil {
		logc.Errorf(ctx.Ctx, "[配置管理] 环境覆盖处理失败: %v", err)
	}

	// 5. 构建策略
	if err := cm.buildStrategiesFromConfig(&config); err != nil {
		return fmt.Errorf("构建策略失败: %w", err)
	}

	// 6. 更新配置
	config.Metadata.UpdatedAt = time.Now()
	cm.configs[configName] = &config
	cm.configPaths[configName] = configPath

	// 7. 更新缓存
	cm.updateCache(configName, &config)

	// 8. 启动配置监控
	if err := cm.configWatcher.WatchConfig(configPath, func(path string, newConfig *UniversalConfig) {
		cm.onConfigChanged(ctx, configName, newConfig)
	}); err != nil {
		logc.Errorf(ctx.Ctx, "[配置管理] 启动配置监控失败: %v", err)
	}

	logc.Infof(ctx.Ctx, "[配置管理] 配置加载完成: name=%s", configName)
	return nil
}

// GetConfig 获取配置
func (cm *ConfigManager) GetConfig(configName string) (*UniversalConfig, error) {
	cm.mutex.RLock()
	defer cm.mutex.RUnlock()

	// 先查缓存
	if cached, exists := cm.configCache[configName]; exists {
		if time.Since(cached.LoadedAt) < cm.cacheTimeout {
			cached.Used++
			return cached.Config, nil
		}
		// 缓存过期，删除
		delete(cm.configCache, configName)
	}

	// 查找配置
	config, exists := cm.configs[configName]
	if !exists {
		return nil, fmt.Errorf("配置不存在: %s", configName)
	}

	// 更新缓存
	cm.configCache[configName] = &CachedConfig{
		Config:   config,
		LoadedAt: time.Now(),
		Hash:     cm.calculateConfigHash(config),
		Used:     1,
	}

	return config, nil
}

// GetConfigByEnvironment 根据环境获取配置
func (cm *ConfigManager) GetConfigByEnvironment(
	configName string,
	environment string,
) (*UniversalConfig, error) {

	baseConfig, err := cm.GetConfig(configName)
	if err != nil {
		return nil, err
	}

	// 如果没有指定环境或环境配置不存在，返回基础配置
	if environment == "" {
		return baseConfig, nil
	}

	envConfig, exists := baseConfig.Environments[environment]
	if !exists || !envConfig.Active {
		return baseConfig, nil
	}

	// 创建环境特定的配置副本
	envSpecificConfig := *baseConfig

	// 应用环境覆盖
	if err := cm.applyOverrides(&envSpecificConfig, envConfig.Overrides); err != nil {
		return nil, fmt.Errorf("应用环境覆盖失败: %w", err)
	}

	return &envSpecificConfig, nil
}

// GetStrategy 获取策略
func (cm *ConfigManager) GetStrategy(strategyName string) (Strategy, error) {
	cm.mutex.RLock()
	defer cm.mutex.RUnlock()

	return cm.strategyFactory.GetStrategy(strategyName)
}

// BuildStrategyFromConfig 从配置构建策略
func (cm *ConfigManager) BuildStrategyFromConfig(config *StrategyConfig) (Strategy, error) {
	return cm.strategyFactory.BuildFromConfig(config)
}

// RegisterStrategyBuilder 注册策略构建器
func (cm *ConfigManager) RegisterStrategyBuilder(name string, builder StrategyBuilder) {
	cm.strategyFactory.RegisterStrategy(name, builder)
}

// ReloadConfig 重新加载配置
func (cm *ConfigManager) ReloadConfig(ctx *ctx.Context, configName string) error {
	cm.mutex.RLock()
	configPath, exists := cm.configPaths[configName]
	cm.mutex.RUnlock()

	if !exists {
		return fmt.Errorf("配置不存在: %s", configName)
	}

	return cm.LoadConfig(ctx, configName, configPath)
}

// ValidateConfigFile 验证配置文件
func (cm *ConfigManager) ValidateConfigFile(configPath string) error {
	configData, err := os.ReadFile(configPath)
	if err != nil {
		return fmt.Errorf("读取配置文件失败: %w", err)
	}

	var config UniversalConfig
	ext := filepath.Ext(configPath)
	switch ext {
	case ".json":
		err = json.Unmarshal(configData, &config)
	case ".yaml", ".yml":
		err = yaml.Unmarshal(configData, &config)
	default:
		return fmt.Errorf("不支持的配置文件格式: %s", ext)
	}

	if err != nil {
		return fmt.Errorf("解析配置文件失败: %w", err)
	}

	return cm.validationEngine.ValidateConfig(&config)
}

// ExportConfig 导出配置
func (cm *ConfigManager) ExportConfig(
	configName string,
	format string,
	outputPath string,
) error {
	config, err := cm.GetConfig(configName)
	if err != nil {
		return err
	}

	var data []byte
	switch format {
	case "json":
		data, err = json.MarshalIndent(config, "", "  ")
	case "yaml":
		data, err = yaml.Marshal(config)
	default:
		return fmt.Errorf("不支持的导出格式: %s", format)
	}

	if err != nil {
		return fmt.Errorf("序列化配置失败: %w", err)
	}

	return os.WriteFile(outputPath, data, 0644)
}

// GetConfigStats 获取配置统计信息
func (cm *ConfigManager) GetConfigStats() map[string]interface{} {
	cm.mutex.RLock()
	defer cm.mutex.RUnlock()

	stats := make(map[string]interface{})
	stats["total_configs"] = len(cm.configs)
	stats["cache_size"] = len(cm.configCache)
	stats["watched_paths"] = len(cm.configWatcher.watchPaths)

	cacheStats := make(map[string]interface{})
	for name, cached := range cm.configCache {
		cacheStats[name] = map[string]interface{}{
			"loaded_at": cached.LoadedAt,
			"used":      cached.Used,
			"hash":      cached.Hash,
		}
	}
	stats["cache_details"] = cacheStats

	return stats
}

// 内部方法

// applyEnvironmentOverrides 应用环境覆盖
func (cm *ConfigManager) applyEnvironmentOverrides(config *UniversalConfig) error {
	// 从环境变量或其他方式确定当前环境
	currentEnv := os.Getenv("ANALYSIS_ENVIRONMENT")
	if currentEnv == "" {
		return nil // 没有指定环境
	}

	envConfig, exists := config.Environments[currentEnv]
	if !exists || !envConfig.Active {
		return nil // 环境配置不存在或未激活
	}

	return cm.applyOverrides(config, envConfig.Overrides)
}

// applyOverrides 应用覆盖配置
func (cm *ConfigManager) applyOverrides(
	config *UniversalConfig,
	overrides map[string]interface{},
) error {
	// 简化的覆盖应用逻辑，实际实现可以更复杂
	for key, value := range overrides {
		switch key {
		case "aiEngine.maxTokens":
			if config.AIEngine != nil {
				if maxTokens, ok := value.(float64); ok {
					config.AIEngine.MaxTokens = int(maxTokens)
				}
			}
		case "aiEngine.temperature":
			if config.AIEngine != nil {
				if temp, ok := value.(float64); ok {
					config.AIEngine.Temperature = temp
				}
			}
		case "dataCollection.maxRelatedMetrics":
			if config.DataCollection != nil {
				if maxMetrics, ok := value.(float64); ok {
					config.DataCollection.MaxRelatedMetrics = int(maxMetrics)
				}
			}
		}
	}

	return nil
}

// buildStrategiesFromConfig 从配置构建策略
func (cm *ConfigManager) buildStrategiesFromConfig(config *UniversalConfig) error {
	if config.Strategies == nil {
		return nil
	}

	for name, strategyConfig := range config.Strategies {
		if !strategyConfig.Enabled {
			continue
		}

		strategy, err := cm.strategyFactory.BuildFromConfig(strategyConfig)
		if err != nil {
			return fmt.Errorf("构建策略失败 [%s]: %w", name, err)
		}

		cm.strategyFactory.RegisterBuiltStrategy(name, strategy)
	}

	return nil
}

// updateCache 更新缓存
func (cm *ConfigManager) updateCache(configName string, config *UniversalConfig) {
	cm.configCache[configName] = &CachedConfig{
		Config:   config,
		LoadedAt: time.Now(),
		Hash:     cm.calculateConfigHash(config),
		Used:     0,
	}
}

// calculateConfigHash 计算配置哈希
func (cm *ConfigManager) calculateConfigHash(config *UniversalConfig) string {
	// 简化的哈希计算
	data, _ := json.Marshal(config)
	return fmt.Sprintf("%x", len(data)) // 实际应该使用proper hash function
}

// onConfigChanged 配置变更回调
func (cm *ConfigManager) onConfigChanged(
	ctx *ctx.Context,
	configName string,
	newConfig *UniversalConfig,
) {
	cm.mutex.Lock()
	defer cm.mutex.Unlock()

	logc.Infof(ctx.Ctx, "[配置管理] 配置变更: name=%s", configName)

	// 更新配置
	cm.configs[configName] = newConfig

	// 清除缓存
	delete(cm.configCache, configName)

	// 重新构建策略
	if err := cm.buildStrategiesFromConfig(newConfig); err != nil {
		logc.Errorf(ctx.Ctx, "[配置管理] 重新构建策略失败: %v", err)
	}
}

// NewStrategyFactory 创建策略工厂
func NewStrategyFactory() *StrategyFactory {
	return &StrategyFactory{
		registeredStrategies: make(map[string]StrategyBuilder),
	}
}

// RegisterStrategy 注册策略
func (sf *StrategyFactory) RegisterStrategy(name string, builder StrategyBuilder) {
	sf.mutex.Lock()
	defer sf.mutex.Unlock()
	sf.registeredStrategies[name] = builder
}

// BuildFromConfig 从配置构建策略
func (sf *StrategyFactory) BuildFromConfig(config *StrategyConfig) (Strategy, error) {
	sf.mutex.RLock()
	builder, exists := sf.registeredStrategies[config.Type]
	sf.mutex.RUnlock()

	if !exists {
		return nil, fmt.Errorf("未知策略类型: %s", config.Type)
	}

	return builder.Build(config.Parameters)
}

// GetStrategy 获取策略
func (sf *StrategyFactory) GetStrategy(name string) (Strategy, error) {
	// 实现策略获取逻辑
	return nil, fmt.Errorf("策略不存在: %s", name)
}

// RegisterBuiltStrategy 注册已构建的策略
func (sf *StrategyFactory) RegisterBuiltStrategy(name string, strategy Strategy) {
	// 实现策略注册逻辑
}

// NewConfigValidationEngine 创建配置验证引擎
func NewConfigValidationEngine() *ConfigValidationEngine {
	return &ConfigValidationEngine{
		validators: make(map[string]ConfigValidator),
		rules:      make(map[string]*ValidationRule),
	}
}

// ValidateConfig 验证配置
func (cve *ConfigValidationEngine) ValidateConfig(config *UniversalConfig) error {
	// 基础验证
	if config.Metadata.Name == "" {
		return fmt.Errorf("配置名称不能为空")
	}

	if config.Metadata.Version == "" {
		return fmt.Errorf("配置版本不能为空")
	}

	// 验证各组件配置
	if config.AIEngine != nil {
		if err := cve.validateAIEngineConfig(config.AIEngine); err != nil {
			return fmt.Errorf("AI引擎配置验证失败: %w", err)
		}
	}

	return nil
}

// validateAIEngineConfig 验证AI引擎配置
func (cve *ConfigValidationEngine) validateAIEngineConfig(config *ai.AIEngineConfig) error {
	if config.APIEndpoint == "" {
		return fmt.Errorf("API端点不能为空")
	}

	if config.MaxTokens <= 0 {
		return fmt.Errorf("MaxTokens必须大于0")
	}

	if config.Temperature < 0 || config.Temperature > 2 {
		return fmt.Errorf("Temperature必须在0-2之间")
	}

	return nil
}

// NewConfigWatcher 创建配置监控器
func NewConfigWatcher() *ConfigWatcher {
	return &ConfigWatcher{
		watchPaths:   make(map[string]bool),
		callbacks:    make(map[string]func(string, *UniversalConfig)),
		stopChannels: make(map[string]chan bool),
	}
}

// WatchConfig 监控配置文件
func (cw *ConfigWatcher) WatchConfig(
	configPath string,
	callback func(string, *UniversalConfig),
) error {
	cw.mutex.Lock()
	defer cw.mutex.Unlock()

	if cw.watchPaths[configPath] {
		return nil // 已经在监控
	}

	stopChan := make(chan bool, 1)
	cw.watchPaths[configPath] = true
	cw.callbacks[configPath] = callback
	cw.stopChannels[configPath] = stopChan

	// 启动监控goroutine（简化实现）
	go cw.watchFile(configPath, callback, stopChan)

	return nil
}

// watchFile 监控文件变化
func (cw *ConfigWatcher) watchFile(
	filePath string,
	callback func(string, *UniversalConfig),
	stopChan chan bool,
) {
	// 简化的文件监控实现
	// 实际应该使用 fsnotify 等库
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	var lastModTime time.Time
	if stat, err := os.Stat(filePath); err == nil {
		lastModTime = stat.ModTime()
	}

	for {
		select {
		case <-stopChan:
			return
		case <-ticker.C:
			if stat, err := os.Stat(filePath); err == nil {
				if stat.ModTime().After(lastModTime) {
					lastModTime = stat.ModTime()
					// 文件已修改，重新加载
					if newConfig, err := cw.loadConfigFile(filePath); err == nil {
						callback(filePath, newConfig)
					}
				}
			}
		}
	}
}

// loadConfigFile 加载配置文件
func (cw *ConfigWatcher) loadConfigFile(filePath string) (*UniversalConfig, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}

	var config UniversalConfig
	ext := filepath.Ext(filePath)
	switch ext {
	case ".json":
		err = json.Unmarshal(data, &config)
	case ".yaml", ".yml":
		err = yaml.Unmarshal(data, &config)
	default:
		return nil, fmt.Errorf("不支持的文件格式: %s", ext)
	}

	return &config, err
}
