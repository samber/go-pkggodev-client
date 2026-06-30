// Package vuln is a minimal client for the Go vulnerability database
// (https://vuln.go.dev). The database is served as static JSON files: a per-ID
// OSV report (/ID/<id>.json) plus index files used for triage
// (/index/modules.json). There are no query parameters, no auth and no POST.
//
// The types mirror the published Go-OSV format (a subset of the OSV schema with
// Go-specific extensions); see golang.org/x/vuln/internal/osv.
package vuln

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"

	"golang.org/x/mod/semver"
)

// DefaultBaseURL is the production Go vulnerability database base URL.
const DefaultBaseURL = "https://vuln.go.dev"

// Ecosystem is the OSV ecosystem of every Go entry.
const Ecosystem = "Go"

// ModuleStdlib and ModuleToolchain are the pseudo-module paths the database
// uses for the standard library and the go command.
const (
	ModuleStdlib    = "stdlib"
	ModuleToolchain = "toolchain"
)

// ReviewStatusReviewed and ReviewStatusUnreviewed are the published values of
// DatabaseSpecific.ReviewStatus. Unreviewed entries are auto-generated and may
// lack affected symbols (carrying custom_ranges instead of imports).
const (
	ReviewStatusReviewed   = "REVIEWED"
	ReviewStatusUnreviewed = "UNREVIEWED"
)

// ErrInvalidID is returned when an ID is not a well-formed GO-YYYY-NNNN
// identifier. It is also a guard: the ID is interpolated into a request path.
var ErrInvalidID = errors.New("vuln: invalid vulnerability id")

// idPattern matches a Go vulnerability ID, e.g. "GO-2022-0191". The entry
// number is variable-width (older entries are 4 digits, newer ones can be more).
var idPattern = regexp.MustCompile(`^GO-\d{4}-\d+$`)

// ValidID reports whether id is a well-formed Go vulnerability identifier.
func ValidID(id string) bool { return idPattern.MatchString(id) }

// Client talks to the Go vulnerability database over HTTP.
type Client struct {
	http      *http.Client
	userAgent string
	baseURL   string // no trailing slash
}

// New builds a vuln Client. A nil http client falls back to http.DefaultClient;
// an empty baseURL falls back to DefaultBaseURL.
func New(h *http.Client, userAgent, baseURL string) *Client {
	if h == nil {
		h = http.DefaultClient
	}
	if baseURL == "" {
		baseURL = DefaultBaseURL
	}
	return &Client{http: h, userAgent: userAgent, baseURL: strings.TrimRight(baseURL, "/")}
}

// --- OSV report (/ID/<id>.json) ---

// Entry is a single OSV vulnerability report.
type Entry struct {
	SchemaVersion    string            `json:"schema_version"`
	ID               string            `json:"id"`
	Modified         time.Time         `json:"modified"`
	Published        time.Time         `json:"published"`
	Withdrawn        *time.Time        `json:"withdrawn"`
	Aliases          []string          `json:"aliases"` // CVE / GHSA identifiers.
	Summary          string            `json:"summary"`
	Details          string            `json:"details"`
	Affected         []Affected        `json:"affected"`
	References       []Reference       `json:"references"`
	Credits          []Credit          `json:"credits"`
	DatabaseSpecific *DatabaseSpecific `json:"database_specific"`
}

// Affected describes how one module is affected. The OSV key is "package" but
// the value is a module (its Module.Path).
type Affected struct {
	Module            Module            `json:"package"`
	Ranges            []Range           `json:"ranges"`
	EcosystemSpecific EcosystemSpecific `json:"ecosystem_specific"`
}

// Module is the affected module: Path is the module path (or ModuleStdlib /
// ModuleToolchain); Ecosystem is always Ecosystem ("Go").
type Module struct {
	Path      string `json:"name"`
	Ecosystem string `json:"ecosystem"`
}

// Range is a version range. Type is "SEMVER".
type Range struct {
	Type   string  `json:"type"`
	Events []Event `json:"events"`
}

// Event is one boundary of a Range: an Introduced or a Fixed version (bare
// semver, no "v" prefix). Introduced "0" means "from the beginning".
type Event struct {
	Introduced string `json:"introduced,omitempty"`
	Fixed      string `json:"fixed,omitempty"`
}

// EcosystemSpecific carries the Go-specific affected packages. Imports is empty
// on auto-generated UNREVIEWED entries (which use custom_ranges instead).
type EcosystemSpecific struct {
	Imports []Package `json:"imports"`
}

// Package is one affected import path, optionally narrowed to symbols / build
// constraints.
type Package struct {
	Path    string   `json:"path"`
	GOOS    []string `json:"goos"`
	GOARCH  []string `json:"goarch"`
	Symbols []string `json:"symbols"`
}

