package stream

import (
	"errors"
	"io"
	"os"
	"runtime"
)

var ErrNotSupported = errors.New("stream: method or operation not supported")

type Stream struct {
	f    interface{}
	attr map[string]interface{}
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
