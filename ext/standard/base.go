package standard

import (
	"errors"
	"fmt"
	"net"
	"strings"
	"unicode"

	"github.com/MagicalTux/goro/core"
	"github.com/MagicalTux/goro/core/phpctx"
	"github.com/MagicalTux/goro/core/phpobj"
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

	// Also check for __invoke via HandleInvoke (e.g., Closure::__invoke)
	if !ok && methodName.ToLower() == "__invoke" {
		if h := class.Handlers(); h != nil && h.HandleInvoke != nil {
			ok = true
		}
	}

	return phpv.ZBool(ok).ZVal(), nil
}

// > func mixed get_cfg_var ( string $option )
func stdFuncGetCfgVar(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var v phpv.ZString
	_, err := core.Expand(ctx, args, &v)
	if err != nil {
		return nil, err
	}
	return ctx.Global().GetGlobalConfig(v, phpv.ZNull{}.ZVal()), nil
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
	// Resolve the callback in the caller's scope so that visibility checks
	// (e.g. private/protected methods) are evaluated from the correct context.
	// For example, call_user_func([$this, 'privateMethod']) from inside a class
	// should succeed because the caller has access to the private method.
	callerCtx := ctx.Parent(1)
	if callerCtx == nil {
		callerCtx = ctx
	}
	var callback phpv.Callable
	_, err := core.Expand(callerCtx, args, &callback)
	if err != nil {
		return nil, err
	}

	return ctx.CallZVal(callerCtx, callback, args[1:], nil)
}

