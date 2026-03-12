package phpctx

import (
	"errors"
	"io"

	"github.com/MagicalTux/goro/core/phpv"
)

const (
	BufferWrite = 0
	BufferStart = 1 << iota
	BufferClean
	BufferFlush
	BufferFinal
	BufferCleanable
	BufferFlushable
	BufferRemovable
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
}

type Flusher interface {
	Flush() error
}

func makeBuffer(g *Global, w io.Writer) *Buffer {
	if b, ok := w.(*Buffer); ok {
		// this is a buffer
		return &Buffer{
			w:     b,
			g:     g,
			level: b.level + 1,
		}
	}

	return &Buffer{
		w:     w,
		g:     g,
		level: 0,
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

func (b *Buffer) Flush() error {
	flag := BufferFlush
	if !b.started {
		b.started = true
		flag |= BufferStart
	}
	return b.flushData(flag)
}

func (b *Buffer) Close() error {
	if b.g.buf != b {
		return errors.New("this buffer cannot be closed, not on top of stack")
	}

	flag := BufferFinal
	if !b.started {
		b.started = true
		flag |= BufferStart
	}

	err := b.flushData(flag)
	if err != nil {
		return err
	}

	// get parent
	if pbuf, ok := b.w.(*Buffer); ok {
		// got parent
		b.g.buf = pbuf
		b.g.out = pbuf
		return nil
	}

	// no parent
	b.g.buf = nil
	b.g.out = b.w
	return nil
}

func (b *Buffer) Clean() {
	// In PHP, ob_clean() invokes the callback with PHP_OUTPUT_HANDLER_CLEAN
	// (and START if first time), then discards the result.
	if b.CB != nil {
		flag := BufferClean
		if !b.started {
			b.started = true
			flag |= BufferStart
		}
		b.invokeCallback(b.b, flag)
	}
	b.b = nil
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
