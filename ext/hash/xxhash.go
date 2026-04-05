package hash

// xxHash implementations: xxh32, xxh64, xxh3, xxh128
// Reference: https://github.com/Cyan4973/xxHash

import (
	"encoding/binary"
	"hash"
	"math/bits"
)

// xxh32 constants
const (
	xxhPrime32_1 = uint32(0x9E3779B1)
	xxhPrime32_2 = uint32(0x85EBCA77)
	xxhPrime32_3 = uint32(0xC2B2AE3D)
	xxhPrime32_4 = uint32(0x27D4EB2F)
	xxhPrime32_5 = uint32(0x165667B1)
)

// xxh64 constants
const (
	xxhPrime64_1 = uint64(0x9E3779B185EBCA87)
	xxhPrime64_2 = uint64(0xC2B2AE3D27D4EB4F)
	xxhPrime64_3 = uint64(0x165667B19E3779F9)
	xxhPrime64_4 = uint64(0x85EBCA77C2B2AE63)
	xxhPrime64_5 = uint64(0x27D4EB2F165667C5)
)

// --- xxh32 ---

type xxh32State struct {
	seed  uint32
	v1    uint32
	v2    uint32
	v3    uint32
	v4    uint32
	total uint64
	buf   [16]byte
	bufN  int
}

func newXXH32(seed uint32) hash.Hash {
	s := &xxh32State{seed: seed}
	s.Reset()
	return s
}

func (s *xxh32State) Reset() {
	s.v1 = s.seed + xxhPrime32_1 + xxhPrime32_2
	s.v2 = s.seed + xxhPrime32_2
	s.v3 = s.seed
	s.v4 = s.seed - xxhPrime32_1
	s.total = 0
	s.bufN = 0
}

func (s *xxh32State) Size() int      { return 4 }
func (s *xxh32State) BlockSize() int { return 16 }

func (s *xxh32State) Write(p []byte) (int, error) {
	n := len(p)
	s.total += uint64(n)

	if s.bufN+len(p) < 16 {
		copy(s.buf[s.bufN:], p)
		s.bufN += len(p)
		return n, nil
	}

	if s.bufN > 0 {
		fill := 16 - s.bufN
		copy(s.buf[s.bufN:], p[:fill])
		p = p[fill:]
		s.bufN = 0
		s.processBlock(s.buf[:])
	}

	for len(p) >= 16 {
		s.processBlock(p[:16])
		p = p[16:]
	}

	if len(p) > 0 {
		copy(s.buf[:], p)
		s.bufN = len(p)
	}
	return n, nil
}

func (s *xxh32State) processBlock(b []byte) {
	s.v1 = xxh32Round(s.v1, binary.LittleEndian.Uint32(b[0:]))
	s.v2 = xxh32Round(s.v2, binary.LittleEndian.Uint32(b[4:]))
	s.v3 = xxh32Round(s.v3, binary.LittleEndian.Uint32(b[8:]))
	s.v4 = xxh32Round(s.v4, binary.LittleEndian.Uint32(b[12:]))
}

func xxh32Round(acc, input uint32) uint32 {
	acc += input * xxhPrime32_2
	acc = bits.RotateLeft32(acc, 13)
	acc *= xxhPrime32_1
	return acc
}

func (s *xxh32State) CloneHash() hash.Hash {
	c := *s
	return &c
}

func (s *xxh32State) Sum(in []byte) []byte {
	h := s.digest()
	var b [4]byte
	binary.BigEndian.PutUint32(b[:], h)
	return append(in, b[:]...)
}

