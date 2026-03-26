package stream

import (
	"github.com/MagicalTux/goro/core/phpobj"
	"github.com/MagicalTux/goro/core/phpv"
)

// UserFilter wraps a PHP user filter object (extending php_user_filter)
// to implement the StreamFilter interface.
type UserFilter struct {
	ctx          phpv.Context
	obj          *phpobj.ZObject
	stream       *Stream
	filterName   string
	params       *phpv.ZVal
	lastStreamErr error // error from setStreamProperty (e.g., TypeError for typed properties)
}

// NewUserFilter creates a new user filter instance.
func NewUserFilter(ctx phpv.Context, obj *phpobj.ZObject, stream *Stream, filterName string, params *phpv.ZVal) *UserFilter {
	return &UserFilter{
		ctx:        ctx,
		obj:        obj,
		stream:     stream,
		filterName: filterName,
		params:     params,
	}
}

// GetObject returns the underlying PHP object
func (f *UserFilter) GetObject() *phpobj.ZObject {
	return f.obj
}

// Process implements StreamFilter by calling the PHP filter() method
func (f *UserFilter) Process(data []byte, closing bool) ([]byte, error) {
	// Use the stream's filter context if available (it's set from the calling code),
	// falling back to the stored context from filter creation
	ctx := f.ctx
	if f.stream != nil && f.stream.filterCtx != nil {
		ctx = f.stream.filterCtx
	}

	// Create input brigade (list of buckets)
	inBrigade := NewBucketBrigade()
	inBrigade.SetCtx(ctx)
	if len(data) > 0 {
		inBrigade.Append(&Bucket{Data: data})
	}

	// Create output brigade
	outBrigade := NewBucketBrigade()
	outBrigade.SetCtx(ctx)

	// Create consumed reference
	consumed := phpv.ZInt(0).ZVal()
	consumed.MakeRef()

	// Set the stream property on the filter object (if the property exists and is appropriate)
	f.lastStreamErr = nil
	f.setStreamProperty()
	if f.lastStreamErr != nil {
		return nil, f.lastStreamErr
	}

	// Mark the stream as being inside a filter operation
	if f.stream != nil {
		f.stream.InFilter = true
		defer func() { f.stream.InFilter = false }()
	}

	// Call filter($in, $out, &$consumed, $closing)
	result, err := f.obj.CallMethod(ctx, "filter",
		inBrigade.ZVal(),
		outBrigade.ZVal(),
		consumed,
		phpv.ZBool(closing).ZVal(),
	)
	if err != nil {
		return nil, err
	}

	// Check return value
	retCode := PSFS_FEED_ME
	if result != nil {
		retCode = int(result.AsInt(ctx))
	}

	switch retCode {
	case PSFS_PASS_ON:
		// Collect output from out brigade
		// Check if there are unprocessed buckets remaining on the input brigade
		if len(inBrigade.buckets) > 0 {
			// There are unprocessed input buckets - this is a warning condition
			return outBrigade.CollectData(), &FilterWarning{
				Message: "Unprocessed filter buckets remaining on input brigade",
				Data:    outBrigade.CollectData(),
			}
		}
		return outBrigade.CollectData(), nil
	case PSFS_FEED_ME:
		// Filter needs more data, return nothing
		return nil, nil
	case PSFS_ERR_FATAL:
		// Fatal error - check for unprocessed input buckets
		if len(inBrigade.buckets) > 0 {
			return nil, &FilterFatalError{
				UnprocessedBuckets: true,
			}
		}
		return nil, &FilterFatalError{
			UnprocessedBuckets: false,
		}
	}

	return nil, nil
}

// OnClose calls the onClose() method on the PHP filter object
func (f *UserFilter) OnClose() {
	if f.obj == nil {
		return
	}
	// Only call onClose if the method exists
	if _, ok := f.obj.Class.GetMethod("onclose"); !ok {
		return
	}
	// Call onClose safely
	defer func() { recover() }()
	ctx := f.ctx
	if f.stream != nil && f.stream.filterCtx != nil {
		ctx = f.stream.filterCtx
	}
	f.obj.CallMethod(ctx, "onClose")
}

// setStreamProperty sets $this->stream to the stream resource if the property
// is declared (or is a dynamic property) and compatible.
func (f *UserFilter) setStreamProperty() {
	if f.stream == nil || f.obj == nil || f.ctx == nil {
		return
	}

	ctx := f.ctx
	if f.stream.filterCtx != nil {
		ctx = f.stream.filterCtx
	}

	// Check if the class has a "stream" property declared
	// If not, don't create it dynamically (PHP 8.2+ deprecates dynamic properties)
	class := f.obj.Class
	hasProp := false
	if zc, ok := class.(*phpobj.ZClass); ok {
		for _, p := range zc.Props {
			if p.VarName == "stream" {
				hasProp = true
				break
			}
		}
		// Also check parent class
		if !hasProp {
			parent := zc.GetParent()
			for parent != nil {
				if pzc, ok := parent.(*phpobj.ZClass); ok {
					for _, p := range pzc.Props {
						if p.VarName == "stream" {
							hasProp = true
							break
						}
					}
					if hasProp {
						break
					}
					parent = pzc.GetParent()
				} else {
					break
				}
			}
		}
	}

	// Also check if the property was dynamically created (e.g., in onCreate)
	if !hasProp {
		if exists, _ := f.obj.HasProp(ctx, phpv.ZStr("stream")); exists {
			hasProp = true
		}
	}

	if !hasProp {
		return
	}

	// Check if the property has a type hint that is incompatible with resource
	if zc, ok := class.(*phpobj.ZClass); ok {
		for _, p := range zc.Props {
			if p.VarName == "stream" && p.TypeHint != nil {
				// Has a type hint - check if resource is compatible
				// If the type is not "resource" and not mixed/null, skip setting
				th := p.TypeHint.String()
				if th != "" && th != "mixed" && th != "resource" {
					// Will throw TypeError - let it propagate
					f.lastStreamErr = f.obj.ObjectSet(ctx, phpv.ZStr("stream"), f.stream.ZVal())
					return
				}
			}
		}
	}

	// No type conflict - set directly (recover from any unexpected panics)
	defer func() { recover() }()
	f.obj.ObjectSet(ctx, phpv.ZStr("stream"), f.stream.ZVal())
}

