package phpv

import "sync"

// Val is a basic value of any PHP kind: null, bool, int, float, string, array, resource or object.
type Val interface {
	GetType() ZType                          // GetType returns the type of the value
	ZVal() *ZVal                             // ZVal returns a ZVal pointing to this value
	Value() Val                              // Value returns the raw value, in case it was in a ZVal
	AsVal(ctx Context, t ZType) (Val, error) // AsVal converts the value to another type
	String() string                          // String should only be used on ZtString values
}

// ZVal is a pointer to a value, that can be used as a Val, a reference, etc.
//
// Eventually, ZVal will only be used for references.
type ZVal struct {
	v    Val
	Name *ZString
	// refCount tracks how many outer ZVals point to this ZVal when used as a
	// reference inner value. Used to determine when to un-ref compound writable
	// by-ref args: if refCount > 1 after a call, another location still
	// references it (e.g. $this->prop = &$param), so don't un-ref.
	refCount int
	// typeCheckers holds type constraint/coercion functions attached to this inner
	// reference ZVal. When this ZVal's value is changed through the reference
	// (e.g. $r = new B where $r is a reference held by typed properties),
	// all type checkers are evaluated. The function may return a coerced Val
	// (non-nil means "use this value instead") or an error if the type is
	// incompatible. Only inner ZVals (refCount > 0) should carry type checkers.
	typeCheckers []func(Context, Val) (Val, error)
	cached       bool // true for pre-allocated cached ZVals (must not be mutated)
}

func NewZVal(v Val) *ZVal {
	return &ZVal{v: v}
}

// MakeCachedZVal returns a cached, immutable *ZVal wrapping v. Used by the
// compiler to pre-build *ZVal wrappers for literals that are evaluated many
// times (e.g. the bound `100000` in `for ($i = 0; $i < 100000; $i++)`).
// The returned ZVal has the cached flag set; any mutation via Set/SetVal
// panics. Name mutations go through SetName which Dup's instead.
func MakeCachedZVal(v Val) *ZVal {
	return &ZVal{v: v, cached: true}
}

// isCached returns true if z is one of the pre-allocated cached ZVals.
func (z *ZVal) isCached() bool {
	return z != nil && z.cached
}

// IsCached is the exported form of isCached. The VM uses this to
// decide whether to Dup a value before storing it in a slot — the
// hashtable already does the same when SetString receives a cached
// pointer.
func (z *ZVal) IsCached() bool {
	return z != nil && z.cached
}

// zvalPool holds short-lived ZVals that are safe to reuse when the caller
// explicitly returns them via PutTempZVal. Used for operator snapshots and
// other temporary wrappers whose lifetime is tightly bounded.
var zvalPool = sync.Pool{
	New: func() any { return &ZVal{} },
}

// NewTempZVal returns a ZVal from the pool wrapping v. The caller MUST call
// PutTempZVal on the returned pointer before its lifetime ends. The ZVal must
// not be stored (in a hash table, outlive the current function, etc.) - only
// used for short-lived intermediate values.
func NewTempZVal(v Val) *ZVal {
	z := zvalPool.Get().(*ZVal)
	z.v = v
	z.Name = nil
	z.refCount = 0
	z.typeCheckers = nil
	return z
}

// PutTempZVal returns a ZVal obtained via NewTempZVal to the pool. Safe to
// call with nil.
func PutTempZVal(z *ZVal) {
	if z == nil {
		return
	}
	z.v = nil
	z.Name = nil
	z.refCount = 0
	z.typeCheckers = nil
	zvalPool.Put(z)
}

func (z *ZVal) GetType() ZType {
	if z == nil {
		return ZtNull
	}
	if z.v == nil {
		return ZtNull
	}
	return z.v.GetType()
}

func MakeZVal(v Val) *ZVal {
	return NewZVal(v)
}

// ZVal will make a copy of a given zval without actually copying memory
func (z *ZVal) ZVal() *ZVal {
	if z == nil || z.GetType() == ZtNull {
		return ZNULL.ZVal()
	}
	if z.v == nil {
		panic("nil zval!")
	}

	var res *ZVal
	switch a := z.v.(type) {
	case *ZArray:
		// special case
		res = a.Dup().ZVal()
	default:
		// Always allocate a fresh ZVal wrapper here (never a cached one)
		// because we will mutate res.Name below. Primitive .ZVal() methods
		// may return cached/pooled instances that we must not mutate.
		res = &ZVal{v: a}
	}
	res.Name = z.Name
	return res
}

