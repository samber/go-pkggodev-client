package pkggodev

import "time"

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
	RepoURL           string    `json:"repoUrl,omitempty"`
	GoModContents     string    `json:"goModContents,omitempty"`
	CommitTime        time.Time `json:"commitTime,omitzero"`
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
	HasGoMod          bool      `json:"hasGoMod"`
	IsRedistributable bool      `json:"isRedistributable"`
	Deprecated        bool      `json:"deprecated"`
	DeprecationReason string    `json:"deprecationReason"`
	Retracted         bool      `json:"retracted"`
	RetractionReason  string    `json:"retractionReason"`
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
