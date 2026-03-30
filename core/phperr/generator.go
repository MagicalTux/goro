package phperr

// GeneratorForceClose is a sentinel error used to force-close a generator.
// When injected into a suspended generator goroutine, it causes the generator
// to unwind through try/finally blocks (running finally code) but is NOT caught
// by PHP catch blocks. If the generator tries to yield in a finally block while
// being force-closed, a real PHP Error is thrown.
type GeneratorForceClose struct{}

func (e *GeneratorForceClose) Error() string {
	return "generator force close"
}
