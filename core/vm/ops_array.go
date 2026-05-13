package vm

import (
	"fmt"

	"github.com/KarpelesLab/goro/core/phpobj"
	"github.com/KarpelesLab/goro/core/phpv"
)

// arrayGet implements OP_ARRAY_GET. Mirrors the AST runArrayAccess.Run
// on the read side, dispatching on the runtime container type:
//   - ZtArray: ZArray.OffsetGetWarn (undefined-key warning fires).
//   - ZtObject: dispatch to ArrayAccess interface via v.Array().
//   - ZtString: read a single-byte substring at the numeric offset.
//   - ZtNull / ZtBool false: warn, return null.
//   - ZtBool true / ZtInt / ZtFloat: "Cannot use a scalar value as an
//     array" Error.
func arrayGet(ctx phpv.Context, container, offset *phpv.ZVal) (*phpv.ZVal, error) {
	switch container.GetType() {
	case phpv.ZtArray:
		arr := container.AsArray(ctx)
		key := offset.Value()
		return arr.OffsetGetWarn(ctx, key)
	case phpv.ZtObject:
		// ArrayAccess interface dispatch. v.Array() returns a
		// ZArrayAccess that delegates to offsetGet/offsetExists.
		access := container.Array()
		if access == nil {
			return nil, phpobj.ThrowError(ctx, phpobj.Error,
				"Cannot use object of type "+string(container.AsObject(ctx).GetClass().GetName())+" as array")
		}
		return access.OffsetGet(ctx, offset.Value())
	case phpv.ZtString:
		// String offset access — the AST routes via v.AsString(ctx).Array().OffsetGet.
		return container.AsString(ctx).Array().OffsetGet(ctx, offset.Value())
	case phpv.ZtNull:
		if err := ctx.Warn("Trying to access array offset on null"); err != nil {
			return nil, err
		}
		return phpv.ZNULL.ZVal(), nil
	case phpv.ZtBool:
		if !bool(container.Value().(phpv.ZBool)) {
			if err := ctx.Warn("Trying to access array offset on false"); err != nil {
				return nil, err
			}
			return phpv.ZNULL.ZVal(), nil
		}
		// true and other scalars: same fatal error as the AST.
		return nil, phpobj.ThrowError(ctx, phpobj.Error,
			fmt.Sprintf("Cannot use a scalar value as an array"))
	case phpv.ZtInt, phpv.ZtFloat:
		return nil, phpobj.ThrowError(ctx, phpobj.Error,
			fmt.Sprintf("Cannot use a scalar value as an array"))
	default:
		return nil, fmt.Errorf("vm: OP_ARRAY_GET on container type %v is not supported", container.GetType())
	}
}

// arraySetLocal does `$local[offset] = value` (offset==nil means
// append). Implements the PHP semantics for the runtime-determined
// container type at the local: array, null/false (auto-vivify),
// string (string-offset write with the same warnings as the AST),
// object (ArrayAccess interface), other (Error).
//
// Takes the frame so it can update both f.locals[idx] (for the slot
// cache) and the FuncContext hashtable in lock-step.
func arraySetLocal(ctx phpv.Context, f *Frame, idx uint16, offset, value *phpv.ZVal) error {
	name := f.fn.Locals[idx]
	cur := f.locals[idx]
	if cur == nil {
		cur = phpv.ZNULL.ZVal()
	}

	// Value semantics: if the stored value is itself an array, Dup
	// it so the container holds an independent COW copy (matches the
	// AST's runOperator `=` path, which calls res.ZVal() that does
	// a.Dup() for ZtArray). Without this, \`$a[0] = $a\` would
	// create a self-recursion instead of snapshotting.
	if value != nil && value.GetType() == phpv.ZtArray && !value.IsRef() {
		value = value.Dup()
	}

	switch cur.GetType() {
	case phpv.ZtArray:
		arr := cur.AsArray(ctx)
		var k phpv.Val
		if offset != nil {
			k = offset.Value()
		}
		if err := arr.OffsetSet(ctx, k, value); err != nil {
			if err == phpv.ErrNextElementOccupied {
				return phpobj.ThrowError(ctx, phpobj.Error, err.Error())
			}
			return err
		}
		return nil

	case phpv.ZtNull:
		// Auto-vivify: replace cur's value with a fresh array.
		newArr := phpv.NewZArrayTracked(ctx.Global().MemMgrTracker())
		cur.SetVal(newArr)
		var k phpv.Val
		if offset != nil {
			k = offset.Value()
		}
		if err := newArr.OffsetSet(ctx, k, value); err != nil {
			return err
		}
		f.locals[idx] = cur
		if f.fn.SlotOnly {
			return nil
		}
		return ctx.OffsetSet(ctx, name, cur)

	case phpv.ZtBool:
		if !bool(cur.Value().(phpv.ZBool)) {
			// false → auto-vivify with PHP 8.1 deprecation warning.
			if err := ctx.Deprecated("Automatic conversion of false to array is deprecated"); err != nil {
				return err
			}
			newArr := phpv.NewZArrayTracked(ctx.Global().MemMgrTracker())
			cur.SetVal(newArr)
			var k phpv.Val
			if offset != nil {
				k = offset.Value()
			}
			if err := newArr.OffsetSet(ctx, k, value); err != nil {
				return err
			}
			f.locals[idx] = cur
			return ctx.OffsetSet(ctx, name, cur)
		}
		return phpobj.ThrowError(ctx, phpobj.Error, "Cannot use a scalar value as an array")

	case phpv.ZtString:
		// Append form is not allowed on strings.
		if offset == nil {
			return phpobj.ThrowError(ctx, phpobj.Error, "[] operator not supported for strings")
		}
		// Use ZStringArray which handles all the PHP string-offset
		// edge cases (illegal-offset warnings, single-byte truncation,
		// padding with spaces, …). The wrapper holds a *ZString so
		// mutation lands in a copy; we then write the result back to
		// the local.
		s := cur.Value().(phpv.ZString)
		sa := s.Array()
		var k phpv.Val
		if offset != nil {
			k = offset.Value()
		}
		if err := sa.OffsetSet(ctx, k, value); err != nil {
			// ZStringArray returns plain errors.New for the
			// "Cannot assign an empty string …" case (and similar).
			// Wrap as a throwable Error so catch() can handle it
			// the way the AST does.
			if _, isPhpErr := err.(*phpv.PhpError); !isPhpErr {
				return phpobj.ThrowError(ctx, phpobj.Error, err.Error())
			}
			return err
		}
		// Update local with the (possibly modified) string.
		cur.SetVal(sa.String())
		f.locals[idx] = cur
		if f.fn.SlotOnly {
			return nil
		}
		return ctx.OffsetSet(ctx, name, cur)

	case phpv.ZtObject:
		// ArrayAccess interface dispatch.
		access := cur.Array()
		if access == nil {
			return phpobj.ThrowError(ctx, phpobj.Error,
				fmt.Sprintf("Cannot use object of type %s as array",
					cur.AsObject(ctx).GetClass().GetName()))
		}
		var k phpv.Val
		if offset != nil {
			k = offset.Value()
		}
		return access.OffsetSet(ctx, k, value)

	case phpv.ZtInt, phpv.ZtFloat:
		return phpobj.ThrowError(ctx, phpobj.Error, "Cannot use a scalar value as an array")
	default:
		return fmt.Errorf("vm: array-write on container type %v not supported", cur.GetType())
	}
}
