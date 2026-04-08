package hash

import (
	"encoding"
	"encoding/binary"
	"fmt"
	gohash "hash"

	"github.com/MagicalTux/goro/core/phpv"
)

// phpHashSerializer converts between Go hash internal state and PHP's
// serialization format (array of int/string values).
type phpHashSerializer interface {
	SerializeState(h gohash.Hash) (*phpv.ZArray, error)
}

// phpSerializeMagic is the magic number PHP uses to validate serialized hash data.
const phpSerializeMagic = 2

// Error codes matching PHP's hash serialization error codes.
const phpHashUnserializeSpecMismatch = -1024

// errSpecMismatch is returned when the internal state format is wrong.
var errSpecMismatch = fmt.Errorf("spec mismatch")

// getAlgoSerializer returns the PHP-compatible serializer for the given algorithm.
func getAlgoSerializer(algo phpv.ZString) phpHashSerializer {
	switch algo {
	case "md5":
		return &md5Serializer{}
	case "sha1":
		return &sha1Serializer{}
	case "sha224", "sha256":
		return &sha256Serializer{}
	case "sha384", "sha512", "sha512/224", "sha512/256":
		return &sha512Serializer{}
	case "adler32", "fnv132", "fnv1a32":
		return &binaryMarshalUint32Serializer{}
	case "crc32":
		return &crc32CustomSerializer{}
	case "crc32b", "crc32c":
		return &binaryMarshalCRC32Serializer{}
	case "fnv164", "fnv1a64":
		return &binaryMarshalUint64Serializer{}
	case "joaat":
		return &joaatSerializer{}
	default:
		return &replaySerializer{}
	}
}

func marshalHash(h gohash.Hash) ([]byte, error) {
	if m, ok := h.(encoding.BinaryMarshaler); ok {
		return m.MarshalBinary()
	}
	if hr, ok := h.(*hashReplayable); ok {
		if m, ok := hr.Hash.(encoding.BinaryMarshaler); ok {
			return m.MarshalBinary()
		}
	}
	return nil, nil
}

func getArrayInt(ctx phpv.Context, arr *phpv.ZArray, idx int) (int64, bool) {
	v, _ := arr.OffsetGet(ctx, phpv.ZInt(idx))
	if v == nil {
		return 0, false
	}
	return int64(v.AsInt(ctx)), true
}

// getArrayIntStrict returns the int value at idx, but only if the type is actually ZtInt.
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

// --- MD5 Serializer ---
type md5Serializer struct{}

func (s *md5Serializer) SerializeState(h gohash.Hash) (*phpv.ZArray, error) {
	data, err := marshalHash(h)
	if err != nil || data == nil || len(data) < 92 {
		return nil, err
	}
	arr := phpv.NewZArray()
	count := binary.BigEndian.Uint64(data[84:92])
	arr.OffsetSet(nil, phpv.ZInt(0).ZVal(), phpv.ZInt(int64(int32(uint32(count)))).ZVal())
	arr.OffsetSet(nil, phpv.ZInt(1).ZVal(), phpv.ZInt(int64(int32(uint32(count>>32)))).ZVal())
	for i := 0; i < 4; i++ {
		v := binary.BigEndian.Uint32(data[4+i*4 : 8+i*4])
		arr.OffsetSet(nil, phpv.ZInt(i+2).ZVal(), phpv.ZInt(int64(int32(v))).ZVal())
	}
	arr.OffsetSet(nil, phpv.ZInt(6).ZVal(), phpv.ZString(data[20:84]).ZVal())
	for i := 0; i < 16; i++ {
		arr.OffsetSet(nil, phpv.ZInt(7+i).ZVal(), phpv.ZInt(0).ZVal())
	}
	return arr, nil
}

// --- SHA1 Serializer ---
type sha1Serializer struct{}

