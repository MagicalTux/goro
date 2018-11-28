package core

import (
	"errors"
	"strings"

	"github.com/MagicalTux/goro/core/phpv"
)

//> func int strlen ( string $string )
func fncStrlen(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var s phpv.ZString
	_, err := Expand(ctx, args, &s)
	if err != nil {
		return nil, err
	}

	return phpv.ZInt(len(s)).ZVal(), nil
}

//> func int error_reporting ([ int $level ] )
func fncErrorReporting(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var level *phpv.ZInt
	_, err := Expand(ctx, args, &level)
	if err != nil {
		return nil, err
	}

	if level != nil {
		ctx.Global().(*Global).SetLocalConfig("error_reporting", (*level).ZVal())
	}

	return ctx.GetConfig("error_reporting", phpv.ZInt(0).ZVal()), nil
}

//> func bool define ( string $name , mixed $value )
func fncDefine(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var name phpv.ZString
	var value *phpv.ZVal
	_, err := Expand(ctx, args, &name, &value)
	if err != nil {
		return nil, err
	}

	g := ctx.Global().(*Global)

	if _, ok := g.constant[name]; ok {
		// TODO trigger notice: Constant %s already defined
		return phpv.ZBool(false).ZVal(), nil
	}

	g.constant[name] = value
	return phpv.ZBool(true).ZVal(), nil
}

//> func bool defined ( string $name )
func fncDefined(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var name phpv.ZString
	_, err := Expand(ctx, args, &name)
	if err != nil {
		return nil, err
	}

	g := ctx.Global().(*Global)

	_, ok := g.constant[name]

	return phpv.ZBool(ok).ZVal(), nil
}

//> func int count ( mixed $array_or_countable [, int $mode = COUNT_NORMAL ] )
func fncCount(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var countable *phpv.ZVal
	var mode *phpv.ZInt
	_, err := Expand(ctx, args, &countable, &mode)
	if err != nil {
		return nil, err
	}

	if mode != nil {
		return nil, errors.New("todo recursive count")
	}

	if v, ok := countable.Value().(phpv.ZCountable); ok {
		return v.Count(ctx).ZVal(), nil
	}

	// make this a warning
	return phpv.ZInt(1).ZVal(), errors.New("count(): Parameter must be an array or an object that implements Countable")
}

//> func int strcmp ( string $str1 , string $str2 )
func fncStrcmp(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var a, b phpv.ZString
	_, err := Expand(ctx, args, &a, &b)
	if err != nil {
		return nil, err
	}

	r := strings.Compare(string(a), string(b))
	return phpv.ZInt(r).ZVal(), nil
}
