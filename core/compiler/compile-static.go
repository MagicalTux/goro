package compiler

import (
	"fmt"
	"io"
	"sync"

	"github.com/KarpelesLab/goro/core/phpv"
	"github.com/KarpelesLab/goro/core/tokenizer"
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
	// PHP's AST dump emits each variable as a separate "static $x" statement.
	// We emit "static $a;\nstatic $b = 0" so DumpStatements appends the final ";\n".
	for i, v := range r.vars {
		if i > 0 {
			if _, err := w.Write([]byte(";\nstatic ")); err != nil {
				return err
			}
		} else {
			if _, err := w.Write([]byte("static ")); err != nil {
				return err
			}
		}
		var err error
		if v.def != nil {
			_, err = fmt.Fprintf(w, "$%s = ", v.varName)
			if err != nil {
				return err
			}
			err = v.def.Dump(w)
		} else {
			_, err = fmt.Fprintf(w, "$%s", v.varName)
		}
		if err != nil {
			return err
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
				var err error
				z, err = runStaticInitial(ctx, v.def)
				if err != nil {
					return nil, err
				}
				v.perClosure.Store(closureKey, z)
			}
			ctx.OffsetUnset(ctx, v.varName)
			ctx.OffsetSet(ctx, v.varName, z)
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
				var err error
				z, err = runStaticInitial(ctx, v.def)
				if err != nil {
					return nil, err
				}
				v.perClass[classKey] = z
			}
		} else {
			if v.z == nil {
				var err error
				v.z, err = runStaticInitial(ctx, v.def)
				if err != nil {
					return nil, err
				}
			}
			z = v.z
		}

		ctx.OffsetUnset(ctx, v.varName)
		ctx.OffsetSet(ctx, v.varName, z)
	}
	return nil, nil
}

// runStaticInitial evaluates the default expression for a static var and
// returns a ZVal we own — i.e. guaranteed not to be one of the cached
// shared instances (small ints, bools). Sharing a cached ZVal would break
// static variable persistence because the hash table inserts a fresh copy
// of any cached ZVal, so mutations via the hash entry (like $n++) never
// propagate back to the stored static value.
func runStaticInitial(ctx phpv.Context, def phpv.Runnable) (*phpv.ZVal, error) {
	if def == nil {
		return phpv.ZNull{}.ZVal(), nil
	}
	z, err := def.Run(ctx)
	if err != nil {
		return nil, err
	}
	if z == nil {
		return phpv.ZNull{}.ZVal(), nil
	}
	// Dup returns a fresh, non-cached ZVal wrapping the same value.
	return z.Dup(), nil
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
