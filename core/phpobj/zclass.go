package phpobj

import (
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/MagicalTux/goro/core/phperr"
	"github.com/MagicalTux/goro/core/phpv"
)

type ZClass struct {
	Name phpv.ZString
	L    *phpv.Loc
	Type phpv.ZClassType
	Attr phpv.ZClassAttr

	// string value of extend & implement (used previous to lookup)
	ExtendsStr    phpv.ZString
	ImplementsStr []phpv.ZString

	parents         map[*ZClass]*ZClass // all parents, extends & implements
	Extends         *ZClass
	Implementations []*ZClass
	Const           map[phpv.ZString]*phpv.ZClassConst // class constants
	ConstOrder      []phpv.ZString                     // declaration order for deterministic iteration
	Props           []*phpv.ZClassProp
	TraitUses       []phpv.ZClassTraitUse
	Methods         map[phpv.ZString]*phpv.ZClassMethod
	MethodOrder     []phpv.ZString // declaration order for deterministic iteration
	StaticProps     *phpv.ZHashTable
	Attributes      []*phpv.ZAttribute // PHP 8.0 attributes

	nextIntanceID int
	constSource   map[phpv.ZString]phpv.ZString // tracks which interface provided each inherited constant

	// class specific handlers
	H *phpv.ZClassHandlers

	// InternalOnly prevents user classes from implementing/extending this class
	InternalOnly bool

	// Enum support (PHP 8.1)
	EnumBackingType phpv.ZType     // 0 for unit enums, ZtString or ZtInt for backed enums
	EnumCases       []phpv.ZString // ordered list of case names
	EnumError       error          // non-nil if enum has a catchable error (e.g. duplicate values)
}

func (c *ZClass) GetName() phpv.ZString {
	if c == nil {
		return ""
	}
	// Anonymous classes have internal names like "class@anonymous\x00path:line$0"
	// GetName() returns the display name (before the null byte)
	if idx := strings.IndexByte(string(c.Name), 0); idx >= 0 {
		return c.Name[:idx]
	}
	return c.Name
}

func (c *ZClass) Run(ctx phpv.Context) (*phpv.ZVal, error) {
	err := ctx.Global().RegisterClass(c.Name, c)
	if err != nil {
		// Check if the conflict is with an alias (class_alias) - those produce a Warning, not Fatal
		type aliasConflictErr interface {
			IsAliasConflict() bool
			RedeclareKind() string
			RedeclarePrevLoc() string
		}
		if aliasErr, ok := err.(aliasConflictErr); ok && aliasErr.IsAliasConflict() {
			// Use the name being declared (c.Name) for the display, not the alias's original name
			ctx.Warn("Cannot redeclare %s %s%s", aliasErr.RedeclareKind(), c.Name, aliasErr.RedeclarePrevLoc())
			return nil, nil
		}
		return nil, c.fatalError(ctx, err.Error())
	}
	err = c.Compile(ctx)
	if err != nil {
		// If compilation fails (e.g. parent class not found), unregister the class
		// so that class_exists() returns false, matching PHP behavior.
		ctx.Global().UnregisterClass(c.Name)
		return nil, err
	}
	return nil, nil
}

