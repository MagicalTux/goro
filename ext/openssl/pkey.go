package openssl

import (
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"math/big"

	"github.com/MagicalTux/goro/core"
	"github.com/MagicalTux/goro/core/phpobj"
	"github.com/MagicalTux/goro/core/phpv"
)

// OpenSSLAsymmetricKey is the PHP 8 class for openssl key resources.
var OpenSSLAsymmetricKey = &phpobj.ZClass{
	Name: "OpenSSLAsymmetricKey",
}

// pkeyData wraps the Go private key inside the opaque object.
type pkeyData struct {
	privKey crypto.PrivateKey
	pubKey  crypto.PublicKey
	keyType int // OPENSSL_KEYTYPE_RSA, OPENSSL_KEYTYPE_EC, etc.
}

// getPkeyData extracts the pkeyData from an OpenSSLAsymmetricKey object.
func getPkeyData(obj *phpobj.ZObject) *pkeyData {
	opaque := obj.GetOpaque(OpenSSLAsymmetricKey)
	if opaque == nil {
		return nil
	}
	return opaque.(*pkeyData)
}

// > func OpenSSLAsymmetricKey openssl_pkey_new ([ array $options ] )
func fncOpensslPkeyNew(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var options **phpv.ZArray

	core.Expand(ctx, args, &options)

	keyType := OPENSSL_KEYTYPE_RSA
	bits := 2048
	curve := "prime256v1"

	if options != nil && *options != nil {
		opts := *options
		// Check "private_key_type"
		if v, err := opts.OffsetGet(ctx, phpv.ZString("private_key_type").ZVal()); err == nil && v != nil && v.GetType() != phpv.ZtNull {
			kt, err := v.As(ctx, phpv.ZtInt)
			if err == nil {
				keyType = int(kt.Value().(phpv.ZInt))
			}
		}
		// Check "private_key_bits"
		if v, err := opts.OffsetGet(ctx, phpv.ZString("private_key_bits").ZVal()); err == nil && v != nil && v.GetType() != phpv.ZtNull {
			b, err := v.As(ctx, phpv.ZtInt)
			if err == nil {
				bits = int(b.Value().(phpv.ZInt))
			}
		}
		// Check "curve_name"
		if v, err := opts.OffsetGet(ctx, phpv.ZString("curve_name").ZVal()); err == nil && v != nil && v.GetType() != phpv.ZtNull {
			c, err := v.As(ctx, phpv.ZtString)
			if err == nil {
				curve = string(c.Value().(phpv.ZString))
			}
		}
	}

	var privKey crypto.PrivateKey
	var pubKey crypto.PublicKey

	switch keyType {
	case OPENSSL_KEYTYPE_RSA:
		if bits < 512 {
			bits = 512
		}
		key, err := rsa.GenerateKey(rand.Reader, bits)
		if err != nil {
			ctx.Warn("openssl_pkey_new(): Failed to generate RSA key: %s", err.Error())
			return phpv.ZBool(false).ZVal(), nil
		}
		privKey = key
		pubKey = &key.PublicKey

	case OPENSSL_KEYTYPE_EC:
		var c elliptic.Curve
		switch curve {
		case "prime256v1", "P-256":
			c = elliptic.P256()
		case "secp384r1", "P-384":
			c = elliptic.P384()
		case "secp521r1", "P-521":
			c = elliptic.P521()
		case "secp256k1":
			// Go's stdlib doesn't support secp256k1 natively
			ctx.Warn("openssl_pkey_new(): Unsupported curve: %s", curve)
			return phpv.ZBool(false).ZVal(), nil
		default:
			ctx.Warn("openssl_pkey_new(): Unknown curve name: %s", curve)
			return phpv.ZBool(false).ZVal(), nil
		}
		key, err := ecdsa.GenerateKey(c, rand.Reader)
		if err != nil {
			ctx.Warn("openssl_pkey_new(): Failed to generate EC key: %s", err.Error())
			return phpv.ZBool(false).ZVal(), nil
		}
		privKey = key
		pubKey = &key.PublicKey

	default:
		ctx.Warn("openssl_pkey_new(): Unsupported key type")
		return phpv.ZBool(false).ZVal(), nil
	}

	pd := &pkeyData{
		privKey: privKey,
		pubKey:  pubKey,
		keyType: keyType,
	}

	obj, err := phpobj.NewZObjectOpaque(ctx, OpenSSLAsymmetricKey, pd)
	if err != nil {
		return nil, err
	}

	return obj.ZVal(), nil
}

