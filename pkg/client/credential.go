// Package client provides Alibaba Cloud API client wrappers including
// credential management, SLS client, and CMS client.
package client

import (
	"errors"
	"fmt"
	"log/slog"
	"os"

	"github.com/aliyun/credentials-go/credentials"
)

// ErrNoCredentials is returned when no credential source provides valid
// AccessKey ID and Secret.
var ErrNoCredentials = errors.New("no credentials configured: set ALIBABA_CLOUD_ACCESS_KEY_ID/ALIBABA_CLOUD_ACCESS_KEY_SECRET environment variables or pass --access-key-id/--access-key-secret CLI flags")

// CredentialProvider is the interface for obtaining Alibaba Cloud credentials.
type CredentialProvider interface {
	// GetAccessKeyID returns the AccessKey ID.
	GetAccessKeyID() (string, error)
	// GetAccessKeySecret returns the AccessKey Secret.
	GetAccessKeySecret() (string, error)
	// GetSecurityToken returns the STS security token. Providers that do
	// not support STS return an empty string with no error.
	GetSecurityToken() (string, error)
}

// --- StaticCredentialProvider ---

// StaticCredentialProvider holds credentials passed directly (e.g. from CLI
// flags). It is the highest-priority provider in the credential chain.
type StaticCredentialProvider struct {
	AccessKeyID     string
	AccessKeySecret string
	SecurityToken   string
}

func (p *StaticCredentialProvider) GetAccessKeyID() (string, error) {
	if p.AccessKeyID == "" {
		return "", ErrNoCredentials
	}
	return p.AccessKeyID, nil
}

func (p *StaticCredentialProvider) GetAccessKeySecret() (string, error) {
	if p.AccessKeySecret == "" {
		return "", ErrNoCredentials
	}
	return p.AccessKeySecret, nil
}

func (p *StaticCredentialProvider) GetSecurityToken() (string, error) {
	return p.SecurityToken, nil
}

// IsValid returns true when both AccessKeyID and AccessKeySecret are non-empty.
func (p *StaticCredentialProvider) IsValid() bool {
	return p.AccessKeyID != "" && p.AccessKeySecret != ""
}

// --- EnvCredentialProvider ---

// EnvCredentialProvider reads credentials from environment variables
// ALIBABA_CLOUD_ACCESS_KEY_ID and ALIBABA_CLOUD_ACCESS_KEY_SECRET.
type EnvCredentialProvider struct{}

func (p *EnvCredentialProvider) GetAccessKeyID() (string, error) {
	v := os.Getenv("ALIBABA_CLOUD_ACCESS_KEY_ID")
	if v == "" {
		return "", ErrNoCredentials
	}
	return v, nil
}

func (p *EnvCredentialProvider) GetAccessKeySecret() (string, error) {
	v := os.Getenv("ALIBABA_CLOUD_ACCESS_KEY_SECRET")
	if v == "" {
		return "", ErrNoCredentials
	}
	return v, nil
}

func (p *EnvCredentialProvider) GetSecurityToken() (string, error) {
	return os.Getenv("ALIBABA_CLOUD_SECURITY_TOKEN"), nil
}

// IsValid returns true when both required env vars are set and non-empty.
func (p *EnvCredentialProvider) IsValid() bool {
	return os.Getenv("ALIBABA_CLOUD_ACCESS_KEY_ID") != "" &&
		os.Getenv("ALIBABA_CLOUD_ACCESS_KEY_SECRET") != ""
}

// --- ChainCredentialProvider ---

// ChainCredentialProvider tries each provider in order and returns the
// credentials from the first one that succeeds. This implements the
// credential chain auto-discovery pattern.
type ChainCredentialProvider struct {
	Providers []CredentialProvider
}

func (p *ChainCredentialProvider) GetAccessKeyID() (string, error) {
	for _, provider := range p.Providers {
		v, err := provider.GetAccessKeyID()
		if err == nil && v != "" {
			return v, nil
		}
	}
	return "", ErrNoCredentials
}

