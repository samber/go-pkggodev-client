package pkggodev

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/samber/mo"
	"golang.org/x/sync/errgroup"

	"github.com/samber/go-pkggodev-client/internal/api"
	"github.com/samber/go-pkggodev-client/internal/godoc"
	"github.com/samber/go-pkggodev-client/internal/gomod"
	"github.com/samber/go-pkggodev-client/internal/majors"
	"github.com/samber/go-pkggodev-client/internal/proxy"
	"github.com/samber/go-pkggodev-client/internal/vuln"
)

// Search finds packages and symbols. Use WithQuery and/or WithSymbol to set the
// query; WithLimit, WithToken and WithFilter tune the listing. Results are
// paginated (see Page and AllSearch to auto-paginate).
func (c *Client) Search(ctx context.Context, opts ...Option) (*Page[SearchResult], error) {
	p := newParams(opts)
	params := api.GetSearchParams{
		Q:      optStr(p.query),
		Symbol: optStr(p.symbol),
		Limit:  optInt(p.limit),
		Token:  optStr(p.token),
		Filter: optStr(p.filter),
	}
	v, err, _ := c.sf.search.Do(sfKey("search", params), func() (*Page[SearchResult], error) {
		res, err := c.raw.GetSearch(ctx, params)
		if err != nil {
			return nil, err
		}
		page, err := decodePage[SearchResult](*res)
		if err != nil {
			return nil, err
		}
		return &page, nil
	})
	return v, err
}

// Package returns documentation and metadata for the package at path. Use
// WithModule to disambiguate the owning module and WithVersion to pin a version;
// WithGOOS/WithGOARCH set the build context and WithDoc the doc format. By
// default docs, examples, imports and licenses are omitted — request them with
// WithDoc-bearing options, WithExamples, WithImports and WithLicenses.
func (c *Client) Package(ctx context.Context, path string, opts ...Option) (*Package, error) {
	p := newParams(opts)
	params := api.GetPackageParams{
		Path:     path,
		Module:   optStr(p.module),
		Version:  optStr(p.version),
		Goos:     optStr(p.goos),
		Goarch:   optStr(p.goarch),
		Doc:      optStr(p.doc),
		Examples: optBool(p.examples),
		Imports:  optBool(p.imports),
		Licenses: optBool(p.licenses),
	}
	v, err, _ := c.sf.pkg.Do(sfKey("package", params), func() (*Package, error) {
		res, err := c.raw.GetPackage(ctx, params)
		if err != nil {
			return nil, err
		}
		return toPackage(res), nil
	})
	return v, err
}

// ImportedBy lists the packages that import the package at path. Use WithModule
// and WithVersion to scope the package; WithLimit, WithToken and WithFilter tune
// the listing. The importer paths are paginated inside ImportedByResult.Packages
// (see AllImportedBy to auto-paginate).
func (c *Client) ImportedBy(ctx context.Context, path string, opts ...Option) (*ImportedByResult, error) {
	p := newParams(opts)
	params := api.GetImportedByParams{
		Path:    path,
		Module:  optStr(p.module),
		Version: optStr(p.version),
		Limit:   optInt(p.limit),
		Token:   optStr(p.token),
		Filter:  optStr(p.filter),
	}
	v, err, _ := c.sf.importedBy.Do(sfKey("importedBy", params), func() (*ImportedByResult, error) {
		res, err := c.raw.GetImportedBy(ctx, params)
		if err != nil {
			return nil, err
		}
		page, err := decodePage[string](res.ImportedBy.Value)
		if err != nil {
			return nil, err
		}
		return &ImportedByResult{ModulePath: mo.EmptyableToOption(res.ModulePath.Value), Version: mo.EmptyableToOption(res.Version.Value), Packages: page}, nil
	})
	return v, err
}

