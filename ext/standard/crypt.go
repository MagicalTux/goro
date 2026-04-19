// Pure-Go implementation of PHP's crypt() function. Replaces the previous
// cgo-backed wrapper around libc's crypt_r(). Each supported algorithm lives
// in its own file (crypt_bcrypt.go, crypt_md5.go, crypt_sha.go,
// crypt_des.go); this file is just the dispatcher and the PHP entry point.
package standard

import (
	"strings"

	"github.com/KarpelesLab/goro/core"
	"github.com/KarpelesLab/goro/core/phpv"
)

// cryptFailure is the sentinel string PHP returns when crypt() cannot compute
// a hash (invalid salt format, unsupported algorithm, etc.).
const cryptFailure = "*0"

// cryptDES is kept as a thin wrapper so password.go's DES verification path
// reads naturally. It returns cryptFailure when DES-style input is malformed.
func cryptDES(password, salt string) string {
	return cryptTraditionalDES(password, salt)
}

// > func string|false crypt ( string $string , string $salt )
func fncCryptImpl(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var password, salt phpv.ZString
	if _, err := core.Expand(ctx, args, &password, &salt); err != nil {
		return nil, err
	}
	return phpv.ZStr(cryptDispatch(string(password), string(salt))), nil
}

// cryptDispatch chooses an algorithm based on the salt prefix, following the
// conventions documented at crypt(5) on Linux.
func cryptDispatch(password, salt string) string {
	if salt == "" {
		return cryptFailure
	}

	// Bcrypt: $2a$, $2b$, $2x$, $2y$ (two-char prefix after the $).
	if len(salt) >= 4 && salt[0] == '$' && salt[1] == '2' && salt[3] == '$' {
		switch salt[2] {
		case 'a', 'b', 'x', 'y':
			return cryptBcrypt(password, salt)
		default:
			return cryptFailure
		}
	}

	switch {
	case strings.HasPrefix(salt, "$1$"):
		return cryptMD5(password, salt)
	case strings.HasPrefix(salt, "$5$"):
		return cryptSHA256(password, salt)
	case strings.HasPrefix(salt, "$6$"):
		return cryptSHA512(password, salt)
	case strings.HasPrefix(salt, "$"):
		// Unknown $-prefixed algorithm.
		return cryptFailure
	case strings.HasPrefix(salt, "_"):
		// Extended DES (BSDi) is not implemented.
		return cryptFailure
	}

	// Traditional DES when no recognised prefix is present.
	return cryptTraditionalDES(password, salt)
}