// --- Bucket Brigade ---

// Bucket represents a single bucket of data in a bucket brigade
type Bucket struct {
	Data []byte
	obj  *phpobj.ZObject // the PHP object representation
}

// BucketBrigade represents a chain of buckets passed between filters
type BucketBrigade struct {
	buckets []*Bucket
	ctx     phpv.Context
	id      int
}

var nextBrigadeID int

func NewBucketBrigade() *BucketBrigade {
	nextBrigadeID++
	return &BucketBrigade{
		id: nextBrigadeID,
	}
}

// SetCtx sets the context for the brigade (used for collecting data)
func (bb *BucketBrigade) SetCtx(ctx phpv.Context) {
	bb.ctx = ctx
}

func (bb *BucketBrigade) Append(b *Bucket) {
	bb.buckets = append(bb.buckets, b)
}

func (bb *BucketBrigade) Prepend(b *Bucket) {
	bb.buckets = append([]*Bucket{b}, bb.buckets...)
}

// MakeWriteable removes and returns the first bucket, or nil if empty
func (bb *BucketBrigade) MakeWriteable(ctx phpv.Context) *phpv.ZVal {
	if len(bb.buckets) == 0 {
		return phpv.ZNULL.ZVal()
	}
	b := bb.buckets[0]
	bb.buckets = bb.buckets[1:]

	// Create or return the bucket object with data/datalen properties
	if b.obj != nil {
		return b.obj.ZVal()
	}

	obj, err := phpobj.NewZObject(ctx, phpobj.StdClass)
	if err != nil {
		return phpv.ZNULL.ZVal()
	}
	// Use ObjectSet for property access ($bucket->data), not OffsetSet (which is for $bucket['data'])
	obj.ObjectSet(ctx, phpv.ZStr("data"), phpv.ZString(b.Data).ZVal())
	obj.ObjectSet(ctx, phpv.ZStr("datalen"), phpv.ZInt(len(b.Data)).ZVal())
	b.obj = obj
	return obj.ZVal()
}

// CollectData collects all data from the brigade into a single byte slice
func (bb *BucketBrigade) CollectData() []byte {
	var result []byte
	for _, b := range bb.buckets {
		if b.obj != nil && bb.ctx != nil {
			// Read data from the PHP object's "data" property (it may have been modified by user code)
			if dataVal, err := b.obj.ObjectGet(bb.ctx, phpv.ZStr("data")); err == nil && dataVal != nil {
				result = append(result, []byte(dataVal.AsString(bb.ctx))...)
				continue
			}
		}
		result = append(result, b.Data...)
	}
	return result
}

// ZVal returns the brigade as a ZVal (opaque value for PHP)
func (bb *BucketBrigade) ZVal() *phpv.ZVal {
	return phpv.NewZVal(bb)
}

func (bb *BucketBrigade) GetType() phpv.ZType { return phpv.ZtResource }
func (bb *BucketBrigade) Value() phpv.Val     { return bb }
func (bb *BucketBrigade) String() string      { return "bucket brigade" }
func (bb *BucketBrigade) AsVal(ctx phpv.Context, t phpv.ZType) (phpv.Val, error) {
	switch t {
	case phpv.ZtResource:
		return bb, nil
	case phpv.ZtBool:
		return phpv.ZBool(true), nil
	}
	return nil, ErrNotSupported
}

// AppendBucketObj appends a PHP bucket object to the brigade.
// If the object is already in the brigade, it is moved to the end (like PHP's linked list behavior).
func (bb *BucketBrigade) AppendBucketObj(obj *phpobj.ZObject) {
	// Remove existing entry with same object (PHP moves the bucket rather than duplicating)
	bb.removeBucketObj(obj)
	b := &Bucket{obj: obj}
	bb.buckets = append(bb.buckets, b)
}

// PrependBucketObj prepends a PHP bucket object to the brigade.
// If the object is already in the brigade, it is moved to the front.
func (bb *BucketBrigade) PrependBucketObj(obj *phpobj.ZObject) {
	bb.removeBucketObj(obj)
	b := &Bucket{obj: obj}
	bb.buckets = append([]*Bucket{b}, bb.buckets...)
}

// removeBucketObj removes any bucket with the given object from the brigade
func (bb *BucketBrigade) removeBucketObj(obj *phpobj.ZObject) {
	newBuckets := make([]*Bucket, 0, len(bb.buckets))
	for _, b := range bb.buckets {
		if b.obj != obj {
			newBuckets = append(newBuckets, b)
		}
	}
	bb.buckets = newBuckets
}
