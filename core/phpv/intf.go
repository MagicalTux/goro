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

type Writable interface {
	WriteValue(ctx Context, value *ZVal) error
}

type Callable interface {
	Val
	Name() string
	Call(ctx Context, args []*ZVal) (*ZVal, error)
}

type Cloneable interface {
	Clone() any
}