func (s *xxh32State) digest() uint32 {
	var h32 uint32
	if s.total >= 16 {
		h32 = bits.RotateLeft32(s.v1, 1) +
			bits.RotateLeft32(s.v2, 7) +
			bits.RotateLeft32(s.v3, 12) +
			bits.RotateLeft32(s.v4, 18)
	} else {
		h32 = s.seed + xxhPrime32_5
	}
	h32 += uint32(s.total)

	tail := s.buf[:s.bufN]
	for len(tail) >= 4 {
		h32 += binary.LittleEndian.Uint32(tail) * xxhPrime32_3
		h32 = bits.RotateLeft32(h32, 17) * xxhPrime32_4
		tail = tail[4:]
	}
	for _, b := range tail {
		h32 += uint32(b) * xxhPrime32_5
		h32 = bits.RotateLeft32(h32, 11) * xxhPrime32_1
	}

	h32 ^= h32 >> 15
	h32 *= xxhPrime32_2
	h32 ^= h32 >> 13
	h32 *= xxhPrime32_3
	h32 ^= h32 >> 16
	return h32
}

// --- xxh64 ---

type xxh64State struct {
	seed  uint64
	v1    uint64
	v2    uint64
	v3    uint64
	v4    uint64
	total uint64
	buf   [32]byte
	bufN  int
}

func newXXH64(seed uint64) hash.Hash {
	s := &xxh64State{seed: seed}
	s.Reset()
	return s
}

func (s *xxh64State) Reset() {
	s.v1 = s.seed + xxhPrime64_1 + xxhPrime64_2
	s.v2 = s.seed + xxhPrime64_2
	s.v3 = s.seed
	s.v4 = s.seed - xxhPrime64_1
	s.total = 0
	s.bufN = 0
}

func (s *xxh64State) Size() int      { return 8 }
func (s *xxh64State) BlockSize() int { return 32 }

func (s *xxh64State) Write(p []byte) (int, error) {
	n := len(p)
	s.total += uint64(n)

	if s.bufN+len(p) < 32 {
		copy(s.buf[s.bufN:], p)
		s.bufN += len(p)
		return n, nil
	}

	if s.bufN > 0 {
		fill := 32 - s.bufN
		copy(s.buf[s.bufN:], p[:fill])
		p = p[fill:]
		s.bufN = 0
		s.processBlock(s.buf[:])
	}

	for len(p) >= 32 {
		s.processBlock(p[:32])
		p = p[32:]
	}

	if len(p) > 0 {
		copy(s.buf[:], p)
		s.bufN = len(p)
	}
	return n, nil
}

func (s *xxh64State) processBlock(b []byte) {
	s.v1 = xxh64Round(s.v1, binary.LittleEndian.Uint64(b[0:]))
	s.v2 = xxh64Round(s.v2, binary.LittleEndian.Uint64(b[8:]))
	s.v3 = xxh64Round(s.v3, binary.LittleEndian.Uint64(b[16:]))
	s.v4 = xxh64Round(s.v4, binary.LittleEndian.Uint64(b[24:]))
}

func xxh64Round(acc, input uint64) uint64 {
	acc += input * xxhPrime64_2
	acc = bits.RotateLeft64(acc, 31)
	acc *= xxhPrime64_1
	return acc
}

func xxh64MergeRound(acc, val uint64) uint64 {
	val = xxh64Round(0, val)
	acc ^= val
	acc = acc*xxhPrime64_1 + xxhPrime64_4
	return acc
}

func (s *xxh64State) CloneHash() hash.Hash {
	c := *s
	return &c
}

func (s *xxh64State) Sum(in []byte) []byte {
	h := s.digest()
	var b [8]byte
	binary.BigEndian.PutUint64(b[:], h)
	return append(in, b[:]...)
}

