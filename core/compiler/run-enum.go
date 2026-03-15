package compiler

import (
	"fmt"
	"io"

	"github.com/MagicalTux/goro/core/phpobj"
	"github.com/MagicalTux/goro/core/phpv"
)

// runEnumCaseInit initializes an enum case constant. It creates the singleton
// enum object the first time it's evaluated and caches it.
type runEnumCaseInit struct {
	className    phpv.ZString
	caseName     phpv.ZString
	backingValue phpv.Runnable // nil for unit enums
	backingType  phpv.ZType
}

func (r *runEnumCaseInit) Dump(w io.Writer) error {
	_, err := fmt.Fprintf(w, "%s::%s", r.className, r.caseName)
	return err
}

func (r *runEnumCaseInit) Run(ctx phpv.Context) (*phpv.ZVal, error) {
	// Look up the enum class
	class, err := ctx.Global().GetClass(ctx, r.className, false)
	if err != nil {
		return nil, err
	}

	zc, ok := class.(*phpobj.ZClass)
	if !ok {
		return nil, fmt.Errorf("enum %s is not a ZClass", r.className)
	}

	// Create the enum case object using the special enum constructor
	// that avoids init() to prevent infinite recursion (since enum cases
	// are stored as class constants, and init resolves constants).
	obj := phpobj.NewZObjectEnum(ctx, zc)

	// Set the name property
	obj.HashTable().SetString("name", phpv.ZString(r.caseName).ZVal())

	// Set the value property for backed enums
	if r.backingValue != nil {
		val, err := r.backingValue.Run(ctx)
		if err != nil {
			return nil, err
		}
		// Validate that the value type matches the backing type
		if val.GetType() != r.backingType {
			return nil, phpobj.ThrowError(ctx, phpobj.TypeError,
				fmt.Sprintf("Enum case type %s does not match enum backing type %s",
					val.GetType().TypeName(), r.backingType.TypeName()))
		}
		obj.HashTable().SetString("value", val)

		// Check for duplicate backing values across all already-initialized cases
		for _, otherCaseName := range zc.EnumCases {
			if otherCaseName == r.caseName {
				continue // skip self
			}
			cc, exists := zc.Const[otherCaseName]
			if !exists {
				continue
			}
			// Only check already-resolved cases (not CompileDelayed ones)
			otherObj, ok := cc.Value.(*phpobj.ZObject)
			if !ok {
				continue
			}
			otherVal := otherObj.HashTable().GetString("value")
			if otherVal == nil {
				continue
			}
			eq, eqErr := phpv.StrictEquals(ctx, val, otherVal)
			if eqErr != nil {
				continue
			}
			if eq {
				return nil, phpobj.ThrowError(ctx, phpobj.ValueError,
					fmt.Sprintf("Duplicate value in enum %s for cases %s and %s", r.className, otherCaseName, r.caseName))
			}
		}
	}

	return obj.ZVal(), nil
}

// runEnumRegister wraps a ZClass (enum) to add enum-specific validation
// during the Compile phase. It intercepts error messages to use "Enum"
// instead of "Class" where appropriate.
type runEnumRegister struct {
	class *phpobj.ZClass
}

func (r *runEnumRegister) Dump(w io.Writer) error {
	return r.class.Dump(w)
}

func (r *runEnumRegister) Run(ctx phpv.Context) (*phpv.ZVal, error) {
	c := r.class

	// Register the class first
	err := ctx.Global().RegisterClass(c.Name, c)
	if err != nil {
		return nil, r.enumFatalError(ctx, err.Error())
	}

	// Do enum-specific pre-validation before Compile runs.
	// This ensures enum-specific error messages are emitted instead of generic
	// "Class X ..." messages from zclass.go's Compile method.
	if err := r.preCompileValidation(ctx); err != nil {
		ctx.Global().UnregisterClass(c.Name)
		return nil, err
	}

	// Run the standard Compile
	err = c.Compile(ctx)
	if err != nil {
		ctx.Global().UnregisterClass(c.Name)
		return nil, err
	}

	// Post-compile validation for enum-specific rules that need resolved interfaces
	if err := r.postCompileValidation(ctx); err != nil {
		ctx.Global().UnregisterClass(c.Name)
		return nil, err
	}

	return nil, nil
}

