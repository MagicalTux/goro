package compiler

import (
	"fmt"
	"io"

	"github.com/MagicalTux/goro/core/logopt"
	"github.com/MagicalTux/goro/core/phpobj"
	"github.com/MagicalTux/goro/core/phpv"
	"github.com/MagicalTux/goro/core/tokenizer"
)

type zList struct {
	elems *phpv.ZArray
}

func (zl *zList) GetType() phpv.ZType { return phpv.ZtArray }
func (zl *zList) ZVal() *phpv.ZVal    { return phpv.NewZVal(zl) }
func (zl *zList) Value() phpv.Val     { return zl }
func (zl *zList) AsVal(ctx phpv.Context, t phpv.ZType) (phpv.Val, error) {
	switch t {
	case phpv.ZtArray:
		return zl, nil
	default:
		return zl.elems.AsVal(ctx, t)
	}
}

func (zl *zList) String() string {
	return "list(...)"
}

func (zl *zList) WriteValue(ctx phpv.Context, value *phpv.ZVal) error {
	if value.GetType() != phpv.ZtArray {
		return nil
	}
	array := value.AsArray(ctx)

	i := -1
	for _, v := range zl.elems.Iterate(ctx) {
		i++
		if v == nil {
			continue
		}
		val, _ := array.OffsetGet(ctx, phpv.ZInt(i))
		if subList, ok := v.Value().(*zList); ok {
			err := subList.WriteValue(ctx, val)
			if err != nil {
				return err
			}
			continue
		}

		if v.GetName() == "" {
			return ctx.Errorf("Assignments can only happen to writable values")
		}

		err := ctx.OffsetSet(ctx, v.GetName(), val.Dup())
		if err != nil {
			return err
		}
	}

	return nil
}

type destructureEntry struct {
	k, v phpv.Runnable
}

type runDestructure struct {
	e []*destructureEntry
	l *phpv.Loc
}

func (rd *runDestructure) Run(ctx phpv.Context) (*phpv.ZVal, error) {
	list := &zList{phpv.NewZArray()}

	var err error
	for _, e := range rd.e {
		var k, v *phpv.ZVal

		if e.k != nil {
			k, err = e.k.Run(ctx)
			if err != nil {
				return nil, err
			}
		}

		if e.v != nil {
			v, err = e.v.Run(ctx)
			if err != nil {
				return nil, err
			}
		}

		list.elems.OffsetSet(ctx, k, v.ZVal())
	}

	if list.elems.Count(ctx) == 0 {
		return nil, ctx.Errorf("Cannot use empty list")
	}

	return list.ZVal(), nil
}

func (a *runDestructure) Dump(w io.Writer) error {
	_, err := w.Write([]byte("list()"))
	if err != nil {
		return err
	}
	for _, s := range a.e {
		if s.k != nil {
			err = s.k.Dump(w)
			if err != nil {
				return err
			}
			_, err = w.Write([]byte("=>"))
			if err != nil {
				return err
			}
		}
		err = s.v.Dump(w)
		if err != nil {
			return err
		}
	}
	_, err = w.Write([]byte{')'})
	return err
}

func (a *runDestructure) WriteValue(ctx phpv.Context, value *phpv.ZVal) error {
	if value == nil || value.GetType() == phpv.ZtNull {
		// list() assignment from null: silently assign null to all elements (PHP 8 behavior)
		for _, e := range a.e {
			if e.v == nil {
				continue
			}
			if sub, ok := e.v.(*runDestructure); ok {
				sub.WriteValue(ctx, phpv.ZNULL.ZVal())
				continue
			}
			if w, ok := e.v.(phpv.Writable); ok {
				w.WriteValue(ctx, phpv.ZNULL.ZVal())
			}
		}
		return nil
	}
	if value.GetType() == phpv.ZtObject {
		// Object type that doesn't implement ArrayAccess: throw Error
		obj := value.Value().(phpv.ZObject)
		return phpobj.ThrowError(ctx, phpobj.Error, fmt.Sprintf("Cannot use object of type %s as array", obj.GetClass().GetName()))
	}
	if value.GetType() != phpv.ZtArray {
		// list() assignment from non-array: warn for each element
		typeName := value.GetType().TypeName()
		for _, e := range a.e {
			if e.v == nil {
				continue
			}
			if sub, ok := e.v.(*runDestructure); ok {
				sub.WriteValue(ctx, value)
				continue
			}
			ctx.Warn("Cannot use %s as array", typeName, logopt.NoFuncName(true))
			if w, ok := e.v.(phpv.Writable); ok {
				w.WriteValue(ctx, phpv.ZNULL.ZVal())
			}
		}
		return nil
	}
	array := value.AsArray(ctx)

	for idx, e := range a.e {
		if e.v == nil {
			continue // skipped slot
		}

		// Determine the array key for this entry
		var key phpv.Val
		if e.k != nil {
			k, err := e.k.Run(ctx)
			if err != nil {
				return err
			}
			key = k
		} else {
			key = phpv.ZInt(idx)
		}

		// Get the value from the source array (with warnings for undefined keys)
		val, _ := array.OffsetGetWarn(ctx, key)

		// Use Writable interface (works for variables, array access,
		// object properties, and nested destructures)
		if w, ok := e.v.(phpv.Writable); ok {
			if err := w.WriteValue(ctx, val.Dup()); err != nil {
				return err
			}
			continue
		}

		// Fallback: evaluate the expression and use its name
		v, err := e.v.Run(ctx)
		if err != nil {
			return err
		}
		name := v.GetName()
		if name == "" {
			return ctx.Errorf("Assignments can only happen to writable values")
		}
		if err := ctx.OffsetSet(ctx, name, val.Dup()); err != nil {
			return err
		}
	}

	return nil
}