func (s *xxh64State) digest() uint64 {
	var h64 uint64
	if s.total >= 32 {
		h64 = bits.RotateLeft64(s.v1, 1) +
			bits.RotateLeft64(s.v2, 7) +
			bits.RotateLeft64(s.v3, 12) +
			bits.RotateLeft64(s.v4, 18)
		h64 = xxh64MergeRound(h64, s.v1)
		h64 = xxh64MergeRound(h64, s.v2)
		h64 = xxh64MergeRound(h64, s.v3)
		h64 = xxh64MergeRound(h64, s.v4)
	} else {
		h64 = s.seed + xxhPrime64_5
	}
	h64 += s.total

	tail := s.buf[:s.bufN]
	for len(tail) >= 8 {
		k1 := binary.LittleEndian.Uint64(tail)
		k1 = xxh64Round(0, k1)
		h64 ^= k1
		h64 = bits.RotateLeft64(h64, 27)*xxhPrime64_1 + xxhPrime64_4
		tail = tail[8:]
	}
	if len(tail) >= 4 {
		h64 ^= uint64(binary.LittleEndian.Uint32(tail)) * xxhPrime64_1
		h64 = bits.RotateLeft64(h64, 23)*xxhPrime64_2 + xxhPrime64_3
		tail = tail[4:]
	}
	for _, b := range tail {
		h64 ^= uint64(b) * xxhPrime64_5
		h64 = bits.RotateLeft64(h64, 11) * xxhPrime64_1
	}

	h64 ^= h64 >> 33
	h64 *= xxhPrime64_2
	h64 ^= h64 >> 29
	h64 *= xxhPrime64_3
	h64 ^= h64 >> 32
	return h64
}

// --- xxh3 (64-bit, seed-based) ---
// Simplified version matching PHP's implementation

// xxh3 secret - the default secret (136 bytes minimum)
var xxh3DefaultSecret = [192]byte{
	0xb8, 0xfe, 0x6c, 0x39, 0x23, 0xa4, 0x4b, 0xbe, 0x7c, 0x01, 0x81, 0x2c, 0xf7, 0x21, 0xad, 0x1c,
	0xde, 0xd4, 0x6d, 0xe9, 0x83, 0x90, 0x97, 0xdb, 0x72, 0x40, 0xa4, 0xa4, 0xb7, 0xb3, 0x67, 0x1f,
	0xcb, 0x79, 0xe6, 0x4e, 0xcc, 0xc0, 0xe5, 0x78, 0x82, 0x5a, 0xd0, 0x7d, 0xcc, 0xff, 0x72, 0x21,
	0xb8, 0x08, 0x46, 0x74, 0xf7, 0x43, 0x24, 0x8e, 0xe0, 0x35, 0x90, 0xe6, 0x81, 0x3a, 0x26, 0x4c,
	0x3c, 0x28, 0x52, 0xbb, 0x91, 0xc3, 0x00, 0xcb, 0x88, 0xd0, 0x65, 0x8b, 0x1b, 0x53, 0x2e, 0xa3,
	0x71, 0x64, 0x48, 0x97, 0xa2, 0x0d, 0xf9, 0x4e, 0x38, 0x19, 0xef, 0x46, 0xa9, 0xde, 0xac, 0xd8,
	0xa8, 0xfa, 0x76, 0x3f, 0xe3, 0x9c, 0x34, 0x3f, 0xf9, 0xdc, 0xbb, 0xc7, 0xc7, 0x0b, 0x4f, 0x1d,
	0x8a, 0x51, 0xe0, 0x4b, 0xcd, 0xb4, 0x59, 0x31, 0xc8, 0x9f, 0x7e, 0xc9, 0xd9, 0x78, 0x73, 0x64,
	0xea, 0xc5, 0xac, 0x83, 0x34, 0xd3, 0xeb, 0xc3, 0xc5, 0x81, 0xa0, 0xff, 0xfa, 0x13, 0x63, 0xeb,
	0x17, 0x0d, 0xdd, 0x51, 0xb7, 0xf0, 0xda, 0x49, 0xd3, 0x16, 0x55, 0x26, 0x29, 0xd4, 0x68, 0x9e,
	0x2b, 0x16, 0xbe, 0x58, 0x7d, 0x47, 0xa1, 0xfc, 0x8f, 0xf8, 0xb8, 0xd1, 0x7a, 0xd0, 0x31, 0xce,
	0x45, 0xcb, 0x3a, 0x8f, 0x95, 0x16, 0x04, 0x28, 0xaf, 0xd7, 0xfb, 0xca, 0xbb, 0x4b, 0x40, 0x7e,
}

