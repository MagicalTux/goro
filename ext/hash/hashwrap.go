package hash

// hashReplayable wraps a hash.Hash and buffers all written data
// so that it can be cloned by replaying the writes on a fresh hash.
// Used for external hash implementations that don't support BinaryMarshaler or CloneHash.

import (
	gohash "hash"
)

type hashReplayable struct {
	gohash.Hash
	newFn func() gohash.Hash
	data  []byte
}

func newHashReplayable(h gohash.Hash, newFn func() gohash.Hash) *hashReplayable {
	return &hashReplayable{Hash: h, newFn: newFn}
}

func (w *hashReplayable) Write(p []byte) (int, error) {
	w.data = append(w.data, p...)
	return w.Hash.Write(p)
}

func (w *hashReplayable) Reset() {
	w.data = w.data[:0]
	w.Hash.Reset()
}

func (w *hashReplayable) CloneHash() gohash.Hash {
	c := w.newFn()
	c.Write(w.data)
	dataCopy := make([]byte, len(w.data))
	copy(dataCopy, w.data)
	return &hashReplayable{Hash: c, newFn: w.newFn, data: dataCopy}
}
