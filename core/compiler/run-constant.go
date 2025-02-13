package compiler

import (
	"io"
	"strings"

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

func (r *runConstant) Run(ctx phpv.Context) (l *phpv.ZVal, err error) {
	switch strings.ToLower(string(r.c)) {
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

	z, ok := ctx.Global().ConstantGet(phpv.ZString(r.c))

	if !ok {
		err := ctx.Warn("Use of undefined constant %s - assumed '%s' (this will throw an Error in a future version of PHP", r.c, r.c)
		return phpv.ZString(r.c).ZVal(), err
	}
	return z.ZVal(), nil
}
