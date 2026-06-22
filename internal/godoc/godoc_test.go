package godoc_test

import (
	"strings"
	"testing"

	"github.com/samber/go-pkggodev-client/internal/godoc"
)

// sampleDoc is a small doc=markdown blob mirroring the pkg.go.dev layout.
var sampleDoc = strings.Join([]string{
	"# package lo",
	"",
	"## Constants",
	"",
	"```go",
	"const Pi = 3.14",
	"```",
	"The Pi constant. Approximate.",
	"",
	"## Variables",
	"",
	"```go",
	"var (",
	"\tErrA = errors.New(\"a\")",
	"\tErrB = errors.New(\"b\")",
	")",
	"```",
	"Sentinel errors.",
	"",
	"## Functions",
	"",
	"```go",
	"func Assign[K comparable, V any](maps ...map[K]V) map[K]V",
	"```",
	"Assign merges maps. Later keys win.",
	"",
	"#### Example (Basic)",
	"",
	"```go",
	"fmt.Println(Assign(a, b))",
	"```",
	"",
	"Output:",
	"",
	"```",
	"map[x:1]",
	"```",
	"",
	"## Types",
	"",
	"```go",
	"type Entry[K comparable, V any] struct{}",
	"```",
	"Entry is a key/value pair.",
	"",
	"```go",
	"func (e Entry[K, V]) Key() K",
	"```",
	"Key returns the key.",
}, "\n")

func TestParse_FunctionWithExample(t *testing.T) {
	t.Parallel()
	got, ok := godoc.Parse(sampleDoc, "Assign", true)
	if !ok {
		t.Fatal("Assign not found")
	}
	if got.Kind != godoc.KindFunction {
		t.Errorf("Kind = %q, want Function", got.Kind)
	}
	if !strings.HasPrefix(got.Signature, "func Assign[") {
		t.Errorf("Signature = %q", got.Signature)
	}
	if got.Synopsis != "Assign merges maps." {
		t.Errorf("Synopsis = %q", got.Synopsis)
	}
	if len(got.Examples) != 1 || got.Examples[0].Name != "Basic" || got.Examples[0].Output != "map[x:1]" {
		t.Errorf("Examples = %+v", got.Examples)
	}
}

func TestParse_WithoutExamples(t *testing.T) {
	t.Parallel()
	got, ok := godoc.Parse(sampleDoc, "Assign", false)
	if !ok || len(got.Examples) != 0 {
		t.Errorf("expected no examples, got %+v", got.Examples)
	}
}

func TestParse_Kinds(t *testing.T) {
	t.Parallel()
	cases := map[string]string{
		"Pi":        godoc.KindConstant,
		"ErrA":      godoc.KindVariable, // grouped var block
		"ErrB":      godoc.KindVariable,
		"Entry":     godoc.KindType,
		"Entry.Key": godoc.KindMethod,
		"Assign":    godoc.KindFunction,
	}
	for name, wantKind := range cases {
		got, ok := godoc.Parse(sampleDoc, name, false)
		if !ok {
			t.Errorf("%s: not found", name)
			continue
		}
		if got.Kind != wantKind {
			t.Errorf("%s: Kind = %q, want %q", name, got.Kind, wantKind)
		}
	}
}

func TestParse_NotFound(t *testing.T) {
	t.Parallel()
	if _, ok := godoc.Parse(sampleDoc, "DoesNotExist", false); ok {
		t.Error("expected not found")
	}
	if _, ok := godoc.Parse(sampleDoc, "assign", false); ok {
		t.Error("matching must be case-sensitive")
	}
}
