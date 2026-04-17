package standard

import (
	"encoding/base64"
	"strconv"

	"golang.org/x/crypto/blowfish"
)

// bcryptAlphabet is the radix-64 alphabet bcrypt uses for its encoded salt
// and hash. It differs from standard base64: the first two digits are "./"
// instead of "+/", and there is no padding.
const bcryptAlphabet = "./ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789"

var bcryptEnc = base64.NewEncoding(bcryptAlphabet).WithPadding(base64.NoPadding)

// magicCipherData is the plaintext bcrypt encrypts 64 times to derive the
// hash. It spells "OrpheanBeholderScryDoubt" in big-endian ASCII.
var magicCipherData = []byte("OrpheanBeholderScryDoubt")

// cryptBcrypt hashes password under the bcrypt algorithm using the given
// salt. salt may be either a full 60-char bcrypt hash (the common
// "verification" pattern, where the caller wants crypt($pw, $hash) == $hash
// when $pw is correct) or the 29-char prefix "$2X$NN$SSSSSSSSSSSSSSSSSSSSSS".
// On any format or parameter error it returns cryptFailure.
func cryptBcrypt(password, salt string) string {
	// Minimum useful salt length: "$2X$NN$SSSSSSSSSSSSSSSSSSSSSS" = 29.
	if len(salt) < 29 {
		return cryptFailure
	}
	// The dispatcher has already validated salt[0..=3]; we're told the
	// minor character ('a' | 'b' | 'x' | 'y') is known-good.
	minor := salt[2]

	// Cost: two ASCII digits followed by '$'.
	if salt[6] != '$' {
		return cryptFailure
	}
	cost, err := strconv.Atoi(salt[4:6])
	if err != nil || cost < 4 || cost > 31 {
		return cryptFailure
	}

	// Exactly 22 chars of base64 salt follow the cost. PHP rejects any '$'
	// in that window — it means the caller passed a malformed salt rather
	// than a full hash.
	saltEncoded := salt[7:29]
	for i := 0; i < len(saltEncoded); i++ {
		if saltEncoded[i] == '$' {
			return cryptFailure
		}
	}

	saltRaw, err := bcryptBase64Decode([]byte(saltEncoded))
	if err != nil || len(saltRaw) < 16 {
		return cryptFailure
	}
	// Bcrypt only ever uses the first 16 bytes of salt.
	saltRaw = saltRaw[:16]

	hash, err := bcryptHash([]byte(password), cost, saltRaw)
	if err != nil {
		return cryptFailure
	}

	// Produce the canonical 60-char output:
	//   $2<minor>$NN$<22-char salt><31-char hash>
	var out [60]byte
	copy(out[:], "$2")
	out[2] = minor
	out[3] = '$'
	out[4] = '0' + byte(cost/10)
	out[5] = '0' + byte(cost%10)
	out[6] = '$'
	copy(out[7:], saltEncoded)
	copy(out[29:], bcryptBase64Encode(hash))
	return string(out[:])
}

// bcryptHash is the Provos-Mazières hashing function. It derives a 23-byte
// hash from password, cost (log2 rounds), and a 16-byte raw salt.
func bcryptHash(password []byte, cost int, salt []byte) ([]byte, error) {
	// Append the trailing NUL that reference bcrypt implementations
	// include during key expansion. Copy first so we don't disturb the
	// caller's slice.
	keyWithNul := make([]byte, len(password)+1)
	copy(keyWithNul, password)

	cipher, err := blowfish.NewSaltedCipher(keyWithNul, salt)
	if err != nil {
		return nil, err
	}

	rounds := uint64(1) << uint(cost)
	for i := uint64(0); i < rounds; i++ {
		blowfish.ExpandKey(keyWithNul, cipher)
		blowfish.ExpandKey(salt, cipher)
	}

	buf := make([]byte, len(magicCipherData))
	copy(buf, magicCipherData)

	for i := 0; i < len(buf); i += blowfish.BlockSize {
		for j := 0; j < 64; j++ {
			cipher.Encrypt(buf[i:i+blowfish.BlockSize], buf[i:i+blowfish.BlockSize])
		}
	}

	// Reference bcrypt encodes only 23 of the 24 encrypted bytes.
	return buf[:23], nil
}

func bcryptBase64Encode(src []byte) []byte {
	n := bcryptEnc.EncodedLen(len(src))
	dst := make([]byte, n)
	bcryptEnc.Encode(dst, src)
	return dst
}

func bcryptBase64Decode(src []byte) ([]byte, error) {
	// Pad up to a multiple of 4 so encoding/base64 is happy even with a
	// fixed-width bcrypt salt.
	pad := (4 - len(src)%4) % 4
	if pad > 0 {
		padded := make([]byte, 0, len(src)+pad)
		padded = append(padded, src...)
		for i := 0; i < pad; i++ {
			padded = append(padded, '=')
		}
		src = padded
	}
	dst := make([]byte, base64.NewEncoding(bcryptAlphabet).DecodedLen(len(src)))
	n, err := base64.NewEncoding(bcryptAlphabet).Decode(dst, src)
	if err != nil {
		return nil, err
	}
	return dst[:n], nil
}