func (s *sha1Serializer) SerializeState(h gohash.Hash) (*phpv.ZArray, error) {
	data, err := marshalHash(h)
	if err != nil || data == nil || len(data) < 96 {
		return nil, err
	}
	arr := phpv.NewZArray()
	for i := 0; i < 5; i++ {
		v := binary.BigEndian.Uint32(data[4+i*4 : 8+i*4])
		arr.OffsetSet(nil, phpv.ZInt(i).ZVal(), phpv.ZInt(int64(int32(v))).ZVal())
	}
	count := binary.BigEndian.Uint64(data[88:96])
	bits := count * 8
	arr.OffsetSet(nil, phpv.ZInt(5).ZVal(), phpv.ZInt(int64(int32(uint32(bits)))).ZVal())
	arr.OffsetSet(nil, phpv.ZInt(6).ZVal(), phpv.ZInt(int64(int32(uint32(bits>>32)))).ZVal())
	arr.OffsetSet(nil, phpv.ZInt(7).ZVal(), phpv.ZString(data[24:88]).ZVal())
	return arr, nil
}

// --- SHA256/SHA224 Serializer ---
type sha256Serializer struct{}

func (s *sha256Serializer) SerializeState(h gohash.Hash) (*phpv.ZArray, error) {
	data, err := marshalHash(h)
	if err != nil || data == nil || len(data) < 108 {
		return nil, err
	}
	arr := phpv.NewZArray()
	for i := 0; i < 8; i++ {
		v := binary.BigEndian.Uint32(data[4+i*4 : 8+i*4])
		arr.OffsetSet(nil, phpv.ZInt(i).ZVal(), phpv.ZInt(int64(int32(v))).ZVal())
	}
	count := binary.BigEndian.Uint64(data[100:108])
	bits := count * 8
	arr.OffsetSet(nil, phpv.ZInt(8).ZVal(), phpv.ZInt(int64(int32(uint32(bits)))).ZVal())
	arr.OffsetSet(nil, phpv.ZInt(9).ZVal(), phpv.ZInt(int64(int32(uint32(bits>>32)))).ZVal())
	arr.OffsetSet(nil, phpv.ZInt(10).ZVal(), phpv.ZString(data[36:100]).ZVal())
	return arr, nil
}

// --- SHA512 family Serializer ---
type sha512Serializer struct{}

func (s *sha512Serializer) SerializeState(h gohash.Hash) (*phpv.ZArray, error) {
	data, err := marshalHash(h)
	if err != nil || data == nil || len(data) < 204 {
		return nil, err
	}
	arr := phpv.NewZArray()
	for i := 0; i < 8; i++ {
		v := binary.BigEndian.Uint64(data[4+i*8 : 12+i*8])
		lo := int32(uint32(v))
		hi := int32(uint32(v >> 32))
		arr.OffsetSet(nil, phpv.ZInt(i*2).ZVal(), phpv.ZInt(int64(lo)).ZVal())
		arr.OffsetSet(nil, phpv.ZInt(i*2+1).ZVal(), phpv.ZInt(int64(hi)).ZVal())
	}
	count := binary.BigEndian.Uint64(data[196:204])
	bits := count * 8
	bitsHi := uint64(0)
	if count > 0 && bits < count {
		bitsHi = 1
	}
	arr.OffsetSet(nil, phpv.ZInt(16).ZVal(), phpv.ZInt(int64(int32(uint32(bits)))).ZVal())
	arr.OffsetSet(nil, phpv.ZInt(17).ZVal(), phpv.ZInt(int64(int32(uint32(bits>>32)))).ZVal())
	arr.OffsetSet(nil, phpv.ZInt(18).ZVal(), phpv.ZInt(int64(int32(uint32(bitsHi)))).ZVal())
	arr.OffsetSet(nil, phpv.ZInt(19).ZVal(), phpv.ZInt(int64(int32(uint32(bitsHi>>32)))).ZVal())
	arr.OffsetSet(nil, phpv.ZInt(20).ZVal(), phpv.ZString(data[68:196]).ZVal())
	return arr, nil
}

// --- Simple 32-bit from BinaryMarshaler ---
type binaryMarshalUint32Serializer struct{}

func (s *binaryMarshalUint32Serializer) SerializeState(h gohash.Hash) (*phpv.ZArray, error) {
	data, err := marshalHash(h)
	if err != nil || data == nil || len(data) < 8 {
		return nil, err
	}
	arr := phpv.NewZArray()
	v := binary.BigEndian.Uint32(data[4:8])
	arr.OffsetSet(nil, phpv.ZInt(0).ZVal(), phpv.ZInt(int64(int32(v))).ZVal())
	return arr, nil
}

