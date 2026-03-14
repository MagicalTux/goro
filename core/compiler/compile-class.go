package compiler

import (
	"fmt"

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

// containsRuntimeOps checks if a compiled expression contains runtime
// operations (variables, function calls) that are not allowed in class constants.
func containsRuntimeOps(r phpv.Runnable) bool {
	switch v := r.(type) {
	case *runVariable, *runVariableRef:
		return true
	case runConcat:
		for _, sub := range v {
			if containsRuntimeOps(sub) {
				return true
			}
		}
	}
	return false
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

	// If called from a modifier token (abstract/final), back it up so
	// parseZClassAttr can consume it, then read the actual class token.
	if i.Type == tokenizer.T_ABSTRACT || i.Type == tokenizer.T_FINAL || i.Type == tokenizer.T_READONLY {
		c.backup()
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

	c = &zclassCompileCtx{c, class}

	// Set the compiling class so that deprecation messages can include the class name
	c.Global().SetCompilingClass(class)
	defer c.Global().SetCompilingClass(nil)

	err = parseClassLine(class, c)
	if err != nil {
		return nil, err
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
		if i.Type == tokenizer.T_STRING || i.Type == tokenizer.T_ARRAY || i.Type == tokenizer.T_CALLABLE || i.IsSingle('?') || i.Type == tokenizer.T_STATIC {
			// Could be a type hint for a property, or a regular class name
			// Peek ahead to check if a T_VARIABLE follows (possibly after namespace parts)
			isNullable := i.IsSingle('?')
			hint := i.Data
			if isNullable {
				i, err = c.NextItem()
				if err != nil {
					return nil, err
				}
				hint = i.Data
			}
			if i.Type == tokenizer.T_STRING || i.Type == tokenizer.T_ARRAY || i.Type == tokenizer.T_CALLABLE || i.Type == tokenizer.T_STATIC {
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

				if i.IsSingle('|') || i.IsSingle('&') {
					// Union (int|string) or intersection (A&B) type hint
					propTypeHint = phpv.ParseTypeHint(phpv.ZString(hint))
					if isNullable {
						propTypeHint.Nullable = true
					}
					propTypeHint, i, err = parseUnionTypeHint(propTypeHint, c)
					if err != nil {
						return nil, err
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

		switch i.Type {
		case tokenizer.T_VAR:
			// class variable, with possible default value
			i, err = c.NextItem()
			if err != nil {
				return nil, err
			}
			if i.Type != tokenizer.T_VARIABLE {
				return nil, i.Unexpected()
			}
			fallthrough
		case tokenizer.T_VARIABLE:
			for {
				prop := &phpv.ZClassProp{Modifiers: attr, SetModifiers: setModifiers, TypeHint: propTypeHint, Attributes: memberAttrs}
				prop.VarName = phpv.ZString(i.Data[1:])

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
					// parse default value for class variable
					prop.Default = &phpv.CompileDelayed{V: r}

					i, err = c.NextItem()
					if err != nil {
						return nil, err
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
				}

				// Property hooks: $prop { get { } set { } }
				if i.IsSingle('{') {
					if err := compilePropertyHooks(prop, class, c); err != nil {
						return nil, err
					}
					class.Props = append(class.Props, prop)
					break
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
			// const K = V [, K2 = V2 ...];
			for {
				// get const name
				i, err = c.NextItem()
				if err != nil {
					return nil, err
				}
				if i.Type != tokenizer.T_STRING {
					return nil, i.Unexpected()
				}
				constName := i.Data

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

				// Check for invalid operations in constant expressions
				if containsRuntimeOps(v) {
					return nil, &phpv.PhpError{
						Err:  fmt.Errorf("Constant expression contains invalid operations"),
						Code: phpv.E_COMPILE_ERROR,
						Loc:  i.Loc(),
					}
				}

				class.Const[phpv.ZString(constName)] = &phpv.ZClassConst{
					Value:     &phpv.CompileDelayed{V: v},
					Modifiers: attr,
				}

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
		case tokenizer.T_USE:
			// Trait usage: use TraitName [, TraitName2] [{ ... }];
			var traitNames []phpv.ZString
			for {
				i, err = c.NextItem()
				if err != nil {
					return nil, err
				}
				if i.Type != tokenizer.T_STRING && i.Type != tokenizer.T_NS_SEPARATOR {
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
							// as [visibility] newname
							i, err = c.NextItem()
							if err != nil {
								return nil, err
							}
							var newAttr phpv.ZObjectAttr
							newName := ""
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
							for {
								i, err = c.NextItem()
								if err != nil {
									return nil, err
								}
								// Skip trait names
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
						} else {
							return nil, i.Unexpected()
						}
					} else if i.Type == tokenizer.T_AS {
						// method as [visibility] newname;
						i, err = c.NextItem()
						if err != nil {
							return nil, err
						}
						var newAttr phpv.ZObjectAttr
						newName := ""
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
			})
		case tokenizer.T_FUNCTION:
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

			if i.Type != tokenizer.T_STRING {
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

			var f phpv.Callable

			optionalBody := class.Type == phpv.ZClassTypeInterface || attr&phpv.ZAttrAbstract != 0
			f, err = compileFunctionWithName(phpv.ZString(i.Data), c, l, rref, optionalBody)
			if err != nil {
				return nil, err
			}
			f.(*ZClosure).class = class

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

			if x := method.Name.ToLower(); x == class.BaseName().ToLower() || x == "__construct" {
				//if class.Constructor != nil {
				class.Handlers().Constructor = method

				// Handle constructor property promotion (PHP 8.0+)
				if fga, ok := f.(phpv.FuncGetArgs); ok {
					for _, arg := range fga.GetArgs() {
						if arg.Promotion != 0 {
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
								if setAccess == phpv.ZAttrPublic {
									return nil, &phpv.PhpError{
										Err:  fmt.Errorf("Visibility of property %s::$%s must not be weaker than set visibility", class.Name, arg.VarName),
										Code: phpv.E_COMPILE_ERROR,
										Loc:  l,
									}
								}
								readAccess := modifiers & phpv.ZAttrAccess
								if readAccess == phpv.ZAttrPrivate && setAccess != phpv.ZAttrPrivate {
									return nil, &phpv.PhpError{
										Err:  fmt.Errorf("Visibility of property %s::$%s must not be weaker than set visibility", class.Name, arg.VarName),
										Code: phpv.E_COMPILE_ERROR,
										Loc:  l,
									}
								}
							}
							prop := &phpv.ZClassProp{
								VarName:      arg.VarName,
								Modifiers:    modifiers,
								SetModifiers: arg.SetPromotion,
								TypeHint:     arg.Hint,
							}
							class.Props = append(class.Props, prop)
						}
					}
				}
			}
			class.Methods[method.Name.ToLower()] = method
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

	return class, nil
}

func parseClassLine(class *phpobj.ZClass, c compileCtx) error {
	i, err := c.NextItem()
	if err != nil {
		return err
	}

	if i.Type == tokenizer.T_STRING {
		className := phpv.ZString(i.Data)
		// Prepend current namespace to class name
		ns := c.getNamespace()
		if ns != "" {
			className = ns + "\\" + className
		}
		class.Name = className
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
		// For interfaces, extends can have multiple comma-separated parents
		class.ExtendsStr, err = compileReadClassIdentifier(c)
		if err != nil {
			return err
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
		// can implement many classes
		for {
			impl, err := compileReadClassIdentifier(c)
			if err != nil {
				return err
			}

			if impl != "" {
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
			if i.Type != tokenizer.T_STRING {
				return res, i.Unexpected()
			}
			res += phpv.ZString(i.Data)
			continue
		}
		if i.Type == tokenizer.T_STRING {
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

