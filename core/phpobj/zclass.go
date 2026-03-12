package phpobj

import (
	"fmt"
	"io"
	"strings"

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
	Props           []*phpv.ZClassProp
	Methods         map[phpv.ZString]*phpv.ZClassMethod
	StaticProps     *phpv.ZHashTable

	nextIntanceID int

	// class specific handlers
	H *phpv.ZClassHandlers
}

func (c *ZClass) GetName() phpv.ZString {
	if c == nil {
		return ""
	}
	return c.Name
}

func (c *ZClass) Run(ctx phpv.Context) (*phpv.ZVal, error) {
	err := ctx.Global().RegisterClass(c.Name, c)
	if err != nil {
		return nil, err
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

		// Check if trying to extend an interface (must use implements instead)
		if c.Type != phpv.ZClassTypeInterface && c.Extends.Type == phpv.ZClassTypeInterface {
			return c.fatalError(ctx, fmt.Sprintf("Class %s cannot extend interface %s", c.Name, c.Extends.Name))
		}

		// Check if parent class is final
		if c.Extends.Attr.Has(phpv.ZClassFinal) {
			return c.fatalError(ctx, fmt.Sprintf("Class %s cannot extend final class %s", c.Name, c.Extends.Name))
		}

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

					// Check method signature compatibility (skip constructors)
					if n != "__construct" {
						if err := c.checkMethodCompatibility(ctx, ours, m); err != nil {
							return err
						}
					}
				}
			} else {
				c.Methods[n] = m
			}
		}

		// Check __construct compatibility against interface declarations inherited from parents
		if ours, gotit := c.Methods["__construct"]; gotit && (ours.Class == nil || ours.Class == c) {
			// Walk the parent chain and check interface constructors
			for p := c.Extends; p != nil; p = p.Extends {
				for _, intf := range p.Implementations {
					if intfCtor, hasCtor := intf.Methods["__construct"]; hasCtor {
						if err := c.checkMethodCompatibility(ctx, ours, intfCtor); err != nil {
							return err
						}
					}
				}
			}
		}

		// Inherit constants from parent (skip private ones)
		for k, v := range c.Extends.Const {
			if v.Modifiers.IsPrivate() {
				continue
			}
			if childConst, exists := c.Const[k]; exists {
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

	// PHP requires that classes implementing Traversable must do so through
	// either Iterator or IteratorAggregate, not directly.
	if c.Type != phpv.ZClassTypeInterface {
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
	}

	for k, cc := range c.Const {
		if r, ok := cc.Value.(*phpv.CompileDelayed); ok {
			z, err := r.Run(ctx)
			if err != nil {
				return err
			}
			c.Const[k].Value = z.Value()
		}
	}
	for _, p := range c.Props {
		if r, ok := p.Default.(*phpv.CompileDelayed); ok {
			z, err := r.Run(ctx)
			if err != nil {
				return err
			}
			p.Default = z.Value()
		}
	}
	// Check interface properties
	if c.Type == phpv.ZClassTypeInterface && len(c.Props) > 0 {
		return c.fatalError(ctx, fmt.Sprintf("Interfaces may only include hooked properties"))
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
		// Warn if magic methods are explicitly non-public (only for methods declared in this class)
		if (m.Modifiers.Has(phpv.ZAttrProtected) || m.Modifiers.Has(phpv.ZAttrPrivate)) && c.Type != phpv.ZClassTypeInterface && (m.Class == nil || m.Class == c) {
			switch m.Name.ToLower() {
			case "__call", "__callstatic", "__get", "__set", "__isset", "__unset",
				"__tostring", "__debuginfo", "__serialize", "__unserialize":
				phpErr := &phpv.PhpError{
					Err:  fmt.Errorf("The magic method %s::%s() must have public visibility", c.Name, m.Name),
					Code: phpv.E_WARNING,
					Loc:  m.Loc,
				}
				ctx.Global().LogError(phpErr)
			}
		}
		if m.Modifiers.Has(phpv.ZAttrAbstract) && m.Modifiers.Has(phpv.ZAttrFinal) {
			return c.fatalError(ctx, "Cannot use the final modifier on an abstract method")
		}
		// Warn about final private methods (they can never be overridden)
		if m.Modifiers.Has(phpv.ZAttrFinal) && m.Modifiers.Has(phpv.ZAttrPrivate) && (m.Class == nil || m.Class == c) && m.Name.ToLower() != "__construct" {
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
				// Check constructor compatibility for interface implementations
				if n == "__construct" {
					if err := c.checkMethodCompatibility(ctx, ours, m); err != nil {
						return err
					}
				}
			}
		}
		for k, v := range intf.Const {
			if _, exists := c.Const[k]; !exists {
				c.Const[k] = v
			}
		}
	}

	// Validate: non-abstract, non-interface classes must implement all abstract methods
	if c.Type != phpv.ZClassTypeInterface && c.Attr&phpv.ZClassAttr(phpv.ZClassExplicitAbstract) == 0 {
		var ownAbstract []string   // abstract methods declared in this class
		var unimplemented []string // inherited abstract methods not implemented
		for _, m := range c.Methods {
			isAbstract := m.Empty && (m.Modifiers.Has(phpv.ZAttrAbstract) || (m.Class != nil && m.Class.GetType() == phpv.ZClassTypeInterface))
			if isAbstract {
				if m.Class == nil || m.Class == c {
					// Declared in this class
					ownAbstract = append(ownAbstract, string(m.Name))
				} else {
					// Inherited from parent/interface
					unimplemented = append(unimplemented, string(m.Class.GetName())+"::"+string(m.Name))
				}
			}
		}
		if len(ownAbstract) > 0 {
			// PHP: "Class X declares abstract method Y() and must therefore be declared abstract"
			return c.fatalError(ctx, fmt.Sprintf("Class %s declares abstract method %s() and must therefore be declared abstract", c.Name, ownAbstract[0]))
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

	return nil
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

	// Child cannot require more parameters than parent
	// Child cannot have fewer total parameters than parent
	incompatible := false
	if childRequired > len(parentArgs) {
		incompatible = true
	}
	if len(childArgs) < len(parentArgs) {
		incompatible = true
	}

	// Check type hint compatibility for each parameter
	if !incompatible {
		for i := 0; i < len(parentArgs) && i < len(childArgs); i++ {
			ph := parentArgs[i].Hint
			ch := childArgs[i].Hint
			if ph == nil && ch == nil {
				continue
			}
			if ph != nil && ch == nil {
				// Child drops type hint - incompatible
				incompatible = true
				break
			}
			if ph == nil && ch != nil {
				// Child adds type hint - incompatible
				incompatible = true
				break
			}
			// Both have hints - check they match
			if ph.Type() != ch.Type() || ph.ClassName() != ch.ClassName() {
				incompatible = true
				break
			}
		}
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
	sig := string(className) + "::" + string(m.Name) + "("
	if fga, ok := m.Method.(phpv.FuncGetArgs); ok {
		args := fga.GetArgs()
		for i, a := range args {
			if i > 0 {
				sig += ", "
			}
			if a.Hint != nil {
				sig += a.Hint.String() + " "
			}
			sig += "$" + string(a.VarName)
			if !a.Required {
				sig += " = " + formatDefault(a.DefaultValue)
			}
		}
	}
	sig += ")"
	return sig
}

func formatDefault(v phpv.Val) string {
	if v == nil {
		return "NULL"
	}
	switch vt := v.(type) {
	case phpv.ZInt:
		return fmt.Sprintf("%d", vt)
	case phpv.ZString:
		return fmt.Sprintf("'%s'", vt)
	case phpv.ZFloat:
		return fmt.Sprintf("%g", vt)
	case phpv.ZBool:
		if vt {
			return "true"
		}
		return "false"
	default:
		return fmt.Sprintf("%v", v)
	}
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
	}

	for name, requiredArgs := range magicArgCounts {
		m, ok := c.Methods[name]
		if !ok {
			continue
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

	return nil
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
	for _, intf := range c.Implementations {
		if class == intf {
			return true
		}
	}
	if c.Extends != nil {
		return c.Extends.Implements(class)
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
		for _, p := range c.Props {
			if !p.Modifiers.IsStatic() {
				continue
			}
			if p.Default == nil {
				c.StaticProps.SetString(p.VarName, phpv.ZNULL.ZVal())
				continue
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

func (c *ZClass) GetProp(name phpv.ZString) (*phpv.ZClassProp, bool) {
	for _, prop := range c.Props {
		if prop.VarName == name {
			return prop, true
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
