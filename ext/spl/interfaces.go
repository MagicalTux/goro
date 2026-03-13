package spl

import "github.com/MagicalTux/goro/core/phpobj"
import "github.com/MagicalTux/goro/core/phpv"

// Countable interface - objects that can be counted with count()
var Countable = &phpobj.ZClass{
	Type: phpv.ZClassTypeInterface,
	Name: "Countable",
}

// OuterIterator interface - for iterators that wrap another iterator
var OuterIterator = &phpobj.ZClass{
	Type:    phpv.ZClassTypeInterface,
	Name:    "OuterIterator",
	Extends: phpobj.Iterator,
}
