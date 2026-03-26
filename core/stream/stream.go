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
	eof  bool

	ResourceType phpv.ResourceType
	ResourceID   int
	Context      *Context

	readFilters  []filterEntry
	writeFilters []filterEntry
	readBuf      []byte // buffered filtered read data
	filterCtx    phpv.Context // context for calling user filters
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
	// If we have read filters, use filtered reading
	if len(s.readFilters) > 0 {
		return s.filteredRead(p)
	}
	if r, ok := s.f.(io.Reader); ok {
		n, err := r.Read(p)
		if err == io.EOF {
			s.eof = true
		}
		return n, err
	}
	return 0, ErrNotSupported
}

// filteredRead reads from the underlying stream, applies read filters, and returns the result
func (s *Stream) filteredRead(p []byte) (int, error) {
	// If we have enough buffered filtered data, return from buffer
	if len(s.readBuf) >= len(p) {
		n := copy(p, s.readBuf)
		s.readBuf = s.readBuf[n:]
		return n, nil
	}

	// If we already have some buffered data and underlying is EOF, drain buffer
	if s.eof && len(s.readBuf) > 0 {
		n := copy(p, s.readBuf)
		s.readBuf = s.readBuf[n:]
		if len(s.readBuf) == 0 {
			return n, io.EOF
		}
		return n, nil
	}
	if s.eof && len(s.readBuf) == 0 {
		return 0, io.EOF
	}

	// Read from underlying stream
	r, ok := s.f.(io.Reader)
	if !ok {
		return 0, ErrNotSupported
	}

	buf := make([]byte, 8192)
	n, err := r.Read(buf)
	atEOF := err == io.EOF

	if n > 0 {
		filtered, ferr := s.ApplyReadFilters(buf[:n], false)
		if ferr != nil {
			return 0, ferr
		}
		s.readBuf = append(s.readBuf, filtered...)
	}

	if atEOF {
		// Send closing signal through filters
		closing, ferr := s.ApplyReadFilters(nil, true)
		if ferr != nil {
			return 0, ferr
		}
		s.readBuf = append(s.readBuf, closing...)
		s.eof = true
	}

	if len(s.readBuf) > 0 {
		nc := copy(p, s.readBuf)
		s.readBuf = s.readBuf[nc:]
		if len(s.readBuf) == 0 && s.eof {
			// All data consumed and at EOF - don't return EOF yet
			// since we did return some data. Next call will return 0, EOF.
		}
		return nc, nil
	}

	if s.eof {
		return 0, io.EOF
	}
	return 0, err
}

func (s *Stream) Write(p []byte) (n int, err error) {
	if len(s.writeFilters) > 0 {
		return s.filteredWrite(p)
	}
	if w, ok := s.f.(io.Writer); ok {
		return w.Write(p)
	}
	return 0, ErrNotSupported
}

// filteredWrite applies write filters before writing to the underlying stream
func (s *Stream) filteredWrite(p []byte) (int, error) {
	filtered, err := s.ApplyWriteFilters(p, false)
	if err != nil {
		return 0, err
	}
	if len(filtered) == 0 {
		// Filters consumed data but produced no output (e.g., buffering)
		return len(p), nil
	}
	w, ok := s.f.(io.Writer)
	if !ok {
		return 0, ErrNotSupported
	}
	_, werr := w.Write(filtered)
	if werr != nil {
		return 0, werr
	}
	return len(p), nil
}

func (s *Stream) Seek(offset int64, whence int) (int64, error) {
	if sk, ok := s.f.(io.Seeker); ok {
		pos, err := sk.Seek(offset, whence)
		if err == nil {
			// Only clear EOF when actually moving the position.
			// Seek(0, SeekCurrent) is just a position query (used by ftell)
			// and should not reset the EOF flag.
			if !(offset == 0 && whence == io.SeekCurrent) {
				s.eof = false
			}
		}
		return pos, err
	}
	return 0, ErrNotSupported
}

func (s *Stream) ReadByte() (byte, error) {
	// When we have read filters, always go through filtered Read
	if len(s.readFilters) > 0 {
		b := make([]byte, 1)
		n, err := s.Read(b)
		if err == io.EOF {
			s.eof = true
		}
		if err == nil && n == 0 {
			return 0, ErrNotSupported
		}
		return b[0], err
	}

	if rb, ok := s.f.(io.ByteReader); ok {
		b, err := rb.ReadByte()
		if err == io.EOF {
			s.eof = true
		}
		return b, err
	}

	b := make([]byte, 1)
	n, err := s.Read(b)

	if err == io.EOF {
		s.eof = true
	}

	if err == nil && n == 0 {
		return 0, ErrNotSupported
	}

	return b[0], err
}

// SetFilterCtx sets the PHP context for calling user filters
func (s *Stream) SetFilterCtx(ctx phpv.Context) {
	s.filterCtx = ctx
}

// GetFilterCtx returns the PHP context for calling user filters
func (s *Stream) GetFilterCtx() phpv.Context {
	return s.filterCtx
}

// Underlying returns the underlying io interface of the stream
func (s *Stream) Underlying() interface{} {
	return s.f
}

// EofChecker is an optional interface for stream backends that need custom EOF logic
// (e.g., user-space stream wrappers that implement stream_eof).
type EofChecker interface {
	EofCheck(ctx phpv.Context) (bool, error)
}

func (s *Stream) Eof() bool {
	return s.eof
}

// EofCheck checks EOF status, calling the underlying stream's EofCheck() method if available.
// This returns an error for user-space stream wrappers that may throw exceptions.
func (s *Stream) EofCheck(ctx phpv.Context) (bool, error) {
	if ec, ok := s.f.(EofChecker); ok {
		return ec.EofCheck(ctx)
	}
	return s.eof, nil
}

func (s *Stream) Close() error {
	// Track user filters we've already called onClose on to avoid duplicates
	// (a filter with STREAM_FILTER_ALL appears in both read and write chains)
	closedFilters := make(map[StreamFilter]bool)

	// Flush write filters before closing
	if len(s.writeFilters) > 0 {
		flushed, _ := s.FlushWriteFilters()
		if len(flushed) > 0 {
			if w, ok := s.f.(io.Writer); ok {
				w.Write(flushed)
			}
		}
		for _, entry := range s.writeFilters {
			if uf, ok := entry.filter.(*UserFilter); ok {
				if !closedFilters[entry.filter] {
					closedFilters[entry.filter] = true
					uf.OnClose()
				}
			}
		}
		s.writeFilters = nil
	}

	// Call onClose for read filters (skip already-closed ones)
	if len(s.readFilters) > 0 {
		for _, entry := range s.readFilters {
			if uf, ok := entry.filter.(*UserFilter); ok {
				if !closedFilters[entry.filter] {
					closedFilters[entry.filter] = true
					uf.OnClose()
				}
			}
		}
		s.readFilters = nil
	}

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
		return phpv.ZBool(true), nil
	case phpv.ZtInt:
		return phpv.ZInt(s.ResourceID), nil
	case phpv.ZtFloat:
		return phpv.ZFloat(s.ResourceID), nil
	case phpv.ZtString:
		return phpv.ZString(s.String()), nil
	case phpv.ZtNull:
		return phpv.ZNull{}, nil
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

func (s *Stream) Truncate(size int64) error {
	if t, ok := s.f.(Truncater); ok {
		return t.Truncate(size)
	}
	return ErrNotSupported
}

func (s *Stream) GetResourceType() phpv.ResourceType { return s.ResourceType }
func (s *Stream) GetResourceID() int                 { return s.ResourceID }
