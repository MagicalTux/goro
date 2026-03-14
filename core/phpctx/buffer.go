package phpctx

import (
	"errors"
	"io"

	"github.com/MagicalTux/goro/core/logopt"
	"github.com/MagicalTux/goro/core/phperr"
	"github.com/MagicalTux/goro/core/phpv"
)

const (
	BufferWrite     = 0
	BufferStart     = 0x0001
	BufferClean     = 0x0002
	BufferFlush     = 0x0004
	BufferFinal     = 0x0008
	BufferCleanable = 0x0010
	BufferFlushable = 0x0020
	BufferRemovable = 0x0040

	// Status flags (reported by ob_get_status, not user-settable)
	BufferTypeUser  = 0x0001 // bit 0 in type field; also set in flags for user handlers
	BufferStarted   = 0x1000
	BufferDisabled  = 0x2000
	BufferProcessed = 0x4000
)

// > const
const (
	PHP_OUTPUT_HANDLER_START     = phpv.ZInt(BufferStart)
	PHP_OUTPUT_HANDLER_WRITE     = phpv.ZInt(BufferWrite)
	PHP_OUTPUT_HANDLER_FLUSH     = phpv.ZInt(BufferFlush)
	PHP_OUTPUT_HANDLER_CLEAN     = phpv.ZInt(BufferClean)
	PHP_OUTPUT_HANDLER_FINAL     = phpv.ZInt(BufferFinal)
	PHP_OUTPUT_HANDLER_CONT      = phpv.ZInt(BufferWrite)
	PHP_OUTPUT_HANDLER_END       = phpv.ZInt(BufferFinal)
	PHP_OUTPUT_HANDLER_CLEANABLE = phpv.ZInt(BufferCleanable)
	PHP_OUTPUT_HANDLER_FLUSHABLE = phpv.ZInt(BufferFlushable)
	PHP_OUTPUT_HANDLER_REMOVABLE = phpv.ZInt(BufferRemovable)
)

// > const
const (
	PHP_OUTPUT_HANDLER_STARTED   = phpv.ZInt(BufferStarted)
	PHP_OUTPUT_HANDLER_DISABLED  = phpv.ZInt(BufferDisabled)
	PHP_OUTPUT_HANDLER_PROCESSED = phpv.ZInt(BufferProcessed)
)

// > const
var PHP_OUTPUT_HANDLER_STDFLAGS = phpv.ZInt(BufferCleanable | BufferFlushable | BufferRemovable)

type Buffer struct {
	w       io.Writer
	b       []byte
	g       *Global
	level   int
	started bool

	ImplicitFlush bool
	ChunkSize     int
	CB            phpv.Callable
	Flags         int // capability flags (cleanable/flushable/removable)
	status        int // runtime status flags (started/disabled/processed)

	// Set by ob_* functions before calling Flush/Close/Clean to provide
	// context for deprecation warnings about output from handlers.
	callerCtx      phpv.Context
	callerFuncName string
	callerLoc      *phpv.Loc // snapshot of caller location at SetCaller time
}

type Flusher interface {
	Flush() error
}

func makeBuffer(g *Global, w io.Writer) *Buffer {
	defaultFlags := BufferCleanable | BufferFlushable | BufferRemovable
	if b, ok := w.(*Buffer); ok {
		// this is a buffer
		return &Buffer{
			w:     b,
			g:     g,
			level: b.level + 1,
			Flags: defaultFlags,
		}
	}

	return &Buffer{
		w:     w,
		g:     g,
		level: 0,
		Flags: defaultFlags,
	}
}

