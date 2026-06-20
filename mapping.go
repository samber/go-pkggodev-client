package pkggodev

import (
	"encoding/json"

	"github.com/samber/go-pkggodev-client/internal/api"
)

// --- public -> ogen optional params ---

func optStr(s string) api.OptString {
	if s == "" {
		return api.OptString{}
	}
	return api.NewOptString(s)
}

func optInt(n int) api.OptInt {
	if n == 0 {
		return api.OptInt{}
	}
	return api.NewOptInt(n)
}

func optBool(b bool) api.OptBool {
	if !b {
		return api.OptBool{}
	}
	return api.NewOptBool(true)
}

// --- ogen -> public clean types ---

func toLicenses(in []api.License) []License {
	if len(in) == 0 {
		return nil
	}
	out := make([]License, 0, len(in))
	for _, l := range in {
		out = append(out, License{Contents: l.Contents.Value, FilePath: l.FilePath.Value, Types: l.Types})
	}
	return out
}

func toPackage(p *api.Package) *Package {
	return &Package{
		Path:              p.Path.Value,
		ModulePath:        p.ModulePath.Value,
		Name:              p.Name.Value,
		Synopsis:          p.Synopsis.Value,
		Version:           p.Version.Value,
		Goos:              p.Goos.Value,
		Goarch:            p.Goarch.Value,
		Docs:              p.Docs.Value,
		Imports:           p.Imports,
		IsLatest:          p.IsLatest.Value,
		IsRedistributable: p.IsRedistributable.Value,
		IsStandardLibrary: p.IsStandardLibrary.Value,
		Licenses:          toLicenses(p.Licenses),
	}
}

func toModule(m *api.Module) *Module {
	return &Module{
		Path:              m.Path.Value,
		Version:           m.Version.Value,
		RepoURL:           m.RepoUrl.Value,
		GoModContents:     m.GoModContents.Value,
		CommitTime:        m.CommitTime.Value,
		HasGoMod:          m.HasGoMod.Value,
		IsLatest:          m.IsLatest.Value,
		IsRedistributable: m.IsRedistributable.Value,
		IsStandardLibrary: m.IsStandardLibrary.Value,
		Licenses:          toLicenses(m.Licenses),
		Readme:            Readme{Contents: m.Readme.Value.Contents.Value, Filepath: m.Readme.Value.Filepath.Value},
	}
}

// decodePage turns an ogen PaginatedResponse (whose items are raw JSON) into a
// typed Page[T] by unmarshalling each item into T.
func decodePage[T any](pr api.PaginatedResponse) (Page[T], error) {
	page := Page[T]{NextToken: pr.NextPageToken.Value, Total: pr.Total.Value}
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
