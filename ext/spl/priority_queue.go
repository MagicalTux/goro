package spl

import (
	"container/heap"
	"fmt"

	"github.com/MagicalTux/goro/core/phpobj"
	"github.com/MagicalTux/goro/core/phpv"
)

const (
	splPriorityQueueExtrData     = 1
	splPriorityQueueExtrPriority = 2
	splPriorityQueueExtrBoth     = 3
)

// priorityEntry holds a value and its priority
type priorityEntry struct {
	value    *phpv.ZVal
	priority *phpv.ZVal
	index    int // insertion order for stable sort
}

// priorityHeap implements heap.Interface
type priorityHeap struct {
	entries   []*priorityEntry
	ctx       phpv.Context
	compareFn func(ctx phpv.Context, a, b *phpv.ZVal) (int, error) // custom compare function
	data      *splPriorityQueueData                                 // back-reference for corruption tracking
}

func (h *priorityHeap) Len() int { return len(h.entries) }

func (h *priorityHeap) Less(i, j int) bool {
	var cmp int
	var err error
	if h.compareFn != nil {
		cmp, err = h.compareFn(h.ctx, h.entries[i].priority, h.entries[j].priority)
	} else {
		// Higher priority comes first (max-heap)
		cmp, err = phpv.Compare(h.ctx, h.entries[i].priority, h.entries[j].priority)
	}
	if err != nil {
		if h.data != nil {
			h.data.corrupted = true
			h.data.compareErr = err
		}
		return false
	}
	if cmp != 0 {
		return cmp > 0
	}
	// Stable: earlier insertions come first
	return h.entries[i].index < h.entries[j].index
}

func (h *priorityHeap) Swap(i, j int) {
	h.entries[i], h.entries[j] = h.entries[j], h.entries[i]
}

func (h *priorityHeap) Push(x interface{}) {
	h.entries = append(h.entries, x.(*priorityEntry))
}

func (h *priorityHeap) Pop() interface{} {
	old := h.entries
	n := len(old)
	item := old[n-1]
	old[n-1] = nil
	h.entries = old[:n-1]
	return item
}

// splPriorityQueueData holds the internal state
type splPriorityQueueData struct {
	heap         *priorityHeap
	extractFlags int
	nextIndex    int
	corrupted    bool
	compareErr   error
	// iterKey tracks the key during iteration
	iterKey int
}

func (d *splPriorityQueueData) Clone() any {
	nd := &splPriorityQueueData{
		heap: &priorityHeap{
			entries: make([]*priorityEntry, len(d.heap.entries)),
			ctx:     d.heap.ctx,
		},
		extractFlags: d.extractFlags,
		nextIndex:    d.nextIndex,
		corrupted:    d.corrupted,
		iterKey:      d.iterKey,
	}
	copy(nd.heap.entries, d.heap.entries)
	return nd
}

func getPriorityQueueData(o *phpobj.ZObject) *splPriorityQueueData {
	d := o.GetOpaque(SplPriorityQueueClass)
	if d == nil {
		return nil
	}
	return d.(*splPriorityQueueData)
}

func (d *splPriorityQueueData) extractValue(entry *priorityEntry) *phpv.ZVal {
	switch d.extractFlags {
	case splPriorityQueueExtrPriority:
		return entry.priority
	case splPriorityQueueExtrBoth:
		arr := phpv.NewZArray()
		arr.OffsetSet(nil, phpv.ZString("data"), entry.value)
		arr.OffsetSet(nil, phpv.ZString("priority"), entry.priority)
		return arr.ZVal()
	default: // EXTR_DATA
		return entry.value
	}
}

func initPriorityQueueData(ctx phpv.Context, o *phpobj.ZObject) *splPriorityQueueData {
	d := &splPriorityQueueData{
		extractFlags: splPriorityQueueExtrData,
		nextIndex:    0,
	}
	d.heap = &priorityHeap{ctx: ctx, data: d}
	heap.Init(d.heap)
	o.SetOpaque(SplPriorityQueueClass, d)
	return d
}

