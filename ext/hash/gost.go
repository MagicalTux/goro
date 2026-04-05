package hash

// GOST R 34.11-94 hash algorithm
// Ported from PHP's hash_gost.c

import (
	"hash"
)

type gostDigest struct {
	state  [16]uint32
	buf    [32]byte
	bufLen int
	count0 uint32
	count1 uint32
	isCrypto bool
}

func newGost(crypto bool) hash.Hash {
	d := &gostDigest{isCrypto: crypto}
	d.Reset()
	return d
}

func (d *gostDigest) Reset() {
	d.state = [16]uint32{}
	d.buf = [32]byte{}
	d.bufLen = 0
	d.count0 = 0
	d.count1 = 0
}

func (d *gostDigest) Size() int      { return 32 }
func (d *gostDigest) BlockSize() int { return 32 }

func (d *gostDigest) tables() *[4][256]uint32 {
	if d.isCrypto {
		return &gostTablesCrypto
	}
	return &gostTablesTest
}

func (d *gostDigest) Write(p []byte) (int, error) {
	n := len(p)

	const maxU32 = uint32(0xffffffff)
	bits := uint32(len(p)) * 8
	if (maxU32 - d.count0) < bits {
		d.count1++
		d.count0 = maxU32 - d.count0
		d.count0 = bits - d.count0
	} else {
		d.count0 += bits
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
		d.gostTransform(d.buf[:])
	}

	for i+32 <= len(p) {
		d.gostTransform(p[i:])
		i += 32
	}

	copy(d.buf[:], p[i:])
	for j := rem; j < 32; j++ {
		d.buf[j] = 0
	}
	d.bufLen = rem
	return n, nil
}

func (d *gostDigest) gostTransform(input []byte) {
	var data [8]uint32
	var temp uint32
	for i, j := 0, 0; i < 8; i++ {
		data[i] = uint32(input[j]) | (uint32(input[j+1]) << 8) | (uint32(input[j+2]) << 16) | (uint32(input[j+3]) << 24)
		j += 4
		d.state[i+8] += data[i] + temp
		if d.state[i+8] < data[i] {
			temp = 1
		} else if d.state[i+8] == data[i] {
			// keep temp
		} else {
			temp = 0
		}
	}
	d.gostCompress(data)
}

