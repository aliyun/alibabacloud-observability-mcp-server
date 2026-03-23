package config

import (
	"os"
	"path/filepath"
	"sync"
	"testing"
)

// TestLoad_Defaults tests that default values are applied when no config file exists
func TestLoad_Defaults(t *testing.T) {
	ResetForTesting()

	// Load with empty path (stdio mode allows missing config.yaml)
	cfg, err := Load("")
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	// Server defaults
	if cfg.Server.Transport != "stdio" {
		t.Errorf("Server.Transport = %q, want %q", cfg.Server.Transport, "stdio")
	}
	if cfg.Server.Host != "0.0.0.0" {
		t.Errorf("Server.Host = %q, want %q", cfg.Server.Host, "0.0.0.0")
	}
	if cfg.Server.Port != 8080 {
		t.Errorf("Server.Port = %d, want %d", cfg.Server.Port, 8080)
	}

	// Logging defaults
	if cfg.Logging.Level != "info" {
		t.Errorf("Logging.Level = %q, want %q", cfg.Logging.Level, "info")
	}
	if cfg.Logging.DebugMode != false {
		t.Errorf("Logging.DebugMode = %v, want false", cfg.Logging.DebugMode)
	}

	// Toolkit defaults
	if cfg.Toolkit.Scope != "all" {
		t.Errorf("Toolkit.Scope = %q, want %q", cfg.Toolkit.Scope, "all")
	}

	// Network defaults
	if cfg.Network.MaxRetry != 1 {
		t.Errorf("Network.MaxRetry = %d, want %d", cfg.Network.MaxRetry, 1)
	}
	if cfg.Network.RetryWaitSeconds != 1 {
		t.Errorf("Network.RetryWaitSeconds = %d, want %d", cfg.Network.RetryWaitSeconds, 1)
	}
	if cfg.Network.ReadTimeoutMs != 610000 {
		t.Errorf("Network.ReadTimeoutMs = %d, want %d", cfg.Network.ReadTimeoutMs, 610000)
	}
	if cfg.Network.ConnectTimeoutMs != 30000 {
		t.Errorf("Network.ConnectTimeoutMs = %d, want %d", cfg.Network.ConnectTimeoutMs, 30000)
	}

	// Locale defaults
	if cfg.Locale.Timezone != "Asia/Shanghai" {
		t.Errorf("Locale.Timezone = %q, want %q", cfg.Locale.Timezone, "Asia/Shanghai")
	}
	if cfg.Locale.Language != "zh-CN" {
		t.Errorf("Locale.Language = %q, want %q", cfg.Locale.Language, "zh-CN")
	}
}

// TestLoad_ConfigYAML tests loading configuration from a YAML file
func TestLoad_ConfigYAML(t *testing.T) {
	ResetForTesting()

	// Create a temporary config file
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")
	configContent := `
server:
  transport: sse
  host: 127.0.0.1
  port: 9090
logging:
  level: debug
  debug_mode: true
toolkit:
  scope: paas
network:
  max_retry: 3
  retry_wait_seconds: 2
  read_timeout_ms: 120000
  connect_timeout_ms: 5000
locale:
  timezone: UTC
  language: en-US
`
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("Failed to write config file: %v", err)
	}

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	// Verify loaded values
	if cfg.Server.Transport != "sse" {
		t.Errorf("Server.Transport = %q, want %q", cfg.Server.Transport, "sse")
	}
	if cfg.Server.Host != "127.0.0.1" {
		t.Errorf("Server.Host = %q, want %q", cfg.Server.Host, "127.0.0.1")
	}
	if cfg.Server.Port != 9090 {
		t.Errorf("Server.Port = %d, want %d", cfg.Server.Port, 9090)
	}
	if cfg.Logging.Level != "debug" {
		t.Errorf("Logging.Level = %q, want %q", cfg.Logging.Level, "debug")
	}
	if cfg.Logging.DebugMode != true {
		t.Errorf("Logging.DebugMode = %v, want true", cfg.Logging.DebugMode)
	}
	if cfg.Toolkit.Scope != "paas" {
		t.Errorf("Toolkit.Scope = %q, want %q", cfg.Toolkit.Scope, "paas")
	}
	if cfg.Network.MaxRetry != 3 {
		t.Errorf("Network.MaxRetry = %d, want %d", cfg.Network.MaxRetry, 3)
	}
	if cfg.Locale.Timezone != "UTC" {
		t.Errorf("Locale.Timezone = %q, want %q", cfg.Locale.Timezone, "UTC")
	}
}

