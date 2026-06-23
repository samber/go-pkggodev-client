package pkggodev_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	pkggodev "github.com/samber/go-pkggodev-client"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// modProxy serves the subset of the proxy protocol used by Dependencies and
// Module(WithSize): /<mod>/@latest, /<mod>/@v/<ver>.mod and a HEAD on
// /<mod>/@v/<ver>.zip. mods maps "<module>@<version>" to go.mod contents; latest
// maps a module path to its @latest version; sizes maps "<module>@<version>" to
// a zip size. Unknown paths return 404.
func modProxy(mods, latest map[string]string, sizes map[string]int64) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		path := strings.TrimPrefix(r.URL.Path, "/")
		switch {
		case strings.HasSuffix(path, "/@latest"):
			mod := strings.TrimSuffix(path, "/@latest")
			v, ok := latest[mod]
			if !ok {
				http.NotFound(w, r)
				return
			}
			_ = json.NewEncoder(w).Encode(map[string]string{"Version": v})
		case strings.Contains(path, "/@v/") && strings.HasSuffix(path, ".mod"):
			mod, ver := splitProxyV(path, ".mod")
			body, ok := mods[mod+"@"+ver]
			if !ok {
				http.NotFound(w, r)
				return
			}
			_, _ = w.Write([]byte(body))
		case strings.Contains(path, "/@v/") && strings.HasSuffix(path, ".zip"):
			mod, ver := splitProxyV(path, ".zip")
			size, ok := sizes[mod+"@"+ver]
			if !ok {
				http.NotFound(w, r)
				return
			}
			w.Header().Set("Content-Length", strconv.FormatInt(size, 10))
			w.WriteHeader(http.StatusOK)
		default:
			http.NotFound(w, r)
		}
	}
}

// splitProxyV splits "<module>/@v/<version><ext>" into module and version.
func splitProxyV(path, ext string) (mod, version string) {
	mod, rest, _ := strings.Cut(path, "/@v/")
	return mod, strings.TrimSuffix(rest, ext)
}

func TestDependencies_FullParse(t *testing.T) {
	t.Parallel()
	const gomod = `module github.com/samber/example

go 1.25

require (
	github.com/samber/lo v1.0.0
	github.com/stretchr/testify v1.11.1
)

require github.com/davecgh/go-spew v1.1.1 // indirect

replace github.com/old/pkg => github.com/new/pkg v1.2.3

exclude github.com/bad/pkg v0.9.0
`
	srv := httptest.NewServer(modProxy(
		map[string]string{"github.com/samber/example@v1.4.0": gomod},
		map[string]string{"github.com/samber/example": "v1.4.0"},
		nil,
	))
	t.Cleanup(srv.Close)
	c, err := pkggodev.New(pkggodev.WithGoproxy(srv.URL))
	require.NoError(t, err)

	res, err := c.Dependencies(context.Background(), "github.com/samber/example")
	require.NoError(t, err)

	assert.Equal(t, "github.com/samber/example", res.ModulePath)
	assert.Equal(t, "v1.4.0", res.Version) // resolved from @latest
	assert.Equal(t, "1.25", res.GoVersion)
	assert.Equal(t, []pkggodev.Dependency{
		{Path: "github.com/samber/lo", Version: "v1.0.0"},
		{Path: "github.com/stretchr/testify", Version: "v1.11.1"},
		{Path: "github.com/davecgh/go-spew", Version: "v1.1.1", Indirect: true},
	}, res.Requires)
	assert.Equal(t, []pkggodev.Replacement{
		{OldPath: "github.com/old/pkg", NewPath: "github.com/new/pkg", NewVersion: "v1.2.3"},
	}, res.Replaces)
	assert.Equal(t, []pkggodev.Dependency{
		{Path: "github.com/bad/pkg", Version: "v0.9.0"},
	}, res.Excludes)
}

func TestDependencies_ExplicitVersion(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(modProxy(
		map[string]string{"github.com/samber/do/v2@v2.0.0": "module github.com/samber/do/v2\n\ngo 1.18\n\nrequire github.com/samber/go-type-to-string v1.8.0\n"},
		nil, // no @latest: WithVersion must be honored without resolution
		nil,
	))
	t.Cleanup(srv.Close)
	c, err := pkggodev.New(pkggodev.WithGoproxy(srv.URL))
	require.NoError(t, err)

	res, err := c.Dependencies(context.Background(), "github.com/samber/do/v2", pkggodev.WithVersion("v2.0.0"))
	require.NoError(t, err)
	assert.Equal(t, "v2.0.0", res.Version)
	assert.Equal(t, "1.18", res.GoVersion)
	require.Len(t, res.Requires, 1)
	assert.Equal(t, "github.com/samber/go-type-to-string", res.Requires[0].Path)
}

