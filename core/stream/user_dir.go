package stream

import (
	"fmt"

	"github.com/KarpelesLab/goro/core/phpobj"
	"github.com/KarpelesLab/goro/core/phpv"
)

// UserDirHandle is a directory handle for user stream wrappers.
// It delegates readdir/rewinddir/closedir to the PHP wrapper object's methods.
type UserDirHandle struct {
	ctx    phpv.Context
	obj    *phpobj.ZObject
	id     int
	closed bool
}

func (d *UserDirHandle) GetType() phpv.ZType { return phpv.ZtResource }
func (d *UserDirHandle) ZVal() *phpv.ZVal    { return phpv.NewZVal(d) }
func (d *UserDirHandle) Value() phpv.Val     { return d }
func (d *UserDirHandle) String() string      { return fmt.Sprintf("Resource id #%d", d.id) }
func (d *UserDirHandle) GetResourceType() phpv.ResourceType {
	if d.closed {
		return phpv.ResourceUnknown
	}
	return phpv.ResourceStream
}
func (d *UserDirHandle) GetResourceID() int { return d.id }
func (d *UserDirHandle) AsVal(ctx phpv.Context, t phpv.ZType) (phpv.Val, error) {
	switch t {
	case phpv.ZtResource:
		return d, nil
	case phpv.ZtBool:
		return phpv.ZTrue, nil
	case phpv.ZtInt:
		return phpv.ZInt(d.id), nil
	case phpv.ZtString:
		return phpv.ZString(d.String()), nil
	}
	return nil, fmt.Errorf("cannot convert resource to %s", t)
}

// Readdir calls dir_readdir() on the wrapper object.
// Returns false when no more entries.
func (d *UserDirHandle) Readdir() *phpv.ZVal {
	if d.closed || d.obj == nil {
		return phpv.ZFalse.ZVal()
	}
	result, err := d.obj.CallMethod(d.ctx, "dir_readdir")
	if err != nil || result == nil {
		return phpv.ZFalse.ZVal()
	}
	if result.GetType() == phpv.ZtBool && !result.AsBool(d.ctx) {
		return phpv.ZFalse.ZVal()
	}
	return result
}

// Rewinddir calls dir_rewinddir() on the wrapper object.
func (d *UserDirHandle) Rewinddir() {
	if d.closed || d.obj == nil {
		return
	}
	d.obj.CallMethod(d.ctx, "dir_rewinddir")
}

// Close calls dir_closedir() on the wrapper object and marks the handle as closed.
func (d *UserDirHandle) Close() {
	if d.closed || d.obj == nil {
		return
	}
	d.closed = true
	if _, ok := d.obj.Class.GetMethod("dir_closedir"); ok {
		d.obj.CallMethod(d.ctx, "dir_closedir")
	}
	d.obj = nil
}
