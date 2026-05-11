// Package config provides centralized configuration management for the MCP server.
// Configuration is loaded from two sources with the following precedence
// (highest to lowest):
//
//  1. .env file (for credentials and runtime parameters)
//  2. config.yaml file (for all non-sensitive configuration)
//  3. Shell environment variables (fallback for credentials in stdio mode)
//  4. Built-in defaults (for stdio mode when config.yaml is not found)
//
// The configuration is initialized once via sync.Once and frozen after loading
// to prevent runtime modification.
package config

import (
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/spf13/viper"
)

// ServerConfig 服务器配置
type ServerConfig struct {
	Transport string `mapstructure:"transport"`
	Host      string `mapstructure:"host"`
	Port      int    `mapstructure:"port"`
}

// LoggingConfig 日志配置
type LoggingConfig struct {
	Level     string `mapstructure:"level"`
	DebugMode bool   `mapstructure:"debug_mode"`
}

// ToolkitConfig 工具集配置
type ToolkitConfig struct {
	Scope        string   `mapstructure:"scope"`
	EnabledTools []string `mapstructure:"enabled_tools"`
}

// NetworkConfig 网络配置
type NetworkConfig struct {
	MaxRetry         int `mapstructure:"max_retry"`
	RetryWaitSeconds int `mapstructure:"retry_wait_seconds"`
	ReadTimeoutMs    int `mapstructure:"read_timeout_ms"`
	ConnectTimeoutMs int `mapstructure:"connect_timeout_ms"`
}

// LocaleConfig 本地化配置
type LocaleConfig struct {
	Timezone string `mapstructure:"timezone"` // 时区，如 Asia/Shanghai
	Language string `mapstructure:"language"` // 语言，如 zh-CN, en-US
}

// EndpointsConfig 端点覆盖配置
type EndpointsConfig struct {
	SLS     map[string]string `mapstructure:"sls"`
	CMS     map[string]string `mapstructure:"cms"`
	StarOps map[string]string `mapstructure:"starops"`
}

// CredentialsConfig 凭证配置（从 .env 加载）
type CredentialsConfig struct {
	AccessKeyID     string
	AccessKeySecret string
	SecurityToken   string // 可选：STS Token
}

// RuntimeConfig 运行时参数
// 优先级: 环境变量 > .env 文件 > config.yaml
type RuntimeConfig struct {
	Region    string `mapstructure:"region"`
	Workspace string `mapstructure:"workspace"`
}

// defaultConfig 内置默认值，用于 stdio 模式下 config.yaml 不存在时的 fallback
var defaultConfig = Config{
	Server: ServerConfig{
		Transport: "stdio",   // 默认 stdio 模式
		Host:      "0.0.0.0", // 监听所有接口
		Port:      8080,      // 默认端口
	},
	Logging: LoggingConfig{
		Level:     "info", // 默认日志级别
		DebugMode: false,  // 默认关闭调试模式
	},
	Toolkit: ToolkitConfig{
		Scope: "all", // 默认启用所有工具
	},
	Network: NetworkConfig{
		MaxRetry:         1,      // 最大重试次数
		RetryWaitSeconds: 1,      // 重试等待时间（秒）
		ReadTimeoutMs:    610000, // 读取超时（毫秒）
		ConnectTimeoutMs: 30000,  // 连接超时（毫秒）
	},
	Locale: LocaleConfig{
		Timezone: "Asia/Shanghai", // 默认时区
		Language: "zh-CN",         // 默认语言
	},
}

// Config holds all server configuration. After Load() returns, the instance
// is considered frozen and must not be modified.
//
// Configuration is organized into nested structs by category:
// - Server: transport, host, port
// - Logging: level, debug_mode
// - Toolkit: scope
// - Network: retry, timeout settings
// - Locale: timezone, language
// - Endpoints: custom SLS/CMS endpoints
// - Credentials: AccessKey (from .env)
// - Runtime: region, workspace (from config.yaml, overridden by .env/env vars)
type Config struct {
	Server      ServerConfig      `mapstructure:"server"`
	Logging     LoggingConfig     `mapstructure:"logging"`
	Toolkit     ToolkitConfig     `mapstructure:"toolkit"`
	Network     NetworkConfig     `mapstructure:"network"`
	Locale      LocaleConfig      `mapstructure:"locale"`
	Endpoints   EndpointsConfig   `mapstructure:"endpoints"`
	Credentials CredentialsConfig // 从 .env 加载
	Runtime     RuntimeConfig     `mapstructure:"runtime"` // 优先级: 环境变量 > .env > config.yaml
}

var (
	globalConfig *Config
	configOnce   sync.Once
)

// Load 从 config.yaml、.env 和环境变量加载配置
// configPath: 配置文件路径，为空时搜索默认位置
// 返回配置和可能的错误
func Load(configPath string) (*Config, error) {
	var loadErr error
	configOnce.Do(func() {
		globalConfig, loadErr = load(configPath)
	})
	return globalConfig, loadErr
}

