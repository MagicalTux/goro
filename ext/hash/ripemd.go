package hash

// RIPEMD-128, RIPEMD-256, RIPEMD-320 implementations
// Reference: https://homes.esat.kuleuven.be/~cosicart/pdf/AB-9601/AB-9601.pdf

import (
	"encoding/binary"
	"hash"
	"math/bits"
)

// ---- Common RIPEMD helpers ----

func ripemdF(x, y, z uint32) uint32 { return x ^ y ^ z }
func ripemdG(x, y, z uint32) uint32 { return (x & y) | (^x & z) }
func ripemdH(x, y, z uint32) uint32 { return (x | ^y) ^ z }
func ripemdI(x, y, z uint32) uint32 { return (x & z) | (y & ^z) }
func ripemdJ(x, y, z uint32) uint32 { return x ^ (y | ^z) }

// Round constants for RIPEMD
var ripemdKL = [5]uint32{0x00000000, 0x5A827999, 0x6ED9EBA1, 0x8F1BBCDC, 0xA953FD4E}
var ripemdKR = [5]uint32{0x50A28BE6, 0x5C4DD124, 0x6D703EF3, 0x7A6D76E9, 0x00000000}

// Round constants for RIPEMD-128/256 right rounds (only 4 rounds, no 0x7A6D76E9)
var ripemdKR128 = [4]uint32{0x50A28BE6, 0x5C4DD124, 0x6D703EF3, 0x00000000}

// Message schedule indices
var ripemdRL = [80]uint32{
	0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15,
	7, 4, 13, 1, 10, 6, 15, 3, 12, 0, 9, 5, 2, 14, 11, 8,
	3, 10, 14, 4, 9, 15, 8, 1, 2, 7, 0, 6, 13, 11, 5, 12,
	1, 9, 11, 10, 0, 8, 12, 4, 13, 3, 7, 15, 14, 5, 6, 2,
	4, 0, 5, 9, 7, 12, 2, 10, 14, 1, 3, 8, 11, 6, 15, 13,
}
var ripemdRR = [80]uint32{
	5, 14, 7, 0, 9, 2, 11, 4, 13, 6, 15, 8, 1, 10, 3, 12,
	6, 11, 3, 7, 0, 13, 5, 10, 14, 15, 8, 12, 4, 9, 1, 2,
	15, 5, 1, 3, 7, 14, 6, 9, 11, 8, 12, 2, 10, 0, 4, 13,
	8, 6, 4, 1, 3, 11, 15, 0, 5, 12, 2, 13, 9, 7, 10, 14,
	12, 15, 10, 4, 1, 5, 8, 7, 6, 2, 13, 14, 0, 3, 9, 11,
}

// Shift amounts
var ripemdSL = [80]uint32{
	11, 14, 15, 12, 5, 8, 7, 9, 11, 13, 14, 15, 6, 7, 9, 8,
	7, 6, 8, 13, 11, 9, 7, 15, 7, 12, 15, 9, 11, 7, 13, 12,
	11, 13, 6, 7, 14, 9, 13, 15, 14, 8, 13, 6, 5, 12, 7, 5,
	11, 12, 14, 15, 14, 15, 9, 8, 9, 14, 5, 6, 8, 6, 5, 12,
	9, 15, 5, 11, 6, 8, 13, 12, 5, 12, 13, 14, 11, 8, 5, 6,
}
var ripemdSR = [80]uint32{
	8, 9, 9, 11, 13, 15, 15, 5, 7, 7, 8, 11, 14, 14, 12, 6,
	9, 13, 15, 7, 12, 8, 9, 11, 7, 7, 12, 7, 6, 15, 13, 11,
	9, 7, 15, 11, 8, 6, 6, 14, 12, 13, 5, 14, 13, 13, 7, 5,
	15, 5, 8, 11, 14, 14, 6, 14, 6, 9, 12, 9, 12, 5, 15, 8,
	8, 5, 12, 9, 12, 5, 14, 6, 8, 13, 6, 5, 15, 13, 11, 11,
}

// ---- RIPEMD-128 ----

const ripemd128Size = 16
const ripemd128BlockSize = 64

type ripemd128Digest struct {
	s   [4]uint32
	x   [ripemd128BlockSize]byte
	nx  int
	len uint64
}

func newRipemd128() hash.Hash {
	d := new(ripemd128Digest)
	d.Reset()
	return d
}

