package core

type ZInt int64

func (z ZInt) GetType() ZType {
	return ZtInt
}