// --- Simple 64-bit from BinaryMarshaler ---
type binaryMarshalUint64Serializer struct{}

func (s *binaryMarshalUint64Serializer) SerializeState(h gohash.Hash) (*phpv.ZArray, error) {
	data, err := marshalHash(h)
	if err != nil || data == nil || len(data) < 12 {
		return nil, err
	}
	arr := phpv.NewZArray()
	v := binary.BigEndian.Uint64(data[4:12])
	lo := int32(uint32(v))
	hi := int32(uint32(v >> 32))
	arr.OffsetSet(nil, phpv.ZInt(0).ZVal(), phpv.ZInt(int64(lo)).ZVal())
	arr.OffsetSet(nil, phpv.ZInt(1).ZVal(), phpv.ZInt(int64(hi)).ZVal())
	return arr, nil
}

// --- CRC32 custom ---
type crc32CustomSerializer struct{}

func (s *crc32CustomSerializer) SerializeState(h gohash.Hash) (*phpv.ZArray, error) {
	if c, ok := h.(*crc32Hash); ok {
		arr := phpv.NewZArray()
		arr.OffsetSet(nil, phpv.ZInt(0).ZVal(), phpv.ZInt(int64(int32(c.crc))).ZVal())
		return arr, nil
	}
	return nil, nil
}

// --- CRC32B/C ---
type binaryMarshalCRC32Serializer struct{}

func (s *binaryMarshalCRC32Serializer) SerializeState(h gohash.Hash) (*phpv.ZArray, error) {
	data, err := marshalHash(h)
	if err != nil || data == nil || len(data) < 12 {
		return nil, err
	}
	arr := phpv.NewZArray()
	// Go stores internal state; PHP stores the complement
	v := binary.BigEndian.Uint32(data[8:12]) ^ 0xFFFFFFFF
	arr.OffsetSet(nil, phpv.ZInt(0).ZVal(), phpv.ZInt(int64(int32(v))).ZVal())
	return arr, nil
}

// --- JOAAT ---
type joaatSerializer struct{}

func (s *joaatSerializer) SerializeState(h gohash.Hash) (*phpv.ZArray, error) {
	if j, ok := h.(*joaatHash); ok {
		arr := phpv.NewZArray()
		arr.OffsetSet(nil, phpv.ZInt(0).ZVal(), phpv.ZInt(int64(int32(j.hash))).ZVal())
		return arr, nil
	}
	return nil, nil
}

// --- Replay-based ---
type replaySerializer struct{}

func (s *replaySerializer) SerializeState(h gohash.Hash) (*phpv.ZArray, error) {
	if hr, ok := h.(*hashReplayable); ok {
		arr := phpv.NewZArray()
		arr.OffsetSet(nil, phpv.ZInt(0).ZVal(), phpv.ZString(hr.data).ZVal())
		arr.OffsetSet(nil, phpv.ZInt(1).ZVal(), phpv.ZInt(int64(len(hr.data))).ZVal())
		return arr, nil
	}
	return nil, nil
}

// --- State reconstruction from PHP format ---

