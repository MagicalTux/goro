package stream

import (
	"io"
	"net/url"
	"os"

	"github.com/KarpelesLab/goro/core/phpobj"
	"github.com/KarpelesLab/goro/core/phpv"
)

// UserStreamHandler implements stream.Handler for PHP user-space stream wrappers
// registered via stream_wrapper_register().
type UserStreamHandler struct {
	ClassName phpv.ZString
}

func NewUserStreamHandler(className phpv.ZString) *UserStreamHandler {
	return &UserStreamHandler{ClassName: className}
}

// setContextOnObject assigns the stream context resource to $this->context on the wrapper object.
func setContextOnObject(ctx phpv.Context, obj *phpobj.ZObject, streamCtx phpv.Resource) {
	if streamCtx == nil {
		return
	}
	// Only set if the class has a "context" property (typed or otherwise)
	if obj.Class == nil {
		return
	}
	contextVal := phpv.NewZVal(streamCtx)
	// Use ObjectSet which handles type checking; silently ignore errors
	obj.ObjectSet(ctx, phpv.ZStr("context"), contextVal)
}

func (h *UserStreamHandler) Open(ctx phpv.Context, path *url.URL, mode string, streamCtx ...phpv.Resource) (*Stream, error) {
	// Look up the class
	class, err := ctx.Global().GetClass(ctx, h.ClassName, true)
	if err != nil {
		return nil, err
	}

	// Create an instance of the class
	obj, err := phpobj.NewZObject(ctx, class)
	if err != nil {
		return nil, err
	}

	// Set $this->context if a stream context was provided
	var ctxRes phpv.Resource
	if len(streamCtx) > 0 && streamCtx[0] != nil {
		ctxRes = streamCtx[0]
	}
	if ctxRes != nil {
		setContextOnObject(ctx, obj, ctxRes)
	}

	// Call stream_open($path, $mode, $options, &$opened_path)
	fullPath := path.String()

	openedPath := phpv.ZString("").ZVal()
	openedPath.MakeRef()
	result, err := obj.CallMethod(ctx, "stream_open",
		phpv.ZString(fullPath).ZVal(),
		phpv.ZString(mode).ZVal(),
		phpv.ZInt(0).ZVal(),
		openedPath,
	)
	if err != nil {
		return nil, err
	}

	if result == nil || !result.AsBool(ctx) {
		return nil, os.ErrNotExist
	}

	globalCtx, ok := ctx.Global().(phpv.Context)
	if !ok {
		globalCtx = ctx
	}
	us := &UserStream{
		ctx: globalCtx,
		obj: obj,
	}
	s := NewStream(us)
	s.SetAttr("wrapper_type", "user-space")
	s.SetAttr("mode", mode)
	s.SetAttr("seekable", true)
	s.SetAttr("uri", fullPath)
	s.ResourceType = phpv.ResourceStream
	s.ResourceID = ctx.Global().NextResourceID()
	if ctxRes != nil {
		if sc, ok := ctxRes.(*Context); ok {
			s.Context = sc
		}
	}
	return s, nil
}

func (h *UserStreamHandler) Exists(path *url.URL) (bool, error) {
	return false, ErrNotSupported
}

func (h *UserStreamHandler) Stat(path *url.URL) (os.FileInfo, error) {
	return nil, ErrNotSupported
}

func (h *UserStreamHandler) Lstat(path *url.URL) (os.FileInfo, error) {
	return nil, ErrNotSupported
}

// OpenDir opens a directory using the user stream wrapper's dir_opendir method.
// Returns a *UserDirHandle that implements the directory handle protocol.
func (h *UserStreamHandler) OpenDir(ctx phpv.Context, path *url.URL, streamCtx phpv.Resource) (*UserDirHandle, error) {
	class, err := ctx.Global().GetClass(ctx, h.ClassName, true)
	if err != nil {
		return nil, err
	}

	obj, err := phpobj.NewZObject(ctx, class)
	if err != nil {
		return nil, err
	}

	if streamCtx != nil {
		setContextOnObject(ctx, obj, streamCtx)
	}

	fullPath := path.String()
	result, err := obj.CallMethod(ctx, "dir_opendir",
		phpv.ZString(fullPath).ZVal(),
		phpv.ZInt(0).ZVal(),
	)
	if err != nil {
		return nil, err
	}
	if result == nil || !result.AsBool(ctx) {
		return nil, os.ErrNotExist
	}

	globalCtx, ok := ctx.Global().(phpv.Context)
	if !ok {
		globalCtx = ctx
	}
	return &UserDirHandle{
		ctx: globalCtx,
		obj: obj,
		id:  ctx.Global().NextResourceID(),
	}, nil
}

// UserStream wraps a PHP object that implements the stream wrapper protocol,
// delegating io operations to PHP method calls.
type UserStream struct {
	ctx  phpv.Context
	obj  *phpobj.ZObject
	rbuf []byte // buffer for data returned by stream_read but not yet consumed
}

func (u *UserStream) Read(p []byte) (int, error) {
	// If we have buffered data, serve from buffer first
	if len(u.rbuf) > 0 {
		n := copy(p, u.rbuf)
		u.rbuf = u.rbuf[n:]
		return n, nil
	}

	result, err := u.obj.CallMethod(u.ctx, "stream_read",
		phpv.ZInt(len(p)).ZVal(),
	)
	if err != nil {
		return 0, err
	}
	if result == nil || (result.GetType() == phpv.ZtBool && !result.AsBool(u.ctx)) {
		return 0, io.EOF
	}
	data := string(result.AsString(u.ctx))
	if len(data) == 0 {
		return 0, io.EOF
	}
	n := copy(p, data)
	// If stream_read returned more than we can consume, buffer the rest
	if n < len(data) {
		u.rbuf = append(u.rbuf[:0], data[n:]...)
	}
	return n, nil
}

func (u *UserStream) Write(p []byte) (int, error) {
	result, err := u.obj.CallMethod(u.ctx, "stream_write",
		phpv.ZString(p).ZVal(),
	)
	if err != nil {
		return 0, err
	}
	if result == nil {
		return len(p), nil
	}
	return int(result.AsInt(u.ctx)), nil
}

// EofCheck calls the PHP stream wrapper's stream_eof method to check if the stream
// has reached end-of-file. Returns true if at EOF or if stream_eof is not defined.
func (u *UserStream) EofCheck(ctx phpv.Context) (bool, error) {
	if u.obj == nil {
		return true, nil
	}
	if _, ok := u.obj.Class.GetMethod("stream_eof"); !ok {
		return true, nil
	}
	result, err := u.obj.CallMethod(ctx, "stream_eof")
	if err != nil {
		return false, err
	}
	if result == nil {
		return false, nil
	}
	return bool(result.AsBool(ctx)), nil
}

func (u *UserStream) Close() (retErr error) {
	if u.obj == nil {
		return nil
	}
	// Recover from panics when called from Go finalizer after PHP context is gone
	defer func() {
		recover()
		u.obj = nil
	}()
	// Only call stream_close if the method exists on the class
	if _, ok := u.obj.Class.GetMethod("stream_close"); ok {
		u.obj.CallMethod(u.ctx, "stream_close")
	}
	u.obj = nil
	return nil
}
