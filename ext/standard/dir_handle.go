package standard

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/MagicalTux/goro/core"
	"github.com/MagicalTux/goro/core/logopt"
	"github.com/MagicalTux/goro/core/phpv"
)

// dirHandle represents an open directory resource for opendir/readdir/closedir
type dirHandle struct {
	entries []os.DirEntry
	pos     int
	path    string
	id      int
	closed  bool
}

func (d *dirHandle) GetType() phpv.ZType { return phpv.ZtResource }
func (d *dirHandle) ZVal() *phpv.ZVal    { return phpv.NewZVal(d) }
func (d *dirHandle) Value() phpv.Val     { return d }
func (d *dirHandle) String() string      { return fmt.Sprintf("Resource id #%d", d.id) }
func (d *dirHandle) GetResourceType() phpv.ResourceType {
	if d.closed {
		return phpv.ResourceUnknown
	}
	return phpv.ResourceStream
}
func (d *dirHandle) GetResourceID() int { return d.id }
func (d *dirHandle) AsVal(ctx phpv.Context, t phpv.ZType) (phpv.Val, error) {
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

var nextDirHandleID = 1000

// > func resource opendir ( string $path [, resource $context ] )
func fncOpendir(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var path phpv.ZString
	_, err := core.Expand(ctx, args, &path)
	if err != nil {
		return nil, err
	}

	if err := ctx.Global().CheckOpenBasedir(ctx, string(path), "opendir"); err != nil {
		ctx.Warn("opendir(%s): Failed to open directory: Operation not permitted", path, logopt.NoFuncName(true))
		return phpv.ZFalse.ZVal(), nil
	}

	p := string(path)
	if !filepath.IsAbs(p) {
		p = filepath.Join(string(ctx.Global().Getwd()), p)
	}

	entries, err := os.ReadDir(p)
	if err != nil {
		return phpv.ZFalse.ZVal(), ctx.Warn("opendir(%s): Failed to open directory: %s", path, err, logopt.NoFuncName(true))
	}

	dh := &dirHandle{
		entries: entries,
		pos:     -2, // -2 = ".", -1 = "..", 0+ = entries
		path:    p,
		id:      nextDirHandleID,
	}
	nextDirHandleID++
	return dh.ZVal(), nil
}

// > func string readdir ( [ resource $dir_handle ] )
func fncReaddir(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	if len(args) == 0 {
		return phpv.ZFalse.ZVal(), nil
	}

	dh, ok := args[0].Value().(*dirHandle)
	if !ok || dh.closed {
		return phpv.ZFalse.ZVal(), nil
	}

	if dh.pos == -2 {
		dh.pos = -1
		return phpv.ZStr("."), nil
	}
	if dh.pos == -1 {
		dh.pos = 0
		return phpv.ZStr(".."), nil
	}
	if dh.pos >= len(dh.entries) {
		return phpv.ZFalse.ZVal(), nil
	}

	name := dh.entries[dh.pos].Name()
	dh.pos++
	return phpv.ZString(name).ZVal(), nil
}

// > func void closedir ( [ resource $dir_handle ] )
func fncClosedir(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	if len(args) == 0 || args[0] == nil || args[0].IsNull() {
		ctx.Deprecated("closedir(): Passing null is deprecated, instead the last opened directory stream should be provided", logopt.NoFuncName(true))
		return phpv.ZNULL.ZVal(), nil
	}

	dh, ok := args[0].Value().(*dirHandle)
	if !ok {
		// Not a directory handle - could be a file stream or other resource
		ctx.Warn("closedir(): Argument #1 ($dir_handle) must be a valid Directory resource", logopt.NoFuncName(true))
		return phpv.ZNULL.ZVal(), nil
	}

	if dh.closed {
		ctx.Warn("closedir(): Argument #1 ($dir_handle) must be an open stream resource", logopt.NoFuncName(true))
		return phpv.ZNULL.ZVal(), nil
	}

	dh.closed = true
	dh.entries = nil
	return phpv.ZNULL.ZVal(), nil
}

// > func void rewinddir ( [ resource $dir_handle ] )
func fncRewinddir(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	if len(args) == 0 {
		return phpv.ZNULL.ZVal(), nil
	}

	dh, ok := args[0].Value().(*dirHandle)
	if !ok {
		return phpv.ZNULL.ZVal(), nil
	}

	dh.pos = -2
	return phpv.ZNULL.ZVal(), nil
}
