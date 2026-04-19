package bz2

import (
	"bytes"
	"compress/bzip2"
	"io"

	"github.com/KarpelesLab/gobzip2"
	"github.com/KarpelesLab/goro/core"
	"github.com/KarpelesLab/goro/core/phpobj"
	"github.com/KarpelesLab/goro/core/phpv"
)

// > func mixed bzdecompress ( string $source [, int $small = 0 ] )
func fncBzDecompress(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var src phpv.ZString
	var small *phpv.ZInt

	_, err := core.Expand(ctx, args, &src, &small)
	if err != nil {
		return nil, err
	}

	in := bytes.NewBuffer([]byte(src))
	b, err := io.ReadAll(bzip2.NewReader(in))
	if err != nil {
		return phpv.ZBool(false).ZVal(), nil
	}
	err = ctx.MemAlloc(ctx, uint64(len(b)))

	return phpv.ZString(b).ZVal(), err
}

// > func string|int bzcompress ( string $source [, int $blocksize = 4 [, int $workfactor = 0 ]] )
func fncBzCompress(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var src phpv.ZString
	var blocksize *phpv.ZInt
	var workfactor *phpv.ZInt

	_, err := core.Expand(ctx, args, &src, &blocksize, &workfactor)
	if err != nil {
		return nil, err
	}

	bs := 4
	if blocksize != nil {
		bs = int(*blocksize)
		if bs < 1 || bs > 9 {
			return nil, phpobj.ThrowError(ctx, phpobj.ValueError,
				"bzcompress(): Argument #2 ($block_size) must be between 1 and 9")
		}
	}

	var buf bytes.Buffer
	w, err := gobzip2.NewWriterLevel(&buf, bs)
	if err != nil {
		return phpv.ZInt(-1).ZVal(), nil
	}
	_, err = w.Write([]byte(src))
	if err != nil {
		return phpv.ZInt(-1).ZVal(), nil
	}
	err = w.Close()
	if err != nil {
		return phpv.ZInt(-1).ZVal(), nil
	}

	result := buf.Bytes()
	ctx.MemAlloc(ctx, uint64(len(result)))
	return phpv.ZString(result).ZVal(), nil
}
