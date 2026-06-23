package pkggodev

import (
	"encoding/json"
	"time"

	"github.com/samber/mo"

	"github.com/samber/go-pkggodev-client/internal/api"
	"github.com/samber/go-pkggodev-client/internal/gomod"
)

// Page is a paginated slice of T returned by listing endpoints.
type Page[T any] struct {
	Items     []T               `json:"items"`
	NextToken mo.Option[string] `json:"nextToken,omitzero"`
	Total     int               `json:"total"`
}

// License describes a license file detected in a module or package.
type License struct {
	Contents mo.Option[string] `json:"contents,omitzero"`
	FilePath mo.Option[string] `json:"filePath,omitzero"`
	Types    []string          `json:"types,omitempty"`
}

// Readme is a module README.
type Readme struct {
	Contents mo.Option[string] `json:"contents,omitzero"`
	Filepath mo.Option[string] `json:"filepath,omitzero"`
}

// Package is documentation and metadata about a single package.
type Package struct {
	Path              string            `json:"path"`
	ModulePath        mo.Option[string] `json:"modulePath,omitzero"`
	Name              mo.Option[string] `json:"name,omitzero"`
	Synopsis          mo.Option[string] `json:"synopsis,omitzero"`
	Version           mo.Option[string] `json:"version,omitzero"`
	Goos              mo.Option[string] `json:"goos,omitzero"`
	Goarch            mo.Option[string] `json:"goarch,omitzero"`
	Docs              mo.Option[string] `json:"docs,omitzero"`
	Imports           []string          `json:"imports,omitempty"`
	IsLatest          bool              `json:"isLatest"`
	IsRedistributable bool              `json:"isRedistributable"`
	IsStandardLibrary bool              `json:"isStandardLibrary"`
	Licenses          []License         `json:"licenses,omitempty"`
}

// Module is metadata about a single module.
type Module struct {
	Path              string               `json:"path"`
	Version           mo.Option[string]    `json:"version,omitzero"`
	GoVersion         mo.Option[string]    `json:"goVersion,omitzero"` // the "go" directive of go.mod, e.g. "1.25".
	RepoURL           mo.Option[string]    `json:"repoUrl,omitzero"`
	GoModContents     mo.Option[string]    `json:"goModContents,omitzero"`
	CommitTime        mo.Option[time.Time] `json:"commitTime,omitzero"`
	Size              mo.Option[int64]     `json:"size,omitzero"` // module zip size in bytes; present only with WithSize.
	HasGoMod          bool                 `json:"hasGoMod"`
	IsLatest          bool                 `json:"isLatest"`
	IsRedistributable bool                 `json:"isRedistributable"`
	IsStandardLibrary bool                 `json:"isStandardLibrary"`
	Licenses          []License            `json:"licenses,omitempty"`
	Readme            mo.Option[Readme]    `json:"readme,omitzero"`
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
	ModulePath        string           `json:"modulePath"`
	Version           string           `json:"version"`
	LatestVersion     string           `json:"latestVersion"`
	CommitTime        time.Time        `json:"commitTime"`
	Size              mo.Option[int64] `json:"size,omitzero"` // version zip size in bytes; present only with WithSize.
	HasGoMod          bool             `json:"hasGoMod"`
	IsRedistributable bool             `json:"isRedistributable"`
	Deprecated        bool             `json:"deprecated"`
	DeprecationReason string           `json:"deprecationReason"`
	Retracted         bool             `json:"retracted"`
	RetractionReason  string           `json:"retractionReason"`
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
	Path      string            `json:"path"`
	Name      string            `json:"name"`
	Kind      string            `json:"kind"` // Function, Method, Type, Variable or Constant.
	Signature string            `json:"signature"`
	Synopsis  mo.Option[string] `json:"synopsis,omitzero"`
	Doc       mo.Option[string] `json:"doc,omitzero"`
	Examples  []Example         `json:"examples,omitempty"` // Populated only when WithExamples is set.
	Version   mo.Option[string] `json:"version,omitzero"`
	Goos      mo.Option[string] `json:"goos,omitzero"`
	Goarch    mo.Option[string] `json:"goarch,omitzero"`
}

// Example is a runnable example attached to a symbol.
type Example struct {
	Name   mo.Option[string] `json:"name,omitzero"` // Suffix of "Example (name)", absent for a bare "Example".
	Code   string            `json:"code"`
	Output mo.Option[string] `json:"output,omitzero"`
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
	ModulePath mo.Option[string] `json:"modulePath,omitzero"`
	Version    mo.Option[string] `json:"version,omitzero"`
	Packages   Page[string]      `json:"packages"`
}

// PackagesResult lists the packages contained in a module.
type PackagesResult struct {
	ModulePath        mo.Option[string] `json:"modulePath,omitzero"`
	Version           mo.Option[string] `json:"version,omitzero"`
	IsStandardLibrary bool              `json:"isStandardLibrary"`
	Packages          Page[PackageInfo] `json:"packages"`
}

// Dependency is one module a target module depends on, parsed from a go.mod
// require (or exclude) directive.
type Dependency struct {
	Path     string            `json:"path"`
	Version  mo.Option[string] `json:"version,omitzero"`
	Indirect bool              `json:"indirect,omitempty"` // true for a "// indirect" require (transitive, not imported directly).
}

