package standard

import (
	"errors"
	"fmt"
	"net"
	"strings"
	"unicode"

	"github.com/MagicalTux/goro/core"
	"github.com/MagicalTux/goro/core/logopt"
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
	isObject := false
	switch objectArg.GetType() {
	case phpv.ZtString:
		className := objectArg.AsString(ctx)
		class, err = ctx.Global().GetClass(ctx, className, true)
		if err != nil {
			return phpv.ZFalse.ZVal(), nil
		}
	case phpv.ZtObject:
		obj := objectArg.AsObject(ctx)
		class = obj.GetClass()
		isObject = true
	default:
		return nil, phpobj.ThrowError(ctx, phpobj.TypeError,
			fmt.Sprintf("method_exists(): Argument #1 ($object_or_class) must be of type object|string, %s given",
				phpv.ZValTypeNameDetailed(objectArg)))
	}
	m, ok := class.GetMethod(methodName)

	// When called with a string class name (not an object), private methods
	// inherited from a parent class should not be considered existing.
	if ok && !isObject && m != nil && m.Modifiers.IsPrivate() {
		// Check if the method is defined in the requested class itself
		if m.Class != nil && m.Class.GetName() != class.GetName() {
			ok = false
		}
	}

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

// > func bool settype ( mixed &$var , string $type )
func fncSettype(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	if len(args) < 2 {
		return nil, ctx.Errorf("settype() expects exactly 2 arguments, %d given", len(args))
	}

	typeName := args[1].AsString(ctx)
	var newVal *phpv.ZVal
	var err error

	switch string(typeName) {
	case "int", "integer":
		newVal, err = args[0].As(ctx, phpv.ZtInt)
	case "float", "double":
		newVal, err = args[0].As(ctx, phpv.ZtFloat)
	case "string":
		newVal, err = args[0].As(ctx, phpv.ZtString)
	case "bool", "boolean":
		newVal, err = args[0].As(ctx, phpv.ZtBool)
	case "array":
		newVal, err = args[0].As(ctx, phpv.ZtArray)
	case "object":
		newVal, err = args[0].As(ctx, phpv.ZtObject)
	case "null":
		newVal, err = args[0].As(ctx, phpv.ZtNull)
	case "resource":
		return nil, phpobj.ThrowError(ctx, phpobj.Error, "Cannot convert to resource type")
	default:
		return nil, phpobj.ThrowError(ctx, phpobj.ValueError, "settype(): Argument #2 ($type) must be a valid type")
	}

	if err != nil {
		return phpv.ZFalse.ZVal(), nil
	}

	args[0].Set(newVal)
	return phpv.ZTrue.ZVal(), nil
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

	// call_user_func always passes arguments by value. Strip the Name
	// from each argument so that the callee's by-ref parameter handling
	// in callZValImpl sees an unnamed value and emits the appropriate
	// "must be passed by reference, value given" warning instead of
	// silently creating a reference.
	cbArgs := make([]*phpv.ZVal, len(args)-1)
	for i, a := range args[1:] {
		cbArgs[i] = a.Dup()
		cbArgs[i].Name = nil
	}
	return ctx.CallZVal(callerCtx, callback, cbArgs, nil)
}

// > func mixed call_user_func_array ( callable $callback , array $param_arr )
func fncCallUserFuncArray(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	// Resolve the callback in the caller's scope so that visibility checks
	// (e.g. private/protected methods) are evaluated from the correct context.
	callerCtx := ctx.Parent(1)
	if callerCtx == nil {
		callerCtx = ctx
	}
	// Validate second argument is an array before expansion
	if len(args) >= 2 && args[1] != nil && args[1].GetType() != phpv.ZtArray {
		return nil, phpobj.ThrowError(ctx, phpobj.TypeError,
			fmt.Sprintf("call_user_func_array(): Argument #2 ($args) must be of type array, %s given",
				phpv.ZValTypeName(args[1])))
	}
	var callback phpv.Callable
	var arrayArgs *phpv.ZArray
	_, err := core.Expand(callerCtx, args, &callback, &arrayArgs)
	if err != nil {
		return nil, err
	}

	// call_user_func_array passes arguments by value unless the array
	// element is already a reference (&$var). For non-reference values,
	// strip the Name so by-ref parameter handling emits the appropriate
	// "must be passed by reference, value given" warning.
	// For actual references, preserve them so they pass through.
	var cbArgs []*phpv.ZVal
	for _, v := range arrayArgs.Iterate(ctx) {
		if v.IsRef() {
			cbArgs = append(cbArgs, v)
		} else {
			dup := v.Dup()
			dup.Name = nil
			cbArgs = append(cbArgs, dup)
		}
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
	callerCtx := ctx.Parent(1)
	if callerCtx == nil {
		callerCtx = ctx
	}

	if len(args) == 0 {
		// PHP 8.0+: calling get_class() without arguments is deprecated
		callerCtx.Deprecated("Calling get_class() without arguments is deprecated", logopt.NoFuncName(true))
		if callerCtx.This() != nil {
			object := callerCtx.This()
			if zo, ok := object.(*phpobj.ZObject); ok {
				if zc, ok := zo.Class.(*phpobj.ZClass); ok {
					return zc.Name.ZVal(), nil
				}
				return zo.Class.GetName().ZVal(), nil
			}
			return object.GetClass().GetName().ZVal(), nil
		}
		if callerCtx.Class() != nil {
			return callerCtx.Class().GetName().ZVal(), nil
		}
		return phpv.ZFalse.ZVal(), nil
	}

	// With argument, must be object type
	arg := args[0]
	if arg.GetType() != phpv.ZtObject {
		return nil, phpobj.ThrowError(ctx, phpobj.TypeError,
			fmt.Sprintf("get_class(): Argument #1 ($object) must be of type object, %s given",
				phpv.ZValTypeNameDetailed(arg)))
	}

	object := arg.AsObject(ctx)
	if object == nil {
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

	if objectArg == nil || (len(args) == 0) {
		// No argument: use current class context
		class = ctx.Class()
	} else if objectArg.GetType() == phpv.ZtString {
		// Class name as string - try to resolve it (with autoload)
		class, err = ctx.Global().GetClass(ctx, objectArg.AsString(ctx), true)
		if err != nil {
			// Invalid class name - throw TypeError
			return nil, phpobj.ThrowError(ctx, phpobj.TypeError,
				fmt.Sprintf("get_parent_class(): Argument #1 ($object_or_class) must be an object or a valid class name, string given"))
		}
	} else if objectArg.GetType() == phpv.ZtObject {
		obj := objectArg.AsObject(ctx)
		if obj == nil {
			return phpv.ZFalse.ZVal(), nil
		}
		class = obj.GetClass()
	} else {
		// PHP 8: TypeError for non-object/non-string types
		return nil, phpobj.ThrowError(ctx, phpobj.TypeError,
			fmt.Sprintf("get_parent_class(): Argument #1 ($object_or_class) must be an object or a valid class name, %s given",
				phpv.ZValTypeNameDetailed(objectArg)))
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
		class, err = ctx.Global().GetClass(ctx, objectArg.AsString(ctx), true)
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
		class, err = ctx.Global().GetClass(ctx, objectArg.AsString(ctx), true)
		if err != nil {
			return phpv.ZFalse.ZVal(), nil
		}
	} else {
		// Non-object/non-string types: just return false (PHP doesn't throw TypeError here)
		return phpv.ZFalse.ZVal(), nil
	}

	// Resolve the target class name to an actual class (handles class_alias)
	targetClass, targetErr := ctx.Global().GetClass(ctx, className, true)

	if targetErr == nil && !phpv.IsNilClass(targetClass) {
		// Use InstanceOf to check, but skip the first class itself (is_subclass_of excludes self)
		parent := class.GetParent()
		for parent != nil && parent.GetName() != "" {
			if parent == targetClass || parent.InstanceOf(targetClass) {
				return phpv.ZTrue.ZVal(), nil
			}
			parent = parent.GetParent()
		}
		// Also check implemented interfaces
		if class.InstanceOf(targetClass) && class != targetClass {
			return phpv.ZTrue.ZVal(), nil
		}
	} else {
		// Fallback: name-based matching
		target := className.ToLower()
		parent := class.GetParent()
		for parent != nil && parent.GetName() != "" {
			if parent.GetName().ToLower() == target {
				return phpv.ZTrue.ZVal(), nil
			}
			parent = parent.GetParent()
		}
	}

	return phpv.ZFalse.ZVal(), nil
}

// > func array get_declared_classes ( void )
func stdGetDeclaredClasses(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	result := phpv.NewZArray()

	classes := ctx.Global().GetDeclaredClasses()
	for _, name := range classes {
		class, err := ctx.Global().GetClass(ctx, name, false)
		if err != nil {
			continue
		}
		// Only include real classes, not interfaces, traits, or enums
		classType := class.GetType()
		if classType&phpv.ZClassTypeInterface != 0 || classType&phpv.ZClassTypeTrait != 0 || classType&phpv.ZClassTypeEnum != 0 {
			continue
		}
		result.OffsetSet(ctx, nil, name.ZVal())
	}

	return result.ZVal(), nil
}

// > func array get_declared_interfaces ( void )
func stdGetDeclaredInterfaces(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	result := phpv.NewZArray()
	classes := ctx.Global().GetDeclaredClasses()
	for _, name := range classes {
		class, err := ctx.Global().GetClass(ctx, name, false)
		if err != nil {
			continue
		}
		if class.GetType()&phpv.ZClassTypeInterface != 0 {
			result.OffsetSet(ctx, nil, name.ZVal())
		}
	}
	return result.ZVal(), nil
}

// > func array get_declared_traits ( void )
func stdGetDeclaredTraits(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	result := phpv.NewZArray()
	classes := ctx.Global().GetDeclaredClasses()
	for _, name := range classes {
		class, err := ctx.Global().GetClass(ctx, name, false)
		if err != nil {
			continue
		}
		if class.GetType()&phpv.ZClassTypeTrait != 0 {
			result.OffsetSet(ctx, nil, name.ZVal())
		}
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

	switch classArg.GetType() {
	case phpv.ZtObject:
		obj := classArg.AsObject(ctx)
		if obj == nil {
			return phpv.ZNULL.ZVal(), nil
		}
		class = obj.GetClass()
	case phpv.ZtString:
		class, err = ctx.Global().GetClass(ctx, classArg.AsString(ctx), true)
		if err != nil {
			return nil, phpobj.ThrowError(ctx, phpobj.TypeError,
				"get_class_methods(): Argument #1 ($object_or_class) must be an object or a valid class name, string given")
		}
	default:
		return nil, phpobj.ThrowError(ctx, phpobj.TypeError,
			fmt.Sprintf("get_class_methods(): Argument #1 ($object_or_class) must be an object or a valid class name, %s given",
				phpv.ZValTypeNameDetailed(classArg)))
	}

	// Determine visibility context: which class is the caller?
	callerCtx := ctx.Parent(1)
	var callerClass phpv.ZClass
	if callerCtx != nil {
		callerClass = callerCtx.Class()
	}

	// Get methods in PHP declaration order using GetMethodsOrdered
	var methods []*phpv.ZClassMethod
	if zc, ok := class.(*phpobj.ZClass); ok {
		methods = zc.GetMethodsOrdered()
	} else {
		// Fallback for non-ZClass types
		for _, m := range class.GetMethods() {
			methods = append(methods, m)
		}
	}

	// Filter by visibility
	result := phpv.NewZArray()
	for _, m := range methods {
		// Filter out private methods
		if m.Modifiers.IsPrivate() {
			declaringClass := m.Class
			if declaringClass == nil {
				declaringClass = class
			}
			if callerClass == nil || callerClass.GetName() != declaringClass.GetName() {
				continue
			}
		}
		// Filter out protected methods when called from outside the hierarchy
		if m.Modifiers.IsProtected() {
			if callerClass == nil || (!callerClass.InstanceOf(class) && !class.InstanceOf(callerClass)) {
				continue
			}
		}
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

// > func bool trait_exists ( string $trait_name [, bool $autoload = true ] )
func stdTraitExists(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
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

	if class.GetType() == phpv.ZClassTypeTrait {
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
				// Resolve CompileDelayed values (e.g., constants used as default values)
				if cd, ok := prop.Default.(*phpv.CompileDelayed); ok {
					// Set compiling class so self:: resolves correctly
					prevCompiling := ctx.Global().GetCompilingClass()
					ctx.Global().SetCompilingClass(zc)
					resolved, err := cd.Run(ctx)
					ctx.Global().SetCompilingClass(prevCompiling)
					if err != nil {
						return nil, err
					}
					prop.Default = resolved.Value()
					def = resolved
				} else {
					def = prop.Default.ZVal()
				}
			}
			result.OffsetSet(ctx, prop.VarName.ZVal(), def)
		}
	}

	return result.ZVal(), nil
}

// > func ?array error_get_last ( void )
func stdErrorGetLast(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	g, ok := ctx.Global().(*phpctx.Global)
	if !ok || g.LastError == nil {
		return phpv.ZNULL.ZVal(), nil
	}

	err := g.LastError
	result := phpv.NewZArray()
	result.OffsetSet(ctx, phpv.ZString("type").ZVal(), phpv.ZInt(err.Code).ZVal())
	result.OffsetSet(ctx, phpv.ZString("message").ZVal(), phpv.ZString(err.Err.Error()).ZVal())
	file := ""
	line := 0
	if err.Loc != nil {
		file = err.Loc.Filename
		line = err.Loc.Line
	}
	result.OffsetSet(ctx, phpv.ZString("file").ZVal(), phpv.ZString(file).ZVal())
	result.OffsetSet(ctx, phpv.ZString("line").ZVal(), phpv.ZInt(line).ZVal())
	return result.ZVal(), nil
}

// > func void error_clear_last ( void )
func stdErrorClearLast(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	if g, ok := ctx.Global().(*phpctx.Global); ok {
		g.LastError = nil
	}
	return nil, nil
}

// > func array get_defined_constants ( bool $categorize = false )
func stdGetDefinedConstants(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var categorize *phpv.ZBool
	_, err := core.Expand(ctx, args, &categorize)
	if err != nil {
		return nil, err
	}

	g, ok := ctx.Global().(*phpctx.Global)
	if !ok {
		return phpv.NewZArray().ZVal(), nil
	}

	constants := g.GetAllConstants()

	if categorize != nil && *categorize {
		// Return categorized constants (simplified: all in "user" category)
		userConsts := phpv.NewZArray()
		for name, val := range constants {
			userConsts.OffsetSet(ctx, phpv.ZString(name).ZVal(), val.ZVal())
		}
		result := phpv.NewZArray()
		result.OffsetSet(ctx, phpv.ZString("user").ZVal(), userConsts.ZVal())
		return result.ZVal(), nil
	}

	// Return flat list of all constants
	result := phpv.NewZArray()
	for name, val := range constants {
		result.OffsetSet(ctx, phpv.ZString(name).ZVal(), val.ZVal())
	}
	return result.ZVal(), nil
}
