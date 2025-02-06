package standard

import (
	"bytes"
	"fmt"
	"strings"

	"github.com/MagicalTux/goro/core"
	"github.com/MagicalTux/goro/core/phpv"
	"github.com/MagicalTux/goro/core/tokenizer"
)

var highlightReplacer = strings.NewReplacer(
	"\n", "<br />",
	" ", "&nbsp;",
	"<", "&lt;",
	">", "&gt;",
	"&", "&amp;",
)

func colorize(s string, color string) string {
	return fmt.Sprintf(`<span style="color: %s">%s</span>`, color, s)
}
func format(s string, color string) string {
	s = highlightReplacer.Replace(s)
	return colorize(s, color)
}

// > func mixed highlight_file ( string $filename [, bool $return = FALSE ] )
// > alias show_source
func fncHighlightFile(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var filename phpv.ZString
	var returnStr core.Optional[phpv.ZString]
	_, err := core.Expand(ctx, args, &filename, &returnStr)
	if err != nil {
		return nil, ctx.FuncError(err)
	}

	file, err := ctx.Global().Open(filename, true)
	if err != nil {
		return nil, ctx.FuncError(err)
	}

	lexer := tokenizer.NewLexer(file, string(filename))

	var buf bytes.Buffer

Loop:
	for {
		t, err := lexer.NextItem()
		if err != nil {
			return nil, ctx.FuncError(err)
		}
		if t.Type == tokenizer.T_EOF {
			break
		}

		switch t.Type {
		case tokenizer.T_OPEN_TAG,
			tokenizer.T_LNUMBER,
			tokenizer.T_STRING,
			tokenizer.T_VARIABLE:
			buf.WriteString(format(t.Data, "#0000BB"))

		case tokenizer.T_RETURN,
			tokenizer.T_ECHO,
			tokenizer.T_IF,
			tokenizer.T_ELSE,
			tokenizer.T_ELSEIF,
			tokenizer.T_FOR,
			tokenizer.T_FOREACH,
			tokenizer.T_IS_EQUAL:
			buf.WriteString(format(t.Data, "#007700"))
		case tokenizer.T_FUNCTION:
			buf.WriteString(format("function", "#007700"))

		case tokenizer.T_COMMENT:
			buf.WriteString(format(t.Data, "#FF8000"))

		case tokenizer.T_WHITESPACE:
			buf.WriteString(highlightReplacer.Replace(t.Data))

		case tokenizer.Rune('"'),
			tokenizer.T_ENCAPSED_AND_WHITESPACE:
			buf.WriteString(format(t.Data, "#DD0000"))

		case tokenizer.Rune('='),
			tokenizer.Rune('('), tokenizer.Rune(')'),
			tokenizer.Rune('{'), tokenizer.Rune('}'),
			tokenizer.Rune('*'),
			tokenizer.Rune(','),
			tokenizer.Rune(';'):
			buf.WriteString(format(t.Data, "#007700"))

		default:
			println("TODO:", t.String())
			break Loop
		}
	}

	output := "<code><span style=\"color: #000000\">\n" + buf.String() + "</span>\n</code>"
	return phpv.ZStr(output), nil
}
