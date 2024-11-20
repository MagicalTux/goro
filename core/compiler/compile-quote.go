package compiler

import (
	"bytes"
	"errors"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/MagicalTux/goro/core/phpv"
	"github.com/MagicalTux/goro/core/tokenizer"
)

func compileQuoteConstant(i *tokenizer.Item, c compileCtx) (phpv.Runnable, error) {
	// i.Data is a string such as 'a string' (quotes included)

	if i.Data[0] != '\'' {
		return nil, errors.New("malformed string")
	}
	if i.Data[len(i.Data)-1] != '\'' {
		return nil, errors.New("malformed string")
	}

	in := i.Data[1 : len(i.Data)-1]
	b := &bytes.Buffer{}
	l := len(in)
	loc := i.Loc()

	for i := 0; i < l; i++ {
		c := in[i]
		if c != '\\' {
			b.WriteByte(c)
			continue
		}
		i += 1
		if i >= l {
			b.WriteByte(c)
			break
		}
		c = in[i]
		switch c {
		case '\\', '\'':
			b.WriteByte(c)
		default:
			b.WriteByte('\\')
			b.WriteByte(c)
		}
	}

	return &runZVal{phpv.ZString(b.String()), loc}, nil
}

func compileQuoteHeredoc(i *tokenizer.Item, c compileCtx) (phpv.Runnable, error) {
	// i == T_START_HEREDOC
	var res runConcat
	var err error

	for {
		i, err = c.NextItem()
		if err != nil {
			return nil, err
		}

		_ = res
		switch i.Type {
		case tokenizer.T_ENCAPSED_AND_WHITESPACE:
			res = append(res, &runZVal{unescapePhpQuotedString(i.Data), i.Loc()})
		case tokenizer.T_VARIABLE:
			res = append(res, &runVariable{phpv.ZString(i.Data[1:]), i.Loc()})
		case tokenizer.T_END_HEREDOC:
			// end of quote
			return res, nil
		default:
			return nil, i.Unexpected()
		}
	}
}

func compileQuoteEncapsed(i *tokenizer.Item, c compileCtx, q rune) (phpv.Runnable, error) {
	// i == '"'

	var res runConcat
	var err error

	for {
		i, err = c.NextItem()
		if err != nil {
			return nil, err
		}

		switch i.Type {
		case tokenizer.T_ENCAPSED_AND_WHITESPACE:
			res = append(res, &runZVal{unescapePhpQuotedString(i.Data), i.Loc()})
		case tokenizer.T_VARIABLE:
			var v phpv.Runnable = &runVariable{phpv.ZString(i.Data[1:]), i.Loc()}

			i, err = c.NextItem()
			if err != nil {
				return nil, err
			}

			// check if there's a [] or -> after $var
			switch i.Type {
			case tokenizer.T_OBJECT_OPERATOR:
				fallthrough
			case tokenizer.Rune('['):
				v, err = compilePostExpr(v, i, c)
				if err != nil {
					return nil, err
				}
				res = append(res, v)
			default:
				c.backup()
				res = append(res, v)
			}
		case tokenizer.Rune('{'):
			v, err := compileQuoteComplexExpr(c)
			if err != nil {
				return nil, err
			}
			res = append(res, v)

		case tokenizer.Rune('$'):
			// just add $ if it's not followed by a valid PHP label
			res = append(res, &runZVal{phpv.ZString(i.Data), i.Loc()})
		case tokenizer.Rune(q):
			// end of quote
			return res, nil
		default:
			return nil, i.Unexpected()
		}
	}
}

func compileQuoteComplexExpr(c compileCtx) (phpv.Runnable, error) {
	// currently at {
	i, err := c.NextItem()
	if err != nil {
		return nil, err
	}
	if i.Type != tokenizer.T_VARIABLE {
		return nil, i.Unexpected()
	}

	// similar to compileExpr, except this should start with a variable
	var v phpv.Runnable = &runVariable{phpv.ZString(i.Data[1:]), i.Loc()}
	for {
		sr, err := compilePostExpr(v, nil, c)
		if err != nil {
			return nil, err
		}
		if sr == nil {
			break
		}
		v = sr
	}

	i, err = c.NextItem()
	if err != nil {
		return nil, err
	}

	// currently at }
	return v, err
}

