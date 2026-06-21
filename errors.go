package pkggodev

import "errors"

// ErrSymbolNotFound is returned by Client.Symbol when the requested symbol is
// absent from the package documentation.
var ErrSymbolNotFound = errors.New("pkggodev: symbol not found")

// ErrInvalidModulePath is returned when a module path cannot be parsed, e.g. by
// MajorVersions.
var ErrInvalidModulePath = errors.New("pkggodev: invalid module path")

// ErrProxyDisabled is returned by MajorVersions when no usable module proxy is
// configured (GOPROXY is "off" or resolves to "direct" only).
var ErrProxyDisabled = errors.New("pkggodev: no usable module proxy (GOPROXY)")
