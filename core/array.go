package core

// php arrays work with two values

type ZArray struct {
	_i map[ZInt]ZVal
	_s map[ZString]ZVal // we cast that to string so it can be used as map key
}

func (a *ZArray) GetType() ZType {
	return ZtArray
}
