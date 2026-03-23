package hash

import (
	"encoding/hex"

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

	opaque := obj.GetOpaque(HashContext)
	if opaque == nil {
		return nil, phpobj.ThrowError(ctx, phpobj.ValueError, "hash_final(): Argument #1 ($context) must be a valid, non-finalized HashContext")
	}

	h := getHash(opaque)
	if h == nil {
		return nil, phpobj.ThrowError(ctx, phpobj.ValueError, "hash_final(): Argument #1 ($context) must be a valid, non-finalized HashContext")
	}

	r := h.Sum(nil)

	// Mark as finalized
	if hcd, ok := opaque.(*hashContextData); ok {
		hcd.finalized = true
	}

	if raw != nil && *raw {
		// return as raw
		return phpv.ZString(r).ZVal(), nil
	}

	// convert to hex
	return phpv.ZString(hex.EncodeToString(r)).ZVal(), nil
}
