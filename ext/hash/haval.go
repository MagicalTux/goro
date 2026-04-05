package hash

// HAVAL hash algorithm
// Ported from PHP's hash_haval.c
// Reference: https://labs.amd.com/techology/crypt/haval/

import (
	"encoding/binary"
	"hash"
	"math/bits"
)

var havalD0 = [8]uint32{
	0x243F6A88, 0x85A308D3, 0x13198A2E, 0x03707344, 0xA4093822, 0x299F31D0, 0x082EFA98, 0xEC4E6C89,
}

var havalK2 = [32]uint32{
	0x452821E6, 0x38D01377, 0xBE5466CF, 0x34E90C6C, 0xC0AC29B7, 0xC97C50DD, 0x3F84D5B5, 0xB5470917,
	0x9216D5D9, 0x8979FB1B, 0xD1310BA6, 0x98DFB5AC, 0x2FFD72DB, 0xD01ADFB7, 0xB8E1AFED, 0x6A267E96,
	0xBA7C9045, 0xF12C7F99, 0x24A19947, 0xB3916CF7, 0x0801F2E2, 0x858EFC16, 0x636920D8, 0x71574E69,
	0xA458FEA3, 0xF4933D7E, 0x0D95748F, 0x728EB658, 0x718BCD58, 0x82154AEE, 0x7B54A41D, 0xC25A59B5,
}

var havalK3 = [32]uint32{
	0x9C30D539, 0x2AF26013, 0xC5D1B023, 0x286085F0, 0xCA417918, 0xB8DB38EF, 0x8E79DCB0, 0x603A180E,
	0x6C9E0E8B, 0xB01E8A3E, 0xD71577C1, 0xBD314B27, 0x78AF2FDA, 0x55605C60, 0xE65525F3, 0xAA55AB94,
	0x57489862, 0x63E81440, 0x55CA396A, 0x2AAB10B6, 0xB4CC5C34, 0x1141E8CE, 0xA15486AF, 0x7C72E993,
	0xB3EE1411, 0x636FBC2A, 0x2BA9C55D, 0x741831F6, 0xCE5C3E16, 0x9B87931E, 0xAFD6BA33, 0x6C24CF5C,
}

var havalK4 = [32]uint32{
	0x7A325381, 0x28958677, 0x3B8F4898, 0x6B4BB9AF, 0xC4BFE81B, 0x66282193, 0x61D809CC, 0xFB21A991,
	0x487CAC60, 0x5DEC8032, 0xEF845D5D, 0xE98575B1, 0xDC262302, 0xEB651B88, 0x23893E81, 0xD396ACC5,
	0x0F6D6FF3, 0x83F44239, 0x2E0B4482, 0xA4842004, 0x69C8F04A, 0x9E1F9B5E, 0x21C66842, 0xF6E96C9A,
	0x670C9C61, 0xABD388F0, 0x6A51A0D2, 0xD8542F68, 0x960FA728, 0xAB5133A3, 0x6EEF0B6C, 0x137A3BE4,
}

var havalK5 = [32]uint32{
	0xBA3BF050, 0x7EFB2A98, 0xA1F1651D, 0x39AF0176, 0x66CA593E, 0x82430E88, 0x8CEE8619, 0x456F9FB4,
	0x7D84A5C3, 0x3B8B5EBE, 0xE06F75D8, 0x85C12073, 0x401A449F, 0x56C16AA6, 0x4ED3AA62, 0x363F7706,
	0x1BFEDF72, 0x429B023D, 0x37D0D724, 0xD00A1248, 0xDB0FEAD3, 0x49F1C09B, 0x075372C9, 0x80991B7B,
	0x25D479D8, 0xF6E8DEF7, 0xE3FE501A, 0xB6794C3B, 0x976CE0BD, 0x04C006BA, 0xC1A94FB6, 0x409F60C4,
}

var havalI2 = [32]int{5, 14, 26, 18, 11, 28, 7, 16, 0, 23, 20, 22, 1, 10, 4, 8,
	30, 3, 21, 9, 17, 24, 29, 6, 19, 12, 15, 13, 2, 25, 31, 27}

var havalI3 = [32]int{19, 9, 4, 20, 28, 17, 8, 22, 29, 14, 25, 12, 24, 30, 16, 26,
	31, 15, 7, 3, 1, 0, 18, 27, 13, 6, 21, 10, 23, 11, 5, 2}

var havalI4 = [32]int{24, 4, 0, 14, 2, 7, 28, 23, 26, 6, 30, 20, 18, 25, 19, 3,
	22, 11, 31, 21, 8, 27, 12, 9, 1, 29, 5, 15, 17, 10, 16, 13}

