package stream

import (
	"bytes"
	"encoding/base64"
	"net/url"
	"os"
	"strings"

	"github.com/MagicalTux/goro/core/phpv"
)

// dataHandler implements the "data:" stream wrapper.
// Supports RFC 2397 data: URIs: data:[<mediatype>][;base64],<data>
type dataHandler struct{}

func init() {
	DataHandler = &dataHandler{}
}

// DataHandler is the global data: stream handler instance, registered during Global init.
var DataHandler Handler

func (h *dataHandler) Open(ctx phpv.Context, u *url.URL, mode string, streamCtx ...phpv.Resource) (*Stream, error) {
	// Reconstruct the data URI from the parsed URL.
	// url.Parse splits "data:text/plain,foobar" into scheme="data" and opaque="text/plain,foobar"
	raw := u.Opaque
	if raw == "" {
		// Fallback: try host + path
		raw = u.Host + u.Path
		if u.RawQuery != "" {
			raw += "?" + u.RawQuery
		}
	}

	// Parse the data URI: [<mediatype>][;base64],<data>
	commaIdx := strings.Index(raw, ",")
	if commaIdx < 0 {
		return nil, os.ErrInvalid
	}

	meta := raw[:commaIdx]
	data := raw[commaIdx+1:]

	isBase64 := strings.HasSuffix(meta, ";base64")

	var content []byte
	if isBase64 {
		decoded, err := base64.StdEncoding.DecodeString(data)
		if err != nil {
			return nil, err
		}
		content = decoded
	} else {
		// URL-decode the data portion
		unescaped, err := url.QueryUnescape(data)
		if err != nil {
			content = []byte(data) // fallback to raw
		} else {
			content = []byte(unescaped)
		}
	}

	reader := bytes.NewReader(content)
	s := NewStream(reader)
	s.SetAttr("stream_type", "RFC2397")
	s.SetAttr("mode", "r")
	s.SetAttr("uri", u.String())
	s.ResourceType = phpv.ResourceStream

	return s, nil
}

func (h *dataHandler) Exists(u *url.URL) (bool, error) {
	return true, nil // data: URIs always "exist"
}

func (h *dataHandler) Stat(u *url.URL) (os.FileInfo, error) {
	return nil, os.ErrNotExist
}

func (h *dataHandler) Lstat(u *url.URL) (os.FileInfo, error) {
	return nil, os.ErrNotExist
}
