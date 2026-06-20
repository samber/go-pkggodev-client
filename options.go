package pkggodev

// Option configures optional query parameters for a single API call. Each
// method ignores options that do not apply to it.
type Option func(*params)

type params struct {
	version, module, filter, token, goos, goarch, doc, query, symbol string
	limit                                                            int
	examples, imports, licenses, readme                              bool
}

// WithVersion selects a module version (semver, "latest", "master" or "main").
func WithVersion(v string) Option { return func(p *params) { p.version = v } }

// WithModule sets the module path for package-scoped calls.
func WithModule(m string) Option { return func(p *params) { p.module = m } }

// WithFilter applies a regular-expression filter to listing results.
func WithFilter(f string) Option { return func(p *params) { p.filter = f } }

// WithToken resumes a listing from a pagination token.
func WithToken(t string) Option { return func(p *params) { p.token = t } }

// WithLimit caps the number of items returned.
func WithLimit(n int) Option { return func(p *params) { p.limit = n } }

// WithGOOS sets the GOOS documentation build context.
func WithGOOS(s string) Option { return func(p *params) { p.goos = s } }

// WithGOARCH sets the GOARCH documentation build context.
func WithGOARCH(s string) Option { return func(p *params) { p.goarch = s } }

// WithDoc sets the documentation format (text, html, md or markdown).
func WithDoc(s string) Option { return func(p *params) { p.doc = s } }

// WithQuery sets the package search query (Search only).
func WithQuery(q string) Option { return func(p *params) { p.query = q } }

// WithSymbol sets the symbol search query (Search only).
func WithSymbol(s string) Option { return func(p *params) { p.symbol = s } }

// WithExamples includes examples with returned documentation.
func WithExamples() Option { return func(p *params) { p.examples = true } }

// WithImports includes the list of packages the target imports.
func WithImports() Option { return func(p *params) { p.imports = true } }

// WithLicenses includes licenses in the result.
func WithLicenses() Option { return func(p *params) { p.licenses = true } }

// WithReadme includes the README in the result (Module only).
func WithReadme() Option { return func(p *params) { p.readme = true } }

func newParams(opts []Option) params {
	var p params
	for _, o := range opts {
		o(&p)
	}
	return p
}
