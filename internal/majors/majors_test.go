package majors

import "testing"

func TestNormalizeBase(t *testing.T) {
	t.Parallel()
	tests := []struct {
		in       string
		wantBase string
		wantGopk bool
		wantOK   bool
	}{
		{"github.com/samber/do", "github.com/samber/do", false, true},
		{"github.com/samber/do/v2", "github.com/samber/do", false, true},
		{"github.com/samber/do/v2/", "github.com/samber/do", false, true},
		{"github.com/samber/do/v123", "github.com/samber/do", false, true},
		{"gopkg.in/yaml.v2", "gopkg.in/yaml", true, true},
		{"gopkg.in/check.v1", "gopkg.in/check", true, true},
		{"gopkg.in/yaml", "gopkg.in/yaml", true, true},
		// Not a major suffix: "vault" must not be stripped.
		{"github.com/hashicorp/vault", "github.com/hashicorp/vault", false, true},
		{"example.com/v2foo", "example.com/v2foo", false, true},
		{"", "", false, false},
		{"   ", "", false, false},
	}
	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
			t.Parallel()
			base, gopk, ok := normalizeBase(tt.in)
			if base != tt.wantBase || gopk != tt.wantGopk || ok != tt.wantOK {
				t.Errorf("normalizeBase(%q) = (%q, %v, %v); want (%q, %v, %v)",
					tt.in, base, gopk, ok, tt.wantBase, tt.wantGopk, tt.wantOK)
			}
		})
	}
}
