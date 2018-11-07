package core

type ZType int

const (
	ZtNull ZType = iota
	ZtInt
	ZtFlat
	ZtString
	ZtArray
	ZtObject
)
