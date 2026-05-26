package compiler

import (
	"fmt"

	"github.com/KarpelesLab/goro/core/logopt"
	"github.com/KarpelesLab/goro/core/phpobj"
	"github.com/KarpelesLab/goro/core/phpv"
	"github.com/KarpelesLab/goro/core/tokenizer"
)

// ResolveCallable converts a runtime value to a phpv.Callable suitable
// for ctx.CallZVal. Handles:
//   - ZObject with __invoke handler (closures, invokable classes)
//   - ZString (function name)
//   - ZArray of [object, method] or [class, method]
//
// The second return value is the implicit `$this` (z.this for
// closures, the array's [0] for object-method array callables, nil
// otherwise) — pass it as the optionalThis argument to CallZVal so
// the callee's body sees the right binding.
//
// Returns nil + error otherwise.
func ResolveCallable(ctx phpv.Context, v *phpv.ZVal) (phpv.Callable, phpv.ZObject, error) {
	switch v.GetType() {
	case phpv.ZtObject:
		obj, ok := v.Value().(*phpobj.ZObject)
		if !ok {
			return nil, nil, fmt.Errorf("ResolveCallable: object is not a *ZObject")
		}
		if h := obj.GetClass().Handlers(); h != nil && h.HandleInvoke != nil {
			// Closure / invokable: extract the underlying ZClosure.
			if op := obj.GetOpaque(Closure); op != nil {
				if zc, ok := op.(phpv.Callable); ok {
					// Closures carry their bound $this; expose it
					// through GetThis if available.
					if g, ok := zc.(interface{ GetThis() phpv.ZObject }); ok {
						return zc, g.GetThis(), nil
					}
					return zc, nil, nil
				}
			}
		}
		// Fallback: treat as object with __invoke method.
		if m, ok := obj.GetClass().GetMethod("__invoke"); ok {
			return m.Method, obj, nil
		}
		return nil, nil, phpobj.ThrowError(ctx, phpobj.Error,
			fmt.Sprintf("Object of type %s is not callable", obj.GetClass().GetName()))
	case phpv.ZtString:
		name := v.Value().(phpv.ZString)
		f, err := ctx.Global().GetFunction(ctx, name)
		if err != nil {
			return nil, nil, err
		}
		return f, nil, nil
	case phpv.ZtArray:
		// [object|className, methodName]
		arr := v.AsArray(ctx)
		recv, _ := arr.OffsetGet(ctx, phpv.ZInt(0))
		methodVal, _ := arr.OffsetGet(ctx, phpv.ZInt(1))
		if recv == nil || methodVal == nil {
			return nil, nil, phpobj.ThrowError(ctx, phpobj.TypeError, "Array callable expects [object|class, method]")
		}
		methodName := methodVal.AsString(ctx)
		switch recv.GetType() {
		case phpv.ZtObject:
			obj, ok := recv.Value().(*phpobj.ZObject)
			if !ok {
				return nil, nil, fmt.Errorf("ResolveCallable: array receiver is not *ZObject")
			}
			m, ok := obj.GetClass().GetMethod(methodName)
			if !ok {
				return nil, nil, phpobj.ThrowError(ctx, phpobj.Error,
					fmt.Sprintf("Call to undefined method %s::%s()", obj.GetClass().GetName(), methodName))
			}
			return m.Method, obj, nil
		case phpv.ZtString:
			className := recv.AsString(ctx)
			class, err := ctx.Global().GetClass(ctx, className, false)
			if err != nil {
				return nil, nil, err
			}
			m, ok := class.GetMethod(methodName)
			if !ok {
				return nil, nil, phpobj.ThrowError(ctx, phpobj.Error,
					fmt.Sprintf("Call to undefined method %s::%s()", className, methodName))
			}
			return m.Method, nil, nil
		}
		return nil, nil, phpobj.ThrowError(ctx, phpobj.TypeError, "Array callable receiver must be object or class name")
	default:
		return nil, nil, phpobj.ThrowError(ctx, phpobj.TypeError,
			fmt.Sprintf("Value of type %s is not callable", v.GetType().TypeName()))
	}
}

