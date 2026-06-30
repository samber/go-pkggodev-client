package pkggodev

import (
	"encoding/json"
	"time"

	"github.com/samber/mo"

	"github.com/samber/go-pkggodev-client/internal/api"
	"github.com/samber/go-pkggodev-client/internal/gomod"
	"github.com/samber/go-pkggodev-client/internal/vuln"
)

// Page is one page of a paginated listing endpoint: the items plus a cursor to
// the next page.
type Page[T any] struct {
	// Items are the entries on this page. SLICE: listing endpoints return many
	// results; empty on a page past the end.
	Items []T `json:"items"`
	// NextToken is the cursor to the next page, passed back via WithToken. None on
	// the last page. The AllXxx iterators follow it automatically.
	NextToken mo.Option[string] `json:"nextToken,omitzero"`
	// Total is the total number of items across all pages.
	Total int `json:"total"`
}

// License describes a license file detected in a module or package.
type License struct {
	// Contents is the full license text. None unless the endpoint returned it.
	Contents mo.Option[string] `json:"contents,omitzero"`
	// FilePath is the license file path within the module. None if unknown.
	FilePath mo.Option[string] `json:"filePath,omitzero"`
	// Types are the detected SPDX license identifiers. SLICE: a single file can
	// match several licenses (e.g. dual-licensed); empty when none were detected.
	Types []string `json:"types,omitempty"`
}

// Readme is a module README file.
type Readme struct {
	// Contents is the raw README text (Markdown). None if absent.
	Contents mo.Option[string] `json:"contents,omitzero"`
	// Filepath is the README path within the module, e.g. "README.md". None if absent.
	Filepath mo.Option[string] `json:"filepath,omitzero"`
}

// Package is documentation and metadata about a single package, returned by
// Client.Package.
type Package struct {
	// Path is the import path, e.g. "github.com/samber/lo". Always present.
	Path string `json:"path"`
	// ModulePath is the path of the module that contains the package. None if unknown.
	ModulePath mo.Option[string] `json:"modulePath,omitzero"`
	// Name is the package clause name (often the last path segment). None if unknown.
	Name mo.Option[string] `json:"name,omitzero"`
	// Synopsis is the one-line package summary. None if absent.
	Synopsis mo.Option[string] `json:"synopsis,omitzero"`
	// Version is the module version the package was rendered at. None if unset.
	Version mo.Option[string] `json:"version,omitzero"`
	// Goos is the GOOS the documentation was rendered for. None unless pinned via WithGOOS.
	Goos mo.Option[string] `json:"goos,omitzero"`
	// Goarch is the GOARCH the documentation was rendered for. None unless pinned via WithGOARCH.
	Goarch mo.Option[string] `json:"goarch,omitzero"`
	// Docs is the rendered package documentation. None unless requested.
	Docs mo.Option[string] `json:"docs,omitzero"`
	// Imports are the packages this package imports. SLICE: a package usually
	// imports many; populated only with WithImports.
	Imports []string `json:"imports,omitempty"`
	// IsLatest reports whether Version is the module's latest released version.
	IsLatest bool `json:"isLatest"`
	// IsRedistributable reports whether the package may be redistributed (license-wise).
	IsRedistributable bool `json:"isRedistributable"`
	// IsStandardLibrary reports whether the package is part of the Go standard library.
	IsStandardLibrary bool `json:"isStandardLibrary"`
	// Licenses are the licenses detected for the package. SLICE: a package can be
	// covered by several license files; populated only with WithLicenses.
	Licenses []License `json:"licenses,omitempty"`
}

