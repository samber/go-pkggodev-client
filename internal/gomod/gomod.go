// Package gomod parses the go.mod file of a module (as served by the Go module
// proxy) into a neutral structure: its Go version, requirements, replacements
// and exclusions. It is the parsing half of Client.Dependencies; the public
// mapping lives in the root package.
package gomod

import (
	"golang.org/x/mod/modfile"
)

// Require is one require directive: a dependency with its version. Indirect
// mirrors the "// indirect" marker (a transitive dependency not imported
// directly by this module's packages).
type Require struct {
	Path     string
	Version  string
	Indirect bool
}

// Replace is one replace directive, redirecting a module (optionally pinned to a
// version) to another module path or a local path. When New has no version it is
// a filesystem replacement.
type Replace struct {
	OldPath    string
	OldVersion string
	NewPath    string
	NewVersion string
}

// Exclude is one exclude directive: a specific module version held back from
// the build.
type Exclude struct {
	Path    string
	Version string
}

// Mod is a parsed go.mod file.
type Mod struct {
	Module    string
	GoVersion string
	Requires  []Require
	Replaces  []Replace
	Excludes  []Exclude
}

// Parse parses the contents of a go.mod file. modulePath is used only for error
// messages. It first tries a strict parse (which records exclude directives,
// unlike ParseLax) and falls back to a lenient parse so an unknown or future
// directive does not break dependency extraction on a newer go.mod.
func Parse(modulePath string, data []byte) (*Mod, error) {
	f, err := modfile.Parse(modulePath, data, nil)
	if err != nil {
		if f, err = modfile.ParseLax(modulePath, data, nil); err != nil {
			return nil, err
		}
	}

	m := &Mod{}
	if f.Module != nil {
		m.Module = f.Module.Mod.Path
	}
	if f.Go != nil {
		m.GoVersion = f.Go.Version
	}
	for _, r := range f.Require {
		m.Requires = append(m.Requires, Require{
			Path:     r.Mod.Path,
			Version:  r.Mod.Version,
			Indirect: r.Indirect,
		})
	}
	for _, r := range f.Replace {
		m.Replaces = append(m.Replaces, Replace{
			OldPath:    r.Old.Path,
			OldVersion: r.Old.Version,
			NewPath:    r.New.Path,
			NewVersion: r.New.Version,
		})
	}
	for _, e := range f.Exclude {
		m.Excludes = append(m.Excludes, Exclude{
			Path:    e.Mod.Path,
			Version: e.Mod.Version,
		})
	}
	return m, nil
}
