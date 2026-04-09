package client

import (
	"os"
	"testing"
)

// --- StaticCredentialProvider tests ---

func TestStaticCredentialProvider_Valid(t *testing.T) {
	p := &StaticCredentialProvider{
		AccessKeyID:     "test-id",
		AccessKeySecret: "test-secret",
		SecurityToken:   "test-token",
	}

	id, err := p.GetAccessKeyID()
	if err != nil || id != "test-id" {
		t.Fatalf("GetAccessKeyID() = %q, %v; want %q, nil", id, err, "test-id")
	}

	secret, err := p.GetAccessKeySecret()
	if err != nil || secret != "test-secret" {
		t.Fatalf("GetAccessKeySecret() = %q, %v; want %q, nil", secret, err, "test-secret")
	}

	token, err := p.GetSecurityToken()
	if err != nil || token != "test-token" {
		t.Fatalf("GetSecurityToken() = %q, %v; want %q, nil", token, err, "test-token")
	}

	if !p.IsValid() {
		t.Fatal("IsValid() = false; want true")
	}
}

func TestStaticCredentialProvider_Empty(t *testing.T) {
	p := &StaticCredentialProvider{}

	_, err := p.GetAccessKeyID()
	if err != ErrNoCredentials {
		t.Fatalf("GetAccessKeyID() error = %v; want ErrNoCredentials", err)
	}

	_, err = p.GetAccessKeySecret()
	if err != ErrNoCredentials {
		t.Fatalf("GetAccessKeySecret() error = %v; want ErrNoCredentials", err)
	}

	// SecurityToken returns empty string without error (optional field).
	token, err := p.GetSecurityToken()
	if err != nil || token != "" {
		t.Fatalf("GetSecurityToken() = %q, %v; want %q, nil", token, err, "")
	}

	if p.IsValid() {
		t.Fatal("IsValid() = true; want false")
	}
}

func TestStaticCredentialProvider_PartiallyEmpty(t *testing.T) {
	// Only ID set, secret missing.
	p := &StaticCredentialProvider{AccessKeyID: "id-only"}
	if p.IsValid() {
		t.Fatal("IsValid() = true with missing secret; want false")
	}
	_, err := p.GetAccessKeySecret()
	if err != ErrNoCredentials {
		t.Fatalf("GetAccessKeySecret() error = %v; want ErrNoCredentials", err)
	}
}

// --- EnvCredentialProvider tests ---

func setEnvCredentials(t *testing.T, id, secret string) {
	t.Helper()
	t.Setenv("ALIBABA_CLOUD_ACCESS_KEY_ID", id)
	t.Setenv("ALIBABA_CLOUD_ACCESS_KEY_SECRET", secret)
}

func TestEnvCredentialProvider_Valid(t *testing.T) {
	setEnvCredentials(t, "env-id", "env-secret")
	t.Setenv("ALIBABA_CLOUD_SECURITY_TOKEN", "env-token")

	p := &EnvCredentialProvider{}

	id, err := p.GetAccessKeyID()
	if err != nil || id != "env-id" {
		t.Fatalf("GetAccessKeyID() = %q, %v; want %q, nil", id, err, "env-id")
	}

	secret, err := p.GetAccessKeySecret()
	if err != nil || secret != "env-secret" {
		t.Fatalf("GetAccessKeySecret() = %q, %v; want %q, nil", secret, err, "env-secret")
	}

	token, err := p.GetSecurityToken()
	if err != nil || token != "env-token" {
		t.Fatalf("GetSecurityToken() = %q, %v; want %q, nil", token, err, "env-token")
	}

	if !p.IsValid() {
		t.Fatal("IsValid() = false; want true")
	}
}

