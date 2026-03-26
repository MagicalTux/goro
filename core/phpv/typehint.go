package phpv

import (
	"math"
	"reflect"
	"sort"
	"strings"
)

type TypeHint struct {
	t            ZType
	s            ZString    // class name, or special value such as "self", "iterable". If t=ZtObject but s="" then any object is ok
	c            ZClass     // looked up class, if any
	Nullable     bool       // true if the type is explicitly nullable (?Type)
	Union        []*TypeHint // for union types (int|string), each alternative
	Intersection []*TypeHint // for intersection types (A&B), all must match
}

func (h *TypeHint) Type() ZType {
	return h.t
}

func (h *TypeHint) ClassName() ZString {
	return h.s
}

// IsNullable returns true if the type hint accepts null values.
// This includes explicitly nullable types (?Type), union types containing null,
// and the mixed type (which implicitly accepts null).
func (h *TypeHint) IsNullable() bool {
	if h == nil {
		return true // no type hint accepts anything including null
	}
	if h.Nullable {
		return true
	}
	// Intersection types (A&B) are never nullable (they require concrete objects)
	if len(h.Intersection) > 0 {
		return false
	}
	// Union types: nullability is determined by members, not the wrapper's zero-value t field
	if len(h.Union) > 0 {
		for _, u := range h.Union {
			if u.IsNullable() {
				return true
			}
		}
		return false
	}
	if h.t == ZtMixed || h.t == ZtNull {
		return true
	}
	return false
}

