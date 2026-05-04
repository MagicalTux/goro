package compiler

import (
	"fmt"

	"github.com/KarpelesLab/goro/core/logopt"
	"github.com/KarpelesLab/goro/core/phpobj"
	"github.com/KarpelesLab/goro/core/phpv"
	"github.com/KarpelesLab/goro/core/tokenizer"
)

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