func TestEnvCredentialProvider_NotSet(t *testing.T) {
	// Ensure env vars are cleared.
	t.Setenv("ALIBABA_CLOUD_ACCESS_KEY_ID", "")
	t.Setenv("ALIBABA_CLOUD_ACCESS_KEY_SECRET", "")

	p := &EnvCredentialProvider{}

	_, err := p.GetAccessKeyID()
	if err != ErrNoCredentials {
		t.Fatalf("GetAccessKeyID() error = %v; want ErrNoCredentials", err)
	}

	_, err = p.GetAccessKeySecret()
	if err != ErrNoCredentials {
		t.Fatalf("GetAccessKeySecret() error = %v; want ErrNoCredentials", err)
	}

	if p.IsValid() {
		t.Fatal("IsValid() = true; want false")
	}
}

// --- ChainCredentialProvider tests ---

func TestChainCredentialProvider_CLIPriority(t *testing.T) {
	// Set env vars to different values.
	setEnvCredentials(t, "env-id", "env-secret")

	chain := NewCredentialProvider("cli-id", "cli-secret", "")

	id, err := chain.GetAccessKeyID()
	if err != nil || id != "cli-id" {
		t.Fatalf("GetAccessKeyID() = %q, %v; want %q, nil", id, err, "cli-id")
	}

	secret, err := chain.GetAccessKeySecret()
	if err != nil || secret != "cli-secret" {
		t.Fatalf("GetAccessKeySecret() = %q, %v; want %q, nil", secret, err, "cli-secret")
	}
}

func TestChainCredentialProvider_FallbackToEnv(t *testing.T) {
	setEnvCredentials(t, "env-id", "env-secret")

	// No CLI credentials — chain should fall back to env.
	chain := NewCredentialProvider("", "", "")

	id, err := chain.GetAccessKeyID()
	if err != nil || id != "env-id" {
		t.Fatalf("GetAccessKeyID() = %q, %v; want %q, nil", id, err, "env-id")
	}

	secret, err := chain.GetAccessKeySecret()
	if err != nil || secret != "env-secret" {
		t.Fatalf("GetAccessKeySecret() = %q, %v; want %q, nil", secret, err, "env-secret")
	}
}

func TestChainCredentialProvider_NoCredentials(t *testing.T) {
	// Clear env vars.
	t.Setenv("ALIBABA_CLOUD_ACCESS_KEY_ID", "")
	t.Setenv("ALIBABA_CLOUD_ACCESS_KEY_SECRET", "")

	chain := NewCredentialProvider("", "", "")

	_, err := chain.GetAccessKeyID()
	if err != ErrNoCredentials {
		t.Fatalf("GetAccessKeyID() error = %v; want ErrNoCredentials", err)
	}

	_, err = chain.GetAccessKeySecret()
	if err != ErrNoCredentials {
		t.Fatalf("GetAccessKeySecret() error = %v; want ErrNoCredentials", err)
	}
}

func TestChainCredentialProvider_SecurityTokenFromActiveProvider(t *testing.T) {
	t.Setenv("ALIBABA_CLOUD_ACCESS_KEY_ID", "env-id")
	t.Setenv("ALIBABA_CLOUD_ACCESS_KEY_SECRET", "env-secret")
	t.Setenv("ALIBABA_CLOUD_SECURITY_TOKEN", "env-token")

	// CLI credentials without token — chain should return token from CLI
	// provider (empty), not from env provider.
	chain := NewCredentialProvider("cli-id", "cli-secret", "")

	token, err := chain.GetSecurityToken()
	if err != nil {
		t.Fatalf("GetSecurityToken() error = %v; want nil", err)
	}
	// The static provider has no token set, so it returns "".
	if token != "" {
		t.Fatalf("GetSecurityToken() = %q; want empty (from static provider)", token)
	}
}

func TestChainCredentialProvider_SecurityTokenFallbackToEnv(t *testing.T) {
	t.Setenv("ALIBABA_CLOUD_ACCESS_KEY_ID", "env-id")
	t.Setenv("ALIBABA_CLOUD_ACCESS_KEY_SECRET", "env-secret")
	t.Setenv("ALIBABA_CLOUD_SECURITY_TOKEN", "env-token")

	// No CLI credentials — token should come from env.
	chain := NewCredentialProvider("", "", "")

	token, err := chain.GetSecurityToken()
	if err != nil {
		t.Fatalf("GetSecurityToken() error = %v; want nil", err)
	}
	if token != "env-token" {
		t.Fatalf("GetSecurityToken() = %q; want %q", token, "env-token")
	}
}