// Packages lists the packages contained in the module at path. Use WithVersion
// to pin a version; WithLimit, WithToken and WithFilter tune the listing. The
// packages are paginated inside PackagesResult.Packages (see AllPackages to
// auto-paginate).
func (c *Client) Packages(ctx context.Context, path string, opts ...Option) (*PackagesResult, error) {
	p := newParams(opts)
	params := api.GetPackagesParams{
		Path:    path,
		Version: optStr(p.version),
		Limit:   optInt(p.limit),
		Token:   optStr(p.token),
		Filter:  optStr(p.filter),
	}
	v, err, _ := c.sf.packages.Do(sfKey("packages", params), func() (*PackagesResult, error) {
		res, err := c.raw.GetPackages(ctx, params)
		if err != nil {
			return nil, err
		}
		page, err := decodePage[PackageInfo](res.Packages.Value)
		if err != nil {
			return nil, err
		}
		return &PackagesResult{
			ModulePath:        mo.EmptyableToOption(res.ModulePath.Value),
			Version:           mo.EmptyableToOption(res.Version.Value),
			IsStandardLibrary: res.IsStandardLibrary.Value,
			Packages:          page,
		}, nil
	})
	return v, err
}

// Module returns metadata for the module at path. Use WithVersion to pin a
// version and WithLicenses/WithReadme to include the licenses/README. WithSize
// fills Module.Size from the module proxy with one extra HEAD request and
// returns ErrProxyDisabled when GOPROXY is "off"/"direct"-only.
func (c *Client) Module(ctx context.Context, path string, opts ...Option) (*Module, error) {
	p := newParams(opts)
	params := api.GetModuleParams{
		Path:     path,
		Version:  optStr(p.version),
		Licenses: optBool(p.licenses),
		Readme:   optBool(p.readme),
	}
	v, err, _ := c.sf.module.Do(sfKey("module", params, p.size), func() (*Module, error) {
		res, err := c.raw.GetModule(ctx, params)
		if err != nil {
			return nil, err
		}
		m := toModule(res)
		if err := c.applyModuleSize(ctx, m, p.size); err != nil {
			return nil, err
		}
		return m, nil
	})
	return v, err
}

// applyModuleSize fills m.Size from the module proxy zip's Content-Length when
// want is set. It needs a usable proxy (returning ErrProxyDisabled otherwise)
// and is a no-op when want is false or the API did not resolve a concrete
// version (the zip endpoint requires one).
func (c *Client) applyModuleSize(ctx context.Context, m *Module, want bool) error {
	if !want {
		return nil
	}
	if !c.proxy.Enabled() {
		return ErrProxyDisabled
	}
	version, ok := m.Version.Get()
	if !ok {
		return nil
	}
	size, found, err := c.proxy.ZipSize(ctx, m.Path, version)
	if err != nil {
		return err
	}
	if found {
		m.Size = mo.Some(size)
	}
	return nil
}

// Versions lists the versions of the module at path. WithLimit, WithToken and
// WithFilter tune the listing; WithSize fills each ModuleVersion.Size from the
// module proxy with one concurrent HEAD per version (ErrProxyDisabled when
// GOPROXY is "off"/"direct"-only). Results are paginated (see Page and
// AllVersions to auto-paginate).
func (c *Client) Versions(ctx context.Context, path string, opts ...Option) (*Page[ModuleVersion], error) {
	p := newParams(opts)
	params := api.GetVersionsParams{
		Path:   path,
		Limit:  optInt(p.limit),
		Token:  optStr(p.token),
		Filter: optStr(p.filter),
	}
	v, err, _ := c.sf.versions.Do(sfKey("versions", params, p.size), func() (*Page[ModuleVersion], error) {
		res, err := c.raw.GetVersions(ctx, params)
		if err != nil {
			return nil, err
		}
		page, err := decodePage[ModuleVersion](*res)
		if err != nil {
			return nil, err
		}
		if err := c.applyVersionSizes(ctx, page.Items, p.size); err != nil {
			return nil, err
		}
		return &page, nil
	})
	return v, err
}

// sizeFetchConcurrency caps the number of concurrent proxy HEAD requests issued
// when filling per-version sizes, so a large versions page does not open one
// connection per version at once.
const sizeFetchConcurrency = 16

