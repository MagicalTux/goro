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

type HttpHandler struct{}

func NewHttpHandler() *HttpHandler {
	return &HttpHandler{}
}

func (hh *HttpHandler) run(ctx phpv.Context, path *url.URL, options ContextOptions) (*http.Response, error) {
	followLocation := true
	maxRedirects := 20
	client := &http.Client{}
	request := &http.Request{
		URL:    path,
		Header: make(http.Header),
		Host:   path.Host,
	}

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
			followLocation = bool(v.AsBool(ctx))
		case "protocol_version":
			// go's HTTP client ignores protocol version,
			// so no choice but to ignore here as well
		case "request_fulluri":
			// ignore as well, since the http.Request takes a url.URL
			// and no way to set this option, it seems
		case "timeout":
			client.Timeout = time.Duration(v.AsInt(ctx)) * time.Second
		case "max_redirects":
			maxRedirects = int(v.AsInt(ctx))
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

	client.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		if maxRedirects <= 0 || !followLocation {
			return http.ErrUseLastResponse
		}
		maxRedirects--
		return nil
	}

	userAgent := ctx.GetConfig("user_agent", phpv.ZNULL.ZVal())
	if userAgent != nil {
		request.Header.Set("User-Agent", string(userAgent.AsString(ctx)))
	}

	request.URL = path
	return client.Do(request)
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

	response, err := hh.run(ctx, path, options)
	if err != nil {
		return nil, err
	}

	stream := NewStream(response.Body)
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