func (c *ZClass) Compile(ctx phpv.Context) error {
	// Set compiling class for self:: resolution in constant initializers
	ctx.Global().SetCompilingClass(c)
	defer ctx.Global().SetCompilingClass(nil)

	c.parents = make(map[*ZClass]*ZClass)

	if c.ExtendsStr != "" {
		// need to lookup extend
		parent, err := ctx.Global().GetClass(ctx, c.ExtendsStr, true)
		if err != nil {
			return err
		}
		if _, found := c.parents[parent.(*ZClass)]; found {
			return ctx.Errorf("class extends loop found")
		}
		c.Extends = parent.(*ZClass)
		c.parents[parent.(*ZClass)] = parent.(*ZClass)
		// Add all of parent's parents (transitive)
		if c.Extends.parents != nil {
			for k, v := range c.Extends.parents {
				c.parents[k] = v
			}
		}

		// Check if trying to extend an interface (must use implements instead)
		if c.Type != phpv.ZClassTypeInterface && c.Extends.Type == phpv.ZClassTypeInterface {
			return c.fatalError(ctx, fmt.Sprintf("Class %s cannot extend interface %s", c.Name, c.Extends.Name))
		}

		// Check if trying to extend a trait (use "use" instead)
		if c.Extends.Type == phpv.ZClassTypeTrait {
			return c.fatalError(ctx, fmt.Sprintf("Class %s cannot extend trait %s", c.Name, c.Extends.Name))
		}

		// Readonly class inheritance checks
		if c.Attr.Has(phpv.ZClassReadonly) && !c.Extends.Attr.Has(phpv.ZClassReadonly) {
			return c.fatalError(ctx, fmt.Sprintf("Readonly class %s cannot extend non-readonly class %s", c.Name, c.Extends.Name))
		}
		if !c.Attr.Has(phpv.ZClassReadonly) && c.Extends.Attr.Has(phpv.ZClassReadonly) {
			return c.fatalError(ctx, fmt.Sprintf("Non-readonly class %s cannot extend readonly class %s", c.Name, c.Extends.Name))
		}

		// Check if parent class is final or an enum (enums cannot be extended)
		if c.Extends.Type.Has(phpv.ZClassTypeEnum) {
			return c.fatalError(ctx, fmt.Sprintf("Class %s cannot extend enum %s", c.Name, c.Extends.Name))
		}
		if c.Extends.Attr.Has(phpv.ZClassFinal) {
			return c.fatalError(ctx, fmt.Sprintf("Class %s cannot extend final class %s", c.Name, c.Extends.Name))
		}

		// Emit warnings about non-public magic methods BEFORE inheritance checks,
		// because PHP emits these warnings before checking access level narrowing.
		c.warnNonPublicMagicMethods(ctx)

		// need to import methods, with validation
		for n, m := range c.Extends.Methods {
			if ours, gotit := c.Methods[n]; gotit {
				// Check final method override (private methods cannot be final for inheritance, except constructors)
				if m.Modifiers.Has(phpv.ZAttrFinal) && (!m.Modifiers.Has(phpv.ZAttrPrivate) || n == "__construct") {
					loc := ours.Loc
					if loc == nil {
						loc = c.L
					}
					return c.fatalErrorAt(ctx, fmt.Sprintf("Cannot override final method %s::%s()", c.Extends.Name, m.Name), loc)
				}

				// Check access level narrowing (skip for private parent methods)
				if !m.Modifiers.Has(phpv.ZAttrPrivate) {
					// Treat implicit public (no modifier) as public
					parentAccess := m.Modifiers.Access()
					if parentAccess == 0 || m.Modifiers.Has(phpv.ZAttrImplicitPublic) {
						parentAccess = phpv.ZAttrPublic
					}
					childAccess := ours.Modifiers.Access()
					if childAccess == 0 || ours.Modifiers.Has(phpv.ZAttrImplicitPublic) {
						childAccess = phpv.ZAttrPublic
					}
					if parentAccess == phpv.ZAttrPublic && childAccess != phpv.ZAttrPublic {
						loc := ours.Loc
						if loc == nil {
							loc = c.L
						}
						return c.fatalErrorAt(ctx, fmt.Sprintf("Access level to %s::%s() must be public (as in class %s)", c.Name, ours.Name, c.Extends.Name), loc)
					}
					if parentAccess == phpv.ZAttrProtected && childAccess == phpv.ZAttrPrivate {
						loc := ours.Loc
						if loc == nil {
							loc = c.L
						}
						return c.fatalErrorAt(ctx, fmt.Sprintf("Access level to %s::%s() must be protected (as in class %s) or weaker", c.Name, ours.Name, c.Extends.Name), loc)
					}

					// Check method signature compatibility
					// For constructors, only enforce when parent has abstract constructor
					if n == "__construct" {
						if m.Modifiers.Has(phpv.ZAttrAbstract) || m.Empty {
							if err := c.checkMethodCompatibility(ctx, ours, m); err != nil {
								return err
							}
						}
					} else {
						if err := c.checkMethodCompatibility(ctx, ours, m); err != nil {
							return err
						}
					}
				}
			} else {
				c.Methods[n] = m
			}
		}

		// Check __construct compatibility against abstract constructors in the ancestor chain
		// and interface constructor declarations inherited from parents.
		if ours, gotit := c.Methods["__construct"]; gotit && (ours.Class == nil || ours.Class == c) {
			for p := c.Extends; p != nil; p = p.Extends {
				// Check abstract constructors in ancestor classes
				if pCtor, hasCtor := p.Methods["__construct"]; hasCtor {
					if pCtor.Modifiers.Has(phpv.ZAttrAbstract) || (pCtor.Empty && pCtor.Class != nil && pCtor.Class.GetType() == phpv.ZClassTypeInterface) {
						if err := c.checkMethodCompatibility(ctx, ours, pCtor); err != nil {
							return err
						}
					}
				}
				// Check interface constructors
				for _, intf := range p.Implementations {
					if intfCtor, hasCtor := intf.Methods["__construct"]; hasCtor {
						if err := c.checkMethodCompatibility(ctx, ours, intfCtor); err != nil {
							return err
						}
					}
				}
			}
		}

		// Inherit constants from parent (skip private ones), preserving order
		for _, k := range c.Extends.ConstOrder {
			v := c.Extends.Const[k]
			if v == nil || v.Modifiers.IsPrivate() {
				continue
			}
			if childConst, exists := c.Const[k]; exists {
				_ = childConst // used below
				// Cannot override final constants
				if v.Modifiers.Has(phpv.ZAttrFinal) {
					return c.fatalError(ctx, fmt.Sprintf("%s::%s cannot override final constant %s::%s", c.Name, k, c.Extends.Name, k))
				}
				// Validate constant visibility is not narrowed
				parentVis := visibilityLevel(v.Modifiers)
				childVis := visibilityLevel(childConst.Modifiers)
				if childVis > parentVis {
					visName := "public"
					weaker := ""
					if v.Modifiers.IsProtected() {
						visName = "protected"
						weaker = " or weaker"
					}
					return c.fatalError(ctx, fmt.Sprintf("Access level to %s::%s must be %s (as in class %s)%s", c.Name, k, visName, c.Extends.Name, weaker))
				}
			} else {
				c.Const[k] = v
				c.ConstOrder = append(c.ConstOrder, k)
				// Track that this constant came from the parent class for ambiguity detection
				if c.constSource == nil {
					c.constSource = make(map[phpv.ZString]phpv.ZString)
				}
				// If parent also tracked a source (inherited from interface), use that
				if c.Extends.constSource != nil {
					if src, ok := c.Extends.constSource[k]; ok {
						c.constSource[k] = src
					} else {
						c.constSource[k] = c.Extends.Name
					}
				} else {
					c.constSource[k] = c.Extends.Name
				}
			}
		}

		// Validate property overrides
		for _, childProp := range c.Props {
			parentProp, found := c.Extends.GetProp(childProp.VarName)
			if !found {
				continue
			}

			// Private parent properties can be freely redeclared
			if parentProp.Modifiers.IsPrivate() {
				continue
			}

			// Check static/non-static mismatch
			parentStatic := parentProp.Modifiers.IsStatic()
			childStatic := childProp.Modifiers.IsStatic()
			if parentStatic && !childStatic {
				return c.fatalError(ctx, fmt.Sprintf("Cannot redeclare static %s::$%s as non static %s::$%s", c.Extends.Name, childProp.VarName, c.Name, childProp.VarName))
			}
			if !parentStatic && childStatic {
				return c.fatalError(ctx, fmt.Sprintf("Cannot redeclare non static %s::$%s as static %s::$%s", c.Extends.Name, childProp.VarName, c.Name, childProp.VarName))
			}

			// Check access level narrowing
			parentAccess := parentProp.Modifiers.Access()
			if parentAccess == 0 {
				parentAccess = phpv.ZAttrPublic
			}
			childAccess := childProp.Modifiers.Access()
			if childAccess == 0 {
				childAccess = phpv.ZAttrPublic
			}
			if parentAccess == phpv.ZAttrPublic && childAccess != phpv.ZAttrPublic {
				return c.fatalError(ctx, fmt.Sprintf("Access level to %s::$%s must be public (as in class %s)", c.Name, childProp.VarName, c.Extends.Name))
			}
			if parentAccess == phpv.ZAttrProtected && childAccess == phpv.ZAttrPrivate {
				return c.fatalError(ctx, fmt.Sprintf("Access level to %s::$%s must be protected (as in class %s) or weaker", c.Name, childProp.VarName, c.Extends.Name))
			}

			// Check readonly mismatch
			parentReadonly := parentProp.Modifiers.IsReadonly()
			childReadonly := childProp.Modifiers.IsReadonly()
			if parentReadonly && !childReadonly {
				return c.fatalError(ctx, fmt.Sprintf("Cannot redeclare readonly property %s::$%s as non-readonly %s::$%s", c.Extends.Name, childProp.VarName, c.Name, childProp.VarName))
			}
			if !parentReadonly && childReadonly {
				return c.fatalError(ctx, fmt.Sprintf("Cannot redeclare non-readonly property %s::$%s as readonly %s::$%s", c.Extends.Name, childProp.VarName, c.Name, childProp.VarName))
			}

			// Cannot override a final property (explicit or implicit via private(set))
			parentSetAccess := parentProp.SetModifiers & phpv.ZAttrAccess
			if parentProp.Modifiers.Has(phpv.ZAttrFinal) || parentSetAccess == phpv.ZAttrPrivate {
				return c.fatalError(ctx, fmt.Sprintf("Cannot override final property %s::$%s", c.Extends.Name, childProp.VarName))
			}

			// Check asymmetric set visibility override compatibility
			childSetAccess := childProp.SetModifiers & phpv.ZAttrAccess

			if parentSetAccess != 0 || childSetAccess != 0 {
				// Parent has no explicit set modifier — child cannot add one
				// (adding a set restriction narrows access)
				if parentSetAccess == 0 && childSetAccess != 0 {
					return c.fatalError(ctx, fmt.Sprintf("Set access level of %s::$%s must be omitted (as in class %s)", c.Name, childProp.VarName, c.Extends.Name))
				}
				// Parent has set modifier, child doesn't — OK (widening)
				// Both have set modifiers — child must not be narrower
				if parentSetAccess != 0 && childSetAccess != 0 {
					parentSetLevel := visibilityLevel(parentSetAccess)
					childSetLevel := visibilityLevel(childSetAccess)
					if childSetLevel > parentSetLevel {
						setName := "protected(set)"
						if parentSetAccess == phpv.ZAttrPublic {
							setName = "omitted"
						}
						return c.fatalError(ctx, fmt.Sprintf("Set access level of %s::$%s must be %s (as in class %s) or weaker", c.Name, childProp.VarName, setName, c.Extends.Name))
					}
				}
			}
		}
	}
	// Interfaces cannot use traits
	if c.Type == phpv.ZClassTypeInterface && len(c.TraitUses) > 0 {
		traitName := c.TraitUses[0].TraitNames[0]
		return c.fatalError(ctx, fmt.Sprintf("Cannot use traits inside of interfaces. %s is used in %s", traitName, c.Name))
	}

	// Resolve trait uses: import methods and properties from traits
	// Track which trait provided each property (across all use statements) for conflict reporting
	type propSourceInfo struct {
		traitName phpv.ZString
	}
	propSources := make(map[phpv.ZString]*propSourceInfo)
	// Track which trait provided each constant (across all use statements) for conflict reporting
	constTraitSources := make(map[phpv.ZString]phpv.ZString)
	// Track which methods have been excluded (for duplicate exclusion detection)
	globalExcluded := make(map[phpv.ZString]map[phpv.ZString]bool) // method -> set of excluded trait names

	for _, tu := range c.TraitUses {
		// Build insteadof exclusion map
		type excludeKey struct {
			method    phpv.ZString
			traitName phpv.ZString
		}
		excluded := make(map[excludeKey]bool)

		// Build map of resolved trait names for validation
		resolvedTraitNames := make(map[phpv.ZString]bool)
		for _, tn := range tu.TraitNames {
			resolvedTraitNames[tn.ToLower()] = true
		}

		for _, io := range tu.Insteadof {
			methodLower := io.MethodName.ToLower()
			// Check reserved names in insteadof references
			switch io.TraitName.ToLower() {
			case "self", "parent", "static":
				return c.fatalError(ctx, fmt.Sprintf("Cannot use \"%s\" as trait name, as it is reserved", io.TraitName))
			}
			// Check that the insteadof source trait is in the use list
			if !resolvedTraitNames[io.TraitName.ToLower()] {
				// Try to resolve the class to give a better error message
				otherClass, lookupErr := ctx.Global().GetClass(ctx, io.TraitName, false)
				if lookupErr != nil || otherClass == nil {
					return c.fatalError(ctx, fmt.Sprintf("Could not find trait %s", io.TraitName))
				}
				oc := otherClass.(*ZClass)
				if oc.Type != phpv.ZClassTypeTrait {
					return c.fatalError(ctx, fmt.Sprintf("Class %s is not a trait, Only traits may be used in 'as' and 'insteadof' statements", io.TraitName))
				}
				return c.fatalError(ctx, fmt.Sprintf("Required Trait %s wasn't added to %s", io.TraitName, c.Name))
			}
			for _, excTrait := range io.InsteadOf {
				// Check reserved names
				switch excTrait.ToLower() {
				case "self", "parent", "static":
					return c.fatalError(ctx, fmt.Sprintf("Cannot use \"%s\" as trait name, as it is reserved", excTrait))
				}
				// Validate: the excluded trait must be in the use list
				if !resolvedTraitNames[excTrait.ToLower()] {
					otherClass, lookupErr := ctx.Global().GetClass(ctx, excTrait, false)
					if lookupErr != nil || otherClass == nil {
						return c.fatalError(ctx, fmt.Sprintf("Could not find trait %s", excTrait))
					}
					oc := otherClass.(*ZClass)
					if oc.Type != phpv.ZClassTypeTrait {
						return c.fatalError(ctx, fmt.Sprintf("Class %s is not a trait, Only traits may be used in 'as' and 'insteadof' statements", excTrait))
					}
					return c.fatalError(ctx, fmt.Sprintf("Required Trait %s wasn't added to %s", excTrait, c.Name))
				}
				// Validate: cannot exclude the same trait that provides the method (inconsistent)
				if excTrait.ToLower() == io.TraitName.ToLower() {
					return c.fatalError(ctx, fmt.Sprintf("Inconsistent insteadof definition. The method %s is to be used from %s, but %s is also on the exclude list", io.MethodName, io.TraitName, io.TraitName))
				}
				// Check for duplicate exclusions across all use statements
				if globalExcluded[methodLower] == nil {
					globalExcluded[methodLower] = make(map[phpv.ZString]bool)
				}
				if globalExcluded[methodLower][excTrait.ToLower()] {
					return c.fatalError(ctx, fmt.Sprintf("Failed to evaluate a trait precedence (%s). Method of trait %s was defined to be excluded multiple times", io.MethodName, excTrait))
				}
				globalExcluded[methodLower][excTrait.ToLower()] = true
				excluded[excludeKey{methodLower, excTrait.ToLower()}] = true
			}
		}

		// Track which trait provided each method for conflict detection
		type methodSource struct {
			traitName phpv.ZString
			method    *phpv.ZClassMethod
		}
		traitMethods := make(map[phpv.ZString]*methodSource)

		var resolvedTraits []*ZClass
		for _, traitName := range tu.TraitNames {
			// Check reserved names
			switch traitName.ToLower() {
			case "self", "parent", "static":
				return c.fatalError(ctx, fmt.Sprintf("Cannot use \"%s\" as trait name, as it is reserved", traitName))
			}
			traitClass, err := ctx.Global().GetClass(ctx, traitName, true)
			if err != nil {
				return ThrowError(ctx, Error, fmt.Sprintf("Trait \"%s\" not found", traitName))
			}
			tc := traitClass.(*ZClass)
			if tc.Type != phpv.ZClassTypeTrait {
				return ThrowError(ctx, Error, fmt.Sprintf("%s cannot use %s - it is not a trait", c.Name, tc.Name))
			}
			resolvedTraits = append(resolvedTraits, tc)
		}

		// Validate insteadof references: check that the method actually exists in the source trait
		for _, io := range tu.Insteadof {
			srcLower := io.TraitName.ToLower()
			methodLower := io.MethodName.ToLower()
			found := false
			for _, tc := range resolvedTraits {
				if tc.Name.ToLower() == srcLower {
					if _, ok := tc.Methods[methodLower]; ok {
						found = true
					}
					break
				}
			}
			if !found {
				return c.fatalError(ctx, fmt.Sprintf("A precedence rule was defined for %s::%s but this method does not exist", io.TraitName, io.MethodName))
			}
		}

		for _, tc := range resolvedTraits {
			for name, m := range tc.Methods {
				if excluded[excludeKey{name, tc.Name.ToLower()}] {
					continue
				}
				if src, exists := traitMethods[name]; exists {
					// Both abstract: OK, skip
					if m.Empty && m.Modifiers.Has(phpv.ZAttrAbstract) && src.method.Empty && src.method.Modifiers.Has(phpv.ZAttrAbstract) {
						continue
					}
					// One abstract, one concrete: the concrete one wins (no conflict)
					if m.Empty && m.Modifiers.Has(phpv.ZAttrAbstract) && !src.method.Empty {
						// Current method is abstract, existing is concrete - existing wins, skip
						continue
					}
					if !m.Empty && src.method.Empty && src.method.Modifiers.Has(phpv.ZAttrAbstract) {
						// Current method is concrete, existing is abstract - replace
						traitMethods[name] = &methodSource{traitName: tc.Name, method: m}
						methodCopy := &phpv.ZClassMethod{
							Name:       m.Name,
							Modifiers:  m.Modifiers,
							Method:     m.Method,
							Class:      c,
							Empty:      m.Empty,
							Loc:        m.Loc,
							Attributes: m.Attributes,
							FromTrait:  tc,
						}
						c.Methods[name] = methodCopy
						if name == "__construct" {
							c.Handlers().Constructor = methodCopy
						}
						continue
					}
					// Both concrete from different traits but same underlying method body
					// (e.g., both inherited from the same base trait through diamond): no conflict
					if src.method.Method == m.Method {
						continue
					}
					return c.fatalError(ctx, fmt.Sprintf("Trait method %s::%s has not been applied as %s::%s, because of collision with %s::%s",
						tc.Name, m.Name, c.Name, m.Name, src.traitName, m.Name))
				}
				traitMethods[name] = &methodSource{traitName: tc.Name, method: m}

				existing, existsInClass := c.Methods[name]
				// If existing method is abstract (from a previous trait or class body) and
				// current trait method is concrete, the concrete method should replace it
				if existsInClass && existing.Empty && (existing.Modifiers.Has(phpv.ZAttrAbstract) || (existing.Class != nil && existing.Class.GetType() == phpv.ZClassTypeInterface)) && !m.Empty {
					// Check compatibility first
					traitMethod := &phpv.ZClassMethod{
						Name:      m.Name,
						Modifiers: m.Modifiers,
						Method:    m.Method,
						Class:     c,
						Empty:     m.Empty,
						Loc:       m.Loc,
					}
					abstractMethod := existing
					if abstractMethod.FromTrait != nil {
						abstractMethod = &phpv.ZClassMethod{
							Name:      existing.Name,
							Modifiers: existing.Modifiers,
							Method:    existing.Method,
							Class:     existing.FromTrait,
							Empty:     existing.Empty,
							Loc:       existing.Loc,
						}
					}
					if err := c.checkMethodCompatibility(ctx, traitMethod, abstractMethod); err != nil {
						return err
					}
					// Replace the abstract method with the concrete one
					methodCopy := &phpv.ZClassMethod{
						Name:       m.Name,
						Modifiers:  m.Modifiers,
						Method:     m.Method,
						Class:      c,
						Empty:      m.Empty,
						Loc:        m.Loc,
						Attributes: m.Attributes,
						FromTrait:  tc,
					}
					c.Methods[name] = methodCopy
					if name == "__construct" {
						c.Handlers().Constructor = methodCopy
					}
					continue
				}
				if existsInClass && existing.Class == c && !existing.Empty && existing.FromTrait == nil {
					// Class has its own concrete implementation (not from a trait).
					// If trait method is abstract, check compatibility.
					if m.Empty && m.Modifiers.Has(phpv.ZAttrAbstract) {
						// Create a temporary method ref with trait's class for error message
						traitMethod := &phpv.ZClassMethod{
							Name:      m.Name,
							Modifiers: m.Modifiers,
							Method:    m.Method,
							Class:     tc,
							Empty:     m.Empty,
							Loc:       m.Loc,
						}
						if err := c.checkMethodCompatibility(ctx, existing, traitMethod); err != nil {
							return err
						}
					}
					// Also check static/non-static mismatch for abstract trait methods
					if m.Empty && m.Modifiers.Has(phpv.ZAttrAbstract) {
						if m.Modifiers.Has(phpv.ZAttrStatic) && !existing.Modifiers.Has(phpv.ZAttrStatic) {
							return c.fatalError(ctx, fmt.Sprintf("Cannot make static method %s::%s() non static in class %s", tc.Name, m.Name, c.Name))
						}
						if !m.Modifiers.Has(phpv.ZAttrStatic) && existing.Modifiers.Has(phpv.ZAttrStatic) {
							return c.fatalError(ctx, fmt.Sprintf("Cannot make non static method %s::%s() static in class %s", tc.Name, m.Name, c.Name))
						}
					}
					// Keep class's own method, don't import trait method
				} else if !existsInClass || (existing.Class != nil && existing.Class != c && !(m.Empty && m.Modifiers.Has(phpv.ZAttrAbstract) && !existing.Empty)) {
					// Check if the inherited method is final (cannot be overridden by trait)
					if existsInClass && existing.Modifiers.Has(phpv.ZAttrFinal) && !m.Empty {
						if existing.Class != nil {
							return c.fatalError(ctx, fmt.Sprintf("Cannot override final method %s::%s()", existing.Class.GetName(), existing.Name))
						}
					}
					// If the trait method replaces an inherited abstract method, check compatibility
					if existsInClass && !m.Empty && existing.Empty && (existing.Modifiers.Has(phpv.ZAttrAbstract) || (existing.Class != nil && existing.Class.GetType() == phpv.ZClassTypeInterface)) {
						// Check static/non-static compatibility
						if existing.Modifiers.Has(phpv.ZAttrStatic) && !m.Modifiers.Has(phpv.ZAttrStatic) {
							return c.fatalError(ctx, fmt.Sprintf("Cannot make static method %s::%s() non static in class %s", existing.Class.GetName(), existing.Name, c.Name))
						}
						if !existing.Modifiers.Has(phpv.ZAttrStatic) && m.Modifiers.Has(phpv.ZAttrStatic) {
							return c.fatalError(ctx, fmt.Sprintf("Cannot make non static method %s::%s() static in class %s", existing.Class.GetName(), existing.Name, c.Name))
						}
						// Check method signature compatibility
						traitMethod := &phpv.ZClassMethod{
							Name:      m.Name,
							Modifiers: m.Modifiers,
							Method:    m.Method,
							Class:     c,
							Empty:     m.Empty,
							Loc:       m.Loc,
						}
						if err := c.checkMethodCompatibility(ctx, traitMethod, existing); err != nil {
							return err
						}
					}
					methodCopy := &phpv.ZClassMethod{
						Name:       m.Name,
						Modifiers:  m.Modifiers,
						Method:     m.Method,
						Class:      c,
						Empty:      m.Empty,
						Loc:        m.Loc,
						Attributes: m.Attributes,
						FromTrait:  tc,
					}
					c.Methods[name] = methodCopy
					if name == "__construct" {
						c.Handlers().Constructor = methodCopy
					}
				}
			}

			for _, tp := range tc.Props {
				found := false
				for _, cp := range c.Props {
					if cp.VarName == tp.VarName {
						found = true
						// Check for property conflict: different visibility, static, or readonly mismatch
						incompatible := false
						if cp.Modifiers&phpv.ZAttrAccess != tp.Modifiers&phpv.ZAttrAccess {
							incompatible = true
						}
						if cp.Modifiers&phpv.ZAttrStatic != tp.Modifiers&phpv.ZAttrStatic {
							incompatible = true
						}
						if cp.Modifiers&phpv.ZAttrReadonly != tp.Modifiers&phpv.ZAttrReadonly {
							incompatible = true
						}
						// Check default value compatibility
						if !incompatible && cp.Default != nil && tp.Default != nil {
							ev := fmt.Sprintf("%v", cp.Default)
							tv := fmt.Sprintf("%v", tp.Default)
							if ev != tv {
								incompatible = true
							}
						} else if !incompatible && ((cp.Default == nil) != (tp.Default == nil)) {
							incompatible = true
						}
						if incompatible {
							// Use the trait name that originally provided the property, not "c.Name"
							firstProvider := c.Name
							if src, ok := propSources[cp.VarName]; ok {
								firstProvider = src.traitName
							}
							return c.fatalError(ctx, fmt.Sprintf("%s and %s define the same property ($%s) in the composition of %s. However, the definition differs and is considered incompatible. Class was composed", firstProvider, tc.Name, tp.VarName, c.Name))
						}
						break
					}
				}
				if !found {
					c.Props = append(c.Props, tp)
					propSources[tp.VarName] = &propSourceInfo{traitName: tc.Name}
				}
			}

			for _, k := range tc.ConstOrder {
				if v := tc.Const[k]; v != nil {
					if existing, exists := c.Const[k]; !exists {
						c.Const[k] = v
						c.ConstOrder = append(c.ConstOrder, k)
						constTraitSources[k] = tc.Name
					} else {
						// Check for constant conflicts: different value, visibility, or finality
						incompatible := false
						if existing.Modifiers&phpv.ZAttrAccess != v.Modifiers&phpv.ZAttrAccess {
							incompatible = true // different visibility
						} else if existing.Modifiers&phpv.ZAttrFinal != v.Modifiers&phpv.ZAttrFinal {
							incompatible = true // different finality
						} else if existing.Value != nil && v.Value != nil {
							// Resolve CompileDelayed values before comparison
							existingVal := existing.Value
							if cd, ok := existingVal.(*phpv.CompileDelayed); ok {
								if z, err := cd.Run(ctx); err == nil {
									existingVal = z.Value()
								}
							}
							traitVal := v.Value
							if cd, ok := traitVal.(*phpv.CompileDelayed); ok {
								if z, err := cd.Run(ctx); err == nil {
									traitVal = z.Value()
								}
							}
							ev := fmt.Sprintf("%v", existingVal)
							tv := fmt.Sprintf("%v", traitVal)
							if ev != tv {
								incompatible = true
							}
						}
						if incompatible {
							// Use the trait name that originally provided the constant, not "c.Name"
							firstProvider := c.Name
							if src, ok := constTraitSources[k]; ok {
								firstProvider = src
							}
							return c.fatalError(ctx, fmt.Sprintf("%s and %s define the same constant (%s) in the composition of %s. However, the definition differs and is considered incompatible. Class was composed", firstProvider, tc.Name, k, c.Name))
						}
					}
				}
			}
		}

		// Build set of all trait method names for alias validation
		allTraitMethodNames := make(map[phpv.ZString]bool)
		for _, tc := range resolvedTraits {
			for name := range tc.Methods {
				allTraitMethodNames[name] = true
			}
		}

		// Apply aliases
		for _, alias := range tu.Aliases {
			// Validate: the alias references a trait in the use list if a trait name is specified
			if alias.TraitName != "" {
				traitNameLower := alias.TraitName.ToLower()
				found := false
				for _, tc := range resolvedTraits {
					if tc.Name.ToLower() == traitNameLower {
						found = true
						break
					}
				}
				if !found {
					// Try to resolve the class to give a better error message
					otherClass, lookupErr := ctx.Global().GetClass(ctx, alias.TraitName, false)
					if lookupErr != nil || otherClass == nil {
						return c.fatalError(ctx, fmt.Sprintf("Could not find trait %s", alias.TraitName))
					}
					oc := otherClass.(*ZClass)
					if oc.Type != phpv.ZClassTypeTrait {
						return c.fatalError(ctx, fmt.Sprintf("Class %s is not a trait, Only traits may be used in 'as' and 'insteadof' statements", alias.TraitName))
					}
					return c.fatalError(ctx, fmt.Sprintf("Required Trait %s wasn't added to %s", alias.TraitName, c.Name))
				}
			}

			if alias.NewName != "" {
				// Find the method to alias
				srcName := alias.MethodName.ToLower()

				// Validate method exists in one of the used traits
				if alias.TraitName != "" {
					// Look specifically in the named trait
					traitNameLower := alias.TraitName.ToLower()
					foundInTrait := false
					for _, tc := range resolvedTraits {
						if tc.Name.ToLower() == traitNameLower {
							if _, ok := tc.Methods[srcName]; ok {
								foundInTrait = true
							}
							break
						}
					}
					if !foundInTrait {
						return c.fatalError(ctx, fmt.Sprintf("An alias was defined for %s::%s but this method does not exist", alias.TraitName, alias.MethodName))
					}
				} else {
					// No trait specified - method must exist in at least one trait
					if !allTraitMethodNames[srcName] {
						return c.fatalError(ctx, fmt.Sprintf("An alias (%s) was defined for method %s(), but this method does not exist", alias.NewName, alias.MethodName))
					}
				}

				if m, ok := c.Methods[srcName]; ok {
					// If a trait name was specified, verify it matches
					if alias.TraitName != "" {
						if m.Class == nil || m.Class.GetName().ToLower() != alias.TraitName.ToLower() {
							// Try to find the method from the specific trait
							traitClass, err := ctx.Global().GetClass(ctx, alias.TraitName, true)
							if err == nil {
								tc := traitClass.(*ZClass)
								if tm, ok := tc.Methods[srcName]; ok {
									m = &phpv.ZClassMethod{
										Name:      tm.Name,
										Modifiers: tm.Modifiers,
										Method:    tm.Method,
										Class:     c,
										Empty:     tm.Empty,
										Loc:       tm.Loc,
									}
								}
							}
						}
					}

					// Check for ambiguity: method exists in multiple traits without resolution
					if alias.TraitName == "" {
						// Count how many traits have this method
						conflictTraits := []phpv.ZString{}
						for _, tc := range resolvedTraits {
							if _, ok := tc.Methods[srcName]; ok {
								conflictTraits = append(conflictTraits, tc.Name)
							}
						}
						if len(conflictTraits) > 1 {
							return c.fatalError(ctx, fmt.Sprintf("An alias was defined for method %s(), which exists in both %s and %s. Use %s::%s or %s::%s to resolve the ambiguity",
								alias.MethodName, conflictTraits[0], conflictTraits[1], conflictTraits[0], alias.MethodName, conflictTraits[1], alias.MethodName))
						}
					}

					// Determine source trait for the aliased method
					aliasTrait := m.FromTrait
					if aliasTrait == nil && m.Class != nil && m.Class != c {
						aliasTrait = m.Class
					}
					newMethod := &phpv.ZClassMethod{
						Name:      alias.NewName,
						Modifiers: m.Modifiers,
						Method:    m.Method,
						Class:     c,
						Empty:     m.Empty,
						Loc:       m.Loc,
						FromTrait: aliasTrait,
					}
					if alias.NewAttr != 0 {
						if alias.NewAttr == phpv.ZAttrFinal {
							// Add final modifier
							newMethod.Modifiers = newMethod.Modifiers | phpv.ZAttrFinal
						} else {
							// Replace access modifiers
							newMethod.Modifiers = (newMethod.Modifiers &^ phpv.ZAttrAccess) | alias.NewAttr
						}
					}
					// Check if the alias name conflicts with an existing trait method from a different trait
					aliasLower := alias.NewName.ToLower()
					if existing, exists := c.Methods[aliasLower]; exists && existing.FromTrait != nil {
						// Determine source trait of the aliased method
						srcTraitName := phpv.ZString("")
						if newMethod.FromTrait != nil {
							srcTraitName = newMethod.FromTrait.GetName()
						} else if alias.TraitName != "" {
							srcTraitName = alias.TraitName
						}
						existingTraitName := existing.FromTrait.GetName()
						if srcTraitName != existingTraitName {
							return c.fatalError(ctx, fmt.Sprintf("Trait method %s::%s has not been applied as %s::%s, because of collision with %s::%s",
								existingTraitName, existing.Name, c.Name, alias.NewName, srcTraitName, alias.NewName))
						}
					}
					c.Methods[aliasLower] = newMethod
				}
			} else if alias.NewAttr != 0 {
				// Visibility change only (no rename)
				srcName := alias.MethodName.ToLower()

				// Validate method exists in one of the used traits
				if alias.TraitName != "" {
					traitNameLower := alias.TraitName.ToLower()
					foundInTrait := false
					for _, tc := range resolvedTraits {
						if tc.Name.ToLower() == traitNameLower {
							if _, ok := tc.Methods[srcName]; ok {
								foundInTrait = true
							}
							break
						}
					}
					if !foundInTrait {
						return c.fatalError(ctx, fmt.Sprintf("The modifiers of the trait method %s() are changed, but this method does not exist. Error", alias.MethodName))
					}
				} else {
					if !allTraitMethodNames[srcName] {
						return c.fatalError(ctx, fmt.Sprintf("The modifiers of the trait method %s() are changed, but this method does not exist. Error", alias.MethodName))
					}
				}

				if m, ok := c.Methods[srcName]; ok {
					// Don't change the original method's modifiers directly if it was from a trait -
					// create a copy so aliasing doesn't affect the original trait method
					var newMods phpv.ZObjectAttr
					if alias.NewAttr == phpv.ZAttrFinal {
						newMods = m.Modifiers | phpv.ZAttrFinal
					} else {
						newMods = (m.Modifiers &^ phpv.ZAttrAccess) | alias.NewAttr
					}
					methodCopy := &phpv.ZClassMethod{
						Name:       m.Name,
						Modifiers:  newMods,
						Method:     m.Method,
						Class:      c,
						Empty:      m.Empty,
						Loc:        m.Loc,
						Attributes: m.Attributes,
					}
					c.Methods[srcName] = methodCopy
				}
			}
		}

		// Validate insteadof references: check that referenced traits are actually in the use list
		// and that they are not regular classes
		for _, io := range tu.Insteadof {
			for _, excTrait := range io.InsteadOf {
				// Check if it's a class (not a trait) - give specific error
				otherClass, err := ctx.Global().GetClass(ctx, excTrait, false)
				if err == nil && otherClass != nil {
					oc := otherClass.(*ZClass)
					if oc.Type != phpv.ZClassTypeTrait {
						return c.fatalError(ctx, fmt.Sprintf("Class %s is not a trait, Only traits may be used in 'as' and 'insteadof' statements", excTrait))
					}
				}
			}
		}
	}

	// Readonly class: validate that all trait-imported properties are readonly
	if c.Attr.Has(phpv.ZClassReadonly) {
		for _, prop := range c.Props {
			if propSources[prop.VarName] != nil && !prop.Modifiers.IsReadonly() {
				src := propSources[prop.VarName]
				return c.fatalError(ctx, fmt.Sprintf("Readonly class %s cannot use trait with a non-readonly property %s::$%s", c.Name, src.traitName, prop.VarName))
			}
		}
	}

	for _, impl := range c.ImplementsStr {
		intf, err := ctx.Global().GetClass(ctx, impl, true)
		if err != nil {
			// Replace "Class" with "Interface" in the error message
			return ThrowError(ctx, Error, fmt.Sprintf("Interface \"%s\" not found", impl))
		}
		intfClass := intf.(*ZClass)
		// Check that we're implementing an interface, not a regular class
		if c.Type != phpv.ZClassTypeInterface && intfClass.Type != phpv.ZClassTypeInterface {
			return c.fatalError(ctx, fmt.Sprintf("%s cannot implement %s - it is not an interface", c.Name, intfClass.Name))
		}
		// Check if this is an internal-only interface that user classes can't implement
		if intfClass.InternalOnly && c.L != nil {
			return c.fatalError(ctx, fmt.Sprintf("%s can't be implemented by user classes", intfClass.Name))
		}
		// Non-enum classes cannot implement UnitEnum or BackedEnum
		if !c.Type.Has(phpv.ZClassTypeEnum) && (intfClass == UnitEnum || intfClass == BackedEnum) {
			return c.fatalError(ctx, fmt.Sprintf("Non-enum class %s cannot implement interface %s", c.Name, intfClass.Name))
		}
		c.Implementations = append(c.Implementations, intfClass)
		// Add interface and its parents to the parents map for InstanceOf checks
		if c.parents == nil {
			c.parents = make(map[*ZClass]*ZClass)
		}
		c.parents[intfClass] = intfClass
		// Also add the interface's own parents (transitively implemented interfaces)
		for p := range intfClass.parents {
			c.parents[p] = p
		}
		// Also add the interface's implementations (interfaces it extends)
		for _, implIntf := range intfClass.Implementations {
			c.parents[implIntf] = implIntf
		}
	}

	// PHP requires that non-abstract classes implementing Traversable must do so through
	// either Iterator or IteratorAggregate, not directly. Abstract classes are allowed
	// to implement Traversable directly.
	if c.Type != phpv.ZClassTypeInterface && c.Attr&phpv.ZClassAttr(phpv.ZClassExplicitAbstract) == 0 {
		implementsTraversable := false
		implementsIteratorOrAggregate := false
		// Check direct implementations and parents (which include transitive interfaces)
		if c.parents != nil {
			if _, ok := c.parents[Traversable]; ok {
				implementsTraversable = true
			}
			if _, ok := c.parents[Iterator]; ok {
				implementsIteratorOrAggregate = true
			}
			if _, ok := c.parents[IteratorAggregate]; ok {
				implementsIteratorOrAggregate = true
			}
		}
		// Also check direct implementations list
		for _, impl := range c.Implementations {
			if impl == Traversable {
				implementsTraversable = true
			}
			if impl == Iterator || impl == IteratorAggregate {
				implementsIteratorOrAggregate = true
			}
		}
		if implementsTraversable && !implementsIteratorOrAggregate {
			return c.fatalError(ctx, fmt.Sprintf("Class %s must implement interface Traversable as part of either Iterator or IteratorAggregate", c.Name))
		}

		// Check mutual exclusion: cannot implement both Iterator and IteratorAggregate
		hasIterator := c.Implements(Iterator)
		hasIteratorAggregate := c.Implements(IteratorAggregate)
		if hasIterator && hasIteratorAggregate {
			return c.fatalError(ctx, fmt.Sprintf("Class %s cannot implement both Iterator and IteratorAggregate at the same time", c.Name))
		}
	}

	// Auto-implement Stringable interface for classes with __toString().
	if c.Type != phpv.ZClassTypeInterface && c.Type != phpv.ZClassTypeTrait {
		if _, hasToString := c.GetMethod("__tostring"); hasToString {
			alreadyImplements := false
			for _, impl := range c.Implementations {
				if impl == Stringable {
					alreadyImplements = true
					break
				}
			}
			if !alreadyImplements && (c.parents == nil || c.parents[Stringable] == nil) {
				c.Implementations = append(c.Implementations, Stringable)
				if c.parents == nil {
					c.parents = make(map[*ZClass]*ZClass)
				}
				c.parents[Stringable] = Stringable
			}
		}
	}

	// Emit Serializable interface deprecation warning (PHP 8.1+)
	// Only emit if the class does NOT also have __serialize/__unserialize (forward-compatible classes)
	// Ignore the error return - this deprecation should not prevent class registration
	if c.Type != phpv.ZClassTypeInterface && c.Type != phpv.ZClassTypeTrait && c.Implements(Serializable) {
		_, hasNewSerialize := c.GetMethod("__serialize")
		_, hasNewUnserialize := c.GetMethod("__unserialize")
		if !hasNewSerialize || !hasNewUnserialize {
			_ = ctx.Deprecated("%s implements the Serializable interface, which is deprecated. Implement __serialize() and __unserialize() instead (or in addition, if support for old PHP versions is necessary)", c.Name)
		}
	}

	// Try to resolve constants eagerly, but if resolution fails (e.g. forward
	// reference to a class not yet defined), leave them as CompileDelayed for
	// lazy resolution when accessed (handled in compile-classref.go).
	for _, k := range c.ConstOrder {
		cc := c.Const[k]
		if cc == nil {
			continue
		}
		if r, ok := cc.Value.(*phpv.CompileDelayed); ok {
			z, err := r.Run(ctx)
			if err == nil {
				c.Const[k].Value = z.Value()
			}
			// If err != nil, leave as CompileDelayed for lazy resolution
		}
	}
	// Property defaults are resolved lazily in GetStaticProps() and
	// ZObject.init() to support forward references to classes/constants
	// not yet defined at class compilation time.
	// Check interface properties: interfaces can only have hooked properties (PHP 8.4+)
	if c.Type == phpv.ZClassTypeInterface && len(c.Props) > 0 {
		for _, prop := range c.Props {
			if !prop.HasHooks {
				return c.fatalError(ctx, fmt.Sprintf("Interfaces may only include hooked properties"))
			}
		}
	}
	for _, m := range c.Methods {
		if c.Type == phpv.ZClassTypeInterface && !m.Empty {
			loc := m.Loc
			if loc == nil {
				loc = c.L
			}
			return c.fatalErrorAt(ctx, fmt.Sprintf("Interface function %s::%s() cannot contain body", c.Name, m.Name), loc)
		}
		// Check private interface methods
		if c.Type == phpv.ZClassTypeInterface && m.Modifiers.Has(phpv.ZAttrPrivate) {
			loc := m.Loc
			if loc == nil {
				loc = c.L
			}
			return c.fatalErrorAt(ctx, fmt.Sprintf("Access type for interface method %s::%s() must be public", c.Name, m.Name), loc)
		}
		// Check final/abstract modifiers on interface methods
		if c.Type == phpv.ZClassTypeInterface && m.Modifiers.Has(phpv.ZAttrFinal) {
			loc := m.Loc
			if loc == nil {
				loc = c.L
			}
			return c.fatalErrorAt(ctx, fmt.Sprintf("Interface method %s::%s() must not be final", c.Name, m.Name), loc)
		}
		if c.Type == phpv.ZClassTypeInterface && m.Modifiers.Has(phpv.ZAttrAbstract) {
			loc := m.Loc
			if loc == nil {
				loc = c.L
			}
			return c.fatalErrorAt(ctx, fmt.Sprintf("Interface method %s::%s() must not be abstract", c.Name, m.Name), loc)
		}
		if m.Modifiers.Has(phpv.ZAttrAbstract) && m.Modifiers.Has(phpv.ZAttrFinal) {
			return c.fatalError(ctx, "Cannot use the final modifier on an abstract method")
		}
		// Warn about final private methods (they can never be overridden)
		// Skip for trait-imported methods where final came from the trait (PHP relaxes this)
		if m.Modifiers.Has(phpv.ZAttrFinal) && m.Modifiers.Has(phpv.ZAttrPrivate) && (m.Class == nil || m.Class == c) && m.Name.ToLower() != "__construct" && m.FromTrait == nil {
			phpErr := &phpv.PhpError{
				Err:  fmt.Errorf("Private methods cannot be final as they are never overridden by other classes"),
				Code: phpv.E_WARNING,
				Loc:  m.Loc,
			}
			ctx.Global().LogError(phpErr)
		}
		if comp, ok := m.Method.(phpv.Compilable); ok {
			err := comp.Compile(ctx)
			if err != nil {
				return err
			}
		}
	}

	// Validate attributes on the class itself, its methods, properties, and constants
	if err := c.validateAttributes(ctx); err != nil {
		return err
	}

	// Validate magic method signatures
	if err := c.validateMagicMethods(ctx); err != nil {
		return err
	}

	// Import abstract methods and constants from interfaces that aren't already defined
	for _, intf := range c.Implementations {
		for n, m := range intf.Methods {
			if ours, gotit := c.Methods[n]; !gotit {
				c.Methods[n] = m
			} else {
				// Check method signature compatibility for interface implementations
				if err := c.checkMethodCompatibility(ctx, ours, m); err != nil {
					return err
				}
			}
		}
		for _, k := range intf.ConstOrder {
			if v := intf.Const[k]; v != nil {
				if existing, exists := c.Const[k]; !exists {
					c.Const[k] = v
					c.ConstOrder = append(c.ConstOrder, k)
					// Track which interface provided this constant for ambiguity detection
					if c.constSource == nil {
						c.constSource = make(map[phpv.ZString]phpv.ZString)
					}
					c.constSource[k] = intf.Name
				} else if existing == v {
					// Same constant object (diamond inheritance) - no conflict
				} else {
					// Check visibility: interface constants are implicitly public,
					// so the implementing class must also make them public
					if !v.Modifiers.IsPrivate() && existing.Modifiers.IsPrivate() {
						return c.fatalError(ctx, fmt.Sprintf("Access level to %s::%s must be public (as in interface %s)", c.Name, k, intf.Name))
					}
					if !v.Modifiers.IsPrivate() && !v.Modifiers.IsProtected() && existing.Modifiers.IsProtected() {
						return c.fatalError(ctx, fmt.Sprintf("Access level to %s::%s must be public (as in interface %s)", c.Name, k, intf.Name))
					}
					if v.Modifiers.Has(phpv.ZAttrFinal) {
						return c.fatalError(ctx, fmt.Sprintf("%s::%s cannot override final constant %s::%s", c.Name, k, intf.Name, k))
					}
					// Check ambiguity: constant from different source
					if src, hasSrc := c.constSource[k]; hasSrc && src != intf.Name {
						classType := "Class"; if c.Type.IsInterface() { classType = "Interface" }; return c.fatalError(ctx, fmt.Sprintf("%s %s inherits both %s::%s and %s::%s, which is ambiguous", classType, c.Name, src, k, intf.Name, k))
					}
				}
			}
		}
	}

	// Special check: even abstract classes must implement private abstract trait methods,
	// because private methods are not inherited to subclasses.
	if c.Type != phpv.ZClassTypeInterface && c.Type != phpv.ZClassTypeTrait && c.Attr&phpv.ZClassAttr(phpv.ZClassExplicitAbstract) != 0 {
		var privateUnimpl []string
		for _, m := range c.Methods {
			if m.Empty && m.Modifiers.Has(phpv.ZAttrAbstract) && m.Modifiers.Has(phpv.ZAttrPrivate) && m.FromTrait != nil {
				privateUnimpl = append(privateUnimpl, string(c.Name)+"::"+string(m.Name))
			}
		}
		if len(privateUnimpl) > 0 {
			msg := fmt.Sprintf("Class %s must implement %d abstract method", c.Name, len(privateUnimpl))
			if len(privateUnimpl) > 1 {
				msg += "s"
			}
			if len(privateUnimpl) > 1 {
				msg += " and must therefore be declared abstract or implement the remaining methods ("
			} else {
				msg += " (" // different format for private abstract trait methods
			}
			for i, u := range privateUnimpl {
				if i > 0 {
					msg += ", "
				}
				msg += u
			}
			msg += ")"
			return c.fatalError(ctx, msg)
		}
	}

	// Validate: non-abstract, non-interface classes must implement all abstract methods
	if c.Type != phpv.ZClassTypeInterface && c.Type != phpv.ZClassTypeTrait && c.Attr&phpv.ZClassAttr(phpv.ZClassExplicitAbstract) == 0 {
		var ownAbstract []string   // abstract methods declared in this class source code
		var unimplemented []string // inherited abstract methods not implemented
		for _, m := range c.Methods {
			isAbstract := m.Empty && (m.Modifiers.Has(phpv.ZAttrAbstract) || (m.Class != nil && m.Class.GetType() == phpv.ZClassTypeInterface))
			if isAbstract {
				// Methods imported from traits are "inherited" (not "declared" in source)
				if m.FromTrait != nil {
					// Trait-imported abstract methods use the composing class name (C::method)
					unimplemented = append(unimplemented, string(c.Name)+"::"+string(m.Name))
				} else if m.Class == nil || m.Class == c {
					// Declared in this class source code
					ownAbstract = append(ownAbstract, string(m.Name))
				} else {
					// Inherited from parent/interface
					unimplemented = append(unimplemented, string(m.Class.GetName())+"::"+string(m.Name))
				}
			}
		}
		if len(ownAbstract) > 0 && len(unimplemented) == 0 {
			// PHP: "Class X declares abstract method Y() and must therefore be declared abstract"
			return c.fatalError(ctx, fmt.Sprintf("Class %s declares abstract method %s() and must therefore be declared abstract", c.Name, ownAbstract[0]))
		}
		// If there are both own abstract and unimplemented, combine them all as unimplemented
		if len(ownAbstract) > 0 {
			for _, name := range ownAbstract {
				unimplemented = append(unimplemented, string(c.Name)+"::"+name)
			}
		}
		if len(unimplemented) > 0 {
			msg := fmt.Sprintf("Class %s contains %d abstract method", c.Name, len(unimplemented))
			if len(unimplemented) > 1 {
				msg += "s"
			}
			if len(unimplemented) > 1 {
				msg += " and must therefore be declared abstract or implement the remaining methods ("
			} else {
				msg += " and must therefore be declared abstract or implement the remaining method ("
			}
			for i, u := range unimplemented {
				if i > 0 {
					msg += ", "
				}
				msg += u
			}
			msg += ")"
			return c.fatalError(ctx, msg)
		}
	}

	// Validate #[\Override] attribute: methods with this attribute must have a
	// matching method in a parent class, implemented interface, or abstract trait method.
	if c.Type != phpv.ZClassTypeTrait {
		for _, m := range c.Methods {
			if !methodHasOverride(m) {
				continue
			}
			// Only check methods defined in this class (not inherited from parent)
			if m.Class != nil && m.Class != c {
				continue
			}

			methodName := m.Name.ToLower()
			found := false

			// Check parent class for a matching method
			if c.Extends != nil {
				if parentMethod, ok := c.Extends.Methods[methodName]; ok {
					if methodName == "__construct" {
						// For __construct, only abstract parent constructors satisfy #[\Override]
						if parentMethod.Modifiers.Has(phpv.ZAttrAbstract) || (parentMethod.Empty && parentMethod.Class != nil && parentMethod.Class.GetType() == phpv.ZClassTypeInterface) {
							found = true
						}
					} else if !parentMethod.Modifiers.Has(phpv.ZAttrPrivate) {
						found = true
					}
				}
			}

			// Check directly implemented interfaces
			if !found {
				for _, intf := range c.Implementations {
					if _, ok := intf.Methods[methodName]; ok {
						found = true
						break
					}
				}
			}

			// Check used traits for abstract methods with the same name
			if !found {
				for _, tu := range c.TraitUses {
					for _, traitName := range tu.TraitNames {
						traitClass, err := ctx.Global().GetClass(ctx, traitName, true)
						if err != nil {
							continue
						}
						if tc, ok := traitClass.(*ZClass); ok {
							if tm, ok := tc.Methods[methodName]; ok {
								if tm.Modifiers.Has(phpv.ZAttrAbstract) || tm.Empty {
									found = true
									break
								}
							}
						}
					}
					if found {
						break
					}
				}
			}

			if !found {
				loc := m.Loc
				if loc == nil {
					loc = c.L
				}
				return c.fatalErrorAt(ctx, fmt.Sprintf("%s::%s() has #[\\Override] attribute, but no matching parent method exists", c.GetName(), m.Name), loc)
			}
		}
	}

	return nil
}

