package zlib

import (
	"bytes"
	"compress/flate"
	"compress/gzip"
	"compress/zlib"
	"io"
	"net/url"
	"os"
	"strings"

	"github.com/MagicalTux/goro/core"
	"github.com/MagicalTux/goro/core/phpctx"
	"github.com/MagicalTux/goro/core/phpobj"
	"github.com/MagicalTux/goro/core/phpv"
	"github.com/MagicalTux/goro/core/stream"
)

// --- gzFile resource ---

// gzFile wraps a gzip reader or writer with a file handle.
type gzFile struct {
	file   *os.File
	reader *gzip.Reader
	writer *gzip.Writer
	mode   string
	pos    int64
}

func (g *gzFile) Read(p []byte) (int, error) {
	if g.reader == nil {
		return 0, stream.ErrNotSupported
	}
	n, err := g.reader.Read(p)
	g.pos += int64(n)
	return n, err
}

func (g *gzFile) Write(p []byte) (int, error) {
	if g.writer == nil {
		return 0, stream.ErrNotSupported
	}
	n, err := g.writer.Write(p)
	g.pos += int64(n)
	return n, err
}

// Seek implements limited seeking for gzip files.
// Only SeekCurrent with offset 0 is supported (for gztell).
// Full seeking is not supported by gzip format.
func (g *gzFile) Seek(offset int64, whence int) (int64, error) {
	if whence == io.SeekCurrent && offset == 0 {
		return g.pos, nil
	}
	return g.pos, stream.ErrNotSupported
}

func (g *gzFile) Close() error {
	var err error
	if g.writer != nil {
		err = g.writer.Close()
		g.writer = nil
	}
	if g.reader != nil {
		g.reader.Close()
		g.reader = nil
	}
	if g.file != nil {
		if ferr := g.file.Close(); ferr != nil && err == nil {
			err = ferr
		}
		g.file = nil
	}
	return err
}

// openGzFile opens a gz file for reading or writing based on mode.
func openGzFile(filename, mode string) (*stream.Stream, error) {
	gz := &gzFile{mode: mode}
	reading := strings.ContainsAny(mode, "r")
	writing := strings.ContainsAny(mode, "wa")

	var flag int
	if reading {
		flag = os.O_RDONLY
	} else if strings.Contains(mode, "a") {
		flag = os.O_WRONLY | os.O_CREATE | os.O_APPEND
	} else {
		flag = os.O_WRONLY | os.O_CREATE | os.O_TRUNC
	}

	f, err := os.OpenFile(filename, flag, 0666)
	if err != nil {
		return nil, err
	}
	gz.file = f

	if reading {
		gz.reader, err = gzip.NewReader(f)
		if err != nil {
			f.Close()
			return nil, err
		}
	} else if writing {
		// Detect compression level from mode string (e.g., "wb9")
		lvl := gzip.DefaultCompression
		for _, c := range mode {
			if c >= '1' && c <= '9' {
				lvl = int(c - '0')
				break
			}
		}
		gz.writer, err = gzip.NewWriterLevel(f, lvl)
		if err != nil {
			f.Close()
			return nil, err
		}
	} else {
		f.Close()
		return nil, os.ErrInvalid
	}

	s := stream.NewStream(gz)
	s.SetAttr("stream_type", "ZLIB")
	s.SetAttr("mode", mode)
	return s, nil
}

// gzopen(string $filename, string $mode, int $use_include_path = 0): resource|false
func fncGzopen(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var filename phpv.ZString
	var mode phpv.ZString
	var useIncludePath *phpv.ZInt
	_, err := core.Expand(ctx, args, &filename, &mode, &useIncludePath)
	if err != nil {
		return nil, err
	}

	if string(filename) == "" {
		return phpv.ZFalse.ZVal(), nil
	}

	s, err := openGzFile(string(filename), string(mode))
	if err != nil {
		ctx.Warn("gzopen(%s): failed to open stream: %s", filename, err.Error())
		return phpv.ZFalse.ZVal(), nil
	}

	s.ResourceType = phpv.ResourceStream
	s.ResourceID = ctx.Global().NextResourceID()

	return s.ZVal(), nil
}

