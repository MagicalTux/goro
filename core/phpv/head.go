package phpv

import "net/http"

type HeaderSender func(headers http.Header, statusCode int) error

type HeaderContext struct {
	Sent         bool
	OutputOrigin *Loc
	Headers      http.Header
	Sender       HeaderSender
	StatusCode   int
	Callbacks    []Callable
}

func (hc *HeaderContext) Add(key, value string, replace bool) {
	if replace {
		hc.Headers.Set(key, value)
	} else {
		hc.Headers.Add(key, value)

	}
}

func (hc *HeaderContext) SendHeaders(ctx Context) error {
	hc.OutputOrigin = ctx.Loc()
	hc.Sent = true

	for _, fn := range hc.Callbacks {
		_, err := ctx.Call(ctx, fn, nil)
		if err != nil {
			return err
		}
	}

	if hc.Sender != nil {
		statusCode := hc.StatusCode
		if statusCode == 0 {
			statusCode = http.StatusOK
		}

		return hc.Sender(hc.Headers, statusCode)
	}
	return nil
}
