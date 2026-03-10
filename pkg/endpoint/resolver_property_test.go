package endpoint

import (
	"strings"
	"testing"

	"github.com/leanovate/gopter"
	"github.com/leanovate/gopter/gen"
	"github.com/leanovate/gopter/prop"
)

// genNonEmptyRegion generates non-empty region ID strings that resemble real
// Alibaba Cloud region identifiers (lowercase alphanumeric with hyphens).
var genNonEmptyRegion = gen.RegexMatch(`[a-z]{2}-[a-z]{3,10}-[0-9]`)

// TestProperty_EndpointTemplateResolution verifies that for any non-empty region ID
// and a Resolver with no overrides, SLS resolves to "{region}.log.aliyuncs.com"
// and CMS resolves to "cms.{region}.aliyuncs.com".
//
// Feature: go-mcp-server-rewrite, Property 5: 端点模板解析正确性
// Validates: Requirements 7.1, 7.2
func TestProperty_EndpointTemplateResolution(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100
	properties := gopter.NewProperties(parameters)

	properties.Property("SLS template resolves to {region}.log.aliyuncs.com", prop.ForAll(
		func(region string) bool {
			r := NewSLSResolver(nil)
			got, err := r.Resolve(region)
			if err != nil {
				return false
			}
			return got == region+".log.aliyuncs.com"
		},
		genNonEmptyRegion,
	))

	properties.Property("CMS template resolves to cms.{region}.aliyuncs.com", prop.ForAll(
		func(region string) bool {
			r := NewCMSResolver(nil)
			got, err := r.Resolve(region)
			if err != nil {
				return false
			}
			return got == "cms."+region+".aliyuncs.com"
		},
		genNonEmptyRegion,
	))

	properties.TestingRun(t)
}

// TestProperty_EndpointOverridePriority verifies that when a region has an
// override configured, the Resolver returns the override value (normalized)
// instead of the template-generated endpoint.
//
// Feature: go-mcp-server-rewrite, Property 6: 端点覆盖优先级
// Validates: Requirements 7.3
func TestProperty_EndpointOverridePriority(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100
	properties := gopter.NewProperties(parameters)

	// Generator for an override endpoint value (plain host, no protocol prefix or trailing slash).
	genOverrideHost := gen.RegexMatch(`[a-z]{3,8}\.[a-z]{3,8}\.com`)

	properties.Property("override takes precedence over SLS template", prop.ForAll(
		func(region, override string) bool {
			r := NewSLSResolver(map[string]string{region: override})
			got, err := r.Resolve(region)
			if err != nil {
				return false
			}
			expected := NormalizeHost(override)
			templateResult := region + ".log.aliyuncs.com"
			return got == expected && got != templateResult
		},
		genNonEmptyRegion,
		genOverrideHost,
	))

	properties.Property("override takes precedence over CMS template", prop.ForAll(
		func(region, override string) bool {
			r := NewCMSResolver(map[string]string{region: override})
			got, err := r.Resolve(region)
			if err != nil {
				return false
			}
			expected := NormalizeHost(override)
			templateResult := "cms." + region + ".aliyuncs.com"
			return got == expected && got != templateResult
		},
		genNonEmptyRegion,
		genOverrideHost,
	))

	properties.TestingRun(t)
}

// TestProperty_NormalizeHostIdempotent verifies that NormalizeHost is idempotent:
// applying it twice yields the same result as applying it once. Additionally,
// the normalized result must not contain "http://", "https://", or a trailing "/".
//
// Feature: go-mcp-server-rewrite, Property 7: 主机地址规范化幂等性
// Validates: Requirements 7.4
func TestProperty_NormalizeHostIdempotent(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100
	properties := gopter.NewProperties(parameters)

	// Generator for host strings that may include protocol prefixes and trailing slashes.
	genHostString := gen.OneGenOf(
		// Plain hosts
		gen.RegexMatch(`[a-z]{3,10}\.[a-z]{2,6}\.[a-z]{2,4}`),
		// With https:// prefix
		gen.RegexMatch(`[a-z]{3,10}\.[a-z]{2,6}\.[a-z]{2,4}`).Map(func(h string) string {
			return "https://" + h
		}),
		// With http:// prefix
		gen.RegexMatch(`[a-z]{3,10}\.[a-z]{2,6}\.[a-z]{2,4}`).Map(func(h string) string {
			return "http://" + h
		}),
		// With trailing slashes
		gen.RegexMatch(`[a-z]{3,10}\.[a-z]{2,6}\.[a-z]{2,4}`).Map(func(h string) string {
			return h + "///"
		}),
		// With prefix and trailing slash
		gen.RegexMatch(`[a-z]{3,10}\.[a-z]{2,6}\.[a-z]{2,4}`).Map(func(h string) string {
			return "https://" + h + "/"
		}),
	)

	properties.Property("NormalizeHost(NormalizeHost(h)) == NormalizeHost(h)", prop.ForAll(
		func(host string) bool {
			once := NormalizeHost(host)
			twice := NormalizeHost(once)
			if once != twice {
				return false
			}
			// Normalized result must not contain protocol prefixes or trailing slash
			if strings.HasPrefix(once, "http://") || strings.HasPrefix(once, "https://") {
				return false
			}
			if strings.HasSuffix(once, "/") {
				return false
			}
			return true
		},
		genHostString,
	))

	properties.TestingRun(t)
}
