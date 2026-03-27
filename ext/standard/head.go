package standard

import (
	"fmt"
	"strings"

	"github.com/MagicalTux/goro/core"
	"github.com/MagicalTux/goro/core/logopt"
	"github.com/MagicalTux/goro/core/phpv"
)

// > func void header ( string $header [, bool $replace = TRUE [, int $http_response_code ]] )
func fncHeader(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var header phpv.ZString
	var replace core.Optional[phpv.ZBool]
	var responseCode core.Optional[phpv.ZInt]
	_, err := core.Expand(ctx, args, &header, &replace, &responseCode)
	if err != nil {
		return nil, err
	}

	h := ctx.HeaderContext()
	if h == nil {
		return nil, nil
	}

	// Check if headers were already sent (output has started)
	if h.Sent {
		// Get the location where output started
		file := "Unknown"
		line := 0
		if h.OutputOrigin != nil {
			file = h.OutputOrigin.Filename
			line = h.OutputOrigin.Line
		}
		ctx.Warn("Cannot modify header information - headers already sent by (output started at %s:%d)", file, line)
		return nil, nil
	}

	// Check for multi-line header (CR or LF in the header value)
	if strings.ContainsAny(string(header), "\r\n") {
		ctx.Warn("Header may not contain more than a single header, new line detected", logopt.NoFuncName(true))
		return nil, nil
	}

	fields := strings.SplitN(string(header), ":", 2)
	if len(fields) < 2 {
		// HTTP status line or invalid header - handle HTTP response code
		if responseCode.HasArg() {
			h.StatusCode = int(responseCode.Get())
		}
		return nil, nil
	}
	key := strings.TrimSpace(fields[0])
	value := strings.TrimSpace(fields[1])

	h.Add(key, value, bool(replace.GetOrDefault(true)))
	if responseCode.HasArg() {
		h.StatusCode = int(responseCode.Get())
	}

	return nil, nil
}

// > func array headers_list ( array )
func fncHeaderList(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	h := ctx.HeaderContext()

	result := phpv.NewZArray()
	if h != nil && h.Sender != nil {
		for k, values := range ctx.HeaderContext().Headers {
			for _, v := range values {
				entry := fmt.Sprintf("%s: %s", k, v)
				result.OffsetSet(ctx, nil, phpv.ZStr(entry))
			}
		}
	}

	return result.ZVal(), nil
}

// > func bool header_register_callback ( ( callable $callback )
func fncHeaderRegisterCallback(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var callable phpv.Callable
	_, err := core.Expand(ctx, args, &callable)
	if err != nil {
		return nil, err
	}

	h := ctx.HeaderContext()
	if h == nil {
		return nil, nil
	}

	h.Callbacks = append(h.Callbacks, callable)

	return phpv.ZTrue.ZVal(), nil
}

// > func void header_remove ([ string $name ] )
func fncHeaderRemove(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var key phpv.ZString
	_, err := core.Expand(ctx, args, &key)
	if err != nil {
		return nil, err
	}

	h := ctx.HeaderContext()
	if h == nil {
		return nil, nil
	}

	h.Headers.Del(string(key))
	return nil, nil
}

// > func void headers_sent ()
func fncHeadersSent(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	h := ctx.HeaderContext()
	if h == nil {
		return phpv.ZFalse.ZVal(), nil
	}
	return phpv.ZBool(h.Sent).ZVal(), nil
}
