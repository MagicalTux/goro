package phpv

import "io"

type ZArrayAccess interface {
	OffsetGet(ctx Context, key Val) (*ZVal, error)
	OffsetSet(ctx Context, key Val, value *ZVal) error
	OffsetUnset(ctx Context, key Val) error
	OffsetExists(ctx Context, key Val) (bool, error)
}

type ZIterable interface {
	NewIterator() ZIterator
}

type ZIterator interface {
	Current(ctx Context) (*ZVal, error)
	Key(ctx Context) (*ZVal, error)
	Next(ctx Context) error
	Rewind(ctx Context) error
	Valid(ctx Context) bool
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
	Call(ctx Context, args []*ZVal) (*ZVal, error)
}

type ObjectCallable interface {
	GetMethod(method ZString, ctx Context, args []*ZVal) (*ZVal, error)
}