// preCompileValidation performs enum-specific checks before the generic Compile.
// These checks need interfaces to be resolved, so we do them here at runtime.
func (r *runEnumRegister) preCompileValidation(ctx phpv.Context) error {
	c := r.class

	// Check each user-specified interface for enum-specific restrictions.
	// We need to resolve the interface classes to check their properties.
	for _, impl := range c.ImplementsStr {
		implLower := impl.ToLower()

		// Skip auto-added UnitEnum and BackedEnum (already validated at compile time)
		if implLower == "unitenum" || implLower == "backedenum" {
			continue
		}

		// Try to resolve the interface
		intf, err := ctx.Global().GetClass(ctx, impl, true)
		if err != nil {
			// Interface not found - let Compile handle this
			continue
		}
		intfClass, ok := intf.(*phpobj.ZClass)
		if !ok {
			continue
		}

		// Check for Throwable
		if intfClass == phpobj.Throwable || intfClass.Implements(phpobj.Throwable) {
			return r.enumFatalError(ctx, fmt.Sprintf("Enum %s cannot implement interface %s", c.Name, intfClass.GetName()))
		}

		// Check for Serializable (direct or indirect)
		if intfClass == phpobj.Serializable || intfClass.Implements(phpobj.Serializable) {
			// Emit the Serializable deprecation warning first
			phpErr := &phpv.PhpError{
				Err:  fmt.Errorf("%s implements the Serializable interface, which is deprecated. Implement __serialize() and __unserialize() instead (or in addition, if support for old PHP versions is necessary)", c.Name),
				Code: phpv.E_DEPRECATED,
				Loc:  c.L,
			}
			ctx.Global().LogError(phpErr)
			return r.enumFatalError(ctx, fmt.Sprintf("Enum %s cannot implement the Serializable interface", c.Name))
		}
	}

	// Pre-resolve interfaces to check for Traversable, duplicate implementations,
	// and unimplemented abstract methods before the generic Compile runs.
	resolvedInterfaces := make(map[*phpobj.ZClass]bool)
	implementsTraversable := false
	implementsIteratorOrAggregate := false
	var allResolvedInterfaces []*phpobj.ZClass

	for _, impl := range c.ImplementsStr {
		implLower := impl.ToLower()
		if implLower == "unitenum" || implLower == "backedenum" {
			continue
		}

		intf, err := ctx.Global().GetClass(ctx, impl, true)
		if err != nil {
			continue // let Compile handle not-found errors
		}
		intfClass, ok := intf.(*phpobj.ZClass)
		if !ok {
			continue
		}

		// Check for duplicate interface implementations
		if resolvedInterfaces[intfClass] {
			return r.enumFatalError(ctx, fmt.Sprintf("Enum %s cannot implement previously implemented interface %s", c.Name, intfClass.GetName()))
		}
		resolvedInterfaces[intfClass] = true
		allResolvedInterfaces = append(allResolvedInterfaces, intfClass)

		// Track Traversable, Iterator, IteratorAggregate
		if intfClass == phpobj.Traversable {
			implementsTraversable = true
		}
		if intfClass == phpobj.Iterator {
			implementsIteratorOrAggregate = true
		}
		if intfClass == phpobj.IteratorAggregate {
			implementsIteratorOrAggregate = true
		}
		// Check transitive implementations
		if intfClass.Implements(phpobj.Traversable) {
			implementsTraversable = true
		}
		if intfClass.Implements(phpobj.Iterator) || intfClass.Implements(phpobj.IteratorAggregate) {
			implementsIteratorOrAggregate = true
		}
	}

	// Check Traversable constraint for enums
	// PHP reports this error at "Unknown on line 0" (no source location)
	if implementsTraversable && !implementsIteratorOrAggregate {
		return r.enumFatalErrorAt(ctx, fmt.Sprintf("Enum %s must implement interface Traversable as part of either Iterator or IteratorAggregate", c.Name), &phpv.Loc{Filename: "Unknown", Line: 0})
	}

	// Check for unimplemented abstract methods from interfaces.
	// Enums cannot be abstract, so any unimplemented interface methods are an error.
	var unimplemented []string
	for _, intfClass := range allResolvedInterfaces {
		for methodName, m := range intfClass.Methods {
			// Interface methods are implicitly abstract
			isAbstract := m.Empty || m.Modifiers.Has(phpv.ZAttrAbstract) ||
				(m.Class != nil && m.Class.GetType() == phpv.ZClassTypeInterface)
			if !isAbstract {
				continue
			}
			// Check if the enum has a concrete implementation
			if ours, gotit := c.Methods[methodName]; gotit {
				if !ours.Empty {
					continue // enum has a concrete implementation
				}
			}
			// This method is not implemented
			className := string(intfClass.GetName())
			unimplemented = append(unimplemented, className+"::"+string(m.Name))
		}
	}
	if len(unimplemented) > 0 {
		msg := fmt.Sprintf("Enum %s must implement %d abstract method", c.Name, len(unimplemented))
		if len(unimplemented) > 1 {
			msg += "s"
		}
		msg += " ("
		for i, u := range unimplemented {
			if i > 0 {
				msg += ", "
			}
			msg += u
		}
		msg += ")"
		return r.enumFatalError(ctx, msg)
	}

	// Check for ambiguous constants from multiple interfaces
	seenConsts := make(map[phpv.ZString]phpv.ZString) // const name -> first interface name
	for _, intfClass := range allResolvedInterfaces {
		for constName := range intfClass.Const {
			// Skip if the enum itself defines this constant
			if _, ownExists := c.Const[constName]; ownExists {
				continue
			}
			if firstIntf, exists := seenConsts[constName]; exists {
				return r.enumFatalError(ctx, fmt.Sprintf("Enum %s inherits both %s::%s and %s::%s, which is ambiguous",
					c.Name, firstIntf, constName, intfClass.GetName(), constName))
			}
			seenConsts[constName] = intfClass.GetName()
		}
	}

	return nil
}