// Module is metadata about a single module, returned by Client.Module.
type Module struct {
	// Path is the module path, e.g. "github.com/samber/lo". Always present.
	Path string `json:"path"`
	// Version is the module version this metadata was read at. None if unset.
	Version mo.Option[string] `json:"version,omitzero"`
	// GoVersion is the "go" directive of go.mod, e.g. "1.25". None if unknown.
	GoVersion mo.Option[string] `json:"goVersion,omitzero"`
	// RepoURL is the source repository URL. None if unknown.
	RepoURL mo.Option[string] `json:"repoUrl,omitzero"`
	// GoModContents is the raw go.mod file. None unless returned.
	GoModContents mo.Option[string] `json:"goModContents,omitzero"`
	// CommitTime is the version's commit/tag time. None if unknown.
	CommitTime mo.Option[time.Time] `json:"commitTime,omitzero"`
	// Size is the module zip size in bytes. None unless requested with WithSize.
	Size mo.Option[int64] `json:"size,omitzero"`
	// HasGoMod reports whether the module has a go.mod file.
	HasGoMod bool `json:"hasGoMod"`
	// IsLatest reports whether Version is the latest released version.
	IsLatest bool `json:"isLatest"`
	// IsRedistributable reports whether the module may be redistributed (license-wise).
	IsRedistributable bool `json:"isRedistributable"`
	// IsStandardLibrary reports whether this is the standard library.
	IsStandardLibrary bool `json:"isStandardLibrary"`
	// Licenses are the licenses detected for the module. SLICE: a module can be
	// covered by several license files; populated only with WithLicenses.
	Licenses []License `json:"licenses,omitempty"`
	// Readme is the module README. None unless requested with WithReadme.
	Readme mo.Option[Readme] `json:"readme,omitzero"`
}

// SearchResult is one entry from a Client.Search response.
type SearchResult struct {
	// ModulePath is the module that contains the matched package.
	ModulePath string `json:"modulePath"`
	// PackagePath is the import path of the matched package.
	PackagePath string `json:"packagePath"`
	// Synopsis is the one-line package summary.
	Synopsis string `json:"synopsis"`
	// Version is the version the result was rendered at.
	Version string `json:"version"`
}

