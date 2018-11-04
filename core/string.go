package core

type ZString string

func (z ZString) GetType() ZType {
	return ZtString
}
