package core

// php arrays work with two kind of keys

// we store values in _d with a regular index

type ZArray ZHashTable

func (a *ZArray) GetType() ZType {
	return ZtArray
}
