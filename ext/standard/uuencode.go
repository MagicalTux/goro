package standard

import (
	"bytes"
	"math"

	"github.com/MagicalTux/goro/core"
	"github.com/MagicalTux/goro/core/phpv"
)

// > func string convert_uuencode ( string $data )
func convertUuEncode(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var dataArg phpv.ZString
	_, err := core.Expand(ctx, args, &dataArg)
	if err != nil {
		return nil, ctx.FuncError(err)
	}

	if len(dataArg) == 0 {
		return phpv.ZFalse.ZVal(), nil
	}

	var buf bytes.Buffer
	var lineBuf bytes.Buffer

	data := []byte(dataArg)
	for len(data) > 0 {
		x := 45
		if x > len(data) {
			x = len(data)
		}

		total := 0
		for k := 0; k < x; k += 3 {
			var n int32
			n |= int32(idx(data, k+0)) << 16
			n |= int32(idx(data, k+1)) << 8
			n |= int32(idx(data, k+2)) << 0

			for _, b := range []byte{
				byte(n >> 18), byte(n >> 12),
				byte(n >> 6), byte(n >> 0),
			} {
				b = (b & 0b111111)
				if b == 0 {
					b = 64
				}
				lineBuf.WriteByte(b + 32)
				total++
			}
		}

		buf.WriteByte(byte(x + 32))
		buf.Write(lineBuf.Bytes())
		buf.WriteByte('\n')
		lineBuf.Reset()

		data = data[x:]
	}
	buf.WriteString("`\n")

	return phpv.ZStr(buf.String()), nil
}

// > func string convert_uudecode ( string $data )
func convertUuDecode(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var dataArg phpv.ZString
	_, err := core.Expand(ctx, args, &dataArg)
	if err != nil {
		return nil, ctx.FuncError(err)
	}

	if len(dataArg) == 0 {
		return phpv.ZFalse.ZVal(), nil
	}

	var buf bytes.Buffer

	data := []byte(dataArg)
	for len(data) > 0 {
		count := int(data[0] - 32)
		if count == 64 {
			break
		}

		count = int(math.Ceil(float64(count)/3) * 4)

		// TODO: handle error casese

		for k := 1; k <= count; k += 4 {
			var n int32
			n |= int32(idx(data, k+0)-32) % 64 << 18
			n |= int32(idx(data, k+1)-32) % 64 << 12
			n |= int32(idx(data, k+2)-32) % 64 << 06
			n |= int32(idx(data, k+3)-32) % 64 << 00

			for _, b := range []int32{n >> 16, n >> 8, n >> 0} {
				if b != 64 {
					buf.WriteByte(byte(b))
				}
			}
		}

		// +2 to skip also the count and the newline
		data = data[min(count+2, len(data)):]
	}

	return phpv.ZStr(buf.String()), nil
}
