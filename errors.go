package pkggodev

import "errors"

// ErrSymbolNotFound is returned by Client.SymbolDoc when the requested symbol is
// absent from the package documentation.
var ErrSymbolNotFound = errors.New("pkggodev: symbol not found")
