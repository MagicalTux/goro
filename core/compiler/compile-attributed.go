package compiler

import (
	"github.com/MagicalTux/goro/core/phpobj"
	"github.com/MagicalTux/goro/core/phpv"
	"github.com/MagicalTux/goro/core/tokenizer"
)

// compileAttributed handles top-level #[...] attribute groups followed by
// a class, function, or enum declaration.
func compileAttributed(i *tokenizer.Item, c compileCtx) (phpv.Runnable, error) {
	// i is the T_ATTRIBUTE token (#[)
	// Parse the attributes first
	attrs, err := parseAttributes(c)
	if err != nil {
		return nil, err
	}

	// Now read what follows - should be class, function, enum, or modifiers
	i, err = c.NextItem()
	if err != nil {
		return nil, err
	}

	// Handle additional attribute groups: #[A] #[B] class Foo {}
	for i.Type == tokenizer.T_ATTRIBUTE {
		moreAttrs, err := parseAttributes(c)
		if err != nil {
			return nil, err
		}
		attrs = append(attrs, moreAttrs...)
		i, err = c.NextItem()
		if err != nil {
			return nil, err
		}
	}

	switch i.Type {
	case tokenizer.T_CLASS, tokenizer.T_INTERFACE, tokenizer.T_TRAIT,
		tokenizer.T_ABSTRACT, tokenizer.T_FINAL, tokenizer.T_READONLY:
		r, err := compileClass(i, c)
		if err != nil {
			return nil, err
		}
		if zc, ok := r.(*phpobj.ZClass); ok {
			zc.Attributes = append(attrs, zc.Attributes...)
		}
		return r, nil

	case tokenizer.T_ENUM:
		r, err := compileEnum(i, c)
		if err != nil {
			return nil, err
		}
		if zc, ok := r.(*phpobj.ZClass); ok {
			zc.Attributes = append(attrs, zc.Attributes...)
		}
		return r, nil

	case tokenizer.T_FUNCTION:
		// Function declaration with attributes
		r, err := compileFunction(i, c)
		if err != nil {
			return nil, err
		}
		// Store attributes on the closure
		if zc, ok := r.(*ZClosure); ok {
			zc.attributes = attrs
		}
		return r, nil

	case tokenizer.T_FN:
		// Arrow function with attributes
		r, err := compileArrowFunction(i, c)
		if err != nil {
			return nil, err
		}
		if zc, ok := r.(*ZClosure); ok {
			zc.attributes = attrs
		}
		return r, nil

	case tokenizer.T_CONST:
		// Top-level constant with attributes: #[Attr] const FOO = expr;
		r, err := compileTopLevelConst(i, c)
		if err != nil {
			return nil, err
		}
		// Store attributes on the constant (for Reflection API)
		_ = attrs // TODO: Store attributes on constants when ZClassConst supports it
		return r, nil

	default:
		return nil, i.Unexpected()
	}
}