// > func bool openssl_pkey_export ( OpenSSLAsymmetricKey $key , string &$output [, string $passphrase [, array $options ]] )
func fncOpensslPkeyExport(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	if len(args) < 2 {
		return nil, fmt.Errorf("openssl_pkey_export() expects at least 2 arguments")
	}

	keyVal := args[0]
	if keyVal == nil || keyVal.GetType() == phpv.ZtNull {
		ctx.Warn("openssl_pkey_export(): Argument #1 ($key) must be of type OpenSSLAsymmetricKey|string")
		return phpv.ZBool(false).ZVal(), nil
	}

	pd, err := resolvePkey(ctx, keyVal)
	if err != nil || pd == nil {
		ctx.Warn("openssl_pkey_export(): Cannot get key from the given parameter")
		return phpv.ZBool(false).ZVal(), nil
	}

	if pd.privKey == nil {
		ctx.Warn("openssl_pkey_export(): Cannot export a public key without the private key")
		return phpv.ZBool(false).ZVal(), nil
	}

	var derBytes []byte
	var pemType string

	switch k := pd.privKey.(type) {
	case *rsa.PrivateKey:
		derBytes = x509.MarshalPKCS1PrivateKey(k)
		pemType = "RSA PRIVATE KEY"
	case *ecdsa.PrivateKey:
		derBytes, err = x509.MarshalECPrivateKey(k)
		if err != nil {
			ctx.Warn("openssl_pkey_export(): Failed to marshal EC key: %s", err.Error())
			return phpv.ZBool(false).ZVal(), nil
		}
		pemType = "EC PRIVATE KEY"
	default:
		ctx.Warn("openssl_pkey_export(): Unsupported key type")
		return phpv.ZBool(false).ZVal(), nil
	}

	pemBlock := &pem.Block{
		Type:  pemType,
		Bytes: derBytes,
	}
	pemBytes := pem.EncodeToMemory(pemBlock)

	// Set &$output
	if len(args) > 1 && args[1] != nil {
		name := args[1].GetName()
		outVal := phpv.ZString(pemBytes).ZVal()
		outVal.Name = &name
		ctx.Parent(1).OffsetSet(ctx, name, outVal)
	}

	return phpv.ZBool(true).ZVal(), nil
}

// > func array openssl_pkey_get_details ( OpenSSLAsymmetricKey $key )
func fncOpensslPkeyGetDetails(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	if len(args) < 1 {
		return nil, fmt.Errorf("openssl_pkey_get_details() expects exactly 1 argument")
	}

	pd, err := resolvePkey(ctx, args[0])
	if err != nil || pd == nil {
		ctx.Warn("openssl_pkey_get_details(): Cannot get key from the given parameter")
		return phpv.ZBool(false).ZVal(), nil
	}

	result := phpv.NewZArray()
	result.OffsetSet(ctx, phpv.ZString("type").ZVal(), phpv.ZInt(pd.keyType).ZVal())

	// Get the public key in PEM format
	pubPem, err := marshalPublicKeyPEM(pd.pubKey)
	if err == nil {
		result.OffsetSet(ctx, phpv.ZString("key").ZVal(), phpv.ZString(pubPem).ZVal())
	}

	switch pub := pd.pubKey.(type) {
	case *rsa.PublicKey:
		result.OffsetSet(ctx, phpv.ZString("bits").ZVal(), phpv.ZInt(pub.N.BitLen()).ZVal())
		rsaArr := phpv.NewZArray()
		rsaArr.OffsetSet(ctx, phpv.ZString("n").ZVal(), phpv.ZString(pub.N.Bytes()).ZVal())
		rsaArr.OffsetSet(ctx, phpv.ZString("e").ZVal(), phpv.ZString(big.NewInt(int64(pub.E)).Bytes()).ZVal())
		if priv, ok := pd.privKey.(*rsa.PrivateKey); ok {
			rsaArr.OffsetSet(ctx, phpv.ZString("d").ZVal(), phpv.ZString(priv.D.Bytes()).ZVal())
			if len(priv.Primes) >= 2 {
				rsaArr.OffsetSet(ctx, phpv.ZString("p").ZVal(), phpv.ZString(priv.Primes[0].Bytes()).ZVal())
				rsaArr.OffsetSet(ctx, phpv.ZString("q").ZVal(), phpv.ZString(priv.Primes[1].Bytes()).ZVal())
			}
		}
		result.OffsetSet(ctx, phpv.ZString("rsa").ZVal(), rsaArr.ZVal())

	case *ecdsa.PublicKey:
		result.OffsetSet(ctx, phpv.ZString("bits").ZVal(), phpv.ZInt(pub.Curve.Params().BitSize).ZVal())
		ecArr := phpv.NewZArray()
		ecArr.OffsetSet(ctx, phpv.ZString("curve_name").ZVal(), phpv.ZString(curveName(pub.Curve)).ZVal())
		ecArr.OffsetSet(ctx, phpv.ZString("curve_oid").ZVal(), phpv.ZString(curveOID(pub.Curve)).ZVal())
		ecArr.OffsetSet(ctx, phpv.ZString("x").ZVal(), phpv.ZString(pub.X.Bytes()).ZVal())
		ecArr.OffsetSet(ctx, phpv.ZString("y").ZVal(), phpv.ZString(pub.Y.Bytes()).ZVal())
		if priv, ok := pd.privKey.(*ecdsa.PrivateKey); ok {
			ecArr.OffsetSet(ctx, phpv.ZString("d").ZVal(), phpv.ZString(priv.D.Bytes()).ZVal())
		}
		result.OffsetSet(ctx, phpv.ZString("ec").ZVal(), ecArr.ZVal())
	}

	return result.ZVal(), nil
}

