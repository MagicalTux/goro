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
	// PHP's write-context for `$x->prop = v`: an undefined `$x` does
	// NOT emit "Undefined variable" — the AST's runVariable.Run sees
	// Parent=runOperator(write) and short-circuits the warning. For
	// the simple-variable receiver, emit the silent OP_LOAD_LOCAL
	// instead of OP_LOAD_LOCAL_OR_WARN.
	recv := lhs.ObjectVarReceiver()
	if v, ok := recv.(variableNode); ok {
		idx := e.localIndex(v.VariableName())
		e.emit(vm.OpLoadLocal, idx, 0, 0)
		e.pushStack(1)
	} else {
		if err := e.withSubexpr(func() error { return e.emitExpr(recv) }); err != nil {
			return err
		}
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
	// `$obj->$x` (no curly braces) parses to runObjectVar with the
	// literal dollar-prefixed token as the name (e.g. "$x"). That form
	// needs runtime variable lookup, which the simple OP_OBJECT_GET
	// path doesn't do — AST-delegate via OpClassConst. Curly-brace
	// form `$obj->{$x}` parses to runObjectDynVar and is intercepted
	// by IsObjectDynVarReadNode earlier.
	if len(name) > 0 && name[0] == '$' {
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
	var bFlag uint16
	if n.ObjectVarIsNullSafe() {
		bFlag = 1
	}
	e.emit(vm.OpObjectGet, idx, bFlag, 0)
	// pop receiver, push value → net stack delta zero
	return nil
}

func (e *emitter) emitObjectFuncCall(n objectFuncNode) error {
	if n.ObjectFuncIsStatic() {
		// Static-style call: Foo::method(args), parent::method(args),
		// self::method(args), static::method(args). Dedicated
		// OP_STATIC_METHOD_CALL routes through EvalStaticMethodCall
		// which dispatches LSB binding + visibility + the
		// "Non-static method called statically" diagnostic via the
		// AST runObjectFunc.Run.
		raw, ok := n.(phpv.Runnable)
		if !ok {
			return unsupportedf("static call: cannot retrieve raw Runnable")
		}
		stmtCtx := e.stmtCtx
		idx := e.astIndex(raw)
		e.emit(vm.OpStaticMethodCall, idx, 0, 0)
		e.pushStack(1)
		if stmtCtx {
			e.emit(vm.OpPop, 0, 0, 0)
			e.popStack(1)
		}
		e.emit(vm.OpRefreshSlots, 0, 0, 0)
		return nil
	}
	name := n.ObjectFuncName()
	args := n.ObjectFuncArgs()
	// `$obj->$x(...)` (no curly braces) parses to runObjectFunc with
	// the literal dollar-prefixed token as the method name. That form
	// needs runtime variable lookup, which the native call path doesn't
	// do — AST-delegate via OpClassConst / OpTryFinally. Curly-brace
	// form `$obj->{$x}(...)` parses to runObjectDynFunc and is
	// intercepted by IsObjectDynFuncNode earlier.
	if len(name) > 0 && name[0] == '$' {
		raw, ok := n.(phpv.Runnable)
		if !ok {
			return unsupportedf("method-call AST delegation: cannot retrieve raw Runnable")
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
	// Special-args / writable-arg calls route through the by-exprs
	// opcode so ctx.Call sees raw arg expressions and binds by-ref
	// params correctly. Nullsafe is encoded via the C flag.
	if compiler.CallHasSpecialArgs(args) || compiler.CallHasWritableArg(args) {
		return e.emitObjectCallByExprs(n.ObjectFuncReceiver(), name, args, n.ObjectFuncIsNullSafe())
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
	var cFlag int32
	if n.ObjectFuncIsNullSafe() {
		cFlag = 1
	}
	e.emit(vm.OpObjectCall, nameIdx, uint16(len(args)), cFlag)
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

// emitObjectCallByExprs lowers $obj->method(args…) when the args need
// the full ctx.Call binding pipeline (by-ref params, named arguments,
// spread). Receiver is evaluated to the stack; method name and arg-
// expression list are stashed in fn.Consts / fn.SubArgs respectively.
func (e *emitter) emitObjectCallByExprs(recvExpr phpv.Runnable, name phpv.ZString, args []phpv.Runnable, nullsafe bool) error {
	if len(args) > 0xFFFF {
		return unsupportedf("by-exprs method call with too many args (>=65536)")
	}
	if err := e.withSubexpr(func() error { return e.emitExpr(recvExpr) }); err != nil {
		return err
	}
	nameIdx := e.constIndex(name)
	argsIdx := e.subArgsIndex(args)
	var cFlag int32
	if nullsafe {
		cFlag = 1
	}
	e.emit(vm.OpObjectCallByExprs, nameIdx, argsIdx, cFlag)
	// Pops 1 receiver, pushes 1 result. Net stack delta: 0.
	e.emit(vm.OpRefreshSlots, 0, 0, 0)
	if e.stmtCtx {
		e.emit(vm.OpPop, 0, 0, 0)
		e.popStack(1)
	}
	return nil
}
