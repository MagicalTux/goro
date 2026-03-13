package phpv

import (
	"io"
	"iter"
)

type ZArrayAccess interface {
	OffsetGet(ctx Context, key Val) (*ZVal, error)
	OffsetSet(ctx Context, key Val, value *ZVal) error
	OffsetUnset(ctx Context, key Val) error
	OffsetExists(ctx Context, key Val) (bool, error)
	OffsetCheck(ctx Context, key Val) (*ZVal, bool, error)
}

type ZIterable interface {
	NewIterator() ZIterator
}

type ZIterator interface {
	Current(ctx Context) (*ZVal, error)
	Key(ctx Context) (*ZVal, error)
	Next(ctx Context) (*ZVal, error)
	Prev(ctx Context) (*ZVal, error)
	Reset(ctx Context) (*ZVal, error)
	ResetIfEnd(ctx Context) (*ZVal, error)
	End(ctx Context) (*ZVal, error)
	Valid(ctx Context) bool
	Iterate(ctx Context) iter.Seq2[*ZVal, *ZVal]
}

type ZCountable interface {
	Count(ctx Context) ZInt
}

type Runnable interface {
	Run(Context) (*ZVal, error)
	Dump(io.Writer) error
}

type RunnableChild interface {
	Runnable
	GetParentNode() Runnable
	SetParentNode(Runnable)
}

type Writable interface {
	WriteValue(ctx Context, value *ZVal) error
}

// WritePreparable is implemented by Writable types that have sub-expressions
// (e.g. array indices, variable-variable names) which need to be evaluated
// before the RHS of an assignment. This ensures correct PHP evaluation order
// where LHS side effects happen before RHS evaluation.
type WritePreparable interface {
	PrepareWrite(ctx Context) error
}

type Callable interface {
	Val
	Name() string
	Call(ctx Context, args []*ZVal) (*ZVal, error)
}

type Cloneable interface {
	Clone() any
}
