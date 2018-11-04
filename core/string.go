package core

type ZString []byte

func (z ZString) GetType() ZType {
	return ZtString
}
