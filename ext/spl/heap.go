package spl

import (
	"container/heap"
	"fmt"

	"github.com/MagicalTux/goro/core/phpobj"
	"github.com/MagicalTux/goro/core/phpv"
)

// splHeapEntry holds a value and its insertion order for stable sorting
type splHeapEntry struct {
	value *phpv.ZVal
	index int
}

// splHeapImpl implements heap.Interface, using a compare function
type splHeapImpl struct {
	entries []*splHeapEntry
	less    func(a, b *splHeapEntry) bool
}

func (h *splHeapImpl) Len() int { return len(h.entries) }

func (h *splHeapImpl) Less(i, j int) bool {
	return h.less(h.entries[i], h.entries[j])
}

func (h *splHeapImpl) Swap(i, j int) {
	h.entries[i], h.entries[j] = h.entries[j], h.entries[i]
}

func (h *splHeapImpl) Push(x interface{}) {
	h.entries = append(h.entries, x.(*splHeapEntry))
}

func (h *splHeapImpl) Pop() interface{} {
	old := h.entries
	n := len(old)
	item := old[n-1]
	old[n-1] = nil
	h.entries = old[:n-1]
	return item
}

// splHeapData holds the internal state for an SplHeap instance
type splHeapData struct {
	heap      *splHeapImpl
	nextIndex int
	corrupted bool
	// compare is the Go-level compare function.
	// For SplMinHeap/SplMaxHeap it uses phpv.Compare directly.
	// For user-extended classes, it calls the PHP compare() method.
	compareFn  func(ctx phpv.Context, a, b *phpv.ZVal) (int, error)
	ctx        phpv.Context
	compareErr error // last compare error (for corruption detection)
	// iterKey tracks the key during iteration (0, 1, 2, ...)
	iterKey int
}

func (d *splHeapData) Clone() any {
	nd := &splHeapData{
		heap: &splHeapImpl{
			entries: make([]*splHeapEntry, len(d.heap.entries)),
		},
		nextIndex: d.nextIndex,
		corrupted: d.corrupted,
		compareFn: d.compareFn,
		ctx:       d.ctx,
		iterKey:   d.iterKey,
	}
	copy(nd.heap.entries, d.heap.entries)
	nd.heap.less = nd.makeLess()
	return nd
}

func (d *splHeapData) makeLess() func(a, b *splHeapEntry) bool {
	return func(a, b *splHeapEntry) bool {
		cmp, err := d.compareFn(d.ctx, a.value, b.value)
		if err != nil {
			d.corrupted = true
			d.compareErr = err
			return false
		}
		if cmp != 0 {
			return cmp > 0
		}
		// Stable: earlier insertions come first
		return a.index < b.index
	}
}

func getHeapData(o *phpobj.ZObject) *splHeapData {
	d := o.GetOpaque(SplHeapClass)
	if d == nil {
		return nil
	}
	return d.(*splHeapData)
}

func initHeapData(ctx phpv.Context, o *phpobj.ZObject, compareFn func(phpv.Context, *phpv.ZVal, *phpv.ZVal) (int, error)) *splHeapData {
	d := &splHeapData{
		heap:      &splHeapImpl{},
		compareFn: compareFn,
		ctx:       ctx,
	}
	d.heap.less = d.makeLess()
	heap.Init(d.heap)
	o.SetOpaque(SplHeapClass, d)
	return d
}

// userCompare calls the PHP-level compare() method on the object
func userCompare(ctx phpv.Context, o *phpobj.ZObject, a, b *phpv.ZVal) (int, error) {
	result, err := o.CallMethod(ctx, "compare", a, b)
	if err != nil {
		return 0, err
	}
	return int(result.AsInt(ctx)), nil
}

// minHeapCompare: returns > 0 when value1 < value2 (smaller values go to top)
func minHeapCompare(ctx phpv.Context, a, b *phpv.ZVal) (int, error) {
	cmp, err := phpv.Compare(ctx, b, a)
	if err != nil {
		return 0, err
	}
	return cmp, nil
}

// maxHeapCompare: returns > 0 when value1 > value2 (larger values go to top)
func maxHeapCompare(ctx phpv.Context, a, b *phpv.ZVal) (int, error) {
	cmp, err := phpv.Compare(ctx, a, b)
	if err != nil {
		return 0, err
	}
	return cmp, nil
}

var SplHeapClass = &phpobj.ZClass{
	Name:            "SplHeap",
	Attr:            phpv.ZClassAttr(phpv.ZClassExplicitAbstract),
	Implementations: []*phpobj.ZClass{phpobj.Iterator, Countable},
}

