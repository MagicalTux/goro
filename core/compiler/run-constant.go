package compiler

import (
	"fmt"
	"io"
	"strings"

	"github.com/MagicalTux/goro/core/phpobj"
	"github.com/MagicalTux/goro/core/phpv"
)

type runConstant struct {
	c string
	l *phpv.Loc
}

func (r *runConstant) Dump(w io.Writer) error {
	_, err := w.Write([]byte(r.c))
	return err
}

// shortName returns the part after the last backslash, or the full name if no backslash.
func shortName(name string) string {
	if idx := strings.LastIndexByte(name, '\\'); idx >= 0 {
		return name[idx+1:]
	}
	return name
}

func (r *runConstant) Run(ctx phpv.Context) (l *phpv.ZVal, err error) {
	// Check special constants using the short (unqualified) name
	short := shortName(r.c)
	switch strings.ToLower(short) {
	case "null":
		return phpv.ZNull{}.ZVal(), nil
	case "true":
		return phpv.ZBool(true).ZVal(), nil
	case "false":
		return phpv.ZBool(false).ZVal(), nil
	case "self":
		return phpv.ZString("self").ZVal(), nil
	case "parent":
		return phpv.ZString("parent").ZVal(), nil
	}

	// Try the full (possibly namespaced) name first
	z, ok := ctx.Global().ConstantGet(phpv.ZString(r.c))
	if ok {
		return z.ZVal(), nil
	}

	// Namespace fallback: if Foo\BAR is not found, try BAR (global)
	if short != r.c {
		z, ok = ctx.Global().ConstantGet(phpv.ZString(short))
		if ok {
			return z.ZVal(), nil
		}
	}

	// PHP 8: using an undefined constant is a fatal Error
	return nil, phpobj.ThrowError(ctx, phpobj.Error, fmt.Sprintf("Undefined constant \"%s\"", r.c))
}