func TestDependencies_NotFound(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(modProxy(nil, nil, nil))
	t.Cleanup(srv.Close)
	c, err := pkggodev.New(pkggodev.WithGoproxy(srv.URL))
	require.NoError(t, err)

	_, err = c.Dependencies(context.Background(), "example.com/missing")
	assert.ErrorIs(t, err, pkggodev.ErrModuleNotFound)
}

func TestDependencies_ProxyDisabled(t *testing.T) {
	t.Parallel()
	c, err := pkggodev.New(pkggodev.WithGoproxy("off"))
	require.NoError(t, err)

	_, err = c.Dependencies(context.Background(), "github.com/samber/lo")
	assert.ErrorIs(t, err, pkggodev.ErrProxyDisabled)
}

func TestModule_WithSize(t *testing.T) {
	t.Parallel()
	// One server plays both the pkg.go.dev API (/v1beta/...) and the module
	// proxy (everything else).
	proxy := modProxy(nil, nil, map[string]int64{"github.com/samber/lo@v1.4.0": 65031})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/v1beta/") {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"path":"github.com/samber/lo","version":"v1.4.0","goModContents":"module github.com/samber/lo\n\ngo 1.25\n"}`))
			return
		}
		proxy(w, r)
	}))
	t.Cleanup(srv.Close)

	c, err := pkggodev.New(pkggodev.WithBaseURL(srv.URL+"/v1beta"), pkggodev.WithGoproxy(srv.URL))
	require.NoError(t, err)

	// Without WithSize: no proxy call, Size stays zero; GoVersion comes free from go.mod.
	m, err := c.Module(context.Background(), "github.com/samber/lo")
	require.NoError(t, err)
	assert.Equal(t, int64(0), m.Size)
	assert.Equal(t, "1.25", m.GoVersion)

	// With WithSize: Content-Length of the proxy zip.
	m, err = c.Module(context.Background(), "github.com/samber/lo", pkggodev.WithSize())
	require.NoError(t, err)
	assert.Equal(t, int64(65031), m.Size)
	assert.Equal(t, "1.25", m.GoVersion)
}

func TestVersions_WithSize(t *testing.T) {
	t.Parallel()
	// One server: pkg.go.dev API for /v1beta/versions/... and the proxy for the
	// per-version zip HEAD requests.
	proxy := modProxy(nil, nil, map[string]int64{
		"github.com/samber/lo@v2.0.0": 200,
		"github.com/samber/lo@v1.0.0": 100,
	})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/v1beta/") {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"items":[` +
				`{"modulePath":"github.com/samber/lo","version":"v2.0.0"},` +
				`{"modulePath":"github.com/samber/lo","version":"v1.0.0"}],"total":2}`))
			return
		}
		proxy(w, r)
	}))
	t.Cleanup(srv.Close)

	c, err := pkggodev.New(pkggodev.WithBaseURL(srv.URL+"/v1beta"), pkggodev.WithGoproxy(srv.URL))
	require.NoError(t, err)

	// Without WithSize: no proxy calls, sizes stay zero.
	page, err := c.Versions(context.Background(), "github.com/samber/lo")
	require.NoError(t, err)
	require.Len(t, page.Items, 2)
	assert.Equal(t, int64(0), page.Items[0].Size)

	// With WithSize: each version carries its zip Content-Length.
	page, err = c.Versions(context.Background(), "github.com/samber/lo", pkggodev.WithSize())
	require.NoError(t, err)
	require.Len(t, page.Items, 2)
	assert.Equal(t, int64(200), page.Items[0].Size)
	assert.Equal(t, int64(100), page.Items[1].Size)
}

func TestVersions_WithSize_ProxyDisabled(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(jsonHandler(t, "",
		`{"items":[{"modulePath":"github.com/samber/lo","version":"v1.0.0"}],"total":1}`))
	t.Cleanup(srv.Close)
	c, err := pkggodev.New(pkggodev.WithBaseURL(srv.URL+"/v1beta"), pkggodev.WithGoproxy("off"))
	require.NoError(t, err)

	_, err = c.Versions(context.Background(), "github.com/samber/lo", pkggodev.WithSize())
	assert.ErrorIs(t, err, pkggodev.ErrProxyDisabled)
}

func TestModule_WithSize_ProxyDisabled(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(jsonHandler(t, "",
		`{"path":"github.com/samber/lo","version":"v1.4.0"}`))
	t.Cleanup(srv.Close)
	c, err := pkggodev.New(pkggodev.WithBaseURL(srv.URL+"/v1beta"), pkggodev.WithGoproxy("off"))
	require.NoError(t, err)

	_, err = c.Module(context.Background(), "github.com/samber/lo", pkggodev.WithSize())
	assert.ErrorIs(t, err, pkggodev.ErrProxyDisabled)
}