// postCompileValidation performs enum-specific checks after Compile.
// At this point, interfaces are resolved and methods imported.
func (r *runEnumRegister) postCompileValidation(ctx phpv.Context) error {
	c := r.class

	// Check for traits with properties
	for _, traitUse := range c.TraitUses {
		for _, traitName := range traitUse.TraitNames {
			traitClass, err := ctx.Global().GetClass(ctx, traitName, false)
			if err != nil {
				continue
			}
			tc, ok := traitClass.(*phpobj.ZClass)
			if !ok {
				continue
			}
			// Check for properties in trait
			if len(tc.Props) > 0 {
				return r.enumFatalError(ctx, fmt.Sprintf("Enum %s cannot include properties", c.Name))
			}
			// Check for forbidden methods in trait
			for _, m := range tc.Methods {
				if isEnumForbiddenMethod(m.Name.ToLower()) {
					return r.enumFatalError(ctx, fmt.Sprintf("Enum %s cannot include magic method %s", c.Name, m.Name))
				}
			}
		}
	}

	return nil
}

// enumFatalError emits a fatal error with enum-specific message formatting.
func (r *runEnumRegister) enumFatalError(ctx phpv.Context, msg string) error {
	return r.enumFatalErrorAt(ctx, msg, r.class.L)
}

// enumFatalErrorAt emits a fatal error at a specific location.
func (r *runEnumRegister) enumFatalErrorAt(ctx phpv.Context, msg string, loc *phpv.Loc) error {
	phpErr := &phpv.PhpError{
		Err:  fmt.Errorf("%s", msg),
		Code: phpv.E_ERROR,
		Loc:  loc,
	}
	ctx.Global().LogError(phpErr)
	return phpv.ExitError(255)
}
