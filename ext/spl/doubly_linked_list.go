package spl

import (
	"fmt"
	"strings"

	"github.com/MagicalTux/goro/core/phpobj"
	"github.com/MagicalTux/goro/core/phpv"
)

const (
	splDllItModeFifo   = 0
	splDllItModeLifo   = 2
	splDllItModeDelete = 1
	splDllItModeKeep   = 0
	splDllItModeFix    = 4 // internal flag always set by PHP
)

// splDllIterState stores saved iterator state for nested foreach support
type splDllIterState struct {
	pos       int
	started   bool
	iterMode  int
	iterFroze bool
}

// splDllData holds the internal state for an SplDoublyLinkedList instance
type splDllData struct {
	data      []*phpv.ZVal
	pos       int
	mode      int // iterator mode (IT_MODE_LIFO|IT_MODE_FIFO + IT_MODE_DELETE|IT_MODE_KEEP)
	started   bool
	iterMode  int  // mode frozen at rewind time for consistent iteration direction
	iterFroze bool // true once rewind sets the iterMode
	// Stack of saved iterator states for nested foreach support
	savedStates []splDllIterState
}

func (d *splDllData) Clone() any {
	nd := &splDllData{
		data:      make([]*phpv.ZVal, len(d.data)),
		pos:       d.pos,
		mode:      d.mode,
		started:   d.started,
		iterMode:  d.iterMode,
		iterFroze: d.iterFroze,
	}
	for i, v := range d.data {
		if v != nil {
			nd.data[i] = v.ZVal()
		}
	}
	return nd
}

// SaveIterState saves the current iterator state to a stack (for nested foreach)
func (d *splDllData) SaveIterState() {
	d.savedStates = append(d.savedStates, splDllIterState{
		pos:       d.pos,
		started:   d.started,
		iterMode:  d.iterMode,
		iterFroze: d.iterFroze,
	})
}

// RestoreIterState restores the most recently saved iterator state
func (d *splDllData) RestoreIterState() {
	if len(d.savedStates) > 0 {
		state := d.savedStates[len(d.savedStates)-1]
		d.savedStates = d.savedStates[:len(d.savedStates)-1]
		d.pos = state.pos
		d.started = state.started
		d.iterMode = state.iterMode
		d.iterFroze = state.iterFroze
	}
}

func getSplDllData(o *phpobj.ZObject, cls *phpobj.ZClass) *splDllData {
	d := o.GetOpaque(cls)
	if d == nil {
		return nil
	}
	return d.(*splDllData)
}

// validateDllIndex validates an index argument for SplDoublyLinkedList offset methods.
// It returns the integer index and nil error, or -1 and the error to throw.
// methodName should be like "SplDoublyLinkedList::offsetGet"
func validateDllIndex(ctx phpv.Context, arg *phpv.ZVal, methodName string) (int, error) {
	if arg == nil || arg.IsNull() {
		return -1, phpobj.ThrowError(ctx, phpobj.TypeError, fmt.Sprintf("%s(): Argument #1 ($index) must be of type int, null given", methodName))
	}
	// Check type: must be int (or numeric string convertible to int)
	switch arg.GetType() {
	case phpv.ZtInt:
		// ok
	case phpv.ZtFloat:
		// PHP allows float but it gets truncated
	case phpv.ZtBool:
		// PHP allows bool (0 or 1)
	case phpv.ZtString:
		// Check if numeric
		s := string(arg.AsString(ctx))
		isNumeric := true
		for i, c := range s {
			if c >= '0' && c <= '9' {
				continue
			}
			if i == 0 && (c == '-' || c == '+') {
				continue
			}
			isNumeric = false
			break
		}
		if !isNumeric || len(s) == 0 {
			return -1, phpobj.ThrowError(ctx, phpobj.TypeError, fmt.Sprintf("%s(): Argument #1 ($index) must be of type int, string given", methodName))
		}
	case phpv.ZtArray:
		return -1, phpobj.ThrowError(ctx, phpobj.TypeError, fmt.Sprintf("%s(): Argument #1 ($index) must be of type int, array given", methodName))
	case phpv.ZtObject:
		typeName := "object"
		if obj, ok := arg.Value().(*phpobj.ZObject); ok {
			typeName = string(obj.GetClass().GetName())
		}
		return -1, phpobj.ThrowError(ctx, phpobj.TypeError, fmt.Sprintf("%s(): Argument #1 ($index) must be of type int, %s given", methodName, typeName))
	default:
		return -1, phpobj.ThrowError(ctx, phpobj.TypeError, fmt.Sprintf("%s(): Argument #1 ($index) must be of type int", methodName))
	}
	return int(arg.AsInt(ctx)), nil
}

