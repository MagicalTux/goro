package hash

// MurmurHash3 implementations for murmur3a (32-bit), murmur3c (128-bit x86),
// and murmur3f (128-bit x64).
// Reference: https://github.com/aappleby/smhasher

import (
	"encoding/binary"
	gohash "hash"
	"math/bits"
)

// --- murmur3a (32-bit) ---

type murmur3A struct {
	seed uint32
	h1   uint32
	buf  [4]byte
	bufN int
	n    int
}

func newMurmur3A(seed uint32) gohash.Hash {
	m := &murmur3A{seed: seed}
	m.Reset()
	return m
}

func (m *murmur3A) Reset() {
	m.h1 = m.seed
	m.bufN = 0
	m.n = 0
}
func (m *murmur3A) Size() int      { return 4 }
func (m *murmur3A) BlockSize() int { return 4 }

func (m *murmur3A) Write(p []byte) (int, error) {
	n := len(p)
	m.n += n

	if m.bufN > 0 {
		avail := 4 - m.bufN
		if len(p) < avail {
			copy(m.buf[m.bufN:], p)
			m.bufN += len(p)
			return n, nil
		}
		copy(m.buf[m.bufN:], p[:avail])
		p = p[avail:]
		m.bufN = 0
		m.processBlock(m.buf[:])
	}

	for len(p) >= 4 {
		m.processBlock(p[:4])
		p = p[4:]
	}

	if len(p) > 0 {
		copy(m.buf[:], p)
		m.bufN = len(p)
	}
	return n, nil
}

func (m *murmur3A) processBlock(b []byte) {
	k1 := binary.LittleEndian.Uint32(b)
	k1 *= 0xcc9e2d51
	k1 = bits.RotateLeft32(k1, 15)
	k1 *= 0x1b873593
	m.h1 ^= k1
	m.h1 = bits.RotateLeft32(m.h1, 13)
	m.h1 = m.h1*5 + 0xe6546b64
}

func (m *murmur3A) CloneHash() gohash.Hash {
	c := *m
	return &c
}

func (m *murmur3A) Sum(in []byte) []byte {
	h := m.finalize()
	var b [4]byte
	binary.BigEndian.PutUint32(b[:], h)
	return append(in, b[:]...)
}

func (m *murmur3A) finalize() uint32 {
	h1 := m.h1

	// tail
	k1 := uint32(0)
	switch m.bufN & 3 {
	case 3:
		k1 ^= uint32(m.buf[2]) << 16
		fallthrough
	case 2:
		k1 ^= uint32(m.buf[1]) << 8
		fallthrough
	case 1:
		k1 ^= uint32(m.buf[0])
		k1 *= 0xcc9e2d51
		k1 = bits.RotateLeft32(k1, 15)
		k1 *= 0x1b873593
		h1 ^= k1
	}

	h1 ^= uint32(m.n)
	h1 = fmix32(h1)
	return h1
}

func fmix32(h uint32) uint32 {
	h ^= h >> 16
	h *= 0x85ebca6b
	h ^= h >> 13
	h *= 0xc2b2ae35
	h ^= h >> 16
	return h
}

// --- murmur3c (128-bit x86, 4 x 32-bit) ---

type murmur3C struct {
	seed uint32
	h1   uint32
	h2   uint32
	h3   uint32
	h4   uint32
	buf  [16]byte
	bufN int
	n    int
}

func newMurmur3C(seed uint32) gohash.Hash {
	m := &murmur3C{seed: seed}
	m.Reset()
	return m
}

func (m *murmur3C) Reset() {
	m.h1 = m.seed
	m.h2 = m.seed
	m.h3 = m.seed
	m.h4 = m.seed
	m.bufN = 0
	m.n = 0
}
func (m *murmur3C) Size() int      { return 16 }
func (m *murmur3C) BlockSize() int { return 16 }

func (m *murmur3C) Write(p []byte) (int, error) {
	n := len(p)
	m.n += n

	if m.bufN > 0 {
		avail := 16 - m.bufN
		if len(p) < avail {
			copy(m.buf[m.bufN:], p)
			m.bufN += len(p)
			return n, nil
		}
		copy(m.buf[m.bufN:], p[:avail])
		p = p[avail:]
		m.bufN = 0
		m.processBlock(m.buf[:])
	}

	for len(p) >= 16 {
		m.processBlock(p[:16])
		p = p[16:]
	}

	if len(p) > 0 {
		copy(m.buf[:], p)
		m.bufN = len(p)
	}
	return n, nil
}

const (
	c1_32 = uint32(0x239b961b)
	c2_32 = uint32(0xab0e9789)
	c3_32 = uint32(0x38b34ae5)
	c4_32 = uint32(0xa1e38b93)
)

