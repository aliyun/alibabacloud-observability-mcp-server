// Package client provides Alibaba Cloud API client wrappers including
// credential management, SLS client, and CMS client.
package client

import (
	"errors"
	"os"
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

// --- Factory ---

// NewCredentialProvider builds a ChainCredentialProvider with the standard
// priority order: CLI parameters (static) > environment variables.
// If accessKeyID and accessKeySecret are non-empty, a StaticCredentialProvider
// is placed first in the chain.
func NewCredentialProvider(accessKeyID, accessKeySecret string) CredentialProvider {
	var providers []CredentialProvider

	// CLI / static credentials have highest priority.
	if accessKeyID != "" && accessKeySecret != "" {
		providers = append(providers, &StaticCredentialProvider{
			AccessKeyID:     accessKeyID,
			AccessKeySecret: accessKeySecret,
		})
	}

	// Environment variables are next.
	providers = append(providers, &EnvCredentialProvider{})

	return &ChainCredentialProvider{Providers: providers}
}