type xxh3State struct {
	seed   uint64
	secret []byte
	acc    [8]uint64
	buf    [256]byte
	bufN   int
	total  uint64
}

func newXXH3WithSeed(seed uint64) hash.Hash {
	return newXXH3WithSeedOrSecret(seed, nil)
}

func newXXH3WithSeedOrSecret(seed uint64, secret []byte) hash.Hash {
	s := &xxh3State{seed: seed}
	if secret != nil {
		s.secret = make([]byte, len(secret))
		copy(s.secret, secret)
	}
	s.Reset()
	return s
}

func (s *xxh3State) getSecret() []byte {
	if len(s.secret) >= 136 {
		return s.secret
	}
	sec := xxh3DefaultSecret[:]
	if s.seed != 0 {
		// Derive secret from seed
		derived := make([]byte, 192)
		for i := 0; i < 12; i++ {
			lo := binary.LittleEndian.Uint64(sec[i*16:]) + s.seed
			hi := binary.LittleEndian.Uint64(sec[i*16+8:]) - s.seed
			binary.LittleEndian.PutUint64(derived[i*16:], lo)
			binary.LittleEndian.PutUint64(derived[i*16+8:], hi)
		}
		return derived
	}
	return sec
}

func (s *xxh3State) Reset() {
	secret := s.getSecret()
	s.acc[0] = uint64(xxhPrime32_3)
	s.acc[1] = xxhPrime64_1
	s.acc[2] = xxhPrime64_2
	s.acc[3] = xxhPrime64_3
	s.acc[4] = xxhPrime64_4
	s.acc[5] = uint64(xxhPrime32_2)
	s.acc[6] = xxhPrime64_5
	s.acc[7] = uint64(xxhPrime32_1)
	_ = secret
	s.bufN = 0
	s.total = 0
}

func (s *xxh3State) Size() int      { return 8 }
func (s *xxh3State) BlockSize() int { return 64 }

func (s *xxh3State) Write(p []byte) (int, error) {
	n := len(p)
	s.total += uint64(n)

	if s.bufN+len(p) < 256 {
		copy(s.buf[s.bufN:], p)
		s.bufN += len(p)
		return n, nil
	}

	if s.bufN > 0 {
		fill := 256 - s.bufN
		copy(s.buf[s.bufN:], p[:fill])
		p = p[fill:]
		s.bufN = 0
		s.processStripe(s.buf[:])
	}

	for len(p) >= 256 {
		s.processStripe(p[:256])
		p = p[256:]
	}

	if len(p) > 0 {
		copy(s.buf[:], p)
		s.bufN = len(p)
	}
	return n, nil
}

func (s *xxh3State) processStripe(data []byte) {
	secret := s.getSecret()
	for i := 0; i < 4; i++ {
		block := data[i*64:]
		for j := 0; j < 8; j++ {
			dataVal := binary.LittleEndian.Uint64(block[j*8:])
			secVal := binary.LittleEndian.Uint64(secret[j*8:])
			mixed := dataVal ^ secVal
			s.acc[j^1] += dataVal
			s.acc[j] += uint64(uint32(mixed)) * (mixed >> 32)
		}
	}
}

func xxh3Mix16B(data, secret []byte, seed uint64) uint64 {
	lo := binary.LittleEndian.Uint64(data)
	hi := binary.LittleEndian.Uint64(data[8:])
	slo := binary.LittleEndian.Uint64(secret) + seed
	shi := binary.LittleEndian.Uint64(secret[8:]) - seed
	return xxh3Mul128(lo^slo, hi^shi)
}

func xxh3Mul128(lo, hi uint64) uint64 {
	// lo * hi as 128-bit, return xor of high and low halves
	hi128, lo128 := bits.Mul64(lo, hi)
	return hi128 ^ lo128
}

