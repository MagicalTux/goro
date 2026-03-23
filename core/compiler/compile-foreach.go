package compiler

import (
	"errors"
	"fmt"
	"io"
	"iter"

	"github.com/MagicalTux/goro/core/phperr"
	"github.com/MagicalTux/goro/core/phpobj"
	"github.com/MagicalTux/goro/core/phpv"
	"github.com/MagicalTux/goro/core/tokenizer"
)

type runnableForeach struct {
	src  phpv.Runnable
	code phpv.Runnable
	k, v phpv.Runnable
	ref  bool
	l    *phpv.Loc
}

func (r *runnableForeach) Run(ctx phpv.Context) (l *phpv.ZVal, err error) {
	z, err := r.src.Run(ctx)
	if err != nil {
		return nil, err
	}

	if z.GetType() != phpv.ZtArray && z.GetType() != phpv.ZtObject {
		typeName := z.GetType().TypeName()
		if z.GetType() == phpv.ZtBool {
			if z.AsBool(ctx) {
				typeName = "true"
			} else {
				typeName = "false"
			}
		}
		phpErr := r.l.Error(ctx, fmt.Errorf("foreach() argument must be of type array|object, %s given", typeName), phpv.E_WARNING)
		ctx.LogError(phpErr)
		return nil, nil
	}

	if z.GetType() == phpv.ZtArray {
		if r.ref {
			// For by-reference foreach, separate this variable's array from
			// any other variables sharing the same *ZArray (PHP COW semantics).
			// Create a new independent array and replace the variable's inner
			// value, so that copies made before the loop retain original data.
			dup := z.Dup()
			dup.HashTable().SeparateCow()
			z.Set(dup)
		} else {
			// For non-reference foreach, snapshot the array so modifications
			// during iteration don't affect the loop (PHP copy-on-write semantics).
			z = z.Dup()
		}
	}

	// For foreach by-reference on objects, check if the class provides an
	// internal array (e.g., ArrayObject, ArrayIterator). If so, use the
	// internal array directly for by-reference iteration.
	if r.ref && z.GetType() == phpv.ZtObject {
		if obj, ok := z.Value().(*phpobj.ZObject); ok {
			if internalArr := getForeachByRefArray(ctx, obj); internalArr != nil {
				z = internalArr.ZVal()
				z.HashTable().SeparateCow()
			}
		}
	}

	var it phpv.ZIterator
	if z.GetType() == phpv.ZtObject {
		obj, ok := z.Value().(*phpobj.ZObject)
		if ok {
			if obj.GetClass().Implements(phpobj.IteratorAggregate) {
				// Call getIterator() to get the Iterator object
				iterResult, err := obj.CallMethod(ctx, "getIterator")
				if err != nil {
					return nil, err
				}
				iteratorErr := func(className phpv.ZString) error {
					return phpobj.ThrowErrorAt(ctx, phpobj.Exception, fmt.Sprintf("Objects returned by %s::getIterator() must be traversable or implement interface Iterator", className), r.l)
				}
				if iterResult == nil || iterResult.GetType() != phpv.ZtObject {
					return nil, iteratorErr(obj.GetClass().GetName())
				}
				iterObj, ok := iterResult.Value().(*phpobj.ZObject)
				if !ok {
					return nil, iteratorErr(obj.GetClass().GetName())
				}
				// Recursively unwrap nested IteratorAggregates
				for iterObj.GetClass().Implements(phpobj.IteratorAggregate) && !iterObj.GetClass().Implements(phpobj.Iterator) {
					iterResult, err = iterObj.CallMethod(ctx, "getIterator")
					if err != nil {
						return nil, err
					}
					if iterResult == nil || iterResult.GetType() != phpv.ZtObject {
						return nil, iteratorErr(iterObj.GetClass().GetName())
					}
					iterObj, ok = iterResult.Value().(*phpobj.ZObject)
					if !ok {
						return nil, iteratorErr(obj.GetClass().GetName())
					}
				}
				if !iterObj.GetClass().Implements(phpobj.Iterator) {
					return nil, iteratorErr(obj.GetClass().GetName())
				}
				if r.ref {
					return nil, phpobj.ThrowError(ctx, phpobj.Error, "An iterator cannot be used with foreach by reference")
				}
				it = &phpObjectIterator{ctx: ctx, obj: iterObj, started: false, fromGetIterator: true}
			} else if obj.GetClass().Implements(phpobj.Iterator) {
				if r.ref {
					return nil, phpobj.ThrowError(ctx, phpobj.Error, "An iterator cannot be used with foreach by reference")
				}
				// Save iterator state for nested foreach support.
				// SPL classes like SplDoublyLinkedList, SplFixedArray use
				// opaque data that includes iterator position. Save it
				// before rewind so nested loops don't clobber outer state.
				type iterStateSaver interface {
					SaveIterState()
					RestoreIterState()
				}
				for _, opaque := range obj.Opaque {
					if saver, ok := opaque.(iterStateSaver); ok {
						saver.SaveIterState()
						defer saver.RestoreIterState()
						break
					}
				}
				it = &phpObjectIterator{ctx: ctx, obj: obj, started: false}
			}
		}
	}
	if it == nil {
		// For objects, use scope-aware iteration to handle property visibility
		if z.GetType() == phpv.ZtObject {
			if obj, ok := z.Value().(*phpobj.ZObject); ok {
				// Use the defining class of the current method as the scope
				scope := ctx.Class()
				it = obj.NewIteratorInScope(scope)
			}
		}
		if it == nil {
			it = z.NewIterator()
		}
	}
	if it == nil {
		return nil, nil
	}

	// Eagerly call __destruct on iterator objects created by getIterator()
	// when the foreach loop ends, since these temporary objects have no other references.
	if poi, ok := it.(*phpObjectIterator); ok && poi.fromGetIterator {
		defer func() {
			if m, hasDestructor := poi.obj.GetClass().GetMethod("__destruct"); hasDestructor {
				ctx.Global().UnregisterDestructor(poi.obj)
				ctx.CallZVal(ctx, m.Method, nil, poi.obj)
			}
		}()
	}

	// Register a cleanup for foreach-by-reference iterators. In PHP, when
	// the loop variable goes out of scope (function return), the refcount
	// on the last iterated element drops to 1 and the reference wrapper
	// is removed. We register a cleanup function on the FuncContext so
	// CleanupRef is called when the function returns.
	if r.ref {
		if cr, ok := it.(interface{ CleanupRef() }); ok {
			if fc, ok2 := ctx.Func().(interface {
				RegisterForeachRefCleanup(func())
			}); ok2 {
				fc.RegisterForeachRefCleanup(cr.CleanupRef)
			}
		}
	}

	for {
		err = ctx.Tick(ctx, r.l)
		if err != nil {
			return nil, err
		}

		if !it.Valid(ctx) {
			// Check if the iterator has a pending error (e.g. exception in rewind/valid)
			if ei, ok := it.(interface{ Err() error }); ok {
				if iterErr := ei.Err(); iterErr != nil {
					return nil, iterErr
				}
			}
			break
		}

		var v *phpv.ZVal
		if r.ref {
			if ri, ok := it.(interface {
				CurrentMakeRef(phpv.Context) (*phpv.ZVal, error)
			}); ok {
				v, err = ri.CurrentMakeRef(ctx)
			} else {
				v, err = it.Current(ctx)
			}
		} else {
			v, err = it.Current(ctx)
		}
		if err != nil {
			return nil, err
		}
		if v == nil {
			break
		}

		// PHP always calls key() on Iterator objects in foreach, even when no key variable
		if r.k != nil {
			k, err := it.Key(ctx)
			if err != nil {
				return nil, err
			}
			if w, ok := r.k.(phpv.Writable); !ok {
				return nil, errors.New("foreach key must be writable")
			} else {
				w.WriteValue(ctx, k.Dup())
			}
		} else if _, isPhpIter := it.(*phpObjectIterator); isPhpIter {
			// For PHP Iterator objects, always call key() to maintain correct method call sequence
			_, err = it.Key(ctx)
			if err != nil {
				return nil, err
			}
		}

		if w, ok := r.v.(phpv.Writable); !ok {
			return nil, errors.New("foreach value must be writable")
		} else {
			if r.ref {
				// Check if creating a reference would violate readonly constraints
				if rc, ok := r.v.(phpv.ReadonlyRefChecker); ok {
					if err := rc.CheckReadonlyRef(ctx); err != nil {
						return nil, err
					}
				}
				// v is already a reference from CurrentMakeRef
				w.WriteValue(ctx, v)
			} else {
				w.WriteValue(ctx, v.Dup())
			}
		}

		if r.code != nil {
			_, err = r.code.Run(ctx)
			if err != nil {
				// Don't wrap PhpThrow (exceptions) - they need to propagate as-is
				// for try/catch to work correctly
				if _, isThrow := err.(*phperr.PhpThrow); isThrow {
					return nil, err
				}
				e := r.l.Error(ctx, err)
				switch br := e.Err.(type) {
				case *phperr.PhpBreak:
					if br.Intv > 1 {
						br.Intv -= 1
						return nil, br
					}
					return nil, nil
				case *phperr.PhpContinue:
					if br.Intv > 1 {
						br.Intv -= 1
						return nil, br
					}
					it.Next(ctx)
					continue
				}
				return nil, e
			}
		}

		it.Next(ctx)
	}

	return nil, nil
}