func compileBaseDestructure(i *tokenizer.Item, c compileCtx) (phpv.Runnable, error) {
	lhs, err := compileDestructure(nil, c)
	if err != nil {
		return nil, err
	}

	i, err = c.NextItem()
	if err != nil {
		return nil, err
	}

	if i.Type != tokenizer.Rune('=') {
		return nil, i.Unexpected()
	}

	rhs, err := compileOpExpr(nil, c)
	if err != nil {
		return nil, err
	}

	return spawnOperator(c, i.Type, lhs, rhs, i.Loc())
}

func compileDestructure(i *tokenizer.Item, c compileCtx) (phpv.Runnable, error) {
	var err error
	if i == nil {
		i, err = c.NextItem()
		if err != nil {
			return nil, err
		}
	}

	// Determine closing delimiter: ')' for list(), ']' for short syntax []
	var closingRune rune
	if i.IsSingle('(') {
		closingRune = ')'
	} else if i.IsSingle('[') {
		closingRune = ']'
	} else {
		return nil, i.Unexpected()
	}

	res := &runDestructure{l: i.Loc()}

	for {
		i, err := c.NextItem()
		if err != nil {
			return nil, err
		}

		if i.IsSingle(closingRune) {
			break
		}

		if i.IsSingle(',') {
			// empty slot is allowed: list($x,) or [$x,]
			res.e = append(res.e, &destructureEntry{v: nil})
			continue
		}

		isList := false
		var k phpv.Runnable
		if i.Type == tokenizer.T_LIST {
			isList = true
			k, err = compileDestructure(nil, c)
		} else if i.IsSingle('[') {
			// Nested short list syntax: [[$a, $b], $c]
			isList = true
			k, err = compileDestructure(i, c)
		} else {
			k, err = compileExpr(i, c)
		}
		if err != nil {
			return nil, err
		}

		i, err = c.NextItem()
		if err != nil {
			return nil, err
		}

		if i.IsSingle(',') {
			res.e = append(res.e, &destructureEntry{v: k})
			continue
		}

		if i.IsSingle(closingRune) {
			res.e = append(res.e, &destructureEntry{v: k})
			break
		}

		doubleArrow := i.Type == tokenizer.T_DOUBLE_ARROW
		// list() cannot be used as a key: list(list() => $x) // invalid
		if (isList && doubleArrow) || (!isList && !doubleArrow) {
			return nil, i.Unexpected()
		}

		i, err = c.NextItem()
		if err != nil {
			return nil, err
		}

		var v phpv.Runnable
		if i.Type == tokenizer.T_LIST {
			v, err = compileDestructure(nil, c)
		} else if i.IsSingle('[') {
			v, err = compileDestructure(i, c)
		} else {
			v, err = compileExpr(i, c)
		}
		if err != nil {
			return nil, err
		}

		res.e = append(res.e, &destructureEntry{k: k, v: v})

		i, err = c.NextItem()
		if err != nil {
			return nil, err
		}

		if i.IsSingle(',') {
			continue
		}

		if i.IsSingle(')') {
			break
		}
		return nil, i.Unexpected()
	}

	// Check for empty list: all entries have nil v (all commas, no actual targets)
	hasNonNil := false
	for _, e := range res.e {
		if e.v != nil {
			hasNonNil = true
			break
		}
	}
	if !hasNonNil {
		return nil, &phpv.PhpError{
			Err:  fmt.Errorf("Cannot use empty list"),
			Code: phpv.E_COMPILE_ERROR,
			Loc:  res.l,
		}
	}

	// Validate keyed destructuring rules:
	// 1. Cannot mix keyed and unkeyed entries
	// 2. Cannot have empty (nil) entries in keyed destructuring
	hasKeyed := false
	hasUnkeyed := false
	for _, e := range res.e {
		if e.k != nil {
			hasKeyed = true
		} else if e.v != nil {
			hasUnkeyed = true
		}
	}
	if hasKeyed && hasUnkeyed {
		return nil, &phpv.PhpError{
			Err:  fmt.Errorf("Cannot mix keyed and unkeyed array entries in assignments"),
			Code: phpv.E_COMPILE_ERROR,
			Loc:  res.l,
		}
	}
	if hasKeyed {
		// Check for empty entries in keyed destructuring
		for _, e := range res.e {
			if e.v == nil && e.k == nil {
				return nil, &phpv.PhpError{
					Err:  fmt.Errorf("Cannot use empty array entries in keyed array assignment"),
					Code: phpv.E_COMPILE_ERROR,
					Loc:  res.l,
				}
			}
		}
	}

	return res, nil
}

// arrayToDestructure converts a runArray (array literal) on the LHS of
// an assignment into a runDestructure for PHP 7.1+ short list syntax:
// [$a, $b] = expr
func arrayToDestructure(arr *runArray) *runDestructure {
	rd := &runDestructure{l: arr.l}
	for _, entry := range arr.e {
		if entry.spread {
			return nil // spread in destructure not supported this way
		}
		de := &destructureEntry{}
		if entry.k != nil {
			de.k = entry.k
		}
		if entry.v != nil {
			// Check if the value is itself an array (nested destructure)
			if innerArr, ok := entry.v.(*runArray); ok {
				inner := arrayToDestructure(innerArr)
				if inner == nil {
					return nil
				}
				de.v = inner
			} else {
				de.v = entry.v
			}
		}
		rd.e = append(rd.e, de)
	}
	return rd
}
