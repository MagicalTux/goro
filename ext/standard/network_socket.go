package standard

import (
	"fmt"
	"net"
	"strings"
	"time"

	"github.com/MagicalTux/goro/core"
	"github.com/MagicalTux/goro/core/phpv"
	"github.com/MagicalTux/goro/core/stream"
)

// parseStreamSocketAddr parses "tcp://host:port", "unix:///path", etc. into (network, address).
func parseStreamSocketAddr(address string) (network, addr string) {
	if strings.HasPrefix(address, "tcp://") {
		return "tcp", address[6:]
	}
	if strings.HasPrefix(address, "tcp6://") {
		return "tcp6", address[7:]
	}
	if strings.HasPrefix(address, "udp://") {
		return "udp", address[6:]
	}
	if strings.HasPrefix(address, "udp6://") {
		return "udp6", address[7:]
	}
	if strings.HasPrefix(address, "unix://") {
		return "unix", address[7:]
	}
	if strings.HasPrefix(address, "unixgram://") {
		return "unixgram", address[11:]
	}
	// No scheme: treat as tcp host:port
	return "tcp", address
}

// streamFromNetConn wraps a net.Conn in a stream.Stream with appropriate metadata.
func streamFromNetConn(ctx phpv.Context, conn net.Conn, remoteAddr string) *stream.Stream {
	s := stream.NewStream(conn)
	s.ResourceType = phpv.ResourceStream
	s.ResourceID = ctx.Global().NextResourceID()
	s.SetAttr("wrapper_type", "tcp_socket")
	s.SetAttr("stream_type", "tcp_socket/ssl")
	s.SetAttr("mode", "r+")
	s.SetAttr("seekable", false)
	s.SetAttr("timed_out", false)
	s.SetAttr("blocked", true)
	s.SetAttr("uri", remoteAddr)
	return s
}

// streamServerListener wraps a net.Listener as a stream resource.
type streamServerListener struct {
	listener    net.Listener
	ResourceID  int
	ResourceTyp phpv.ResourceType
	addr        string
}

func (l *streamServerListener) GetType() phpv.ZType          { return phpv.ZtResource }
func (l *streamServerListener) ZVal() *phpv.ZVal             { return phpv.NewZVal(l) }
func (l *streamServerListener) Value() phpv.Val              { return l }
func (l *streamServerListener) String() string               { return fmt.Sprintf("resource(%d) of type (stream)", l.ResourceID) }
func (l *streamServerListener) GetResourceType() phpv.ResourceType { return l.ResourceTyp }
func (l *streamServerListener) GetResourceID() int           { return l.ResourceID }
func (l *streamServerListener) AsVal(ctx phpv.Context, t phpv.ZType) (phpv.Val, error) {
	switch t {
	case phpv.ZtString:
		return phpv.ZString(l.String()), nil
	}
	return nil, fmt.Errorf("cannot convert stream listener to %s", t)
}

// > func resource|false stream_socket_server(string $address, int &$error_code = null, string &$error_message = null, int $flags = STREAM_SERVER_BIND|STREAM_SERVER_LISTEN, resource $context = null)
func fncStreamSocketServer(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var address phpv.ZString
	_, err := core.Expand(ctx, args, &address)
	if err != nil {
		return nil, err
	}

	network, addr := parseStreamSocketAddr(string(address))

	var ln net.Listener
	var listenErr error

	if strings.HasPrefix(network, "udp") {
		// For UDP, create a packet conn
		pc, err := net.ListenPacket(network, addr)
		if err != nil {
			listenErr = err
		} else {
			// Wrap UDPConn as a stream
			if udpConn, ok := pc.(*net.UDPConn); ok {
				s := streamFromNetConn(ctx, udpConn, string(address))
				// Set error args to 0/"" if provided
				setStreamErrArgs(ctx, args, 0, "")
				return s.ZVal(), nil
			}
		}
	} else {
		ln, listenErr = net.Listen(network, addr)
	}

	if listenErr != nil {
		errCode := 1
		errMsg := listenErr.Error()
		setStreamErrArgs(ctx, args, errCode, errMsg)
		return phpv.ZFalse.ZVal(), ctx.Warn("stream_socket_server(): %s", errMsg)
	}

	setStreamErrArgs(ctx, args, 0, "")

	sl := &streamServerListener{
		listener:    ln,
		ResourceID:  ctx.Global().NextResourceID(),
		ResourceTyp: phpv.ResourceStream,
		addr:        string(address),
	}
	return sl.ZVal(), nil
}

// setStreamErrArgs sets the error_code and error_message reference arguments (indices 1 and 2)
func setStreamErrArgs(ctx phpv.Context, args []*phpv.ZVal, code int, msg string) {
	if len(args) > 1 && args[1] != nil && args[1].GetName() != "" {
		name := args[1].GetName()
		v := phpv.ZInt(code).ZVal()
		v.Name = &name
		ctx.Parent(1).OffsetSet(ctx, name, v)
	}
	if len(args) > 2 && args[2] != nil && args[2].GetName() != "" {
		name := args[2].GetName()
		v := phpv.ZString(msg).ZVal()
		v.Name = &name
		ctx.Parent(1).OffsetSet(ctx, name, v)
	}
}

