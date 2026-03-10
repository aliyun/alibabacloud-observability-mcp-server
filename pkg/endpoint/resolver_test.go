package endpoint

import (
	"testing"
)

func TestNewSLSResolver(t *testing.T) {
	r := NewSLSResolver(nil)
	if r.template != slsTemplate {
		t.Errorf("expected template %q, got %q", slsTemplate, r.template)
	}
}

func TestNewCMSResolver(t *testing.T) {
	r := NewCMSResolver(nil)
	if r.template != cmsTemplate {
		t.Errorf("expected template %q, got %q", cmsTemplate, r.template)
	}
}

func TestResolve(t *testing.T) {
	tests := []struct {
		name      string
		resolver  *Resolver
		region    string
		want      string
		wantErr   bool
	}{
		{
			name:     "SLS cn-hangzhou",
			resolver: NewSLSResolver(nil),
			region:   "cn-hangzhou",
			want:     "cn-hangzhou.log.aliyuncs.com",
		},
		{
			name:     "SLS cn-shanghai",
			resolver: NewSLSResolver(nil),
			region:   "cn-shanghai",
			want:     "cn-shanghai.log.aliyuncs.com",
		},
		{
			name:     "CMS cn-hangzhou",
			resolver: NewCMSResolver(nil),
			region:   "cn-hangzhou",
			want:     "cms.cn-hangzhou.aliyuncs.com",
		},
		{
			name:     "CMS us-west-1",
			resolver: NewCMSResolver(nil),
			region:   "us-west-1",
			want:     "cms.us-west-1.aliyuncs.com",
		},
		{
			name:     "SLS empty region",
			resolver: NewSLSResolver(nil),
			region:   "",
			wantErr:  true,
		},
		{
			name:     "CMS empty region",
			resolver: NewCMSResolver(nil),
			region:   "",
			wantErr:  true,
		},
		{
			name:     "SLS override takes precedence",
			resolver: NewSLSResolver(map[string]string{"cn-hangzhou": "custom-sls.example.com"}),
			region:   "cn-hangzhou",
			want:     "custom-sls.example.com",
		},
		{
			name:     "CMS override takes precedence",
			resolver: NewCMSResolver(map[string]string{"cn-shanghai": "custom-cms.example.com"}),
			region:   "cn-shanghai",
			want:     "custom-cms.example.com",
		},
		{
			name:     "override falls back to template for other regions",
			resolver: NewSLSResolver(map[string]string{"cn-hangzhou": "custom.example.com"}),
			region:   "cn-beijing",
			want:     "cn-beijing.log.aliyuncs.com",
		},
		{
			name:     "override with protocol prefix is normalized",
			resolver: NewSLSResolver(map[string]string{"cn-hangzhou": "https://custom.example.com/"}),
			region:   "cn-hangzhou",
			want:     "custom.example.com",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := tt.resolver.Resolve(tt.region)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestNormalizeHost(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"plain host", "example.com", "example.com"},
		{"https prefix", "https://example.com", "example.com"},
		{"http prefix", "http://example.com", "example.com"},
		{"trailing slash", "example.com/", "example.com"},
		{"https and trailing slash", "https://example.com/", "example.com"},
		{"http and trailing slash", "http://example.com/", "example.com"},
		{"multiple trailing slashes", "example.com///", "example.com"},
		{"empty string", "", ""},
		{"only protocol", "https://", ""},
		{"host with port", "https://example.com:8080/", "example.com:8080"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := NormalizeHost(tt.input)
			if got != tt.want {
				t.Errorf("NormalizeHost(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestResolve_EmptyRegionErrorMessage(t *testing.T) {
	r := NewSLSResolver(nil)
	_, err := r.Resolve("")
	if err == nil {
		t.Fatal("expected error for empty region")
	}
	if got := err.Error(); got == "" {
		t.Error("error message should be descriptive, got empty string")
	}
}
