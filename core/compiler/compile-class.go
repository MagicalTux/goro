package compiler

import (
	"fmt"
	"io"
	"strings"

	"github.com/MagicalTux/goro/core/logopt"
	"github.com/MagicalTux/goro/core/phpobj"
	"github.com/MagicalTux/goro/core/phpv"
	"github.com/MagicalTux/goro/core/tokenizer"
)

// traitAlias is used during compilation to collect trait alias/insteadof directives.
type traitAlias struct {
	traitName  phpv.ZString
	methodName phpv.ZString
	newName    phpv.ZString
	newAttr    phpv.ZObjectAttr
}

func convertAliases(aliases []traitAlias) []phpv.ZClassTraitAlias {
	out := make([]phpv.ZClassTraitAlias, len(aliases))
	for i, a := range aliases {
		out[i] = phpv.ZClassTraitAlias{
			TraitName:  a.traitName,
			MethodName: a.methodName,
			NewName:    a.newName,
			NewAttr:    a.newAttr,
		}
	}
	return out
}

// traitInsteadof records "TraitName::method insteadof OtherTrait" during compilation.
type traitInsteadof struct {
	traitName  phpv.ZString
	methodName phpv.ZString
	insteadOf  []phpv.ZString
}

func convertInsteadofs(insteadofs []traitInsteadof) []phpv.ZClassTraitInsteadof {
	out := make([]phpv.ZClassTraitInsteadof, len(insteadofs))
	for i, io := range insteadofs {
		out[i] = phpv.ZClassTraitInsteadof{
			TraitName:  io.traitName,
			MethodName: io.methodName,
			InsteadOf:  io.insteadOf,
		}
	}
	return out
}

// containsRuntimeOps checks if a compiled expression contains runtime
// operations (variables, function calls) that are not allowed in class constants.
func containsRuntimeOps(r phpv.Runnable) bool {
	if r == nil {
		return false
	}
	switch v := r.(type) {
	case *runVariable, *runVariableRef:
		return true
	case *runOperator:
		return containsRuntimeOps(v.a) || containsRuntimeOps(v.b)
	case *runnableFunctionCallRef:
		return true
	case *runClassStaticObjRef:
		return containsRuntimeOps(v.className)
	case *runClassStaticVarRef:
		return containsRuntimeOps(v.className)
	case runConcat:
		for _, sub := range v {
			if containsRuntimeOps(sub) {
				return true
			}
		}
	}
	return false
}

// containsNewExpr checks if a compiled expression contains a `new` expression.
// Used to reject `new` in contexts where it is not allowed (class constants,
// non-static property defaults).
func containsNewExpr(r phpv.Runnable) bool {
	if r == nil {
		return false
	}
	switch v := r.(type) {
	case *runNewObject:
		return true
	case *runOperator:
		return containsNewExpr(v.a) || containsNewExpr(v.b)
	case runConcat:
		for _, sub := range v {
			if containsNewExpr(sub) {
				return true
			}
		}
	case *runArray:
		for _, e := range v.e {
			if containsNewExpr(e.k) || containsNewExpr(e.v) {
				return true
			}
		}
	case *runParentheses:
		return containsNewExpr(v.r)
	}
	return false
}

// checkStaticClassInConstExpr checks if a compiled expression contains static::class
// or static::CONST which cannot be used in compile-time class name resolution.
// Returns the location of the static:: if found, nil otherwise.
func checkStaticClassInConstExpr(r phpv.Runnable) *phpv.Loc {
	if r == nil {
		return nil
	}
	switch v := r.(type) {
	case *runClassNameOf:
		if zv, ok2 := v.className.(*runZVal); ok2 {
			if s, ok3 := zv.v.(phpv.ZString); ok3 && s.ToLower() == "static" {
				return v.l
			}
		}
	case *runClassStaticObjRef:
		if zv, ok2 := v.className.(*runZVal); ok2 {
			if s, ok3 := zv.v.(phpv.ZString); ok3 && s.ToLower() == "static" {
				return v.l
			}
		}
	case *runClassStaticVarRef:
		if zv, ok2 := v.className.(*runZVal); ok2 {
			if s, ok3 := zv.v.(phpv.ZString); ok3 && s.ToLower() == "static" {
				return v.l
			}
		}
	case *runOperator:
		if loc := checkStaticClassInConstExpr(v.a); loc != nil {
			return loc
		}
		return checkStaticClassInConstExpr(v.b)
	}
	return nil
}

// containsAttrDynamicClassName checks for dynamic class names in
// class constant references within attribute arguments, including
// object property access used as class names (e.g., a->b::c).
func containsAttrDynamicClassName(r phpv.Runnable) bool {
	if r == nil {
		return false
	}
	switch v := r.(type) {
	case *runClassStaticObjRef:
		switch v.className.(type) {
		case *runObjectVar, *runObjectDynVar, *runObjectFunc:
			return true
		}
		return containsAttrDynamicClassName(v.className)
	case *runClassStaticVarRef:
		switch v.className.(type) {
		case *runObjectVar, *runObjectDynVar, *runObjectFunc:
			return true
		}
		return containsAttrDynamicClassName(v.className)
	case *runOperator:
		return containsAttrDynamicClassName(v.a) || containsAttrDynamicClassName(v.b)
	}
	return false
}

// containsAttrRuntimeOps checks for additional runtime operations
// that are not allowed specifically in attribute arguments, including
// function calls (not first-class callables).
func containsAttrRuntimeOps(r phpv.Runnable) bool {
	if r == nil {
		return false
	}
	switch v := r.(type) {
	case *runnableFunctionCall:
		return true
	case *runOperator:
		return containsAttrRuntimeOps(v.a) || containsAttrRuntimeOps(v.b)
	case *runClassStaticObjRef:
		return containsAttrRuntimeOps(v.className)
	case *runClassStaticVarRef:
		return containsAttrRuntimeOps(v.className)
	case runConcat:
		for _, sub := range v {
			if containsAttrRuntimeOps(sub) {
				return true
			}
		}
	}
	return false
}

// containsDynamicClassName checks if an expression uses a dynamic class name
// (e.g., $var::CONST) which is not allowed in compile-time constants.
func containsDynamicClassName(r phpv.Runnable) bool {
	if r == nil {
		return false
	}
	switch v := r.(type) {
	case *runClassStaticObjRef:
		if _, ok := v.className.(*runVariable); ok {
			return true
		}
		if _, ok := v.className.(*runVariableRef); ok {
			return true
		}
	case *runClassStaticVarRef:
		if _, ok := v.className.(*runVariable); ok {
			return true
		}
		if _, ok := v.className.(*runVariableRef); ok {
			return true
		}
	case *runOperator:
		return containsDynamicClassName(v.a) || containsDynamicClassName(v.b)
	}
	return false
}

// checkExpressionClassInConstExpr checks if a compiled expression contains
// (expression)::class which cannot be used in constant expressions.
// Returns the location if found, nil otherwise.
func checkExpressionClassInConstExpr(r phpv.Runnable) *phpv.Loc {
	if cn, ok := r.(*runClassNameOf); ok {
		// Check if className is a non-constant expression (array, variable, etc.)
		switch cn.className.(type) {
		case *runZVal:
			// Literal class name or self/parent/static — handled elsewhere
			return nil
		default:
			// Any other expression (array, variable, function call, etc.)
			return cn.l
		}
	}
	return nil
}

// checkParentClassInConstExpr checks if parent::class is used in a class constant
// context where the class has no parent.
func checkParentClassInConstExpr(r phpv.Runnable, class *phpobj.ZClass) (phpv.ZString, *phpv.Loc) {
	if cn, ok := r.(*runClassNameOf); ok {
		if zv, ok2 := cn.className.(*runZVal); ok2 {
			if s, ok3 := zv.v.(phpv.ZString); ok3 && s.ToLower() == "parent" {
				if class.ExtendsStr == "" {
					return s, cn.l
				}
			}
		}
	}
	return "", nil
}

type zclassCompileCtx struct {
	compileCtx
	class *phpobj.ZClass
}

func (z *zclassCompileCtx) getClass() *phpobj.ZClass {
	return z.class
}

func (z *zclassCompileCtx) getNamespace() phpv.ZString {
	return z.compileCtx.getNamespace()
}

func (z *zclassCompileCtx) resolveClassName(name phpv.ZString) phpv.ZString {
	return z.compileCtx.resolveClassName(name)
}

func (z *zclassCompileCtx) resolveFunctionName(name phpv.ZString) phpv.ZString {
	return z.compileCtx.resolveFunctionName(name)
}

func (z *zclassCompileCtx) resolveConstantName(name string) string {
	return z.compileCtx.resolveConstantName(name)
}

func (z *zclassCompileCtx) isTopLevel() bool {
	return false // inside a class is never top level
}

