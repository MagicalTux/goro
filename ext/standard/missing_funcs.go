package standard

import (
	"bytes"
	"io"
	"strings"

	"github.com/MagicalTux/goro/core"
	"github.com/MagicalTux/goro/core/phpv"
	"github.com/MagicalTux/goro/core/stream"
)

// > func void clearstatcache ([ bool $clear_realpath_cache = FALSE [, string $filename = "" ]] )
func fncClearstatcache(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	// No-op in Go - we don't cache stat results
	return nil, nil
}

// > func mixed fscanf ( resource $handle , string $format [, mixed &$... ] )
func fncFscanf(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var handle phpv.Resource
	var fmt phpv.ZString
	n, err := core.Expand(ctx, args, &handle, &fmt)
	if err != nil {
		return nil, err
	}
	if handle == nil {
		return phpv.ZFalse.ZVal(), nil
	}

	var file *stream.Stream
	if handle.GetResourceType() == phpv.ResourceStream {
		file, _ = handle.(*stream.Stream)
	}
	if file == nil {
		return phpv.ZFalse.ZVal(), nil
	}

	// Read one line from the stream
	var buf []byte
	for {
		b, err := file.ReadByte()
		if err != nil {
			break
		}
		if b == '\n' {
			break
		}
		buf = append(buf, b)
	}

	if len(buf) == 0 && file.Eof() {
		return phpv.ZFalse.ZVal(), nil
	}

	line := string(buf)
	r := strings.NewReader(line)
	output, err := core.Zscanf(ctx, r, fmt, args[n:]...)
	if err != nil {
		return nil, err
	}

	return output, nil
}

// > func array get_included_files ( void )
func fncGetIncludedFiles(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	files := ctx.Global().GetIncludedFiles()
	result := phpv.NewZArray()
	for _, f := range files {
		result.OffsetSet(ctx, nil, phpv.ZString(f).ZVal())
	}
	return result.ZVal(), nil
}

// > func int|false readfile ( string $filename [, bool $use_include_path = FALSE [, resource $context ]] )
func fncReadfile(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var filename phpv.ZString
	var useIncludePath *phpv.ZBool
	var contextResource core.Optional[phpv.Resource]
	_, err := core.Expand(ctx, args, &filename, &useIncludePath, &contextResource)
	if err != nil {
		return nil, err
	}

	usePath := useIncludePath != nil && bool(*useIncludePath)

	f, err := ctx.Global().Open(ctx, filename, "r", usePath)
	if err != nil {
		return phpv.ZFalse.ZVal(), ctx.Warn("readfile(%s): Failed to open stream: %s", filename, err)
	}
	defer f.Close()

	n, err := io.Copy(ctx, f)
	if err != nil {
		return phpv.ZFalse.ZVal(), nil
	}

	return phpv.ZInt(n).ZVal(), nil
}

// > func string set_include_path ( string $new_include_path )
func fncSetIncludePath(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var newPath phpv.ZString
	_, err := core.Expand(ctx, args, &newPath)
	if err != nil {
		return nil, err
	}

	old := ctx.GetConfig("include_path", phpv.ZString(".").ZVal())
	ctx.Global().SetLocalConfig("include_path", newPath.ZVal())
	return old, nil
}

// > func string get_include_path ( void )
func fncGetIncludePath(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	return ctx.GetConfig("include_path", phpv.ZString(".").ZVal()), nil
}

// > func int|false fputcsv ( resource $handle , array $fields [, string $separator = "," [, string $enclosure = '"' [, string $escape = "\\" ]]] )
func fncFputcsv(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var handle phpv.Resource
	var fields *phpv.ZArray
	var sepArg, encArg, escArg *phpv.ZString
	_, err := core.Expand(ctx, args, &handle, &fields, &sepArg, &encArg, &escArg)
	if err != nil {
		return nil, err
	}
	if handle == nil {
		return phpv.ZFalse.ZVal(), nil
	}

	var file *stream.Stream
	if handle.GetResourceType() == phpv.ResourceStream {
		file, _ = handle.(*stream.Stream)
	}
	if file == nil {
		return phpv.ZFalse.ZVal(), nil
	}

	sep := byte(',')
	enc := byte('"')
	esc := byte('\\')

	if sepArg != nil && len(*sepArg) > 0 {
		sep = (*sepArg)[0]
	}
	if encArg != nil && len(*encArg) > 0 {
		enc = (*encArg)[0]
	}
	if escArg != nil && len(*escArg) > 0 {
		esc = (*escArg)[0]
	}

	var buf bytes.Buffer
	first := true

	it := fields.NewIterator()
	for ; it.Valid(ctx); it.Next(ctx) {
		if !first {
			buf.WriteByte(sep)
		}
		first = false

		val, err := it.Current(ctx)
		if err != nil {
			return nil, err
		}
		field := val.String()

		// Check if enclosure is needed
		needsEnclose := strings.ContainsAny(field, string([]byte{sep, enc, '\n', '\r'}))
		if esc != 0 && esc != enc {
			needsEnclose = needsEnclose || strings.ContainsRune(field, rune(esc))
		}

		if needsEnclose {
			buf.WriteByte(enc)
			for i := 0; i < len(field); i++ {
				c := field[i]
				if c == enc {
					if esc != 0 {
						buf.WriteByte(esc)
					}
					buf.WriteByte(enc)
				} else if c == esc && esc != enc && esc != 0 {
					buf.WriteByte(esc)
					buf.WriteByte(c)
				} else {
					buf.WriteByte(c)
				}
			}
			buf.WriteByte(enc)
		} else {
			buf.WriteString(field)
		}
	}
	buf.WriteByte('\n')

	n, err := file.Write(buf.Bytes())
	if err != nil {
		return phpv.ZFalse.ZVal(), nil
	}

	return phpv.ZInt(n).ZVal(), nil
}

