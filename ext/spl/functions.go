package spl

import (
	"fmt"
	"iter"

	"github.com/MagicalTux/goro/core"
	"github.com/MagicalTux/goro/core/phpobj"
	"github.com/MagicalTux/goro/core/phpv"
)

// > func string spl_object_hash ( object $object )
func splObjectHash(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var obj *phpobj.ZObject
	_, err := core.Expand(ctx, args, &obj)
	if err != nil {
		return nil, err
	}

	hash := fmt.Sprintf("%032x", obj.ID)
	return phpv.ZString(hash).ZVal(), nil
}

// > func int spl_object_id ( object $object )
func splObjectId(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var obj *phpobj.ZObject
	_, err := core.Expand(ctx, args, &obj)
	if err != nil {
		return nil, err
	}

	return phpv.ZInt(obj.ID).ZVal(), nil
}

// > func array iterator_to_array ( Traversable $iterator [, bool $preserve_keys = true ] )
func iteratorToArray(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	if len(args) < 1 {
		return nil, ctx.Errorf("iterator_to_array() expects at least 1 argument, 0 given")
	}

	z := args[0]
	preserveKeys := true
	if len(args) > 1 {
		preserveKeys = bool(args[1].AsBool(ctx))
	}

	if z.GetType() != phpv.ZtObject {
		return nil, ctx.Errorf("iterator_to_array(): Argument #1 ($iterator) must be of type Traversable, %s given", z.GetType())
	}

	obj, ok := z.Value().(*phpobj.ZObject)
	if !ok {
		return nil, ctx.Errorf("iterator_to_array(): Argument #1 ($iterator) must be of type Traversable, %s given", z.GetType())
	}

	// Get the iterator - handle IteratorAggregate, Iterator, and plain objects
	var it phpv.ZIterator
	iterObj := obj

	if obj.GetClass().Implements(phpobj.IteratorAggregate) {
		// Call getIterator() to get the Iterator object
		iterResult, err := obj.CallMethod(ctx, "getIterator")
		if err != nil {
			return nil, err
		}
		if iterResult == nil || iterResult.GetType() != phpv.ZtObject {
			return nil, ctx.Errorf("Objects returned by %s::getIterator() must be traversable or implement interface Iterator", obj.GetClass().GetName())
		}
		io, ok := iterResult.Value().(*phpobj.ZObject)
		if !ok {
			return nil, ctx.Errorf("Objects returned by %s::getIterator() must be traversable or implement interface Iterator", obj.GetClass().GetName())
		}
		// Recursively unwrap nested IteratorAggregates
		for io.GetClass().Implements(phpobj.IteratorAggregate) && !io.GetClass().Implements(phpobj.Iterator) {
			iterResult, err = io.CallMethod(ctx, "getIterator")
			if err != nil {
				return nil, err
			}
			if iterResult == nil || iterResult.GetType() != phpv.ZtObject {
				return nil, ctx.Errorf("Objects returned by %s::getIterator() must be traversable or implement interface Iterator", io.GetClass().GetName())
			}
			io, ok = iterResult.Value().(*phpobj.ZObject)
			if !ok {
				return nil, ctx.Errorf("Objects returned by %s::getIterator() must be traversable or implement interface Iterator", obj.GetClass().GetName())
			}
		}
		iterObj = io
	}

	if iterObj.GetClass().Implements(phpobj.Iterator) {
		it = &phpIteratorWrapper{ctx: ctx, obj: iterObj}
	} else {
		it = iterObj.NewIterator()
	}

	result := phpv.NewZArray()
	idx := 0

	// For PHP Iterator objects, call rewind/valid/current/key/next
	if piw, ok := it.(*phpIteratorWrapper); ok {
		_, err := piw.obj.CallMethod(ctx, "rewind")
		if err != nil {
			return nil, err
		}

		for {
			validResult, err := piw.obj.CallMethod(ctx, "valid")
			if err != nil {
				return nil, err
			}
			if !bool(validResult.AsBool(ctx)) {
				break
			}

			value, err := piw.obj.CallMethod(ctx, "current")
			if err != nil {
				return nil, err
			}

			if preserveKeys {
				key, err := piw.obj.CallMethod(ctx, "key")
				if err != nil {
					return nil, err
				}
				result.OffsetSet(ctx, key.Value(), value)
			} else {
				result.OffsetSet(ctx, phpv.ZInt(idx), value)
				idx++
			}

			_, err = piw.obj.CallMethod(ctx, "next")
			if err != nil {
				return nil, err
			}
		}
	} else {
		// Use the standard iterator interface
		it.Reset(ctx)
		for it.Valid(ctx) {
			value, err := it.Current(ctx)
			if err != nil {
				return nil, err
			}

			if preserveKeys {
				key, err := it.Key(ctx)
				if err != nil {
					return nil, err
				}
				result.OffsetSet(ctx, key.Value(), value)
			} else {
				result.OffsetSet(ctx, phpv.ZInt(idx), value)
				idx++
			}

			it.Next(ctx)
		}
	}

	return result.ZVal(), nil
}