// Reference is an external link. Type is one of ADVISORY, ARTICLE, REPORT, FIX,
// PACKAGE, EVIDENCE, WEB.
type Reference struct {
	Type string `json:"type"`
	URL  string `json:"url"`
}

// Credit is a person or organization credited for the report.
type Credit struct {
	Name string `json:"name"`
}

// DatabaseSpecific holds Go database metadata: the human-readable URL and the
// review status (see ReviewStatusReviewed / ReviewStatusUnreviewed).
type DatabaseSpecific struct {
	URL          string `json:"url"`
	ReviewStatus string `json:"review_status"`
}

// --- triage index (/index/modules.json) ---

// ModuleVulns is one module's entry in the triage index: the module path and
// the vulnerabilities affecting it.
type ModuleVulns struct {
	Path  string      `json:"path"`
	Vulns []IndexVuln `json:"vulns"`
}

// IndexVuln references a vulnerability from the triage index. Fixed is the bare
// semver of the latest mapped fix; it is empty when no fix is mapped.
type IndexVuln struct {
	ID       string    `json:"id"`
	Modified time.Time `json:"modified"`
	Fixed    string    `json:"fixed"`
}

// Entry fetches the OSV report for id (/ID/<id>.json). ok is false when the
// database has no such ID (HTTP 404/410).
func (c *Client) Entry(ctx context.Context, id string) (entry *Entry, ok bool, err error) {
	if !ValidID(id) {
		return nil, false, fmt.Errorf("%w: %q", ErrInvalidID, id)
	}
	body, ok, err := c.get(ctx, "/ID/"+id+".json")
	if err != nil || !ok {
		return nil, ok, err
	}
	var e Entry
	if err := json.Unmarshal(body, &e); err != nil {
		return nil, false, err
	}
	return &e, true, nil
}

// Modules fetches the full triage index (/index/modules.json): every affected
// module and the vulnerabilities affecting it. Callers typically fetch this
// once and look up a single module path.
func (c *Client) Modules(ctx context.Context) ([]ModuleVulns, error) {
	body, ok, err := c.get(ctx, "/index/modules.json")
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, fmt.Errorf("vuln: %s/index/modules.json: not found", c.baseURL)
	}
	var mods []ModuleVulns
	if err := json.Unmarshal(body, &mods); err != nil {
		return nil, err
	}
	return mods, nil
}

// get issues a GET for baseURL+p. ok is false on 404/410 (not found); any other
// non-2xx status is an error. The HTTP transport handles wire gzip transparently.
func (c *Client) get(ctx context.Context, p string) (body []byte, ok bool, err error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+p, nil)
	if err != nil {
		return nil, false, err
	}
	if c.userAgent != "" {
		req.Header.Set("User-Agent", c.userAgent)
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, false, err
	}
	defer func() { _ = resp.Body.Close() }()
	switch resp.StatusCode {
	case http.StatusOK:
		data, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, false, err
		}
		return data, true, nil
	case http.StatusNotFound, http.StatusGone:
		return nil, false, nil
	default:
		return nil, false, fmt.Errorf("vuln: %s: unexpected status %d", req.URL, resp.StatusCode)
	}
}

// RangeAffected reports whether version falls inside any affected segment of the
// given OSV events, and returns the bare-semver fix that covers it (empty when
// version is not affected, or affected but unfixed). version may be given with
// or without a "v" prefix; a non-semver version (e.g. "latest") is treated as
// not affected. Events follow OSV semantics: ascending Introduced/Fixed
// boundaries, with Introduced "0" meaning "from the beginning".
func RangeAffected(events []Event, version string) (affected bool, fixed string) {
	v := ensureV(version)
	if !semver.IsValid(v) {
		return false, ""
	}
	for _, e := range events {
		switch {
		case e.Introduced == "0":
			affected = true
		case e.Introduced != "":
			if semver.Compare(v, ensureV(e.Introduced)) >= 0 {
				affected = true
			}
		case e.Fixed != "":
			if semver.Compare(v, ensureV(e.Fixed)) >= 0 {
				affected = false // v is at or past this fix
			} else if affected && fixed == "" {
				fixed = e.Fixed // this fix ends the affected segment containing v
			}
		}
	}
	return affected, fixed
}

// IsSemver reports whether version is a valid semantic version (with or without
// a "v" prefix), as opposed to a symbolic version such as "latest" or "master".
func IsSemver(version string) bool { return semver.IsValid(ensureV(version)) }

// ensureV prepends a "v" to a bare semver so golang.org/x/mod/semver accepts it.
func ensureV(s string) string {
	if s == "" || s[0] == 'v' {
		return s
	}
	return "v" + s
}