func compileClass(i *tokenizer.Item, c compileCtx) (phpv.Runnable, error) {
	var attr phpv.ZClassAttr
	var classAttrs []*phpv.ZAttribute
	var err error

	// Handle #[...] attributes before class declaration
	if i.Type == tokenizer.T_ATTRIBUTE {
		classAttrs, err = parseAttributes(c)
		if err != nil {
			return nil, err
		}
		i, err = c.NextItem()
		if err != nil {
			return nil, err
		}
	}

	// If called from a modifier token (abstract/final/readonly), apply the
	// modifier directly rather than backing up (backup can lose a peeked token
	// when T_READONLY was preceded by a peekType() call in compileBaseSingle).
	// Then call parseZClassAttr to pick up any remaining modifiers.
	if i.Type == tokenizer.T_ABSTRACT || i.Type == tokenizer.T_FINAL || i.Type == tokenizer.T_READONLY {
		switch i.Type {
		case tokenizer.T_ABSTRACT:
			attr |= phpv.ZClassAbstract | phpv.ZClassExplicitAbstract
		case tokenizer.T_FINAL:
			attr |= phpv.ZClassFinal
		case tokenizer.T_READONLY:
			attr |= phpv.ZClassReadonly
		}
		err = parseZClassAttr(&attr, c)
		if err != nil {
			return nil, &phpv.PhpError{Err: err, Code: phpv.E_COMPILE_ERROR, Loc: i.Loc()}
		}
		i, err = c.NextItem()
		if err != nil {
			return nil, err
		}
	} else {
		err = parseZClassAttr(&attr, c)
		if err != nil {
			return nil, &phpv.PhpError{Err: err, Code: phpv.E_COMPILE_ERROR, Loc: i.Loc()}
		}
	}

	class := &phpobj.ZClass{
		L:          i.Loc(),
		Attr:       attr,
		Methods:    make(map[phpv.ZString]*phpv.ZClassMethod),
		Const:      make(map[phpv.ZString]*phpv.ZClassConst),
		H:          &phpv.ZClassHandlers{},
		Attributes: classAttrs,
	}

	switch i.Type {
	case tokenizer.T_CLASS:
	case tokenizer.T_INTERFACE:
		class.Type = phpv.ZClassTypeInterface
	case tokenizer.T_TRAIT:
		class.Type = phpv.ZClassTypeTrait
	default:
		return nil, i.Unexpected()
	}

	// Save enclosing scope info before wrapping in class context
	enclosingFunc := c.getFunc()
	enclosingClass := c.getClass()

	c = &zclassCompileCtx{c, class}

	// Set the compiling class so that deprecation messages can include the class name
	c.Global().SetCompilingClass(class)
	defer c.Global().SetCompilingClass(nil)

	err = parseClassLine(class, c)
	if err != nil {
		return nil, err
	}

	// PHP 8: Named class declarations may not be nested (inside a method within a class).
	// Anonymous classes (class.Name is empty) are allowed inside methods.
	if class.Name != "" && enclosingFunc != nil && enclosingClass != nil {
		return nil, &phpv.PhpError{
			Err:  fmt.Errorf("Class declarations may not be nested"),
			Code: phpv.E_COMPILE_ERROR,
			Loc:  class.L,
		}
	}

	i, err = c.NextItem()
	if err != nil {
		return nil, err
	}

	if !i.IsSingle('{') {
		return nil, i.Unexpected()
	}

	for {
		// we just read this item to grab location and check for '}'
		i, err := c.NextItem()
		if err != nil {
			return nil, err
		}

		// Skip doc comments
		for i.Type == tokenizer.T_DOC_COMMENT || i.Type == tokenizer.T_COMMENT {
			i, err = c.NextItem()
			if err != nil {
				return nil, err
			}
		}

		if i.IsSingle('}') {
			// end of class
			break
		}
		l := i.Loc()
		c.backup()

		// parse attrs if any (including #[...] attributes)
		var attr phpv.ZObjectAttr
		var setModifiers phpv.ZObjectAttr
		var memberAttrs []*phpv.ZAttribute
		if err := parseZObjectAttrFull(&attr, &setModifiers, &memberAttrs, c); err != nil {
			return nil, &phpv.PhpError{Err: err, Code: phpv.E_COMPILE_ERROR, Loc: l}
		}

		// read whatever comes next
		i, err = c.NextItem()
		if err != nil {
			return nil, err
		}

		// Check for typed property: type hint before $variable
		var propTypeHint *phpv.TypeHint
		// Handle DNF type when '(' was consumed by tryParseAsymmetricSet
		if parenConsumedByAsymmetric {
			c.backup()
			intersect, next, pErr := parseParenIntersection(c)
			if pErr != nil {
				return nil, pErr
			}
			if next.IsSingle('|') {
				propTypeHint, i, err = parseUnionTypeHint(intersect, c)
				if err != nil {
					return nil, err
				}
			} else {
				propTypeHint = intersect
				i = next
			}
		} else if i.IsSingle('(') {
			intersect, next, pErr := parseParenIntersection(c)
			if pErr != nil {
				return nil, pErr
			}
			if next.IsSingle('|') {
				propTypeHint, i, err = parseUnionTypeHint(intersect, c)
				if err != nil {
					return nil, err
				}
			} else {
				propTypeHint = intersect
				i = next
			}
		} else if i.Type == tokenizer.T_STRING || i.Type == tokenizer.T_ARRAY || i.Type == tokenizer.T_CALLABLE || i.IsSingle('?') {
			// Could be a type hint for a property, or a regular class name
			// Peek ahead to check if a T_VARIABLE follows (possibly after namespace parts)
			// Note: T_STATIC is NOT accepted here - "static" is not a valid property type
			isNullable := i.IsSingle('?')
			hint := i.Data
			if isNullable {
				i, err = c.NextItem()
				if err != nil {
					return nil, err
				}
				hint = i.Data
			}
			if i.Type == tokenizer.T_STRING || i.Type == tokenizer.T_ARRAY || i.Type == tokenizer.T_CALLABLE {
				// Consume namespace parts
				for {
					peek, err := c.NextItem()
					if err != nil {
						return nil, err
					}
					if peek.Type == tokenizer.T_NS_SEPARATOR {
						next, err := c.NextItem()
						if err != nil {
							return nil, err
						}
						hint = hint + "\\" + next.Data
					} else {
						i = peek
						break
					}
				}

				if i.IsSingle('|') {
					// Union type hint (int|string)
					propTypeHint = phpv.ParseTypeHint(phpv.ZString(hint))
					if isNullable {
						propTypeHint.Nullable = true
					}
					propTypeHint, i, err = parseUnionTypeHint(propTypeHint, c)
					if err != nil {
						return nil, err
					}
				} else if i.IsSingle('&') {
					// Intersection type hint (A&B)
					propTypeHint = phpv.ParseTypeHint(phpv.ZString(hint))
					if isNullable {
						propTypeHint.Nullable = true
					}
					// Read the next token (the type name after &)
					nextTok, nextErr := c.NextItem()
					if nextErr != nil {
						return nil, nextErr
					}
					if nextTok.Type == tokenizer.T_STRING || nextTok.Type == tokenizer.T_ARRAY || nextTok.Type == tokenizer.T_CALLABLE {
						propTypeHint, i, err = parseIntersectionTypeHint(propTypeHint, nextTok, c)
						if err != nil {
							return nil, err
						}
						// After intersection, check if followed by | for DNF: A&B|C
						if i.IsSingle('|') {
							propTypeHint, i, err = parseUnionTypeHint(propTypeHint, c)
							if err != nil {
								return nil, err
							}
						}
					} else {
						// Invalid token after &, return parse error
						return nil, nextTok.Unexpected()
					}
				}
				if i.Type == tokenizer.T_VARIABLE {
					// It was a type hint (or we already parsed the union)
					if propTypeHint == nil {
						// Resolve type hint through namespace for class type hints
						resolvedHint := string(c.resolveClassName(phpv.ZString(hint)))
						propTypeHint = phpv.ParseTypeHint(phpv.ZString(resolvedHint))
						if isNullable {
							propTypeHint.Nullable = true
						}
					}
				} else {
					// Not a typed property, back up
					c.backup()
					return nil, i.Unexpected()
				}
			} else if isNullable {
				return nil, i.Unexpected()
			} else {
				// Not a type hint, restore position
				// This shouldn't happen as we only enter this block for specific token types
				c.backup()
			}
		}

		// Validate property type hint
		if propTypeHint != nil {
			if err := validateTypeHint(propTypeHint, i.Loc()); err != nil {
				return nil, err
			}
		}

		switch i.Type {
		case tokenizer.T_VAR:
			// class variable, with possible default value and optional type hint
			i, err = c.NextItem()
			if err != nil {
				return nil, err
			}
			// After 'var', a type hint may appear before the $variable
			if propTypeHint == nil && i.Type != tokenizer.T_VARIABLE {
				// Try to parse a type hint
				if i.IsSingle('(') {
					// DNF type: var (A&B)|C $prop
					intersect, next, pErr := parseParenIntersection(c)
					if pErr != nil {
						return nil, pErr
					}
					if next.IsSingle('|') {
						propTypeHint, i, err = parseUnionTypeHint(intersect, c)
						if err != nil {
							return nil, err
						}
					} else {
						propTypeHint = intersect
						i = next
					}
				} else if i.Type == tokenizer.T_STRING || i.Type == tokenizer.T_ARRAY || i.Type == tokenizer.T_CALLABLE || i.IsSingle('?') {
					isNullable := i.IsSingle('?')
					hint := i.Data
					if isNullable {
						i, err = c.NextItem()
						if err != nil {
							return nil, err
						}
						hint = i.Data
					}
					if i.Type == tokenizer.T_STRING || i.Type == tokenizer.T_ARRAY || i.Type == tokenizer.T_CALLABLE {
						// Consume namespace parts
						for {
							peek, err := c.NextItem()
							if err != nil {
								return nil, err
							}
							if peek.Type == tokenizer.T_NS_SEPARATOR {
								next, err := c.NextItem()
								if err != nil {
									return nil, err
								}
								hint = hint + "\\" + next.Data
							} else {
								i = peek
								break
							}
						}
						if i.IsSingle('|') {
							propTypeHint = phpv.ParseTypeHint(phpv.ZString(hint))
							if isNullable {
								propTypeHint.Nullable = true
							}
							propTypeHint, i, err = parseUnionTypeHint(propTypeHint, c)
							if err != nil {
								return nil, err
							}
						} else if i.IsSingle('&') {
							propTypeHint = phpv.ParseTypeHint(phpv.ZString(hint))
							if isNullable {
								propTypeHint.Nullable = true
							}
							nextTok, nextErr := c.NextItem()
							if nextErr != nil {
								return nil, nextErr
							}
							if nextTok.Type == tokenizer.T_STRING || nextTok.Type == tokenizer.T_ARRAY || nextTok.Type == tokenizer.T_CALLABLE {
								propTypeHint, i, err = parseIntersectionTypeHint(propTypeHint, nextTok, c)
								if err != nil {
									return nil, err
								}
								if i.IsSingle('|') {
									propTypeHint, i, err = parseUnionTypeHint(propTypeHint, c)
									if err != nil {
										return nil, err
									}
								}
							} else {
								return nil, nextTok.Unexpected()
							}
						} else if i.Type == tokenizer.T_VARIABLE {
							resolvedHint := string(c.resolveClassName(phpv.ZString(hint)))
							propTypeHint = phpv.ParseTypeHint(phpv.ZString(resolvedHint))
							if isNullable {
								propTypeHint.Nullable = true
							}
						}
					}
				}
				if i.Type != tokenizer.T_VARIABLE {
					return nil, i.Unexpected()
				}
			}
			fallthrough
		case tokenizer.T_VARIABLE:
			for {
				prop := &phpv.ZClassProp{Modifiers: attr, SetModifiers: setModifiers, TypeHint: propTypeHint, Attributes: memberAttrs}
				prop.VarName = phpv.ZString(i.Data[1:])

				// PHP: certain types cannot be used for properties
				if propTypeHint != nil {
					if err := validatePropertyTypeHint(propTypeHint, class.Name, prop.VarName, i.Loc()); err != nil {
						return nil, err
					}
				}

				// Check for duplicate property declaration
				for _, existing := range class.Props {
					if existing.VarName == prop.VarName {
						return nil, &phpv.PhpError{
							Err:  fmt.Errorf("Cannot redeclare %s::$%s", class.Name, prop.VarName),
							Code: phpv.E_COMPILE_ERROR,
							Loc:  i.Loc(),
						}
					}
				}

				// Readonly class: all properties are implicitly readonly
				if class.Attr.Has(phpv.ZClassReadonly) {
					prop.Modifiers |= phpv.ZAttrReadonly
				}

				// check for default value
				i, err = c.NextItem()
				if err != nil {
					return nil, err
				}

				if i.IsSingle('=') {
					r, err := compileExpr(nil, c)
					if err != nil {
						return nil, err
					}
					// Validate closures in constant expressions (property defaults)
					if zc, ok := r.(*ZClosure); ok {
						if !zc.isStatic {
							return nil, &phpv.PhpError{
								Err:  fmt.Errorf("Closures in constant expressions must be static"),
								Code: phpv.E_COMPILE_ERROR,
								Loc:  zc.start,
							}
						}
						if len(zc.use) > 0 {
							return nil, &phpv.PhpError{
								Err:  fmt.Errorf("Cannot use(...) variables in constant expression"),
								Code: phpv.E_COMPILE_ERROR,
								Loc:  zc.start,
							}
						}
					}
					// New expressions are not allowed in non-static property defaults
					if !prop.Modifiers.IsStatic() && containsNewExpr(r) {
						return nil, &phpv.PhpError{
							Err:  fmt.Errorf("New expressions are not supported in this context"),
							Code: phpv.E_COMPILE_ERROR,
							Loc:  l,
						}
					}
					// Object casts are not allowed in non-static property defaults
					if !prop.Modifiers.IsStatic() {
						if op, ok := r.(*runOperator); ok && op.op == tokenizer.T_OBJECT_CAST {
							return nil, &phpv.PhpError{
								Err:  fmt.Errorf("Object casts are not supported in this context"),
								Code: phpv.E_COMPILE_ERROR,
								Loc:  l,
							}
						}
					}
					// parse default value for class variable
					prop.Default = &phpv.CompileDelayed{V: r}

					// Validate property default value against type hint at compile time
					if prop.TypeHint != nil {
						if err := validatePropertyDefault(r, prop.TypeHint, class.Name, prop.VarName, l); err != nil {
							return nil, err
						}
					}

					i, err = c.NextItem()
					if err != nil {
						return nil, err
					}
				}

				// Validate final+private property combination (not allowed)
				if prop.Modifiers.Has(phpv.ZAttrFinal) && prop.Modifiers.IsPrivate() {
					return nil, &phpv.PhpError{
						Err:  fmt.Errorf("Property cannot be both final and private"),
						Code: phpv.E_COMPILE_ERROR,
						Loc:  l,
					}
				}

				// Validate readonly property constraints
				if prop.Modifiers.IsReadonly() {
					if prop.TypeHint == nil {
						return nil, &phpv.PhpError{
							Err:  fmt.Errorf("Readonly property %s::$%s must have type", class.Name, prop.VarName),
							Code: phpv.E_COMPILE_ERROR,
							Loc:  l,
						}
					}
					if prop.Modifiers.IsStatic() {
						return nil, &phpv.PhpError{
							Err:  fmt.Errorf("Static property %s::$%s cannot be readonly", class.Name, prop.VarName),
							Code: phpv.E_COMPILE_ERROR,
							Loc:  l,
						}
					}
					if prop.Default != nil {
						return nil, &phpv.PhpError{
							Err:  fmt.Errorf("Readonly property %s::$%s cannot have default value", class.Name, prop.VarName),
							Code: phpv.E_COMPILE_ERROR,
							Loc:  l,
						}
					}
				}

				// Validate asymmetric visibility constraints
				if prop.SetModifiers != 0 {
					if prop.TypeHint == nil {
						return nil, &phpv.PhpError{
							Err:  fmt.Errorf("Property with asymmetric visibility %s::$%s must have type", class.Name, prop.VarName),
							Code: phpv.E_COMPILE_ERROR,
							Loc:  l,
						}
					}
					// Validate: set visibility must not be wider than read visibility
					setAccess := prop.SetModifiers & phpv.ZAttrAccess
					readAccess := prop.Modifiers & phpv.ZAttrAccess
					if setAccess == phpv.ZAttrPublic && readAccess != phpv.ZAttrPublic {
						return nil, &phpv.PhpError{
							Err:  fmt.Errorf("Visibility of property %s::$%s must not be weaker than set visibility", class.Name, prop.VarName),
							Code: phpv.E_COMPILE_ERROR,
							Loc:  l,
						}
					}
					if readAccess == phpv.ZAttrPrivate && setAccess != phpv.ZAttrPrivate {
						return nil, &phpv.PhpError{
							Err:  fmt.Errorf("Visibility of property %s::$%s must not be weaker than set visibility", class.Name, prop.VarName),
							Code: phpv.E_COMPILE_ERROR,
							Loc:  l,
						}
					}
				}

				// Property hooks: $prop { get { } set { } }
				if i.IsSingle('{') {
					// Abstract properties cannot be final
					if prop.Modifiers.Has(phpv.ZAttrAbstract) && prop.Modifiers.Has(phpv.ZAttrFinal) {
						return nil, &phpv.PhpError{
							Err:  fmt.Errorf("Cannot use the final modifier on an abstract property"),
							Code: phpv.E_COMPILE_ERROR,
							Loc:  l,
						}
					}

					// Hooked properties cannot be readonly
					if prop.Modifiers.IsReadonly() {
						return nil, &phpv.PhpError{
							Err:  fmt.Errorf("Hooked properties cannot be readonly"),
							Code: phpv.E_COMPILE_ERROR,
							Loc:  l,
						}
					}

					// Static properties cannot have hooks
					if prop.Modifiers.IsStatic() {
						return nil, &phpv.PhpError{
							Err:  fmt.Errorf("Cannot declare hooks for static property"),
							Code: phpv.E_COMPILE_ERROR,
							Loc:  l,
						}
					}

					// Interface-specific property validations
					if class.Type == phpv.ZClassTypeInterface {
						if prop.Modifiers.IsPrivate() || prop.Modifiers.IsProtected() {
							return nil, &phpv.PhpError{
								Err:  fmt.Errorf("Property in interface cannot be protected or private"),
								Code: phpv.E_COMPILE_ERROR,
								Loc:  l,
							}
						}
						if prop.Modifiers.Has(phpv.ZAttrAbstract) {
							return nil, &phpv.PhpError{
								Err:  fmt.Errorf("Property in interface cannot be explicitly abstract. All interface members are implicitly abstract"),
								Code: phpv.E_COMPILE_ERROR,
								Loc:  l,
							}
						}
						if prop.Modifiers.Has(phpv.ZAttrFinal) {
							return nil, &phpv.PhpError{
								Err:  fmt.Errorf("Property in interface cannot be final"),
								Code: phpv.E_COMPILE_ERROR,
								Loc:  l,
							}
						}
					}

					if err := compilePropertyHooks(prop, class, c); err != nil {
						return nil, err
					}

					// Virtual property with default value is not allowed.
					// A property is virtual when both hooks are present and neither
					// references the backing store. Get-only or set-only properties
					// with defaults are always backed.
					if prop.Default != nil && !prop.IsBacked &&
						(prop.HasGetDeclared || prop.GetHook != nil) &&
						(prop.HasSetDeclared || prop.SetHook != nil) {
						return nil, &phpv.PhpError{
							Err:  fmt.Errorf("Cannot specify default value for virtual hooked property %s::$%s", class.Name, prop.VarName),
							Code: phpv.E_COMPILE_ERROR,
							Loc:  l,
						}
					}

					// Validate asymmetric visibility on virtual properties (PHP 8.4)
					// A virtual property with only a get hook is "read-only virtual"
					// A virtual property with only a set hook is "write-only virtual"
					// Neither should specify asymmetric visibility.
					if prop.SetModifiers != 0 && prop.IsVirtual() {
						if prop.GetHook != nil && prop.SetHook == nil {
							return nil, &phpv.PhpError{
								Err:  fmt.Errorf("Read-only virtual property %s::$%s must not specify asymmetric visibility", class.Name, prop.VarName),
								Code: phpv.E_COMPILE_ERROR,
								Loc:  l,
							}
						}
						if prop.SetHook != nil && prop.GetHook == nil {
							return nil, &phpv.PhpError{
								Err:  fmt.Errorf("Write-only virtual property %s::$%s must not specify asymmetric visibility", class.Name, prop.VarName),
								Code: phpv.E_COMPILE_ERROR,
								Loc:  l,
							}
						}
					}

					// Validate abstract property: must have at least one abstract hook
					if prop.Modifiers.Has(phpv.ZAttrAbstract) {
						if !prop.GetIsAbstract && !prop.SetIsAbstract {
							return nil, &phpv.PhpError{
								Err:  fmt.Errorf("Abstract property %s::$%s must specify at least one abstract hook", class.Name, prop.VarName),
								Code: phpv.E_COMPILE_ERROR,
								Loc:  l,
							}
						}
					}

					class.Props = append(class.Props, prop)
					break
				}

				// Abstract properties without hooks are not allowed
				if attr&phpv.ZAttrAbstract != 0 {
					return nil, &phpv.PhpError{
						Err:  fmt.Errorf("Only hooked properties may be declared abstract"),
						Code: phpv.E_COMPILE_ERROR,
						Loc:  l,
					}
				}

				class.Props = append(class.Props, prop)
				if i.IsSingle(';') {
					break
				}
				if i.IsSingle(',') {
					i, err = c.NextItem()
					if err != nil {
						return nil, err
					}

					if i.Type != tokenizer.T_VARIABLE {
						return nil, i.Unexpected()
					}
					continue
				}

				return nil, i.Unexpected()
			}
		case tokenizer.T_CONST:
			// Check for invalid readonly modifier on constants
			if attr&phpv.ZAttrReadonly != 0 {
				phpErr := &phpv.PhpError{
					Err:  fmt.Errorf("Cannot use the readonly modifier on a class constant"),
					Code: phpv.E_COMPILE_ERROR,
					Loc:  i.Loc(),
				}
				c.Global().LogError(phpErr)
				return nil, phpv.ExitError(255)
			}
			// Check for invalid modifiers on constants
			if attr.IsStatic() {
				phpErr := &phpv.PhpError{
					Err:  fmt.Errorf("Cannot use the static modifier on a class constant"),
					Code: phpv.E_COMPILE_ERROR,
					Loc:  i.Loc(),
				}
				c.Global().LogError(phpErr)
				return nil, phpv.ExitError(255)
			}
			if attr.Has(phpv.ZAttrAbstract) {
				phpErr := &phpv.PhpError{
					Err:  fmt.Errorf("Cannot use the abstract modifier on a class constant"),
					Code: phpv.E_COMPILE_ERROR,
					Loc:  i.Loc(),
				}
				c.Global().LogError(phpErr)
				return nil, phpv.ExitError(255)
			}
			// const [TYPE] K = V [, K2 = V2 ...];
			// PHP 8.3+ typed class constants: const TYPE NAME = VALUE;

			// Parse optional type hint before the constant name(s).
			var constTypeHint *phpv.TypeHint

			i, err = c.NextItem()
			if err != nil {
				return nil, err
			}

			if i.IsSingle('(') {
				// DNF type starting with '(' e.g. const (A&B)|C NAME = value;
				intersect, next, pErr := parseParenIntersection(c)
				if pErr != nil {
					return nil, pErr
				}
				if next.IsSingle('|') {
					constTypeHint, i, err = parseUnionTypeHint(intersect, c)
					if err != nil {
						return nil, err
					}
				} else {
					constTypeHint = intersect
					i = next
				}
			} else if i.IsSingle('?') {
				// Nullable type: const ?TYPE NAME = value;
				ni, nErr := c.NextItem()
				if nErr != nil {
					return nil, nErr
				}
				hint := ni.Data
				for {
					peek, pErr := c.NextItem()
					if pErr != nil {
						return nil, pErr
					}
					if peek.Type == tokenizer.T_NS_SEPARATOR {
						next2, nErr2 := c.NextItem()
						if nErr2 != nil {
							return nil, nErr2
						}
						hint = hint + "\\" + next2.Data
					} else {
						i = peek
						break
					}
				}
				if i.IsSingle('|') {
					constTypeHint = phpv.ParseTypeHint(phpv.ZString(hint))
					constTypeHint.Nullable = true
					constTypeHint, i, err = parseUnionTypeHint(constTypeHint, c)
					if err != nil {
						return nil, err
					}
				} else {
					resolvedHint := string(c.resolveClassName(phpv.ZString(hint)))
					constTypeHint = phpv.ParseTypeHint(phpv.ZString(resolvedHint))
					constTypeHint.Nullable = true
				}
			} else if i.IsSemiReserved() || i.Type == tokenizer.T_NS_SEPARATOR {
				// Could be a const name (untyped) or a type hint (typed).
				firstToken := i
				hint := firstToken.Data

				if firstToken.Type == tokenizer.T_NS_SEPARATOR {
					ni, nErr := c.NextItem()
					if nErr != nil {
						return nil, nErr
					}
					hint = "\\" + ni.Data
				}

				// Consume namespace parts
				for {
					peek, pErr := c.NextItem()
					if pErr != nil {
						return nil, pErr
					}
					if peek.Type == tokenizer.T_NS_SEPARATOR {
						next2, nErr := c.NextItem()
						if nErr != nil {
							return nil, nErr
						}
						hint = hint + "\\" + next2.Data
					} else {
						i = peek
						break
					}
				}

				if i.IsSingle('=') {
					// No type hint, 'hint' is the const name
					c.backup()
					i = firstToken
					i.Data = hint
				} else if i.IsSingle('|') {
					// Union type
					constTypeHint = phpv.ParseTypeHint(phpv.ZString(c.resolveClassName(phpv.ZString(hint))))
					constTypeHint, i, err = parseUnionTypeHint(constTypeHint, c)
					if err != nil {
						return nil, err
					}
				} else if i.IsSingle('&') {
					// Intersection type
					constTypeHint = phpv.ParseTypeHint(phpv.ZString(c.resolveClassName(phpv.ZString(hint))))
					nextTok, nextErr := c.NextItem()
					if nextErr != nil {
						return nil, nextErr
					}
					constTypeHint, i, err = parseIntersectionTypeHint(constTypeHint, nextTok, c)
					if err != nil {
						return nil, err
					}
					if i.IsSingle('|') {
						constTypeHint, i, err = parseUnionTypeHint(constTypeHint, c)
						if err != nil {
							return nil, err
						}
					}
				} else if i.IsSemiReserved() {
					// 'hint' was the type, i is the const name
					resolvedHint := string(c.resolveClassName(phpv.ZString(hint)))
					constTypeHint = phpv.ParseTypeHint(phpv.ZString(resolvedHint))
				} else {
					return nil, i.Unexpected()
				}
			} else {
				return nil, i.Unexpected()
			}

			// Validate constant type hint if present
			if constTypeHint != nil {
				if err := validateTypeHint(constTypeHint, i.Loc()); err != nil {
					return nil, err
				}
			}

			for {
				if !i.IsSemiReserved() {
					return nil, i.Unexpected()
				}
				constName := i.Data

				// Validate class constant type restrictions
				if constTypeHint != nil {
					if err := validateClassConstTypeHint(constTypeHint, class.Name, constName, i.Loc()); err != nil {
						return nil, err
					}
				}

				// 'class' is reserved for class name fetching (Foo::class)
				if phpv.ZString(constName).ToLower() == "class" {
					return nil, &phpv.PhpError{
						Err:  fmt.Errorf("A class constant must not be called 'class'; it is reserved for class name fetching"),
						Code: phpv.E_COMPILE_ERROR,
						Loc:  i.Loc(),
					}
				}

				// Check for duplicate constant
				if _, exists := class.Const[phpv.ZString(constName)]; exists {
					phpErr := &phpv.PhpError{
						Err:  fmt.Errorf("Cannot redefine class constant %s::%s", class.Name, constName),
						Code: phpv.E_COMPILE_ERROR,
						Loc:  i.Loc(),
					}
					c.Global().LogError(phpErr)
					return nil, phpv.ExitError(255)
				}

				// Interface constants must be public
				if class.Type == phpv.ZClassTypeInterface && (attr.IsPrivate() || attr.IsProtected()) {
					phpErr := &phpv.PhpError{
						Err:  fmt.Errorf("Access type for interface constant %s::%s must be public", class.Name, constName),
						Code: phpv.E_COMPILE_ERROR,
						Loc:  i.Loc(),
					}
					c.Global().LogError(phpErr)
					return nil, phpv.ExitError(255)
				}

				// =
				i, err = c.NextItem()
				if err != nil {
					return nil, err
				}
				if !i.IsSingle('=') {
					return nil, i.Unexpected()
				}

				var v phpv.Runnable
				v, err = compileExpr(nil, c)
				if err != nil {
					return nil, err
				}

				// Validate closures in class constant expressions
				if zc, ok := v.(*ZClosure); ok {
					if !zc.isStatic {
						return nil, &phpv.PhpError{
							Err:  fmt.Errorf("Closures in constant expressions must be static"),
							Code: phpv.E_COMPILE_ERROR,
							Loc:  zc.start,
						}
					}
					if len(zc.use) > 0 {
						return nil, &phpv.PhpError{
							Err:  fmt.Errorf("Cannot use(...) variables in constant expression"),
							Code: phpv.E_COMPILE_ERROR,
							Loc:  zc.start,
						}
					}
				}

				// Check for static::class in class constant (compile-time error)
				if loc := checkStaticClassInConstExpr(v); loc != nil {
					return nil, &phpv.PhpError{
						Err:  fmt.Errorf("static::class cannot be used for compile-time class name resolution"),
						Code: phpv.E_COMPILE_ERROR,
						Loc:  loc,
					}
				}
				// Check for parent::class in class with no parent
				if _, loc := checkParentClassInConstExpr(v, class); loc != nil {
					return nil, &phpv.PhpError{
						Err:  fmt.Errorf("Cannot use \"parent\" when current class scope has no parent"),
						Code: phpv.E_COMPILE_ERROR,
						Loc:  loc,
					}
				}

				// Check for invalid operations in constant expressions
				if containsRuntimeOps(v) {
					return nil, &phpv.PhpError{
						Err:  fmt.Errorf("Constant expression contains invalid operations"),
						Code: phpv.E_COMPILE_ERROR,
						Loc:  i.Loc(),
					}
				}

				// New expressions are not allowed in class constants
				if containsNewExpr(v) {
					return nil, &phpv.PhpError{
						Err:  fmt.Errorf("New expressions are not supported in this context"),
						Code: phpv.E_COMPILE_ERROR,
						Loc:  i.Loc(),
					}
				}

				cn := phpv.ZString(constName)
				// Private final constants are not visible to subclasses, so final is useless
				if attr.Has(phpv.ZAttrFinal) && attr.IsPrivate() {
					return nil, &phpv.PhpError{
						Err:  fmt.Errorf("Private constant %s::%s cannot be final as it is not visible to other classes", class.Name, cn),
						Code: phpv.E_COMPILE_ERROR,
						Loc:  i.Loc(),
					}
				}
				class.Const[cn] = &phpv.ZClassConst{
					Value:      &phpv.CompileDelayed{V: v},
					Modifiers:  attr,
					Attributes: memberAttrs,
					TypeHint:   constTypeHint,
				}
				class.ConstOrder = append(class.ConstOrder, cn)

				i, err = c.NextItem()
				if err != nil {
					return nil, err
				}
				if i.IsSingle(';') {
					break
				}
				if !i.IsSingle(',') {
					return nil, i.Unexpected()
				}

				// For comma-separated constants, read the next const name
				i, err = c.NextItem()
				if err != nil {
					return nil, err
				}
			}
		case tokenizer.T_USE:
			// Trait usage: use TraitName [, TraitName2] [{ ... }];
			var traitNames []phpv.ZString
			for {
				i, err = c.NextItem()
				if err != nil {
					return nil, err
				}
				if i.Type != tokenizer.T_STRING && i.Type != tokenizer.T_NS_SEPARATOR && i.Type != tokenizer.T_STATIC && i.Type != tokenizer.T_READONLY {
					return nil, i.Unexpected()
				}
				// Parse potentially namespaced trait name
				fullyQualified := false
				if i.Type == tokenizer.T_NS_SEPARATOR {
					fullyQualified = true
					i, err = c.NextItem()
					if err != nil {
						return nil, err
					}
				}
				name := i.Data
				for {
					peek, err := c.NextItem()
					if err != nil {
						return nil, err
					}
					if peek.Type == tokenizer.T_NS_SEPARATOR {
						next, err := c.NextItem()
						if err != nil {
							return nil, err
						}
						name += "\\" + next.Data
					} else {
						c.backup()
						break
					}
				}
				// Resolve trait name through namespace
				var resolved phpv.ZString
				if fullyQualified {
					resolved = c.resolveClassName("\\" + phpv.ZString(name))
				} else {
					resolved = c.resolveClassName(phpv.ZString(name))
				}
				traitNames = append(traitNames, resolved)

				i, err = c.NextItem()
				if err != nil {
					return nil, err
				}
				if i.IsSingle(',') {
					continue
				}
				break
			}

			// Handle trait adaptation block { ... } or semicolon
			var aliases []traitAlias
			var insteadofs []traitInsteadof
			if i.IsSingle('{') {
				// Parse trait adaptations
				for {
					i, err = c.NextItem()
					if err != nil {
						return nil, err
					}
					if i.IsSingle('}') {
						break
					}
					// Parse: [TraitName::]method as [visibility] newname;
					// Parse: TraitName::method insteadof OtherTrait;
					firstName := i.Data
					i, err = c.NextItem()
					if err != nil {
						return nil, err
					}
					if i.Type == tokenizer.T_PAAMAYIM_NEKUDOTAYIM {
						// TraitName::method
						methodItem, err := c.NextItem()
						if err != nil {
							return nil, err
						}
						methodName := methodItem.Data
						i, err = c.NextItem()
						if err != nil {
							return nil, err
						}
						if i.Type == tokenizer.T_AS {
							// as [visibility] [final] newname
							i, err = c.NextItem()
							if err != nil {
								return nil, err
							}
							var newAttr phpv.ZObjectAttr
							newName := ""
							// Check for abstract/static (not allowed)
							if i.Type == tokenizer.T_ABSTRACT {
								return nil, &phpv.PhpError{
									Err:  fmt.Errorf("Cannot use \"abstract\" as method modifier in trait alias"),
									Code: phpv.E_COMPILE_ERROR,
									Loc:  i.Loc(),
								}
							}
							if i.Type == tokenizer.T_STATIC {
								return nil, &phpv.PhpError{
									Err:  fmt.Errorf("Cannot use \"static\" as method modifier in trait alias"),
									Code: phpv.E_COMPILE_ERROR,
									Loc:  i.Loc(),
								}
							}
							if i.Type == tokenizer.T_READONLY {
								return nil, &phpv.PhpError{
									Err:  fmt.Errorf("Cannot use the readonly modifier on a method"),
									Code: phpv.E_COMPILE_ERROR,
									Loc:  i.Loc(),
								}
							}
							// Check for visibility modifier
							switch i.Type {
							case tokenizer.T_PUBLIC:
								newAttr = phpv.ZAttrPublic
								i, err = c.NextItem()
								if err != nil {
									return nil, err
								}
							case tokenizer.T_PROTECTED:
								newAttr = phpv.ZAttrProtected
								i, err = c.NextItem()
								if err != nil {
									return nil, err
								}
							case tokenizer.T_PRIVATE:
								newAttr = phpv.ZAttrPrivate
								i, err = c.NextItem()
								if err != nil {
									return nil, err
								}
							case tokenizer.T_FINAL:
								newAttr = phpv.ZAttrFinal
								i, err = c.NextItem()
								if err != nil {
									return nil, err
								}
							}
							if i.Type == tokenizer.T_STRING {
								newName = i.Data
								i, err = c.NextItem()
								if err != nil {
									return nil, err
								}
							}
							if !i.IsSingle(';') {
								return nil, i.Unexpected()
							}
							aliases = append(aliases, traitAlias{
								traitName:  phpv.ZString(firstName),
								methodName: phpv.ZString(methodName),
								newName:    phpv.ZString(newName),
								newAttr:    newAttr,
							})
						} else if i.Type == tokenizer.T_INSTEADOF {
							// insteadof OtherTrait [, OtherTrait2];
							var excludedTraits []phpv.ZString
							for {
								i, err = c.NextItem()
								if err != nil {
									return nil, err
								}
								excludedTraits = append(excludedTraits, phpv.ZString(i.Data))
								i, err = c.NextItem()
								if err != nil {
									return nil, err
								}
								if i.IsSingle(';') {
									break
								}
								if !i.IsSingle(',') {
									return nil, i.Unexpected()
								}
							}
							insteadofs = append(insteadofs, traitInsteadof{
								traitName:  phpv.ZString(firstName),
								methodName: phpv.ZString(methodName),
								insteadOf:  excludedTraits,
							})
						} else {
							return nil, i.Unexpected()
						}
					} else if i.Type == tokenizer.T_AS {
						// method as [visibility] [final] newname;
						i, err = c.NextItem()
						if err != nil {
							return nil, err
						}
						var newAttr phpv.ZObjectAttr
						newName := ""
						// Check for abstract/static (not allowed)
						if i.Type == tokenizer.T_ABSTRACT {
							return nil, &phpv.PhpError{
								Err:  fmt.Errorf("Cannot use \"abstract\" as method modifier in trait alias"),
								Code: phpv.E_COMPILE_ERROR,
								Loc:  i.Loc(),
							}
						}
						if i.Type == tokenizer.T_STATIC {
							return nil, &phpv.PhpError{
								Err:  fmt.Errorf("Cannot use \"static\" as method modifier in trait alias"),
								Code: phpv.E_COMPILE_ERROR,
								Loc:  i.Loc(),
							}
						}
						if i.Type == tokenizer.T_READONLY {
							return nil, &phpv.PhpError{
								Err:  fmt.Errorf("Cannot use the readonly modifier on a method"),
								Code: phpv.E_COMPILE_ERROR,
								Loc:  i.Loc(),
							}
						}
						switch i.Type {
						case tokenizer.T_PUBLIC:
							newAttr = phpv.ZAttrPublic
							i, err = c.NextItem()
							if err != nil {
								return nil, err
							}
						case tokenizer.T_PROTECTED:
							newAttr = phpv.ZAttrProtected
							i, err = c.NextItem()
							if err != nil {
								return nil, err
							}
						case tokenizer.T_PRIVATE:
							newAttr = phpv.ZAttrPrivate
							i, err = c.NextItem()
							if err != nil {
								return nil, err
							}
						case tokenizer.T_FINAL:
							newAttr = phpv.ZAttrFinal
							i, err = c.NextItem()
							if err != nil {
								return nil, err
							}
						}
						if i.Type == tokenizer.T_STRING {
							newName = i.Data
							i, err = c.NextItem()
							if err != nil {
								return nil, err
							}
						}
						if !i.IsSingle(';') {
							return nil, i.Unexpected()
						}
						aliases = append(aliases, traitAlias{
							methodName: phpv.ZString(firstName),
							newName:    phpv.ZString(newName),
							newAttr:    newAttr,
						})
					} else {
						return nil, i.Unexpected()
					}
				}
			} else if !i.IsSingle(';') {
				return nil, i.Unexpected()
			}

			// Store trait usage info on the class for runtime resolution
			class.TraitUses = append(class.TraitUses, phpv.ZClassTraitUse{
				TraitNames: traitNames,
				Aliases:    convertAliases(aliases),
				Insteadof:  convertInsteadofs(insteadofs),
			})
		case tokenizer.T_FUNCTION:
			// Check for invalid readonly modifier on methods
			if attr&phpv.ZAttrReadonly != 0 {
				return nil, &phpv.PhpError{
					Err:  fmt.Errorf("Cannot use the readonly modifier on a method"),
					Code: phpv.E_COMPILE_ERROR,
					Loc:  l,
				}
			}
			// next must be a string (method name)
			i, err := c.NextItem()
			if err != nil {
				return nil, err
			}

			rref := false
			if i.IsSingle('&') {
				rref = true
				i, err = c.NextItem()
				if err != nil {
					return nil, err
				}
			}

			if !i.IsSemiReserved() {
				return nil, i.Unexpected()
			}

			// Check for invalid abstract+final combination on methods
			if attr&phpv.ZAttrAbstract != 0 && attr&phpv.ZAttrFinal != 0 {
				return nil, &phpv.PhpError{
					Err:  fmt.Errorf("Cannot use the final modifier on an abstract method"),
					Code: phpv.E_COMPILE_ERROR,
					Loc:  l,
				}
			}

			// Check for abstract+private combination (abstract methods cannot be private,
			// EXCEPT in traits where PHP 8.0+ allows private abstract methods)
			if attr&phpv.ZAttrAbstract != 0 && attr&phpv.ZAttrPrivate != 0 && class.Type != phpv.ZClassTypeTrait {
				return nil, &phpv.PhpError{
					Err:  fmt.Errorf("Abstract function %s::%s() cannot be declared private", class.Name, i.Data),
					Code: phpv.E_COMPILE_ERROR,
					Loc:  l,
				}
			}

			var f phpv.Callable

			optionalBody := class.Type == phpv.ZClassTypeInterface || attr&phpv.ZAttrAbstract != 0
			f, err = compileFunctionWithName(phpv.ZString(i.Data), c, l, rref, optionalBody)
			if err != nil {
				return nil, err
			}
			f.(*ZClosure).class = class

			// Copy attributes to the ZClosure so that deprecation checks work
			// when the method is called through call_user_func or other indirect paths
			if len(memberAttrs) > 0 {
				f.(*ZClosure).attributes = append(f.(*ZClosure).attributes, memberAttrs...)
			}

			// an interface method with a body is not a parse error,
			// so delay returning an error when code is ran
			_, emptyBody := f.(*ZClosure).code.(phpv.RunNull)

			// register method
			method := &phpv.ZClassMethod{
				Name:       phpv.ZString(i.Data),
				Modifiers:  attr,
				Method:     f,
				Class:      class,
				Empty:      emptyBody,
				Loc:        l,
				Attributes: memberAttrs,
			}

			if x := method.Name.ToLower(); x == "__construct" {
				//if class.Constructor != nil {
				class.Handlers().Constructor = method

				// Handle constructor property promotion (PHP 8.0+)
				isConstructor := method.Name.ToLower() == "__construct"
				if fga, ok := f.(phpv.FuncGetArgs); ok {
					for _, arg := range fga.GetArgs() {
						if arg.Promotion != 0 {
							// Promoted properties only allowed in constructors
							if !isConstructor {
								return nil, &phpv.PhpError{
									Err:  fmt.Errorf("Cannot declare promoted property outside a constructor"),
									Code: phpv.E_COMPILE_ERROR,
									Loc:  l,
								}
							}
							// Variadic parameters cannot be promoted
							if arg.Variadic {
								return nil, &phpv.PhpError{
									Err:  fmt.Errorf("Cannot declare variadic promoted property"),
									Code: phpv.E_COMPILE_ERROR,
									Loc:  l,
								}
							}
							// callable type cannot be used as property type
							if arg.Hint != nil && arg.Hint.String() == "callable" {
								argLoc := l
								if arg.Loc != nil {
									argLoc = arg.Loc
								}
								return nil, &phpv.PhpError{
									Err:  fmt.Errorf("Property %s::$%s cannot have type callable", class.Name, arg.VarName),
									Code: phpv.E_COMPILE_ERROR,
									Loc:  argLoc,
								}
							}
							// Check for duplicate property names
							for _, existing := range class.Props {
								if existing.VarName == arg.VarName {
									return nil, &phpv.PhpError{
										Err:  fmt.Errorf("Cannot redeclare %s::$%s", class.Name, arg.VarName),
										Code: phpv.E_COMPILE_ERROR,
										Loc:  l,
									}
								}
							}
							modifiers := arg.Promotion
							// Readonly class: all properties are implicitly readonly
							if class.Attr.Has(phpv.ZClassReadonly) {
								modifiers |= phpv.ZAttrReadonly
							}
							// Validate asymmetric visibility constraints for CPP
							if arg.SetPromotion != 0 {
								if arg.Hint == nil {
									return nil, &phpv.PhpError{
										Err:  fmt.Errorf("Property with asymmetric visibility %s::$%s must have type", class.Name, arg.VarName),
										Code: phpv.E_COMPILE_ERROR,
										Loc:  l,
									}
								}
								// Validate: set visibility must not be wider than read visibility
								setAccess := arg.SetPromotion & phpv.ZAttrAccess
								readAccess := modifiers & phpv.ZAttrAccess
								if setAccess == phpv.ZAttrPublic && readAccess != phpv.ZAttrPublic {
									return nil, &phpv.PhpError{
										Err:  fmt.Errorf("Visibility of property %s::$%s must not be weaker than set visibility", class.Name, arg.VarName),
										Code: phpv.E_COMPILE_ERROR,
										Loc:  l,
									}
								}
								if readAccess == phpv.ZAttrPrivate && setAccess != phpv.ZAttrPrivate {
									return nil, &phpv.PhpError{
										Err:  fmt.Errorf("Visibility of property %s::$%s must not be weaker than set visibility", class.Name, arg.VarName),
										Code: phpv.E_COMPILE_ERROR,
										Loc:  l,
									}
								}
							}
							// Validate readonly promoted property must have type
							if modifiers.IsReadonly() && arg.Hint == nil {
								return nil, &phpv.PhpError{
									Err:  fmt.Errorf("Readonly property %s::$%s must have type", class.Name, arg.VarName),
									Code: phpv.E_COMPILE_ERROR,
									Loc:  l,
								}
							}
							prop := &phpv.ZClassProp{
								VarName:      arg.VarName,
								Modifiers:    modifiers,
								SetModifiers: arg.SetPromotion,
								TypeHint:     arg.Hint,
								Attributes:   arg.Attributes,
							}
							// Copy property hooks from promoted property (PHP 8.4)
							if arg.PromotionHooks != nil {
								// Hooked properties cannot be readonly
								if modifiers.IsReadonly() {
									return nil, &phpv.PhpError{
										Err:  fmt.Errorf("Hooked properties cannot be readonly"),
										Code: phpv.E_COMPILE_ERROR,
										Loc:  l,
									}
								}
								prop.HasHooks = arg.PromotionHooks.HasHooks
								prop.GetHook = arg.PromotionHooks.GetHook
								prop.SetHook = arg.PromotionHooks.SetHook
								prop.SetParam = arg.PromotionHooks.SetParam
								prop.HasGetDeclared = arg.PromotionHooks.HasGetDeclared
								prop.HasSetDeclared = arg.PromotionHooks.HasSetDeclared
								prop.GetIsAbstract = arg.PromotionHooks.GetIsAbstract
								prop.SetIsAbstract = arg.PromotionHooks.SetIsAbstract
								prop.IsBacked = arg.PromotionHooks.IsBacked
							}
							class.Props = append(class.Props, prop)
						}
					}
				}
			} else {
				// Non-constructor methods: check for promoted properties (not allowed)
				if fga, ok := f.(phpv.FuncGetArgs); ok {
					for _, arg := range fga.GetArgs() {
						if arg.Promotion != 0 {
							return nil, &phpv.PhpError{
								Err:  fmt.Errorf("Cannot declare promoted property outside a constructor"),
								Code: phpv.E_COMPILE_ERROR,
								Loc:  l,
							}
						}
					}
				}
			}
			// Validate magic method return type declarations
			if err := validateMagicMethodReturnType(class, method, l); err != nil {
				return nil, err
			}

			methodKey := method.Name.ToLower()
			if _, ok := class.Methods[methodKey]; ok {
				return nil, &phpv.PhpError{
					Err:  fmt.Errorf("Cannot redeclare %s::%s()", class.Name, method.Name),
					Code: phpv.E_COMPILE_ERROR,
					Loc:  l,
				}
			}
			class.Methods[methodKey] = method
			class.MethodOrder = append(class.MethodOrder, methodKey)
		case tokenizer.T_CASE:
			// "case" keyword used in a class (not an enum)
			return nil, &phpv.PhpError{
				Err:  fmt.Errorf("Case can only be used in enums"),
				Code: phpv.E_COMPILE_ERROR,
				Loc:  i.Loc(),
			}
		default:
			return nil, i.Unexpected()
		}
	}

	// Validate: non-abstract, non-interface classes must not have abstract methods
	if !class.Attr.Has(phpv.ZClassAbstract) && class.Type != phpv.ZClassTypeInterface && class.Type != phpv.ZClassTypeTrait {
		for _, m := range class.Methods {
			if m.Modifiers&phpv.ZAttrAbstract != 0 {
				var errMsg string
				if class.Name == "" {
					// Anonymous class
					errMsg = fmt.Sprintf("Anonymous class method %s() must not be abstract", m.Name)
				} else {
					errMsg = fmt.Sprintf("Class %s declares abstract method %s() and must therefore be declared abstract", class.Name, m.Name)
				}
				return nil, &phpv.PhpError{
					Err:  fmt.Errorf("%s", errMsg),
					Code: phpv.E_COMPILE_ERROR,
					Loc:  m.Loc,
				}
			}
		}
	}

	// Wrap class to perform post-compilation checks (trait deprecation, property Override)
	if len(class.TraitUses) > 0 || classHasOverrideProperty(class) {
		return &runClassWithTraitDeprecationCheck{class: class}, nil
	}

	return class, nil
}

