package spl

import "github.com/MagicalTux/goro/core/phpobj"
import "github.com/MagicalTux/goro/core/phpv"

// Countable interface - objects that can be counted with count()
var Countable = &phpobj.ZClass{
	Type: phpv.ZClassTypeInterface,
	Name: "Countable",
	Methods: map[phpv.ZString]*phpv.ZClassMethod{
		"count": {Name: "count", Modifiers: phpv.ZAttrPublic, Empty: true},
	},
}

// OuterIterator interface - for iterators that wrap another iterator
var OuterIterator = &phpobj.ZClass{
	Type:    phpv.ZClassTypeInterface,
	Name:    "OuterIterator",
	Extends: phpobj.Iterator,
}

// SeekableIterator interface - extends Iterator with seek()
var SeekableIterator = &phpobj.ZClass{
	Type:    phpv.ZClassTypeInterface,
	Name:    "SeekableIterator",
	Extends: phpobj.Iterator,
	Methods: map[phpv.ZString]*phpv.ZClassMethod{
		"seek": {Name: "seek", Modifiers: phpv.ZAttrPublic, Empty: true},
	},
}

// SplObserver interface
var SplObserver = &phpobj.ZClass{
	Type: phpv.ZClassTypeInterface,
	Name: "SplObserver",
	Methods: map[phpv.ZString]*phpv.ZClassMethod{
		"update": {Name: "update", Modifiers: phpv.ZAttrPublic, Empty: true},
	},
}

// SplSubject interface
var SplSubject = &phpobj.ZClass{
	Type: phpv.ZClassTypeInterface,
	Name: "SplSubject",
	Methods: map[phpv.ZString]*phpv.ZClassMethod{
		"attach": {Name: "attach", Modifiers: phpv.ZAttrPublic, Empty: true},
		"detach": {Name: "detach", Modifiers: phpv.ZAttrPublic, Empty: true},
		"notify": {Name: "notify", Modifiers: phpv.ZAttrPublic, Empty: true},
	},
}
