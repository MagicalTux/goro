package hash

import (
	"github.com/MagicalTux/goro/core"
	"github.com/MagicalTux/goro/core/phpobj"
	"github.com/MagicalTux/goro/core/phpv"
)

// > func bool hash_update ( HashContext $context , string $data )
func fncHashUpdate(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	obj := &phpobj.ZObject{Class: HashContext}
	var data phpv.ZString

	_, err := core.Expand(ctx, args, &obj, &data)
	if err != nil {
		return nil, err
	}

	opaque := obj.GetOpaque(HashContext)
	if opaque == nil {
		return nil, phpobj.ThrowError(ctx, phpobj.ValueError, "hash_update(): Argument #1 ($context) must be a valid, non-finalized HashContext")
	}

	h := getHash(opaque)
	if h == nil {
		return nil, phpobj.ThrowError(ctx, phpobj.ValueError, "hash_update(): Argument #1 ($context) must be a valid, non-finalized HashContext")
	}

	_, err = h.Write([]byte(data))
	if err != nil {
		return nil, err
	}

	return phpv.ZBool(true).ZVal(), nil
}
