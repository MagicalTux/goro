package curl

import (
	"bytes"
	"crypto/tls"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/KarpelesLab/goro/core"
	"github.com/KarpelesLab/goro/core/phpobj"
	"github.com/KarpelesLab/goro/core/phpv"
)

// CurlHandle is the PHP 8 class for curl handles.
var CurlHandle = &phpobj.ZClass{
	Name:         "CurlHandle",
	InternalOnly: true,
}

// curlData stores curl handle state.
type curlData struct {
	options  map[int]*phpv.ZVal
	errno    int
	errmsg   string
	response *curlResponse
}

// curlResponse stores the last response info.
type curlResponse struct {
	httpCode     int
	contentType  string
	effectiveURL string
	totalTime    float64
	headers      []byte
	body         []byte
}

func newCurlData() *curlData {
	return &curlData{
		options: make(map[int]*phpv.ZVal),
	}
}

// getCurlData extracts curlData from a ZObject.
func getCurlData(obj *phpobj.ZObject) *curlData {
	opaque := obj.GetOpaque(CurlHandle)
	if opaque == nil {
		return nil
	}
	return opaque.(*curlData)
}

// getCurlObj extracts the ZObject from args[0] and validates it.
func getCurlObj(ctx phpv.Context, args []*phpv.ZVal, funcName string) (*phpobj.ZObject, *curlData, error) {
	if len(args) < 1 || args[0] == nil {
		return nil, nil, phpobj.ThrowError(ctx, phpobj.TypeError,
			fmt.Sprintf("%s(): Argument #1 ($handle) must be of type CurlHandle, null given", funcName))
	}
	obj := &phpobj.ZObject{Class: CurlHandle}
	_, err := core.Expand(ctx, args[:1], &obj)
	if err != nil {
		return nil, nil, err
	}
	cd := getCurlData(obj)
	if cd == nil {
		return nil, nil, phpobj.ThrowError(ctx, phpobj.TypeError,
			fmt.Sprintf("%s(): Argument #1 ($handle) must be of type CurlHandle", funcName))
	}
	return obj, cd, nil
}

// > func CurlHandle curl_init ( ?string $url = null )
func fncCurlInit(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var urlStr *phpv.ZString
	core.Expand(ctx, args, &urlStr)

	cd := newCurlData()

	if urlStr != nil {
		cd.options[CURLOPT_URL] = phpv.ZString(*urlStr).ZVal()
	}

	obj, err := phpobj.NewZObjectOpaque(ctx, CurlHandle, cd)
	if err != nil {
		return nil, err
	}

	return obj.ZVal(), nil
}

// > func bool curl_setopt ( CurlHandle $handle, int $option, mixed $value )
func fncCurlSetopt(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	if len(args) < 3 {
		return nil, phpobj.ThrowError(ctx, phpobj.TypeError,
			fmt.Sprintf("curl_setopt() expects exactly 3 arguments, %d given", len(args)))
	}

	_, cd, err := getCurlObj(ctx, args, "curl_setopt")
	if err != nil {
		return nil, err
	}

	optVal := args[1]
	opt, err2 := optVal.As(ctx, phpv.ZtInt)
	if err2 != nil {
		return phpv.ZBool(false).ZVal(), nil
	}
	optInt := int(opt.Value().(phpv.ZInt))

	cd.options[optInt] = args[2]

	return phpv.ZBool(true).ZVal(), nil
}

// > func bool curl_setopt_array ( CurlHandle $handle, array $options )
func fncCurlSetoptArray(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	if len(args) < 2 {
		return nil, phpobj.ThrowError(ctx, phpobj.TypeError,
			fmt.Sprintf("curl_setopt_array() expects exactly 2 arguments, %d given", len(args)))
	}

	_, cd, err := getCurlObj(ctx, args, "curl_setopt_array")
	if err != nil {
		return nil, err
	}

	if args[1] == nil || args[1].GetType() != phpv.ZtArray {
		return phpv.ZBool(false).ZVal(), nil
	}

	arr := args[1].Value().(*phpv.ZArray)
	for k, v := range arr.Iterate(ctx) {
		if k == nil {
			continue
		}
		optVal, err2 := k.As(ctx, phpv.ZtInt)
		if err2 != nil {
			continue
		}
		optInt := int(optVal.Value().(phpv.ZInt))
		cd.options[optInt] = v
	}

	return phpv.ZBool(true).ZVal(), nil
}

