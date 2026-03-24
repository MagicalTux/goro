package core

import (
	"fmt"
	"strings"
	"time"
	"unsafe"

	"github.com/MagicalTux/goro/core/logopt"
	"github.com/MagicalTux/goro/core/phpobj"
	"github.com/MagicalTux/goro/core/phpv"
)

// normalizeConstantNamespace normalizes the namespace portion of a constant name
// to lowercase. In PHP, the namespace part is case-insensitive.
// e.g., "NS1\ns2\const1" -> "ns1\ns2\const1"
func normalizeConstantNamespace(name string) string {
	idx := strings.LastIndex(name, "\\")
	if idx < 0 {
		return name // no namespace
	}
	return strings.ToLower(name[:idx]) + name[idx:]
}

// > const
const (
	COUNT_NORMAL phpv.ZInt = iota
	COUNT_RECURSIVE
)

// > func int strlen ( string $string )
func fncStrlen(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	// PHP 8.1+ deprecation: passing null to non-nullable string parameter
	if len(args) > 0 && args[0] != nil && args[0].GetType() == phpv.ZtNull {
		ctx.Deprecated("strlen(): Passing null to parameter #1 ($string) of type string is deprecated", logopt.NoFuncName(true))
	}
	var s phpv.ZString
	_, err := Expand(ctx, args, &s)
	if err != nil {
		return nil, err
	}

	return phpv.ZInt(len(s)).ZVal(), nil
}

// > func int error_reporting ([ int $level ] )
func fncErrorReporting(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	oldVal := ctx.GetConfig("error_reporting", phpv.ZInt(0).ZVal())

	if len(args) >= 1 && args[0] != nil && !args[0].IsNull() {
		switch args[0].GetType() {
		case phpv.ZtObject, phpv.ZtArray:
			return nil, phpobj.ThrowError(ctx, phpobj.TypeError,
				fmt.Sprintf("error_reporting(): Argument #1 ($error_level) must be of type ?int, %s given", phpv.ZValTypeName(args[0])))
		}
		var level phpv.ZInt
		_, err := Expand(ctx, args, &level)
		if err != nil {
			return nil, err
		}
		ctx.Global().SetLocalConfig("error_reporting", level.ZVal())
	}

	return oldVal, nil
}

