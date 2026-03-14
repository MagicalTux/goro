package spl

import (
	"container/heap"

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
	compareFn func(ctx phpv.Context, a, b *phpv.ZVal) (int, error)
	ctx       phpv.Context
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
	}
	copy(nd.heap.entries, d.heap.entries)
	nd.heap.less = nd.makeLess()
	return nd
}

func (d *splHeapData) makeLess() func(a, b *splHeapEntry) bool {
	return func(a, b *splHeapEntry) bool {
		cmp, err := d.compareFn(d.ctx, a.value, b.value)
		if err != nil {
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
				if len(args) == 0 {
					return nil, phpobj.ThrowError(ctx, phpobj.TypeError, "SplHeap::insert() expects exactly 1 argument")
				}
				d.ctx = ctx
				entry := &splHeapEntry{
					value: args[0],
					index: d.nextIndex,
				}
				d.nextIndex++
				heap.Push(d.heap, entry)
				return phpv.ZTrue.ZVal(), nil
			}),
		},
		"extract": {
			Name: "extract",
			Method: phpobj.NativeMethod(func(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
				d := getHeapData(o)
				if d == nil || d.heap.Len() == 0 {
					return nil, phpobj.ThrowError(ctx, phpobj.RuntimeException, "Can't extract from an empty datastructure")
				}
				d.ctx = ctx
				entry := heap.Pop(d.heap).(*splHeapEntry)
				return entry.value, nil
			}),
		},
		"top": {
			Name: "top",
			Method: phpobj.NativeMethod(func(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
				d := getHeapData(o)
				if d == nil || d.heap.Len() == 0 {
					return nil, phpobj.ThrowError(ctx, phpobj.RuntimeException, "Can't peek at an empty datastructure")
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
					return phpv.ZFalse.ZVal(), nil
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
				return phpv.ZInt(d.nextIndex - d.heap.Len()).ZVal(), nil
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
				heap.Pop(d.heap)
				return nil, nil
			}),
		},
		"rewind": {
			Name: "rewind",
			Method: phpobj.NativeMethod(func(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
				// SplHeap::rewind() is a no-op - the heap is iterated destructively
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