// getGzStream extracts a *stream.Stream from a resource arg
func getGzStream(handle phpv.Resource, fname string) (*stream.Stream, error) {
	if handle == nil {
		return nil, phpobj.ThrowError(nil, phpobj.TypeError, fname+": expects a valid resource")
	}
	if handle.GetResourceType() == phpv.ResourceUnknown {
		return nil, phpobj.ThrowError(nil, phpobj.TypeError, fname+": expects an open stream resource")
	}
	s, ok := handle.(*stream.Stream)
	if !ok {
		return nil, phpobj.ThrowError(nil, phpobj.TypeError, fname+": expects a stream resource")
	}
	return s, nil
}

// gzclose(resource $stream): bool
func fncGzclose(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var handle phpv.Resource
	_, err := core.Expand(ctx, args, &handle)
	if err != nil {
		return nil, err
	}

	s, serr := getGzStream(handle, "gzclose")
	if serr != nil {
		ctx.Warn("%s", serr.Error())
		return phpv.ZFalse.ZVal(), nil
	}

	if err := s.Close(); err != nil {
		return phpv.ZFalse.ZVal(), nil
	}
	s.ResourceType = phpv.ResourceUnknown
	return phpv.ZTrue.ZVal(), nil
}

// gzread(resource $stream, int $length): string|false
func fncGzread(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var handle phpv.Resource
	var length phpv.ZInt
	_, err := core.Expand(ctx, args, &handle, &length)
	if err != nil {
		return nil, err
	}

	s, serr := getGzStream(handle, "gzread")
	if serr != nil {
		return phpv.ZFalse.ZVal(), nil
	}

	if length <= 0 {
		return nil, phpobj.ThrowError(ctx, phpobj.ValueError, "gzread(): Argument #2 ($length) must be greater than 0")
	}

	buf := make([]byte, int(length))
	n, readErr := s.Read(buf)
	if readErr != nil && n == 0 {
		if readErr == io.EOF {
			return phpv.ZString("").ZVal(), nil
		}
		return phpv.ZFalse.ZVal(), nil
	}
	return phpv.ZString(buf[:n]).ZVal(), nil
}

// gzwrite(resource $stream, string $data, ?int $length = null): int|false
// alias gzputs
func fncGzwrite(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var handle phpv.Resource
	var data phpv.ZString
	var length *phpv.ZInt
	_, err := core.Expand(ctx, args, &handle, &data, &length)
	if err != nil {
		return nil, err
	}

	s, serr := getGzStream(handle, "gzwrite")
	if serr != nil {
		return phpv.ZFalse.ZVal(), nil
	}

	b := []byte(data)
	if length != nil && *length >= 0 && int(*length) < len(b) {
		b = b[:int(*length)]
	}

	n, err := s.Write(b)
	if err != nil {
		return phpv.ZFalse.ZVal(), nil
	}
	return phpv.ZInt(n).ZVal(), nil
}

// gzgets(resource $stream, ?int $length = null): string|false
func fncGzgets(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var handle phpv.Resource
	var length *phpv.ZInt
	_, err := core.Expand(ctx, args, &handle, &length)
	if err != nil {
		return nil, err
	}

	s, serr := getGzStream(handle, "gzgets")
	if serr != nil {
		return phpv.ZFalse.ZVal(), nil
	}

	maxLen := 1024
	if length != nil && *length > 0 {
		maxLen = int(*length)
	}

	var buf bytes.Buffer
	tmp := make([]byte, 1)
	for buf.Len() < maxLen {
		n, readErr := s.Read(tmp)
		if n > 0 {
			buf.Write(tmp[:n])
			if tmp[0] == '\n' {
				break
			}
		}
		if readErr == io.EOF {
			break
		}
		if readErr != nil {
			if buf.Len() == 0 {
				return phpv.ZFalse.ZVal(), nil
			}
			break
		}
	}
	if buf.Len() == 0 {
		return phpv.ZFalse.ZVal(), nil
	}
	return phpv.ZString(buf.Bytes()).ZVal(), nil
}

