package standard

/*
#cgo CFLAGS: -D_GNU_SOURCE
#cgo LDFLAGS: -lcrypt
#include <crypt.h>
#include <stdlib.h>
#include <string.h>

static char* do_crypt(const char *key, const char *salt) {
    struct crypt_data data;
    memset(&data, 0, sizeof(data));
    return crypt_r(key, salt, &data);
}
*/
import "C"
import (
	"strings"
	"unsafe"

	"github.com/MagicalTux/goro/core"
	"github.com/MagicalTux/goro/core/phpv"
)

// cryptDES performs DES-based crypt using the system library
func cryptDES(password, salt string) string {
	cKey := C.CString(password)
	defer C.free(unsafe.Pointer(cKey))
	cSalt := C.CString(salt)
	defer C.free(unsafe.Pointer(cSalt))

	result := C.do_crypt(cKey, cSalt)
	if result == nil {
		return "*0"
	}
	return C.GoString(result)
}

// > func string|false crypt ( string $string , string $salt )
func fncCryptImpl(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var str phpv.ZString
	var salt phpv.ZString
	_, err := core.Expand(ctx, args, &str, &salt)
	if err != nil {
		return nil, err
	}

	saltStr := string(salt)
	strStr := string(str)

	// Validate salt is not empty
	if len(saltStr) == 0 {
		return phpv.ZStr("*0").ZVal(), nil
	}

	// Use system crypt
	cKey := C.CString(strStr)
	defer C.free(unsafe.Pointer(cKey))
	cSalt := C.CString(saltStr)
	defer C.free(unsafe.Pointer(cSalt))

	result := C.do_crypt(cKey, cSalt)
	if result == nil {
		return phpv.ZStr("*0").ZVal(), nil
	}

	resultStr := C.GoString(result)
	// PHP returns "*0" or "*1" on failure
	if strings.HasPrefix(resultStr, "*") {
		return phpv.ZStr("*0").ZVal(), nil
	}

	return phpv.ZStr(resultStr), nil
}