func (p *ChainCredentialProvider) GetAccessKeySecret() (string, error) {
	for _, provider := range p.Providers {
		v, err := provider.GetAccessKeySecret()
		if err == nil && v != "" {
			return v, nil
		}
	}
	return "", ErrNoCredentials
}

func (p *ChainCredentialProvider) GetSecurityToken() (string, error) {
	// Return the token from the first provider that has valid credentials.
	for _, provider := range p.Providers {
		// Check if this provider has valid key ID first.
		id, err := provider.GetAccessKeyID()
		if err == nil && id != "" {
			return provider.GetSecurityToken()
		}
	}
	return "", nil
}

// --- SDKCredentialProvider ---

// SDKCredentialProvider wraps the Alibaba Cloud credentials SDK default
// credential chain. It supports automatic credential discovery including:
//   - Environment variables (ALIBABA_CLOUD_ACCESS_KEY_ID, etc.)
//   - RAM Role ARN (ALIBABA_CLOUD_ROLE_ARN)
//   - ECS RAM Role (for ECS/FC instances)
//   - OIDC Role ARN
//   - Credentials file (~/.alibabacloud/credentials)
//
// This matches the Python SDK's CredClient() behavior.
type SDKCredentialProvider struct {
	cred credentials.Credential
}

// NewSDKCredentialProvider creates a provider using the Alibaba Cloud default
// credential chain. Returns nil if the SDK cannot initialize.
func NewSDKCredentialProvider() *SDKCredentialProvider {
	cred, err := credentials.NewCredential(nil)
	if err != nil {
		slog.Debug("sdk default credential chain not available", "error", err)
		return nil
	}
	return &SDKCredentialProvider{cred: cred}
}

func (p *SDKCredentialProvider) GetAccessKeyID() (string, error) {
	model, err := p.cred.GetCredential()
	if err != nil {
		return "", fmt.Errorf("sdk credential: %w", err)
	}
	if model.AccessKeyId == nil || *model.AccessKeyId == "" {
		return "", ErrNoCredentials
	}
	return *model.AccessKeyId, nil
}

func (p *SDKCredentialProvider) GetAccessKeySecret() (string, error) {
	model, err := p.cred.GetCredential()
	if err != nil {
		return "", fmt.Errorf("sdk credential: %w", err)
	}
	if model.AccessKeySecret == nil || *model.AccessKeySecret == "" {
		return "", ErrNoCredentials
	}
	return *model.AccessKeySecret, nil
}

func (p *SDKCredentialProvider) GetSecurityToken() (string, error) {
	model, err := p.cred.GetCredential()
	if err != nil {
		return "", fmt.Errorf("sdk credential: %w", err)
	}
	if model.SecurityToken == nil {
		return "", nil
	}
	return *model.SecurityToken, nil
}

// --- Factory ---

// NewCredentialProvider builds a ChainCredentialProvider with the standard
// priority order:
//  1. CLI parameters (static) — highest priority
//  2. Environment variables (ALIBABA_CLOUD_ACCESS_KEY_ID, etc.)
//  3. Alibaba Cloud SDK default credential chain (ECS RAM Role, OIDC, etc.)
//
// This matches the Python SDK's credential resolution behavior.
func NewCredentialProvider(accessKeyID, accessKeySecret, securityToken string) CredentialProvider {
	var providers []CredentialProvider

	// CLI / static credentials have highest priority.
	if accessKeyID != "" && accessKeySecret != "" {
		providers = append(providers, &StaticCredentialProvider{
			AccessKeyID:     accessKeyID,
			AccessKeySecret: accessKeySecret,
			SecurityToken:   securityToken,
		})
	}

	// Environment variables are next.
	providers = append(providers, &EnvCredentialProvider{})

	// Alibaba Cloud SDK default credential chain as final fallback.
	if sdkProvider := NewSDKCredentialProvider(); sdkProvider != nil {
		providers = append(providers, sdkProvider)
	}

	return &ChainCredentialProvider{Providers: providers}
}
