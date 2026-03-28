package phpv

import "iter"

// NewStdClassFunc is a factory for creating stdClass objects.
// It is set by the phpobj package during init to break the cyclic dependency.
var NewStdClassFunc func(ctx Context) (ZObject, error)

// ZAttribute represents a PHP attribute (e.g. #[MyAttribute(args...)]).
type ZAttribute struct {
	ClassName   ZString    // fully qualified attribute class name
	Args        []*ZVal    // evaluated arguments (nil if no args)
	ArgExprs    []Runnable // unevaluated argument expressions for lazy evaluation (nil if fully resolved)
	ArgNames    []ZString  // named argument names parallel to Args (empty string = positional)
	Resolving   bool       // true while lazy args are being resolved (prevents re-entrant deprecation)
	StrictTypes bool       // true if declare(strict_types=1) was in effect at the declaration site
}

type ZClassProp struct {
	VarName       ZString
	Default       Val
	Modifiers     ZObjectAttr
	SetModifiers  ZObjectAttr // PHP 8.4 asymmetric visibility: separate write visibility (0 = same as Modifiers)
	TypeHint      *TypeHint
	Attributes    []*ZAttribute // PHP 8.0 attributes

	// Property hooks (PHP 8.4)
	GetHook         Runnable // get { ... } hook body
	SetHook         Runnable // set($value) { ... } hook body
	SetParam        ZString  // parameter name for set hook (default "$value")
	HasHooks        bool     // true if property declared with hook syntax (even abstract)
	IsBacked        bool     // true if hooks reference $this->prop (backing store exists)
	GetIsAbstract   bool     // true if get hook is abstract (get;)
	SetIsAbstract   bool     // true if set hook is abstract (set;)
	GetIsFinal      bool     // true if get hook is declared final
	SetIsFinal      bool     // true if set hook is declared final
	HasGetDeclared   bool     // true if get hook was declared (even abstract)
	HasSetDeclared   bool     // true if set hook was declared (even abstract)
	SetParamHasType  bool     // true if set hook parameter has explicit type hint
}

// IsVirtual returns true if this property is virtual (has hooks but no backing store).
// A property is virtual if:
// - It has hooks, AND
// - It has no default value, AND
// - Its hooks never reference the backing store ($this->propName).
// The IsBacked flag is set at compile time by analyzing the hook bodies.
func (p *ZClassProp) IsVirtual() bool {
	if !p.HasHooks {
		return false
	}
	if p.Default != nil || p.IsBacked {
		return false
	}
	return true
}

// ZClassTraitUse represents a single "use TraitA, TraitB { ... }" statement in a class body.
type ZClassTraitUse struct {
	TraitNames []ZString
	Aliases    []ZClassTraitAlias
	Insteadof  []ZClassTraitInsteadof
}

// ZClassTraitAlias represents a trait alias or visibility change:
// "TraitName::method as [visibility] newname"
type ZClassTraitAlias struct {
	TraitName  ZString     // optional: the trait the method belongs to
	MethodName ZString     // original method name
	NewName    ZString     // alias name (empty = just visibility change)
	NewAttr    ZObjectAttr // new visibility (0 = unchanged)
}

// ZClassTraitInsteadof represents "TraitName::method insteadof OtherTrait [, OtherTrait2]"
type ZClassTraitInsteadof struct {
	TraitName    ZString   // the trait whose method wins
	MethodName   ZString   // the method name being resolved
	InsteadOf    []ZString // traits whose methods are excluded
}

type ZClassMethod struct {
	Name       ZString
	Modifiers  ZObjectAttr
	Method     Callable
	Class      ZClass
	Empty      bool
	Loc        *Loc
	Attributes []*ZAttribute // PHP 8.0 attributes
	FromTrait  ZClass        // non-nil if this method was imported from a trait
	Prototype  ZClass        // interface/class that defines the prototype for this method
	ReturnType *TypeHint     // return type hint for the method (for reflection)
}

