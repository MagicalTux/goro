package vm

import (
	"github.com/KarpelesLab/goro/core/compiler"
	"github.com/KarpelesLab/goro/core/phpv"
	"github.com/KarpelesLab/goro/core/tokenizer"
)

// Each helper here is a thin wrapper around the AST's exported operator
// functions in core/compiler. Routing through them keeps the VM's
// semantics identical to the AST — type juggling, GMP overload, GMP-
// specific deprecation warnings, divide-by-zero behaviour, etc.

func opAdd(ctx phpv.Context, a, b *phpv.ZVal) (*phpv.ZVal, error) {
	return compiler.OperatorMath(ctx, tokenizer.Rune('+'), a, b)
}
func opSub(ctx phpv.Context, a, b *phpv.ZVal) (*phpv.ZVal, error) {
	return compiler.OperatorMath(ctx, tokenizer.Rune('-'), a, b)
}
func opMul(ctx phpv.Context, a, b *phpv.ZVal) (*phpv.ZVal, error) {
	return compiler.OperatorMath(ctx, tokenizer.Rune('*'), a, b)
}
func opDiv(ctx phpv.Context, a, b *phpv.ZVal) (*phpv.ZVal, error) {
	return compiler.OperatorMath(ctx, tokenizer.Rune('/'), a, b)
}
func opPow(ctx phpv.Context, a, b *phpv.ZVal) (*phpv.ZVal, error) {
	return compiler.OperatorMath(ctx, tokenizer.T_POW, a, b)
}
func opMod(ctx phpv.Context, a, b *phpv.ZVal) (*phpv.ZVal, error) {
	return compiler.OperatorMathLogic(ctx, tokenizer.Rune('%'), a, b)
}
func opBitAnd(ctx phpv.Context, a, b *phpv.ZVal) (*phpv.ZVal, error) {
	return compiler.OperatorMathLogic(ctx, tokenizer.Rune('&'), a, b)
}
func opBitOr(ctx phpv.Context, a, b *phpv.ZVal) (*phpv.ZVal, error) {
	return compiler.OperatorMathLogic(ctx, tokenizer.Rune('|'), a, b)
}
func opBitXor(ctx phpv.Context, a, b *phpv.ZVal) (*phpv.ZVal, error) {
	return compiler.OperatorMathLogic(ctx, tokenizer.Rune('^'), a, b)
}
func opShl(ctx phpv.Context, a, b *phpv.ZVal) (*phpv.ZVal, error) {
	return compiler.OperatorMathLogic(ctx, tokenizer.T_SL, a, b)
}
func opShr(ctx phpv.Context, a, b *phpv.ZVal) (*phpv.ZVal, error) {
	return compiler.OperatorMathLogic(ctx, tokenizer.T_SR, a, b)
}
func opConcat(ctx phpv.Context, a, b *phpv.ZVal) (*phpv.ZVal, error) {
	return compiler.OperatorAppend(ctx, tokenizer.Rune('.'), a, b)
}

// opNeg implements unary `-`. PHP evaluates `-$x` as `0 - $x`, which
// matches what OperatorMath does for a left operand of int(0). This
// preserves the existing edge cases (string coercion, GMP, INF, etc.).
func opNeg(ctx phpv.Context, a *phpv.ZVal) (*phpv.ZVal, error) {
	zero := phpv.ZInt(0).ZVal()
	return compiler.OperatorMath(ctx, tokenizer.Rune('-'), zero, a)
}

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
// ItemType) to the corresponding (a, b) -> result helper. Returns nil
// if the op isn't a compound-assignment we support.
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
