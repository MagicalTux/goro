package standard

import (
	"fmt"
	"log/syslog"
	"net"
	"strings"
	"time"

	"github.com/MagicalTux/goro/core"
	"github.com/MagicalTux/goro/core/phpobj"
	"github.com/MagicalTux/goro/core/phpv"
	"github.com/MagicalTux/goro/core/stream"
)

func fncGethostbyname(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var hostname phpv.ZString
	_, err := core.Expand(ctx, args, &hostname)
	if err != nil { return nil, err }
	host := string(hostname)
	if strings.ContainsRune(host, 0) {
		return nil, phpobj.ThrowError(ctx, phpobj.ValueError, "gethostbyname(): Argument #1 ($hostname) must not contain any null bytes")
	}
	if host == "" { return phpv.ZString("").ZVal(), nil }
	addrs, e := net.LookupHost(host)
	if e != nil || len(addrs) == 0 { return hostname.ZVal(), nil }
	for _, a := range addrs {
		if ip := net.ParseIP(a); ip != nil && ip.To4() != nil { return phpv.ZString(a).ZVal(), nil }
	}
	return phpv.ZString(addrs[0]).ZVal(), nil
}

func fncGethostbyaddr(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var ipStr phpv.ZString
	_, err := core.Expand(ctx, args, &ipStr)
	if err != nil { return nil, err }
	if ipStr == "" { return phpv.ZFalse.ZVal(), ctx.Warn("gethostbyaddr(): Address is not a valid IPv4 or IPv6 address") }
	names, e := net.LookupAddr(string(ipStr))
	if e != nil || len(names) == 0 { return phpv.ZFalse.ZVal(), ctx.Warn("gethostbyaddr(): Host not found") }
	return phpv.ZString(strings.TrimSuffix(names[0], ".")).ZVal(), nil
}

func fncGethostbynamel(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var hostname phpv.ZString
	_, err := core.Expand(ctx, args, &hostname)
	if err != nil { return nil, err }
	addrs, e := net.LookupHost(string(hostname))
	if e != nil { return phpv.ZFalse.ZVal(), nil }
	result := phpv.NewZArray()
	for _, a := range addrs {
		if ip := net.ParseIP(a); ip != nil && ip.To4() != nil { result.OffsetSet(ctx, nil, phpv.ZString(a).ZVal()) }
	}
	return result.ZVal(), nil
}

func fncOpenlog(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) { return phpv.ZTrue.ZVal(), nil }
func fncCloselog(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) { return phpv.ZTrue.ZVal(), nil }

func fncSyslog(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var priority phpv.ZInt
	var message phpv.ZString
	_, err := core.Expand(ctx, args, &priority, &message)
	if err != nil { return nil, err }
	var p syslog.Priority
	switch int(priority) {
	case 0: p = syslog.LOG_EMERG
	case 1: p = syslog.LOG_ALERT
	case 2: p = syslog.LOG_CRIT
	case 3: p = syslog.LOG_ERR
	case 4: p = syslog.LOG_WARNING
	case 5: p = syslog.LOG_NOTICE
	case 6: p = syslog.LOG_INFO
	default: p = syslog.LOG_DEBUG
	}
	w, e := syslog.New(p, "")
	if e != nil { return phpv.ZFalse.ZVal(), nil }
	defer w.Close()
	fmt.Fprint(w, string(message))
	return phpv.ZTrue.ZVal(), nil
}

func fncGetprotobyname(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var name phpv.ZString
	_, err := core.Expand(ctx, args, &name)
	if err != nil { return nil, err }
	m := map[string]int{"ip":0,"icmp":1,"igmp":2,"ggp":3,"tcp":6,"egp":8,"pup":12,"udp":17,"hmp":20,"xns-idp":22,"rdp":27}
	if v, ok := m[strings.ToLower(string(name))]; ok { return phpv.ZInt(v).ZVal(), nil }
	return phpv.ZFalse.ZVal(), nil
}

func fncGetprotobynumber(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var number phpv.ZInt
	_, err := core.Expand(ctx, args, &number)
	if err != nil { return nil, err }
	m := map[int]string{0:"ip",1:"icmp",2:"igmp",3:"ggp",6:"tcp",8:"egp",12:"pup",17:"udp",20:"hmp",22:"xns-idp",27:"rdp"}
	if v, ok := m[int(number)]; ok { return phpv.ZString(v).ZVal(), nil }
	return phpv.ZFalse.ZVal(), nil
}

