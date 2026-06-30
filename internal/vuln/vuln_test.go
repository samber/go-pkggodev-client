package vuln

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidID(t *testing.T) {
	t.Parallel()
	tests := []struct {
		in   string
		want bool
	}{
		{"GO-2022-0191", true},
		{"GO-2026-4461", true},
		{"GO-2024-12345", true}, // variable-width entry number
		{"GO-22-0191", false},
		{"GO-2022-", false},
		{"CVE-2021-38561", false},
		{"../ID/secret", false},
		{"", false},
	}
	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.want, ValidID(tt.in))
		})
	}
}

func TestIsSemver(t *testing.T) {
	t.Parallel()
	assert.True(t, IsSemver("0.3.7"))    // bare
	assert.True(t, IsSemver("v0.3.7"))   // v-prefixed
	assert.True(t, IsSemver("1.11.0-0")) // prerelease boundary
	assert.False(t, IsSemver("latest"))
	assert.False(t, IsSemver("master"))
	assert.False(t, IsSemver(""))
}

func TestRangeAffected(t *testing.T) {
	t.Parallel()
	// Single segment [0, 1.10.6): everything before the fix is affected.
	single := []Event{{Introduced: "0"}, {Fixed: "1.10.6"}}
	// Two segments [0, 1.10.6) and [1.11.0-0, 1.11.3).
	multi := []Event{{Introduced: "0"}, {Fixed: "1.10.6"}, {Introduced: "1.11.0-0"}, {Fixed: "1.11.3"}}

	tests := []struct {
		name     string
		events   []Event
		version  string
		affected bool
		fixed    string
	}{
		{"before fix", single, "1.10.0", true, "1.10.6"},
		{"at fix", single, "1.10.6", false, ""},
		{"after fix", single, "1.10.7", false, ""},
		{"bare version", single, "v1.10.0", true, "1.10.6"},
		{"first segment", multi, "1.9.0", true, "1.10.6"},
		{"gap between segments", multi, "1.10.6", false, ""},
		{"second segment", multi, "1.11.1", true, "1.11.3"},
		{"after second fix", multi, "1.11.3", false, ""},
		{"non-semver", single, "latest", false, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			affected, fixed := RangeAffected(tt.events, tt.version)
			assert.Equal(t, tt.affected, affected)
			assert.Equal(t, tt.fixed, fixed)
		})
	}
}

func newTestClient(t *testing.T, files map[string]string) *Client {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, ok := files[r.URL.Path]
		if !ok {
			http.NotFound(w, r)
			return
		}
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)
	return New(srv.Client(), "test-agent", srv.URL)
}

func TestEntry(t *testing.T) {
	t.Parallel()
	c := newTestClient(t, map[string]string{
		"/ID/GO-2022-0191.json": `{"id":"GO-2022-0191","summary":"s","aliases":["CVE-2018-16875"],` +
			`"affected":[{"package":{"name":"stdlib","ecosystem":"Go"},` +
			`"ranges":[{"type":"SEMVER","events":[{"introduced":"0"},{"fixed":"1.10.6"}]}],` +
			`"ecosystem_specific":{"imports":[{"path":"crypto/x509","symbols":["Certificate.buildChains"]}]}}],` +
			`"database_specific":{"url":"https://pkg.go.dev/vuln/GO-2022-0191","review_status":"REVIEWED"}}`,
	})

	e, ok, err := c.Entry(context.Background(), "GO-2022-0191")
	require.NoError(t, err)
	require.True(t, ok)
	assert.Equal(t, "GO-2022-0191", e.ID)
	assert.Equal(t, []string{"CVE-2018-16875"}, e.Aliases)
	require.Len(t, e.Affected, 1)
	assert.Equal(t, "stdlib", e.Affected[0].Module.Path)
	assert.Equal(t, "Go", e.Affected[0].Module.Ecosystem)
	require.Len(t, e.Affected[0].EcosystemSpecific.Imports, 1)
	assert.Equal(t, "crypto/x509", e.Affected[0].EcosystemSpecific.Imports[0].Path)
	require.NotNil(t, e.DatabaseSpecific)
	assert.Equal(t, ReviewStatusReviewed, e.DatabaseSpecific.ReviewStatus)
}

func TestEntry_NotFound(t *testing.T) {
	t.Parallel()
	c := newTestClient(t, nil) // every path 404s

	e, ok, err := c.Entry(context.Background(), "GO-2099-9999")
	require.NoError(t, err)
	assert.False(t, ok)
	assert.Nil(t, e)
}

func TestEntry_InvalidID(t *testing.T) {
	t.Parallel()
	c := New(nil, "", "")

	_, _, err := c.Entry(context.Background(), "not-an-id")
	require.ErrorIs(t, err, ErrInvalidID)
}

func TestModules(t *testing.T) {
	t.Parallel()
	c := newTestClient(t, map[string]string{
		"/index/modules.json": `[{"path":"golang.org/x/text","vulns":[` +
			`{"id":"GO-2021-0113","modified":"2024-05-20T16:03:47Z","fixed":"0.3.7"},` +
			`{"id":"GO-2099-0001","modified":"2024-05-20T16:03:47Z"}]}]`,
	})

	mods, err := c.Modules(context.Background())
	require.NoError(t, err)
	require.Len(t, mods, 1)
	assert.Equal(t, "golang.org/x/text", mods[0].Path)
	require.Len(t, mods[0].Vulns, 2)
	assert.Equal(t, "GO-2021-0113", mods[0].Vulns[0].ID)
	assert.Equal(t, "0.3.7", mods[0].Vulns[0].Fixed)
	assert.Empty(t, mods[0].Vulns[1].Fixed) // no fix mapped
}
