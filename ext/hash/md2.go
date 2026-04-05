package hash

// MD2 hash algorithm implementation.
// Reference: RFC 1319

import (
	"encoding/binary"
	"hash"
	"io"
)

const md2DigestSize = 16
const md2BlockSize = 16

// MD2 S-box (permutation of 0..255 based on pi)
var md2S = [256]byte{
	41, 46, 67, 201, 162, 216, 124, 1, 61, 54, 84, 161, 236, 240, 6,
	19, 98, 167, 5, 243, 192, 199, 115, 140, 152, 147, 43, 217, 188,
	76, 130, 202, 30, 155, 87, 60, 253, 212, 224, 22, 103, 66, 111, 24,
	138, 23, 229, 18, 190, 78, 196, 214, 218, 158, 222, 73, 160, 251,
	245, 142, 187, 47, 238, 122, 169, 104, 121, 145, 21, 178, 7, 63,
	148, 194, 16, 137, 11, 34, 95, 33, 128, 127, 93, 154, 90, 144, 50,
	39, 53, 62, 204, 231, 191, 247, 151, 3, 255, 25, 48, 179, 72, 165,
	181, 209, 215, 94, 146, 42, 172, 86, 170, 198, 79, 184, 56, 210,
	150, 164, 125, 182, 118, 252, 107, 226, 156, 116, 4, 241, 69, 157,
	112, 89, 100, 113, 135, 32, 134, 91, 207, 101, 230, 45, 168, 2, 27,
	96, 37, 173, 174, 176, 185, 246, 28, 70, 97, 105, 52, 64, 126, 15,
	85, 71, 163, 35, 221, 81, 175, 58, 195, 92, 249, 206, 186, 197,
	234, 38, 44, 83, 13, 110, 133, 40, 132, 9, 211, 223, 205, 244, 65,
	129, 77, 82, 106, 220, 55, 200, 108, 193, 171, 250, 36, 225, 123,
	8, 12, 189, 177, 74, 120, 136, 149, 139, 227, 99, 232, 109, 233,
	203, 213, 254, 59, 0, 29, 57, 242, 239, 183, 14, 102, 88, 208, 228,
	166, 119, 114, 248, 235, 117, 75, 10, 49, 68, 80, 180, 143, 237,
	31, 26, 219, 153, 141, 51, 159, 17, 131, 20,
}

type md2Digest struct {
	state    [48]byte
	checksum [16]byte
	buf      [16]byte
	bufLen   int
	L        byte
}

func newMD2() hash.Hash {
	return &md2Digest{}
}

func (d *md2Digest) Reset() {
	*d = md2Digest{}
}

func (d *md2Digest) BlockSize() int { return md2BlockSize }
func (d *md2Digest) Size() int      { return md2DigestSize }

func (d *md2Digest) Write(p []byte) (n int, err error) {
	n = len(p)
	for len(p) > 0 {
		avail := md2BlockSize - d.bufLen
		if len(p) < avail {
			copy(d.buf[d.bufLen:], p)
			d.bufLen += len(p)
			return
		}
		copy(d.buf[d.bufLen:], p[:avail])
		p = p[avail:]
		d.bufLen = 0
		d.processBlock()
	}
	return
}

func (d *md2Digest) processBlock() {
	// Update state
	copy(d.state[16:], d.buf[:])
	for j := 0; j < 16; j++ {
		d.state[32+j] = d.buf[j] ^ d.state[j]
	}

	t := byte(0)
	for j := 0; j < 18; j++ {
		for k := 0; k < 48; k++ {
			d.state[k] ^= md2S[t]
			t = d.state[k]
		}
		t += byte(j)
	}

	// Update checksum
	l := d.L
	for j := 0; j < 16; j++ {
		c := d.buf[j]
		d.checksum[j] ^= md2S[c^l]
		l = d.checksum[j]
	}
	d.L = l
}

func (d *md2Digest) CloneHash() hash.Hash {
	c := *d
	return &c
}

// MarshalBinary serializes the hash state for copying.
func (d *md2Digest) MarshalBinary() ([]byte, error) {
	b := make([]byte, 48+16+16+4+1)
	copy(b[0:48], d.state[:])
	copy(b[48:64], d.checksum[:])
	copy(b[64:80], d.buf[:])
	binary.LittleEndian.PutUint32(b[80:], uint32(d.bufLen))
	b[84] = d.L
	return b, nil
}

// UnmarshalBinary restores the hash state.
func (d *md2Digest) UnmarshalBinary(b []byte) error {
	if len(b) < 85 {
		return io.ErrUnexpectedEOF
	}
	copy(d.state[:], b[0:48])
	copy(d.checksum[:], b[48:64])
	copy(d.buf[:], b[64:80])
	d.bufLen = int(binary.LittleEndian.Uint32(b[80:]))
	d.L = b[84]
	return nil
}

func (d *md2Digest) Sum(in []byte) []byte {
	d0 := *d
	hash := d0.checkSum()
	return append(in, hash[:]...)
}

func (d *md2Digest) checkSum() [md2DigestSize]byte {
	// Padding
	padLen := md2BlockSize - d.bufLen
	for i := d.bufLen; i < md2BlockSize; i++ {
		d.buf[i] = byte(padLen)
	}
	d.bufLen = 0
	d.processBlock()

	// Append checksum
	copy(d.buf[:], d.checksum[:])
	d.bufLen = 0
	d.processBlock()

	var digest [md2DigestSize]byte
	copy(digest[:], d.state[:16])
	return digest
}
