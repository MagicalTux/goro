package core

type TypeHint struct {
	t ZType
	s ZString // class name, or special value such as "self", "iterable". If t=ZtObject but s="" then any object is ok
	c *ZClass // looked up class, if any
}

func ParseTypeHint(s ZString) *TypeHint {
	switch s.ToLower() {
	case "self":
		return &TypeHint{t: ZtObject, s: "self"}
	case "iterable":
		return &TypeHint{t: ZtObject, s: "iterable"}
	case "object":
		return &TypeHint{t: ZtObject}
	case "array":
		return &TypeHint{t: ZtArray}
	case "callable":
		return &TypeHint{t: ZtObject, s: "callable"}
	case "bool":
		return &TypeHint{t: ZtBool}
	case "float":
		return &TypeHint{t: ZtFloat}
	case "int":
		return &TypeHint{t: ZtInt}
	case "string":
		return &TypeHint{t: ZtString}
	default:
		return &TypeHint{t: ZtObject, s: s}
	}
}
