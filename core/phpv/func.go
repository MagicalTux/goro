package phpv

type FuncArg struct {
	VarName      ZString
	Ref          bool
	Required     bool
	DefaultValue Val
	Hint         *TypeHint
}

type FuncUse struct {
	VarName ZString
	Value   *ZVal
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
