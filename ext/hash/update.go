package hash

import (
	gohash "hash"

	"github.com/MagicalTux/goro/core"
	"github.com/MagicalTux/goro/core/phpobj"
	"github.com/MagicalTux/goro/core/phpv"
)

//> func bool hash_update ( HashContext $context , string $data )
func fncHashUpdate(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	obj := &phpobj.ZObject{Class: HashContext}
	var data phpv.ZString

	_, err := core.Expand(ctx, args, &obj, &data)
	if err != nil {
		return nil, err
	}

	h := obj.GetOpaque(HashContext).(gohash.Hash)
	_, err = h.Write([]byte(data))
	if err != nil {
		return nil, err
	}

	return phpv.ZBool(true).ZVal(), nil
}
