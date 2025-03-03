package standard

import (
	"bytes"
	"fmt"
	"io"
	"strings"

	"github.com/MagicalTux/goro/core"
	"github.com/MagicalTux/goro/core/phpv"
	"github.com/MagicalTux/goro/core/tokenizer"
)

var highlightReplacer = strings.NewReplacer(
	"\n", "<br />",
	"\t", "&nbsp;&nbsp;&nbsp;&nbsp;",
	" ", "&nbsp;",
	"<", "&lt;",
	">", "&gt;",
	"&", "&amp;",
)

func highlightString(ctx phpv.Context, r io.Reader, filename string) (string, error) {
	lexer := tokenizer.NewLexer(r, string(filename))
	var buf bytes.Buffer
	var nodeBuf bytes.Buffer

	colorString := string(ctx.GetConfig("highlight.string", phpv.ZStr("#DD0000")).AsString(ctx))
	colorKeyword := string(ctx.GetConfig("highlight.keyword", phpv.ZStr("#007700")).AsString(ctx))
	colorDefault := string(ctx.GetConfig("highlight.default", phpv.ZStr("#0000BB")).AsString(ctx))
	colorComment := string(ctx.GetConfig("highlight.comment", phpv.ZStr("#FF8000")).AsString(ctx))
	colorHTML := string(ctx.GetConfig("highlight.html", phpv.ZStr("#000000")).AsString(ctx))
	currentColor := colorHTML

	output := func(s string, color string) {
		// consecutive texts with the same color are placed in the same <span>
		s = highlightReplacer.Replace(s)
		if color != currentColor {
			out := nodeBuf.String()
			if currentColor != colorHTML {
				out = fmt.Sprintf(`<span style="color: %s">%s</span>`, currentColor, out)
			}
			currentColor = color
			buf.WriteString(out)
			nodeBuf.Reset()
		}
		nodeBuf.WriteString(s)
	}

	for {
		t, err := lexer.NextItem()
		if err != nil {
			break
		}
		if t.Type == tokenizer.T_EOF {
			break
		}

		switch t.Type {
		case tokenizer.T_OPEN_TAG,
			tokenizer.T_CLOSE_TAG,
			tokenizer.T_FILE,
			tokenizer.T_DIR,
			tokenizer.T_CLASS,
			tokenizer.T_FUNC_C,
			tokenizer.T_LNUMBER,
			tokenizer.T_DNUMBER,
			tokenizer.T_STRING,
			tokenizer.T_VARIABLE:
			output(t.Data, colorDefault)

		case tokenizer.T_FUNCTION:
			output("function", colorKeyword)

		case tokenizer.T_COMMENT:
			output(t.Data, colorComment)

		case tokenizer.T_WHITESPACE:
			output(t.Data, currentColor)

		case tokenizer.Rune('"'),
			tokenizer.T_ENCAPSED_AND_WHITESPACE,
			tokenizer.T_CONSTANT_ENCAPSED_STRING:
			output(t.Data, colorString)

		case tokenizer.T_INLINE_HTML:
			output(t.Data, colorHTML)

		default:
			output(t.Data, colorKeyword)
		}
	}
	if nodeBuf.Len() > 0 {
		out := nodeBuf.String()
		nodeBuf.Reset()
		if currentColor != colorHTML {
			out = fmt.Sprintf(`<span style="color: %s">%s</span>%s`, currentColor, out, "\n")
		}
		buf.WriteString(out)
	}

	return buf.String(), nil
}

// > func mixed highlight_string ( string $str [, bool $return = FALSE ] )
func fncHighlightString(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var str phpv.ZString
	var returnStr core.Optional[phpv.ZBool]
	_, err := core.Expand(ctx, args, &str, &returnStr)
	if err != nil {
		return nil, ctx.FuncError(err)
	}

	r := strings.NewReader(string(str))

	output, err := highlightString(ctx, r, "")
	if err != nil {
		return nil, ctx.FuncError(err)
	}

	result := "<code><span style=\"color: #000000\">\n" + output + "</span>\n</code>"
	if returnStr.GetOrDefault(phpv.ZFalse) {
		return phpv.ZStr(result), nil
	}

	ctx.Write([]byte(result))
	return nil, nil
}

// > func mixed highlight_file ( string $filename [, bool $return = FALSE ] )
// > alias show_source
func fncHighlightFile(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var filename phpv.ZString
	var returnStr core.Optional[phpv.ZBool]
	_, err := core.Expand(ctx, args, &filename, &returnStr)
	if err != nil {
		return nil, ctx.FuncError(err)
	}

	file, err := ctx.Global().Open(ctx, filename, "r", true)
	if err != nil {
		return nil, ctx.FuncError(err)
	}
	defer file.Close()

	output, err := highlightString(ctx, file, string(filename))
	if err != nil {
		return nil, ctx.FuncError(err)
	}

	result := "<code><span style=\"color: #000000\">\n" + output + "</span>\n</code>"
	if returnStr.GetOrDefault(phpv.ZFalse) {
		return phpv.ZStr(result), nil
	}

	ctx.Write([]byte(result))
	return nil, nil
}
