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
	return nil, r.bindAll(ctx)
}

// bindAll performs the per-call static-variable binding work shared
// by the AST runner and the VM's OP_STATIC_VAR_BIND: per-closure /
// per-class / global storage of each entry's value cell, and an
// OffsetSet to install it in the current scope.
func (r *runStaticVar) bindAll(ctx phpv.Context) error {
	var closureKey uintptr
	if cvkp, ok := ctx.(phpv.ClosureStaticVarKeyProvider); ok {
		closureKey = cvkp.ClosureStaticVarKey()
	}

	for _, v := range r.vars {
		if closureKey != 0 {
			existing, loaded := v.perClosure.Load(closureKey)
			var z *phpv.ZVal
			if loaded {
				z = existing.(*phpv.ZVal)
			} else {
				var err error
				z, err = runStaticInitial(ctx, v.def)
				if err != nil {
					return err
				}
				v.perClosure.Store(closureKey, z)
			}
			ctx.OffsetUnset(ctx, v.varName)
			ctx.OffsetSet(ctx, v.varName, z)
			continue
		}

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
					return err
				}
				v.perClass[classKey] = z
			}
		} else {
			if v.z == nil {
				var err error
				v.z, err = runStaticInitial(ctx, v.def)
				if err != nil {
					return err
				}
			}
			z = v.z
		}

		ctx.OffsetUnset(ctx, v.varName)
		ctx.OffsetSet(ctx, v.varName, z)
	}
	return nil
}

// BindStaticVars runs the per-call static-variable binding pipeline
// on a *runStaticVar AST node. Used by the VM's OP_STATIC_VAR_BIND.
func BindStaticVars(ctx phpv.Context, r phpv.Runnable) error {
	return r.(*runStaticVar).bindAll(ctx)
}

// IsStaticVarDecl reports whether r is a `static $x = …;` declaration.
func IsStaticVarDecl(r phpv.Runnable) bool {
	_, ok := r.(*runStaticVar)
	return ok
}

// runStaticInitial evaluates the default expression for a static var and
// returns a ZVal we own — a fresh, non-cached ZVal. For scalar values we
// additionally wrap it as a reference so writes via the hash table
// propagate into the stored cell.
//
// Why a reference for scalars? `$n++` snapshots the LHS value into a pooled
// temp, mutates the temp, and then writes back via WriteValue (which calls
// OffsetSet). OffsetSet mutates a hash-table entry in place only when the
// entry is already a reference — otherwise it replaces the pointer, which
// would disconnect our stored static value from the new hash-entry value
// and reset the initializer every call.
//
// For ZObject values we skip MakeRef: objects are always shared by handle
// in PHP, so `static $o = new Foo()` is already observed correctly without
// a reference wrapper, and wrapping adds a layer that confuses code which
// reads the ZVal directly (var_dump's object ID sequence, Nude-on-read
// helpers, etc.).
func runStaticInitial(ctx phpv.Context, def phpv.Runnable) (*phpv.ZVal, error) {
	var z *phpv.ZVal
	if def == nil {
		z = phpv.ZNull{}.ZVal()
	} else {
		v, err := def.Run(ctx)
		if err != nil {
			return nil, err
		}
		if v == nil {
			z = phpv.ZNull{}.ZVal()
		} else {
			// Dup returns a fresh, non-cached ZVal so MakeRef below is safe.
			z = v.Dup()
		}
	}
	if z.GetType() != phpv.ZtObject {
		z.MakeRef()
	}
	return z, nil
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
