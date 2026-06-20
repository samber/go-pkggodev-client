package pkggodev_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	pkggodev "github.com/samber/go-pkggodev-client"
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
	assert.Equal(t, "github.com/samber/lo", pkg.Path) // plain string, no .Value
	assert.Equal(t, "lo", pkg.Name)
	assert.True(t, pkg.IsLatest)
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

func TestVulns_NullItems(t *testing.T) {
	t.Parallel()
	c := newClient(t, jsonHandler(t, "/v1beta/vulns/golang.org/x/text",
		`{"items":null,"total":0}`))

	page, err := c.Vulns(context.Background(), "golang.org/x/text")
	require.NoError(t, err)
	assert.Empty(t, page.Items)
	assert.Equal(t, 0, page.Total)
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
