package pkggodev

import (
	"context"
	"fmt"

	"github.com/samber/go-pkggodev-client/internal/api"
)

// Search finds packages and symbols. Use WithQuery and/or WithSymbol.
func (c *Client) Search(ctx context.Context, opts ...Option) (*Page[SearchResult], error) {
	p := newParams(opts)
	res, err := c.raw.GetSearch(ctx, api.GetSearchParams{
		Q:      optStr(p.query),
		Symbol: optStr(p.symbol),
		Limit:  optInt(p.limit),
		Token:  optStr(p.token),
		Filter: optStr(p.filter),
	})
	if err != nil {
		return nil, err
	}
	page, err := decodePage[SearchResult](*res)
	if err != nil {
		return nil, err
	}
	return &page, nil
}

// Package returns documentation and metadata for the package at path.
func (c *Client) Package(ctx context.Context, path string, opts ...Option) (*Package, error) {
	p := newParams(opts)
	res, err := c.raw.GetPackage(ctx, api.GetPackageParams{
		Path:     path,
		Module:   optStr(p.module),
		Version:  optStr(p.version),
		Goos:     optStr(p.goos),
		Goarch:   optStr(p.goarch),
		Doc:      optStr(p.doc),
		Examples: optBool(p.examples),
		Imports:  optBool(p.imports),
		Licenses: optBool(p.licenses),
	})
	if err != nil {
		return nil, err
	}
	return toPackage(res), nil
}

// ImportedBy lists the packages that import the package at path.
func (c *Client) ImportedBy(ctx context.Context, path string, opts ...Option) (*ImportedByResult, error) {
	p := newParams(opts)
	res, err := c.raw.GetImportedBy(ctx, api.GetImportedByParams{
		Path:    path,
		Module:  optStr(p.module),
		Version: optStr(p.version),
		Limit:   optInt(p.limit),
		Token:   optStr(p.token),
		Filter:  optStr(p.filter),
	})
	if err != nil {
		return nil, err
	}
	page, err := decodePage[string](res.ImportedBy.Value)
	if err != nil {
		return nil, err
	}
	return &ImportedByResult{ModulePath: res.ModulePath.Value, Version: res.Version.Value, Packages: page}, nil
}

// Packages lists the packages contained in the module at path.
func (c *Client) Packages(ctx context.Context, path string, opts ...Option) (*PackagesResult, error) {
	p := newParams(opts)
	res, err := c.raw.GetPackages(ctx, api.GetPackagesParams{
		Path:    path,
		Version: optStr(p.version),
		Limit:   optInt(p.limit),
		Token:   optStr(p.token),
		Filter:  optStr(p.filter),
	})
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
}

// Module returns metadata for the module at path.
func (c *Client) Module(ctx context.Context, path string, opts ...Option) (*Module, error) {
	p := newParams(opts)
	res, err := c.raw.GetModule(ctx, api.GetModuleParams{
		Path:     path,
		Version:  optStr(p.version),
		Licenses: optBool(p.licenses),
		Readme:   optBool(p.readme),
	})
	if err != nil {
		return nil, err
	}
	return toModule(res), nil
}

// Versions lists the versions of the module at path.
func (c *Client) Versions(ctx context.Context, path string, opts ...Option) (*Page[ModuleVersion], error) {
	p := newParams(opts)
	res, err := c.raw.GetVersions(ctx, api.GetVersionsParams{
		Path:   path,
		Limit:  optInt(p.limit),
		Token:  optStr(p.token),
		Filter: optStr(p.filter),
	})
	if err != nil {
		return nil, err
	}
	page, err := decodePage[ModuleVersion](*res)
	if err != nil {
		return nil, err
	}
	return &page, nil
}

// Symbols lists the exported symbols of the package at path.
func (c *Client) Symbols(ctx context.Context, path string, opts ...Option) (*Page[SymbolInfo], error) {
	p := newParams(opts)
	res, err := c.raw.GetSymbols(ctx, api.GetSymbolsParams{
		Path:    path,
		Module:  optStr(p.module),
		Version: optStr(p.version),
		Goos:    optStr(p.goos),
		Goarch:  optStr(p.goarch),
		Limit:   optInt(p.limit),
		Token:   optStr(p.token),
		Filter:  optStr(p.filter),
	})
	if err != nil {
		return nil, err
	}
	page, err := decodePage[SymbolInfo](res.Symbols.Value)
	if err != nil {
		return nil, err
	}
	return &page, nil
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
	res, err := c.raw.GetPackage(ctx, api.GetPackageParams{
		Path:     path,
		Module:   optStr(p.module),
		Version:  optStr(p.version),
		Goos:     optStr(p.goos),
		Goarch:   optStr(p.goarch),
		Doc:      api.NewOptString("markdown"),
		Examples: optBool(p.examples),
	})
	if err != nil {
		return nil, err
	}
	sym, ok := parseSymbol(res.Docs.Value, symbol, p.examples)
	if !ok {
		return nil, fmt.Errorf("%w: %s", ErrSymbolNotFound, symbol)
	}
	sym.Path = path
	sym.Name = symbol
	sym.Version = res.Version.Value
	sym.Goos = res.Goos.Value
	sym.Goarch = res.Goarch.Value
	return sym, nil
}

// Vulns lists known vulnerabilities for the module or package at path.
func (c *Client) Vulns(ctx context.Context, path string, opts ...Option) (*Page[Vulnerability], error) {
	p := newParams(opts)
	res, err := c.raw.GetVulns(ctx, api.GetVulnsParams{
		Path:    path,
		Module:  optStr(p.module),
		Version: optStr(p.version),
		Limit:   optInt(p.limit),
		Token:   optStr(p.token),
		Filter:  optStr(p.filter),
	})
	if err != nil {
		return nil, err
	}
	page, err := decodePage[Vulnerability](*res)
	if err != nil {
		return nil, err
	}
	return &page, nil
}