// Get returns the global configuration. If Load() has not been called yet,
// it triggers loading with default values (empty configPath).
func Get() *Config {
	cfg, _ := Load("")
	return cfg
}

// loadDotEnv reads a .env file and returns key-value pairs.
// Returns nil if .env file is not found (optional file).
func loadDotEnv() map[string]string {
	envViper := viper.New()
	envViper.SetConfigFile(".env")
	envViper.SetConfigType("env")
	if err := envViper.ReadInConfig(); err != nil {
		// .env file is optional — silently skip if not found
		return nil
	}
	result := make(map[string]string)
	for _, key := range envViper.AllKeys() {
		if val := envViper.GetString(key); val != "" {
			result[key] = val
		}
	}
	return result
}

// loadCredentials 加载凭证配置
// 优先级: .env 文件 > shell 环境变量
func loadCredentials() CredentialsConfig {
	envValues := loadDotEnv()

	// Helper function to get value from .env or shell env
	getEnvValue := func(envKey string) string {
		// First try .env file
		if envValues != nil {
			// Try lowercase key (viper normalizes to lowercase)
			if val, ok := envValues[strings.ToLower(envKey)]; ok && val != "" {
				return val
			}
		}
		// Fallback to shell environment variable
		return os.Getenv(envKey)
	}

	return CredentialsConfig{
		AccessKeyID:     getEnvValue("ALIBABA_CLOUD_ACCESS_KEY_ID"),
		AccessKeySecret: getEnvValue("ALIBABA_CLOUD_ACCESS_KEY_SECRET"),
		SecurityToken:   getEnvValue("ALIBABA_CLOUD_SECURITY_TOKEN"),
	}
}

// loadRuntime 加载运行时配置
// 优先级: 环境变量 > .env 文件 > config.yaml（yamlDefaults 参数）
func loadRuntime(yamlDefaults RuntimeConfig) RuntimeConfig {
	envValues := loadDotEnv()

	// Helper function to get value from .env or shell env
	getEnvValue := func(envKey string) string {
		// First try .env file
		if envValues != nil {
			if val, ok := envValues[strings.ToLower(envKey)]; ok && val != "" {
				return val
			}
		}
		// Fallback to shell environment variable
		return os.Getenv(envKey)
	}

	// 环境变量覆盖 config.yaml 的值
	region := getEnvValue("ALIBABA_CLOUD_REGION")
	if region == "" {
		region = yamlDefaults.Region
	}

	workspace := getEnvValue("ALIBABA_CLOUD_WORKSPACE")
	if workspace == "" {
		workspace = yamlDefaults.Workspace
	}

	return RuntimeConfig{
		Region:    region,
		Workspace: workspace,
	}
}

// load performs the actual configuration loading. Called exactly once.
//
// configPath: 配置文件路径，为空时搜索默认位置
//
// 加载流程:
//  1. 从 defaultConfig 开始
//  2. 尝试加载 config.yaml
//  3. 使用 viper 解析 YAML 并 unmarshal 到 cfg
//  4. 调用 loadCredentials() 加载凭证
//  5. 调用 cfg.Validate() 验证配置
//  6. 返回配置
func load(configPath string) (*Config, error) {
	// 1. 从 defaultConfig 开始
	cfg := defaultConfig

	// 2. 尝试加载 config.yaml
	v := viper.New()
	v.SetConfigType("yaml")

	var configFileFound bool
	if configPath != "" {
		// 使用指定路径
		v.SetConfigFile(configPath)
	} else {
		// 搜索默认位置
		v.SetConfigName("config")
		v.AddConfigPath(".")        // current working directory
		v.AddConfigPath("./config") // ./config/ subdirectory
	}

	if err := v.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); ok {
			// config.yaml 未找到
			configFileFound = false
		} else {
			// 配置文件存在但解析失败
			return nil, fmt.Errorf("failed to parse config file: %w", err)
		}
	} else {
		configFileFound = true
	}

	// 3. 根据 transport 模式决定是否需要 config.yaml
	// 先从 viper 获取 transport，如果没有则使用默认值
	transport := strings.ToLower(strings.TrimSpace(v.GetString("server.transport")))
	if transport == "" {
		transport = cfg.Server.Transport // 使用默认值 "stdio"
	}

	// HTTP/SSE 模式下 config.yaml 必需
	if !configFileFound && (transport == "sse" || transport == "streamable-http") {
		searchPaths := "., ./config/"
		if configPath != "" {
			searchPaths = configPath
		}
		return nil, fmt.Errorf("config file not found: searched [%s]. Required for HTTP/SSE mode", searchPaths)
	}

	// 4. 如果找到配置文件，unmarshal 到 cfg
	if configFileFound {
		if err := v.Unmarshal(&cfg); err != nil {
			return nil, fmt.Errorf("failed to unmarshal config: %w", err)
		}
	}

	// 5. 调用 loadCredentials() 加载凭证
	cfg.Credentials = loadCredentials()
	cfg.Runtime = loadRuntime(cfg.Runtime)

	// 6. 调用 cfg.Validate() 验证配置（不验证凭证）
	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	// 7. 返回配置
	return &cfg, nil
}