// validateAttributes validates attributes on the class itself, its methods,
// properties, and class constants. This performs compile-time attribute validation
// for known internal attributes (target checking, repeat checking) and special
// rules like #[Attribute] only on non-abstract concrete classes.
func (c *ZClass) validateAttributes(ctx phpv.Context) error {
	// Validate class-level attributes
	if len(c.Attributes) > 0 {
		// Special validation for #[Attribute]: can only be applied to non-abstract,
		// non-interface, non-trait, non-enum classes
		for _, attr := range c.Attributes {
			name := attr.ClassName
			if name == "Attribute" || name == "\\Attribute" {
				if c.Type.Has(phpv.ZClassTypeInterface) {
					return c.fatalError(ctx, fmt.Sprintf("Cannot apply #[\\Attribute] to interface %s", c.GetName()))
				} else if c.Type.Has(phpv.ZClassTypeTrait) {
					return c.fatalError(ctx, fmt.Sprintf("Cannot apply #[\\Attribute] to trait %s", c.GetName()))
				} else if c.Type.Has(phpv.ZClassTypeEnum) {
					return c.fatalError(ctx, fmt.Sprintf("Cannot apply #[\\Attribute] to enum %s", c.GetName()))
				} else if c.Attr.Has(phpv.ZClassExplicitAbstract) {
					return c.fatalError(ctx, fmt.Sprintf("Cannot apply #[\\Attribute] to abstract class %s", c.GetName()))
				}
			}
			// Special validation for #[AllowDynamicProperties]
			if name == "AllowDynamicProperties" || name == "\\AllowDynamicProperties" {
				if c.Type.Has(phpv.ZClassTypeInterface) {
					return c.fatalError(ctx, fmt.Sprintf("Cannot apply #[\\AllowDynamicProperties] to interface %s", c.GetName()))
				} else if c.Type.Has(phpv.ZClassTypeTrait) {
					return c.fatalError(ctx, fmt.Sprintf("Cannot apply #[\\AllowDynamicProperties] to trait %s", c.GetName()))
				} else if c.Type.Has(phpv.ZClassTypeEnum) {
					return c.fatalError(ctx, fmt.Sprintf("Cannot apply #[\\AllowDynamicProperties] to enum %s", c.GetName()))
				} else if c.Attr.Has(phpv.ZClassReadonly) {
					return c.fatalError(ctx, fmt.Sprintf("Cannot apply #[\\AllowDynamicProperties] to readonly class %s", c.GetName()))
				}
			}
		}

		// Validate target and repeat for class attributes (internal attrs only at compile time)
		if msg := ValidateInternalAttributeList(ctx, c.Attributes, AttributeTARGET_CLASS); msg != "" {
			return c.fatalError(ctx, msg)
		}
	}

	// Validate method-level attributes (internal attrs only at compile time)
	for _, m := range c.Methods {
		// Only validate methods defined in this class (not inherited)
		if m.Class != nil && m.Class != c {
			continue
		}
		if len(m.Attributes) > 0 {
			if msg := ValidateInternalAttributeList(ctx, m.Attributes, AttributeTARGET_METHOD); msg != "" {
				loc := m.Loc
				if loc == nil {
					loc = c.L
				}
				return c.fatalErrorAt(ctx, msg, loc)
			}
			// NoDiscard-specific validations
			for _, attr := range m.Attributes {
				if attr.ClassName == "NoDiscard" || attr.ClassName == "\\NoDiscard" {
					lowerName := string(m.Name.ToLower())
					if lowerName == "__construct" || lowerName == "__clone" {
						return c.fatalError(ctx, fmt.Sprintf("Method %s::%s cannot be #[\\NoDiscard]", c.Name, m.Name))
					}
					// Check return type
					type returnTypeGetter interface {
						GetReturnType() *phpv.TypeHint
					}
					if rtg, ok := m.Method.(returnTypeGetter); ok {
						if rt := rtg.GetReturnType(); rt != nil {
							if rt.Type() == phpv.ZtVoid {
								return c.fatalError(ctx, "A void function does not return a value, but #[\\NoDiscard] requires a return value")
							}
							if rt.Type() == phpv.ZtNever {
								return c.fatalError(ctx, "A never returning function does not return a value, but #[\\NoDiscard] requires a return value")
							}
						}
					}
				}
			}
		}
	}

	// Validate property-level attributes (internal attrs only at compile time)
	for _, p := range c.Props {
		if len(p.Attributes) > 0 {
			if msg := ValidateInternalAttributeList(ctx, p.Attributes, AttributeTARGET_PROPERTY); msg != "" {
				return c.fatalError(ctx, msg)
			}
		}
	}

	// Validate class constant attributes (internal attrs only at compile time)
	for _, k := range c.ConstOrder {
		cc := c.Const[k]
		if cc == nil {
			continue
		}
		if len(cc.Attributes) > 0 {
			if msg := ValidateInternalAttributeList(ctx, cc.Attributes, AttributeTARGET_CLASS_CONSTANT); msg != "" {
				return c.fatalError(ctx, msg)
			}
		}
	}

	return nil
}

