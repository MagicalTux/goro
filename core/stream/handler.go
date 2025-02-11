package stream

import (
	"net/url"
	"os"
)

type Handler interface {
	Open(path *url.URL, mode ...string) (*Stream, error)
	Exists(path *url.URL) (bool, error)
	Stat(path *url.URL) (os.FileInfo, error)
	Lstat(path *url.URL) (os.FileInfo, error)
}

type HandlerWriter interface {
	Rename(from, to *url.URL) error
}

type HandlerDir interface {
	OpenDir(path *url.URL) (*Stream, error) // stream?
	Mkdir(path *url.URL)
	RmDir(path *url.URL)
}

// TODO move chdir to global context
type Chdir interface {
	Chdir(path string) error
	Getwd() string
}
