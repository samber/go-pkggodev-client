// Package majors discovers the major versions of a Go module from the module
// proxy.
//
// In Go, majors beyond v1 live as separate modules (path, path/v2, path/v3...)
// and can be non-contiguous (golang/go#76718 cites v1, v2, v4, v6). pkg.go.dev
// does not (yet) expose a MajorVersions endpoint, so the answer is derived from
// the proxy: the base path's tagged versions yield v0, v1 and any "+incompatible"
// majors that share it, then path/vN is probed for higher majors.
package majors

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"golang.org/x/mod/module"
	"golang.org/x/mod/semver"

	"github.com/samber/go-pkggodev-client/internal/proxy"
)

// ErrInvalidModulePath is returned when modulePath cannot be parsed.
var ErrInvalidModulePath = errors.New("majors: invalid module path")

const (
	// missLimit is how many consecutive absent majors end the module-aware /vN
	// probe. Major versions can be non-contiguous, so a single gap must not stop
	// discovery.
	missLimit = 5
	// probeCap is a hard upper bound on the major number we probe, a backstop
	// against a misbehaving proxy.
	probeCap = 300
)

// Major is one discovered major version of a module.
type Major struct {
	ModulePath string // e.g. github.com/samber/do/v2
	Major      string // e.g. "v2"
	Version    string // latest version in this major, e.g. v2.0.0
}

// pseudoVersionRE matches Go pseudo-versions (untagged commits), e.g.
// v0.0.0-20240101000000-abcdef123456. Mirrors cmd/go's definition.
var pseudoVersionRE = regexp.MustCompile(`^v[0-9]+\.(0\.0-|\d+\.\d+-([^+]*\.)?0\.)\d{14}-[A-Za-z0-9]+(\+incompatible)?$`)

func isPseudoVersion(v string) bool { return pseudoVersionRE.MatchString(v) }

// majorNum returns the numeric major of a version or major string ("v2.3.1" and
// "v2" both yield 2; build metadata such as "+incompatible" is ignored).
func majorNum(v string) int {
	n, _ := strconv.Atoi(strings.TrimPrefix(semver.Major(v), "v"))
	return n
}

// Discover returns the major versions of modulePath, sorted newest-major-first.
// When excludePseudo is set, majors whose latest version is a pseudo-version are
// dropped. The proxy is assumed to be enabled.
func Discover(ctx context.Context, p *proxy.Client, modulePath string, excludePseudo bool) ([]Major, error) {
	// Validate the caller-supplied path up front (it flows into proxy request
	// URLs): EscapePath rejects schemes, "..", and other unclean paths. gopkg.in
	// paths are only valid with their ".vN" suffix, so this validates the full
	// input rather than the stripped base.
	clean := strings.TrimSuffix(strings.TrimSpace(modulePath), "/")
	if _, err := module.EscapePath(clean); err != nil {
		return nil, fmt.Errorf("%w: %q", ErrInvalidModulePath, modulePath)
	}
	base, gopkgIn, ok := normalizeBase(clean)
	if !ok {
		return nil, fmt.Errorf("%w: %q", ErrInvalidModulePath, modulePath)
	}

	found := map[int]Major{}
	if gopkgIn {
		// gopkg.in encodes the major in the path itself (pkg.vN) with no shared
		// base path, so every major is probed independently from v1.
		if err := discoverGopkgIn(ctx, p, base, found); err != nil {
			return nil, err
		}
	} else {
		floor, err := discoverBase(ctx, p, base, found)
		if err != nil {
			return nil, err
		}
		if err := discoverModuleAware(ctx, p, base, floor, found); err != nil {
			return nil, err
		}
	}

	items := make([]Major, 0, len(found))
	for _, m := range found {
		if excludePseudo && isPseudoVersion(m.Version) {
			continue
		}
		items = append(items, m)
	}
	sort.Slice(items, func(i, j int) bool {
		return majorNum(items[i].Major) > majorNum(items[j].Major)
	})
	return items, nil
}

// normalizeBase strips a trailing major-version suffix from modulePath and
// reports whether it uses the gopkg.in ".vN" convention. It is deterministic and
// does not depend on golang.org/x/mod's path parsing.
func normalizeBase(modulePath string) (base string, gopkgIn, ok bool) {
	p := strings.TrimSuffix(strings.TrimSpace(modulePath), "/")
	if p == "" {
		return "", false, false
	}
	if strings.HasPrefix(p, "gopkg.in/") {
		if i := strings.LastIndex(p, ".v"); i >= 0 && isAllDigits(p[i+2:]) {
			p = p[:i]
		}
		return p, true, true
	}
	if i := strings.LastIndex(p, "/v"); i >= 0 && isAllDigits(p[i+2:]) {
		p = p[:i]
	}
	return p, false, true
}

func isAllDigits(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

// discoverBase records one major per distinct major found among the base path's
// tagged versions (v0, v1 and any "+incompatible" majors all share it), each with
// its latest version. It returns the highest major seen, which is the floor below
// which absent module-aware majors do not count toward the stop.
func discoverBase(ctx context.Context, p *proxy.Client, base string, out map[int]Major) (int, error) {
	versions, ok, err := p.List(ctx, base)
	if err != nil {
		return 0, err
	}
	floor := 1
	latestByMajor := map[int]string{}
	for _, v := range versions {
		if !semver.IsValid(v) {
			continue
		}
		m := majorNum(v)
		if cur, exists := latestByMajor[m]; !exists || semver.Compare(v, cur) > 0 {
			latestByMajor[m] = v
		}
	}
	if len(latestByMajor) == 0 && ok {
		// No tagged versions: fall back to @latest, which may be a pseudo-version.
		if v, found, lerr := p.Latest(ctx, base); lerr != nil {
			return 0, lerr
		} else if found && v != "" {
			latestByMajor[majorNum(v)] = v
		}
	}
	for m, v := range latestByMajor {
		out[m] = Major{ModulePath: base, Major: semver.Major(v), Version: v}
		if m > floor {
			floor = m
		}
	}
	return floor, nil
}

// discoverModuleAware probes base/vN for N >= 2, recording each existing major.
// Absences only count toward the stop once N exceeds the base floor, so
// "+incompatible" majors (which 404 at /vN but live on the base path) and small
// gaps do not end discovery early.
func discoverModuleAware(ctx context.Context, p *proxy.Client, base string, floor int, out map[int]Major) error {
	miss := 0
	for n := 2; n <= probeCap; n++ {
		if _, exists := out[n]; exists {
			continue // already known from the base path (an +incompatible major)
		}
		path := base + "/v" + strconv.Itoa(n)
		v, ok, err := p.Latest(ctx, path)
		if err != nil {
			return err
		}
		if ok && v != "" {
			out[n] = Major{ModulePath: path, Major: "v" + strconv.Itoa(n), Version: v}
			miss = 0
			continue
		}
		if n > floor {
			if miss++; miss >= missLimit {
				break
			}
		}
	}
	return nil
}

// discoverGopkgIn probes gopkg.in-style major paths (base.vN) from v1.
func discoverGopkgIn(ctx context.Context, p *proxy.Client, base string, out map[int]Major) error {
	miss := 0
	for n := 1; n <= probeCap; n++ {
		path := base + ".v" + strconv.Itoa(n)
		v, ok, err := p.Latest(ctx, path)
		if err != nil {
			return err
		}
		if ok && v != "" {
			out[n] = Major{ModulePath: path, Major: "v" + strconv.Itoa(n), Version: v}
			miss = 0
			continue
		}
		if miss++; miss >= missLimit {
			break
		}
	}
	return nil
}
