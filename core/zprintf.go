package core

import (
	"bytes"
	"io"
	"strconv"
	"strings"

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

// Zprintf implements printf with zvals
func ZFprintf(ctx phpv.Context, w printfWriter, fmt phpv.ZString, arg ...*phpv.ZVal) (int, error) {
	var err error
	var bytesWritten counter
	in := []byte(fmt)
	argp := 0

	defaultPrecision := int(ctx.GetConfig("precision", phpv.ZInt(14).ZVal()).AsInt(ctx))

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

		// TODO support Position specifier

		if len(arg) <= argp {
			// argument not found
			return bytesWritten.Value, ctx.Warn("Too few arguments")
		}

		v := arg[argp]
		argp++

		// TODO support printf format modifiers
		floatPrecision := defaultPrecision

		fChar := in[1]
		in = in[2:]

		switch fChar {
		case 'b': // binary
			// next arg is an int
			v, err = v.As(ctx, phpv.ZtInt)
			if err != nil {
				goto Return
			}
			err = bytesWritten.Add(w.WriteString(strconv.FormatInt(int64(v.Value().(phpv.ZInt)), 2)))
			if err != nil {
				goto Return
			}
		case 'c':
			// next arg is an int, but will be added as a single char
			v, err = v.As(ctx, phpv.ZtInt)
			if err != nil {
				goto Return
			}
			err = w.WriteByte(byte(v.Value().(phpv.ZInt)))
			if err != nil {
				goto Return
			}
			bytesWritten.Value++
		case 'd':
			// next arg is an int
			v, err = v.As(ctx, phpv.ZtInt)
			if err != nil {
				goto Return
			}
			s := strconv.FormatInt(int64(v.Value().(phpv.ZInt)), 10)
			err = bytesWritten.Add(w.WriteString(s))
			if err != nil {
				goto Return
			}
		case 'e', 'E', 'g', 'G':
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
			s := strconv.FormatFloat(float64(v.Value().(phpv.ZFloat)), fChar, expPrecision, 64)
			plusIndex := strings.LastIndexByte(s, '+')
			if plusIndex >= 0 && plusIndex < len(s)-1 && s[plusIndex+1] == '0' {
				// this code removes that leading 0, so the tests
				// can pass, but probably could be removed later
				// since it's not that important.
				err = bytesWritten.Add(w.WriteString(s[0 : plusIndex+1]))
				if err != nil {
					goto Return
				}
				err = bytesWritten.Add(w.WriteString(s[plusIndex+2:]))
				if err != nil {
					goto Return
				}
			} else {
				err = bytesWritten.Add(w.WriteString(s))
				if err != nil {
					goto Return
				}
			}
		case 'f', 'F':
			// next arg is a float
			// TODO: f is locale aware, F is not
			v, err = v.As(ctx, phpv.ZtFloat)
			if err != nil {
				goto Return
			}
			s := strconv.FormatFloat(float64(v.Value().(phpv.ZFloat)), 'f', floatPrecision, 64)
			err = bytesWritten.Add(w.WriteString(s))
			if err != nil {
				goto Return
			}
		case 'o':
			// next arg is an int
			v, err = v.As(ctx, phpv.ZtInt)
			if err != nil {
				goto Return
			}
			s := strconv.FormatUint(uint64(v.Value().(phpv.ZInt)), 8)
			err = bytesWritten.Add(w.WriteString(s))
			if err != nil {
				goto Return
			}
		case 's':
			// next arg is a string
			v, err = v.As(ctx, phpv.ZtString)
			if err != nil {
				goto Return
			}
			s := string(v.Value().(phpv.ZString))
			err = bytesWritten.Add(w.WriteString(s))
			if err != nil {
				goto Return
			}
		case 'u':
			// next arg is an int
			v, err = v.As(ctx, phpv.ZtInt)
			if err != nil {
				goto Return
			}
			s := strconv.FormatUint(uint64(v.Value().(phpv.ZInt)), 10)
			err = bytesWritten.Add(w.WriteString(s))
			if err != nil {
				goto Return
			}
		case 'x':
			// next arg is an int
			v, err = v.As(ctx, phpv.ZtInt)
			if err != nil {
				goto Return
			}
			s := strconv.FormatInt(int64(v.Value().(phpv.ZInt)), 16)
			err = bytesWritten.Add(w.WriteString(s))
			if err != nil {
				goto Return
			}
		case 'X':
			// next arg is an int
			v, err = v.As(ctx, phpv.ZtInt)
			if err != nil {
				goto Return
			}
			s := strconv.FormatInt(int64(v.Value().(phpv.ZInt)), 16)
			s = strings.ToUpper(s)
			err = bytesWritten.Add(w.WriteString(s))
			if err != nil {
				goto Return
			}
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
