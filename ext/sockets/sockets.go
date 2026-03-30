package sockets

import (
	"fmt"
	"net"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/MagicalTux/goro/core"
	"github.com/MagicalTux/goro/core/phpobj"
	"github.com/MagicalTux/goro/core/phpv"
)

// SocketClass is the PHP 8 Socket class.
var SocketClass = &phpobj.ZClass{
	Name:         "Socket",
	InternalOnly: true,
}

// socketData holds the underlying Go socket state.
type socketData struct {
	domain   int
	sockType int
	protocol int
	conn     net.Conn
	listener net.Listener
	lastErr  int
	blocking bool
}

func getSocketData(obj *phpobj.ZObject) *socketData {
	opaque := obj.GetOpaque(SocketClass)
	if opaque == nil {
		return nil
	}
	return opaque.(*socketData)
}

func socketDomainNetwork(domain, sockType int) string {
	switch domain {
	case AF_UNIX:
		if sockType == SOCK_DGRAM {
			return "unixgram"
		}
		return "unix"
	case AF_INET6:
		if sockType == SOCK_DGRAM {
			return "udp6"
		}
		return "tcp6"
	default: // AF_INET
		if sockType == SOCK_DGRAM {
			return "udp"
		}
		return "tcp"
	}
}

func errnoToInt(err error) int {
	if err == nil {
		return 0
	}
	if errno, ok := err.(syscall.Errno); ok {
		return int(errno)
	}
	return 111 // ECONNREFUSED as a fallback
}

// > func Socket socket_create(int $domain, int $type, int $protocol)
func fncSocketCreate(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var domain phpv.ZInt
	var sockType phpv.ZInt
	var protocol phpv.ZInt
	_, err := core.Expand(ctx, args, &domain, &sockType, &protocol)
	if err != nil {
		return nil, err
	}

	sd := &socketData{
		domain:   int(domain),
		sockType: int(sockType),
		protocol: int(protocol),
		blocking: true,
	}

	obj, err := phpobj.NewZObjectOpaque(ctx, SocketClass, sd)
	if err != nil {
		return nil, err
	}
	return obj.ZVal(), nil
}

// > func bool socket_bind(Socket $socket, string $address, int $port = 0)
func fncSocketBind(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var socketVal *phpv.ZVal
	var address phpv.ZString
	var port core.Optional[phpv.ZInt]
	_, err := core.Expand(ctx, args, &socketVal, &address, &port)
	if err != nil {
		return nil, err
	}

	if socketVal == nil || socketVal.GetType() != phpv.ZtObject {
		return phpv.ZFalse.ZVal(), ctx.Warn("socket_bind(): Argument #1 must be of type Socket")
	}
	obj := socketVal.Value().(*phpobj.ZObject)
	sd := getSocketData(obj)
	if sd == nil {
		return phpv.ZFalse.ZVal(), ctx.Warn("socket_bind(): Invalid socket")
	}

	portNum := 0
	if port.HasArg() {
		portNum = int(port.Get())
	}

	var addr string
	if sd.domain == AF_UNIX {
		addr = string(address)
	} else {
		addr = fmt.Sprintf("%s:%d", string(address), portNum)
	}

	network := socketDomainNetwork(sd.domain, sd.sockType)
	lc := &net.ListenConfig{}

	if sd.sockType == SOCK_DGRAM {
		pc, err := lc.ListenPacket(ctx, network, addr)
		if err != nil {
			sd.lastErr = errnoToInt(err)
			return phpv.ZFalse.ZVal(), ctx.Warn("socket_bind(): %s", err.Error())
		}
		if udpConn, ok := pc.(*net.UDPConn); ok {
			sd.conn = udpConn
		}
	} else {
		ln, err := lc.Listen(ctx, network, addr)
		if err != nil {
			sd.lastErr = errnoToInt(err)
			return phpv.ZFalse.ZVal(), ctx.Warn("socket_bind(): %s", err.Error())
		}
		sd.listener = ln
	}

	return phpv.ZTrue.ZVal(), nil
}

