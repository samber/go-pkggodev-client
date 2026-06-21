// Package proxy is a minimal Go module proxy client
// (https://go.dev/ref/mod#goproxy-protocol). It is used to discover the major
// versions of a module, since pkg.go.dev does not (yet) expose a MajorVersions
// endpoint (golang/go#76718).
package proxy

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"

	"golang.org/x/mod/module"
)

// ErrDisabled is returned when no usable proxy is configured (GOPROXY is "off"
// or resolves to "direct" only).
var ErrDisabled = errors.New("proxy: no usable module proxy (GOPROXY)")

// ErrInvalidModulePath is returned when a module path cannot be escaped for the
// proxy protocol.
var ErrInvalidModulePath = errors.New("proxy: invalid module path")

// defaultGoproxy is the public module proxy used when GOPROXY is unset.
const defaultGoproxy = "https://proxy.golang.org"

// Client talks the Go module proxy protocol against an ordered list of proxies.
type Client struct {
	http      *http.Client
	userAgent string
	bases     []string // ordered proxy base URLs (https?://...)
}

// New builds a proxy Client. bases is an ordered list of proxy base URLs
// (typically from ResolveGoproxy). A nil http client falls back to
// http.DefaultClient.
func New(h *http.Client, userAgent string, bases []string) *Client {
	if h == nil {
		h = http.DefaultClient
	}
	return &Client{http: h, userAgent: userAgent, bases: bases}
}

// Enabled reports whether at least one usable proxy is configured.
func (c *Client) Enabled() bool { return len(c.bases) > 0 }

// ResolveGoproxy turns a GOPROXY-syntax string into an ordered list of HTTP
// proxy base URLs, dropping the "direct" and "off" keywords. When s is empty it
// falls back to the GOPROXY environment variable, then to the public proxy.
func ResolveGoproxy(s string) []string {
	if s == "" {
		s = os.Getenv("GOPROXY")
	}
	if s == "" {
		s = defaultGoproxy
	}
	fields := strings.FieldsFunc(s, func(r rune) bool { return r == ',' || r == '|' })
	bases := make([]string, 0, len(fields))
	for _, f := range fields {
		f = strings.TrimSpace(f)
		if f == "" || f == "direct" || f == "off" {
			continue
		}
		if strings.HasPrefix(f, "https://") || strings.HasPrefix(f, "http://") {
			bases = append(bases, strings.TrimRight(f, "/"))
		}
	}
	return bases
}

// get fetches escapedPath+suffix from each proxy in turn. A 404/410 means "not
// found, try the next proxy" per the proxy protocol; if every proxy reports not
// found, get returns ok=false with a nil error. Any other status (or transport
// error) is remembered and returned only if no proxy ultimately answers.
func (c *Client) get(ctx context.Context, escapedPath, suffix string) (body []byte, ok bool, err error) {
	if len(c.bases) == 0 {
		return nil, false, ErrDisabled
	}
	var lastErr error
	for _, base := range c.bases {
		url := base + "/" + escapedPath + suffix
		req, reqErr := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		if reqErr != nil {
			return nil, false, reqErr
		}
		if c.userAgent != "" {
			req.Header.Set("User-Agent", c.userAgent)
		}
		resp, doErr := c.http.Do(req)
		if doErr != nil {
			lastErr = doErr
			continue
		}
		data, readErr := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		switch resp.StatusCode {
		case http.StatusOK:
			if readErr != nil {
				return nil, false, readErr
			}
			return data, true, nil
		case http.StatusNotFound, http.StatusGone:
			continue // not found on this proxy; try the next one
		case http.StatusTooManyRequests:
			lastErr = fmt.Errorf("module proxy rate limited (429): %s", url)
		default:
			lastErr = fmt.Errorf("module proxy %s: unexpected status %d", url, resp.StatusCode)
		}
	}
	if lastErr != nil {
		return nil, false, lastErr
	}
	return nil, false, nil
}

// versionInfo is the JSON payload of a proxy @latest / @v/<ver>.info response.
type versionInfo struct {
	Version string `json:"Version"`
}

// Latest returns the version reported by <proxy>/<module>/@latest. ok is false
// when the module (at this exact path) is unknown to every proxy.
func (c *Client) Latest(ctx context.Context, modulePath string) (version string, ok bool, err error) {
	esc, err := module.EscapePath(modulePath)
	if err != nil {
		return "", false, fmt.Errorf("%w: %q", ErrInvalidModulePath, modulePath)
	}
	body, ok, err := c.get(ctx, esc, "/@latest")
	if err != nil || !ok {
		return "", ok, err
	}
	var v versionInfo
	if err := json.Unmarshal(body, &v); err != nil {
		return "", false, err
	}
	return v.Version, true, nil
}

// List returns the tagged versions from <proxy>/<module>/@v/list (one per
// line). The proxy omits pseudo-versions from this list. ok is false when the
// module path is unknown to every proxy.
func (c *Client) List(ctx context.Context, modulePath string) (versions []string, ok bool, err error) {
	esc, err := module.EscapePath(modulePath)
	if err != nil {
		return nil, false, fmt.Errorf("%w: %q", ErrInvalidModulePath, modulePath)
	}
	body, ok, err := c.get(ctx, esc, "/@v/list")
	if err != nil || !ok {
		return nil, ok, err
	}
	for line := range strings.SplitSeq(string(body), "\n") {
		if line = strings.TrimSpace(line); line != "" {
			versions = append(versions, line)
		}
	}
	return versions, true, nil
}
