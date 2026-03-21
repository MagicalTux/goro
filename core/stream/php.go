package stream

import (
	"bytes"
	"io"
	"net/url"
	"os"
	"strings"

	"github.com/MagicalTux/goro/core/phpv"
)

var phpH *phpHandler

var Stdin *Stream
var Stdout *Stream
var Stderr *Stream

func init() {
	Stdin = NewStream(os.Stdin)
	Stdout = NewStream(os.Stdout)
	Stderr = NewStream(os.Stderr)

	Stdin.SetAttr("stream_type", "Go")
	Stdin.SetAttr("mode", "r")
	Stdin.ResourceType = phpv.ResourceStream
	Stdin.ResourceID = 1

	Stdout.SetAttr("stream_type", "Go")
	Stdout.SetAttr("mode", "w")
	Stdout.ResourceType = phpv.ResourceStream
	Stdout.ResourceID = 2

	Stderr.SetAttr("stream_type", "Go")
	Stderr.SetAttr("mode", "w")
	Stderr.ResourceType = phpv.ResourceStream
	Stderr.ResourceID = 3

	phpH = &phpHandler{Stdin, Stdout, Stderr}
}

type phpHandler struct {
	stdin, stdout, stderr *Stream
}

func PhpHandler() Handler {
	return phpH
}

func (h *phpHandler) getPath(p *url.URL) string {
	if p.Path != "" {
		return p.Path
	}
	// some urls such as php://stdin has an empty path
	// so return the host instead, which would return
	// the expected "stdin" part
	return p.Host
}

// RequestBodyProvider is implemented by Global to provide the raw request body
type RequestBodyProvider interface {
	GetRequestBody() []byte
}

// StdinProvider is implemented by Global to provide a custom stdin stream
type StdinProvider interface {
	GetStdin() *Stream
}

func (h *phpHandler) Open(ctx phpv.Context, p *url.URL, mode string, _ ...phpv.Resource) (*Stream, error) {
	switch h.getPath(p) {
	case "stdin":
		// Check for context-specific stdin (e.g., test STDIN section)
		if g := ctx.Global(); g != nil {
			if sp, ok := g.(StdinProvider); ok {
				if s := sp.GetStdin(); s != nil {
					return s, nil
				}
			}
		}
		return h.stdin, nil
	case "stdout":
		// Use the context's output writer so that php://stdout goes through
		// the SAPI output (test buffers, web output, etc.) instead of raw os.Stdout.
		if g := ctx.Global(); g != nil {
			s := NewStream(g)
			s.SetAttr("stream_type", "Go")
			s.SetAttr("mode", "w")
			s.ResourceType = phpv.ResourceStream
			return s, nil
		}
		return h.stdout, nil
	case "stderr":
		return h.stderr, nil
	case "input":
		// php://input provides access to the raw request body
		if g := ctx.Global(); g != nil {
			if rbp, ok := g.(RequestBodyProvider); ok {
				body := rbp.GetRequestBody()
				if body == nil {
					body = []byte{}
				}
				s := NewStream(bytes.NewReader(body))
				s.SetAttr("stream_type", "Input")
				s.SetAttr("mode", "rb")
				s.ResourceType = phpv.ResourceStream
				return s, nil
			}
		}
		// No request body available, return empty stream
		s := NewStream(bytes.NewReader([]byte{}))
		s.SetAttr("stream_type", "Input")
		s.SetAttr("mode", "rb")
		s.ResourceType = phpv.ResourceStream
		return s, nil
	case "temp", "memory":
		// php://temp and php://memory both provide read-write temporary streams
		// php://temp may use a temporary file for large data, but we use in-memory for both
		appendMode := strings.Contains(mode, "a")
		buf := &readWriteBuffer{appendMode: appendMode}
		s := NewStream(buf)
		s.SetAttr("stream_type", "TEMP")
		s.SetAttr("mode", mode)
		s.ResourceType = phpv.ResourceStream
		return s, nil
	default:
		return nil, os.ErrNotExist
	}
}

func (h *phpHandler) Exists(p *url.URL) (bool, error) {
	switch h.getPath(p) {
	case "stdin", "stdout", "stderr", "input":
		return true, nil
	case "memory", "temp":
		return true, nil
	}
	return false, nil
}

func (f *phpHandler) Stat(p *url.URL) (os.FileInfo, error) {
	return nil, ErrNotSupported
}

func (f *phpHandler) Lstat(p *url.URL) (os.FileInfo, error) {
	return nil, ErrNotSupported
}

// readWriteBuffer implements a seekable read-write in-memory buffer for php://temp and php://memory
type readWriteBuffer struct {
	data       []byte
	pos        int
	appendMode bool // when true, writes always go to the end (like "a+" mode)
}

func (b *readWriteBuffer) Read(p []byte) (int, error) {
	if b.pos >= len(b.data) {
		return 0, io.EOF
	}
	n := copy(p, b.data[b.pos:])
	b.pos += n
	return n, nil
}

func (b *readWriteBuffer) Write(p []byte) (int, error) {
	if len(p) == 0 {
		return 0, nil
	}
	// In append mode, writes always go to the end
	if b.appendMode {
		b.data = append(b.data, p...)
		b.pos = len(b.data)
		return len(p), nil
	}
	// If pos is past end, zero-fill the gap
	if b.pos > len(b.data) {
		b.data = append(b.data, make([]byte, b.pos-len(b.data))...)
	}
	end := b.pos + len(p)
	if end <= len(b.data) {
		// Overwrite existing data
		copy(b.data[b.pos:end], p)
	} else if b.pos < len(b.data) {
		// Partial overwrite + extend
		copy(b.data[b.pos:], p[:len(b.data)-b.pos])
		b.data = append(b.data, p[len(b.data)-b.pos:]...)
	} else {
		// Pure append
		b.data = append(b.data, p...)
	}
	b.pos = end
	return len(p), nil
}

func (b *readWriteBuffer) Seek(offset int64, whence int) (int64, error) {
	var newPos int64
	switch whence {
	case io.SeekStart:
		newPos = offset
	case io.SeekCurrent:
		newPos = int64(b.pos) + offset
	case io.SeekEnd:
		newPos = int64(len(b.data)) + offset
	default:
		return 0, ErrNotSupported
	}
	if newPos < 0 {
		return int64(b.pos), ErrNotSupported
	}
	b.pos = int(newPos)
	return newPos, nil
}