// ModuleVersion is one entry from a Client.Versions response.
type ModuleVersion struct {
	// ModulePath is the module path.
	ModulePath string `json:"modulePath"`
	// Version is this released version, e.g. "v1.2.3".
	Version string `json:"version"`
	// LatestVersion is the module's latest version (identical across the listing).
	LatestVersion string `json:"latestVersion"`
	// CommitTime is when this version was committed/tagged.
	CommitTime time.Time `json:"commitTime"`
	// Size is the version zip size in bytes. None unless requested with WithSize.
	Size mo.Option[int64] `json:"size,omitzero"`
	// HasGoMod reports whether this version has a go.mod file.
	HasGoMod bool `json:"hasGoMod"`
	// IsRedistributable reports whether this version may be redistributed.
	IsRedistributable bool `json:"isRedistributable"`
	// Deprecated reports whether the module is marked deprecated at this version.
	Deprecated bool `json:"deprecated"`
	// DeprecationReason is the deprecation message, empty if not deprecated.
	DeprecationReason string `json:"deprecationReason"`
	// Retracted reports whether this version is retracted.
	Retracted bool `json:"retracted"`
	// RetractionReason is the retraction message, empty if not retracted.
	RetractionReason string `json:"retractionReason"`
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

// Vulnerability is a known vulnerability from the Go vulnerability database
// (https://vuln.go.dev), in OSV format, scoped to the module or package queried
// via Vulns. The OSV report's affected[] array (one entry per affected module
// path — Go major versions are distinct modules) is flattened here to the single
// module that matched the query: its version ranges and packages are hoisted to
// Ranges and Packages below.
type Vulnerability struct {
	// ID is the Go advisory identifier, e.g. "GO-2022-0191". Always present.
	ID string `json:"id"`
	// Summary is a one-line title. May be empty on auto-generated entries.
	Summary string `json:"summary"`
	// Details is the full Markdown description.
	Details string `json:"details"`

	// Ranges are the affected version intervals of the queried module, each a
	// half-open [Introduced, Fixed); Fixed is the per-interval fix (there is no
	// single "fixed version"). SLICE: one module can be affected over several
	// DISJOINT intervals — a bug fixed in one release can be reintroduced in a
	// later branch and fixed again (e.g. stdlib GO-2022-0191: [0,1.10.6) AND
	// [1.11.0-0,1.11.3)). Versions are raw OSV semver (no "v"; "0" = from the start).
	Ranges []VersionRange `json:"ranges,omitempty"`
	// Packages are the affected import paths of the queried module. SLICE: a
	// single vulnerability can span several packages within the same module (e.g.
	// .../encoding/unicode AND .../transform). Empty on auto-generated UNREVIEWED
	// entries, which carry no package-level detail.
	Packages []AffectedPackage `json:"packages,omitempty"`

	// Aliases are external identifiers for the same vulnerability. SLICE: a Go
	// advisory routinely maps to one CVE plus one or more GHSA IDs.
	Aliases []string `json:"aliases,omitempty"`
	// Published is when the advisory was first published. None if absent.
	Published mo.Option[time.Time] `json:"published,omitzero"`
	// Modified is when the advisory last changed. None if absent.
	Modified mo.Option[time.Time] `json:"modified,omitzero"`
	// Withdrawn is set only for withdrawn (retracted) advisories; None otherwise.
	Withdrawn mo.Option[time.Time] `json:"withdrawn,omitzero"`
	// URL is the human-readable page, e.g. https://pkg.go.dev/vuln/GO-2022-0191.
	URL mo.Option[string] `json:"url,omitzero"`
	// ReviewStatus is "REVIEWED" or "UNREVIEWED" (auto-generated, less precise).
	ReviewStatus mo.Option[string] `json:"reviewStatus,omitzero"`
	// References are external links. SLICE: an advisory usually has several — fix
	// commits/CLs, the issue report, third-party advisories. See Reference.Type.
	References []Reference `json:"references,omitempty"`
	// Credits are the people/organizations credited. SLICE: a report can credit
	// multiple reporters.
	Credits []string `json:"credits,omitempty"`
}

// VersionRange is one affected version interval, half-open [Introduced, Fixed).
// Versions are raw OSV semver (no "v" prefix); an Introduced of "0" means "from
// the beginning" and an absent Fixed means "no fix in this interval".
type VersionRange struct {
	Introduced mo.Option[string] `json:"introduced,omitzero"`
	Fixed      mo.Option[string] `json:"fixed,omitzero"`
}

// AffectedPackage is one affected import path within the queried module.
type AffectedPackage struct {
	// Path is the affected import path, e.g. "crypto/x509".
	Path string `json:"path"`
	// Symbols are the affected exported identifiers ("Fn", "Type.Method"). SLICE:
	// a vulnerability can live in several functions/methods of the package. EMPTY
	// means the whole package is affected (no symbol-level narrowing) — treat
	// empty as "all symbols", not "none".
	Symbols []string `json:"symbols,omitempty"`
	// Goos restricts the affected build to these operating systems. SLICE: a vuln
	// can be specific to multiple GOOS values (e.g. "windows", "darwin"). EMPTY
	// means all operating systems.
	Goos []string `json:"goos,omitempty"`
	// Goarch restricts the affected build to these architectures. SLICE: same
	// rationale as Goos. EMPTY means all architectures.
	Goarch []string `json:"goarch,omitempty"`
}

// Reference is an external link attached to a vulnerability.
type Reference struct {
	// Type is the OSV reference kind: ADVISORY, ARTICLE, REPORT, FIX, PACKAGE,
	// EVIDENCE or WEB.
	Type string `json:"type"`
	URL  string `json:"url"`
}

// SymbolInfo is one entry from a Client.Symbols response: lightweight metadata
// about a package symbol. Use Client.Symbol to fetch the full documentation of
// one symbol.
type SymbolInfo struct {
	// Name is the exported identifier, e.g. "Map" or "Either.ForEach".
	Name string `json:"name"`
	// Kind is one of Function, Method, Type, Variable or Constant.
	Kind string `json:"kind"`
	// Synopsis is the one-line summary of the symbol.
	Synopsis string `json:"synopsis"`
	// Parent is the enclosing type for a method/field, empty otherwise.
	Parent string `json:"parent"`
}

// Symbol is the full documentation of a single package symbol, derived
// client-side from the package documentation. See Client.Symbol.
type Symbol struct {
	// Path is the package import path the symbol belongs to.
	Path string `json:"path"`
	// Name is the exported identifier, e.g. "Map" or "Either.ForEach".
	Name string `json:"name"`
	// Kind is one of Function, Method, Type, Variable or Constant.
	Kind string `json:"kind"`
	// Signature is the symbol's Go declaration.
	Signature string `json:"signature"`
	// Synopsis is the one-line summary. None if absent.
	Synopsis mo.Option[string] `json:"synopsis,omitzero"`
	// Doc is the full documentation text. None if absent.
	Doc mo.Option[string] `json:"doc,omitzero"`
	// Examples are runnable examples for the symbol. SLICE: a symbol can have
	// several (bare "Example" plus named "Example_xxx"); populated only with WithExamples.
	Examples []Example `json:"examples,omitempty"`
	// Version is the module version the doc was rendered at. None if unset.
	Version mo.Option[string] `json:"version,omitzero"`
	// Goos is the GOOS the doc was rendered for. None unless pinned via WithGOOS.
	Goos mo.Option[string] `json:"goos,omitzero"`
	// Goarch is the GOARCH the doc was rendered for. None unless pinned via WithGOARCH.
	Goarch mo.Option[string] `json:"goarch,omitzero"`
}

// Example is a runnable example attached to a symbol.
type Example struct {
	// Name is the suffix of "Example (name)"; None for a bare "Example".
	Name mo.Option[string] `json:"name,omitzero"`
	// Code is the example source.
	Code string `json:"code"`
	// Output is the expected output (the "// Output:" block). None if absent.
	Output mo.Option[string] `json:"output,omitzero"`
}

// PackageInfo is one entry from a Client.Packages response.
type PackageInfo struct {
	// Path is the package import path.
	Path string `json:"path"`
	// Name is the package clause name.
	Name string `json:"name"`
	// Synopsis is the one-line package summary.
	Synopsis string `json:"synopsis"`
	// IsRedistributable reports whether the package may be redistributed.
	IsRedistributable bool `json:"isRedistributable"`
}

// ImportedByResult lists the packages that import a given package, returned by
// Client.ImportedBy.
type ImportedByResult struct {
	// ModulePath is the module of the queried package. None if unknown.
	ModulePath mo.Option[string] `json:"modulePath,omitzero"`
	// Version is the version the result was computed at. None if unset.
	Version mo.Option[string] `json:"version,omitzero"`
	// Packages is the paginated page of importer package paths (see Page; many
	// packages may import one).
	Packages Page[string] `json:"packages"`
}

// PackagesResult lists the packages contained in a module, returned by
// Client.Packages.
type PackagesResult struct {
	// ModulePath is the queried module. None if unknown.
	ModulePath mo.Option[string] `json:"modulePath,omitzero"`
	// Version is the version the listing was computed at. None if unset.
	Version mo.Option[string] `json:"version,omitzero"`
	// IsStandardLibrary reports whether the module is the standard library.
	IsStandardLibrary bool `json:"isStandardLibrary"`
	// Packages is the paginated page of packages in the module (see Page).
	Packages Page[PackageInfo] `json:"packages"`
}

// Dependency is one module a target module depends on, parsed from a go.mod
// require (or exclude) directive.
type Dependency struct {
	// Path is the dependency module path.
	Path string `json:"path"`
	// Version is the required version. None for an exclude (which has no single version).
	Version mo.Option[string] `json:"version,omitzero"`
	// Indirect is true for a "// indirect" require (transitive, not imported directly).
	Indirect bool `json:"indirect,omitempty"`
}

// Replacement is one go.mod replace directive, redirecting a module (optionally
// pinned to OldVersion) to NewPath. A NewPath that is a filesystem path with no
// NewVersion is a local replacement.
type Replacement struct {
	// OldPath is the replaced module path.
	OldPath string `json:"oldPath"`
	// OldVersion is the specific version replaced. None when all versions are replaced.
	OldVersion mo.Option[string] `json:"oldVersion,omitzero"`
	// NewPath is the replacement module path or filesystem path.
	NewPath string `json:"newPath"`
	// NewVersion is the replacement version. None for a local (filesystem) replacement.
	NewVersion mo.Option[string] `json:"newVersion,omitzero"`
}

// DependenciesResult is the parsed go.mod of a module: the dependencies it
// declares (requires), plus any replace/exclude directives, fetched from the Go
// module proxy. See Client.Dependencies.
type DependenciesResult struct {
	// ModulePath is the queried module path.
	ModulePath string `json:"modulePath"`
	// Version is the concrete version the go.mod was read at.
	Version string `json:"version"`
	// GoVersion is the "go" directive, e.g. "1.25". None if absent.
	GoVersion mo.Option[string] `json:"goVersion,omitzero"`
	// Requires are the module's direct and indirect requirements. SLICE: a module
	// typically requires many dependencies.
	Requires []Dependency `json:"requires,omitempty"`
	// Replaces are the go.mod replace directives. SLICE: a go.mod can hold several.
	Replaces []Replacement `json:"replaces,omitempty"`
	// Excludes are the go.mod exclude directives. SLICE: a go.mod can hold several.
	Excludes []Dependency `json:"excludes,omitempty"`
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
			Contents: mo.EmptyableToOption(l.Contents.Value),
			FilePath: mo.EmptyableToOption(l.FilePath.Value),
			Types:    l.Types,
		})
	}
	return out
}