// phpIteratorWrapper is a minimal wrapper to tag PHP Iterator objects
type phpIteratorWrapper struct {
	ctx phpv.Context
	obj *phpobj.ZObject
}

func (w *phpIteratorWrapper) Current(ctx phpv.Context) (*phpv.ZVal, error) {
	return w.obj.CallMethod(ctx, "current")
}

func (w *phpIteratorWrapper) Key(ctx phpv.Context) (*phpv.ZVal, error) {
	return w.obj.CallMethod(ctx, "key")
}

func (w *phpIteratorWrapper) Next(ctx phpv.Context) (*phpv.ZVal, error) {
	return w.obj.CallMethod(ctx, "next")
}

func (w *phpIteratorWrapper) Prev(ctx phpv.Context) (*phpv.ZVal, error) {
	return nil, nil
}

func (w *phpIteratorWrapper) Reset(ctx phpv.Context) (*phpv.ZVal, error) {
	return w.obj.CallMethod(ctx, "rewind")
}

func (w *phpIteratorWrapper) ResetIfEnd(ctx phpv.Context) (*phpv.ZVal, error) {
	return nil, nil
}

func (w *phpIteratorWrapper) End(ctx phpv.Context) (*phpv.ZVal, error) {
	return nil, nil
}

func (w *phpIteratorWrapper) Valid(ctx phpv.Context) bool {
	v, err := w.obj.CallMethod(ctx, "valid")
	if err != nil {
		return false
	}
	return bool(v.AsBool(ctx))
}

func (w *phpIteratorWrapper) Iterate(ctx phpv.Context) iter.Seq2[*phpv.ZVal, *phpv.ZVal] {
	return func(yield func(*phpv.ZVal, *phpv.ZVal) bool) {
		w.obj.CallMethod(ctx, "rewind")
		for w.Valid(ctx) {
			key, _ := w.Key(ctx)
			value, _ := w.Current(ctx)
			if !yield(key, value) {
				break
			}
			w.Next(ctx)
		}
	}
}

// > func int iterator_count ( Traversable $iterator )
func iteratorCount(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	if len(args) < 1 {
		return nil, ctx.Errorf("iterator_count() expects exactly 1 argument, 0 given")
	}

	z := args[0]

	if z.GetType() != phpv.ZtObject {
		return nil, ctx.Errorf("iterator_count(): Argument #1 ($iterator) must be of type Traversable, %s given", z.GetType())
	}

	obj, ok := z.Value().(*phpobj.ZObject)
	if !ok {
		return nil, ctx.Errorf("iterator_count(): Argument #1 ($iterator) must be of type Traversable, %s given", z.GetType())
	}

	// Get the iterator - handle IteratorAggregate
	iterObj := obj
	if obj.GetClass().Implements(phpobj.IteratorAggregate) {
		iterResult, err := obj.CallMethod(ctx, "getIterator")
		if err != nil {
			return nil, err
		}
		if iterResult == nil || iterResult.GetType() != phpv.ZtObject {
			return nil, ctx.Errorf("Objects returned by %s::getIterator() must be traversable or implement interface Iterator", obj.GetClass().GetName())
		}
		io, ok := iterResult.Value().(*phpobj.ZObject)
		if !ok {
			return nil, ctx.Errorf("Objects returned by %s::getIterator() must be traversable or implement interface Iterator", obj.GetClass().GetName())
		}
		for io.GetClass().Implements(phpobj.IteratorAggregate) && !io.GetClass().Implements(phpobj.Iterator) {
			iterResult, err = io.CallMethod(ctx, "getIterator")
			if err != nil {
				return nil, err
			}
			if iterResult == nil || iterResult.GetType() != phpv.ZtObject {
				return nil, ctx.Errorf("Objects returned by %s::getIterator() must be traversable or implement interface Iterator", io.GetClass().GetName())
			}
			io, ok = iterResult.Value().(*phpobj.ZObject)
			if !ok {
				return nil, ctx.Errorf("Objects returned by %s::getIterator() must be traversable or implement interface Iterator", obj.GetClass().GetName())
			}
		}
		iterObj = io
	}

	count := 0

	if iterObj.GetClass().Implements(phpobj.Iterator) {
		_, err := iterObj.CallMethod(ctx, "rewind")
		if err != nil {
			return nil, err
		}
		for {
			validResult, err := iterObj.CallMethod(ctx, "valid")
			if err != nil {
				return nil, err
			}
			if !bool(validResult.AsBool(ctx)) {
				break
			}
			count++
			_, err = iterObj.CallMethod(ctx, "next")
			if err != nil {
				return nil, err
			}
		}
	} else {
		it := iterObj.NewIterator()
		it.Reset(ctx)
		for it.Valid(ctx) {
			count++
			it.Next(ctx)
		}
	}

	return phpv.ZInt(count).ZVal(), nil
}

