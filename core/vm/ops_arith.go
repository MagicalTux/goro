package vm

import (
	"github.com/KarpelesLab/goro/core/compiler"
	"github.com/KarpelesLab/goro/core/phpv"
	"github.com/KarpelesLab/goro/core/tokenizer"
)

// Each helper here is a thin wrapper around compiler.EvalBinop, which
// implements the same semantic dispatch as runOperator.Run (array
// union, GMP overload, type-error checking, numeric coercion, …).
// Routing through one helper keeps the VM identical in behaviour to
// the AST and centralises any future PHP-version updates.

func opAdd(ctx phpv.Context, a, b *phpv.ZVal) (*phpv.ZVal, error) {
	return compiler.EvalBinop(ctx, tokenizer.Rune('+'), a, b, nil)
}
func opSub(ctx phpv.Context, a, b *phpv.ZVal) (*phpv.ZVal, error) {
	return compiler.EvalBinop(ctx, tokenizer.Rune('-'), a, b, nil)
}
func opMul(ctx phpv.Context, a, b *phpv.ZVal) (*phpv.ZVal, error) {
	return compiler.EvalBinop(ctx, tokenizer.Rune('*'), a, b, nil)
}
func opDiv(ctx phpv.Context, a, b *phpv.ZVal) (*phpv.ZVal, error) {
	return compiler.EvalBinop(ctx, tokenizer.Rune('/'), a, b, nil)
}
func opPow(ctx phpv.Context, a, b *phpv.ZVal) (*phpv.ZVal, error) {
	return compiler.EvalBinop(ctx, tokenizer.T_POW, a, b, nil)
}
func opMod(ctx phpv.Context, a, b *phpv.ZVal) (*phpv.ZVal, error) {
	return compiler.EvalBinop(ctx, tokenizer.Rune('%'), a, b, nil)
}
func opBitAnd(ctx phpv.Context, a, b *phpv.ZVal) (*phpv.ZVal, error) {
	return compiler.EvalBinop(ctx, tokenizer.Rune('&'), a, b, nil)
}
func opBitOr(ctx phpv.Context, a, b *phpv.ZVal) (*phpv.ZVal, error) {
	return compiler.EvalBinop(ctx, tokenizer.Rune('|'), a, b, nil)
}
func opBitXor(ctx phpv.Context, a, b *phpv.ZVal) (*phpv.ZVal, error) {
	return compiler.EvalBinop(ctx, tokenizer.Rune('^'), a, b, nil)
}
func opShl(ctx phpv.Context, a, b *phpv.ZVal) (*phpv.ZVal, error) {
	return compiler.EvalBinop(ctx, tokenizer.T_SL, a, b, nil)
}
func opShr(ctx phpv.Context, a, b *phpv.ZVal) (*phpv.ZVal, error) {
	return compiler.EvalBinop(ctx, tokenizer.T_SR, a, b, nil)
}
func opConcat(ctx phpv.Context, a, b *phpv.ZVal) (*phpv.ZVal, error) {
	return compiler.OperatorAppend(ctx, tokenizer.Rune('.'), a, b)
}

// opNeg implements unary `-`. Mirrors the dedicated unary-minus path
// in runOperator.Run (run-operator.go:705-732): direct negation for
// numeric types (preserves -0.0 semantics), GMP overload via
// HandleDoOperation, fallback to OperatorMath for other types.
func opNeg(ctx phpv.Context, b *phpv.ZVal) (*phpv.ZVal, error) {
	bType := b.GetType()
	if bType == phpv.ZtObject {
		if obj, ok := b.Value().(phpv.ZObject); ok {
			if h := obj.GetClass().Handlers(); h != nil && h.HandleDoOperation != nil {
				return h.HandleDoOperation(ctx, int(tokenizer.Rune('-')), nil, b)
			}
		}
	}
	if bType == phpv.ZtInt || bType == phpv.ZtFloat {
		switch v := b.Value().(type) {
		case phpv.ZInt:
			if v == 0 {
				return phpv.ZInt(0).ZVal(), nil
			}
			// math.MinInt64 negation overflows to a float — mirror AST.
			if v == minInt64 {
				return phpv.ZFloat(-float64(v)).ZVal(), nil
			}
			return (-v).ZVal(), nil
		case phpv.ZFloat:
			return (-v).ZVal(), nil
		}
	}
	// String/bool/null/etc.: route through EvalBinop with int(0)
	// LHS so the same numeric-conversion warnings fire.
	zero := phpv.ZInt(0).ZVal()
	return compiler.EvalBinop(ctx, tokenizer.Rune('-'), zero, b, nil)
}

// minInt64 mirrors math.MinInt64 without the math import (avoids a
// stdlib import for one constant). This is a compile-time constant.
const minInt64 = -1 << 63

// opBitNot implements unary `~`. The AST routes ~ via OperatorMathLogic
// with a left operand of 0; mirror that here.
func opBitNot(ctx phpv.Context, a *phpv.ZVal) (*phpv.ZVal, error) {
	zero := phpv.ZInt(0).ZVal()
	return compiler.OperatorMathLogic(ctx, tokenizer.Rune('~'), zero, a)
}

// opNot implements unary `!`. OperatorNot expects (a, b) and reads b's
// bool value; the left operand is unused in the AST helper.
func opNot(ctx phpv.Context, a *phpv.ZVal) (*phpv.ZVal, error) {
	return compiler.OperatorNot(ctx, tokenizer.Rune('!'), nil, a)
}

// compoundOp resolves an OP_OP_ASSIGN_LOCAL B-field (a tokenizer
// ItemType) to the corresponding (a, b) -> result helper.
func compoundOp(op tokenizer.ItemType) func(phpv.Context, *phpv.ZVal, *phpv.ZVal) (*phpv.ZVal, error) {
	switch op {
	case tokenizer.T_PLUS_EQUAL:
		return opAdd
	case tokenizer.T_MINUS_EQUAL:
		return opSub
	case tokenizer.T_MUL_EQUAL:
		return opMul
	case tokenizer.T_DIV_EQUAL:
		return opDiv
	case tokenizer.T_POW_EQUAL:
		return opPow
	case tokenizer.T_MOD_EQUAL:
		return opMod
	case tokenizer.T_AND_EQUAL:
		return opBitAnd
	case tokenizer.T_OR_EQUAL:
		return opBitOr
	case tokenizer.T_XOR_EQUAL:
		return opBitXor
	case tokenizer.T_SL_EQUAL:
		return opShl
	case tokenizer.T_SR_EQUAL:
		return opShr
	case tokenizer.T_CONCAT_EQUAL:
		return opConcat
	}
	return nil
}
