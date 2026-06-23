package pkggodev

import (
	"encoding/json"
	"time"

	"github.com/samber/go-pkggodev-client/internal/api"
	"github.com/samber/go-pkggodev-client/internal/gomod"
)

// Page is a paginated slice of T returned by listing endpoints.
type Page[T any] struct {
	Items     []T    `json:"items"`
	NextToken string `json:"nextToken,omitempty"`
	Total     int    `json:"total"`
}

// License describes a license file detected in a module or package.
type License struct {
	Contents string   `json:"contents,omitempty"`
	FilePath string   `json:"filePath,omitempty"`
	Types    []string `json:"types,omitempty"`
}

// Readme is a module README.
type Readme struct {
	Contents string `json:"contents,omitempty"`
	Filepath string `json:"filepath,omitempty"`
}

// Package is documentation and metadata about a single package.
type Package struct {
	Path              string    `json:"path"`
	ModulePath        string    `json:"modulePath,omitempty"`
	Name              string    `json:"name,omitempty"`
	Synopsis          string    `json:"synopsis,omitempty"`
	Version           string    `json:"version,omitempty"`
	Goos              string    `json:"goos,omitempty"`
	Goarch            string    `json:"goarch,omitempty"`
	Docs              string    `json:"docs,omitempty"`
	Imports           []string  `json:"imports,omitempty"`
	IsLatest          bool      `json:"isLatest"`
	IsRedistributable bool      `json:"isRedistributable"`
	IsStandardLibrary bool      `json:"isStandardLibrary"`
	Licenses          []License `json:"licenses,omitempty"`
}

// Module is metadata about a single module.
type Module struct {
	Path              string    `json:"path"`
	Version           string    `json:"version,omitempty"`
	GoVersion         string    `json:"goVersion,omitempty"` // the "go" directive of go.mod, e.g. "1.25".
	RepoURL           string    `json:"repoUrl,omitempty"`
	GoModContents     string    `json:"goModContents,omitempty"`
	CommitTime        time.Time `json:"commitTime,omitzero"`
	Size              int64     `json:"size,omitempty"` // module zip size in bytes; set only with WithSize.
	HasGoMod          bool      `json:"hasGoMod"`
	IsLatest          bool      `json:"isLatest"`
	IsRedistributable bool      `json:"isRedistributable"`
	IsStandardLibrary bool      `json:"isStandardLibrary"`
	Licenses          []License `json:"licenses,omitempty"`
	Readme            Readme    `json:"readme,omitzero"`
}

// SearchResult is one entry from a /search response.
type SearchResult struct {
	ModulePath  string `json:"modulePath"`
	PackagePath string `json:"packagePath"`
	Synopsis    string `json:"synopsis"`
	Version     string `json:"version"`
}

// ModuleVersion is one entry from a /versions response.
type ModuleVersion struct {
	ModulePath        string    `json:"modulePath"`
	Version           string    `json:"version"`
	LatestVersion     string    `json:"latestVersion"`
	CommitTime        time.Time `json:"commitTime"`
	Size              int64     `json:"size,omitempty"` // version zip size in bytes; set only with WithSize.
	HasGoMod          bool      `json:"hasGoMod"`
	IsRedistributable bool      `json:"isRedistributable"`
	Deprecated        bool      `json:"deprecated"`
	DeprecationReason string    `json:"deprecationReason"`
	Retracted         bool      `json:"retracted"`
	RetractionReason  string    `json:"retractionReason"`
}

// MajorVersion is one major version of a module, discovered via the module
// proxy (see Client.MajorVersions). Majors beyond v1 live as separate modules
// (path, path/v2, path/v3...), so each MajorVersion carries its own module path.
type MajorVersion struct {
	ModulePath string `json:"modulePath"` // e.g. "github.com/samber/do/v2"
	Major      string `json:"major"`      // e.g. "v2"
	Version    string `json:"version"`    // latest version in this major, e.g. "v2.0.0"
	IsLatest   bool   `json:"isLatest"`   // true for the highest major
}

