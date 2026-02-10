package openrouter

import "testing"

func TestExtractJSONObject(t *testing.T) {
	tests := []struct {
		name    string
		in      string
		wantSub string
		wantErr bool
	}{
		{"raw", `{"clips":[{"idx":0,"start_sec":0,"end_sec":1,"title":"t","caption":"c","tags":[],"reason":"r"}]}`, `"clips"`, false},
		{"fenced", "```json\n{\"clips\":[]}\n```", `"clips"`, false},
		{"preface", "sure! {\"clips\":[]} thanks", `"clips"`, false},
		{"empty", "   ", "", true},
		{"nojson", "hello", "", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := extractJSONObject(tt.in)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if tt.wantSub != "" && !contains(got, tt.wantSub) {
				t.Fatalf("expected %q to contain %q", got, tt.wantSub)
			}
		})
	}
}

func contains(s, sub string) bool {
	return len(sub) == 0 || (len(s) >= len(sub) && (index(s, sub) >= 0))
}

func index(s, sub string) int {
	// tiny helper to avoid importing strings just for tests
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
