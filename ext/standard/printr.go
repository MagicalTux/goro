package standard

import (
	"bytes"
	"fmt"
	"unsafe"

	"github.com/MagicalTux/goro/core"
	"github.com/MagicalTux/goro/core/phpctx"
	"github.com/MagicalTux/goro/core/phpobj"
	"github.com/MagicalTux/goro/core/phpv"
)

//> func mixed print_r ( mixed $expression [, bool $return = FALSE ] )
func fncPrintR(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var expr *phpv.ZVal
	var ret *phpv.ZBool
	var b *bytes.Buffer

	_, err := core.Expand(ctx, args, &expr, &ret)
	if err != nil {
		return nil, err
	}

	if ret != nil && *ret {
		// use buffer
		b = &bytes.Buffer{}
		ctx = phpctx.NewBufContext(ctx, b) // grab output
	}

	err = doPrintR(ctx, expr, "", nil)
	if err != nil {
		return nil, err
	}

	if b != nil {
		return phpv.ZString(b.String()).ZVal(), nil
	}
	return phpv.ZBool(true).ZVal(), nil
}

func doPrintR(ctx phpv.Context, z *phpv.ZVal, linePfx string, recurs map[uintptr]bool) error {
	var isRef string
	if z.IsRef() {
		isRef = "&"
	}

	if recurs == nil {
		recurs = make(map[uintptr]bool)
	}

	v := uintptr(unsafe.Pointer(z))
	if _, n := recurs[v]; n {
		fmt.Fprintf(ctx, "%s*RECURSION*\n", linePfx)
		return nil
	} else {
		recurs[v] = true
	}

	switch z.GetType() {
	case phpv.ZtArray:
		fmt.Fprintf(ctx, "%sArray\n%s(\n", isRef, linePfx)
		localPfx := linePfx + "    "
		it := z.NewIterator()
		for {
			if !it.Valid(ctx) {
				break
			}
			k, err := it.Key(ctx)
			if err != nil {
				return err
			}
			fmt.Fprintf(ctx, "%s[%s] => ", localPfx, k)
			v, err := it.Current(ctx)
			if err != nil {
				return err
			}
			doPrintR(ctx, v, localPfx+"    ", recurs)
			ctx.Write([]byte{'\n'})
			it.Next(ctx)
		}
		fmt.Fprintf(ctx, "%s)\n", linePfx)
	case phpv.ZtObject:
		v := z.Value()
		if obj, ok := v.(*phpobj.ZObject); ok {
			fmt.Fprintf(ctx, "%s%s Object\n%s(\n", isRef, obj.Class.GetName(), linePfx)
		} else {
			fmt.Fprintf(ctx, "%s? object(?)\n%s(\n", isRef, linePfx)
		}
		localPfx := linePfx + "    "
		it := z.NewIterator()
		if it != nil {
			for {
				if !it.Valid(ctx) {
					break
				}
				k, err := it.Key(ctx)
				if err != nil {
					return err
				}
				fmt.Fprintf(ctx, "%s[%s] => ", localPfx, k)
				v, err := it.Current(ctx)
				if err != nil {
					return err
				}
				doPrintR(ctx, v, localPfx+"    ", recurs)
				it.Next(ctx)
			}
		}
		fmt.Fprintf(ctx, "%s)\n", linePfx)
	default:
		z, _ = z.As(ctx, phpv.ZtString)
		fmt.Fprintf(ctx, "%s", z)
	}
	return nil
}
