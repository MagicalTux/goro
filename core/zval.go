package core

import (
	"fmt"
	"io"
)

type Val interface {
	GetType() ZType
	ZVal() *ZVal
	AsVal(ctx Context, t ZType) (Val, error)
}

type ZVal struct {
	v Val
}

type runZVal struct {
	v Val
	l *Loc
}

func (z *ZVal) GetType() ZType {
	if z == nil {
		return ZtNull
	}
	if z.v == nil {
		return ZtNull
	}
	return z.v.GetType()
}

func (z *runZVal) Run(ctx Context) (*ZVal, error) {
	return &ZVal{z.v}, nil
}

func (z *runZVal) Loc() *Loc {
	return z.l
}

func (z *runZVal) Dump(w io.Writer) error {
	// TODO
	_, err := fmt.Fprintf(w, "%#v", z.v)
	return err
}

func (z *ZVal) ZVal() *ZVal {
	return z
}

func (z *ZVal) Dup() *ZVal {
	// TODO duplicate contents if array
	switch v := z.v.(type) {
	case *ZVal:
		return &ZVal{v.v}
	default:
		return &ZVal{z.v}
	}
}

// Ref returns a reference to this zval
func (z *ZVal) Ref() *ZVal {
	if _, isRef := z.v.(*ZVal); isRef {
		return z
	}
	return &ZVal{z}
}

func (z *ZVal) IsRef() bool {
	if z == nil {
		return false
	}
	_, isRef := z.v.(*ZVal)
	return isRef
}

func (z *ZVal) Value() Val {
	if z == nil {
		return nil
	}
	if sz, ok := z.v.(*ZVal); ok {
		return sz.Value()
	}
	return z.v
}

func (z *ZVal) Set(nz *ZVal) {
	// set value of this zval to given zval
	if rz, isRef := z.v.(*ZVal); isRef {
		rz.Set(nz)
		return
	}

	// simple set
	z.v = nz.v
}