// getOptBool returns an option as bool (returns defVal if not set).
func getOptBool(cd *curlData, opt int, defVal bool) bool {
	v, ok := cd.options[opt]
	if !ok || v == nil {
		return defVal
	}
	switch v.GetType() {
	case phpv.ZtBool:
		return bool(v.Value().(phpv.ZBool))
	case phpv.ZtInt:
		return int(v.Value().(phpv.ZInt)) != 0
	}
	return defVal
}

// getOptInt returns an option as int (returns defVal if not set).
func getOptInt(cd *curlData, opt int, defVal int) int {
	v, ok := cd.options[opt]
	if !ok || v == nil {
		return defVal
	}
	if v.GetType() == phpv.ZtInt {
		return int(v.Value().(phpv.ZInt))
	}
	return defVal
}

// getOptString returns an option as string (returns "" if not set).
func getOptString(cd *curlData, opt int) string {
	v, ok := cd.options[opt]
	if !ok || v == nil {
		return ""
	}
	if v.GetType() == phpv.ZtString {
		return string(v.Value().(phpv.ZString))
	}
	return ""
}

// > func string|bool curl_exec ( CurlHandle $handle )
func fncCurlExec(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	_, cd, err := getCurlObj(ctx, args, "curl_exec")
	if err != nil {
		return nil, err
	}

	// Reset error state
	cd.errno = CURLE_OK
	cd.errmsg = ""
	cd.response = nil

	// Get URL
	rawURL := getOptString(cd, CURLOPT_URL)
	if rawURL == "" {
		cd.errno = CURLE_URL_MALFORMAT
		cd.errmsg = "No URL set"
		return phpv.ZBool(false).ZVal(), nil
	}

	// Validate URL
	parsedURL, parseErr := url.Parse(rawURL)
	if parseErr != nil || parsedURL.Scheme == "" {
		cd.errno = CURLE_URL_MALFORMAT
		cd.errmsg = fmt.Sprintf("URL malformed: %s", rawURL)
		return phpv.ZBool(false).ZVal(), nil
	}

	scheme := strings.ToLower(parsedURL.Scheme)
	if scheme != "http" && scheme != "https" {
		cd.errno = CURLE_UNSUPPORTED_PROTOCOL
		cd.errmsg = fmt.Sprintf("Protocol \"%s\" not supported", parsedURL.Scheme)
		return phpv.ZBool(false).ZVal(), nil
	}

	// Determine method
	method := "GET"
	nobody := getOptBool(cd, CURLOPT_NOBODY, false)
	if nobody {
		method = "HEAD"
	}
	customReq := getOptString(cd, CURLOPT_CUSTOMREQUEST)
	if customReq != "" {
		method = strings.ToUpper(customReq)
	}
	isPost := getOptBool(cd, CURLOPT_POST, false)
	if isPost && method == "GET" {
		method = "POST"
	}

	// Build body
	var bodyReader io.Reader
	postFields := ""
	if pf, ok := cd.options[CURLOPT_POSTFIELDS]; ok && pf != nil {
		switch pf.GetType() {
		case phpv.ZtString:
			postFields = string(pf.Value().(phpv.ZString))
		case phpv.ZtArray:
			// Encode form data
			arr := pf.Value().(*phpv.ZArray)
			vals := url.Values{}
			for k, v := range arr.Iterate(ctx) {
				var key, val string
				if k != nil {
					ks, _ := k.As(ctx, phpv.ZtString)
					if ks != nil {
						key = string(ks.Value().(phpv.ZString))
					}
				}
				if v != nil {
					vs, _ := v.As(ctx, phpv.ZtString)
					if vs != nil {
						val = string(vs.Value().(phpv.ZString))
					}
				}
				vals.Set(key, val)
			}
			postFields = vals.Encode()
		}
		if postFields != "" && method == "GET" {
			method = "POST"
		}
	}
	if postFields != "" {
		bodyReader = strings.NewReader(postFields)
	}

	// Build request
	req, reqErr := http.NewRequest(method, rawURL, bodyReader)
	if reqErr != nil {
		cd.errno = CURLE_URL_MALFORMAT
		cd.errmsg = reqErr.Error()
		return phpv.ZBool(false).ZVal(), nil
	}

	// Default Content-Type for POST
	if method == "POST" && bodyReader != nil {
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	}

	// Set User-Agent
	ua := getOptString(cd, CURLOPT_USERAGENT)
	if ua != "" {
		req.Header.Set("User-Agent", ua)
	}

	// Set encoding (Accept-Encoding)
	enc := getOptString(cd, CURLOPT_ENCODING)
	if enc != "" {
		req.Header.Set("Accept-Encoding", enc)
	}

	// Set custom headers (may override Content-Type)
	if hdrOpt, ok := cd.options[CURLOPT_HTTPHEADER]; ok && hdrOpt != nil && hdrOpt.GetType() == phpv.ZtArray {
		arr := hdrOpt.Value().(*phpv.ZArray)
		for _, v := range arr.Iterate(ctx) {
			if v == nil {
				continue
			}
			vs, _ := v.As(ctx, phpv.ZtString)
			if vs == nil {
				continue
			}
			hdrLine := string(vs.Value().(phpv.ZString))
			if idx := strings.Index(hdrLine, ":"); idx > 0 {
				name := strings.TrimSpace(hdrLine[:idx])
				val := strings.TrimSpace(hdrLine[idx+1:])
				req.Header.Set(name, val)
			}
		}
	}

	// Set HTTP Basic Auth (CURLOPT_USERPWD = "user:pass")
	if userpwd := getOptString(cd, CURLOPT_USERPWD); userpwd != "" {
		if idx := strings.Index(userpwd, ":"); idx >= 0 {
			user := userpwd[:idx]
			pass := userpwd[idx+1:]
			req.SetBasicAuth(user, pass)
		}
	}

	// Build HTTP client
	timeout := getOptInt(cd, CURLOPT_TIMEOUT, 0)
	connectTimeout := getOptInt(cd, CURLOPT_CONNECTTIMEOUT, 0)
	followLoc := getOptBool(cd, CURLOPT_FOLLOWLOCATION, false)
	maxRedirs := getOptInt(cd, CURLOPT_MAXREDIRS, 10)
	sslVerifyPeer := getOptBool(cd, CURLOPT_SSL_VERIFYPEER, true)

	effectiveTimeout := time.Duration(timeout) * time.Second
	if connectTimeout > 0 {
		ct := time.Duration(connectTimeout) * time.Second
		if effectiveTimeout == 0 || ct < effectiveTimeout {
			effectiveTimeout = ct
		}
	}

	transport := &http.Transport{
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: !sslVerifyPeer, //nolint:gosec
		},
	}

	client := &http.Client{
		Transport: transport,
	}

	if effectiveTimeout > 0 {
		client.Timeout = effectiveTimeout
	}

	if !followLoc {
		client.CheckRedirect = func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		}
	} else if maxRedirs >= 0 {
		mr := maxRedirs
		client.CheckRedirect = func(req *http.Request, via []*http.Request) error {
			if len(via) >= mr {
				return fmt.Errorf("stopped after %d redirects", mr)
			}
			return nil
		}
	}

	// Execute request
	startTime := time.Now()
	resp, doErr := client.Do(req)
	elapsed := time.Since(startTime).Seconds()

	if doErr != nil {
		errStr := doErr.Error()
		if strings.Contains(errStr, "no such host") || strings.Contains(errStr, "lookup") {
			cd.errno = CURLE_COULDNT_RESOLVE_HOST
		} else if strings.Contains(errStr, "connection refused") || strings.Contains(errStr, "connect:") {
			cd.errno = CURLE_COULDNT_CONNECT
		} else if strings.Contains(errStr, "timeout") || strings.Contains(errStr, "deadline exceeded") {
			cd.errno = CURLE_OPERATION_TIMEDOUT
		} else if strings.Contains(errStr, "tls") || strings.Contains(errStr, "certificate") || strings.Contains(errStr, "x509") {
			cd.errno = CURLE_SSL_CONNECT_ERROR
		} else {
			cd.errno = CURLE_COULDNT_CONNECT
		}
		cd.errmsg = errStr
		return phpv.ZBool(false).ZVal(), nil
	}
	defer resp.Body.Close()

	// Capture headers if CURLOPT_HEADER is set
	includeHeader := getOptBool(cd, CURLOPT_HEADER, false)
	var headerBuf bytes.Buffer
	if includeHeader {
		statusText := resp.Status
		if len(statusText) > 4 {
			statusText = statusText[4:]
		}
		fmt.Fprintf(&headerBuf, "HTTP/%d.%d %d %s\r\n", resp.ProtoMajor, resp.ProtoMinor, resp.StatusCode, statusText)
		for name, vals := range resp.Header {
			for _, val := range vals {
				fmt.Fprintf(&headerBuf, "%s: %s\r\n", name, val)
			}
		}
		headerBuf.WriteString("\r\n")
	}

	// Handle CURLOPT_HEADERFUNCTION callback
	if hfOpt, ok := cd.options[CURLOPT_HEADERFUNCTION]; ok && hfOpt != nil {
		callable, err2 := core.SpawnCallable(ctx, hfOpt)
		if err2 == nil && callable != nil {
			statusText := resp.Status
			if len(statusText) > 4 {
				statusText = statusText[4:]
			}
			statusLine := fmt.Sprintf("HTTP/%d.%d %d %s\r\n", resp.ProtoMajor, resp.ProtoMinor, resp.StatusCode, statusText)
			ctx.CallZVal(ctx, callable, []*phpv.ZVal{phpv.ZString(statusLine).ZVal()}, nil)
			for name, vals := range resp.Header {
				for _, val := range vals {
					line := fmt.Sprintf("%s: %s\r\n", name, val)
					ctx.CallZVal(ctx, callable, []*phpv.ZVal{phpv.ZString(line).ZVal()}, nil)
				}
			}
			ctx.CallZVal(ctx, callable, []*phpv.ZVal{phpv.ZString("\r\n").ZVal()}, nil)
		}
	}

	// Read body
	bodyBytes, readErr := io.ReadAll(resp.Body)
	if readErr != nil {
		cd.errno = CURLE_HTTP_RETURNED_ERROR
		cd.errmsg = readErr.Error()
		return phpv.ZBool(false).ZVal(), nil
	}

	// Store response info
	effectiveURL := rawURL
	if resp.Request != nil && resp.Request.URL != nil {
		effectiveURL = resp.Request.URL.String()
	}
	cd.response = &curlResponse{
		httpCode:     resp.StatusCode,
		contentType:  resp.Header.Get("Content-Type"),
		effectiveURL: effectiveURL,
		totalTime:    elapsed,
		headers:      headerBuf.Bytes(),
		body:         bodyBytes,
	}

	// Build output
	var output []byte
	if includeHeader {
		output = append(headerBuf.Bytes(), bodyBytes...)
	} else {
		output = bodyBytes
	}

	// Handle CURLOPT_WRITEFUNCTION callback
	if wfOpt, ok := cd.options[CURLOPT_WRITEFUNCTION]; ok && wfOpt != nil {
		callable, err2 := core.SpawnCallable(ctx, wfOpt)
		if err2 == nil && callable != nil {
			ctx.CallZVal(ctx, callable, []*phpv.ZVal{phpv.ZString(output).ZVal()}, nil)
		}
	}

	returnTransfer := getOptBool(cd, CURLOPT_RETURNTRANSFER, false)
	if returnTransfer {
		return phpv.ZString(output).ZVal(), nil
	}

	// Write to output
	ctx.Write(output)
	return phpv.ZBool(true).ZVal(), nil
}

