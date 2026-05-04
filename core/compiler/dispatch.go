package compiler

import (
	"fmt"

	"github.com/KarpelesLab/goro/core/logopt"
	"github.com/KarpelesLab/goro/core/phpobj"
	"github.com/KarpelesLab/goro/core/phpv"
	"github.com/KarpelesLab/goro/core/tokenizer"
)

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
func IsSlotSafe(r phpv.Runnable) bool {
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
	case *runStaticVar:
		return false
	case *runGlobal:
		return false
	case *ZClosure:
		// Top-level function/method declaration.
		return false
	}
	for _, c := range GetChildren(r) {
		if !IsSlotSafe(c) {
			return false
		}
	}
	return true
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
	// for parent::-style dispatch, then invoke directly. Keep the
	// callable un-wrapped to preserve object identity (the AST does
	// the same for the simple case).
	var objBound phpv.ZObject = zobj
	if method.Class != nil {
		if kin := zobj.GetKin(string(method.Class.GetName())); kin != nil {
			objBound = kin
		}
	}
	return ctx.CallZVal(ctx, method.Method, args, objBound)
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
				if !isLeadingNumeric(s) {
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
				if !isLeadingNumeric(s) {
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
