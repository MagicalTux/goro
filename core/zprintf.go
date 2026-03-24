package core

import (
	"bytes"
	"fmt"
	"io"
	"math"
	"strconv"
	"strings"
	"unicode"

	"github.com/MagicalTux/goro/core/locale"
	"github.com/MagicalTux/goro/core/phpobj"
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
	if len(in) == 0 {
		return -1, in
	}

	// Handle $s without preceding digits (e.g., %$s) - treat as position 0
	if in[0] == '$' {
		return 0, in[1:]
	}

	if !unicode.IsDigit(rune(in[0])) {
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

var errMissingPadChar = fmt.Errorf("Missing padding character")

func readFormatOptions(in []byte) (*formatOptions, []byte, error) {
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
				return nil, nil, errMissingPadChar
			}
			i++
			o.padChar = in[i]
		default:
			return o, in[i:], nil
		}
		i++
	}
	return nil, nil, nil
}

// formatWidthResult holds the parsed width/precision and whether they use star (*) specifiers
type formatWidthResult struct {
	width     int
	precision int
	widthStar bool // width uses * (take from argument)
	precStar  bool // precision uses * (take from argument)
	// Position specifiers for star args (e.g., %*2$d means width from arg 2)
	widthStarPos int // 0 = sequential, >0 = positional
	precStarPos  int // 0 = sequential, >0 = positional
}

func readFormatWidth(in []byte) (formatWidthResult, []byte) {
	r := formatWidthResult{width: 0, precision: -1}
	if len(in) == 0 {
		return r, in
	}

	// Check for star width
	if in[0] == '*' {
		r.widthStar = true
		in = in[1:]
		// Check for positional specifier after star: *2$
		if len(in) > 0 {
			pos, rest := readStarPositionSpecifier(in)
			if pos > 0 {
				r.widthStarPos = pos
				in = rest
			}
		}
	} else {
		// Parse numeric width
		i := 0
		for i < len(in) && unicode.IsDigit(rune(in[i])) {
			i++
		}
		if i > 0 {
			w, _ := strconv.ParseInt(string(in[:i]), 10, 64)
			r.width = int(w)
			in = in[i:]
		}
	}

	if len(in) == 0 {
		r.width = -1
		r.precision = -1
		return r, nil
	}

	// Check for precision
	if in[0] == '.' {
		in = in[1:]
		if len(in) == 0 {
			r.width = -1
			r.precision = -1
			return r, nil
		}
		// Check for star precision
		if in[0] == '*' {
			r.precStar = true
			in = in[1:]
			// Check for positional specifier after star: .*2$
			if len(in) > 0 {
				pos, rest := readStarPositionSpecifier(in)
				if pos > 0 {
					r.precStarPos = pos
					in = rest
				}
			}
		} else {
			// Parse numeric precision
			i := 0
			for i < len(in) && unicode.IsDigit(rune(in[i])) {
				i++
			}
			if i > 0 {
				p, _ := strconv.ParseInt(string(in[:i]), 10, 64)
				r.precision = int(p)
			} else {
				r.precision = 0
			}
			in = in[i:]
		}
	}

	if len(in) == 0 {
		r.width = -1
		r.precision = -1
		return r, nil
	}

	return r, in
}

