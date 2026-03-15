package compiler

import (
	"fmt"

	"github.com/MagicalTux/goro/core/phpobj"
	"github.com/MagicalTux/goro/core/phpv"
	"github.com/MagicalTux/goro/core/tokenizer"
)

func compileEnum(i *tokenizer.Item, c compileCtx) (phpv.Runnable, error) {
	// enum Name [: string|int] [implements Interface, ...] { ... }
	class := &phpobj.ZClass{
		L:       i.Loc(),
		Attr:    phpv.ZClassAttr(phpv.ZClassFinal),
		Type:    phpv.ZClassTypeEnum,
		Methods: make(map[phpv.ZString]*phpv.ZClassMethod),
		Const:   make(map[phpv.ZString]*phpv.ZClassConst),
		H:       &phpv.ZClassHandlers{},
	}

	c = &zclassCompileCtx{c, class}
	c.Global().SetCompilingClass(class)
	defer c.Global().SetCompilingClass(nil)

	// Read enum name
	i, err := c.NextItem()
	if err != nil {
		return nil, err
	}
	if i.Type != tokenizer.T_STRING {
		return nil, i.Unexpected()
	}
	enumName := phpv.ZString(i.Data)
	// Prepend current namespace to enum name
	ns := c.getNamespace()
	if ns != "" {
		enumName = ns + "\\" + enumName
	}
	class.Name = enumName

	// Check for backing type ": string" or ": int"
	var backingType phpv.ZType
	i, err = c.NextItem()
	if err != nil {
		return nil, err
	}
	if i.IsSingle(':') {
		i, err = c.NextItem()
		if err != nil {
			return nil, err
		}
		switch {
		case i.Type == tokenizer.T_STRING && i.Data == "string":
			backingType = phpv.ZtString
		case i.Type == tokenizer.T_STRING && i.Data == "int":
			backingType = phpv.ZtInt
		default:
			return nil, &phpv.PhpError{
				Err:  fmt.Errorf("Enum backing type must be int or string, %s given", i.Data),
				Code: phpv.E_COMPILE_ERROR,
				Loc:  i.Loc(),
			}
		}
		// Check for union type (e.g., string|int) which is not allowed
		typeName := i.Data
		i, err = c.NextItem()
		if err != nil {
			return nil, err
		}
		if i.IsSingle('|') {
			// Read the other type
			i2, err := c.NextItem()
			if err != nil {
				return nil, err
			}
			return nil, &phpv.PhpError{
				Err:  fmt.Errorf("Enum backing type must be int or string, %s|%s given", typeName, i2.Data),
				Code: phpv.E_COMPILE_ERROR,
				Loc:  i.Loc(),
			}
		}
	}

	// Check for implements
	if i.Type == tokenizer.T_IMPLEMENTS {
		for {
			impl, err := compileReadClassIdentifier(c)
			if err != nil {
				return nil, err
			}
			if impl != "" {
				class.ImplementsStr = append(class.ImplementsStr, impl)
			}
			i, err = c.NextItem()
			if err != nil {
				return nil, err
			}
			if i.IsSingle(',') {
				continue
			}
			break
		}
	}

	if !i.IsSingle('{') {
		return nil, i.Unexpected()
	}

	// Track enum cases for ::cases() method
	type enumCase struct {
		name  phpv.ZString
		value phpv.Runnable // nil for unit enums
	}
	var cases []enumCase

	// Parse enum body
	for {
		i, err = c.NextItem()
		if err != nil {
			return nil, err
		}

		// Skip comments
		for i.Type == tokenizer.T_DOC_COMMENT || i.Type == tokenizer.T_COMMENT {
			i, err = c.NextItem()
			if err != nil {
				return nil, err
			}
		}

		if i.IsSingle('}') {
			break
		}

		l := i.Loc()

		switch i.Type {
		case tokenizer.T_CASE:
			// case Name [= value];
			i, err = c.NextItem()
			if err != nil {
				return nil, err
			}
			if i.Type != tokenizer.T_STRING {
				return nil, i.Unexpected()
			}
			caseName := phpv.ZString(i.Data)

			i, err = c.NextItem()
			if err != nil {
				return nil, err
			}

			var caseValue phpv.Runnable
			if i.IsSingle('=') {
				if backingType == 0 {
					return nil, &phpv.PhpError{
						Err:  fmt.Errorf("Case %s of non-backed enum %s must not have a value", caseName, class.Name),
						Code: phpv.E_COMPILE_ERROR,
						Loc:  l,
					}
				}
				caseValue, err = compileExpr(nil, c)
				if err != nil {
					return nil, err
				}
				i, err = c.NextItem()
				if err != nil {
					return nil, err
				}
			} else if backingType != 0 {
				return nil, &phpv.PhpError{
					Err:  fmt.Errorf("Case %s of backed enum %s must have a value", caseName, class.Name),
					Code: phpv.E_COMPILE_ERROR,
					Loc:  l,
				}
			}

			if !i.IsSingle(';') {
				return nil, i.Unexpected()
			}

			cases = append(cases, enumCase{name: caseName, value: caseValue})

			// Store as class constant
			class.Const[caseName] = &phpv.ZClassConst{
				Value:     &phpv.CompileDelayed{V: &runEnumCaseInit{className: class.Name, caseName: caseName, backingValue: caseValue, backingType: backingType}},
				Modifiers: phpv.ZAttrPublic,
			}

		case tokenizer.T_CONST:
			// Regular constant in enum
			for {
				i, err = c.NextItem()
				if err != nil {
					return nil, err
				}
				if i.Type != tokenizer.T_STRING {
					return nil, i.Unexpected()
				}
				constName := i.Data

				i, err = c.NextItem()
				if err != nil {
					return nil, err
				}
				if !i.IsSingle('=') {
					return nil, i.Unexpected()
				}

				v, err := compileExpr(nil, c)
				if err != nil {
					return nil, err
				}

				class.Const[phpv.ZString(constName)] = &phpv.ZClassConst{
					Value:     &phpv.CompileDelayed{V: v},
					Modifiers: phpv.ZAttrPublic,
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
			// Trait usage in enum
			for {
				i, err = c.NextItem()
				if err != nil {
					return nil, err
				}
				if i.Type != tokenizer.T_STRING && i.Type != tokenizer.T_NS_SEPARATOR {
					return nil, i.Unexpected()
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
				class.TraitUses = append(class.TraitUses, phpv.ZClassTraitUse{
					TraitNames: []phpv.ZString{phpv.ZString(name)},
				})

				i, err = c.NextItem()
				if err != nil {
					return nil, err
				}
				if i.IsSingle(',') {
					continue
				}
				break
			}
			if i.IsSingle('{') {
				// Skip trait adaptation block for now
				depth := 1
				for depth > 0 {
					i, err = c.NextItem()
					if err != nil {
						return nil, err
					}
					if i.IsSingle('{') {
						depth++
					} else if i.IsSingle('}') {
						depth--
					}
				}
			} else if !i.IsSingle(';') {
				return nil, i.Unexpected()
			}

		default:
			// Methods (possibly with modifiers and #[...] attributes)
			c.backup()

			var attr phpv.ZObjectAttr
			var memberAttrs []*phpv.ZAttribute
			if err := parseZObjectAttrWithAttrs(&attr, &memberAttrs, c); err != nil {
				return nil, &phpv.PhpError{Err: err, Code: phpv.E_COMPILE_ERROR, Loc: l}
			}

			i, err = c.NextItem()
			if err != nil {
				return nil, err
			}

			if i.Type != tokenizer.T_FUNCTION {
				// If we see a variable or type hint followed by a variable, it's a property declaration
				if i.Type == tokenizer.T_VARIABLE || i.Type == tokenizer.T_STRING {
					return nil, &phpv.PhpError{
						Err:  fmt.Errorf("Enum %s cannot include properties", class.Name),
						Code: phpv.E_COMPILE_ERROR,
						Loc:  l,
					}
				}
				return nil, i.Unexpected()
			}

			i, err = c.NextItem()
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

			// Check for abstract methods on enums (enums cannot have abstract methods)
			if attr&phpv.ZAttrAbstract != 0 {
				return nil, &phpv.PhpError{
					Err:  fmt.Errorf("Enum method %s::%s() must not be abstract", class.Name, i.Data),
					Code: phpv.E_COMPILE_ERROR,
					Loc:  l,
				}
			}

			// Check for forbidden magic methods on enums
			methodNameLower := phpv.ZString(i.Data).ToLower()
			if isEnumForbiddenMethod(methodNameLower) {
				return nil, &phpv.PhpError{
					Err:  fmt.Errorf("Enum %s cannot include magic method %s", class.Name, i.Data),
					Code: phpv.E_COMPILE_ERROR,
					Loc:  l,
				}
			}

			// For abstract methods, optionalBody should be true to avoid "must contain body" error
			// But since we already reject abstract above, use false
			f, err := compileFunctionWithName(phpv.ZString(i.Data), c, l, rref, false)
			if err != nil {
				return nil, err
			}
			f.(*ZClosure).class = class

			_, emptyBody := f.(*ZClosure).code.(phpv.RunNull)

			method := &phpv.ZClassMethod{
				Name:       phpv.ZString(i.Data),
				Modifiers:  attr,
				Method:     f,
				Class:      class,
				Empty:      emptyBody,
				Loc:        l,
				Attributes: memberAttrs,
			}
			class.Methods[method.Name.ToLower()] = method
		}
	}

	// Store enum metadata on the class for runtime use
	class.EnumBackingType = backingType
	for _, ec := range cases {
		class.EnumCases = append(class.EnumCases, phpv.ZString(ec.name))
	}

	// Validate user-specified implements list for enums
	for _, impl := range class.ImplementsStr {
		implLower := impl.ToLower()
		switch implLower {
		case "unitenum":
			return nil, &phpv.PhpError{
				Err:  fmt.Errorf("Enum %s cannot implement previously implemented interface UnitEnum", class.Name),
				Code: phpv.E_ERROR,
				Loc:  class.L,
			}
		case "backedenum":
			if backingType != 0 {
				return nil, &phpv.PhpError{
					Err:  fmt.Errorf("Enum %s cannot implement previously implemented interface BackedEnum", class.Name),
					Code: phpv.E_ERROR,
					Loc:  class.L,
				}
			}
			// Non-backed enum trying to implement BackedEnum
			return nil, &phpv.PhpError{
				Err:  fmt.Errorf("Non-backed enum %s cannot implement interface BackedEnum", class.Name),
				Code: phpv.E_ERROR,
				Loc:  class.L,
			}
		}
	}

	// Check for duplicate interfaces in the implements list
	seen := make(map[phpv.ZString]bool)
	for _, impl := range class.ImplementsStr {
		implLower := impl.ToLower()
		if seen[implLower] {
			return nil, &phpv.PhpError{
				Err:  fmt.Errorf("Enum %s cannot implement previously implemented interface %s", class.Name, impl),
				Code: phpv.E_ERROR,
				Loc:  class.L,
			}
		}
		seen[implLower] = true
	}

	// All enums implement UnitEnum; backed enums also implement BackedEnum
	class.ImplementsStr = append(class.ImplementsStr, "UnitEnum")
	if backingType != 0 {
		class.ImplementsStr = append(class.ImplementsStr, "BackedEnum")
	}

	// Reject user-defined built-in methods
	if _, exists := class.Methods["cases"]; exists {
		return nil, &phpv.PhpError{
			Err:  fmt.Errorf("Cannot redeclare %s::cases()", class.Name),
			Code: phpv.E_COMPILE_ERROR,
			Loc:  i.Loc(),
		}
	}
	if backingType != 0 {
		if m, exists := class.Methods["from"]; exists {
			loc := m.Loc
			if loc == nil {
				loc = i.Loc()
			}
			return nil, &phpv.PhpError{
				Err:  fmt.Errorf("Cannot redeclare %s::from()", class.Name),
				Code: phpv.E_COMPILE_ERROR,
				Loc:  loc,
			}
		}
		if m, exists := class.Methods["tryfrom"]; exists {
			loc := m.Loc
			if loc == nil {
				loc = i.Loc()
			}
			return nil, &phpv.PhpError{
				Err:  fmt.Errorf("Cannot redeclare %s::tryFrom()", class.Name),
				Code: phpv.E_COMPILE_ERROR,
				Loc:  loc,
			}
		}
	}

	// Add built-in enum methods

	// cases() - returns array of all enum cases
	if _, exists := class.Methods["cases"]; !exists {
		class.Methods["cases"] = &phpv.ZClassMethod{
			Name:      "cases",
			Modifiers: phpv.ZAttrPublic | phpv.ZAttrStatic,
			Method: phpobj.NativeStaticMethod(func(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
				cls := ctx.Class()
				if cls == nil {
					return nil, fmt.Errorf("cannot call cases() outside of enum context")
				}
				zc, ok := cls.(*phpobj.ZClass)
				if !ok {
					return nil, fmt.Errorf("cases() called on non-ZClass")
				}
				result := phpv.NewZArray()
				for _, caseName := range zc.EnumCases {
					cc, exists := zc.Const[caseName]
					if !exists {
						continue
					}
					val := cc.Value
					// Resolve CompileDelayed if needed
					if cd, ok := val.(*phpv.CompileDelayed); ok {
						z, err := cd.Run(ctx)
						if err != nil {
							return nil, err
						}
						zc.Const[caseName].Value = z.Value()
						val = z.Value()
					}
					result.OffsetSet(ctx, nil, val.ZVal())
				}
				return result.ZVal(), nil
			}),
			Class: class,
		}
	}

	// from() and tryFrom() - only for backed enums
	if backingType != 0 {
		if _, exists := class.Methods["from"]; !exists {
			enumBackingType := backingType // capture for closure
			class.Methods["from"] = &phpv.ZClassMethod{
				Name:      "from",
				Modifiers: phpv.ZAttrPublic | phpv.ZAttrStatic,
				Method: phpobj.NativeStaticMethod(func(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
					if len(args) < 1 {
						return nil, fmt.Errorf("from() expects exactly 1 argument")
					}
					cls := ctx.Class()
					if cls == nil {
						return nil, fmt.Errorf("cannot call from() outside of enum context")
					}
					zc, ok := cls.(*phpobj.ZClass)
					if !ok {
						return nil, fmt.Errorf("from() called on non-ZClass")
					}
					needle := args[0]
					// Type check: int-backed enums require int, string-backed enums accept int|string
					if enumBackingType == phpv.ZtInt {
						if needle.GetType() != phpv.ZtInt {
							actualType := needle.GetType().TypeName()
							if needle.GetType() == phpv.ZtObject {
								if obj, ok2 := needle.Value().(phpv.ZObject); ok2 {
									actualType = string(obj.GetClass().GetName())
								}
							}
							return nil, phpobj.ThrowError(ctx, phpobj.TypeError,
								fmt.Sprintf("%s::from(): Argument #1 ($value) must be of type int, %s given", zc.GetName(), actualType))
						}
					} else if enumBackingType == phpv.ZtString {
						// String-backed enums accept int and string (int is coerced to string for lookup)
						if needle.GetType() != phpv.ZtString && needle.GetType() != phpv.ZtInt {
							actualType := needle.GetType().TypeName()
							if needle.GetType() == phpv.ZtObject {
								if obj, ok2 := needle.Value().(phpv.ZObject); ok2 {
									actualType = string(obj.GetClass().GetName())
								}
							}
							return nil, phpobj.ThrowError(ctx, phpobj.TypeError,
								fmt.Sprintf("%s::from(): Argument #1 ($value) must be of type string, %s given", zc.GetName(), actualType))
						}
					}
					for _, caseName := range zc.EnumCases {
						cc, exists := zc.Const[caseName]
						if !exists {
							continue
						}
						val := cc.Value
						if cd, ok := val.(*phpv.CompileDelayed); ok {
							z, err := cd.Run(ctx)
							if err != nil {
								return nil, err
							}
							zc.Const[caseName].Value = z.Value()
							val = z.Value()
						}
						obj, ok := val.(*phpobj.ZObject)
						if !ok {
							continue
						}
						backingVal := obj.HashTable().GetString("value")
						if backingVal == nil {
							continue
						}
						// Strict comparison
						eq, err := phpv.StrictEquals(ctx, needle, backingVal)
						if err != nil {
							return nil, err
						}
						if eq {
							return val.ZVal(), nil
						}
					}
					// Throw ValueError with appropriate formatting
					// String values (including ints coerced to string) are quoted
					var valueStr string
					if enumBackingType == phpv.ZtString {
						valueStr = fmt.Sprintf("\"%s\"", needle.String())
					} else {
						valueStr = needle.String()
					}
					return nil, phpobj.ThrowError(ctx, phpobj.ValueError,
						fmt.Sprintf("%s is not a valid backing value for enum %s", valueStr, zc.GetName()))
				}),
				Class: class,
			}
		}

		if _, exists := class.Methods["tryfrom"]; !exists {
			enumBackingType := backingType // capture for closure
			class.Methods["tryfrom"] = &phpv.ZClassMethod{
				Name:      "tryFrom",
				Modifiers: phpv.ZAttrPublic | phpv.ZAttrStatic,
				Method: phpobj.NativeStaticMethod(func(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
					if len(args) < 1 {
						return nil, fmt.Errorf("tryFrom() expects exactly 1 argument")
					}
					cls := ctx.Class()
					if cls == nil {
						return nil, fmt.Errorf("cannot call tryFrom() outside of enum context")
					}
					zc, ok := cls.(*phpobj.ZClass)
					if !ok {
						return nil, fmt.Errorf("tryFrom() called on non-ZClass")
					}
					needle := args[0]
					// Type check: int-backed enums require int, string-backed enums accept int|string
					if enumBackingType == phpv.ZtInt {
						if needle.GetType() != phpv.ZtInt {
							actualType := needle.GetType().TypeName()
							if needle.GetType() == phpv.ZtObject {
								if obj, ok2 := needle.Value().(phpv.ZObject); ok2 {
									actualType = string(obj.GetClass().GetName())
								}
							}
							return nil, phpobj.ThrowError(ctx, phpobj.TypeError,
								fmt.Sprintf("%s::tryFrom(): Argument #1 ($value) must be of type int, %s given", zc.GetName(), actualType))
						}
					} else if enumBackingType == phpv.ZtString {
						if needle.GetType() != phpv.ZtString && needle.GetType() != phpv.ZtInt {
							actualType := needle.GetType().TypeName()
							if needle.GetType() == phpv.ZtObject {
								if obj, ok2 := needle.Value().(phpv.ZObject); ok2 {
									actualType = string(obj.GetClass().GetName())
								}
							}
							return nil, phpobj.ThrowError(ctx, phpobj.TypeError,
								fmt.Sprintf("%s::tryFrom(): Argument #1 ($value) must be of type string, %s given", zc.GetName(), actualType))
						}
					}
					for _, caseName := range zc.EnumCases {
						cc, exists := zc.Const[caseName]
						if !exists {
							continue
						}
						val := cc.Value
						if cd, ok := val.(*phpv.CompileDelayed); ok {
							z, err := cd.Run(ctx)
							if err != nil {
								return nil, err
							}
							zc.Const[caseName].Value = z.Value()
							val = z.Value()
						}
						obj, ok := val.(*phpobj.ZObject)
						if !ok {
							continue
						}
						backingVal := obj.HashTable().GetString("value")
						if backingVal == nil {
							continue
						}
						eq, err := phpv.StrictEquals(ctx, needle, backingVal)
						if err != nil {
							return nil, err
						}
						if eq {
							return val.ZVal(), nil
						}
					}
					return phpv.ZNULL.ZVal(), nil
				}),
				Class: class,
			}
		}
	}

	return &runEnumRegister{class: class}, nil
}

// isEnumForbiddenMethod checks if a method name is a forbidden magic method for enums.
// PHP enums cannot declare: __construct, __destruct, __clone, __get, __set, __isset,
// __unset, __toString, __debugInfo, __sleep, __wakeup, __serialize, __unserialize, __set_state.
// Allowed: __call, __callStatic, __invoke.
func isEnumForbiddenMethod(name phpv.ZString) bool {
	switch name {
	case "__construct", "__destruct", "__clone",
		"__get", "__set", "__isset", "__unset",
		"__tostring", "__debuginfo",
		"__sleep", "__wakeup", "__serialize", "__unserialize",
		"__set_state":
		return true
	}
	return false
}