// getForeachByRefArray checks if an object provides an internal array for
// foreach by-reference iteration (e.g., ArrayObject, ArrayIterator).
// It walks up the class hierarchy checking for HandleForeachByRef handlers.
func getForeachByRefArray(ctx phpv.Context, obj *phpobj.ZObject) *phpv.ZArray {
	cls := obj.GetClass()
	for cls != nil {
		if h := cls.Handlers(); h != nil && h.HandleForeachByRef != nil {
			arr, err := h.HandleForeachByRef(ctx, obj)
			if err == nil && arr != nil {
				return arr
			}
		}
		cls = cls.GetParent()
	}
	return nil
}

// phpObjectIterator wraps a PHP Iterator object to implement phpv.ZIterator
type phpObjectIterator struct {
	ctx             phpv.Context
	obj             *phpobj.ZObject
	started         bool
	valid           bool
	needsValid      bool // set after Next() to defer valid() call to Valid()
	err             error
	fromGetIterator bool // true if created from IteratorAggregate::getIterator()
}

func (it *phpObjectIterator) ensureStarted() {
	if !it.started {
		it.started = true
		it.needsValid = false
		_, it.err = it.obj.CallMethod(it.ctx, "rewind")
		if it.err == nil {
			v, err := it.obj.CallMethod(it.ctx, "valid")
			if err != nil {
				it.err = err
				it.valid = false
			} else {
				it.valid = v != nil && bool(v.AsBool(it.ctx))
			}
		} else {
			it.valid = false
		}
	}
}

