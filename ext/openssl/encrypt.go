package openssl

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/des"
	"encoding/base64"
	"fmt"
	"sort"
	"strings"

	"github.com/MagicalTux/goro/core"
	"github.com/MagicalTux/goro/core/phpv"
)

// cipherInfo describes a symmetric cipher configuration.
type cipherInfo struct {
	keyLen  int // key length in bytes
	ivLen   int // IV length in bytes
	mode    string // "cbc", "ecb", "ctr", "cfb", "ofb", "gcm"
	newFunc func(key []byte) (cipher.Block, error)
}

// cipherRegistry maps lowercase cipher method names to their configuration.
var cipherRegistry map[string]*cipherInfo

func init() {
	cipherRegistry = make(map[string]*cipherInfo)

	// AES ciphers
	for _, bits := range []int{128, 192, 256} {
		keyLen := bits / 8
		for _, mode := range []string{"cbc", "ecb", "ctr", "cfb", "ofb", "gcm"} {
			name := fmt.Sprintf("aes-%d-%s", bits, mode)
			ivLen := aes.BlockSize
			if mode == "ecb" {
				ivLen = 0
			} else if mode == "gcm" {
				ivLen = 12 // standard GCM nonce size
			}
			cipherRegistry[name] = &cipherInfo{
				keyLen:  keyLen,
				ivLen:   ivLen,
				mode:    mode,
				newFunc: aes.NewCipher,
			}
		}
	}

	// DES ciphers
	cipherRegistry["des-cbc"] = &cipherInfo{keyLen: 8, ivLen: des.BlockSize, mode: "cbc", newFunc: des.NewCipher}
	cipherRegistry["des-ecb"] = &cipherInfo{keyLen: 8, ivLen: 0, mode: "ecb", newFunc: des.NewCipher}
	cipherRegistry["des-cfb"] = &cipherInfo{keyLen: 8, ivLen: des.BlockSize, mode: "cfb", newFunc: des.NewCipher}
	cipherRegistry["des-ofb"] = &cipherInfo{keyLen: 8, ivLen: des.BlockSize, mode: "ofb", newFunc: des.NewCipher}

	// Triple DES (DES-EDE3) ciphers
	cipherRegistry["des-ede3-cbc"] = &cipherInfo{keyLen: 24, ivLen: des.BlockSize, mode: "cbc", newFunc: des.NewTripleDESCipher}
	cipherRegistry["des-ede3-ecb"] = &cipherInfo{keyLen: 24, ivLen: 0, mode: "ecb", newFunc: des.NewTripleDESCipher}
	cipherRegistry["des-ede3-cfb"] = &cipherInfo{keyLen: 24, ivLen: des.BlockSize, mode: "cfb", newFunc: des.NewTripleDESCipher}
	cipherRegistry["des-ede3-ofb"] = &cipherInfo{keyLen: 24, ivLen: des.BlockSize, mode: "ofb", newFunc: des.NewTripleDESCipher}
}

// adjustKey pads or truncates the key to the required length.
// PHP zero-pads short keys and truncates long keys.
func adjustKey(key []byte, required int) []byte {
	if len(key) >= required {
		return key[:required]
	}
	padded := make([]byte, required)
	copy(padded, key)
	return padded
}

// adjustIV pads or truncates the IV to the required length.
func adjustIV(iv []byte, required int) []byte {
	if required == 0 {
		return nil
	}
	if len(iv) >= required {
		return iv[:required]
	}
	padded := make([]byte, required)
	copy(padded, iv)
	return padded
}

// pkcs7Pad applies PKCS#7 padding to data.
func pkcs7Pad(data []byte, blockSize int) []byte {
	padding := blockSize - (len(data) % blockSize)
	pad := make([]byte, padding)
	for i := range pad {
		pad[i] = byte(padding)
	}
	return append(data, pad...)
}

// pkcs7Unpad removes PKCS#7 padding from data.
func pkcs7Unpad(data []byte, blockSize int) ([]byte, error) {
	if len(data) == 0 {
		return data, nil
	}
	if len(data)%blockSize != 0 {
		return nil, fmt.Errorf("data length is not a multiple of block size")
	}
	padding := int(data[len(data)-1])
	if padding == 0 || padding > blockSize {
		return nil, fmt.Errorf("invalid padding")
	}
	for i := len(data) - padding; i < len(data); i++ {
		if data[i] != byte(padding) {
			return nil, fmt.Errorf("invalid padding")
		}
	}
	return data[:len(data)-padding], nil
}

