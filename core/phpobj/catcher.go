package phpobj

import "github.com/MagicalTux/goro/core/phpv"

type callCatcher struct {
	name   phpv.ZString
	target phpv.Callable
}

func (c *callCatcher) Call(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	a := phpv.NewZArray()
	for _, sub := range args {
		a.OffsetSet(ctx, nil, sub)
	}
	rArgs := []*phpv.ZVal{c.name.ZVal(), a.ZVal()}

	return c.target.Call(ctx, rArgs)
}
