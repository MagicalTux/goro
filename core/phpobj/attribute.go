package phpobj

import (
	"fmt"

	"github.com/MagicalTux/goro/core/phpv"
)

// PHP 8.0 Attribute class constants
const (
	AttributeTARGET_CLASS          = 1
	AttributeTARGET_FUNCTION       = 2
	AttributeTARGET_METHOD         = 4
	AttributeTARGET_PROPERTY       = 8
	AttributeTARGET_CLASS_CONSTANT = 16
	AttributeTARGET_PARAMETER      = 32
	AttributeTARGET_ALL            = 63
	AttributeIS_REPEATABLE         = 64
)

// AttributeClass is the built-in PHP Attribute class used with #[Attribute]
var AttributeClass *ZClass

func init() {
	AttributeClass = &ZClass{
		Name: "Attribute",
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
			"TARGET_ALL":            {Value: phpv.ZInt(AttributeTARGET_ALL)},
			"IS_REPEATABLE":         {Value: phpv.ZInt(AttributeIS_REPEATABLE)},
		},
		Methods: map[phpv.ZString]*phpv.ZClassMethod{
			"__construct": {Name: "__construct", Method: NativeMethod(attributeConstruct)},
		},
	}
}

func attributeConstruct(ctx phpv.Context, o *ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	flags := phpv.ZInt(AttributeTARGET_ALL)
	if len(args) > 0 {
		flags = args[0].AsInt(ctx)
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
		targetStr := "unknown"
		switch target {
		case AttributeTARGET_CLASS:
			targetStr = "class"
		case AttributeTARGET_FUNCTION:
			targetStr = "function"
		case AttributeTARGET_METHOD:
			targetStr = "method"
		case AttributeTARGET_PROPERTY:
			targetStr = "property"
		case AttributeTARGET_CLASS_CONSTANT:
			targetStr = "class constant"
		case AttributeTARGET_PARAMETER:
			targetStr = "parameter"
		}
		return fmt.Sprintf("Attribute \"%s\" cannot target %s (allowed targets: %s)",
			attr.ClassName, targetStr, describeTargets(int(flags)))
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
	if len(parts) == 0 {
		return "none"
	}
	result := parts[0]
	for i := 1; i < len(parts); i++ {
		result += ", " + parts[i]
	}
	return result
}
