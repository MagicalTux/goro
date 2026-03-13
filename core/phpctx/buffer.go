package phpctx

import (
	"errors"
	"io"

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
	PHP_OUTPUT_HANDLER_STARTED  = phpv.ZInt(BufferStarted)
	PHP_OUTPUT_HANDLER_DISABLED = phpv.ZInt(BufferDisabled)
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
func (b *Buffer) invokeCallback(d []byte, flag int) ([]byte, error) {
	args := []*phpv.ZVal{phpv.ZString(d).ZVal(), phpv.ZInt(flag).ZVal()}
	ctx := WithConfig(b.g, "ob_in_handler", phpv.ZBool(true).ZVal())
	ctx = NewBufContext(ctx, nil)
	r, err := ctx.CallZVal(ctx, b.CB, args, nil)
	if err != nil {
		// Disable the callback on failure so subsequent writes pass through
		b.CB = nil
		b.status |= BufferDisabled
		return nil, err
	}
	// If callback returns false, use original data unchanged
	if r.GetType() == phpv.ZtBool && !bool(r.Value().(phpv.ZBool)) {
		return d, nil
	}
	r, err = r.As(b.g, phpv.ZtString)
	if err != nil {
		return nil, err
	}
	return []byte(r.AsString(b.g)), nil
}

func (b *Buffer) Write(d []byte) (int, error) {
	olen := len(d)

	// Always buffer raw data; the callback is invoked only on flush/close.
	if len(d) > 0 {
		b.b = append(b.b, d...)
	}

	// should we flush
	if b.ImplicitFlush {
		return olen, b.Flush()
	} else if (b.ChunkSize != 0) && (len(b.b) >= b.ChunkSize) {
		return olen, b.Flush()
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
			// For catchable exceptions (PhpThrow like TypeError), flush the raw
			// data and propagate the exception so try/catch can handle it.
			// For fatal errors (PhpError), don't flush — just propagate.
			if _, ok := err.(*phperr.PhpThrow); ok {
				for len(data) > 0 {
					n, werr := b.w.Write(data)
					if n == len(data) {
						break
					} else if n > 0 {
						data = data[n:]
					}
					if werr != nil {
						return werr
					}
				}
			}
			return err
		}
		data = transformed
	}

	// Write data to the underlying writer
	for len(data) > 0 {
		n, err := b.w.Write(data)
		if n == len(data) {
			break
		} else if n > 0 {
			data = data[n:]
		}
		if err != nil {
			return err
		}
	}
	return nil
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
	if b.g.buf != b {
		return errors.New("this buffer cannot be closed, not on top of stack")
	}

	flag := BufferFinal
	if !b.started {
		b.started = true
		b.status |= BufferStarted
		flag |= BufferStart
	}

	flushErr := b.flushData(flag)

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