func (d *ripemd128Digest) Reset() {
	d.s[0] = 0x67452301
	d.s[1] = 0xEFCDAB89
	d.s[2] = 0x98BADCFE
	d.s[3] = 0x10325476
	d.nx = 0
	d.len = 0
}

func (d *ripemd128Digest) Size() int      { return ripemd128Size }
func (d *ripemd128Digest) BlockSize() int { return ripemd128BlockSize }

func (d *ripemd128Digest) Write(p []byte) (nn int, err error) {
	nn = len(p)
	d.len += uint64(nn)
	if d.nx > 0 {
		n := copy(d.x[d.nx:], p)
		d.nx += n
		if d.nx == ripemd128BlockSize {
			ripemd128Block(d, d.x[:])
			d.nx = 0
		}
		p = p[n:]
	}
	for len(p) >= ripemd128BlockSize {
		ripemd128Block(d, p[:ripemd128BlockSize])
		p = p[ripemd128BlockSize:]
	}
	if len(p) > 0 {
		d.nx = copy(d.x[:], p)
	}
	return
}

func (d *ripemd128Digest) CloneHash() hash.Hash {
	c := *d
	return &c
}

func (d *ripemd128Digest) Sum(in []byte) []byte {
	d0 := *d
	hash := d0.checkSum()
	return append(in, hash[:]...)
}

func (d *ripemd128Digest) checkSum() [ripemd128Size]byte {
	tc := d.len
	var tmp [64]byte
	tmp[0] = 0x80
	if tc%64 < 56 {
		d.Write(tmp[0 : 56-tc%64])
	} else {
		d.Write(tmp[0 : 64+56-tc%64])
	}
	tc <<= 3
	for i := uint(0); i < 8; i++ {
		tmp[i] = byte(tc >> (8 * i))
	}
	d.Write(tmp[0:8])

	var digest [ripemd128Size]byte
	for i, s := range d.s {
		digest[i*4] = byte(s)
		digest[i*4+1] = byte(s >> 8)
		digest[i*4+2] = byte(s >> 16)
		digest[i*4+3] = byte(s >> 24)
	}
	return digest
}

func ripemd128Block(dig *ripemd128Digest, p []byte) {
	var X [16]uint32
	for i := 0; i < 16; i++ {
		X[i] = binary.LittleEndian.Uint32(p[i*4:])
	}

	a, b, c, d := dig.s[0], dig.s[1], dig.s[2], dig.s[3]
	aa, bb, cc, dd := a, b, c, d

	// Left rounds
	for i := 0; i < 16; i++ {
		T := bits.RotateLeft32(a+ripemdF(b, c, d)+X[ripemdRL[i]]+ripemdKL[0], int(ripemdSL[i]))
		a, d, c, b = d, c, b, T
	}
	for i := 16; i < 32; i++ {
		T := bits.RotateLeft32(a+ripemdG(b, c, d)+X[ripemdRL[i]]+ripemdKL[1], int(ripemdSL[i]))
		a, d, c, b = d, c, b, T
	}
	for i := 32; i < 48; i++ {
		T := bits.RotateLeft32(a+ripemdH(b, c, d)+X[ripemdRL[i]]+ripemdKL[2], int(ripemdSL[i]))
		a, d, c, b = d, c, b, T
	}
	for i := 48; i < 64; i++ {
		T := bits.RotateLeft32(a+ripemdI(b, c, d)+X[ripemdRL[i]]+ripemdKL[3], int(ripemdSL[i]))
		a, d, c, b = d, c, b, T
	}

	// Right rounds
	for i := 0; i < 16; i++ {
		T := bits.RotateLeft32(aa+ripemdI(bb, cc, dd)+X[ripemdRR[i]]+ripemdKR128[0], int(ripemdSR[i]))
		aa, dd, cc, bb = dd, cc, bb, T
	}
	for i := 16; i < 32; i++ {
		T := bits.RotateLeft32(aa+ripemdH(bb, cc, dd)+X[ripemdRR[i]]+ripemdKR128[1], int(ripemdSR[i]))
		aa, dd, cc, bb = dd, cc, bb, T
	}
	for i := 32; i < 48; i++ {
		T := bits.RotateLeft32(aa+ripemdG(bb, cc, dd)+X[ripemdRR[i]]+ripemdKR128[2], int(ripemdSR[i]))
		aa, dd, cc, bb = dd, cc, bb, T
	}
	for i := 48; i < 64; i++ {
		T := bits.RotateLeft32(aa+ripemdF(bb, cc, dd)+X[ripemdRR[i]]+ripemdKR128[3], int(ripemdSR[i]))
		aa, dd, cc, bb = dd, cc, bb, T
	}

	T := dig.s[1] + c + dd
	dig.s[1] = dig.s[2] + d + aa
	dig.s[2] = dig.s[3] + a + bb
	dig.s[3] = dig.s[0] + b + cc
	dig.s[0] = T
}

