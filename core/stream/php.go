package stream

import (
	"bytes"
	"fmt"
	"io"
	"net/url"
	"os"
	"strings"

	"github.com/MagicalTux/goro/core/phpobj"
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

// FilterOpener is implemented by Global to open streams with applied filters.
type FilterOpener interface {
	Open(ctx phpv.Context, fn phpv.ZString, mode phpv.ZString, useIncludePath bool, streamContext ...phpv.Resource) (phpv.Stream, error)
}

// FilterRegistryProvider provides access to the per-request filter registry
type FilterRegistryProvider interface {
	GetFilterRegistry() *FilterRegistry
}

// getRegistry returns the per-request filter registry if available, else the global one
func getRegistry(ctx phpv.Context) *FilterRegistry {
	if g := ctx.Global(); g != nil {
		if p, ok := g.(FilterRegistryProvider); ok {
			return p.GetFilterRegistry()
		}
	}
	return GetFilterRegistry()
}

func (h *phpHandler) Open(ctx phpv.Context, p *url.URL, mode string, _ ...phpv.Resource) (*Stream, error) {
	// Handle php://filter/... before the regular path switch
	// For php://filter/read=.../resource=..., Host is "filter" and Path has the filter spec
	if p.Host == "filter" || strings.HasPrefix(h.getPath(p), "filter") {
		return h.openFilter(ctx, p, mode)
	}

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

// openFilter implements php://filter/read=.../resource=... URL handling
func (h *phpHandler) openFilter(ctx phpv.Context, p *url.URL, mode string) (*Stream, error) {
	// Reconstruct the full path from the URL parts
	// url.Parse of "php://filter/read=foo/resource=/path/to/file" gives:
	// Scheme=php, Host=filter, Path=/read=foo/resource=/path/to/file
	fullPath := ""
	if p.Host == "filter" {
		fullPath = strings.TrimPrefix(p.Path, "/")
	} else {
		// Fallback: host might not be "filter" if URL was parsed differently
		fullPath = strings.TrimPrefix(p.Host+p.Path, "filter/")
	}

	var readFilters, writeFilters []string
	var resourcePath string

	// First, extract the resource= part (everything after "resource=")
	// since the resource path may contain "/"
	if idx := strings.Index(fullPath, "resource="); idx >= 0 {
		resourcePath = fullPath[idx+len("resource="):]
		fullPath = fullPath[:idx]
	}

	// Now parse the remaining parts for filter names
	parts := strings.Split(strings.TrimSuffix(fullPath, "/"), "/")
	for _, part := range parts {
		if strings.HasPrefix(part, "read=") {
			filters := strings.TrimPrefix(part, "read=")
			for _, f := range strings.Split(filters, "|") {
				if f != "" {
					readFilters = append(readFilters, f)
				}
			}
		} else if strings.HasPrefix(part, "write=") {
			filters := strings.TrimPrefix(part, "write=")
			for _, f := range strings.Split(filters, "|") {
				if f != "" {
					writeFilters = append(writeFilters, f)
				}
			}
		} else if part != "" {
			// Bare filter name (applies to both read and write)
			readFilters = append(readFilters, part)
			writeFilters = append(writeFilters, part)
		}
	}

	if resourcePath == "" {
		return nil, os.ErrNotExist
	}

	// Open the underlying resource
	opener, ok := ctx.Global().(FilterOpener)
	if !ok {
		return nil, os.ErrNotExist
	}

	streamVal, err := opener.Open(ctx, phpv.ZString(resourcePath), phpv.ZString(mode), false)
	if err != nil {
		return nil, err
	}

	s, ok := streamVal.(*Stream)
	if !ok {
		return nil, os.ErrNotExist
	}

	s.SetFilterCtx(ctx)

	// Apply read filters
	for _, filterName := range readFilters {
		filter := CreateBuiltinFilter(filterName, nil)
		if filter == nil {
			// Try user filter
			reg := getRegistry(ctx)
			if className, found := reg.Lookup(filterName); found {
				class, err := ctx.Global().GetClass(ctx, phpv.ZString(className), true)
				if err != nil {
					ctx.Warn("Unable to create or locate filter \"%s\"", filterName)
					ctx.Warn("Unable to create filter (%s)", filterName)
					s.Close()
					return nil, err
				}
				obj, err := newFilterObject(ctx, class, filterName)
				if err != nil {
					ctx.Warn("Unable to create or locate filter \"%s\"", filterName)
					ctx.Warn("Unable to create filter (%s)", filterName)
					s.Close()
					return nil, err
				}
				filter = NewUserFilter(ctx, obj, s, filterName, nil)
			} else {
				ctx.Warn("Unable to create or locate filter \"%s\"", filterName)
				ctx.Warn("Unable to create filter (%s)", filterName)
				s.Close()
				return nil, fmt.Errorf("Unable to create filter (%s)", filterName)
			}
		}
		filterRes := &StreamFilterResource{
			ResourceID:   ctx.Global().NextResourceID(),
			ResourceType: phpv.ResourceStreamFilter,
			FilterName:   filterName,
			Direction:    FilterRead,
			Filter:       filter,
			Stream:       s,
		}
		s.AddReadFilter(filterRes, false)
	}

	// Apply write filters
	for _, filterName := range writeFilters {
		filter := CreateBuiltinFilter(filterName, nil)
		if filter == nil {
			// Try user filter (similar to above)
			continue
		}
		filterRes := &StreamFilterResource{
			ResourceID:   ctx.Global().NextResourceID(),
			ResourceType: phpv.ResourceStreamFilter,
			FilterName:   filterName,
			Direction:    FilterWrite,
			Filter:       filter,
			Stream:       s,
		}
		s.AddWriteFilter(filterRes, false)
	}

	return s, nil
}

// newFilterObject creates a new instance of a user filter class
func newFilterObject(ctx phpv.Context, class phpv.ZClass, filterName string) (*phpobj.ZObject, error) {
	obj, err := phpobj.NewZObject(ctx, class)
	if err != nil {
		return nil, err
	}
	obj.ObjectSet(ctx, phpv.ZStr("filtername"), phpv.ZString(filterName).ZVal())
	obj.ObjectSet(ctx, phpv.ZStr("params"), phpv.ZStr("").ZVal())

	// Call onCreate
	result, err := obj.CallMethod(ctx, "onCreate")
	if err != nil {
		return nil, err
	}
	if result != nil && !result.AsBool(ctx) {
		return nil, fmt.Errorf("onCreate returned false")
	}
	return obj, nil
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

func (b *readWriteBuffer) Truncate(size int64) error {
	s := int(size)
	if s < len(b.data) {
		b.data = b.data[:s]
	} else if s > len(b.data) {
		b.data = append(b.data, make([]byte, s-len(b.data))...)
	}
	// For in-memory streams, if position is beyond the new size,
	// move it to the new size (matches PHP's php://memory behavior)
	if b.pos > len(b.data) {
		b.pos = len(b.data)
	}
	return nil
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