func (s *xxh3State) CloneHash() hash.Hash {
	c := *s
	if s.secret != nil {
		c.secret = make([]byte, len(s.secret))
		copy(c.secret, s.secret)
	}
	return &c
}

func (s *xxh3State) Sum(in []byte) []byte {
	h := s.digest()
	var b [8]byte
	binary.BigEndian.PutUint64(b[:], h)
	return append(in, b[:]...)
}

func (s *xxh3State) digest() uint64 {
	secret := s.getSecret()
	seed := s.seed

	// For short inputs, use the default secret (or custom if provided) with seed passed to Mix16B.
	// The derived secret (from seed) is only correct for long-input streaming paths.
	// Using derived secret AND passing seed to Mix16B double-applies the seed.
	shortSecret := secret
	if len(s.secret) < 136 {
		// No custom secret provided: use default for short input paths
		shortSecret = xxh3DefaultSecret[:]
	}

	if s.total <= 16 {
		return xxh3Len0to16(s.buf[:s.bufN], shortSecret, seed, s.total)
	} else if s.total <= 128 {
		return xxh3Len17to128(s.buf[:s.bufN], shortSecret, seed, s.total)
	} else if s.total <= 240 {
		return xxh3Len129to240(s.buf[:s.bufN], shortSecret, seed, s.total)
	}

	// Long input: use accumulated state
	acc := s.acc
	// Process remaining buffer
	if s.bufN > 0 {
		// Fill last stripe
		buf := make([]byte, 256)
		copy(buf, s.buf[:s.bufN])
		// last stripe
		last := s.bufN
		if last > 64 {
			last = ((last + 63) / 64) * 64
		}
		// scramble
		for j := 0; j < 8; j++ {
			dataVal := binary.LittleEndian.Uint64(buf[j*8:])
			secVal := binary.LittleEndian.Uint64(secret[j*8:])
			mixed := dataVal ^ secVal
			acc[j^1] += dataVal
			acc[j] += uint64(uint32(mixed)) * (mixed >> 32)
		}
	}

	// Final merge
	h64 := s.total*xxhPrime64_1 + uint64(uint32(s.total>>32))

	for i := 0; i < 8; i++ {
		h64 = xxh3Avalanche(h64 ^ xxh3Mix16B(
			func() []byte {
				b := make([]byte, 16)
				binary.LittleEndian.PutUint64(b, acc[i])
				binary.LittleEndian.PutUint64(b[8:], 0)
				return b
			}(),
			secret[64+i*8:],
			seed,
		))
	}

	return xxh3Avalanche(h64)
}

func xxh3Len0to16(input, secret []byte, seed, total uint64) uint64 {
	if total > 8 {
		// 9 to 16
		lo := binary.LittleEndian.Uint64(input) ^ (binary.LittleEndian.Uint64(secret[24:]) + seed)
		hi := binary.LittleEndian.Uint64(input[total-8:]) ^ (binary.LittleEndian.Uint64(secret[32:]) - seed)
		acc := total + bits.ReverseBytes64(lo) + hi + xxh3Mul128(lo, hi)
		return xxh3Avalanche(acc)
	} else if total >= 4 {
		// 4 to 8
		lo := uint64(binary.LittleEndian.Uint32(input))
		hi := uint64(binary.LittleEndian.Uint32(input[total-4:]))
		input64 := lo | (hi << 32)
		keyed := input64 ^ (binary.LittleEndian.Uint64(secret[8:]) + seed)
		mixed := keyed * xxhPrime64_1
		mixed = bits.RotateLeft64(mixed, -49)
		mixed *= xxhPrime64_2
		mixed ^= mixed >> 35
		mixed *= xxhPrime64_3
		mixed ^= mixed >> 28
		return mixed
	} else if total > 0 {
		// 1 to 3
		c1 := uint32(input[0])
		c2 := uint32(input[total>>1])
		c3 := uint32(input[total-1])
		combined := uint32(c1<<16) | uint32(c2<<24) | c3 | uint32(total<<8)
		lo := uint64(combined) ^ uint64(binary.LittleEndian.Uint32(secret)+uint32(seed))
		hi := uint64(bits.ReverseBytes32(combined)) ^ uint64(binary.LittleEndian.Uint32(secret[4:])-uint32(seed))
		h64 := (lo + hi) * xxhPrime64_5
		return xxh64Avalanche(h64)
	}
	// empty
	h64 := (binary.LittleEndian.Uint64(secret[56:]) + seed) ^ (binary.LittleEndian.Uint64(secret[64:]) - seed)
	return xxh64Avalanche(h64)
}

