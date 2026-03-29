package standard

import (
	"fmt"
	"strings"
	"unsafe"

	"github.com/MagicalTux/goro/core/logopt"
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
	if z.IsRef() && linePfx != "" {
		// PHP shows & prefix for non-objects, and also for enum objects
		if z.GetType() != phpv.ZtObject {
			isRef = "&"
		} else if obj, ok := z.Value().(*phpobj.ZObject); ok && obj.GetClass().GetType()&phpv.ZClassTypeEnum != 0 {
			isRef = "&"
		}
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

	// Track recursion by underlying value pointer for arrays and objects.
	// This prevents infinite recursion when the same array/object is referenced
	// via different ZVals (e.g., references, closures with use(&$self) in __debugInfo).
	switch z.GetType() {
	case phpv.ZtArray:
		arrayPtr := uintptr(unsafe.Pointer(z.Value().(*phpv.ZArray)))
		if _, n := recurs[arrayPtr]; n {
			fmt.Fprintf(ctx, "%s*RECURSION*\n", linePfx)
			return nil
		}
		recurs[arrayPtr] = true
	case phpv.ZtObject:
		if obj, ok := z.Value().(*phpobj.ZObject); ok {
			objPtr := uintptr(unsafe.Pointer(obj))
			if _, n := recurs[objPtr]; n {
				fmt.Fprintf(ctx, "%s*RECURSION*\n", linePfx)
				return nil
			}
			recurs[objPtr] = true
		} else {
			v := uintptr(unsafe.Pointer(z))
			if _, n := recurs[v]; n {
				fmt.Fprintf(ctx, "%s*RECURSION*\n", linePfx)
				return nil
			}
			recurs[v] = true
		}
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
			if _, hasDebugInfo := obj.GetClass().GetMethod("__debuginfo"); hasDebugInfo {
				result, err := obj.CallMethod(ctx, "__debugInfo")
				if err != nil {
					return err
				}
				if result != nil {
					switch result.GetType() {
					case phpv.ZtArray:
						debugInfoArr = result.AsArray(ctx)
					case phpv.ZtNull:
						// Returning null is deprecated since PHP 8.2
						ctx.Deprecated("Returning null from %s::__debugInfo() is deprecated, return an empty array instead", obj.GetClass().GetName(), logopt.NoFuncName(true))
						debugInfoArr = phpv.NewZArray()
					default:
						// Non-array, non-null return is a fatal error
						return &phpv.PhpError{
							Err:  fmt.Errorf("__debuginfo() must return an array"),
							Code: phpv.E_ERROR,
							Loc:  ctx.Loc(),
						}
					}
				}
			}
		}

		if obj, ok := v.(*phpobj.ZObject); ok {
			// Lazy object: special var_dump format
			if obj.LazyState == phpobj.LazyGhostUninitialized {
				// Uninitialized ghost: "lazy ghost object(C)#N (count) { ... }"
				count := obj.Count(ctx)
				fmt.Fprintf(ctx, "%s%slazy ghost object(%s)#%d (%d) {\n", linePfx, isRef, obj.Class.GetName(), obj.ID, count)
				localPfx := linePfx + "  "
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
					val, hasVal, hookErr := obj.GetPropValueOrHook(ctx, prop)
					if hookErr != nil {
						return hookErr
					}
					if hasVal {
						doVarDump(ctx, val, localPfx, recurs)
					} else if prop.TypeHint != nil {
						fmt.Fprintf(ctx, "%suninitialized(%s)\n", localPfx, prop.TypeHint.String())
					} else {
						vv := obj.GetPropValue(prop)
						doVarDump(ctx, vv, localPfx, recurs)
					}
				}
				fmt.Fprintf(ctx, "%s}\n", linePfx)
				return nil
			} else if obj.LazyState == phpobj.LazyProxyUninitialized {
				// Uninitialized proxy: "lazy proxy object(C)#N (count) { ... }"
				count := obj.Count(ctx)
				fmt.Fprintf(ctx, "%s%slazy proxy object(%s)#%d (%d) {\n", linePfx, isRef, obj.Class.GetName(), obj.ID, count)
				localPfx := linePfx + "  "
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
					val, hasVal, hookErr := obj.GetPropValueOrHook(ctx, prop)
					if hookErr != nil {
						return hookErr
					}
					if hasVal {
						doVarDump(ctx, val, localPfx, recurs)
					} else if prop.TypeHint != nil {
						fmt.Fprintf(ctx, "%suninitialized(%s)\n", localPfx, prop.TypeHint.String())
					} else {
						vv := obj.GetPropValue(prop)
						doVarDump(ctx, vv, localPfx, recurs)
					}
				}
				fmt.Fprintf(ctx, "%s}\n", linePfx)
				return nil
			} else if obj.LazyState == phpobj.LazyProxyInitialized && obj.LazyInstance != nil {
				// Initialized proxy: "lazy proxy object(C)#N (1) { ["instance"]=> object(...) }"
				fmt.Fprintf(ctx, "%s%slazy proxy object(%s)#%d (1) {\n", linePfx, isRef, obj.Class.GetName(), obj.ID)
				localPfx := linePfx + "  "
				fmt.Fprintf(ctx, "%s[\"instance\"]=>\n", localPfx)
				doVarDump(ctx, obj.LazyInstance.ZVal(), localPfx, recurs)
				fmt.Fprintf(ctx, "%s}\n", linePfx)
				return nil
			}

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
					ks := k.String()
					// Handle PHP's internal property naming convention:
					// \0*\0name -> protected, \0ClassName\0name -> private
					if len(ks) > 0 && ks[0] == 0 {
						if len(ks) > 2 && ks[1] == '*' && ks[2] == 0 {
							// Protected: \0*\0name
							fmt.Fprintf(ctx, "%s[\"%s\":protected]=>\n", localPfx, ks[3:])
						} else {
							// Private: \0ClassName\0name
							idx := strings.IndexByte(ks[1:], 0)
							if idx >= 0 {
								className := ks[1 : idx+1]
								propName := ks[idx+2:]
								fmt.Fprintf(ctx, "%s[\"%s\":\"%s\":private]=>\n", localPfx, propName, className)
							} else {
								fmt.Fprintf(ctx, "%s[\"%s\"]=>\n", localPfx, ks)
							}
						}
					} else {
						fmt.Fprintf(ctx, "%s[\"%s\"]=>\n", localPfx, ks)
					}
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

				// Try to get value, calling get hooks for virtual hooked properties
				val, hasVal, hookErr := obj.GetPropValueOrHook(ctx, prop)
				if hookErr != nil {
					return hookErr
				}
				if hasVal {
					doVarDump(ctx, val, localPfx, recurs)
				} else if prop.TypeHint != nil {
					// Typed property that has not been initialized
					fmt.Fprintf(ctx, "%suninitialized(%s)\n", localPfx, prop.TypeHint.String())
				} else {
					v := obj.GetPropValue(prop)
					doVarDump(ctx, v, localPfx, recurs)
				}
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