// IsSlotSafe reports whether the function body in r is safe for the
// VM's slot-only optimization — i.e. local writes can skip mirroring
// to the FuncContext hashtable.
//
// A body is unsafe when it (transitively) does any of:
//   - Calls a builtin that reads or writes locals through the
//     FuncContext (extract, compact, get_defined_vars, parse_str,
//     mb_parse_str, func_get_args, func_num_args, func_get_arg).
//   - Uses a variable-variable expression ($$x, runVariableRef).
//   - References $GLOBALS (the array-of-globals magic; reads / writes
//     go through the hashtable).
//   - Declares a `global $x` or `static $x = …` (both bind locals to
//     storage that lives outside the slot array).
//   - Declares a function (`function foo() { … }`) — that function's
//     body may use `global $x` to bind to a top-level local.
func IsSlotSafe(g phpv.GlobalContext, r phpv.Runnable) bool {
	if r == nil {
		return true
	}
	switch n := r.(type) {
	case *runVariableRef:
		return false
	case *runVariable:
		if n.v == "GLOBALS" {
			return false
		}
	case *runnableFunctionCall:
		if slotUnsafeFuncs[n.name.ToLower()] {
			return false
		}
		// Calls with named/spread arguments, by-ref builtins, or
		// by-ref user-function targets are AST-delegated; the AST
		// runner reads args from the hashtable.
		if CallHasSpecialArgs(n.args) {
			return false
		}
		if ByRefBuiltins[n.name.ToLower()] {
			return false
		}
		// User-function calls whose callee has by-ref params:
		// AST-delegated at the call site, reads args from hashtable.
		// Note: unknown callees aren't flagged here even when args
		// are writable; the emit-side falls back via the same gate,
		// and forcing slot-unsafety on every function-call body
		// regressed many independent tests.
		hit, byRef := LookupLazyByRefSafe(g, n.name)
		if hit && byRef {
			return false
		}
	case *runnableFunctionCallRef:
		// Indirect calls AST-delegate when args include named/spread
		// or a writable lvalue (in case the resolved callable takes
		// the writable by ref). Body must be slot-unsafe so the
		// AST reads local args from the hashtable.
		if CallHasSpecialArgs(n.args) || CallHasWritableArg(n.args) {
			return false
		}
	case *runStaticVar:
		return false
	case *runGlobal:
		return false
	case *runnableTry:
		// try / try+catch / try+finally are lowered natively by the
		// emitter. The recursive walk over GetChildren still picks up
		// any slot-unsafe construct inside the try body, catches, or
		// finally body, so no extra gate is needed here.
	case *runOperator:
		// Assignment / compound-assign / inc-dec to a property,
		// static prop, or array element (when the LHS isn't a
		// simple-local container) is AST-delegated; the AST
		// reads/writes locals through ctx.OffsetSet, so the
		// hashtable must be authoritative.
		isWrite := n.opD != nil && n.opD.write
		isIncDec := n.op == tokenizer.T_INC || n.op == tokenizer.T_DEC
		// Compound assign (anything that's a write op + has an
		// underlying op, e.g. +=, .=) is AST-delegated by the
		// emitter when the LHS is a non-variable; for array-element
		// LHS it's always AST-delegated regardless of container.
		isCompound := isWrite && n.opD.op != nil
		if isWrite || isIncDec {
			target := n.a
			if target == nil {
				target = n.b // prefix ++/-- carries the target on b
			}
			switch t := target.(type) {
			case *runObjectVar:
				// Plain `=`, compound (+=, .=), and inc/dec on a
				// non-nullsafe object property now all emit natively
				// (OP_OBJECT_SET / OP_OBJECT_COMPOUND_ASSIGN /
				// OP_INC_DEC_OBJ_PROP, with dyn-name variants too).
				// Nullsafe in write context is parse-rejected, so any
				// nullsafe LHS here would already have failed earlier.
				if t.nullsafe {
					return false
				}
				// Fall through — native emit covers static-name and
				// dollar-prefix dyn-name shapes (OP_OBJECT_DYN_SET /
				// OP_OBJECT_DYN_COMPOUND_ASSIGN / OP_INC_DEC_OBJ_DYN_PROP).
			case *runClassStaticVarRef:
				// Plain `=`, compound, and inc/dec all emit natively via
				// OP_STATIC_PROP_SET / OP_STATIC_PROP_COMPOUND_ASSIGN /
				// OP_INC_DEC_STATIC_PROP. None touch caller locals.
				_ = t
			case *runClassStaticDynVarRef:
				// Dyn-name static prop write/compound/incdec all emit
				// natively (OP_STATIC_PROP_DYN_SET /
				// OP_STATIC_PROP_DYN_COMPOUND_ASSIGN /
				// OP_INC_DEC_STATIC_PROP_DYN). None touch caller locals.
			case *runDestructure, *runVariableRef:
				return false
			case *runArrayAccess:
				// $local[k] OP= rhs and $local[k]++/-- now emit natively
				// via OP_ARRAY_COMPOUND_ASSIGN_LOCAL /
				// OP_ARRAY_INC_DEC_LOCAL when the container is a simple
				// variable. Nested-container shapes (`$obj->arr[i] += v`,
				// `$a[i][j]++`, …) still AST-delegate.
				if _, ok := t.value.(*runVariable); !ok {
					return false
				}
				// Append form `$local[] OP= v` is invalid syntax (parse
				// rejects). Plain `$local[] = v` allowed by emit when
				// offset == nil; compound never has nil offset.
				if (isCompound || isIncDec) && t.offset == nil {
					return false
				}
			}
		}
		// Coalesce (??) with a non-local LHS is AST-delegated; the
		// AST suppresses undefined-index/property warnings during
		// the LHS read.
		if n.op == tokenizer.T_COALESCE && n.a != nil {
			if _, ok := n.a.(*runVariable); !ok {
				return false
			}
		}
	case *runObjectFunc:
		// Static-style calls (Foo::method, parent::, self::) are
		// AST-delegated; their arg evaluation reads caller locals
		// via the hashtable.
		if n.static {
			return false
		}
		// Nullsafe/dynamic-name/special-args method calls are also
		// AST-delegated; same hashtable requirement.
		if n.nullsafe || n.nullChain {
			return false
		}
		if len(n.op) > 0 && n.op[0] == '$' {
			return false
		}
		if CallHasSpecialArgs(n.args) {
			return false
		}
		// Writable-arg method calls are conservatively AST-delegated
		// so the resolved method's by-ref params (if any) bind
		// through the AST's CallZVal path.
		if CallHasWritableArg(n.args) {
			return false
		}
	case *runObjectVar:
		// Nullsafe / dynamic-name property reads are AST-delegated.
		// `writeContext` paths are already caught by the runOperator
		// case above (LHS *runObjectVar in a write op).
		if n.nullsafe || n.nullChain {
			return false
		}
		if len(n.varName) > 0 && n.varName[0] == '$' {
			return false
		}
	case *runNewObject:
		// AST-delegated when the class is anonymous, the name is
		// dynamic, or the args list has named/spread wrappers.
		if n.cl != nil || n.obj == "" || CallHasSpecialArgs(n.newArg) {
			return false
		}
	case *runNewAnonymousClass:
		// `new class { … }` registers the class and constructs an
		// instance; constructor args read locals via the hashtable.
		return false
	// match (…) { … } and switch (…) { … } are now lowered natively
	// (`e0f1951a` and the follow-on switch lowering) so they no longer
	// force slot-unsafe.
	case *runDestructure:
		// `[$a, $b] = $arr` and `list(...)` write to multiple
		// targets through ctx.OffsetSet; the hashtable must be
		// authoritative.
		return false
	// `#[NoDiscard]` wraps are native-emitted (OP_NODISCARD_ENTER/EXIT
	// bracket); the body's slot-safety follows from the inner stmt's
	// own analysis.
	case *runClassStaticVarRef:
		// Class-level constant / static-prop / static-name access
		// (`Foo::{$x}`, `$obj::CONST`, `Foo::$bar`, etc.) is
		// AST-delegated via OpClassConst. The AST runner reads any
		// local operands from the FuncContext hashtable.
		return false
	case *runReturn:
		// `return $expr` is AST-delegated when the surrounding
		// function declares a return type hint (the AST does the
		// type coercion). The delegated AST reads `$expr` from
		// the FuncContext hashtable, so the body must be slot-
		// unsafe to keep the hashtable mirrored.
		if n.returnType != nil {
			return false
		}
	case *runnableClone:
		// Basic `clone $x` is native and slot-safe by itself; only
		// the extended forms (clone with withProperties, spread, named
		// args) still AST-delegate and need the hashtable mirrored.
		if !n.CloneIsBasic() {
			return false
		}
	case *runObjectDynVar,
		*runObjectDynFunc,
		*runRef,
		*runEnumRegister:
		// All AST-delegated — operands evaluated by AST through
		// the FuncContext hashtable.
		return false
	case *runnableForeach:
		// foreach-by-ref or with non-simple-local key/value targets
		// is AST-delegated; the loop body reads caller locals via
		// the hashtable.
		if n.ref {
			return false
		}
		if _, ok := n.v.(*runVariable); !ok {
			return false
		}
		if n.k != nil {
			if _, ok := n.k.(*runVariable); !ok {
				return false
			}
		}
	case *runnableUnset:
		// Simple-local and array-access-on-simple-local unsets lower
		// natively. Other target shapes (object prop, static prop,
		// nested array access) still AST-delegate.
		if !UnsetAllSupported(r) {
			return false
		}
	case *runnableIsset, *runnableEmpty:
		// All-simple-local AND array-access-on-simple-local isset/empty
		// are now lowered natively. Only the complex shapes (object-
		// prop, dyn-name, class-static, etc.) still AST-delegate and
		// need the hashtable populated.
		if !IssetEmptyAllSupported(r) {
			return false
		}
	case *ZClosure:
		// A *named* ZClosure at expression level is a function
		// declaration that registers a global symbol; functions
		// declared like that may use `global $x` to bind to the
		// surrounding scope's locals, so the surrounding scope can't
		// be slot-only. Anonymous closures (name == "") only produce
		// a value and don't expose the surrounding locals to other
		// callers, so they're safe.
		if n.name != "" {
			return false
		}
		// For anonymous closures, any `use` capture means the
		// surrounding function's locals must be visible via the
		// FuncContext hashtable at closure-creation time (the AST
		// uses ctx.OffsetCheck / ctx.OffsetGet to fetch them).
		// SlotOnly skips the hashtable mirror, so the captures
		// would silently be null. Mark the surrounding scope unsafe
		// when the closure has any use captures (by-ref or by-value).
		if len(n.use) > 0 {
			return false
		}
		// Body is checked separately when the closure itself is VM-
		// compiled. From the surrounding function's perspective, the
		// closure body is an opaque value. Stop recursing.
		return true
	}
	for _, c := range GetChildren(r) {
		if !IsSlotSafe(g, c) {
			return false
		}
	}
	return true
}

