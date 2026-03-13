package spl

import (
	"github.com/MagicalTux/goro/core"
	"github.com/MagicalTux/goro/core/phpctx"
	"github.com/MagicalTux/goro/core/phpobj"
	"github.com/MagicalTux/goro/core/phpv"
)

func init() {
	initArrayIterator()
	initInfiniteIterator()

	phpctx.RegisterExt(&phpctx.Ext{
		Name:    "SPL",
		Version: core.VERSION,
		Classes: []*phpobj.ZClass{
			Countable,
			OuterIterator,
			ArrayIteratorClass,
			InfiniteIteratorClass,
		},
		Functions: map[string]*phpctx.ExtFunction{},
		Constants: map[phpv.ZString]phpv.Val{},
	})
}
