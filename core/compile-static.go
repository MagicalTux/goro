package core

import (
	"fmt"
	"io"

	"github.com/MagicalTux/goro/core/phpv"
	"github.com/MagicalTux/goro/core/tokenizer"
)

type staticVarInfo struct {
	varName phpv.ZString
	def     phpv.Runnable
	z       *phpv.ZVal
}

type runStaticVar struct {
	vars []*staticVarInfo
	l    *phpv.Loc
}

func (r *runStaticVar) Dump(w io.Writer) error {
	_, err := w.Write([]byte("static "))
	if err != nil {
		return err
	}

	first := true
	for _, v := range r.vars {
		if !first {
			_, err = w.Write([]byte(", "))
			if err != nil {
				return err
			}
		}
		first = false

		if v.def != nil {
			_, err = fmt.Fprintf(w, "$%s = ", v.varName)
			if err != nil {
				return err
			}
			err = v.def.Dump(w)
			if err != nil {
				return err
			}
		} else {
			_, err = fmt.Fprintf(w, "$%s", v.varName)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func (r *runStaticVar) Run(ctx phpv.Context) (*phpv.ZVal, error) {
	// set vars in ctx
	for _, v := range r.vars {
		if v.z == nil {
			if v.def == nil {
				v.z = phpv.ZNull{}.ZVal()
			} else {
				var err error
				v.z, err = v.def.Run(ctx)
				if err != nil {
					return nil, err
				}
			}
		}
		ctx.OffsetUnset(ctx, v.varName.ZVal())
		ctx.OffsetSet(ctx, v.varName.ZVal(), v.z)
	}
	return nil, nil
}

func compileStaticVar(i *tokenizer.Item, c compileCtx) (phpv.Runnable, error) {
	r := &runStaticVar{l: phpv.MakeLoc(i.Loc())}

	// static $var [= value] [, $var [= value]] ...
	// static followed by T_PAAMAYIM_NEKUDOTAYIM means a static call (compiling is handled separately)

	for {
		i, err := c.NextItem()
		if err != nil {
			return nil, err
		}

		if i.Type != tokenizer.T_VARIABLE {
			return nil, i.Unexpected()
		}
		stv := &staticVarInfo{varName: phpv.ZString(i.Data[1:])}

		// parse default value, if any
		i, err = c.NextItem()
		if err != nil {
			return nil, err
		}

		if i.IsSingle('=') {
			// default value!
			r, err := compileExpr(nil, c)
			if err != nil {
				return nil, err
			}
			stv.def = r

			i, err = c.NextItem()
			if err != nil {
				return nil, err
			}
		}

		r.vars = append(r.vars, stv)

		if i.IsSingle(',') {
			// there's more!
			continue
		}

		if i.IsSingle(';') {
			c.backup()
			return r, nil
		}

		return nil, i.Unexpected()
	}
}