// --- NewCredentialProvider factory tests ---

func TestNewCredentialProvider_WithCLIParams(t *testing.T) {
	cp := NewCredentialProvider("my-id", "my-secret", "")
	chain, ok := cp.(*ChainCredentialProvider)
	if !ok {
		t.Fatal("NewCredentialProvider should return *ChainCredentialProvider")
	}
	// Should have 3 providers: static + env + sdk.
	if len(chain.Providers) < 2 {
		t.Fatalf("len(Providers) = %d; want at least 2", len(chain.Providers))
	}
	if _, ok := chain.Providers[0].(*StaticCredentialProvider); !ok {
		t.Fatal("first provider should be *StaticCredentialProvider")
	}
	if _, ok := chain.Providers[1].(*EnvCredentialProvider); !ok {
		t.Fatal("second provider should be *EnvCredentialProvider")
	}
}

func TestNewCredentialProvider_WithoutCLIParams(t *testing.T) {
	cp := NewCredentialProvider("", "", "")
	chain, ok := cp.(*ChainCredentialProvider)
	if !ok {
		t.Fatal("NewCredentialProvider should return *ChainCredentialProvider")
	}
	// Should have at least 1 provider: env (+ optional sdk).
	if len(chain.Providers) < 1 {
		t.Fatalf("len(Providers) = %d; want at least 1", len(chain.Providers))
	}
	if _, ok := chain.Providers[0].(*EnvCredentialProvider); !ok {
		t.Fatal("first provider should be *EnvCredentialProvider")
	}
}

// --- Error message clarity test ---

func TestErrNoCredentials_Message(t *testing.T) {
	msg := ErrNoCredentials.Error()
	// Verify the error message mentions both env vars and CLI flags.
	for _, substr := range []string{
		"ALIBABA_CLOUD_ACCESS_KEY_ID",
		"ALIBABA_CLOUD_ACCESS_KEY_SECRET",
		"--access-key-id",
		"--access-key-secret",
	} {
		if !contains(msg, substr) {
			t.Errorf("ErrNoCredentials message missing %q", substr)
		}
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && searchSubstring(s, substr)
}

func searchSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// --- Edge case: env var set to whitespace ---

func TestEnvCredentialProvider_WhitespaceEnvVar(t *testing.T) {
	// os.Getenv returns the raw value; whitespace-only is technically non-empty.
	// The provider treats any non-empty string as valid (trimming is the
	// caller's responsibility, matching the Python behaviour).
	t.Setenv("ALIBABA_CLOUD_ACCESS_KEY_ID", "  ")
	t.Setenv("ALIBABA_CLOUD_ACCESS_KEY_SECRET", "  ")

	p := &EnvCredentialProvider{}
	id, err := p.GetAccessKeyID()
	if err != nil {
		t.Fatalf("GetAccessKeyID() error = %v; want nil (whitespace is non-empty)", err)
	}
	if id != "  " {
		t.Fatalf("GetAccessKeyID() = %q; want %q", id, "  ")
	}
}

// --- Verify os.Unsetenv behaviour ---

func TestEnvCredentialProvider_UnsetVsEmpty(t *testing.T) {
	// Unset env var should behave the same as empty string for os.Getenv.
	os.Unsetenv("ALIBABA_CLOUD_ACCESS_KEY_ID")
	os.Unsetenv("ALIBABA_CLOUD_ACCESS_KEY_SECRET")

	p := &EnvCredentialProvider{}
	_, err := p.GetAccessKeyID()
	if err != ErrNoCredentials {
		t.Fatalf("GetAccessKeyID() with unset env = %v; want ErrNoCredentials", err)
	}
}