type ZClassConst struct {
	Value      Val
	Modifiers  ZObjectAttr
	Resolving  bool           // true while the constant is being resolved (circular reference detection)
	Attributes []*ZAttribute  // PHP 8.0 attributes
	TypeHint   *TypeHint      // PHP 8.3 typed class constants
}

type ZClassHandlers struct {
	Constructor      *ZClassMethod
	HandleInvoke     func(ctx Context, o ZObject, args []Runnable) (*ZVal, error)
	HandleDecRef     func(ctx Context, o ZObject) // called when object refcount is decremented during scope cleanup
	HandleCastArray  func(ctx Context, o ZObject) (*ZArray, error) // override (array) cast
	HandleCompare    func(ctx Context, a, b ZObject) (int, error)  // override == comparison; return 0=equal, non-0=not-equal
	HandleCast       func(ctx Context, o ZObject, t ZType) (Val, error)          // override type casting (int, float, bool)
	HandleDoOperation func(ctx Context, op int, a, b *ZVal) (*ZVal, error)       // override arithmetic/bitwise operators; op is tokenizer.ItemType
	HandleForeachByRef func(ctx Context, o ZObject) (*ZArray, error)            // provide internal array for foreach by-reference (e.g., ArrayObject/ArrayIterator)
	// HandleIssetDim overrides isset($obj[$key]) behavior. Return true if the key exists
	// and the value is considered "set" (not null). Only called for isset(), not for direct
	// offsetExists() calls.
	HandleIssetDim func(ctx Context, o ZObject, key *ZVal) (bool, error)
	// HandlePropGet intercepts property read access before __get. Return (nil, nil) to fall through to normal handling.
	HandlePropGet   func(ctx Context, o ZObject, key ZString) (*ZVal, error)
	// HandlePropSet intercepts property write access before __set. Return false to fall through to normal handling.
	HandlePropSet   func(ctx Context, o ZObject, key ZString, value *ZVal) (bool, error)
	// HandlePropIsset intercepts isset($obj->prop) before __isset. Return (false, false, nil) to fall through.
	// First bool is the isset result, second bool indicates whether the handler handled it.
	HandlePropIsset func(ctx Context, o ZObject, key ZString) (bool, bool, error)
	// HandlePropUnset intercepts unset($obj->prop) before __unset. Return false to fall through.
	HandlePropUnset func(ctx Context, o ZObject, key ZString) (bool, error)
}

type ZClass interface {
	GetName() ZString
	InstanceOf(parent ZClass) bool
	Implements(intf ZClass) bool
	BaseName() ZString
	GetStaticProps(ctx Context) (*ZHashTable, error)
	GetProp(name ZString) (*ZClassProp, bool)
	GetMethod(name ZString) (*ZClassMethod, bool)
	GetMethods() map[ZString]*ZClassMethod
	GetType() ZClassType
	Handlers() *ZClassHandlers
	GetParent() ZClass
	NextInstanceID() int
}

type ZObjectAccess interface {
	ObjectGet(ctx Context, key Val) (*ZVal, error)
	ObjectSet(ctx Context, key Val, value *ZVal) error
}

type ZObject interface {
	ZObjectAccess
	Val

	GetOpaque(c ZClass) interface{}
	SetOpaque(c ZClass, v interface{})
	GetClass() ZClass
	NewIterator() ZIterator
	HashTable() *ZHashTable
	Clone(ctx Context) (ZObject, error)
	GetParent() ZObject
	GetKin(className string) ZObject
	IterProps(ctx Context) iter.Seq[*ZClassProp]

	// IncrJsonApplyCount increments the json_encode recursion guard counter.
	// Returns the count before incrementing; if > 0 the object is already
	// being json-encoded.
	IncrJsonApplyCount() int32
	// DecrJsonApplyCount decrements the json_encode recursion guard counter.
	DecrJsonApplyCount()

	// IncrSerializeApplyCount increments the serialize recursion guard counter.
	// Returns the count before incrementing; if > 0 the object is already
	// being serialized.
	IncrSerializeApplyCount() int32
	// DecrSerializeApplyCount decrements the serialize recursion guard counter.
	DecrSerializeApplyCount()
}