func unserializeFromPHPState(ctx phpv.Context, algo phpv.ZString, stateArr *phpv.ZArray) (*hashContextData, error) {
	algoLower := algo.ToLower()

	switch algoLower {
	case "md5":
		return unserializeMD5(ctx, stateArr)
	case "sha1":
		return unserializeSHA1(ctx, stateArr)
	case "sha224", "sha256":
		return unserializeSHA256(ctx, algoLower, stateArr)
	case "sha384", "sha512", "sha512/224", "sha512/256":
		return unserializeSHA512(ctx, algoLower, stateArr)
	}

	// Check the state array format
	count := stateArr.Count(ctx)

	// Simple 32-bit hashes (1 element = int32 state)
	if count == 1 {
		stateVal, _ := getArrayInt(ctx, stateArr, 0)
		return unserializeSimple32(algoLower, uint32(int32(stateVal)))
	}

	// Check for replay format: 2 elements where first is a string
	if count == 2 {
		dataVal, _ := stateArr.OffsetGet(ctx, phpv.ZInt(0))
		if dataVal != nil && dataVal.GetType() == phpv.ZtString {
			data := []byte(dataVal.Value().(phpv.ZString))
			return recreateHashContext(algoLower, false, nil, 0, 0, nil, data)
		}
		// 64-bit hash state (2 int32 values = lo, hi)
		lo, _ := getArrayInt(ctx, stateArr, 0)
		hi, _ := getArrayInt(ctx, stateArr, 1)
		state64 := uint64(uint32(int32(lo))) | (uint64(uint32(int32(hi))) << 32)
		return unserializeSimple64(algoLower, state64)
	}

	return nil, fmt.Errorf("unsupported state format for %s", algo)
}

func unserializeSimple32(algo phpv.ZString, state uint32) (*hashContextData, error) {
	switch algo {
	case "adler32":
		// Go adler32: magic "adl\x01" + 4 bytes BE
		goState := make([]byte, 8)
		copy(goState[0:4], "adl\x01")
		binary.BigEndian.PutUint32(goState[4:8], state)
		h := algos[algo]()
		if u, ok := h.(encoding.BinaryUnmarshaler); ok {
			if err := u.UnmarshalBinary(goState); err != nil {
				return nil, err
			}
		}
		return &hashContextData{Hash: h, algo: algo}, nil
	case "crc32":
		// Custom crc32 - directly set state
		h := &crc32Hash{crc: state}
		return &hashContextData{Hash: h, algo: algo}, nil
	case "crc32b":
		goState := make([]byte, 12)
		copy(goState[0:4], "crc\x01")
		binary.BigEndian.PutUint32(goState[4:8], crc32IEEEChecksum)
		binary.BigEndian.PutUint32(goState[8:12], state^0xFFFFFFFF) // PHP stores complement
		h := algos[algo]()
		if u, ok := h.(encoding.BinaryUnmarshaler); ok {
			if err := u.UnmarshalBinary(goState); err != nil {
				return nil, err
			}
		}
		return &hashContextData{Hash: h, algo: algo}, nil
	case "crc32c":
		goState := make([]byte, 12)
		copy(goState[0:4], "crc\x01")
		binary.BigEndian.PutUint32(goState[4:8], crc32CastagnoliChecksum)
		binary.BigEndian.PutUint32(goState[8:12], state^0xFFFFFFFF) // PHP stores complement
		h := algos[algo]()
		if u, ok := h.(encoding.BinaryUnmarshaler); ok {
			if err := u.UnmarshalBinary(goState); err != nil {
				return nil, err
			}
		}
		return &hashContextData{Hash: h, algo: algo}, nil
	case "fnv132":
		goState := make([]byte, 8)
		copy(goState[0:4], "fnv\x01")
		binary.BigEndian.PutUint32(goState[4:8], state)
		h := algos[algo]()
		if u, ok := h.(encoding.BinaryUnmarshaler); ok {
			if err := u.UnmarshalBinary(goState); err != nil {
				return nil, err
			}
		}
		return &hashContextData{Hash: h, algo: algo}, nil
	case "fnv1a32":
		goState := make([]byte, 8)
		copy(goState[0:4], "fnv\x02")
		binary.BigEndian.PutUint32(goState[4:8], state)
		h := algos[algo]()
		if u, ok := h.(encoding.BinaryUnmarshaler); ok {
			if err := u.UnmarshalBinary(goState); err != nil {
				return nil, err
			}
		}
		return &hashContextData{Hash: h, algo: algo}, nil
	case "joaat":
		h := &joaatHash{hash: state}
		return &hashContextData{Hash: h, algo: algo}, nil
	default:
		return nil, fmt.Errorf("unsupported simple32 hash: %s", algo)
	}
}

