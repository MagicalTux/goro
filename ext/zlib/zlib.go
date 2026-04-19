package zlib

import (
	"bytes"
	"compress/flate"
	"compress/gzip"
	"compress/zlib"
	"hash/crc32"
	"io"

	"github.com/KarpelesLab/goro/core"
	"github.com/KarpelesLab/goro/core/phpobj"
	"github.com/KarpelesLab/goro/core/phpv"
)

// gzcompress(string $data, int $level = -1): string|false
// Compress a string using DEFLATE with zlib header (RFC 1950)
func fncGzcompress(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var data phpv.ZString
	var level *phpv.ZInt
	_, err := core.Expand(ctx, args, &data, &level)
	if err != nil {
		return nil, err
	}

	lvl := zlib.DefaultCompression
	if level != nil {
		lvl = int(*level)
	}
	if lvl < -1 || lvl > 9 {
		return phpv.ZFalse.ZVal(), ctx.Warn("gzcompress(): compression level (%d) must be within -1..9", lvl)
	}

	var buf bytes.Buffer
	w, err := zlib.NewWriterLevel(&buf, lvl)
	if err != nil {
		return phpv.ZFalse.ZVal(), nil
	}
	w.Write([]byte(data))
	w.Close()
	return phpv.ZString(buf.Bytes()).ZVal(), nil
}

// gzuncompress(string $data, int $max_length = 0): string|false
// Uncompress a zlib-compressed string
func fncGzuncompress(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var data phpv.ZString
	var maxLength *phpv.ZInt
	_, err := core.Expand(ctx, args, &data, &maxLength)
	if err != nil {
		return nil, err
	}

	r, err := zlib.NewReader(bytes.NewReader([]byte(data)))
	if err != nil {
		ctx.Warn("gzuncompress(): data error")
		return phpv.ZFalse.ZVal(), nil
	}
	defer r.Close()

	var out []byte
	if maxLength != nil && *maxLength > 0 {
		out, err = readUpTo(r, int(*maxLength))
	} else {
		out, err = io.ReadAll(r)
	}
	if err != nil {
		ctx.Warn("gzuncompress(): data error")
		return phpv.ZFalse.ZVal(), nil
	}

	return phpv.ZString(out).ZVal(), nil
}

// gzdeflate(string $data, int $level = -1, int $encoding = ZLIB_ENCODING_RAW): string|false
// Deflate a string using raw DEFLATE (RFC 1951)
func fncGzdeflate(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var data phpv.ZString
	var level *phpv.ZInt
	var encoding *phpv.ZInt
	_, err := core.Expand(ctx, args, &data, &level, &encoding)
	if err != nil {
		return nil, err
	}

	lvl := flate.DefaultCompression
	if level != nil {
		lvl = int(*level)
	}
	if lvl < -1 || lvl > 9 {
		return phpv.ZFalse.ZVal(), ctx.Warn("gzdeflate(): compression level (%d) must be within -1..9", lvl)
	}

	var buf bytes.Buffer
	w, err := flate.NewWriter(&buf, lvl)
	if err != nil {
		return phpv.ZFalse.ZVal(), nil
	}
	w.Write([]byte(data))
	w.Close()
	return phpv.ZString(buf.Bytes()).ZVal(), nil
}

// gzinflate(string $data, int $max_length = 0): string|false
// Inflate a raw DEFLATE-compressed string
func fncGzinflate(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var data phpv.ZString
	var maxLength *phpv.ZInt
	_, err := core.Expand(ctx, args, &data, &maxLength)
	if err != nil {
		return nil, err
	}

	r := flate.NewReader(bytes.NewReader([]byte(data)))
	defer r.Close()

	var out []byte
	if maxLength != nil && *maxLength > 0 {
		out, err = readUpTo(r, int(*maxLength))
	} else {
		out, err = io.ReadAll(r)
	}
	if err != nil {
		ctx.Warn("gzinflate(): data error")
		return phpv.ZFalse.ZVal(), nil
	}

	return phpv.ZString(out).ZVal(), nil
}

