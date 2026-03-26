package stream

import (
	"github.com/MagicalTux/goro/core/phpobj"
	"github.com/MagicalTux/goro/core/phpv"
)

// UserFilter wraps a PHP user filter object (extending php_user_filter)
// to implement the StreamFilter interface.
type UserFilter struct {
	ctx        phpv.Context
	obj        *phpobj.ZObject
	stream     *Stream
	filterName string
	params     *phpv.ZVal
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
	f.setStreamProperty()

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
		return outBrigade.CollectData(), nil
	case PSFS_FEED_ME:
		// Filter needs more data, return nothing
		return nil, nil
	case PSFS_ERR_FATAL:
		// Fatal error
		return nil, nil
	}

	return nil, nil
}

// OnClose calls the onClose() method on the PHP filter object
func (f *UserFilter) OnClose() {
	if f.obj == nil {
		return
	}
	// Call onClose safely
	defer func() { recover() }()
	f.obj.CallMethod(f.ctx, "onClose")
}

// setStreamProperty sets $this->stream to the stream resource if the property
// is declared and compatible. It recovers from any panics caused by type
// mismatches or missing contexts.
func (f *UserFilter) setStreamProperty() {
	if f.stream == nil || f.obj == nil || f.ctx == nil {
		return
	}
	// Recover from panics - OffsetSet might trigger ThrowError for typed properties
	defer func() { recover() }()
	f.obj.OffsetSet(f.ctx, phpv.ZStr("stream"), f.stream.ZVal())
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
		// Update data/datalen from the bucket in case they were modified
		return b.obj.ZVal()
	}

	obj, err := phpobj.NewZObject(ctx, phpobj.StdClass)
	if err != nil {
		return phpv.ZNULL.ZVal()
	}
	obj.OffsetSet(ctx, phpv.ZStr("data"), phpv.ZString(b.Data).ZVal())
	obj.OffsetSet(ctx, phpv.ZStr("datalen"), phpv.ZInt(len(b.Data)).ZVal())
	b.obj = obj
	return obj.ZVal()
}

// CollectData collects all data from the brigade into a single byte slice
func (bb *BucketBrigade) CollectData() []byte {
	var result []byte
	for _, b := range bb.buckets {
		if b.obj != nil {
			// Read data from the PHP object (it may have been modified)
			if dataVal, err := b.obj.OffsetGet(bb.ctx, phpv.ZStr("data")); err == nil && dataVal != nil {
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

// AppendBucketObj appends a PHP bucket object to the brigade
func (bb *BucketBrigade) AppendBucketObj(obj *phpobj.ZObject) {
	b := &Bucket{obj: obj}
	bb.buckets = append(bb.buckets, b)
}

// PrependBucketObj prepends a PHP bucket object to the brigade
func (bb *BucketBrigade) PrependBucketObj(obj *phpobj.ZObject) {
	b := &Bucket{obj: obj}
	bb.buckets = append([]*Bucket{b}, bb.buckets...)
}