// > func resource|false stream_socket_client(string $address, int &$error_code = null, string &$error_message = null, ?float $timeout = null, int $flags = STREAM_CLIENT_CONNECT, resource $context = null)
func fncStreamSocketClient(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var address phpv.ZString
	_, err := core.Expand(ctx, args, &address)
	if err != nil {
		return nil, err
	}

	network, addr := parseStreamSocketAddr(string(address))

	timeout := 30 * time.Second
	if len(args) > 3 && args[3] != nil && args[3].GetType() != phpv.ZtNull {
		t, _ := args[3].As(ctx, phpv.ZtFloat)
		if t != nil {
			secs := float64(t.Value().(phpv.ZFloat))
			timeout = time.Duration(secs * float64(time.Second))
		}
	}

	conn, dialErr := net.DialTimeout(network, addr, timeout)
	if dialErr != nil {
		errCode := 1
		errMsg := dialErr.Error()
		setStreamErrArgs(ctx, args, errCode, errMsg)
		return phpv.ZFalse.ZVal(), ctx.Warn("stream_socket_client(): %s", errMsg)
	}

	setStreamErrArgs(ctx, args, 0, "")
	s := streamFromNetConn(ctx, conn, string(address))
	return s.ZVal(), nil
}

// > func resource|false stream_socket_accept(resource $server, ?float $timeout = null, string &$peer_name = null)
func fncStreamSocketAccept(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var serverRes phpv.Resource
	_, err := core.Expand(ctx, args, &serverRes)
	if err != nil {
		return nil, err
	}

	sl, ok := serverRes.(*streamServerListener)
	if !ok {
		return phpv.ZFalse.ZVal(), ctx.Warn("stream_socket_accept(): Not a valid server socket")
	}

	// Handle optional timeout
	if len(args) > 1 && args[1] != nil && args[1].GetType() != phpv.ZtNull {
		t, _ := args[1].As(ctx, phpv.ZtFloat)
		if t != nil {
			secs := float64(t.Value().(phpv.ZFloat))
			timeout := time.Duration(secs * float64(time.Second))
			sl.listener.(*net.TCPListener).SetDeadline(time.Now().Add(timeout))
			defer sl.listener.(*net.TCPListener).SetDeadline(time.Time{})
		}
	}

	conn, acceptErr := sl.listener.Accept()
	if acceptErr != nil {
		return phpv.ZFalse.ZVal(), nil
	}

	peerAddr := conn.RemoteAddr().String()

	// Set &$peer_name (arg index 2)
	if len(args) > 2 && args[2] != nil {
		name := args[2].GetName()
		v := phpv.ZString(peerAddr).ZVal()
		v.Name = &name
		ctx.Parent(1).OffsetSet(ctx, name, v)
	}

	s := streamFromNetConn(ctx, conn, peerAddr)
	return s.ZVal(), nil
}

// > func array|false stream_socket_pair(int $domain, int $type, int $protocol)
func fncStreamSocketPair(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	// Create a connected pair using a local listener
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return phpv.ZFalse.ZVal(), ctx.Warn("stream_socket_pair(): %s", err.Error())
	}
	defer ln.Close()

	addr := ln.Addr().String()

	// Dial the listener
	conn1, err := net.Dial("tcp", addr)
	if err != nil {
		return phpv.ZFalse.ZVal(), ctx.Warn("stream_socket_pair(): %s", err.Error())
	}

	conn2, err := ln.Accept()
	if err != nil {
		conn1.Close()
		return phpv.ZFalse.ZVal(), ctx.Warn("stream_socket_pair(): %s", err.Error())
	}

	s1 := streamFromNetConn(ctx, conn1, addr)
	s2 := streamFromNetConn(ctx, conn2, addr)

	result := phpv.NewZArray()
	result.OffsetSet(ctx, phpv.ZInt(0).ZVal(), s1.ZVal())
	result.OffsetSet(ctx, phpv.ZInt(1).ZVal(), s2.ZVal())
	return result.ZVal(), nil
}

// > func bool stream_socket_shutdown(resource $stream, int $mode)
func fncStreamSocketShutdown(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var streamRes phpv.Resource
	var mode phpv.ZInt
	_, err := core.Expand(ctx, args, &streamRes, &mode)
	if err != nil {
		return nil, err
	}

	s, ok := streamRes.(*stream.Stream)
	if !ok {
		return phpv.ZFalse.ZVal(), nil
	}

	conn, ok := s.Underlying().(net.Conn)
	if !ok {
		return phpv.ZFalse.ZVal(), nil
	}

	type halfCloser interface {
		CloseRead() error
		CloseWrite() error
	}

	switch int(mode) {
	case 0: // STREAM_SHUT_RD
		if hc, ok := conn.(halfCloser); ok {
			hc.CloseRead()
		}
	case 1: // STREAM_SHUT_WR
		if hc, ok := conn.(halfCloser); ok {
			hc.CloseWrite()
		}
	default: // STREAM_SHUT_RDWR
		conn.Close()
	}

	return phpv.ZTrue.ZVal(), nil
}

// > func string|false stream_socket_get_name(resource $stream, bool $remote)
func fncStreamSocketGetName(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var streamRes phpv.Resource
	var remote phpv.ZBool
	_, err := core.Expand(ctx, args, &streamRes, &remote)
	if err != nil {
		return nil, err
	}

	s, ok := streamRes.(*stream.Stream)
	if !ok {
		// Check if it's a server listener
		if sl, ok := streamRes.(*streamServerListener); ok {
			if !bool(remote) {
				return phpv.ZString(sl.listener.Addr().String()).ZVal(), nil
			}
		}
		return phpv.ZFalse.ZVal(), nil
	}

	conn, ok := s.Underlying().(net.Conn)
	if !ok {
		return phpv.ZFalse.ZVal(), nil
	}

	var addr net.Addr
	if bool(remote) {
		addr = conn.RemoteAddr()
	} else {
		addr = conn.LocalAddr()
	}

	if addr == nil {
		return phpv.ZFalse.ZVal(), nil
	}

	return phpv.ZString(addr.String()).ZVal(), nil
}

// Ensure unused imports are referenced
var _ = fmt.Sprintf
var _ = strings.HasPrefix
var _ = time.Second
