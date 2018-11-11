package core

import (
	"fmt"

	"github.com/MagicalTux/gophp/core/tokenizer"
)

type runCast struct {
	t ZType
	v Runnable
	l *Loc
}

func spawnRunCast(t tokenizer.ItemType, v Runnable, l *Loc) (Runnable, error) {
	switch t {
	case tokenizer.T_BOOL_CAST:
		return &runCast{ZtBool, v, l}, nil
	case tokenizer.T_INT_CAST:
		return &runCast{ZtInt, v, l}, nil
	case tokenizer.T_ARRAY_CAST:
		return &runCast{ZtArray, v, l}, nil
	case tokenizer.T_DOUBLE_CAST:
		return &runCast{ZtFloat, v, l}, nil
	case tokenizer.T_OBJECT_CAST:
		return &runCast{ZtObject, v, l}, nil
	case tokenizer.T_STRING_CAST:
		return &runCast{ZtString, v, l}, nil
	default:
		return nil, fmt.Errorf("Unexpected %s", t)
	}
}

func (r *runCast) Run(ctx Context) (*ZVal, error) {
	v, err := r.v.Run(ctx)
	if err != nil {
		return nil, err
	}
	return v.As(ctx, r.t)
}

func (r *runCast) Loc() *Loc {
	return r.l
}