// > func OpenSSLAsymmetricKey openssl_pkey_get_public ( string $public_key )
func fncOpensslPkeyGetPublic(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	if len(args) < 1 {
		return nil, fmt.Errorf("openssl_pkey_get_public() expects exactly 1 argument")
	}

	val := args[0]
	if val == nil {
		ctx.Warn("openssl_pkey_get_public(): Argument #1 cannot be null")
		return phpv.ZBool(false).ZVal(), nil
	}

	// If it's already an OpenSSLAsymmetricKey object, return it
	if val.GetType() == phpv.ZtObject {
		obj := val.Value().(*phpobj.ZObject)
		if obj.Class == OpenSSLAsymmetricKey {
			return val, nil
		}
	}

	// Parse PEM string
	pemStr, err := val.As(ctx, phpv.ZtString)
	if err != nil {
		ctx.Warn("openssl_pkey_get_public(): Cannot get key from the given parameter")
		return phpv.ZBool(false).ZVal(), nil
	}

	pubKey, err := parsePublicKeyPEM([]byte(pemStr.Value().(phpv.ZString)))
	if err != nil {
		ctx.Warn("openssl_pkey_get_public(): Cannot get key from the given parameter")
		return phpv.ZBool(false).ZVal(), nil
	}

	kt := OPENSSL_KEYTYPE_RSA
	if _, ok := pubKey.(*ecdsa.PublicKey); ok {
		kt = OPENSSL_KEYTYPE_EC
	}

	pd := &pkeyData{
		pubKey:  pubKey,
		keyType: kt,
	}

	obj, err := phpobj.NewZObjectOpaque(ctx, OpenSSLAsymmetricKey, pd)
	if err != nil {
		return nil, err
	}

	return obj.ZVal(), nil
}

// > func OpenSSLAsymmetricKey openssl_pkey_get_private ( string $private_key [, string $passphrase = "" ] )
func fncOpensslPkeyGetPrivate(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	if len(args) < 1 {
		return nil, fmt.Errorf("openssl_pkey_get_private() expects at least 1 argument")
	}

	val := args[0]
	if val == nil {
		ctx.Warn("openssl_pkey_get_private(): Argument #1 cannot be null")
		return phpv.ZBool(false).ZVal(), nil
	}

	// If it's already an OpenSSLAsymmetricKey object, return it
	if val.GetType() == phpv.ZtObject {
		obj := val.Value().(*phpobj.ZObject)
		if obj.Class == OpenSSLAsymmetricKey {
			return val, nil
		}
	}

	pemStr, err := val.As(ctx, phpv.ZtString)
	if err != nil {
		ctx.Warn("openssl_pkey_get_private(): Cannot get key from the given parameter")
		return phpv.ZBool(false).ZVal(), nil
	}

	privKey, pubKey, kt, err := parsePrivateKeyPEM([]byte(pemStr.Value().(phpv.ZString)))
	if err != nil {
		ctx.Warn("openssl_pkey_get_private(): Cannot get key from the given parameter")
		return phpv.ZBool(false).ZVal(), nil
	}

	pd := &pkeyData{
		privKey: privKey,
		pubKey:  pubKey,
		keyType: kt,
	}

	obj, err := phpobj.NewZObjectOpaque(ctx, OpenSSLAsymmetricKey, pd)
	if err != nil {
		return nil, err
	}

	return obj.ZVal(), nil
}

