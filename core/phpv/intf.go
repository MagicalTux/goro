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

// NamedArgument is implemented by Runnable types that represent PHP 8.0 named arguments.
type NamedArgument interface {
	ArgName() ZString
	Inner() Runnable
}

// SpreadArgument is implemented by Runnable types that represent argument unpacking: func(...$arr)
type SpreadArgument interface {
	Inner() Runnable
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

// UndefinedChecker is implemented by Runnable types that represent simple
// variable accesses (e.g. $foo). It allows callers to check whether the
// variable is defined before evaluating the expression — useful for emitting
// "Undefined variable" warnings when passing undefined vars to functions.
type UndefinedChecker interface {
	IsUnDefined(ctx Context) bool
	VarName() ZString
}

// CompoundWritable is a marker interface for Writable types that represent
// compound expressions (array elements, object properties) rather than simple
// variables. These need special handling for by-ref parameter passing because
// the reference needs to be created and cleaned up explicitly.
type CompoundWritable interface {
	Writable
	IsCompoundWritable()
}

// ReadonlyRefChecker is implemented by Runnable types (like object property
// access) that can check whether creating a reference would violate readonly
// constraints. Used by the by-ref parameter passing code to throw
// "Cannot indirectly modify readonly property" before making a reference.
type ReadonlyRefChecker interface {
	CheckReadonlyRef(ctx Context) error
}

// WriteContextSetter is implemented by Runnable types (like array access
// expressions) that can be put into write context to suppress warnings
// during auto-vivification. Used when passing array elements by reference.
type WriteContextSetter interface {
	SetWriteContext(bool)
}
