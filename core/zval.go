package core

type Val interface {
	GetType() ZType
}

type ZVal struct {
	v Val
}
