package hash

// Tiger hash algorithm
// Reference: https://www.cs.technion.ac.il/~biham/Reports/Tiger/

import (
	"encoding/binary"
	"hash"
)

type tigerDigest struct {
	state  [3]uint64
	buf    [64]byte
	bufN   int
	length uint64
	passes int
	size   int // output size: 128, 160, or 192 bits
}

func newTiger(bits, passes int) hash.Hash {
	d := &tigerDigest{passes: passes, size: bits / 8}
	d.Reset()
	return d
}

func (d *tigerDigest) Reset() {
	d.state[0] = 0x0123456789ABCDEF
	d.state[1] = 0xFEDCBA9876543210
	d.state[2] = 0xF096A5B4C3B2E187
	d.bufN = 0
	d.length = 0
}

func (d *tigerDigest) Size() int      { return d.size }
func (d *tigerDigest) BlockSize() int { return 64 }

func (d *tigerDigest) Write(p []byte) (int, error) {
	n := len(p)
	d.length += uint64(n)

	if d.bufN > 0 {
		avail := 64 - d.bufN
		if len(p) < avail {
			copy(d.buf[d.bufN:], p)
			d.bufN += len(p)
			return n, nil
		}
		copy(d.buf[d.bufN:], p[:avail])
		p = p[avail:]
		d.bufN = 0
		tigerProcess(&d.state, &d.buf, d.passes)
	}

	for len(p) >= 64 {
		var block [64]byte
		copy(block[:], p[:64])
		tigerProcess(&d.state, &block, d.passes)
		p = p[64:]
	}

	if len(p) > 0 {
		copy(d.buf[:], p)
		d.bufN = len(p)
	}
	return n, nil
}

func (d *tigerDigest) CloneHash() hash.Hash {
	c := *d
	return &c
}

func (d *tigerDigest) Sum(in []byte) []byte {
	d0 := *d
	hash := d0.checkSum()
	return append(in, hash[:d.size]...)
}

func (d *tigerDigest) checkSum() [24]byte {
	// Padding
	d.buf[d.bufN] = 0x01
	d.bufN++
	for d.bufN < 56 {
		d.buf[d.bufN] = 0
		d.bufN++
	}
	if d.bufN > 56 {
		for d.bufN < 64 {
			d.buf[d.bufN] = 0
			d.bufN++
		}
		tigerProcess(&d.state, &d.buf, d.passes)
		d.bufN = 0
		for d.bufN < 56 {
			d.buf[d.bufN] = 0
			d.bufN++
		}
	}
	// Length in bits, little-endian
	binary.LittleEndian.PutUint64(d.buf[56:], d.length*8)
	tigerProcess(&d.state, &d.buf, d.passes)

	var out [24]byte
	binary.LittleEndian.PutUint64(out[0:], d.state[0])
	binary.LittleEndian.PutUint64(out[8:], d.state[1])
	binary.LittleEndian.PutUint64(out[16:], d.state[2])
	return out
}

func tigerProcess(state *[3]uint64, block *[64]byte, passes int) {
	var x [8]uint64
	for i := 0; i < 8; i++ {
		x[i] = binary.LittleEndian.Uint64(block[i*8:])
	}

	aa, bb, cc := state[0], state[1], state[2]
	a, b, c := aa, bb, cc

	tigerPass(&a, &b, &c, &x, 5)
	tigerKeySchedule(&x)
	tigerPass(&c, &a, &b, &x, 7)
	tigerKeySchedule(&x)
	tigerPass(&b, &c, &a, &x, 9)

	// Extra passes for 4-pass tiger
	for i := 3; i < passes; i++ {
		tigerKeySchedule(&x)
		tigerPass(&a, &b, &c, &x, 9)
		// rotate: tmpa=a; a=c; c=b; b=tmpa
		a, b, c = c, a, b
	}

	// Feedforward
	state[0] = a ^ aa
	state[1] = b - bb
	state[2] = c + cc
}

func tigerKeySchedule(x *[8]uint64) {
	x[0] -= x[7] ^ 0xA5A5A5A5A5A5A5A5
	x[1] ^= x[0]
	x[2] += x[1]
	x[3] -= x[2] ^ (^x[1] << 19)
	x[4] ^= x[3]
	x[5] += x[4]
	x[6] -= x[5] ^ (^x[4] >> 23)
	x[7] ^= x[6]
	x[0] += x[7]
	x[1] -= x[0] ^ (^x[7] << 19)
	x[2] ^= x[1]
	x[3] += x[2]
	x[4] -= x[3] ^ (^x[2] >> 23)
	x[5] ^= x[4]
	x[6] += x[5]
	x[7] -= x[6] ^ 0x0123456789ABCDEF
}

