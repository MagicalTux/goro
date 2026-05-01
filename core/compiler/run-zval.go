package compiler

import (
	"fmt"
	"io"
	"math"
	"strconv"
	"strings"

	"github.com/KarpelesLab/goro/core/phpv"
)

type runZVal struct {
	v      phpv.Val
	l      *phpv.Loc
	cached *phpv.ZVal // pre-built cached *ZVal for scalar literals; built on first call
}

func (z *runZVal) Run(ctx phpv.Context) (*phpv.ZVal, error) {
	if z.cached != nil {
		return z.cached, nil
	}
	// Cache scalar literals (string/int/float/bool/null). Arrays and other
	// types must not be cached because their .ZVal() may dup state.
	switch z.v.(type) {
	case phpv.ZInt, phpv.ZFloat, phpv.ZString, phpv.ZBool, phpv.ZNull:
		z.cached = phpv.MakeCachedZVal(z.v)
		return z.cached, nil
	}
	return z.v.ZVal(), nil
}

func (z *runZVal) Dump(w io.Writer) error {
	switch v := z.v.(type) {
	case phpv.ZFloat:
		// PHP AST pretty printer always includes a decimal point for floats.
		// Use strconv.FormatFloat with 'G' format (shortest representation)
		// and ensure the result contains a decimal point.
		f := float64(v)
		if math.IsInf(f, 1) {
			_, err := w.Write([]byte("INF"))
			return err
		}
		if math.IsInf(f, -1) {
			_, err := w.Write([]byte("-INF"))
			return err
		}
		if math.IsNaN(f) {
			_, err := w.Write([]byte("NAN"))
			return err
		}
		s := strconv.FormatFloat(f, 'G', -1, 64)
		// Ensure decimal point for integer-valued floats (e.g., "0" -> "0.0")
		if !strings.ContainsAny(s, ".eE") {
			s += ".0"
		}
		_, err := w.Write([]byte(s))
		return err
	case phpv.ZString:
		// PHP AST uses single-quoted strings
		_, err := fmt.Fprintf(w, "'%s'", phpEscapeSingleQuote(string(v)))
		return err
	case phpv.ZBool:
		if bool(v) {
			_, err := w.Write([]byte("true"))
			return err
		}
		_, err := w.Write([]byte("false"))
		return err
	case phpv.ZInt:
		_, err := fmt.Fprintf(w, "%d", int64(v))
		return err
	default:
		_, err := fmt.Fprintf(w, "%#v", z.v)
		return err
	}
}

// phpEscapeSingleQuote escapes single quotes and backslashes for PHP single-quoted string literals.
func phpEscapeSingleQuote(s string) string {
	s = strings.ReplaceAll(s, "\\", "\\\\")
	s = strings.ReplaceAll(s, "'", "\\'")
	return s
}
