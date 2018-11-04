package tokenizer

//go:generate stringer -type=ItemType
type ItemType int

const (
	ItemError ItemType = iota
	T_DNUMBER
	T_LNUMBER
	ItemText
	ItemEOF
)

type item struct {
	t    ItemType
	data string
}
