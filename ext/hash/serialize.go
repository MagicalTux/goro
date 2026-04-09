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
const phpHashUnserializeSpecMismatch = -1024

var errSpecMismatch = fmt.Errorf("spec mismatch")

// serializeHashState extracts the hash state into a PHP array using anyhash's PHPMarshaler.
func serializeHashState(h gohash.Hash) (*phpv.ZArray, error) {
	pm, ok := h.(anyhash.PHPMarshaler)
	if !ok {
		return nil, nil
	}
	ints, buf := pm.MarshalPHP()

	arr := phpv.NewZArray()
	idx := 0
	for _, v := range ints {
		arr.OffsetSet(nil, phpv.ZInt(idx).ZVal(), phpv.ZInt(int64(v)).ZVal())
		idx++
	}
	if len(buf) > 0 {
		arr.OffsetSet(nil, phpv.ZInt(idx).ZVal(), phpv.ZString(buf).ZVal())
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
func unserializeFromPHPState(ctx phpv.Context, algo phpv.ZString, stateArr *phpv.ZArray) (*hashContextData, error) {
	algoLower := algo.ToLower()
	count := stateArr.Count(ctx)

	// Collect int32 values and buffer from the PHP array
	var ints []int32
	var buf []byte
	for i := 0; i < int(count); i++ {
		v, _ := stateArr.OffsetGet(ctx, phpv.ZInt(i))
		if v == nil {
			break
		}
		if v.GetType() == phpv.ZtString {
			buf = []byte(v.Value().(phpv.ZString))
		} else {
			ints = append(ints, int32(v.AsInt(ctx)))
		}
	}

	// Try anyhash.UnmarshalPHP first
	h, err := anyhash.UnmarshalPHP(string(algoLower), ints, buf)
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
