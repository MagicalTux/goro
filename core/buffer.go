package core

import (
	"errors"
	"io"
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

//> const PHP_OUTPUT_HANDLER_START: ZInt(BufferStart)
//> const PHP_OUTPUT_HANDLER_WRITE: ZInt(BufferWrite)
//> const PHP_OUTPUT_HANDLER_FLUSH: ZInt(BufferFlush)
//> const PHP_OUTPUT_HANDLER_CLEAN: ZInt(BufferClean)
//> const PHP_OUTPUT_HANDLER_FINAL: ZInt(BufferFinal)
//> const PHP_OUTPUT_HANDLER_CONT: ZInt(BufferWrite)
//> const PHP_OUTPUT_HANDLER_END: ZInt(BufferFinal)
//> const PHP_OUTPUT_HANDLER_CLEANABLE: ZInt(BufferCleanable)
//> const PHP_OUTPUT_HANDLER_FLUSHABLE: ZInt(BufferFlushable)
//> const PHP_OUTPUT_HANDLER_REMOVABLE: ZInt(BufferRemovable)
//> const PHP_OUTPUT_HANDLER_STDFLAGS: ZInt(BufferCleanable|BufferFlushable|BufferRemovable)

type Buffer struct {
	w       io.Writer
	b       []byte
	g       *Global
	level   int
	started bool

	ImplicitFlush bool
	ChunkSize     int
	CB            Callable
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
		level: 1,
	}
}

func (b *Buffer) add(d []byte, flag int) error {
	if b.CB == nil {
		if len(d) == 0 {
			return nil
		}
		b.b = append(b.b, d...)
		return nil
	}

	// pass d through output buffer callback
	args := []*ZVal{ZString(d).ZVal(), ZInt(flag).ZVal()}
	ctx := WithConfig(b.g.Root(), "ob_in_handler", ZBool(true).ZVal())
	ctx = NewBufContext(ctx, nil)
	r, err := ctx.CallZVal(ctx, b.CB, args, nil)
	if err != nil {
		return err
	}
	r, err = r.As(b.g.Root(), ZtString)
	if err != nil {
		return err
	}
	d = []byte(r.AsString(b.g.Root()))

	if len(d) == 0 {
		return nil
	}

	b.b = append(b.b, d...)
	return nil
}

func (b *Buffer) Write(d []byte) (int, error) {
	olen := len(d)

	flag := BufferWrite | BufferFlush
	if !b.started {
		b.started = true
		flag |= BufferStart
	}
	b.add(d, flag)

	// should we flush
	if b.ImplicitFlush {
		return olen, b.Flush()
	} else if (b.ChunkSize != 0) && (len(b.b) >= b.ChunkSize) {
		return olen, b.Flush()
	}
	return olen, nil
}

func (b *Buffer) Flush() error {
	// perform flush
	for {
		if len(b.b) == 0 {
			return nil
		}

		n, err := b.w.Write(b.b)
		if n == len(b.b) {
			b.b = nil // do not keep buffer as to allow garbage collector
		} else if n > 0 {
			b.b = b.b[n:]
		}
		if err != nil {
			return err
		}
	}
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
	b.add(nil, flag)

	err := b.Flush()
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
	b.b = nil
}

func (b *Buffer) Level() int {
	return b.level
}

func (b *Buffer) Get() []byte {
	return b.b
}
