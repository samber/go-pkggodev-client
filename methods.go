package pkggodev

import (
	"context"
	"errors"
	"fmt"
	"regexp"

	"golang.org/x/sync/errgroup"

	"github.com/samber/go-pkggodev-client/internal/api"
	"github.com/samber/go-pkggodev-client/internal/godoc"
	"github.com/samber/go-pkggodev-client/internal/gomod"
	"github.com/samber/go-pkggodev-client/internal/majors"
	"github.com/samber/go-pkggodev-client/internal/proxy"
)

// Search finds packages and symbols. Use WithQuery and/or WithSymbol.
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

// Package returns documentation and metadata for the package at path.
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

// ImportedBy lists the packages that import the package at path.
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
		return &ImportedByResult{ModulePath: res.ModulePath.Value, Version: res.Version.Value, Packages: page}, nil
	})
	return v, err
}

// Packages lists the packages contained in the module at path.
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
			ModulePath:        res.ModulePath.Value,
			Version:           res.Version.Value,
			IsStandardLibrary: res.IsStandardLibrary.Value,
			Packages:          page,
		}, nil
	})
	return v, err
}

// Module returns metadata for the module at path.
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
	if m.Version == "" {
		return nil
	}
	size, ok, err := c.proxy.ZipSize(ctx, m.Path, m.Version)
	if err != nil {
		return err
	}
	if ok {
		m.Size = size
	}
	return nil
}

// Versions lists the versions of the module at path.
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
				items[i].Size = size
			}
			return nil
		})
	}
	return g.Wait()
}

// Symbols lists the exported symbols of the package at path.
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
			Synopsis:  parsed.Synopsis,
			Doc:       parsed.Doc,
			Examples:  toExamples(parsed.Examples),
			Version:   res.Version.Value,
			Goos:      res.Goos.Value,
			Goarch:    res.Goarch.Value,
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
		out = append(out, Example{Name: e.Name, Code: e.Code, Output: e.Output})
	}
	return out
}

// Vulns lists known vulnerabilities for the module or package at path.
func (c *Client) Vulns(ctx context.Context, path string, opts ...Option) (*Page[Vulnerability], error) {
	p := newParams(opts)
	params := api.GetVulnsParams{
		Path:    path,
		Module:  optStr(p.module),
		Version: optStr(p.version),
		Limit:   optInt(p.limit),
		Token:   optStr(p.token),
		Filter:  optStr(p.filter),
	}
	v, err, _ := c.sf.vulns.Do(sfKey("vulns", params), func() (*Page[Vulnerability], error) {
		res, err := c.raw.GetVulns(ctx, params)
		if err != nil {
			return nil, err
		}
		page, err := decodePage[Vulnerability](*res)
		if err != nil {
			return nil, err
		}
		return &page, nil
	})
	return v, err
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