// methodHasOverride checks if a method has the #[\Override] attribute.
func methodHasOverride(m *phpv.ZClassMethod) bool {
	for _, attr := range m.Attributes {
		if attr.ClassName == "Override" {
			return true
		}
	}
	return false
}

func (c *ZClass) checkMethodCompatibility(ctx phpv.Context, child *phpv.ZClassMethod, parent *phpv.ZClassMethod) error {
	// Check static/non-static mismatch
	childStatic := child.Modifiers.Has(phpv.ZAttrStatic)
	parentStatic := parent.Modifiers.Has(phpv.ZAttrStatic)
	if parentStatic && !childStatic {
		loc := child.Loc
		if loc == nil {
			loc = c.L
		}
		return c.fatalErrorAt(ctx, fmt.Sprintf("Cannot make static method %s::%s() non static in class %s", parent.Class.GetName(), parent.Name, c.Name), loc)
	}
	if !parentStatic && childStatic {
		loc := child.Loc
		if loc == nil {
			loc = c.L
		}
		return c.fatalErrorAt(ctx, fmt.Sprintf("Cannot make non static method %s::%s() static in class %s", parent.Class.GetName(), parent.Name, c.Name), loc)
	}

	childFGA, childOK := child.Method.(phpv.FuncGetArgs)
	parentFGA, parentOK := parent.Method.(phpv.FuncGetArgs)
	if !childOK || !parentOK {
		return nil
	}

	childArgs := childFGA.GetArgs()
	parentArgs := parentFGA.GetArgs()

	// Count required args
	childRequired := 0
	for _, a := range childArgs {
		if a.Required {
			childRequired++
		}
	}
	parentRequired := 0
	for _, a := range parentArgs {
		if a.Required {
			parentRequired++
		}
	}

	// Count non-variadic parent params for comparison
	parentNonVariadic := len(parentArgs)
	parentHasVariadic := false
	for i, a := range parentArgs {
		if a.Variadic {
			parentNonVariadic = i
			parentHasVariadic = true
			break
		}
	}

	childNonVariadic := len(childArgs)
	childHasVariadic := false
	for i, a := range childArgs {
		if a.Variadic {
			childNonVariadic = i
			childHasVariadic = true
			break
		}
	}

	// Child cannot require more parameters than parent requires
	// Child cannot have fewer non-variadic parameters than parent's non-variadic count
	// (unless child has a variadic parameter to absorb the rest)
	incompatible := false
	if childRequired > parentRequired {
		incompatible = true
	}
	if !childHasVariadic && childNonVariadic < parentNonVariadic {
		incompatible = true
	}

	// Check by-reference flag compatibility for each parameter
	if !incompatible {
		limit := parentNonVariadic
		if childNonVariadic < limit {
			limit = childNonVariadic
		}
		for i := 0; i < limit; i++ {
			if parentArgs[i].Ref != childArgs[i].Ref {
				incompatible = true
				break
			}
		}
	}

	// If parent has variadic, check by-ref compatibility of child's extra params against parent variadic
	if !incompatible && parentHasVariadic {
		parentVariadicArg := parentArgs[parentNonVariadic]
		for i := parentNonVariadic; i < childNonVariadic; i++ {
			if parentVariadicArg.Ref != childArgs[i].Ref {
				incompatible = true
				break
			}
		}
	}

	// Check return-by-reference compatibility
	if !incompatible {
		type retByRefGetter interface {
			ReturnsByRef() bool
		}
		var parentRetRef, childRetRef bool
		if rr, ok := parent.Method.(retByRefGetter); ok {
			parentRetRef = rr.ReturnsByRef()
		}
		if rr, ok := child.Method.(retByRefGetter); ok {
			childRetRef = rr.ReturnsByRef()
		}
		if parentRetRef != childRetRef {
			incompatible = true
		}
	}

	// Check type hint compatibility for each parameter (contravariance: child can widen)
	if !incompatible {
		for i := 0; i < parentNonVariadic && i < childNonVariadic; i++ {
			ph := parentArgs[i].Hint
			ch := childArgs[i].Hint
			if ph == nil && ch == nil {
				continue
			}
			if ph != nil && ch == nil {
				// Child drops type hint — this is always a widening (accepts anything)
				// which is compatible with contravariance
				continue
			}
			if ph == nil && ch != nil {
				// Child adds type hint where parent had none — narrowing, incompatible
				incompatible = true
				break
			}
			// Both have hints — check if child type is a supertype of (or equal to) parent type.
			// For contravariance, the child must accept at least everything the parent accepts.
			if !typeHintIsWidening(ctx, ch, ph) {
				incompatible = true
				break
			}
		}
	}

	// Check return type covariance (child must return subtype of parent)
	if !incompatible {
		type retTypeGetter interface {
			GetReturnType() *phpv.TypeHint
		}
		var parentRT, childRT *phpv.TypeHint
		if rtg, ok := parent.Method.(retTypeGetter); ok {
			parentRT = rtg.GetReturnType()
		}
		if rtg, ok := child.Method.(retTypeGetter); ok {
			childRT = rtg.GetReturnType()
		}
		if parentRT != nil && childRT != nil {
			// Both have return types — child's return type must be a subtype of parent's
			// (covariance: child must be narrower or equal)
			// Special case: 'never' is the bottom type, always a valid covariant return type
			if childRT.Type() != phpv.ZtNever && !typeHintIsWidening(ctx, parentRT, childRT) {
				incompatible = true
			}
		} else if parentRT != nil && childRT == nil {
			// Parent has return type, child drops it — incompatible
			incompatible = true
		}
		// If parent has no return type and child adds one, that's okay (child is narrower)
	}

	if incompatible {
		loc := child.Loc
		if loc == nil {
			loc = c.L
		}
		childSig := formatMethodSignature(c.Name, child)
		parentSig := formatMethodSignature(parent.Class.GetName(), parent)
		return c.fatalErrorAt(ctx, fmt.Sprintf("Declaration of %s must be compatible with %s", childSig, parentSig), loc)
	}
	return nil
}