func xxh64Avalanche(h64 uint64) uint64 {
	h64 ^= h64 >> 33
	h64 *= xxhPrime64_2
	h64 ^= h64 >> 29
	h64 *= xxhPrime64_3
	h64 ^= h64 >> 32
	return h64
}

func xxh3Len17to128(input, secret []byte, seed, total uint64) uint64 {
	acc := total * xxhPrime64_1
	if total > 96 {
		acc += xxh3Mix16B(input[48:], secret[96:], seed)
		acc += xxh3Mix16B(input[total-64:], secret[112:], seed)
	}
	if total > 64 {
		acc += xxh3Mix16B(input[32:], secret[64:], seed)
		acc += xxh3Mix16B(input[total-48:], secret[80:], seed)
	}
	if total > 32 {
		acc += xxh3Mix16B(input[16:], secret[32:], seed)
		acc += xxh3Mix16B(input[total-32:], secret[48:], seed)
	}
	acc += xxh3Mix16B(input, secret[0:], seed)
	acc += xxh3Mix16B(input[total-16:], secret[16:], seed)
	return xxh3Avalanche(acc)
}

func xxh3Len129to240(input, secret []byte, seed, total uint64) uint64 {
	acc := total * xxhPrime64_1
	nbRounds := int(total / 16)
	for i := 0; i < 8; i++ {
		acc += xxh3Mix16B(input[16*i:], secret[16*i:], seed)
	}
	acc = xxh3Avalanche(acc)
	for i := 8; i < nbRounds; i++ {
		acc += xxh3Mix16B(input[16*i:], secret[16*(i-8)+3:], seed)
	}
	acc += xxh3Mix16B(input[total-16:], secret[136-17:], seed)
	return xxh3Avalanche(acc)
}

func xxh3Avalanche(h64 uint64) uint64 {
	h64 ^= h64 >> 37
	h64 *= 0x165667919E3779F9
	h64 ^= h64 >> 32
	return h64
}

// --- xxh128 ---

type xxh128State struct {
	s *xxh3State
}

func newXXH128WithSeed(seed uint64) hash.Hash {
	return &xxh128State{s: newXXH3WithSeedOrSecret(seed, nil).(*xxh3State)}
}

func newXXH128WithSeedOrSecret(seed uint64, secret []byte) hash.Hash {
	return &xxh128State{s: newXXH3WithSeedOrSecret(seed, secret).(*xxh3State)}
}

func (s *xxh128State) Reset()         { s.s.Reset() }
func (s *xxh128State) Size() int      { return 16 }
func (s *xxh128State) BlockSize() int { return 64 }

func (s *xxh128State) Write(p []byte) (int, error) {
	return s.s.Write(p)
}

func (s *xxh128State) CloneHash() hash.Hash {
	cloned := s.s.CloneHash().(*xxh3State)
	return &xxh128State{s: cloned}
}

func (s *xxh128State) Sum(in []byte) []byte {
	lo, hi := s.digest()
	var b [16]byte
	// Canonical format: high64 first, then low64 (both big-endian)
	binary.BigEndian.PutUint64(b[0:], hi)
	binary.BigEndian.PutUint64(b[8:], lo)
	return append(in, b[:]...)
}