// > func bool socket_listen(Socket $socket, int $backlog = 128)
func fncSocketListen(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var socketVal *phpv.ZVal
	var backlog core.Optional[phpv.ZInt]
	_, err := core.Expand(ctx, args, &socketVal, &backlog)
	if err != nil {
		return nil, err
	}

	if socketVal == nil || socketVal.GetType() != phpv.ZtObject {
		return phpv.ZFalse.ZVal(), ctx.Warn("socket_listen(): Argument #1 must be of type Socket")
	}
	obj := socketVal.Value().(*phpobj.ZObject)
	sd := getSocketData(obj)
	if sd == nil {
		return phpv.ZFalse.ZVal(), ctx.Warn("socket_listen(): Invalid socket")
	}

	if sd.listener == nil {
		return phpv.ZFalse.ZVal(), ctx.Warn("socket_listen(): Socket not bound")
	}

	// In Go's net package, listen is done at socket creation time, so this is a no-op
	return phpv.ZTrue.ZVal(), nil
}

// > func Socket|false socket_accept(Socket $socket)
func fncSocketAccept(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var socketVal *phpv.ZVal
	_, err := core.Expand(ctx, args, &socketVal)
	if err != nil {
		return nil, err
	}

	if socketVal == nil || socketVal.GetType() != phpv.ZtObject {
		return phpv.ZFalse.ZVal(), ctx.Warn("socket_accept(): Argument #1 must be of type Socket")
	}
	obj := socketVal.Value().(*phpobj.ZObject)
	sd := getSocketData(obj)
	if sd == nil || sd.listener == nil {
		return phpv.ZFalse.ZVal(), ctx.Warn("socket_accept(): Socket not listening")
	}

	conn, err := sd.listener.Accept()
	if err != nil {
		sd.lastErr = errnoToInt(err)
		return phpv.ZFalse.ZVal(), nil
	}

	newSd := &socketData{
		domain:   sd.domain,
		sockType: sd.sockType,
		protocol: sd.protocol,
		conn:     conn,
		blocking: true,
	}

	newObj, err := phpobj.NewZObjectOpaque(ctx, SocketClass, newSd)
	if err != nil {
		conn.Close()
		return nil, err
	}
	return newObj.ZVal(), nil
}

// > func bool socket_connect(Socket $socket, string $address, ?int $port = null)
func fncSocketConnect(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var socketVal *phpv.ZVal
	var address phpv.ZString
	var port core.Optional[phpv.ZInt]
	_, err := core.Expand(ctx, args, &socketVal, &address, &port)
	if err != nil {
		return nil, err
	}

	if socketVal == nil || socketVal.GetType() != phpv.ZtObject {
		return phpv.ZFalse.ZVal(), ctx.Warn("socket_connect(): Argument #1 must be of type Socket")
	}
	obj := socketVal.Value().(*phpobj.ZObject)
	sd := getSocketData(obj)
	if sd == nil {
		return phpv.ZFalse.ZVal(), ctx.Warn("socket_connect(): Invalid socket")
	}

	var addr string
	if sd.domain == AF_UNIX {
		addr = string(address)
	} else {
		portNum := 0
		if port.HasArg() {
			portNum = int(port.Get())
		}
		addr = fmt.Sprintf("%s:%d", string(address), portNum)
	}

	network := socketDomainNetwork(sd.domain, sd.sockType)
	conn, err := net.Dial(network, addr)
	if err != nil {
		sd.lastErr = errnoToInt(err)
		return phpv.ZFalse.ZVal(), ctx.Warn("socket_connect(): %s", err.Error())
	}
	sd.conn = conn
	return phpv.ZTrue.ZVal(), nil
}

// > func string|false socket_read(Socket $socket, int $length, int $mode = PHP_BINARY_READ)
func fncSocketRead(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var socketVal *phpv.ZVal
	var length phpv.ZInt
	var mode core.Optional[phpv.ZInt]
	_, err := core.Expand(ctx, args, &socketVal, &length, &mode)
	if err != nil {
		return nil, err
	}

	if socketVal == nil || socketVal.GetType() != phpv.ZtObject {
		return phpv.ZFalse.ZVal(), ctx.Warn("socket_read(): Argument #1 must be of type Socket")
	}
	obj := socketVal.Value().(*phpobj.ZObject)
	sd := getSocketData(obj)
	if sd == nil || sd.conn == nil {
		return phpv.ZFalse.ZVal(), ctx.Warn("socket_read(): Socket not connected")
	}

	readMode := PHP_BINARY_READ
	if mode.HasArg() {
		readMode = int(mode.Get())
	}

	buf := make([]byte, int(length))

	if readMode == PHP_NORMAL_READ {
		// Read until newline or EOF
		var result []byte
		oneByte := make([]byte, 1)
		for i := 0; i < int(length); i++ {
			n, err := sd.conn.Read(oneByte)
			if n > 0 {
				result = append(result, oneByte[0])
				if oneByte[0] == '\n' {
					break
				}
			}
			if err != nil {
				if len(result) == 0 {
					sd.lastErr = errnoToInt(err)
					return phpv.ZFalse.ZVal(), nil
				}
				break
			}
		}
		return phpv.ZString(result).ZVal(), nil
	}

	// PHP_BINARY_READ
	n, err := sd.conn.Read(buf)
	if err != nil && n == 0 {
		sd.lastErr = errnoToInt(err)
		return phpv.ZFalse.ZVal(), nil
	}
	return phpv.ZString(buf[:n]).ZVal(), nil
}

