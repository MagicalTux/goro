package phpv

import "iter"

// NewStdClassFunc is a factory for creating stdClass objects.
// It is set by the phpobj package during init to break the cyclic dependency.
var NewStdClassFunc func(ctx Context) (ZObject, error)

// ZAttribute represents a PHP attribute (e.g. #[MyAttribute(args...)]).
type ZAttribute struct {
	ClassName ZString    // fully qualified attribute class name
	Args      []*ZVal    // evaluated arguments (nil if no args)
	ArgExprs  []Runnable // unevaluated argument expressions for lazy evaluation (nil if fully resolved)
	Resolving bool       // true while lazy args are being resolved (prevents re-entrant deprecation)
}

type ZClassProp struct {
	VarName       ZString
	Default       Val
	Modifiers     ZObjectAttr
	SetModifiers  ZObjectAttr // PHP 8.4 asymmetric visibility: separate write visibility (0 = same as Modifiers)
	TypeHint      *TypeHint
	Attributes    []*ZAttribute // PHP 8.0 attributes

	// Property hooks (PHP 8.4)
	GetHook  Runnable // get { ... } hook body
	SetHook  Runnable // set($value) { ... } hook body
	SetParam ZString  // parameter name for set hook (default "$value")
	HasHooks bool     // true if property declared with hook syntax (even abstract)
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
}

type ZClassConst struct {
	Value      Val
	Modifiers  ZObjectAttr
	Resolving  bool           // true while the constant is being resolved (circular reference detection)
	Attributes []*ZAttribute  // PHP 8.0 attributes
}

type ZClassHandlers struct {
	Constructor  *ZClassMethod
	HandleInvoke func(ctx Context, o ZObject, args []Runnable) (*ZVal, error)
	HandleDecRef func(ctx Context, o ZObject) // called when object refcount is decremented during scope cleanup
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
