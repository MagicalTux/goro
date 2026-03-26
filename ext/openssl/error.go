package openssl

import (
	"github.com/MagicalTux/goro/core/phpv"
)

// > func string openssl_error_string ( void )
// In real PHP, this returns errors from the OpenSSL error queue.
// Since we use Go's crypto, we simply return false (no error).
func fncOpensslErrorString(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	// We don't maintain an OpenSSL error queue; always return false
	// to indicate no more errors.
	return phpv.ZBool(false).ZVal(), nil
}
