package standard

import (
	"bytes"

	"github.com/MagicalTux/goro/core"
	"github.com/MagicalTux/goro/core/phpv"
)

// > func string quoted_printable_decode ( string $str )
func fncQuotedPrintableDecode(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var str phpv.ZString
	_, err := core.Expand(ctx, args, &str)
	if err != nil {
		return nil, ctx.FuncError(err)
	}

	runes := []rune(str)
	var buf bytes.Buffer
	for i := 0; i < len(runes); i++ {
		if str[i] != '=' {
			buf.WriteRune(runes[i])
			continue
		}

		c1 := core.Idx(runes, i+1)
		c2 := core.Idx(runes, i+2)
		if isHex(c1) && isHex(c2) {
			buf.WriteByte(unhex(byte(c1))<<4 | unhex(byte(c2)))
			i += 2
			continue
		}

	SkipEqWhitespace:
		for j := i + 1; j < len(runes); j++ {
			switch c := runes[j]; c {
			case ' ', '\t':
			default:
				switch c {
				case '\n', '\r':
					i = j
				default:
					buf.WriteRune('=')
				}
				break SkipEqWhitespace
			}
		}
	}

	return phpv.ZStr(buf.String()), nil
}
