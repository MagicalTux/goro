package phpobj

import "github.com/MagicalTux/goro/core/phpv"

func init() {
	phpv.NewStdClassFunc = func(ctx phpv.Context) (phpv.ZObject, error) {
		return NewZObject(ctx, StdClass)
	}
}

// > class StdClass
var StdClass = &ZClass{
	Name: "stdClass",
}

// > class __PHP_Incomplete_Class
var IncompleteClass = &ZClass{
	Name: "__PHP_Incomplete_Class",
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