// Returns actual zval, dropping status of reference
func (z *ZVal) Nude() *ZVal {
	var res *ZVal
	switch v := z.v.(type) {
	case *ZVal:
		res = v.Nude()
	default:
		res = z
	}
	res.Name = z.Name
	return res
}

func (z *ZVal) Dup() *ZVal {
	if z == nil || z.GetType() == ZtNull {
		return ZNULL.ZVal()
	}

	var res *ZVal
	switch v := z.v.(type) {
	case *ZVal:
		// detach reference
		res = v.Dup()
	case *ZArray:
		res = (&ZArray{h: v.h.Dup()}).ZVal()
	default:
		res = NewZVal(z.v)
	}
	res.Name = z.Name
	return res
}

// Ref returns a reference to this zval while making it itself a ref.
// If z is an inner reference ZVal (has refCount > 0), creating a new
// wrapper increments the refCount to track the additional alias.
func (z *ZVal) Ref() *ZVal {
	if _, isRef := z.v.(*ZVal); isRef {
		return z
	}
	if z.refCount > 0 {
		z.refCount++
	}
	return NewZVal(z)
}

func (z *ZVal) IsRef() bool {
	if z == nil {
		return false
	}
	_, isRef := z.v.(*ZVal)
	return isRef
}

// MakeRef converts a plain ZVal into a reference in-place by wrapping its
// current value in an inner ZVal. If already a reference, this is a no-op.
// This is used when a hash table entry needs to become a reference without
// going through Set() which has self-reference detection that creates an
// unwanted double-reference.
func (z *ZVal) MakeRef() {
	if z == nil {
		return
	}
	if z.isCached() {
		panic("MakeRef() called on cached ZVal")
	}
	if _, isRef := z.v.(*ZVal); isRef {
		return // already a ref
	}
	z.v = &ZVal{v: z.v, refCount: 1}
}

// RefTarget returns the inner ZVal of a reference without incrementing refCount.
// Used for identity comparison (e.g. serialize() detecting shared references).
// Returns nil if not a reference.
func (z *ZVal) RefTarget() *ZVal {
	if z == nil {
		return nil
	}
	if inner, ok := z.v.(*ZVal); ok {
		return inner
	}
	return nil
}

// RefInner returns the inner ZVal of a reference and increments its refCount.
// This should be called when creating a new alias to the inner value
// (e.g. $this->prop = &$param). Returns nil if not a reference.
func (z *ZVal) RefInner() *ZVal {
	if z == nil {
		return nil
	}
	if inner, ok := z.v.(*ZVal); ok {
		inner.refCount++
		return inner
	}
	return nil
}

// UnRef unwraps a reference, replacing the outer ZVal's value with the inner
// value. This simulates PHP's refcount-based un-ref when refcount drops to 1.
func (z *ZVal) UnRef() {
	if z == nil {
		return
	}
	if inner, ok := z.v.(*ZVal); ok {
		z.v = inner.v
	}
}

// AliasCount returns how many outer ZVals currently share this ZVal when it
// is used as the inner of a reference. For a non-reference (or a
// reference-inner with no recorded aliases) this is 1 — meaning "only the
// caller has a handle on it".
//
// Typical use: after `unset($alias)` on a two-way reference, the surviving
// reference-typed hash entry still carries the IS_REFERENCE flag but its
// inner's refCount drops back to 1. Code that wants to distinguish a "live"
// multi-alias reference from a phantom-ref-of-one should compare
// AliasCount() > 1.
func (z *ZVal) AliasCount() int {
	if z == nil {
		return 0
	}
	if z.refCount < 1 {
		return 1
	}
	return z.refCount
}

// UnRefIfAlone unwraps a reference only if the inner ZVal's refCount is <= 1,
// meaning no other location holds a reference to the same inner value.
// Used after function calls for compound writable by-ref args.
func (z *ZVal) UnRefIfAlone() {
	if z == nil {
		return
	}
	if inner, ok := z.v.(*ZVal); ok {
		if inner.refCount <= 1 {
			z.v = inner.v
		}
	}
}

func (z *ZVal) Value() Val {
	if z == nil {
		panic("nil zval")
	}
	if sz, ok := z.v.(*ZVal); ok {
		return sz.Value()
	}
	return z.v
}

func (z *ZVal) GetName() ZString {
	if z == nil {
		panic("nil zval")
	}
	if z.Name != nil {
		return *z.Name
	}
	if sz, ok := z.v.(*ZVal); ok {
		return sz.GetName()
	}
	return ""
}

