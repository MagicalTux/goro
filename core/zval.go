package core

type Val interface {
	GetType() ZType
}

type ZVal struct {
	v Val
}

func (z *ZVal) run(ctx Context) (*ZVal, error) {
	return z, nil
}

func (z *ZVal) IsNull() bool {
	if z == nil {
		return true
	}
	if z.v == nil {
		return true
	}
	return false
}