// > func int|false socket_write(Socket $socket, string $data, ?int $length = null)
func fncSocketWrite(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var socketVal *phpv.ZVal
	var data phpv.ZString
	var length core.Optional[phpv.ZInt]
	_, err := core.Expand(ctx, args, &socketVal, &data, &length)
	if err != nil {
		return nil, err
	}

	if socketVal == nil || socketVal.GetType() != phpv.ZtObject {
		return phpv.ZFalse.ZVal(), ctx.Warn("socket_write(): Argument #1 must be of type Socket")
	}
	obj := socketVal.Value().(*phpobj.ZObject)
	sd := getSocketData(obj)
	if sd == nil || sd.conn == nil {
		return phpv.ZFalse.ZVal(), ctx.Warn("socket_write(): Socket not connected")
	}

	writeData := []byte(data)
	if length.HasArg() && int(length.Get()) < len(writeData) {
		writeData = writeData[:int(length.Get())]
	}

	n, err := sd.conn.Write(writeData)
	if err != nil {
		sd.lastErr = errnoToInt(err)
		return phpv.ZFalse.ZVal(), nil
	}
	return phpv.ZInt(n).ZVal(), nil
}

// > func bool socket_close(Socket $socket)
func fncSocketClose(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var socketVal *phpv.ZVal
	_, err := core.Expand(ctx, args, &socketVal)
	if err != nil {
		return nil, err
	}

	if socketVal == nil || socketVal.GetType() != phpv.ZtObject {
		return nil, ctx.Warn("socket_close(): Argument #1 must be of type Socket")
	}
	obj := socketVal.Value().(*phpobj.ZObject)
	sd := getSocketData(obj)
	if sd == nil {
		return nil, ctx.Warn("socket_close(): Invalid socket")
	}

	if sd.conn != nil {
		sd.conn.Close()
		sd.conn = nil
	}
	if sd.listener != nil {
		sd.listener.Close()
		sd.listener = nil
	}
	return nil, nil
}

// > func int socket_last_error(?Socket $socket = null)
func fncSocketLastError(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var socketVal core.Optional[*phpv.ZVal]
	_, err := core.Expand(ctx, args, &socketVal)
	if err != nil {
		return nil, err
	}

	if socketVal.HasArg() && socketVal.Get() != nil && socketVal.Get().GetType() == phpv.ZtObject {
		obj := socketVal.Get().Value().(*phpobj.ZObject)
		sd := getSocketData(obj)
		if sd != nil {
			return phpv.ZInt(sd.lastErr).ZVal(), nil
		}
	}

	return phpv.ZInt(0).ZVal(), nil
}

// > func string socket_strerror(int $error_code)
func fncSocketStrerror(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var errorCode phpv.ZInt
	_, err := core.Expand(ctx, args, &errorCode)
	if err != nil {
		return nil, err
	}

	errno := syscall.Errno(errorCode)
	return phpv.ZString(errno.Error()).ZVal(), nil
}

// > func bool socket_clear_error(?Socket $socket = null)
func fncSocketClearError(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var socketVal core.Optional[*phpv.ZVal]
	_, err := core.Expand(ctx, args, &socketVal)
	if err != nil {
		return nil, err
	}

	if socketVal.HasArg() && socketVal.Get() != nil && socketVal.Get().GetType() == phpv.ZtObject {
		obj := socketVal.Get().Value().(*phpobj.ZObject)
		sd := getSocketData(obj)
		if sd != nil {
			sd.lastErr = 0
		}
	}
	return phpv.ZTrue.ZVal(), nil
}

