package vm

import (
	"fmt"

	"github.com/KarpelesLab/goro/core/compiler"
	"github.com/KarpelesLab/goro/core/phpobj"
	"github.com/KarpelesLab/goro/core/phpv"
)

// foreachInit takes the source container and returns a ZIterator
// suitable for the loop. Returns (nil, nil) when src isn't iterable
// — the caller emits the standard PHP warning and skips the loop.
//
// For arrays, snapshot the source via Dup() so mutations during the
// loop don't affect iteration (matches PHP COW semantics in
// runnableForeach.Run). For objects, defer to z.NewIterator() which
// dispatches to Iterator/IteratorAggregate / generic property
// iteration via the same code path the AST uses.
//
// By-ref foreach is out of scope — the emitter must not call this for
// a by-ref loop (returns ErrUnsupported at compile time).
func foreachInit(ctx phpv.Context, src *phpv.ZVal) (phpv.ZIterator, error) {
	if src == nil {
		return nil, nil
	}
	switch src.GetType() {
	case phpv.ZtArray:
		// Snapshot the array so loop body mutations don't affect iteration.
		snap := src.Dup()
		if arr, ok := snap.Value().(*phpv.ZArray); ok {
			arr.IncRefObjects()
			// Decref happens at OP_FOREACH_UNWIND time via the
			// iterator's Cleanup hook — we attach it below.
			it := snap.NewIterator()
			if _, err := it.Reset(ctx); err != nil {
				return nil, err
			}
			return &arrayIterCleanup{ZIterator: it, arr: arr, ctx: ctx}, nil
		}
		it := snap.NewIterator()
		if _, err := it.Reset(ctx); err != nil {
			return nil, err
		}
		return it, nil
	case phpv.ZtObject:
		// Object iteration mirrors runnableForeach.Run:
		//  - IteratorAggregate: call getIterator() and recurse.
		//  - Iterator (incl. Generator): drive via rewind/valid/
		//    current/key/next methods through the AST's
		//    phpObjectIterator (exposed by compiler).
		//  - Otherwise: NewIteratorInScope using the calling class
		//    so property visibility is honored.
		var it phpv.ZIterator
		if obj, ok := src.Value().(*phpobj.ZObject); ok {
			if obj.GetClass().Implements(phpobj.IteratorAggregate) {
				iterResult, err := obj.CallMethod(ctx, "getIterator")
				if err != nil {
					return nil, err
				}
				if iterResult != nil && iterResult.GetType() == phpv.ZtObject {
					if iterObj, ok := iterResult.Value().(*phpobj.ZObject); ok && iterObj.GetClass().Implements(phpobj.Iterator) {
						it = compiler.NewObjectMethodIterator(ctx, iterObj)
					}
				}
			} else if obj.GetClass().Implements(phpobj.Iterator) {
				// Generators and any class implementing Iterator
				// directly — drive via methods.
				it = compiler.NewObjectMethodIterator(ctx, obj)
			}
			if it == nil {
				if obj.IsLazy() {
					if err := obj.TriggerLazyInit(ctx); err != nil {
						return nil, err
					}
				}
				target := obj
				if obj.LazyState == phpobj.LazyProxyInitialized && obj.LazyInstance != nil {
					target = obj.ResolveProxy()
				}
				scope := ctx.Class()
				it = target.NewIteratorInScope(scope)
			}
		}
		if it == nil {
			it = src.NewIterator()
		}
		if it == nil {
			return nil, nil
		}
		if _, err := it.Reset(ctx); err != nil {
			return nil, err
		}
		return it, nil
	case phpv.ZtNull:
		// PHP 8: foreach over null emits a warning, no iteration.
		return nil, nil
	default:
		return nil, nil
	}
}

// arrayIterCleanup wraps an array iterator and adjusts object refcounts
// when iteration ends, mirroring the defer in runnableForeach.Run.
type arrayIterCleanup struct {
	phpv.ZIterator
	arr *phpv.ZArray
	ctx phpv.Context
}

// Cleanup is called by OP_FOREACH_UNWIND for snapshotted arrays.
func (c *arrayIterCleanup) Cleanup() {
	if c.arr != nil {
		c.arr.DecRefObjects(c.ctx)
		c.arr = nil
	}
}

// foreachWarnInvalid emits the same PHP warning runnableForeach.Run
// does when the source isn't iterable.
func foreachWarnInvalid(ctx phpv.Context, src *phpv.ZVal, loc *phpv.Loc) error {
	typeName := src.GetType().TypeName()
	if src.GetType() == phpv.ZtBool {
		if bool(src.AsBool(ctx)) {
			typeName = "true"
		} else {
			typeName = "false"
		}
	}
	phpErr := loc.Error(ctx, fmt.Errorf("foreach() argument must be of type array|object, %s given", typeName), phpv.E_WARNING)
	ctx.LogError(phpErr)
	return nil
}