func (it *phpObjectIterator) Current(ctx phpv.Context) (*phpv.ZVal, error) {
	it.ensureStarted()
	if it.err != nil {
		return nil, it.err
	}
	if !it.valid {
		return nil, nil
	}
	return it.obj.CallMethod(ctx, "current")
}

func (it *phpObjectIterator) Key(ctx phpv.Context) (*phpv.ZVal, error) {
	it.ensureStarted()
	if it.err != nil {
		return nil, it.err
	}
	if !it.valid {
		return nil, nil
	}
	return it.obj.CallMethod(ctx, "key")
}

func (it *phpObjectIterator) Next(ctx phpv.Context) (*phpv.ZVal, error) {
	it.ensureStarted()
	if it.err != nil {
		return nil, it.err
	}
	_, err := it.obj.CallMethod(ctx, "next")
	if err != nil {
		it.err = err
		it.valid = false
		return nil, err
	}
	// Mark that valid() needs to be called on the next Valid() check.
	// This ensures valid() is called at the top of the next loop iteration,
	// not immediately after next().
	it.needsValid = true
	return nil, nil
}

func (it *phpObjectIterator) Prev(ctx phpv.Context) (*phpv.ZVal, error) {
	return nil, nil // Not supported for PHP iterators
}

func (it *phpObjectIterator) Reset(ctx phpv.Context) (*phpv.ZVal, error) {
	it.started = false
	it.err = nil
	it.ensureStarted()
	return it.Current(ctx)
}