func unescapePhpQuotedString(in string) phpv.ZString {
	t := &bytes.Buffer{}

	for len(in) > 0 {
		if in[0] != '\\' {
			t.WriteByte(in[0])
			in = in[1:]
			continue
		}
		if len(in) == 1 {
			// end of string
			t.WriteByte(in[0])
			break
		}
		in = in[1:]

		switch in[0] {
		case 't':
			t.WriteByte('\t')
		case 'n':
			t.WriteByte('\n')
		case 'v':
			t.WriteByte('\v')
		case 'f':
			t.WriteByte('\f')
		case 'r':
			t.WriteByte('\r')
		case '"', '\\':
			t.WriteByte(in[0])
		case '0', '1', '2', '3', '4', '5', '6', '7':
			t.WriteByte(in[0] - '0')
		case 'x':
			if len(in) < 3 {
				t.WriteByte('\\')
				t.WriteByte(in[0])
				break
			}
			i, err := strconv.ParseUint(in[1:3], 16, 8)
			if err != nil {
				t.WriteByte('\\')
				t.WriteByte(in[0])
				break
			}
			t.WriteByte(byte(i))
			in = in[2:]
		case 'u':
			if len(in) < 3 || in[1] != '{' {
				// too short
				t.WriteByte('\\')
				t.WriteByte(in[0])
				break
			}
			pos := strings.IndexByte(in, '}')
			if pos == -1 {
				t.WriteByte('\\')
				t.WriteByte(in[0])
				break
			}
			i, err := strconv.ParseUint(in[2:pos], 16, 64)
			if err != nil {
				t.WriteByte('\\')
				t.WriteByte(in[0])
				break
			}
			if i >= surrogateMin && i <= surrogateMax {
				// Surrogate pairs are non-well-formed UTF-8 - however, it is sometimes useful
				// to be able to produce these (e.g. CESU-8 handling)
				t.Write(utf8EncodeRune(rune(i)))
			} else {
				t.WriteRune(rune(i))
			}
			in = in[pos:]
		default:
			t.WriteByte('\\')
			t.WriteByte(in[0])
		}
		in = in[1:]
	}

	return phpv.ZString(t.String())
}

// code from encoding/utf8 so we can encode surrogate values (required by PHP tests)
// not really optimized as writing directly to the bytes.Buffer would be better...
const (
	t1 = 0x00 // 0000 0000
	tx = 0x80 // 1000 0000
	t2 = 0xC0 // 1100 0000
	t3 = 0xE0 // 1110 0000
	t4 = 0xF0 // 1111 0000
	t5 = 0xF8 // 1111 1000

	maskx = 0x3F // 0011 1111
	mask2 = 0x1F // 0001 1111
	mask3 = 0x0F // 0000 1111
	mask4 = 0x07 // 0000 0111

	rune1Max = 1<<7 - 1
	rune2Max = 1<<11 - 1
	rune3Max = 1<<16 - 1

	surrogateMin = 0xD800
	surrogateMax = 0xDFFF
)

func utf8EncodeRune(r rune) []byte {
	// Negative values are erroneous. Making it unsigned addresses the problem.
	switch i := uint32(r); {
	case i <= rune1Max:
		return []byte{byte(r)}
	case i <= rune2Max:
		return []byte{
			t2 | byte(r>>6),
			tx | byte(r)&maskx,
		}
	case i > utf8.MaxRune:
		r = utf8.RuneError
		fallthrough
	case i <= rune3Max:
		return []byte{
			t3 | byte(r>>12),
			tx | byte(r>>6)&maskx,
			tx | byte(r)&maskx,
		}
	default:
		return []byte{
			t4 | byte(r>>18),
			tx | byte(r>>12)&maskx,
			tx | byte(r>>6)&maskx,
			tx | byte(r)&maskx,
		}
	}
}
