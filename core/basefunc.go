package core

import (
	"strings"
	"unsafe"

	"github.com/MagicalTux/goro/core/logopt"
	"github.com/MagicalTux/goro/core/phpobj"
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
	var levelArg Optional[phpv.ZInt]
	_, err := Expand(ctx, args, &levelArg)
	if err != nil {
		return nil, err
	}

	if levelArg.HasArg() {
		level := levelArg.Get()
		ctx.Global().SetLocalConfig("error_reporting", level.ZVal())
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

	// Class constants cannot be defined via define()
	if strings.Contains(string(name), "::") {
		return nil, phpobj.ThrowError(ctx, phpobj.ValueError, "define(): Argument #1 ($constant_name) cannot be a class constant")
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

	value := ctx.Global().GetConfig(varName, phpv.ZStr(""))
	return value.AsString(ctx).ZVal(), nil
}

// > func string ini_restore ( string $varname)
func fncIniRestore(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var varName phpv.ZString
	_, err := Expand(ctx, args, &varName)
	if err != nil {
		return nil, err
	}

	ctx.Global().RestoreConfig(varName)
	return nil, nil
}

// > func string ini_set ( string $varname, string $newvalue )
// > alias ini_alter
func fncIniSet(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var varName phpv.ZString
	var newValue phpv.ZString
	_, err := Expand(ctx, args, &varName, &newValue)
	if err != nil {
		return nil, err
	}

	oldValue, ok := ctx.Global().SetLocalConfig(varName, newValue.ZVal())
	if !ok {
		return phpv.ZFalse.ZVal(), nil
	}

	return oldValue.ZVal(), nil
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
			entry.OffsetSet(ctx, phpv.ZStr("global_value"), v.Local.ZVal())
			entry.OffsetSet(ctx, phpv.ZStr("local_value"), v.Global.ZVal())
			result.OffsetSet(ctx, phpv.ZString(k), entry.ZVal())
		}
	} else {
		g := ctx.Global()
		for k, v := range g.IterateConfig() {
			result.OffsetSet(ctx, phpv.ZString(k), v.Get().ZVal())
		}
	}
	return result.ZVal(), nil
}

// > func array get_defined_functions ([ bool $exclude_disabled = true ] )
func fncGetDefinedFunctions(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	if len(args) > 0 {
		ctx.Deprecated("The $exclude_disabled parameter has no effect since PHP 8.0")
	}

	// Since PHP 8.0, disabled functions are always excluded
	result, err := ctx.Global().GetDefinedFunctions(ctx, true)
	if err != nil {
		return nil, ctx.FuncError(err)
	}

	return result.ZVal(), nil
}

// > func int ini_parse_quantity ( string $shorthand )
func fncIniParseQuantity(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var shorthand phpv.ZString
	_, err := Expand(ctx, args, &shorthand)
	if err != nil {
		return nil, err
	}

	s := strings.TrimSpace(string(shorthand))
	if s == "" {
		return phpv.ZInt(0).ZVal(), nil
	}

	// Parse sign
	negative := false
	if len(s) > 0 && s[0] == '-' {
		negative = true
		s = s[1:]
	} else if len(s) > 0 && s[0] == '+' {
		s = s[1:]
	}

	// Parse numeric value (supports hex 0x, octal 0, decimal)
	var num int64
	numEnd := 0
	if strings.HasPrefix(s, "0x") || strings.HasPrefix(s, "0X") {
		numEnd = 2
		for numEnd < len(s) {
			c := s[numEnd]
			if c >= '0' && c <= '9' {
				num = num*16 + int64(c-'0')
			} else if c >= 'a' && c <= 'f' {
				num = num*16 + int64(c-'a'+10)
			} else if c >= 'A' && c <= 'F' {
				num = num*16 + int64(c-'A'+10)
			} else {
				break
			}
			numEnd++
		}
	} else {
		for numEnd < len(s) {
			c := s[numEnd]
			if c >= '0' && c <= '9' {
				num = num*10 + int64(c-'0')
			} else {
				break
			}
			numEnd++
		}
	}

	// Skip past any trailing junk to find the multiplier
	rest := strings.TrimSpace(s[numEnd:])
	multiplier := int64(1)
	if len(rest) > 0 {
		last := rest[len(rest)-1]
		switch last | 0x20 {
		case 'k':
			multiplier = 1024
		case 'm':
			multiplier = 1024 * 1024
		case 'g':
			multiplier = 1024 * 1024 * 1024
		default:
			// Unknown suffix character - emit warning
			origStr := string(shorthand)
			numStr := s[:numEnd]
			if negative {
				numStr = "-" + numStr
			}
			ctx.Warn("Invalid quantity \"%s\": unknown multiplier \"%c\", interpreting as \"%s\" for backwards compatibility",
				origStr, last, numStr, logopt.Data{NoFuncName: true})
		}
		// Check for extra chars between number and multiplier
		if multiplier > 1 && len(rest) > 1 {
			origStr := string(shorthand)
			numStr := s[:numEnd]
			if negative {
				numStr = "-" + numStr
			}
			ctx.Warn("Invalid quantity \"%s\", interpreting as \"%s %c\" for backwards compatibility",
				origStr, numStr, rest[len(rest)-1], logopt.Data{NoFuncName: true})
		}
	}

	num *= multiplier
	if negative {
		num = -num
	}
	return phpv.ZInt(num).ZVal(), nil
}

// > func string get_debug_type ( mixed $value )
func fncGetDebugType(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var v *phpv.ZVal
	_, err := Expand(ctx, args, &v)
	if err != nil {
		return nil, err
	}

	var typeName string
	switch v.GetType() {
	case phpv.ZtNull:
		typeName = "null"
	case phpv.ZtBool:
		typeName = "bool"
	case phpv.ZtInt:
		typeName = "int"
	case phpv.ZtFloat:
		typeName = "float"
	case phpv.ZtString:
		typeName = "string"
	case phpv.ZtArray:
		typeName = "array"
	case phpv.ZtObject:
		if obj, ok := v.Value().(*phpobj.ZObject); ok {
			typeName = string(obj.GetClass().GetName())
		} else {
			typeName = "object"
		}
	case phpv.ZtResource:
		if r, ok := v.Value().(phpv.Resource); ok {
			typeName = "resource (" + r.GetResourceType().String() + ")"
		} else {
			typeName = "resource (Unknown)"
		}
	default:
		typeName = "unknown"
	}
	return phpv.ZString(typeName).ZVal(), nil
}
