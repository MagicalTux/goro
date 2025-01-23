package phpv

import (
	"os"
)

type Stream interface {
	Resource
	Read(p []byte) (int, error)
	Write(p []byte) (n int, err error)
	Seek(offset int64, whence int) (int64, error)
	ReadByte() (byte, error)
	Close() error
	SetAttr(k string, v interface{})
	Attr(v interface{}) interface{}
	Stat() (os.FileInfo, error)
	Flush() error
	Sync() error
}

type AttrStream interface {
	Attr(v interface{}) interface{}
}
