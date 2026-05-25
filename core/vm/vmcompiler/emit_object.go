package vmcompiler

import (
	"github.com/KarpelesLab/goro/core/compiler"
	"github.com/KarpelesLab/goro/core/phpv"
	"github.com/KarpelesLab/goro/core/tokenizer"
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
	if lhs.ObjectVarIsNullSafe() {
		return unsupportedf("nullsafe property assign")
	}
	name := lhs.ObjectVarName()
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
	// `$obj->$x = v` has varName prefixed with `$`. Push the local
	// holding the dyn name on the stack and use OP_OBJECT_DYN_SET.
	if len(name) > 0 && name[0] == '$' {
		localIdx := e.localIndex(name[1:])
		e.emit(vm.OpLoadLocal, localIdx, 0, 0)
		e.pushStack(1)
		if err := e.withSubexpr(func() error { return e.emitExpr(rhs) }); err != nil {
			return err
		}
		var a uint16
		if !stmtCtx {
			a |= 1 // keep value on stack
		}
		e.emit(vm.OpObjectDynSet, a, 0, 0)
		if stmtCtx {
			e.popStack(3) // pop receiver+name+value, push nothing
		} else {
			e.popStack(2) // pop receiver+name+value, push value back → net -2
		}
		return nil
	}
	if err := e.withSubexpr(func() error { return e.emitExpr(rhs) }); err != nil {
		return err
	}
	idx := e.constIndex(name)
	if stmtCtx {
		e.emit(vm.OpObjectSet, idx, 0, 0)
		e.popStack(2) // pop receiver + value, no push
	} else {
		// Expr context: OpObjectSet B=1 leaves the value on the stack
		// after the write — semantics of `$x = ($obj->prop = v)`.
		e.emit(vm.OpObjectSet, idx, 1, 0)
		e.popStack(1) // pop receiver+value, push value back → net -1
	}
	return nil
}

// emitObjectDynVarAssign emits `$obj->{$x} = v` natively. Receiver and
// name are evaluated and pushed; OP_OBJECT_DYN_SET dispatches to
// ZObject.ObjectSet (typed props, hooks, asymmetric visibility).
//
// Note: unlike runObjectVar.WriteValue, the runObjectDynVar AST runner
// does NOT call SetWriteContext on its receiver. An undefined simple-
// variable receiver still emits the "Undefined variable" warning. Use
// OP_LOAD_LOCAL_OR_WARN here to match AST semantics — the dyn form
// behaves like a value read on the receiver, then a write on the
// resolved property.
func (e *emitter) emitObjectDynVarAssign(lhs objectDynVarReadNode, rhs phpv.Runnable, stmtCtx bool) error {
	recv := lhs.ObjectDynVarReceiver()
	if err := e.withSubexpr(func() error { return e.emitExpr(recv) }); err != nil {
		return err
	}
	if err := e.withSubexpr(func() error { return e.emitExpr(lhs.ObjectDynVarNameExpr()) }); err != nil {
		return err
	}
	if err := e.withSubexpr(func() error { return e.emitExpr(rhs) }); err != nil {
		return err
	}
	var a uint16
	if !stmtCtx {
		a |= 1
	}
	e.emit(vm.OpObjectDynSet, a, 0, 0)
	if stmtCtx {
		e.popStack(3)
	} else {
		e.popStack(2)
	}
	return nil
}

// emitObjectVarCompoundAssign emits `$obj->prop OP= rhs` for static-name,
// non-nullsafe property access. The receiver is evaluated once; the
// resulting value handle is reused for both objectGet (read current) and
// objectSet (write back). The PHP semantics-level dance — typed prop
// coercion, hook dispatch, asymmetric visibility — happens inside
// ZObject.ObjectSet, same as for plain `=`.
func (e *emitter) emitObjectVarCompoundAssign(lhs objectVarNode, rhs phpv.Runnable, op tokenizer.ItemType, stmtCtx bool) error {
	if lhs.ObjectVarIsNullSafe() {
		return unsupportedf("nullsafe property compound assign")
	}
	name := lhs.ObjectVarName()
	if len(name) > 0 && name[0] == '$' {
		return unsupportedf("dynamic property name in compound assign")
	}
	recv := lhs.ObjectVarReceiver()
	// Receiver write-context: an undefined simple-variable receiver
	// must not warn. Mirror emitObjectVarAssign's silent load path.
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
	nameIdx := e.constIndex(name)
	var c int32
	if !stmtCtx {
		c = 1 // keep post-op value on stack
	}
	e.emit(vm.OpObjectCompoundAssign, nameIdx, uint16(op), c)
	if stmtCtx {
		e.popStack(2) // pop receiver + rhs, nothing pushed
	} else {
		e.popStack(1) // pop receiver+rhs, push result → net -1
	}
	return nil
}