func formatMethodSignature(className phpv.ZString, m *phpv.ZClassMethod) string {
	// Check if method returns by reference
	retByRef := ""
	if rr, ok := m.Method.(interface{ ReturnsByRef() bool }); ok && rr.ReturnsByRef() {
		retByRef = "& "
	}
	sig := retByRef + string(className) + "::" + string(m.Name) + "("
	if fga, ok := m.Method.(phpv.FuncGetArgs); ok {
		args := fga.GetArgs()
		for i, a := range args {
			if i > 0 {
				sig += ", "
			}
			if a.Hint != nil {
				sig += a.Hint.String() + " "
			}
			if a.Variadic {
				sig += "..."
			}
			if a.Ref {
				sig += "&"
			}
			sig += "$" + string(a.VarName)
			if !a.Required && !a.Variadic && !a.ImplicitlyNullable {
				sig += " = " + formatDefault(a.DefaultValue)
			}
		}
	}
	sig += ")"
	// Include return type if available
	if rt, ok := m.Method.(interface {
		GetReturnType() *phpv.TypeHint
	}); ok {
		if retType := rt.GetReturnType(); retType != nil {
			sig += ": " + retType.String()
		}
	}
	return sig
}

func formatDefault(v phpv.Val) string {
	if v == nil {
		return "NULL"
	}
	switch vt := v.(type) {
	case phpv.ZNull:
		return "null"
	case phpv.ZInt:
		return fmt.Sprintf("%d", vt)
	case phpv.ZString:
		s := string(vt)
		if len(s) > 10 {
			s = s[:10] + "..."
		}
		return fmt.Sprintf("'%s'", s)
	case phpv.ZFloat:
		return strconv.FormatFloat(float64(vt), 'G', 14, 64)
	case phpv.ZBool:
		if vt {
			return "true"
		}
		return "false"
	case *phpv.ZArray:
		if vt.HashTable().Count() == 0 {
			return "[]"
		}
		return "[...]"
	case *phpv.CompileDelayed:
		// Default value hasn't been resolved yet — try to format the runnable
		return formatDelayedDefault(vt.V)
	default:
		return fmt.Sprintf("%v", v)
	}
}