// > func bool socket_set_option(Socket $socket, int $level, int $optname, int|string|array $optval)
func fncSocketSetOption(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	// Stub: Go handles socket options differently, but we accept the call
	if len(args) < 4 {
		return phpv.ZFalse.ZVal(), nil
	}
	return phpv.ZTrue.ZVal(), nil
}

// > func mixed socket_get_option(Socket $socket, int $level, int $optname)
func fncSocketGetOption(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var socketVal *phpv.ZVal
	var level phpv.ZInt
	var optname phpv.ZInt
	_, err := core.Expand(ctx, args, &socketVal, &level, &optname)
	if err != nil {
		return nil, err
	}

	// Return stub values for common options
	switch int(optname) {
	case SO_REUSEADDR:
		return phpv.ZInt(1).ZVal(), nil
	case SO_KEEPALIVE:
		return phpv.ZInt(0).ZVal(), nil
	case SO_RCVTIMEO, SO_SNDTIMEO:
		result := phpv.NewZArray()
		result.OffsetSet(ctx, phpv.ZStr("sec"), phpv.ZInt(0).ZVal())
		result.OffsetSet(ctx, phpv.ZStr("usec"), phpv.ZInt(0).ZVal())
		return result.ZVal(), nil
	}
	return phpv.ZFalse.ZVal(), nil
}

// > func bool socket_set_nonblock(Socket $socket)
func fncSocketSetNonblock(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var socketVal *phpv.ZVal
	_, err := core.Expand(ctx, args, &socketVal)
	if err != nil {
		return nil, err
	}

	if socketVal == nil || socketVal.GetType() != phpv.ZtObject {
		return phpv.ZFalse.ZVal(), nil
	}
	obj := socketVal.Value().(*phpobj.ZObject)
	sd := getSocketData(obj)
	if sd == nil {
		return phpv.ZFalse.ZVal(), nil
	}
	sd.blocking = false
	return phpv.ZTrue.ZVal(), nil
}

// > func bool socket_set_block(Socket $socket)
func fncSocketSetBlock(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var socketVal *phpv.ZVal
	_, err := core.Expand(ctx, args, &socketVal)
	if err != nil {
		return nil, err
	}

	if socketVal == nil || socketVal.GetType() != phpv.ZtObject {
		return phpv.ZFalse.ZVal(), nil
	}
	obj := socketVal.Value().(*phpobj.ZObject)
	sd := getSocketData(obj)
	if sd == nil {
		return phpv.ZFalse.ZVal(), nil
	}
	sd.blocking = true
	return phpv.ZTrue.ZVal(), nil
}

// > func bool socket_getpeername(Socket $socket, string &$address, ?int &$port = null)
func fncSocketGetpeername(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var socketVal *phpv.ZVal
	_, err := core.Expand(ctx, args, &socketVal)
	if err != nil {
		return nil, err
	}

	if socketVal == nil || socketVal.GetType() != phpv.ZtObject {
		return phpv.ZFalse.ZVal(), nil
	}
	obj := socketVal.Value().(*phpobj.ZObject)
	sd := getSocketData(obj)
	if sd == nil || sd.conn == nil {
		return phpv.ZFalse.ZVal(), nil
	}

	remoteAddr := sd.conn.RemoteAddr()
	if remoteAddr == nil {
		return phpv.ZFalse.ZVal(), nil
	}

	addrStr := remoteAddr.String()
	host, portStr, parseErr := net.SplitHostPort(addrStr)
	if parseErr != nil {
		host = addrStr
		portStr = "0"
	}

	// Set &$address (arg index 1)
	if len(args) > 1 && args[1] != nil {
		name := args[1].GetName()
		addrVal := phpv.ZString(host).ZVal()
		addrVal.Name = &name
		ctx.Parent(1).OffsetSet(ctx, name, addrVal)
	}

	// Set &$port (arg index 2) if provided
	if len(args) > 2 && args[2] != nil {
		portNum, _ := strconv.Atoi(portStr)
		name := args[2].GetName()
		portVal := phpv.ZInt(portNum).ZVal()
		portVal.Name = &name
		ctx.Parent(1).OffsetSet(ctx, name, portVal)
	}

	return phpv.ZTrue.ZVal(), nil
}

