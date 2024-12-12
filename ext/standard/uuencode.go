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

func PHP_UU_DEC(b byte) byte {
	return ((b - ' ') & 077)
}

// > func string convert_uudecode ( string $data )
// To copy some of uudecode's weird error cases and accepted inputs,
// this function is almost directly translated from uuencode.c
// except the pointer arithmetic is replaced with regular integer indices.
func convertUuDecode(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var dataArg phpv.ZString
	_, err := core.Expand(ctx, args, &dataArg)
	if err != nil {
		return nil, ctx.FuncError(err)
	}

	if len(dataArg) == 0 {
		return phpv.ZFalse.ZVal(), nil
	}

	totalLen := 0
	src := []byte(dataArg)
	srcLen := len(src)
	dest := make([]byte, int(math.Ceil(float64(srcLen)*0.75)))
	p := 0
	s := 0
	e := srcLen

	for s < e {
		length := PHP_UU_DEC(src[s])
		s++
		if length == 0 {
			break
		}

		if int(length) > srcLen {
			goto fail_err
		}

		totalLen += int(length)

		var ee int
		if length == 45 {
			ee = s+60
		} else {
			ee = s+int(math.Floor(float64(length) * 1.33))
		}

		if ee > e {
			goto fail_err
		}

		for s < ee {
			if s+4 > e {
				goto fail_err
			}

			dest[p] = PHP_UU_DEC(src[s])<<2 | PHP_UU_DEC(src[s+1])>>4
			p++
			dest[p] = PHP_UU_DEC(src[s+1])<<4 | PHP_UU_DEC(src[s+2])>>2
			p++
			dest[p] = PHP_UU_DEC(src[s+2])<<6 | PHP_UU_DEC(src[s+3])
			p++

			s += 4
		}

		if length < 45 {
			break
		}

		// skip \n
		s++
	}

	if length := totalLen; length > p {
		dest[p] = PHP_UU_DEC(src[s])<<2 | PHP_UU_DEC(src[s+1])>>4
		p++
		if length > 1 {
			dest[p] = PHP_UU_DEC(src[s+1])<<4 | PHP_UU_DEC(src[s+2])>>2
			p++
			if length > 2 {
				dest[p] = PHP_UU_DEC(src[s+2])<<6 | PHP_UU_DEC(src[s+3])
				p++
			}
		}
	}

	return phpv.ZStr(string(dest[:totalLen])), nil

fail_err:
	ctx.Warn("The given parameter is not a valid uuencoded string")
	return phpv.ZFalse.ZVal(), nil
}
