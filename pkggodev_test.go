package pkggodev_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	pkggodev "github.com/samber/go-pkggodev-client"
	"github.com/samber/mo"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newClient(t *testing.T, h http.HandlerFunc) *pkggodev.Client {
	t.Helper()
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	c, err := pkggodev.New(pkggodev.WithBaseURL(srv.URL + "/v1beta"))
	require.NoError(t, err)
	return c
}

func jsonHandler(t *testing.T, wantPath, body string) http.HandlerFunc {
	t.Helper()
	return func(w http.ResponseWriter, r *http.Request) {
		if wantPath != "" {
			assert.Equal(t, wantPath, r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(body))
	}
}

func TestPackage_CleanTypes(t *testing.T) {
	t.Parallel()
	c := newClient(t, jsonHandler(t, "/v1beta/package/github.com/samber/lo",
		`{"path":"github.com/samber/lo","name":"lo","isLatest":true}`))

	pkg, err := c.Package(context.Background(), "github.com/samber/lo")
	require.NoError(t, err)
	assert.Equal(t, "github.com/samber/lo", pkg.Path) // required field, plain string
	assert.Equal(t, "lo", pkg.Name.OrEmpty())         // optional field, mo.Option[string]
	assert.True(t, pkg.IsLatest)
}

func TestPackage_EmptyOptionalIsNone(t *testing.T) {
	t.Parallel()
	c := newClient(t, jsonHandler(t, "/v1beta/package/github.com/samber/lo",
		`{"path":"github.com/samber/lo","name":"","synopsis":"pkg","isLatest":true}`))

	pkg, err := c.Package(context.Background(), "github.com/samber/lo")
	require.NoError(t, err)
	assert.False(t, pkg.Name.IsPresent())          // empty string -> None
	assert.Equal(t, "pkg", pkg.Synopsis.OrEmpty()) // non-empty -> Some
}

func TestVersions_DecodeItems(t *testing.T) {
	t.Parallel()
	c := newClient(t, jsonHandler(t, "/v1beta/versions/github.com/samber/lo",
		`{"items":[{"version":"v1.0.0","commitTime":"2026-03-02T15:10:24Z","modulePath":"github.com/samber/lo"}],"total":1}`))

	page, err := c.Versions(context.Background(), "github.com/samber/lo")
	require.NoError(t, err)
	require.Len(t, page.Items, 1)
	assert.Equal(t, "v1.0.0", page.Items[0].Version)
	assert.Equal(t, 2026, page.Items[0].CommitTime.Year())
	assert.Equal(t, time.March, page.Items[0].CommitTime.Month())
	assert.Equal(t, 1, page.Total)
}

// newVulnClient builds a Client whose vulnerability database points at a test
// server (serving /index/modules.json and /ID/<id>.json).
func newVulnClient(t *testing.T, files map[string]string) *pkggodev.Client {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, ok := files[r.URL.Path]
		if !ok {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)
	c, err := pkggodev.New(pkggodev.WithVulnBaseURL(srv.URL))
	require.NoError(t, err)
	return c
}

const vulnTextOSV = `{"id":"GO-2021-0113","summary":"s","details":"d",` +
	`"aliases":["CVE-2021-38561","GHSA-ppp9-7jff-5vj2"],` +
	`"published":"2022-01-01T00:00:00Z","modified":"2024-05-20T16:03:47Z",` +
	`"affected":[{"package":{"name":"golang.org/x/text","ecosystem":"Go"},` +
	`"ranges":[{"type":"SEMVER","events":[{"introduced":"0"},{"fixed":"0.3.7"}]}],` +
	`"ecosystem_specific":{"imports":[{"path":"golang.org/x/text/language","symbols":["Parse"]}]}}],` +
	`"references":[{"type":"FIX","url":"https://go.dev/cl/12345"}],` +
	`"database_specific":{"url":"https://pkg.go.dev/vuln/GO-2021-0113","review_status":"REVIEWED"}}`

func vulnTextFiles() map[string]string {
	return map[string]string{
		"/index/modules.json": `[{"path":"golang.org/x/text","vulns":[` +
			`{"id":"GO-2021-0113","modified":"2024-05-20T16:03:47Z","fixed":"0.3.7"}]}]`,
		"/ID/GO-2021-0113.json": vulnTextOSV,
	}
}

func TestVulns_ModuleScoped(t *testing.T) {
	t.Parallel()
	c := newVulnClient(t, vulnTextFiles())

	vulns, err := c.Vulns(context.Background(), "golang.org/x/text")
	require.NoError(t, err)
	require.Len(t, vulns, 1)

	v := vulns[0]
	assert.Equal(t, "GO-2021-0113", v.ID)
	assert.Equal(t, []string{"CVE-2021-38561", "GHSA-ppp9-7jff-5vj2"}, v.Aliases)
	assert.Equal(t, "REVIEWED", v.ReviewStatus.OrEmpty())
	assert.Equal(t, "https://pkg.go.dev/vuln/GO-2021-0113", v.URL.OrEmpty())
	assert.True(t, v.Published.IsPresent())
	require.Len(t, v.References, 1)
	assert.Equal(t, "FIX", v.References[0].Type)

	// Versions and packages of the queried module are hoisted to the root.
	require.Len(t, v.Ranges, 1)
	assert.Equal(t, "0", v.Ranges[0].Introduced.OrEmpty())
	assert.Equal(t, "0.3.7", v.Ranges[0].Fixed.OrEmpty()) // per-interval fix (no scalar FixedVersion).
	require.Len(t, v.Packages, 1)
	assert.Equal(t, "golang.org/x/text/language", v.Packages[0].Path)
	assert.Equal(t, []string{"Parse"}, v.Packages[0].Symbols)
}

func TestVulns_NoVulns(t *testing.T) {
	t.Parallel()
	c := newVulnClient(t, map[string]string{"/index/modules.json": `[{"path":"other.example/x","vulns":[]}]`})

	vulns, err := c.Vulns(context.Background(), "golang.org/x/text")
	require.NoError(t, err)
	assert.Empty(t, vulns)
}

func TestVulns_VersionScoped(t *testing.T) {
	t.Parallel()
	c := newVulnClient(t, vulnTextFiles())

	// A version before the fix is affected; the covering fix lives in Ranges.
	affected, err := c.Vulns(context.Background(), "golang.org/x/text", pkggodev.WithVersion("v0.3.0"))
	require.NoError(t, err)
	require.Len(t, affected, 1)
	require.Len(t, affected[0].Ranges, 1)
	assert.Equal(t, "0.3.7", affected[0].Ranges[0].Fixed.OrEmpty())

	// A version at or after the fix is not affected.
	patched, err := c.Vulns(context.Background(), "golang.org/x/text", pkggodev.WithVersion("v0.3.7"))
	require.NoError(t, err)
	assert.Empty(t, patched)
}

func TestVulns_PackageScoped(t *testing.T) {
	t.Parallel()
	c := newVulnClient(t, vulnTextFiles())

	// The affected import path is returned.
	hit, err := c.Vulns(context.Background(), "golang.org/x/text/language")
	require.NoError(t, err)
	require.Len(t, hit, 1)
	assert.Equal(t, "GO-2021-0113", hit[0].ID)

	// A sibling package of the same module, not listed as affected, is filtered out.
	miss, err := c.Vulns(context.Background(), "golang.org/x/text/encoding")
	require.NoError(t, err)
	assert.Empty(t, miss)
}

func TestVulnerability_Marshal(t *testing.T) {
	t.Parallel()

	// Empty optional/slice fields are omitted thanks to the ,omitempty/,omitzero tags.
	bare, err := json.Marshal(pkggodev.Vulnerability{ID: "A"})
	require.NoError(t, err)
	assert.JSONEq(t, `{"id":"A","summary":"","details":""}`, string(bare))

	// Ranges marshal as an array of {introduced, fixed}; None boundaries are omitted.
	withRanges, err := json.Marshal(pkggodev.Vulnerability{
		ID:     "A",
		Ranges: []pkggodev.VersionRange{{Introduced: mo.Some("0"), Fixed: mo.Some("0.3.7")}},
	})
	require.NoError(t, err)
	assert.JSONEq(t, `{"id":"A","summary":"","details":"","ranges":[{"introduced":"0","fixed":"0.3.7"}]}`, string(withRanges))
}

func TestImportedBy_StringItems(t *testing.T) {
	t.Parallel()
	c := newClient(t, jsonHandler(t, "/v1beta/imported-by/github.com/samber/lo",
		`{"modulePath":"github.com/samber/lo","importedBy":{"items":["example.com/a","example.com/b"],"total":2}}`))

	res, err := c.ImportedBy(context.Background(), "github.com/samber/lo")
	require.NoError(t, err)
	assert.Equal(t, []string{"example.com/a", "example.com/b"}, res.Packages.Items)
	assert.Equal(t, 2, res.Packages.Total)
}

func TestAllVersions_AutoPaginates(t *testing.T) {
	t.Parallel()
	c := newClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Query().Get("token") {
		case "": // first page
			_, _ = w.Write([]byte(`{"items":[{"version":"v2.0.0"}],"nextPageToken":"p2","total":2}`))
		case "p2": // second (last) page
			_, _ = w.Write([]byte(`{"items":[{"version":"v1.0.0"}],"total":2}`))
		default:
			t.Errorf("unexpected token %q", r.URL.Query().Get("token"))
		}
	})

	var got []string
	for v, err := range c.AllVersions(context.Background(), "github.com/samber/lo") {
		require.NoError(t, err)
		got = append(got, v.Version)
	}
	assert.Equal(t, []string{"v2.0.0", "v1.0.0"}, got)
}