// normalizeHost strips scheme prefix and trailing slash from a host string.
func normalizeHost(val string) string {
	v := strings.TrimSpace(val)
	v = strings.TrimPrefix(v, "https://")
	v = strings.TrimPrefix(v, "http://")
	v = strings.TrimRight(v, "/")
	return v
}

// ResetForTesting resets the global singleton so Load() can be called again.
// This MUST only be used in tests.
func ResetForTesting() {
	globalConfig = nil
	configOnce = sync.Once{}
	viper.Reset()
}

// String returns a human-readable summary of the configuration (for logging).
func (c *Config) String() string {
	return fmt.Sprintf(
		"Config{Server={Transport=%s, Host=%s, Port=%d}, Logging={Level=%s, DebugMode=%v}, "+
			"Toolkit={Scope=%s}, Network={MaxRetry=%d, RetryWaitSeconds=%d, ReadTimeoutMs=%d, ConnectTimeoutMs=%d}}",
		c.Server.Transport, c.Server.Host, c.Server.Port,
		c.Logging.Level, c.Logging.DebugMode,
		c.Toolkit.Scope,
		c.Network.MaxRetry, c.Network.RetryWaitSeconds, c.Network.ReadTimeoutMs, c.Network.ConnectTimeoutMs,
	)
}

// stringSliceContains checks if a string slice contains a specific item.
func stringSliceContains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

// Validate 验证服务器配置
func (c *ServerConfig) Validate() error {
	validTransports := []string{"stdio", "sse", "streamable-http"}
	if !stringSliceContains(validTransports, c.Transport) {
		return fmt.Errorf("invalid transport '%s': must be one of %v", c.Transport, validTransports)
	}
	if c.Port < 1 || c.Port > 65535 {
		return fmt.Errorf("invalid port '%d': must be between 1 and 65535", c.Port)
	}
	return nil
}

// Validate 验证日志配置
func (c *LoggingConfig) Validate() error {
	validLevels := []string{"debug", "info", "warn", "error"}
	if !stringSliceContains(validLevels, c.Level) {
		return fmt.Errorf("invalid log level '%s': must be one of %v", c.Level, validLevels)
	}
	return nil
}

// Validate 验证工具集配置
func (c *ToolkitConfig) Validate() error {
	// 当 enabled_tools 非空时，scope 被忽略，无需校验
	if len(c.EnabledTools) > 0 {
		return nil
	}
	validScopes := []string{"all", "paas", "iaas"}
	if !stringSliceContains(validScopes, c.Scope) {
		return fmt.Errorf("invalid toolkit scope '%s': must be one of %v", c.Scope, validScopes)
	}
	return nil
}

// Validate 验证本地化配置
func (c *LocaleConfig) Validate() error {
	if c.Timezone != "" {
		if _, err := time.LoadLocation(c.Timezone); err != nil {
			return fmt.Errorf("invalid timezone '%s': %w", c.Timezone, err)
		}
	}
	return nil
}

// Validate 验证凭证配置
func (c *CredentialsConfig) Validate() error {
	if c.AccessKeyID == "" || c.AccessKeySecret == "" {
		return fmt.Errorf("credentials not configured: set ALIBABA_CLOUD_ACCESS_KEY_ID and ALIBABA_CLOUD_ACCESS_KEY_SECRET")
	}
	return nil
}

// Validator 配置验证接口
type Validator interface {
	Validate() error
}

// Validate 验证整体配置（调用各子配置的验证方法）
// 注意：Credentials 不在此处验证，通过 ValidateCredentials() 单独验证（延迟验证）
func (c *Config) Validate() error {
	validators := []Validator{&c.Server, &c.Logging, &c.Toolkit, &c.Locale}
	for _, v := range validators {
		if err := v.Validate(); err != nil {
			return err
		}
	}
	return nil
}

// ValidateCredentials 单独验证凭证（延迟验证）
// 凭证验证与其他配置分开，因为在某些场景下（如 stdio 模式）
// 凭证可能在启动后才通过环境变量提供
func (c *Config) ValidateCredentials() error {
	return c.Credentials.Validate()
}

// GetTimezoneLocation returns the configured timezone as a *time.Location.
// Returns nil and error if the timezone is invalid.
func (c *Config) GetTimezoneLocation() (*time.Location, error) {
	return time.LoadLocation(c.Locale.Timezone)
}

// GetReadTimeout returns the read timeout as a time.Duration.
func (c *Config) GetReadTimeout() time.Duration {
	return time.Duration(c.Network.ReadTimeoutMs) * time.Millisecond
}

// GetConnectTimeout returns the connect timeout as a time.Duration.
func (c *Config) GetConnectTimeout() time.Duration {
	return time.Duration(c.Network.ConnectTimeoutMs) * time.Millisecond
}
