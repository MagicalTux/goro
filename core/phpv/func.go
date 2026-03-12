package phpv

type FuncArg struct {
	VarName            ZString
	Ref                bool
	Required           bool
	DefaultValue       Val
	Hint               *TypeHint
	Promotion          ZObjectAttr // Non-zero if this is a constructor promoted property
	ImplicitlyNullable bool        // type hint + NULL default without explicit ?
}

type FuncUse struct {
	VarName ZString
	Value   *ZVal
	Ref     bool
}

type FuncGetArgs interface {
	GetArgs() []*FuncArg
}

type ZClosure interface {
	FuncGetArgs
	Callable
	Runnable

	GetClass() ZClass
}