// FunctionTakesByRef reports whether the named function (in the
// current global scope) declares any by-reference parameter. Used by
// the VM emitter to fall back to AST for calls that need by-ref
// binding (which the VM's value-passing call protocol can't provide).
//
// This intentionally does NOT trigger lazy-function resolution: that
// would Run() the registration Runnable as a side effect of an
// emit-time query, which conflicts with the VM later emitting an
// OP_MAKE_CLOSURE for the same declaration. Instead we inspect
// (a) already-registered user/internal functions, and
// (b) lazy entries via a separate LookupLazyByRef hook installed by
//     core/phpctx.
//
// Returns false when the function isn't registered or known yet —
// pessimistic users should fall back conservatively.
func FunctionTakesByRef(g phpv.GlobalContext, name phpv.ZString) bool {
	// Try the lazy-aware probe first (set by core/phpctx).
	if LookupLazyByRef != nil {
		if hit, byRef := LookupLazyByRef(g, name); hit {
			return byRef
		}
	}
	return false
}

// LookupLazyByRef is an optional hook installed by core/phpctx that
// looks up a function (including the lazy table) and reports whether
// any of its parameters are by-reference. hit=true means "I know about
// this function"; byRef=true means "yes, it has by-ref params". The
// hook never triggers a registration side effect.
var LookupLazyByRef func(g phpv.GlobalContext, name phpv.ZString) (hit, byRef bool)

