package standard

import (
	"fmt"
	"unsafe"

	"github.com/MagicalTux/goro/core/phpobj"
	"github.com/MagicalTux/goro/core/phpv"
)

// > func void var_dump ( mixed $expression [, mixed $... ] )
func stdFuncVarDump(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	for _, z := range args {
		err := doVarDump(ctx, z, "", nil)
		if err != nil {
			return nil, err
		}
	}
	return nil, nil
}

func doVarDump(ctx phpv.Context, z *phpv.ZVal, linePfx string, recurs map[uintptr]bool) error {
	var isRef string
	if z.IsRef() && z.GetType() != phpv.ZtObject && linePfx != "" {
		isRef = "&"
	}

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

	v := uintptr(unsafe.Pointer(z))
	if _, n := recurs[v]; n {
		fmt.Fprintf(ctx, "%s*RECURSION*\n", linePfx)
		return nil
	} else {
		recurs[v] = true
	}

	switch z.GetType() {
	case phpv.ZtNull:
		fmt.Fprintf(ctx, "%s%sNULL\n", linePfx, isRef)
	case phpv.ZtBool:
		if z.Value().(phpv.ZBool) {
			fmt.Fprintf(ctx, "%s%sbool(true)\n", linePfx, isRef)
		} else {
			fmt.Fprintf(ctx, "%s%sbool(false)\n", linePfx, isRef)
		}
	case phpv.ZtInt:
		fmt.Fprintf(ctx, "%s%sint(%d)\n", linePfx, isRef, z.Value())
	case phpv.ZtFloat:
		p := phpv.GetSerializePrecision(ctx)
		s := phpv.FormatFloatPrecision(float64(z.Value().(phpv.ZFloat)), p)
		fmt.Fprintf(ctx, "%s%sfloat(%s)\n", linePfx, isRef, s)
	case phpv.ZtString:
		s := z.Value().(phpv.ZString)
		fmt.Fprintf(ctx, "%s%sstring(%d) \"%s\"\n", linePfx, isRef, len(s), s)
	case phpv.ZtArray:
		c := z.Value().(phpv.ZCountable).Count(ctx)
		fmt.Fprintf(ctx, "%s%sarray(%d) {\n", linePfx, isRef, c)
		localPfx := linePfx + "  "
		it := z.NewIterator()
		type refIterator interface {
			CurrentRef(phpv.Context) (*phpv.ZVal, error)
		}
		ri, hasRef := it.(refIterator)
		for {
			if !it.Valid(ctx) {
				break
			}
			// Check deadline during long iterations
			if err := ctx.Tick(ctx, nil); err != nil {
				return err
			}
			k, err := it.Key(ctx)
			if err != nil {
				return err
			}
			if k.GetType() == phpv.ZtInt {
				fmt.Fprintf(ctx, "%s[%s]=>\n", localPfx, k)
			} else {
				fmt.Fprintf(ctx, "%s[\"%s\"]=>\n", localPfx, k)
			}
			var v *phpv.ZVal
			if hasRef {
				v, err = ri.CurrentRef(ctx)
			} else {
				v, err = it.Current(ctx)
			}
			if err != nil {
				return err
			}
			doVarDump(ctx, v, localPfx, recurs)
			it.Next(ctx)
		}
		fmt.Fprintf(ctx, "%s}\n", linePfx)
	case phpv.ZtObject:
		v := z.Value()

		// Special handling for enum cases: display as enum(ClassName::CaseName)
		if obj, ok := v.(*phpobj.ZObject); ok {
			if obj.GetClass().GetType()&phpv.ZClassTypeEnum != 0 {
				// Get the case name from the "name" property
				caseName := ""
				if nameVal, err := obj.ObjectGet(ctx, phpv.ZString("name")); err == nil && nameVal != nil {
					caseName = nameVal.String()
				}
				fmt.Fprintf(ctx, "%s%senum(%s::%s)\n", linePfx, isRef, obj.GetClass().GetName(), caseName)
				return nil
			}
		}

		// Check for __debugInfo() method - if present, use its return value
		var debugInfoArr *phpv.ZArray
		if obj, ok := v.(*phpobj.ZObject); ok {
			if debugInfoMethod, hasDebugInfo := obj.GetClass().GetMethod("__debuginfo"); hasDebugInfo {
				result, err := ctx.Global().CallZVal(ctx, debugInfoMethod.Method, nil, obj)
				if err == nil && result != nil && result.GetType() == phpv.ZtArray {
					debugInfoArr = result.AsArray(ctx)
				}
			}
		}

		if obj, ok := v.(*phpobj.ZObject); ok {
			count := obj.Count(ctx)
			if debugInfoArr != nil {
				count = phpv.ZInt(debugInfoArr.Count(ctx))
			}
			fmt.Fprintf(ctx, "%s%sobject(%s)#%d (%d) {\n", linePfx, isRef, obj.Class.GetName(), obj.ID, count)
		} else if c, ok := v.(phpv.ZCountable); ok {
			fmt.Fprintf(ctx, "%s%sobject(?) (%d) {\n", linePfx, isRef, c.Count(ctx))
		} else {
			fmt.Fprintf(ctx, "%s%sobject(?) (#) {\n", linePfx, isRef)
		}

		localPfx := linePfx + "  "
		if debugInfoArr != nil {
			// Use __debugInfo return value instead of regular properties
			it := debugInfoArr.NewIterator()
			for {
				if !it.Valid(ctx) {
					break
				}
				k, err := it.Key(ctx)
				if err != nil {
					return err
				}
				if k.GetType() == phpv.ZtInt {
					fmt.Fprintf(ctx, "%s[%s]=>\n", localPfx, k)
				} else {
					fmt.Fprintf(ctx, "%s[\"%s\"]=>\n", localPfx, k)
				}
				val, err := it.Current(ctx)
				if err != nil {
					return err
				}
				doVarDump(ctx, val, localPfx, recurs)
				it.Next(ctx)
			}
		} else if obj, ok := v.(*phpobj.ZObject); ok {
			for prop := range obj.IterProps(ctx) {
				suffix := ""
				switch {
				case prop.Modifiers.IsPrivate():
					className := string(obj.GetDeclClassName(prop))
					suffix = `:"` + className + `":private`
				case prop.Modifiers.IsProtected():
					suffix = ":protected"
				}
				fmt.Fprintf(ctx, "%s[\"%s\"%s]=>\n", localPfx, prop.VarName, suffix)

				v := obj.GetPropValue(prop)
				doVarDump(ctx, v, localPfx, recurs)
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
						fmt.Fprintf(ctx, "x%s[%s]=>\n", localPfx, k)
					} else {
						fmt.Fprintf(ctx, "%s[\"%s\"]=>\n", localPfx, k)
					}
					v, err := it.Current(ctx)
					if err != nil {
						return err
					}
					doVarDump(ctx, v, localPfx, recurs)
					it.Next(ctx)
				}
			}
		}
		fmt.Fprintf(ctx, "%s}\n", linePfx)
	case phpv.ZtResource:
		r := z.Value().(phpv.Resource)
		fmt.Fprintf(ctx, "%sresource(%d) of type (%s)\n", linePfx, r.GetResourceID(), r.GetResourceType())
	default:
		fmt.Fprintf(ctx, "Unknown[%T]:%+v\n", z.Value(), z.Value())
	}
	return nil
}