func (s *xxh128State) digest() (uint64, uint64) {
	st := s.s
	secret := st.getSecret()
	seed := st.seed

	// For short inputs, use the default secret (or custom if provided) with seed passed to Mix16B.
	shortSecret := secret
	if len(st.secret) < 136 {
		shortSecret = xxh3DefaultSecret[:]
	}

	if st.total <= 16 {
		return xxh3Len0to16_128(st.buf[:st.bufN], shortSecret, seed, st.total)
	} else if st.total <= 128 {
		return xxh3Len17to128_128(st.buf[:st.bufN], shortSecret, seed, st.total)
	} else if st.total <= 240 {
		return xxh3Len129to240_128(st.buf[:st.bufN], shortSecret, seed, st.total)
	}

	// Long: use accumulated state
	acc := st.acc
	lo := xxh3MergeAccs(acc[:], secret[11:], st.total*xxhPrime64_1)
	hi := xxh3MergeAccs(acc[:], secret[len(secret)-64:], ^(st.total*xxhPrime64_2))
	return lo, hi
}

func xxh3MergeAccs(acc []uint64, secret []byte, start uint64) uint64 {
	result := start
	for i := 0; i < 4; i++ {
		result ^= xxh64MergeRound(acc[2*i], binary.LittleEndian.Uint64(secret[i*16:]))
		result ^= xxh64MergeRound(acc[2*i+1], binary.LittleEndian.Uint64(secret[i*16+8:]))
	}
	return xxh3Avalanche(result)
}

func xxh3Len0to16_128(input, secret []byte, seed, total uint64) (uint64, uint64) {
	if total > 8 {
		lo := binary.LittleEndian.Uint64(input) ^ (binary.LittleEndian.Uint64(secret[24:]) + seed)
		hi := binary.LittleEndian.Uint64(input[total-8:]) ^ (binary.LittleEndian.Uint64(secret[32:]) - seed)
		hihi, hilo := bits.Mul64(lo, hi)
		loAcc := total + bits.ReverseBytes64(lo) + hi + hilo
		hiAcc := ^(total + bits.ReverseBytes64(hi) + lo + hihi)
		return xxh3Avalanche(loAcc), xxh3Avalanche(hiAcc)
	} else if total >= 4 {
		lo := uint64(binary.LittleEndian.Uint32(input))
		hi := uint64(binary.LittleEndian.Uint32(input[total-4:]))
		input64 := lo | (hi << 32)
		lo64 := input64 ^ (binary.LittleEndian.Uint64(secret[8:]) + seed)
		hi64 := input64 ^ (binary.LittleEndian.Uint64(secret[16:]) + seed)
		loAcc := xxh64Avalanche(lo64)
		hiAcc := xxh64Avalanche(hi64)
		return loAcc, hiAcc
	} else if total > 0 {
		c1 := uint32(input[0])
		c2 := uint32(input[total>>1])
		c3 := uint32(input[total-1])
		combined := uint32(c1<<16) | uint32(c2<<24) | c3 | uint32(total<<8)
		lo64 := uint64(combined) ^ uint64(binary.LittleEndian.Uint32(secret)+uint32(seed))
		hi64 := uint64(bits.ReverseBytes32(combined)) ^ uint64(binary.LittleEndian.Uint32(secret[4:])-uint32(seed))
		loAcc := xxh64Avalanche((lo64 + hi64) * xxhPrime64_5)
		hiAcc := xxh64Avalanche((lo64+hi64)*xxhPrime64_5 ^ xxhPrime64_3)
		return loAcc, hiAcc
	}
	lo := xxh64Avalanche(binary.LittleEndian.Uint64(secret[64:]) + seed)
	hi := xxh64Avalanche(binary.LittleEndian.Uint64(secret[72:]) - seed)
	return lo, hi
}

