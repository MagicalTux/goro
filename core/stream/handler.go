package stream

import "net/url"

type Handler interface {
	Open(path *url.URL) (*Stream, error)
}

type Chdir interface {
	Chdir(path string) error
	Getwd() string
}
