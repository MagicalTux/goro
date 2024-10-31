package hash

import (
	"encoding/hex"
	gohash "hash"

	"github.com/MagicalTux/goro/core"
	"github.com/MagicalTux/goro/core/phpobj"
	"github.com/MagicalTux/goro/core/phpv"
)

// > func string hash_final ( HashContext $context [, bool $raw_output = FALSE ] )
func fncHashFinal(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	obj := &phpobj.ZObject{Class: HashContext}
	var raw *phpv.ZBool

	_, err := core.Expand(ctx, args, &obj, &raw)
	if err != nil {
		return nil, err
	}

	h := obj.GetOpaque(HashContext).(gohash.Hash)
	r := h.Sum(nil)

	if raw != nil && *raw {
		// return as raw
		return phpv.ZString(r).ZVal(), nil
	}

	// convert to hex
	return phpv.ZString(hex.EncodeToString(r)).ZVal(), nil
}
