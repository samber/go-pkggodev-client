package gomod

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParse(t *testing.T) {
	t.Parallel()

	const data = `module github.com/samber/example/v2

go 1.25

require (
	github.com/samber/lo v1.0.0
	github.com/stretchr/testify v1.11.1
)

require github.com/davecgh/go-spew v1.1.1 // indirect

replace github.com/old/pkg => github.com/new/pkg v1.2.3

replace github.com/local/pkg => ../local

exclude github.com/bad/pkg v0.9.0
`

	m, err := Parse("github.com/samber/example/v2", []byte(data))
	require.NoError(t, err)

	assert.Equal(t, "github.com/samber/example/v2", m.Module)
	assert.Equal(t, "1.25", m.GoVersion)

	assert.Equal(t, []Require{
		{Path: "github.com/samber/lo", Version: "v1.0.0"},
		{Path: "github.com/stretchr/testify", Version: "v1.11.1"},
		{Path: "github.com/davecgh/go-spew", Version: "v1.1.1", Indirect: true},
	}, m.Requires)

	assert.Equal(t, []Replace{
		{OldPath: "github.com/old/pkg", NewPath: "github.com/new/pkg", NewVersion: "v1.2.3"},
		{OldPath: "github.com/local/pkg", NewPath: "../local"},
	}, m.Replaces)

	assert.Equal(t, []Exclude{
		{Path: "github.com/bad/pkg", Version: "v0.9.0"},
	}, m.Excludes)
}

func TestParse_Minimal(t *testing.T) {
	t.Parallel()

	m, err := Parse("example.com/m", []byte("module example.com/m\n"))
	require.NoError(t, err)
	assert.Equal(t, "example.com/m", m.Module)
	assert.Empty(t, m.GoVersion)
	assert.Empty(t, m.Requires)
	assert.Empty(t, m.Replaces)
	assert.Empty(t, m.Excludes)
}

// TestParse_Lax checks that an unknown/future directive does not break parsing.
func TestParse_Lax(t *testing.T) {
	t.Parallel()

	const data = `module example.com/m

go 1.25

tool example.com/m/cmd/gen

require github.com/samber/lo v1.0.0
`
	m, err := Parse("example.com/m", []byte(data))
	require.NoError(t, err)
	assert.Equal(t, "1.25", m.GoVersion)
	require.Len(t, m.Requires, 1)
	assert.Equal(t, "github.com/samber/lo", m.Requires[0].Path)
}
