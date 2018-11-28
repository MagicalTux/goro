package core

import "github.com/MagicalTux/goro/core/phpv"

type TypeHint struct {
	t phpv.ZType
	s phpv.ZString // class name, or special value such as "self", "iterable". If t=phpv.ZtObject but s="" then any object is ok
	c *ZClass      // looked up class, if any
}

func ParseTypeHint(s phpv.ZString) *TypeHint {
	switch s.ToLower() {
	case "self":
		return &TypeHint{t: phpv.ZtObject, s: "self"}
	case "iterable":
		return &TypeHint{t: phpv.ZtObject, s: "iterable"}
	case "object":
		return &TypeHint{t: phpv.ZtObject}
	case "array":
		return &TypeHint{t: phpv.ZtArray}
	case "callable":
		return &TypeHint{t: phpv.ZtObject, s: "callable"}
	case "bool":
		return &TypeHint{t: phpv.ZtBool}
	case "float":
		return &TypeHint{t: phpv.ZtFloat}
	case "int":
		return &TypeHint{t: phpv.ZtInt}
	case "string":
		return &TypeHint{t: phpv.ZtString}
	default:
		return &TypeHint{t: phpv.ZtObject, s: s}
	}
}
