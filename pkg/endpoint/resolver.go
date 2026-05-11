// Package endpoint provides endpoint resolution for Alibaba Cloud services.
package endpoint

import (
	"fmt"
	"strings"
)

const (
	// slsTemplate is the default SLS endpoint template.
	slsTemplate = "{region}.log.aliyuncs.com"
	// cmsTemplate is the default CMS endpoint template for workspace/SPL APIs.
	cmsTemplate = "cms.{region}.aliyuncs.com"
	// staropsTemplate is the default StarOps endpoint template for chat/thread APIs.
	staropsTemplate = "starops.{region}.aliyuncs.com"
)

// Resolver resolves service endpoints for a given region.
type Resolver struct {
	template  string
	overrides map[string]string
}

// NewSLSResolver creates a new Resolver for SLS endpoints.
func NewSLSResolver(overrides map[string]string) *Resolver {
	return &Resolver{
		template:  slsTemplate,
		overrides: overrides,
	}
}

// NewCMSResolver creates a new Resolver for CMS endpoints (workspace/SPL APIs).
func NewCMSResolver(overrides map[string]string) *Resolver {
	return &Resolver{
		template:  cmsTemplate,
		overrides: overrides,
	}
}

// NewStarOpsResolver creates a new Resolver for StarOps endpoints (chat/thread APIs).
func NewStarOpsResolver(overrides map[string]string) *Resolver {
	return &Resolver{
		template:  staropsTemplate,
		overrides: overrides,
	}
}

// Resolve returns the endpoint for the given region.
// It returns an error if region is empty.
// If an override exists for the region, it takes precedence over the template.
func (r *Resolver) Resolve(region string) (string, error) {
	if region == "" {
		return "", fmt.Errorf("endpoint: region ID must not be empty")
	}

	if r.overrides != nil {
		if ep, ok := r.overrides[region]; ok {
			return NormalizeHost(ep), nil
		}
	}

	return strings.ReplaceAll(r.template, "{region}", region), nil
}

// NormalizeHost strips protocol prefixes (http://, https://) and trailing
// slashes from a host string.
func NormalizeHost(host string) string {
	h := host
	h = strings.TrimPrefix(h, "https://")
	h = strings.TrimPrefix(h, "http://")
	h = strings.TrimRight(h, "/")
	return h
}
