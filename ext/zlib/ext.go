package zlib

import (
	"compress/flate"

	"github.com/MagicalTux/goro/core"
	"github.com/MagicalTux/goro/core/phpctx"
	"github.com/MagicalTux/goro/core/phpobj"
	"github.com/MagicalTux/goro/core/phpv"
	"github.com/MagicalTux/goro/core/stream"
)

// Constants
const (
	ZLIB_ENCODING_RAW     = -15
	ZLIB_ENCODING_GZIP    = 31
	ZLIB_ENCODING_DEFLATE = 15
	ZLIB_NO_FLUSH         = 0
	ZLIB_PARTIAL_FLUSH    = 1
	ZLIB_SYNC_FLUSH       = 2
	ZLIB_FULL_FLUSH       = 3
	ZLIB_FINISH           = 4
	FORCE_GZIP            = 31
	FORCE_DEFLATE         = 15
	ZLIB_FILTERED         = 1
	ZLIB_HUFFMAN_ONLY     = 2
	ZLIB_RLE              = 3
	ZLIB_FIXED            = 4
	ZLIB_DEFAULT_STRATEGY = 0
)

func init() {
	// Register zlib.deflate and zlib.inflate as builtin stream filters
	stream.RegisterBuiltinFilterFactory(
		[]string{"zlib.deflate", "zlib.inflate"},
		func(name string, params map[string]interface{}) stream.StreamFilter {
			switch name {
			case "zlib.deflate":
				level := flate.DefaultCompression
				if lvl, ok := params["level"]; ok {
					switch v := lvl.(type) {
					case int:
						level = v
					case phpv.ZInt:
						level = int(v)
					}
				}
				return NewZlibDeflateFilter(level)
			case "zlib.inflate":
				return &ZlibInflateFilter{}
			}
			return nil
		},
	)

	phpctx.RegisterExt(&phpctx.Ext{
		Name:    "zlib",
		Version: core.VERSION,
		Classes: []*phpobj.ZClass{},
		Functions: map[string]*phpctx.ExtFunction{
			// Compression / decompression
			"gzcompress":   {Func: fncGzcompress},
			"gzuncompress": {Func: fncGzuncompress},
			"gzdeflate":    {Func: fncGzdeflate},
			"gzinflate":    {Func: fncGzinflate},
			"gzencode":     {Func: fncGzencode},
			"gzdecode":     {Func: fncGzdecode},
			"zlib_encode":  {Func: fncZlibEncode},
			"zlib_decode":  {Func: fncZlibDecode},
			// CRC
			"crc32": {Func: fncCrc32},
			// Gzip file functions
			"gzopen":     {Func: fncGzopen},
			"gzclose":    {Func: fncGzclose},
			"gzread":     {Func: fncGzread},
			"gzwrite":    {Func: fncGzwrite},
			"gzputs":     {Func: fncGzwrite}, // alias for gzwrite
			"gzgets":     {Func: fncGzgets},
			"gzeof":      {Func: fncGzeof},
			"gzseek":     {Func: fncGzseek},
			"gztell":     {Func: fncGztell},
			"gzrewind":   {Func: fncGzrewind},
			"gzpassthru": {Func: fncGzpassthru},
			"gzgetc":     {Func: fncGzgetc},
			"readgzfile": {Func: fncReadgzfile},
			// Output buffering
			"ob_gzhandler": {Func: fncObGzhandler},
		},
		Constants: map[phpv.ZString]phpv.Val{
			"ZLIB_ENCODING_RAW":     phpv.ZInt(ZLIB_ENCODING_RAW),
			"ZLIB_ENCODING_GZIP":    phpv.ZInt(ZLIB_ENCODING_GZIP),
			"ZLIB_ENCODING_DEFLATE": phpv.ZInt(ZLIB_ENCODING_DEFLATE),
			"ZLIB_NO_FLUSH":         phpv.ZInt(ZLIB_NO_FLUSH),
			"ZLIB_PARTIAL_FLUSH":    phpv.ZInt(ZLIB_PARTIAL_FLUSH),
			"ZLIB_SYNC_FLUSH":       phpv.ZInt(ZLIB_SYNC_FLUSH),
			"ZLIB_FULL_FLUSH":       phpv.ZInt(ZLIB_FULL_FLUSH),
			"ZLIB_FINISH":           phpv.ZInt(ZLIB_FINISH),
			"FORCE_GZIP":            phpv.ZInt(FORCE_GZIP),
			"FORCE_DEFLATE":         phpv.ZInt(FORCE_DEFLATE),
			"ZLIB_FILTERED":         phpv.ZInt(ZLIB_FILTERED),
			"ZLIB_HUFFMAN_ONLY":     phpv.ZInt(ZLIB_HUFFMAN_ONLY),
			"ZLIB_RLE":              phpv.ZInt(ZLIB_RLE),
			"ZLIB_FIXED":            phpv.ZInt(ZLIB_FIXED),
			"ZLIB_DEFAULT_STRATEGY": phpv.ZInt(ZLIB_DEFAULT_STRATEGY),
		},
		// Register the compress.zlib:// stream wrapper per-global
		OnGlobalCreate: func(g *phpctx.Global) {
			g.RegisterStreamHandler("compress.zlib", &ZlibStreamHandler{})
		},
	})
}