func unserializeSimple64(algo phpv.ZString, state uint64) (*hashContextData, error) {
	switch algo {
	case "fnv164":
		goState := make([]byte, 12)
		copy(goState[0:4], "fnv\x03")
		binary.BigEndian.PutUint64(goState[4:12], state)
		h := algos[algo]()
		if u, ok := h.(encoding.BinaryUnmarshaler); ok {
			if err := u.UnmarshalBinary(goState); err != nil {
				return nil, err
			}
		}
		return &hashContextData{Hash: h, algo: algo}, nil
	case "fnv1a64":
		goState := make([]byte, 12)
		copy(goState[0:4], "fnv\x04")
		binary.BigEndian.PutUint64(goState[4:12], state)
		h := algos[algo]()
		if u, ok := h.(encoding.BinaryUnmarshaler); ok {
			if err := u.UnmarshalBinary(goState); err != nil {
				return nil, err
			}
		}
		return &hashContextData{Hash: h, algo: algo}, nil
	default:
		return nil, fmt.Errorf("unsupported simple64 hash: %s", algo)
	}
}

// CRC32 table checksum constants used in Go's BinaryMarshal format
const (
	crc32IEEEChecksum       = 0xca87914d // IEEE table checksum
	crc32CastagnoliChecksum = 0x77428481 // Castagnoli table checksum
)

func unserializeMD5(ctx phpv.Context, arr *phpv.ZArray) (*hashContextData, error) {
	countLo, ok := getArrayInt(ctx, arr, 0)
	if !ok {
		return nil, fmt.Errorf("invalid md5 state")
	}
	countHi, _ := getArrayInt(ctx, arr, 1)
	count := uint64(uint32(countLo)) | (uint64(uint32(countHi)) << 32)

	var state [4]uint32
	for i := 0; i < 4; i++ {
		v, ok := getArrayInt(ctx, arr, i+2)
		if !ok {
			return nil, fmt.Errorf("invalid md5 state")
		}
		state[i] = uint32(int32(v))
	}

	bufStr, ok := getArrayString(ctx, arr, 6)
	if !ok {
		return nil, fmt.Errorf("invalid md5 state")
	}
	buf := []byte(bufStr)
	if len(buf) != 64 {
		return nil, fmt.Errorf("invalid md5 buffer size")
	}

	goState := make([]byte, 92)
	copy(goState[0:4], "md5\x01")
	for i := 0; i < 4; i++ {
		binary.BigEndian.PutUint32(goState[4+i*4:8+i*4], state[i])
	}
	copy(goState[20:84], buf)
	binary.BigEndian.PutUint64(goState[84:92], count)

	h := algos["md5"]()
	if u, ok2 := h.(encoding.BinaryUnmarshaler); ok2 {
		if err := u.UnmarshalBinary(goState); err != nil {
			return nil, err
		}
	}

	return &hashContextData{Hash: h, algo: "md5"}, nil
}

func unserializeSHA1(ctx phpv.Context, arr *phpv.ZArray) (*hashContextData, error) {
	var state [5]uint32
	for i := 0; i < 5; i++ {
		v, ok := getArrayIntStrict(ctx, arr, i)
		if !ok {
			return nil, fmt.Errorf("invalid sha1 state")
		}
		state[i] = uint32(int32(v))
	}

	bitsLo, ok := getArrayIntStrict(ctx, arr, 5)
	if !ok {
		return nil, fmt.Errorf("invalid sha1 state")
	}
	bitsHi, ok := getArrayIntStrict(ctx, arr, 6)
	if !ok {
		return nil, errSpecMismatch
	}
	bits := uint64(uint32(bitsLo)) | (uint64(uint32(bitsHi)) << 32)
	count := bits / 8

	bufStr, ok := getArrayString(ctx, arr, 7)
	if !ok {
		return nil, fmt.Errorf("invalid sha1 state")
	}
	buf := []byte(bufStr)
	if len(buf) != 64 {
		return nil, fmt.Errorf("invalid sha1 buffer size")
	}

	goState := make([]byte, 96)
	copy(goState[0:4], "sha\x01")
	for i := 0; i < 5; i++ {
		binary.BigEndian.PutUint32(goState[4+i*4:8+i*4], state[i])
	}
	copy(goState[24:88], buf)
	binary.BigEndian.PutUint64(goState[88:96], count)

	h := algos["sha1"]()
	if u, ok2 := h.(encoding.BinaryUnmarshaler); ok2 {
		if err := u.UnmarshalBinary(goState); err != nil {
			return nil, err
		}
	}

	return &hashContextData{Hash: h, algo: "sha1"}, nil
}

