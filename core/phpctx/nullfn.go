package phpctx

import "github.com/KarpelesLab/goro/core/phpv"

type nullCallable struct {
	phpv.CallableVal
}

func (nc *nullCallable) Call(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	return phpv.ZNULL.ZVal(), nil
}

var noOp = &nullCallable{}
