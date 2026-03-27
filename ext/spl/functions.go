package spl

import (
	"fmt"
	"iter"

	"github.com/MagicalTux/goro/core"
	"github.com/MagicalTux/goro/core/logopt"
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

// > func array iterator_to_array ( Traversable|array $iterator [, bool $preserve_keys = true ] )
func iteratorToArray(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	if len(args) < 1 {
		return nil, phpobj.ThrowError(ctx, phpobj.TypeError, "iterator_to_array() expects at least 1 argument, 0 given")
	}

	z := args[0]
	preserveKeys := true
	if len(args) > 1 {
		preserveKeys = bool(args[1].AsBool(ctx))
	}

	// PHP 8.2+ supports arrays directly
	if z.GetType() == phpv.ZtArray {
		if preserveKeys {
			return z.Dup(), nil
		}
		// Reindex
		arr := z.Value().(*phpv.ZArray)
		result := phpv.NewZArray()
		idx := 0
		for _, v := range arr.Iterate(ctx) {
			result.OffsetSet(ctx, phpv.ZInt(idx), v)
			idx++
		}
		return result.ZVal(), nil
	}

	if z.GetType() != phpv.ZtObject {
		return nil, phpobj.ThrowError(ctx, phpobj.TypeError,
			fmt.Sprintf("iterator_to_array(): Argument #1 ($iterator) must be of type Traversable|array, %s given", z.GetType()))
	}

	obj, ok := z.Value().(*phpobj.ZObject)
	if !ok {
		return nil, phpobj.ThrowError(ctx, phpobj.TypeError,
			fmt.Sprintf("iterator_to_array(): Argument #1 ($iterator) must be of type Traversable|array, %s given", z.GetType()))
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

func (w *phpIteratorWrapper) IterateRaw(ctx phpv.Context) iter.Seq2[*phpv.ZVal, *phpv.ZVal] {
	return w.Iterate(ctx)
}

// > func int iterator_count ( Traversable|array $iterator )
func iteratorCount(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	if len(args) < 1 {
		return nil, phpobj.ThrowError(ctx, phpobj.TypeError, "iterator_count() expects exactly 1 argument, 0 given")
	}

	z := args[0]

	// PHP 8.2+ supports arrays directly
	if z.GetType() == phpv.ZtArray {
		arr := z.Value().(*phpv.ZArray)
		return phpv.ZInt(arr.Count(ctx)).ZVal(), nil
	}

	if z.GetType() != phpv.ZtObject {
		return nil, phpobj.ThrowError(ctx, phpobj.TypeError,
			fmt.Sprintf("iterator_count(): Argument #1 ($iterator) must be of type Traversable|array, %s given", z.GetType()))
	}

	obj, ok := z.Value().(*phpobj.ZObject)
	if !ok {
		return nil, phpobj.ThrowError(ctx, phpobj.TypeError,
			fmt.Sprintf("iterator_count(): Argument #1 ($iterator) must be of type Traversable|array, %s given", z.GetType()))
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
// funcName is used for error messages (e.g., "class_implements", "class_parents", "class_uses").
func resolveClass(ctx phpv.Context, z *phpv.ZVal, autoload bool, funcName ...string) (*phpobj.ZClass, error) {
	fn := "class_implements"
	if len(funcName) > 0 {
		fn = funcName[0]
	}
	switch z.GetType() {
	case phpv.ZtObject:
		obj, ok := z.Value().(*phpobj.ZObject)
		if !ok {
			return nil, phpobj.ThrowError(ctx, phpobj.TypeError, fmt.Sprintf("%s(): Argument #1 ($object_or_class) must be of type object|string, %s given", fn, z.GetType()))
		}
		c, ok := obj.Class.(*phpobj.ZClass)
		if !ok {
			return nil, phpobj.ThrowError(ctx, phpobj.TypeError, fmt.Sprintf("%s(): Argument #1 ($object_or_class) must be of type object|string, %s given", fn, z.GetType()))
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
			return nil, fmt.Errorf("could not resolve class")
		}
		return c, nil
	default:
		return nil, phpobj.ThrowError(ctx, phpobj.TypeError, fmt.Sprintf("%s(): Argument #1 ($object_or_class) must be of type object|string, %s given", fn, phpv.ZValTypeNameDetailed(z)))
	}
}

// > func array|false class_implements ( object|string $object_or_class [, bool $autoload = true ] )
func classImplements(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	if len(args) < 1 {
		return nil, phpobj.ThrowError(ctx, phpobj.ArgumentCountError, "class_implements() expects at least 1 argument, 0 given")
	}

	autoload := true
	if len(args) > 1 {
		autoload = bool(args[1].AsBool(ctx))
	}

	cls, err := resolveClass(ctx, args[0], autoload, "class_implements")
	if err != nil {
		if args[0].GetType() == phpv.ZtString {
			className := args[0].AsString(ctx)
			if autoload {
				ctx.Warn("class_implements(): Class %s does not exist and could not be loaded", className, logopt.NoFuncName(true))
			} else {
				ctx.Warn("class_implements(): Class %s does not exist", className, logopt.NoFuncName(true))
			}
			return phpv.ZFalse.ZVal(), nil
		}
		return nil, err
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
		return nil, phpobj.ThrowError(ctx, phpobj.ArgumentCountError, "class_parents() expects at least 1 argument, 0 given")
	}

	autoload := true
	if len(args) > 1 {
		autoload = bool(args[1].AsBool(ctx))
	}

	cls, err := resolveClass(ctx, args[0], autoload, "class_parents")
	if err != nil {
		if args[0].GetType() == phpv.ZtString {
			className := args[0].AsString(ctx)
			if autoload {
				ctx.Warn("class_parents(): Class %s does not exist and could not be loaded", className, logopt.NoFuncName(true))
			} else {
				ctx.Warn("class_parents(): Class %s does not exist", className, logopt.NoFuncName(true))
			}
			return phpv.ZFalse.ZVal(), nil
		}
		return nil, err
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
		return nil, phpobj.ThrowError(ctx, phpobj.ArgumentCountError, "class_uses() expects at least 1 argument, 0 given")
	}

	autoload := true
	if len(args) > 1 {
		autoload = bool(args[1].AsBool(ctx))
	}

	cls, err := resolveClass(ctx, args[0], autoload, "class_uses")
	if err != nil {
		if args[0].GetType() == phpv.ZtString {
			className := args[0].AsString(ctx)
			if autoload {
				ctx.Warn("class_uses(): Class %s does not exist and could not be loaded", className, logopt.NoFuncName(true))
			} else {
				ctx.Warn("class_uses(): Class %s does not exist", className, logopt.NoFuncName(true))
			}
			return phpv.ZFalse.ZVal(), nil
		}
		return nil, err
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
		// Convert callable to the PHP representation:
		// - Named functions: string (function name)
		// - Method callables: [object/class, method] array
		// - Closures: Closure object
		var entry *phpv.ZVal
		switch c := loader.(type) {
		case *phpv.MethodCallable:
			arr := phpv.NewZArray()
			if c.Static {
				arr.OffsetSet(ctx, nil, phpv.ZString(c.Class.GetName()).ZVal())
			} else if bc, ok := loader.(*phpv.BoundedCallable); ok && bc.This != nil {
				arr.OffsetSet(ctx, nil, bc.This.ZVal())
			} else {
				arr.OffsetSet(ctx, nil, phpv.ZString(c.Class.GetName()).ZVal())
			}
			arr.OffsetSet(ctx, nil, phpv.ZString(c.Callable.Name()).ZVal())
			entry = arr.ZVal()
		case *phpv.BoundedCallable:
			if c.This != nil {
				arr := phpv.NewZArray()
				arr.OffsetSet(ctx, nil, c.This.ZVal())
				arr.OffsetSet(ctx, nil, phpv.ZString(c.Callable.Name()).ZVal())
				entry = arr.ZVal()
			} else {
				// Check if the inner callable is a closure (has Spawn)
				if spawner, ok := c.Callable.(interface {
					Spawn(phpv.Context) (*phpv.ZVal, error)
				}); ok {
					if v, err := spawner.Spawn(ctx); err == nil {
						entry = v
					}
				}
				if entry == nil {
					entry = phpv.ZString(c.Name()).ZVal()
				}
			}
		default:
			// Check if this is a closure (has Spawn method)
			if spawner, ok := loader.(interface {
				Spawn(phpv.Context) (*phpv.ZVal, error)
			}); ok {
				if v, err := spawner.Spawn(ctx); err == nil {
					entry = v
				}
			}
			if entry == nil {
				// For named functions (including spl_autoload), return the name as string
				name := phpv.CallableDisplayName(loader)
				entry = phpv.ZString(name).ZVal()
			}
		}
		result.OffsetSet(ctx, nil, entry)
	}

	return result.ZVal(), nil
}

// > func int iterator_apply ( Traversable $iterator , callable $callback [, array $args = [] ] )
func iteratorApply(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	if len(args) < 2 {
		return nil, phpobj.ThrowError(ctx, phpobj.TypeError, fmt.Sprintf("iterator_apply() expects at least 2 arguments, %d given", len(args)))
	}
	if len(args) > 3 {
		return nil, phpobj.ThrowError(ctx, phpobj.TypeError, fmt.Sprintf("iterator_apply() expects at most 3 arguments, %d given", len(args)))
	}

	z := args[0]
	callback := args[1]

	if z.GetType() != phpv.ZtObject {
		return nil, phpobj.ThrowError(ctx, phpobj.TypeError, fmt.Sprintf("iterator_apply(): Argument #1 ($iterator) must be of type Traversable, %s given", z.GetType()))
	}

	// Validate callback
	if callback.GetType() != phpv.ZtString && callback.GetType() != phpv.ZtArray && callback.GetType() != phpv.ZtObject {
		return nil, phpobj.ThrowError(ctx, phpobj.TypeError, fmt.Sprintf("iterator_apply(): Argument #2 ($callback) must be a valid callback, no array or string given"))
	}

	// Validate optional args parameter
	var extraArgs []*phpv.ZVal
	if len(args) > 2 {
		if args[2].GetType() != phpv.ZtArray && !args[2].IsNull() {
			return nil, phpobj.ThrowError(ctx, phpobj.TypeError, fmt.Sprintf("iterator_apply(): Argument #3 ($args) must be of type ?array, %s given", args[2].GetType()))
		}
		if args[2].GetType() == phpv.ZtArray {
			arr := args[2].Value().(*phpv.ZArray)
			for _, v := range arr.Iterate(ctx) {
				extraArgs = append(extraArgs, v)
			}
		}
	}

	obj, ok := z.Value().(*phpobj.ZObject)
	if !ok {
		return nil, phpobj.ThrowError(ctx, phpobj.TypeError, fmt.Sprintf("iterator_apply(): Argument #1 ($iterator) must be of type Traversable, %s given", z.GetType()))
	}

	// Resolve callback
	cb, err := core.SpawnCallable(ctx, callback)
	if err != nil {
		return nil, err
	}

	// Get the iterator - handle IteratorAggregate
	iterObj := obj
	if obj.GetClass().Implements(phpobj.IteratorAggregate) {
		iterResult, err := obj.CallMethod(ctx, "getIterator")
		if err != nil {
			return nil, err
		}
		if iterResult != nil && iterResult.GetType() == phpv.ZtObject {
			if io, ok := iterResult.Value().(*phpobj.ZObject); ok {
				iterObj = io
			}
		}
	}

	count := 0

	_, err = iterObj.CallMethod(ctx, "rewind")
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

		result, err := ctx.CallZVal(ctx, cb, extraArgs, nil)
		if err != nil {
			return nil, err
		}
		count++

		if result == nil || !bool(result.AsBool(ctx)) {
			break
		}

		_, err = iterObj.CallMethod(ctx, "next")
		if err != nil {
			return nil, err
		}
	}

	return phpv.ZInt(count).ZVal(), nil
}

// > func void spl_autoload_call ( string $class )
func splAutoloadCall(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	if len(args) < 1 {
		return nil, phpobj.ThrowError(ctx, phpobj.TypeError, "spl_autoload_call() expects exactly 1 argument, 0 given")
	}

	if args[0].GetType() != phpv.ZtString {
		return nil, phpobj.ThrowError(ctx, phpobj.TypeError, fmt.Sprintf("spl_autoload_call(): Argument #1 ($class) must be of type string, %s given", args[0].GetType()))
	}

	className := args[0].AsString(ctx)

	// Try to load the class using all registered autoloaders
	loaders := ctx.Global().GetAutoloadFunctions()
	for _, loader := range loaders {
		ctx.CallZVal(ctx, loader, []*phpv.ZVal{className.ZVal()}, nil)
		// Check if the class now exists
		if _, err := ctx.Global().GetClass(ctx, className, false); err == nil {
			return nil, nil
		}
	}

	return nil, nil
}

// splAutoloadExtensionsValue stores the current file extensions for spl_autoload
var splAutoloadExtensionsValue = ".inc,.php"

// > func string spl_autoload_extensions ( [string $file_extensions] )
func splAutoloadExtensions(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	if len(args) > 0 {
		splAutoloadExtensionsValue = string(args[0].AsString(ctx))
	}
	return phpv.ZString(splAutoloadExtensionsValue).ZVal(), nil
}

// > func array spl_classes ( void )
func splClasses(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	result := phpv.NewZArray()

	// Return a list of all SPL classes
	classes := []string{
		"AppendIterator",
		"ArrayIterator",
		"ArrayObject",
		"BadFunctionCallException",
		"BadMethodCallException",
		"CachingIterator",
		"CallbackFilterIterator",
		"DirectoryIterator",
		"DomainException",
		"EmptyIterator",
		"FilesystemIterator",
		"FilterIterator",
		"GlobIterator",
		"InfiniteIterator",
		"InvalidArgumentException",
		"IteratorIterator",
		"LengthException",
		"LimitIterator",
		"LogicException",
		"MultipleIterator",
		"NoRewindIterator",
		"OutOfBoundsException",
		"OutOfRangeException",
		"OverflowException",
		"ParentIterator",
		"RangeException",
		"RecursiveArrayIterator",
		"RecursiveCachingIterator",
		"RecursiveCallbackFilterIterator",
		"RecursiveDirectoryIterator",
		"RecursiveFilterIterator",
		"RecursiveIteratorIterator",
		"RecursiveRegexIterator",
		"RecursiveTreeIterator",
		"RegexIterator",
		"RuntimeException",
		"SplDoublyLinkedList",
		"SplFileInfo",
		"SplFileObject",
		"SplFixedArray",
		"SplHeap",
		"SplMaxHeap",
		"SplMinHeap",
		"SplObjectStorage",
		"SplPriorityQueue",
		"SplQueue",
		"SplStack",
		"SplTempFileObject",
		"UnderflowException",
		"UnexpectedValueException",
	}

	for _, name := range classes {
		result.OffsetSet(ctx, phpv.ZString(name), phpv.ZString(name).ZVal())
	}

	return result.ZVal(), nil
}
