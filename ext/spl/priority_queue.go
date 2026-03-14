package spl

import (
	"container/heap"

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
	entries []*priorityEntry
	ctx     phpv.Context
}

func (h *priorityHeap) Len() int { return len(h.entries) }

func (h *priorityHeap) Less(i, j int) bool {
	// Higher priority comes first (max-heap)
	cmp, err := phpv.Compare(h.ctx, h.entries[i].priority, h.entries[j].priority)
	if err != nil {
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
	// For iteration: we build a sorted snapshot
	iterItems []*priorityEntry
	iterPos   int
}

func (d *splPriorityQueueData) Clone() any {
	nd := &splPriorityQueueData{
		heap: &priorityHeap{
			entries: make([]*priorityEntry, len(d.heap.entries)),
			ctx:     d.heap.ctx,
		},
		extractFlags: d.extractFlags,
		nextIndex:    d.nextIndex,
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

func initPriorityQueue() {
	SplPriorityQueueClass.Methods = map[phpv.ZString]*phpv.ZClassMethod{
		"__construct": {
			Name: "__construct",
			Method: phpobj.NativeMethod(func(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
				d := &splPriorityQueueData{
					heap: &priorityHeap{
						entries: nil,
						ctx:     ctx,
					},
					extractFlags: splPriorityQueueExtrData,
					nextIndex:    0,
				}
				heap.Init(d.heap)
				o.SetOpaque(SplPriorityQueueClass, d)
				return nil, nil
			}),
		},
		"insert": {
			Name: "insert",
			Method: phpobj.NativeMethod(func(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
				d := getPriorityQueueData(o)
				if d == nil {
					return phpv.ZTrue.ZVal(), nil
				}
				if len(args) < 2 {
					return nil, phpobj.ThrowError(ctx, phpobj.TypeError, "SplPriorityQueue::insert() expects exactly 2 arguments")
				}
				d.heap.ctx = ctx
				entry := &priorityEntry{
					value:    args[0],
					priority: args[1],
					index:    d.nextIndex,
				}
				d.nextIndex++
				heap.Push(d.heap, entry)
				// Invalidate iteration snapshot
				d.iterItems = nil
				return phpv.ZTrue.ZVal(), nil
			}),
		},
		"extract": {
			Name: "extract",
			Method: phpobj.NativeMethod(func(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
				d := getPriorityQueueData(o)
				if d == nil || d.heap.Len() == 0 {
					return nil, phpobj.ThrowError(ctx, phpobj.RuntimeException, "Can't extract from an empty datastructure")
				}
				d.heap.ctx = ctx
				entry := heap.Pop(d.heap).(*priorityEntry)
				// Invalidate iteration snapshot
				d.iterItems = nil
				return d.extractValue(entry), nil
			}),
		},
		"current": {
			Name: "current",
			Method: phpobj.NativeMethod(func(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
				d := getPriorityQueueData(o)
				if d == nil || d.heap.Len() == 0 {
					return phpv.ZFalse.ZVal(), nil
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
					return nil, phpobj.ThrowError(ctx, phpobj.RuntimeException, "Can't peek at an empty datastructure")
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
				// Key returns the current count position (total inserted minus remaining)
				return phpv.ZInt(d.nextIndex - d.heap.Len()).ZVal(), nil
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
				return nil, nil
			}),
		},
		"rewind": {
			Name: "rewind",
			Method: phpobj.NativeMethod(func(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
				// SplPriorityQueue::rewind() is a no-op - the queue is iterated destructively
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
					return nil, phpobj.ThrowError(ctx, phpobj.RuntimeException, "Must specify a valid extract flag")
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
