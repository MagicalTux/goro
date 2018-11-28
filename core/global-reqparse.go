package core

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
	p := strings.IndexByte(k, '[')
	if p == -1 {
		// simple
		return a.OffsetSet(ctx, phpv.ZString(k).ZVal(), v.ZVal())
	}
	if p == 0 {
		// failure
		return errors.New("invalid key")
	}

	n := a
	zk := phpv.ZString(k[:p]).ZVal()

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
