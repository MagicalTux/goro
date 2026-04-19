package phpobj

import (
	"fmt"

	"github.com/KarpelesLab/goro/core/logopt"
	"github.com/KarpelesLab/goro/core/phpv"
)

func init() {
	phpv.NewStdClassFunc = func(ctx phpv.Context) (phpv.ZObject, error) {
		return NewZObject(ctx, StdClass)
	}
}

// > class StdClass
var StdClass = &ZClass{
	Name: "stdClass",
}

// incompleteClassName returns the original class name from a __PHP_Incomplete_Class object.
// Returns "unknown" if the class name was not stored (e.g. when created directly with new).
func incompleteClassName(o phpv.ZObject) string {
	if cnVal, ok := o.HashTable().GetStringB("__PHP_Incomplete_Class_Name"); ok && cnVal != nil && !cnVal.IsNull() {
		return string(cnVal.Value().(phpv.ZString))
	}
	return "unknown"
}

// incompleteClassWarnFuncName returns "main" if at global scope, otherwise the current function name.
// This matches PHP behavior where property access on __PHP_Incomplete_Class emits "main():" prefix.
func incompleteClassWarnFuncName(ctx phpv.Context) string {
	if name := ctx.GetFuncName(); name != "" {
		return name
	}
	return "main"
}

// > class __PHP_Incomplete_Class
var IncompleteClass = &ZClass{
	Name: "__PHP_Incomplete_Class",
	H: &phpv.ZClassHandlers{
		HandlePropGetEager: true,
		HandlePropSet: func(ctx phpv.Context, o phpv.ZObject, key phpv.ZString, value *phpv.ZVal) (bool, error) {
			// Allow setting __PHP_Incomplete_Class_Name (used internally by unserialize).
			if key == "__PHP_Incomplete_Class_Name" {
				return false, nil // fall through to normal handling
			}
			cn := incompleteClassName(o)
			return true, ThrowError(ctx, Error, fmt.Sprintf(
				"The script tried to modify a property on an incomplete object. Please ensure that the class definition \"%s\" of the object you are trying to operate on was loaded _before_ unserialize() gets called or provide an autoloader to load the class definition",
				cn))
		},
		HandlePropGet: func(ctx phpv.Context, o phpv.ZObject, key phpv.ZString) (*phpv.ZVal, error) {
			// Allow reading __PHP_Incomplete_Class_Name (used internally).
			if key == "__PHP_Incomplete_Class_Name" {
				return nil, nil // fall through to normal handling
			}
			cn := incompleteClassName(o)
			funcName := incompleteClassWarnFuncName(ctx)
			// Use NoFuncName to suppress automatic prepending; we include it manually.
			ctx.Warn("%s(): The script tried to access a property on an incomplete object. Please ensure that the class definition \"%s\" of the object you are trying to operate on was loaded _before_ unserialize() gets called or provide an autoloader to load the class definition",
				funcName, cn, logopt.NoFuncName(true))
			return phpv.ZNULL.ZVal(), nil
		},
	},
}

// > class Traversable
var Traversable = &ZClass{
	Type: phpv.ZClassTypeInterface,
	Name: "Traversable",
}

// > class IteratorAggregate
var IteratorAggregate = &ZClass{
	Type:    phpv.ZClassTypeInterface,
	Name:    "IteratorAggregate",
	Extends: Traversable,
	Methods: map[phpv.ZString]*phpv.ZClassMethod{
		"getiterator": {Name: "getIterator", Modifiers: phpv.ZAttrPublic, Empty: true},
	},
}

// > class Iterator
var Iterator = &ZClass{
	Type:    phpv.ZClassTypeInterface,
	Name:    "Iterator",
	Extends: Traversable,
	Methods: map[phpv.ZString]*phpv.ZClassMethod{
		"current": {Name: "current", Modifiers: phpv.ZAttrPublic, Empty: true},
		"key":     {Name: "key", Modifiers: phpv.ZAttrPublic, Empty: true},
		"next":    {Name: "next", Modifiers: phpv.ZAttrPublic, Empty: true},
		"rewind":  {Name: "rewind", Modifiers: phpv.ZAttrPublic, Empty: true},
		"valid":   {Name: "valid", Modifiers: phpv.ZAttrPublic, Empty: true},
	},
}

// > class ArrayAccess
var ArrayAccess = &ZClass{
	Type: phpv.ZClassTypeInterface,
	Name: "ArrayAccess",
	Methods: map[phpv.ZString]*phpv.ZClassMethod{
		"offsetexists": {Name: "offsetExists", Modifiers: phpv.ZAttrPublic, Empty: true},
		"offsetget":    {Name: "offsetGet", Modifiers: phpv.ZAttrPublic, Empty: true},
		"offsetset":    {Name: "offsetSet", Modifiers: phpv.ZAttrPublic, Empty: true},
		"offsetunset":  {Name: "offsetUnset", Modifiers: phpv.ZAttrPublic, Empty: true},
	},
}

// > class Serializable
var Serializable = &ZClass{
	Type: phpv.ZClassTypeInterface,
	Name: "Serializable",
	Methods: map[phpv.ZString]*phpv.ZClassMethod{
		"serialize":   {Name: "serialize", Modifiers: phpv.ZAttrPublic, Empty: true},
		"unserialize": {Name: "unserialize", Modifiers: phpv.ZAttrPublic, Empty: true},
	},
}

// > interface Stringable
var Stringable = &ZClass{
	Type: phpv.ZClassTypeInterface,
	Name: "Stringable",
	Methods: map[phpv.ZString]*phpv.ZClassMethod{
		"__tostring": {Name: "__toString", Modifiers: phpv.ZAttrPublic, Empty: true},
	},
}

// > interface Reflector extends Stringable
var Reflector = &ZClass{
	Type:    phpv.ZClassTypeInterface,
	Name:    "Reflector",
	Extends: Stringable,
	Methods: map[phpv.ZString]*phpv.ZClassMethod{
		"__tostring": {Name: "__toString", Modifiers: phpv.ZAttrPublic, Empty: true},
	},
}

// > interface UnitEnum
// All PHP enums implicitly implement UnitEnum
var UnitEnum = &ZClass{
	Type: phpv.ZClassTypeInterface,
	Name: "UnitEnum",
}

// > interface BackedEnum extends UnitEnum
// Backed enums (with int or string backing type) implicitly implement BackedEnum
var BackedEnum = &ZClass{
	Type:    phpv.ZClassTypeInterface,
	Name:    "BackedEnum",
	Extends: UnitEnum,
}