func makeSplDllMethods(cls *phpobj.ZClass, defaultMode int) map[phpv.ZString]*phpv.ZClassMethod {
	getData := func(o *phpobj.ZObject) *splDllData {
		return getSplDllData(o, cls)
	}

	// Always use "SplDoublyLinkedList" for error messages since the methods
	// are defined on that class (PHP behavior: error messages show the declaring class)
	clsName := "SplDoublyLinkedList"

	return map[phpv.ZString]*phpv.ZClassMethod{
		"__construct": {
			Name: "__construct",
			Method: phpobj.NativeMethod(func(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
				d := &splDllData{
					mode: defaultMode,
				}
				o.SetOpaque(cls, d)
				return nil, nil
			}),
		},
		"push": {
			Name: "push",
			Method: phpobj.NativeMethod(func(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
				d := getData(o)
				if d == nil {
					return nil, phpobj.ThrowError(ctx, phpobj.RuntimeException, "Internal data not initialized")
				}
				if len(args) == 0 {
					return nil, phpobj.ThrowError(ctx, phpobj.TypeError, fmt.Sprintf("%s::push(): Argument #1 ($value) not passed", cls.Name))
				}
				d.data = append(d.data, args[0])
				return nil, nil
			}),
		},
		"enqueue": {
			Name: "enqueue",
			Method: phpobj.NativeMethod(func(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
				d := getData(o)
				if d == nil {
					return nil, phpobj.ThrowError(ctx, phpobj.RuntimeException, "Internal data not initialized")
				}
				if len(args) == 0 {
					return nil, phpobj.ThrowError(ctx, phpobj.TypeError, fmt.Sprintf("%s::enqueue(): Argument #1 ($value) not passed", cls.Name))
				}
				d.data = append(d.data, args[0])
				return nil, nil
			}),
		},
		"pop": {
			Name: "pop",
			Method: phpobj.NativeMethod(func(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
				d := getData(o)
				if d == nil || len(d.data) == 0 {
					return nil, phpobj.ThrowError(ctx, phpobj.UnderflowException, "Can't pop from an empty datastructure")
				}
				v := d.data[len(d.data)-1]
				d.data = d.data[:len(d.data)-1]
				return v, nil
			}),
		},
		"unshift": {
			Name: "unshift",
			Method: phpobj.NativeMethod(func(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
				d := getData(o)
				if d == nil {
					return nil, phpobj.ThrowError(ctx, phpobj.RuntimeException, "Internal data not initialized")
				}
				if len(args) == 0 {
					return nil, phpobj.ThrowError(ctx, phpobj.TypeError, fmt.Sprintf("%s::unshift(): Argument #1 ($value) not passed", cls.Name))
				}
				d.data = append([]*phpv.ZVal{args[0]}, d.data...)
				// Adjust iterator position if it's past the beginning
				if d.started && d.pos >= 0 {
					d.pos++
				}
				return nil, nil
			}),
		},
		"shift": {
			Name: "shift",
			Method: phpobj.NativeMethod(func(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
				d := getData(o)
				if d == nil || len(d.data) == 0 {
					return nil, phpobj.ThrowError(ctx, phpobj.UnderflowException, "Can't shift from an empty datastructure")
				}
				v := d.data[0]
				d.data = d.data[1:]
				// Adjust iterator position
				if d.started && d.pos > 0 {
					d.pos--
				}
				return v, nil
			}),
		},
		"dequeue": {
			Name: "dequeue",
			Method: phpobj.NativeMethod(func(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
				d := getData(o)
				if d == nil || len(d.data) == 0 {
					return nil, phpobj.ThrowError(ctx, phpobj.UnderflowException, "Can't shift from an empty datastructure")
				}
				v := d.data[0]
				d.data = d.data[1:]
				if d.started && d.pos > 0 {
					d.pos--
				}
				return v, nil
			}),
		},
		"top": {
			Name: "top",
			Method: phpobj.NativeMethod(func(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
				d := getData(o)
				if d == nil || len(d.data) == 0 {
					return nil, phpobj.ThrowError(ctx, phpobj.UnderflowException, "Can't peek at an empty datastructure")
				}
				return d.data[len(d.data)-1], nil
			}),
		},
		"bottom": {
			Name: "bottom",
			Method: phpobj.NativeMethod(func(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
				d := getData(o)
				if d == nil || len(d.data) == 0 {
					return nil, phpobj.ThrowError(ctx, phpobj.UnderflowException, "Can't peek at an empty datastructure")
				}
				return d.data[0], nil
			}),
		},
		"count": {
			Name: "count",
			Method: phpobj.NativeMethod(func(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
				d := getData(o)
				if d == nil {
					return phpv.ZInt(0).ZVal(), nil
				}
				return phpv.ZInt(len(d.data)).ZVal(), nil
			}),
		},
		"isempty": {
			Name: "isEmpty",
			Method: phpobj.NativeMethod(func(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
				d := getData(o)
				if d == nil || len(d.data) == 0 {
					return phpv.ZTrue.ZVal(), nil
				}
				return phpv.ZFalse.ZVal(), nil
			}),
		},
		"offsetexists": {
			Name: "offsetExists",
			Method: phpobj.NativeMethod(func(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
				d := getData(o)
				if d == nil || len(args) == 0 || args[0] == nil {
					return phpv.ZFalse.ZVal(), nil
				}
				idx := int(args[0].AsInt(ctx))
				if idx >= 0 && idx < len(d.data) {
					return phpv.ZTrue.ZVal(), nil
				}
				return phpv.ZFalse.ZVal(), nil
			}),
		},
		"offsetget": {
			Name: "offsetGet",
			Method: phpobj.NativeMethod(func(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
				d := getData(o)
				if d == nil {
					return nil, phpobj.ThrowError(ctx, phpobj.RuntimeException, "Internal data not initialized")
				}
				if len(args) == 0 {
					return nil, phpobj.ThrowError(ctx, phpobj.OutOfRangeException, fmt.Sprintf("%s::offsetGet(): Argument #1 ($index) is out of range", clsName))
				}
				idx, err := validateDllIndex(ctx, args[0], clsName+"::offsetGet")
				if err != nil {
					return nil, err
				}
				if idx < 0 || idx >= len(d.data) {
					return nil, phpobj.ThrowError(ctx, phpobj.OutOfRangeException, fmt.Sprintf("%s::offsetGet(): Argument #1 ($index) is out of range", clsName))
				}
				return d.data[idx], nil
			}),
		},
		"offsetset": {
			Name: "offsetSet",
			Method: phpobj.NativeMethod(func(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
				d := getData(o)
				if d == nil {
					return nil, phpobj.ThrowError(ctx, phpobj.RuntimeException, "Internal data not initialized")
				}
				if len(args) < 2 {
					return nil, phpobj.ThrowError(ctx, phpobj.OutOfRangeException, fmt.Sprintf("%s::offsetSet(): Argument #1 ($index) is out of range", clsName))
				}
				// null key means push/append
				if args[0] == nil || args[0].IsNull() {
					d.data = append(d.data, args[1])
					return nil, nil
				}
				idx, err := validateDllIndex(ctx, args[0], clsName+"::offsetSet")
				if err != nil {
					return nil, err
				}
				if idx < 0 || idx >= len(d.data) {
					return nil, phpobj.ThrowError(ctx, phpobj.OutOfRangeException, fmt.Sprintf("%s::offsetSet(): Argument #1 ($index) is out of range", clsName))
				}
				d.data[idx] = args[1]
				return nil, nil
			}),
		},
		"offsetunset": {
			Name: "offsetUnset",
			Method: phpobj.NativeMethod(func(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
				d := getData(o)
				if d == nil {
					return nil, phpobj.ThrowError(ctx, phpobj.RuntimeException, "Internal data not initialized")
				}
				if len(args) == 0 || args[0] == nil {
					return nil, phpobj.ThrowError(ctx, phpobj.OutOfRangeException, fmt.Sprintf("%s::offsetUnset(): Argument #1 ($index) is out of range", clsName))
				}
				idx, err := validateDllIndex(ctx, args[0], clsName+"::offsetUnset")
				if err != nil {
					return nil, err
				}
				if idx < 0 || idx >= len(d.data) {
					return nil, phpobj.ThrowError(ctx, phpobj.OutOfRangeException, fmt.Sprintf("%s::offsetUnset(): Argument #1 ($index) is out of range", clsName))
				}
				d.data = append(d.data[:idx], d.data[idx+1:]...)
				// Adjust iterator position
				if d.started && d.pos > idx {
					d.pos--
				}
				return nil, nil
			}),
		},
		"current": {
			Name: "current",
			Method: phpobj.NativeMethod(func(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
				d := getData(o)
				if d == nil {
					// PHP returns NULL for current() on uninitialized list
					return phpv.ZNULL.ZVal(), nil
				}
				if d.pos < 0 || d.pos >= len(d.data) {
					return phpv.ZNULL.ZVal(), nil
				}
				return d.data[d.pos], nil
			}),
		},
		"key": {
			Name: "key",
			Method: phpobj.NativeMethod(func(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
				d := getData(o)
				if d == nil {
					return phpv.ZInt(0).ZVal(), nil
				}
				return phpv.ZInt(d.pos).ZVal(), nil
			}),
		},
		"next": {
			Name: "next",
			Method: phpobj.NativeMethod(func(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
				d := getData(o)
				if d == nil {
					return nil, nil
				}
				// Use iterMode (frozen at rewind) for direction if available
				mode := d.mode
				if d.iterFroze {
					mode = d.iterMode
				}

				// IT_MODE_DELETE: remove current element before advancing
				if mode&splDllItModeDelete != 0 {
					if d.pos >= 0 && d.pos < len(d.data) {
						d.data = append(d.data[:d.pos], d.data[d.pos+1:]...)
						// In FIFO+DELETE mode: pos stays the same (points to next element)
						// In LIFO+DELETE mode: pos decrements (was at end, remove it, now go to new end)
						if mode&splDllItModeLifo != 0 {
							d.pos--
						}
						// For FIFO, pos stays the same, pointing to the next element
						return nil, nil
					}
				}

				if mode&splDllItModeLifo != 0 {
					d.pos--
				} else {
					d.pos++
				}
				return nil, nil
			}),
		},
		"prev": {
			Name: "prev",
			Method: phpobj.NativeMethod(func(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
				d := getData(o)
				if d == nil {
					return nil, nil
				}
				mode := d.mode
				if d.iterFroze {
					mode = d.iterMode
				}
				if mode&splDllItModeLifo != 0 {
					d.pos++
				} else {
					d.pos--
				}
				return nil, nil
			}),
		},
		"rewind": {
			Name: "rewind",
			Method: phpobj.NativeMethod(func(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
				d := getData(o)
				if d == nil {
					return nil, nil
				}
				d.started = true
				// Freeze the mode at rewind time so changing mode mid-iteration doesn't affect direction
				d.iterMode = d.mode
				d.iterFroze = true
				if d.mode&splDllItModeLifo != 0 {
					d.pos = len(d.data) - 1
				} else {
					d.pos = 0
				}
				return nil, nil
			}),
		},
		"valid": {
			Name: "valid",
			Method: phpobj.NativeMethod(func(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
				d := getData(o)
				if d == nil || d.pos < 0 || d.pos >= len(d.data) {
					return phpv.ZFalse.ZVal(), nil
				}
				return phpv.ZTrue.ZVal(), nil
			}),
		},
		"setiteratormode": {
			Name: "setIteratorMode",
			Method: phpobj.NativeMethod(func(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
				d := getData(o)
				if d == nil {
					return nil, phpobj.ThrowError(ctx, phpobj.RuntimeException, "Internal data not initialized")
				}
				if len(args) == 0 || args[0] == nil {
					return nil, phpobj.ThrowError(ctx, phpobj.TypeError, "SplDoublyLinkedList::setIteratorMode(): Argument #1 ($mode) must be of type int")
				}
				mode := int(args[0].AsInt(ctx))

				// SplStack and SplQueue have frozen LIFO/FIFO direction
				if cls == SplStackClass {
					// SplStack must be LIFO - if trying to set FIFO (bit 1 cleared), throw
					if mode&splDllItModeLifo == 0 {
						return nil, phpobj.ThrowError(ctx, phpobj.RuntimeException, "Iterators' LIFO/FIFO modes for SplStack/SplQueue objects are frozen")
					}
				} else if cls == SplQueueClass {
					// SplQueue must be FIFO - if trying to set LIFO (bit 1 set), throw
					if mode&splDllItModeLifo != 0 {
						return nil, phpobj.ThrowError(ctx, phpobj.RuntimeException, "Iterators' LIFO/FIFO modes for SplStack/SplQueue objects are frozen")
					}
				}

				d.mode = mode
				return phpv.ZInt(mode).ZVal(), nil
			}),
		},
		"getiteratormode": {
			Name: "getIteratorMode",
			Method: phpobj.NativeMethod(func(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
				d := getData(o)
				if d == nil {
					return phpv.ZInt(0).ZVal(), nil
				}
				return phpv.ZInt(d.mode).ZVal(), nil
			}),
		},
		"serialize": {
			Name: "serialize",
			Method: phpobj.NativeMethod(func(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
				d := getData(o)
				if d == nil {
					return phpv.ZString("i:0;").ZVal(), nil
				}
				// PHP serializes as: i:<mode>;:<count>:{<serialized elements>}
				var sb strings.Builder
				sb.WriteString(fmt.Sprintf("i:%d;", d.mode))
				for _, v := range d.data {
					if v == nil || v.IsNull() {
						sb.WriteString("N;")
					} else {
						switch v.GetType() {
						case phpv.ZtInt:
							sb.WriteString(fmt.Sprintf("i:%d;", v.AsInt(ctx)))
						case phpv.ZtString:
							s := v.AsString(ctx)
							sb.WriteString(fmt.Sprintf("s:%d:\"%s\";", len(s), s))
						case phpv.ZtFloat:
							sb.WriteString(fmt.Sprintf("d:%v;", v.Value().(phpv.ZFloat)))
						case phpv.ZtBool:
							if v.AsBool(ctx) {
								sb.WriteString("b:1;")
							} else {
								sb.WriteString("b:0;")
							}
						default:
							sb.WriteString("N;")
						}
					}
				}
				return phpv.ZString(sb.String()).ZVal(), nil
			}),
		},
		"unserialize": {
			Name: "unserialize",
			Method: phpobj.NativeMethod(func(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
				d := getData(o)
				if d == nil {
					d = &splDllData{mode: defaultMode}
					o.SetOpaque(cls, d)
				}
				if len(args) == 0 || args[0] == nil {
					return nil, nil
				}
				// Basic unserialization - parse mode and elements
				s := string(args[0].AsString(ctx))
				// Parse mode: i:<mode>;
				if len(s) > 2 && s[0] == 'i' && s[1] == ':' {
					end := strings.IndexByte(s[2:], ';')
					if end >= 0 {
						mode := 0
						fmt.Sscanf(s[2:2+end], "%d", &mode)
						d.mode = mode
						s = s[2+end+1:]
					}
				}
				// Parse remaining serialized values
				d.data = nil
				for len(s) > 0 {
					v, rest := unserializeValue(ctx, s)
					if rest == s {
						break // no progress
					}
					if v != nil {
						d.data = append(d.data, v)
					}
					s = rest
				}
				return nil, nil
			}),
		},
		"__serialize": {Name: "__serialize", Method: phpobj.NativeMethod(func(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
			d := getData(o)
			result := phpv.NewZArray()
			flags := 0
			if d != nil {
				flags = d.mode
			}
			result.OffsetSet(ctx, phpv.ZInt(0), phpv.ZInt(flags).ZVal())
			dataArr := phpv.NewZArray()
			if d != nil {
				for i, v := range d.data {
					if v == nil {
						dataArr.OffsetSet(ctx, phpv.ZInt(i), phpv.ZNULL.ZVal())
					} else {
						dataArr.OffsetSet(ctx, phpv.ZInt(i), v)
					}
				}
			}
			result.OffsetSet(ctx, phpv.ZInt(1), dataArr.ZVal())
			result.OffsetSet(ctx, phpv.ZInt(2), phpv.NewZArray().ZVal())
			return result.ZVal(), nil
		})},
		"__unserialize": {Name: "__unserialize", Method: phpobj.NativeMethod(func(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
			d := getData(o)
			if d == nil {
				d = &splDllData{mode: defaultMode}
				o.SetOpaque(cls, d)
			}
			if len(args) == 0 || args[0] == nil {
				return nil, nil
			}
			arr := args[0].AsArray(ctx)
			if arr == nil {
				return nil, nil
			}
			if flagsVal, err := arr.OffsetGet(ctx, phpv.ZInt(0).ZVal()); err == nil && flagsVal != nil {
				d.mode = int(flagsVal.AsInt(ctx))
			}
			if dataVal, err := arr.OffsetGet(ctx, phpv.ZInt(1).ZVal()); err == nil && dataVal != nil {
				dataArr := dataVal.AsArray(ctx)
				if dataArr != nil {
					d.data = nil
					it := dataArr.NewIterator()
					for ; it.Valid(ctx); it.Next(ctx) {
						v, _ := it.Current(ctx)
						d.data = append(d.data, v)
					}
				}
			}
			return nil, nil
		})},
		"add": {
			Name: "add",
			Method: phpobj.NativeMethod(func(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
				d := getData(o)
				if d == nil {
					return nil, phpobj.ThrowError(ctx, phpobj.RuntimeException, "Internal data not initialized")
				}
				if len(args) < 2 {
					return nil, phpobj.ThrowError(ctx, phpobj.OutOfRangeException, fmt.Sprintf("%s::add(): Argument #1 ($index) is out of range", clsName))
				}
				if args[0] == nil {
					return nil, phpobj.ThrowError(ctx, phpobj.OutOfRangeException, fmt.Sprintf("%s::add(): Argument #1 ($index) is out of range", clsName))
				}
				// Type check the index
				idx, err := validateDllIndex(ctx, args[0], clsName+"::add")
				if err != nil {
					return nil, err
				}
				if idx < 0 || idx > len(d.data) {
					return nil, phpobj.ThrowError(ctx, phpobj.OutOfRangeException, fmt.Sprintf("%s::add(): Argument #1 ($index) is out of range", clsName))
				}
				// Insert at position
				d.data = append(d.data, nil)
				copy(d.data[idx+1:], d.data[idx:])
				d.data[idx] = args[1]
				return nil, nil
			}),
		},
		"__debuginfo": {
			Name: "__debugInfo",
			Method: phpobj.NativeMethod(func(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
				d := getData(o)
				result := phpv.NewZArray()
				flags := 0
				if d != nil {
					flags = d.mode
				}
				// Private property: flags
				result.OffsetSet(ctx, phpv.ZString("\x00SplDoublyLinkedList\x00flags"), phpv.ZInt(flags).ZVal())
				// Private property: dllist
				dllist := phpv.NewZArray()
				if d != nil {
					for i, v := range d.data {
						if v == nil {
							dllist.OffsetSet(ctx, phpv.ZInt(i), phpv.ZNULL.ZVal())
						} else {
							dllist.OffsetSet(ctx, phpv.ZInt(i), v)
						}
					}
				}
				result.OffsetSet(ctx, phpv.ZString("\x00SplDoublyLinkedList\x00dllist"), dllist.ZVal())
				return result.ZVal(), nil
			}),
		},
	}
}

