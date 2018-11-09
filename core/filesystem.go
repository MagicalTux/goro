package core

type FileSystem interface {
	Open(name string) (*Stream, error)
}