// formatDelayedDefault attempts to produce a display string for a CompileDelayed
// default value by inspecting the underlying Runnable expression.
func formatDelayedDefault(r phpv.Runnable) string {
	if r == nil {
		return "<default>"
	}
	// Use Dump to get a representation, falling back to a generic placeholder
	var buf strings.Builder
	if err := r.Dump(&buf); err == nil {
		s := buf.String()
		if s != "" {
			return s
		}
	}
	return "<default>"
}

func (c *ZClass) validateMagicMethods(ctx phpv.Context) error {
	// Validate magic method argument counts
	magicArgCounts := map[phpv.ZString]int{
		"__call":       2,
		"__callstatic": 2,
		"__get":        1,
		"__set":        2,
		"__isset":      1,
		"__unset":      1,
		"__unserialize": 1,
	}

	for name, requiredArgs := range magicArgCounts {
		m, ok := c.Methods[name]
		if !ok {
			continue
		}
		if m.Class != nil && m.Class != c {
			continue // inherited, don't re-validate
		}
		if fga, ok := m.Method.(phpv.FuncGetArgs); ok {
			args := fga.GetArgs()
			if len(args) != requiredArgs {
				loc := m.Loc
				if loc == nil {
					loc = c.L
				}
				return c.fatalErrorAt(ctx, fmt.Sprintf("Method %s::%s() must take exactly %d argument%s", c.Name, m.Name, requiredArgs, pluralS(requiredArgs)), loc)
			}
		}
	}

	// Validate methods that cannot take arguments:
	// __clone, __destruct, __serialize, __sleep, __wakeup, __toString
	noArgMethods := []phpv.ZString{"__clone", "__destruct", "__serialize", "__sleep", "__wakeup", "__tostring"}
	for _, name := range noArgMethods {
		m, ok := c.Methods[name]
		if !ok {
			continue
		}
		if m.Class != nil && m.Class != c {
			continue // inherited, don't re-validate
		}
		if fga, ok := m.Method.(phpv.FuncGetArgs); ok {
			args := fga.GetArgs()
			if len(args) > 0 {
				loc := m.Loc
				if loc == nil {
					loc = c.L
				}
				return c.fatalErrorAt(ctx, fmt.Sprintf("Method %s::%s() cannot take arguments", c.Name, m.Name), loc)
			}
		}
	}

	// Validate __construct, __destruct, __clone cannot be static
	// Also __call, __get, __set, __isset, __unset, __toString cannot be static
	noStaticMethods := []phpv.ZString{
		"__construct", "__destruct", "__clone",
		"__call", "__get", "__set", "__isset", "__unset", "__tostring",
	}
	for _, name := range noStaticMethods {
		m, ok := c.Methods[name]
		if !ok {
			continue
		}
		if m.Class != nil && m.Class != c {
			continue // inherited
		}
		if m.Modifiers.Has(phpv.ZAttrStatic) {
			loc := m.Loc
			if loc == nil {
				loc = c.L
			}
			return c.fatalErrorAt(ctx, fmt.Sprintf("Method %s::%s() cannot be static", c.Name, m.Name), loc)
		}
	}

	// Validate __callStatic and __set_state must be static
	mustBeStatic := []phpv.ZString{"__callstatic", "__set_state"}
	for _, name := range mustBeStatic {
		m, ok := c.Methods[name]
		if !ok {
			continue
		}
		if m.Class != nil && m.Class != c {
			continue // inherited
		}
		if !m.Modifiers.Has(phpv.ZAttrStatic) {
			loc := m.Loc
			if loc == nil {
				loc = c.L
			}
			return c.fatalErrorAt(ctx, fmt.Sprintf("Method %s::%s() must be static", c.Name, m.Name), loc)
		}
	}

	// Validate magic methods cannot take arguments by reference
	noRefMethods := []phpv.ZString{
		"__call", "__callstatic", "__get", "__set", "__isset", "__unset",
	}
	for _, name := range noRefMethods {
		m, ok := c.Methods[name]
		if !ok {
			continue
		}
		if m.Class != nil && m.Class != c {
			continue // inherited
		}
		if fga, ok := m.Method.(phpv.FuncGetArgs); ok {
			for _, arg := range fga.GetArgs() {
				if arg.Ref {
					loc := m.Loc
					if loc == nil {
						loc = c.L
					}
					return c.fatalErrorAt(ctx, fmt.Sprintf("Method %s::%s() cannot take arguments by reference", c.Name, m.Name), loc)
				}
			}
		}
	}

	// Validate parameter type hints for magic methods
	// Map of method name → parameter index → required type name
	type paramTypeReq struct {
		paramIdx int
		typeName string // "string" or "array"
	}
	paramTypeChecks := map[phpv.ZString][]paramTypeReq{
		"__get":         {{0, "string"}},
		"__set":         {{0, "string"}},
		"__isset":       {{0, "string"}},
		"__unset":       {{0, "string"}},
		"__call":        {{0, "string"}, {1, "array"}},
		"__callstatic":  {{0, "string"}, {1, "array"}},
		"__unserialize": {{0, "array"}},
		"__set_state":   {{0, "array"}},
	}
	for name, checks := range paramTypeChecks {
		m, ok := c.Methods[name]
		if !ok {
			continue
		}
		if m.Class != nil && m.Class != c {
			continue // inherited
		}
		fga, ok := m.Method.(phpv.FuncGetArgs)
		if !ok {
			continue
		}
		args := fga.GetArgs()
		for _, check := range checks {
			if check.paramIdx >= len(args) {
				continue
			}
			arg := args[check.paramIdx]
			if arg.Hint == nil {
				continue
			}
			// Check if the type hint is compatible. The type hint must be the required
			// type or a union that includes the required type. If the type hint is
			// something incompatible (e.g. "int" when "string" is required), error.
			if !magicParamTypeCompatible(arg.Hint, check.typeName) {
				loc := m.Loc
				if loc == nil {
					loc = c.L
				}
				return c.fatalErrorAt(ctx, fmt.Sprintf("%s::%s(): Parameter #%d ($%s) must be of type %s when declared",
					c.Name, m.Name, check.paramIdx+1, arg.VarName, check.typeName), loc)
			}
		}
	}

	// Note: non-public magic method warnings are emitted by warnNonPublicMagicMethods()
	// which is called earlier in Compile() before inheritance checks.
	// We only call it here for classes without extends (so it always runs).
	if c.Extends == nil {
		c.warnNonPublicMagicMethods(ctx)
	}

	return nil
}

