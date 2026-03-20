package pcre

import (
	"strings"

	"github.com/MagicalTux/goro/core"
	"github.com/MagicalTux/goro/core/phpv"
)

// > func string preg_quote ( string $str [, string $delimiter = NULL ] )
func pregQuote(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var str phpv.ZString
	var delimiter *phpv.ZString
	_, err := core.Expand(ctx, args, &str, &delimiter)
	if err != nil {
		return nil, err
	}

	// Characters that preg_quote must escape (PHP manual)
	toEscape := ".\\+*?[^]$(){}=!<>|:-#\000"
	if delimiter != nil {
		d := string(*delimiter)
		if len(d) > 0 && !strings.ContainsRune(toEscape, rune(d[0])) {
			toEscape += string(d[0])
		}
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
		if target == nil {
			// need to create initial target
			target = make([]byte, p)
			copy(target, []byte(str[:p]))
		}
		if c == 0 {
			target = append(target, '\\', '0', '0', '0')
		} else {
			target = append(target, '\\', c)
		}
	}

	if target == nil {
		// no change
		return str.ZVal(), nil
	}
	return phpv.ZString(target).ZVal(), nil
}