// > func bool socket_getsockname(Socket $socket, string &$address, ?int &$port = null)
func fncSocketGetsockname(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var socketVal *phpv.ZVal
	_, err := core.Expand(ctx, args, &socketVal)
	if err != nil {
		return nil, err
	}

	if socketVal == nil || socketVal.GetType() != phpv.ZtObject {
		return phpv.ZFalse.ZVal(), nil
	}
	obj := socketVal.Value().(*phpobj.ZObject)
	sd := getSocketData(obj)

	var localAddr net.Addr
	if sd.conn != nil {
		localAddr = sd.conn.LocalAddr()
	} else if sd.listener != nil {
		localAddr = sd.listener.Addr()
	} else {
		return phpv.ZFalse.ZVal(), nil
	}

	addrStr := localAddr.String()
	host, portStr, parseErr := net.SplitHostPort(addrStr)
	if parseErr != nil {
		host = addrStr
		portStr = "0"
	}

	// Set &$address (arg index 1)
	if len(args) > 1 && args[1] != nil {
		name := args[1].GetName()
		addrVal := phpv.ZString(host).ZVal()
		addrVal.Name = &name
		ctx.Parent(1).OffsetSet(ctx, name, addrVal)
	}

	// Set &$port (arg index 2) if provided
	if len(args) > 2 && args[2] != nil {
		portNum, _ := strconv.Atoi(portStr)
		name := args[2].GetName()
		portVal := phpv.ZInt(portNum).ZVal()
		portVal.Name = &name
		ctx.Parent(1).OffsetSet(ctx, name, portVal)
	}

	return phpv.ZTrue.ZVal(), nil
}

// > func bool socket_shutdown(Socket $socket, int $mode = 2)
func fncSocketShutdown(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var socketVal *phpv.ZVal
	var mode core.Optional[phpv.ZInt]
	_, err := core.Expand(ctx, args, &socketVal, &mode)
	if err != nil {
		return nil, err
	}

	if socketVal == nil || socketVal.GetType() != phpv.ZtObject {
		return phpv.ZFalse.ZVal(), nil
	}
	obj := socketVal.Value().(*phpobj.ZObject)
	sd := getSocketData(obj)
	if sd == nil {
		return phpv.ZFalse.ZVal(), nil
	}

	shutMode := 2
	if mode.HasArg() {
		shutMode = int(mode.Get())
	}

	if sd.conn != nil {
		// Go doesn't expose SHUT_RD/SHUT_WR directly on net.Conn, but we can use type assertions
		type halfCloser interface {
			CloseRead() error
			CloseWrite() error
		}
		if hc, ok := sd.conn.(halfCloser); ok {
			switch shutMode {
			case 0:
				hc.CloseRead()
			case 1:
				hc.CloseWrite()
			default:
				sd.conn.Close()
				sd.conn = nil
			}
		} else {
			if shutMode == 2 {
				sd.conn.Close()
				sd.conn = nil
			}
		}
	}

	return phpv.ZTrue.ZVal(), nil
}

// > func int|false socket_select(array &$read, array &$write, array &$except, ?int $seconds, int $microseconds = 0)
func fncSocketSelect(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	if len(args) < 4 {
		return phpv.ZFalse.ZVal(), nil
	}

	// Get timeout
	seconds := -1
	microseconds := 0

	if args[3] != nil && args[3].GetType() != phpv.ZtNull {
		seconds = int(args[3].AsInt(ctx))
	}
	if len(args) > 4 && args[4] != nil {
		microseconds = int(args[4].AsInt(ctx))
	}

	var timeout time.Duration
	if seconds < 0 {
		timeout = -1 // indefinite
	} else {
		timeout = time.Duration(seconds)*time.Second + time.Duration(microseconds)*time.Microsecond
	}

	// Collect read sockets
	readArr := []*socketData{}
	if args[0] != nil && args[0].GetType() == phpv.ZtArray {
		for _, v := range args[0].AsArray(ctx).Iterate(ctx) {
			if v.GetType() == phpv.ZtObject {
				if obj, ok := v.Value().(*phpobj.ZObject); ok {
					if sd := getSocketData(obj); sd != nil {
						readArr = append(readArr, sd)
					}
				}
			}
		}
	}

	if len(readArr) == 0 {
		return phpv.ZInt(0).ZVal(), nil
	}

	ready := 0
	deadline := time.Now().Add(timeout)

	for _, sd := range readArr {
		if sd.conn != nil {
			if timeout >= 0 {
				sd.conn.SetReadDeadline(deadline)
			}
			buf := make([]byte, 1)
			sd.conn.SetReadDeadline(time.Now().Add(1 * time.Millisecond))
			n, err := sd.conn.Read(buf)
			if n > 0 || err == nil {
				ready++
			}
			// Reset deadline
			sd.conn.SetReadDeadline(time.Time{})
		} else if sd.listener != nil {
			ready++ // simplistic: assume listener is always ready
		}
	}

	return phpv.ZInt(ready).ZVal(), nil
}

