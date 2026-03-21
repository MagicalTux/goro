package stream

import (
	"bytes"
	"net/url"
	"os"

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
		buf := &readWriteBuffer{Buffer: bytes.NewBuffer(nil)}
		s := NewStream(buf)
		s.SetAttr("stream_type", "TEMP")
		s.SetAttr("mode", "w+b")
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
	*bytes.Buffer
	pos int
}

func (b *readWriteBuffer) Read(p []byte) (int, error) {
	data := b.Buffer.Bytes()
	if b.pos >= len(data) {
		return 0, nil
	}
	n := copy(p, data[b.pos:])
	b.pos += n
	return n, nil
}

func (b *readWriteBuffer) Write(p []byte) (int, error) {
	// Ensure we write at the current position
	data := b.Buffer.Bytes()
	if b.pos < len(data) {
		// Overwrite from pos
		end := b.pos + len(p)
		if end <= len(data) {
			copy(data[b.pos:end], p)
		} else {
			copy(data[b.pos:], p[:len(data)-b.pos])
			b.Buffer.Write(p[len(data)-b.pos:])
		}
	} else {
		// Append
		b.Buffer.Write(p)
	}
	b.pos += len(p)
	return len(p), nil
}