// > func bool flock ( resource $handle , int $operation [, int &$wouldblock ] )
// Stub implementation - Go doesn't easily support flock across platforms.
func fncFlock(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var handle phpv.Resource
	var operation phpv.ZInt
	_, err := core.Expand(ctx, args, &handle, &operation)
	if err != nil {
		return nil, err
	}
	if handle == nil {
		return phpv.ZFalse.ZVal(), nil
	}

	// Set wouldblock to 0 if passed by reference
	if len(args) > 2 && args[2] != nil {
		args[2].Set(phpv.ZInt(0).ZVal())
	}

	return phpv.ZTrue.ZVal(), nil
}

// > func int|bool version_compare ( string $version1, string $version2 [, string $operator ] )
func fncVersionCompare(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var v1, v2 phpv.ZString
	var op *phpv.ZString
	_, err := core.Expand(ctx, args, &v1, &v2, &op)
	if err != nil {
		return nil, err
	}

	cmp := compareVersions(string(v1), string(v2))
	
	if op == nil {
		if cmp < 0 { return phpv.ZInt(-1).ZVal(), nil }
		if cmp > 0 { return phpv.ZInt(1).ZVal(), nil }
		return phpv.ZInt(0).ZVal(), nil
	}

	var result bool
	switch string(*op) {
	case "<", "lt":
		result = cmp < 0
	case "<=", "le":
		result = cmp <= 0
	case ">", "gt":
		result = cmp > 0
	case ">=", "ge":
		result = cmp >= 0
	case "==", "eq":
		result = cmp == 0
	case "!=", "ne", "<>":
		result = cmp != 0
	default:
		return phpv.ZNULL.ZVal(), nil
	}
	return phpv.ZBool(result).ZVal(), nil
}

func compareVersions(v1, v2 string) int {
	parts1 := splitVersion(v1)
	parts2 := splitVersion(v2)
	
	maxLen := len(parts1)
	if len(parts2) > maxLen { maxLen = len(parts2) }
	
	for i := 0; i < maxLen; i++ {
		var p1, p2 string
		if i < len(parts1) { p1 = parts1[i] }
		if i < len(parts2) { p2 = parts2[i] }
		
		cmp := compareVersionPart(p1, p2)
		if cmp != 0 { return cmp }
	}
	return 0
}

func splitVersion(v string) []string {
	var parts []string
	cur := ""
	for _, c := range v {
		if c == '.' || c == '-' || c == '_' {
			if cur != "" { parts = append(parts, cur) }
			cur = ""
		} else {
			cur += string(c)
		}
	}
	if cur != "" { parts = append(parts, cur) }
	return parts
}

func compareVersionPart(a, b string) int {
	// Special version strings have specific ordering
	specials := map[string]int{"dev": 0, "alpha": 1, "a": 1, "beta": 2, "b": 2, "rc": 3, "p": 5, "pl": 5}
	
	aNum, aIsNum := isVersionNum(a)
	bNum, bIsNum := isVersionNum(b)
	
	if aIsNum && bIsNum {
		if aNum < bNum { return -1 }
		if aNum > bNum { return 1 }
		return 0
	}
	
	aSpec, aIsSpec := specials[a]
	bSpec, bIsSpec := specials[b]
	
	if aIsSpec && bIsSpec {
		if aSpec < bSpec { return -1 }
		if aSpec > bSpec { return 1 }
		return 0
	}
	
	// Number > special string
	if aIsNum && bIsSpec { return 1 }
	if aIsSpec && bIsNum { return -1 }
	
	// Fallback to string comparison
	if a < b { return -1 }
	if a > b { return 1 }
	return 0
}

func isVersionNum(s string) (int, bool) {
	if s == "" { return 0, true }
	n := 0
	for _, c := range s {
		if c < '0' || c > '9' { return 0, false }
		n = n*10 + int(c-'0')
	}
	return n, true
}
