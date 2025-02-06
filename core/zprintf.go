package core

import (
	"bytes"
	"strconv"
	"strings"

	"github.com/MagicalTux/goro/core/phpv"
)

// Zprintf implements printf with zvals
func Zprintf(ctx phpv.Context, fmt phpv.ZString, arg ...*phpv.ZVal) (*phpv.ZVal, error) {
	var r []byte
	var err error
	in := []byte(fmt)
	argp := 0

	defaultPrecision := int(ctx.GetConfig("precision", phpv.ZInt(6).ZVal()).AsInt(ctx))

	for {
		p := bytes.IndexByte(in, '%')
		if p == -1 {
			if r == nil {
				// no format
				return fmt.ZVal(), nil
			}
			r = append(r, in...)
			return phpv.ZString(r).ZVal(), nil
		}
		r = append(r, in[:p]...)
		in = in[p:]

		if len(in) < 2 {
			// string ends in a '%'
			r = append(r, in...)
			return phpv.ZString(r).ZVal(), nil
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
			return phpv.ZBool(false).ZVal(), nil
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
				return nil, err
			}
			r = strconv.AppendInt(r, int64(v.Value().(phpv.ZInt)), 2)
		case 'c':
			// next arg is an int, but will be added as a single char
			v, err = v.As(ctx, phpv.ZtInt)
			if err != nil {
				return nil, err
			}
			r = append(r, byte(v.Value().(phpv.ZInt)))
		case 'd':
			// next arg is an int
			v, err = v.As(ctx, phpv.ZtInt)
			if err != nil {
				return nil, err
			}
			r = strconv.AppendInt(r, int64(v.Value().(phpv.ZInt)), 10)
		case 'e', 'E', 'g', 'G':
			// next arg is a float
			v, err = v.As(ctx, phpv.ZtFloat)
			if err != nil {
				return nil, err
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
				r = append(r, s[0:plusIndex+1]...)
				r = append(r, s[plusIndex+2:]...)
			} else {
				r = append(r, []byte(s)...)
			}
		case 'f', 'F':
			// next arg is a float
			// TODO: f is locale aware, F is not
			v, err = v.As(ctx, phpv.ZtFloat)
			if err != nil {
				return nil, err
			}
			r = strconv.AppendFloat(r, float64(v.Value().(phpv.ZFloat)), 'f', floatPrecision, 64)
		case 'o':
			// next arg is an int
			v, err = v.As(ctx, phpv.ZtInt)
			if err != nil {
				return nil, err
			}
			r = strconv.AppendInt(r, int64(v.Value().(phpv.ZInt))&0xFFFFFFFF, 8)
		case 's':
			// next arg is a string
			v, err = v.As(ctx, phpv.ZtString)
			if err != nil {
				return nil, err
			}
			r = append(r, []byte(v.Value().(phpv.ZString))...)
			// TODO add more
		case 'u':
			// next arg is an int
			v, err = v.As(ctx, phpv.ZtInt)
			if err != nil {
				return nil, err
			}
			r = strconv.AppendUint(r, uint64(v.Value().(phpv.ZInt))&0xFFFFFFFF, 10)
		case 'x':
			// next arg is an int
			v, err = v.As(ctx, phpv.ZtInt)
			if err != nil {
				return nil, err
			}
			r = strconv.AppendInt(r, int64(v.Value().(phpv.ZInt))&0xFFFFFFFF, 16)
		case 'X':
			// next arg is an int
			v, err = v.As(ctx, phpv.ZtInt)
			if err != nil {
				return nil, err
			}

			s := strconv.FormatInt(int64(v.Value().(phpv.ZInt))&0xFFFFFFFF, 16)
			s = strings.ToUpper(s)
			r = append(r, []byte(s)...)
		}
	}
}