// LookupLazyByRefSafe wraps LookupLazyByRef so callers don't have to
// check for nil and missing globals. Returns (false, false) when the
// hook is unset or the global isn't a *phpctx.Global.
func LookupLazyByRefSafe(g phpv.GlobalContext, name phpv.ZString) (hit, byRef bool) {
	if LookupLazyByRef == nil || g == nil {
		return false, false
	}
	return LookupLazyByRef(g, name)
}

// ByRefBuiltins lists builtin functions that take at least one
// argument by reference. The VM emitter falls back to AST for calls
// to any of these because the VM's value-passing call protocol
// can't bind a Writable argument.
//
// This static list is a fallback for emit-time queries that don't have
// a *phpctx.Global available (some test fixtures, early compile-time
// hooks). With a Global, `LookupByRef` introspects each callable's
// ExtFunctionArg metadata directly and catches every by-ref signature
// without needing this list. Keep entries in sync as new by-ref
// builtins are added; missing entries don't break runtime behaviour
// (the dynamic probe covers them) but degrade gracefully when no
// Global is wired in.
var ByRefBuiltins = map[phpv.ZString]bool{
	// Array internal-pointer mutators
	"end": true, "reset": true, "next": true, "prev": true,
	"current": true, "key": true, "each": true,
	// Array sort / shuffle (mutate in place)
	"sort": true, "rsort": true, "asort": true, "arsort": true,
	"ksort": true, "krsort": true, "usort": true, "uasort": true, "uksort": true,
	"natsort": true, "natcasesort": true, "shuffle": true,
	// Array structural ops
	"array_walk": true, "array_walk_recursive": true,
	"array_push": true, "array_pop": true,
	"array_shift": true, "array_unshift": true,
	"array_splice": true, "array_multisort": true,
	// Misc / IO / process
	"settype": true, "parse_str": true, "mb_parse_str": true,
	"mb_convert_variables": true,
	"extract":               true,
	"exec":                  true, "passthru": true, "system": true,
	"flock":   true,
	"fscanf":  true,
	"sscanf":  true,
	"is_callable":            true,
	"getimagesize":           true, "getimagesizefromstring": true,
	"similar_text":           true,
	"str_replace":            true, "str_ireplace": true,
	"openssl_random_pseudo_bytes": true,
	// preg_match / sscanf write capture-groups via &$matches / args
	"preg_match":                  true,
	"preg_match_all":              true,
	"preg_replace":                true,
	"preg_replace_callback":       true,
	"preg_replace_callback_array": true,
}

var slotUnsafeFuncs = map[phpv.ZString]bool{
	"extract":          true,
	"compact":          true,
	"get_defined_vars": true,
	"func_get_args":    true,
	"func_num_args":    true,
	"func_get_arg":     true,
	"parse_str":        true, // can write callee locals when called with no second arg
	"mb_parse_str":     true,
	// include/require/eval all execute code in the caller's scope and
	// reach for caller locals through the FuncContext hashtable. Mark
	// the surrounding body slot-unsafe so writes mirror to the hashtable
	// before the include/eval runs.
	"include":      true,
	"include_once": true,
	"require":      true,
	"require_once": true,
	"eval":         true,
}