// > func mixed curl_getinfo ( CurlHandle $handle, ?int $option = null )
func fncCurlGetinfo(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	_, cd, err := getCurlObj(ctx, args, "curl_getinfo")
	if err != nil {
		return nil, err
	}

	var optPtr *phpv.ZInt
	if len(args) >= 2 && args[1] != nil && args[1].GetType() != phpv.ZtNull {
		core.Expand(ctx, args[1:2], &optPtr)
	}

	if cd.response == nil {
		if optPtr != nil {
			switch int(*optPtr) {
			case CURLINFO_HTTP_CODE:
				return phpv.ZInt(0).ZVal(), nil
			case CURLINFO_CONTENT_TYPE:
				return phpv.ZBool(false).ZVal(), nil
			case CURLINFO_EFFECTIVE_URL:
				return phpv.ZString("").ZVal(), nil
			case CURLINFO_TOTAL_TIME:
				return phpv.ZFloat(0).ZVal(), nil
			}
			return phpv.ZBool(false).ZVal(), nil
		}
		a := phpv.NewZArray()
		a.OffsetSet(ctx, phpv.ZString("http_code"), phpv.ZInt(0).ZVal())
		a.OffsetSet(ctx, phpv.ZString("content_type"), phpv.ZBool(false).ZVal())
		a.OffsetSet(ctx, phpv.ZString("url"), phpv.ZString("").ZVal())
		a.OffsetSet(ctx, phpv.ZString("total_time"), phpv.ZFloat(0).ZVal())
		return a.ZVal(), nil
	}

	r := cd.response

	if optPtr != nil {
		switch int(*optPtr) {
		case CURLINFO_HTTP_CODE:
			return phpv.ZInt(r.httpCode).ZVal(), nil
		case CURLINFO_CONTENT_TYPE:
			if r.contentType == "" {
				return phpv.ZBool(false).ZVal(), nil
			}
			return phpv.ZString(r.contentType).ZVal(), nil
		case CURLINFO_EFFECTIVE_URL:
			return phpv.ZString(r.effectiveURL).ZVal(), nil
		case CURLINFO_TOTAL_TIME:
			return phpv.ZFloat(phpv.ZFloat(r.totalTime)).ZVal(), nil
		}
		return phpv.ZBool(false).ZVal(), nil
	}

	// Return full info array
	a := phpv.NewZArray()
	a.OffsetSet(ctx, phpv.ZString("url"), phpv.ZString(r.effectiveURL).ZVal())
	a.OffsetSet(ctx, phpv.ZString("http_code"), phpv.ZInt(r.httpCode).ZVal())
	a.OffsetSet(ctx, phpv.ZString("content_type"), phpv.ZString(r.contentType).ZVal())
	a.OffsetSet(ctx, phpv.ZString("total_time"), phpv.ZFloat(phpv.ZFloat(r.totalTime)).ZVal())
	a.OffsetSet(ctx, phpv.ZString("size_download"), phpv.ZInt(len(r.body)).ZVal())
	return a.ZVal(), nil
}

