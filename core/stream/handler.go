package stream

type FileSystem interface {
	Open(name string) (*Stream, error)
}