// warnNonPublicMagicMethods emits warnings about non-public magic methods.
// Called early in Compile() so warnings appear before inheritance errors.
func (c *ZClass) warnNonPublicMagicMethods(ctx phpv.Context) {
	mustBePublic := []phpv.ZString{
		"__call", "__callstatic", "__get", "__set", "__isset", "__unset",
		"__debuginfo", "__serialize", "__unserialize", "__invoke",
		"__tostring",
	}
	for _, name := range mustBePublic {
		m, ok := c.Methods[name]
		if !ok {
			continue
		}
		// Only warn if explicitly declared private or protected in this class (not inherited)
		if (m.Modifiers.Has(phpv.ZAttrPrivate) || m.Modifiers.Has(phpv.ZAttrProtected)) && (m.Class == nil || m.Class == c) {
			loc := m.Loc
			if loc == nil {
				loc = c.L
			}
			phpErr := &phpv.PhpError{
				Err:  fmt.Errorf("The magic method %s::%s() must have public visibility", c.Name, m.Name),
				Code: phpv.E_WARNING,
				Loc:  loc,
			}
			ctx.Global().LogError(phpErr)
		}
	}
}

// typeHintIsWidening checks if childHint accepts at least everything parentHint accepts.
// This implements contravariance for parameter types: the child can accept more types.
func typeHintIsWidening(ctx phpv.Context, childHint, parentHint *phpv.TypeHint) bool {
	if childHint == nil {
		// No type hint accepts everything — always a widening
		return true
	}
	if parentHint == nil {
		// Parent has no type but child does — narrowing
		return false
	}
	// If child is "mixed", it accepts everything
	if childHint.Type() == phpv.ZtMixed {
		return true
	}
	// If parent is "mixed", child must also be mixed to be compatible
	if parentHint.Type() == phpv.ZtMixed {
		return childHint.Type() == phpv.ZtMixed
	}

	// Check nullable compatibility: if parent accepts null, child must also accept null
	if parentHint.IsNullable() && !childHint.IsNullable() {
		// Parent accepts null but child doesn't — narrowing
		return false
	}

	// Gather all "leaf" types from parent (flatten unions)
	parentTypes := flattenTypeHint(parentHint)

	// For each type the parent accepts, the child must also accept it
	for _, pt := range parentTypes {
		if !typeHintContains(ctx, childHint, pt) {
			return false
		}
	}
	return true
}

// flattenTypeHint returns all the individual type hints in a (possibly union) type.
func flattenTypeHint(h *phpv.TypeHint) []*phpv.TypeHint {
	if len(h.Union) > 0 {
		var result []*phpv.TypeHint
		for _, u := range h.Union {
			result = append(result, flattenTypeHint(u)...)
		}
		return result
	}
	return []*phpv.TypeHint{h}
}

// typeHintContains checks if a type hint (possibly union) contains a specific single type.
func typeHintContains(ctx phpv.Context, h *phpv.TypeHint, target *phpv.TypeHint) bool {
	if len(h.Union) > 0 {
		for _, u := range h.Union {
			if typeHintContains(ctx, u, target) {
				return true
			}
		}
		return false
	}
	if h.Type() == phpv.ZtMixed {
		return true
	}
	// Compare single types
	if h.Type() != target.Type() {
		return false
	}
	// For object types, check class name
	if h.Type() == phpv.ZtObject {
		if h.ClassName() == "" || target.ClassName() == "" {
			// "object" matches any object type
			return h.ClassName() == "" || target.ClassName() == ""
		}
		// Direct name comparison first
		if h.ClassName().ToLower() == target.ClassName().ToLower() {
			return true
		}
		// Check if the class names resolve to the same class (handles class_alias)
		if ctx != nil {
			hClass, err1 := ctx.Global().GetClass(ctx, h.ClassName(), false)
			tClass, err2 := ctx.Global().GetClass(ctx, target.ClassName(), false)
			if err1 == nil && err2 == nil && !phpv.IsNilClass(hClass) && !phpv.IsNilClass(tClass) {
				// Same class object means they are aliases of each other
				if hClass == tClass {
					return true
				}
				// Also check instanceof relationship for inheritance
				if hClass.InstanceOf(tClass) || tClass.InstanceOf(hClass) {
					return true
				}
			}
		}
		return false
	}
	// For bool with "true"/"false" specifiers
	if h.Type() == phpv.ZtBool {
		return h.ClassName() == target.ClassName()
	}
	return true
}

