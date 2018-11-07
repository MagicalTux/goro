package core

import (
	"bytes"

	"git.atonline.com/tristantech/gophp/core/tokenizer"
)

func compileQuoteEncapsed(i *tokenizer.Item, c *compileCtx) (runnable, error) {
	// i == '"'

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
			res = append(res, &ZVal{unescapePhpQuotedString(i.Data)})
		case tokenizer.T_VARIABLE:
			res = append(res, runVariable(i.Data[1:]))
		case tokenizer.ItemSingleChar:
			switch []rune(i.Data)[0] {
			case '"':
				// end of quote
				return res, nil
			}
		default:
			return nil, i.Unexpected()
		}
	}
}

func unescapePhpQuotedString(in string) ZString {
	t := &bytes.Buffer{}
	l := len(in)

	for i := 0; i < l; i++ {
		c := in[i]
		if c != '\\' {
			t.WriteByte(c)
			continue
		}
		i += 1
		if i >= l {
			t.WriteByte('\\')
			break
		}
		c = in[i]

		switch c {
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
		case '\\':
			t.WriteByte('\\')
		// TODO: handle \x##
		default:
			t.WriteByte('\\')
			t.WriteByte(c)
		}
	}

	return ZString(t.String())
}