// applyVersionSizes fills each item's Size from its module-proxy zip
// Content-Length when want is set, fetching them concurrently. It requires a
// usable proxy (ErrProxyDisabled otherwise). Versions unknown to the proxy keep
// Size zero; any other proxy error fails the call.
func (c *Client) applyVersionSizes(ctx context.Context, items []ModuleVersion, want bool) error {
	if !want {
		return nil
	}
	if !c.proxy.Enabled() {
		return ErrProxyDisabled
	}
	g, ctx := errgroup.WithContext(ctx)
	g.SetLimit(sizeFetchConcurrency)
	for i := range items {
		if items[i].ModulePath == "" || items[i].Version == "" {
			continue
		}
		g.Go(func() error {
			size, ok, err := c.proxy.ZipSize(ctx, items[i].ModulePath, items[i].Version)
			if err != nil {
				return err
			}
			if ok {
				items[i].Size = mo.Some(size)
			}
			return nil
		})
	}
	return g.Wait()
}

// Symbols lists the exported symbols of the package at path as lightweight
// SymbolInfo values (use Client.Symbol for one symbol's full doc). Use WithModule
// and WithVersion to scope the package and WithGOOS/WithGOARCH the build context;
// WithLimit, WithToken and WithFilter tune the listing. Results are paginated
// (see Page and AllSymbols to auto-paginate).
func (c *Client) Symbols(ctx context.Context, path string, opts ...Option) (*Page[SymbolInfo], error) {
	p := newParams(opts)
	params := api.GetSymbolsParams{
		Path:    path,
		Module:  optStr(p.module),
		Version: optStr(p.version),
		Goos:    optStr(p.goos),
		Goarch:  optStr(p.goarch),
		Limit:   optInt(p.limit),
		Token:   optStr(p.token),
		Filter:  optStr(p.filter),
	}
	v, err, _ := c.sf.symbols.Do(sfKey("symbols", params), func() (*Page[SymbolInfo], error) {
		res, err := c.raw.GetSymbols(ctx, params)
		if err != nil {
			return nil, err
		}
		page, err := decodePage[SymbolInfo](res.Symbols.Value)
		if err != nil {
			return nil, err
		}
		return &page, nil
	})
	return v, err
}

// Symbol returns the full documentation of a single symbol of the package at path.
// symbol is the exported identifier (e.g. "Map") or "Type.Method" (e.g. "Either.ForEach");
// matching is case-sensitive. WithExamples includes runnable examples.
//
// The documentation is derived client-side from the package documentation, which is always
// requested in Markdown form, so WithDoc has no effect here. It returns ErrSymbolNotFound when
// the symbol is absent from the package.
func (c *Client) Symbol(ctx context.Context, path, symbol string, opts ...Option) (*Symbol, error) {
	p := newParams(opts)
	params := api.GetPackageParams{
		Path:     path,
		Module:   optStr(p.module),
		Version:  optStr(p.version),
		Goos:     optStr(p.goos),
		Goarch:   optStr(p.goarch),
		Doc:      api.NewOptString("markdown"),
		Examples: optBool(p.examples),
	}
	v, err, _ := c.sf.symbol.Do(sfKey("symbol", params, symbol), func() (*Symbol, error) {
		res, err := c.raw.GetPackage(ctx, params)
		if err != nil {
			return nil, err
		}
		parsed, ok := godoc.Parse(res.Docs.Value, symbol, p.examples)
		if !ok {
			return nil, fmt.Errorf("%w: %s", ErrSymbolNotFound, symbol)
		}
		return &Symbol{
			Path:      path,
			Name:      symbol,
			Kind:      parsed.Kind,
			Signature: parsed.Signature,
			Synopsis:  mo.EmptyableToOption(parsed.Synopsis),
			Doc:       mo.EmptyableToOption(parsed.Doc),
			Examples:  toExamples(parsed.Examples),
			Version:   mo.EmptyableToOption(res.Version.Value),
			Goos:      mo.EmptyableToOption(res.Goos.Value),
			Goarch:    mo.EmptyableToOption(res.Goarch.Value),
		}, nil
	})
	return v, err
}

