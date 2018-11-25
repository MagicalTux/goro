package hash

import (
	gohash "hash"

	"github.com/MagicalTux/goro/core"
)

//> func bool hash_update ( HashContext $context , string $data )
func fncHashUpdate(ctx core.Context, args []*core.ZVal) (*core.ZVal, error) {
	obj := &core.ZObject{Class: HashContext}
	var data core.ZString

	_, err := core.Expand(ctx, args, &obj, &data)
	if err != nil {
		return nil, err
	}

	h := obj.GetOpaque(HashContext).(gohash.Hash)
	_, err = h.Write([]byte(data))
	if err != nil {
		return nil, err
	}

	return core.ZBool(true).ZVal(), nil
}