// gzencode(string $data, int $level = -1, int $encoding = ZLIB_ENCODING_GZIP): string|false
// Create a gzip-compressed string
func fncGzencode(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var data phpv.ZString
	var level *phpv.ZInt
	var encoding *phpv.ZInt
	_, err := core.Expand(ctx, args, &data, &level, &encoding)
	if err != nil {
		return nil, err
	}

	lvl := gzip.DefaultCompression
	if level != nil {
		lvl = int(*level)
	}
	if lvl < -1 || lvl > 9 {
		return phpv.ZFalse.ZVal(), ctx.Warn("gzencode(): compression level (%d) must be within -1..9", lvl)
	}

	enc := ZLIB_ENCODING_GZIP
	if encoding != nil {
		enc = int(*encoding)
	}

	return encodeWithFormat(ctx, []byte(data), lvl, enc)
}

// gzdecode(string $data, int $max_length = 0): string|false
// Decodes a gzip-compressed string
func fncGzdecode(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var data phpv.ZString
	var maxLength *phpv.ZInt
	_, err := core.Expand(ctx, args, &data, &maxLength)
	if err != nil {
		return nil, err
	}

	r, err := gzip.NewReader(bytes.NewReader([]byte(data)))
	if err != nil {
		ctx.Warn("gzdecode(): data error")
		return phpv.ZFalse.ZVal(), nil
	}
	defer r.Close()

	var out []byte
	if maxLength != nil && *maxLength > 0 {
		out, err = readUpTo(r, int(*maxLength))
	} else {
		out, err = io.ReadAll(r)
	}
	if err != nil {
		ctx.Warn("gzdecode(): data error")
		return phpv.ZFalse.ZVal(), nil
	}

	return phpv.ZString(out).ZVal(), nil
}

// zlib_encode(string $data, int $encoding, int $level = -1): string|false
// Compress data with the specified encoding
func fncZlibEncode(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var data phpv.ZString
	var encoding phpv.ZInt
	var level *phpv.ZInt
	_, err := core.Expand(ctx, args, &data, &encoding, &level)
	if err != nil {
		return nil, err
	}

	lvl := -1
	if level != nil {
		lvl = int(*level)
	}
	if lvl < -1 || lvl > 9 {
		return nil, phpobj.ThrowError(ctx, phpobj.ValueError, "zlib_encode(): compression level must be -1 or between 0 and 9")
	}

	return encodeWithFormat(ctx, []byte(data), lvl, int(encoding))
}

// zlib_decode(string $data, int $max_length = 0): string|false
// Uncompress any raw/gzip/zlib encoded data
func fncZlibDecode(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var data phpv.ZString
	var maxLength *phpv.ZInt
	_, err := core.Expand(ctx, args, &data, &maxLength)
	if err != nil {
		return nil, err
	}

	raw := []byte(data)
	maxLen := 0
	if maxLength != nil {
		maxLen = int(*maxLength)
	}

	// Try gzip first (magic bytes 1f 8b)
	if len(raw) >= 2 && raw[0] == 0x1f && raw[1] == 0x8b {
		r, err := gzip.NewReader(bytes.NewReader(raw))
		if err == nil {
			defer r.Close()
			var out []byte
			if maxLen > 0 {
				out, err = readUpTo(r, maxLen)
			} else {
				out, err = io.ReadAll(r)
			}
			if err == nil {
				return phpv.ZString(out).ZVal(), nil
			}
		}
	}

	// Try zlib (CMF=0x78 for deflate with various flags)
	if len(raw) >= 2 {
		r, err := zlib.NewReader(bytes.NewReader(raw))
		if err == nil {
			defer r.Close()
			var out []byte
			if maxLen > 0 {
				out, err = readUpTo(r, maxLen)
			} else {
				out, err = io.ReadAll(r)
			}
			if err == nil {
				return phpv.ZString(out).ZVal(), nil
			}
		}
	}

	// Try raw deflate
	r2 := flate.NewReader(bytes.NewReader(raw))
	defer r2.Close()
	var out []byte
	if maxLen > 0 {
		out, err = readUpTo(r2, maxLen)
	} else {
		out, err = io.ReadAll(r2)
	}
	if err != nil {
		ctx.Warn("zlib_decode(): data error")
		return phpv.ZFalse.ZVal(), nil
	}

	return phpv.ZString(out).ZVal(), nil
}