// resolveClass resolves a class from either an object or a class name string.
func resolveClass(ctx phpv.Context, z *phpv.ZVal, autoload bool) (*phpobj.ZClass, error) {
	switch z.GetType() {
	case phpv.ZtObject:
		obj, ok := z.Value().(*phpobj.ZObject)
		if !ok {
			return nil, ctx.Errorf("expected object")
		}
		c, ok := obj.Class.(*phpobj.ZClass)
		if !ok {
			return nil, ctx.Errorf("could not resolve class")
		}
		return c, nil
	case phpv.ZtString:
		className := z.AsString(ctx)
		cls, err := ctx.Global().GetClass(ctx, className, autoload)
		if err != nil {
			return nil, err
		}
		c, ok := cls.(*phpobj.ZClass)
		if !ok {
			return nil, ctx.Errorf("could not resolve class")
		}
		return c, nil
	default:
		return nil, ctx.Errorf("class_implements(): Argument #1 ($object_or_class) must be of type object|string, %s given", z.GetType())
	}
}

// > func array|false class_implements ( object|string $object_or_class [, bool $autoload = true ] )
func classImplements(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	if len(args) < 1 {
		return nil, ctx.Errorf("class_implements() expects at least 1 argument, 0 given")
	}

	autoload := true
	if len(args) > 1 {
		autoload = bool(args[1].AsBool(ctx))
	}

	cls, err := resolveClass(ctx, args[0], autoload)
	if err != nil {
		return phpv.ZFalse.ZVal(), nil
	}

	result := phpv.NewZArray()
	collectInterfaces(cls, result, ctx)

	return result.ZVal(), nil
}

// collectInterfaces collects all interfaces implemented by a class (recursively).
func collectInterfaces(cls *phpobj.ZClass, result *phpv.ZArray, ctx phpv.Context) {
	for _, impl := range cls.Implementations {
		name := impl.GetName()
		result.OffsetSet(ctx, name, name.ZVal())
		// Recursively collect interfaces that this interface extends
		collectInterfaces(impl, result, ctx)
	}
	if cls.Extends != nil {
		collectInterfaces(cls.Extends, result, ctx)
	}
}

// > func array|false class_parents ( object|string $object_or_class [, bool $autoload = true ] )
func classParents(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	if len(args) < 1 {
		return nil, ctx.Errorf("class_parents() expects at least 1 argument, 0 given")
	}

	autoload := true
	if len(args) > 1 {
		autoload = bool(args[1].AsBool(ctx))
	}

	cls, err := resolveClass(ctx, args[0], autoload)
	if err != nil {
		return phpv.ZFalse.ZVal(), nil
	}

	result := phpv.NewZArray()
	parent := cls.Extends
	for parent != nil {
		name := parent.GetName()
		result.OffsetSet(ctx, name, name.ZVal())
		parent = parent.Extends
	}

	return result.ZVal(), nil
}

// > func array|false class_uses ( object|string $object_or_class [, bool $autoload = true ] )
func classUses(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	if len(args) < 1 {
		return nil, ctx.Errorf("class_uses() expects at least 1 argument, 0 given")
	}

	autoload := true
	if len(args) > 1 {
		autoload = bool(args[1].AsBool(ctx))
	}

	cls, err := resolveClass(ctx, args[0], autoload)
	if err != nil {
		return phpv.ZFalse.ZVal(), nil
	}

	result := phpv.NewZArray()
	for _, tu := range cls.TraitUses {
		for _, traitName := range tu.TraitNames {
			result.OffsetSet(ctx, traitName, traitName.ZVal())
		}
	}

	return result.ZVal(), nil
}

// > func array spl_autoload_functions ( void )
func splAutoloadFunctions(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	loaders := ctx.Global().GetAutoloadFunctions()
	result := phpv.NewZArray()

	for _, loader := range loaders {
		result.OffsetSet(ctx, nil, loader.ZVal())
	}

	return result.ZVal(), nil
}
