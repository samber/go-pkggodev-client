// Package godoc derives the documentation of a single symbol from the package
// documentation returned by pkg.go.dev in Markdown form (doc=markdown).
//
// The pkg.go.dev backend exposes no per-symbol documentation endpoint, so the
// package doc blob is parsed client-side. The parsing therefore depends on the
// upstream Markdown layout: section headers (## Functions, ...), fenced ```go
// declaration blocks, prose paragraphs and #### Example sections. It is isolated
// here and covered by fixture tests so any upstream format drift is caught
// quickly.
package godoc

import (
	"slices"
	"strings"
)

// Symbol kinds.
const (
	KindFunction = "Function"
	KindMethod   = "Method"
	KindType     = "Type"
	KindVariable = "Variable"
	KindConstant = "Constant"
)

// Symbol is the parsed documentation of a single package symbol.
type Symbol struct {
	Kind      string // Function, Method, Type, Variable or Constant.
	Signature string
	Synopsis  string
	Doc       string
	Examples  []Example
}

// Example is a runnable example attached to a symbol.
type Example struct {
	Name   string
	Code   string
	Output string
}

// tokenKind classifies a line-level token of the Markdown doc blob.
type tokenKind int

const (
	tokProse        tokenKind = iota
	tokH2                     // "## Section" header.
	tokExample                // "#### Example" / "#### Example (name)" header.
	tokOutputMarker           // a lone "Output:" line.
	tokGoCode                 // a fenced ```go ... ``` block.
	tokPlainCode              // a fenced ``` ... ``` block (no language).
)

type docToken struct {
	kind tokenKind
	text string
}

// symbolEntry is a declaration block with its attached prose and examples.
type symbolEntry struct {
	names     []string
	kind      string
	signature string
	doc       []string
	examples  []Example
}

// Parse extracts the documentation of symbol from a doc=markdown blob. It returns
// false when the symbol is not declared in the package.
func Parse(md, symbol string, withExamples bool) (Symbol, bool) {
	entries := buildEntries(tokenizeDoc(md), withExamples)
	for i := range entries {
		if !slices.Contains(entries[i].names, symbol) {
			continue
		}
		doc := strings.TrimSpace(strings.Join(entries[i].doc, "\n"))
		return Symbol{
			Kind:      entries[i].kind,
			Signature: strings.TrimSpace(entries[i].signature),
			Synopsis:  firstSentence(doc),
			Doc:       doc,
			Examples:  entries[i].examples,
		}, true
	}
	return Symbol{}, false
}

// tokenizeDoc splits the Markdown blob into line-level tokens.
func tokenizeDoc(md string) []docToken {
	lines := strings.Split(md, "\n")
	var toks []docToken
	for i := 0; i < len(lines); i++ {
		line := lines[i]
		switch {
		case line == "```go":
			body, next := readFence(lines, i+1)
			toks = append(toks, docToken{kind: tokGoCode, text: body})
			i = next
		case line == "```":
			body, next := readFence(lines, i+1)
			toks = append(toks, docToken{kind: tokPlainCode, text: body})
			i = next
		case strings.HasPrefix(line, "## "):
			toks = append(toks, docToken{kind: tokH2, text: strings.TrimPrefix(line, "## ")})
		case strings.HasPrefix(line, "#### Example"):
			toks = append(toks, docToken{kind: tokExample, text: exampleName(line)})
		case strings.TrimSpace(line) == "Output:":
			toks = append(toks, docToken{kind: tokOutputMarker})
		default:
			toks = append(toks, docToken{kind: tokProse, text: line})
		}
	}
	return toks
}

// readFence collects fenced-block lines starting at start until the closing
// "```", returning the joined body and the index of the closing fence.
func readFence(lines []string, start int) (string, int) {
	i := start
	for i < len(lines) && lines[i] != "```" {
		i++
	}
	return strings.Join(lines[start:i], "\n"), i
}

// buildEntries turns the token stream into declaration entries, attaching doc
// prose and (when requested) examples to the current entry.
func buildEntries(toks []docToken, withExamples bool) []symbolEntry {
	var entries []symbolEntry
	section := ""
	cur := -1
	for i := 0; i < len(toks); i++ {
		switch toks[i].kind {
		case tokH2:
			section, cur = toks[i].text, -1
		case tokGoCode:
			names, kind := declNames(toks[i].text, section)
			entries = append(entries, symbolEntry{names: names, kind: kind, signature: toks[i].text})
			cur = len(entries) - 1
		case tokExample:
			if cur >= 0 {
				ex, next := parseExample(toks, i, toks[i].text)
				if withExamples {
					entries[cur].examples = append(entries[cur].examples, ex)
				}
				i = next
			}
		case tokProse:
			if cur >= 0 && len(entries[cur].examples) == 0 {
				entries[cur].doc = append(entries[cur].doc, toks[i].text)
			}
		case tokOutputMarker, tokPlainCode:
			// Only meaningful inside an example, consumed by parseExample.
		}
	}
	return entries
}