// Replacement is one go.mod replace directive, redirecting a module (optionally
// pinned to OldVersion) to NewPath. A NewPath that is a filesystem path with no
// NewVersion is a local replacement.
type Replacement struct {
	OldPath    string            `json:"oldPath"`
	OldVersion mo.Option[string] `json:"oldVersion,omitzero"`
	NewPath    string            `json:"newPath"`
	NewVersion mo.Option[string] `json:"newVersion,omitzero"`
}

// DependenciesResult is the parsed go.mod of a module: the dependencies it
// declares (requires), plus any replace/exclude directives, fetched from the Go
// module proxy. See Client.Dependencies.
type DependenciesResult struct {
	ModulePath string            `json:"modulePath"`
	Version    string            `json:"version"`            // the concrete version the go.mod was read at.
	GoVersion  mo.Option[string] `json:"goVersion,omitzero"` // the "go" directive, e.g. "1.25".
	Requires   []Dependency      `json:"requires,omitempty"`
	Replaces   []Replacement     `json:"replaces,omitempty"`
	Excludes   []Dependency      `json:"excludes,omitempty"`
}

// --- public -> ogen optional params ---
//
// Each helper routes a public zero-able value through mo.Option (zero value ->
// None) before lowering it to the ogen optional the generated client expects.

func optStr(s string) api.OptString {
	if v, ok := mo.EmptyableToOption(s).Get(); ok {
		return api.NewOptString(v)
	}
	return api.OptString{}
}

func optInt(n int) api.OptInt {
	if v, ok := mo.EmptyableToOption(n).Get(); ok {
		return api.NewOptInt(v)
	}
	return api.OptInt{}
}

func optBool(b bool) api.OptBool {
	if v, ok := mo.EmptyableToOption(b).Get(); ok {
		return api.NewOptBool(v)
	}
	return api.OptBool{}
}

// --- ogen -> public clean types ---

func toLicenses(in []api.License) []License {
	if len(in) == 0 {
		return nil
	}
	out := make([]License, 0, len(in))
	for _, l := range in {
		out = append(out, License{
			Contents: mo.TupleToOption(l.Contents.Get()),
			FilePath: mo.TupleToOption(l.FilePath.Get()),
			Types:    l.Types,
		})
	}
	return out
}

func toPackage(p *api.Package) *Package {
	return &Package{
		Path:              p.Path.Value,
		ModulePath:        mo.TupleToOption(p.ModulePath.Get()),
		Name:              mo.TupleToOption(p.Name.Get()),
		Synopsis:          mo.TupleToOption(p.Synopsis.Get()),
		Version:           mo.TupleToOption(p.Version.Get()),
		Goos:              mo.TupleToOption(p.Goos.Get()),
		Goarch:            mo.TupleToOption(p.Goarch.Get()),
		Docs:              mo.TupleToOption(p.Docs.Get()),
		Imports:           p.Imports,
		IsLatest:          p.IsLatest.Value,
		IsRedistributable: p.IsRedistributable.Value,
		IsStandardLibrary: p.IsStandardLibrary.Value,
		Licenses:          toLicenses(p.Licenses),
	}
}

func toModule(m *api.Module) *Module {
	readme := mo.None[Readme]()
	if r, ok := m.Readme.Get(); ok {
		readme = mo.Some(Readme{
			Contents: mo.TupleToOption(r.Contents.Get()),
			Filepath: mo.TupleToOption(r.Filepath.Get()),
		})
	}
	return &Module{
		Path:              m.Path.Value,
		Version:           mo.TupleToOption(m.Version.Get()),
		GoVersion:         mo.EmptyableToOption(goVersionOf(m.Path.Value, m.GoModContents.Value)),
		RepoURL:           mo.TupleToOption(m.RepoUrl.Get()),
		GoModContents:     mo.TupleToOption(m.GoModContents.Get()),
		CommitTime:        mo.TupleToOption(m.CommitTime.Get()),
		Size:              mo.None[int64](), // filled by applyModuleSize when WithSize is set.
		HasGoMod:          m.HasGoMod.Value,
		IsLatest:          m.IsLatest.Value,
		IsRedistributable: m.IsRedistributable.Value,
		IsStandardLibrary: m.IsStandardLibrary.Value,
		Licenses:          toLicenses(m.Licenses),
		Readme:            readme,
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
		GoVersion:  mo.EmptyableToOption(m.GoVersion),
	}
	for _, r := range m.Requires {
		res.Requires = append(res.Requires, Dependency{Path: r.Path, Version: mo.EmptyableToOption(r.Version), Indirect: r.Indirect})
	}
	for _, r := range m.Replaces {
		res.Replaces = append(res.Replaces, Replacement{
			OldPath:    r.OldPath,
			OldVersion: mo.EmptyableToOption(r.OldVersion),
			NewPath:    r.NewPath,
			NewVersion: mo.EmptyableToOption(r.NewVersion),
		})
	}
	for _, e := range m.Excludes {
		res.Excludes = append(res.Excludes, Dependency{Path: e.Path, Version: mo.EmptyableToOption(e.Version)})
	}
	return res
}

// decodePage turns an ogen PaginatedResponse (whose items are raw JSON) into a
// typed Page[T] by unmarshalling each item into T.
func decodePage[T any](pr api.PaginatedResponse) (Page[T], error) {
	page := Page[T]{NextToken: mo.TupleToOption(pr.NextPageToken.Get()), Total: pr.Total.Value}
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