func (d *gostDigest) gostCompress(m [8]uint32) {
	tables := d.tables()
	h := &d.state

	var l, r, t uint32
	var key, u, v, w, s [8]uint32

	copy(u[:], h[:8])
	copy(v[:], m[:])

	for i := 0; i < 8; i += 2 {
		// X
		for j := 0; j < 8; j++ {
			w[j] = u[j] ^ v[j]
		}
		// P
		key[0] = (w[0] & 0x000000ff) | ((w[2] & 0x000000ff) << 8) | ((w[4] & 0x000000ff) << 16) | ((w[6] & 0x000000ff) << 24)
		key[1] = ((w[0] & 0x0000ff00) >> 8) | (w[2] & 0x0000ff00) | ((w[4] & 0x0000ff00) << 8) | ((w[6] & 0x0000ff00) << 16)
		key[2] = ((w[0] & 0x00ff0000) >> 16) | ((w[2] & 0x00ff0000) >> 8) | (w[4] & 0x00ff0000) | ((w[6] & 0x00ff0000) << 8)
		key[3] = ((w[0] & 0xff000000) >> 24) | ((w[2] & 0xff000000) >> 16) | ((w[4] & 0xff000000) >> 8) | (w[6] & 0xff000000)
		key[4] = (w[1] & 0x000000ff) | ((w[3] & 0x000000ff) << 8) | ((w[5] & 0x000000ff) << 16) | ((w[7] & 0x000000ff) << 24)
		key[5] = ((w[1] & 0x0000ff00) >> 8) | (w[3] & 0x0000ff00) | ((w[5] & 0x0000ff00) << 8) | ((w[7] & 0x0000ff00) << 16)
		key[6] = ((w[1] & 0x00ff0000) >> 16) | ((w[3] & 0x00ff0000) >> 8) | (w[5] & 0x00ff0000) | ((w[7] & 0x00ff0000) << 8)
		key[7] = ((w[1] & 0xff000000) >> 24) | ((w[3] & 0xff000000) >> 16) | ((w[5] & 0xff000000) >> 8) | (w[7] & 0xff000000)

		// R - block cipher
		r = h[i]
		l = h[i+1]
		gostRound(tables, key[0], key[1], &l, &r, &t)
		gostRound(tables, key[2], key[3], &l, &r, &t)
		gostRound(tables, key[4], key[5], &l, &r, &t)
		gostRound(tables, key[6], key[7], &l, &r, &t)
		gostRound(tables, key[0], key[1], &l, &r, &t)
		gostRound(tables, key[2], key[3], &l, &r, &t)
		gostRound(tables, key[4], key[5], &l, &r, &t)
		gostRound(tables, key[6], key[7], &l, &r, &t)
		gostRound(tables, key[0], key[1], &l, &r, &t)
		gostRound(tables, key[2], key[3], &l, &r, &t)
		gostRound(tables, key[4], key[5], &l, &r, &t)
		gostRound(tables, key[6], key[7], &l, &r, &t)
		gostRound(tables, key[7], key[6], &l, &r, &t)
		gostRound(tables, key[5], key[4], &l, &r, &t)
		gostRound(tables, key[3], key[2], &l, &r, &t)
		gostRound(tables, key[1], key[0], &l, &r, &t)
		t = r; r = l; l = t

		// S: s[i] = r, s[i+1] = l (PHP macro: s[i] = r; s[i+1] = l)
		s[i] = r
		s[i+1] = l

		if i != 6 {
			// A(u)
			al := u[0] ^ u[2]
			ar := u[1] ^ u[3]
			u[0] = u[2]; u[1] = u[3]; u[2] = u[4]; u[3] = u[5]
			u[4] = u[6]; u[5] = u[7]; u[6] = al; u[7] = ar
			if i == 2 {
				// C(u)
				u[0] ^= 0xff00ff00; u[1] ^= 0xff00ff00; u[2] ^= 0x00ff00ff; u[3] ^= 0x00ff00ff
				u[4] ^= 0x00ffff00; u[5] ^= 0xff0000ff; u[6] ^= 0x000000ff; u[7] ^= 0xff00ffff
			}
			// AA(v)
			al = v[0]; ar = v[2]
			v[0] = v[4]; v[2] = v[6]; v[4] = al ^ ar; v[6] = v[0] ^ ar
			al = v[1]; ar = v[3]
			v[1] = v[5]; v[3] = v[7]; v[5] = al ^ ar; v[7] = v[1] ^ ar
		}
	}

	// SHIFT12
	var u2 [8]uint32
	u2[0] = m[0] ^ s[6]
	u2[1] = m[1] ^ s[7]
	u2[2] = m[2] ^ (s[0]<<16) ^ (s[0]>>16) ^ (s[0]&0xffff) ^
		(s[1]&0xffff) ^ (s[1]>>16) ^ (s[2]<<16) ^ s[6] ^ (s[6]<<16) ^
		(s[7]&0xffff0000) ^ (s[7]>>16)
	u2[3] = m[3] ^ (s[0]&0xffff) ^ (s[0]<<16) ^ (s[1]&0xffff) ^
		(s[1]<<16) ^ (s[1]>>16) ^ (s[2]<<16) ^ (s[2]>>16) ^
		(s[3]<<16) ^ s[6] ^ (s[6]<<16) ^ (s[6]>>16) ^ (s[7]&0xffff) ^
		(s[7]<<16) ^ (s[7]>>16)
	u2[4] = m[4] ^
		(s[0]&0xffff0000) ^ (s[0]<<16) ^ (s[0]>>16) ^
		(s[1]&0xffff0000) ^ (s[1]>>16) ^ (s[2]<<16) ^ (s[2]>>16) ^
		(s[3]<<16) ^ (s[3]>>16) ^ (s[4]<<16) ^ (s[6]<<16) ^
		(s[6]>>16) ^ (s[7]&0xffff) ^ (s[7]<<16) ^ (s[7]>>16)
	u2[5] = m[5] ^ (s[0]<<16) ^ (s[0]>>16) ^ (s[0]&0xffff0000) ^
		(s[1]&0xffff) ^ s[2] ^ (s[2]>>16) ^ (s[3]<<16) ^ (s[3]>>16) ^
		(s[4]<<16) ^ (s[4]>>16) ^ (s[5]<<16) ^ (s[6]<<16) ^
		(s[6]>>16) ^ (s[7]&0xffff0000) ^ (s[7]<<16) ^ (s[7]>>16)
	u2[6] = m[6] ^ s[0] ^ (s[1]>>16) ^ (s[2]<<16) ^ s[3] ^ (s[3]>>16) ^
		(s[4]<<16) ^ (s[4]>>16) ^ (s[5]<<16) ^ (s[5]>>16) ^ s[6] ^
		(s[6]<<16) ^ (s[6]>>16) ^ (s[7]<<16)
	u2[7] = m[7] ^ (s[0]&0xffff0000) ^ (s[0]<<16) ^ (s[1]&0xffff) ^
		(s[1]<<16) ^ (s[2]>>16) ^ (s[3]<<16) ^ s[4] ^ (s[4]>>16) ^
		(s[5]<<16) ^ (s[5]>>16) ^ (s[6]>>16) ^ (s[7]&0xffff) ^
		(s[7]<<16) ^ (s[7]>>16)

	// SHIFT16: v = h XOR shifted u
	var v2 [8]uint32
	v2[0] = h[0] ^ (u2[1] << 16) ^ (u2[0] >> 16)
	v2[1] = h[1] ^ (u2[2] << 16) ^ (u2[1] >> 16)
	v2[2] = h[2] ^ (u2[3] << 16) ^ (u2[2] >> 16)
	v2[3] = h[3] ^ (u2[4] << 16) ^ (u2[3] >> 16)
	v2[4] = h[4] ^ (u2[5] << 16) ^ (u2[4] >> 16)
	v2[5] = h[5] ^ (u2[6] << 16) ^ (u2[5] >> 16)
	v2[6] = h[6] ^ (u2[7] << 16) ^ (u2[6] >> 16)
	v2[7] = h[7] ^ (u2[0]&0xffff0000) ^ (u2[0] << 16) ^ (u2[7] >> 16) ^ (u2[1] & 0xffff0000) ^ (u2[1] << 16) ^ (u2[6] << 16) ^ (u2[7] & 0xffff0000)

	// SHIFT61: h = shifted v
	h[0] = (v2[0] & 0xffff0000) ^ (v2[0] << 16) ^ (v2[0] >> 16) ^ (v2[1] >> 16) ^ (v2[1] & 0xffff0000) ^ (v2[2] << 16) ^ (v2[3] >> 16) ^ (v2[4] << 16) ^ (v2[5] >> 16) ^ v2[5] ^ (v2[6] >> 16) ^ (v2[7] << 16) ^ (v2[7] >> 16) ^ (v2[7] & 0xffff)
	h[1] = (v2[0] << 16) ^ (v2[0] >> 16) ^ (v2[0] & 0xffff0000) ^ (v2[1] & 0xffff) ^ v2[2] ^ (v2[2] >> 16) ^ (v2[3] << 16) ^ (v2[4] >> 16) ^ (v2[5] << 16) ^ (v2[6] << 16) ^ v2[6] ^ (v2[7] & 0xffff0000) ^ (v2[7] >> 16)
	h[2] = (v2[0] & 0xffff) ^ (v2[0] << 16) ^ (v2[1] << 16) ^ (v2[1] >> 16) ^ (v2[1] & 0xffff0000) ^ (v2[2] << 16) ^ (v2[3] >> 16) ^ v2[3] ^ (v2[4] << 16) ^ (v2[5] >> 16) ^ v2[6] ^ (v2[6] >> 16) ^ (v2[7] & 0xffff) ^ (v2[7] << 16) ^ (v2[7] >> 16)
	h[3] = (v2[0] << 16) ^ (v2[0] >> 16) ^ (v2[0] & 0xffff0000) ^ (v2[1] & 0xffff0000) ^ (v2[1] >> 16) ^ (v2[2] << 16) ^ (v2[2] >> 16) ^ v2[2] ^ (v2[3] << 16) ^ (v2[4] >> 16) ^ v2[4] ^ (v2[5] << 16) ^ (v2[6] << 16) ^ (v2[7] & 0xffff) ^ (v2[7] >> 16)
	h[4] = (v2[0] >> 16) ^ (v2[1] << 16) ^ v2[1] ^ (v2[2] >> 16) ^ v2[2] ^ (v2[3] << 16) ^ (v2[3] >> 16) ^ v2[3] ^ (v2[4] << 16) ^ (v2[5] >> 16) ^ v2[5] ^ (v2[6] << 16) ^ (v2[6] >> 16) ^ (v2[7] << 16)
	h[5] = (v2[0] << 16) ^ (v2[0] & 0xffff0000) ^ (v2[1] << 16) ^ (v2[1] >> 16) ^ (v2[1] & 0xffff0000) ^ (v2[2] << 16) ^ v2[2] ^ (v2[3] >> 16) ^ v2[3] ^ (v2[4] << 16) ^ (v2[4] >> 16) ^ v2[4] ^ (v2[5] << 16) ^ (v2[6] << 16) ^ (v2[6] >> 16) ^ v2[6] ^ (v2[7] << 16) ^ (v2[7] >> 16) ^ (v2[7] & 0xffff0000)
	h[6] = v2[0] ^ v2[2] ^ (v2[2] >> 16) ^ v2[3] ^ (v2[3] << 16) ^ v2[4] ^ (v2[4] >> 16) ^ (v2[5] << 16) ^ (v2[5] >> 16) ^ v2[5] ^ (v2[6] << 16) ^ (v2[6] >> 16) ^ v2[6] ^ (v2[7] << 16) ^ v2[7]
	h[7] = v2[0] ^ (v2[0] >> 16) ^ (v2[1] << 16) ^ (v2[1] >> 16) ^ (v2[2] << 16) ^ (v2[3] >> 16) ^ v2[3] ^ (v2[4] << 16) ^ v2[4] ^ (v2[5] >> 16) ^ v2[5] ^ (v2[6] << 16) ^ (v2[6] >> 16) ^ (v2[7] << 16) ^ v2[7]
}

