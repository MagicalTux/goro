package vmcompiler

import (
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
	if n.NewObjectIsAnonymous() {
		return unsupportedf("new class { … } (anonymous class)")
	}
	className := n.NewObjectClassName()
	if className == "" {
		return unsupportedf("new with dynamic class name")
	}
	args := n.NewObjectArgs()
	if len(args) > 0xFFFF {
		return unsupportedf("new with too many args")
	}
	for _, a := range args {
		if _, ok := a.(phpv.NamedArgument); ok {
			return unsupportedf("named argument in new")
		}
		if _, ok := a.(phpv.SpreadArgument); ok {
			return unsupportedf("spread argument in new")
		}
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

func (e *emitter) emitObjectVarRead(n objectVarNode) error {
	if n.ObjectVarIsNullSafe() {
		return unsupportedf("nullsafe property access")
	}
	name := n.ObjectVarName()
	// runObjectVar uses a "$"-prefixed varName to encode dynamic
	// access (`$this->$x`). We don't lower that natively yet — it
	// requires evaluating the name expression at runtime.
	if len(name) > 0 && name[0] == '$' {
		return unsupportedf("dynamic property name")
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
		return unsupportedf("static-style call via -> AST")
	}
	if n.ObjectFuncIsNullSafe() {
		return unsupportedf("nullsafe method call")
	}
	name := n.ObjectFuncName()
	if len(name) > 0 && name[0] == '$' {
		return unsupportedf("dynamic method name")
	}
	args := n.ObjectFuncArgs()
	if len(args) > 0xFFFF {
		return unsupportedf("method call with too many args (>=65536)")
	}
	for _, a := range args {
		if _, ok := a.(phpv.NamedArgument); ok {
			return unsupportedf("named argument in method call")
		}
		if _, ok := a.(phpv.SpreadArgument); ok {
			return unsupportedf("spread argument in method call")
		}
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
