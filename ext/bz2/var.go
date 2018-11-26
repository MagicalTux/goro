package bz2

import (
	"bytes"
	"compress/bzip2"
	"io/ioutil"

	"github.com/MagicalTux/goro/core"
)

// operations on local variables

//> func mixed bzdecompress ( string $source [, int $small = 0 ] )
func fncBzDecompress(ctx core.Context, args []*core.ZVal) (*core.ZVal, error) {
	var src core.ZString
	var small *core.ZInt

	_, err := core.Expand(ctx, args, &src, &small)
	if err != nil {
		return nil, err
	}

	// NOTE: small not supported by go implementation

	in := bytes.NewBuffer([]byte(src))
	b, err := ioutil.ReadAll(bzip2.NewReader(in))
	if err != nil {
		return nil, err
	}
	err = ctx.MemAlloc(ctx, uint64(len(b)))

	return core.ZString(b).ZVal(), err
}