// xxh128mix32B implements XXH128_mix32B from the reference:
// acc.low64  += XXH3_mix16B(input_1, secret+0, seed)
// acc.low64  ^= readLE64(input_2) + readLE64(input_2+8)
// acc.high64 += XXH3_mix16B(input_2, secret+16, seed)
// acc.high64 ^= readLE64(input_1) + readLE64(input_1+8)
func xxh128mix32B(accLo, accHi uint64, in1, in2, secret []byte, seed uint64) (uint64, uint64) {
	accLo += xxh3Mix16B(in1, secret, seed)
	accLo ^= binary.LittleEndian.Uint64(in2) + binary.LittleEndian.Uint64(in2[8:])
	accHi += xxh3Mix16B(in2, secret[16:], seed)
	accHi ^= binary.LittleEndian.Uint64(in1) + binary.LittleEndian.Uint64(in1[8:])
	return accLo, accHi
}

func xxh3Len17to128_128(input, secret []byte, seed, total uint64) (uint64, uint64) {
	// Reference: XXH3_len_17to128_128b
	accLo := total * xxhPrime64_1
	accHi := uint64(0)
	if total > 32 {
		if total > 64 {
			if total > 96 {
				accLo, accHi = xxh128mix32B(accLo, accHi, input[48:], input[total-64:], secret[96:], seed)
			}
			accLo, accHi = xxh128mix32B(accLo, accHi, input[32:], input[total-48:], secret[64:], seed)
		}
		accLo, accHi = xxh128mix32B(accLo, accHi, input[16:], input[total-32:], secret[32:], seed)
	}
	accLo, accHi = xxh128mix32B(accLo, accHi, input, input[total-16:], secret, seed)
	// h128.low64  = acc.low64 + acc.high64
	// h128.high64 = (acc.low64 * PRIME64_1) + (acc.high64 * PRIME64_4) + ((len-seed) * PRIME64_2)
	// h128.low64  = XXH3_avalanche(h128.low64)
	// h128.high64 = (uint64_t)0 - XXH3_avalanche(h128.high64)  [two's complement negation]
	low64 := xxh3Avalanche(accLo + accHi)
	high64 := -xxh3Avalanche(accLo*xxhPrime64_1 + accHi*xxhPrime64_4 + (total-seed)*xxhPrime64_2)
	return low64, high64
}

func xxh3Len129to240_128(input, secret []byte, seed, total uint64) (uint64, uint64) {
	nbRounds := int(total / 32)
	acc0, acc1 := uint64(total)*xxhPrime64_1, ^uint64(total)*xxhPrime64_2
	for i := 0; i < 4; i++ {
		acc0 ^= xxh3Mix16B(input[32*i:], secret[32*i:], seed)
		acc0 ^= xxh3Mix16B(input[32*i+16:], secret[32*i+16:], seed)
		acc1 ^= xxh3Mix16B(input[32*i:], secret[32*i+16:], seed)
		acc1 ^= xxh3Mix16B(input[32*i+16:], secret[32*i:], seed)
	}
	acc0 = xxh3Avalanche(acc0)
	acc1 = xxh3Avalanche(acc1)
	for i := 4; i < nbRounds; i++ {
		acc0 ^= xxh3Mix16B(input[32*i:], secret[32*(i-4)+3:], seed)
		acc0 ^= xxh3Mix16B(input[32*i+16:], secret[32*(i-4)+19:], seed)
		acc1 ^= xxh3Mix16B(input[32*i:], secret[32*(i-4)+19:], seed)
		acc1 ^= xxh3Mix16B(input[32*i+16:], secret[32*(i-4)+3:], seed)
	}
	acc0 ^= xxh3Mix16B(input[total-16:], secret[136-17:], seed)
	acc0 ^= xxh3Mix16B(input[total-32:], secret[136-33:], seed)
	acc1 ^= xxh3Mix16B(input[total-16:], secret[136-33:], seed)
	acc1 ^= xxh3Mix16B(input[total-32:], secret[136-17:], seed)
	lo := xxh3Avalanche(acc0 + acc1)
	hi := ^xxh3Avalanche(acc0*xxhPrime64_4 + acc1*xxhPrime64_2 + (total-seed)*xxhPrime64_3)
	return lo, hi
}
