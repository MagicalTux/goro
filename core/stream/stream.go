package stream

import (
	"errors"
	"fmt"
	"io"
	"os"
	"runtime"

	"github.com/MagicalTux/goro/core/phpobj"
	"github.com/MagicalTux/goro/core/phpv"
)

var ErrNotSupported = errors.New("stream: method or operation not supported")

type Stream struct {
	f    interface{}
	attr map[string]interface{}

	ResourceType phpv.ResourceType
	ResourceID   int
}

func streamFinalizer(s *Stream) {
	s.Close()
}

func NewStream(f interface{}) *Stream {
	res := &Stream{f: f}
	runtime.SetFinalizer(res, streamFinalizer)
	return res
}

func (s *Stream) Read(p []byte) (int, error) {
	if r, ok := s.f.(io.Reader); ok {
		return r.Read(p)
	}
	return 0, ErrNotSupported
}

func (s *Stream) Write(p []byte) (n int, err error) {
	if w, ok := s.f.(io.Writer); ok {
		return w.Write(p)
	}
	return 0, ErrNotSupported
}

func (s *Stream) Seek(offset int64, whence int) (int64, error) {
	if sk, ok := s.f.(io.Seeker); ok {
		return sk.Seek(offset, whence)
	}
	return 0, ErrNotSupported
}

func (s *Stream) ReadByte() (byte, error) {
	if rb, ok := s.f.(io.ByteReader); ok {
		return rb.ReadByte()
	}

	b := make([]byte, 1)
	n, err := s.Read(b)

	if err == nil && n == 0 {
		return 0, ErrNotSupported
	}

	return b[0], err
}

func (s *Stream) Close() error {
	if cl, ok := s.f.(io.Closer); ok {
		return cl.Close()
	}
	// do not fail if no close
	return nil
}

func (s *Stream) SetAttr(k string, v interface{}) {
	if s.attr == nil {
		s.attr = make(map[string]interface{})
	}
	s.attr[k] = v
}

type AttrStream interface {
	Attr(v interface{}) interface{}
}

func (s *Stream) Attr(v interface{}) interface{} {
	if str, ok := v.(string); ok && s.attr != nil {
		if v, ok2 := s.attr[str]; ok2 {
			return v
		}
	}
	if a, ok := s.f.(AttrStream); ok {
		return a.Attr(v)
	}
	// return nil
	return nil
}

func (s *Stream) Stat() (os.FileInfo, error) {
	if f, ok := s.f.(Stater); ok {
		return f.Stat()
	}

	return nil, ErrNotSupported
}

func (s *Stream) Flush() error {
	if f, ok := s.f.(Flusher); ok {
		return f.Flush()
	}

	return nil // if Flush is not supported, it usually means there is nothing to flush, so no error
}

func (s *Stream) Sync() error {
	if f, ok := s.f.(Syncer); ok {
		return f.Sync()
	}

	return nil // if no Sync, no need to sync, probably
}

func (s *Stream) GetType() phpv.ZType {
	return phpv.ZtResource
}
func (s *Stream) ZVal() *phpv.ZVal {
	return phpv.NewZVal(s)
}
func (s *Stream) Value() phpv.Val {
	return s
}
func (s *Stream) AsVal(ctx phpv.Context, t phpv.ZType) (phpv.Val, error) {
	switch t {
	case phpv.ZtResource:
		return s, nil
	case phpv.ZtBool:
		return phpv.ZTrue.ZVal(), nil
	case phpv.ZtInt:
		return phpv.ZInt(s.ResourceID).ZVal(), nil
	case phpv.ZtString:
		return phpv.ZStr(s.String()), nil
	case phpv.ZtArray:
		arr := phpv.NewZArray()
		arr.OffsetSet(ctx, nil, s.ZVal())
		return arr, nil
	case phpv.ZtObject:
		obj, err := phpobj.NewZObject(ctx, phpobj.StdClass)
		if err != nil {
			return nil, err
		}
		obj.OffsetSet(ctx, phpv.ZStr("scalar"), s.ZVal())
		return obj, nil
	}
	return nil, ctx.Errorf("cannot convert stream to %s", t.String())
}
func (s *Stream) String() string {
	return fmt.Sprintf("Resource id #%d", s.ResourceID)
}

func (s *Stream) GetResourceType() phpv.ResourceType { return s.ResourceType }
func (s *Stream) GetResourceID() int                 { return s.ResourceID }
