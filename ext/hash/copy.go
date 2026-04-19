package hash

import (
	"bytes"
	"crypto/hmac"
	"errors"
	gohash "hash"
	"io"
	"os"
	"path/filepath"

	"github.com/KarpelesLab/anyhash"
	"github.com/KarpelesLab/goro/core"
	"github.com/KarpelesLab/goro/core/logopt"
	"github.com/KarpelesLab/goro/core/phpobj"
	"github.com/KarpelesLab/goro/core/phpv"
)

// hashContextData wraps a hash.Hash with metadata needed for copy/clone.
type hashContextData struct {
	gohash.Hash
	algo        phpv.ZString
	isHmac      bool
	hmacKey     []byte
	finalized   bool
	seed        uint32 // for seeded hashes like murmur3/xxhash
	seed64      uint64 // for 64-bit seeded hashes
	secret      []byte // for xxh3 secret
	writtenData []byte // buffered input for serialization
}

// Clone implements phpv.Cloneable to allow proper cloning of HashContext.
// The actual finalized check is done in the __clone method on HashContext class.
func (hcd *hashContextData) Clone() any {
	if hcd.finalized {
		// Return a finalized clone so __clone() can detect and throw the error
		return &hashContextData{finalized: true, algo: hcd.algo}
	}
	cloned, err := cloneHashContext(hcd)
	if err != nil {
		// Return self unchanged on error (best effort)
		return hcd
	}
	return cloned
}

// recreateFromWrittenData creates a new hashContextData by replaying written data.
// Used for serialization/unserialization where binary state is not available.
func recreateHashContext(algo phpv.ZString, isHmac bool, hmacKey []byte, seed uint32, seed64 uint64, secret []byte, writtenData []byte) (*hashContextData, error) {
	algoLower := algo.ToLower()

	var h gohash.Hash
	if isHmac {
		an, ok := algos[algoLower]
		if !ok {
			return nil, io.ErrUnexpectedEOF
		}
		h = hmac.New(an, hmacKey)
	} else if isSeededAlgo(algoLower) || isSeeded64Algo(algoLower) {
		var opts anyhash.Options
		if isSeeded64Algo(algoLower) {
			opts.Seed = seed64
			opts.Secret = secret
		} else {
			opts.Seed = uint64(seed)
		}
		var err error
		h, err = anyhash.New(anyhashName(algoLower), opts)
		if err != nil {
			return nil, io.ErrUnexpectedEOF
		}
	} else {
		an, ok := algos[algoLower]
		if !ok {
			return nil, io.ErrUnexpectedEOF
		}
		h = an()
	}

	if len(writtenData) > 0 {
		h.Write(writtenData)
	}

	wdCopy := make([]byte, len(writtenData))
	copy(wdCopy, writtenData)

	return &hashContextData{
		Hash:        h,
		algo:        algoLower,
		isHmac:      isHmac,
		hmacKey:     hmacKey,
		seed:        seed,
		seed64:      seed64,
		secret:      secret,
		writtenData: wdCopy,
	}, nil
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

// hashCloner is implemented by hash types that can clone themselves.
type hashCloner interface {
	CloneHash() gohash.Hash
}

func cloneHashContext(hcd *hashContextData) (*hashContextData, error) {
	wdCopy := make([]byte, len(hcd.writtenData))
	copy(wdCopy, hcd.writtenData)

	base := hashContextData{
		algo:        hcd.algo,
		isHmac:      hcd.isHmac,
		hmacKey:     hcd.hmacKey,
		seed:        hcd.seed,
		seed64:      hcd.seed64,
		secret:      hcd.secret,
		writtenData: wdCopy,
	}

	// Try anyhash Clone() first (handles most algorithms)
	if c, ok := hcd.Hash.(interface{ Clone() anyhash.Hash }); ok {
		base.Hash = c.Clone()
		return &base, nil
	}

	// Try CloneHash (murmur3/xxhash custom impls)
	if c, ok := hcd.Hash.(hashCloner); ok {
		base.Hash = c.CloneHash()
		return &base, nil
	}

	// Fallback: replay written data on a fresh hash
	fresh, err := recreateHashContext(hcd.algo, hcd.isHmac, hcd.hmacKey, hcd.seed, hcd.seed64, hcd.secret, hcd.writtenData)
	if err != nil {
		return nil, err
	}
	return fresh, nil
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
		if errors.Is(err, os.ErrNotExist) {
			return phpv.ZBool(false).ZVal(), ctx.Warn("hash_update_file(%s): Failed to open stream: No such file or directory", fname, logopt.NoFuncName(true))
		}
		return phpv.ZBool(false).ZVal(), ctx.Warn("hash_update_file(%s): Failed to open stream: %s", fname, err, logopt.NoFuncName(true))
	}
	defer f.Close()

	// Buffer written data for serialization support
	if hcd, ok := opaque.(*hashContextData); ok {
		var buf bytes.Buffer
		tr := io.TeeReader(f, &buf)
		if _, err := io.Copy(h, tr); err != nil {
			return phpv.ZBool(false).ZVal(), nil
		}
		hcd.writtenData = append(hcd.writtenData, buf.Bytes()...)
	} else {
		if _, err := io.Copy(h, f); err != nil {
			return phpv.ZBool(false).ZVal(), nil
		}
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
	// Buffer written data for serialization support
	if hcd, ok := opaque.(*hashContextData); ok {
		var buf bytes.Buffer
		tr := io.TeeReader(reader, &buf)
		if maxLen >= 0 {
			n, err = io.CopyN(h, tr, maxLen)
		} else {
			n, err = io.Copy(h, tr)
		}
		hcd.writtenData = append(hcd.writtenData, buf.Bytes()...)
	} else {
		if maxLen >= 0 {
			n, err = io.CopyN(h, reader, maxLen)
		} else {
			n, err = io.Copy(h, reader)
		}
	}
	if err != nil && err != io.EOF {
		return phpv.ZInt(n).ZVal(), nil
	}

	return phpv.ZInt(n).ZVal(), nil
}

// > func array hash_hmac_algos ( void )
func fncHashHmacAlgos(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	a := phpv.NewZArray()

	for _, n := range phpHashAlgoOrder {
		zn := phpv.ZString(n)
		if _, ok := algos[zn]; ok && !nonCryptoAlgos[zn] {
			a.OffsetSet(ctx, nil, zn.ZVal())
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
