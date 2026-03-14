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
		Functions: map[string]*phpctx.ExtFunction{
			"spl_object_hash":        {Func: splObjectHash, Args: []*phpctx.ExtFunctionArg{}},
			"spl_object_id":          {Func: splObjectId, Args: []*phpctx.ExtFunctionArg{}},
			"iterator_to_array":      {Func: iteratorToArray, Args: []*phpctx.ExtFunctionArg{}},
			"iterator_count":         {Func: iteratorCount, Args: []*phpctx.ExtFunctionArg{}},
			"class_implements":       {Func: classImplements, Args: []*phpctx.ExtFunctionArg{}},
			"class_parents":          {Func: classParents, Args: []*phpctx.ExtFunctionArg{}},
			"class_uses":             {Func: classUses, Args: []*phpctx.ExtFunctionArg{}},
			"spl_autoload_functions": {Func: splAutoloadFunctions, Args: []*phpctx.ExtFunctionArg{}},
		},
		Constants: map[phpv.ZString]phpv.Val{},
	})
}
