package hash

import (
	"encoding/hex"
	gohash "hash"

	"github.com/MagicalTux/goro/core"
)

//> func string hash_final ( HashContext $context [, bool $raw_output = FALSE ] )
func fncHashFinal(ctx core.Context, args []*core.ZVal) (*core.ZVal, error) {
	obj := &core.ZObject{Class: HashContext}
	var raw *core.ZBool

	_, err := core.Expand(ctx, args, &obj, &raw)
	if err != nil {
		return nil, err
	}

	h := obj.GetOpaque(HashContext).(gohash.Hash)
	r := h.Sum(nil)

	if raw != nil && *raw {
		// return as raw
		return core.ZString(r).ZVal(), nil
	}

	// convert to hex
	return core.ZString(hex.EncodeToString(r)).ZVal(), nil
}