func (it *phpObjectIterator) ResetIfEnd(ctx phpv.Context) (*phpv.ZVal, error) {
	return nil, nil
}

func (it *phpObjectIterator) End(ctx phpv.Context) (*phpv.ZVal, error) {
	return nil, nil
}

func (it *phpObjectIterator) Valid(ctx phpv.Context) bool {
	it.ensureStarted()
	if it.err != nil {
		return false
	}
	if it.needsValid {
		it.needsValid = false
		v, err := it.obj.CallMethod(ctx, "valid")
		if err != nil {
			it.err = err
			it.valid = false
			return false
		}
		it.valid = v != nil && bool(v.AsBool(ctx))
	}
	return it.valid
}

func (it *phpObjectIterator) Err() error {
	return it.err
}

func (it *phpObjectIterator) Iterate(ctx phpv.Context) iter.Seq2[*phpv.ZVal, *phpv.ZVal] {
	return func(yield func(*phpv.ZVal, *phpv.ZVal) bool) {
		it.ensureStarted()
		for it.valid {
			key, _ := it.Key(ctx)
			value, _ := it.Current(ctx)
			if !yield(key, value) {
				break
			}
			it.Next(ctx)
		}
	}
}

func (r *runnableForeach) Dump(w io.Writer) error {
	_, err := w.Write([]byte("foreach("))
	if err != nil {
		return err
	}
	err = r.src.Dump(w)
	if err != nil {
		return err
	}
	_, err = w.Write([]byte(" as "))
	if err != nil {
		return err
	}
	if r.k == nil {
		_, err = fmt.Fprintf(w, "$%s) {", r.v)
	} else {
		_, err = fmt.Fprintf(w, "$%s => $%s) {", r.k, r.v)
	}
	if err != nil {
		return err
	}
	err = r.code.Dump(w)
	if err != nil {
		return err
	}
	_, err = w.Write([]byte{'}'})
	return err
}

func compileForeachExpr(i *tokenizer.Item, c compileCtx) (phpv.Runnable, error) {
	var err error
	if i == nil {
		i, err = c.NextItem()
		if err != nil {
			return nil, err
		}
	}

	var res phpv.Runnable

	// in addition to the list() and $varname,
	// foreach key/val take any LHS expression, such as:
	// - $x
	// - $x['a'][0]
	// - $obj->x
	// - $obj->x->y
	// - &$x
	// - foo()[$x]
	// The following are not parse errors, but still throws an error:
	// - foo()
	// - ""

	switch i.Type {
	case tokenizer.T_LIST:
		res, err = compileDestructure(nil, c)
		if err != nil {
			return nil, err
		}
	case tokenizer.Rune('['):
		// Short list syntax: [$a, $b] = ...
		// Reuse compileDestructure by passing the '[' token
		res, err = compileDestructure(i, c)
		if err != nil {
			return nil, err
		}
	case tokenizer.T_VARIABLE:
		// store in r.k or r.v ?
		res = &runVariable{v: phpv.ZString(i.Data[1:]), l: i.Loc()}
		// Handle chained access: $var[...], $var->prop, $var->prop[...], etc.
		for {
			i2, err := c.NextItem()
			if err != nil {
				return nil, err
			}
			if i2.IsSingle('[') || i2.Type == tokenizer.T_OBJECT_OPERATOR || i2.Type == tokenizer.T_NULLSAFE_OBJECT_OPERATOR {
				res, err = compilePostExpr(res, i2, c)
				if err != nil {
					return nil, err
				}
			} else {
				c.backup()
				break
			}
		}

	default:
		return nil, i.Unexpected()
	}

	return res, nil
}