// runClassWithTraitDeprecationCheck wraps a ZClass to perform post-compilation checks:
// - Trait deprecation: emit E_USER_DEPRECATED for deprecated traits
// - Property Override: validate #[\Override] on properties
type runClassWithTraitDeprecationCheck struct {
	class *phpobj.ZClass
}

func (r *runClassWithTraitDeprecationCheck) Run(ctx phpv.Context) (*phpv.ZVal, error) {
	// First, register and compile the class normally
	val, err := r.class.Run(ctx)
	if err != nil {
		return val, err
	}

	// Validate #[\Override] on properties
	if err := validatePropertyOverride(ctx, r.class); err != nil {
		return nil, err
	}

	// After compilation, check if any used traits have #[\Deprecated]
	for _, tu := range r.class.TraitUses {
		for _, traitName := range tu.TraitNames {
			traitClass, err := ctx.Global().GetClass(ctx, traitName, false)
			if err != nil {
				continue
			}
			tc, ok := traitClass.(*phpobj.ZClass)
			if !ok {
				continue
			}
			for _, attr := range tc.Attributes {
				if attr.ClassName == "Deprecated" || attr.ClassName == "\\Deprecated" {
					usingName := string(r.class.GetName())
					traitDisplayName := string(tc.GetName())
					msg := FormatDeprecatedMsg("Trait", traitDisplayName+" used by "+usingName, attr)
					if err := ctx.UserDeprecated("%s", msg, logopt.NoFuncName(true)); err != nil {
						return nil, err
					}
					break
				}
			}
		}
	}

	return val, nil
}