// toExamples maps internal godoc examples to the public Example type.
func toExamples(in []godoc.Example) []Example {
	if len(in) == 0 {
		return nil
	}
	out := make([]Example, 0, len(in))
	for _, e := range in {
		out = append(out, Example{Name: mo.EmptyableToOption(e.Name), Code: e.Code, Output: mo.EmptyableToOption(e.Output)})
	}
	return out
}

// vulnFetchConcurrency caps the number of concurrent OSV report fetches issued
// when expanding a module's vulnerabilities into full entries.
const vulnFetchConcurrency = 16

// Vulns lists known vulnerabilities for the module or package at path, sourced
// from the Go vulnerability database (https://vuln.go.dev) in OSV format.
//
// path may be a module path (exact match) or a package path (matched to its
// owning module); standard-library imports such as "crypto/x509" resolve to the
// "stdlib" pseudo-module. With WithVersion set to a concrete semver, only the
// vulnerabilities actually affecting that version are returned (a symbolic
// version like "latest" disables this filtering; read the covering fix from each
// Vulnerability.Ranges). WithLimit caps the result, which is ordered by ID.
//
// The result is scoped to the queried module: each Vulnerability hoists that one
// module's version ranges and packages to the root. Unlike the other listing
// methods this returns a plain slice — the database is fetched whole, so there
// is no server-side pagination.
func (c *Client) Vulns(ctx context.Context, path string, opts ...Option) ([]Vulnerability, error) {
	p := newParams(opts)
	v, err, _ := c.sf.vulns.Do(sfKey("vulns", path, p.version, p.limit), func() ([]Vulnerability, error) {
		return c.vulnsFor(ctx, path, p.version, p.limit)
	})
	return v, err
}

// vulnsFor implements Vulns: triage the module index for candidate IDs, fetch
// each OSV report concurrently, then apply version and package scoping.
func (c *Client) vulnsFor(ctx context.Context, path, version string, limit int) ([]Vulnerability, error) {
	index, err, _ := c.sf.vulnIndex.Do("modules", func() ([]vuln.ModuleVulns, error) {
		return c.vuln.Modules(ctx)
	})
	if err != nil {
		return nil, err
	}

	cands, moduleQuery := vulnCandidates(index, path)
	if len(cands) == 0 {
		return nil, nil
	}

	ids := make([]string, 0, len(cands))
	for id := range cands {
		ids = append(ids, id)
	}
	sort.Strings(ids)

	versionScoped := vuln.IsSemver(version)
	results := make([]*Vulnerability, len(ids))
	g, ctx := errgroup.WithContext(ctx)
	g.SetLimit(vulnFetchConcurrency)
	for i, id := range ids {
		module := cands[id]
		g.Go(func() error {
			v, err := c.resolveVuln(ctx, id, module, path, version, moduleQuery, versionScoped)
			if err != nil {
				return err
			}
			results[i] = v
			return nil
		})
	}
	if err := g.Wait(); err != nil {
		return nil, err
	}

	out := make([]Vulnerability, 0, len(results))
	for _, r := range results {
		if r != nil {
			out = append(out, *r)
		}
	}
	if limit > 0 && len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}

// vulnCandidates triages the module index for the vulnerabilities relevant to
// path. It returns a map of ID to the owning module path, and whether path is
// itself an indexed module (a module-scoped query) rather than a package.
//
// A package belongs to exactly one module: when path is a package, the owner is
// the most specific (longest) indexed module path that owns it, so nested
// modules (e.g. example.com/root vs example.com/root/sub) resolve deterministically
// regardless of index order. Candidates are collected from that one module only.
func vulnCandidates(index []vuln.ModuleVulns, path string) (map[string]string, bool) {
	moduleQuery := false
	owner := "" // most specific indexed module that owns path
	for _, m := range index {
		switch {
		case m.Path == path:
			moduleQuery, owner = true, m.Path
		case !moduleQuery && moduleOwnsPackage(m.Path, path) && len(m.Path) > len(owner):
			owner = m.Path
		}
	}

	cands := make(map[string]string)
	if owner == "" {
		return cands, moduleQuery
	}
	for _, m := range index {
		if m.Path != owner {
			continue
		}
		for _, iv := range m.Vulns {
			cands[iv.ID] = m.Path
		}
	}
	return cands, moduleQuery
}

