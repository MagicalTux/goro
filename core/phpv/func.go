package phpv

type FuncArg struct {
	VarName            ZString
	Ref                bool
	Required           bool
	Variadic           bool        // ...param (collects remaining args into array)
	DefaultValue       Val
	Hint               *TypeHint
	Promotion          ZObjectAttr // Non-zero if this is a constructor promoted property
	SetPromotion       ZObjectAttr // PHP 8.4 asymmetric visibility for CPP (0 = same as Promotion)
	ImplicitlyNullable bool        // type hint + NULL default without explicit ?
	Attributes         []*ZAttribute // PHP 8.0 attributes
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
// a Notice rather than a Fatal Error.
type FuncCallExpression interface {
	IsFuncCallExpression()
}

type ZClosure interface {
	FuncGetArgs
	Callable
	Runnable

	GetClass() ZClass
	IsStatic() bool    // true for static function() {} and static fn() =>
	GetThis() ZObject  // the captured $this (nil for static closures / free functions)
}