func (r *runClassWithTraitDeprecationCheck) Dump(w io.Writer) error {
	return r.class.Dump(w)
}

// GetClass returns the underlying ZClass, used by anonymous class compilation.
func (r *runClassWithTraitDeprecationCheck) GetClass() *phpobj.ZClass {
	return r.class
}

func parseClassLine(class *phpobj.ZClass, c compileCtx) error {
	i, err := c.NextItem()
	if err != nil {
		return err
	}

	// Semi-reserved keywords (like 'enum') can be used as class names,
	// but NOT extends/implements/readonly which start the class body definition.
	if i.Type == tokenizer.T_STRING || (i.IsSemiReserved() && i.Type != tokenizer.T_EXTENDS && i.Type != tokenizer.T_IMPLEMENTS && i.Type != tokenizer.T_READONLY) {
		className := phpv.ZString(i.Data)
		// Prepend current namespace to class name
		ns := c.getNamespace()
		if ns != "" {
			className = ns + "\\" + className
		}
		// Check reserved class names (use the short name, not the namespace-qualified name)
		lowerName := phpv.ZString(strings.ToLower(i.Data))
		classKind := "class"
		if class.Type == phpv.ZClassTypeInterface {
			classKind = "interface"
		} else if class.Type == phpv.ZClassTypeTrait {
			classKind = "trait"
		}
		switch lowerName {
		case "self", "parent", "static",
			"int", "float", "bool", "string", "void", "null", "false", "true", "mixed", "never",
			"array", "callable", "object", "iterable":
			// Class/interface declaration: use article "a"/"an" and no comma
			article := "a"
			if classKind == "interface" {
				article = "an"
			}
			return &phpv.PhpError{
				Err:  fmt.Errorf("Cannot use \"%s\" as %s %s name as it is reserved", i.Data, article, classKind),
				Code: phpv.E_COMPILE_ERROR,
				Loc:  i.Loc(),
			}
		}
		class.Name = className
		// PHP 8.4+: Using "_" as a class/interface/trait name is deprecated
		if i.Data == "_" {
			article := "a"
			if classKind == "interface" {
				article = "an"
			}
			c.Deprecated("Using \"_\" as %s %s name is deprecated since 8.4", article, classKind)
		}
		// Track short class name for use-statement conflict detection
		if root := getRootCtx(c); root != nil && root.nsClassNames != nil {
			shortName := phpv.ZString(i.Data) // the unqualified name
			root.nsClassNames[shortName] = true
		}
		i, err = c.NextItem()
	} else if class.Name == "" && (i.IsSingle('{') || i.Type == tokenizer.T_EXTENDS || i.Type == tokenizer.T_IMPLEMENTS) {
		// Anonymous class - no name, proceed directly to extends/implements/body
	} else {
		return i.Unexpected()
	}

	if err != nil {
		return err
	}
	if err != nil {
		return err
	}

	if i.Type == tokenizer.T_EXTENDS {
		// Traits cannot extend classes
		if class.Type == phpv.ZClassTypeTrait {
			return &phpv.PhpError{
				Err:  fmt.Errorf("syntax error, unexpected token \"extends\", expecting \"{\""),
				Code: phpv.E_PARSE,
				Loc:  i.Loc(),
			}
		}
		// For interfaces, extends can have multiple comma-separated parents
		class.ExtendsStr, err = compileReadClassIdentifier(c)
		if err != nil {
			return err
		}
		// Validate that self/parent/static aren't used as parent class names
		switch class.ExtendsStr.ToLower() {
		case "self", "parent", "static":
			noun := "class"
			if class.Type == phpv.ZClassTypeInterface {
				noun = "interface"
			}
			return &phpv.PhpError{
				Err:  fmt.Errorf("Cannot use \"%s\" as %s name, as it is reserved", class.ExtendsStr, noun),
				Code: phpv.E_COMPILE_ERROR,
				Loc:  i.Loc(),
			}
		}

		i, err = c.NextItem()
		if err != nil {
			return err
		}

		// Check for comma-separated extends (interfaces only)
		if i.IsSingle(',') && class.Type == phpv.ZClassTypeInterface {
			// first extends goes to ImplementsStr for interfaces
			class.ImplementsStr = append(class.ImplementsStr, class.ExtendsStr)
			for {
				impl, err := compileReadClassIdentifier(c)
				if err != nil {
					return err
				}
				class.ImplementsStr = append(class.ImplementsStr, impl)
				i, err = c.NextItem()
				if err != nil {
					return err
				}
				if !i.IsSingle(',') {
					break
				}
			}
			// Move first extends back
			class.ExtendsStr = class.ImplementsStr[0]
			class.ImplementsStr = class.ImplementsStr[1:]
		}
	}
	if i.Type == tokenizer.T_IMPLEMENTS {
		// Traits cannot implement interfaces
		if class.Type == phpv.ZClassTypeTrait {
			return &phpv.PhpError{
				Err:  fmt.Errorf("syntax error, unexpected token \"implements\", expecting \"{\""),
				Code: phpv.E_PARSE,
				Loc:  i.Loc(),
			}
		}
		// can implement many classes
		for {
			impl, err := compileReadClassIdentifier(c)
			if err != nil {
				return err
			}

			if impl != "" {
				// Check reserved names in implements
				switch impl.ToLower() {
				case "self", "parent", "static":
					return &phpv.PhpError{
						Err:  fmt.Errorf("Cannot use \"%s\" as interface name, as it is reserved", impl),
						Code: phpv.E_COMPILE_ERROR,
						Loc:  i.Loc(),
					}
				}
				class.ImplementsStr = append(class.ImplementsStr, impl)
			}

			// read next
			i, err = c.NextItem()
			if err != nil {
				return err
			}

			if i.IsSingle(',') {
				// there's more
				continue
			}
			break
		}
	}

	c.backup()

	return nil
}