// CheckStrict returns true if the value matches this type hint using strict_types rules.
// In strict mode, only exact type matches are allowed (no coercion), except:
// - int is accepted for float parameters (int-to-float widening)
// - null is accepted for nullable parameters
func (h *TypeHint) CheckStrict(ctx Context, val *ZVal) bool {
	if h == nil {
		return true
	}

	// Nullable types accept null
	if h.Nullable && val.IsNull() {
		return true
	}

	// Union type: check each alternative (any must match in strict mode)
	if len(h.Union) > 0 {
		for _, alt := range h.Union {
			if alt.CheckStrict(ctx, val) {
				return true
			}
		}
		return false
	}

	// Intersection type: all must match
	if len(h.Intersection) > 0 {
		for _, part := range h.Intersection {
			if !part.CheckStrict(ctx, val) {
				return false
			}
		}
		return true
	}

	if h.t == ZtObject {
		// callable and iterable are handled same as weak mode
		if h.s == "callable" {
			return h.Check(ctx, val)
		}
		if h.s == "iterable" {
			return val.GetType() == ZtArray || val.GetType() == ZtObject
		}
		// Class/interface type hint
		if val.GetType() != ZtObject {
			return false
		}
		if h.s == "" {
			return true
		}
		if h.s == "self" {
			return true
		}
		obj := val.AsObject(ctx)
		if obj == nil {
			return false
		}
		return ClassNameMatch(obj.GetClass(), h.s, ctx)
	}

	// "mixed" accepts anything
	if h.t == ZtMixed {
		return true
	}

	// Handle "false" and "true" standalone types
	if h.t == ZtBool && h.s == "false" {
		return val.GetType() == ZtBool && !bool(val.Value().(ZBool))
	}
	if h.t == ZtBool && h.s == "true" {
		return val.GetType() == ZtBool && bool(val.Value().(ZBool))
	}

	// Strict mode: only exact type match, except int->float widening
	valType := val.GetType()
	switch h.t {
	case ZtFloat:
		// float accepts both float and int in strict mode (int-to-float widening)
		return valType == ZtFloat || valType == ZtInt
	default:
		return valType == h.t
	}
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

	// Union type: check each alternative (any must match)
	if len(h.Union) > 0 {
		for _, alt := range h.Union {
			if alt.Check(ctx, val) {
				return true
			}
		}
		return false
	}

	// Intersection type: all must match
	if len(h.Intersection) > 0 {
		for _, part := range h.Intersection {
			if !part.Check(ctx, val) {
				return false
			}
		}
		return true
	}

	if h.t == ZtObject {
		// "callable" type hint: accepts strings, arrays, closures, and invocable objects.
		// Validation is intentionally permissive -- detailed checks (visibility,
		// static vs instance) happen at call time via SpawnCallable.
		if h.s == "callable" {
			switch val.GetType() {
			case ZtString:
				s := string(val.AsString(ctx))
				if s == "" {
					return false
				}
				if strings.Contains(s, "::") {
					parts := strings.SplitN(s, "::", 2)
					if parts[0] == "" || parts[1] == "" {
						return false
					}
					// Use autoload=false to avoid side effects during type checking
					cls, err := ctx.Global().GetClass(ctx, ZString(parts[0]), false)
					if err != nil || cls == nil {
						return false
					}
					m, ok := cls.GetMethod(ZString(parts[1]).ToLower())
					if !ok {
						return false
					}
					if m.Modifiers.Has(ZAttrAbstract) {
						return false
					}
					return true
				}
				_, err := ctx.Global().GetFunction(ctx, ZString(s))
				return err == nil
			case ZtArray:
				// Validate structure: second element must be a string (method name)
				arr := val.Array()
				if arr == nil {
					return false
				}
				second, err := arr.OffsetGet(ctx, ZInt(1))
				if err != nil || second == nil || second.GetType() != ZtString {
					return false
				}
				return true
			case ZtObject:
				return true // closure or object with __invoke
			case ZtCallable:
				return true
			default:
				return false
			}
		}
		// iterable accepts arrays and Traversable objects
		if h.s == "iterable" {
			return val.GetType() == ZtArray || val.GetType() == ZtObject
		}
		// Class/interface type hint
		if val.GetType() != ZtObject {
			return false
		}
		if h.s == "" {
			return true // any object
		}
		if h.s == "self" {
			return true // TODO: proper check
		}
		// Check instanceof by class name
		obj := val.AsObject(ctx)
		if obj == nil {
			return false
		}
		return ClassNameMatch(obj.GetClass(), h.s, ctx)
	}

	// "mixed" accepts anything
	if h.t == ZtMixed {
		return true
	}

	// Handle "false" and "true" standalone types
	if h.t == ZtBool && h.s == "false" {
		return val.GetType() == ZtBool && !bool(val.Value().(ZBool))
	}
	if h.t == ZtBool && h.s == "true" {
		return val.GetType() == ZtBool && bool(val.Value().(ZBool))
	}

	// PHP non-strict mode type coercion rules:
	// int accepts: int, float (if no fractional part), bool, numeric strings
	// float accepts: float, int, bool, numeric strings
	// string accepts: string, int, float, bool
	// bool accepts: bool, int, float, string, null
	valType := val.GetType()
	switch h.t {
	case ZtInt:
		switch valType {
		case ZtInt:
			return true
		case ZtFloat:
			// PHP coerces float->int, but NaN and Infinity cannot be converted
			f := float64(val.Value().(ZFloat))
			if math.IsNaN(f) || math.IsInf(f, 0) {
				return false
			}
			return true
		case ZtBool:
			return true
		case ZtString:
			return ZString(val.String()).IsNumeric()
		}
		return false
	case ZtFloat:
		switch valType {
		case ZtFloat, ZtInt, ZtBool:
			return true
		case ZtString:
			return ZString(val.String()).IsNumeric()
		}
		return false
	case ZtString:
		switch valType {
		case ZtString, ZtInt, ZtFloat, ZtBool:
			return true
		case ZtObject:
			// PHP weak mode: Stringable objects are accepted for string type hints
			if obj := val.AsObject(ctx); obj != nil {
				if _, hasToString := obj.GetClass().GetMethod("__tostring"); hasToString {
					return true
				}
			}
		}
		return false
	case ZtBool:
		// Bool accepts any scalar
		switch valType {
		case ZtBool, ZtInt, ZtFloat, ZtString, ZtNull:
			return true
		}
		return false
	}

	return valType == h.t
}

// typeHintSortOrder returns a sort key for canonical union display order.
// PHP normalizes union types so that named class types and intersection groups
// preserve their relative declaration order. Bare "object" sorts after named
// class types but before scalars. Scalar types sort after them, and null last.
func typeHintSortOrder(h *TypeHint) int {
	if len(h.Intersection) > 0 {
		return 0 // intersection groups: preserve position among class types
	}
	switch h.t {
	case ZtObject:
		if h.s == "" {
			return 5 // bare "object" after named class types, before scalars
		}
		if h.s == "callable" {
			return 6 // callable after object
		}
		return 0 // named class/interface types preserve declaration order
	case ZtArray:
		return 10
	case ZtString:
		return 20
	case ZtInt:
		return 21
	case ZtFloat:
		return 22
	case ZtBool:
		if h.s == "false" {
			return 31
		}
		if h.s == "true" {
			return 30
		}
		return 23
	case ZtNull:
		return 50
	case ZtVoid:
		return 60
	case ZtNever:
		return 70
	case ZtMixed:
		return 0
	}
	return 100
}

