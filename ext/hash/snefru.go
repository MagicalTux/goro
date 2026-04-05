package hash

// Snefru hash algorithm
// Ported from PHP's hash_snefru.c

import (
	"encoding/binary"
	"hash"
)

const (
	snefruBlockSize128 = 32
	snefruBlockSize256 = 32
)

type snefruDigest struct {
	state  [16]uint32
	buf    [32]byte
	bufLen int
	count0 uint32
	count1 uint32
	// output size: 16 or 32 bytes (128 or 256 bits)
	size int
}

func newSnefru(bits int) hash.Hash {
	d := &snefruDigest{size: bits / 8}
	d.Reset()
	return d
}

func (d *snefruDigest) Reset() {
	d.state = [16]uint32{}
	d.buf = [32]byte{}
	d.bufLen = 0
	d.count0 = 0
	d.count1 = 0
}

func (d *snefruDigest) Size() int      { return d.size }
func (d *snefruDigest) BlockSize() int { return 32 }

func (d *snefruDigest) Write(p []byte) (int, error) {
	n := len(p)

	const maxU32 = uint32(0xffffffff)
	bits64 := uint64(len(p)) * 8
	if uint64(maxU32-d.count1) < bits64 {
		d.count0++
		d.count1 = maxU32 - d.count1
		d.count1 = uint32(bits64) - d.count1
	} else {
		d.count1 += uint32(bits64)
	}

	if d.bufLen+len(p) < 32 {
		copy(d.buf[d.bufLen:], p)
		d.bufLen += len(p)
		return n, nil
	}

	i := 0
	rem := (d.bufLen + len(p)) % 32
	if d.bufLen > 0 {
		i = 32 - d.bufLen
		copy(d.buf[d.bufLen:], p[:i])
		d.transform(d.buf[:])
	}

	for i+32 <= len(p) {
		d.transform(p[i:])
		i += 32
	}

	copy(d.buf[:], p[i:])
	for j := rem; j < 32; j++ {
		d.buf[j] = 0
	}
	d.bufLen = rem
	return n, nil
}

func (d *snefruDigest) transform(input []byte) {
	for i, j := 0, 0; i < 32; i += 4 {
		d.state[8+j] = (uint32(input[i]) << 24) | (uint32(input[i+1]) << 16) |
			(uint32(input[i+2]) << 8) | uint32(input[i+3])
		j++
	}
	snefruProcess(&d.state)
	// Clear the data portion
	for i := 8; i < 16; i++ {
		d.state[i] = 0
	}
}

func (d *snefruDigest) CloneHash() hash.Hash {
	c := *d
	return &c
}

func (d *snefruDigest) Sum(in []byte) []byte {
	d0 := *d
	hash := d0.checkSum()
	return append(in, hash[:d.size]...)
}

func (d *snefruDigest) checkSum() [32]byte {
	if d.bufLen > 0 {
		d.transform(d.buf[:])
	}
	d.state[14] = d.count0
	d.state[15] = d.count1
	snefruProcess(&d.state)

	var digest [32]byte
	for i, j := 0, 0; j < 32; i++ {
		digest[j] = byte(d.state[i] >> 24)
		digest[j+1] = byte(d.state[i] >> 16)
		digest[j+2] = byte(d.state[i] >> 8)
		digest[j+3] = byte(d.state[i])
		j += 4
	}
	return digest
}

var snefruShifts = [4]uint{16, 8, 16, 24}

func snefruProcess(state *[16]uint32) {
	B00 := state[0]
	B01 := state[1]
	B02 := state[2]
	B03 := state[3]
	B04 := state[4]
	B05 := state[5]
	B06 := state[6]
	B07 := state[7]
	B08 := state[8]
	B09 := state[9]
	B10 := state[10]
	B11 := state[11]
	B12 := state[12]
	B13 := state[13]
	B14 := state[14]
	B15 := state[15]

	for index := 0; index < 8; index++ {
		t0 := snefruTables[2*index+0]
		t1 := snefruTables[2*index+1]
		for b := 0; b < 4; b++ {
			sbe := t0[B00&0xff]; B15 ^= sbe; B01 ^= sbe
			sbe = t0[B01&0xff]; B00 ^= sbe; B02 ^= sbe
			sbe = t1[B02&0xff]; B01 ^= sbe; B03 ^= sbe
			sbe = t1[B03&0xff]; B02 ^= sbe; B04 ^= sbe
			sbe = t0[B04&0xff]; B03 ^= sbe; B05 ^= sbe
			sbe = t0[B05&0xff]; B04 ^= sbe; B06 ^= sbe
			sbe = t1[B06&0xff]; B05 ^= sbe; B07 ^= sbe
			sbe = t1[B07&0xff]; B06 ^= sbe; B08 ^= sbe
			sbe = t0[B08&0xff]; B07 ^= sbe; B09 ^= sbe
			sbe = t0[B09&0xff]; B08 ^= sbe; B10 ^= sbe
			sbe = t1[B10&0xff]; B09 ^= sbe; B11 ^= sbe
			sbe = t1[B11&0xff]; B10 ^= sbe; B12 ^= sbe
			sbe = t0[B12&0xff]; B11 ^= sbe; B13 ^= sbe
			sbe = t0[B13&0xff]; B12 ^= sbe; B14 ^= sbe
			sbe = t1[B14&0xff]; B13 ^= sbe; B15 ^= sbe
			sbe = t1[B15&0xff]; B14 ^= sbe; B00 ^= sbe

			rshift := snefruShifts[b]
			lshift := 32 - rshift
			B00 = (B00 >> rshift) | (B00 << lshift)
			B01 = (B01 >> rshift) | (B01 << lshift)
			B02 = (B02 >> rshift) | (B02 << lshift)
			B03 = (B03 >> rshift) | (B03 << lshift)
			B04 = (B04 >> rshift) | (B04 << lshift)
			B05 = (B05 >> rshift) | (B05 << lshift)
			B06 = (B06 >> rshift) | (B06 << lshift)
			B07 = (B07 >> rshift) | (B07 << lshift)
			B08 = (B08 >> rshift) | (B08 << lshift)
			B09 = (B09 >> rshift) | (B09 << lshift)
			B10 = (B10 >> rshift) | (B10 << lshift)
			B11 = (B11 >> rshift) | (B11 << lshift)
			B12 = (B12 >> rshift) | (B12 << lshift)
			B13 = (B13 >> rshift) | (B13 << lshift)
			B14 = (B14 >> rshift) | (B14 << lshift)
			B15 = (B15 >> rshift) | (B15 << lshift)
		}
	}
	state[0] ^= B15
	state[1] ^= B14
	state[2] ^= B13
	state[3] ^= B12
	state[4] ^= B11
	state[5] ^= B10
	state[6] ^= B09
	state[7] ^= B08
}

// suppress unused import warning
var _ = binary.BigEndian
