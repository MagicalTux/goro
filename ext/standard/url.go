package standard

import (
	"net/url"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/MagicalTux/goro/core"
	"github.com/MagicalTux/goro/core/phpv"
)

// PHP_URL_* constants for parse_url component parameter
const (
	PHP_URL_SCHEME   phpv.ZInt = 0
	PHP_URL_HOST     phpv.ZInt = 1
	PHP_URL_PORT     phpv.ZInt = 2
	PHP_URL_USER     phpv.ZInt = 3
	PHP_URL_PASS     phpv.ZInt = 4
	PHP_URL_PATH     phpv.ZInt = 5
	PHP_URL_QUERY    phpv.ZInt = 6
	PHP_URL_FRAGMENT phpv.ZInt = 7
)

// PATHINFO_* constants
const (
	PATHINFO_DIRNAME  phpv.ZInt = 1
	PATHINFO_BASENAME phpv.ZInt = 2
	PATHINFO_EXTENSION phpv.ZInt = 4
	PATHINFO_FILENAME phpv.ZInt = 8
	PATHINFO_ALL      phpv.ZInt = 15
)

// PHP_QUERY_* constants for http_build_query
const (
	PHP_QUERY_RFC1738 phpv.ZInt = 1
	PHP_QUERY_RFC3986 phpv.ZInt = 2
)

// > func mixed parse_url ( string $url [, int $component = -1 ] )
func fncParseUrl(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var urlStr string
	var component *phpv.ZInt
	_, err := core.Expand(ctx, args, &urlStr, &component)
	if err != nil {
		return nil, err
	}

	comp := phpv.ZInt(-1)
	if component != nil {
		comp = *component
	}

	// PHP's parse_url is more lenient than Go's url.Parse.
	// We need to handle some edge cases.

	// If URL has no scheme, Go's url.Parse treats it differently.
	// PHP parse_url("//host/path") should parse host correctly.
	// PHP parse_url("/path") should return just path.

	parsed, parseErr := url.Parse(urlStr)
	if parseErr != nil {
		return phpv.ZFalse.ZVal(), nil
	}

	// Extract components
	scheme := parsed.Scheme
	host := parsed.Hostname()
	portStr := parsed.Port()
	var port int
	var hasPort bool
	if portStr != "" {
		port, err = strconv.Atoi(portStr)
		if err == nil {
			hasPort = true
		}
	}
	user := ""
	pass := ""
	hasPass := false
	if parsed.User != nil {
		user = parsed.User.Username()
		pass, hasPass = parsed.User.Password()
	}
	path := parsed.Path
	query := parsed.RawQuery
	fragment := parsed.Fragment

	// If a specific component is requested, return just that
	if comp >= 0 {
		switch comp {
		case PHP_URL_SCHEME:
			if scheme == "" {
				return phpv.ZNULL.ZVal(), nil
			}
			return phpv.ZString(scheme).ZVal(), nil
		case PHP_URL_HOST:
			if host == "" {
				return phpv.ZNULL.ZVal(), nil
			}
			return phpv.ZString(host).ZVal(), nil
		case PHP_URL_PORT:
			if !hasPort {
				return phpv.ZNULL.ZVal(), nil
			}
			return phpv.ZInt(port).ZVal(), nil
		case PHP_URL_USER:
			if user == "" && parsed.User == nil {
				return phpv.ZNULL.ZVal(), nil
			}
			return phpv.ZString(user).ZVal(), nil
		case PHP_URL_PASS:
			if !hasPass {
				return phpv.ZNULL.ZVal(), nil
			}
			return phpv.ZString(pass).ZVal(), nil
		case PHP_URL_PATH:
			if path == "" {
				return phpv.ZNULL.ZVal(), nil
			}
			return phpv.ZString(path).ZVal(), nil
		case PHP_URL_QUERY:
			if query == "" {
				return phpv.ZNULL.ZVal(), nil
			}
			return phpv.ZString(query).ZVal(), nil
		case PHP_URL_FRAGMENT:
			if fragment == "" {
				return phpv.ZNULL.ZVal(), nil
			}
			return phpv.ZString(fragment).ZVal(), nil
		default:
			return phpv.ZFalse.ZVal(), nil
		}
	}

	// Return associative array with all components
	result := phpv.NewZArray()
	if scheme != "" {
		result.OffsetSet(ctx, phpv.ZString("scheme"), phpv.ZString(scheme).ZVal())
	}
	if host != "" {
		result.OffsetSet(ctx, phpv.ZString("host"), phpv.ZString(host).ZVal())
	}
	if hasPort {
		result.OffsetSet(ctx, phpv.ZString("port"), phpv.ZInt(port).ZVal())
	}
	if parsed.User != nil {
		result.OffsetSet(ctx, phpv.ZString("user"), phpv.ZString(user).ZVal())
		if hasPass {
			result.OffsetSet(ctx, phpv.ZString("pass"), phpv.ZString(pass).ZVal())
		}
	}
	if path != "" {
		result.OffsetSet(ctx, phpv.ZString("path"), phpv.ZString(path).ZVal())
	}
	if query != "" {
		result.OffsetSet(ctx, phpv.ZString("query"), phpv.ZString(query).ZVal())
	}
	if fragment != "" {
		result.OffsetSet(ctx, phpv.ZString("fragment"), phpv.ZString(fragment).ZVal())
	}

	return result.ZVal(), nil
}

