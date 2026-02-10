package highlights

import "testing"

func TestScore_Table(t *testing.T) {
	tests := []struct {
		name     string
		text     string
		wantInfo bool
		wantHook bool
	}{
		{"empty", "", false, false},
		{"numbers", "Step 1: do X. Step 2: measure 42ms.", true, true},
		{"howto", "How to fix it: first do this, then do that.", true, false},
		{"hook", "Here is why this is important!", false, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			info, hook := Score(tt.text)
			if tt.wantInfo && info <= 0 {
				t.Fatalf("expected info>0, got %v", info)
			}
			if !tt.wantInfo && info != 0 {
				t.Fatalf("expected info==0, got %v", info)
			}
			if tt.wantHook && hook <= 0 {
				t.Fatalf("expected hook>0, got %v", hook)
			}
		})
	}
}
