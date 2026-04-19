package hash

import (
	"fmt"
	gohash "hash"
	"strings"

	"github.com/KarpelesLab/anyhash"
	"github.com/KarpelesLab/goro/core/phpv"
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

// serializeHashState extracts the hash state into a PHP array using anyhash's PHPMarshaler.
// The returned array contains int32 and/or string values in PHP's expected order.
func serializeHashState(h gohash.Hash) (*phpv.ZArray, error) {
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
func unserializeFromPHPState(ctx phpv.Context, algo phpv.ZString, stateArr *phpv.ZArray) (*hashContextData, error) {
	algoLower := algo.ToLower()
	count := stateArr.Count(ctx)

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

	// Try anyhash.UnmarshalPHP first
	h, err := anyhash.UnmarshalPHP(anyhashName(algoLower), state)
	if err == nil {
		return &hashContextData{Hash: h, algo: algoLower}, nil
	}
	if strings.Contains(err.Error(), "invalid buffer length") {
		return nil, errInvalidSize
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
