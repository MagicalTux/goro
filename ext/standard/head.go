package standard

import (
	"fmt"
	"strings"

	"github.com/MagicalTux/goro/core"
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

	fields := strings.Split(string(header), ":")
	if len(fields) < 2 {
		return phpv.ZFalse.ZVal(), nil
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
