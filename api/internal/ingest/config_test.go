package ingest

import "testing"

func TestCheckConfig(t *testing.T) {
	fields := []Field{{Name: "board", Pattern: BoardID}, {Name: "region", OneOf: []string{"eu"}, Optional: true}}
	for _, tc := range []struct {
		cfg map[string]any
		ok  bool
	}{
		{map[string]any{"board": "acme"}, true},
		{map[string]any{"board": "acme", "region": "eu", "company": "Acme"}, true},
		{map[string]any{}, false},
		{map[string]any{"board": ""}, false},
		{map[string]any{"board": 5}, false},
		{map[string]any{"board": "../x"}, false},
		{map[string]any{"board": "acme", "region": "us"}, false},
		{map[string]any{"board": "acme", "company": "  "}, false},
		{map[string]any{"board": "acme", "extra": "x"}, false},
	} {
		if err := CheckConfig(tc.cfg, fields...); (err == nil) != tc.ok {
			t.Errorf("CheckConfig(%v) = %v, want ok=%v", tc.cfg, err, tc.ok)
		}
	}
}
