package vm

import (
	"fmt"

	"github.com/KarpelesLab/goro/core/compiler"
	"github.com/KarpelesLab/goro/core/phpobj"
	"github.com/KarpelesLab/goro/core/phpv"
)

// objectGet implements OP_OBJECT_GET. Mirrors the AST runObjectVar.Run
// for the read case on the supported subset:
//   - ZtObject: dispatch to ZObject.ObjectGet (handles __get, visibility,
//     lazy init, virtual properties, etc.).
//   - ZtNull: warn "Attempt to read property on null", return null.
//   - ZtBool false: same.
//   - ZtBool true / ZtInt / ZtFloat / ZtString / ZtArray: the AST emits
//     a warning and returns null. Mirror that behaviour.
func objectGet(ctx phpv.Context, receiver *phpv.ZVal, name phpv.ZString) (*phpv.ZVal, error) {
	switch receiver.GetType() {
	case phpv.ZtObject:
		obj, ok := receiver.Value().(*phpobj.ZObject)
		if !ok {
			// Non-internal objects implementing ZObject without being
			// *ZObject are rare. Use the interface form.
			if zo, ok := receiver.Value().(phpv.ZObject); ok {
				return zo.ObjectGet(ctx, name)
			}
			return nil, fmt.Errorf("vm: receiver is not a ZObject")
		}
		return obj.ObjectGet(ctx, name)
	case phpv.ZtNull:
		if err := ctx.Warn("Attempt to read property \"%s\" on null", string(name)); err != nil {
			return nil, err
		}
		return phpv.ZNULL.ZVal(), nil
	case phpv.ZtBool:
		if !bool(receiver.Value().(phpv.ZBool)) {
			if err := ctx.Warn("Attempt to read property \"%s\" on false", string(name)); err != nil {
				return nil, err
			}
			return phpv.ZNULL.ZVal(), nil
		}
		if err := ctx.Warn("Attempt to read property \"%s\" on true", string(name)); err != nil {
			return nil, err
		}
		return phpv.ZNULL.ZVal(), nil
	case phpv.ZtInt:
		if err := ctx.Warn("Attempt to read property \"%s\" on int", string(name)); err != nil {
			return nil, err
		}
		return phpv.ZNULL.ZVal(), nil
	case phpv.ZtFloat:
		if err := ctx.Warn("Attempt to read property \"%s\" on float", string(name)); err != nil {
			return nil, err
		}
		return phpv.ZNULL.ZVal(), nil
	case phpv.ZtString:
		if err := ctx.Warn("Attempt to read property \"%s\" on string", string(name)); err != nil {
			return nil, err
		}
		return phpv.ZNULL.ZVal(), nil
	case phpv.ZtArray:
		if err := ctx.Warn("Attempt to read property \"%s\" on array", string(name)); err != nil {
			return nil, err
		}
		return phpv.ZNULL.ZVal(), nil
	default:
		return nil, fmt.Errorf("vm: receiver type %v not supported", receiver.GetType())
	}
}

// objectGetSafe implements OP_OBJECT_GET_SAFE: like objectGet but
// silences the "Undefined property" warning when the property is
// missing on a ZtObject receiver. Used as the LHS read for coalesce
// (`$obj->prop ?? default`) and similar permissive read sites. The
// non-object receiver warnings ("Attempt to read property on null/int/
// bool/...") still fire — PHP only treats the property-existence side
// permissively under `??`, not the receiver-type side.
func objectGetSafe(ctx phpv.Context, receiver *phpv.ZVal, name phpv.ZString) (*phpv.ZVal, error) {
	switch receiver.GetType() {
	case phpv.ZtObject:
		obj, ok := receiver.Value().(*phpobj.ZObject)
		if !ok {
			// Non-internal objects: fall back to the loud path; rare.
			if zo, ok := receiver.Value().(phpv.ZObject); ok {
				return zo.ObjectGet(ctx, name)
			}
			return nil, fmt.Errorf("vm: receiver is not a ZObject")
		}
		v, _, err := obj.ObjectGetQuiet(ctx, name)
		if err != nil {
			return nil, err
		}
		if v == nil {
			return phpv.ZNULL.ZVal(), nil
		}
		return v, nil
	default:
		// Non-object receivers behave the same as the loud path.
		return objectGet(ctx, receiver, name)
	}
}

// objectSet implements OP_OBJECT_SET: writes receiver->name = value.
// Routes through ZObject.ObjectSet which handles __set, typed
// properties, asymmetric visibility, etc.
func objectSet(ctx phpv.Context, receiver *phpv.ZVal, name phpv.ZString, value *phpv.ZVal) error {
	switch receiver.GetType() {
	case phpv.ZtObject:
		obj, ok := receiver.Value().(*phpobj.ZObject)
		if !ok {
			if zo, ok := receiver.Value().(phpv.ZObject); ok {
				return zo.ObjectSet(ctx, name, value)
			}
			return fmt.Errorf("vm: receiver is not a ZObject")
		}
		return obj.ObjectSet(ctx, name, value)
	default:
		// PHP's value-specific naming: "true"/"false"/"null" rather
		// than the generic type names. Matches the AST runner via
		// compiler.PhpValueTypeName.
		return phpobj.ThrowError(ctx, phpobj.Error,
			fmt.Sprintf("Attempt to assign property \"%s\" on %s", string(name), compiler.PhpValueTypeName(receiver)))
	}
}

// objectCall implements OP_OBJECT_CALL: invokes receiver->name(args).
// Routes through compiler.CallInstanceMethod which mirrors the AST's
// runObjectFunc visibility and dispatch logic (private/protected
// checks, abstract rejection, __call fallback). It still uses
// pre-evaluated ZVal args so by-ref parameters won't propagate
// mutations back to the caller's locals — for now those callers
// fall back to AST naturally because their bodies use other
// unsupported features.
func objectCall(ctx phpv.Context, receiver *phpv.ZVal, name phpv.ZString, args []*phpv.ZVal) (*phpv.ZVal, error) {
	if receiver == nil || receiver.GetType() != phpv.ZtObject {
		typeName := "null"
		if receiver != nil {
			typeName = receiver.GetType().TypeName()
		}
		return nil, phpobj.ThrowError(ctx, phpobj.Error,
			fmt.Sprintf("Call to a member function %s() on %s", string(name), typeName))
	}
	return compiler.CallInstanceMethod(ctx, receiver, name, args)
}