var SplMinHeapClass = &phpobj.ZClass{
	Name:            "SplMinHeap",
	Extends:         SplHeapClass,
	Implementations: []*phpobj.ZClass{phpobj.Iterator, Countable},
}

var SplMaxHeapClass = &phpobj.ZClass{
	Name:            "SplMaxHeap",
	Extends:         SplHeapClass,
	Implementations: []*phpobj.ZClass{phpobj.Iterator, Countable},
}

func initSplHeap() {
	SplHeapClass.Methods = map[phpv.ZString]*phpv.ZClassMethod{
		"__construct": {
			Name: "__construct",
			Method: phpobj.NativeMethod(func(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
				// Determine compare function based on actual class
				cls := o.GetClass()
				var compareFn func(phpv.Context, *phpv.ZVal, *phpv.ZVal) (int, error)

				if cls == SplMinHeapClass {
					compareFn = minHeapCompare
				} else if cls == SplMaxHeapClass {
					compareFn = maxHeapCompare
				} else {
					// User-extended class - call the PHP compare() method
					compareFn = func(ctx phpv.Context, a, b *phpv.ZVal) (int, error) {
						return userCompare(ctx, o, a, b)
					}
				}
				initHeapData(ctx, o, compareFn)
				return nil, nil
			}),
		},
		"compare": {
			Name:      "compare",
			Modifiers: phpv.ZAttrProtected | phpv.ZAttrAbstract,
			Empty:     true,
		},
		"insert": {
			Name: "insert",
			Method: phpobj.NativeMethod(func(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
				d := getHeapData(o)
				if d == nil {
					return phpv.ZTrue.ZVal(), nil
				}
				if d.corrupted {
					return nil, phpobj.ThrowError(ctx, phpobj.RuntimeException, "Heap is corrupted, heap properties are no longer ensured.")
				}
				if len(args) == 0 {
					return nil, phpobj.ThrowError(ctx, phpobj.TypeError, "SplHeap::insert() expects exactly 1 argument")
				}
				d.ctx = ctx
				d.compareErr = nil
				entry := &splHeapEntry{
					value: args[0].Dup(),
					index: d.nextIndex,
				}
				d.nextIndex++
				heap.Push(d.heap, entry)
				if d.compareErr != nil {
					err := d.compareErr
					d.compareErr = nil
					return nil, err
				}
				return phpv.ZTrue.ZVal(), nil
			}),
		},
		"extract": {
			Name: "extract",
			Method: phpobj.NativeMethod(func(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
				d := getHeapData(o)
				if d == nil || d.heap.Len() == 0 {
					return nil, phpobj.ThrowError(ctx, phpobj.RuntimeException, "Can't extract from an empty heap")
				}
				if d.corrupted {
					return nil, phpobj.ThrowError(ctx, phpobj.RuntimeException, "Heap is corrupted, heap properties are no longer ensured.")
				}
				d.ctx = ctx
				d.compareErr = nil
				entry := heap.Pop(d.heap).(*splHeapEntry)
				if d.compareErr != nil {
					err := d.compareErr
					d.compareErr = nil
					return nil, err
				}
				return entry.value, nil
			}),
		},
		"top": {
			Name: "top",
			Method: phpobj.NativeMethod(func(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
				d := getHeapData(o)
				if d == nil || d.heap.Len() == 0 {
					return nil, phpobj.ThrowError(ctx, phpobj.RuntimeException, "Can't peek at an empty heap")
				}
				if d.corrupted {
					return nil, phpobj.ThrowError(ctx, phpobj.RuntimeException, "Heap is corrupted, heap properties are no longer ensured.")
				}
				return d.heap.entries[0].value, nil
			}),
		},
		"count": {
			Name: "count",
			Method: phpobj.NativeMethod(func(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
				d := getHeapData(o)
				if d == nil {
					return phpv.ZInt(0).ZVal(), nil
				}
				return phpv.ZInt(d.heap.Len()).ZVal(), nil
			}),
		},
		"isempty": {
			Name: "isEmpty",
			Method: phpobj.NativeMethod(func(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
				d := getHeapData(o)
				if d == nil {
					return phpv.ZTrue.ZVal(), nil
				}
				return phpv.ZBool(d.heap.Len() == 0).ZVal(), nil
			}),
		},
		"current": {
			Name: "current",
			Method: phpobj.NativeMethod(func(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
				d := getHeapData(o)
				if d == nil || d.heap.Len() == 0 {
					return phpv.ZNULL.ZVal(), nil
				}
				return d.heap.entries[0].value, nil
			}),
		},
		"key": {
			Name: "key",
			Method: phpobj.NativeMethod(func(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
				d := getHeapData(o)
				if d == nil || d.heap.Len() == 0 {
					return phpv.ZNULL.ZVal(), nil
				}
				// PHP SplHeap key() returns remaining_count - 1
				return phpv.ZInt(d.heap.Len() - 1).ZVal(), nil
			}),
		},
		"next": {
			Name: "next",
			Method: phpobj.NativeMethod(func(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
				d := getHeapData(o)
				if d == nil || d.heap.Len() == 0 {
					return nil, nil
				}
				d.ctx = ctx
				d.compareErr = nil
				heap.Pop(d.heap)
				if d.compareErr != nil {
					err := d.compareErr
					d.compareErr = nil
					return nil, err
				}
				d.iterKey++
				return nil, nil
			}),
		},
		"rewind": {
			Name: "rewind",
			Method: phpobj.NativeMethod(func(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
				// SplHeap::rewind() resets the key counter
				d := getHeapData(o)
				if d != nil {
					d.iterKey = 0
				}
				return nil, nil
			}),
		},
		"valid": {
			Name: "valid",
			Method: phpobj.NativeMethod(func(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
				d := getHeapData(o)
				if d == nil {
					return phpv.ZFalse.ZVal(), nil
				}
				return phpv.ZBool(d.heap.Len() > 0).ZVal(), nil
			}),
		},
		"iscorrupted": {
			Name: "isCorrupted",
			Method: phpobj.NativeMethod(func(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
				d := getHeapData(o)
				if d == nil {
					return phpv.ZFalse.ZVal(), nil
				}
				return phpv.ZBool(d.corrupted).ZVal(), nil
			}),
		},
		"recoverfromcorruption": {
			Name: "recoverFromCorruption",
			Method: phpobj.NativeMethod(func(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
				d := getHeapData(o)
				if d != nil {
					d.corrupted = false
				}
				return phpv.ZTrue.ZVal(), nil
			}),
		},
		"__serialize": {Name: "__serialize", Method: phpobj.NativeMethod(func(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
			d := getHeapData(o)
			if d != nil && d.corrupted {
				return nil, phpobj.ThrowError(ctx, phpobj.RuntimeException, "Heap is corrupted, heap properties are no longer ensured.")
			}
			result := phpv.NewZArray()
			// Key 0: member (user/dynamic) properties
			memberProps := phpv.NewZArray()
			for prop := range o.IterProps(ctx) {
				v := o.GetPropValue(prop)
				if prop.Modifiers.IsPrivate() {
					// Private: \0ClassName\0propName
					key := "\x00" + string(o.GetClass().GetName()) + "\x00" + string(prop.VarName)
					memberProps.OffsetSet(ctx, phpv.ZString(key), v)
				} else if prop.Modifiers.IsProtected() {
					// Protected: \0*\0propName
					key := "\x00*\x00" + string(prop.VarName)
					memberProps.OffsetSet(ctx, phpv.ZString(key), v)
				} else {
					memberProps.OffsetSet(ctx, prop.VarName.ZVal(), v)
				}
			}
			result.OffsetSet(ctx, phpv.ZInt(0), memberProps.ZVal())
			// Key 1: internal data
			internalData := phpv.NewZArray()
			internalData.OffsetSet(ctx, phpv.ZString("flags"), phpv.ZInt(0).ZVal())
			heapElements := phpv.NewZArray()
			if d != nil {
				for i, entry := range d.heap.entries {
					heapElements.OffsetSet(ctx, phpv.ZInt(i), entry.value)
				}
			}
			internalData.OffsetSet(ctx, phpv.ZString("heap_elements"), heapElements.ZVal())
			result.OffsetSet(ctx, phpv.ZInt(1), internalData.ZVal())
			return result.ZVal(), nil
		})},
		"__unserialize": {Name: "__unserialize", Method: phpobj.NativeMethod(func(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
			if len(args) == 0 || args[0] == nil {
				return nil, nil
			}
			arr := args[0].AsArray(ctx)
			if arr == nil {
				return nil, phpobj.ThrowError(ctx, phpobj.UnexpectedValueException,
					fmt.Sprintf("Invalid serialization data for %s object", o.GetClass().GetName()))
			}

			// Key 0: member properties
			memberVal, merr := arr.OffsetGet(ctx, phpv.ZInt(0).ZVal())
			if merr == nil && memberVal != nil && memberVal.GetType() == phpv.ZtArray {
				memberArr := memberVal.AsArray(ctx)
				for k, v := range memberArr.Iterate(ctx) {
					o.ObjectSet(ctx, k, v)
				}
			}

			// Key 1: internal data
			internalVal, err := arr.OffsetGet(ctx, phpv.ZInt(1).ZVal())
			if err != nil || internalVal == nil || internalVal.GetType() != phpv.ZtArray {
				return nil, phpobj.ThrowError(ctx, phpobj.UnexpectedValueException,
					fmt.Sprintf("Invalid serialization data for %s object", o.GetClass().GetName()))
			}
			internal := internalVal.AsArray(ctx)
			heapElementsVal, err := internal.OffsetGet(ctx, phpv.ZString("heap_elements").ZVal())
			if err != nil || heapElementsVal == nil || heapElementsVal.GetType() != phpv.ZtArray {
				return nil, phpobj.ThrowError(ctx, phpobj.UnexpectedValueException,
					fmt.Sprintf("Invalid serialization data for %s object", o.GetClass().GetName()))
			}
			elements := heapElementsVal.AsArray(ctx)
			// Determine compare function based on class
			var compareFn func(phpv.Context, *phpv.ZVal, *phpv.ZVal) (int, error)
			cls := o.GetClass()
			switch {
			case cls == SplMinHeapClass || cls.InstanceOf(SplMinHeapClass):
				compareFn = minHeapCompare
			case cls == SplMaxHeapClass || cls.InstanceOf(SplMaxHeapClass):
				compareFn = maxHeapCompare
			default:
				compareFn = func(ctx2 phpv.Context, a, b *phpv.ZVal) (int, error) {
					return userCompare(ctx2, o, a, b)
				}
			}
			d := initHeapData(ctx, o, compareFn)
			it := elements.NewIterator()
			for ; it.Valid(ctx); it.Next(ctx) {
				v, _ := it.Current(ctx)
				d.heap.Push(&splHeapEntry{value: v, index: d.nextIndex})
				d.nextIndex++
			}
			heap.Init(d.heap)
			return nil, nil
		})},
		"__debuginfo": {
			Name: "__debugInfo",
			Method: phpobj.NativeMethod(func(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
				d := getHeapData(o)
				result := phpv.NewZArray()

				// Determine class name for private property prefix
				className := "SplHeap"

				// flags (private to SplHeap)
				result.OffsetSet(ctx, phpv.ZString("\x00"+className+"\x00flags"), phpv.ZInt(0).ZVal())

				// isCorrupted (private to SplHeap)
				corrupted := false
				if d != nil {
					corrupted = d.corrupted
				}
				result.OffsetSet(ctx, phpv.ZString("\x00"+className+"\x00isCorrupted"), phpv.ZBool(corrupted).ZVal())

				// heap (private to SplHeap) - array of values
				heapArr := phpv.NewZArray()
				if d != nil {
					for i, entry := range d.heap.entries {
						heapArr.OffsetSet(ctx, phpv.ZInt(i), entry.value)
					}
				}
				result.OffsetSet(ctx, phpv.ZString("\x00"+className+"\x00heap"), heapArr.ZVal())

				return result.ZVal(), nil
			}),
		},
	}

	// Copy parent methods into SplMinHeap and SplMaxHeap, then override compare
	SplMinHeapClass.Methods = make(map[phpv.ZString]*phpv.ZClassMethod)
	for k, v := range SplHeapClass.Methods {
		SplMinHeapClass.Methods[k] = v
	}
	SplMinHeapClass.Methods["compare"] = &phpv.ZClassMethod{
		Name:      "compare",
		Modifiers: phpv.ZAttrProtected,
		Method: phpobj.NativeMethod(func(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
			if len(args) < 2 {
				return phpv.ZInt(0).ZVal(), nil
			}
			// SplMinHeap::compare($a, $b) returns > 0 when $b > $a (smaller values first)
			cmp, err := phpv.Compare(ctx, args[1], args[0])
			if err != nil {
				return phpv.ZInt(0).ZVal(), nil
			}
			return phpv.ZInt(cmp).ZVal(), nil
		}),
	}

	SplMaxHeapClass.Methods = make(map[phpv.ZString]*phpv.ZClassMethod)
	for k, v := range SplHeapClass.Methods {
		SplMaxHeapClass.Methods[k] = v
	}
	SplMaxHeapClass.Methods["compare"] = &phpv.ZClassMethod{
		Name:      "compare",
		Modifiers: phpv.ZAttrProtected,
		Method: phpobj.NativeMethod(func(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
			if len(args) < 2 {
				return phpv.ZInt(0).ZVal(), nil
			}
			// SplMaxHeap::compare($a, $b) returns > 0 when $a > $b (larger values first)
			cmp, err := phpv.Compare(ctx, args[0], args[1])
			if err != nil {
				return phpv.ZInt(0).ZVal(), nil
			}
			return phpv.ZInt(cmp).ZVal(), nil
		}),
	}
}
