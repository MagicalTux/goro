package standard

import (
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/KarpelesLab/goro/core"
	"github.com/KarpelesLab/goro/core/logopt"
	"github.com/KarpelesLab/goro/core/phpctx"
	"github.com/KarpelesLab/goro/core/phpv"
	"github.com/KarpelesLab/goro/core/stream"
)

// dirHandle represents an open directory resource for opendir/readdir/closedir
type dirHandle struct {
	entries []os.DirEntry
	pos     int
	path    string
	id      int
	closed  bool
}

// getUserDirHandler checks if a path uses a registered user stream wrapper and
// returns the handler if found.
func getUserDirHandler(ctx phpv.Context, path string) *stream.UserStreamHandler {
	idx := strings.Index(path, "://")
	if idx < 1 {
		return nil
	}
	scheme := path[:idx]
	switch scheme {
	case "file", "php", "http", "https", "data", "glob", "phar", "ftp", "ftps",
		"zlib", "compress.zlib", "compress.bzip2":
		return nil
	}
	g := ctx.Global().(*phpctx.Global)
	if h, ok := g.GetStreamHandler(scheme); ok {
		if ush, ok := h.(*stream.UserStreamHandler); ok {
			return ush
		}
	}
	return nil
}

// openUserDir opens a directory on a user stream wrapper.
func openUserDir(ctx phpv.Context, path string, ush *stream.UserStreamHandler, streamCtxRes phpv.Resource) (*stream.UserDirHandle, error) {
	u, err := url.Parse(path)
	if err != nil {
		return nil, err
	}
	return ush.OpenDir(ctx, u, streamCtxRes)
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
	case phpv.ZtFloat:
		return phpv.ZFloat(d.id), nil
	case phpv.ZtString:
		return phpv.ZString(d.String()), nil
	case phpv.ZtNull:
		return phpv.ZNull{}, nil
	case phpv.ZtArray:
		arr := phpv.NewZArray()
		arr.OffsetSet(ctx, nil, d.ZVal())
		return arr, nil
	}
	return nil, fmt.Errorf("cannot convert resource to %s", t)
}

var nextDirHandleID = 1000
var lastDirHandle *dirHandle
var lastUserDirHandle *stream.UserDirHandle

// > func resource opendir ( string $path [, resource $context ] )
func fncOpendir(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var path phpv.ZString
	var contextArg core.Optional[phpv.Resource]
	_, err := core.Expand(ctx, args, &path, &contextArg)
	if err != nil {
		return nil, err
	}

	var streamCtxRes phpv.Resource
	if contextArg.HasArg() {
		streamCtxRes = contextArg.Get()
	}

	pathStr := string(path)

	// Check for user stream wrapper
	if ush := getUserDirHandler(ctx, pathStr); ush != nil {
		udh, err := openUserDir(ctx, pathStr, ush, streamCtxRes)
		if err != nil {
			return phpv.ZFalse.ZVal(), ctx.Warn("opendir(%s): Failed to open dir: operation failed", path, logopt.NoFuncName(true))
		}
		lastUserDirHandle = udh
		lastDirHandle = nil
		return udh.ZVal(), nil
	}

	if err := ctx.Global().CheckOpenBasedir(ctx, pathStr, "opendir"); err != nil {
		ctx.Warn("opendir(%s): Failed to open directory: Operation not permitted", path, logopt.NoFuncName(true))
		return phpv.ZFalse.ZVal(), nil
	}

	p := pathStr
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
	lastDirHandle = dh
	lastUserDirHandle = nil
	return dh.ZVal(), nil
}

// > func string readdir ( [ resource $dir_handle ] )
func fncReaddir(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	if len(args) == 0 || args[0] == nil || args[0].IsNull() {
		// Use last opened handle (either user or regular)
		if lastUserDirHandle != nil {
			return lastUserDirHandle.Readdir(), nil
		}
		dh := lastDirHandle
		if dh == nil || dh.closed {
			return phpv.ZFalse.ZVal(), nil
		}
		return readFromDirHandle(dh), nil
	}

	// Check for user dir handle
	if udh, ok := args[0].Value().(*stream.UserDirHandle); ok {
		return udh.Readdir(), nil
	}

	dh, ok := args[0].Value().(*dirHandle)
	if !ok {
		return phpv.ZFalse.ZVal(), nil
	}
	if dh == nil || dh.closed {
		return phpv.ZFalse.ZVal(), nil
	}
	return readFromDirHandle(dh), nil
}

func readFromDirHandle(dh *dirHandle) *phpv.ZVal {
	if dh.pos == -2 {
		dh.pos = -1
		return phpv.ZStr(".")
	}
	if dh.pos == -1 {
		dh.pos = 0
		return phpv.ZStr("..")
	}
	if dh.pos >= len(dh.entries) {
		return phpv.ZFalse.ZVal()
	}
	name := dh.entries[dh.pos].Name()
	dh.pos++
	return phpv.ZString(name).ZVal()
}

// > func void closedir ( [ resource $dir_handle ] )
func fncClosedir(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	if len(args) == 0 || args[0] == nil || args[0].IsNull() {
		ctx.Deprecated("closedir(): Passing null is deprecated, instead the last opened directory stream should be provided", logopt.NoFuncName(true))
		// Use the last opened directory handle (user or regular)
		if lastUserDirHandle != nil {
			lastUserDirHandle.Close()
			lastUserDirHandle = nil
			return phpv.ZNULL.ZVal(), nil
		}
		if lastDirHandle != nil && !lastDirHandle.closed {
			lastDirHandle.closed = true
			lastDirHandle.entries = nil
		}
		return phpv.ZNULL.ZVal(), nil
	}

	// Check for user dir handle
	if udh, ok := args[0].Value().(*stream.UserDirHandle); ok {
		udh.Close()
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
	if len(args) == 0 || args[0] == nil || args[0].IsNull() {
		if lastUserDirHandle != nil {
			lastUserDirHandle.Rewinddir()
			return phpv.ZNULL.ZVal(), nil
		}
		dh := lastDirHandle
		if dh == nil || dh.closed {
			return phpv.ZNULL.ZVal(), nil
		}
		dh.pos = -2
		return phpv.ZNULL.ZVal(), nil
	}

	// Check for user dir handle
	if udh, ok := args[0].Value().(*stream.UserDirHandle); ok {
		udh.Rewinddir()
		return phpv.ZNULL.ZVal(), nil
	}

	dh, ok := args[0].Value().(*dirHandle)
	if !ok {
		return phpv.ZNULL.ZVal(), nil
	}
	if dh == nil || dh.closed {
		return phpv.ZNULL.ZVal(), nil
	}
	dh.pos = -2
	return phpv.ZNULL.ZVal(), nil
}
