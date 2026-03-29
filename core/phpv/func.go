package phpv

type FuncArg struct {
	VarName            ZString
	Ref                bool
	PreferRef          bool // ZEND_SEND_PREFER_REF: silently accepts non-ref values (no warning)
	Required           bool
	Variadic           bool        // ...param (collects remaining args into array)
	DefaultValue       Val
	Hint               *TypeHint
	Promotion          ZObjectAttr // Non-zero if this is a constructor promoted property
	SetPromotion       ZObjectAttr // PHP 8.4 asymmetric visibility for CPP (0 = same as Promotion)
	ImplicitlyNullable bool        // type hint + NULL default without explicit ?
	Attributes         []*ZAttribute // PHP 8.0 attributes
	Loc                *Loc // Source location of this parameter

	// Property hooks for promoted properties (PHP 8.4)
	PromotionHooks *ZClassProp // non-nil if promoted property has hooks
}

type FuncUse struct {
	VarName ZString
	Value   *ZVal
	Ref     bool
}

type FuncGetArgs interface {
	GetArgs() []*FuncArg
}

// FuncCallExpression is a marker interface for expressions that represent
// function/method calls. When passed to a by-reference parameter, these produce
// a Fatal Error in PHP 8+ ("could not be passed by reference").
type FuncCallExpression interface {
	IsFuncCallExpression()
}

// ParenthesizedExpression is a marker interface for parenthesized expressions.
// When passed to a by-reference parameter, these produce a Notice ("Only
// variables should be passed by reference") rather than a Fatal Error.
type ParenthesizedExpression interface {
	IsParenthesizedExpression()
}

// PreEvaluatedArg is a marker interface for pre-evaluated arguments (e.g.,
// those passed via call_user_func). When passed to a by-reference parameter,
// these produce a Warning ("FuncName(): Argument #N must be passed by reference,
// value given") rather than a Notice or Fatal Error.
type PreEvaluatedArg interface {
	IsPreEvaluatedArg()
}

type ZClosure interface {
	FuncGetArgs
	Callable
	Runnable

	GetClass() ZClass
	IsStatic() bool    // true for static function() {} and static fn() =>
	GetThis() ZObject  // the captured $this (nil for static closures / free functions)
}

// AttributeGetter is implemented by callables that have PHP attributes.
type AttributeGetter interface {
	GetAttributes() []*ZAttribute
}

// ClosureInstanceKeyProvider is implemented by closure instances that support
// per-instance static variable storage. The key uniquely identifies this
// specific closure instance (distinct from other closures defined in the same source).
type ClosureInstanceKeyProvider interface {
	ClosureInstanceKey() uintptr
}
