package vm

import (
	"github.com/KarpelesLab/goro/core/compiler"
	"github.com/KarpelesLab/goro/core/phpv"
	"github.com/KarpelesLab/goro/core/tokenizer"
)

func opCmpEq(ctx phpv.Context, a, b *phpv.ZVal) (*phpv.ZVal, error) {
	return compiler.OperatorCompare(ctx, tokenizer.T_IS_EQUAL, a, b)
}
func opCmpNe(ctx phpv.Context, a, b *phpv.ZVal) (*phpv.ZVal, error) {
	return compiler.OperatorCompare(ctx, tokenizer.T_IS_NOT_EQUAL, a, b)
}
func opCmpLt(ctx phpv.Context, a, b *phpv.ZVal) (*phpv.ZVal, error) {
	return compiler.OperatorCompare(ctx, tokenizer.Rune('<'), a, b)
}
func opCmpLe(ctx phpv.Context, a, b *phpv.ZVal) (*phpv.ZVal, error) {
	return compiler.OperatorCompare(ctx, tokenizer.T_IS_SMALLER_OR_EQUAL, a, b)
}
func opCmpGt(ctx phpv.Context, a, b *phpv.ZVal) (*phpv.ZVal, error) {
	return compiler.OperatorCompare(ctx, tokenizer.Rune('>'), a, b)
}
func opCmpGe(ctx phpv.Context, a, b *phpv.ZVal) (*phpv.ZVal, error) {
	return compiler.OperatorCompare(ctx, tokenizer.T_IS_GREATER_OR_EQUAL, a, b)
}
func opCmpSpaceship(ctx phpv.Context, a, b *phpv.ZVal) (*phpv.ZVal, error) {
	return compiler.OperatorCompare(ctx, tokenizer.T_SPACESHIP, a, b)
}
func opCmpId(ctx phpv.Context, a, b *phpv.ZVal) (*phpv.ZVal, error) {
	return compiler.OperatorCompareStrict(ctx, tokenizer.T_IS_IDENTICAL, a, b)
}
func opCmpNid(ctx phpv.Context, a, b *phpv.ZVal) (*phpv.ZVal, error) {
	return compiler.OperatorCompareStrict(ctx, tokenizer.T_IS_NOT_IDENTICAL, a, b)
}