// ---- RIPEMD-256 ----

const ripemd256Size = 32

type ripemd256Digest struct {
	s   [8]uint32
	x   [ripemd128BlockSize]byte
	nx  int
	len uint64
}

func newRipemd256() hash.Hash {
	d := new(ripemd256Digest)
	d.Reset()
	return d
}

func (d *ripemd256Digest) Reset() {
	d.s[0] = 0x67452301
	d.s[1] = 0xEFCDAB89
	d.s[2] = 0x98BADCFE
	d.s[3] = 0x10325476
	d.s[4] = 0x76543210
	d.s[5] = 0xFEDCBA98
	d.s[6] = 0x89ABCDEF
	d.s[7] = 0x01234567
	d.nx = 0
	d.len = 0
}

func (d *ripemd256Digest) Size() int      { return ripemd256Size }
func (d *ripemd256Digest) BlockSize() int { return ripemd128BlockSize }

func (d *ripemd256Digest) Write(p []byte) (nn int, err error) {
	nn = len(p)
	d.len += uint64(nn)
	if d.nx > 0 {
		n := copy(d.x[d.nx:], p)
		d.nx += n
		if d.nx == ripemd128BlockSize {
			ripemd256Block(d, d.x[:])
			d.nx = 0
		}
		p = p[n:]
	}
	for len(p) >= ripemd128BlockSize {
		ripemd256Block(d, p[:ripemd128BlockSize])
		p = p[ripemd128BlockSize:]
	}
	if len(p) > 0 {
		d.nx = copy(d.x[:], p)
	}
	return
}

func (d *ripemd256Digest) CloneHash() hash.Hash {
	c := *d
	return &c
}

func (d *ripemd256Digest) Sum(in []byte) []byte {
	d0 := *d
	hash := d0.checkSum()
	return append(in, hash[:]...)
}

func (d *ripemd256Digest) checkSum() [ripemd256Size]byte {
	tc := d.len
	var tmp [64]byte
	tmp[0] = 0x80
	if tc%64 < 56 {
		d.Write(tmp[0 : 56-tc%64])
	} else {
		d.Write(tmp[0 : 64+56-tc%64])
	}
	tc <<= 3
	for i := uint(0); i < 8; i++ {
		tmp[i] = byte(tc >> (8 * i))
	}
	d.Write(tmp[0:8])

	var digest [ripemd256Size]byte
	for i, s := range d.s {
		digest[i*4] = byte(s)
		digest[i*4+1] = byte(s >> 8)
		digest[i*4+2] = byte(s >> 16)
		digest[i*4+3] = byte(s >> 24)
	}
	return digest
}

func ripemd256Block(dig *ripemd256Digest, p []byte) {
	var X [16]uint32
	for i := 0; i < 16; i++ {
		X[i] = binary.LittleEndian.Uint32(p[i*4:])
	}

	a, b, c, d := dig.s[0], dig.s[1], dig.s[2], dig.s[3]
	aa, bb, cc, dd := dig.s[4], dig.s[5], dig.s[6], dig.s[7]

	// Round 1 left, right
	for i := 0; i < 16; i++ {
		T := bits.RotateLeft32(a+ripemdF(b, c, d)+X[ripemdRL[i]]+ripemdKL[0], int(ripemdSL[i]))
		a, d, c, b = d, c, b, T
	}
	for i := 0; i < 16; i++ {
		T := bits.RotateLeft32(aa+ripemdI(bb, cc, dd)+X[ripemdRR[i]]+ripemdKR128[0], int(ripemdSR[i]))
		aa, dd, cc, bb = dd, cc, bb, T
	}
	a, aa = aa, a

	for i := 16; i < 32; i++ {
		T := bits.RotateLeft32(a+ripemdG(b, c, d)+X[ripemdRL[i]]+ripemdKL[1], int(ripemdSL[i]))
		a, d, c, b = d, c, b, T
	}
	for i := 16; i < 32; i++ {
		T := bits.RotateLeft32(aa+ripemdH(bb, cc, dd)+X[ripemdRR[i]]+ripemdKR128[1], int(ripemdSR[i]))
		aa, dd, cc, bb = dd, cc, bb, T
	}
	b, bb = bb, b

	for i := 32; i < 48; i++ {
		T := bits.RotateLeft32(a+ripemdH(b, c, d)+X[ripemdRL[i]]+ripemdKL[2], int(ripemdSL[i]))
		a, d, c, b = d, c, b, T
	}
	for i := 32; i < 48; i++ {
		T := bits.RotateLeft32(aa+ripemdG(bb, cc, dd)+X[ripemdRR[i]]+ripemdKR128[2], int(ripemdSR[i]))
		aa, dd, cc, bb = dd, cc, bb, T
	}
	c, cc = cc, c

	for i := 48; i < 64; i++ {
		T := bits.RotateLeft32(a+ripemdI(b, c, d)+X[ripemdRL[i]]+ripemdKL[3], int(ripemdSL[i]))
		a, d, c, b = d, c, b, T
	}
	for i := 48; i < 64; i++ {
		T := bits.RotateLeft32(aa+ripemdF(bb, cc, dd)+X[ripemdRR[i]]+ripemdKR128[3], int(ripemdSR[i]))
		aa, dd, cc, bb = dd, cc, bb, T
	}
	d, dd = dd, d

	dig.s[0] += a
	dig.s[1] += b
	dig.s[2] += c
	dig.s[3] += d
	dig.s[4] += aa
	dig.s[5] += bb
	dig.s[6] += cc
	dig.s[7] += dd
}

