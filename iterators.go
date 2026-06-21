package pkggodev

import (
	"context"
	"iter"
)

// paginate turns a page-fetching function into an auto-paginating iterator: it
// follows NextToken until exhausted, yielding each item. A fetch error is
// yielded once (with the zero value) and ends iteration.
func paginate[T any](fetch func(token string) (Page[T], error)) iter.Seq2[T, error] {
	return func(yield func(T, error) bool) {
		token := ""
		for {
			page, err := fetch(token)
			if err != nil {
				var zero T
				yield(zero, err)
				return
			}
			for _, item := range page.Items {
				if !yield(item, nil) {
					return
				}
			}
			if page.NextToken == "" || len(page.Items) == 0 {
				return
			}
			token = page.NextToken
		}
	}
}

// withToken appends a starting-cursor option without mutating the caller slice.
func withToken(opts []Option, token string) []Option {
	if token == "" {
		return opts
	}
	return append(append([]Option{}, opts...), WithToken(token))
}

// AllSearch iterates all search results, fetching pages lazily.
func (c *Client) AllSearch(ctx context.Context, opts ...Option) iter.Seq2[SearchResult, error] {
	return paginate(func(token string) (Page[SearchResult], error) {
		p, err := c.Search(ctx, withToken(opts, token)...)
		if err != nil {
			return Page[SearchResult]{}, err
		}
		return *p, nil
	})
}

// AllVersions iterates all versions of the module at path, fetching pages lazily.
func (c *Client) AllVersions(ctx context.Context, path string, opts ...Option) iter.Seq2[ModuleVersion, error] {
	return paginate(func(token string) (Page[ModuleVersion], error) {
		p, err := c.Versions(ctx, path, withToken(opts, token)...)
		if err != nil {
			return Page[ModuleVersion]{}, err
		}
		return *p, nil
	})
}

// AllVulns iterates all vulnerabilities for the module or package at path.
func (c *Client) AllVulns(ctx context.Context, path string, opts ...Option) iter.Seq2[Vulnerability, error] {
	return paginate(func(token string) (Page[Vulnerability], error) {
		p, err := c.Vulns(ctx, path, withToken(opts, token)...)
		if err != nil {
			return Page[Vulnerability]{}, err
		}
		return *p, nil
	})
}

// AllSymbols iterates all exported symbols of the package at path.
func (c *Client) AllSymbols(ctx context.Context, path string, opts ...Option) iter.Seq2[SymbolInfo, error] {
	return paginate(func(token string) (Page[SymbolInfo], error) {
		p, err := c.Symbols(ctx, path, withToken(opts, token)...)
		if err != nil {
			return Page[SymbolInfo]{}, err
		}
		return *p, nil
	})
}

// AllPackages iterates all packages contained in the module at path.
func (c *Client) AllPackages(ctx context.Context, path string, opts ...Option) iter.Seq2[PackageInfo, error] {
	return paginate(func(token string) (Page[PackageInfo], error) {
		r, err := c.Packages(ctx, path, withToken(opts, token)...)
		if err != nil {
			return Page[PackageInfo]{}, err
		}
		return r.Packages, nil
	})
}

// AllImportedBy iterates all packages that import the package at path.
func (c *Client) AllImportedBy(ctx context.Context, path string, opts ...Option) iter.Seq2[string, error] {
	return paginate(func(token string) (Page[string], error) {
		r, err := c.ImportedBy(ctx, path, withToken(opts, token)...)
		if err != nil {
			return Page[string]{}, err
		}
		return r.Packages, nil
	})
}
