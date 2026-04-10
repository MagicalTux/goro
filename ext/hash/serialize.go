package hash

import (
	"fmt"
	gohash "hash"

	"github.com/KarpelesLab/anyhash"
	"github.com/MagicalTux/goro/core/phpv"
)

// phpSerializeMagic is the magic number PHP uses to validate serialized hash data.
const phpSerializeMagic = 2

// Error codes matching PHP's hash serialization error codes.
const (
	phpHashUnserializeSpecMismatch = -1024
	phpHashUnserializeInvalidSize  = -2000
)

var (
	errSpecMismatch = fmt.Errorf("spec mismatch")
	errInvalidSize  = fmt.Errorf("invalid size")
)

// getStateInt returns the int32 value at index i, or 0 if not an int.
func getStateInt(state []any, i int) int32 {
	if i >= len(state) {
		return 0
	}
	if v, ok := state[i].(int32); ok {
		return v
	}
	return 0
}

// validateXXHashState checks that buffer size fields are within valid ranges
// to prevent anyhash panics on invalid serialized state.
func validateXXHashState(algo phpv.ZString, state []any) error {
	switch algo {
	case "xxh32":
		// state[10] = n (bytes in 16-byte buffer)
		if len(state) < 12 {
			return errSpecMismatch
		}
		if n := getStateInt(state, 10); n < 0 || n >= 16 {
			return errInvalidSize
		}
	case "xxh64":
		// state[18] = n (bytes in 32-byte buffer)
		if len(state) < 22 {
			return errSpecMismatch
		}
		if n := getStateInt(state, 18); n < 0 || n >= 32 {
			return errInvalidSize
		}
	}
	return nil
}

// brokenPHPMarshalerAlgos lists algorithms where anyhash's PHPMarshaler has
// known bugs (e.g., carry packing truncates buffers). For these, fall back
// to the writtenData replay format.
var brokenPHPMarshalerAlgos = map[phpv.ZString]bool{
	"murmur3c": true, // carry field can't hold >4 bytes of buffer state
	"murmur3f": true, // same issue
}

// serializeHashState extracts the hash state into a PHP array using anyhash's PHPMarshaler.
// The returned array contains int32 and/or string values in PHP's expected order.
// Returns nil if the algorithm is known to have bugs — caller should use replay fallback.
func serializeHashState(h gohash.Hash, algo phpv.ZString) (*phpv.ZArray, error) {
	if brokenPHPMarshalerAlgos[algo] {
		return nil, nil
	}
	pm, ok := h.(anyhash.PHPMarshaler)
	if !ok {
		return nil, nil
	}
	state := pm.MarshalPHP()

	arr := phpv.NewZArray()
	for i, v := range state {
		switch val := v.(type) {
		case int32:
			arr.OffsetSet(nil, phpv.ZInt(i).ZVal(), phpv.ZInt(int64(val)).ZVal())
		case []byte:
			arr.OffsetSet(nil, phpv.ZInt(i).ZVal(), phpv.ZString(val).ZVal())
		default:
			return nil, fmt.Errorf("unsupported PHP state element type %T at index %d", v, i)
		}
	}
	return arr, nil
}

func getArrayInt(ctx phpv.Context, arr *phpv.ZArray, idx int) (int64, bool) {
	v, _ := arr.OffsetGet(ctx, phpv.ZInt(idx))
	if v == nil {
		return 0, false
	}
	return int64(v.AsInt(ctx)), true
}

func getArrayIntStrict(ctx phpv.Context, arr *phpv.ZArray, idx int) (int64, bool) {
	v, _ := arr.OffsetGet(ctx, phpv.ZInt(idx))
	if v == nil {
		return 0, false
	}
	if v.GetType() != phpv.ZtInt {
		return 0, false
	}
	return int64(v.Value().(phpv.ZInt)), true
}

func getArrayString(ctx phpv.Context, arr *phpv.ZArray, idx int) (string, bool) {
	v, _ := arr.OffsetGet(ctx, phpv.ZInt(idx))
	if v == nil {
		return "", false
	}
	if v.GetType() != phpv.ZtString {
		return "", false
	}
	return string(v.Value().(phpv.ZString)), true
}

// unserializeFromPHPState reconstructs a hash from PHP serialized state.
func unserializeFromPHPState(ctx phpv.Context, algo phpv.ZString, stateArr *phpv.ZArray) (hcd *hashContextData, err error) {
	// Recover from panics in anyhash UnmarshalPHP (some algorithms panic
	// on invalid state rather than returning an error).
	defer func() {
		if r := recover(); r != nil {
			hcd = nil
			err = errInvalidSize
		}
	}()
	return doUnserializeFromPHPState(ctx, algo, stateArr)
}

func doUnserializeFromPHPState(ctx phpv.Context, algo phpv.ZString, stateArr *phpv.ZArray) (*hashContextData, error) {
	algoLower := algo.ToLower()
	count := stateArr.Count(ctx)

	// For algorithms with known PHPMarshaler bugs, use replay format
	if brokenPHPMarshalerAlgos[algoLower] && count == 2 {
		dataVal, _ := stateArr.OffsetGet(ctx, phpv.ZInt(0))
		if dataVal != nil && dataVal.GetType() == phpv.ZtString {
			data := []byte(dataVal.Value().(phpv.ZString))
			return recreateHashContext(algoLower, false, nil, 0, 0, nil, data)
		}
	}

	// Collect values from the PHP array as []any (int32 or []byte)
	state := make([]any, 0, count)
	for i := 0; i < int(count); i++ {
		v, _ := stateArr.OffsetGet(ctx, phpv.ZInt(i))
		if v == nil {
			break
		}
		switch v.GetType() {
		case phpv.ZtString:
			state = append(state, []byte(v.Value().(phpv.ZString)))
		default:
			state = append(state, int32(v.AsInt(ctx)))
		}
	}

	// Validate buffer size bounds for xxhash algorithms (anyhash doesn't
	// validate and can panic on out-of-range values).
	if err := validateXXHashState(algoLower, state); err != nil {
		return nil, err
	}

	// Try anyhash.UnmarshalPHP first
	h, err := anyhash.UnmarshalPHP(anyhashName(algoLower), state)
	if err == nil {
		return &hashContextData{Hash: h, algo: algoLower}, nil
	}

	// Fallback: check for replay format (string data + length)
	if count == 2 {
		dataVal, _ := stateArr.OffsetGet(ctx, phpv.ZInt(0))
		if dataVal != nil && dataVal.GetType() == phpv.ZtString {
			data := []byte(dataVal.Value().(phpv.ZString))
			return recreateHashContext(algoLower, false, nil, 0, 0, nil, data)
		}
	}

	return nil, fmt.Errorf("unsupported state format for %s", algo)
}