func compileReadClassIdentifier(c compileCtx) (phpv.ZString, error) {
	var res phpv.ZString
	fullyQualified := false

	for {
		i, err := c.NextItem()
		if err != nil {
			return res, err
		}

		// T_NS_SEPARATOR
		if i.Type == tokenizer.T_NS_SEPARATOR {
			if res == "" {
				fullyQualified = true
			} else {
				res += "\\"
			}
			i, err := c.NextItem()
			if err != nil {
				return res, err
			}
			if i.Type != tokenizer.T_STRING && !i.IsSemiReserved() {
				return res, i.Unexpected()
			}
			res += phpv.ZString(i.Data)
			continue
		}
		if i.Type == tokenizer.T_STRING || (i.IsSemiReserved() && i.Type != tokenizer.T_EXTENDS && i.Type != tokenizer.T_IMPLEMENTS && i.Type != tokenizer.T_READONLY) {
			res += phpv.ZString(i.Data)
			continue
		}

		c.backup()
		if fullyQualified {
			return c.resolveClassName("\\" + res), nil
		}
		return c.resolveClassName(res), nil
	}
}


// classHasOverrideProperty returns true if any property in the class has #[\Override].
func classHasOverrideProperty(class *phpobj.ZClass) bool {
	for _, p := range class.Props {
		if propHasOverride(p) {
			return true
		}
	}
	return false
}