var havalI5 = [32]int{27, 3, 21, 26, 17, 11, 20, 29, 19, 0, 12, 7, 13, 8, 31, 10,
	5, 9, 14, 30, 18, 6, 28, 24, 2, 23, 16, 22, 4, 1, 25, 15}

// Index tables M0-M7: M_k[i] = (k - i) mod 8 = (k + 8 - i%8) % 8
// These are precomputed: M_k[i] = (8 + k - i%8) % 8
var havalM = [8][32]int{}

func init() {
	for k := 0; k < 8; k++ {
		for i := 0; i < 32; i++ {
			havalM[k][i] = (8 + k - i%8) % 8
		}
	}
}

// Boolean functions
func havalF1(x6, x5, x4, x3, x2, x1, x0 uint32) uint32 {
	return (x1&x4)^(x2&x5)^(x3&x6)^(x0&x1)^x0
}

func havalF2(x6, x5, x4, x3, x2, x1, x0 uint32) uint32 {
	return (x1&x2&x3)^(x2&x4&x5)^(x1&x2)^(x1&x4)^(x2&x6)^(x3&x5)^(x4&x5)^(x0&x2)^x0
}

func havalF3(x6, x5, x4, x3, x2, x1, x0 uint32) uint32 {
	return (x1&x2&x3)^(x1&x4)^(x2&x5)^(x3&x6)^(x0&x3)^x0
}

func havalF4(x6, x5, x4, x3, x2, x1, x0 uint32) uint32 {
	return (x1&x2&x3)^(x2&x4&x5)^(x3&x4&x6)^(x1&x4)^(x2&x6)^(x3&x4)^(x3&x5)^(x3&x6)^(x4&x5)^(x4&x6)^(x0&x4)^x0
}

func havalF5(x6, x5, x4, x3, x2, x1, x0 uint32) uint32 {
	return (x1&x4)^(x2&x5)^(x3&x6)^(x0&x1&x2&x3)^(x0&x5)^x0
}

type havalDigest struct {
	state  [8]uint32
	buf    [128]byte
	bufLen int
	count0 uint32
	count1 uint32
	passes int
	output int // output size in bits (128, 160, 192, 224, 256)
}

func newHaval(bits, passes int) hash.Hash {
	d := &havalDigest{passes: passes, output: bits}
	d.Reset()
	return d
}

func (d *havalDigest) Reset() {
	d.state = havalD0
	d.buf = [128]byte{}
	d.bufLen = 0
	d.count0 = 0
	d.count1 = 0
}

func (d *havalDigest) Size() int      { return d.output / 8 }
func (d *havalDigest) BlockSize() int { return 128 }

func (d *havalDigest) Write(p []byte) (int, error) {
	n := len(p)
	inputLen := uint32(len(p))

	// Update bit count
	if d.count0 += inputLen << 3; d.count0 < inputLen<<3 {
		d.count1++
	}
	d.count1 += inputLen >> 29

	// index is the current position within the 128-byte block
	index := (d.count0>>3)&0x7F - uint32(inputLen&0x7F)
	if index < 0 || inputLen == 0 {
		index = 0
	}
	// Actually use bufLen
	index = uint32(d.bufLen)

	partLen := uint32(128) - index
	i := uint32(0)
	if inputLen >= partLen {
		copy(d.buf[index:], p[:partLen])
		d.transform(d.buf[:])
		i = partLen
		for i+127 < inputLen {
			d.transform(p[i : i+128])
			i += 128
		}
		d.bufLen = 0
	}
	copy(d.buf[d.bufLen:], p[i:])
	d.bufLen += int(inputLen - i)
	return n, nil
}

func (d *havalDigest) CloneHash() hash.Hash {
	c := *d
	return &c
}

func (d *havalDigest) Sum(in []byte) []byte {
	d0 := *d
	hash := d0.checkSum()
	return append(in, hash[:d.output/8]...)
}

const havalVersion = 0x01