func (m *murmur3C) processBlock(b []byte) {
	k1 := binary.LittleEndian.Uint32(b[0:])
	k2 := binary.LittleEndian.Uint32(b[4:])
	k3 := binary.LittleEndian.Uint32(b[8:])
	k4 := binary.LittleEndian.Uint32(b[12:])

	k1 *= c1_32
	k1 = bits.RotateLeft32(k1, 15)
	k1 *= c2_32
	m.h1 ^= k1

	m.h1 = bits.RotateLeft32(m.h1, 19)
	m.h1 += m.h2
	m.h1 = m.h1*5 + 0x561ccd1b

	k2 *= c2_32
	k2 = bits.RotateLeft32(k2, 16)
	k2 *= c3_32
	m.h2 ^= k2

	m.h2 = bits.RotateLeft32(m.h2, 17)
	m.h2 += m.h3
	m.h2 = m.h2*5 + 0x0bcaa747

	k3 *= c3_32
	k3 = bits.RotateLeft32(k3, 17)
	k3 *= c4_32
	m.h3 ^= k3

	m.h3 = bits.RotateLeft32(m.h3, 15)
	m.h3 += m.h4
	m.h3 = m.h3*5 + 0x96cd1c35

	k4 *= c4_32
	k4 = bits.RotateLeft32(k4, 18)
	k4 *= c1_32
	m.h4 ^= k4

	m.h4 = bits.RotateLeft32(m.h4, 13)
	m.h4 += m.h1
	m.h4 = m.h4*5 + 0x32ac3b17
}

func (m *murmur3C) CloneHash() gohash.Hash {
	c := *m
	return &c
}

func (m *murmur3C) Sum(in []byte) []byte {
	h1, h2, h3, h4 := m.finalize()
	var b [16]byte
	binary.BigEndian.PutUint32(b[0:], h1)
	binary.BigEndian.PutUint32(b[4:], h2)
	binary.BigEndian.PutUint32(b[8:], h3)
	binary.BigEndian.PutUint32(b[12:], h4)
	return append(in, b[:]...)
}

func (m *murmur3C) finalize() (uint32, uint32, uint32, uint32) {
	h1, h2, h3, h4 := m.h1, m.h2, m.h3, m.h4

	k1 := uint32(0)
	k2 := uint32(0)
	k3 := uint32(0)
	k4 := uint32(0)

	tail := m.buf[:m.bufN]
	switch m.bufN & 15 {
	case 15:
		k4 ^= uint32(tail[14]) << 16
		fallthrough
	case 14:
		k4 ^= uint32(tail[13]) << 8
		fallthrough
	case 13:
		k4 ^= uint32(tail[12])
		k4 *= c4_32
		k4 = bits.RotateLeft32(k4, 18)
		k4 *= c1_32
		h4 ^= k4
		fallthrough
	case 12:
		k3 ^= uint32(tail[11]) << 24
		fallthrough
	case 11:
		k3 ^= uint32(tail[10]) << 16
		fallthrough
	case 10:
		k3 ^= uint32(tail[9]) << 8
		fallthrough
	case 9:
		k3 ^= uint32(tail[8])
		k3 *= c3_32
		k3 = bits.RotateLeft32(k3, 17)
		k3 *= c4_32
		h3 ^= k3
		fallthrough
	case 8:
		k2 ^= uint32(tail[7]) << 24
		fallthrough
	case 7:
		k2 ^= uint32(tail[6]) << 16
		fallthrough
	case 6:
		k2 ^= uint32(tail[5]) << 8
		fallthrough
	case 5:
		k2 ^= uint32(tail[4])
		k2 *= c2_32
		k2 = bits.RotateLeft32(k2, 16)
		k2 *= c3_32
		h2 ^= k2
		fallthrough
	case 4:
		k1 ^= uint32(tail[3]) << 24
		fallthrough
	case 3:
		k1 ^= uint32(tail[2]) << 16
		fallthrough
	case 2:
		k1 ^= uint32(tail[1]) << 8
		fallthrough
	case 1:
		k1 ^= uint32(tail[0])
		k1 *= c1_32
		k1 = bits.RotateLeft32(k1, 15)
		k1 *= c2_32
		h1 ^= k1
	}

	h1 ^= uint32(m.n)
	h2 ^= uint32(m.n)
	h3 ^= uint32(m.n)
	h4 ^= uint32(m.n)

	h1 += h2
	h1 += h3
	h1 += h4
	h2 += h1
	h3 += h1
	h4 += h1

	h1 = fmix32(h1)
	h2 = fmix32(h2)
	h3 = fmix32(h3)
	h4 = fmix32(h4)

	h1 += h2
	h1 += h3
	h1 += h4
	h2 += h1
	h3 += h1
	h4 += h1

	return h1, h2, h3, h4
}

// --- murmur3f (128-bit x64, 2 x 64-bit) ---

