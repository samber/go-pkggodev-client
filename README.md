
# Typed Go client for pkg.go.dev

[![tag](https://img.shields.io/github/tag/samber/go-pkggodev-client.svg)](https://github.com/samber/go-pkggodev-client/releases)
![Go Version](https://img.shields.io/badge/Go-%3E%3D%201.25-%23007d9c)
[![GoDoc](https://godoc.org/github.com/samber/go-pkggodev-client?status.svg)](https://pkg.go.dev/github.com/samber/go-pkggodev-client)
![Build Status](https://github.com/samber/go-pkggodev-client/actions/workflows/test.yml/badge.svg)
[![Go report](https://goreportcard.com/badge/github.com/samber/go-pkggodev-client)](https://goreportcard.com/report/github.com/samber/go-pkggodev-client)
[![Contributors](https://img.shields.io/github/contributors/samber/go-pkggodev-client)](https://github.com/samber/go-pkggodev-client/graphs/contributors)
[![License](https://img.shields.io/github/license/samber/go-pkggodev-client)](./LICENSE)

A typed Go client for the [pkg.go.dev](https://pkg.go.dev) API (the "Go Pkgsite API",
`https://pkg.go.dev/v1beta`): search packages and symbols, read documentation, list versions,
importers and known vulnerabilities.

The public API lives at the module root (`package pkggodev`): **context-first methods**,
**functional options**, **clean typed results** (no codegen leakage) and **auto-paginating
iterators**. It wraps an [ogen](https://github.com/ogen-go/ogen)-generated client kept under
[`internal/api`](internal/api).

> [!TIP]
> Looking for a **CLI** instead of a Go library? Use [`samber/godig`](https://github.com/samber/godig) — the pkg.go.dev CLI powered by this client.

## 🚀 Install

```sh
go get github.com/samber/go-pkggodev-client
```

This library is v0 and follows SemVer. It is compatible with Go >= 1.25 (uses `iter.Seq2`).

## 💡 Quick start

```go
import pkggodev "github.com/samber/go-pkggodev-client"

c, err := pkggodev.New(pkggodev.WithUserAgent("my-app/1.0"))
if err != nil {
	panic(err)
}

// Single object.
pkg, _ := c.Package(ctx, "github.com/samber/lo")
fmt.Println(pkg.Path, pkg.Synopsis) // clean strings, no Opt wrappers

// One page.
page, _ := c.Versions(ctx, "github.com/samber/lo", pkggodev.WithLimit(10))
fmt.Println(page.Total, page.NextToken)

// All results, auto-paginated.
for v, err := range c.AllVersions(ctx, "github.com/samber/lo") {
	if err != nil {
		break
	}
	fmt.Println(v.Version, v.CommitTime)
}
```

## 🧠 Spec

GoDoc: [https://pkg.go.dev/github.com/samber/go-pkggodev-client](https://pkg.go.dev/github.com/samber/go-pkggodev-client)

### Client constructor

```go
func New(opts ...ClientOption) (*Client, error)
```

- `WithBaseURL(url)` — override the API base URL.
- `WithHTTPClient(*http.Client)` — custom timeouts / transport.
- `WithUserAgent(string)` — set the `User-Agent` header.

### Methods

All take `context.Context` first and return clean, typed values:

| Method                           | Returns                |
| -------------------------------- | ---------------------- |
| `Search(ctx, opts...)`           | `*Page[SearchResult]`  |
| `Package(ctx, path, opts...)`    | `*Package`             |
| `ImportedBy(ctx, path, opts...)` | `*ImportedByResult`    |
| `Packages(ctx, path, opts...)`   | `*PackagesResult`      |
| `Module(ctx, path, opts...)`     | `*Module`              |
| `Versions(ctx, path, opts...)`   | `*Page[ModuleVersion]` |
| `Symbols(ctx, path, opts...)`    | `*Page[Symbol]`        |
| `SymbolDoc(ctx, path, symbol, opts...)` | `*SymbolDoc`    |
| `Vulns(ctx, path, opts...)`      | `*Page[Vulnerability]` |

`SymbolDoc` returns the documentation of a single symbol (`func`, `type`, `method`, `var` or
`const`) instead of the whole package doc blob — handy to keep token usage low. `symbol` is the
exported identifier (`"Map"`) or `"Type.Method"` (`"Either.ForEach"`); matching is case-sensitive.
The doc is derived client-side from the package documentation (always fetched as Markdown, so
`WithDoc` is ignored here) and the method returns `ErrSymbolNotFound` when the symbol is absent.
Pass `WithExamples` to include runnable examples.

### Call options

`WithVersion`, `WithModule`, `WithLimit`, `WithToken`, `WithFilter`, `WithGOOS`, `WithGOARCH`,
`WithDoc`, `WithQuery`, `WithSymbol`, `WithExamples`, `WithImports`, `WithLicenses`, `WithReadme`.
Each method ignores options that do not apply to it.

### Iterators (auto-pagination)

Each listing endpoint has an `All…` variant returning a Go 1.25 `iter.Seq2[T, error]` that lazily
follows `NextToken` across pages:

`AllSearch`, `AllVersions`, `AllVulns`, `AllSymbols`, `AllPackages`, `AllImportedBy`.

```go
for v, err := range c.AllVersions(ctx, "github.com/samber/lo") {
	if err != nil {
		return err
	}
	fmt.Println(v.Version, v.CommitTime)
}
```

`WithLimit` controls the page size; `WithToken` sets the starting cursor.

## 🤝 Contributing

```sh
# Install dev dependencies
make tools

# Run tests
make test

# Lint
make lint
```

## 👤 Contributors

![Contributors](https://contrib.rocks/image?repo=samber/go-pkggodev-client)

## 💫 Show your support

Give a ⭐️ if this project helped you!

[![GitHub Sponsors](https://img.shields.io/github/sponsors/samber?style=for-the-badge)](https://github.com/sponsors/samber)

## 📝 License

Copyright © 2026 [Samuel Berthe](https://github.com/samber).

This project is [MIT](./LICENSE) licensed.
