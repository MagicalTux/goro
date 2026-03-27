package standard

import (
	"bytes"
	"fmt"
	"strings"
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
	} else {
		// duplicate to avoid side effects across siblings
		n := make(map[uintptr]bool)
		for k, v := range recurs {
			n[k] = v
		}
		recurs = n
	}

	switch z.GetType() {
	case phpv.ZtArray:
		// Track by underlying ZArray pointer for arrays, not ZVal pointer,
		// because references create different ZVal wrappers for the same array
		arrayPtr := uintptr(unsafe.Pointer(z.Value().(*phpv.ZArray)))
		if _, n := recurs[arrayPtr]; n {
			fmt.Fprintf(ctx, "%s*RECURSION*\n", linePfx)
			return nil
		} else {
			recurs[arrayPtr] = true
		}
	case phpv.ZtObject:
		// Track by underlying object pointer for objects
		if obj, ok := z.Value().(*phpobj.ZObject); ok {
			objPtr := uintptr(unsafe.Pointer(obj))
			if _, n := recurs[objPtr]; n {
				fmt.Fprintf(ctx, "%s*RECURSION*\n", linePfx)
				return nil
			} else {
				recurs[objPtr] = true
			}
		} else {
			v := uintptr(unsafe.Pointer(z))
			if _, n := recurs[v]; n {
				fmt.Fprintf(ctx, "%s*RECURSION*\n", linePfx)
				return nil
			} else {
				recurs[v] = true
			}
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
					// Handle PHP's internal property naming convention:
					// \0*\0name -> protected, \0ClassName\0name -> private
					if len(keyStr) > 0 && keyStr[0] == 0 {
						if len(keyStr) > 2 && keyStr[1] == '*' && keyStr[2] == 0 {
							// Protected: \0*\0name -> name:protected
							fmt.Fprintf(ctx, "%s[%s:protected] => ", localPfx, keyStr[3:])
						} else {
							// Private: \0ClassName\0name -> name:ClassName:private
							idx := strings.IndexByte(keyStr[1:], 0)
							if idx >= 0 {
								className := keyStr[1 : idx+1]
								propName := keyStr[idx+2:]
								fmt.Fprintf(ctx, "%s[%s:%s:private] => ", localPfx, propName, className)
							} else {
								fmt.Fprintf(ctx, "%s[%s] => ", localPfx, keyStr)
							}
						}
					} else {
						fmt.Fprintf(ctx, "%s[%s] => ", localPfx, keyStr)
					}
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
