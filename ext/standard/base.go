package standard

import (
	"errors"
	"net"
	"strings"
	"unicode"

	"github.com/MagicalTux/goro/core"
	"github.com/MagicalTux/goro/core/phpctx"
	"github.com/MagicalTux/goro/core/phpv"
)

// > func bool dl ( string $library )
func stdFuncDl(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	return nil, errors.New("Dynamically loaded extensions aren't enabled")
}

// > func bool extension_loaded ( string $name )
func stdFunc(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var name string
	_, err := core.Expand(ctx, args, &name)
	if err != nil {
		return nil, err
	}
	return phpv.ZBool(phpctx.HasExt(name)).ZVal(), nil
}

// > func bool function_exists ( string $function_name )
func stdFuncFuncExists(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var fname phpv.ZString
	_, err := core.Expand(ctx, args, &fname)
	if err != nil {
		return nil, err
	}

	f, _ := ctx.Global().GetFunction(ctx, fname)
	return phpv.ZBool(f != nil).ZVal(), nil
}

// > func bool method_exists (  mixed $object , string $method_name )
func stdFuncMethodExists(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var objectArg *phpv.ZVal
	var methodName phpv.ZString
	_, err := core.Expand(ctx, args, &objectArg, &methodName)
	if err != nil {
		return nil, err
	}

	var class phpv.ZClass
	switch objectArg.GetType() {
	case phpv.ZtString:
		className := objectArg.AsString(ctx)
		class, err = ctx.Global().GetClass(ctx, className, false)
		if err != nil {
			return phpv.ZFalse.ZVal(), nil
		}
	case phpv.ZtObject:
		obj := objectArg.AsObject(ctx)
		class = obj.GetClass()
	default:
		return nil, errors.New("Argument #1 ($object_or_class) must be of type object|string")
	}
	_, ok := class.GetMethod(methodName)

	return phpv.ZBool(ok).ZVal(), nil
}

// > func mixed get_cfg_var ( string $option )
func stdFuncGetCfgVar(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var v phpv.ZString
	_, err := core.Expand(ctx, args, &v)
	if err != nil {
		return nil, err
	}
	return ctx.Global().GetConfig(v, phpv.ZNull{}.ZVal()), nil
}

// > func string php_sapi_name ( void )
func stdFuncSapiName(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	v, ok := ctx.Global().ConstantGet("PHP_SAPI")
	if !ok {
		return phpv.ZString("php").ZVal(), nil
	}
	return v.ZVal(), nil
}

// > func string gettype ( mixed $var )
func fncGettype(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var v *phpv.ZVal
	_, err := core.Expand(ctx, args, &v)
	if err != nil {
		return nil, err
	}

	t := v.GetType()
	return phpv.ZString(t.String()).ZVal(), nil
}

// > func void flush ( void )
func fncFlush(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	ctx.Global().Flush()
	return phpv.ZNULL.ZVal(), nil
}

// > func mixed call_user_func ( callable $callback [, mixed $... ] )
func fncCallUserFunc(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var callback phpv.Callable
	_, err := core.Expand(ctx, args, &callback)
	if err != nil {
		return nil, err
	}

	return ctx.CallZVal(ctx, callback, args[1:], nil)
}

// > func mixed call_user_func_array ( callable $callback , array $param_arr )
func fncCallUserFuncArray(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var callback phpv.Callable
	var arrayArgs *phpv.ZArray
	_, err := core.Expand(ctx, args, &callback, arrayArgs)
	if err != nil {
		return nil, err
	}

	var cbArgs []*phpv.ZVal
	for _, v := range arrayArgs.Iterate(ctx) {
		cbArgs = append(cbArgs, v)
	}
	return ctx.CallZVal(ctx, callback, cbArgs, nil)
}

// > func string inet_ntop ( string $in_addr )
func fncInetNtop(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var addr []byte
	_, err := core.Expand(ctx, args, &addr)
	if err != nil {
		return nil, err
	}

	if len(addr) != 4 && len(addr) != 16 {
		return phpv.ZFalse.ZVal(), nil
	}

	ip := net.IP(addr)
	if ip == nil {
		return phpv.ZFalse.ZVal(), nil
	}
	return phpv.ZStr(ip.String()), nil
}

// > func string inet_pton ( string $address )
func fncInetPton(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var addr phpv.ZString
	_, err := core.Expand(ctx, args, &addr)
	if err != nil {
		return nil, err
	}

	ip := net.ParseIP(string(addr)).To16()
	if ip == nil {
		return phpv.ZFalse.ZVal(), nil
	}
	if !strings.Contains(string(addr), "::") {
		ip = ip.To4()
	}

	return phpv.ZStr(string(ip)), nil
}