func fncCheckdnsrr(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var host phpv.ZString
	var dnsType core.Optional[phpv.ZString]
	_, err := core.Expand(ctx, args, &host, &dnsType)
	if err != nil { return nil, err }
	if host == "" { return phpv.ZFalse.ZVal(), ctx.Warn("checkdnsrr(): Argument #1 ($hostname) must not be empty") }
	return phpv.ZFalse.ZVal(), nil
}

func fncDnsGetRecord(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var hostname phpv.ZString
	_, err := core.Expand(ctx, args, &hostname)
	if err != nil { return nil, err }
	if hostname == "" { return phpv.ZFalse.ZVal(), ctx.Warn("dns_get_record(): Argument #1 ($hostname) must not be empty") }
	return phpv.NewZArray().ZVal(), nil
}

func fncGetmxrr(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	return phpv.ZFalse.ZVal(), nil
}

func fncFsockopen(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var hostname phpv.ZString
	var port core.Optional[phpv.ZInt]
	_, err := core.Expand(ctx, args, &hostname, &port)
	if err != nil { return nil, err }
	host := string(hostname)
	if strings.ContainsRune(host, 0) {
		return phpv.ZFalse.ZVal(), ctx.Warn("fsockopen(): Argument #1 ($hostname) must not contain any null bytes")
	}
	proto := "tcp"
	if strings.HasPrefix(host, "tcp://") { host = host[6:] } else if strings.HasPrefix(host, "udp://") { proto = "udp"; host = host[6:] }
	p := 0
	if port.HasArg() { p = int(port.Get()) }
	addr := fmt.Sprintf("%s:%d", host, p)
	conn, de := net.DialTimeout(proto, addr, 60*time.Second)
	if de != nil {
		return phpv.ZFalse.ZVal(), ctx.Warn("fsockopen(): Unable to connect to %s (%s)", addr, de.Error())
	}
	s := stream.NewStream(conn)
	s.ResourceType = phpv.ResourceStream
	s.ResourceID = ctx.Global().NextResourceID()
	return s.ZVal(), nil
}

func fncSetcookie(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	if len(args) == 0 { return nil, fmt.Errorf("setcookie() expects at least 1 argument") }
	name := args[0].String()
	value, path, domain, samesite := "", "", "", ""
	expires := 0
	secure, httponly, partitioned := false, false, false
	if len(args) >= 2 { value = args[1].String() }
	if len(args) >= 3 && args[2].GetType() == phpv.ZtArray {
		opts := args[2].AsArray(ctx)
		if v, _, e := opts.OffsetCheck(ctx, phpv.ZStr("expires")); e == nil && v != nil { expires = int(v.AsInt(ctx)) }
		if v, _, e := opts.OffsetCheck(ctx, phpv.ZStr("path")); e == nil && v != nil { path = v.String() }
		if v, _, e := opts.OffsetCheck(ctx, phpv.ZStr("domain")); e == nil && v != nil { domain = v.String() }
		if v, _, e := opts.OffsetCheck(ctx, phpv.ZStr("secure")); e == nil && v != nil { secure = bool(v.AsBool(ctx)) }
		if v, _, e := opts.OffsetCheck(ctx, phpv.ZStr("httponly")); e == nil && v != nil { httponly = bool(v.AsBool(ctx)) }
		if v, _, e := opts.OffsetCheck(ctx, phpv.ZStr("samesite")); e == nil && v != nil { samesite = v.String() }
		if v, _, e := opts.OffsetCheck(ctx, phpv.ZStr("partitioned")); e == nil && v != nil { partitioned = bool(v.AsBool(ctx)) }
	} else {
		if len(args) >= 3 { expires = int(args[2].AsInt(ctx)) }
		if len(args) >= 4 { path = args[3].String() }
		if len(args) >= 5 { domain = args[4].String() }
		if len(args) >= 6 { secure = bool(args[5].AsBool(ctx)) }
		if len(args) >= 7 { httponly = bool(args[6].AsBool(ctx)) }
	}
	h := ctx.HeaderContext()
	if h == nil { return phpv.ZFalse.ZVal(), nil }
	var c strings.Builder
	if value == "" && expires == 0 {
		c.WriteString(name + "=deleted; expires=Thu, 01 Jan 1970 00:00:01 GMT; Max-Age=0")
	} else {
		c.WriteString(name + "=" + phpCookieEncode(value))
		if expires != 0 {
			ma := int64(expires) - time.Now().Unix()
			if ma < 0 { ma = 0 }
			t := time.Unix(int64(expires), 0).UTC()
			c.WriteString("; expires=" + t.Format("Mon, 02 Jan 2006 15:04:05") + " GMT")
			c.WriteString(fmt.Sprintf("; Max-Age=%d", ma))
		}
	}
	if path != "" { c.WriteString("; path=" + path) }
	if domain != "" { c.WriteString("; domain=" + domain) }
	if secure { c.WriteString("; secure") }
	if httponly { c.WriteString("; HttpOnly") }
	if samesite != "" { c.WriteString("; SameSite=" + samesite) }
	if partitioned { c.WriteString("; Partitioned") }
	h.Add("Set-Cookie", c.String(), false)
	return phpv.ZTrue.ZVal(), nil
}