// gzeof(resource $stream): bool
func fncGzeof(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var handle phpv.Resource
	_, err := core.Expand(ctx, args, &handle)
	if err != nil {
		return nil, err
	}

	s, serr := getGzStream(handle, "gzeof")
	if serr != nil {
		return phpv.ZTrue.ZVal(), nil
	}

	eof, _ := s.EofCheck(ctx)
	return phpv.ZBool(eof).ZVal(), nil
}

// gzseek(resource $stream, int $offset, int $whence = SEEK_SET): int
func fncGzseek(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var handle phpv.Resource
	var offset phpv.ZInt
	var whence *phpv.ZInt
	_, err := core.Expand(ctx, args, &handle, &offset, &whence)
	if err != nil {
		return nil, err
	}

	s, serr := getGzStream(handle, "gzseek")
	if serr != nil {
		return phpv.ZInt(-1).ZVal(), nil
	}

	w := io.SeekStart
	if whence != nil {
		w = int(*whence)
	}

	_, err = s.Seek(int64(offset), w)
	if err != nil {
		return phpv.ZInt(-1).ZVal(), nil
	}
	return phpv.ZInt(0).ZVal(), nil
}

// gztell(resource $stream): int|false
func fncGztell(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var handle phpv.Resource
	_, err := core.Expand(ctx, args, &handle)
	if err != nil {
		return nil, err
	}

	s, serr := getGzStream(handle, "gztell")
	if serr != nil {
		return phpv.ZFalse.ZVal(), nil
	}

	pos, err := s.Seek(0, io.SeekCurrent)
	if err != nil {
		return phpv.ZFalse.ZVal(), nil
	}
	return phpv.ZInt(pos).ZVal(), nil
}

// gzrewind(resource $stream): bool
func fncGzrewind(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var handle phpv.Resource
	_, err := core.Expand(ctx, args, &handle)
	if err != nil {
		return nil, err
	}

	s, serr := getGzStream(handle, "gzrewind")
	if serr != nil {
		return phpv.ZFalse.ZVal(), nil
	}

	_, err = s.Seek(0, io.SeekStart)
	if err != nil {
		return phpv.ZFalse.ZVal(), nil
	}
	return phpv.ZTrue.ZVal(), nil
}

// gzpassthru(resource $stream): int|false
// Output all remaining data on a gz-file pointer
func fncGzpassthru(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var handle phpv.Resource
	_, err := core.Expand(ctx, args, &handle)
	if err != nil {
		return nil, err
	}

	s, serr := getGzStream(handle, "gzpassthru")
	if serr != nil {
		return phpv.ZFalse.ZVal(), nil
	}

	data, err := io.ReadAll(s)
	if err != nil && len(data) == 0 {
		return phpv.ZFalse.ZVal(), nil
	}

	ctx.Write(data)
	return phpv.ZInt(len(data)).ZVal(), nil
}

// gzgetc(resource $stream): string|false
func fncGzgetc(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var handle phpv.Resource
	_, err := core.Expand(ctx, args, &handle)
	if err != nil {
		return nil, err
	}

	s, serr := getGzStream(handle, "gzgetc")
	if serr != nil {
		return phpv.ZFalse.ZVal(), nil
	}

	b := make([]byte, 1)
	n, readErr := s.Read(b)
	if n == 0 || (readErr != nil && readErr != io.EOF) {
		return phpv.ZFalse.ZVal(), nil
	}
	return phpv.ZString(b[:n]).ZVal(), nil
}

