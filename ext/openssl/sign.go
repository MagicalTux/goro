package openssl

import (
	"crypto"
	"crypto/ecdsa"
	"crypto/rand"
	"crypto/rsa"
	"fmt"

	"github.com/MagicalTux/goro/core"
	"github.com/MagicalTux/goro/core/phpv"
)

// getSignatureHash returns the crypto.Hash for a given OPENSSL_ALGO_* constant
// or string algorithm name.
func getSignatureHash(ctx phpv.Context, args []*phpv.ZVal, argIndex int) crypto.Hash {
	if len(args) <= argIndex || args[argIndex] == nil {
		return crypto.SHA1 // default
	}

	arg := args[argIndex]

	// Check if it's an integer (OPENSSL_ALGO_* constant)
	if arg.GetType() == phpv.ZtInt {
		switch int(arg.Value().(phpv.ZInt)) {
		case OPENSSL_ALGO_SHA1:
			return crypto.SHA1
		case OPENSSL_ALGO_SHA224:
			return crypto.SHA224
		case OPENSSL_ALGO_SHA256:
			return crypto.SHA256
		case OPENSSL_ALGO_SHA384:
			return crypto.SHA384
		case OPENSSL_ALGO_SHA512:
			return crypto.SHA512
		default:
			return crypto.SHA1
		}
	}

	// String algorithm name
	s, err := arg.As(ctx, phpv.ZtString)
	if err != nil {
		return crypto.SHA1
	}
	name := string(s.Value().(phpv.ZString))
	switch name {
	case "sha1", "SHA1":
		return crypto.SHA1
	case "sha224", "SHA224":
		return crypto.SHA224
	case "sha256", "SHA256":
		return crypto.SHA256
	case "sha384", "SHA384":
		return crypto.SHA384
	case "sha512", "SHA512":
		return crypto.SHA512
	case "md5", "MD5":
		return crypto.MD5
	default:
		return crypto.SHA1
	}
}

// > func bool openssl_sign ( string $data , string &$signature , OpenSSLAsymmetricKey|string $private_key [, int|string $algorithm = OPENSSL_ALGO_SHA1 ] )
func fncOpensslSign(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	if len(args) < 3 {
		return nil, fmt.Errorf("openssl_sign() expects at least 3 arguments")
	}

	var data phpv.ZString
	_, err := core.Expand(ctx, args, &data)
	if err != nil {
		return nil, err
	}

	pd, err := resolvePkey(ctx, args[2])
	if err != nil || pd == nil || pd.privKey == nil {
		ctx.Warn("openssl_sign(): Cannot get private key from the given parameter")
		return phpv.ZBool(false).ZVal(), nil
	}

	hashAlgo := getSignatureHash(ctx, args, 3)

	h := hashAlgo.New()
	h.Write([]byte(data))
	digest := h.Sum(nil)

	var signature []byte

	switch key := pd.privKey.(type) {
	case *rsa.PrivateKey:
		signature, err = rsa.SignPKCS1v15(rand.Reader, key, hashAlgo, digest)
		if err != nil {
			ctx.Warn("openssl_sign(): Signing failed: %s", err.Error())
			return phpv.ZBool(false).ZVal(), nil
		}
	case *ecdsa.PrivateKey:
		signature, err = ecdsa.SignASN1(rand.Reader, key, digest)
		if err != nil {
			ctx.Warn("openssl_sign(): Signing failed: %s", err.Error())
			return phpv.ZBool(false).ZVal(), nil
		}
	default:
		ctx.Warn("openssl_sign(): Unsupported key type for signing")
		return phpv.ZBool(false).ZVal(), nil
	}

	// Set &$signature (args[1])
	if len(args) > 1 && args[1] != nil {
		name := args[1].GetName()
		sigVal := phpv.ZString(signature).ZVal()
		sigVal.Name = &name
		ctx.Parent(1).OffsetSet(ctx, name, sigVal)
	}

	return phpv.ZBool(true).ZVal(), nil
}

// > func int openssl_verify ( string $data , string $signature , OpenSSLAsymmetricKey|string $public_key [, int|string $algorithm = OPENSSL_ALGO_SHA1 ] )
// Returns 1 on success, 0 on failure, -1 on error
func fncOpensslVerify(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	if len(args) < 3 {
		return nil, fmt.Errorf("openssl_verify() expects at least 3 arguments")
	}

	var data phpv.ZString
	var sig phpv.ZString
	_, err := core.Expand(ctx, args, &data, &sig)
	if err != nil {
		return nil, err
	}

	pd, err := resolvePkey(ctx, args[2])
	if err != nil || pd == nil || pd.pubKey == nil {
		ctx.Warn("openssl_verify(): Cannot get public key from the given parameter")
		return phpv.ZInt(-1).ZVal(), nil
	}

	hashAlgo := getSignatureHash(ctx, args, 3)

	h := hashAlgo.New()
	h.Write([]byte(data))
	digest := h.Sum(nil)

	switch key := pd.pubKey.(type) {
	case *rsa.PublicKey:
		err = rsa.VerifyPKCS1v15(key, hashAlgo, digest, []byte(sig))
		if err != nil {
			return phpv.ZInt(0).ZVal(), nil
		}
		return phpv.ZInt(1).ZVal(), nil
	case *ecdsa.PublicKey:
		ok := ecdsa.VerifyASN1(key, digest, []byte(sig))
		if !ok {
			return phpv.ZInt(0).ZVal(), nil
		}
		return phpv.ZInt(1).ZVal(), nil
	default:
		ctx.Warn("openssl_verify(): Unsupported key type for verification")
		return phpv.ZInt(-1).ZVal(), nil
	}
}
