package standard

import (
	"crypto/sha256"
	"crypto/sha512"
	"hash"
	"strconv"
	"strings"
)

// SHA-crypt parameters, per Ulrich Drepper's specification:
//   http://www.akkadia.org/drepper/SHA-crypt.txt
const (
	shaCryptDefaultRounds = 5000
	shaCryptMinRounds     = 1000
	shaCryptMaxRounds     = 999999999
	shaCryptMaxSaltLen    = 16
)

// shaCryptParams captures the "rounds=N$" prefix PHP/Glibc allow on a
// SHA-crypt salt, along with the rest of the salt itself.
type shaCryptParams struct {
	rounds       int
	roundsInSalt bool
	salt         string
}

func parseSHACryptSalt(saltInput, prefix string) (shaCryptParams, bool) {
	if !strings.HasPrefix(saltInput, prefix) {
		return shaCryptParams{}, false
	}
	rest := saltInput[len(prefix):]

	params := shaCryptParams{rounds: shaCryptDefaultRounds}
	if strings.HasPrefix(rest, "rounds=") {
		rest = rest[len("rounds="):]
		i := strings.IndexByte(rest, '$')
		if i < 0 {
			return shaCryptParams{}, false
		}
		r, err := strconv.Atoi(rest[:i])
		if err != nil {
			return shaCryptParams{}, false
		}
		// Clamp into the POSIX-documented range.
		if r < shaCryptMinRounds {
			r = shaCryptMinRounds
		} else if r > shaCryptMaxRounds {
			r = shaCryptMaxRounds
		}
		params.rounds = r
		params.roundsInSalt = true
		rest = rest[i+1:]
	}

	// Salt runs until the next '$' or end of string, capped at 16 chars.
	salt := rest
	if i := strings.IndexByte(salt, '$'); i >= 0 {
		salt = salt[:i]
	}
	if len(salt) > shaCryptMaxSaltLen {
		salt = salt[:shaCryptMaxSaltLen]
	}
	params.salt = salt
	return params, true
}

// shaCryptCore runs Drepper's algorithm with an arbitrary hash and returns
// the raw digest (32 bytes for SHA-256, 64 for SHA-512).
func shaCryptCore(newHash func() hash.Hash, password, salt []byte, rounds int) []byte {
	size := newHash().Size()

	// --- Digest A ---
	// A = H(password + salt + H(password + salt + password)[:|password|])
	b := newHash()
	b.Write(password)
	b.Write(salt)
	b.Write(password)
	digestB := b.Sum(nil)

	a := newHash()
	a.Write(password)
	a.Write(salt)

	// Append |password| bytes of digestB, repeating as needed.
	for i := len(password); i > 0; {
		if i >= size {
			a.Write(digestB)
			i -= size
		} else {
			a.Write(digestB[:i])
			i = 0
		}
	}

	// For each bit of |password| (high bits first — Drepper: right-shift),
	// append either digestB or the password. 1-bit → digestB, 0-bit →
	// password.
	for bits := len(password); bits > 0; bits >>= 1 {
		if bits&1 != 0 {
			a.Write(digestB)
		} else {
			a.Write(password)
		}
	}
	digestA := a.Sum(nil)

	// --- Digest DP: password expanded to its own length ---
	dp := newHash()
	for i := 0; i < len(password); i++ {
		dp.Write(password)
	}
	digestDP := dp.Sum(nil)
	// Sequence P is the first |password| bytes of DP, repeating.
	p := extendTo(digestDP, len(password), size)

	// --- Digest DS: salt expanded to (16 + A[0]) repetitions ---
	ds := newHash()
	iterations := 16 + int(digestA[0])
	for i := 0; i < iterations; i++ {
		ds.Write(salt)
	}
	digestDS := ds.Sum(nil)
	// Sequence S is the first |salt| bytes of DS, repeating.
	s := extendTo(digestDS, len(salt), size)

	// --- Strengthening loop ---
	cur := digestA
	for i := 0; i < rounds; i++ {
		c := newHash()
		if i&1 != 0 {
			c.Write(p)
		} else {
			c.Write(cur)
		}
		if i%3 != 0 {
			c.Write(s)
		}
		if i%7 != 0 {
			c.Write(p)
		}
		if i&1 != 0 {
			c.Write(cur)
		} else {
			c.Write(p)
		}
		cur = c.Sum(nil)
	}
	return cur
}