// CallInstanceMethod invokes obj->name(args...) with full PHP
// dispatch semantics: abstract-method check, private/protected
// visibility against the calling class, __call fallback when
// inaccessible, late static binding. Used by the VM's OP_OBJECT_CALL
// so behaviour matches the AST runObjectFunc path byte-for-byte.
//
// The receiver must be a ZtObject ZVal; the caller is responsible
// for the upstream "method on null/scalar" diagnostic.
func CallInstanceMethod(ctx phpv.Context, obj *phpv.ZVal, name phpv.ZString, args []*phpv.ZVal) (*phpv.ZVal, error) {
	zobj, ok := obj.Value().(*phpobj.ZObject)
	if !ok {
		// Fall through to the interface form (rare, e.g. some internal
		// proxy types). It can't do full visibility checks, but at
		// least keeps the simple cases working.
		if zo, ok := obj.Value().(phpv.ZObject); ok {
			m, mok := zo.GetClass().GetMethod(name)
			if !mok {
				return nil, phpobj.ThrowError(ctx, phpobj.Error,
					fmt.Sprintf("Call to undefined method %s::%s()", zo.GetClass().GetName(), name))
			}
			return ctx.CallZVal(ctx, m.Method, args, zo)
		}
		return nil, fmt.Errorf("CallInstanceMethod: receiver is not a ZObject")
	}

	class := zobj.GetClass()
	method, ok := class.GetMethod(name)
	if !ok {
		// __call magic
		if cm, hasCall := class.GetMethod("__call"); hasCall {
			a := phpv.NewZArray()
			for _, sub := range args {
				a.OffsetSet(ctx, nil, sub.Dup())
			}
			return ctx.CallZVal(ctx, cm.Method, []*phpv.ZVal{name.ZVal(), a.ZVal()}, zobj)
		}
		return nil, phpobj.ThrowError(ctx, phpobj.Error,
			fmt.Sprintf("Call to undefined method %s::%s()", class.GetName(), name))
	}

	// Abstract method cannot be called directly.
	if method.Modifiers.Has(phpv.ZAttrAbstract) || (method.Empty && method.Class != nil && method.Class.GetType() != phpv.ZClassTypeInterface) {
		return nil, phpobj.ThrowError(ctx, phpobj.Error,
			fmt.Sprintf("Cannot call abstract method %s::%s()", method.Class.GetName(), method.Name))
	}

	// Visibility check.
	methodNotVisible := false
	var visErrMsg string
	if method.Modifiers.Has(phpv.ZAttrPrivate) {
		callerClass := ctx.Class()
		methodClass := method.Class
		if callerClass == nil || methodClass == nil || callerClass.GetName() != methodClass.GetName() {
			methodClassName := class.GetName()
			if methodClass != nil {
				methodClassName = methodClass.GetName()
			}
			scope := "global scope"
			if callerClass != nil {
				scope = "scope " + string(callerClass.GetName())
			}
			methodNotVisible = true
			visErrMsg = fmt.Sprintf("Call to private method %s::%s() from %s", methodClassName, method.Name, scope)
		}
	} else if method.Modifiers.Has(phpv.ZAttrProtected) {
		callerClass := ctx.Class()
		if callerClass == nil {
			methodNotVisible = true
			visErrMsg = fmt.Sprintf("Call to protected method %s::%s() from global scope", class.GetName(), method.Name)
		} else if !callerClass.InstanceOf(method.Class) && !method.Class.InstanceOf(callerClass) && !callerClass.InstanceOf(class) && !class.InstanceOf(callerClass) {
			// Sibling-class case: walk up to find common protected ancestor.
			protectedVisible := false
			if method.Class != nil {
				rootClass := method.Class
				for rootClass.GetParent() != nil {
					if pm, ok := rootClass.GetParent().GetMethod(method.Name); ok && pm.Modifiers.Has(phpv.ZAttrProtected) {
						rootClass = rootClass.GetParent()
					} else {
						break
					}
				}
				if callerClass.InstanceOf(rootClass) {
					protectedVisible = true
				}
			}
			if !protectedVisible {
				methodNotVisible = true
				visErrMsg = fmt.Sprintf("Call to protected method %s::%s() from scope %s", class.GetName(), method.Name, callerClass.GetName())
			}
		}
	}
	if methodNotVisible {
		// __call fallback
		if cm, hasCall := class.GetMethod("__call"); hasCall {
			a := phpv.NewZArray()
			for _, sub := range args {
				a.OffsetSet(ctx, nil, sub.Dup())
			}
			return ctx.CallZVal(ctx, cm.Method, []*phpv.ZVal{name.ZVal(), a.ZVal()}, zobj)
		}
		return nil, phpobj.ThrowError(ctx, phpobj.Error, visErrMsg)
	}

	// For static methods, $this is not passed; bind for late-static.
	if method.Modifiers.IsStatic() {
		m := phpv.BindClassLSB(method.Method, class, class, true)
		m.Attributes = method.Attributes
		return ctx.CallZVal(ctx, m, args, nil)
	}

	// Non-static instance methods: narrow $this to the defining class
	// for parent::-style dispatch, then invoke directly.
	var objBound phpv.ZObject = zobj
	if method.Class != nil {
		if kin := zobj.GetKin(string(method.Class.GetName())); kin != nil {
			objBound = kin
		}
	}
	// Wrap native methods that carry attributes (e.g. NoDiscard on
	// DateTimeImmutable::setTimestamp) in MethodCallable so the call
	// dispatch can find them via AttributeGetter. User-defined
	// methods are already AttributeGetters via their underlying
	// ZClosure, so this only matters for builtin methods.
	if len(method.Attributes) > 0 {
		// AliasName carries the method name into NoDiscard messages
		// (NativeMethod.Name() is empty; the .Name field on the
		// ZClassMethod record holds the actual name).
		wrapped := &phpv.MethodCallable{
			Callable:   method.Method,
			Class:      class,
			Attributes: method.Attributes,
			AliasName:  string(method.Name),
		}
		return ctx.CallZVal(ctx, wrapped, args, objBound)
	}
	return ctx.CallZVal(ctx, method.Method, args, objBound)
}

