package core

import (
	"errors"
	"io"
)

var ErrNotSupported = errors.New("stream: method or operation not supported")

type Stream struct {
	f interface{}
}

func NewStream(f interface{}) *Stream {
	return &Stream{f}
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
