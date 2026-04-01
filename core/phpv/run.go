package phpv

import "io"

// CompoundDumper is implemented by compound statements (if, for, while, foreach, class, etc.)
// that end with "}" in their Dump output and should not have ";" appended by DumpStatements.
type CompoundDumper interface {
	IsCompoundDump()
}

type Runnables []Runnable

func (r Runnables) Run(ctx Context) (l *ZVal, err error) {
	g := ctx.Global()
	for _, v := range r {
		l, err = v.Run(ctx)
		// After each statement, release any temporary objects (e.g., GMP results
		// that were passed to var_dump/echo but never stored in a PHP variable).
		// This mimics PHP's deterministic refcount-based freeing.
		g.DrainTempObjects()
		if err != nil {
			return
		}
	}
	return
}

func (r Runnables) Dump(w io.Writer) error {
	return r.DumpWith(w, []byte{';'})
}

// DumpStatements dumps each statement followed by ";\n" for simple statements,
// or just "\n" for compound statements (if, for, while, class, etc.) that end with "}".
func (r Runnables) DumpStatements(w io.Writer) error {
	for _, s := range r {
		err := s.Dump(w)
		if err != nil {
			return err
		}
		if _, ok := s.(CompoundDumper); ok {
			_, err = w.Write([]byte("\n"))
		} else {
			_, err = w.Write([]byte(";\n"))
		}
		if err != nil {
			return err
		}
	}
	return nil
}

func (r Runnables) DumpWith(w io.Writer, sep []byte) error {
	for i, s := range r {
		err := s.Dump(w)
		if err != nil {
			return err
		}
		// Only add separator between elements, not after the last one
		if i < len(r)-1 {
			_, err = w.Write(sep)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

type RunNull struct{}

func (r RunNull) Run(ctx Context) (*ZVal, error) {
	return ZNULL.ZVal(), nil
}

func (r RunNull) Dump(w io.Writer) error {
	return nil
}