// TestLoad_HTTPModeRequiresConfig tests that HTTP/SSE modes require config.yaml
func TestLoad_HTTPModeRequiresConfig(t *testing.T) {
	ResetForTesting()

	// Create a config file with SSE transport but then delete it
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "nonexistent.yaml")

	// Try to load non-existent config - should fail for HTTP mode
	// Since we can't specify transport without a config file, this test
	// verifies the error message when config is not found
	_, err := Load(configPath)
	if err == nil {
		t.Error("Load() should fail when config file not found for specified path")
	}
}

// TestLoad_CredentialsFromEnv tests loading credentials from environment variables
func TestLoad_CredentialsFromEnv(t *testing.T) {
	ResetForTesting()

	t.Setenv("ALIBABA_CLOUD_ACCESS_KEY_ID", "test-ak-id")
	t.Setenv("ALIBABA_CLOUD_ACCESS_KEY_SECRET", "test-ak-secret")
	t.Setenv("ALIBABA_CLOUD_SECURITY_TOKEN", "test-token")
	t.Setenv("ALIBABA_CLOUD_REGION", "cn-hongkong")
	t.Setenv("ALIBABA_CLOUD_WORKSPACE", "test-workspace")

	cfg, err := Load("")
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.Credentials.AccessKeyID != "test-ak-id" {
		t.Errorf("Credentials.AccessKeyID = %q, want %q", cfg.Credentials.AccessKeyID, "test-ak-id")
	}
	if cfg.Credentials.AccessKeySecret != "test-ak-secret" {
		t.Errorf("Credentials.AccessKeySecret = %q, want %q", cfg.Credentials.AccessKeySecret, "test-ak-secret")
	}
	if cfg.Credentials.SecurityToken != "test-token" {
		t.Errorf("Credentials.SecurityToken = %q, want %q", cfg.Credentials.SecurityToken, "test-token")
	}
	if cfg.Runtime.Region != "cn-hongkong" {
		t.Errorf("Runtime.Region = %q, want %q", cfg.Runtime.Region, "cn-hongkong")
	}
	if cfg.Runtime.Workspace != "test-workspace" {
		t.Errorf("Runtime.Workspace = %q, want %q", cfg.Runtime.Workspace, "test-workspace")
	}
}

// TestGet_ReturnsSameAsLoad tests that Get() returns the same config as Load()
func TestGet_ReturnsSameAsLoad(t *testing.T) {
	ResetForTesting()

	loaded, err := Load("")
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	got := Get()

	if loaded != got {
		t.Errorf("Get() returned different pointer than Load()")
	}
}

// TestLoad_Singleton tests that Load() returns the same instance on multiple calls
func TestLoad_Singleton(t *testing.T) {
	ResetForTesting()

	first, err1 := Load("")
	if err1 != nil {
		t.Fatalf("First Load() error = %v", err1)
	}

	second, err2 := Load("")
	if err2 != nil {
		t.Fatalf("Second Load() error = %v", err2)
	}

	if first != second {
		t.Errorf("Load() returned different pointers: %p vs %p", first, second)
	}
}

// TestConfig_String tests the String() method
func TestConfig_String(t *testing.T) {
	ResetForTesting()
	cfg, err := Load("")
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	s := cfg.String()
	if s == "" {
		t.Error("String() returned empty string")
	}
	// Sanity check it contains key info
	if !contains(s, "Transport=stdio") {
		t.Errorf("String() missing Transport info: %s", s)
	}
}