// > func string http_build_query ( mixed $query_data [, string $numeric_prefix = "" [, string $arg_separator = "&" [, int $enc_type = PHP_QUERY_RFC1738 ]]] )
func fncHttpBuildQuery(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var data *phpv.ZVal
	var numericPrefix *phpv.ZString
	var argSeparator *phpv.ZString
	var encType *phpv.ZInt
	_, err := core.Expand(ctx, args, &data, &numericPrefix, &argSeparator, &encType)
	if err != nil {
		return nil, err
	}

	prefix := ""
	if numericPrefix != nil {
		prefix = string(*numericPrefix)
	}
	sep := "&"
	if argSeparator != nil {
		sep = string(*argSeparator)
	}
	enc := PHP_QUERY_RFC1738
	if encType != nil {
		enc = *encType
	}

	var pairs []string

	if data.GetType() == phpv.ZtArray {
		arr := data.AsArray(ctx)
		buildQueryRecursive(ctx, arr, prefix, "", sep, enc, &pairs)
	} else if data.GetType() == phpv.ZtObject {
		// Convert object to array-like iteration
		obj := data.AsObject(ctx)
		it := obj.NewIterator()
		for ; it.Valid(ctx); it.Next(ctx) {
			k, kerr := it.Key(ctx)
			if kerr != nil {
				continue
			}
			v, verr := it.Current(ctx)
			if verr != nil {
				continue
			}
			key := k.String()
			// Check if key is numeric
			if _, numErr := strconv.Atoi(key); numErr == nil {
				key = prefix + key
			}
			encodedKey := queryEncode(key, enc)
			encodedVal := queryEncode(v.String(), enc)
			pairs = append(pairs, encodedKey+"="+encodedVal)
		}
	}

	return phpv.ZString(strings.Join(pairs, sep)).ZVal(), nil
}

func buildQueryRecursive(ctx phpv.Context, arr *phpv.ZArray, numericPrefix string, parentKey string, sep string, enc phpv.ZInt, pairs *[]string) {
	for k, v := range arr.Iterate(ctx) {
		key := k.String()

		// Apply numeric prefix for top-level numeric keys
		if parentKey == "" {
			if _, err := strconv.Atoi(key); err == nil {
				key = numericPrefix + key
			}
		}

		var fullKey string
		if parentKey == "" {
			fullKey = key
		} else {
			fullKey = parentKey + "[" + key + "]"
		}

		if v.GetType() == phpv.ZtArray {
			subArr := v.AsArray(ctx)
			buildQueryRecursive(ctx, subArr, numericPrefix, fullKey, sep, enc, pairs)
		} else {
			encodedKey := queryEncode(fullKey, enc)
			encodedVal := queryEncode(v.String(), enc)
			*pairs = append(*pairs, encodedKey+"="+encodedVal)
		}
	}
}

func queryEncode(s string, enc phpv.ZInt) string {
	if enc == PHP_QUERY_RFC3986 {
		return url.PathEscape(s)
	}
	// RFC1738: spaces become +, use url.QueryEscape
	return url.QueryEscape(s)
}

// > func mixed pathinfo ( string $path [, int $flags = PATHINFO_ALL ] )
func fncPathinfo(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var pathStr string
	var flags *phpv.ZInt
	_, err := core.Expand(ctx, args, &pathStr, &flags)
	if err != nil {
		return nil, err
	}

	f := PATHINFO_ALL
	if flags != nil {
		f = *flags
	}

	dir := filepath.Dir(pathStr)
	base := filepath.Base(pathStr)
	ext := filepath.Ext(pathStr)
	filename := base
	if ext != "" {
		filename = strings.TrimSuffix(base, ext)
		ext = ext[1:] // remove leading dot
	}

	// If a single flag is specified, return just that component as a string
	if f != PATHINFO_ALL {
		switch f {
		case PATHINFO_DIRNAME:
			return phpv.ZString(dir).ZVal(), nil
		case PATHINFO_BASENAME:
			return phpv.ZString(base).ZVal(), nil
		case PATHINFO_EXTENSION:
			return phpv.ZString(ext).ZVal(), nil
		case PATHINFO_FILENAME:
			return phpv.ZString(filename).ZVal(), nil
		}
	}

	result := phpv.NewZArray()
	if f&PATHINFO_DIRNAME != 0 {
		result.OffsetSet(ctx, phpv.ZString("dirname"), phpv.ZString(dir).ZVal())
	}
	if f&PATHINFO_BASENAME != 0 {
		result.OffsetSet(ctx, phpv.ZString("basename"), phpv.ZString(base).ZVal())
	}
	if f&PATHINFO_EXTENSION != 0 {
		result.OffsetSet(ctx, phpv.ZString("extension"), phpv.ZString(ext).ZVal())
	}
	if f&PATHINFO_FILENAME != 0 {
		result.OffsetSet(ctx, phpv.ZString("filename"), phpv.ZString(filename).ZVal())
	}

	return result.ZVal(), nil
}