// readgzfile(string $filename, int $use_include_path = 0): int|false
// Output a gz-file and return the number of (uncompressed) bytes read
func fncReadgzfile(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var filename phpv.ZString
	var useIncludePath *phpv.ZInt
	_, err := core.Expand(ctx, args, &filename, &useIncludePath)
	if err != nil {
		return nil, err
	}

	s, err := openGzFile(string(filename), "rb")
	if err != nil {
		ctx.Warn("readgzfile(%s): failed to open stream: %s", filename, err.Error())
		return phpv.ZFalse.ZVal(), nil
	}
	defer s.Close()

	data, err := io.ReadAll(s)
	if err != nil && len(data) == 0 {
		return phpv.ZFalse.ZVal(), nil
	}

	ctx.Write(data)
	return phpv.ZInt(len(data)).ZVal(), nil
}

// --- compress.zlib:// stream wrapper ---

// ZlibStreamHandler implements stream.Handler for compress.zlib:// URLs
type ZlibStreamHandler struct{}

func (h *ZlibStreamHandler) Open(ctx phpv.Context, path *url.URL, mode string, streamCtx ...phpv.Resource) (*stream.Stream, error) {
	filename := path.Host + path.Path
	if filename == "" {
		filename = path.Opaque
	}

	reading := strings.ContainsAny(mode, "r")
	writing := strings.ContainsAny(mode, "wa")

	if reading {
		f, err := os.Open(filename)
		if err != nil {
			return nil, err
		}

		// Determine format by trying gzip first
		buf := make([]byte, 2)
		n, err := f.Read(buf)
		f.Seek(0, io.SeekStart)

		if n >= 2 && buf[0] == 0x1f && buf[1] == 0x8b {
			// gzip format
			gr, err := gzip.NewReader(f)
			if err != nil {
				f.Close()
				return nil, err
			}
			wrapper := &zlibReadWrapper{reader: gr, closer: f}
			s := stream.NewStream(wrapper)
			s.SetAttr("stream_type", "ZLIB")
			s.SetAttr("mode", mode)
			return s, nil
		}

		// Try zlib format
		zr, err2 := zlib.NewReader(f)
		if err2 == nil {
			wrapper := &zlibReadWrapper{reader: zr, closer: f}
			s := stream.NewStream(wrapper)
			s.SetAttr("stream_type", "ZLIB")
			s.SetAttr("mode", mode)
			return s, nil
		}

		// Fall back to raw deflate
		f.Seek(0, io.SeekStart)
		fr := flate.NewReader(f)
		wrapper := &zlibReadWrapper{reader: fr, closer: f}
		s := stream.NewStream(wrapper)
		s.SetAttr("stream_type", "ZLIB")
		s.SetAttr("mode", mode)
		return s, nil
	} else if writing {
		var flag int
		if strings.Contains(mode, "a") {
			flag = os.O_WRONLY | os.O_CREATE | os.O_APPEND
		} else {
			flag = os.O_WRONLY | os.O_CREATE | os.O_TRUNC
		}
		f, err := os.OpenFile(filename, flag, 0666)
		if err != nil {
			return nil, err
		}
		gw := gzip.NewWriter(f)
		wrapper := &zlibWriteWrapper{writer: gw, closer: f}
		s := stream.NewStream(wrapper)
		s.SetAttr("stream_type", "ZLIB")
		s.SetAttr("mode", mode)
		return s, nil
	}

	return nil, os.ErrInvalid
}

func (h *ZlibStreamHandler) Exists(path *url.URL) (bool, error) {
	filename := path.Host + path.Path
	if filename == "" {
		filename = path.Opaque
	}
	_, err := os.Stat(filename)
	return err == nil, nil
}

func (h *ZlibStreamHandler) Stat(path *url.URL) (os.FileInfo, error) {
	filename := path.Host + path.Path
	if filename == "" {
		filename = path.Opaque
	}
	return os.Stat(filename)
}

