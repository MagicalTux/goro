package vm

import (
	"fmt"

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

// objectCall implements OP_OBJECT_CALL: invokes receiver->name(args).
// Routes through ZObject.GetMethod + ctx.CallZVal so visibility,
// __call, returns-by-ref, and stack-trace bookkeeping are all
// handled by the same code paths the AST uses.
//
// A null receiver throws Error (as in PHP); other scalar receivers
// likewise throw — same semantics as runObjectFunc.Run.
func objectCall(ctx phpv.Context, receiver *phpv.ZVal, name phpv.ZString, args []*phpv.ZVal) (*phpv.ZVal, error) {
	if receiver == nil || receiver.GetType() != phpv.ZtObject {
		typeName := "null"
		if receiver != nil {
			typeName = receiver.GetType().TypeName()
		}
		return nil, phpobj.ThrowError(ctx, phpobj.Error,
			fmt.Sprintf("Call to a member function %s() on %s", string(name), typeName))
	}
	obj, ok := receiver.Value().(*phpobj.ZObject)
	if !ok {
		if zo, ok := receiver.Value().(phpv.ZObject); ok {
			// The interface form lacks GetMethod; fall back to a runtime
			// error so the caller can choose to skip this opcode for
			// unusual ZObject implementations.
			_ = zo
		}
		return nil, fmt.Errorf("vm: receiver is not a *ZObject")
	}
	m, err := obj.GetMethod(name, ctx)
	if err != nil {
		return nil, err
	}
	return ctx.CallZVal(ctx, m, args, obj)
}
