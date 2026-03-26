package stream

import "os"

// Flusher is a stream with a flush method, such as bufio.Writer
type Flusher interface {
	Flush() error
}

type Stater interface {
	Stat() (os.FileInfo, error)
}

type Syncer interface {
	Sync() error
}

// Truncater is a stream that supports truncation (e.g., os.File, readWriteBuffer)
type Truncater interface {
	Truncate(size int64) error
}
