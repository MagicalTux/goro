package phpv

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
}

func NewZVal(v Val) *ZVal {
	return &ZVal{v: v}
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
	if z.GetType() == ZtNull {
		return z
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
		res = a.ZVal()
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
	if z.GetType() == ZtNull {
		return z
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

// Ref returns a reference to this zval while making it itself a ref
func (z *ZVal) Ref() *ZVal {
	if _, isRef := z.v.(*ZVal); isRef {
		return z
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

func (z *ZVal) Set(nz *ZVal) {
	if nz == nil {
		panic("nil zval")
	}
	if _, isRef := nz.v.(*ZVal); isRef {
		// simple set, keep reference alive
		z.v = nz.v
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
