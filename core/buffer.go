package core

import "io"

type Buffer struct {
	w io.Writer
	b []byte

	ImplicitFlush bool
}

func (b *Buffer) Write(d []byte) (int, error) {
	b.b = append(b.b, d...)
	// should we flush
	if b.ImplicitFlush {
		return len(d), b.Flush()
	}
	return len(d), nil
}

func (b *Buffer) Flush() error {
	// perform flush
	for {
		if len(b.b) == 0 {
			// nothing to send
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