// TestServerConfig_Validate tests server config validation
func TestServerConfig_Validate(t *testing.T) {
	tests := []struct {
		name      string
		transport string
		port      int
		wantErr   bool
	}{
		{"valid stdio", "stdio", 8080, false},
		{"valid sse", "sse", 8080, false},
		{"valid http", "streamable-http", 8080, false},
		{"invalid transport", "invalid", 8080, true},
		{"invalid port low", "stdio", 0, true},
		{"invalid port high", "stdio", 70000, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &ServerConfig{Transport: tt.transport, Port: tt.port}
			err := cfg.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

// TestLoggingConfig_Validate tests logging config validation
func TestLoggingConfig_Validate(t *testing.T) {
	tests := []struct {
		name    string
		level   string
		wantErr bool
	}{
		{"valid debug", "debug", false},
		{"valid info", "info", false},
		{"valid warn", "warn", false},
		{"valid error", "error", false},
		{"invalid level", "invalid", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &LoggingConfig{Level: tt.level}
			err := cfg.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

// TestToolkitConfig_Validate tests toolkit config validation
func TestToolkitConfig_Validate(t *testing.T) {
	tests := []struct {
		name    string
		scope   string
		wantErr bool
	}{
		{"valid all", "all", false},
		{"valid paas", "paas", false},
		{"valid iaas", "iaas", false},
		{"invalid scope", "invalid", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &ToolkitConfig{Scope: tt.scope}
			err := cfg.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

// TestLocaleConfig_Validate tests locale config validation
func TestLocaleConfig_Validate(t *testing.T) {
	tests := []struct {
		name     string
		timezone string
		wantErr  bool
	}{
		{"valid Shanghai", "Asia/Shanghai", false},
		{"valid UTC", "UTC", false},
		{"valid empty", "", false},
		{"invalid timezone", "Invalid/Timezone", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &LocaleConfig{Timezone: tt.timezone}
			err := cfg.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

// TestCredentialsConfig_Validate tests credentials config validation
func TestCredentialsConfig_Validate(t *testing.T) {
	tests := []struct {
		name    string
		akID    string
		akSec   string
		wantErr bool
	}{
		{"valid credentials", "ak-id", "ak-secret", false},
		{"missing id", "", "ak-secret", true},
		{"missing secret", "ak-id", "", true},
		{"both missing", "", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &CredentialsConfig{AccessKeyID: tt.akID, AccessKeySecret: tt.akSec}
			err := cfg.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

// TestConfig_GetTimezoneLocation tests the GetTimezoneLocation helper
func TestConfig_GetTimezoneLocation(t *testing.T) {
	cfg := &Config{Locale: LocaleConfig{Timezone: "Asia/Shanghai"}}
	loc, err := cfg.GetTimezoneLocation()
	if err != nil {
		t.Fatalf("GetTimezoneLocation() error = %v", err)
	}
	if loc.String() != "Asia/Shanghai" {
		t.Errorf("GetTimezoneLocation() = %q, want %q", loc.String(), "Asia/Shanghai")
	}
}

// TestConfig_GetReadTimeout tests the GetReadTimeout helper
func TestConfig_GetReadTimeout(t *testing.T) {
	cfg := &Config{Network: NetworkConfig{ReadTimeoutMs: 5000}}
	timeout := cfg.GetReadTimeout()
	if timeout.Milliseconds() != 5000 {
		t.Errorf("GetReadTimeout() = %v, want 5000ms", timeout)
	}
}

// TestConfig_GetConnectTimeout tests the GetConnectTimeout helper
func TestConfig_GetConnectTimeout(t *testing.T) {
	cfg := &Config{Network: NetworkConfig{ConnectTimeoutMs: 3000}}
	timeout := cfg.GetConnectTimeout()
	if timeout.Milliseconds() != 3000 {
		t.Errorf("GetConnectTimeout() = %v, want 3000ms", timeout)
	}
}

// TestNormalizeHost tests the normalizeHost helper function
func TestNormalizeHost(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"example.com", "example.com"},
		{"https://example.com", "example.com"},
		{"http://example.com", "example.com"},
		{"https://example.com/", "example.com"},
		{"  https://example.com/  ", "example.com"},
		{"", ""},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := normalizeHost(tt.input)
			if got != tt.want {
				t.Errorf("normalizeHost(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

// TestGlobalConfigSingleton tests concurrent access to the singleton
func TestGlobalConfigSingleton(t *testing.T) {
	ResetForTesting()

	numGoroutines := 10
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
		t.Fatal("Get() returned nil")
	}
	for i := 1; i < numGoroutines; i++ {
		if results[i] != first {
			t.Errorf("Get() returned different pointer at index %d", i)
		}
	}
}

// contains is a small helper to check if a string contains a substring
func contains(s, substr string) bool {
	return len(s) >= len(substr) && searchString(s, substr)
}

func searchString(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// Ensure unused import doesn't cause issues
var _ = os.Getenv
