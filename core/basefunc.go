package core

import (
	"strings"
	"unsafe"

	"github.com/MagicalTux/goro/core/phpctx"
	"github.com/MagicalTux/goro/core/phpv"
)

// > const
const (
	COUNT_NORMAL phpv.ZInt = iota
	COUNT_RECURSIVE
)

// > func int strlen ( string $string )
func fncStrlen(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var s phpv.ZString
	_, err := Expand(ctx, args, &s)
	if err != nil {
		return nil, err
	}

	return phpv.ZInt(len(s)).ZVal(), nil
}

// > func int error_reporting ([ int $level ] )
func fncErrorReporting(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var level *phpv.ZInt
	_, err := Expand(ctx, args, &level)
	if err != nil {
		return nil, err
	}

	if level != nil {
		ctx.Global().SetLocalConfig("error_reporting", (*level).ZVal())
	}

	return ctx.GetConfig("error_reporting", phpv.ZInt(0).ZVal()), nil
}

// > func bool define ( string $name , mixed $value )
func fncDefine(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var name phpv.ZString
	var value *phpv.ZVal
	_, err := Expand(ctx, args, &name, &value)
	if err != nil {
		return nil, err
	}

	g := ctx.Global()

	ok := g.ConstantSet(name, value.Value())
	if !ok {
		// TODO trigger notice: Constant %s already defined
		return phpv.ZBool(false).ZVal(), nil
	}

	return phpv.ZBool(true).ZVal(), nil
}

// > func bool defined ( string $name )
func fncDefined(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var name phpv.ZString
	_, err := Expand(ctx, args, &name)
	if err != nil {
		return nil, err
	}

	g := ctx.Global()

	_, ok := g.ConstantGet(name)

	return phpv.ZBool(ok).ZVal(), nil
}

// > func int count ( mixed $array_or_countable [, int $mode = COUNT_NORMAL ] )
// > alias sizeof
func fncCount(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var countable *phpv.ZVal
	var modeArg *phpv.ZInt
	_, err := Expand(ctx, args, &countable, &modeArg)
	if err != nil {
		return nil, err
	}

	mode := COUNT_NORMAL
	if modeArg != nil {
		mode = *modeArg
	}

	if mode == COUNT_RECURSIVE && countable.GetType() == phpv.ZtArray {
		visisted := map[uintptr]struct{}{}
		count, err := recursiveCount(ctx, countable.AsArray(ctx), visisted)
		if err != nil {
			return nil, ctx.FuncError(err)
		}
		return phpv.ZInt(count).ZVal(), nil
	}

	if v, ok := countable.Value().(phpv.ZCountable); ok {
		return v.Count(ctx).ZVal(), nil
	}

	if err := ctx.Warn("Parameter must be an array or an object that implements Countable"); err != nil {
		return nil, err
	}
	return phpv.ZInt(1).ZVal(), nil
}

func recursiveCount(ctx phpv.Context, array *phpv.ZArray, visited map[uintptr]struct{}) (int, error) {
	var err error
	ptr := uintptr(unsafe.Pointer(array))
	if _, seen := visited[ptr]; seen {
		if err = ctx.Warn("recursive loop detected while counting"); err != nil {
			return 0, err
		}
		return 0, nil
	}

	visited[ptr] = struct{}{}

	count := 0
	for _, elem := range array.Iterate(ctx) {
		count++
		if elem.GetType() == phpv.ZtArray {
			n, err := recursiveCount(ctx, elem.AsArray(ctx), visited)
			if err != nil {
				return 0, err
			}
			count += n
		} else if v, ok := elem.Value().(phpv.ZCountable); ok {
			count += int(v.Count(ctx))
		}
	}

	return count, nil
}

// > func int strcmp ( string $str1 , string $str2 )
func fncStrcmp(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var a, b phpv.ZString
	_, err := Expand(ctx, args, &a, &b)
	if err != nil {
		return nil, err
	}

	r := strings.Compare(string(a), string(b))
	return phpv.ZInt(r).ZVal(), nil
}

// > func bool empty ( mixed $var )
func fncEmpty(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var v *phpv.ZVal
	_, err := Expand(ctx, args, &v)
	if err != nil {
		return nil, err
	}

	switch v.GetType() {
	case phpv.ZtNull:
		return phpv.ZBool(true).ZVal(), nil
	case phpv.ZtBool:
		return phpv.ZBool(v.Value().(phpv.ZBool) == false).ZVal(), nil
	case phpv.ZtInt:
		return phpv.ZBool(v.Value().(phpv.ZInt) == 0).ZVal(), nil
	case phpv.ZtFloat:
		return phpv.ZBool(v.Value().(phpv.ZFloat) == 0).ZVal(), nil
	case phpv.ZtString:
		s := v.Value().(phpv.ZString)
		return phpv.ZBool(s == "" || s == "0").ZVal(), nil
	case phpv.ZtArray:
		s := v.Value().(*phpv.ZArray)
		return phpv.ZBool(s.Count(ctx) == 0).ZVal(), nil
	}
	return phpv.ZBool(false).ZVal(), nil // unsupported type
}

// > func array get_loaded_extensions ([ bool $zend_extensions = FALSE ])
func fncLoadedExtensions(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var zendOnly *phpv.ZBool
	_, err := Expand(ctx, args, &zendOnly)
	if err != nil {
		return nil, err
	}

	result := phpv.NewZArray()
	if Deref(zendOnly, false) {
		// TODO
	} else {
		for _, elem := range ctx.Global().GetLoadedExtensions() {
			result.OffsetSet(ctx, nil, phpv.ZStr(elem))
		}
	}
	return result.ZVal(), nil
}

// > func string ini_get ( string $varname)
func fncIniGet(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var varName phpv.ZString
	_, err := Expand(ctx, args, &varName)
	if err != nil {
		return nil, err
	}

	return ctx.Global().GetConfig(varName, phpv.ZNULL.ZVal()), nil
}

// > func array ini_get_all ([ string $extension [, bool $details = TRUE ]] )
func fncIniGetAll(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var extension *phpv.ZString
	var details *phpv.ZBool
	_, err := Expand(ctx, args, &extension, &details)
	if err != nil {
		return nil, ctx.FuncError(err)
	}

	result := phpv.NewZArray()
	if Deref(details, true) {
		for k, v := range ctx.Global().IterateConfig() {
			entry := phpv.NewZArray()
			entry.OffsetSet(ctx, phpv.ZStr("global_value"), v.Local)
			entry.OffsetSet(ctx, phpv.ZStr("local_value"), v.Global)
			result.OffsetSet(ctx, phpv.ZString(k), entry.ZVal())
		}
	} else {
		g := ctx.Global()
		for k, v := range g.IterateConfig() {
			value := v.Local
			if value == nil {
				value = v.Global
			}
			result.OffsetSet(ctx, phpv.ZString(k), value)
		}
	}
	return result.ZVal(), nil
}

// > func int get_defined_functions ( [ bool $exclude_disabled = FALSE ] )
func fncGetDefinedFunctions(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var excludeDisabled Optional[phpv.ZBool]
	_, err := Expand(ctx, args, &excludeDisabled)
	if err != nil {
		return nil, ctx.FuncError(err)
	}

	g := ctx.Global().(*phpctx.Global)
	result, err := g.GetDefinedFunctions(ctx, bool(excludeDisabled.GetOrDefault(false)))
	if err != nil {
		return nil, ctx.FuncError(err)
	}

	return result.ZVal(), nil
}