// > func int curl_errno ( CurlHandle $handle )
func fncCurlErrno(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	_, cd, err := getCurlObj(ctx, args, "curl_errno")
	if err != nil {
		return nil, err
	}
	return phpv.ZInt(cd.errno).ZVal(), nil
}

// > func string curl_error ( CurlHandle $handle )
func fncCurlError(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	_, cd, err := getCurlObj(ctx, args, "curl_error")
	if err != nil {
		return nil, err
	}
	return phpv.ZString(cd.errmsg).ZVal(), nil
}

// > func void curl_close ( CurlHandle $handle )
func fncCurlClose(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	// In PHP 8, curl_close is a no-op; the handle is cleaned up by GC.
	_, _, err := getCurlObj(ctx, args, "curl_close")
	if err != nil {
		return nil, err
	}
	return phpv.ZBool(true).ZVal(), nil
}

// > func void curl_reset ( CurlHandle $handle )
func fncCurlReset(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	_, cd, err := getCurlObj(ctx, args, "curl_reset")
	if err != nil {
		return nil, err
	}
	cd.options = make(map[int]*phpv.ZVal)
	cd.errno = CURLE_OK
	cd.errmsg = ""
	cd.response = nil
	return phpv.ZBool(true).ZVal(), nil
}

// > func array curl_version ( ?int $age = null )
func fncCurlVersion(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	a := phpv.NewZArray()
	a.OffsetSet(ctx, phpv.ZString("version_number"), phpv.ZInt(0x075400).ZVal()) // 7.84.0
	a.OffsetSet(ctx, phpv.ZString("version"), phpv.ZString("7.84.0").ZVal())
	a.OffsetSet(ctx, phpv.ZString("ssl_version"), phpv.ZString("OpenSSL/3.0.0").ZVal())
	a.OffsetSet(ctx, phpv.ZString("ssl_version_number"), phpv.ZInt(0).ZVal())
	a.OffsetSet(ctx, phpv.ZString("libz_version"), phpv.ZString("1.2.11").ZVal())
	a.OffsetSet(ctx, phpv.ZString("host"), phpv.ZString("goro").ZVal())
	a.OffsetSet(ctx, phpv.ZString("age"), phpv.ZInt(10).ZVal())
	a.OffsetSet(ctx, phpv.ZString("features"), phpv.ZInt(0).ZVal())
	a.OffsetSet(ctx, phpv.ZString("protocols"), phpv.NewZArray().ZVal())
	return a.ZVal(), nil
}