// magicParamTypeCompatible checks if a type hint is compatible with the required type
// for a magic method parameter. The type hint must be exactly the required type, or a
// union/nullable type that includes the required type.
func magicParamTypeCompatible(hint *phpv.TypeHint, requiredType string) bool {
	// Union types: check if any alternative is the required type
	if len(hint.Union) > 0 {
		for _, alt := range hint.Union {
			if magicParamTypeCompatible(alt, requiredType) {
				return true
			}
		}
		return false
	}
	// Nullable: the base type must match
	switch requiredType {
	case "string":
		return hint.Type() == phpv.ZtString || hint.Type() == phpv.ZtMixed
	case "array":
		return hint.Type() == phpv.ZtArray || hint.Type() == phpv.ZtMixed
	}
	return true
}

func pluralS(n int) string {
	if n == 1 {
		return ""
	}
	return "s"
}

// fatalError writes a fatal error to the output buffer and returns an exit error
// so execution stops but the error message is properly formatted in PHP style.
func (c *ZClass) fatalError(ctx phpv.Context, msg string) error {
	return c.fatalErrorAt(ctx, msg, c.L)
}

func (c *ZClass) fatalErrorAt(ctx phpv.Context, msg string, loc *phpv.Loc) error {
	phpErr := &phpv.PhpError{
		Err:  fmt.Errorf("%s", msg),
		Code: phpv.E_ERROR,
		Loc:  loc,
	}
	ctx.Global().LogError(phpErr)
	return phpv.ExitError(255)
}

func (c *ZClass) InstanceOf(parentClass phpv.ZClass) bool {
	if c == nil || parentClass == nil {
		return false
	}
	if parentClass == c {
		return true
	}
	pc, ok := parentClass.(*ZClass)
	if !ok {
		return false
	}
	if c.parents != nil {
		if _, ok := c.parents[pc]; ok {
			return true
		}
	}
	// Check implementations (interfaces)
	for _, impl := range c.Implementations {
		if impl == pc {
			return true
		}
		if impl.InstanceOf(parentClass) {
			return true
		}
	}
	// Also walk the Extends chain (for built-in classes that aren't Compile'd)
	if c.Extends != nil {
		return c.Extends.InstanceOf(parentClass)
	}
	return false
}

func (c *ZClass) Implements(class phpv.ZClass) bool {
	return c.implementsWithGuard(class, make(map[phpv.ZClass]bool))
}

func (c *ZClass) implementsWithGuard(class phpv.ZClass, seen map[phpv.ZClass]bool) bool {
	if c == class {
		return true
	}
	if seen[c] {
		return false // cycle detected
	}
	seen[c] = true
	for _, intf := range c.Implementations {
		if intf.implementsWithGuard(class, seen) {
			return true
		}
	}
	if c.Extends != nil {
		return c.Extends.implementsWithGuard(class, seen)
	}
	return false
}

func (c *ZClass) Dump(w io.Writer) error {
	_, err := fmt.Fprintf(w, "%sclass %s {", c.Attr, c.Name)
	if err != nil {
		return err
	}
	// TODO
	_, err = fmt.Fprintf(w, "TODO }")
	return err
}

func (c *ZClass) BaseName() phpv.ZString {
	// rturn class name without namespaces/etc
	pos := strings.LastIndexByte(string(c.Name), '\\')
	if pos == -1 {
		return c.Name
	}
	return c.Name[pos+1:]
}

func (c *ZClass) GetStaticProps(ctx phpv.Context) (*phpv.ZHashTable, error) {
	if c.StaticProps == nil {
		c.StaticProps = phpv.NewHashTable()
		// Set compiling class for self::/parent:: resolution in property defaults
		ctx.Global().SetCompilingClass(c)
		defer ctx.Global().SetCompilingClass(nil)
		for _, p := range c.Props {
			if !p.Modifiers.IsStatic() {
				continue
			}
			if p.Default == nil {
				c.StaticProps.SetString(p.VarName, phpv.ZNULL.ZVal())
				continue
			}
			// Resolve CompileDelayed defaults lazily
			if cd, ok := p.Default.(*phpv.CompileDelayed); ok {
				z, err := cd.Run(ctx)
				if err != nil {
					return nil, err
				}
				p.Default = z.Value()
			}
			c.StaticProps.SetString(p.VarName, p.Default.ZVal())
		}
	}
	return c.StaticProps, nil
}

// FindStaticProp looks up a static property by name in the class hierarchy,
// walking up through parent classes if the property is not found locally.
// Returns the hash table containing the property and true if found.
func (c *ZClass) FindStaticProp(ctx phpv.Context, name phpv.ZString) (*phpv.ZHashTable, bool, error) {
	for cur := c; cur != nil; cur = cur.Extends {
		p, err := cur.GetStaticProps(ctx)
		if err != nil {
			return nil, false, err
		}
		if p.HasString(name) {
			return p, true, nil
		}
	}
	return nil, false, nil
}

// IsStaticPropAccessible checks whether the calling context has visibility
// access to a static property on the given class. Returns true when the
// property is public, when no declaration is found (the caller will handle
// the "undeclared" error separately), or when the caller's class satisfies
// the private/protected rules.
func IsStaticPropAccessible(ctx phpv.Context, c *ZClass, name phpv.ZString) bool {
	for cur := c; cur != nil; cur = cur.Extends {
		for _, p := range cur.Props {
			if p.VarName == name && p.Modifiers.IsStatic() {
				if p.Modifiers.IsPrivate() {
					callerClass := ctx.Class()
					if callerClass == nil || callerClass.GetName() != cur.GetName() {
						return false
					}
				} else if p.Modifiers.IsProtected() {
					callerClass := ctx.Class()
					if callerClass == nil || (!callerClass.InstanceOf(cur) && !cur.InstanceOf(callerClass)) {
						return false
					}
				}
				return true
			}
		}
	}
	return true
}

// ResolveConstants resolves any remaining CompileDelayed constants in the class
// and its parent classes. Called when the class is first instantiated.
func (c *ZClass) ResolveConstants(ctx phpv.Context) error {
	for cur := c; cur != nil; cur = cur.Extends {
		ctx.Global().SetCompilingClass(cur)
		for _, k := range cur.ConstOrder {
			cc := cur.Const[k]
			if cc == nil {
				continue
			}
			if r, ok := cc.Value.(*phpv.CompileDelayed); ok {
				z, err := r.Run(ctx)
				if err != nil {
					ctx.Global().SetCompilingClass(nil)
					// Add a synthetic [constant expression] frame to the stack trace
					// to match PHP's behavior when constant expression evaluation fails.
					if ex, ok := err.(*phperr.PhpThrow); ok {
						AddConstantExpressionFrame(ex, ctx)
					}
					return err
				}
				cur.Const[k].Value = z.Value()
			}
		}
	}
	ctx.Global().SetCompilingClass(nil)
	return nil
}

// AddConstantExpressionFrame prepends a [constant expression]() frame to an
// exception's stack trace, matching PHP's behavior for errors during class
// constant expression evaluation.
func AddConstantExpressionFrame(ex *phperr.PhpThrow, ctx phpv.Context) {
	// Get caller location for the frame
	loc := ctx.Loc()
	filename := ""
	line := 0
	if loc != nil {
		filename = loc.Filename
		line = loc.Line
	}

	syntheticFrame := &phpv.StackTraceEntry{
		FuncName: "[constant expression]",
		Filename: filename,
		Line:     line,
	}

	// Walk the class hierarchy to find the trace
	cls := ex.Obj.GetClass()
	for cls != nil {
		if opaque := ex.Obj.GetOpaque(cls); opaque != nil {
			if trace, ok := opaque.([]*phpv.StackTraceEntry); ok {
				newTrace := make([]*phpv.StackTraceEntry, 0, len(trace)+1)
				newTrace = append(newTrace, syntheticFrame)
				newTrace = append(newTrace, trace...)
				ex.Obj.SetOpaque(cls, newTrace)
				// Also update Exception opaque if different
				if cls != Exception {
					ex.Obj.SetOpaque(Exception, newTrace)
				}
				return
			}
		}
		cls = cls.GetParent()
	}

	// No existing trace found (e.g., error at global scope with nil trace).
	// Create a new trace with just the synthetic frame.
	newTrace := []*phpv.StackTraceEntry{syntheticFrame}
	ex.Obj.SetOpaque(Exception, newTrace)
	ex.Obj.SetOpaque(ex.Obj.GetClass(), newTrace)
}

func (c *ZClass) GetProp(name phpv.ZString) (*phpv.ZClassProp, bool) {
	// Handle mangled private property names: \0ClassName\0propName
	if len(name) > 0 && name[0] == 0 {
		// Extract class name and property name
		end := strings.IndexByte(string(name[1:]), 0)
		if end > 0 {
			className := name[1 : end+1]
			propName := name[end+2:]
			// Find the class and look for the private property
			for cur := c; cur != nil; cur = cur.Extends {
				if cur.Name == className {
					for _, prop := range cur.Props {
						if prop.VarName == propName {
							return prop, true
						}
					}
				}
			}
		}
		return nil, false
	}
	// Walk class hierarchy for unmangled names
	for cur := c; cur != nil; cur = cur.Extends {
		for _, prop := range cur.Props {
			if prop.VarName == name {
				return prop, true
			}
		}
	}
	return nil, false
}

func (c *ZClass) GetMethod(name phpv.ZString) (*phpv.ZClassMethod, bool) {
	name = name.ToLower()
	r, ok := c.Methods[name]
	return r, ok
}

func (c *ZClass) GetMethods() map[phpv.ZString]*phpv.ZClassMethod {
	return c.Methods
}

// GetMethodsOrdered returns methods in PHP declaration order:
// the class's own methods first (in MethodOrder), then inherited methods
// from parent, grandparent, etc.
func (c *ZClass) GetMethodsOrdered() []*phpv.ZClassMethod {
	var result []*phpv.ZClassMethod
	seen := make(map[phpv.ZString]bool)

	// Walk from this class up to ancestors
	for cur := c; cur != nil; cur = cur.Extends {
		// Use MethodOrder if available (compilation order)
		if len(cur.MethodOrder) > 0 {
			for _, name := range cur.MethodOrder {
				if seen[name] {
					continue
				}
				if m, ok := cur.Methods[name]; ok {
					// Only include if this class actually declares this method
					if m.Class == nil || m.Class.GetName() == cur.GetName() {
						seen[name] = true
						result = append(result, m)
					}
				}
			}
		}
		// Also check methods not in MethodOrder (e.g., builtin methods, trait methods)
		for name, m := range cur.Methods {
			if seen[name] {
				continue
			}
			if m.Class == nil || m.Class.GetName() == cur.GetName() {
				seen[name] = true
				result = append(result, m)
			}
		}
	}
	return result
}

func (c *ZClass) GetType() phpv.ZClassType {
	return c.Type
}

func (c *ZClass) Handlers() *phpv.ZClassHandlers {
	return c.H
}

func (c *ZClass) GetParent() phpv.ZClass {
	if c.Extends == nil {
		return nil
	}
	return c.Extends
}

// visibilityLevel returns 0 for public, 1 for protected, 2 for private
func visibilityLevel(m phpv.ZObjectAttr) int {
	if m.IsPrivate() {
		return 2
	}
	if m.IsProtected() {
		return 1
	}
	return 0
}

func (c *ZClass) NextInstanceID() int {
	c.nextIntanceID++
	id := c.nextIntanceID
	return id
}