// propHasOverride checks if a property has the #[\Override] attribute.
func propHasOverride(p *phpv.ZClassProp) bool {
	for _, attr := range p.Attributes {
		if attr.ClassName == "Override" || attr.ClassName == "\\Override" {
			return true
		}
	}
	return false
}

// validatePropertyOverride checks that every property with #[\Override] has a matching
// non-private parent property. Properties don't satisfy Override via interfaces.
func validatePropertyOverride(ctx phpv.Context, class *phpobj.ZClass) error {
	if class.Type == phpv.ZClassTypeTrait {
		return nil
	}

	for _, p := range class.Props {
		if !propHasOverride(p) {
			continue
		}

		propName := p.VarName
		found := false

		// Check parent class for a matching non-private property
		if class.Extends != nil {
			if parentProp, ok := class.Extends.GetProp(propName); ok {
				if !parentProp.Modifiers.IsPrivate() {
					found = true
				}
			}
		}

		// Check implemented interfaces for a matching property
		if !found {
			for _, intf := range class.Implementations {
				if _, ok := intf.GetProp(propName); ok {
					found = true
					break
				}
			}
		}

		if !found {
			displayName := string(class.GetName())
			phpErr := &phpv.PhpError{
				Err:  fmt.Errorf("%s::$%s has #[\\Override] attribute, but no matching parent property exists", displayName, propName),
				Code: phpv.E_ERROR,
				Loc:  class.L,
			}
			ctx.Global().LogError(phpErr)
			return phpv.ExitError(255)
		}
	}

	return nil
}