// > func string openssl_encrypt ( string $data , string $method , string $key [, int $options = 0 [, string $iv = "" [, string &$tag = NULL [, string $aad = "" [, int $tag_length = 16 ]]]]] )
func fncOpensslEncrypt(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var data phpv.ZString
	var method phpv.ZString
	var key phpv.ZString
	var options *phpv.ZInt
	var iv *phpv.ZString

	_, err := core.Expand(ctx, args, &data, &method, &key, &options, &iv)
	if err != nil {
		return nil, err
	}

	opts := 0
	if options != nil {
		opts = int(*options)
	}

	ivBytes := []byte{}
	if iv != nil {
		ivBytes = []byte(*iv)
	}

	methodLower := strings.ToLower(string(method))
	ci, ok := cipherRegistry[methodLower]
	if !ok {
		ctx.Warn("openssl_encrypt(): Unknown cipher algorithm")
		return phpv.ZBool(false).ZVal(), nil
	}

	keyBytes := adjustKey([]byte(key), ci.keyLen)
	ivBytes = adjustIV(ivBytes, ci.ivLen)

	// Warn if IV length doesn't match expected
	if ci.ivLen > 0 && iv != nil && len(*iv) != ci.ivLen {
		ctx.Warn("openssl_encrypt(): Using an IV that is %d bytes long, cipher expects an IV of precisely %d bytes, padding with \\0", len(*iv), ci.ivLen)
	}

	block, err := ci.newFunc(keyBytes)
	if err != nil {
		ctx.Warn("openssl_encrypt(): %s", err.Error())
		return phpv.ZBool(false).ZVal(), nil
	}

	plaintext := []byte(data)

	var ciphertext []byte

	switch ci.mode {
	case "gcm":
		aead, err := cipher.NewGCM(block)
		if err != nil {
			ctx.Warn("openssl_encrypt(): %s", err.Error())
			return phpv.ZBool(false).ZVal(), nil
		}
		// For GCM, get AAD from args[6] if provided
		aad := []byte{}
		if len(args) > 6 {
			aadVal, err := args[6].As(ctx, phpv.ZtString)
			if err == nil {
				aad = []byte(aadVal.Value().(phpv.ZString))
			}
		}
		nonce := ivBytes
		if len(nonce) != aead.NonceSize() {
			nonce = adjustIV(nonce, aead.NonceSize())
		}
		sealed := aead.Seal(nil, nonce, plaintext, aad)
		// GCM appends tag to ciphertext; split them
		tagLen := 16
		if len(args) > 7 {
			tagLenVal, err := args[7].As(ctx, phpv.ZtInt)
			if err == nil {
				tagLen = int(tagLenVal.Value().(phpv.ZInt))
			}
		}
		if tagLen < 4 || tagLen > 16 {
			tagLen = 16
		}
		ciphertext = sealed[:len(sealed)-aead.Overhead()]
		tag := sealed[len(sealed)-aead.Overhead():]
		if len(tag) > tagLen {
			tag = tag[:tagLen]
		}
		// Set &$tag (args[5]) if provided
		if len(args) > 5 && args[5] != nil {
			tagStr := phpv.ZString(tag)
			name := args[5].GetName()
			tagZVal := tagStr.ZVal()
			tagZVal.Name = &name
			ctx.Parent(1).OffsetSet(ctx, name, tagZVal)
		}

	case "cbc":
		if opts&OPENSSL_ZERO_PADDING == 0 {
			plaintext = pkcs7Pad(plaintext, block.BlockSize())
		} else {
			// zero padding: pad to block size with zeros
			if len(plaintext)%block.BlockSize() != 0 {
				padded := make([]byte, len(plaintext)+(block.BlockSize()-len(plaintext)%block.BlockSize()))
				copy(padded, plaintext)
				plaintext = padded
			}
		}
		ciphertext = make([]byte, len(plaintext))
		mode := cipher.NewCBCEncrypter(block, ivBytes)
		mode.CryptBlocks(ciphertext, plaintext)

	case "ecb":
		if opts&OPENSSL_ZERO_PADDING == 0 {
			plaintext = pkcs7Pad(plaintext, block.BlockSize())
		} else {
			if len(plaintext)%block.BlockSize() != 0 {
				padded := make([]byte, len(plaintext)+(block.BlockSize()-len(plaintext)%block.BlockSize()))
				copy(padded, plaintext)
				plaintext = padded
			}
		}
		ciphertext = make([]byte, len(plaintext))
		for i := 0; i < len(plaintext); i += block.BlockSize() {
			block.Encrypt(ciphertext[i:i+block.BlockSize()], plaintext[i:i+block.BlockSize()])
		}

	case "ctr":
		ciphertext = make([]byte, len(plaintext))
		stream := cipher.NewCTR(block, ivBytes)
		stream.XORKeyStream(ciphertext, plaintext)

	case "cfb":
		ciphertext = make([]byte, len(plaintext))
		stream := cipher.NewCFBEncrypter(block, ivBytes)
		stream.XORKeyStream(ciphertext, plaintext)

	case "ofb":
		ciphertext = make([]byte, len(plaintext))
		stream := cipher.NewOFB(block, ivBytes)
		stream.XORKeyStream(ciphertext, plaintext)

	default:
		ctx.Warn("openssl_encrypt(): Unknown cipher mode")
		return phpv.ZBool(false).ZVal(), nil
	}

	if opts&OPENSSL_RAW_DATA != 0 {
		return phpv.ZString(ciphertext).ZVal(), nil
	}

	// base64 encode by default
	encoded := base64.StdEncoding.EncodeToString(ciphertext)
	return phpv.ZString(encoded).ZVal(), nil
}

