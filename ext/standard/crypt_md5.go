package standard

import (
	"crypto/md5"
	"strings"
)

// cryptMD5 implements FreeBSD-style MD5-crypt ($1$). The algorithm is the
// same one glibc and OpenBSD use. See crypt(3) or Poul-Henning Kamp's
// original write-up for the full derivation.
func cryptMD5(password, salt string) string {
	if !strings.HasPrefix(salt, "$1$") {
		return cryptFailure
	}

	// Salt ends at the next '$' or the end of string, up to 8 chars.
	saltStr := salt[3:]
	if i := strings.IndexByte(saltStr, '$'); i >= 0 {
		saltStr = saltStr[:i]
	}
	if len(saltStr) > 8 {
		saltStr = saltStr[:8]
	}

	pwBytes := []byte(password)
	saltBytes := []byte(saltStr)

	// Step 1. Initial digest: password + magic + salt.
	d := md5.New()
	d.Write(pwBytes)
	d.Write([]byte("$1$"))
	d.Write(saltBytes)

	// Step 2. Alternate digest: password + salt + password, hashed once.
	alt := md5.New()
	alt.Write(pwBytes)
	alt.Write(saltBytes)
	alt.Write(pwBytes)
	altSum := alt.Sum(nil)

	// Step 3. Fold altSum into d, len(password) bytes at a time.
	for i := len(pwBytes); i > 0; i -= md5.Size {
		if i > md5.Size {
			d.Write(altSum)
		} else {
			d.Write(altSum[:i])
		}
	}

	// Step 4. For each 1-bit of len(password) from LSB upward, append a
	// zero byte; for each 0-bit, append the first byte of password. This
	// is the historical weird bit that makes MD5-crypt incompatible with
	// a simple HMAC construction.
	for i := len(pwBytes); i != 0; i >>= 1 {
		if i&1 != 0 {
			d.Write([]byte{0})
		} else {
			d.Write(pwBytes[:1])
		}
	}
	sum := d.Sum(nil)

	// Step 5. 1000-round strengthening loop. Each round re-hashes sum with
	// either the password or the salt prepended/appended in a pattern that
	// depends on the round index.
	for i := 0; i < 1000; i++ {
		r := md5.New()
		if i&1 != 0 {
			r.Write(pwBytes)
		} else {
			r.Write(sum)
		}
		if i%3 != 0 {
			r.Write(saltBytes)
		}
		if i%7 != 0 {
			r.Write(pwBytes)
		}
		if i&1 != 0 {
			r.Write(sum)
		} else {
			r.Write(pwBytes)
		}
		sum = r.Sum(nil)
	}

	// Step 6. Encode in the interleaved order MD5-crypt uses. The source
	// byte permutation matches glibc's implementation.
	var out strings.Builder
	out.WriteString("$1$")
	out.WriteString(saltStr)
	out.WriteByte('$')
	encodeCryptBase64(&out, sum[0], sum[6], sum[12], 4)
	encodeCryptBase64(&out, sum[1], sum[7], sum[13], 4)
	encodeCryptBase64(&out, sum[2], sum[8], sum[14], 4)
	encodeCryptBase64(&out, sum[3], sum[9], sum[15], 4)
	encodeCryptBase64(&out, sum[4], sum[10], sum[5], 4)
	encodeCryptBase64(&out, 0, 0, sum[11], 2)
	return out.String()
}

// cryptSaltAlphabet is the radix-64 alphabet shared by MD5-crypt and
// SHA-crypt output encoding. It differs from the bcrypt alphabet by the
// order of the first two characters.
const cryptSaltAlphabet = "./0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"

// encodeCryptBase64 writes n 6-bit digits (little-endian) representing the
// 24-bit group (b2<<16 | b1<<8 | b0) using the MD5/SHA crypt alphabet.
func encodeCryptBase64(dst *strings.Builder, b2, b1, b0 byte, n int) {
	w := uint32(b2)<<16 | uint32(b1)<<8 | uint32(b0)
	for ; n > 0; n-- {
		dst.WriteByte(cryptSaltAlphabet[w&0x3f])
		w >>= 6
	}
}
