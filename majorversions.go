package pkggodev

import (
	"context"
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"golang.org/x/mod/module"
	"golang.org/x/mod/semver"
)

const (
	// majorMissLimit is how many consecutive absent majors end the module-aware
	// /vN probe. Major versions can be non-contiguous (golang/go#76718 cites
	// v1, v2, v4, v6), so a single gap must not stop discovery.
	majorMissLimit = 5
	// majorProbeCap is a hard upper bound on the major number we probe, a
	// backstop against a misbehaving proxy.
	majorProbeCap = 300
)

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

// MajorVersions discovers the major versions of the module at modulePath.
//
// In Go, majors beyond v1 live as separate modules (path, path/v2, path/v3...),
// and they can be non-contiguous. pkg.go.dev does not (yet) expose a
// MajorVersions endpoint (golang/go#76718), so this derives the answer from the
// module proxy (honoring GOPROXY, see WithGoproxy): it lists the tagged versions
// of the base path — which yields v0, v1 and any "+incompatible" majors that
// share it — then probes path/vN for higher majors until majorMissLimit
// consecutive majors are absent.
//
// modulePath may already carry a major suffix (path/v2 or gopkg.in/pkg.v2); it
// is normalized to the base path first. WithExcludePseudo drops majors whose
// latest version is a pseudo-version. WithFilter applies a regular expression to
// each major's module path. WithLimit caps the number of returned majors (the
// proposal's Max), keeping the most recent ones.
//
// The module proxy has no pagination cursor, so the result is always a single
// page (NextToken is empty); Total is the count before WithLimit is applied.
func (c *Client) MajorVersions(ctx context.Context, modulePath string, opts ...Option) (*Page[MajorVersion], error) {
	p := newParams(opts)

	if !c.proxy.Enabled() {
		return nil, ErrProxyDisabled
	}

	base, _, ok := module.SplitPathVersion(strings.TrimSuffix(modulePath, "/"))
	if !ok || base == "" {
		return nil, fmt.Errorf("%w: %q", ErrInvalidModulePath, modulePath)
	}
	if _, err := module.EscapePath(base); err != nil {
		return nil, fmt.Errorf("%w: %q", ErrInvalidModulePath, modulePath)
	}

	majors := map[int]MajorVersion{}
	if strings.HasPrefix(base, "gopkg.in/") {
		// gopkg.in encodes the major in the path itself (pkg.vN) with no shared
		// base path, so every major is probed independently from v1.
		if err := c.discoverGopkgInMajors(ctx, base, majors); err != nil {
			return nil, err
		}
	} else {
		floor, err := c.discoverBaseMajors(ctx, base, majors)
		if err != nil {
			return nil, err
		}
		if err := c.discoverModuleAwareMajors(ctx, base, floor, majors); err != nil {
			return nil, err
		}
	}

	return c.buildMajorPage(p, majors)
}

// discoverBaseMajors records one major per distinct major found among the base
// path's tagged versions (v0, v1 and any "+incompatible" majors all share it),
// each with its latest version. It returns the highest major seen, which is the
// floor below which absent module-aware majors do not count toward the stop.
func (c *Client) discoverBaseMajors(ctx context.Context, base string, out map[int]MajorVersion) (int, error) {
	versions, ok, err := c.proxy.List(ctx, base)
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
		if v, found, lerr := c.proxy.Latest(ctx, base); lerr != nil {
			return 0, lerr
		} else if found && v != "" {
			latestByMajor[majorNum(v)] = v
		}
	}
	for m, v := range latestByMajor {
		out[m] = MajorVersion{ModulePath: base, Major: semver.Major(v), Version: v}
		if m > floor {
			floor = m
		}
	}
	return floor, nil
}

// discoverModuleAwareMajors probes base/vN for N >= 2, recording each existing
// major. Absences only count toward the stop once N exceeds the base floor, so
// "+incompatible" majors (which 404 at /vN but live on the base path) and small
// gaps do not end discovery early.
func (c *Client) discoverModuleAwareMajors(ctx context.Context, base string, floor int, out map[int]MajorVersion) error {
	miss := 0
	for n := 2; n <= majorProbeCap; n++ {
		if _, exists := out[n]; exists {
			continue // already known from the base path (an +incompatible major)
		}
		path := base + "/v" + strconv.Itoa(n)
		v, ok, err := c.proxy.Latest(ctx, path)
		if err != nil {
			return err
		}
		if ok && v != "" {
			out[n] = MajorVersion{ModulePath: path, Major: "v" + strconv.Itoa(n), Version: v}
			miss = 0
			continue
		}
		if n > floor {
			if miss++; miss >= majorMissLimit {
				break
			}
		}
	}
	return nil
}

// discoverGopkgInMajors probes gopkg.in-style major paths (base.vN) from v1.
func (c *Client) discoverGopkgInMajors(ctx context.Context, base string, out map[int]MajorVersion) error {
	miss := 0
	for n := 1; n <= majorProbeCap; n++ {
		path := base + ".v" + strconv.Itoa(n)
		v, ok, err := c.proxy.Latest(ctx, path)
		if err != nil {
			return err
		}
		if ok && v != "" {
			out[n] = MajorVersion{ModulePath: path, Major: "v" + strconv.Itoa(n), Version: v}
			miss = 0
			continue
		}
		if miss++; miss >= majorMissLimit {
			break
		}
	}
	return nil
}

// buildMajorPage applies WithExcludePseudo/WithFilter, sorts newest-major-first,
// flags the latest, and applies WithLimit.
func (c *Client) buildMajorPage(p params, majors map[int]MajorVersion) (*Page[MajorVersion], error) {
	var re *regexp.Regexp
	if p.filter != "" {
		var err error
		if re, err = regexp.Compile(p.filter); err != nil {
			return nil, fmt.Errorf("invalid filter: %w", err)
		}
	}

	items := make([]MajorVersion, 0, len(majors))
	for _, mv := range majors {
		if p.excludePseudo && isPseudoVersion(mv.Version) {
			continue
		}
		if re != nil && !re.MatchString(mv.ModulePath) {
			continue
		}
		items = append(items, mv)
	}

	sort.Slice(items, func(i, j int) bool {
		return majorNum(items[i].Major) > majorNum(items[j].Major)
	})
	if len(items) > 0 {
		items[0].IsLatest = true
	}

	total := len(items)
	if p.limit > 0 && len(items) > p.limit {
		items = items[:p.limit]
	}
	return &Page[MajorVersion]{Items: items, Total: total}, nil
}
