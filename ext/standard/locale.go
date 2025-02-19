package standard

// #include <locale.h>
import (
	"C"
)

import (
	"os"

	"github.com/MagicalTux/goro/core"
	"github.com/MagicalTux/goro/core/phpv"
)

// > const
const (
	LC_CTYPE phpv.ZInt = iota
	LC_NUMERIC
	LC_TIME
	LC_COLLATE
	LC_MONETARY
	LC_MESSAGES
	LC_ALL
)

// > func string setlocale ( int $category , string $locale [, string $... ] )
func fncSetLocale(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var category phpv.ZInt
	var locale phpv.ZString
	_, err := core.Expand(ctx, args, &category, &locale)
	if err != nil {
		return nil, err
	}
	var otherLocales []phpv.ZString
	for i := 2; i < len(args); i++ {
		var l phpv.ZString
		err := core.ExpandAt(ctx, args, i, &l)
		if err != nil {
			return nil, err
		}
		otherLocales = append(otherLocales, l)
	}

	if locale == "0" {
		return ctx.Global().GetLocale(category).ZVal(), nil
	}

	if locale == "" {
		var envName string
		switch category {
		case LC_CTYPE:
			envName = "LC_CTYPE"
		case LC_NUMERIC:
			envName = "LC_NUMERIC"
		case LC_TIME:
			envName = "LC_TIME"
		case LC_COLLATE:
			envName = "LC_COLLATE"
		case LC_MONETARY:
			envName = "LC_MONETARY"
		case LC_MESSAGES:
			envName = "LC_MESSAGES"
		case LC_ALL:
			envName = "LC_ALL"
		}
		locale = phpv.ZString(os.Getenv(envName))
	}

	res, ok := SetLocale(C.int(category), string(locale))
	if !ok {
		return phpv.ZFalse.ZVal(), nil
	}
	ctx.Global().SetLocale(category, locale)

	return phpv.ZStr(res), nil
}

func SetLocale(lc C.int, locale string) (string, bool) {
	param := C.CString(locale)
	ret := C.setlocale(lc, param)
	if ret == nil || *ret == 0 {
		return "", false
	}
	return C.GoString(ret), true
}