func compileForeach(i *tokenizer.Item, c compileCtx) (phpv.Runnable, error) {
	// T_FOREACH (expression T_AS T_VARIABLE [=> T_VARIABLE]) ...?
	l := i.Loc()

	// parse while expression
	i, err := c.NextItem()
	if err != nil {
		return nil, err
	}
	if !i.IsSingle('(') {
		return nil, i.Unexpected()
	}

	r := &runnableForeach{l: l}
	r.src, err = compileExpr(nil, c)
	if err != nil {
		return nil, err
	}

	// check for T_AS
	i, err = c.NextItem()
	if err != nil {
		return nil, err
	}
	if i.Type != tokenizer.T_AS {
		return nil, i.Unexpected()
	}

	// Peek to check for & (by-reference)
	i, err = c.NextItem()
	if err != nil {
		return nil, err
	}
	if i.IsSingle('&') {
		r.ref = true
	} else {
		c.backup()
	}

	r.v, err = compileForeachExpr(nil, c)
	if err != nil {
		return nil, err
	}

	// check for ) or =>
	i, err = c.NextItem()
	if err != nil {
		return nil, err
	}

	if i.Type == tokenizer.T_DOUBLE_ARROW {
		if _, ok := r.v.(*runDestructure); ok {
			// foreach($arr as list(...) => $x) is invalid
			return nil, &phpv.PhpError{
				Err:  fmt.Errorf("Cannot use list as key element"),
				Code: phpv.E_COMPILE_ERROR,
				Loc:  l,
			}
		}

		// If & was before the key (foreach($a as &$k => $v)), that's a fatal error
		if r.ref {
			phpErr := &phpv.PhpError{
				Err:  fmt.Errorf("Key element cannot be a reference"),
				Code: phpv.E_COMPILE_ERROR,
				Loc:  l,
			}
			c.Global().LogError(phpErr)
			return nil, phpv.ExitError(255)
		}

		// check for T_VARIABLE or T_LIST again
		r.k = r.v
		r.ref = false // reset ref flag, it's for the value not key

		// Peek again for & before value
		i, err = c.NextItem()
		if err != nil {
			return nil, err
		}
		if i.IsSingle('&') {
			r.ref = true
		} else {
			c.backup()
		}

		r.v, err = compileForeachExpr(nil, c)
		if err != nil {
			return nil, err
		}

		i, err = c.NextItem()
		if err != nil {
			return nil, err
		}
	}

	if !i.IsSingle(')') {
		return nil, i.Unexpected()
	}

	// Cannot re-assign $this in foreach
	if isThisVariable(r.v) {
		return nil, &phpv.PhpError{
			Err:  fmt.Errorf("Cannot re-assign $this"),
			Code: phpv.E_COMPILE_ERROR,
			Loc:  l,
		}
	}
	if isThisVariable(r.k) {
		return nil, &phpv.PhpError{
			Err:  fmt.Errorf("Cannot re-assign $this"),
			Code: phpv.E_COMPILE_ERROR,
			Loc:  l,
		}
	}

	// Cannot use nullsafe operator as foreach write target
	if containsNullSafe(r.v) {
		phpErr := &phpv.PhpError{
			Err:  fmt.Errorf("Can't use nullsafe operator in write context"),
			Code: phpv.E_COMPILE_ERROR,
			Loc:  l,
		}
		c.Global().LogError(phpErr)
		return nil, phpv.ExitError(255)
	}

	// check for ;
	i, err = c.NextItem()
	if err != nil {
		return nil, err
	}
	if i.IsSingle(';') {
		return r, nil
	}

	altForm := i.IsSingle(':')
	c.backup()

	r.code, err = compileBaseSingle(nil, c)
	if err != nil {
		return nil, err
	}

	if altForm {
		i, err = c.NextItem()
		if err != nil {
			return nil, err
		}
		if i.Type != tokenizer.T_ENDFOREACH {
			return nil, i.Unexpected()
		}
		i, err = c.NextItem()
		if err != nil {
			return nil, err
		}
		if !i.IsExpressionEnd() {
			return nil, i.Unexpected()
		}
	}

	return r, nil
}