// emitObjectVarIncDec emits `$obj->prop++` / `++$obj->prop` and dec
// variants for static-name, non-nullsafe property access. Evaluates the
// receiver once, then OP_INC_DEC_OBJ_PROP handles the read-mutate-write
// cycle. The pre/post-mutation value lands on the stack only in
// expression context.
func (e *emitter) emitObjectVarIncDec(lhs objectVarNode, inc bool, post bool, stmtCtx bool) error {
	if lhs.ObjectVarIsNullSafe() {
		return unsupportedf("nullsafe property inc/dec")
	}
	name := lhs.ObjectVarName()
	if len(name) > 0 && name[0] == '$' {
		return unsupportedf("dynamic property name in inc/dec")
	}
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
	nameIdx := e.constIndex(name)
	var b uint16
	if inc {
		b |= 1
	}
	if post {
		b |= 2
	}
	var c int32
	if !stmtCtx {
		c = 1
	}
	e.emit(vm.OpIncDecObjProp, nameIdx, b, c)
	if stmtCtx {
		e.popStack(1) // receiver consumed, nothing pushed
	}
	// expr-context: receiver consumed, pre/post value pushed → net 0
	return nil
}

// emitObjectDynVarCompoundAssign emits `$obj->$x OP= rhs` and
// `$obj->{$x} OP= rhs` for dynamic-name property access. Receiver and
// name are evaluated and pushed; OP_OBJECT_DYN_COMPOUND_ASSIGN reads
// the current value via objectGet, applies the compound op, and writes
// back via objectSet (typed props, hooks, asymmetric visibility).
//
// `recvSilent` mirrors the dollar-prefix shape semantics: an undefined
// simple-variable receiver does not warn (write-context). The curly-
// brace form (`*runObjectDynVar`) does NOT suppress the warning, so it
// uses the default emitExpr path.
func (e *emitter) emitObjectDynVarCompoundAssign(recv phpv.Runnable, pushName func() error, rhs phpv.Runnable, op tokenizer.ItemType, recvSilent bool, stmtCtx bool) error {
	if recvSilent {
		if v, ok := recv.(variableNode); ok {
			idx := e.localIndex(v.VariableName())
			e.emit(vm.OpLoadLocal, idx, 0, 0)
			e.pushStack(1)
		} else {
			if err := e.withSubexpr(func() error { return e.emitExpr(recv) }); err != nil {
				return err
			}
		}
	} else {
		if err := e.withSubexpr(func() error { return e.emitExpr(recv) }); err != nil {
			return err
		}
	}
	if err := e.withSubexpr(pushName); err != nil {
		return err
	}
	if err := e.withSubexpr(func() error { return e.emitExpr(rhs) }); err != nil {
		return err
	}
	var c int32
	if !stmtCtx {
		c = 1
	}
	e.emit(vm.OpObjectDynCompoundAssign, 0, uint16(op), c)
	if stmtCtx {
		e.popStack(3) // pop receiver+name+rhs
	} else {
		e.popStack(2) // pop receiver+name+rhs, push result → net -2
	}
	return nil
}

// emitObjectDynVarIncDec emits `$obj->$x++` / `++$obj->{$x}` and dec
// variants. Receiver and name are evaluated and pushed; OP_INC_DEC_OBJ_DYN_PROP
// performs read-mutate-write through objectGet/objectSet.
func (e *emitter) emitObjectDynVarIncDec(recv phpv.Runnable, pushName func() error, inc bool, post bool, recvSilent bool, stmtCtx bool) error {
	if recvSilent {
		if v, ok := recv.(variableNode); ok {
			idx := e.localIndex(v.VariableName())
			e.emit(vm.OpLoadLocal, idx, 0, 0)
			e.pushStack(1)
		} else {
			if err := e.withSubexpr(func() error { return e.emitExpr(recv) }); err != nil {
				return err
			}
		}
	} else {
		if err := e.withSubexpr(func() error { return e.emitExpr(recv) }); err != nil {
			return err
		}
	}
	if err := e.withSubexpr(pushName); err != nil {
		return err
	}
	var b uint16
	if inc {
		b |= 1
	}
	if post {
		b |= 2
	}
	var c int32
	if !stmtCtx {
		c = 1
	}
	e.emit(vm.OpIncDecObjDynProp, 0, b, c)
	if stmtCtx {
		e.popStack(2) // receiver+name consumed, nothing pushed
	} else {
		e.popStack(1) // receiver+name consumed, pre/post pushed → net -1
	}
	return nil
}

func (e *emitter) emitObjectVarRead(n objectVarNode) error {
	name := n.ObjectVarName()
	// `$obj->$x` (no curly braces) parses to runObjectVar with the
	// literal dollar-prefixed token as the name (e.g. "$x"). Strip the
	// `$` and route through the existing OP_OBJECT_DYN_GET pipeline:
	// receiver + local-read of the name variable on the stack. This is
	// semantically identical to the curly-brace form `$obj->{$x}` once
	// the local has been evaluated. Same nullsafe encoding via A bit 0.
	if len(name) > 0 && name[0] == '$' {
		if err := e.withSubexpr(func() error { return e.emitExpr(n.ObjectVarReceiver()) }); err != nil {
			return err
		}
		localIdx := e.localIndex(name[1:])
		e.emit(vm.OpLoadLocal, localIdx, 0, 0)
		e.pushStack(1)
		var aFlags uint16
		if n.ObjectVarIsNullSafe() {
			aFlags |= 1
		}
		e.emit(vm.OpObjectDynGet, aFlags, 0, 0)
		e.popStack(1) // pops 2, pushes 1 → net -1
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