// > func string openssl_decrypt ( string $data , string $method , string $key [, int $options = 0 [, string $iv = "" [, string $tag = "" [, string $aad = "" ]]]] )
func fncOpensslDecrypt(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var data phpv.ZString
	var method phpv.ZString
	var key phpv.ZString
	var options *phpv.ZInt
	var iv *phpv.ZString

	_, err := core.Expand(ctx, args, &data, &method, &key, &options, &iv)
	if err != nil {
		return nil, err
	}

	opts := 0
	if options != nil {
		opts = int(*options)
	}

	ivBytes := []byte{}
	if iv != nil {
		ivBytes = []byte(*iv)
	}

	methodLower := strings.ToLower(string(method))
	ci, ok := cipherRegistry[methodLower]
	if !ok {
		ctx.Warn("openssl_decrypt(): Unknown cipher algorithm")
		return phpv.ZBool(false).ZVal(), nil
	}

	keyBytes := adjustKey([]byte(key), ci.keyLen)
	ivBytes = adjustIV(ivBytes, ci.ivLen)

	// Decode input
	var ciphertext []byte
	if opts&OPENSSL_RAW_DATA != 0 {
		ciphertext = []byte(data)
	} else {
		decoded, err := base64.StdEncoding.DecodeString(string(data))
		if err != nil {
			ctx.Warn("openssl_decrypt(): Failed to base64 decode the input")
			return phpv.ZBool(false).ZVal(), nil
		}
		ciphertext = decoded
	}

	block, err := ci.newFunc(keyBytes)
	if err != nil {
		ctx.Warn("openssl_decrypt(): %s", err.Error())
		return phpv.ZBool(false).ZVal(), nil
	}

	var plaintext []byte

	switch ci.mode {
	case "gcm":
		aead, err := cipher.NewGCM(block)
		if err != nil {
			ctx.Warn("openssl_decrypt(): %s", err.Error())
			return phpv.ZBool(false).ZVal(), nil
		}
		// Get tag from args[5]
		tag := []byte{}
		if len(args) > 5 {
			tagVal, err := args[5].As(ctx, phpv.ZtString)
			if err == nil {
				tag = []byte(tagVal.Value().(phpv.ZString))
			}
		}
		// Get AAD from args[6]
		aad := []byte{}
		if len(args) > 6 {
			aadVal, err := args[6].As(ctx, phpv.ZtString)
			if err == nil {
				aad = []byte(aadVal.Value().(phpv.ZString))
			}
		}
		// Append tag to ciphertext for Go's GCM
		combined := append(ciphertext, tag...)
		nonce := ivBytes
		if len(nonce) != aead.NonceSize() {
			nonce = adjustIV(nonce, aead.NonceSize())
		}
		plaintext, err = aead.Open(nil, nonce, combined, aad)
		if err != nil {
			ctx.Warn("openssl_decrypt(): Authentication failed")
			return phpv.ZBool(false).ZVal(), nil
		}

	case "cbc":
		if len(ciphertext)%block.BlockSize() != 0 {
			ctx.Warn("openssl_decrypt(): Data is not a multiple of block size")
			return phpv.ZBool(false).ZVal(), nil
		}
		plaintext = make([]byte, len(ciphertext))
		mode := cipher.NewCBCDecrypter(block, ivBytes)
		mode.CryptBlocks(plaintext, ciphertext)
		if opts&OPENSSL_ZERO_PADDING == 0 {
			plaintext, err = pkcs7Unpad(plaintext, block.BlockSize())
			if err != nil {
				ctx.Warn("openssl_decrypt(): Padding is invalid")
				return phpv.ZBool(false).ZVal(), nil
			}
		}

	case "ecb":
		if len(ciphertext)%block.BlockSize() != 0 {
			ctx.Warn("openssl_decrypt(): Data is not a multiple of block size")
			return phpv.ZBool(false).ZVal(), nil
		}
		plaintext = make([]byte, len(ciphertext))
		for i := 0; i < len(ciphertext); i += block.BlockSize() {
			block.Decrypt(plaintext[i:i+block.BlockSize()], ciphertext[i:i+block.BlockSize()])
		}
		if opts&OPENSSL_ZERO_PADDING == 0 {
			plaintext, err = pkcs7Unpad(plaintext, block.BlockSize())
			if err != nil {
				ctx.Warn("openssl_decrypt(): Padding is invalid")
				return phpv.ZBool(false).ZVal(), nil
			}
		}

	case "ctr":
		plaintext = make([]byte, len(ciphertext))
		stream := cipher.NewCTR(block, ivBytes)
		stream.XORKeyStream(plaintext, ciphertext)

	case "cfb":
		plaintext = make([]byte, len(ciphertext))
		stream := cipher.NewCFBDecrypter(block, ivBytes)
		stream.XORKeyStream(plaintext, ciphertext)

	case "ofb":
		plaintext = make([]byte, len(ciphertext))
		stream := cipher.NewOFB(block, ivBytes)
		stream.XORKeyStream(plaintext, ciphertext)

	default:
		ctx.Warn("openssl_decrypt(): Unknown cipher mode")
		return phpv.ZBool(false).ZVal(), nil
	}

	return phpv.ZString(plaintext).ZVal(), nil
}

