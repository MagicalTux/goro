package core

import (
	"io"
	"net/http"
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
	w     io.Writer
	b     []byte
	g     *Global
	level int

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
			w:             b,
			g:             g,
			level:         b.level + 1,
			ImplicitFlush: true,
		}
	}

	return &Buffer{
		w:             w,
		g:             g,
		level:         1,
		ImplicitFlush: true,
	}
}

func (b *Buffer) Write(d []byte) (int, error) {
	b.b = append(b.b, d...)
	// should we flush
	if b.ImplicitFlush {
		return len(d), b.Flush()
	} else if (b.ChunkSize != 0) && (len(b.b) >= b.ChunkSize) {
		return len(d), b.Flush()
	}
	return len(d), nil
}

func (b *Buffer) Flush() error {
	// perform flush
	for {
		if len(b.b) == 0 {
			// nothing to send, flush underlying buffer if needed
			if f, ok := b.w.(Flusher); ok {
				return f.Flush()
			}
			// also check for http.Flusher(), almost same as us except returns no error
			if f, ok := b.w.(http.Flusher); ok {
				f.Flush()
				return nil
			}
			return nil
		}

		// TODO check for b.CB

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
