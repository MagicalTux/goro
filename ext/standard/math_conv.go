package standard

import (
	"bytes"
	"fmt"
	"strconv"
	"unicode"

	"github.com/MagicalTux/goro/core"
	"github.com/MagicalTux/goro/core/phpv"
)

// > func string decbin ( float $number )
func mathDecBin(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var num phpv.ZInt
	_, err := core.Expand(ctx, args, &num)
	if err != nil {
		return nil, ctx.FuncError(err)
	}

	return phpv.ZStr(fmt.Sprintf("%b", uint64(num))), nil
}

// > func string dechex ( float $number )
func mathDecHex(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var num phpv.ZInt
	_, err := core.Expand(ctx, args, &num)
	if err != nil {
		return nil, ctx.FuncError(err)
	}

	return phpv.ZStr(fmt.Sprintf("%x", uint64(num))), nil
}

// > func string decoct ( float $number )
func mathDecOct(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var num phpv.ZInt
	_, err := core.Expand(ctx, args, &num)
	if err != nil {
		return nil, ctx.FuncError(err)
	}

	return phpv.ZStr(fmt.Sprintf("%o", uint64(num))), nil
}

// > func number bindec ( string $hex_string )
func mathBinDec(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var num phpv.ZString
	_, err := core.Expand(ctx, args, &num)
	if err != nil {
		return nil, ctx.FuncError(err)
	}

	if len(num) == 0 {
		return phpv.ZInt(0).ZVal(), nil
	}

	var buf bytes.Buffer

	// ignore all invalid characters
	for _, c := range num {
		switch c {
		case '0', '1':
			buf.WriteRune(c)
		}
	}

	s := buf.String()
	if s == "" {
		return phpv.ZInt(0).ZVal(), nil
	}

	return ParseInt(s, 2, 64)
}

// > func number octdec ( string $oct_string )
func mathOctDec(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var num phpv.ZString
	_, err := core.Expand(ctx, args, &num)
	if err != nil {
		return nil, ctx.FuncError(err)
	}

	if len(num) == 0 {
		return phpv.ZInt(0).ZVal(), nil
	}

	var buf bytes.Buffer

	// ignore all invalid characters
	for _, c := range num {
		switch c {
		case '0', '1', '2', '3', '4', '5', '6', '7':
			buf.WriteRune(c)
		}
	}

	return ParseInt(buf.String(), 8, 64)
}

// > func number hexdec ( string $hex_string )
func mathHexDec(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var num phpv.ZString
	_, err := core.Expand(ctx, args, &num)
	if err != nil {
		return nil, ctx.FuncError(err)
	}

	if len(num) == 0 {
		return phpv.ZInt(0).ZVal(), nil
	}

	var buf bytes.Buffer

	// ignore all invalid characters
	for _, c := range num {
		switch c {
		case 'a', 'b', 'c', 'd', 'e', 'f',
			'A', 'B', 'C', 'D', 'E', 'F',
			'0', '1', '2', '3', '4', '5', '6', '7', '8', '9':
			buf.WriteRune(c)
		}
	}

	s := buf.String()
	if s == "" {
		return phpv.ZInt(0).ZVal(), nil
	}

	return ParseInt(s, 16, 64)
}

// > func number base_convert ( string $number , int $frombase , int $tobase )
func mathBaseConvert(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var num phpv.ZString
	var fromBase, toBase phpv.ZInt
	_, err := core.Expand(ctx, args, &num, &fromBase, &toBase)
	if err != nil {
		return nil, ctx.FuncError(err)
	}

	const allowedConvChars = "0123456789abcdefghijklmnopqrstuvwxyz"
	validChars := map[rune]struct{}{}
	for _, c := range allowedConvChars[:fromBase] {
		validChars[c] = struct{}{}
	}

	var buf bytes.Buffer
	// ignore all invalid characters
	for _, c := range num {
		c = unicode.ToLower(c)
		if _, ok := validChars[c]; ok {
			buf.WriteRune(c)
		}
	}

	if buf.Len() == 0 {
		return phpv.ZInt(0).ZVal(), nil
	}

	n, err := strconv.ParseInt(buf.String(), int(fromBase), 64)
	if err != nil {
		return nil, ctx.FuncError(err)
	}

	return phpv.ZStr(strconv.FormatInt(n, int(toBase))), nil
}