// > func bool define ( string $name , mixed $value )
func fncDefine(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	if len(args) < 2 {
		return nil, phpobj.ThrowError(ctx, phpobj.ArgumentCountError, fmt.Sprintf("define() expects exactly 2 arguments, %d given", len(args)))
	}
	// Check argument type - PHP uses coercion mode for internal functions,
	// so scalars (int, float, bool) are coerced to string.
	// Only objects and arrays cause a TypeError.
	switch args[0].GetType() {
	case phpv.ZtObject, phpv.ZtArray:
		return nil, phpobj.ThrowError(ctx, phpobj.TypeError, fmt.Sprintf("define(): Argument #1 ($constant_name) must be of type string, %s given", phpv.ZValTypeName(args[0])))
	}
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

	// Normalize namespace part of constant name to lowercase
	// In PHP, namespaced constants have case-insensitive namespace portions
	name = phpv.ZString(normalizeConstantNamespace(string(name)))

	g := ctx.Global()

	// __COMPILER_HALT_OFFSET__ is a reserved "magic" constant.
	// PHP always warns when trying to define() it, even if it hasn't been set by __halt_compiler().
	if name == "__COMPILER_HALT_OFFSET__" {
		if err := ctx.Warn("Constant %s already defined, this will be an error in PHP 9", name, logopt.NoFuncName(true)); err != nil {
			return nil, err
		}
		return phpv.ZBool(false).ZVal(), nil
	}

	ok := g.ConstantSet(name, value.Value())
	if !ok {
		if err := ctx.Warn("Constant %s already defined, this will be an error in PHP 9", name, logopt.NoFuncName(true)); err != nil {
			return nil, err
		}
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

	// Strip leading backslash (global namespace prefix)
	if len(name) > 0 && name[0] == '\\' {
		name = name[1:]
	}

	g := ctx.Global()

	// Check for class constant (ClassName::CONST_NAME)
	if idx := strings.Index(string(name), "::"); idx != -1 {
		className := phpv.ZString(name[:idx])
		constName := phpv.ZString(name[idx+2:])
		class, err := g.GetClass(ctx, className, false)
		if err != nil {
			return phpv.ZBool(false).ZVal(), nil
		}
		if zc, ok := class.(*phpobj.ZClass); ok {
			_, exists := zc.Const[constName]
			return phpv.ZBool(exists).ZVal(), nil
		}
		return phpv.ZBool(false).ZVal(), nil
	}

	// Normalize namespace part of constant name to lowercase
	normalizedName := phpv.ZString(normalizeConstantNamespace(string(name)))
	_, ok := g.ConstantGet(normalizedName)

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

	// Validate mode
	if mode != COUNT_NORMAL && mode != COUNT_RECURSIVE {
		return nil, phpobj.ThrowError(ctx, phpobj.ValueError,
			"count(): Argument #2 ($mode) must be either COUNT_NORMAL or COUNT_RECURSIVE")
	}

	if mode == COUNT_RECURSIVE && countable.GetType() == phpv.ZtArray {
		visisted := map[uintptr]struct{}{}
		count, err := recursiveCount(ctx, countable.AsArray(ctx), visisted)
		if err != nil {
			return nil, ctx.FuncError(err)
		}
		return phpv.ZInt(count).ZVal(), nil
	}

	// For arrays, use the ZCountable interface (ZArray implements it)
	if countable.GetType() == phpv.ZtArray {
		if v, ok := countable.Value().(phpv.ZCountable); ok {
			return v.Count(ctx).ZVal(), nil
		}
	}

	// For objects implementing the PHP Countable interface, call their count() method
	if countable.GetType() == phpv.ZtObject {
		if obj, ok := countable.Value().(*phpobj.ZObject); ok {
			// Check if the class implements the PHP Countable interface
			if implementsCountable(obj.GetClass()) {
				if m, hasCount := obj.GetClass().GetMethod("count"); hasCount {
					result, err := ctx.CallZVal(ctx, m.Method, nil, obj)
					if err != nil {
						return nil, err
					}
					if result != nil {
						return phpv.ZInt(result.AsInt(ctx)).ZVal(), nil
					}
					return phpv.ZInt(0).ZVal(), nil
				}
			}
		}
	}

	// PHP 8.0+: TypeError for non-countable types
	typeName := phpv.ZValTypeNameDetailed(countable)
	return nil, phpobj.ThrowError(ctx, phpobj.TypeError,
		fmt.Sprintf("count(): Argument #1 ($value) must be of type Countable|array, %s given", typeName))
}

// implementsCountable checks if a class implements the Countable interface
func implementsCountable(c phpv.ZClass) bool {
	if c == nil {
		return false
	}
	if strings.EqualFold(string(c.GetName()), "countable") {
		return true
	}
	// Check concrete *phpobj.ZClass for Implementations and Extends fields
	if zc, ok := c.(*phpobj.ZClass); ok {
		for _, impl := range zc.Implementations {
			if implementsCountable(impl) {
				return true
			}
		}
		if zc.Extends != nil {
			return implementsCountable(zc.Extends)
		}
	}
	return false
}

func recursiveCount(ctx phpv.Context, array *phpv.ZArray, visited map[uintptr]struct{}) (int, error) {
	var err error
	ptr := uintptr(unsafe.Pointer(array))
	if _, seen := visited[ptr]; seen {
		if err = ctx.Warn("count(): Recursion detected", logopt.NoFuncName(true)); err != nil {
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

// iniNotFoundSentinel is a sentinel value used by ini_get to detect unknown settings.
var iniNotFoundSentinel = &phpv.ZVal{}

// > func string ini_get ( string $varname)
func fncIniGet(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var varName phpv.ZString
	_, err := Expand(ctx, args, &varName)
	if err != nil {
		return nil, err
	}

	// PHP returns false if the configuration option doesn't exist
	value := ctx.Global().GetConfig(varName, iniNotFoundSentinel)
	if value == iniNotFoundSentinel {
		return phpv.ZFalse.ZVal(), nil
	}
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

	// Check open_basedir for path-related INI settings
	// "syslog" and "" are special values that bypass the check
	if varName == "error_log" && string(newValue) != "" && string(newValue) != "syslog" {
		if err := ctx.Global().CheckOpenBasedir(ctx, string(newValue), "ini_set"); err != nil {
			return phpv.ZFalse.ZVal(), nil
		}
	}

	// zend.assertions: cannot completely enable/disable at runtime
	if varName == "zend.assertions" {
		newInt := newValue.ZVal().AsInt(ctx)
		oldVal := ctx.GetConfig("zend.assertions", phpv.ZInt(1).ZVal()).AsInt(ctx)
		// Cannot switch between enabled (1) and completely disabled (-1) at runtime
		if (newInt == -1 && oldVal != -1) || (newInt != -1 && oldVal == -1) {
			ctx.Warn("zend.assertions may be completely enabled or disabled only in php.ini", logopt.NoFuncName(true))
			return phpv.ZFalse.ZVal(), nil
		}
	}

	// Validate date.timezone value
	if varName == "date.timezone" && string(newValue) != "" {
		_, err := time.LoadLocation(string(newValue))
		if err != nil {
			oldTz := ctx.GetConfig("date.timezone", phpv.ZString("UTC").ZVal()).String()
			ctx.Warn("ini_set(): Invalid date.timezone value '%s', using '%s' instead", newValue, oldTz, logopt.NoFuncName(true))
			return phpv.ZFalse.ZVal(), nil
		}
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

// fncClone implements the clone() function (PHP 8.5+).
// clone(object $object, ?array $withProperties = null): object
func fncClone(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	if len(args) < 1 {
		return nil, phpobj.ThrowError(ctx, phpobj.TypeError, "clone() expects at least 1 argument, 0 given")
	}

	v := args[0]
	if v.GetType() != phpv.ZtObject {
		return nil, phpobj.ThrowError(ctx, phpobj.TypeError, fmt.Sprintf("clone(): Argument #1 ($object) must be of type object, %s given", v.GetType().TypeName()))
	}

	obj := v.Value().(phpv.ZObject)

	// Enums cannot be cloned
	if obj.GetClass().GetType()&phpv.ZClassTypeEnum != 0 {
		return nil, phpobj.ThrowError(ctx, phpobj.Error, fmt.Sprintf("Trying to clone an uncloneable object of class %s", obj.GetClass().GetName()))
	}

	// Check __clone visibility
	if m, ok := obj.GetClass().GetMethod("__clone"); ok {
		if m.Modifiers.IsPrivate() || m.Modifiers.IsProtected() {
			callerClass := ctx.Class()
			if m.Modifiers.IsPrivate() {
				if callerClass == nil || callerClass.GetName() != obj.GetClass().GetName() {
					return nil, phpobj.ThrowError(ctx, phpobj.Error, fmt.Sprintf("Call to private method %s::__clone() from global scope", obj.GetClass().GetName()))
				}
			} else {
				// protected
				if callerClass == nil || (!callerClass.InstanceOf(obj.GetClass()) && !obj.GetClass().InstanceOf(callerClass)) {
					scope := "global scope"
					if callerClass != nil {
						scope = fmt.Sprintf("scope %s", callerClass.GetName())
					}
					return nil, phpobj.ThrowError(ctx, phpobj.Error, fmt.Sprintf("Call to protected method %s::__clone() from %s", obj.GetClass().GetName(), scope))
				}
			}
		}
	}

	// Validate second argument type if provided
	if len(args) > 1 && args[1] != nil && !args[1].IsNull() {
		if args[1].GetType() != phpv.ZtArray {
			return nil, phpobj.ThrowError(ctx, phpobj.TypeError, fmt.Sprintf("clone(): Argument #2 ($withProperties) must be of type array, %s given", args[1].GetType().TypeName()))
		}
	}

	cloned, err := obj.Clone(ctx)
	if err != nil {
		return nil, err
	}

	// Apply withProperties
	if len(args) > 1 && args[1] != nil && !args[1].IsNull() && args[1].GetType() == phpv.ZtArray {
		arr := args[1].AsArray(ctx)
		for k, val := range arr.Iterate(ctx) {
			keyStr := k.AsString(ctx)
			err = cloned.ObjectSet(ctx, keyStr, val.ZVal())
			if err != nil {
				return nil, err
			}
		}
	}

	return cloned.ZVal(), nil
}