func phpCookieEncode(s string) string {
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		ch := s[i]
		if (ch >= 'A' && ch <= 'Z') || (ch >= 'a' && ch <= 'z') || (ch >= '0' && ch <= '9') || ch == '-' || ch == '.' || ch == '_' || ch == '~' {
			b.WriteByte(ch)
		} else { fmt.Fprintf(&b, "%%%02X", ch) }
	}
	return b.String()
}

func fncSetrawcookie(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) { return fncSetcookie(ctx, args) }

func fncStreamGetTransports(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	r := phpv.NewZArray()
	for _, t := range []string{"tcp","udp","unix","udg"} { r.OffsetSet(ctx, nil, phpv.ZString(t).ZVal()) }
	return r.ZVal(), nil
}

func fncStreamGetWrappers(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	r := phpv.NewZArray()
	for _, w := range []string{"php","file","glob","data","http","ftp"} { r.OffsetSet(ctx, nil, phpv.ZString(w).ZVal()) }
	return r.ZVal(), nil
}

func fncStreamSetBlocking(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) { return phpv.ZTrue.ZVal(), nil }
func fncStreamSetTimeout(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) { return phpv.ZTrue.ZVal(), nil }
func fncStreamSetWriteBuffer(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) { return phpv.ZInt(0).ZVal(), nil }
func fncStreamSetReadBuffer(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) { return phpv.ZInt(0).ZVal(), nil }
func fncStreamSetChunkSize(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) { return phpv.ZInt(8192).ZVal(), nil }
func fncStreamSelect(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	if len(args) < 4 { return phpv.ZFalse.ZVal(), nil }
	cnt := 0; if args[0].GetType() == phpv.ZtArray { cnt = int(args[0].AsArray(ctx).Count(ctx)) }
	return phpv.ZInt(cnt).ZVal(), nil
}
func fncStreamResolveIncludePath(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	return phpv.ZFalse.ZVal(), nil
}
func fncNetGetInterfaces(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	ifaces, err := net.Interfaces()
	if err != nil { return phpv.ZFalse.ZVal(), nil }
	result := phpv.NewZArray()
	for _, iface := range ifaces {
		ia := phpv.NewZArray()
		ia.OffsetSet(ctx, phpv.ZStr("description"), phpv.ZString(iface.Name).ZVal())
		ia.OffsetSet(ctx, phpv.ZStr("mtu"), phpv.ZInt(iface.MTU).ZVal())
		ia.OffsetSet(ctx, phpv.ZStr("up"), phpv.ZBool(iface.Flags&net.FlagUp!=0).ZVal())
		addrs, _ := iface.Addrs()
		ua := phpv.NewZArray()
		for _, addr := range addrs {
			ae := phpv.NewZArray()
			if ipn, ok := addr.(*net.IPNet); ok {
				if ipn.IP.To4() != nil {
					ae.OffsetSet(ctx, phpv.ZStr("family"), phpv.ZInt(2).ZVal())
					ae.OffsetSet(ctx, phpv.ZStr("address"), phpv.ZString(ipn.IP.String()).ZVal())
				} else {
					ae.OffsetSet(ctx, phpv.ZStr("family"), phpv.ZInt(10).ZVal())
					ae.OffsetSet(ctx, phpv.ZStr("address"), phpv.ZString(ipn.IP.String()).ZVal())
				}
			}
			ua.OffsetSet(ctx, nil, ae.ZVal())
		}
		ia.OffsetSet(ctx, phpv.ZStr("unicast"), ua.ZVal())
		result.OffsetSet(ctx, phpv.ZStr(iface.Name), ia.ZVal())
	}
	return result.ZVal(), nil
}
func fncFpassthru(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var h phpv.Resource
	_, err := core.Expand(ctx, args, &h)
	if err != nil { return nil, err }
	s, ok := h.(*stream.Stream)
	if !ok { return phpv.ZFalse.ZVal(), nil }
	tot := 0
	buf := make([]byte, 8192)
	for { n, re := s.Read(buf); if n > 0 { ctx.Write(buf[:n]); tot += n }; if re != nil { break } }
	return phpv.ZInt(tot).ZVal(), nil
}

