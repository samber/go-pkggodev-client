// Package pkggodev is a typed Go client for the pkg.go.dev API.
//
// It wraps the ogen-generated client (under internal/api) with an
// ergonomic, context-first surface and clean response types.
package pkggodev

import (
	"net/http"

	"github.com/samber/go-pkggodev-client/internal/api"
)

// DefaultBaseURL is the production pkg.go.dev API base URL.
const DefaultBaseURL = api.DefaultBaseURL

// Client is the pkg.go.dev API api.
type Client struct {
	raw *api.Client
}

// ClientOption configures the Client built by New.
type ClientOption func(*clientConfig)

type clientConfig struct {
	baseURL   string
	http      *http.Client
	userAgent string
}

// WithBaseURL overrides the API base URL.
func WithBaseURL(u string) ClientOption { return func(c *clientConfig) { c.baseURL = u } }

// WithHTTPClient sets a custom *http.Client (timeouts, transport, etc.).
func WithHTTPClient(h *http.Client) ClientOption { return func(c *clientConfig) { c.http = h } }

// WithUserAgent sets the User-Agent header sent on every request.
func WithUserAgent(ua string) ClientOption { return func(c *clientConfig) { c.userAgent = ua } }

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
	if cfg.http != nil {
		raw = append(raw, api.WithHTTPClient(cfg.http))
	}

	c, err := api.New(raw...)
	if err != nil {
		return nil, err
	}
	return &Client{raw: c}, nil
}