// crc32(string $str): int
// Calculates the crc32 polynomial of a string.
// On 64-bit PHP 8 systems, returns an unsigned (positive) integer.
func fncCrc32(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var str phpv.ZString
	_, err := core.Expand(ctx, args, &str)
	if err != nil {
		return nil, err
	}

	checksum := crc32.ChecksumIEEE([]byte(str))
	// PHP 8 on 64-bit: returns as unsigned value (fits in int64 positive range)
	return phpv.ZInt(checksum).ZVal(), nil
}

// ob_gzhandler(string $data, int $status): string|false
// Output buffering callback function to gzip output
func fncObGzhandler(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var data phpv.ZString
	var status phpv.ZInt
	_, err := core.Expand(ctx, args, &data, &status)
	if err != nil {
		return nil, err
	}

	// Check if client accepts gzip encoding
	// Since we don't have access to headers in a simple way here, just compress
	var buf bytes.Buffer
	w := gzip.NewWriter(&buf)
	w.Write([]byte(data))
	w.Close()
	return phpv.ZString(buf.Bytes()).ZVal(), nil
}

// encodeWithFormat encodes data with the given zlib format constant
func encodeWithFormat(ctx phpv.Context, data []byte, level int, encoding int) (*phpv.ZVal, error) {
	var buf bytes.Buffer

	switch encoding {
	case ZLIB_ENCODING_GZIP: // same as FORCE_GZIP = 31
		lvl := level
		if lvl == -1 {
			lvl = gzip.DefaultCompression
		}
		w, err := gzip.NewWriterLevel(&buf, lvl)
		if err != nil {
			return phpv.ZFalse.ZVal(), nil
		}
		w.Write(data)
		w.Close()

	case ZLIB_ENCODING_DEFLATE: // same as FORCE_DEFLATE = 15
		lvl := level
		if lvl == -1 {
			lvl = zlib.DefaultCompression
		}
		w, err := zlib.NewWriterLevel(&buf, lvl)
		if err != nil {
			return phpv.ZFalse.ZVal(), nil
		}
		w.Write(data)
		w.Close()

	case ZLIB_ENCODING_RAW:
		lvl := level
		if lvl == -1 {
			lvl = flate.DefaultCompression
		}
		w, err := flate.NewWriter(&buf, lvl)
		if err != nil {
			return phpv.ZFalse.ZVal(), nil
		}
		w.Write(data)
		w.Close()

	default:
		return nil, phpobj.ThrowError(ctx, phpobj.ValueError, "zlib_encode(): encoding mode must be ZLIB_ENCODING_RAW, ZLIB_ENCODING_GZIP or ZLIB_ENCODING_DEFLATE")
	}

	return phpv.ZString(buf.Bytes()).ZVal(), nil
}

// readUpTo reads at most maxLen bytes from r
func readUpTo(r io.Reader, maxLen int) ([]byte, error) {
	buf := make([]byte, maxLen)
	n, err := io.ReadAtLeast(r, buf, 1)
	if err == io.ErrUnexpectedEOF {
		// Less data than maxLen - that's fine
		return buf[:n], nil
	}
	if err != nil {
		return buf[:n], err
	}
	// Check if there's more data (would exceed maxLen)
	extra := make([]byte, 1)
	n2, _ := r.Read(extra)
	if n2 > 0 {
		// Data exceeds maxLen - PHP returns false in this case
		return nil, io.ErrUnexpectedEOF
	}
	return buf[:n], nil
}