// unserializeValue parses a single PHP serialized value from the string
// and returns the value and the remaining string.
func unserializeValue(ctx phpv.Context, s string) (*phpv.ZVal, string) {
	if len(s) == 0 {
		return nil, s
	}
	switch s[0] {
	case 'N':
		// N;
		if len(s) >= 2 && s[1] == ';' {
			return phpv.ZNULL.ZVal(), s[2:]
		}
		return nil, s
	case 'i':
		// i:<number>;
		if len(s) >= 3 && s[1] == ':' {
			end := strings.IndexByte(s[2:], ';')
			if end >= 0 {
				var n int64
				fmt.Sscanf(s[2:2+end], "%d", &n)
				return phpv.ZInt(n).ZVal(), s[2+end+1:]
			}
		}
		return nil, s
	case 'd':
		// d:<number>;
		if len(s) >= 3 && s[1] == ':' {
			end := strings.IndexByte(s[2:], ';')
			if end >= 0 {
				var f float64
				fmt.Sscanf(s[2:2+end], "%g", &f)
				return phpv.ZFloat(f).ZVal(), s[2+end+1:]
			}
		}
		return nil, s
	case 'b':
		// b:<0|1>;
		if len(s) >= 4 && s[1] == ':' && s[3] == ';' {
			if s[2] == '1' {
				return phpv.ZTrue.ZVal(), s[4:]
			}
			return phpv.ZFalse.ZVal(), s[4:]
		}
		return nil, s
	case 's':
		// s:<len>:"<string>";
		if len(s) >= 3 && s[1] == ':' {
			end := strings.IndexByte(s[2:], ':')
			if end >= 0 {
				var slen int
				fmt.Sscanf(s[2:2+end], "%d", &slen)
				// Skip :"
				start := 2 + end + 2 // past :"
				if start+slen+2 <= len(s) {
					str := s[start : start+slen]
					return phpv.ZString(str).ZVal(), s[start+slen+2:] // skip ";
				}
			}
		}
		return nil, s
	}
	return nil, s
}

