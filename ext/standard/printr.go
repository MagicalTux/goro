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

// > func mixed print_r ( mixed $expression [, bool $return = FALSE ] )
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

	switch z.GetType() {
	case phpv.ZtArray, phpv.ZtObject:
		v := uintptr(unsafe.Pointer(z))
		if _, n := recurs[v]; n {
			fmt.Fprintf(ctx, "%s*RECURSION*\n", linePfx)
			return nil
		} else {
			recurs[v] = true
		}
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
		// Special handling for enum cases
		if obj, ok := v.(*phpobj.ZObject); ok && obj.GetClass().GetType()&phpv.ZClassTypeEnum != 0 {
			zc := obj.GetClass().(*phpobj.ZClass)
			// Format: "ClassName Enum[:backingType]\n(\n    [name] => CaseName\n    [value] => BackingValue\n)\n"
			header := string(obj.GetClass().GetName()) + " Enum"
			if zc.EnumBackingType == phpv.ZtInt {
				header += ":int"
			} else if zc.EnumBackingType == phpv.ZtString {
				header += ":string"
			}
			fmt.Fprintf(ctx, "%s%s\n%s(\n", isRef, header, linePfx)
			localPfx := linePfx + "    "
			// Always print name
			nameVal := obj.HashTable().GetString("name")
			if nameVal != nil {
				fmt.Fprintf(ctx, "%s[name] => %s\n", localPfx, nameVal.String())
			}
			// Print value for backed enums
			if zc.EnumBackingType != 0 {
				valVal := obj.HashTable().GetString("value")
				if valVal != nil {
					fmt.Fprintf(ctx, "%s[value] => %s\n", localPfx, valVal.String())
				}
			}
			fmt.Fprintf(ctx, "%s)\n", linePfx)
			return nil
		}
		if obj, ok := v.(*phpobj.ZObject); ok {
			fmt.Fprintf(ctx, "%s%s Object\n%s(\n", isRef, obj.Class.GetName(), linePfx)
			localPfx := linePfx + "    "

			// Check for __debugInfo() method
			var debugInfoArr *phpv.ZArray
			if debugInfoMethod, hasDebugInfo := obj.GetClass().GetMethod("__debuginfo"); hasDebugInfo {
				result, err := ctx.Global().CallZVal(ctx, debugInfoMethod.Method, nil, obj)
				if err == nil && result != nil && !result.IsNull() {
					debugInfoArr = result.AsArray(ctx)
				}
			}

			if debugInfoArr != nil {
				// Use __debugInfo return value
				for key, val := range debugInfoArr.Iterate(ctx) {
					keyStr := key.String()
					fmt.Fprintf(ctx, "%s[%s] => ", localPfx, keyStr)
					doPrintR(ctx, val, localPfx+"    ", recurs)
					ctx.Write([]byte{'\n'})
				}
			} else {
				for prop := range obj.IterProps(ctx) {
					suffix := ""
					switch {
					case prop.Modifiers.IsPrivate():
						className := string(obj.GetDeclClassName(prop))
						suffix = ":" + className + ":private"
					case prop.Modifiers.IsProtected():
						suffix = ":protected"
					}
					fmt.Fprintf(ctx, "%s[%s%s] => ", localPfx, prop.VarName, suffix)
					val := obj.GetPropValue(prop)
					doPrintR(ctx, val, localPfx+"    ", recurs)
					ctx.Write([]byte{'\n'})
				}
			}
			fmt.Fprintf(ctx, "%s)\n", linePfx)
		} else {
			fmt.Fprintf(ctx, "%s? object(?)\n%s(\n", isRef, linePfx)
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
					val, err := it.Current(ctx)
					if err != nil {
						return err
					}
					doPrintR(ctx, val, localPfx+"    ", recurs)
					ctx.Write([]byte{'\n'})
					it.Next(ctx)
				}
			}
			fmt.Fprintf(ctx, "%s)\n", linePfx)
		}
	default:
		z, _ = z.As(ctx, phpv.ZtString)
		fmt.Fprintf(ctx, "%s", z)
	}
	return nil
}