// Vulnerability is one entry from a /vulns response.
type Vulnerability struct {
	ID           string `json:"id"`
	Summary      string `json:"summary"`
	Details      string `json:"details"`
	FixedVersion string `json:"fixedVersion"`
}

// SymbolInfo is one entry from a /symbols response: lightweight metadata about a
// package symbol. Use Client.Symbol to fetch the full documentation of one symbol.
type SymbolInfo struct {
	Name     string `json:"name"`
	Kind     string `json:"kind"`
	Synopsis string `json:"synopsis"`
	Parent   string `json:"parent"`
}

// Symbol is the full documentation of a single package symbol, derived
// client-side from the package documentation. See Client.Symbol.
type Symbol struct {
	Path      string    `json:"path"`
	Name      string    `json:"name"`
	Kind      string    `json:"kind"` // Function, Method, Type, Variable or Constant.
	Signature string    `json:"signature"`
	Synopsis  string    `json:"synopsis,omitempty"`
	Doc       string    `json:"doc,omitempty"`
	Examples  []Example `json:"examples,omitempty"` // Populated only when WithExamples is set.
	Version   string    `json:"version,omitempty"`
	Goos      string    `json:"goos,omitempty"`
	Goarch    string    `json:"goarch,omitempty"`
}

// Example is a runnable example attached to a symbol.
type Example struct {
	Name   string `json:"name,omitempty"` // Suffix of "Example (name)", empty for a bare "Example".
	Code   string `json:"code"`
	Output string `json:"output,omitempty"`
}

// PackageInfo is one entry from a /packages response.
type PackageInfo struct {
	Path              string `json:"path"`
	Name              string `json:"name"`
	Synopsis          string `json:"synopsis"`
	IsRedistributable bool   `json:"isRedistributable"`
}

// ImportedByResult lists the packages that import a given package.
type ImportedByResult struct {
	ModulePath string       `json:"modulePath,omitempty"`
	Version    string       `json:"version,omitempty"`
	Packages   Page[string] `json:"packages"`
}

// PackagesResult lists the packages contained in a module.
type PackagesResult struct {
	ModulePath        string            `json:"modulePath,omitempty"`
	Version           string            `json:"version,omitempty"`
	IsStandardLibrary bool              `json:"isStandardLibrary"`
	Packages          Page[PackageInfo] `json:"packages"`
}

// Dependency is one module a target module depends on, parsed from a go.mod
// require (or exclude) directive.
type Dependency struct {
	Path     string `json:"path"`
	Version  string `json:"version,omitempty"`
	Indirect bool   `json:"indirect,omitempty"` // true for a "// indirect" require (transitive, not imported directly).
}

// Replacement is one go.mod replace directive, redirecting a module (optionally
// pinned to OldVersion) to NewPath. A NewPath that is a filesystem path with no
// NewVersion is a local replacement.
type Replacement struct {
	OldPath    string `json:"oldPath"`
	OldVersion string `json:"oldVersion,omitempty"`
	NewPath    string `json:"newPath"`
	NewVersion string `json:"newVersion,omitempty"`
}

// DependenciesResult is the parsed go.mod of a module: the dependencies it
// declares (requires), plus any replace/exclude directives, fetched from the Go
// module proxy. See Client.Dependencies.
type DependenciesResult struct {
	ModulePath string        `json:"modulePath"`
	Version    string        `json:"version"`             // the concrete version the go.mod was read at.
	GoVersion  string        `json:"goVersion,omitempty"` // the "go" directive, e.g. "1.25".
	Requires   []Dependency  `json:"requires,omitempty"`
	Replaces   []Replacement `json:"replaces,omitempty"`
	Excludes   []Dependency  `json:"excludes,omitempty"`
}

// --- public -> ogen optional params ---

func optStr(s string) api.OptString {
	if s == "" {
		return api.OptString{}
	}
	return api.NewOptString(s)
}

