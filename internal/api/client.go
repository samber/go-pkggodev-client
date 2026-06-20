package api

import "net/http"

// DefaultBaseURL is the production pkg.go.dev API base URL.
const DefaultBaseURL = "https://pkg.go.dev/v1beta"

type options struct {
	baseURL    string
	httpClient *http.Client
	userAgent  string
}

// Opt configures the client built by New.
type Opt func(*options)

// WithBaseURL overrides the API base URL (useful for tests or proxies).
func WithBaseURL(u string) Opt { return func(o *options) { o.baseURL = u } }

// WithHTTPClient sets a custom *http.Client (timeouts, transport, etc.).
func WithHTTPClient(c *http.Client) Opt { return func(o *options) { o.httpClient = c } }

// WithUserAgent sets the User-Agent header sent on every request.
func WithUserAgent(ua string) Opt { return func(o *options) { o.userAgent = ua } }

// New builds a configured pkg.go.dev client with sane defaults.
func New(opts ...Opt) (*Client, error) {
	o := options{
		baseURL:    DefaultBaseURL,
		httpClient: &http.Client{},
		userAgent:  "go-pkggodev-client",
	}
	for _, fn := range opts {
		fn(&o)
	}

	// Copy the client so we can inject a User-Agent transport without mutating the caller's value.
	hc := *o.httpClient
	base := hc.Transport
	if base == nil {
		base = http.DefaultTransport
	}
	hc.Transport = &uaTransport{rt: base, ua: o.userAgent}

	return NewClient(o.baseURL, WithClient(&hc))
}

// uaTransport injects a User-Agent header on every outgoing request.
type uaTransport struct {
	rt http.RoundTripper
	ua string
}

func (t *uaTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	r.Header.Set("User-Agent", t.ua)
	return t.rt.RoundTrip(r)
}
