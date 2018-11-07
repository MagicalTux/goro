package core

import (
	"errors"
	"fmt"
	"log"
)

type runOperator struct {
	op string

	a, b runnable
}

func (r *runOperator) run(ctx Context) (*ZVal, error) {
	switch r.op {
	case "=":
		// left needs to be something that can be a reference ("write context")
		a, ok := r.a.(Writable)
		if !ok {
			return nil, fmt.Errorf("Can't use %T value in write context", r.a)
		}
		b, err := r.b.run(ctx)
		if err != nil {
			return nil, err
		}
		return b, a.WriteValue(ctx, b)
	}

	a, err := r.a.run(ctx)
	if err != nil {
		return nil, err
	}
	b, err := r.b.run(ctx)
	if err != nil {
		return nil, err
	}

	if a.v.GetType() != b.v.GetType() {
		a, _ = a.AsNumeric(ctx)
		b, _ = b.AsNumeric(ctx)

		if a.v.GetType() == ZtFloat || b.v.GetType() == ZtFloat {
			a, _ = a.As(ctx, ZtFloat)
			b, _ = b.As(ctx, ZtFloat)
		} else {
			a, _ = a.As(ctx, ZtInt)
			b, _ = b.As(ctx, ZtInt)
		}
	}

	switch r.op {
	case "+":
		switch a.v.GetType() {
		case ZtInt:
			r := &ZVal{a.v.(ZInt) + b.v.(ZInt)}
			return r, nil
		case ZtFloat:
			r := &ZVal{a.v.(ZFloat) + b.v.(ZFloat)}
			return r, nil
		default:
			return nil, errors.New("todo operator type unsupported")
		}
	}
	// TODO
	log.Printf("operator %s %s %s", r.op, a, b)
	return nil, errors.New("todo operator")
}
