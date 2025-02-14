package core

import (
	"bytes"
	"io"
	"strconv"
	"strings"
	"unicode"

	"github.com/MagicalTux/goro/core/phpv"
)

type counter struct {
	Value int
}

func (c *counter) Add(nn int, err error) error {
	c.Value += nn
	return err
}

type printfWriter interface {
	io.Writer
	io.ByteWriter
	io.StringWriter
}

type formatOptions struct {
	leftJustify bool
	signed      bool
	padChar     byte
}

func padLeft(s string, minWidth int, pad byte) string {
	return strings.Repeat(string(pad), minWidth-len(s)) + s
}
func padRight(s string, minWidth int, pad byte) string {
	return s + strings.Repeat(string(pad), minWidth-len(s))
}

func readPositionSpecifier(in []byte) (int, []byte) {
	if len(in) == 0 || !unicode.IsDigit(rune(in[0])) {
		return -1, in
	}

	i := 1
	for i < len(in) {
		switch in[i] {
		case '0', '1', '2', '3', '4', '5', '6', '7', '8', '9':
		case '$':
			n, _ := strconv.Atoi(string(in[0:i]))
			return n, in[i+1:]
		default:
			return -1, in
		}
		i++
	}

	return -1, in
}

func readFormatOptions(in []byte) (*formatOptions, []byte) {
	o := &formatOptions{
		padChar: ' ',
	}
	i := 0
	for i < len(in) {
		switch c := in[i]; c {
		case '-':
			o.leftJustify = true
		case '+':
			o.signed = true
		case '0', ' ':
			o.padChar = c
		case '\'':
			if i+1 >= len(in) {
				return nil, nil
			}
			i++
			o.padChar = in[i]
		default:
			return o, in[i:]
		}
		i++
	}
	return nil, nil
}

func readFormatWidth(in []byte) (int, int, []byte) {
	if len(in) == 0 {
		return -1, -1, in
	}

	i := 0

	for unicode.IsDigit(rune(in[i])) {
		i++
		if i >= len(in) {
			return -1, -1, nil
		}
	}
	width, _ := strconv.ParseInt(string(in[:i]), 10, 32)
	precision := int64(-1)

	in = in[i:]
	if in[0] == '.' {
		i = 1
		for unicode.IsDigit(rune(in[i])) {
			i++
			if i >= len(in) {
				return -1, -1, nil
			}
		}
		precision, _ = strconv.ParseInt(string(in[1:i]), 10, 32)
		in = in[i:]
	}

	return int(width), int(precision), in
}