// CallInstanceMethodByExprs is the by-AST-expression variant of
// CallInstanceMethod. Used by OP_OBJECT_CALL_BY_EXPRS for method calls
// whose argument shape needs the full ctx.Call binding pipeline
// (by-ref parameters, named arguments, spread). The dispatch logic
// mirrors CallInstanceMethod byte-for-byte; only the terminal call
// switches from ctx.CallZVal to ctx.Call so the binding layer sees the
// raw argument expressions.
//
// The __call magic-method fallback still needs ZVal args (it builds
// a magic array), so this evaluates exprs up-front for that branch
// only — by-ref binding doesn't apply through __call anyway, since
// the user wrote `__call($name, $args)` with the args array.
func CallInstanceMethodByExprs(ctx phpv.Context, obj *phpv.ZVal, name phpv.ZString, exprs []phpv.Runnable) (*phpv.ZVal, error) {
	rawZobj, ok := obj.Value().(*phpobj.ZObject)
	if !ok {
		if zo, ok := obj.Value().(phpv.ZObject); ok {
			m, mok := zo.GetClass().GetMethod(name)
			if !mok {
				return nil, phpobj.ThrowError(ctx, phpobj.Error,
					fmt.Sprintf("Call to undefined method %s::%s()", zo.GetClass().GetName(), name))
			}
			return ctx.Call(ctx, m.Method, exprs, zo)
		}
		return nil, fmt.Errorf("CallInstanceMethodByExprs: receiver is not a ZObject")
	}

	// Unwrap any kin marker so method lookup sees the actual class of
	// the object — not the narrowed parent class that a $this kin would
	// expose. This mirrors runObjectFunc.Run's `obj.Value().Unwrap()`
	// behaviour and ensures `$this->childMethod()` from a parent method
	// resolves through the child class's vtable.
	zobjI := rawZobj.Unwrap()
	zobj, ok := zobjI.(*phpobj.ZObject)
	if !ok {
		// Unwrap returned a non-*ZObject (rare); fall back to the raw
		// pointer so we still attempt a lookup.
		zobj = rawZobj
	}
	class := zobj.GetClass()
	method, ok := class.GetMethod(name)

	// PHP resolves private methods from the caller's class scope, not
	// the runtime class — private methods are not virtual. When calling
	// $this->method() from within a class that defines a private method
	// with that name, use the caller's private method regardless of
	// what the runtime class hierarchy provides. Mirrors the AST's
	// runObjectFunc.Run (compile-object.go ~912).
	callerClass := ctx.Class()
	if callerClass != nil {
		if callerMethod, callerOk := callerClass.GetMethod(name); callerOk && callerMethod.Modifiers.Has(phpv.ZAttrPrivate) && callerMethod.Class != nil && callerMethod.Class.GetName() == callerClass.GetName() {
			method = callerMethod
			ok = true
		}
	}

	if !ok {
		if cm, hasCall := class.GetMethod("__call"); hasCall {
			args, err := evalExprArgs(ctx, exprs)
			if err != nil {
				return nil, err
			}
			a := phpv.NewZArray()
			for _, sub := range args {
				a.OffsetSet(ctx, nil, sub.Dup())
			}
			return ctx.CallZVal(ctx, cm.Method, []*phpv.ZVal{name.ZVal(), a.ZVal()}, zobj)
		}
		return nil, phpobj.ThrowError(ctx, phpobj.Error,
			fmt.Sprintf("Call to undefined method %s::%s()", class.GetName(), name))
	}

	if method.Modifiers.Has(phpv.ZAttrAbstract) || (method.Empty && method.Class != nil && method.Class.GetType() != phpv.ZClassTypeInterface) {
		return nil, phpobj.ThrowError(ctx, phpobj.Error,
			fmt.Sprintf("Cannot call abstract method %s::%s()", method.Class.GetName(), method.Name))
	}

	methodNotVisible := false
	var visErrMsg string
	if method.Modifiers.Has(phpv.ZAttrPrivate) {
		callerClass := ctx.Class()
		methodClass := method.Class
		if callerClass == nil || methodClass == nil || callerClass.GetName() != methodClass.GetName() {
			methodClassName := class.GetName()
			if methodClass != nil {
				methodClassName = methodClass.GetName()
			}
			scope := "global scope"
			if callerClass != nil {
				scope = "scope " + string(callerClass.GetName())
			}
			methodNotVisible = true
			visErrMsg = fmt.Sprintf("Call to private method %s::%s() from %s", methodClassName, method.Name, scope)
		}
	} else if method.Modifiers.Has(phpv.ZAttrProtected) {
		callerClass := ctx.Class()
		if callerClass == nil {
			methodNotVisible = true
			visErrMsg = fmt.Sprintf("Call to protected method %s::%s() from global scope", class.GetName(), method.Name)
		} else if !callerClass.InstanceOf(method.Class) && !method.Class.InstanceOf(callerClass) && !callerClass.InstanceOf(class) && !class.InstanceOf(callerClass) {
			protectedVisible := false
			if method.Class != nil {
				rootClass := method.Class
				for rootClass.GetParent() != nil {
					if pm, ok := rootClass.GetParent().GetMethod(method.Name); ok && pm.Modifiers.Has(phpv.ZAttrProtected) {
						rootClass = rootClass.GetParent()
					} else {
						break
					}
				}
				if callerClass.InstanceOf(rootClass) {
					protectedVisible = true
				}
			}
			if !protectedVisible {
				methodNotVisible = true
				visErrMsg = fmt.Sprintf("Call to protected method %s::%s() from scope %s", class.GetName(), method.Name, callerClass.GetName())
			}
		}
	}
	if methodNotVisible {
		if cm, hasCall := class.GetMethod("__call"); hasCall {
			args, err := evalExprArgs(ctx, exprs)
			if err != nil {
				return nil, err
			}
			a := phpv.NewZArray()
			for _, sub := range args {
				a.OffsetSet(ctx, nil, sub.Dup())
			}
			return ctx.CallZVal(ctx, cm.Method, []*phpv.ZVal{name.ZVal(), a.ZVal()}, zobj)
		}
		return nil, phpobj.ThrowError(ctx, phpobj.Error, visErrMsg)
	}

	if method.Modifiers.IsStatic() {
		m := phpv.BindClassLSB(method.Method, class, class, true)
		m.Attributes = method.Attributes
		return ctx.Call(ctx, m, exprs, nil)
	}

	var objBound phpv.ZObject = zobj
	if method.Class != nil {
		if kin := zobj.GetKin(string(method.Class.GetName())); kin != nil {
			objBound = kin
		}
	}
	// Wrap in MethodCallable when the underlying callable would lose
	// the original called name (trait alias) or its declaring-class
	// identity (trait body). Mirrors the AST's runObjectFunc.Run
	// (compile-object.go ~1245): callers expect the stack trace to
	// show e.g. "Model::__t_get" rather than the trait method's
	// raw "T::__get". Also wrap when the method carries attributes
	// so NoDiscard etc. see them via AttributeGetter.
	callable := phpv.Callable(method.Method)
	declClass := class
	if method.Class != nil {
		declClass = method.Class
	}
	if method.FromTrait != nil || method.Method.Name() != string(name) {
		mc := phpv.BindClass(method.Method, declClass, false)
		if method.Method.Name() != string(name) {
			mc.AliasName = string(name)
		}
		mc.Attributes = method.Attributes
		callable = mc
	} else if len(method.Attributes) > 0 {
		mc := phpv.BindClass(method.Method, declClass, false)
		mc.Attributes = method.Attributes
		callable = mc
	}
	return ctx.Call(ctx, callable, exprs, objBound)
}