func gostRound(tables *[4][256]uint32, k1, k2 uint32, l, r, t *uint32) {
	*t = k1 + *r
	*l ^= tables[0][*t&0xff] ^ tables[1][(*t>>8)&0xff] ^ tables[2][(*t>>16)&0xff] ^ tables[3][*t>>24]
	*t = k2 + *l
	*r ^= tables[0][*t&0xff] ^ tables[1][(*t>>8)&0xff] ^ tables[2][(*t>>16)&0xff] ^ tables[3][*t>>24]
}

func (d *gostDigest) CloneHash() hash.Hash {
	c := *d
	return &c
}

func (d *gostDigest) Sum(in []byte) []byte {
	d0 := *d
	hash := d0.checkSum()
	return append(in, hash[:]...)
}

func (d *gostDigest) checkSum() [32]byte {
	if d.bufLen > 0 {
		d.gostTransform(d.buf[:])
	}

	var l [8]uint32
	copy(l[:2], d.state[0:2]) // count is count0, count1
	l[0] = d.count0
	l[1] = d.count1
	d.gostCompress(l)

	copy(l[:], d.state[8:16])
	d.gostCompress(l)

	var digest [32]byte
	for i, j := 0, 0; j < 32; i++ {
		digest[j] = byte(d.state[i] & 0xff)
		digest[j+1] = byte((d.state[i] >> 8) & 0xff)
		digest[j+2] = byte((d.state[i] >> 16) & 0xff)
		digest[j+3] = byte((d.state[i] >> 24) & 0xff)
		j += 4
	}
	return digest
}