func toPackage(p *api.Package) *Package {
	return &Package{
		Path:              p.Path.Value,
		ModulePath:        mo.EmptyableToOption(p.ModulePath.Value),
		Name:              mo.EmptyableToOption(p.Name.Value),
		Synopsis:          mo.EmptyableToOption(p.Synopsis.Value),
		Version:           mo.EmptyableToOption(p.Version.Value),
		Goos:              mo.EmptyableToOption(p.Goos.Value),
		Goarch:            mo.EmptyableToOption(p.Goarch.Value),
		Docs:              mo.EmptyableToOption(p.Docs.Value),
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
			Contents: mo.EmptyableToOption(r.Contents.Value),
			Filepath: mo.EmptyableToOption(r.Filepath.Value),
		})
	}
	return &Module{
		Path:              m.Path.Value,
		Version:           mo.EmptyableToOption(m.Version.Value),
		GoVersion:         mo.EmptyableToOption(goVersionOf(m.Path.Value, m.GoModContents.Value)),
		RepoURL:           mo.EmptyableToOption(m.RepoUrl.Value),
		GoModContents:     mo.EmptyableToOption(m.GoModContents.Value),
		CommitTime:        mo.EmptyableToOption(m.CommitTime.Value),
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
	page := Page[T]{NextToken: mo.EmptyableToOption(pr.NextPageToken.Value), Total: pr.Total.Value}
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

// --- vuln OSV -> public Vulnerability ---

// toVulnerability maps an OSV entry to the public Vulnerability, scoped to
// module: the affected version ranges and packages of that one module path are
// hoisted to the root (the entry's other affected modules — typically other Go
// major versions — are dropped, since Vulns is queried for a single module).
func toVulnerability(e *vuln.Entry, module string) Vulnerability {
	v := Vulnerability{
		ID:        e.ID,
		Summary:   e.Summary,
		Details:   e.Details,
		Aliases:   e.Aliases,
		Published: timeOption(e.Published),
		Modified:  timeOption(e.Modified),
		Credits:   creditNames(e.Credits),
	}
	if e.Withdrawn != nil {
		v.Withdrawn = timeOption(*e.Withdrawn)
	}
	if e.DatabaseSpecific != nil {
		v.URL = mo.EmptyableToOption(e.DatabaseSpecific.URL)
		v.ReviewStatus = mo.EmptyableToOption(e.DatabaseSpecific.ReviewStatus)
	}
	for _, a := range e.Affected {
		if a.Module.Path != module {
			continue
		}
		v.Ranges = append(v.Ranges, toVersionRanges(a.Ranges)...)
		for _, p := range a.EcosystemSpecific.Imports {
			v.Packages = append(v.Packages, AffectedPackage{
				Path:    p.Path,
				Symbols: p.Symbols,
				Goos:    p.GOOS,
				Goarch:  p.GOARCH,
			})
		}
	}
	for _, r := range e.References {
		v.References = append(v.References, Reference{Type: r.Type, URL: r.URL})
	}
	return v
}

// toVersionRanges pairs OSV range events into half-open [introduced, fixed)
// intervals. A bare "introduced" with no following "fixed" yields an open-ended
// interval; a "fixed" with no preceding "introduced" yields a fixed-only one.
func toVersionRanges(ranges []vuln.Range) []VersionRange {
	var out []VersionRange
	open := -1 // index of the interval awaiting its Fixed boundary, or -1.
	for _, r := range ranges {
		for _, e := range r.Events {
			switch {
			case e.Introduced != "":
				out = append(out, VersionRange{Introduced: mo.Some(e.Introduced)})
				open = len(out) - 1
			case e.Fixed != "":
				if open >= 0 {
					out[open].Fixed = mo.Some(e.Fixed)
					open = -1
				} else {
					out = append(out, VersionRange{Fixed: mo.Some(e.Fixed)})
				}
			}
		}
	}
	return out
}

// timeOption maps a time.Time to None when zero, Some otherwise.
func timeOption(t time.Time) mo.Option[time.Time] {
	if t.IsZero() {
		return mo.None[time.Time]()
	}
	return mo.Some(t)
}

// creditNames flattens OSV credits to their names.
func creditNames(in []vuln.Credit) []string {
	if len(in) == 0 {
		return nil
	}
	out := make([]string, 0, len(in))
	for _, c := range in {
		out = append(out, c.Name)
	}
	return out
}
