package stream

import "net/url"

type Handler interface {
	Open(path *url.URL) (*Stream, error)
}
