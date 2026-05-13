package vmcompiler

import (
	"github.com/KarpelesLab/goro/core/compiler"
	"github.com/KarpelesLab/goro/core/phpv"
	"github.com/KarpelesLab/goro/core/vm"
)

// newObjectNode matches *compiler.runNewObject.
type newObjectNode interface {
	NewObjectClassName() phpv.ZString
	NewObjectArgs() phpv.Runnables
	NewObjectIsAnonymous() bool
	NewObjectLoc() *phpv.Loc
}

// objectVarNode matches *compiler.runObjectVar.
type objectVarNode interface {
	ObjectVarReceiver() phpv.Runnable
	ObjectVarName() phpv.ZString
	ObjectVarLoc() *phpv.Loc
	ObjectVarIsNullSafe() bool
}

// objectFuncNode matches *compiler.runObjectFunc.
type objectFuncNode interface {
	ObjectFuncReceiver() phpv.Runnable
	ObjectFuncName() phpv.ZString
	ObjectFuncArgs() phpv.Runnables
	ObjectFuncLoc() *phpv.Loc
	ObjectFuncIsStatic() bool
	ObjectFuncIsNullSafe() bool
}

func (e *emitter) emitNewObject(n newObjectNode) error {
	className := n.NewObjectClassName()
	args := n.NewObjectArgs()
	// AST-delegate the `new` expression when the class is anonymous,
	// the name is dynamic, or the args list has named/spread
	// wrappers. The AST runner pushes the resulting object.
	if n.NewObjectIsAnonymous() || className == "" || compiler.CallHasSpecialArgs(args) {
		raw, ok := n.(phpv.Runnable)
		if !ok {
			return unsupportedf("new AST delegation: cannot retrieve raw Runnable")
		}
		idx := e.astIndex(raw)
		e.emit(vm.OpClassConst, idx, 0, 0)
		e.pushStack(1)
		return nil
	}
	if len(args) > 0xFFFF {
		return unsupportedf("new with too many args")
	}
	for _, a := range args {
		if err := e.withSubexpr(func() error { return e.emitExpr(a) }); err != nil {
			return err
		}
	}
	idx := e.constIndex(className)
	e.emit(vm.OpNewObject, idx, uint16(len(args)), 0)
	// pops argc args, pushes 1 result. Net delta: -(argc-1)
	if len(args) > 0 {
		e.popStack(len(args) - 1)
	} else {
		e.pushStack(1)
	}
	return nil
}

func (e *emitter) emitObjectVarAssign(lhs objectVarNode, rhs phpv.Runnable, stmtCtx bool) error {
	if !stmtCtx {
		return unsupportedf("property assign in non-statement context")
	}
	if lhs.ObjectVarIsNullSafe() {
		return unsupportedf("nullsafe property assign")
	}
	name := lhs.ObjectVarName()
	if len(name) > 0 && name[0] == '$' {
		return unsupportedf("dynamic property name in assign")
	}
	if err := e.withSubexpr(func() error { return e.emitExpr(lhs.ObjectVarReceiver()) }); err != nil {
		return err
	}
	if err := e.withSubexpr(func() error { return e.emitExpr(rhs) }); err != nil {
		return err
	}
	idx := e.constIndex(name)
	e.emit(vm.OpObjectSet, idx, 0, 0)
	e.popStack(2) // pop receiver + value, no push
	return nil
}

func (e *emitter) emitObjectVarRead(n objectVarNode) error {
	name := n.ObjectVarName()
	// Nullsafe or dynamic-name property reads route through the AST
	// runner. Both shapes need branchy chain semantics that we don't
	// lower piecewise.
	if n.ObjectVarIsNullSafe() || (len(name) > 0 && name[0] == '$') {
		raw, ok := n.(phpv.Runnable)
		if !ok {
			return unsupportedf("property-read AST delegation: cannot retrieve raw Runnable")
		}
		idx := e.astIndex(raw)
		e.emit(vm.OpClassConst, idx, 0, 0)
		e.pushStack(1)
		return nil
	}
	if err := e.withSubexpr(func() error { return e.emitExpr(n.ObjectVarReceiver()) }); err != nil {
		return err
	}
	idx := e.constIndex(name)
	e.emit(vm.OpObjectGet, idx, 0, 0)
	// pop receiver, push value → net stack delta zero
	return nil
}

func (e *emitter) emitObjectFuncCall(n objectFuncNode) error {
	if n.ObjectFuncIsStatic() {
		// Static-style call: Foo::method(args), parent::method(args),
		// etc. The AST runObjectFunc.Run handles LSB binding, static
		// vs instance dispatch, and the "Non-static method called
		// statically" diagnostic. Delegate.
		raw, ok := n.(phpv.Runnable)
		if !ok {
			return unsupportedf("static call: cannot retrieve raw Runnable")
		}
		stmtCtx := e.stmtCtx
		idx := e.astIndex(raw)
		if stmtCtx {
			e.emit(vm.OpTryFinally, idx, 0, 0)
		} else {
			e.emit(vm.OpClassConst, idx, 0, 0)
			e.pushStack(1)
		}
		e.emit(vm.OpRefreshSlots, 0, 0, 0)
		return nil
	}
	name := n.ObjectFuncName()
	args := n.ObjectFuncArgs()
	// AST-delegate nullsafe, dynamic-name, or special-args method
	// calls. The shape itself isn't lowered piecewise.
	if n.ObjectFuncIsNullSafe() || (len(name) > 0 && name[0] == '$') || compiler.CallHasSpecialArgs(args) {
		raw, ok := n.(phpv.Runnable)
		if !ok {
			return unsupportedf("method-call AST delegation: cannot retrieve raw Runnable")
		}
		return e.emitCallViaAST(raw)
	}
	// Method-call writable-arg case: we can't statically know if
	// the resolved method declares a by-ref param (polymorphism).
	// AST-delegate with OP_SYNC_SLOTS so the AST sees fresh locals
	// in slot-only bodies, then refresh after. Avoids marking the
	// surrounding body slot-unsafe.
	if compiler.CallHasWritableArg(args) {
		raw, ok := n.(phpv.Runnable)
		if !ok {
			return unsupportedf("method-call AST delegation: cannot retrieve raw Runnable")
		}
		return e.emitCallViaASTSync(raw)
	}
	if len(args) > 0xFFFF {
		return unsupportedf("method call with too many args (>=65536)")
	}
	// Push receiver, then args.
	if err := e.withSubexpr(func() error { return e.emitExpr(n.ObjectFuncReceiver()) }); err != nil {
		return err
	}
	for _, a := range args {
		if err := e.withSubexpr(func() error { return e.emitExpr(a) }); err != nil {
			return err
		}
	}
	nameIdx := e.constIndex(n.ObjectFuncName())
	e.emit(vm.OpObjectCall, nameIdx, uint16(len(args)), 0)
	// Pops receiver + argc args, pushes 1 result. Net delta: -(1 + argc) + 1 = -argc.
	if len(args) > 0 {
		e.popStack(len(args))
	}

	// In statement context discard the result.
	if e.stmtCtx {
		e.emit(vm.OpPop, 0, 0, 0)
		e.popStack(1)
	}
	return nil
}