// invokeCallback passes data through the output buffer callback and returns the transformed data.
// If the callback returns false, the original data is returned unchanged.
// The second return value may be a deprecation error if the callback produced output.
func (b *Buffer) invokeCallback(d []byte, flag int) ([]byte, error) {
	args := []*phpv.ZVal{phpv.ZString(d).ZVal(), phpv.ZInt(flag).ZVal()}
	// Use the caller context (from ob_* function) if available so the ob_*
	// function appears in stack traces. Otherwise fall back to the global.
	var baseCtx phpv.Context = b.g
	if b.callerCtx != nil {
		baseCtx = b.callerCtx
	}
	ctx := WithConfig(baseCtx, "ob_in_handler", phpv.ZBool(true).ZVal())
	detector := &outputDetector{}
	ctx = NewBufContext(ctx, detector)
	r, err := b.g.CallZValInternal(ctx, b.CB, args, nil)
	if err != nil {
		// Disable the callback on failure so subsequent writes pass through
		b.CB = nil
		b.status |= BufferDisabled
		return nil, err
	}

	// Check if the callback produced output (deprecated in PHP 8.4)
	var deprecationErr error
	if detector.hasOutput && b.callerFuncName != "" && b.callerCtx != nil {
		cbName := phpv.CallableDisplayName(b.CB)
		// Temporarily redirect output to the underlying writer so the
		// deprecation message bypasses this buffer (matching PHP behavior).
		savedOut := b.g.out
		b.g.out = b.w
		// Use the captured caller location so the error points to the
		// ob_end_flush() call, not the echo inside the handler.
		var locOpt logopt.Data
		locOpt.IsInternal = true // error originates from internal OB system
		if b.callerLoc != nil {
			locOpt.Loc = b.callerLoc
		}
		deprecationErr = b.callerCtx.Deprecated(
			"Producing output from user output handler %s is deprecated",
			cbName, locOpt,
		)
		b.g.out = savedOut
	}

	// If callback returns false, use original data unchanged
	if r.GetType() == phpv.ZtBool && !bool(r.Value().(phpv.ZBool)) {
		if deprecationErr != nil {
			return d, deprecationErr
		}
		return d, nil
	}
	r, err = r.As(b.g, phpv.ZtString)
	if err != nil {
		return nil, err
	}
	result := []byte(r.AsString(b.g))
	if deprecationErr != nil {
		return result, deprecationErr
	}
	return result, nil
}

func (b *Buffer) Write(d []byte) (int, error) {
	olen := len(d)

	// Always buffer raw data; the callback is invoked only on flush/close.
	if len(d) > 0 {
		b.b = append(b.b, d...)
	}

	// Check if we should auto-flush (chunk-size overflow or implicit flush).
	// Auto-flush uses WRITE/START flags, not FLUSH (which is for explicit ob_flush).
	if b.ImplicitFlush || (b.ChunkSize > 0 && len(b.b) >= b.ChunkSize) {
		flag := BufferWrite
		if !b.started {
			b.started = true
			b.status |= BufferStarted
			flag |= BufferStart
		}
		err := b.flushData(flag)
		if err == nil {
			b.status |= BufferProcessed
		}
		return olen, err
	}
	return olen, nil
}

// flushData passes the buffered data through the callback (if any), then writes
// the result to the underlying writer.
func (b *Buffer) flushData(flag int) error {
	if len(b.b) == 0 && b.CB == nil {
		return nil
	}

	data := b.b
	b.b = nil

	if b.CB != nil {
		transformed, err := b.invokeCallback(data, flag)
		if err != nil {
			// If the callback returned transformed data along with an error
			// (e.g., deprecation for producing output), write the transformed
			// data first, then return the error.
			if transformed != nil {
				writeAll(b.w, transformed)
			} else if _, ok := err.(*phperr.PhpThrow); ok {
				// For catchable exceptions (PhpThrow like TypeError), flush the raw
				// data and propagate the exception so try/catch can handle it.
				// For fatal errors (PhpError), don't flush — just propagate.
				writeAll(b.w, data)
			}
			return err
		}
		data = transformed
	}

	// Write data to the underlying writer
	writeAll(b.w, data)
	return nil
}

// writeAll writes all data to the writer, retrying on short writes.
func writeAll(w io.Writer, data []byte) {
	for len(data) > 0 {
		n, err := w.Write(data)
		if n == len(data) {
			break
		} else if n > 0 {
			data = data[n:]
		}
		if err != nil {
			return
		}
	}
}

func (b *Buffer) IsCleanable() bool {
	return b.Flags&BufferCleanable != 0
}

func (b *Buffer) IsFlushable() bool {
	return b.Flags&BufferFlushable != 0
}

