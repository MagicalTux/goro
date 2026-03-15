package phpobj

import (
	"fmt"

	"github.com/MagicalTux/goro/core/phpv"
)

// PHP 8.5 Attribute class constants
const (
	AttributeTARGET_CLASS          = 1
	AttributeTARGET_FUNCTION       = 2
	AttributeTARGET_METHOD         = 4
	AttributeTARGET_PROPERTY       = 8
	AttributeTARGET_CLASS_CONSTANT = 16
	AttributeTARGET_PARAMETER      = 32
	AttributeTARGET_CONSTANT       = 64
	AttributeTARGET_ALL            = 127
	AttributeIS_REPEATABLE         = 128
)

// AttributeClass is the built-in PHP Attribute class used with #[Attribute]
var AttributeClass *ZClass

func init() {
	AttributeClass = &ZClass{
		Name: "Attribute",
		// The Attribute class itself has #[Attribute(Attribute::TARGET_CLASS)]
		Attributes: []*phpv.ZAttribute{
			{ClassName: "Attribute", Args: []*phpv.ZVal{phpv.ZInt(AttributeTARGET_CLASS).ZVal()}},
		},
		Props: []*phpv.ZClassProp{
			{VarName: "flags", Default: phpv.ZInt(0).ZVal(), Modifiers: phpv.ZAttrPublic},
		},
		Const: map[phpv.ZString]*phpv.ZClassConst{
			"TARGET_CLASS":          {Value: phpv.ZInt(AttributeTARGET_CLASS)},
			"TARGET_FUNCTION":       {Value: phpv.ZInt(AttributeTARGET_FUNCTION)},
			"TARGET_METHOD":         {Value: phpv.ZInt(AttributeTARGET_METHOD)},
			"TARGET_PROPERTY":       {Value: phpv.ZInt(AttributeTARGET_PROPERTY)},
			"TARGET_CLASS_CONSTANT": {Value: phpv.ZInt(AttributeTARGET_CLASS_CONSTANT)},
			"TARGET_PARAMETER":      {Value: phpv.ZInt(AttributeTARGET_PARAMETER)},
			"TARGET_CONSTANT":       {Value: phpv.ZInt(AttributeTARGET_CONSTANT)},
			"TARGET_ALL":            {Value: phpv.ZInt(AttributeTARGET_ALL)},
			"IS_REPEATABLE":         {Value: phpv.ZInt(AttributeIS_REPEATABLE)},
		},
		Methods: map[phpv.ZString]*phpv.ZClassMethod{
			"__construct": {Name: "__construct", Method: NativeMethod(attributeConstruct)},
		},
	}
}

// DeprecatedClass is the built-in #[\Deprecated] attribute class (PHP 8.4+)
var DeprecatedClass *ZClass

// OverrideClass is the built-in #[\Override] attribute class (PHP 8.3+)
var OverrideClass *ZClass

// NoDiscardClass is the built-in #[\NoDiscard] attribute class (PHP 8.5+)
var NoDiscardClass *ZClass

// AllowDynamicPropertiesClass is the built-in #[\AllowDynamicProperties] attribute class (PHP 8.2+)
var AllowDynamicPropertiesClass *ZClass

