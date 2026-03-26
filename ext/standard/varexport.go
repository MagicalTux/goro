package standard

import (
	"bytes"
	"fmt"
	"io"
	"strings"
	"unsafe"

	"github.com/MagicalTux/goro/core"
	"github.com/MagicalTux/goro/core/logopt"
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

	// For circular reference detection, we need to check the underlying value pointer
	// for arrays and objects, not the ZVal wrapper pointer. When references are used
	// (e.g., $a[] =& $a), the same underlying array/object is wrapped in different ZVal objects.
	var valuePtr uintptr
	switch z.GetType() {
	case phpv.ZtArray:
		valuePtr = uintptr(unsafe.Pointer(z.Value().(*phpv.ZArray)))
	case phpv.ZtObject:
		if obj, ok := z.Value().(*phpobj.ZObject); ok {
			valuePtr = uintptr(unsafe.Pointer(obj))
		} else {
			valuePtr = uintptr(unsafe.Pointer(z))
		}
	default:
		valuePtr = uintptr(unsafe.Pointer(z))
	}

	if _, found := recurs[valuePtr]; found {
		if err := ctx.Warn("var_export does not handle circular references", logopt.NoFuncName(true)); err != nil {
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
		n := int64(z.Value().(phpv.ZInt))
		if n == -9223372036854775808 {
			// PHP_INT_MIN can't be expressed as a literal because the positive form overflows
			fmt.Fprintf(w, "(-9223372036854775807-1)")
		} else {
			fmt.Fprintf(w, "%d", n)
		}
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
		p := uintptr(unsafe.Pointer(z.Value().(*phpv.ZArray)))
		recurs[p] = true

		if linePfx != "" {
			fmt.Fprintf(w, "\n%sarray (\n", linePfx)
		} else {
			fmt.Fprintf(w, "array (\n")
		}
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
		// Lazy objects: var_export triggers initialization
		if obj, ok := z.Value().(*phpobj.ZObject); ok && obj.IsLazy() {
			if err := obj.TriggerLazyInit(ctx); err != nil {
				return err
			}
		}
		// For initialized proxies, use the real instance
		if obj, ok := z.Value().(*phpobj.ZObject); ok && obj.LazyState == phpobj.LazyProxyInitialized && obj.LazyInstance != nil {
			z = obj.LazyInstance.ZVal()
		}

		if obj, ok := z.Value().(*phpobj.ZObject); ok {
			recurs[uintptr(unsafe.Pointer(obj))] = true
		} else {
			recurs[uintptr(unsafe.Pointer(z))] = true
		}

		v := z.Value()
		// Check if this is an enum case - var_export prints \ClassName::CaseName
		// When nested (e.g., inside an array), add a newline and indent prefix
		if obj, ok := v.(*phpobj.ZObject); ok && obj.GetClass().GetType()&phpv.ZClassTypeEnum != 0 {
			caseName := obj.HashTable().GetString("name")
			if caseName != nil {
				if linePfx != "" {
					fmt.Fprintf(w, "\n%s\\%s::%s", linePfx, obj.Class.GetName(), caseName.String())
				} else {
					fmt.Fprintf(w, "\\%s::%s", obj.Class.GetName(), caseName.String())
				}
				return nil
			}
		}

		if obj, ok := v.(*phpobj.ZObject); ok {
			className := obj.Class.GetName()
			if className == "stdClass" {
				if linePfx != "" {
					fmt.Fprintf(w, "\n%s(object) array(\n", linePfx)
				} else {
					fmt.Fprintf(w, "(object) array(\n")
				}
			} else {
				if linePfx != "" {
					fmt.Fprintf(w, "\n%s\\%s::__set_state(array(\n", linePfx, className)
				} else {
					fmt.Fprintf(w, "\\%s::__set_state(array(\n", className)
				}
			}
		} else {
			fmt.Fprintf(w, "%sarray(\n", linePfx)
		}

		localPfx := linePfx + "  "
		if obj, ok := v.(*phpobj.ZObject); ok {
			// Use IterProps to get all properties (including private/protected)
			for prop := range obj.IterProps(ctx) {
				fmt.Fprintf(w, "%s %s => ", localPfx, varExportString(prop.VarName.String()))

				propVal := obj.GetPropValue(prop)
				if propVal == nil {
					propVal = phpv.ZNULL.ZVal()
				}

				doVarExport(ctx, w, propVal, localPfx, recurs)
				fmt.Fprintf(w, ",\n")
			}
		} else {
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
						fmt.Fprintf(w, "%s %s => ", localPfx, k)
					} else {
						fmt.Fprintf(w, "%s %s => ", localPfx, varExportString(k.String()))
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
		}

		if obj, ok := v.(*phpobj.ZObject); ok {
			if obj.Class.GetName() == "stdClass" {
				fmt.Fprintf(w, "%s)", linePfx)
			} else {
				fmt.Fprintf(w, "%s))", linePfx)
			}
		} else {
			fmt.Fprintf(w, "%s)", linePfx)
		}
	default:
		fmt.Fprintf(w, "// Unknown[%T]:%+v\n", z.Value(), z.Value())
	}
	return nil
}

// varExportString formats a string for var_export output, handling NUL bytes,
// single quote escaping, and backslash escaping.
func varExportString(s string) string {
	// PHP's var_export escapes single quotes and backslashes in single-quoted strings
	escapeSingleQuoted := func(s string) string {
		s = strings.ReplaceAll(s, `\`, `\\`)
		s = strings.ReplaceAll(s, "'", `\'`)
		return s
	}

	if !strings.Contains(s, "\x00") {
		return "'" + escapeSingleQuoted(s) + "'"
	}
	parts := strings.Split(s, "\x00")
	var result strings.Builder
	for i, part := range parts {
		if i > 0 {
			result.WriteString(` . "\0" . `)
		}
		result.WriteByte('\'')
		result.WriteString(escapeSingleQuoted(part))
		result.WriteByte('\'')
	}
	return result.String()
}