// > func mixed call_user_func_array ( callable $callback , array $param_arr )
func fncCallUserFuncArray(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	// Resolve the callback in the caller's scope so that visibility checks
	// (e.g. private/protected methods) are evaluated from the correct context.
	callerCtx := ctx.Parent(1)
	if callerCtx == nil {
		callerCtx = ctx
	}
	var callback phpv.Callable
	var arrayArgs *phpv.ZArray
	_, err := core.Expand(callerCtx, args, &callback, &arrayArgs)
	if err != nil {
		return nil, err
	}

	var cbArgs []*phpv.ZVal
	for _, v := range arrayArgs.Iterate(ctx) {
		cbArgs = append(cbArgs, v)
	}
	return ctx.CallZVal(callerCtx, callback, cbArgs, nil)
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

// > func string get_class ([ object $object ] )
func stdGetClass(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var objectArg core.Optional[phpv.ZObject]
	_, err := core.Expand(ctx, args, &objectArg)
	if err != nil {
		return nil, err
	}

	ctx = ctx.Parent(1)

	object := objectArg.GetOrDefault(ctx.This())
	if object == nil {
		if ctx.Class() != nil {
			return ctx.Class().GetName().ZVal(), nil
		}

		return phpv.ZFalse.ZVal(), nil
	}

	// Use the original class (not CurrentClass from GetKin).
	// Return the raw internal name (including null byte + path for anonymous classes)
	// so that PHP code can parse it with strstr($name, "\0", true).
	if zo, ok := object.(*phpobj.ZObject); ok {
		if zc, ok := zo.Class.(*phpobj.ZClass); ok {
			return zc.Name.ZVal(), nil
		}
		return zo.Class.GetName().ZVal(), nil
	}
	if zc, ok := object.GetClass().(*phpobj.ZClass); ok {
		return zc.Name.ZVal(), nil
	}
	return object.GetClass().GetName().ZVal(), nil
}

// > func bool class_exists ( string $class_name [, bool $autoload = TRUE ] )
func stdClassExists(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var className phpv.ZString
	var autoloadArg *phpv.ZBool

	_, err := core.Expand(ctx, args, &className, &autoloadArg)
	if err != nil {
		return nil, err
	}

	autoload := true
	if autoloadArg != nil {
		autoload = bool(*autoloadArg)
	}

	cls, err := ctx.Global().GetClass(ctx, className, autoload)
	if err != nil {
		return phpv.ZFalse.ZVal(), nil
	}
	// class_exists returns false for interfaces, traits, and enums
	t := cls.GetType()
	if t.Has(phpv.ZClassTypeInterface) || t.Has(phpv.ZClassTypeTrait) || t.Has(phpv.ZClassTypeEnum) {
		return phpv.ZFalse.ZVal(), nil
	}
	return phpv.ZTrue.ZVal(), nil
}

// > func bool enum_exists ( string $enum [, bool $autoload = TRUE ] )
func stdEnumExists(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var className phpv.ZString
	var autoloadArg *phpv.ZBool

	_, err := core.Expand(ctx, args, &className, &autoloadArg)
	if err != nil {
		return nil, err
	}

	autoload := true
	if autoloadArg != nil {
		autoload = bool(*autoloadArg)
	}

	class, err := ctx.Global().GetClass(ctx, className, autoload)
	if err != nil {
		return phpv.ZFalse.ZVal(), nil
	}
	if class.GetType()&phpv.ZClassTypeEnum != 0 {
		return phpv.ZTrue.ZVal(), nil
	}
	return phpv.ZFalse.ZVal(), nil
}

// > func string get_parent_class ([ mixed $object ] )
func stdGetParentClass(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var objectArg *phpv.ZVal

	_, err := core.Expand(ctx, args, &objectArg)
	if err != nil {
		return nil, err
	}

	var class phpv.ZClass

	if objectArg == nil || objectArg.IsNull() {
		// Use current class context
		class = ctx.Class()
	} else if objectArg.GetType() == phpv.ZtString {
		// Class name as string
		class, err = ctx.Global().GetClass(ctx, objectArg.AsString(ctx), false)
		if err != nil {
			return phpv.ZFalse.ZVal(), nil
		}
	} else if objectArg.GetType() == phpv.ZtObject {
		obj := objectArg.AsObject(ctx)
		if obj == nil {
			return phpv.ZFalse.ZVal(), nil
		}
		class = obj.GetClass()
	} else {
		return phpv.ZFalse.ZVal(), nil
	}

	if class == nil {
		return phpv.ZFalse.ZVal(), nil
	}

	parent := class.GetParent()
	if parent == nil {
		return phpv.ZFalse.ZVal(), nil
	}
	parentName := parent.GetName()
	if parentName == "" {
		return phpv.ZFalse.ZVal(), nil
	}

	return parentName.ZVal(), nil
}

// > func bool is_a ( object $object , string $class_name [, bool $allow_string = FALSE ] )
func stdIsA(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var objectArg *phpv.ZVal
	var className phpv.ZString
	var allowStringArg *phpv.ZBool

	_, err := core.Expand(ctx, args, &objectArg, &className, &allowStringArg)
	if err != nil {
		return nil, err
	}

	allowString := false
	if allowStringArg != nil {
		allowString = bool(*allowStringArg)
	}

	var class phpv.ZClass

	if objectArg.GetType() == phpv.ZtObject {
		obj := objectArg.AsObject(ctx)
		if obj == nil {
			return phpv.ZFalse.ZVal(), nil
		}
		// Use original class, not CurrentClass from GetKin
		if zo, ok := obj.(*phpobj.ZObject); ok {
			class = zo.Class
		} else {
			class = obj.GetClass()
		}
	} else if allowString && objectArg.GetType() == phpv.ZtString {
		class, err = ctx.Global().GetClass(ctx, objectArg.AsString(ctx), false)
		if err != nil {
			return phpv.ZFalse.ZVal(), nil
		}
	} else {
		return phpv.ZFalse.ZVal(), nil
	}

	// Check using InstanceOf which handles both extends and implements
	targetClass, err := ctx.Global().GetClass(ctx, className, false)
	if err == nil {
		return phpv.ZBool(class.InstanceOf(targetClass)).ZVal(), nil
	}

	return phpv.ZFalse.ZVal(), nil
}

// > func bool is_subclass_of ( mixed $object , string $class_name [, bool $allow_string = TRUE ] )
func stdIsSubclassOf(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var objectArg *phpv.ZVal
	var className phpv.ZString
	var allowStringArg *phpv.ZBool

	_, err := core.Expand(ctx, args, &objectArg, &className, &allowStringArg)
	if err != nil {
		return nil, err
	}

	allowString := true
	if allowStringArg != nil {
		allowString = bool(*allowStringArg)
	}

	var class phpv.ZClass

	if objectArg.GetType() == phpv.ZtObject {
		obj := objectArg.AsObject(ctx)
		if obj == nil {
			return phpv.ZFalse.ZVal(), nil
		}
		class = obj.GetClass()
	} else if allowString && objectArg.GetType() == phpv.ZtString {
		class, err = ctx.Global().GetClass(ctx, objectArg.AsString(ctx), false)
		if err != nil {
			return phpv.ZFalse.ZVal(), nil
		}
	} else {
		return phpv.ZFalse.ZVal(), nil
	}

	// Skip the first class itself, check parents only
	target := className.ToLower()
	class = class.GetParent()
	for class != nil && class.GetName() != "" {
		if class.GetName().ToLower() == target {
			return phpv.ZTrue.ZVal(), nil
		}
		class = class.GetParent()
	}

	return phpv.ZFalse.ZVal(), nil
}

// > func array get_declared_classes ( void )
func stdGetDeclaredClasses(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	result := phpv.NewZArray()

	classes := ctx.Global().GetDeclaredClasses()
	for _, name := range classes {
		result.OffsetSet(ctx, nil, name.ZVal())
	}

	return result.ZVal(), nil
}

// > func array get_class_methods ( mixed $class_name )
func stdGetClassMethods(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var classArg *phpv.ZVal
	_, err := core.Expand(ctx, args, &classArg)
	if err != nil {
		return nil, err
	}

	var class phpv.ZClass

	if classArg.GetType() == phpv.ZtObject {
		obj := classArg.AsObject(ctx)
		if obj == nil {
			return phpv.ZNULL.ZVal(), nil
		}
		class = obj.GetClass()
	} else {
		class, err = ctx.Global().GetClass(ctx, classArg.AsString(ctx), true)
		if err != nil {
			return phpv.ZNULL.ZVal(), nil
		}
	}

	result := phpv.NewZArray()
	for _, m := range class.GetMethods() {
		result.OffsetSet(ctx, nil, phpv.ZString(m.Name).ZVal())
	}
	return result.ZVal(), nil
}

// > func bool interface_exists ( string $interface_name [, bool $autoload = true ] )
func stdInterfaceExists(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var name phpv.ZString
	var autoloadArg *phpv.ZBool
	_, err := core.Expand(ctx, args, &name, &autoloadArg)
	if err != nil {
		return nil, err
	}

	autoload := true
	if autoloadArg != nil {
		autoload = bool(*autoloadArg)
	}

	class, err := ctx.Global().GetClass(ctx, name, autoload)
	if err != nil || class == nil {
		return phpv.ZFalse.ZVal(), nil
	}

	if class.GetType() == phpv.ZClassTypeInterface {
		return phpv.ZTrue.ZVal(), nil
	}
	return phpv.ZFalse.ZVal(), nil
}

// > func array get_object_vars ( object $obj )
func stdGetObjectVars(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var obj *phpv.ZVal
	_, err := core.Expand(ctx, args, &obj)
	if err != nil {
		return nil, err
	}

	if obj.GetType() != phpv.ZtObject {
		return phpv.ZFalse.ZVal(), nil
	}

	o := obj.AsObject(ctx)
	if o == nil {
		return phpv.ZFalse.ZVal(), nil
	}

	// Use the calling scope's class for visibility checks.
	// Use Parent(1) since we're inside the get_object_vars function context.
	callerCtx := ctx.Parent(1)
	var scope phpv.ZClass
	if callerCtx != nil {
		scope = callerCtx.Class()
	}

	result := phpv.NewZArray()
	if scopedIterator, ok := o.(interface {
		NewIteratorInScope(phpv.ZClass) phpv.ZIterator
	}); ok {
		it := scopedIterator.NewIteratorInScope(scope)
		for ; it.Valid(ctx); it.Next(ctx) {
			k, err := it.Key(ctx)
			if err != nil {
				continue
			}
			v, err := it.Current(ctx)
			if err != nil {
				continue
			}
			result.OffsetSet(ctx, k, v)
		}
	} else {
		it := o.NewIterator()
		for ; it.Valid(ctx); it.Next(ctx) {
			k, err := it.Key(ctx)
			if err != nil {
				continue
			}
			v, err := it.Current(ctx)
			if err != nil {
				continue
			}
			result.OffsetSet(ctx, k, v)
		}
	}
	return result.ZVal(), nil
}

// > func bool property_exists ( object|string $object_or_class , string $property )
func stdPropertyExists(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	if len(args) < 2 {
		return nil, phpobj.ThrowError(ctx, phpobj.ArgumentCountError, fmt.Sprintf("property_exists() expects exactly 2 arguments, %d given", len(args)))
	}

	propName := args[1].AsString(ctx)

	var class phpv.ZClass
	if args[0].GetType() == phpv.ZtObject {
		class = args[0].AsObject(ctx).GetClass()
	} else if args[0].GetType() == phpv.ZtString {
		className := args[0].AsString(ctx)
		var err error
		class, err = ctx.Global().GetClass(ctx, className, true)
		if err != nil {
			return phpv.ZFalse.ZVal(), nil
		}
	} else {
		return nil, phpobj.ThrowError(ctx, phpobj.TypeError, fmt.Sprintf("property_exists(): Argument #1 ($object_or_class) must be of type object|string, %s given", args[0].GetType().TypeName()))
	}

	// Check if the property is declared on the class
	if _, found := class.GetProp(propName); found {
		return phpv.ZTrue.ZVal(), nil
	}

	// For objects, also check dynamic properties
	if args[0].GetType() == phpv.ZtObject {
		if obj, ok := args[0].Value().(*phpobj.ZObject); ok {
			if _, found := obj.HashTable().GetStringB(propName); found {
				return phpv.ZTrue.ZVal(), nil
			}
		}
	}

	return phpv.ZFalse.ZVal(), nil
}

// > func array get_class_vars ( string $class_name )
func stdGetClassVars(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	if len(args) < 1 {
		return nil, phpobj.ThrowError(ctx, phpobj.ArgumentCountError, "get_class_vars() expects exactly 1 argument, 0 given")
	}

	className := args[0].AsString(ctx)
	class, err := ctx.Global().GetClass(ctx, className, true)
	if err != nil {
		return nil, phpobj.ThrowError(ctx, phpobj.Error, fmt.Sprintf("Class \"%s\" does not exist", className))
	}

	result := phpv.NewZArray()

	// Get the caller's class scope for visibility checks
	callerCtx := ctx.Parent(1)
	var scope phpv.ZClass
	if callerCtx != nil {
		scope = callerCtx.Class()
	}

	// Iterate over class properties and check visibility
	if zc, ok := class.(*phpobj.ZClass); ok {
		for _, prop := range zc.Props {
			// Check visibility
			if prop.Modifiers.IsPrivate() {
				if scope == nil || scope.GetName() != class.GetName() {
					continue
				}
			} else if prop.Modifiers.IsProtected() {
				if scope == nil || (!scope.InstanceOf(class) && !class.InstanceOf(scope)) {
					continue
				}
			}
			// Get default value
			def := phpv.ZNULL.ZVal()
			if prop.Default != nil {
				def = prop.Default.ZVal()
			}
			result.OffsetSet(ctx, prop.VarName.ZVal(), def)
		}
	}

	return result.ZVal(), nil
}

// > func ?array error_get_last ( void )
func stdErrorGetLast(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	// TODO: implement proper error tracking
	return phpv.ZNULL.ZVal(), nil
}

// > func array get_defined_constants ( bool $categorize = false )
func stdGetDefinedConstants(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	// TODO: proper implementation with categorization
	result := phpv.NewZArray()
	return result.ZVal(), nil
}
