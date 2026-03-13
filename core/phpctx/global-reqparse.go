package phpctx

import (
	"bytes"
	"errors"
	"io"
	"io/ioutil"
	"mime"
	"mime/multipart"
	"net/url"
	"strings"

	"github.com/MagicalTux/goro/core/phpv"
)

//functions for parsing request, including GET, POST, etc

func (g *Global) parsePost(p, f *phpv.ZArray) error {
	if g.req.Body == nil {
		return errors.New("missing form body")
	}
	ct := g.req.Header.Get("Content-Type")
	// RFC 7231, section 3.1.1.5 - empty type MAY be treated as application/octet-stream
	if ct == "" {
		ct = "application/octet-stream"
	}
	ct, params, _ := mime.ParseMediaType(ct)

	switch {
	case ct == "application/x-www-form-urlencoded":
		var reader io.Reader = g.req.Body
		maxFormSize := int64(10 << 20) // 10 MB is a lot of text.
		reader = io.LimitReader(g.req.Body, maxFormSize+1)
		b, err := ioutil.ReadAll(reader)
		if err != nil {
			return err
		}
		if int64(len(b)) > maxFormSize {
			return errors.New("http: POST too large")
		}
		err = g.MemAlloc(g, uint64(len(b)))
		if err != nil {
			return err
		}
		return ParseQueryToArray(g, string(b), p)
	case ct == "multipart/form-data": //, "multipart/mixed": // should we allow mixed?
		boundary, ok := params["boundary"]
		if !ok {
			return errors.New("http: POST form-data missing boundary")
		}
		read := multipart.NewReader(io.LimitReader(g.req.Body, 64*1024*1024), boundary) // max 64MB body size, TODO use php.ini to set this value

		for {
			part, err := read.NextPart()
			if err != nil {
				if err == io.EOF {
					break
				}
				return err
			}

			k := part.FormName()
			fn := part.FileName()
			if fn != "" {
				// THIS IS A FILE
				// TODO
				continue
			}
			if k == "" {
				// TODO what should we do with these?
				continue
			}

			b := &bytes.Buffer{}
			_, err = g.mem.Copy(b, part) // count size against memory usage
			if err != nil {
				return err
			}

			err = setUrlValueToArray(g, k, phpv.ZString(b.Bytes()), p)
			if err != nil {
				return err
			}
		}
		return nil
	default:
		return errors.New("Failed to parse POST: unknown content type")
	}
}

// phpURLDecode decodes a URL-encoded string like PHP's urldecode:
// - Converts %XX hex sequences to bytes
// - Leaves malformed percent sequences (like %&' or trailing %) as-is
// - Does NOT convert '+' to space (unlike query strings)
func phpURLDecode(s string) string {
	var buf strings.Builder
	for i := 0; i < len(s); i++ {
		if s[i] == '%' && i+2 < len(s) {
			hi := unhex(s[i+1])
			lo := unhex(s[i+2])
			if hi >= 0 && lo >= 0 {
				buf.WriteByte(byte(hi<<4 | lo))
				i += 2
				continue
			}
		}
		buf.WriteByte(s[i])
	}
	return buf.String()
}

func unhex(c byte) int {
	switch {
	case '0' <= c && c <= '9':
		return int(c - '0')
	case 'a' <= c && c <= 'f':
		return int(c - 'a' + 10)
	case 'A' <= c && c <= 'F':
		return int(c - 'A' + 10)
	}
	return -1
}

// parseCookiesToArray parses a Cookie header value into a ZArray following PHP's rules:
// - Cookies separated by ';'
// - Cookie names are NOT URL-decoded
// - Cookie values ARE URL-decoded
// - Dots and spaces in cookie names are replaced with underscores
// - Empty cookies (no name) are skipped
// - First occurrence wins for duplicate cookie names
func parseCookiesToArray(ctx phpv.Context, cookieHeader string, a *phpv.ZArray) {
	if cookieHeader == "" {
		return
	}

	for _, cookie := range strings.Split(cookieHeader, ";") {
		cookie = strings.TrimLeft(cookie, " \t")
		if cookie == "" {
			continue
		}

		eqIdx := strings.IndexByte(cookie, '=')
		var name, value string
		if eqIdx == -1 {
			// Cookie without '=' — PHP treats this as name="" (empty value)
			name = cookie
			value = ""
		} else {
			name = cookie[:eqIdx]
			value = cookie[eqIdx+1:]
		}

		// Trim leading/trailing spaces from name
		name = strings.TrimSpace(name)
		if name == "" {
			continue
		}

		// URL-decode the value using PHP-compatible decoding that handles
		// malformed percent sequences gracefully (leaves them as-is)
		if eqIdx != -1 {
			value = phpURLDecode(value)
		}

		// Normalize the name: replace dots and spaces with underscores
		normalizedName := strings.NewReplacer(".", "_", " ", "_").Replace(name)

		// First occurrence wins: check if key already exists
		if exists, _ := a.OffsetExists(ctx, phpv.ZString(normalizedName).ZVal()); exists {
			continue
		}

		// Use setUrlValueToArray which handles nested array syntax (name[key])
		setUrlValueToArray(ctx, name, phpv.ZString(value), a)
	}
}

