package core

import (
	"bytes"
	"strconv"
)

// Zprintf implements printf with zvals
func Zprintf(ctx Context, fmt ZString, arg ...*ZVal) (*ZVal, error) {
	var r []byte
	var err error
	in := []byte(fmt)
	argp := 0

	defaultPrecision := int(ctx.GetConfig("precision", ZInt(14).ZVal()).AsInt(ctx))

	for {
		p := bytes.IndexByte(in, '%')
		if p == -1 {
			if r == nil {
				// no format
				return fmt.ZVal(), nil
			}
			r = append(r, in...)
			return ZString(r).ZVal(), nil
		}
		r = append(r, in[:p]...)
		in = in[p:]

		if len(in) < 2 {
			// string ends in a '%'
			r = append(r, in...)
			return ZString(r).ZVal(), nil
		}

		if in[1] == '%' {
			// escaped '%'
			r = append(r, '%')
			in = in[2:]
			continue
		}

		// TODO support Position specifier

		if len(arg) <= argp {
			// argument not found
			// trigger warning: sprintf(): Too few arguments
			return ZBool(false).ZVal(), nil
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
			v, err = v.As(ctx, ZtInt)
			if err != nil {
				return nil, err
			}
			r = strconv.AppendInt(r, int64(v.Value().(ZInt)), 2)
		case 'c':
			// next arg is an int, but will be added as a single char
			v, err = v.As(ctx, ZtInt)
			if err != nil {
				return nil, err
			}
			r = append(r, byte(v.Value().(ZInt)))
		case 'd':
			// next arg is an int
			v, err = v.As(ctx, ZtInt)
			if err != nil {
				return nil, err
			}
			r = strconv.AppendInt(r, int64(v.Value().(ZInt)), 10)
		case 'e', 'E', 'g', 'G':
			// next arg is a float
			v, err = v.As(ctx, ZtFloat)
			if err != nil {
				return nil, err
			}
			r = strconv.AppendFloat(r, float64(v.Value().(ZFloat)), fChar, floatPrecision, 64)
		case 'f', 'F':
			// next arg is a float
			// TODO: f is locale aware, F is not
			v, err = v.As(ctx, ZtFloat)
			if err != nil {
				return nil, err
			}
			r = strconv.AppendFloat(r, float64(v.Value().(ZFloat)), 'f', floatPrecision, 64)
		case 'o':
			// next arg is an int
			v, err = v.As(ctx, ZtInt)
			if err != nil {
				return nil, err
			}
			r = strconv.AppendInt(r, int64(v.Value().(ZInt)), 8)
		case 's':
			// next arg is a string
			v, err = v.As(ctx, ZtString)
			if err != nil {
				return nil, err
			}
			r = append(r, []byte(v.Value().(ZString))...)
			// TODO add more
		case 'u':
			// next arg is an int
			v, err = v.As(ctx, ZtInt)
			if err != nil {
				return nil, err
			}
			r = strconv.AppendUint(r, uint64(v.Value().(ZInt)), 8)
		case 'x':
			// next arg is an int
			v, err = v.As(ctx, ZtInt)
			if err != nil {
				return nil, err
			}
			r = strconv.AppendInt(r, int64(v.Value().(ZInt)), 16)
		}
	}
}