// resolvePkey resolves a key argument to a pkeyData, handling both
// OpenSSLAsymmetricKey objects and PEM strings.
func resolvePkey(ctx phpv.Context, val *phpv.ZVal) (*pkeyData, error) {
	if val == nil || val.GetType() == phpv.ZtNull {
		return nil, fmt.Errorf("null key")
	}

	// OpenSSLAsymmetricKey object
	if val.GetType() == phpv.ZtObject {
		obj := val.Value().(*phpobj.ZObject)
		pd := getPkeyData(obj)
		if pd != nil {
			return pd, nil
		}
	}

	// Try as PEM string
	pemStr, err := val.As(ctx, phpv.ZtString)
	if err != nil {
		return nil, err
	}
	pemBytes := []byte(pemStr.Value().(phpv.ZString))

	// Try private key first
	privKey, pubKey, kt, err := parsePrivateKeyPEM(pemBytes)
	if err == nil {
		return &pkeyData{privKey: privKey, pubKey: pubKey, keyType: kt}, nil
	}

	// Try public key
	pub, err := parsePublicKeyPEM(pemBytes)
	if err == nil {
		kt := OPENSSL_KEYTYPE_RSA
		if _, ok := pub.(*ecdsa.PublicKey); ok {
			kt = OPENSSL_KEYTYPE_EC
		}
		return &pkeyData{pubKey: pub, keyType: kt}, nil
	}

	return nil, fmt.Errorf("cannot parse key")
}

// parsePublicKeyPEM parses a PEM-encoded public key.
func parsePublicKeyPEM(pemBytes []byte) (crypto.PublicKey, error) {
	block, _ := pem.Decode(pemBytes)
	if block == nil {
		return nil, fmt.Errorf("failed to decode PEM")
	}

	switch block.Type {
	case "PUBLIC KEY":
		pub, err := x509.ParsePKIXPublicKey(block.Bytes)
		if err != nil {
			return nil, err
		}
		return pub, nil
	case "RSA PUBLIC KEY":
		pub, err := x509.ParsePKCS1PublicKey(block.Bytes)
		if err != nil {
			return nil, err
		}
		return pub, nil
	case "CERTIFICATE":
		cert, err := x509.ParseCertificate(block.Bytes)
		if err != nil {
			return nil, err
		}
		return cert.PublicKey, nil
	default:
		return nil, fmt.Errorf("unsupported PEM type: %s", block.Type)
	}
}

// parsePrivateKeyPEM parses a PEM-encoded private key.
func parsePrivateKeyPEM(pemBytes []byte) (crypto.PrivateKey, crypto.PublicKey, int, error) {
	block, _ := pem.Decode(pemBytes)
	if block == nil {
		return nil, nil, 0, fmt.Errorf("failed to decode PEM")
	}

	switch block.Type {
	case "RSA PRIVATE KEY":
		key, err := x509.ParsePKCS1PrivateKey(block.Bytes)
		if err != nil {
			return nil, nil, 0, err
		}
		return key, &key.PublicKey, OPENSSL_KEYTYPE_RSA, nil
	case "EC PRIVATE KEY":
		key, err := x509.ParseECPrivateKey(block.Bytes)
		if err != nil {
			return nil, nil, 0, err
		}
		return key, &key.PublicKey, OPENSSL_KEYTYPE_EC, nil
	case "PRIVATE KEY":
		key, err := x509.ParsePKCS8PrivateKey(block.Bytes)
		if err != nil {
			return nil, nil, 0, err
		}
		switch k := key.(type) {
		case *rsa.PrivateKey:
			return k, &k.PublicKey, OPENSSL_KEYTYPE_RSA, nil
		case *ecdsa.PrivateKey:
			return k, &k.PublicKey, OPENSSL_KEYTYPE_EC, nil
		default:
			return nil, nil, 0, fmt.Errorf("unsupported private key type in PKCS#8")
		}
	default:
		return nil, nil, 0, fmt.Errorf("unsupported PEM type: %s", block.Type)
	}
}

// marshalPublicKeyPEM marshals a public key to PEM format.
func marshalPublicKeyPEM(pub crypto.PublicKey) ([]byte, error) {
	derBytes, err := x509.MarshalPKIXPublicKey(pub)
	if err != nil {
		return nil, err
	}
	block := &pem.Block{
		Type:  "PUBLIC KEY",
		Bytes: derBytes,
	}
	return pem.EncodeToMemory(block), nil
}

// curveName returns the OpenSSL name for a Go elliptic curve.
func curveName(c elliptic.Curve) string {
	switch c {
	case elliptic.P224():
		return "secp224r1"
	case elliptic.P256():
		return "prime256v1"
	case elliptic.P384():
		return "secp384r1"
	case elliptic.P521():
		return "secp521r1"
	default:
		return "unknown"
	}
}

// curveOID returns the OID string for a Go elliptic curve.
func curveOID(c elliptic.Curve) string {
	switch c {
	case elliptic.P224():
		return "1.3.132.0.33"
	case elliptic.P256():
		return "1.2.840.10045.3.1.7"
	case elliptic.P384():
		return "1.3.132.0.34"
	case elliptic.P521():
		return "1.3.132.0.35"
	default:
		return ""
	}
}