func (d *havalDigest) checkSum() [32]byte {
	var tailBits [10]byte
	// Version, Passes, and Digest Length
	outputBits := uint32(d.output)
	tailBits[0] = byte((havalVersion & 0x07) | ((d.passes & 0x07) << 3) | (int(outputBits&0x03) << 6))
	tailBits[1] = byte(outputBits >> 2)
	// Save number of bits (count as little-endian uint32s)
	binary.LittleEndian.PutUint32(tailBits[2:], d.count0)
	binary.LittleEndian.PutUint32(tailBits[6:], d.count1)

	// Pad out to 118 mod 128
	index := (d.count0 >> 3) & 0x7F
	var padLen uint32
	if index < 118 {
		padLen = 118 - index
	} else {
		padLen = 246 - index
	}

	padding := make([]byte, padLen)
	padding[0] = 1
	d.Write(padding)
	d.Write(tailBits[:10])

	// Now apply output-length-specific folding
	switch d.output {
	case 128:
		d.state[3] += (d.state[7] & 0xFF000000) |
			(d.state[6] & 0x00FF0000) |
			(d.state[5] & 0x0000FF00) |
			(d.state[4] & 0x000000FF)
		d.state[2] += (((d.state[7] & 0x00FF0000) |
			(d.state[6] & 0x0000FF00) |
			(d.state[5] & 0x000000FF)) << 8) |
			((d.state[4] & 0xFF000000) >> 24)
		d.state[1] += (((d.state[7] & 0x0000FF00) |
			(d.state[6] & 0x000000FF)) << 16) |
			(((d.state[5] & 0xFF000000) |
				(d.state[4] & 0x00FF0000)) >> 16)
		d.state[0] += ((d.state[7] & 0x000000FF) << 24) |
			(((d.state[6] & 0xFF000000) |
				(d.state[5] & 0x00FF0000) |
				(d.state[4] & 0x0000FF00)) >> 8)
	case 160:
		d.state[4] += ((d.state[7] & 0xFE000000) |
			(d.state[6] & 0x01F80000) |
			(d.state[5] & 0x0007F000)) >> 12
		d.state[3] += ((d.state[7] & 0x01F80000) |
			(d.state[6] & 0x0007F000) |
			(d.state[5] & 0x00000FC0)) >> 6
		d.state[2] += (d.state[7] & 0x0007F000) |
			(d.state[6] & 0x00000FC0) |
			(d.state[5] & 0x0000003F)
		d.state[1] += bits.RotateLeft32((d.state[7]&0x00000FC0)|
			(d.state[6]&0x0000003F)|
			(d.state[5]&0xFE000000), -25)
		d.state[0] += bits.RotateLeft32((d.state[7]&0x0000003F)|
			(d.state[6]&0xFE000000)|
			(d.state[5]&0x01F80000), -19)
	case 192:
		d.state[5] += ((d.state[7] & 0xFC000000) | (d.state[6] & 0x03E00000)) >> 21
		d.state[4] += ((d.state[7] & 0x03E00000) | (d.state[6] & 0x001F0000)) >> 16
		d.state[3] += ((d.state[7] & 0x001F0000) | (d.state[6] & 0x0000FC00)) >> 10
		d.state[2] += ((d.state[7] & 0x0000FC00) | (d.state[6] & 0x000003E0)) >> 5
		d.state[1] += (d.state[7] & 0x000003E0) | (d.state[6] & 0x0000001F)
		d.state[0] += bits.RotateLeft32((d.state[7]&0x0000001F)|(d.state[6]&0xFC000000), -26)
	case 224:
		d.state[6] += d.state[7] & 0x0000000F
		d.state[5] += (d.state[7] >> 4) & 0x0000001F
		d.state[4] += (d.state[7] >> 9) & 0x0000000F
		d.state[3] += (d.state[7] >> 13) & 0x0000001F
		d.state[2] += (d.state[7] >> 18) & 0x0000000F
		d.state[1] += (d.state[7] >> 22) & 0x0000001F
		d.state[0] += (d.state[7] >> 27) & 0x0000001F
	case 256:
		// no folding needed
	}

	var digest [32]byte
	for i, j := 0, 0; j < 32; i++ {
		digest[j] = byte(d.state[i])
		digest[j+1] = byte(d.state[i] >> 8)
		digest[j+2] = byte(d.state[i] >> 16)
		digest[j+3] = byte(d.state[i] >> 24)
		j += 4
	}
	return digest
}

func (d *havalDigest) transform(block []byte) {
	var x [32]uint32
	for i := 0; i < 32; i++ {
		x[i] = binary.LittleEndian.Uint32(block[i*4:])
	}

	e := d.state

	// Pass 1 (common to all)
	switch d.passes {
	case 3:
		d.pass3(e[:], x[:])
	case 4:
		d.pass4(e[:], x[:])
	case 5:
		d.pass5(e[:], x[:])
	}

	for i := 0; i < 8; i++ {
		d.state[i] += e[i]
	}
}

func rotr32(x uint32, n int) uint32 {
	return bits.RotateLeft32(x, -n)
}

