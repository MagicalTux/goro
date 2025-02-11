package core

import (
	"bufio"
	"fmt"
	"io"

	"github.com/MagicalTux/goro/core/phpv"
)

type errBadScanChar struct {
	Code byte
}

func (err *errBadScanChar) Error() string {
	return fmt.Sprintf(`Bad scan conversion character "%c"`, err.Code)
}

func skipWhitespaces(r *bufio.Reader) error {
	for {
		c, err := r.ReadByte()
		if err != nil {
			if err == io.EOF {
				return nil
			}
			return err
		}
		switch c {
		case ' ', '\n', '\t':
		default:
			r.UnreadByte()
			return nil
		}
	}
}

func countRemainingScanCodes(format phpv.ZString, startIndex int) int {
	count := 0
	for i := startIndex; i < len(format); i++ {
		switch format[i] {
		case '%':
			i++
			if i < len(format) {
				count++
			}
		default:
			continue
		}
	}

	return count
}

func zscanRead(r io.Reader, format phpv.ZString) ([]*phpv.ZVal, int, error) {
	buf := bufio.NewReader(r)
	argNum := 0
	var pos int
	failed := false
	result := []*phpv.ZVal{}

Loop:
	for pos = 0; pos < len(format); pos++ {
		c := format[pos]

		switch c {
		case ' ', '\t', '\n':
			err := skipWhitespaces(buf)
			if err != nil {
				return nil, 0, err
			}
			continue
		default:
			var c2 byte
			c2, err := buf.ReadByte()
			if err != nil {
				if err != io.EOF {
					return nil, 0, err
				}
			}
			if c != c2 {
				break Loop
			}

			continue

		case '%':
			// proceed below
		}

		pos++
		if pos >= len(format) {
			break
		}

		err := skipWhitespaces(buf)
		if err != nil {
			return nil, 0, err
		}

		var val *phpv.ZVal
		c = format[pos]

		switch c {
		default:
			return nil, 0, &errBadScanChar{c}
		case 'n':
			val = phpv.ZInt(pos + 1).ZVal()

		case 'c':
			b, err := buf.ReadByte()
			if err != nil {
				failed = true
				break Loop
			}
			val = phpv.ZStr(string(b))
		case 's':
			var word string
			_, err := fmt.Fscanf(buf, "%s", &word)
			if err != nil {
				failed = true
				break Loop
			}
			val = phpv.ZStr(word)

		case 'f', 'e', 'E':
			var n phpv.ZFloat
			_, err := fmt.Fscanf(buf, "%"+string(c), &n)
			if err != nil {
				failed = true
				break Loop
			}
			val = phpv.ZFloat(n).ZVal()
		case 'd', 'o', 'x', 'X':
			var n int
			_, err := fmt.Fscanf(buf, "%"+string(c), &n)
			if err != nil {
				failed = true
				break Loop
			}
			val = phpv.ZInt(n).ZVal()

		case 'i':
			// TODO: copy PHP's scanf base detection
		}

		result = append(result, val)
		argNum++
	}

	if failed {
		// +1 to include failed scan field
		argNum += countRemainingScanCodes(format, pos) + 1
	}

	return result, argNum, nil
}

func zscanfIntoArray(ctx phpv.Context, r io.Reader, format phpv.ZString) (*phpv.ZVal, error) {
	values, count, err := zscanRead(r, format)
	if err != nil {
		if _, ok := err.(*errBadScanChar); ok {
			return nil, ctx.Warn(err.Error())
		}
		return nil, err
	}
	result := phpv.NewZArray()
	for _, v := range values {
		result.OffsetSet(ctx, nil, v)
	}

	for d := count - len(values); d > 0; d-- {
		result.OffsetSet(ctx, nil, phpv.ZNULL.ZVal())
	}

	return result.ZVal(), nil
}

func zscanfIntoRef(ctx phpv.Context, r io.Reader, format phpv.ZString, args ...*phpv.ZVal) (*phpv.ZVal, error) {
	values, count, err := zscanRead(r, format)
	if err != nil {
		if _, ok := err.(*errBadScanChar); ok {
			return nil, ctx.Warn(err.Error())
		}
		return nil, err
	}

	count = countRemainingScanCodes(format, 0)

	if count < len(args) {
		if err = ctx.Warn("Variable is not assigned by any conversion specifiers"); err != nil {
			return nil, err
		}
		return phpv.ZInt(-1).ZVal(), nil
	}

	if count > len(args) {
		if err = ctx.Warn("Different numbers of variable names and field specifiers"); err != nil {
			return nil, err
		}
		return phpv.ZInt(-1).ZVal(), nil
	}

	for i, val := range values {
		varName := args[i].Name
		if varName != nil {
			ctx.Parent(1).OffsetSet(ctx, *varName, val)
		}
	}

	return phpv.ZInt(len(values)).ZVal(), nil
}

func Zscanf(ctx phpv.Context, r io.Reader, format phpv.ZString, args ...*phpv.ZVal) (*phpv.ZVal, error) {
	if len(args) > 0 {
		return zscanfIntoRef(ctx, r, format, args...)
	} else {
		return zscanfIntoArray(ctx, r, format)
	}
}