// validateMagicMethodReturnType checks that magic methods declare valid return types.
// PHP enforces specific return type constraints on magic methods at compile time.
func validateMagicMethodReturnType(class *phpobj.ZClass, method *phpv.ZClassMethod, l *phpv.Loc) error {
	zc, ok := method.Method.(*ZClosure)
	if !ok {
		return nil
	}
	rt := zc.GetReturnType()
	name := method.Name.ToLower()

	switch name {
	case "__construct", "__destruct":
		// Cannot declare any return type
		if rt != nil {
			return &phpv.PhpError{
				Err:  fmt.Errorf("Method %s::%s() cannot declare a return type", class.Name, method.Name),
				Code: phpv.E_COMPILE_ERROR,
				Loc:  l,
			}
		}
	case "__clone", "__set", "__unset", "__unserialize", "__wakeup":
		// Return type must be void when declared
		if rt != nil && rt.Type() != phpv.ZtVoid {
			return &phpv.PhpError{
				Err:  fmt.Errorf("%s::%s(): Return type must be void when declared", class.Name, method.Name),
				Code: phpv.E_COMPILE_ERROR,
				Loc:  l,
			}
		}
	case "__isset":
		// Return type must be bool when declared
		if rt != nil {
			if rt.Type() != phpv.ZtBool || rt.ClassName() == "false" || rt.ClassName() == "true" || len(rt.Union) > 0 {
				return &phpv.PhpError{
					Err:  fmt.Errorf("%s::%s(): Return type must be bool when declared", class.Name, method.Name),
					Code: phpv.E_COMPILE_ERROR,
					Loc:  l,
				}
			}
		}
	case "__tostring":
		// Return type must be string when declared (never is allowed as covariant)
		if rt != nil && rt.Type() != phpv.ZtString && rt.Type() != phpv.ZtNever {
			return &phpv.PhpError{
				Err:  fmt.Errorf("%s::%s(): Return type must be string when declared", class.Name, method.Name),
				Code: phpv.E_COMPILE_ERROR,
				Loc:  l,
			}
		}
	case "__debuginfo":
		// Return type must be ?array when declared (accepts array, ?array, array|null)
		if rt != nil {
			valid := false
			if rt.Type() == phpv.ZtArray {
				valid = true // array or ?array (nullable is ok)
			}
			// Check union types: all members must be array or null
			if len(rt.Union) > 0 {
				valid = true
				for _, alt := range rt.Union {
					if alt.Type() != phpv.ZtArray && alt.Type() != phpv.ZtNull {
						valid = false
						break
					}
				}
			}
			if !valid {
				return &phpv.PhpError{
					Err:  fmt.Errorf("%s::%s(): Return type must be ?array when declared", class.Name, method.Name),
					Code: phpv.E_COMPILE_ERROR,
					Loc:  l,
				}
			}
		}
	case "__serialize", "__sleep":
		// Return type must be array when declared
		if rt != nil && rt.Type() != phpv.ZtArray {
			return &phpv.PhpError{
				Err:  fmt.Errorf("%s::%s(): Return type must be array when declared", class.Name, method.Name),
				Code: phpv.E_COMPILE_ERROR,
				Loc:  l,
			}
		}
	case "__set_state":
		// Return type must be object when declared.
		// Allow: object, self, static, parent, class names, union/intersection of objects.
		if rt != nil && !isObjectLikeReturnType(rt) {
			return &phpv.PhpError{
				Err:  fmt.Errorf("%s::%s(): Return type must be object when declared", class.Name, method.Name),
				Code: phpv.E_COMPILE_ERROR,
				Loc:  l,
			}
		}
	}

	return nil
}

