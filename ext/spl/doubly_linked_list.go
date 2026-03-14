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
)

// splDllData holds the internal state for an SplDoublyLinkedList instance
type splDllData struct {
	data    []*phpv.ZVal
	pos     int
	mode    int // iterator mode (IT_MODE_LIFO|IT_MODE_FIFO + IT_MODE_DELETE|IT_MODE_KEEP)
	started bool
}

func (d *splDllData) Clone() any {
	nd := &splDllData{
		data:    make([]*phpv.ZVal, len(d.data)),
		pos:     d.pos,
		mode:    d.mode,
		started: d.started,
	}
	for i, v := range d.data {
		if v != nil {
			nd.data[i] = v.ZVal()
		}
	}
	return nd
}

func getSplDllData(o *phpobj.ZObject, cls *phpobj.ZClass) *splDllData {
	d := o.GetOpaque(cls)
	if d == nil {
		return nil
	}
	return d.(*splDllData)
}

func makeSplDllMethods(cls *phpobj.ZClass, defaultMode int) map[phpv.ZString]*phpv.ZClassMethod {
	getData := func(o *phpobj.ZObject) *splDllData {
		return getSplDllData(o, cls)
	}

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
					return nil, phpobj.ThrowError(ctx, phpobj.UnderflowException, "Can't dequeue from an empty datastructure")
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
				if len(args) == 0 || args[0] == nil {
					return nil, phpobj.ThrowError(ctx, phpobj.OutOfRangeException, "Offset invalid or out of range")
				}
				idx := int(args[0].AsInt(ctx))
				if idx < 0 || idx >= len(d.data) {
					return nil, phpobj.ThrowError(ctx, phpobj.OutOfRangeException, "Offset out of range")
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
					return nil, phpobj.ThrowError(ctx, phpobj.OutOfRangeException, "Offset invalid or out of range")
				}
				// null key means push/append
				if args[0] == nil || args[0].IsNull() {
					d.data = append(d.data, args[1])
					return nil, nil
				}
				idx := int(args[0].AsInt(ctx))
				if idx < 0 || idx >= len(d.data) {
					return nil, phpobj.ThrowError(ctx, phpobj.OutOfRangeException, "Offset out of range")
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
					return nil, phpobj.ThrowError(ctx, phpobj.OutOfRangeException, "Offset invalid or out of range")
				}
				idx := int(args[0].AsInt(ctx))
				if idx < 0 || idx >= len(d.data) {
					return nil, phpobj.ThrowError(ctx, phpobj.OutOfRangeException, "Offset out of range")
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
				if d == nil || d.pos < 0 || d.pos >= len(d.data) {
					return phpv.ZFalse.ZVal(), nil
				}
				return d.data[d.pos], nil
			}),
		},
		"key": {
			Name: "key",
			Method: phpobj.NativeMethod(func(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
				d := getData(o)
				if d == nil || d.pos < 0 || d.pos >= len(d.data) {
					return phpv.ZNULL.ZVal(), nil
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
				if d.mode&splDllItModeLifo != 0 {
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
				if d.mode&splDllItModeLifo != 0 {
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
		"add": {
			Name: "add",
			Method: phpobj.NativeMethod(func(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
				d := getData(o)
				if d == nil {
					return nil, phpobj.ThrowError(ctx, phpobj.RuntimeException, "Internal data not initialized")
				}
				if len(args) < 2 || args[0] == nil {
					return nil, phpobj.ThrowError(ctx, phpobj.OutOfRangeException, "Offset invalid or out of range")
				}
				idx := int(args[0].AsInt(ctx))
				if idx < 0 || idx > len(d.data) {
					return nil, phpobj.ThrowError(ctx, phpobj.OutOfRangeException, "Offset out of range")
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
				if d == nil {
					return phpv.NewZArray().ZVal(), nil
				}
				arr := phpv.NewZArray()
				for i, v := range d.data {
					if v == nil {
						arr.OffsetSet(ctx, phpv.ZInt(i), phpv.ZNULL.ZVal())
					} else {
						arr.OffsetSet(ctx, phpv.ZInt(i), v)
					}
				}
				return arr.ZVal(), nil
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
		"IT_MODE_DELETE":  {Value: phpv.ZInt(splDllItModeDelete)},
		"IT_MODE_KEEP":   {Value: phpv.ZInt(splDllItModeKeep)},
	}
	SplDoublyLinkedListClass.Methods = makeSplDllMethods(SplDoublyLinkedListClass, splDllItModeFifo)

	// SplStack: same methods but default mode is LIFO
	SplStackClass.Const = map[phpv.ZString]*phpv.ZClassConst{
		"IT_MODE_LIFO":   {Value: phpv.ZInt(splDllItModeLifo)},
		"IT_MODE_FIFO":   {Value: phpv.ZInt(splDllItModeFifo)},
		"IT_MODE_DELETE":  {Value: phpv.ZInt(splDllItModeDelete)},
		"IT_MODE_KEEP":   {Value: phpv.ZInt(splDllItModeKeep)},
	}
	SplStackClass.Methods = makeSplDllMethods(SplStackClass, splDllItModeLifo|splDllItModeKeep)

	// SplQueue: same methods but default mode is FIFO + DELETE for dequeue
	SplQueueClass.Const = map[phpv.ZString]*phpv.ZClassConst{
		"IT_MODE_LIFO":   {Value: phpv.ZInt(splDllItModeLifo)},
		"IT_MODE_FIFO":   {Value: phpv.ZInt(splDllItModeFifo)},
		"IT_MODE_DELETE":  {Value: phpv.ZInt(splDllItModeDelete)},
		"IT_MODE_KEEP":   {Value: phpv.ZInt(splDllItModeKeep)},
	}
	SplQueueClass.Methods = makeSplDllMethods(SplQueueClass, splDllItModeFifo|splDllItModeKeep)
}

