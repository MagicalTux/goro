package hash

// Whirlpool hash algorithm implementation
// Ported from PHP's hash_whirlpool.c
// Reference: https://www.larc.usp.br/~pbarreto/whirlpool.zip

import (
	"encoding/binary"
	"hash"
)

const whirlpoolDigestSize = 64
const whirlpoolBlockSize = 64

type whirlpoolDigest struct {
	state  [8]uint64
	buf    [whirlpoolBlockSize]byte
	bufLen int
	// bit length counter: 256-bit value stored as 4 x uint64 (little endian)
	bitLen [4]uint64
}

func newWhirlpool() hash.Hash {
	return &whirlpoolDigest{}
}

func (d *whirlpoolDigest) Reset() {
	*d = whirlpoolDigest{}
}

func (d *whirlpoolDigest) BlockSize() int { return whirlpoolBlockSize }
func (d *whirlpoolDigest) Size() int      { return whirlpoolDigestSize }

func (d *whirlpoolDigest) Write(p []byte) (n int, err error) {
	n = len(p)
	// Update bit length counter
	bits := uint64(len(p)) * 8
	prev := d.bitLen[0]
	d.bitLen[0] += bits
	if d.bitLen[0] < prev { // overflow
		prev2 := d.bitLen[1]
		d.bitLen[1]++
		if d.bitLen[1] < prev2 {
			prev3 := d.bitLen[2]
			d.bitLen[2]++
			if d.bitLen[2] < prev3 {
				d.bitLen[3]++
			}
		}
	}

	for len(p) > 0 {
		avail := whirlpoolBlockSize - d.bufLen
		if len(p) < avail {
			copy(d.buf[d.bufLen:], p)
			d.bufLen += len(p)
			return
		}
		copy(d.buf[d.bufLen:], p[:avail])
		p = p[avail:]
		d.bufLen = 0
		whirlpoolTransform(&d.state, &d.buf)
	}
	return
}

func (d *whirlpoolDigest) CloneHash() hash.Hash {
	c := *d
	return &c
}

func (d *whirlpoolDigest) Sum(in []byte) []byte {
	d0 := *d
	hash := d0.checkSum()
	return append(in, hash[:]...)
}

func (d *whirlpoolDigest) checkSum() [whirlpoolDigestSize]byte {
	// Padding
	d.buf[d.bufLen] = 0x80
	d.bufLen++

	if d.bufLen > 32 {
		// not enough room: pad to end of block, process, then pad new block
		for d.bufLen < whirlpoolBlockSize {
			d.buf[d.bufLen] = 0
			d.bufLen++
		}
		whirlpoolTransform(&d.state, &d.buf)
		d.bufLen = 0
	}

	// Zero fill up to position 32
	for d.bufLen < 32 {
		d.buf[d.bufLen] = 0
		d.bufLen++
	}

	// Append 256-bit bit length in big-endian
	binary.BigEndian.PutUint64(d.buf[32:], d.bitLen[3])
	binary.BigEndian.PutUint64(d.buf[40:], d.bitLen[2])
	binary.BigEndian.PutUint64(d.buf[48:], d.bitLen[1])
	binary.BigEndian.PutUint64(d.buf[56:], d.bitLen[0])
	whirlpoolTransform(&d.state, &d.buf)

	var digest [whirlpoolDigestSize]byte
	for i := 0; i < 8; i++ {
		binary.BigEndian.PutUint64(digest[i*8:], d.state[i])
	}
	return digest
}