// extendTo returns a slice of length target bytes, produced by repeating the
// first hashSize bytes of src until target bytes have been emitted. This is
// Drepper's "sequence P" / "sequence S" construction.
func extendTo(src []byte, target, hashSize int) []byte {
	out := make([]byte, target)
	for i := 0; i < target; i += hashSize {
		n := hashSize
		if i+n > target {
			n = target - i
		}
		copy(out[i:], src[:n])
	}
	return out
}

// cryptSHA256 implements the $5$ SHA-256-crypt algorithm.
func cryptSHA256(password, salt string) string {
	p, ok := parseSHACryptSalt(salt, "$5$")
	if !ok {
		return cryptFailure
	}
	digest := shaCryptCore(sha256.New, []byte(password), []byte(p.salt), p.rounds)

	// SHA-256-crypt encodes 20 groups of 3 bytes + 1 group of 2 bytes,
	// with Drepper's specific index permutation.
	var out strings.Builder
	out.WriteString("$5$")
	if p.roundsInSalt {
		out.WriteString("rounds=")
		out.WriteString(strconv.Itoa(p.rounds))
		out.WriteByte('$')
	}
	out.WriteString(p.salt)
	out.WriteByte('$')
	enc := func(b2, b1, b0 byte, n int) { encodeCryptBase64(&out, b2, b1, b0, n) }
	enc(digest[0], digest[10], digest[20], 4)
	enc(digest[21], digest[1], digest[11], 4)
	enc(digest[12], digest[22], digest[2], 4)
	enc(digest[3], digest[13], digest[23], 4)
	enc(digest[24], digest[4], digest[14], 4)
	enc(digest[15], digest[25], digest[5], 4)
	enc(digest[6], digest[16], digest[26], 4)
	enc(digest[27], digest[7], digest[17], 4)
	enc(digest[18], digest[28], digest[8], 4)
	enc(digest[9], digest[19], digest[29], 4)
	enc(0, digest[31], digest[30], 3)
	return out.String()
}

// cryptSHA512 implements the $6$ SHA-512-crypt algorithm.
func cryptSHA512(password, salt string) string {
	p, ok := parseSHACryptSalt(salt, "$6$")
	if !ok {
		return cryptFailure
	}
	digest := shaCryptCore(sha512.New, []byte(password), []byte(p.salt), p.rounds)

	// SHA-512-crypt has its own 21-group permutation; last group is 2
	// bytes emitted as 2 digits (1 byte promoted to the MS position).
	var out strings.Builder
	out.WriteString("$6$")
	if p.roundsInSalt {
		out.WriteString("rounds=")
		out.WriteString(strconv.Itoa(p.rounds))
		out.WriteByte('$')
	}
	out.WriteString(p.salt)
	out.WriteByte('$')
	enc := func(b2, b1, b0 byte, n int) { encodeCryptBase64(&out, b2, b1, b0, n) }
	enc(digest[0], digest[21], digest[42], 4)
	enc(digest[22], digest[43], digest[1], 4)
	enc(digest[44], digest[2], digest[23], 4)
	enc(digest[3], digest[24], digest[45], 4)
	enc(digest[25], digest[46], digest[4], 4)
	enc(digest[47], digest[5], digest[26], 4)
	enc(digest[6], digest[27], digest[48], 4)
	enc(digest[28], digest[49], digest[7], 4)
	enc(digest[50], digest[8], digest[29], 4)
	enc(digest[9], digest[30], digest[51], 4)
	enc(digest[31], digest[52], digest[10], 4)
	enc(digest[53], digest[11], digest[32], 4)
	enc(digest[12], digest[33], digest[54], 4)
	enc(digest[34], digest[55], digest[13], 4)
	enc(digest[56], digest[14], digest[35], 4)
	enc(digest[15], digest[36], digest[57], 4)
	enc(digest[37], digest[58], digest[16], 4)
	enc(digest[59], digest[17], digest[38], 4)
	enc(digest[18], digest[39], digest[60], 4)
	enc(digest[40], digest[61], digest[19], 4)
	enc(digest[62], digest[20], digest[41], 4)
	enc(0, 0, digest[63], 2)
	return out.String()
}