// > func int|false socket_send(Socket $socket, string $data, int $length, int $flags)
func fncSocketSend(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var socketVal *phpv.ZVal
	var data phpv.ZString
	var length phpv.ZInt
	var flags phpv.ZInt
	_, err := core.Expand(ctx, args, &socketVal, &data, &length, &flags)
	if err != nil {
		return nil, err
	}

	if socketVal == nil || socketVal.GetType() != phpv.ZtObject {
		return phpv.ZFalse.ZVal(), nil
	}
	obj := socketVal.Value().(*phpobj.ZObject)
	sd := getSocketData(obj)
	if sd == nil || sd.conn == nil {
		return phpv.ZFalse.ZVal(), nil
	}

	writeData := []byte(data)
	if int(length) < len(writeData) {
		writeData = writeData[:int(length)]
	}

	n, err := sd.conn.Write(writeData)
	if err != nil {
		sd.lastErr = errnoToInt(err)
		return phpv.ZFalse.ZVal(), nil
	}
	return phpv.ZInt(n).ZVal(), nil
}

// > func int|false socket_recv(Socket $socket, string &$data, int $length, int $flags)
func fncSocketRecv(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var socketVal *phpv.ZVal
	var length phpv.ZInt
	var flags phpv.ZInt
	_, err := core.Expand(ctx, args, &socketVal, nil, &length, &flags)
	if err != nil {
		return nil, err
	}

	if socketVal == nil || socketVal.GetType() != phpv.ZtObject {
		return phpv.ZFalse.ZVal(), nil
	}
	obj := socketVal.Value().(*phpobj.ZObject)
	sd := getSocketData(obj)
	if sd == nil || sd.conn == nil {
		return phpv.ZFalse.ZVal(), nil
	}

	buf := make([]byte, int(length))
	n, readErr := sd.conn.Read(buf)
	if readErr != nil && n == 0 {
		sd.lastErr = errnoToInt(readErr)
		return phpv.ZFalse.ZVal(), nil
	}

	// Set &$data (arg index 1)
	if len(args) > 1 && args[1] != nil {
		name := args[1].GetName()
		dataVal := phpv.ZString(buf[:n]).ZVal()
		dataVal.Name = &name
		ctx.Parent(1).OffsetSet(ctx, name, dataVal)
	}

	return phpv.ZInt(n).ZVal(), nil
}

// > func int|false socket_sendto(Socket $socket, string $data, int $length, int $flags, string $address, int $port = 0)
func fncSocketSendto(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var socketVal *phpv.ZVal
	var data phpv.ZString
	var length phpv.ZInt
	var flags phpv.ZInt
	var address phpv.ZString
	var port core.Optional[phpv.ZInt]
	_, err := core.Expand(ctx, args, &socketVal, &data, &length, &flags, &address, &port)
	if err != nil {
		return nil, err
	}

	if socketVal == nil || socketVal.GetType() != phpv.ZtObject {
		return phpv.ZFalse.ZVal(), nil
	}
	obj := socketVal.Value().(*phpobj.ZObject)
	sd := getSocketData(obj)
	if sd == nil {
		return phpv.ZFalse.ZVal(), nil
	}

	portNum := 0
	if port.HasArg() {
		portNum = int(port.Get())
	}

	addr := fmt.Sprintf("%s:%d", string(address), portNum)
	writeData := []byte(data)
	if int(length) < len(writeData) {
		writeData = writeData[:int(length)]
	}

	if sd.conn != nil {
		// UDP conn already established
		if udpConn, ok := sd.conn.(*net.UDPConn); ok {
			udpAddr, err := net.ResolveUDPAddr("udp", addr)
			if err != nil {
				return phpv.ZFalse.ZVal(), nil
			}
			n, err := udpConn.WriteTo(writeData, udpAddr)
			if err != nil {
				return phpv.ZFalse.ZVal(), nil
			}
			return phpv.ZInt(n).ZVal(), nil
		}
		n, err := sd.conn.Write(writeData)
		if err != nil {
			return phpv.ZFalse.ZVal(), nil
		}
		return phpv.ZInt(n).ZVal(), nil
	}

	// No connection yet, dial first
	network := socketDomainNetwork(sd.domain, sd.sockType)
	conn, err := net.Dial(network, addr)
	if err != nil {
		sd.lastErr = errnoToInt(err)
		return phpv.ZFalse.ZVal(), nil
	}
	n, err := conn.Write(writeData)
	conn.Close()
	if err != nil {
		return phpv.ZFalse.ZVal(), nil
	}
	return phpv.ZInt(n).ZVal(), nil
}