// isObjectLikeReturnType checks if a return type hint is compatible with "object".
// This includes: object, self, static, parent, class names, and union/intersection types
// where all alternatives resolve to objects.
func isObjectLikeReturnType(rt *phpv.TypeHint) bool {
	if rt == nil {
		return true
	}
	// Union types: all alternatives must be object-like
	if len(rt.Union) > 0 {
		for _, u := range rt.Union {
			if !isObjectLikeReturnType(u) {
				return false
			}
		}
		return true
	}
	// Intersection types: all must be object-like (they always are since they require classes)
	if len(rt.Intersection) > 0 {
		return true
	}
	// Object type (generic or specific class name, including self/static/parent)
	if rt.Type() == phpv.ZtObject {
		return true
	}
	return false
}

// validatePropertyDefault checks if a property's default value is compatible with its type hint.
// Only checks literal values (runZVal) at compile time.
func validatePropertyDefault(r phpv.Runnable, th *phpv.TypeHint, className phpv.ZString, varName phpv.ZString, loc *phpv.Loc) error {
	if th == nil || r == nil {
		return nil
	}

	// Validate literal values, double-quoted string constants (runConcat),
	// and null/true/false constants (runConstant)
	var val phpv.Val
	if zv, ok := r.(*runZVal); ok {
		val = zv.v
	} else if _, ok := r.(runConcat); ok {
		// Double-quoted strings compile to runConcat; treat as string type
		val = phpv.ZString("")
	} else if rc, ok := r.(*runConstant); ok {
		// Handle null/true/false constants
		switch strings.ToLower(shortName(rc.c)) {
		case "null":
			val = phpv.ZNull{}
		case "true":
			val = phpv.ZBool(true)
		case "false":
			val = phpv.ZBool(false)
		default:
			return nil // other constants can't be validated at compile time
		}
	} else {
		return nil
	}

	// Special case: null default value on non-nullable type
	isNull := val == nil || val.GetType() == phpv.ZtNull
	if isNull {
		if !th.IsNullable() {
			// Object type hint: special message
			if th.Type() == phpv.ZtObject && th.ClassName() != "" && th.ClassName() != "callable" && th.ClassName() != "iterable" {
				return &phpv.PhpError{
					Err:  fmt.Errorf("Default value for property of type %s may not be null. Use the nullable type ?%s to allow null default value", th.String(), th.String()),
					Code: phpv.E_COMPILE_ERROR,
					Loc:  loc,
				}
			}
			// For scalar types
			return &phpv.PhpError{
				Err:  fmt.Errorf("Default value for property of type %s may not be null. Use the nullable type ?%s to allow null default value", th.String(), th.String()),
				Code: phpv.E_COMPILE_ERROR,
				Loc:  loc,
			}
		}
		return nil
	}

	// Skip complex types (unions, intersections) for now - too complex for compile-time validation
	if len(th.Union) > 0 || len(th.Intersection) > 0 {
		return nil
	}

	// Check if the default literal type matches the property type
	valType := val.GetType()
	hintType := th.Type()

	// For object type hints (class names), only null can be a default (checked above)
	if hintType == phpv.ZtObject {
		return nil
	}

	// mixed accepts anything
	if hintType == phpv.ZtMixed {
		return nil
	}

	// Type compatibility checks
	compatible := false
	switch hintType {
	case phpv.ZtInt:
		compatible = valType == phpv.ZtInt
	case phpv.ZtFloat:
		// float accepts int and float
		compatible = valType == phpv.ZtFloat || valType == phpv.ZtInt
	case phpv.ZtString:
		compatible = valType == phpv.ZtString
	case phpv.ZtBool:
		compatible = valType == phpv.ZtBool
	case phpv.ZtArray:
		compatible = valType == phpv.ZtArray
	case phpv.ZtNull:
		compatible = true // null type only accepts null, already checked above
	default:
		compatible = true // unknown types, let it pass
	}

	if !compatible {
		return &phpv.PhpError{
			Err:  fmt.Errorf("Cannot use %s as default value for property %s::$%s of type %s", valType.TypeName(), className, varName, th.String()),
			Code: phpv.E_COMPILE_ERROR,
			Loc:  loc,
		}
	}

	return nil
}

// validatePropertyTypeHint checks that a type hint is valid for a property declaration.
// PHP disallows callable, void, and never as property types.
func validatePropertyTypeHint(th *phpv.TypeHint, className phpv.ZString, varName phpv.ZString, loc *phpv.Loc) error {
	if th == nil {
		return nil
	}
	// Check union members
	if len(th.Union) > 0 {
		for _, u := range th.Union {
			if err := validatePropertyTypeHint(u, className, varName, loc); err != nil {
				return err
			}
		}
		return nil
	}
	// Check intersection members
	if len(th.Intersection) > 0 {
		for _, part := range th.Intersection {
			if err := validatePropertyTypeHint(part, className, varName, loc); err != nil {
				return err
			}
		}
		return nil
	}

	if th.Type() == phpv.ZtObject && th.ClassName() == "callable" {
		typeName := "callable"
		if th.Nullable {
			typeName = "?callable"
		}
		return &phpv.PhpError{
			Err:  fmt.Errorf("Property %s::$%s cannot have type %s", className, varName, typeName),
			Code: phpv.E_COMPILE_ERROR,
			Loc:  loc,
		}
	}
	if th.Type() == phpv.ZtVoid {
		return &phpv.PhpError{
			Err:  fmt.Errorf("Property %s::$%s cannot have type void", className, varName),
			Code: phpv.E_COMPILE_ERROR,
			Loc:  loc,
		}
	}
	if th.Type() == phpv.ZtNever {
		return &phpv.PhpError{
			Err:  fmt.Errorf("Property %s::$%s cannot have type never", className, varName),
			Code: phpv.E_COMPILE_ERROR,
			Loc:  loc,
		}
	}
	return nil
}

// validateClassConstTypeHint checks that a type hint is valid for a class constant.
// PHP disallows callable, void, and never as class constant types.
func validateClassConstTypeHint(th *phpv.TypeHint, className phpv.ZString, constName string, loc *phpv.Loc) error {
	if th == nil {
		return nil
	}
	// Check union members
	if len(th.Union) > 0 {
		for _, u := range th.Union {
			if err := validateClassConstTypeHint(u, className, constName, loc); err != nil {
				return err
			}
		}
		return nil
	}
	// Check intersection members
	if len(th.Intersection) > 0 {
		for _, part := range th.Intersection {
			if err := validateClassConstTypeHint(part, className, constName, loc); err != nil {
				return err
			}
		}
		return nil
	}

	if th.Type() == phpv.ZtObject && th.ClassName() == "callable" {
		return &phpv.PhpError{
			Err:  fmt.Errorf("Class constant %s::%s cannot have type callable", className, constName),
			Code: phpv.E_COMPILE_ERROR,
			Loc:  loc,
		}
	}
	if th.Type() == phpv.ZtVoid {
		return &phpv.PhpError{
			Err:  fmt.Errorf("Class constant %s::%s cannot have type void", className, constName),
			Code: phpv.E_COMPILE_ERROR,
			Loc:  loc,
		}
	}
	if th.Type() == phpv.ZtNever {
		return &phpv.PhpError{
			Err:  fmt.Errorf("Class constant %s::%s cannot have type never", className, constName),
			Code: phpv.E_COMPILE_ERROR,
			Loc:  loc,
		}
	}
	return nil
}