func optInt(n int) api.OptInt {
	if n == 0 {
		return api.OptInt{}
	}
	return api.NewOptInt(n)
}

func optBool(b bool) api.OptBool {
	if !b {
		return api.OptBool{}
	}
	return api.NewOptBool(true)
}

// --- ogen -> public clean types ---

func toLicenses(in []api.License) []License {
	if len(in) == 0 {
		return nil
	}
	out := make([]License, 0, len(in))
	for _, l := range in {
		out = append(out, License{Contents: l.Contents.Value, FilePath: l.FilePath.Value, Types: l.Types})
	}
	return out
}

func toPackage(p *api.Package) *Package {
	return &Package{
		Path:              p.Path.Value,
		ModulePath:        p.ModulePath.Value,
		Name:              p.Name.Value,
		Synopsis:          p.Synopsis.Value,
		Version:           p.Version.Value,
		Goos:              p.Goos.Value,
		Goarch:            p.Goarch.Value,
		Docs:              p.Docs.Value,
		Imports:           p.Imports,
		IsLatest:          p.IsLatest.Value,
		IsRedistributable: p.IsRedistributable.Value,
		IsStandardLibrary: p.IsStandardLibrary.Value,
		Licenses:          toLicenses(p.Licenses),
	}
}

func toModule(m *api.Module) *Module {
	return &Module{
		Path:              m.Path.Value,
		Version:           m.Version.Value,
		GoVersion:         goVersionOf(m.Path.Value, m.GoModContents.Value),
		RepoURL:           m.RepoUrl.Value,
		GoModContents:     m.GoModContents.Value,
		CommitTime:        m.CommitTime.Value,
		HasGoMod:          m.HasGoMod.Value,
		IsLatest:          m.IsLatest.Value,
		IsRedistributable: m.IsRedistributable.Value,
		IsStandardLibrary: m.IsStandardLibrary.Value,
		Licenses:          toLicenses(m.Licenses),
		Readme:            Readme{Contents: m.Readme.Value.Contents.Value, Filepath: m.Readme.Value.Filepath.Value},
	}
}

// goVersionOf extracts the "go" directive from go.mod contents, best-effort: an
// empty or unparsable go.mod yields "".
func goVersionOf(modulePath, contents string) string {
	if contents == "" {
		return ""
	}
	m, err := gomod.Parse(modulePath, []byte(contents))
	if err != nil {
		return ""
	}
	return m.GoVersion
}

// toDependenciesResult maps a parsed go.mod into the public DependenciesResult.
func toDependenciesResult(modulePath, version string, m *gomod.Mod) *DependenciesResult {
	res := &DependenciesResult{
		ModulePath: modulePath,
		Version:    version,
		GoVersion:  m.GoVersion,
	}
	for _, r := range m.Requires {
		res.Requires = append(res.Requires, Dependency{Path: r.Path, Version: r.Version, Indirect: r.Indirect})
	}
	for _, r := range m.Replaces {
		res.Replaces = append(res.Replaces, Replacement{
			OldPath:    r.OldPath,
			OldVersion: r.OldVersion,
			NewPath:    r.NewPath,
			NewVersion: r.NewVersion,
		})
	}
	for _, e := range m.Excludes {
		res.Excludes = append(res.Excludes, Dependency{Path: e.Path, Version: e.Version})
	}
	return res
}

// decodePage turns an ogen PaginatedResponse (whose items are raw JSON) into a
// typed Page[T] by unmarshalling each item into T.
func decodePage[T any](pr api.PaginatedResponse) (Page[T], error) {
	page := Page[T]{NextToken: pr.NextPageToken.Value, Total: pr.Total.Value}
	raws, ok := pr.Items.Get()
	if !ok {
		return page, nil
	}
	page.Items = make([]T, 0, len(raws))
	for _, r := range raws {
		var v T
		if err := json.Unmarshal(r, &v); err != nil {
			return Page[T]{}, err
		}
		page.Items = append(page.Items, v)
	}
	return page, nil
}