func (d *havalDigest) pass3(e []uint32, x []uint32) {
	m := havalM
	for i := 0; i < 32; i++ {
		e[7-(i%8)] = rotr32(havalF1(e[m[1][i]], e[m[0][i]], e[m[3][i]], e[m[5][i]], e[m[6][i]], e[m[2][i]], e[m[4][i]]), 7) + rotr32(e[m[7][i]], 11) + x[i]
	}
	for i := 0; i < 32; i++ {
		e[7-(i%8)] = rotr32(havalF2(e[m[4][i]], e[m[2][i]], e[m[1][i]], e[m[0][i]], e[m[5][i]], e[m[3][i]], e[m[6][i]]), 7) + rotr32(e[m[7][i]], 11) + x[havalI2[i]] + havalK2[i]
	}
	for i := 0; i < 32; i++ {
		e[7-(i%8)] = rotr32(havalF3(e[m[6][i]], e[m[1][i]], e[m[2][i]], e[m[3][i]], e[m[4][i]], e[m[5][i]], e[m[0][i]]), 7) + rotr32(e[m[7][i]], 11) + x[havalI3[i]] + havalK3[i]
	}
}

func (d *havalDigest) pass4(e []uint32, x []uint32) {
	m := havalM
	for i := 0; i < 32; i++ {
		e[7-(i%8)] = rotr32(havalF1(e[m[2][i]], e[m[6][i]], e[m[1][i]], e[m[4][i]], e[m[5][i]], e[m[3][i]], e[m[0][i]]), 7) + rotr32(e[m[7][i]], 11) + x[i]
	}
	for i := 0; i < 32; i++ {
		e[7-(i%8)] = rotr32(havalF2(e[m[3][i]], e[m[5][i]], e[m[2][i]], e[m[0][i]], e[m[1][i]], e[m[6][i]], e[m[4][i]]), 7) + rotr32(e[m[7][i]], 11) + x[havalI2[i]] + havalK2[i]
	}
	for i := 0; i < 32; i++ {
		e[7-(i%8)] = rotr32(havalF3(e[m[1][i]], e[m[4][i]], e[m[3][i]], e[m[6][i]], e[m[0][i]], e[m[2][i]], e[m[5][i]]), 7) + rotr32(e[m[7][i]], 11) + x[havalI3[i]] + havalK3[i]
	}
	for i := 0; i < 32; i++ {
		e[7-(i%8)] = rotr32(havalF4(e[m[6][i]], e[m[4][i]], e[m[0][i]], e[m[5][i]], e[m[2][i]], e[m[1][i]], e[m[3][i]]), 7) + rotr32(e[m[7][i]], 11) + x[havalI4[i]] + havalK4[i]
	}
}

func (d *havalDigest) pass5(e []uint32, x []uint32) {
	m := havalM
	for i := 0; i < 32; i++ {
		e[7-(i%8)] = rotr32(havalF1(e[m[3][i]], e[m[4][i]], e[m[1][i]], e[m[0][i]], e[m[5][i]], e[m[2][i]], e[m[6][i]]), 7) + rotr32(e[m[7][i]], 11) + x[i]
	}
	for i := 0; i < 32; i++ {
		e[7-(i%8)] = rotr32(havalF2(e[m[6][i]], e[m[2][i]], e[m[1][i]], e[m[0][i]], e[m[3][i]], e[m[4][i]], e[m[5][i]]), 7) + rotr32(e[m[7][i]], 11) + x[havalI2[i]] + havalK2[i]
	}
	for i := 0; i < 32; i++ {
		e[7-(i%8)] = rotr32(havalF3(e[m[2][i]], e[m[6][i]], e[m[0][i]], e[m[4][i]], e[m[3][i]], e[m[1][i]], e[m[5][i]]), 7) + rotr32(e[m[7][i]], 11) + x[havalI3[i]] + havalK3[i]
	}
	for i := 0; i < 32; i++ {
		e[7-(i%8)] = rotr32(havalF4(e[m[1][i]], e[m[5][i]], e[m[3][i]], e[m[2][i]], e[m[0][i]], e[m[4][i]], e[m[6][i]]), 7) + rotr32(e[m[7][i]], 11) + x[havalI4[i]] + havalK4[i]
	}
	for i := 0; i < 32; i++ {
		e[7-(i%8)] = rotr32(havalF5(e[m[2][i]], e[m[5][i]], e[m[0][i]], e[m[6][i]], e[m[4][i]], e[m[3][i]], e[m[1][i]]), 7) + rotr32(e[m[7][i]], 11) + x[havalI5[i]] + havalK5[i]
	}
}
