package standard

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"math"
	"strings"

	"github.com/MagicalTux/goro/core"
	"github.com/MagicalTux/goro/core/phpv"
)

// > func string pack ( string $format [, mixed $... ] )
func fncPack(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	if len(args) < 1 {
		return nil, fmt.Errorf("pack() expects at least 1 argument")
	}

	format := args[0].AsString(ctx)
	argIdx := 1
	var buf bytes.Buffer

	i := 0
	for i < len(format) {
		code := format[i]
		i++

		// Parse repeat count
		repeat := 1
		if i < len(format) && format[i] == '*' {
			repeat = -1 // special: all remaining
			i++
		} else {
			n := 0
			hasN := false
			for i < len(format) && format[i] >= '0' && format[i] <= '9' {
				n = n*10 + int(format[i]-'0')
				hasN = true
				i++
			}
			if hasN {
				repeat = n
			}
		}

		switch code {
		case 'a', 'A': // NUL-padded / space-padded string
			if argIdx >= len(args) {
				return nil, fmt.Errorf("pack(): Type %c: too few arguments", code)
			}
			s := string(args[argIdx].AsString(ctx))
			argIdx++
			if repeat == -1 {
				repeat = len(s)
			}
			if len(s) >= repeat {
				buf.WriteString(s[:repeat])
			} else {
				buf.WriteString(s)
				pad := byte(0)
				if code == 'A' {
					pad = ' '
				}
				for j := len(s); j < repeat; j++ {
					buf.WriteByte(pad)
				}
			}

		case 'H', 'h': // hex string, high/low nibble first
			if argIdx >= len(args) {
				return nil, fmt.Errorf("pack(): Type %c: too few arguments", code)
			}
			s := string(args[argIdx].AsString(ctx))
			argIdx++
			if repeat == -1 {
				repeat = len(s)
			}
			nibbles := repeat
			for j := 0; j < nibbles; j += 2 {
				var hi, lo byte
				if j < len(s) {
					hi = hexVal(s[j])
				}
				if j+1 < len(s) {
					lo = hexVal(s[j+1])
				}
				if code == 'H' {
					buf.WriteByte(hi<<4 | lo)
				} else {
					buf.WriteByte(lo<<4 | hi)
				}
			}

		case 'c', 'C': // signed/unsigned char
			count := repeat
			if count == -1 {
				count = len(args) - argIdx
			}
			for j := 0; j < count; j++ {
				if argIdx >= len(args) {
					return nil, fmt.Errorf("pack(): Type %c: too few arguments", code)
				}
				v := args[argIdx].AsInt(ctx)
				argIdx++
				buf.WriteByte(byte(v))
			}

		case 's', 'S': // signed/unsigned short (16 bit, little endian)
			count := repeat
			if count == -1 {
				count = len(args) - argIdx
			}
			for j := 0; j < count; j++ {
				if argIdx >= len(args) {
					return nil, fmt.Errorf("pack(): Type %c: too few arguments", code)
				}
				v := args[argIdx].AsInt(ctx)
				argIdx++
				binary.Write(&buf, binary.LittleEndian, uint16(v))
			}

		case 'n': // unsigned short (16 bit, big endian)
			count := repeat
			if count == -1 {
				count = len(args) - argIdx
			}
			for j := 0; j < count; j++ {
				if argIdx >= len(args) {
					return nil, fmt.Errorf("pack(): Type %c: too few arguments", code)
				}
				v := args[argIdx].AsInt(ctx)
				argIdx++
				binary.Write(&buf, binary.BigEndian, uint16(v))
			}

		case 'v': // unsigned short (16 bit, little endian)
			count := repeat
			if count == -1 {
				count = len(args) - argIdx
			}
			for j := 0; j < count; j++ {
				if argIdx >= len(args) {
					return nil, fmt.Errorf("pack(): Type %c: too few arguments", code)
				}
				v := args[argIdx].AsInt(ctx)
				argIdx++
				binary.Write(&buf, binary.LittleEndian, uint16(v))
			}

		case 'l', 'L': // signed/unsigned long (32 bit, machine byte order)
			count := repeat
			if count == -1 {
				count = len(args) - argIdx
			}
			for j := 0; j < count; j++ {
				if argIdx >= len(args) {
					return nil, fmt.Errorf("pack(): Type %c: too few arguments", code)
				}
				v := args[argIdx].AsInt(ctx)
				argIdx++
				binary.Write(&buf, binary.LittleEndian, uint32(v))
			}

		case 'N': // unsigned long (32 bit, big endian)
			count := repeat
			if count == -1 {
				count = len(args) - argIdx
			}
			for j := 0; j < count; j++ {
				if argIdx >= len(args) {
					return nil, fmt.Errorf("pack(): Type %c: too few arguments", code)
				}
				v := args[argIdx].AsInt(ctx)
				argIdx++
				binary.Write(&buf, binary.BigEndian, uint32(v))
			}

		case 'V': // unsigned long (32 bit, little endian)
			count := repeat
			if count == -1 {
				count = len(args) - argIdx
			}
			for j := 0; j < count; j++ {
				if argIdx >= len(args) {
					return nil, fmt.Errorf("pack(): Type %c: too few arguments", code)
				}
				v := args[argIdx].AsInt(ctx)
				argIdx++
				binary.Write(&buf, binary.LittleEndian, uint32(v))
			}

		case 'q', 'Q': // signed/unsigned long long (64 bit, machine byte order)
			count := repeat
			if count == -1 {
				count = len(args) - argIdx
			}
			for j := 0; j < count; j++ {
				if argIdx >= len(args) {
					return nil, fmt.Errorf("pack(): Type %c: too few arguments", code)
				}
				v := args[argIdx].AsInt(ctx)
				argIdx++
				binary.Write(&buf, binary.LittleEndian, uint64(v))
			}

		case 'J': // unsigned long long (64 bit, big endian)
			count := repeat
			if count == -1 {
				count = len(args) - argIdx
			}
			for j := 0; j < count; j++ {
				if argIdx >= len(args) {
					return nil, fmt.Errorf("pack(): Type %c: too few arguments", code)
				}
				v := args[argIdx].AsInt(ctx)
				argIdx++
				binary.Write(&buf, binary.BigEndian, uint64(v))
			}

		case 'P': // unsigned long long (64 bit, little endian)
			count := repeat
			if count == -1 {
				count = len(args) - argIdx
			}
			for j := 0; j < count; j++ {
				if argIdx >= len(args) {
					return nil, fmt.Errorf("pack(): Type %c: too few arguments", code)
				}
				v := args[argIdx].AsInt(ctx)
				argIdx++
				binary.Write(&buf, binary.LittleEndian, uint64(v))
			}

		case 'f', 'g': // float (little endian for g)
			count := repeat
			if count == -1 {
				count = len(args) - argIdx
			}
			for j := 0; j < count; j++ {
				if argIdx >= len(args) {
					return nil, fmt.Errorf("pack(): Type %c: too few arguments", code)
				}
				v := float32(args[argIdx].AsFloat(ctx))
				argIdx++
				binary.Write(&buf, binary.LittleEndian, v)
			}

		case 'G': // float (big endian)
			count := repeat
			if count == -1 {
				count = len(args) - argIdx
			}
			for j := 0; j < count; j++ {
				if argIdx >= len(args) {
					return nil, fmt.Errorf("pack(): Type %c: too few arguments", code)
				}
				v := float32(args[argIdx].AsFloat(ctx))
				argIdx++
				binary.Write(&buf, binary.BigEndian, v)
			}

		case 'd', 'e': // double (little endian for e)
			count := repeat
			if count == -1 {
				count = len(args) - argIdx
			}
			for j := 0; j < count; j++ {
				if argIdx >= len(args) {
					return nil, fmt.Errorf("pack(): Type %c: too few arguments", code)
				}
				v := float64(args[argIdx].AsFloat(ctx))
				argIdx++
				binary.Write(&buf, binary.LittleEndian, v)
			}

		case 'E': // double (big endian)
			count := repeat
			if count == -1 {
				count = len(args) - argIdx
			}
			for j := 0; j < count; j++ {
				if argIdx >= len(args) {
					return nil, fmt.Errorf("pack(): Type %c: too few arguments", code)
				}
				v := float64(args[argIdx].AsFloat(ctx))
				argIdx++
				binary.Write(&buf, binary.BigEndian, v)
			}

		case 'x': // NUL byte
			count := repeat
			if count == -1 {
				count = 1
			}
			for j := 0; j < count; j++ {
				buf.WriteByte(0)
			}

		case 'X': // back up one byte
			count := repeat
			if count == -1 {
				count = 1
			}
			cur := buf.Len()
			if count > cur {
				count = cur
			}
			buf.Truncate(cur - count)

		case 'Z': // NUL-padded string (like 'a' but with trailing NUL)
			if argIdx >= len(args) {
				return nil, fmt.Errorf("pack(): Type Z: too few arguments")
			}
			s := string(args[argIdx].AsString(ctx))
			argIdx++
			if repeat == -1 {
				buf.WriteString(s)
				buf.WriteByte(0)
			} else {
				if repeat <= 0 {
					// zero-length Z format: no output
				} else if len(s) >= repeat {
					buf.WriteString(s[:repeat-1])
					buf.WriteByte(0)
				} else {
					buf.WriteString(s)
					for j := len(s); j < repeat; j++ {
						buf.WriteByte(0)
					}
				}
			}

		case '@': // NUL-fill to absolute position
			if repeat == -1 {
				repeat = 0
			}
			cur := buf.Len()
			if repeat > cur {
				for j := cur; j < repeat; j++ {
					buf.WriteByte(0)
				}
			} else {
				buf.Truncate(repeat)
			}

		default:
			return nil, fmt.Errorf("pack(): Type %c: unknown format code", code)
		}
	}

	return phpv.ZString(buf.Bytes()).ZVal(), nil
}

