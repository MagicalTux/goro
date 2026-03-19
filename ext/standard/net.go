package standard

import (
	"encoding/binary"
	"fmt"
	"net"
	"os"

	"github.com/MagicalTux/goro/core"
	"github.com/MagicalTux/goro/core/phpv"
)

// > func int|false ip2long ( string $ip_address )
func fncIp2long(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var ipStr string
	_, err := core.Expand(ctx, args, &ipStr)
	if err != nil {
		return nil, err
	}

	ip := net.ParseIP(ipStr)
	if ip == nil {
		return phpv.ZFalse.ZVal(), nil
	}

	// Convert to IPv4
	ip4 := ip.To4()
	if ip4 == nil {
		return phpv.ZFalse.ZVal(), nil
	}

	// Convert 4 bytes to uint32 in big-endian order
	val := binary.BigEndian.Uint32(ip4)
	return phpv.ZInt(val).ZVal(), nil
}

// > func string long2ip ( int $ip )
func fncLong2ip(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var ipLong phpv.ZInt
	_, err := core.Expand(ctx, args, &ipLong)
	if err != nil {
		return nil, err
	}

	b := make([]byte, 4)
	binary.BigEndian.PutUint32(b, uint32(ipLong))
	ip := net.IPv4(b[0], b[1], b[2], b[3])
	return phpv.ZString(ip.To4().String()).ZVal(), nil
}

// > func string|false gethostname ( void )
func fncGethostname(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	hostname, err := os.Hostname()
	if err != nil {
		return phpv.ZFalse.ZVal(), nil
	}
	return phpv.ZString(hostname).ZVal(), nil
}

// > func int|bool http_response_code ([ int $response_code = 0 ] )
func fncHttpResponseCode(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var responseCode *phpv.ZInt
	_, err := core.Expand(ctx, args, &responseCode)
	if err != nil {
		return nil, err
	}

	h := ctx.HeaderContext()

	if responseCode == nil || *responseCode == 0 {
		// Get mode: return current response code
		if h == nil {
			return phpv.ZFalse.ZVal(), nil
		}
		code := h.StatusCode
		if code == 0 {
			return phpv.ZFalse.ZVal(), nil
		}
		return phpv.ZInt(code).ZVal(), nil
	}

	// Set mode
	if h == nil {
		return phpv.ZFalse.ZVal(), nil
	}

	// Check if headers are already sent
	if h.Sent {
		return phpv.ZFalse.ZVal(), ctx.Warn("http_response_code(): Cannot set response code - headers already sent")
	}

	code := int(*responseCode)
	if code < 100 || code > 599 {
		return nil, fmt.Errorf("http_response_code(): Invalid HTTP response code %d", code)
	}

	oldCode := h.StatusCode
	h.StatusCode = code
	if oldCode == 0 {
		return phpv.ZTrue.ZVal(), nil
	}
	return phpv.ZInt(oldCode).ZVal(), nil
}