// > func string curl_strerror ( int $error_number )
func fncCurlStrerror(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var errNo phpv.ZInt
	_, err := core.Expand(ctx, args, &errNo)
	if err != nil {
		return nil, err
	}

	msg := curlErrMsg(int(errNo))
	if msg == "" {
		return phpv.ZBool(false).ZVal(), nil
	}
	return phpv.ZString(msg).ZVal(), nil
}

// > func string curl_escape ( CurlHandle $handle, string $string )
func fncCurlEscape(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	_, _, err := getCurlObj(ctx, args, "curl_escape")
	if err != nil {
		return nil, err
	}
	if len(args) < 2 || args[1] == nil {
		return nil, phpobj.ThrowError(ctx, phpobj.TypeError, "curl_escape(): Argument #2 ($string) must be of type string")
	}
	sv, err2 := args[1].As(ctx, phpv.ZtString)
	if err2 != nil {
		return phpv.ZBool(false).ZVal(), nil
	}
	s := string(sv.Value().(phpv.ZString))
	return phpv.ZString(url.QueryEscape(s)).ZVal(), nil
}

// > func string curl_unescape ( CurlHandle $handle, string $string )
func fncCurlUnescape(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	_, _, err := getCurlObj(ctx, args, "curl_unescape")
	if err != nil {
		return nil, err
	}
	if len(args) < 2 || args[1] == nil {
		return nil, phpobj.ThrowError(ctx, phpobj.TypeError, "curl_unescape(): Argument #2 ($string) must be of type string")
	}
	sv, err2 := args[1].As(ctx, phpv.ZtString)
	if err2 != nil {
		return phpv.ZBool(false).ZVal(), nil
	}
	s := string(sv.Value().(phpv.ZString))
	decoded, decErr := url.QueryUnescape(s)
	if decErr != nil {
		return phpv.ZBool(false).ZVal(), nil
	}
	return phpv.ZString(decoded).ZVal(), nil
}

// curlErrMsg returns the error message for a CURLE_* code.
func curlErrMsg(code int) string {
	switch code {
	case CURLE_OK:
		return "No error"
	case CURLE_UNSUPPORTED_PROTOCOL:
		return "Unsupported protocol"
	case CURLE_URL_MALFORMAT:
		return "URL malformed"
	case CURLE_COULDNT_RESOLVE_HOST:
		return "Could not resolve host"
	case CURLE_COULDNT_CONNECT:
		return "Could not connect"
	case CURLE_HTTP_RETURNED_ERROR:
		return "HTTP returned error"
	case CURLE_OPERATION_TIMEDOUT:
		return "Operation timed out"
	case CURLE_SSL_CONNECT_ERROR:
		return "SSL connection error"
	}
	return ""
}