func (h *ZlibStreamHandler) Lstat(path *url.URL) (os.FileInfo, error) {
	filename := path.Host + path.Path
	if filename == "" {
		filename = path.Opaque
	}
	return os.Lstat(filename)
}

// zlibReadWrapper wraps a reader and an io.Closer (e.g. the underlying file)
type zlibReadWrapper struct {
	reader io.Reader
	closer io.Closer
}

func (w *zlibReadWrapper) Read(p []byte) (int, error) {
	return w.reader.Read(p)
}

func (w *zlibReadWrapper) Close() error {
	if c, ok := w.reader.(io.Closer); ok {
		c.Close()
	}
	return w.closer.Close()
}

// zlibWriteWrapper wraps a gzip.Writer and an underlying io.Closer
type zlibWriteWrapper struct {
	writer *gzip.Writer
	closer io.Closer
}

func (w *zlibWriteWrapper) Write(p []byte) (int, error) {
	return w.writer.Write(p)
}

func (w *zlibWriteWrapper) Close() error {
	err := w.writer.Close()
	if err2 := w.closer.Close(); err == nil {
		err = err2
	}
	return err
}

// --- Stream Filters ---

// ZlibDeflateFilter implements the zlib.deflate stream filter
type ZlibDeflateFilter struct {
	level int
	buf   bytes.Buffer
	done  bool
}

func NewZlibDeflateFilter(level int) *ZlibDeflateFilter {
	if level < -1 || level > 9 {
		level = flate.DefaultCompression
	}
	return &ZlibDeflateFilter{level: level}
}

func (f *ZlibDeflateFilter) Process(data []byte, closing bool) ([]byte, error) {
	f.buf.Write(data)

	if !closing {
		return nil, nil
	}

	// On close, compress all buffered data
	var out bytes.Buffer
	w, err := flate.NewWriter(&out, f.level)
	if err != nil {
		return nil, err
	}
	f.buf.WriteTo(w)
	w.Close()
	return out.Bytes(), nil
}

// ZlibInflateFilter implements the zlib.inflate stream filter
type ZlibInflateFilter struct {
	buf bytes.Buffer
}

func (f *ZlibInflateFilter) Process(data []byte, closing bool) ([]byte, error) {
	f.buf.Write(data)

	if !closing {
		return nil, nil
	}

	// On close, decompress all buffered data
	r := flate.NewReader(&f.buf)
	defer r.Close()
	out, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}
	return out, nil
}

// registerZlibFilters adds zlib.deflate and zlib.inflate to the builtin filter registry.
// This is called by modifying stream.CreateBuiltinFilter and stream.IsBuiltinFilter.
// Since we can't modify those files directly, we use the global filter factory registry.
//
// We register a wrapper in the phpctx Global via the Ext.OnGlobalInit mechanism.
// Since no such mechanism exists, we hook into the filter creation at stream filter
// creation time by storing factories in a package-level map.

var zlibFilterFactories = map[string]func() stream.StreamFilter{
	"zlib.deflate": func() stream.StreamFilter { return NewZlibDeflateFilter(flate.DefaultCompression) },
	"zlib.inflate": func() stream.StreamFilter { return &ZlibInflateFilter{} },
}

// RegisterZlibStreamHandler registers the compress.zlib:// handler with a given global context.
func RegisterZlibStreamHandler(g interface{ RegisterStreamHandler(string, stream.Handler) }) {
	g.RegisterStreamHandler("compress.zlib", &ZlibStreamHandler{})
}

// ensureZlibHandler registers the compress.zlib:// stream handler with the current global if not already done.
func ensureZlibHandler(ctx phpv.Context) {
	if g, ok := ctx.Global().(*phpctx.Global); ok {
		if !g.HasStreamHandler("compress.zlib") {
			g.RegisterStreamHandler("compress.zlib", &ZlibStreamHandler{})
		}
	}
}