// Zprintf implements printf with zvals
func ZFprintf(ctx phpv.Context, w printfWriter, format phpv.ZString, arg ...*phpv.ZVal) (int, error) {
	var err error
	var bytesWritten counter
	in := []byte(format)
	argp := 0

	defaultPrecision := 6

	for {
		p := bytes.IndexByte(in, '%')
		if p == -1 {
			// no more %, write the rest of the string
			err = bytesWritten.Add(w.Write(in))
			goto Return
		}

		err = bytesWritten.Add(w.Write(in[:p]))
		if err != nil {
			goto Return
		}
		in = in[p:]

		if len(in) < 2 {
			// string ends in a '%'
			err = bytesWritten.Add(w.Write(in))
			if err != nil {
				goto Return
			}
		}

		if in[1] == '%' {
			// escaped '%'
			err = w.WriteByte('%')
			if err != nil {
				goto Return
			}
			bytesWritten.Value++
			in = in[2:]
			continue
		}

		if len(arg) <= argp {
			// argument not found
			return bytesWritten.Value, ctx.Warn("Too few arguments")
		}

		var posSpec int
		posSpec, in = readPositionSpecifier(in[1:])
		if posSpec == 0 {
			return bytesWritten.Value, ctx.Warn("Argument number must be greater than zero")
		} else if posSpec-1 >= len(arg) {
			return bytesWritten.Value, ctx.Warn("Too few arguments")
		}

		var options *formatOptions
		var minWidth, precision int
		options, in = readFormatOptions(in)
		if options == nil {
			goto Return
		}
		minWidth, precision, in = readFormatWidth(in)
		if minWidth < 0 {
			goto Return
		}

		var v *phpv.ZVal
		if posSpec > 0 {
			v = arg[posSpec-1]
		} else {
			v = arg[argp]
			argp++
		}

		floatPrecision := defaultPrecision
		if precision >= 0 {
			floatPrecision = precision
		}

		fChar := in[0]
		in = in[1:]

		signed := false
		var output string
		switch fChar {
		case 'b': // binary
			// next arg is an int
			v, err = v.As(ctx, phpv.ZtInt)
			if err != nil {
				goto Return
			}
			output = strconv.FormatInt(int64(v.Value().(phpv.ZInt)), 2)
		case 'c':
			// next arg is an int, but will be added as a single char
			v, err = v.As(ctx, phpv.ZtInt)
			if err != nil {
				goto Return
			}
			b := byte(int(v.Value().(phpv.ZInt)))
			output = string(b)
		case 'd':
			signed = true
			// next arg is an int
			v, err = v.As(ctx, phpv.ZtInt)
			if err != nil {
				goto Return
			}
			output = strconv.FormatInt(int64(v.Value().(phpv.ZInt)), 10)
		case 'e', 'E', 'g', 'G':
			signed = true
			// next arg is a float
			v, err = v.As(ctx, phpv.ZtFloat)
			if err != nil {
				goto Return
			}
			// this format option is not affected by the ini config precision
			expPrecision := 6

			// In Go, the exponent has a leading 0 if it's less than 10
			//   Go:  1.123456E+01
			//   PHP: 1.123456E+1
			output = strconv.FormatFloat(float64(v.Value().(phpv.ZFloat)), fChar, expPrecision, 64)
			plusIndex := strings.LastIndexByte(output, '+')
			if plusIndex >= 0 && plusIndex < len(output)-1 && output[plusIndex+1] == '0' {
				// this code removes that leading 0, so the tests
				// can pass, but probably could be removed later
				// since it's not that important.
				output = output[0:plusIndex+1] + output[plusIndex+2:]
			}
		case 'f', 'F':
			signed = true
			// next arg is a float
			// TODO: f is locale aware, F is not
			v, err = v.As(ctx, phpv.ZtFloat)
			if err != nil {
				goto Return
			}
			output = strconv.FormatFloat(float64(v.Value().(phpv.ZFloat)), 'f', floatPrecision, 64)
		case 'o':
			// next arg is an int
			v, err = v.As(ctx, phpv.ZtInt)
			if err != nil {
				goto Return
			}
			output = strconv.FormatUint(uint64(v.Value().(phpv.ZInt)), 8)
		case 's':
			// next arg is a string
			v, err = v.As(ctx, phpv.ZtString)
			if err != nil {
				goto Return
			}
			output = string(v.Value().(phpv.ZString))
			if precision >= 0 {
				output = output[:precision]
			}
		case 'u':
			// next arg is an int
			v, err = v.As(ctx, phpv.ZtInt)
			if err != nil {
				goto Return
			}
			output = strconv.FormatUint(uint64(v.Value().(phpv.ZInt)), 10)
		case 'x':
			// next arg is an int
			v, err = v.As(ctx, phpv.ZtInt)
			if err != nil {
				goto Return
			}
			output = strconv.FormatInt(int64(v.Value().(phpv.ZInt)), 16)
		case 'X':
			// next arg is an int
			v, err = v.As(ctx, phpv.ZtInt)
			if err != nil {
				goto Return
			}
			output = strconv.FormatInt(int64(v.Value().(phpv.ZInt)), 16)
			output = strings.ToUpper(output)
		}

		if options.signed && signed && len(output) > 0 && output[0] != '-' {
			output = "+" + output
		}

		if len(output) < minWidth {
			if !options.leftJustify {
				output = padLeft(output, minWidth, options.padChar)
			} else {
				output = padRight(output, minWidth, options.padChar)
			}
		}

		err = bytesWritten.Add(w.WriteString(output))
		if err != nil {
			goto Return
		}
	}

Return:
	return bytesWritten.Value, err
}

func Zprintf(ctx phpv.Context, fmt phpv.ZString, arg ...*phpv.ZVal) (*phpv.ZVal, error) {
	buf := new(bytes.Buffer)
	_, err := ZFprintf(ctx, buf, fmt, arg...)
	if err != nil {
		return nil, err
	}
	return phpv.ZString(buf.String()).ZVal(), nil
}
