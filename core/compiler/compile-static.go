package compiler

import (
	"fmt"
	"io"
	"sync"

	"github.com/MagicalTux/goro/core/phpv"
	"github.com/MagicalTux/goro/core/tokenizer"
)

type staticVarInfo struct {
	varName    phpv.ZString
	def        phpv.Runnable
	z          *phpv.ZVal
	perClass   map[phpv.ZString]*phpv.ZVal // per-class storage for trait method isolation
	perClosure sync.Map                    // per-closure-instance storage: uintptr -> *phpv.ZVal
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
	// Check if we're running inside a specific closure instance.
	// If so, use per-closure static variable storage so that different
	// closure instances have independent static variables.
	var closureKey uintptr
	if cvkp, ok := ctx.(phpv.ClosureStaticVarKeyProvider); ok {
		closureKey = cvkp.ClosureStaticVarKey()
	}

	for _, v := range r.vars {
		// Use per-closure storage when inside a closure instance (for closure isolation)
		if closureKey != 0 {
			existing, loaded := v.perClosure.Load(closureKey)
			var z *phpv.ZVal
			if loaded {
				z = existing.(*phpv.ZVal)
			} else {
				if v.def == nil {
					z = phpv.ZNull{}.ZVal()
				} else {
					var err error
					z, err = v.def.Run(ctx)
					if err != nil {
						return nil, err
					}
				}
				v.perClosure.Store(closureKey, z)
			}
			ctx.OffsetUnset(ctx, v.varName.ZVal())
			ctx.OffsetSet(ctx, v.varName.ZVal(), z)
			continue
		}

		// Use per-class storage when inside a class method (for trait isolation)
		var classKey phpv.ZString
		if cls := ctx.Class(); cls != nil {
			classKey = cls.GetName()
		}

		var z *phpv.ZVal
		if classKey != "" {
			if v.perClass == nil {
				v.perClass = make(map[phpv.ZString]*phpv.ZVal)
			}
			if existing, ok := v.perClass[classKey]; ok {
				z = existing
			} else {
				if v.def == nil {
					z = phpv.ZNull{}.ZVal()
				} else {
					var err error
					z, err = v.def.Run(ctx)
					if err != nil {
						return nil, err
					}
				}
				v.perClass[classKey] = z
			}
		} else {
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
			z = v.z
		}

		ctx.OffsetUnset(ctx, v.varName.ZVal())
		ctx.OffsetSet(ctx, v.varName.ZVal(), z)
	}
	return nil, nil
}

func compileStaticVar(i *tokenizer.Item, c compileCtx) (phpv.Runnable, error) {
	r := &runStaticVar{l: i.Loc()}

	// static $var [= value] [, $var [= value]] ...
	// static followed by T_PAAMAYIM_NEKUDOTAYIM means a static call (compiling is handled separately)
	// static followed by T_FUNCTION is a static closure (static function() { ... })

	for {
		i, err := c.NextItem()
		if err != nil {
			return nil, err
		}

		// Handle "static function() { ... }" or "static fn() => ..." as a static closure
		if i.Type == tokenizer.T_FUNCTION {
			// Compile the closure then mark it static
			r, err := compileFunction(i, c)
			if err != nil {
				return nil, err
			}
			if zc, ok := r.(*ZClosure); ok {
				zc.isStatic = true
			}
			return r, nil
		}
		if i.Type == tokenizer.T_FN {
			// static fn() => expr - arrow function static closure
			r, err := compileArrowFunction(i, c)
			if err != nil {
				return nil, err
			}
			if zc, ok := r.(*ZClosure); ok {
				zc.isStatic = true
			}
			return r, nil
		}

		// Handle "static::" as a late static binding expression (e.g. static::$foo = ..., static::method())
		if i.Type == tokenizer.T_PAAMAYIM_NEKUDOTAYIM {
			c.backup() // back up the :: token
			// Build the "static" value, then parse as a full expression via compileExpr
			staticItem := &tokenizer.Item{Type: tokenizer.T_STATIC, Data: "static", Filename: r.l.Filename, Line: r.l.Line}
			return compileExpr(staticItem, c)
		}

		if i.Type != tokenizer.T_VARIABLE {
			return nil, i.Unexpected()
		}
		if i.Data[1:] == "this" {
			return nil, &phpv.PhpError{
				Err:  fmt.Errorf("Cannot use $this as static variable"),
				Code: phpv.E_COMPILE_ERROR,
				Loc:  i.Loc(),
			}
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