// > func array getopt ( string $options [, array $longopts [, int &$optind ]] )
func fncGetOpt(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var optionsArg phpv.ZString
	var longOpts **phpv.ZArray
	var optionIndex core.OptionalRef[phpv.ZInt]
	_, err := core.Expand(ctx, args, &optionsArg, &longOpts, &optionIndex)
	if err != nil {
		return nil, err
	}

	const (
		argNoValue = iota
		argRequired
		argOptional
	)

	result := phpv.NewZArray()

	options := []byte(optionsArg)

	argNameMap := map[string]int{}
	for i := 0; i < len(options); i++ {
		c := rune(optionsArg[i])
		if !unicode.IsLetter(c) && !unicode.IsDigit(c) {
			return phpv.ZFalse.ZVal(), nil
		}
		if core.Idx(options, i+1) != ':' {
			argNameMap[string(c)] = argNoValue
		} else {
			if core.Idx(options, i+2) == ':' {
				argNameMap[string(c)] = argOptional
				i++
			} else {
				argNameMap[string(c)] = argRequired
			}
			i++
		}
	}
	if longOpts != nil {
		for _, arg := range (*longOpts).Iterate(ctx) {
			argName := string(arg.AsString(ctx))
			argType := argNoValue
			if strings.HasSuffix(argName, "::") {
				argType = argOptional
				argName = argName[:len(argName)-2]
			} else if strings.HasSuffix(argName, ":") {
				argName = argName[:len(argName)-1]
				argType = argRequired
			}
			argNameMap[argName] = argType
		}
	}

	i := 1
	argv := ctx.Global().Argv()

	for ; i < len(argv); i++ {
		arg := argv[i]

		if !strings.HasPrefix(arg, "-") {
			break
		}

		if strings.HasPrefix(arg, "--") {
			arg = arg[2:]
			var argName, argVal string
			eqIndex := strings.Index(arg, "=")
			if eqIndex >= 0 {
				argName = arg[:eqIndex]
				argVal = arg[eqIndex+1:]
			} else {
				argName = arg
			}
			argType, ok := argNameMap[argName]
			if !ok {
				continue
			}

			switch argType {
			case argNoValue:
				result.OffsetSet(ctx, phpv.ZStr(argName), phpv.ZFalse.ZVal())
			case argRequired:
				if argVal == "" && i < len(argv) {
					argVal = argv[i]
					i++
				}
				if argVal != "" {
					result.OffsetSet(ctx, phpv.ZStr(argName), phpv.ZStr(argVal))
				}
			case argOptional:
				if argVal != "" {
					result.OffsetSet(ctx, phpv.ZStr(argName), phpv.ZStr(argVal))
				} else {
					result.OffsetSet(ctx, phpv.ZStr(argName), phpv.ZFalse.ZVal())
				}
			}
		} else {
			arg = arg[1:]

			for j := 0; j < len(arg); j++ {
				c := string(arg[j])
				argType, ok := argNameMap[string(c)]
				if !ok {
					continue
				}
				switch argType {
				case argNoValue:
					if ok, _ := result.OffsetExists(ctx, phpv.ZStr(c)); ok {
						elem, _ := result.OffsetGet(ctx, phpv.ZStr(c))
						if elem.GetType() == phpv.ZtArray {
							elem.AsArray(ctx).OffsetSet(ctx, nil, phpv.ZFalse.ZVal())
						} else {
							array := phpv.NewZArray()
							array.OffsetSet(ctx, nil, elem)
							array.OffsetSet(ctx, nil, phpv.ZFalse.ZVal())
							result.OffsetUnset(ctx, phpv.ZStr(c))
							result.OffsetSet(ctx, phpv.ZStr(c), array.ZVal())
						}
					} else {
						result.OffsetSet(ctx, phpv.ZStr(c), phpv.ZFalse.ZVal())
					}
				case argRequired:
					j++
					if core.Idx([]byte(arg), j) == '=' {
						j++
					}
					value := arg[j:]
					if value == "" {
						if i+1 < len(argv) {
							// always get the following arg, even if it starts with -
							// e.g.: -q -w must give -q="-w"
							i++
							value = string(argv[i])
							result.OffsetSet(ctx, phpv.ZStr(c), phpv.ZStr(value))
						}
					} else {
						result.OffsetSet(ctx, phpv.ZStr(c), phpv.ZStr(value))
					}
					j = len(arg)
				case argOptional:
					j++
					hasEq := core.Idx([]byte(arg), j) == '='
					if hasEq {
						j++
					}
					value := arg[j:]
					if value == "" {
						if hasEq {
							// -a=  must give a="", not a=false
							result.OffsetSet(ctx, phpv.ZStr(c), phpv.ZStr(""))
						} else {
							result.OffsetSet(ctx, phpv.ZStr(c), phpv.ZFalse.ZVal())
						}
					} else {
						result.OffsetSet(ctx, phpv.ZStr(c), phpv.ZStr(value))
					}
					j = len(arg)
				}
			}
		}
	}

	if optionIndex.HasArg() {
		optionIndex.Set(ctx, phpv.ZInt(i))
	}

	return result.ZVal(), nil
}