func init() {
	DeprecatedClass = &ZClass{
		Name: "Deprecated",
		Attributes: []*phpv.ZAttribute{
			{ClassName: "Attribute", Args: []*phpv.ZVal{phpv.ZInt(
				AttributeTARGET_FUNCTION | AttributeTARGET_METHOD | AttributeTARGET_CLASS_CONSTANT | AttributeTARGET_CONSTANT,
			).ZVal()}},
		},
		Props: []*phpv.ZClassProp{
			{VarName: "message", Default: phpv.ZString("").ZVal(), Modifiers: phpv.ZAttrPublic | phpv.ZAttrReadonly},
			{VarName: "since", Default: phpv.ZString("").ZVal(), Modifiers: phpv.ZAttrPublic | phpv.ZAttrReadonly},
		},
		Methods: map[phpv.ZString]*phpv.ZClassMethod{
			"__construct": {Name: "__construct", Method: NativeMethod(func(ctx phpv.Context, o *ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
				msg := phpv.ZString("")
				since := phpv.ZString("")
				if len(args) > 0 {
					msg = phpv.ZString(args[0].String())
				}
				if len(args) > 1 {
					since = phpv.ZString(args[1].String())
				}
				o.HashTable().SetString("message", msg.ZVal())
				o.HashTable().SetString("since", since.ZVal())
				return nil, nil
			})},
		},
	}

	OverrideClass = &ZClass{
		Name: "Override",
		Attributes: []*phpv.ZAttribute{
			{ClassName: "Attribute", Args: []*phpv.ZVal{phpv.ZInt(AttributeTARGET_METHOD).ZVal()}},
		},
		Methods: map[phpv.ZString]*phpv.ZClassMethod{
			"__construct": {Name: "__construct", Method: NativeMethod(func(ctx phpv.Context, o *ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
				return nil, nil
			})},
		},
	}

	NoDiscardClass = &ZClass{
		Name: "NoDiscard",
		Attributes: []*phpv.ZAttribute{
			{ClassName: "Attribute", Args: []*phpv.ZVal{phpv.ZInt(
				AttributeTARGET_FUNCTION | AttributeTARGET_METHOD,
			).ZVal()}},
		},
		Props: []*phpv.ZClassProp{
			{VarName: "message", Default: phpv.ZString("").ZVal(), Modifiers: phpv.ZAttrPublic | phpv.ZAttrReadonly},
		},
		Methods: map[phpv.ZString]*phpv.ZClassMethod{
			"__construct": {Name: "__construct", Method: NativeMethod(func(ctx phpv.Context, o *ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
				msg := phpv.ZString("")
				if len(args) > 0 {
					msg = phpv.ZString(args[0].String())
				}
				o.HashTable().SetString("message", msg.ZVal())
				return nil, nil
			})},
		},
	}

	AllowDynamicPropertiesClass = &ZClass{
		Name: "AllowDynamicProperties",
		Attributes: []*phpv.ZAttribute{
			{ClassName: "Attribute", Args: []*phpv.ZVal{phpv.ZInt(AttributeTARGET_CLASS).ZVal()}},
		},
		Methods: map[phpv.ZString]*phpv.ZClassMethod{
			"__construct": {Name: "__construct", Method: NativeMethod(func(ctx phpv.Context, o *ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
				return nil, nil
			})},
		},
	}
}

func attributeConstruct(ctx phpv.Context, o *ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	flags := phpv.ZInt(AttributeTARGET_ALL)
	if len(args) > 0 {
		// Validate that flags argument is an integer type
		arg := args[0]
		switch arg.GetType() {
		case phpv.ZtInt, phpv.ZtBool:
			flags = arg.AsInt(ctx)
		default:
			return nil, ThrowError(ctx, TypeError,
				fmt.Sprintf("Attribute::__construct(): Argument #1 ($flags) must be of type int, %s given", arg.GetType().String()))
		}
		// Validate flags value is within valid range
		maxValid := int64(AttributeTARGET_ALL | AttributeIS_REPEATABLE)
		if int64(flags) < 0 || int64(flags) > maxValid {
			return nil, ThrowError(ctx, ValueError, "Invalid attribute flags specified")
		}
	}

	o.HashTable().SetString("flags", flags.ZVal())
	return nil, nil
}

// GetAttributeFlags returns the Attribute flags for a class, checking if it
// has the #[Attribute] attribute. Returns -1 if the class is not an attribute class.
func GetAttributeFlags(ctx phpv.Context, class phpv.ZClass) int64 {
	zc, ok := class.(*ZClass)
	if !ok {
		return -1
	}
	for _, attr := range zc.Attributes {
		if attr.ClassName == "Attribute" || attr.ClassName == "\\Attribute" {
			if len(attr.Args) > 0 {
				return int64(attr.Args[0].AsInt(ctx))
			}
			return int64(AttributeTARGET_ALL)
		}
	}
	return -1
}

// TargetName returns the human-readable name for an attribute target constant.
func TargetName(target int) string {
	switch target {
	case AttributeTARGET_CLASS:
		return "class"
	case AttributeTARGET_FUNCTION:
		return "function"
	case AttributeTARGET_METHOD:
		return "method"
	case AttributeTARGET_PROPERTY:
		return "property"
	case AttributeTARGET_CLASS_CONSTANT:
		return "class constant"
	case AttributeTARGET_PARAMETER:
		return "parameter"
	case AttributeTARGET_CONSTANT:
		return "constant"
	default:
		return "unknown"
	}
}

// ValidateAttributeTarget checks if an attribute is valid for the given target.
// Returns an error string if invalid, empty string if valid.
func ValidateAttributeTarget(ctx phpv.Context, attr *phpv.ZAttribute, target int) string {
	// Look up the attribute class
	class, err := ctx.Global().GetClass(ctx, attr.ClassName, false)
	if err != nil {
		// Class not found - that's OK, attribute may be used without class
		return ""
	}

	flags := GetAttributeFlags(ctx, class)
	if flags < 0 {
		// Not declared as an attribute class
		return ""
	}

	if int(flags)&target == 0 {
		return fmt.Sprintf("Attribute \"%s\" cannot target %s (allowed targets: %s)",
			attr.ClassName, TargetName(target), describeTargets(int(flags)))
	}

	return ""
}