const (
	LOG_EMERG phpv.ZInt = 0; LOG_ALERT phpv.ZInt = 1; LOG_CRIT phpv.ZInt = 2; LOG_ERR phpv.ZInt = 3
	LOG_WARNING phpv.ZInt = 4; LOG_NOTICE phpv.ZInt = 5; LOG_INFO phpv.ZInt = 6; LOG_DEBUG phpv.ZInt = 7
	LOG_KERN phpv.ZInt = 0; LOG_USER phpv.ZInt = 8; LOG_MAIL phpv.ZInt = 16; LOG_DAEMON phpv.ZInt = 24
	LOG_AUTH phpv.ZInt = 32; LOG_SYSLOG phpv.ZInt = 40; LOG_LPR phpv.ZInt = 48; LOG_NEWS phpv.ZInt = 56
	LOG_UUCP phpv.ZInt = 64; LOG_CRON phpv.ZInt = 72; LOG_AUTHPRIV phpv.ZInt = 80
	LOG_LOCAL0 phpv.ZInt = 128; LOG_LOCAL1 phpv.ZInt = 136; LOG_LOCAL2 phpv.ZInt = 144; LOG_LOCAL3 phpv.ZInt = 152
	LOG_LOCAL4 phpv.ZInt = 160; LOG_LOCAL5 phpv.ZInt = 168; LOG_LOCAL6 phpv.ZInt = 176; LOG_LOCAL7 phpv.ZInt = 184
	LOG_PID phpv.ZInt = 1; LOG_CONS phpv.ZInt = 2; LOG_ODELAY phpv.ZInt = 4
	LOG_NDELAY phpv.ZInt = 8; LOG_NOWAIT phpv.ZInt = 16; LOG_PERROR phpv.ZInt = 32
	STREAM_SERVER_BIND phpv.ZInt = 4; STREAM_SERVER_LISTEN phpv.ZInt = 8; STREAM_CLIENT_CONNECT phpv.ZInt = 4
	STREAM_FILTER_READ phpv.ZInt = 1; STREAM_FILTER_WRITE phpv.ZInt = 2; STREAM_FILTER_ALL phpv.ZInt = 3
	STREAM_IS_URL phpv.ZInt = 1
	PSFS_PASS_ON phpv.ZInt = 2; PSFS_FEED_ME phpv.ZInt = 0; PSFS_ERR_FATAL phpv.ZInt = 1
	STREAM_SHUT_RD phpv.ZInt = 0; STREAM_SHUT_WR phpv.ZInt = 1; STREAM_SHUT_RDWR phpv.ZInt = 2
	DNS_A phpv.ZInt = 1; DNS_NS phpv.ZInt = 2; DNS_CNAME phpv.ZInt = 16; DNS_SOA phpv.ZInt = 32
	DNS_PTR phpv.ZInt = 2048; DNS_HINFO phpv.ZInt = 4096; DNS_CAA phpv.ZInt = 8192; DNS_MX phpv.ZInt = 16384
	DNS_TXT phpv.ZInt = 32768; DNS_SRV phpv.ZInt = 33554432; DNS_NAPTR phpv.ZInt = 67108864
	DNS_AAAA phpv.ZInt = 134217728; DNS_ANY phpv.ZInt = 268435456; DNS_ALL phpv.ZInt = 251721779
)