// evalExprArgs evaluates each AST argument expression to a ZVal,
// stopping at the first error. Used only by the __call magic-method
// fallback paths in CallInstanceMethodByExprs, which need ZVal args
// to build the magic array.
func evalExprArgs(ctx phpv.Context, exprs []phpv.Runnable) ([]*phpv.ZVal, error) {
	out := make([]*phpv.ZVal, 0, len(exprs))
	for _, e := range exprs {
		v, err := e.Run(ctx)
		if err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, nil
}

// EvalBinop computes the result of a binary operator with full PHP
// semantics. The VM uses this so its arithmetic / bitwise / compare /
// concat opcodes match the AST runOperator path byte-for-byte.
//
// Pre-condition: a and b are already evaluated. Loc is the source
// location of the operator (used for warnings).
//
// Specifically, the dispatch handles:
//   - array + array union (PHP's `+` operator on arrays).
//   - Object operator overloading via HandleDoOperation (e.g. GMP).
//   - PHP 8 TypeError for unsupported operand types (array/resource
//     mixed with numeric ops, completely-non-numeric strings).
//   - "A non-numeric value encountered" warnings for leading-numeric
//     strings.
//   - Implicit numeric coercion (int / float / bitwise-int).
//   - Final dispatch to OperatorMath / OperatorMathLogic / etc. via
//     the operator's op.op pointer.
//
// Compound assignment (op.write) write-back is NOT done here — the
// caller is responsible. For pure binary ops, op.write is false and
// this function returns the computed result.
func EvalBinop(ctx phpv.Context, op tokenizer.ItemType, a, b *phpv.ZVal, loc *phpv.Loc) (*phpv.ZVal, error) {
	opD, ok := operatorList[op]
	if !ok {
		return nil, fmt.Errorf("unknown operator %v", op)
	}

	// Numeric ops — array union, object overload, coercion, etc.
	if opD.numeric {
		aType := a.GetType()
		bType := b.GetType()
		isPlus := op == tokenizer.Rune('+') || op == tokenizer.T_PLUS_EQUAL

		// array + array → union (preserves left, fills missing keys
		// from right). Identical to runOperator.Run's path.
		if isPlus && aType == phpv.ZtArray && bType == phpv.ZtArray {
			result := a.AsArray(ctx).Dup()
			bArr := b.AsArray(ctx)
			for k, v := range bArr.Iterate(ctx) {
				if exists, _ := result.OffsetExists(ctx, k); !exists {
					result.OffsetSet(ctx, k, v)
				}
			}
			return result.ZVal(), nil
		}

		// Bitwise ops on strings: operate on raw bytes. OperatorMathLogic
		// handles that natively.
		isBitwiseOp := op == tokenizer.Rune('|') || op == tokenizer.Rune('^') ||
			op == tokenizer.Rune('&') || op == tokenizer.Rune('~') ||
			op == tokenizer.T_OR_EQUAL || op == tokenizer.T_XOR_EQUAL ||
			op == tokenizer.T_AND_EQUAL
		skipNumericConversion := isBitwiseOp && aType == phpv.ZtString && (bType == phpv.ZtString || op == tokenizer.Rune('~'))

		if !skipNumericConversion {
			// Object overload (GMP, custom classes with HandleDoOperation).
			if aType == phpv.ZtObject || bType == phpv.ZtObject {
				var handler func(phpv.Context, int, *phpv.ZVal, *phpv.ZVal) (*phpv.ZVal, error)
				if aType == phpv.ZtObject {
					if obj, ok := a.Value().(phpv.ZObject); ok {
						if h := obj.GetClass().Handlers(); h != nil && h.HandleDoOperation != nil {
							handler = h.HandleDoOperation
						}
					}
				}
				if handler == nil && bType == phpv.ZtObject {
					if obj, ok := b.Value().(phpv.ZObject); ok {
						if h := obj.GetClass().Handlers(); h != nil && h.HandleDoOperation != nil {
							handler = h.HandleDoOperation
						}
					}
				}
				if handler != nil {
					return handler(ctx, int(op), a, b)
				}
				return nil, phpobj.ThrowError(ctx, phpobj.TypeError,
					fmt.Sprintf("Unsupported operand types: %s %s %s", phpTypeName(a), op.OpString(), phpTypeName(b)))
			}
			// PHP 8: TypeError for array/resource in arithmetic.
			if aType == phpv.ZtArray || bType == phpv.ZtArray {
				return nil, phpobj.ThrowError(ctx, phpobj.TypeError,
					fmt.Sprintf("Unsupported operand types: %s %s %s", phpTypeName(a), op.OpString(), phpTypeName(b)))
			}
			if aType == phpv.ZtResource || bType == phpv.ZtResource {
				return nil, phpobj.ThrowError(ctx, phpobj.TypeError,
					fmt.Sprintf("Unsupported operand types: %s %s %s", phpTypeName(a), op.OpString(), phpTypeName(b)))
			}

			// Non-numeric strings: TypeError; leading-numeric: warning.
			if aType == phpv.ZtString {
				s := string(a.Value().(phpv.ZString))
				if !IsLeadingNumeric(s) {
					return nil, phpobj.ThrowError(ctx, phpobj.TypeError,
						fmt.Sprintf("Unsupported operand types: %s %s %s", phpTypeName(a), op.OpString(), phpTypeName(b)))
				}
				if !isNumericString(s) {
					if err := ctx.Warn("A non-numeric value encountered", logopt.Data{Loc: loc, NoFuncName: true}); err != nil {
						return nil, err
					}
				}
			}
			if bType == phpv.ZtString {
				s := string(b.Value().(phpv.ZString))
				if !IsLeadingNumeric(s) {
					return nil, phpobj.ThrowError(ctx, phpobj.TypeError,
						fmt.Sprintf("Unsupported operand types: %s %s %s", phpTypeName(a), op.OpString(), phpTypeName(b)))
				}
				if !isNumericString(s) {
					if err := ctx.Warn("A non-numeric value encountered", logopt.Data{Loc: loc, NoFuncName: true}); err != nil {
						return nil, err
					}
				}
			}
			a, _ = a.AsNumeric(ctx)
			b, _ = b.AsNumeric(ctx)

			isBitwise := op == tokenizer.T_SL || op == tokenizer.T_SR ||
				op == tokenizer.T_SL_EQUAL || op == tokenizer.T_SR_EQUAL ||
				op == tokenizer.Rune('|') || op == tokenizer.Rune('^') ||
				op == tokenizer.Rune('&') || op == tokenizer.Rune('%') ||
				op == tokenizer.T_OR_EQUAL || op == tokenizer.T_XOR_EQUAL ||
				op == tokenizer.T_AND_EQUAL || op == tokenizer.T_MOD_EQUAL
			if isBitwise {
				var err error
				a, err = implicitToInt(ctx, a)
				if err != nil {
					return nil, err
				}
				b, err = implicitToInt(ctx, b)
				if err != nil {
					return nil, err
				}
			} else if a.GetType() == phpv.ZtFloat || b.GetType() == phpv.ZtFloat {
				a, _ = a.As(ctx, phpv.ZtFloat)
				b, _ = b.As(ctx, phpv.ZtFloat)
			} else {
				a, _ = a.As(ctx, phpv.ZtInt)
				b, _ = b.As(ctx, phpv.ZtInt)
			}
		}
	}

	if opD.op != nil {
		return opD.op(ctx, op, a, b)
	}
	return b, nil
}
