package util

import (
	"bytes"
	"fmt"
	"net/url"
	"strings"
)

// parse PHP compatible query string, return as map[string]interface{}

// multiple cases may happen:
// a=b (simple)
// a[b]=c (object)
// a[]=c (array)
// a[b][]=c (multi levels)
// a[][][]=c (wtf)

func ParsePhpQuery(q string) map[string]interface{} {
	res := make(map[string]interface{})

	for {
		p := strings.IndexByte(q, '&')
		if p == -1 {
			parsePhpQ(res, q)
			break
		} else {
			parsePhpQ(res, q[:p])
			q = q[p+1:]
		}
	}

	parsePhpFix(res)
	return res
}

func parsePhpFix(i interface{}) interface{} {
	switch j := i.(type) {
	case map[string]interface{}:
		for k, v := range j {
			j[k] = parsePhpFix(v)
		}
		return j
	case []interface{}:
		for k, v := range j {
			j[k] = parsePhpFix(v)
		}
		return j
	case *[]interface{}:
		return parsePhpFix(*j)
	default:
		return i
	}
}

func parsePhpQ(res map[string]interface{}, sub string) {
	var val interface{}
	if p := strings.IndexByte(sub, '='); p != -1 {
		if p == 0 {
			// ignore variable
			return
		}
		val, _ = url.QueryUnescape(sub[p+1:])
		sub = sub[:p]
	}

	sub, _ = url.QueryUnescape(sub)

	p := strings.IndexByte(sub, '[')
	if p == -1 {
		// simple
		res[sub] = val
		return
	}
	if p == 0 {
		// failure, cannot be parsed
		return
	}

	depth := []string{sub[:p]}
	sub = sub[p:]

	for {
		if len(sub) < 2 {
			break
		}
		if sub[0] != '[' {
			break
		}
		if sub[1] == ']' {
			depth = append(depth, "")
			sub = sub[2:]
			continue
		}
		p = strings.IndexByte(sub, ']')
		if p == -1 {
			break
		}
		depth = append(depth, sub[1:p])
		sub = sub[p+1:]
	}

	var resA *[]interface{}
	prev := depth[0]
	depth = depth[1:]

	for _, s := range depth {
		if s == "" {
			n := new([]interface{})
			if prev == "" {
				*resA = append(*resA, n)
			} else {
				if subn, ok := res[prev].(*[]interface{}); ok {
					n = subn
				} else {
					res[prev] = n
				}
			}
			resA = n
		} else {
			n := make(map[string]interface{})
			if prev == "" {
				*resA = append(*resA, n)
				resA = nil
			} else {
				if subn, ok := res[prev].(map[string]interface{}); ok {
					n = subn
				} else {
					res[prev] = n
				}
			}
			res = n
		}
		prev = s
	}

	if prev == "" {
		*resA = append(*resA, val)
	} else {
		res[prev] = val
	}
}

func EncodePhpQuery(q map[string]interface{}) string {
	// encode a php query
	var res []byte
	for k, v := range q {
		res = encodePhpQueryAppend(res, v, k)
	}
	return string(res)
}

func encodePhpQueryAppend(res []byte, v interface{}, k string) []byte {
	switch rv := v.(type) {
	case map[string]interface{}:
		for subk, subv := range rv {
			res = encodePhpQueryAppend(res, subv, k+"["+subk+"]")
		}
	case []interface{}:
		for _, subv := range rv {
			res = encodePhpQueryAppend(res, subv, k+"[]")
		}
	case string:
		if len(res) > 0 {
			res = append(res, '&')
		}
		res = append(res, []byte(url.QueryEscape(k))...)
		res = append(res, '=')
		res = append(res, []byte(url.QueryEscape(rv))...)
	case []byte:
		return encodePhpQueryAppend(res, string(rv), k)
	case *bytes.Buffer:
		return encodePhpQueryAppend(res, string(rv.Bytes()), k)
	default:
		res = encodePhpQueryAppend(res, []byte(fmt.Sprintf("%+v", v)), k)
	}
	return res
}