// resolveVuln fetches (and deduplicates) the OSV report for id and maps it to a
// Vulnerability scoped to module, applying version and package scoping. It
// returns nil (and no error) when the report is absent or scoped out, so the
// caller drops it.
func (c *Client) resolveVuln(ctx context.Context, id, module, path, version string, moduleQuery, versionScoped bool) (*Vulnerability, error) {
	e, err, _ := c.sf.vulnEntry.Do(id, func() (*vuln.Entry, error) {
		entry, ok, err := c.vuln.Entry(ctx, id)
		if err != nil {
			return nil, err
		}
		if !ok {
			return nil, nil // indexed but no report served.
		}
		return entry, nil
	})
	if err != nil || e == nil {
		return nil, err
	}

	if versionScoped && !versionAffected(e, module, version) {
		return nil, nil // not affected at the requested version.
	}
	if !moduleQuery && !packageAffected(e, module, path) {
		return nil, nil // package not among the affected imports.
	}

	v := toVulnerability(e, module)
	return &v, nil
}

// moduleOwnsPackage reports whether the package pkgPath belongs to modulePath:
// the module path is an exact or directory prefix of the package path. The
// "stdlib" pseudo-module owns every standard-library import (a path whose first
// segment has no dot); "toolchain" owns no importable package.
func moduleOwnsPackage(modulePath, pkgPath string) bool {
	switch modulePath {
	case vuln.ModuleStdlib:
		return isStdlibImport(pkgPath)
	case vuln.ModuleToolchain:
		return false
	}
	return pkgPath == modulePath || strings.HasPrefix(pkgPath, modulePath+"/")
}

// isStdlibImport reports whether p looks like a standard-library import path,
// i.e. its first segment carries no dot (so it is not a domain-qualified module).
func isStdlibImport(p string) bool {
	first, _, _ := strings.Cut(p, "/")
	return first != "" && !strings.Contains(first, ".")
}

// versionAffected reports whether version falls in an affected range of module
// within entry e. The covering fix, if any, is exposed via Vulnerability.Ranges.
func versionAffected(e *vuln.Entry, module, version string) bool {
	for _, a := range e.Affected {
		if a.Module.Path != module {
			continue
		}
		for _, r := range a.Ranges {
			if affected, _ := vuln.RangeAffected(r.Events, version); affected {
				return true
			}
		}
	}
	return false
}

// packageAffected reports whether pkgPath is among the affected imports of
// module within entry e. When the module lists no imports at all (e.g. an
// auto-generated UNREVIEWED entry) it cannot be narrowed, so the whole module is
// considered affected.
func packageAffected(e *vuln.Entry, module, pkgPath string) bool {
	hasImports := false
	for _, a := range e.Affected {
		if a.Module.Path != module {
			continue
		}
		for _, p := range a.EcosystemSpecific.Imports {
			hasImports = true
			if p.Path == pkgPath {
				return true
			}
		}
	}
	return !hasImports
}