// ---- RIPEMD-320 ----

const ripemd320Size = 40
const ripemd320BlockSize = 64

type ripemd320Digest struct {
	s   [10]uint32
	x   [ripemd320BlockSize]byte
	nx  int
	len uint64
}

func newRipemd320() hash.Hash {
	d := new(ripemd320Digest)
	d.Reset()
	return d
}

func (d *ripemd320Digest) Reset() {
	d.s[0] = 0x67452301
	d.s[1] = 0xEFCDAB89
	d.s[2] = 0x98BADCFE
	d.s[3] = 0x10325476
	d.s[4] = 0xC3D2E1F0
	d.s[5] = 0x76543210
	d.s[6] = 0xFEDCBA98
	d.s[7] = 0x89ABCDEF
	d.s[8] = 0x01234567
	d.s[9] = 0x3C2D1E0F
	d.nx = 0
	d.len = 0
}

func (d *ripemd320Digest) Size() int      { return ripemd320Size }
func (d *ripemd320Digest) BlockSize() int { return ripemd320BlockSize }

func (d *ripemd320Digest) Write(p []byte) (nn int, err error) {
	nn = len(p)
	d.len += uint64(nn)
	if d.nx > 0 {
		n := copy(d.x[d.nx:], p)
		d.nx += n
		if d.nx == ripemd320BlockSize {
			ripemd320Block(d, d.x[:])
			d.nx = 0
		}
		p = p[n:]
	}
	for len(p) >= ripemd320BlockSize {
		ripemd320Block(d, p[:ripemd320BlockSize])
		p = p[ripemd320BlockSize:]
	}
	if len(p) > 0 {
		d.nx = copy(d.x[:], p)
	}
	return
}

func (d *ripemd320Digest) CloneHash() hash.Hash {
	c := *d
	return &c
}

func (d *ripemd320Digest) Sum(in []byte) []byte {
	d0 := *d
	hash := d0.checkSum()
	return append(in, hash[:]...)
}

func (d *ripemd320Digest) checkSum() [ripemd320Size]byte {
	tc := d.len
	var tmp [64]byte
	tmp[0] = 0x80
	if tc%64 < 56 {
		d.Write(tmp[0 : 56-tc%64])
	} else {
		d.Write(tmp[0 : 64+56-tc%64])
	}
	tc <<= 3
	for i := uint(0); i < 8; i++ {
		tmp[i] = byte(tc >> (8 * i))
	}
	d.Write(tmp[0:8])

	var digest [ripemd320Size]byte
	for i, s := range d.s {
		digest[i*4] = byte(s)
		digest[i*4+1] = byte(s >> 8)
		digest[i*4+2] = byte(s >> 16)
		digest[i*4+3] = byte(s >> 24)
	}
	return digest
}

// Round constants for RIPEMD-160/320 left
var ripemd320KL = [5]uint32{0x00000000, 0x5A827999, 0x6ED9EBA1, 0x8F1BBCDC, 0xA953FD4E}

