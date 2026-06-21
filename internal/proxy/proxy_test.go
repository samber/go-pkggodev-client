package proxy

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestResolveGoproxy(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		in   string
		want []string
	}{
		{"default public proxy", defaultGoproxy, []string{defaultGoproxy}},
		{"trailing slash trimmed", defaultGoproxy + "/", []string{defaultGoproxy}},
		{"comma list with direct dropped", "https://a.example,https://b.example,direct", []string{"https://a.example", "https://b.example"}},
		{"pipe separator", "https://a.example|https://b.example", []string{"https://a.example", "https://b.example"}},
		{"off only", "off", []string{}},
		{"direct only", "direct", []string{}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.want, ResolveGoproxy(tt.in))
		})
	}
}

func TestClientEnabled(t *testing.T) {
	t.Parallel()
	assert.True(t, New(nil, "", []string{defaultGoproxy}).Enabled())
	assert.False(t, New(nil, "", nil).Enabled())
}
