package phpv

import "reflect"

type TypeHint struct {
	t        ZType
	s        ZString // class name, or special value such as "self", "iterable". If t=ZtObject but s="" then any object is ok
	c        ZClass  // looked up class, if any
	Nullable bool    // true if the type is explicitly nullable (?Type)
}

func (h *TypeHint) Type() ZType {
	return h.t
}

func (h *TypeHint) ClassName() ZString {
	return h.s
}

// Check returns true if the value matches this type hint
func (h *TypeHint) Check(ctx Context, val *ZVal) bool {
	if h == nil {
		return true
	}

	// Nullable types accept null
	if h.Nullable && val.IsNull() {
		return true
	}

	if h.t == ZtObject {
		// Class/interface type hint
		if val.GetType() != ZtObject {
			return false
		}
		if h.s == "" {
			return true // any object
		}
		if h.s == "self" || h.s == "iterable" || h.s == "callable" {
			return true // TODO: proper check
		}
		// Check instanceof by class name
		obj := val.AsObject(ctx)
		if obj == nil {
			return false
		}
		return classNameMatch(obj.GetClass(), h.s, ctx)
	}

	return val.GetType() == h.t
}

// String returns the PHP type name for error messages
func (h *TypeHint) String() string {
	prefix := ""
	if h.Nullable {
		prefix = "?"
	}
	if h.s != "" {
		return prefix + string(h.s)
	}
	return prefix + h.t.TypeName()
}

func classNameMatch(c ZClass, name ZString, ctx Context) bool {
	if IsNilClass(c) {
		return false
	}
	// If we have a context, try to look up the target class and use InstanceOf
	// which properly checks parent classes and implemented interfaces
	if ctx != nil {
		if targetClass, err := ctx.Global().GetClass(ctx, name, false); err == nil && !IsNilClass(targetClass) {
			return c.InstanceOf(targetClass)
		}
	}
	// Fallback: name-based matching walking the parent chain
	nameLower := name.ToLower()
	for cur := c; !IsNilClass(cur); cur = cur.GetParent() {
		if cur.GetName().ToLower() == nameLower {
			return true
		}
	}
	return false
}

// IsNilClass checks if a ZClass interface value is nil (handles the nil pointer in non-nil interface case)
func IsNilClass(c ZClass) bool {
	if c == nil {
		return true
	}
	// Use reflect to check if the interface wraps a nil pointer
	return reflect.ValueOf(c).IsNil()
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