// Round constants for RIPEMD-160/320 right
var ripemd320KR = [5]uint32{0x50A28BE6, 0x5C4DD124, 0x6D703EF3, 0x7A6D76E9, 0x00000000}

func ripemd320Block(dig *ripemd320Digest, p []byte) {
	var X [16]uint32
	for i := 0; i < 16; i++ {
		X[i] = binary.LittleEndian.Uint32(p[i*4:])
	}

	a, b, c, d, e := dig.s[0], dig.s[1], dig.s[2], dig.s[3], dig.s[4]
	aa, bb, cc, dd, ee := dig.s[5], dig.s[6], dig.s[7], dig.s[8], dig.s[9]

	// Round 1 left
	for i := 0; i < 16; i++ {
		T := bits.RotateLeft32(a+ripemdF(b, c, d)+X[ripemdRL[i]]+ripemd320KL[0], int(ripemdSL[i])) + e
		a, e, d, c, b = e, d, bits.RotateLeft32(c, 10), b, T
	}
	// Round 1 right
	for i := 0; i < 16; i++ {
		T := bits.RotateLeft32(aa+ripemdJ(bb, cc, dd)+X[ripemdRR[i]]+ripemd320KR[0], int(ripemdSR[i])) + ee
		aa, ee, dd, cc, bb = ee, dd, bits.RotateLeft32(cc, 10), bb, T
	}
	b, bb = bb, b

	// Round 2 left
	for i := 16; i < 32; i++ {
		T := bits.RotateLeft32(a+ripemdG(b, c, d)+X[ripemdRL[i]]+ripemd320KL[1], int(ripemdSL[i])) + e
		a, e, d, c, b = e, d, bits.RotateLeft32(c, 10), b, T
	}
	// Round 2 right
	for i := 16; i < 32; i++ {
		T := bits.RotateLeft32(aa+ripemdI(bb, cc, dd)+X[ripemdRR[i]]+ripemd320KR[1], int(ripemdSR[i])) + ee
		aa, ee, dd, cc, bb = ee, dd, bits.RotateLeft32(cc, 10), bb, T
	}
	d, dd = dd, d

	// Round 3 left
	for i := 32; i < 48; i++ {
		T := bits.RotateLeft32(a+ripemdH(b, c, d)+X[ripemdRL[i]]+ripemd320KL[2], int(ripemdSL[i])) + e
		a, e, d, c, b = e, d, bits.RotateLeft32(c, 10), b, T
	}
	// Round 3 right
	for i := 32; i < 48; i++ {
		T := bits.RotateLeft32(aa+ripemdH(bb, cc, dd)+X[ripemdRR[i]]+ripemd320KR[2], int(ripemdSR[i])) + ee
		aa, ee, dd, cc, bb = ee, dd, bits.RotateLeft32(cc, 10), bb, T
	}
	a, aa = aa, a

	// Round 4 left
	for i := 48; i < 64; i++ {
		T := bits.RotateLeft32(a+ripemdI(b, c, d)+X[ripemdRL[i]]+ripemd320KL[3], int(ripemdSL[i])) + e
		a, e, d, c, b = e, d, bits.RotateLeft32(c, 10), b, T
	}
	// Round 4 right
	for i := 48; i < 64; i++ {
		T := bits.RotateLeft32(aa+ripemdG(bb, cc, dd)+X[ripemdRR[i]]+ripemd320KR[3], int(ripemdSR[i])) + ee
		aa, ee, dd, cc, bb = ee, dd, bits.RotateLeft32(cc, 10), bb, T
	}
	c, cc = cc, c

	// Round 5 left
	for i := 64; i < 80; i++ {
		T := bits.RotateLeft32(a+ripemdJ(b, c, d)+X[ripemdRL[i]]+ripemd320KL[4], int(ripemdSL[i])) + e
		a, e, d, c, b = e, d, bits.RotateLeft32(c, 10), b, T
	}
	// Round 5 right
	for i := 64; i < 80; i++ {
		T := bits.RotateLeft32(aa+ripemdF(bb, cc, dd)+X[ripemdRR[i]]+ripemd320KR[4], int(ripemdSR[i])) + ee
		aa, ee, dd, cc, bb = ee, dd, bits.RotateLeft32(cc, 10), bb, T
	}
	e, ee = ee, e

	dig.s[0] += a
	dig.s[1] += b
	dig.s[2] += c
	dig.s[3] += d
	dig.s[4] += e
	dig.s[5] += aa
	dig.s[6] += bb
	dig.s[7] += cc
	dig.s[8] += dd
	dig.s[9] += ee
}