// ParseQueryToArray will parse a given query string into a ZArray with PHP parsing rules
func ParseQueryToArray(ctx phpv.Context, q string, a *phpv.ZArray) error {
	// parse this ourselves instead of using url.Values so we can keep the order right
	for len(q) > 0 {
		p := strings.IndexByte(q, '&')
		if p == -1 {
			return parseQueryFragmentToArray(ctx, q, a)
		} else {
			err := parseQueryFragmentToArray(ctx, q[:p], a)
			if err != nil {
				return err
			}
			q = q[p+1:]
		}
	}
	return nil
}

func parseQueryFragmentToArray(ctx phpv.Context, f string, a *phpv.ZArray) error {
	p := strings.IndexByte(f, '=')
	if p == -1 {
		f, _ = url.QueryUnescape(f) // ignore errors
		return setUrlValueToArray(ctx, f, phpv.ZNULL, a)
	}
	k, _ := url.QueryUnescape(f[:p])
	f, _ = url.QueryUnescape(f[p+1:])
	return setUrlValueToArray(ctx, k, phpv.ZString(f), a)
}

func setUrlValueToArray(ctx phpv.Context, k string, v phpv.Val, a *phpv.ZArray) error {
	// Normalize dots and spaces in the first key component (PHP behavior)
	normalizeKey := func(s string) string {
		return strings.NewReplacer(".", "_", " ", "_").Replace(s)
	}

	p := strings.IndexByte(k, '[')
	if p == -1 {
		// simple
		return a.OffsetSet(ctx, phpv.ZString(normalizeKey(k)).ZVal(), v.ZVal())
	}
	if p == 0 {
		// failure
		return errors.New("invalid key")
	}

	// Check if there's a matching ] after the [
	q := strings.IndexByte(k[p:], ']')
	if q == -1 {
		// No matching ], treat entire key as flat name
		// Replace [, ., and space with _ (PHP behavior)
		flat := strings.NewReplacer(".", "_", " ", "_", "[", "_").Replace(k)
		return a.OffsetSet(ctx, phpv.ZString(flat).ZVal(), v.ZVal())
	}

	n := a
	zk := phpv.ZString(normalizeKey(k[:p])).ZVal()

	// loop through what remains of k
	k = k[p:]

	for {
		if len(k) == 0 {
			break
		}
		if k[0] != '[' {
			// php will ignore data after last bracket
			break
		}
		k = k[1:]
		p = strings.IndexByte(k, ']')
		if p == -1 {
			break // php will ignore data after last bracket
		}

		// use zk
		if zk == nil {
			xn := phpv.NewZArray()
			err := n.OffsetSet(ctx, zk, xn.ZVal())
			if err != nil {
				return err
			}
			n = xn
		} else if has, err := n.OffsetExists(ctx, zk); err != nil {
			return err
		} else if has {
			z, err := n.OffsetGet(ctx, zk)
			if err != nil {
				return err
			}
			z, err = z.As(ctx, phpv.ZtArray)
			if err != nil {
				return err
			}
			n = z.Value().(*phpv.ZArray)
		} else {
			xn := phpv.NewZArray()
			err = n.OffsetSet(ctx, zk, xn.ZVal())
			if err != nil {
				return err
			}
			n = xn
		}

		// update zk
		if p == 0 {
			zk = nil
			k = k[1:]
			continue
		}

		zk = phpv.ZString(k[:p]).ZVal()
		k = k[p+1:]
	}
	return n.OffsetSet(ctx, zk, v.ZVal())
}