type murmur3F struct {
	seed uint32
	h1   uint64
	h2   uint64
	buf  [16]byte
	bufN int
	n    int
}

func newMurmur3F(seed uint32) gohash.Hash {
	m := &murmur3F{seed: seed}
	m.Reset()
	return m
}

func (m *murmur3F) Reset() {
	m.h1 = uint64(m.seed)
	m.h2 = uint64(m.seed)
	m.bufN = 0
	m.n = 0
}
func (m *murmur3F) Size() int      { return 16 }
func (m *murmur3F) BlockSize() int { return 16 }

const (
	c1_64 = uint64(0x87c37b91114253d5)
	c2_64 = uint64(0x4cf5ad432745937f)
)

func (m *murmur3F) Write(p []byte) (int, error) {
	n := len(p)
	m.n += n

	if m.bufN > 0 {
		avail := 16 - m.bufN
		if len(p) < avail {
			copy(m.buf[m.bufN:], p)
			m.bufN += len(p)
			return n, nil
		}
		copy(m.buf[m.bufN:], p[:avail])
		p = p[avail:]
		m.bufN = 0
		m.processBlock(m.buf[:])
	}

	for len(p) >= 16 {
		m.processBlock(p[:16])
		p = p[16:]
	}

	if len(p) > 0 {
		copy(m.buf[:], p)
		m.bufN = len(p)
	}
	return n, nil
}

func (m *murmur3F) processBlock(b []byte) {
	k1 := binary.LittleEndian.Uint64(b[0:])
	k2 := binary.LittleEndian.Uint64(b[8:])

	k1 *= c1_64
	k1 = bits.RotateLeft64(k1, 31)
	k1 *= c2_64
	m.h1 ^= k1

	m.h1 = bits.RotateLeft64(m.h1, 27)
	m.h1 += m.h2
	m.h1 = m.h1*5 + 0x52dce729

	k2 *= c2_64
	k2 = bits.RotateLeft64(k2, 33)
	k2 *= c1_64
	m.h2 ^= k2

	m.h2 = bits.RotateLeft64(m.h2, 31)
	m.h2 += m.h1
	m.h2 = m.h2*5 + 0x38495ab5
}

func (m *murmur3F) CloneHash() gohash.Hash {
	c := *m
	return &c
}

func (m *murmur3F) Sum(in []byte) []byte {
	h1, h2 := m.finalize()
	var b [16]byte
	binary.BigEndian.PutUint64(b[0:], h1)
	binary.BigEndian.PutUint64(b[8:], h2)
	return append(in, b[:]...)
}

func (m *murmur3F) finalize() (uint64, uint64) {
	h1, h2 := m.h1, m.h2

	tail := m.buf[:m.bufN]
	k1 := uint64(0)
	k2 := uint64(0)

	switch m.bufN & 15 {
	case 15:
		k2 ^= uint64(tail[14]) << 48
		fallthrough
	case 14:
		k2 ^= uint64(tail[13]) << 40
		fallthrough
	case 13:
		k2 ^= uint64(tail[12]) << 32
		fallthrough
	case 12:
		k2 ^= uint64(tail[11]) << 24
		fallthrough
	case 11:
		k2 ^= uint64(tail[10]) << 16
		fallthrough
	case 10:
		k2 ^= uint64(tail[9]) << 8
		fallthrough
	case 9:
		k2 ^= uint64(tail[8])
		k2 *= c2_64
		k2 = bits.RotateLeft64(k2, 33)
		k2 *= c1_64
		h2 ^= k2
		fallthrough
	case 8:
		k1 ^= uint64(tail[7]) << 56
		fallthrough
	case 7:
		k1 ^= uint64(tail[6]) << 48
		fallthrough
	case 6:
		k1 ^= uint64(tail[5]) << 40
		fallthrough
	case 5:
		k1 ^= uint64(tail[4]) << 32
		fallthrough
	case 4:
		k1 ^= uint64(tail[3]) << 24
		fallthrough
	case 3:
		k1 ^= uint64(tail[2]) << 16
		fallthrough
	case 2:
		k1 ^= uint64(tail[1]) << 8
		fallthrough
	case 1:
		k1 ^= uint64(tail[0])
		k1 *= c1_64
		k1 = bits.RotateLeft64(k1, 31)
		k1 *= c2_64
		h1 ^= k1
	}

	h1 ^= uint64(m.n)
	h2 ^= uint64(m.n)

	h1 += h2
	h2 += h1

	h1 = fmix64(h1)
	h2 = fmix64(h2)

	h1 += h2
	h2 += h1

	return h1, h2
}

func fmix64(k uint64) uint64 {
	k ^= k >> 33
	k *= 0xff51afd7ed558ccd
	k ^= k >> 33
	k *= 0xc4ceb9fe1a85ec53
	k ^= k >> 33
	return k
}
