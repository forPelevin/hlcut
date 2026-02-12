package openrouter

import "testing"

func TestValidateBaseURL(t *testing.T) {
	tests := []struct {
		name         string
		baseURL      string
		allowedHosts []string
		wantErr      bool
	}{
		{
			name:    "default host with https",
			baseURL: "https://openrouter.ai",
		},
		{
			name:    "default api host with https",
			baseURL: "https://api.openrouter.ai",
		},
		{
			name:    "reject non-absolute URL",
			baseURL: "openrouter.ai",
			wantErr: true,
		},
		{
			name:    "reject http by default",
			baseURL: "http://openrouter.ai",
			wantErr: true,
		},
		{
			name:    "reject unknown host by default",
			baseURL: "https://evil.example",
			wantErr: true,
		},
		{
			name:         "allow configured host",
			baseURL:      "https://proxy.internal",
			allowedHosts: []string{"proxy.internal"},
		},
		{
			name:    "reject query",
			baseURL: "https://openrouter.ai?x=1",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateBaseURL(tt.baseURL, tt.allowedHosts)
			if tt.wantErr && err == nil {
				t.Fatalf("expected error, got nil")
			}
			if !tt.wantErr && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

func TestNormalizeAllowedHosts_DefaultWhenEmpty(t *testing.T) {
	out := normalizeAllowedHosts([]string{" ", "https://", "http://"})
	if len(out) != len(defaultAllowedHosts) {
		t.Fatalf("expected default allowed hosts, got %v", out)
	}
}