func tigerPass(a, b, c *uint64, x *[8]uint64, mul uint64) {
	*c ^= x[0]
	*a -= tigerT1[*c&0xFF] ^ tigerT2[(*c>>16)&0xFF] ^ tigerT3[(*c>>32)&0xFF] ^ tigerT4[(*c>>48)&0xFF]
	*b += tigerT4[(*c>>8)&0xFF] ^ tigerT3[(*c>>24)&0xFF] ^ tigerT2[(*c>>40)&0xFF] ^ tigerT1[(*c>>56)&0xFF]
	*b *= mul

	*a ^= x[1]
	*b -= tigerT1[*a&0xFF] ^ tigerT2[(*a>>16)&0xFF] ^ tigerT3[(*a>>32)&0xFF] ^ tigerT4[(*a>>48)&0xFF]
	*c += tigerT4[(*a>>8)&0xFF] ^ tigerT3[(*a>>24)&0xFF] ^ tigerT2[(*a>>40)&0xFF] ^ tigerT1[(*a>>56)&0xFF]
	*c *= mul

	*b ^= x[2]
	*c -= tigerT1[*b&0xFF] ^ tigerT2[(*b>>16)&0xFF] ^ tigerT3[(*b>>32)&0xFF] ^ tigerT4[(*b>>48)&0xFF]
	*a += tigerT4[(*b>>8)&0xFF] ^ tigerT3[(*b>>24)&0xFF] ^ tigerT2[(*b>>40)&0xFF] ^ tigerT1[(*b>>56)&0xFF]
	*a *= mul

	*c ^= x[3]
	*a -= tigerT1[*c&0xFF] ^ tigerT2[(*c>>16)&0xFF] ^ tigerT3[(*c>>32)&0xFF] ^ tigerT4[(*c>>48)&0xFF]
	*b += tigerT4[(*c>>8)&0xFF] ^ tigerT3[(*c>>24)&0xFF] ^ tigerT2[(*c>>40)&0xFF] ^ tigerT1[(*c>>56)&0xFF]
	*b *= mul

	*a ^= x[4]
	*b -= tigerT1[*a&0xFF] ^ tigerT2[(*a>>16)&0xFF] ^ tigerT3[(*a>>32)&0xFF] ^ tigerT4[(*a>>48)&0xFF]
	*c += tigerT4[(*a>>8)&0xFF] ^ tigerT3[(*a>>24)&0xFF] ^ tigerT2[(*a>>40)&0xFF] ^ tigerT1[(*a>>56)&0xFF]
	*c *= mul

	*b ^= x[5]
	*c -= tigerT1[*b&0xFF] ^ tigerT2[(*b>>16)&0xFF] ^ tigerT3[(*b>>32)&0xFF] ^ tigerT4[(*b>>48)&0xFF]
	*a += tigerT4[(*b>>8)&0xFF] ^ tigerT3[(*b>>24)&0xFF] ^ tigerT2[(*b>>40)&0xFF] ^ tigerT1[(*b>>56)&0xFF]
	*a *= mul

	*c ^= x[6]
	*a -= tigerT1[*c&0xFF] ^ tigerT2[(*c>>16)&0xFF] ^ tigerT3[(*c>>32)&0xFF] ^ tigerT4[(*c>>48)&0xFF]
	*b += tigerT4[(*c>>8)&0xFF] ^ tigerT3[(*c>>24)&0xFF] ^ tigerT2[(*c>>40)&0xFF] ^ tigerT1[(*c>>56)&0xFF]
	*b *= mul

	*a ^= x[7]
	*b -= tigerT1[*a&0xFF] ^ tigerT2[(*a>>16)&0xFF] ^ tigerT3[(*a>>32)&0xFF] ^ tigerT4[(*a>>48)&0xFF]
	*c += tigerT4[(*a>>8)&0xFF] ^ tigerT3[(*a>>24)&0xFF] ^ tigerT2[(*a>>40)&0xFF] ^ tigerT1[(*a>>56)&0xFF]
	*c *= mul
}