// ValidateAttributeRepeat checks if a non-repeatable attribute is used more than once.
// attrs is the full list of attributes on the target.
// Returns an error string if invalid, empty string if valid.
func ValidateAttributeRepeat(ctx phpv.Context, attrs []*phpv.ZAttribute) string {
	seen := make(map[phpv.ZString]bool)
	for _, attr := range attrs {
		lowerName := attr.ClassName.ToLower()
		if seen[lowerName] {
			// Check if this attribute is repeatable
			class, err := ctx.Global().GetClass(ctx, attr.ClassName, false)
			if err != nil {
				continue
			}
			flags := GetAttributeFlags(ctx, class)
			if flags < 0 {
				continue
			}
			if int(flags)&AttributeIS_REPEATABLE == 0 {
				return fmt.Sprintf("Attribute \"%s\" must not be repeated", attr.ClassName)
			}
		}
		seen[lowerName] = true
	}
	return ""
}

// ValidateAttributeList validates all attributes on a target for target matching
// and repeat constraints. Returns an error string if invalid, empty string if valid.
func ValidateAttributeList(ctx phpv.Context, attrs []*phpv.ZAttribute, target int) string {
	// First check target validity
	for _, attr := range attrs {
		if msg := ValidateAttributeTarget(ctx, attr, target); msg != "" {
			return msg
		}
	}
	// Then check repeatable constraints
	if msg := ValidateAttributeRepeat(ctx, attrs); msg != "" {
		return msg
	}
	return ""
}

// isInternalAttributeClass returns true if the given attribute name refers to a
// built-in PHP attribute class that should be validated at compile time.
func isInternalAttributeClass(name phpv.ZString) bool {
	switch name {
	case "Attribute", "\\Attribute",
		"Override", "\\Override",
		"Deprecated", "\\Deprecated",
		"NoDiscard", "\\NoDiscard",
		"AllowDynamicProperties", "\\AllowDynamicProperties":
		return true
	}
	return false
}

// ValidateInternalAttributeList validates only internal/built-in attribute
// classes on a target. Userland attributes are only validated at Reflection
// newInstance() time. Returns an error string if invalid, empty string if valid.
func ValidateInternalAttributeList(ctx phpv.Context, attrs []*phpv.ZAttribute, target int) string {
	// Check target validity for internal attributes only
	for _, attr := range attrs {
		if !isInternalAttributeClass(attr.ClassName) {
			continue
		}
		if msg := ValidateAttributeTarget(ctx, attr, target); msg != "" {
			return msg
		}
	}
	// Check repeatable constraints for internal attributes only
	seen := make(map[phpv.ZString]bool)
	for _, attr := range attrs {
		if !isInternalAttributeClass(attr.ClassName) {
			continue
		}
		lowerName := attr.ClassName.ToLower()
		if seen[lowerName] {
			class, err := ctx.Global().GetClass(ctx, attr.ClassName, false)
			if err != nil {
				continue
			}
			flags := GetAttributeFlags(ctx, class)
			if flags < 0 {
				continue
			}
			if int(flags)&AttributeIS_REPEATABLE == 0 {
				return fmt.Sprintf("Attribute \"%s\" must not be repeated", attr.ClassName)
			}
		}
		seen[lowerName] = true
	}
	return ""
}

func describeTargets(flags int) string {
	var parts []string
	if flags&AttributeTARGET_CLASS != 0 {
		parts = append(parts, "class")
	}
	if flags&AttributeTARGET_FUNCTION != 0 {
		parts = append(parts, "function")
	}
	if flags&AttributeTARGET_METHOD != 0 {
		parts = append(parts, "method")
	}
	if flags&AttributeTARGET_PROPERTY != 0 {
		parts = append(parts, "property")
	}
	if flags&AttributeTARGET_CLASS_CONSTANT != 0 {
		parts = append(parts, "class constant")
	}
	if flags&AttributeTARGET_PARAMETER != 0 {
		parts = append(parts, "parameter")
	}
	if flags&AttributeTARGET_CONSTANT != 0 {
		parts = append(parts, "constant")
	}
	if len(parts) == 0 {
		return "none"
	}
	result := parts[0]
	for i := 1; i < len(parts); i++ {
		result += ", " + parts[i]
	}
	return result
}