func initPriorityQueue() {
	SplPriorityQueueClass.Methods = map[phpv.ZString]*phpv.ZClassMethod{
		"__construct": {
			Name: "__construct",
			Method: phpobj.NativeMethod(func(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
				d := &splPriorityQueueData{
					extractFlags: splPriorityQueueExtrData,
					nextIndex:    0,
				}
				h := &priorityHeap{
					entries: nil,
					ctx:     ctx,
					data:    d,
				}
				// Check if the user has overridden the compare method
				cls := o.GetClass()
				if cls != SplPriorityQueueClass {
					// User subclass - call PHP compare() method
					h.compareFn = func(ctx phpv.Context, a, b *phpv.ZVal) (int, error) {
						result, err := o.CallMethod(ctx, "compare", a, b)
						if err != nil {
							return 0, err
						}
						return int(result.AsInt(ctx)), nil
					}
				}
				d.heap = h
				heap.Init(d.heap)
				o.SetOpaque(SplPriorityQueueClass, d)
				return nil, nil
			}),
		},
		"compare": {
			Name:      "compare",
			Modifiers: phpv.ZAttrPublic,
			Method: phpobj.NativeMethod(func(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
				if len(args) < 2 {
					return phpv.ZInt(0).ZVal(), nil
				}
				cmp, err := phpv.Compare(ctx, args[0], args[1])
				if err != nil {
					return phpv.ZInt(0).ZVal(), nil
				}
				return phpv.ZInt(cmp).ZVal(), nil
			}),
		},
		"insert": {
			Name: "insert",
			Method: phpobj.NativeMethod(func(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
				d := getPriorityQueueData(o)
				if d == nil {
					return phpv.ZTrue.ZVal(), nil
				}
				if d.corrupted {
					return nil, phpobj.ThrowError(ctx, phpobj.RuntimeException, "Heap is corrupted, heap properties are no longer ensured.")
				}
				if len(args) < 2 {
					return nil, phpobj.ThrowError(ctx, phpobj.TypeError, "SplPriorityQueue::insert() expects exactly 2 arguments")
				}
				d.heap.ctx = ctx
				d.compareErr = nil
				entry := &priorityEntry{
					value:    args[0].Dup(),
					priority: args[1].Dup(),
					index:    d.nextIndex,
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
				d := getPriorityQueueData(o)
				if d == nil || d.heap.Len() == 0 {
					return nil, phpobj.ThrowError(ctx, phpobj.RuntimeException, "Can't extract from an empty heap")
				}
				if d.corrupted {
					return nil, phpobj.ThrowError(ctx, phpobj.RuntimeException, "Heap is corrupted, heap properties are no longer ensured.")
				}
				d.heap.ctx = ctx
				d.compareErr = nil
				entry := heap.Pop(d.heap).(*priorityEntry)
				if d.compareErr != nil {
					err := d.compareErr
					d.compareErr = nil
					return nil, err
				}
				return d.extractValue(entry), nil
			}),
		},
		"current": {
			Name: "current",
			Method: phpobj.NativeMethod(func(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
				d := getPriorityQueueData(o)
				if d == nil || d.heap.Len() == 0 {
					return phpv.ZNULL.ZVal(), nil
				}
				d.heap.ctx = ctx
				// Return top element without removing
				entry := d.heap.entries[0]
				return d.extractValue(entry), nil
			}),
		},
		"count": {
			Name: "count",
			Method: phpobj.NativeMethod(func(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
				d := getPriorityQueueData(o)
				if d == nil {
					return phpv.ZInt(0).ZVal(), nil
				}
				return phpv.ZInt(d.heap.Len()).ZVal(), nil
			}),
		},
		"isempty": {
			Name: "isEmpty",
			Method: phpobj.NativeMethod(func(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
				d := getPriorityQueueData(o)
				if d == nil {
					return phpv.ZTrue.ZVal(), nil
				}
				return phpv.ZBool(d.heap.Len() == 0).ZVal(), nil
			}),
		},
		"top": {
			Name: "top",
			Method: phpobj.NativeMethod(func(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
				d := getPriorityQueueData(o)
				if d == nil || d.heap.Len() == 0 {
					return nil, phpobj.ThrowError(ctx, phpobj.RuntimeException, "Can't peek at an empty heap")
				}
				if d.corrupted {
					return nil, phpobj.ThrowError(ctx, phpobj.RuntimeException, "Heap is corrupted, heap properties are no longer ensured.")
				}
				d.heap.ctx = ctx
				entry := d.heap.entries[0]
				return d.extractValue(entry), nil
			}),
		},
		"key": {
			Name: "key",
			Method: phpobj.NativeMethod(func(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
				d := getPriorityQueueData(o)
				if d == nil || d.heap.Len() == 0 {
					return phpv.ZNULL.ZVal(), nil
				}
				// PHP SplPriorityQueue key() returns remaining_count - 1
				return phpv.ZInt(d.heap.Len() - 1).ZVal(), nil
			}),
		},
		"next": {
			Name: "next",
			Method: phpobj.NativeMethod(func(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
				d := getPriorityQueueData(o)
				if d == nil || d.heap.Len() == 0 {
					return nil, nil
				}
				d.heap.ctx = ctx
				heap.Pop(d.heap)
				d.iterKey++
				return nil, nil
			}),
		},
		"rewind": {
			Name: "rewind",
			Method: phpobj.NativeMethod(func(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
				// SplPriorityQueue::rewind() resets the key counter
				d := getPriorityQueueData(o)
				if d != nil {
					d.iterKey = 0
				}
				return nil, nil
			}),
		},
		"valid": {
			Name: "valid",
			Method: phpobj.NativeMethod(func(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
				d := getPriorityQueueData(o)
				if d == nil {
					return phpv.ZFalse.ZVal(), nil
				}
				return phpv.ZBool(d.heap.Len() > 0).ZVal(), nil
			}),
		},
		"setextractflags": {
			Name: "setExtractFlags",
			Method: phpobj.NativeMethod(func(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
				d := getPriorityQueueData(o)
				if d == nil {
					return phpv.ZInt(0).ZVal(), nil
				}
				if len(args) == 0 {
					return nil, phpobj.ThrowError(ctx, phpobj.TypeError, "SplPriorityQueue::setExtractFlags() expects exactly 1 argument")
				}
				flags := int(args[0].AsInt(ctx))
				if flags < 1 || flags > 3 {
					return nil, phpobj.ThrowError(ctx, phpobj.RuntimeException, "Must specify at least one extract flag")
				}
				d.extractFlags = flags
				return phpv.ZInt(flags).ZVal(), nil
			}),
		},
		"getextractflags": {
			Name: "getExtractFlags",
			Method: phpobj.NativeMethod(func(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
				d := getPriorityQueueData(o)
				if d == nil {
					return phpv.ZInt(splPriorityQueueExtrData).ZVal(), nil
				}
				return phpv.ZInt(d.extractFlags).ZVal(), nil
			}),
		},
		"iscorrupted": {
			Name: "isCorrupted",
			Method: phpobj.NativeMethod(func(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
				d := getPriorityQueueData(o)
				if d == nil {
					return phpv.ZFalse.ZVal(), nil
				}
				return phpv.ZBool(d.corrupted).ZVal(), nil
			}),
		},
		"recoverfromcorruption": {
			Name: "recoverFromCorruption",
			Method: phpobj.NativeMethod(func(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
				d := getPriorityQueueData(o)
				if d != nil {
					d.corrupted = false
				}
				return phpv.ZTrue.ZVal(), nil
			}),
		},
		"__serialize": {Name: "__serialize", Method: phpobj.NativeMethod(func(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
			d := getPriorityQueueData(o)
			if d != nil && d.corrupted {
				return nil, phpobj.ThrowError(ctx, phpobj.RuntimeException, "Heap is corrupted, heap properties are no longer ensured.")
			}
			result := phpv.NewZArray()
			result.OffsetSet(ctx, phpv.ZInt(0), phpv.NewZArray().ZVal())
			internalData := phpv.NewZArray()
			flags := splPriorityQueueExtrData
			if d != nil {
				flags = d.extractFlags
			}
			internalData.OffsetSet(ctx, phpv.ZString("flags"), phpv.ZInt(flags).ZVal())
			heapElements := phpv.NewZArray()
			if d != nil {
				for i, entry := range d.heap.entries {
					pair := phpv.NewZArray()
					pair.OffsetSet(ctx, phpv.ZString("data"), entry.value)
					pair.OffsetSet(ctx, phpv.ZString("priority"), entry.priority)
					heapElements.OffsetSet(ctx, phpv.ZInt(i), pair.ZVal())
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
			internalVal, err := arr.OffsetGet(ctx, phpv.ZInt(1).ZVal())
			if err != nil || internalVal == nil || internalVal.GetType() != phpv.ZtArray {
				return nil, phpobj.ThrowError(ctx, phpobj.UnexpectedValueException,
					fmt.Sprintf("Invalid serialization data for %s object", o.GetClass().GetName()))
			}
			internal := internalVal.AsArray(ctx)
			d := initPriorityQueueData(ctx, o)
			if flagsVal, err2 := internal.OffsetGet(ctx, phpv.ZString("flags").ZVal()); err2 == nil && flagsVal != nil {
				d.extractFlags = int(flagsVal.AsInt(ctx))
			}
			heapElementsVal, err := internal.OffsetGet(ctx, phpv.ZString("heap_elements").ZVal())
			if err != nil || heapElementsVal == nil || heapElementsVal.GetType() != phpv.ZtArray {
				return nil, nil
			}
			elements := heapElementsVal.AsArray(ctx)
			it := elements.NewIterator()
			for ; it.Valid(ctx); it.Next(ctx) {
				v, _ := it.Current(ctx)
				pairArr := v.AsArray(ctx)
				if pairArr == nil {
					continue
				}
				dataVal, _ := pairArr.OffsetGet(ctx, phpv.ZString("data").ZVal())
				priorityVal, _ := pairArr.OffsetGet(ctx, phpv.ZString("priority").ZVal())
				if dataVal == nil || priorityVal == nil {
					continue
				}
				d.heap.Push(&priorityEntry{value: dataVal, priority: priorityVal, index: d.nextIndex})
				d.nextIndex++
			}
			heap.Init(d.heap)
			return nil, nil
		})},
		"__debuginfo": {
			Name: "__debugInfo",
			Method: phpobj.NativeMethod(func(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
				d := getPriorityQueueData(o)
				result := phpv.NewZArray()

				className := "SplPriorityQueue"

				// flags (private)
				flags := splPriorityQueueExtrData
				if d != nil {
					flags = d.extractFlags
				}
				result.OffsetSet(ctx, phpv.ZString("\x00"+className+"\x00flags"), phpv.ZInt(flags).ZVal())

				// isCorrupted (private)
				corrupted := false
				if d != nil {
					corrupted = d.corrupted
				}
				result.OffsetSet(ctx, phpv.ZString("\x00"+className+"\x00isCorrupted"), phpv.ZBool(corrupted).ZVal())

				// heap (private) - array of data/priority pairs
				heapArr := phpv.NewZArray()
				if d != nil {
					for i, entry := range d.heap.entries {
						pair := phpv.NewZArray()
						pair.OffsetSet(ctx, phpv.ZString("data"), entry.value)
						pair.OffsetSet(ctx, phpv.ZString("priority"), entry.priority)
						heapArr.OffsetSet(ctx, phpv.ZInt(i), pair.ZVal())
					}
				}
				result.OffsetSet(ctx, phpv.ZString("\x00"+className+"\x00heap"), heapArr.ZVal())

				return result.ZVal(), nil
			}),
		},
	}
}

var SplPriorityQueueClass = &phpobj.ZClass{
	Name:            "SplPriorityQueue",
	Implementations: []*phpobj.ZClass{phpobj.Iterator, Countable},
	Const: map[phpv.ZString]*phpv.ZClassConst{
		"EXTR_DATA":     {Value: phpv.ZInt(splPriorityQueueExtrData)},
		"EXTR_PRIORITY": {Value: phpv.ZInt(splPriorityQueueExtrPriority)},
		"EXTR_BOTH":     {Value: phpv.ZInt(splPriorityQueueExtrBoth)},
	},
}
