// Package pkggodev is a typed Go client for the pkg.go.dev API.
//
// It wraps the ogen-generated client (under internal/api) with an
// ergonomic, context-first surface and clean response types.
package pkggodev

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/samber/go-singleflightx"

	"github.com/samber/go-pkggodev-client/internal/api"
	"github.com/samber/go-pkggodev-client/internal/majors"
	"github.com/samber/go-pkggodev-client/internal/proxy"
)

// DefaultBaseURL is the production pkg.go.dev API base URL.
const DefaultBaseURL = api.DefaultBaseURL

// ErrSymbolNotFound is returned by Client.Symbol when the requested symbol is
// absent from the package documentation.
var ErrSymbolNotFound = errors.New("pkggodev: symbol not found")

// ErrInvalidModulePath is returned when a module path cannot be parsed, e.g. by
// MajorVersions.
var ErrInvalidModulePath = errors.New("pkggodev: invalid module path")

// ErrProxyDisabled is returned by MajorVersions when no usable module proxy is
// configured (GOPROXY is "off" or resolves to "direct" only).
var ErrProxyDisabled = errors.New("pkggodev: no usable module proxy (GOPROXY)")

// Client is the pkg.go.dev API api.
type Client struct {
	raw   *api.Client
	proxy *proxy.Client
	sf    singleflightGroups
}

// singleflightGroups deduplicates concurrent, identical external calls: when
// several goroutines request the same endpoint with the same parameters at the
// same time, only one in-flight request hits the network (or the module proxy)
// and every caller receives the shared result.
//
// Each endpoint gets its own group typed on its return value, keyed by a string
// derived from the request parameters (see sfKey).
type singleflightGroups struct {
	search        singleflightx.Group[string, *Page[SearchResult]]
	pkg           singleflightx.Group[string, *Package]
	importedBy    singleflightx.Group[string, *ImportedByResult]
	packages      singleflightx.Group[string, *PackagesResult]
	module        singleflightx.Group[string, *Module]
	versions      singleflightx.Group[string, *Page[ModuleVersion]]
	symbols       singleflightx.Group[string, *Page[SymbolInfo]]
	symbol        singleflightx.Group[string, *Symbol]
	vulns         singleflightx.Group[string, *Page[Vulnerability]]
	majorVersions singleflightx.Group[string, []majors.Major]
}

// sfKey builds a singleflight deduplication key from an endpoint name and its
// request parameters. The parameter structs are comparable value types, so
// their %+v rendering is a stable identity for the request.
func sfKey(endpoint string, params ...any) string {
	return fmt.Sprintf("%s:%+v", endpoint, params)
}

// ClientOption configures the Client built by New.
type ClientOption func(*clientConfig)

type clientConfig struct {
	baseURL   string
	http      *http.Client
	userAgent string
	goproxy   string
}

// WithBaseURL overrides the API base URL.
func WithBaseURL(u string) ClientOption { return func(c *clientConfig) { c.baseURL = u } }

// WithHTTPClient sets a custom *http.Client (timeouts, transport, etc.).
func WithHTTPClient(h *http.Client) ClientOption { return func(c *clientConfig) { c.http = h } }

// WithUserAgent sets the User-Agent header sent on every request.
func WithUserAgent(ua string) ClientOption { return func(c *clientConfig) { c.userAgent = ua } }

// WithGoproxy overrides the module proxy list used by MajorVersions. The value
// uses the same syntax as the GOPROXY environment variable (a "," or "|"
// separated list of base URLs, plus the keywords "direct" and "off"). When
// unset, the client honors the GOPROXY environment variable, defaulting to
// https://proxy.golang.org.
func WithGoproxy(s string) ClientOption { return func(c *clientConfig) { c.goproxy = s } }

// New builds a pkg.go.dev client with sane defaults.
func New(opts ...ClientOption) (*Client, error) {
	cfg := clientConfig{baseURL: DefaultBaseURL, userAgent: "go-pkggodev-client"}
	for _, o := range opts {
		o(&cfg)
	}

	raw := []api.Opt{
		api.WithBaseURL(cfg.baseURL),
		api.WithUserAgent(cfg.userAgent),
	}
	httpClient := cfg.http
	if httpClient == nil {
		httpClient = http.DefaultClient
	}
	if cfg.http != nil {
		raw = append(raw, api.WithHTTPClient(cfg.http))
	}

	c, err := api.New(raw...)
	if err != nil {
		return nil, err
	}
	return &Client{
		raw:   c,
		proxy: proxy.New(httpClient, cfg.userAgent, proxy.ResolveGoproxy(cfg.goproxy)),
	}, nil
}