func (b *Buffer) IsRemovable() bool {
	return b.Flags&BufferRemovable != 0
}

func (b *Buffer) Flush() error {
	flag := BufferFlush
	if !b.started {
		b.started = true
		b.status |= BufferStarted
		flag |= BufferStart
	}
	err := b.flushData(flag)
	if err == nil {
		b.status |= BufferProcessed
	}
	return err
}

func (b *Buffer) Close() error {
	return b.CloseWithFlags(BufferFinal, true)
}

// CloseClean closes and discards the buffer (ob_end_clean behavior).
// Invokes callback with CLEAN|FINAL, discards result, removes buffer.
func (b *Buffer) CloseClean() error {
	return b.CloseWithFlags(BufferClean|BufferFinal, false)
}

// CloseWithFlags closes the buffer with the given flags.
// If writeOutput is true, writes the callback result to the underlying writer.
func (b *Buffer) CloseWithFlags(flag int, writeOutput bool) error {
	if b.g.buf != b {
		return errors.New("this buffer cannot be closed, not on top of stack")
	}

	if !b.started {
		b.started = true
		b.status |= BufferStarted
		flag |= BufferStart
	}

	var flushErr error
	if writeOutput {
		flushErr = b.flushData(flag)
	} else {
		// Invoke callback but discard the result
		if b.CB != nil {
			_, flushErr = b.invokeCallback(b.b, flag)
		}
		b.b = nil
	}

	b.status |= BufferProcessed

	// Always remove the buffer from the stack, even on error
	if pbuf, ok := b.w.(*Buffer); ok {
		b.g.buf = pbuf
		b.g.out = pbuf
	} else {
		b.g.buf = nil
		b.g.out = b.w
	}

	return flushErr
}

func (b *Buffer) Clean() error {
	// In PHP, ob_clean() invokes the callback with PHP_OUTPUT_HANDLER_CLEAN
	// (and START if first time), then discards the result.
	var err error
	if b.CB != nil {
		flag := BufferClean
		if !b.started {
			b.started = true
			b.status |= BufferStarted
			flag |= BufferStart
		}
		_, err = b.invokeCallback(b.b, flag)
	}
	b.b = nil
	return err
}

// StatusFlags returns the combined flags for ob_get_status reporting.
func (b *Buffer) StatusFlags() int {
	flags := b.Flags
	if b.CB != nil {
		flags |= BufferTypeUser
	}
	flags |= b.status
	return flags
}

// Type returns 0 for internal (no callback) or 1 for user callback.
func (b *Buffer) Type() int {
	if b.CB != nil {
		return 1
	}
	return 0
}

// CallbackName returns the display name of the callback for ob_get_status/ob_list_handlers.
func (b *Buffer) CallbackName() string {
	if b.CB == nil {
		return "default output handler"
	}
	return phpv.CallableDisplayName(b.CB)
}

func (b *Buffer) Level() int {
	return b.level
}

func (b *Buffer) Get() []byte {
	return b.b
}

func (b *Buffer) Parent() *Buffer {
	if p, ok := b.w.(*Buffer); ok {
		return p
	}
	return nil
}

// SetCaller stores the calling function context for deprecation warnings.
// The location is captured at call time so it doesn't change during callback execution.
func (b *Buffer) SetCaller(ctx phpv.Context, funcName string) {
	b.callerCtx = ctx
	b.callerFuncName = funcName
	// Snapshot the location now, before the callback runs and potentially
	// changes the global location pointer.
	if loc := ctx.Loc(); loc != nil {
		b.callerLoc = &phpv.Loc{Filename: loc.Filename, Line: loc.Line}
	}
}

// ClearCaller removes the calling function context.
func (b *Buffer) ClearCaller() {
	b.callerCtx = nil
	b.callerFuncName = ""
	b.callerLoc = nil
}

// outputDetector is a writer that tracks whether any output was produced.
type outputDetector struct {
	hasOutput bool
}

func (d *outputDetector) Write(p []byte) (int, error) {
	if len(p) > 0 {
		d.hasOutput = true
	}
	return len(p), nil
}