// SetName safely assigns the Name field, Dup'ing z first if it is a cached
// shared ZVal (e.g. the singleton wrappers for small integers or
// true/false). Cached ZVals must never be mutated — otherwise a later
// reader sees the leftover name and the variadic packer turns the literal
// into a named-keyed entry like `["count"]=>int(1)`.
//
// Callers should assign the returned ZVal back to their local:
//
//	val = val.SetName(&nameStr)
//
// When z is not cached, SetName mutates in place and returns z unchanged.
func (z *ZVal) SetName(name *ZString) *ZVal {
	if z == nil {
		return nil
	}
	if z.cached {
		d := z.Dup()
		d.Name = name
		return d
	}
	z.Name = name
	return z
}

func (z *ZVal) Set(nz *ZVal) {
	if z == nil || nz == nil {
		return
	}
	// Debug: catch mutation of cached ZVals
	if z.isCached() {
		panic("Set() called on cached ZVal")
	}
	if _, isRef := nz.v.(*ZVal); isRef {
		// simple set, keep reference alive
		if z != nz.v {
			z.v = nz.v
		} else {
			z.v = NewZVal(z.v)
		}
		return
	}

	// set value of this zval's target to given zval
	if rz, isRef := z.v.(*ZVal); isRef {
		rz.Set(nz)
		return
	}

	// simple set
	z.v = nz.v
}

// SetVal is a wrapper-free fast path for Set when the new value is
// already a phpv.Val rather than a *ZVal. Equivalent to z.Set(v.ZVal())
// without allocating the intermediate ZVal — heavily exercised by
// doInc, where v is a freshly-incremented phpv.ZInt or phpv.ZFloat.
func (z *ZVal) SetVal(v Val) {
	if z == nil {
		return
	}
	if z.isCached() {
		panic("SetVal() called on cached ZVal")
	}
	if rz, isRef := z.v.(*ZVal); isRef {
		rz.SetVal(v)
		return
	}
	z.v = v
}

// HasTypeCheckers returns true if this ZVal or its referenced inner ZVal has
// any type constraint checkers registered.
func (z *ZVal) HasTypeCheckers() bool {
	if z == nil {
		return false
	}
	if len(z.typeCheckers) > 0 {
		return true
	}
	if inner, ok := z.v.(*ZVal); ok {
		return inner.HasTypeCheckers()
	}
	return false
}

// AddTypeChecker attaches a type constraint/coercion function to this ZVal.
// The function is called whenever the ZVal's value is changed via SetWithCtx.
// If the function returns a non-nil Val, that value is used instead (coercion).
// If the function returns an error, the assignment is rejected.
func (z *ZVal) AddTypeChecker(fn func(Context, Val) (Val, error)) {
	if z == nil || fn == nil {
		return
	}
	z.typeCheckers = append(z.typeCheckers, fn)
}

// SetWithCtx sets this ZVal's value (following the reference chain), running
// any registered type checkers before committing the new value.
// Returns an error if a type constraint is violated.
func (z *ZVal) SetWithCtx(ctx Context, nz *ZVal) error {
	if z == nil || nz == nil {
		return nil
	}
	if _, isRef := nz.v.(*ZVal); isRef {
		// Assigning a reference: delegate to Set.
		z.Set(nz)
		return nil
	}

	// Follow the reference chain to find the inner ZVal, running any type
	// checkers encountered along the way. Type checkers may be on any ZVal
	// in the chain (not just the deepest one). This handles the case where
	// ZVal.Set's self-reference detection creates an extra level of wrapping.
	if rz, isRef := z.v.(*ZVal); isRef {
		// If this ZVal has type checkers, run them before following the chain.
		if len(z.typeCheckers) > 0 {
			newVal := nz.v
			for _, check := range z.typeCheckers {
				coerced, err := check(ctx, newVal)
				if err != nil {
					return err
				}
				if coerced != nil {
					newVal = coerced
				}
			}
			// Update nz with the coerced value before following the chain
			nz.v = newVal
		}
		return rz.SetWithCtx(ctx, nz)
	}

	// z is the inner ZVal. Check type constraints before setting.
	// Type checkers may also coerce the value (returning a non-nil Val means
	// "use this coerced value"). The coerced value is written back to nz so
	// callers (e.g. assignment expressions) can return the coerced value.
	newVal := nz.v
	for _, check := range z.typeCheckers {
		coerced, err := check(ctx, newVal)
		if err != nil {
			return err
		}
		if coerced != nil {
			newVal = coerced
		}
	}
	z.v = newVal
	// Update nz so the caller sees the coerced value (e.g. var_dump($ref = 0)
	// should show string "0" when $ref is tied to a ?string typed property).
	nz.v = newVal
	return nil
}