func unserializeSHA256(ctx phpv.Context, algoLower phpv.ZString, arr *phpv.ZArray) (*hashContextData, error) {
	var state [8]uint32
	for i := 0; i < 8; i++ {
		v, ok := getArrayInt(ctx, arr, i)
		if !ok {
			return nil, fmt.Errorf("invalid sha256 state")
		}
		state[i] = uint32(int32(v))
	}

	bitsLo, ok := getArrayInt(ctx, arr, 8)
	if !ok {
		return nil, fmt.Errorf("invalid sha256 state")
	}
	bitsHi, _ := getArrayInt(ctx, arr, 9)
	bits := uint64(uint32(bitsLo)) | (uint64(uint32(bitsHi)) << 32)
	count := bits / 8

	bufStr, ok := getArrayString(ctx, arr, 10)
	if !ok {
		return nil, fmt.Errorf("invalid sha256 state")
	}
	buf := []byte(bufStr)
	if len(buf) != 64 {
		return nil, fmt.Errorf("invalid sha256 buffer size")
	}

	magic := "sha\x03"
	if algoLower == "sha224" {
		magic = "sha\x02"
	}
	goState := make([]byte, 108)
	copy(goState[0:4], magic)
	for i := 0; i < 8; i++ {
		binary.BigEndian.PutUint32(goState[4+i*4:8+i*4], state[i])
	}
	copy(goState[36:100], buf)
	binary.BigEndian.PutUint64(goState[100:108], count)

	h := algos[algoLower]()
	if u, ok2 := h.(encoding.BinaryUnmarshaler); ok2 {
		if err := u.UnmarshalBinary(goState); err != nil {
			return nil, err
		}
	}

	return &hashContextData{Hash: h, algo: algoLower}, nil
}

func unserializeSHA512(ctx phpv.Context, algoLower phpv.ZString, arr *phpv.ZArray) (*hashContextData, error) {
	var state [8]uint64
	for i := 0; i < 8; i++ {
		lo, ok := getArrayInt(ctx, arr, i*2)
		if !ok {
			return nil, fmt.Errorf("invalid sha512 state")
		}
		hi, _ := getArrayInt(ctx, arr, i*2+1)
		state[i] = uint64(uint32(int32(lo))) | (uint64(uint32(int32(hi))) << 32)
	}

	bitsLo, ok := getArrayInt(ctx, arr, 16)
	if !ok {
		return nil, fmt.Errorf("invalid sha512 state")
	}
	bitsLoHi, _ := getArrayInt(ctx, arr, 17)
	bits := uint64(uint32(int32(bitsLo))) | (uint64(uint32(int32(bitsLoHi))) << 32)
	count := bits / 8

	bufStr, ok := getArrayString(ctx, arr, 20)
	if !ok {
		return nil, fmt.Errorf("invalid sha512 state")
	}
	buf := []byte(bufStr)
	if len(buf) != 128 {
		return nil, fmt.Errorf("invalid sha512 buffer size")
	}

	var magic string
	switch algoLower {
	case "sha384":
		magic = "sha\x04"
	case "sha512/224":
		magic = "sha\x05"
	case "sha512/256":
		magic = "sha\x06"
	default:
		magic = "sha\x07"
	}

	goState := make([]byte, 204)
	copy(goState[0:4], magic)
	for i := 0; i < 8; i++ {
		binary.BigEndian.PutUint64(goState[4+i*8:12+i*8], state[i])
	}
	copy(goState[68:196], buf)
	binary.BigEndian.PutUint64(goState[196:204], count)

	h := algos[algoLower]()
	if u, ok2 := h.(encoding.BinaryUnmarshaler); ok2 {
		if err := u.UnmarshalBinary(goState); err != nil {
			return nil, err
		}
	}

	return &hashContextData{Hash: h, algo: algoLower}, nil
}
