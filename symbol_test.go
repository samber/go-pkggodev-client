package pkggodev_test

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	pkggodev "github.com/samber/go-pkggodev-client"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// sampleDoc is a representative doc=markdown blob mirroring the pkg.go.dev layout.
var sampleDoc = strings.Join([]string{
	"# package lo",
	"",
	"## Variables",
	"",
	"```go",
	"var (",
	"\t//nolint:revive",
	"\tLowerCaseLettersCharset = []rune(\"abcdefghijklmnopqrstuvwxyz\")",
	"\tUpperCaseLettersCharset = []rune(\"ABCDEFGHIJKLMNOPQRSTUVWXYZ\")",
	")",
	"```",
	"",
	"## Functions",
	"",
	"```go",
	"func Assign[K comparable, V any, Map ~map[K]V](maps ...Map) Map",
	"```",
	"Assign merges multiple maps from left to right. Play: [link](https://go.dev/play/p/x)",
	"",
	"## Types",
	"",
	"```go",
	"type Entry[K comparable, V any] struct {",
	"\tKey   K",
	"\tValue V",
	"}",
	"```",
	"Entry defines a key/value pairs.",
	"",
	"```go",
	"func Entries[K comparable, V any](in map[K]V) []Entry[K, V]",
	"```",
	"Entries transforms a map into a slice of key/value pairs.",
	"",
	"#### Example",
	"",
	"```go",
	"{",
	"\tresult := Entries(kv)",
	"\tfmt.Printf(\"%v\", result)",
	"}",
	"```",
	"Output:",
	"",
	"```",
	"[{bar 2} {baz 3} {foo 1}]",
	"```",
	"",
	"```go",
	"func (e Either[L, R]) ForEach(leftCb func(L), rightCb func(R))",
	"```",
	"ForEach executes the given side-effecting function.",
}, "\n")

// docHandler serves sampleDoc as a /package response and records the request query.
func docHandler(t *testing.T, query *map[string]string) http.HandlerFunc {
	t.Helper()
	return func(w http.ResponseWriter, r *http.Request) {
		if query != nil {
			m := map[string]string{}
			for k := range r.URL.Query() {
				m[k] = r.URL.Query().Get(k)
			}
			*query = m
		}
		w.Header().Set("Content-Type", "application/json")
		body, _ := json.Marshal(map[string]any{
			"version": "v1.53.0",
			"goos":    "all",
			"goarch":  "all",
			"docs":    sampleDoc,
		})
		_, _ = w.Write(body)
	}
}

func TestSymbol_Function(t *testing.T) {
	t.Parallel()
	var q map[string]string
	c := newClient(t, docHandler(t, &q))

	sd, err := c.Symbol(context.Background(), "github.com/samber/lo", "Assign")
	require.NoError(t, err)
	assert.Equal(t, "markdown", q["doc"]) // always requested in Markdown form
	assert.Equal(t, "Assign", sd.Name)
	assert.Equal(t, "github.com/samber/lo", sd.Path)
	assert.Equal(t, "Function", sd.Kind)
	assert.Equal(t, "func Assign[K comparable, V any, Map ~map[K]V](maps ...Map) Map", sd.Signature)
	assert.Equal(t, "Assign merges multiple maps from left to right.", sd.Synopsis)
	assert.Contains(t, sd.Doc, "Play:")
	assert.Equal(t, "v1.53.0", sd.Version)
	assert.Equal(t, "all", sd.Goos)
	assert.Empty(t, sd.Examples)
}

func TestSymbol_Type(t *testing.T) {
	t.Parallel()
	c := newClient(t, docHandler(t, nil))

	sd, err := c.Symbol(context.Background(), "github.com/samber/lo", "Entry")
	require.NoError(t, err)
	assert.Equal(t, "Type", sd.Kind)
	assert.Contains(t, sd.Signature, "type Entry[K comparable, V any] struct {")
	assert.Contains(t, sd.Signature, "Key") // full multi-line signature
	assert.Equal(t, "Entry defines a key/value pairs.", sd.Synopsis)
}

func TestSymbol_TypeMethod(t *testing.T) {
	t.Parallel()
	c := newClient(t, docHandler(t, nil))

	sd, err := c.Symbol(context.Background(), "github.com/samber/lo", "Either.ForEach")
	require.NoError(t, err)
	assert.Equal(t, "Method", sd.Kind)
	assert.Equal(t, "func (e Either[L, R]) ForEach(leftCb func(L), rightCb func(R))", sd.Signature)
	assert.Equal(t, "ForEach executes the given side-effecting function.", sd.Synopsis)
}

func TestSymbol_GroupedVariable(t *testing.T) {
	t.Parallel()
	c := newClient(t, docHandler(t, nil))

	sd, err := c.Symbol(context.Background(), "github.com/samber/lo", "UpperCaseLettersCharset")
	require.NoError(t, err)
	assert.Equal(t, "Variable", sd.Kind)
	assert.Contains(t, sd.Signature, "var (")
}

func TestSymbol_NotFound(t *testing.T) {
	t.Parallel()
	c := newClient(t, docHandler(t, nil))

	sd, err := c.Symbol(context.Background(), "github.com/samber/lo", "DoesNotExist")
	require.ErrorIs(t, err, pkggodev.ErrSymbolNotFound)
	assert.Nil(t, sd)
}

func TestSymbol_CaseSensitive(t *testing.T) {
	t.Parallel()
	c := newClient(t, docHandler(t, nil))

	_, err := c.Symbol(context.Background(), "github.com/samber/lo", "assign")
	require.ErrorIs(t, err, pkggodev.ErrSymbolNotFound)
}

func TestSymbol_WithExamples(t *testing.T) {
	t.Parallel()
	var q map[string]string
	c := newClient(t, docHandler(t, &q))

	sd, err := c.Symbol(context.Background(), "github.com/samber/lo", "Entries", pkggodev.WithExamples())
	require.NoError(t, err)
	assert.Equal(t, "true", q["examples"])
	require.Len(t, sd.Examples, 1)
	assert.Empty(t, sd.Examples[0].Name)
	assert.Contains(t, sd.Examples[0].Code, "Entries(kv)")
	assert.Equal(t, "[{bar 2} {baz 3} {foo 1}]", sd.Examples[0].Output)
}

func TestSymbol_WithoutExamples(t *testing.T) {
	t.Parallel()
	c := newClient(t, docHandler(t, nil))

	sd, err := c.Symbol(context.Background(), "github.com/samber/lo", "Entries")
	require.NoError(t, err)
	assert.Empty(t, sd.Examples)
	// Example code must not leak into the doc prose.
	assert.NotContains(t, sd.Doc, "fmt.Printf")
}