// > func array openssl_get_cipher_methods ([ bool $aliases = false ])
func fncOpensslGetCipherMethods(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var aliases *phpv.ZBool
	core.Expand(ctx, args, &aliases)

	// Collect cipher names
	names := make([]string, 0, len(cipherRegistry))
	for name := range cipherRegistry {
		names = append(names, name)
	}
	sort.Strings(names)

	a := phpv.NewZArray()
	for _, name := range names {
		a.OffsetSet(ctx, nil, phpv.ZString(name).ZVal())
	}

	// If aliases requested, add uppercase versions
	if aliases != nil && bool(*aliases) {
		for _, name := range names {
			upper := strings.ToUpper(name)
			a.OffsetSet(ctx, nil, phpv.ZString(upper).ZVal())
		}
	}

	return a.ZVal(), nil
}

// > func string openssl_cipher_iv_length ( string $method )
// This is commonly used alongside encrypt/decrypt. Let's include it too.
func fncOpensslCipherIvLength(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var method phpv.ZString

	_, err := core.Expand(ctx, args, &method)
	if err != nil {
		return nil, err
	}

	methodLower := strings.ToLower(string(method))
	ci, ok := cipherRegistry[methodLower]
	if !ok {
		ctx.Warn("openssl_cipher_iv_length(): Unknown cipher algorithm")
		return phpv.ZBool(false).ZVal(), nil
	}

	return phpv.ZInt(ci.ivLen).ZVal(), nil
}

// > func int openssl_cipher_key_length ( string $method )
func fncOpensslCipherKeyLength(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var method phpv.ZString

	_, err := core.Expand(ctx, args, &method)
	if err != nil {
		return nil, err
	}

	methodLower := strings.ToLower(string(method))
	ci, ok := cipherRegistry[methodLower]
	if !ok {
		ctx.Warn("openssl_cipher_key_length(): Unknown cipher algorithm")
		return phpv.ZBool(false).ZVal(), nil
	}

	return phpv.ZInt(ci.keyLen).ZVal(), nil
}
