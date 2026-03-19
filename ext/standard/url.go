package standard

import (
	"net/url"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/MagicalTux/goro/core"
	"github.com/MagicalTux/goro/core/phpobj"
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

type phpParsedURL struct {
	scheme, user, pass, host, path, query, fragment string
	port                                            int
	hasScheme, hasUser, hasPass, hasHost, hasPort    bool
	hasPath, hasQuery, hasFragment                   bool
}

func isURLAlpha(c byte) bool { return (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') }

func isSchemeValid(s string) bool {
	for i := 0; i < len(s); i++ {
		c := s[i]
		if !isURLAlpha(c) && (c < '0' || c > '9') && c != '+' && c != '-' && c != '.' {
			return false
		}
	}
	return true
}

func phpParseURL(raw string) (*phpParsedURL, bool) {
	r := &phpParsedURL{}
	s := raw
	if len(s) == 0 {
		r.path = ""
		r.hasPath = true
		return r, true
	}
	if i := strings.IndexByte(s, '#'); i >= 0 {
		r.fragment = s[i+1:]
		r.hasFragment = true
		s = s[:i]
	}
	if i := strings.IndexByte(s, '?'); i >= 0 {
		r.query = s[i+1:]
		r.hasQuery = true
		s = s[:i]
	}
	// Protocol-relative //authority
	if len(s) >= 2 && s[0] == '/' && s[1] == '/' {
		rest := s[2:]
		if !phpParseAuth(r, &rest) {
			return nil, false
		}
		if len(rest) > 0 {
			r.path = rest
			r.hasPath = true
		}
		return r, true
	}
	ci := strings.IndexByte(s, ':')
	if ci < 0 {
		r.path = s
		r.hasPath = true
		return r, true
	}
	if ci == 0 {
		return nil, false
	}
	before := s[:ci]
	after := s[ci+1:]
	// scheme://authority
	if len(after) >= 2 && after[0] == '/' && after[1] == '/' {
		if !isSchemeValid(before) {
			r.path = s
			r.hasPath = true
			return r, true
		}
		r.scheme = before
		r.hasScheme = true
		rest := after[2:]
		if len(rest) == 0 || rest[0] == '/' {
			// Empty authority - only valid for file
			if strings.EqualFold(before, "file") {
				if len(rest) > 0 {
					pp := rest
					if len(pp) >= 3 && pp[0] == '/' && isURLAlpha(pp[1]) && pp[2] == ':' {
						pp = pp[1:]
					}
					r.path = pp
					r.hasPath = true
				}
				return r, true
			}
			return nil, false
		}
		if !phpParseAuth(r, &rest) {
			return nil, false
		}
		if len(rest) > 0 {
			r.path = rest
			r.hasPath = true
		}
		return r, true
	}
	// Check host:port/path pattern
	allDigit := len(after) > 0
	slashAt := -1
	for i := 0; i < len(after); i++ {
		if after[i] == '/' {
			slashAt = i
			break
		}
		if after[i] < '0' || after[i] > '9' {
			allDigit = false
			break
		}
	}
	if allDigit && slashAt >= 0 {
		ps := after[:slashAt]
		if len(ps) > 0 {
			p, e := strconv.Atoi(ps)
			if e == nil && p <= 65535 {
				r.host = before
				r.hasHost = true
				r.port = p
				r.hasPort = true
				r.path = after[slashAt:]
				r.hasPath = true
				return r, true
			}
		}
	}
	if allDigit && slashAt < 0 && !r.hasQuery && !r.hasFragment {
		ps := after
		if len(ps) > 0 {
			p, e := strconv.Atoi(ps)
			if e == nil && p <= 65535 {
				r.host = before
				r.hasHost = true
				r.port = p
				r.hasPort = true
				return r, true
			}
		} else {
			r.scheme = before
			r.hasScheme = true
			return r, true
		}
	}
	if !isSchemeValid(before) {
		r.path = s
		r.hasPath = true
		return r, true
	}
	r.scheme = before
	r.hasScheme = true
	if len(after) > 0 {
		r.path = after
		r.hasPath = true
	}
	return r, true
}

func phpParseAuth(r *phpParsedURL, s *string) bool {
	a := *s
	pi := strings.IndexByte(a, '/')
	var auth string
	if pi >= 0 {
		auth = a[:pi]
		*s = a[pi:]
	} else {
		auth = a
		*s = ""
	}
	ai := strings.LastIndexByte(auth, '@')
	if ai >= 0 {
		ui := auth[:ai]
		auth = auth[ai+1:]
		ci := strings.IndexByte(ui, ':')
		if ci >= 0 {
			r.user = ui[:ci]
			r.pass = ui[ci+1:]
			r.hasUser = true
			r.hasPass = true
		} else {
			r.user = ui
			r.hasUser = true
		}
	}
	if len(auth) == 0 {
		return false
	}
	if auth[0] == '[' {
		bi := strings.IndexByte(auth, ']')
		if bi < 0 {
			return false
		}
		r.host = auth[:bi+1]
		r.hasHost = true
		auth = auth[bi+1:]
		if len(auth) > 0 && auth[0] == ':' {
			auth = auth[1:]
			if len(auth) > 0 {
				p, e := strconv.Atoi(auth)
				if e != nil || p > 65535 {
					return false
				}
				r.port = p
				r.hasPort = true
			}
		}
		return true
	}
	ci := strings.LastIndexByte(auth, ':')
	if ci >= 0 {
		hp := auth[:ci]
		pp := auth[ci+1:]
		if len(pp) == 0 {
			if len(hp) == 0 {
				return false
			}
			r.host = hp
			r.hasHost = true
			return true
		}
		de := 0
		for de < len(pp) && pp[de] >= '0' && pp[de] <= '9' {
			de++
		}
		if de == 0 {
			return false
		}
		p, e := strconv.Atoi(pp[:de])
		if e != nil || p > 65535 {
			return false
		}
		if len(hp) == 0 {
			return false
		}
		r.host = hp
		r.hasHost = true
		r.port = p
		r.hasPort = true
	} else {
		r.host = auth
		r.hasHost = true
	}
	return true
}

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
	if comp > 7 {
		return nil, phpobj.ThrowError(ctx, phpobj.ValueError,
			"parse_url(): Argument #2 ($component) must be a valid URL component identifier, "+strconv.FormatInt(int64(comp), 10)+" given")
	}
	parsed, ok := phpParseURL(urlStr)
	if !ok {
		return phpv.ZFalse.ZVal(), nil
	}
	if comp >= 0 {
		switch comp {
		case PHP_URL_SCHEME:
			if !parsed.hasScheme {
				return phpv.ZNULL.ZVal(), nil
			}
			return phpv.ZString(parsed.scheme).ZVal(), nil
		case PHP_URL_HOST:
			if !parsed.hasHost {
				return phpv.ZNULL.ZVal(), nil
			}
			return phpv.ZString(parsed.host).ZVal(), nil
		case PHP_URL_PORT:
			if !parsed.hasPort {
				return phpv.ZNULL.ZVal(), nil
			}
			return phpv.ZInt(parsed.port).ZVal(), nil
		case PHP_URL_USER:
			if !parsed.hasUser {
				return phpv.ZNULL.ZVal(), nil
			}
			return phpv.ZString(parsed.user).ZVal(), nil
		case PHP_URL_PASS:
			if !parsed.hasPass {
				return phpv.ZNULL.ZVal(), nil
			}
			return phpv.ZString(parsed.pass).ZVal(), nil
		case PHP_URL_PATH:
			if !parsed.hasPath {
				return phpv.ZNULL.ZVal(), nil
			}
			return phpv.ZString(parsed.path).ZVal(), nil
		case PHP_URL_QUERY:
			if !parsed.hasQuery {
				return phpv.ZNULL.ZVal(), nil
			}
			return phpv.ZString(parsed.query).ZVal(), nil
		case PHP_URL_FRAGMENT:
			if !parsed.hasFragment {
				return phpv.ZNULL.ZVal(), nil
			}
			return phpv.ZString(parsed.fragment).ZVal(), nil
		default:
			return phpv.ZFalse.ZVal(), nil
		}
	}
	result := phpv.NewZArray()
	if parsed.hasScheme {
		result.OffsetSet(ctx, phpv.ZString("scheme"), phpv.ZString(parsed.scheme).ZVal())
	}
	if parsed.hasHost && parsed.host != "" {
		result.OffsetSet(ctx, phpv.ZString("host"), phpv.ZString(parsed.host).ZVal())
	}
	if parsed.hasPort {
		result.OffsetSet(ctx, phpv.ZString("port"), phpv.ZInt(parsed.port).ZVal())
	}
	if parsed.hasUser {
		result.OffsetSet(ctx, phpv.ZString("user"), phpv.ZString(parsed.user).ZVal())
	}
	if parsed.hasPass {
		result.OffsetSet(ctx, phpv.ZString("pass"), phpv.ZString(parsed.pass).ZVal())
	}
	if parsed.hasPath {
		result.OffsetSet(ctx, phpv.ZString("path"), phpv.ZString(parsed.path).ZVal())
	}
	if parsed.hasQuery {
		result.OffsetSet(ctx, phpv.ZString("query"), phpv.ZString(parsed.query).ZVal())
	}
	if parsed.hasFragment {
		result.OffsetSet(ctx, phpv.ZString("fragment"), phpv.ZString(parsed.fragment).ZVal())
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