// > func array unpack ( string $format, string $data [, int $offset = 0 ] )
func fncUnpack(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var format phpv.ZString
	var data phpv.ZString
	var offset *phpv.ZInt

	_, err := core.Expand(ctx, args, &format, &data, &offset)
	if err != nil {
		return nil, err
	}

	d := []byte(data)
	pos := 0
	if offset != nil {
		pos = int(*offset)
	}

	result := phpv.NewZArray()
	fmtStr := string(format)

	i := 0
	for i < len(fmtStr) {
		code := fmtStr[i]
		i++

		// Parse repeat count
		repeat := 1
		if i < len(fmtStr) && fmtStr[i] == '*' {
			repeat = -1
			i++
		} else {
			n := 0
			hasN := false
			for i < len(fmtStr) && fmtStr[i] >= '0' && fmtStr[i] <= '9' {
				n = n*10 + int(fmtStr[i]-'0')
				hasN = true
				i++
			}
			if hasN {
				repeat = n
			}
		}

		// Parse optional name
		name := ""
		if i < len(fmtStr) && fmtStr[i] != '/' {
			start := i
			for i < len(fmtStr) && fmtStr[i] != '/' {
				i++
			}
			name = fmtStr[start:i]
		}
		if i < len(fmtStr) && fmtStr[i] == '/' {
			i++
		}

		switch code {
		case 'a', 'A', 'Z': // NUL/space-padded string
			count := repeat
			if count == -1 {
				count = len(d) - pos
			}
			if pos+count > len(d) {
				return nil, fmt.Errorf("unpack(): Type %c: not enough input", code)
			}
			s := string(d[pos : pos+count])
			if code == 'A' {
				s = strings.TrimRight(s, " \x00")
			} else if code == 'a' {
				// keep as is
			} else { // Z
				if idx := strings.IndexByte(s, 0); idx >= 0 {
					s = s[:idx]
				}
			}
			pos += count
			setUnpackResult(result, ctx, name, phpv.ZString(s).ZVal(), 1)

		case 'H', 'h': // hex string
			count := repeat
			if count == -1 {
				count = (len(d) - pos) * 2
			}
			bytesNeeded := (count + 1) / 2
			if pos+bytesNeeded > len(d) {
				return nil, fmt.Errorf("unpack(): Type %c: not enough input", code)
			}
			var hex strings.Builder
			for j := 0; j < count; j++ {
				byteIdx := j / 2
				b := d[pos+byteIdx]
				if code == 'H' {
					if j%2 == 0 {
						hex.WriteByte(hexChar(b >> 4))
					} else {
						hex.WriteByte(hexChar(b & 0x0f))
					}
				} else {
					if j%2 == 0 {
						hex.WriteByte(hexChar(b & 0x0f))
					} else {
						hex.WriteByte(hexChar(b >> 4))
					}
				}
			}
			pos += bytesNeeded
			setUnpackResult(result, ctx, name, phpv.ZString(hex.String()).ZVal(), 1)

		case 'c': // signed char
			count := repeat
			if count == -1 {
				count = len(d) - pos
			}
			for j := 0; j < count; j++ {
				if pos >= len(d) {
					return nil, fmt.Errorf("unpack(): Type c: not enough input")
				}
				v := int8(d[pos])
				pos++
				setUnpackResult(result, ctx, name, phpv.ZInt(v).ZVal(), j+1)
			}

		case 'C': // unsigned char
			count := repeat
			if count == -1 {
				count = len(d) - pos
			}
			for j := 0; j < count; j++ {
				if pos >= len(d) {
					return nil, fmt.Errorf("unpack(): Type C: not enough input")
				}
				v := d[pos]
				pos++
				setUnpackResult(result, ctx, name, phpv.ZInt(v).ZVal(), j+1)
			}

		case 's': // signed short (16 bit, machine/little endian)
			count := repeat
			if count == -1 {
				count = (len(d) - pos) / 2
			}
			for j := 0; j < count; j++ {
				if pos+2 > len(d) {
					return nil, fmt.Errorf("unpack(): Type s: not enough input")
				}
				v := int16(binary.LittleEndian.Uint16(d[pos:]))
				pos += 2
				setUnpackResult(result, ctx, name, phpv.ZInt(v).ZVal(), j+1)
			}

		case 'S', 'v': // unsigned short (16 bit, little endian)
			count := repeat
			if count == -1 {
				count = (len(d) - pos) / 2
			}
			for j := 0; j < count; j++ {
				if pos+2 > len(d) {
					return nil, fmt.Errorf("unpack(): Type %c: not enough input", code)
				}
				v := binary.LittleEndian.Uint16(d[pos:])
				pos += 2
				setUnpackResult(result, ctx, name, phpv.ZInt(v).ZVal(), j+1)
			}

		case 'n': // unsigned short (16 bit, big endian)
			count := repeat
			if count == -1 {
				count = (len(d) - pos) / 2
			}
			for j := 0; j < count; j++ {
				if pos+2 > len(d) {
					return nil, fmt.Errorf("unpack(): Type n: not enough input")
				}
				v := binary.BigEndian.Uint16(d[pos:])
				pos += 2
				setUnpackResult(result, ctx, name, phpv.ZInt(v).ZVal(), j+1)
			}

		case 'l': // signed long (32 bit)
			count := repeat
			if count == -1 {
				count = (len(d) - pos) / 4
			}
			for j := 0; j < count; j++ {
				if pos+4 > len(d) {
					return nil, fmt.Errorf("unpack(): Type l: not enough input")
				}
				v := int32(binary.LittleEndian.Uint32(d[pos:]))
				pos += 4
				setUnpackResult(result, ctx, name, phpv.ZInt(v).ZVal(), j+1)
			}

		case 'L', 'V': // unsigned long (32 bit, little endian)
			count := repeat
			if count == -1 {
				count = (len(d) - pos) / 4
			}
			for j := 0; j < count; j++ {
				if pos+4 > len(d) {
					return nil, fmt.Errorf("unpack(): Type %c: not enough input", code)
				}
				v := binary.LittleEndian.Uint32(d[pos:])
				pos += 4
				setUnpackResult(result, ctx, name, phpv.ZInt(v).ZVal(), j+1)
			}

		case 'N': // unsigned long (32 bit, big endian)
			count := repeat
			if count == -1 {
				count = (len(d) - pos) / 4
			}
			for j := 0; j < count; j++ {
				if pos+4 > len(d) {
					return nil, fmt.Errorf("unpack(): Type N: not enough input")
				}
				v := binary.BigEndian.Uint32(d[pos:])
				pos += 4
				setUnpackResult(result, ctx, name, phpv.ZInt(v).ZVal(), j+1)
			}

		case 'q': // signed long long (64 bit)
			count := repeat
			if count == -1 {
				count = (len(d) - pos) / 8
			}
			for j := 0; j < count; j++ {
				if pos+8 > len(d) {
					return nil, fmt.Errorf("unpack(): Type q: not enough input")
				}
				v := int64(binary.LittleEndian.Uint64(d[pos:]))
				pos += 8
				setUnpackResult(result, ctx, name, phpv.ZInt(v).ZVal(), j+1)
			}

		case 'Q', 'P': // unsigned long long (64 bit, little endian)
			count := repeat
			if count == -1 {
				count = (len(d) - pos) / 8
			}
			for j := 0; j < count; j++ {
				if pos+8 > len(d) {
					return nil, fmt.Errorf("unpack(): Type %c: not enough input", code)
				}
				v := binary.LittleEndian.Uint64(d[pos:])
				pos += 8
				setUnpackResult(result, ctx, name, phpv.ZInt(int64(v)).ZVal(), j+1)
			}

		case 'J': // unsigned long long (64 bit, big endian)
			count := repeat
			if count == -1 {
				count = (len(d) - pos) / 8
			}
			for j := 0; j < count; j++ {
				if pos+8 > len(d) {
					return nil, fmt.Errorf("unpack(): Type J: not enough input")
				}
				v := binary.BigEndian.Uint64(d[pos:])
				pos += 8
				setUnpackResult(result, ctx, name, phpv.ZInt(int64(v)).ZVal(), j+1)
			}

		case 'f', 'g': // float (little endian)
			count := repeat
			if count == -1 {
				count = (len(d) - pos) / 4
			}
			for j := 0; j < count; j++ {
				if pos+4 > len(d) {
					return nil, fmt.Errorf("unpack(): Type %c: not enough input", code)
				}
				bits := binary.LittleEndian.Uint32(d[pos:])
				v := math.Float32frombits(bits)
				pos += 4
				setUnpackResult(result, ctx, name, phpv.ZFloat(v).ZVal(), j+1)
			}

		case 'G': // float (big endian)
			count := repeat
			if count == -1 {
				count = (len(d) - pos) / 4
			}
			for j := 0; j < count; j++ {
				if pos+4 > len(d) {
					return nil, fmt.Errorf("unpack(): Type G: not enough input")
				}
				bits := binary.BigEndian.Uint32(d[pos:])
				v := math.Float32frombits(bits)
				pos += 4
				setUnpackResult(result, ctx, name, phpv.ZFloat(v).ZVal(), j+1)
			}

		case 'd', 'e': // double (little endian)
			count := repeat
			if count == -1 {
				count = (len(d) - pos) / 8
			}
			for j := 0; j < count; j++ {
				if pos+8 > len(d) {
					return nil, fmt.Errorf("unpack(): Type %c: not enough input", code)
				}
				bits := binary.LittleEndian.Uint64(d[pos:])
				v := math.Float64frombits(bits)
				pos += 8
				setUnpackResult(result, ctx, name, phpv.ZFloat(v).ZVal(), j+1)
			}

		case 'E': // double (big endian)
			count := repeat
			if count == -1 {
				count = (len(d) - pos) / 8
			}
			for j := 0; j < count; j++ {
				if pos+8 > len(d) {
					return nil, fmt.Errorf("unpack(): Type E: not enough input")
				}
				bits := binary.BigEndian.Uint64(d[pos:])
				v := math.Float64frombits(bits)
				pos += 8
				setUnpackResult(result, ctx, name, phpv.ZFloat(v).ZVal(), j+1)
			}

		case 'x': // NUL byte (skip forward)
			count := repeat
			if count == -1 {
				count = 1
			}
			pos += count

		case 'X': // back up one byte
			count := repeat
			if count == -1 {
				count = 1
			}
			pos -= count
			if pos < 0 {
				pos = 0
			}

		case '@': // absolute position
			if repeat == -1 {
				pos = 0
			} else {
				pos = repeat
			}

		default:
			return nil, fmt.Errorf("unpack(): Type %c: unknown format code", code)
		}
	}

	return result.ZVal(), nil
}

func setUnpackResult(result *phpv.ZArray, ctx phpv.Context, name string, val *phpv.ZVal, idx int) {
	if name == "" {
		name = fmt.Sprintf("%d", idx)
	} else if idx > 1 {
		name = fmt.Sprintf("%s%d", name, idx)
	}
	result.OffsetSet(ctx, phpv.ZString(name), val)
}

func hexVal(c byte) byte {
	switch {
	case c >= '0' && c <= '9':
		return c - '0'
	case c >= 'a' && c <= 'f':
		return c - 'a' + 10
	case c >= 'A' && c <= 'F':
		return c - 'A' + 10
	}
	return 0
}

func hexChar(v byte) byte {
	if v < 10 {
		return '0' + v
	}
	return 'a' + v - 10
}
