package pcre

import (
	"strings"

	"github.com/MagicalTux/goro/core"
	"github.com/MagicalTux/goro/core/phpv"
)

// > func string preg_quote ( string $str [, string $delimiter = NULL ] )
func pregQuote(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	// this version won't accept UTF-8 characters as delimiter. If this is an issue, replace loop below to use string()
	var str phpv.ZString
	var delimiter *phpv.ZString
	_, err := core.Expand(ctx, args, &str, &delimiter)
	if err != nil {
		return nil, err
	}

	toEscape := ".\\+*?[^]$(){}=!<>|:-" // according to http://php.net/manual/en/function.preg-quote.php
	if delimiter != nil {
		toEscape += string(*delimiter)
	}

	var target []byte

	for p, c := range []byte(str) {
		if strings.IndexByte(toEscape, c) == -1 {
			if target != nil {
				target = append(target, c)
			}
			continue
		}
		// need to escape this
		if target == nil && p > 0 {
			// need to create initial target
			target = make([]byte, p)
			copy(target, []byte(str))
		}
		target = append(target, c)
	}

	if target == nil {
		// no change
		return str.ZVal(), nil
	} else {
		return phpv.ZString(target).ZVal(), nil
	}
}
