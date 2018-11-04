package tokenizer

type ItemType int

const (
	ItemError ItemType = iota
	ItemText
	ItemEOF
)

type item struct {
	t    ItemType
	data string
}
