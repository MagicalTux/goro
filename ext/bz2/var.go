package bz2

import (
	"bytes"
	"compress/bzip2"
	"io/ioutil"

	"github.com/MagicalTux/goro/core"
	"github.com/MagicalTux/goro/core/phpv"
)

// operations on local variables

// > func mixed bzdecompress ( string $source [, int $small = 0 ] )
func fncBzDecompress(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var src phpv.ZString
	var small *phpv.ZInt

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

	return phpv.ZString(b).ZVal(), err
}
