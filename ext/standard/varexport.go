package standard

import (
	"bytes"
	"fmt"
	"io"
	"strings"
	"unsafe"

	"github.com/MagicalTux/goro/core"
	"github.com/MagicalTux/goro/core/phpobj"
	"github.com/MagicalTux/goro/core/phpv"
)

// > func mixed var_export ( mixed $expression  [, $return = FALSE ] )
func stdFuncVarExport(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var expr *phpv.ZVal
	var returnOutput *phpv.ZBool
	_, err := core.Expand(ctx, args, &expr, &returnOutput)
	if err != nil {
		return nil, ctx.Error(err)
	}

	if returnOutput != nil && *returnOutput {
		var buf bytes.Buffer
		doVarExport(ctx, &buf, expr, "", nil)
		return phpv.ZStr(buf.String()), nil
	} else {
		doVarExport(ctx, ctx, expr, "", nil)
		return phpv.ZNULL.ZVal(), nil
	}
}

func doVarExport(ctx phpv.Context, w io.Writer, z *phpv.ZVal, linePfx string, recurs map[uintptr]bool) error {
	if recurs == nil {
		recurs = make(map[uintptr]bool)
	} else {
		// duplicate
		n := make(map[uintptr]bool)
		for k, v := range recurs {
			n[k] = v
		}
		recurs = n
	}

	p := uintptr(unsafe.Pointer(z))
	if _, n := recurs[p]; n {
		if err := ctx.Warn("does not handle circular references"); err != nil {
			return err
		}
		fmt.Fprintf(w, "NULL")
		return nil
	}

	switch z.GetType() {
	case phpv.ZtNull:
		fmt.Fprintf(w, "NULL")
	case phpv.ZtBool:
		if z.Value().(phpv.ZBool) {
			fmt.Fprintf(w, "true")
		} else {
			fmt.Fprintf(w, "false")
		}
	case phpv.ZtInt:
		fmt.Fprintf(w, "%d", z.Value())
	case phpv.ZtFloat:
		p := phpv.GetSerializePrecision(ctx)
		s := phpv.FormatFloatPrecision(float64(z.Value().(phpv.ZFloat)), p)
		// var_export always needs a decimal point so the output is valid PHP float
		if !strings.Contains(s, ".") && !strings.Contains(s, "E") && s != "INF" && s != "-INF" && s != "NAN" {
			s += ".0"
		}
		fmt.Fprintf(w, "%s", s)
	case phpv.ZtString:
		s := z.Value().(phpv.ZString)
		fmt.Fprintf(w, "%s", varExportString(string(s)))
	case phpv.ZtArray:
		p := uintptr(unsafe.Pointer(z))
		recurs[p] = true

		fmt.Fprintf(w, "array (\n")
		localPfx := linePfx + "  "
		it := z.NewIterator()
		for {
			if !it.Valid(ctx) {
				break
			}
			k, err := it.Key(ctx)
			if err != nil {
				return err
			}
			if k.GetType() == phpv.ZtInt {
				fmt.Fprintf(w, "%s%s => ", localPfx, k)
			} else {
				fmt.Fprintf(w, "%s%s => ", localPfx, varExportString(k.String()))
			}
			v, err := it.Current(ctx)
			if err != nil {
				return err
			}

			doVarExport(ctx, w, v, localPfx, recurs)
			fmt.Fprintf(w, ",\n")
			it.Next(ctx)
		}
		fmt.Fprintf(w, "%s)", linePfx)
	case phpv.ZtObject:
		p := uintptr(unsafe.Pointer(z))
		recurs[p] = true

		v := z.Value()
		// Check if this is an enum case - var_export prints \ClassName::CaseName
		if obj, ok := v.(*phpobj.ZObject); ok && obj.GetClass().GetType()&phpv.ZClassTypeEnum != 0 {
			caseName := obj.HashTable().GetString("name")
			if caseName != nil {
				fmt.Fprintf(w, "\\%s::%s", obj.Class.GetName(), caseName.String())
				return nil
			}
		}

		if obj, ok := v.(*phpobj.ZObject); ok {
			fmt.Fprintf(w, "\n%s%s::__set_state(array(\n", linePfx, obj.Class.GetName())
		} else {
			fmt.Fprintf(w, "%sarray(\n", linePfx)
		}

		localPfx := linePfx + "  "
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
				if k.GetType() == phpv.ZtInt {
					fmt.Fprintf(w, "%s%s => ", localPfx, k)
				} else {
					fmt.Fprintf(w, "%s%s => ", localPfx, varExportString(k.String()))
				}
				v, err := it.Current(ctx)
				if err != nil {
					return err
				}

				doVarExport(ctx, w, v, localPfx, recurs)
				fmt.Fprintf(w, ",\n")
				it.Next(ctx)
			}
		}

		if _, ok := v.(*phpobj.ZObject); ok {
			fmt.Fprintf(w, "%s))", linePfx)
		} else {
			fmt.Fprintf(w, "%s)", linePfx)
		}
	default:
		fmt.Fprintf(w, "// Unknown[%T]:%+v\n", z.Value(), z.Value())
	}
	return nil
}

// varExportString formats a string for var_export output, handling NUL bytes
// and single quote escaping. NUL bytes are output as ” . "\0" . ” concatenation.
func varExportString(s string) string {
	if !strings.Contains(s, "\x00") {
		return "'" + strings.ReplaceAll(s, `'`, `\'`) + "'"
	}
	parts := strings.Split(s, "\x00")
	var result strings.Builder
	for i, part := range parts {
		if i > 0 {
			result.WriteString(" . \"\\0\" . ")
		}
		result.WriteString("'")
		result.WriteString(strings.ReplaceAll(part, `'`, `\'`))
		result.WriteString("'")
	}
	return result.String()
}
