package pkggodev_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	pkggodev "github.com/samber/go-pkggodev-client"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeProxy serves the subset of the Go module proxy protocol used by
// MajorVersions: /<module>/@v/list and /<module>/@latest. lists maps a module
// path to its tagged versions; latest maps a module path to its @latest version.
// Unknown paths return 404 (the proxy's "not found").
func fakeProxy(lists map[string][]string, latest map[string]string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		path := strings.TrimPrefix(r.URL.Path, "/")
		switch {
		case strings.HasSuffix(path, "/@v/list"):
			mod := strings.TrimSuffix(path, "/@v/list")
			vs, ok := lists[mod]
			if !ok {
				http.NotFound(w, r)
				return
			}
			_, _ = w.Write([]byte(strings.Join(vs, "\n")))
		case strings.HasSuffix(path, "/@latest"):
			mod := strings.TrimSuffix(path, "/@latest")
			v, ok := latest[mod]
			if !ok {
				http.NotFound(w, r)
				return
			}
			_ = json.NewEncoder(w).Encode(map[string]string{"Version": v})
		default:
			http.NotFound(w, r)
		}
	}
}

func newProxyBackedClient(t *testing.T, lists map[string][]string, latest map[string]string) *pkggodev.Client {
	t.Helper()
	srv := httptest.NewServer(fakeProxy(lists, latest))
	t.Cleanup(srv.Close)
	c, err := pkggodev.New(pkggodev.WithGoproxy(srv.URL))
	require.NoError(t, err)
	return c
}

// majorOf flattens a page into a major->version map for easy assertions.
func majorOf(items []pkggodev.MajorVersion) map[string]string {
	m := make(map[string]string, len(items))
	for _, it := range items {
		m[it.Major] = it.Version
	}
	return m
}

func TestMajorVersions_BasePlusModuleAware(t *testing.T) {
	t.Parallel()
	c := newProxyBackedClient(t,
		map[string][]string{"github.com/samber/do": {"v1.0.0", "v1.6.0"}},
		map[string]string{"github.com/samber/do/v2": "v2.0.0"},
	)

	page, err := c.MajorVersions(context.Background(), "github.com/samber/do")
	require.NoError(t, err)
	require.Len(t, page.Items, 2)
	assert.Equal(t, 2, page.Total)

	// Newest major first, flagged IsLatest.
	assert.Equal(t, "v2", page.Items[0].Major)
	assert.Equal(t, "github.com/samber/do/v2", page.Items[0].ModulePath)
	assert.Equal(t, "v2.0.0", page.Items[0].Version)
	assert.True(t, page.Items[0].IsLatest)

	assert.Equal(t, "v1", page.Items[1].Major)
	assert.Equal(t, "github.com/samber/do", page.Items[1].ModulePath)
	assert.Equal(t, "v1.6.0", page.Items[1].Version)
	assert.False(t, page.Items[1].IsLatest)
}

func TestMajorVersions_NormalizesMajorSuffixInput(t *testing.T) {
	t.Parallel()
	c := newProxyBackedClient(t,
		map[string][]string{"github.com/samber/do": {"v1.6.0"}},
		map[string]string{"github.com/samber/do/v2": "v2.0.0"},
	)

	// Input already carries a /v2 suffix: it must be normalized to the base path.
	page, err := c.MajorVersions(context.Background(), "github.com/samber/do/v2")
	require.NoError(t, err)
	assert.Equal(t, map[string]string{"v1": "v1.6.0", "v2": "v2.0.0"}, majorOf(page.Items))
}

func TestMajorVersions_DifferentiatesV0AndV1(t *testing.T) {
	t.Parallel()
	c := newProxyBackedClient(t,
		map[string][]string{"example.com/m": {"v0.1.0", "v0.2.0", "v1.0.0", "v1.8.0"}},
		nil,
	)

	page, err := c.MajorVersions(context.Background(), "example.com/m")
	require.NoError(t, err)
	assert.Equal(t, map[string]string{"v0": "v0.2.0", "v1": "v1.8.0"}, majorOf(page.Items))
	assert.True(t, page.Items[0].IsLatest) // v1 is latest
	assert.Equal(t, "v1", page.Items[0].Major)
}

func TestMajorVersions_IncompatibleMajorsOnBasePath(t *testing.T) {
	t.Parallel()
	// v2..v4 live on the base path as +incompatible; v5 is a real module-aware major.
	c := newProxyBackedClient(t,
		map[string][]string{"example.com/m": {"v1.0.0", "v2.0.0+incompatible", "v3.0.0+incompatible", "v4.4.0+incompatible"}},
		map[string]string{"example.com/m/v5": "v5.4.0"},
	)

	page, err := c.MajorVersions(context.Background(), "example.com/m")
	require.NoError(t, err)
	assert.Equal(t, map[string]string{
		"v1": "v1.0.0",
		"v2": "v2.0.0+incompatible",
		"v3": "v3.0.0+incompatible",
		"v4": "v4.4.0+incompatible",
		"v5": "v5.4.0",
	}, majorOf(page.Items))

	// The incompatible majors keep the base path; v5 is a separate module.
	assert.Equal(t, "example.com/m", page.Items[4].ModulePath)    // v1
	assert.Equal(t, "example.com/m/v5", page.Items[0].ModulePath) // v5
	assert.True(t, page.Items[0].IsLatest)
}

