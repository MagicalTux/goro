package phpobj

import "github.com/MagicalTux/goro/core/phpv"

//> class StdClass
var StdClass = &ZClass{
	Name: "stdClass",
}

//> class Traversable
var Traversable = &ZClass{
	Type: phpv.ZClassTypeInterface,
	Name: "Traversable",
}

//> class IteratorAggregate
var IteratorAggregate = &ZClass{
	Type:    phpv.ZClassTypeInterface,
	Name:    "IteratorAggregate",
	Extends: Traversable,
}

//> class Iterator
var Iterator = &ZClass{
	Type:    phpv.ZClassTypeInterface,
	Name:    "Iterator",
	Extends: Traversable,
}

//> class ArrayAccess
var ArrayAccess = &ZClass{
	Type: phpv.ZClassTypeInterface,
	Name: "ArrayAccess",
}

//> class Serializable
var Serializable = &ZClass{
	Type: phpv.ZClassTypeInterface,
	Name: "Serializable",
}