// > func int|false socket_recvfrom(Socket $socket, string &$buf, int $length, int $flags, string &$name, int &$port = null)
func fncSocketRecvfrom(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var socketVal *phpv.ZVal
	var length phpv.ZInt
	var flags phpv.ZInt
	_, err := core.Expand(ctx, args, &socketVal, nil, &length, &flags)
	if err != nil {
		return nil, err
	}

	if socketVal == nil || socketVal.GetType() != phpv.ZtObject {
		return phpv.ZFalse.ZVal(), nil
	}
	obj := socketVal.Value().(*phpobj.ZObject)
	sd := getSocketData(obj)
	if sd == nil || sd.conn == nil {
		return phpv.ZFalse.ZVal(), nil
	}

	buf := make([]byte, int(length))

	var n int
	var from net.Addr

	if udpConn, ok := sd.conn.(*net.UDPConn); ok {
		var readErr error
		n, from, readErr = udpConn.ReadFrom(buf)
		if readErr != nil && n == 0 {
			return phpv.ZFalse.ZVal(), nil
		}
	} else {
		var readErr error
		n, readErr = sd.conn.Read(buf)
		if readErr != nil && n == 0 {
			return phpv.ZFalse.ZVal(), nil
		}
		from = sd.conn.RemoteAddr()
	}

	// Set &$buf (arg index 1)
	if len(args) > 1 && args[1] != nil {
		name := args[1].GetName()
		dataVal := phpv.ZString(buf[:n]).ZVal()
		dataVal.Name = &name
		ctx.Parent(1).OffsetSet(ctx, name, dataVal)
	}

	// Set &$name (arg index 4)
	if len(args) > 4 && args[4] != nil && from != nil {
		addrStr := from.String()
		host, portStr, parseErr := net.SplitHostPort(addrStr)
		if parseErr != nil {
			host = addrStr
		}
		name := args[4].GetName()
		nameVal := phpv.ZString(host).ZVal()
		nameVal.Name = &name
		ctx.Parent(1).OffsetSet(ctx, name, nameVal)

		// Set &$port (arg index 5)
		if len(args) > 5 && args[5] != nil {
			portNum, _ := strconv.Atoi(portStr)
			pname := args[5].GetName()
			portVal := phpv.ZInt(portNum).ZVal()
			portVal.Name = &pname
			ctx.Parent(1).OffsetSet(ctx, pname, portVal)
		}
	}

	return phpv.ZInt(n).ZVal(), nil
}

// parseStreamAddress parses "tcp://host:port" or "unix:///path" or "host:port"
func parseStreamAddress(address string) (network, addr string, err error) {
	if strings.HasPrefix(address, "tcp://") {
		return "tcp", address[6:], nil
	}
	if strings.HasPrefix(address, "tcp6://") {
		return "tcp6", address[7:], nil
	}
	if strings.HasPrefix(address, "udp://") {
		return "udp", address[6:], nil
	}
	if strings.HasPrefix(address, "udp6://") {
		return "udp6", address[7:], nil
	}
	if strings.HasPrefix(address, "unix://") {
		return "unix", address[7:], nil
	}
	if strings.HasPrefix(address, "unixgram://") {
		return "unixgram", address[11:], nil
	}
	// Default: treat as tcp host:port
	return "tcp", address, nil
}

// formatStreamAddr formats a net.Addr for PHP stream functions
func formatStreamAddr(addr net.Addr) string {
	if addr == nil {
		return ""
	}
	network := addr.Network()
	s := addr.String()
	switch network {
	case "unix", "unixgram":
		return "unix://" + s
	case "tcp6", "udp6":
		return network + "://" + s
	default:
		return s
	}
}

// Unused import prevention for fmt/strings
var _ = fmt.Sprintf
var _ = strings.HasPrefix