func whirlpoolTransform(state *[8]uint64, buf *[64]byte) {
	var K [8]uint64
	var block [8]uint64
	var L [8]uint64
	var stateArr [8]uint64

	// Read block as big-endian uint64s
	for i := 0; i < 8; i++ {
		b := buf[i*8 : i*8+8]
		block[i] = (uint64(b[0]) << 56) |
			(uint64(b[1]) << 48) |
			(uint64(b[2]) << 40) |
			(uint64(b[3]) << 32) |
			(uint64(b[4]) << 24) |
			(uint64(b[5]) << 16) |
			(uint64(b[6]) << 8) |
			uint64(b[7])
	}

	// Apply K^0
	for i := 0; i < 8; i++ {
		stateArr[i] = block[i] ^ (func() uint64 { K[i] = state[i]; return K[i] })()
	}

	// 10 rounds
	for r := 1; r <= 10; r++ {
		// Compute K^r from K^{r-1}
		L[0] = wpC0[K[0]>>56] ^ wpC1[(K[7]>>48)&0xff] ^ wpC2[(K[6]>>40)&0xff] ^ wpC3[(K[5]>>32)&0xff] ^ wpC4[(K[4]>>24)&0xff] ^ wpC5[(K[3]>>16)&0xff] ^ wpC6[(K[2]>>8)&0xff] ^ wpC7[K[1]&0xff] ^ wpRC[r]
		L[1] = wpC0[K[1]>>56] ^ wpC1[(K[0]>>48)&0xff] ^ wpC2[(K[7]>>40)&0xff] ^ wpC3[(K[6]>>32)&0xff] ^ wpC4[(K[5]>>24)&0xff] ^ wpC5[(K[4]>>16)&0xff] ^ wpC6[(K[3]>>8)&0xff] ^ wpC7[K[2]&0xff]
		L[2] = wpC0[K[2]>>56] ^ wpC1[(K[1]>>48)&0xff] ^ wpC2[(K[0]>>40)&0xff] ^ wpC3[(K[7]>>32)&0xff] ^ wpC4[(K[6]>>24)&0xff] ^ wpC5[(K[5]>>16)&0xff] ^ wpC6[(K[4]>>8)&0xff] ^ wpC7[K[3]&0xff]
		L[3] = wpC0[K[3]>>56] ^ wpC1[(K[2]>>48)&0xff] ^ wpC2[(K[1]>>40)&0xff] ^ wpC3[(K[0]>>32)&0xff] ^ wpC4[(K[7]>>24)&0xff] ^ wpC5[(K[6]>>16)&0xff] ^ wpC6[(K[5]>>8)&0xff] ^ wpC7[K[4]&0xff]
		L[4] = wpC0[K[4]>>56] ^ wpC1[(K[3]>>48)&0xff] ^ wpC2[(K[2]>>40)&0xff] ^ wpC3[(K[1]>>32)&0xff] ^ wpC4[(K[0]>>24)&0xff] ^ wpC5[(K[7]>>16)&0xff] ^ wpC6[(K[6]>>8)&0xff] ^ wpC7[K[5]&0xff]
		L[5] = wpC0[K[5]>>56] ^ wpC1[(K[4]>>48)&0xff] ^ wpC2[(K[3]>>40)&0xff] ^ wpC3[(K[2]>>32)&0xff] ^ wpC4[(K[1]>>24)&0xff] ^ wpC5[(K[0]>>16)&0xff] ^ wpC6[(K[7]>>8)&0xff] ^ wpC7[K[6]&0xff]
		L[6] = wpC0[K[6]>>56] ^ wpC1[(K[5]>>48)&0xff] ^ wpC2[(K[4]>>40)&0xff] ^ wpC3[(K[3]>>32)&0xff] ^ wpC4[(K[2]>>24)&0xff] ^ wpC5[(K[1]>>16)&0xff] ^ wpC6[(K[0]>>8)&0xff] ^ wpC7[K[7]&0xff]
		L[7] = wpC0[K[7]>>56] ^ wpC1[(K[6]>>48)&0xff] ^ wpC2[(K[5]>>40)&0xff] ^ wpC3[(K[4]>>32)&0xff] ^ wpC4[(K[3]>>24)&0xff] ^ wpC5[(K[2]>>16)&0xff] ^ wpC6[(K[1]>>8)&0xff] ^ wpC7[K[0]&0xff]
		K = L

		// Apply round to state
		L[0] = wpC0[stateArr[0]>>56] ^ wpC1[(stateArr[7]>>48)&0xff] ^ wpC2[(stateArr[6]>>40)&0xff] ^ wpC3[(stateArr[5]>>32)&0xff] ^ wpC4[(stateArr[4]>>24)&0xff] ^ wpC5[(stateArr[3]>>16)&0xff] ^ wpC6[(stateArr[2]>>8)&0xff] ^ wpC7[stateArr[1]&0xff] ^ K[0]
		L[1] = wpC0[stateArr[1]>>56] ^ wpC1[(stateArr[0]>>48)&0xff] ^ wpC2[(stateArr[7]>>40)&0xff] ^ wpC3[(stateArr[6]>>32)&0xff] ^ wpC4[(stateArr[5]>>24)&0xff] ^ wpC5[(stateArr[4]>>16)&0xff] ^ wpC6[(stateArr[3]>>8)&0xff] ^ wpC7[stateArr[2]&0xff] ^ K[1]
		L[2] = wpC0[stateArr[2]>>56] ^ wpC1[(stateArr[1]>>48)&0xff] ^ wpC2[(stateArr[0]>>40)&0xff] ^ wpC3[(stateArr[7]>>32)&0xff] ^ wpC4[(stateArr[6]>>24)&0xff] ^ wpC5[(stateArr[5]>>16)&0xff] ^ wpC6[(stateArr[4]>>8)&0xff] ^ wpC7[stateArr[3]&0xff] ^ K[2]
		L[3] = wpC0[stateArr[3]>>56] ^ wpC1[(stateArr[2]>>48)&0xff] ^ wpC2[(stateArr[1]>>40)&0xff] ^ wpC3[(stateArr[0]>>32)&0xff] ^ wpC4[(stateArr[7]>>24)&0xff] ^ wpC5[(stateArr[6]>>16)&0xff] ^ wpC6[(stateArr[5]>>8)&0xff] ^ wpC7[stateArr[4]&0xff] ^ K[3]
		L[4] = wpC0[stateArr[4]>>56] ^ wpC1[(stateArr[3]>>48)&0xff] ^ wpC2[(stateArr[2]>>40)&0xff] ^ wpC3[(stateArr[1]>>32)&0xff] ^ wpC4[(stateArr[0]>>24)&0xff] ^ wpC5[(stateArr[7]>>16)&0xff] ^ wpC6[(stateArr[6]>>8)&0xff] ^ wpC7[stateArr[5]&0xff] ^ K[4]
		L[5] = wpC0[stateArr[5]>>56] ^ wpC1[(stateArr[4]>>48)&0xff] ^ wpC2[(stateArr[3]>>40)&0xff] ^ wpC3[(stateArr[2]>>32)&0xff] ^ wpC4[(stateArr[1]>>24)&0xff] ^ wpC5[(stateArr[0]>>16)&0xff] ^ wpC6[(stateArr[7]>>8)&0xff] ^ wpC7[stateArr[6]&0xff] ^ K[5]
		L[6] = wpC0[stateArr[6]>>56] ^ wpC1[(stateArr[5]>>48)&0xff] ^ wpC2[(stateArr[4]>>40)&0xff] ^ wpC3[(stateArr[3]>>32)&0xff] ^ wpC4[(stateArr[2]>>24)&0xff] ^ wpC5[(stateArr[1]>>16)&0xff] ^ wpC6[(stateArr[0]>>8)&0xff] ^ wpC7[stateArr[7]&0xff] ^ K[6]
		L[7] = wpC0[stateArr[7]>>56] ^ wpC1[(stateArr[6]>>48)&0xff] ^ wpC2[(stateArr[5]>>40)&0xff] ^ wpC3[(stateArr[4]>>32)&0xff] ^ wpC4[(stateArr[3]>>24)&0xff] ^ wpC5[(stateArr[2]>>16)&0xff] ^ wpC6[(stateArr[1]>>8)&0xff] ^ wpC7[stateArr[0]&0xff] ^ K[7]
		stateArr = L
	}

	// Feed-forward
	for i := 0; i < 8; i++ {
		state[i] ^= stateArr[i] ^ block[i]
	}
}
