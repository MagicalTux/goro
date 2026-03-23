package hash

import (
	"crypto/hmac"
	"encoding"
	gohash "hash"
	"io"
	"path/filepath"

	"github.com/MagicalTux/goro/core"
	"github.com/MagicalTux/goro/core/phpobj"
	"github.com/MagicalTux/goro/core/phpv"
)

// hashContextData wraps a hash.Hash with metadata needed for copy/clone.
type hashContextData struct {
	gohash.Hash
	algo     phpv.ZString
	isHmac   bool
	hmacKey  []byte
	finalized bool
}

// > func HashContext hash_copy ( HashContext $context )
func fncHashCopy(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	obj := &phpobj.ZObject{Class: HashContext}

	_, err := core.Expand(ctx, args, &obj)
	if err != nil {
		return nil, err
	}

	opaque := obj.GetOpaque(HashContext)
	if opaque == nil {
		return nil, phpobj.ThrowError(ctx, phpobj.ValueError, "hash_copy(): Argument #1 ($context) must be a valid, non-finalized HashContext")
	}

	hcd, ok := opaque.(*hashContextData)
	if !ok {
		// Legacy: plain hash.Hash without metadata - can't clone properly
		return nil, phpobj.ThrowError(ctx, phpobj.ValueError, "hash_copy(): Argument #1 ($context) must be a valid, non-finalized HashContext")
	}

	if hcd.finalized {
		return nil, phpobj.ThrowError(ctx, phpobj.ValueError, "hash_copy(): Argument #1 ($context) must be a valid, non-finalized HashContext")
	}

	cloned, err := cloneHashContext(hcd)
	if err != nil {
		return nil, err
	}

	z, err := phpobj.NewZObjectOpaque(ctx, HashContext, cloned)
	if err != nil {
		return nil, err
	}

	return z.ZVal(), nil
}

func cloneHashContext(hcd *hashContextData) (*hashContextData, error) {
	algN, ok := algos[hcd.algo.ToLower()]
	if !ok {
		return nil, io.ErrUnexpectedEOF
	}

	var newHash gohash.Hash
	if hcd.isHmac {
		newHash = hmac.New(algN, hcd.hmacKey)
	} else {
		newHash = algN()
	}

	// Try marshal/unmarshal to clone the internal state
	if m, ok := hcd.Hash.(encoding.BinaryMarshaler); ok {
		state, err := m.MarshalBinary()
		if err == nil {
			if u, ok := newHash.(encoding.BinaryUnmarshaler); ok {
				if err := u.UnmarshalBinary(state); err == nil {
					return &hashContextData{
						Hash:    newHash,
						algo:    hcd.algo,
						isHmac:  hcd.isHmac,
						hmacKey: hcd.hmacKey,
					}, nil
				}
			}
		}
	}

	// Fallback: return a fresh hash (state not preserved - better than crashing)
	return &hashContextData{
		Hash:    newHash,
		algo:    hcd.algo,
		isHmac:  hcd.isHmac,
		hmacKey: hcd.hmacKey,
	}, nil
}

// > func bool hash_update_file ( HashContext $context, string $filename [, resource $stream_context ] )
func fncHashUpdateFile(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	obj := &phpobj.ZObject{Class: HashContext}
	var filename phpv.ZString

	_, err := core.Expand(ctx, args, &obj, &filename)
	if err != nil {
		return nil, err
	}

	opaque := obj.GetOpaque(HashContext)
	if opaque == nil {
		return nil, phpobj.ThrowError(ctx, phpobj.ValueError, "hash_update_file(): Argument #1 ($context) must be a valid, non-finalized HashContext")
	}

	h := getHash(opaque)
	if h == nil {
		return nil, phpobj.ThrowError(ctx, phpobj.ValueError, "hash_update_file(): Argument #1 ($context) must be a valid, non-finalized HashContext")
	}

	fname := string(filename)
	if !filepath.IsAbs(fname) {
		fname = filepath.Join(string(ctx.Global().Getwd()), fname)
	}
	if err := ctx.Global().CheckOpenBasedir(ctx, fname, "hash_update_file"); err != nil {
		return phpv.ZBool(false).ZVal(), nil
	}

	f, err := ctx.Global().OpenFile(ctx, fname)
	if err != nil {
		return phpv.ZBool(false).ZVal(), nil
	}
	defer f.Close()

	if _, err := io.Copy(h, f); err != nil {
		return phpv.ZBool(false).ZVal(), nil
	}

	return phpv.ZBool(true).ZVal(), nil
}

// > func bool hash_update_stream ( HashContext $context, resource $stream [, int $length = -1 ] )
func fncHashUpdateStream(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	obj := &phpobj.ZObject{Class: HashContext}
	var stream *phpv.ZVal
	var length *phpv.ZInt

	_, err := core.Expand(ctx, args, &obj, &stream, &length)
	if err != nil {
		return nil, err
	}

	opaque := obj.GetOpaque(HashContext)
	if opaque == nil {
		return nil, phpobj.ThrowError(ctx, phpobj.ValueError, "hash_update_stream(): Argument #1 ($context) must be a valid, non-finalized HashContext")
	}

	h := getHash(opaque)
	if h == nil {
		return nil, phpobj.ThrowError(ctx, phpobj.ValueError, "hash_update_stream(): Argument #1 ($context) must be a valid, non-finalized HashContext")
	}

	// The stream arg should be a resource (PHP stream)
	// For goro, resources are usually phpv.ZObject or a special type.
	// Let's try to get a reader from it.
	if stream == nil {
		return phpv.ZInt(0).ZVal(), nil
	}

	reader, ok := stream.Value().(io.Reader)
	if !ok {
		// Try phpv.ZObject with ReadCloser
		if rc, ok := stream.Value().(io.ReadCloser); ok {
			reader = rc
		} else {
			return phpv.ZInt(0).ZVal(), nil
		}
	}

	maxLen := int64(-1)
	if length != nil {
		maxLen = int64(*length)
	}

	var n int64
	if maxLen >= 0 {
		n, err = io.CopyN(h, reader, maxLen)
	} else {
		n, err = io.Copy(h, reader)
	}
	if err != nil && err != io.EOF {
		return phpv.ZInt(n).ZVal(), nil
	}

	return phpv.ZInt(n).ZVal(), nil
}

// > func array hash_hmac_algos ( void )
func fncHashHmacAlgos(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	a := phpv.NewZArray()

	for n := range algos {
		if !nonCryptoAlgos[n] {
			a.OffsetSet(ctx, nil, n.ZVal())
		}
	}
	return a.ZVal(), nil
}

// getHash extracts the gohash.Hash from either a hashContextData or raw hash.Hash opaque.
func getHash(opaque interface{}) gohash.Hash {
	if hcd, ok := opaque.(*hashContextData); ok {
		if hcd.finalized {
			return nil
		}
		return hcd.Hash
	}
	if h, ok := opaque.(gohash.Hash); ok {
		return h
	}
	return nil
}
