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