// readStarPositionSpecifier reads a positional specifier after a star: e.g. "2$" returns (2, rest)
func readStarPositionSpecifier(in []byte) (int, []byte) {
	i := 0
	for i < len(in) && unicode.IsDigit(rune(in[i])) {
		i++
	}
	if i > 0 && i < len(in) && in[i] == '$' {
		n, _ := strconv.Atoi(string(in[:i]))
		return n, in[i+1:]
	}
	return 0, in
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

		var posSpec int
		posSpec, in = readPositionSpecifier(in[1:])
		if posSpec == 0 || posSpec >= 2147483647 {
			return bytesWritten.Value, phpobj.ThrowError(ctx, phpobj.ValueError, "Argument number specifier must be greater than zero and less than 2147483647")
		} else if posSpec > 0 && posSpec-1 >= len(arg) {
			return bytesWritten.Value, phpobj.ThrowError(ctx, phpobj.ArgumentCountError, fmt.Sprintf("%d arguments are required, %d given", posSpec+1, len(arg)+1))
		}
		if posSpec < 0 && len(arg) <= argp {
			// argument not found (sequential mode)
			return bytesWritten.Value, phpobj.ThrowError(ctx, phpobj.ArgumentCountError, fmt.Sprintf("%d arguments are required, %d given", argp+2, len(arg)+1))
		}

		var options *formatOptions
		var fmtErr error
		options, in, fmtErr = readFormatOptions(in)
		if fmtErr == errMissingPadChar {
			return bytesWritten.Value, phpobj.ThrowError(ctx, phpobj.ValueError, "Missing padding character")
		}
		if options == nil {
			goto Return
		}
		var fmtWidth formatWidthResult
		fmtWidth, in = readFormatWidth(in)
		if fmtWidth.width < 0 {
			goto Return
		}

		// Resolve star width from arguments
		minWidth := fmtWidth.width
		if fmtWidth.widthStar {
			var widthVal *phpv.ZVal
			if fmtWidth.widthStarPos > 0 {
				if fmtWidth.widthStarPos-1 >= len(arg) {
					return bytesWritten.Value, phpobj.ThrowError(ctx, phpobj.ArgumentCountError, fmt.Sprintf("%d arguments are required, %d given", fmtWidth.widthStarPos+1, len(arg)+1))
				}
				widthVal = arg[fmtWidth.widthStarPos-1]
			} else {
				if posSpec < 0 {
					if argp >= len(arg) {
						return bytesWritten.Value, phpobj.ThrowError(ctx, phpobj.ArgumentCountError, fmt.Sprintf("%d arguments are required, %d given", argp+2, len(arg)+1))
					}
					widthVal = arg[argp]
					argp++
				} else {
					// Star with positional format arg: use next sequential arg
					if argp >= len(arg) {
						return bytesWritten.Value, phpobj.ThrowError(ctx, phpobj.ArgumentCountError, fmt.Sprintf("%d arguments are required, %d given", argp+2, len(arg)+1))
					}
					widthVal = arg[argp]
					argp++
				}
			}
			wv, convErr := widthVal.As(ctx, phpv.ZtInt)
			if convErr != nil {
				return bytesWritten.Value, phpobj.ThrowError(ctx, phpobj.ValueError, "Width must be an integer")
			}
			w := int(wv.Value().(phpv.ZInt))
			if w < 0 || w > 2147483646 {
				return bytesWritten.Value, phpobj.ThrowError(ctx, phpobj.ValueError, "Width must be between 0 and 2147483647")
			}
			minWidth = w
		}

		// Resolve star precision from arguments
		precision := fmtWidth.precision
		if fmtWidth.precStar {
			var precVal *phpv.ZVal
			if fmtWidth.precStarPos > 0 {
				if fmtWidth.precStarPos-1 >= len(arg) {
					return bytesWritten.Value, phpobj.ThrowError(ctx, phpobj.ArgumentCountError, fmt.Sprintf("%d arguments are required, %d given", fmtWidth.precStarPos+1, len(arg)+1))
				}
				precVal = arg[fmtWidth.precStarPos-1]
			} else {
				if posSpec < 0 {
					if argp >= len(arg) {
						return bytesWritten.Value, phpobj.ThrowError(ctx, phpobj.ArgumentCountError, fmt.Sprintf("%d arguments are required, %d given", argp+2, len(arg)+1))
					}
					precVal = arg[argp]
					argp++
				} else {
					if argp >= len(arg) {
						return bytesWritten.Value, phpobj.ThrowError(ctx, phpobj.ArgumentCountError, fmt.Sprintf("%d arguments are required, %d given", argp+2, len(arg)+1))
					}
					precVal = arg[argp]
					argp++
				}
			}
			pv, convErr := precVal.As(ctx, phpv.ZtInt)
			if convErr != nil {
				return bytesWritten.Value, phpobj.ThrowError(ctx, phpobj.ValueError, "Precision must be an integer")
			}
			precision = int(pv.Value().(phpv.ZInt))
		}

		var v *phpv.ZVal
		if posSpec > 0 {
			v = arg[posSpec-1]
		} else {
			if argp >= len(arg) {
				return bytesWritten.Value, phpobj.ThrowError(ctx, phpobj.ArgumentCountError, fmt.Sprintf("%d arguments are required, %d given", argp+2, len(arg)+1))
			}
			v = arg[argp]
			argp++
		}

		floatPrecision := defaultPrecision
		if precision >= 0 {
			floatPrecision = precision
		}

		fChar := in[0]
		in = in[1:]

		// Skip 'l' length modifier (no-op in PHP, C compatibility)
		if fChar == 'l' && len(in) > 0 {
			fChar = in[0]
			in = in[1:]
		}

		signed := false
		var output string
		switch fChar {
		case 'b': // binary
			// next arg is an int (unsigned representation, like PHP)
			v, err = v.As(ctx, phpv.ZtInt)
			if err != nil {
				goto Return
			}
			output = strconv.FormatUint(uint64(v.Value().(phpv.ZInt)), 2)
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
		case 'e', 'E', 'g', 'G', 'h', 'H':
			signed = true
			// next arg is a float
			v, err = v.As(ctx, phpv.ZtFloat)
			if err != nil {
				goto Return
			}
			f := float64(v.Value().(phpv.ZFloat))
			if math.IsInf(f, 1) {
				output = "INF"
			} else if math.IsInf(f, -1) {
				output = "-INF"
			} else if math.IsNaN(f) {
				output = "NaN"
			} else {
				// Use precision if specified, otherwise default to 6
				expPrecision := 6
				if precision >= 0 {
					expPrecision = precision
				}

				// Map format character to Go format
				goFmt := fChar
				if fChar == 'h' {
					goFmt = 'g'
				} else if fChar == 'H' {
					goFmt = 'G'
				}

				// precision -1 means "as many digits as needed" (round-trip accuracy)
				if precision == -1 && (fChar == 'g' || fChar == 'G' || fChar == 'h' || fChar == 'H') {
					expPrecision = -1
				}

				// In Go, the exponent has a leading 0 if it's less than 10
				//   Go:  1.123456E+01
				//   PHP: 1.123456E+1
				output = strconv.FormatFloat(f, byte(goFmt), expPrecision, 64)
				plusIndex := strings.LastIndexByte(output, '+')
				if plusIndex >= 0 && plusIndex < len(output)-1 && output[plusIndex+1] == '0' {
					output = output[0:plusIndex+1] + output[plusIndex+2:]
				}
				// Also strip leading zero from negative exponents
				minusIndex := strings.LastIndexByte(output, '-')
				if minusIndex > 0 && minusIndex < len(output)-1 && output[minusIndex-1] == 'e' || (minusIndex > 0 && minusIndex < len(output)-1 && output[minusIndex-1] == 'E') {
					if minusIndex+1 < len(output) && output[minusIndex+1] == '0' {
						output = output[0:minusIndex+1] + output[minusIndex+2:]
					}
				}
			}
		case 'f', 'F':
			signed = true
			// next arg is a float
			// 'f' is locale aware, 'F' is not
			v, err = v.As(ctx, phpv.ZtFloat)
			if err != nil {
				goto Return
			}
			f := float64(v.Value().(phpv.ZFloat))
			if math.IsInf(f, 1) {
				output = "INF"
			} else if math.IsInf(f, -1) {
				output = "-INF"
			} else if math.IsNaN(f) {
				output = "NaN"
			} else {
				output = strconv.FormatFloat(f, 'f', floatPrecision, 64)
				if fChar == 'f' {
					lc := locale.Localeconv()
					if lc.DecimalPoint != "" && lc.DecimalPoint != "." {
						output = strings.Replace(output, ".", lc.DecimalPoint, 1)
					}
				}
			}
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
			if precision >= 0 && precision < len(output) {
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
			// next arg is an int (unsigned representation, like PHP)
			v, err = v.As(ctx, phpv.ZtInt)
			if err != nil {
				goto Return
			}
			output = strconv.FormatUint(uint64(v.Value().(phpv.ZInt)), 16)
		case 'X':
			// next arg is an int (unsigned representation, like PHP)
			v, err = v.As(ctx, phpv.ZtInt)
			if err != nil {
				goto Return
			}
			output = strconv.FormatUint(uint64(v.Value().(phpv.ZInt)), 16)
			output = strings.ToUpper(output)
		}

		if options.signed && signed && len(output) > 0 && output[0] != '-' {
			output = "+" + output
		}

		if len(output) < minWidth {
			if !options.leftJustify {
				// When zero-padding, the sign must come before the zeros
				if options.padChar == '0' && len(output) > 0 && (output[0] == '-' || output[0] == '+') {
					sign := output[0:1]
					rest := output[1:]
					output = sign + padLeft(rest, minWidth-1, '0')
				} else {
					output = padLeft(output, minWidth, options.padChar)
				}
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