// parseExample reads one example (code and optional output) starting at the
// "#### Example" token at start, returning it and the index of its last token.
func parseExample(toks []docToken, start int, name string) (Example, int) {
	ex := Example{Name: name}
	i := start + 1
	for i < len(toks) && toks[i].kind != tokGoCode {
		if toks[i].kind == tokH2 || toks[i].kind == tokExample {
			return ex, i - 1
		}
		i++
	}
	if i >= len(toks) {
		return ex, len(toks) - 1
	}
	ex.Code = strings.TrimSpace(toks[i].text)
	last := i
	j := i + 1
	for j < len(toks) && toks[j].kind == tokProse {
		j++
	}
	if j < len(toks) && toks[j].kind == tokOutputMarker {
		if k := nextPlainCode(toks, j+1); k >= 0 {
			ex.Output, last = strings.TrimSpace(toks[k].text), k
		}
	}
	return ex, last
}

// nextPlainCode returns the index of the next plain code block, skipping prose,
// or -1 if another structural token appears first.
func nextPlainCode(toks []docToken, from int) int {
	for j := from; j < len(toks); j++ {
		if toks[j].kind == tokProse {
			continue
		}
		if toks[j].kind == tokPlainCode {
			return j
		}
		return -1
	}
	return -1
}

// declNames returns the identifiers declared by a ```go block and their kind.
func declNames(code, section string) ([]string, string) {
	line := strings.TrimSpace(firstNonEmptyLine(code))
	switch {
	case strings.HasPrefix(line, "func ("):
		return []string{methodName(line)}, KindMethod
	case strings.HasPrefix(line, "func "):
		return []string{identAfter(line, "func ")}, KindFunction
	case strings.HasPrefix(line, "type "):
		return []string{identAfter(line, "type ")}, KindType
	case strings.HasPrefix(line, "var "):
		return varConstNames(code, line, "var"), KindVariable
	case strings.HasPrefix(line, "const "):
		return varConstNames(code, line, "const"), KindConstant
	}
	return nil, kindFromSection(section)
}

// methodName returns "Receiver.Method" for a method declaration line.
func methodName(line string) string {
	open := strings.IndexByte(line, '(')
	closeIdx := strings.IndexByte(line, ')')
	if open < 0 || closeIdx <= open {
		return ""
	}
	recv := strings.TrimSpace(line[open+1 : closeIdx])
	if sp := strings.IndexByte(recv, ' '); sp >= 0 {
		recv = recv[sp+1:]
	}
	recv = strings.TrimPrefix(recv, "*")
	if b := strings.IndexByte(recv, '['); b >= 0 {
		recv = recv[:b]
	}
	method := leadingIdent(strings.TrimSpace(line[closeIdx+1:]))
	if recv == "" || method == "" {
		return ""
	}
	return recv + "." + method
}

// identAfter returns the leading identifier following prefix in line.
func identAfter(line, prefix string) string {
	return leadingIdent(strings.TrimSpace(strings.TrimPrefix(line, prefix)))
}

// varConstNames returns the names declared by a var/const block, handling both
// single declarations and grouped "var ( ... )" blocks.
func varConstNames(code, firstLine, keyword string) []string {
	if strings.HasPrefix(firstLine, keyword+" (") {
		return groupedNames(code)
	}
	if n := identAfter(firstLine, keyword+" "); n != "" {
		return []string{n}
	}
	return nil
}

// groupedNames extracts the leading identifier of each declaration line inside a
// grouped var/const block.
func groupedNames(code string) []string {
	var names []string
	for raw := range strings.SplitSeq(code, "\n") {
		l := strings.TrimSpace(raw)
		switch {
		case l == "" || l == ")" || l == "(":
			continue
		case strings.HasPrefix(l, "//"):
			continue
		case strings.HasPrefix(l, "var") || strings.HasPrefix(l, "const"):
			continue
		}
		if n := leadingIdent(l); n != "" {
			names = append(names, n)
		}
	}
	return names
}

// firstNonEmptyLine returns the first non-blank line of s.
func firstNonEmptyLine(s string) string {
	for l := range strings.SplitSeq(s, "\n") {
		if strings.TrimSpace(l) != "" {
			return l
		}
	}
	return ""
}

// firstSentence returns the first sentence of s (up to the first period followed
// by a space, newline or end of string), mirroring go/doc synopsis behaviour.
func firstSentence(s string) string {
	s = strings.TrimSpace(s)
	for i := 0; i < len(s); i++ {
		if s[i] != '.' {
			continue
		}
		if i+1 == len(s) || s[i+1] == ' ' || s[i+1] == '\n' {
			return s[:i+1]
		}
	}
	return s
}

// leadingIdent returns the leading run of Go identifier characters of s.
func leadingIdent(s string) string {
	i := 0
	for i < len(s) && isIdentChar(s[i]) {
		i++
	}
	return s[:i]
}

func isIdentChar(b byte) bool {
	return b == '_' ||
		(b >= 'a' && b <= 'z') ||
		(b >= 'A' && b <= 'Z') ||
		(b >= '0' && b <= '9')
}

// exampleName extracts the optional name from a "#### Example (name)" header.
func exampleName(line string) string {
	rest := strings.TrimSpace(strings.TrimPrefix(line, "#### Example"))
	rest = strings.TrimPrefix(rest, "(")
	rest = strings.TrimSuffix(rest, ")")
	return strings.TrimSpace(rest)
}

// kindFromSection maps a "## Section" title to a symbol kind.
func kindFromSection(section string) string {
	switch section {
	case "Functions":
		return KindFunction
	case "Types":
		return KindType
	case "Variables":
		return KindVariable
	case "Constants":
		return KindConstant
	}
	return ""
}
