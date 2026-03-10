package config

import (
	"sync"
	"testing"

	"github.com/leanovate/gopter"
	"github.com/leanovate/gopter/gen"
	"github.com/leanovate/gopter/prop"
)

// TestProperty_GlobalConfigSingleton verifies that concurrent goroutines calling
// config.Get() all receive the exact same *Config pointer, ensuring the singleton
// guarantee provided by sync.Once.
//
// Feature: config-refactor, Property: 全局配置单例
// Validates: Requirements 4.4
func TestProperty_GlobalConfigSingleton(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100
	properties := gopter.NewProperties(parameters)

	properties.Property("concurrent Get() calls return identical pointer", prop.ForAll(
		func(numGoroutines int) bool {
			ResetForTesting()

			results := make([]*Config, numGoroutines)
			var wg sync.WaitGroup
			barrier := make(chan struct{})

			for i := 0; i < numGoroutines; i++ {
				wg.Add(1)
				go func(idx int) {
					defer wg.Done()
					<-barrier
					results[idx] = Get()
				}(i)
			}

			close(barrier)
			wg.Wait()

			first := results[0]
			if first == nil {
				return false
			}
			for i := 1; i < numGoroutines; i++ {
				if results[i] != first {
					return false
				}
			}
			return true
		},
		gen.IntRange(2, 50),
	))

	properties.TestingRun(t)
}

// TestProperty_ServerConfigValidation verifies that ServerConfig.Validate()
// correctly accepts valid transports and rejects invalid ones.
//
// Feature: config-refactor, Property: 服务器配置验证
// Validates: Requirements 5.1
func TestProperty_ServerConfigValidation(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100
	properties := gopter.NewProperties(parameters)

	// Valid transports should pass validation
	properties.Property("valid transports pass validation", prop.ForAll(
		func(transport string) bool {
			cfg := &ServerConfig{Transport: transport, Host: "0.0.0.0", Port: 8080}
			return cfg.Validate() == nil
		},
		gen.OneConstOf("stdio", "sse", "streamable-http"),
	))

	// Valid ports should pass validation
	properties.Property("valid ports pass validation", prop.ForAll(
		func(port int) bool {
			cfg := &ServerConfig{Transport: "stdio", Host: "0.0.0.0", Port: port}
			return cfg.Validate() == nil
		},
		gen.IntRange(1, 65535),
	))

	// Invalid ports should fail validation
	properties.Property("invalid ports fail validation", prop.ForAll(
		func(port int) bool {
			cfg := &ServerConfig{Transport: "stdio", Host: "0.0.0.0", Port: port}
			return cfg.Validate() != nil
		},
		gen.OneConstOf(0, -1, 65536, 70000),
	))

	properties.TestingRun(t)
}

// TestProperty_LoggingConfigValidation verifies that LoggingConfig.Validate()
// correctly accepts valid log levels and rejects invalid ones.
//
// Feature: config-refactor, Property: 日志配置验证
// Validates: Requirements 5.2
func TestProperty_LoggingConfigValidation(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100
	properties := gopter.NewProperties(parameters)

	// Valid log levels should pass validation
	properties.Property("valid log levels pass validation", prop.ForAll(
		func(level string) bool {
			cfg := &LoggingConfig{Level: level}
			return cfg.Validate() == nil
		},
		gen.OneConstOf("debug", "info", "warn", "error"),
	))

	// Invalid log levels should fail validation
	properties.Property("invalid log levels fail validation", prop.ForAll(
		func(level string) bool {
			cfg := &LoggingConfig{Level: level}
			return cfg.Validate() != nil
		},
		gen.OneConstOf("invalid", "trace", "fatal", "DEBUG", "INFO"),
	))

	properties.TestingRun(t)
}

// TestProperty_ToolkitConfigValidation verifies that ToolkitConfig.Validate()
// correctly accepts valid scopes and rejects invalid ones.
//
// Feature: config-refactor, Property: 工具集配置验证
// Validates: Requirements 5.3
func TestProperty_ToolkitConfigValidation(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100
	properties := gopter.NewProperties(parameters)

	// Valid scopes should pass validation
	properties.Property("valid scopes pass validation", prop.ForAll(
		func(scope string) bool {
			cfg := &ToolkitConfig{Scope: scope}
			return cfg.Validate() == nil
		},
		gen.OneConstOf("all", "paas", "iaas"),
	))

	// Invalid scopes should fail validation
	properties.Property("invalid scopes fail validation", prop.ForAll(
		func(scope string) bool {
			cfg := &ToolkitConfig{Scope: scope}
			return cfg.Validate() != nil
		},
		gen.OneConstOf("invalid", "ALL", "PAAS", "both", ""),
	))

	properties.TestingRun(t)
}

// TestProperty_CredentialsConfigValidation verifies that CredentialsConfig.Validate()
// correctly validates credential presence.
//
// Feature: config-refactor, Property: 凭证配置验证
// Validates: Requirements 5.5
func TestProperty_CredentialsConfigValidation(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100
	properties := gopter.NewProperties(parameters)

	// Non-empty credentials should pass validation
	properties.Property("non-empty credentials pass validation", prop.ForAll(
		func(akID, akSecret string) bool {
			if akID == "" || akSecret == "" {
				return true // Skip empty cases
			}
			cfg := &CredentialsConfig{AccessKeyID: akID, AccessKeySecret: akSecret}
			return cfg.Validate() == nil
		},
		gen.AlphaString().SuchThat(func(s string) bool { return len(s) > 0 }),
		gen.AlphaString().SuchThat(func(s string) bool { return len(s) > 0 }),
	))

	// Empty credentials should fail validation
	properties.Property("empty credentials fail validation", prop.ForAll(
		func(akID, akSecret string) bool {
			cfg := &CredentialsConfig{AccessKeyID: akID, AccessKeySecret: akSecret}
			return cfg.Validate() != nil
		},
		gen.OneConstOf("", ""),
		gen.OneConstOf("", ""),
	))

	properties.TestingRun(t)
}

// TestProperty_TimeoutConversion verifies that timeout conversion methods
// correctly convert milliseconds to time.Duration.
//
// Feature: config-refactor, Property: 超时转换
// Validates: Requirements 2.1
func TestProperty_TimeoutConversion(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100
	properties := gopter.NewProperties(parameters)

	// GetReadTimeout should return correct duration
	properties.Property("GetReadTimeout returns correct duration", prop.ForAll(
		func(ms int) bool {
			cfg := &Config{Network: NetworkConfig{ReadTimeoutMs: ms}}
			return cfg.GetReadTimeout().Milliseconds() == int64(ms)
		},
		gen.IntRange(0, 1000000),
	))

	// GetConnectTimeout should return correct duration
	properties.Property("GetConnectTimeout returns correct duration", prop.ForAll(
		func(ms int) bool {
			cfg := &Config{Network: NetworkConfig{ConnectTimeoutMs: ms}}
			return cfg.GetConnectTimeout().Milliseconds() == int64(ms)
		},
		gen.IntRange(0, 1000000),
	))

	properties.TestingRun(t)
}
