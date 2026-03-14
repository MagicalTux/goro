package core

import (
	"github.com/MagicalTux/goro/core/phpv"
)

// > func bool class_alias ( string $class , string $alias [, bool $autoload = true ] )
func fncClassAlias(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var className phpv.ZString
	var alias phpv.ZString
	var autoloadArg Optional[phpv.ZBool]
	_, err := Expand(ctx, args, &className, &alias, &autoloadArg)
	if err != nil {
		return nil, err
	}

	autoload := bool(autoloadArg.GetOrDefault(phpv.ZBool(true)))

	// Resolve the original class
	class, err := ctx.Global().GetClass(ctx, className, autoload)
	if err != nil {
		return phpv.ZFalse.ZVal(), nil
	}

	// Register the class under the alias name
	err = ctx.Global().RegisterClass(alias, class)
	if err != nil {
		return phpv.ZFalse.ZVal(), nil
	}

	return phpv.ZTrue.ZVal(), nil
}