func TestMajorVersions_NonContiguousGap(t *testing.T) {
	t.Parallel()
	// A gap at v2..v6 must not stop discovery before v7 (go-redis shape).
	c := newProxyBackedClient(t,
		map[string][]string{"example.com/m": {"v6.0.0+incompatible"}},
		map[string]string{
			"example.com/m/v7": "v7.4.1",
			"example.com/m/v8": "v8.11.5",
			"example.com/m/v9": "v9.20.1",
		},
	)

	page, err := c.MajorVersions(context.Background(), "example.com/m")
	require.NoError(t, err)
	assert.Equal(t, map[string]string{
		"v6": "v6.0.0+incompatible",
		"v7": "v7.4.1",
		"v8": "v8.11.5",
		"v9": "v9.20.1",
	}, majorOf(page.Items))
	assert.Equal(t, "v9", page.Items[0].Major)
	assert.True(t, page.Items[0].IsLatest)
}

func TestMajorVersions_GopkgIn(t *testing.T) {
	t.Parallel()
	c := newProxyBackedClient(t, nil, map[string]string{
		"gopkg.in/yaml.v2": "v2.4.0",
		"gopkg.in/yaml.v3": "v3.0.1",
	})

	page, err := c.MajorVersions(context.Background(), "gopkg.in/yaml.v2")
	require.NoError(t, err)
	require.Len(t, page.Items, 2)
	assert.Equal(t, map[string]string{"v2": "v2.4.0", "v3": "v3.0.1"}, majorOf(page.Items))
	assert.Equal(t, "gopkg.in/yaml.v3", page.Items[0].ModulePath)
	assert.True(t, page.Items[0].IsLatest)
}

func TestMajorVersions_ExcludePseudo(t *testing.T) {
	t.Parallel()
	// Module exists but has no tags: @v/list is empty and @latest is a pseudo-version.
	pseudo := "v0.0.0-20240101000000-abcdef123456"
	lists := map[string][]string{"example.com/m": {}}
	latest := map[string]string{"example.com/m": pseudo}

	c := newProxyBackedClient(t, lists, latest)
	page, err := c.MajorVersions(context.Background(), "example.com/m")
	require.NoError(t, err)
	require.Len(t, page.Items, 1)
	assert.Equal(t, pseudo, page.Items[0].Version)

	c2 := newProxyBackedClient(t, lists, latest)
	page2, err := c2.MajorVersions(context.Background(), "example.com/m", pkggodev.WithExcludePseudo())
	require.NoError(t, err)
	assert.Empty(t, page2.Items)
	assert.Equal(t, 0, page2.Total)
}

func TestMajorVersions_WithLimit(t *testing.T) {
	t.Parallel()
	c := newProxyBackedClient(t,
		map[string][]string{"example.com/m": {"v1.0.0", "v2.0.0+incompatible", "v3.0.0+incompatible"}},
		map[string]string{"example.com/m/v4": "v4.0.0"},
	)

	page, err := c.MajorVersions(context.Background(), "example.com/m", pkggodev.WithLimit(2))
	require.NoError(t, err)
	require.Len(t, page.Items, 2)
	assert.Equal(t, 4, page.Total) // Total reflects all discovered majors
	assert.Equal(t, "v4", page.Items[0].Major)
	assert.Equal(t, "v3", page.Items[1].Major)
}

func TestMajorVersions_WithFilter(t *testing.T) {
	t.Parallel()
	c := newProxyBackedClient(t,
		map[string][]string{"github.com/samber/do": {"v1.6.0"}},
		map[string]string{"github.com/samber/do/v2": "v2.0.0"},
	)

	page, err := c.MajorVersions(context.Background(), "github.com/samber/do", pkggodev.WithFilter(`/v2$`))
	require.NoError(t, err)
	require.Len(t, page.Items, 1)
	assert.Equal(t, "github.com/samber/do/v2", page.Items[0].ModulePath)
}

func TestMajorVersions_ProxyDisabled(t *testing.T) {
	t.Parallel()
	c, err := pkggodev.New(pkggodev.WithGoproxy("off"))
	require.NoError(t, err)

	_, err = c.MajorVersions(context.Background(), "github.com/samber/do")
	assert.ErrorIs(t, err, pkggodev.ErrProxyDisabled)
}