var SplDoublyLinkedListClass = &phpobj.ZClass{
	Name:            "SplDoublyLinkedList",
	Implementations: []*phpobj.ZClass{phpobj.Iterator, Countable, phpobj.ArrayAccess, phpobj.Serializable},
}

var SplStackClass = &phpobj.ZClass{
	Name:            "SplStack",
	Extends:         SplDoublyLinkedListClass,
	Implementations: []*phpobj.ZClass{phpobj.Iterator, Countable, phpobj.ArrayAccess, phpobj.Serializable},
}

var SplQueueClass = &phpobj.ZClass{
	Name:            "SplQueue",
	Extends:         SplDoublyLinkedListClass,
	Implementations: []*phpobj.ZClass{phpobj.Iterator, Countable, phpobj.ArrayAccess, phpobj.Serializable},
}

func initSplDoublyLinkedList() {
	SplDoublyLinkedListClass.Const = map[phpv.ZString]*phpv.ZClassConst{
		"IT_MODE_LIFO":   {Value: phpv.ZInt(splDllItModeLifo)},
		"IT_MODE_FIFO":   {Value: phpv.ZInt(splDllItModeFifo)},
		"IT_MODE_DELETE": {Value: phpv.ZInt(splDllItModeDelete)},
		"IT_MODE_KEEP":   {Value: phpv.ZInt(splDllItModeKeep)},
	}
	SplDoublyLinkedListClass.Methods = makeSplDllMethods(SplDoublyLinkedListClass, splDllItModeFifo)

	// SplStack: same methods but default mode is LIFO
	SplStackClass.Const = map[phpv.ZString]*phpv.ZClassConst{
		"IT_MODE_LIFO":   {Value: phpv.ZInt(splDllItModeLifo)},
		"IT_MODE_FIFO":   {Value: phpv.ZInt(splDllItModeFifo)},
		"IT_MODE_DELETE": {Value: phpv.ZInt(splDllItModeDelete)},
		"IT_MODE_KEEP":   {Value: phpv.ZInt(splDllItModeKeep)},
	}
	SplStackClass.Methods = makeSplDllMethods(SplStackClass, splDllItModeLifo|splDllItModeKeep|splDllItModeFix)

	// SplQueue: same methods but default mode is FIFO + KEEP + FIX
	SplQueueClass.Const = map[phpv.ZString]*phpv.ZClassConst{
		"IT_MODE_LIFO":   {Value: phpv.ZInt(splDllItModeLifo)},
		"IT_MODE_FIFO":   {Value: phpv.ZInt(splDllItModeFifo)},
		"IT_MODE_DELETE": {Value: phpv.ZInt(splDllItModeDelete)},
		"IT_MODE_KEEP":   {Value: phpv.ZInt(splDllItModeKeep)},
	}
	SplQueueClass.Methods = makeSplDllMethods(SplQueueClass, splDllItModeFifo|splDllItModeKeep|splDllItModeFix)
}
