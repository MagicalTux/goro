package core

//> class stdClass
var stdClass = &ZClass{
	Name: "stdClass",
}

//> class Traversable
var Traversable = &ZClass{
	Type: ZClassTypeInterface,
	Name: "Traversable",
}

//> class IteratorAggregate
var IteratorAggregate = &ZClass{
	Type:    ZClassTypeInterface,
	Name:    "IteratorAggregate",
	Extends: Traversable,
}

//> class Iterator
var Iterator = &ZClass{
	Type:    ZClassTypeInterface,
	Name:    "Iterator",
	Extends: Traversable,
}

//> class ArrayAccess
var ArrayAccess = &ZClass{
	Type: ZClassTypeInterface,
	Name: "ArrayAccess",
}

//> class Serializable
var Serializable = &ZClass{
	Type: ZClassTypeInterface,
	Name: "Serializable",
}