// String returns the PHP type name for error messages
func (h *TypeHint) String() string {
	if len(h.Union) > 0 {
		type sortEntry struct {
			s     string
			order int
		}
		var entries []sortEntry
		for _, alt := range h.Union {
			// Expand iterable into Traversable and array as separate entries
			// so they sort independently (Traversable with class types, array
			// with scalars), matching PHP's internal expansion of iterable.
			if alt.t == ZtObject && alt.s == "iterable" {
				entries = append(entries,
					sortEntry{s: "Traversable", order: 0},
					sortEntry{s: "array", order: 10},
				)
				continue
			}
			s := alt.String()
			// Wrap intersection groups in parentheses for DNF display
			if len(alt.Intersection) > 0 {
				s = "(" + s + ")"
			}
			entries = append(entries, sortEntry{s: s, order: typeHintSortOrder(alt)})
		}
		sort.SliceStable(entries, func(i, j int) bool {
			return entries[i].order < entries[j].order
		})
		parts := make([]string, len(entries))
		for i, e := range entries {
			parts[i] = e.s
		}
		return strings.Join(parts, "|")
	}
	if len(h.Intersection) > 0 {
		parts := make([]string, len(h.Intersection))
		for i, part := range h.Intersection {
			parts[i] = part.String()
		}
		return strings.Join(parts, "&")
	}
	// iterable is displayed as Traversable|array in error messages (PHP 8.4+)
	if h.s == "iterable" {
		if h.Nullable {
			return "Traversable|array|null"
		}
		return "Traversable|array"
	}
	prefix := ""
	if h.Nullable {
		prefix = "?"
	}
	if h.s != "" {
		return prefix + string(h.s)
	}
	return prefix + h.t.TypeName()
}

func ClassNameMatch(c ZClass, name ZString, ctx Context) bool {
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

// ParseTypeHint converts a type name string to a TypeHint struct.
func ParseTypeHint(s ZString) *TypeHint {
	switch s.ToLower() {
	case "self":
		return &TypeHint{t: ZtObject, s: "self"}
	case "static":
		return &TypeHint{t: ZtObject, s: "static"}
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
	case "false":
		return &TypeHint{t: ZtBool, s: "false"}
	case "true":
		return &TypeHint{t: ZtBool, s: "true"}
	case "float":
		return &TypeHint{t: ZtFloat}
	case "int":
		return &TypeHint{t: ZtInt}
	case "string":
		return &TypeHint{t: ZtString}
	case "mixed":
		return &TypeHint{t: ZtMixed}
	case "void":
		return &TypeHint{t: ZtVoid}
	case "never":
		return &TypeHint{t: ZtNever}
	case "null":
		return &TypeHint{t: ZtNull}
	default:
		return &TypeHint{t: ZtObject, s: s}
	}
}

// ResolveTypeHintSelf returns the string representation of the type hint
// with "self" resolved to the given class name. This is used for error
// messages when trait methods use "self" in their signatures.
func ResolveTypeHintSelf(h *TypeHint, className ZString) string {
	if h == nil {
		return ""
	}
	if h.t == ZtObject && h.s == "self" {
		prefix := ""
		if h.Nullable {
			prefix = "?"
		}
		return prefix + string(className)
	}
	if len(h.Union) > 0 {
		parts := make([]string, len(h.Union))
		for i, alt := range h.Union {
			parts[i] = ResolveTypeHintSelf(alt, className)
		}
		return strings.Join(parts, "|")
	}
	if len(h.Intersection) > 0 {
		parts := make([]string, len(h.Intersection))
		for i, part := range h.Intersection {
			parts[i] = ResolveTypeHintSelf(part, className)
		}
		return strings.Join(parts, "&")
	}
	return h.String()
}