// MajorVersions discovers the major versions of the module at modulePath.
//
// In Go, majors beyond v1 live as separate modules (path, path/v2, path/v3...)
// and can be non-contiguous. pkg.go.dev does not (yet) expose a MajorVersions
// endpoint (golang/go#76718), so this derives the answer from the module proxy
// (honoring GOPROXY, see WithGoproxy). modulePath may already carry a major
// suffix (path/v2 or gopkg.in/pkg.v2); it is normalized to the base path first.
//
// WithExcludePseudo drops majors whose latest version is a pseudo-version.
// WithFilter applies a regular expression to each major's module path. WithLimit
// caps the number of returned majors (the proposal's Max), keeping the most
// recent ones. The module proxy has no pagination cursor, so the result is always
// a single page (NextToken is empty); Total is the count before WithLimit.
func (c *Client) MajorVersions(ctx context.Context, modulePath string, opts ...Option) (*Page[MajorVersion], error) {
	p := newParams(opts)
	if !c.proxy.Enabled() {
		return nil, ErrProxyDisabled
	}

	found, err, _ := c.sf.majorVersions.Do(sfKey("majorVersions", modulePath, p.excludePseudo), func() ([]majors.Major, error) {
		return majors.Discover(ctx, c.proxy, modulePath, p.excludePseudo)
	})
	if err != nil {
		if errors.Is(err, majors.ErrInvalidModulePath) {
			return nil, fmt.Errorf("%w: %q", ErrInvalidModulePath, modulePath)
		}
		return nil, err
	}

	var re *regexp.Regexp
	if p.filter != "" {
		if re, err = regexp.Compile(p.filter); err != nil {
			return nil, fmt.Errorf("invalid filter: %w", err)
		}
	}

	// found is sorted newest-major-first by the majors package.
	items := make([]MajorVersion, 0, len(found))
	for _, m := range found {
		if re != nil && !re.MatchString(m.ModulePath) {
			continue
		}
		items = append(items, MajorVersion{ModulePath: m.ModulePath, Major: m.Major, Version: m.Version})
	}
	if len(items) > 0 {
		items[0].IsLatest = true
	}

	total := len(items)
	if p.limit > 0 && len(items) > p.limit {
		items = items[:p.limit]
	}
	return &Page[MajorVersion]{Items: items, Total: total}, nil
}

// Dependencies returns the dependencies declared in the go.mod of the module at
// modulePath, parsed from the Go module proxy (honoring GOPROXY, see
// WithGoproxy). It reports the require directives — each with its version and
// whether it is "// indirect" — plus any replace and exclude directives and the
// module's go directive.
//
// WithVersion selects the version; when unset (or "latest") the proxy's latest
// version is used, and the result's Version is the concrete version the go.mod
// was read at. Dependencies returns ErrProxyDisabled when GOPROXY is
// "off"/"direct"-only, ErrInvalidModulePath for an unparsable path, and
// ErrModuleNotFound when the module version is unknown to every proxy.
func (c *Client) Dependencies(ctx context.Context, modulePath string, opts ...Option) (*DependenciesResult, error) {
	p := newParams(opts)
	if !c.proxy.Enabled() {
		return nil, ErrProxyDisabled
	}

	v, err, _ := c.sf.dependencies.Do(sfKey("dependencies", modulePath, p.version), func() (*DependenciesResult, error) {
		version := p.version
		if version == "" || version == "latest" {
			latest, ok, err := c.proxy.Latest(ctx, modulePath)
			if err != nil {
				return nil, mapProxyErr(err, modulePath)
			}
			if !ok {
				return nil, fmt.Errorf("%w: %q", ErrModuleNotFound, modulePath)
			}
			version = latest
		}

		data, ok, err := c.proxy.Mod(ctx, modulePath, version)
		if err != nil {
			return nil, mapProxyErr(err, modulePath)
		}
		if !ok {
			return nil, fmt.Errorf("%w: %q@%s", ErrModuleNotFound, modulePath, version)
		}

		mod, err := gomod.Parse(modulePath, data)
		if err != nil {
			return nil, fmt.Errorf("parse go.mod for %s@%s: %w", modulePath, version, err)
		}
		return toDependenciesResult(modulePath, version, mod), nil
	})
	return v, err
}

// mapProxyErr translates an internal proxy invalid-path error into the public
// ErrInvalidModulePath, leaving other errors untouched.
func mapProxyErr(err error, modulePath string) error {
	if errors.Is(err, proxy.ErrInvalidModulePath) {
		return fmt.Errorf("%w: %q", ErrInvalidModulePath, modulePath)
	}
	return err
}
