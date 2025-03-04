package stream

import (
	"errors"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/MagicalTux/goro/core/phpv"
)

type HttpReader struct {
	url            url.URL
	response       *http.Response
	requestFullURI bool
	followLocation bool
	ignoreErrors   bool
}

func (hr *HttpReader) Read(p []byte) (int, error) {
	return hr.response.Body.Read(p)
}

type HttpHandler struct{}

func NewHttpHandler() *HttpHandler {
	return &HttpHandler{}
}

func (hh *HttpHandler) run(ctx phpv.Context, hr *HttpReader, options ContextOptions) error {
	client := &http.Client{}
	request := &http.Request{}
	for k, v := range options {
		strVal := string(v.AsString(ctx))
		switch k {
		case "method":
			request.Method = strVal
		case "user_agent":
			request.Header.Add("User-Agent", strVal)
		case "content":
			body := strings.NewReader(strVal)
			request.Body = io.NopCloser(body)
		case "proxy":
			proxyURL, _ := url.Parse(strVal)
			client.Transport = &http.Transport{
				Proxy: http.ProxyURL(proxyURL),
			}
		case "follow_location":
			hr.followLocation = bool(v.AsBool(ctx))
		case "protocol_version":
			// go's HTTP client ignores protocol version,
			// so no choice but to ignore here as well
		case "request_fulluri":
			// ignore as well, since the http.Request takes a url.URL
			// and no way to set this option, it seems
		case "timeout":
			client.Timeout = time.Duration(v.AsInt(ctx)) * time.Second
		case "ignore_errors":
			hr.ignoreErrors = bool(v.AsBool(ctx))
		case "max_redirects":
			maxRedirect := int(v.AsInt(ctx))
			client.CheckRedirect = func(req *http.Request, via []*http.Request) error {
				if maxRedirect <= 0 || !hr.followLocation {
					return http.ErrUseLastResponse
				}
				maxRedirect--
				return nil
			}
		case "header":
			switch v.GetType() {
			case phpv.ZtString:
				fields := strings.Split(strVal, ":")
				if len(fields) >= 2 {
					key := strings.TrimSpace(fields[0])
					value := strings.TrimSpace(fields[1])
					request.Header.Add(key, value)
				}
			case phpv.ZtArray:
				for k, v := range v.AsArray(ctx).Iterate(ctx) {
					str := string(v.AsString(ctx))
					if k.GetType() == phpv.ZtInt {
						fields := strings.Split(str, ":")
						if len(fields) >= 2 {
							key := strings.TrimSpace(fields[0])
							value := strings.TrimSpace(fields[1])
							request.Header.Add(key, value)
						}
					} else {
						key := strings.TrimSpace(string(k.AsString(ctx)))
						value := strings.TrimSpace(string(v.AsString(ctx)))
						request.Header.Add(key, value)
					}
				}
			}
		}
	}

	if hr.requestFullURI {
		hr.url.Path = hr.url.RequestURI()
	}

	var err error
	hr.response, err = client.Do(request)
	return err
}

func (hh *HttpHandler) Open(ctx phpv.Context, path *url.URL, mode string, streamCtx ...phpv.Resource) (*Stream, error) {
	if mode != "r" {
		return nil, errors.New("can only read from HTTP requests")
	}
	var options ContextOptions
	if len(streamCtx) > 0 && streamCtx[0] != nil {
		if streamContext, ok := streamCtx[0].(*Context); ok {
			options = streamContext.Options["http"]
		}
	} else {
		options = ContextOptions{}
	}

	r := &HttpReader{}
	if err := hh.run(ctx, r, options); err != nil {
		return nil, err
	}
	stream := NewStream(r)

	return stream, nil
}

func (hh *HttpHandler) Exists(path *url.URL) (bool, error) {
	return false, nil
}

func (hh *HttpHandler) Stat(path *url.URL) (os.FileInfo, error) {
	return nil, ErrNotSupported
}

func (hh *HttpHandler) Lstat(path *url.URL) (os.FileInfo, error) {
	return nil, ErrNotSupported
}
