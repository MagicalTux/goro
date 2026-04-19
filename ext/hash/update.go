package hash

import (
	"github.com/KarpelesLab/goro/core"
	"github.com/KarpelesLab/goro/core/phpobj"
	"github.com/KarpelesLab/goro/core/phpv"
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

	b := []byte(data)
	_, err = h.Write(b)
	if err != nil {
		return nil, err
	}

	// Buffer written data for serialization support
	if hcd, ok := opaque.(*hashContextData); ok {
		hcd.writtenData = append(hcd.writtenData, b...)
	}

	return phpv.ZBool(true).ZVal(), nil
}
