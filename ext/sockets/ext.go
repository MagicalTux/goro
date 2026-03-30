package sockets

import (
	"github.com/MagicalTux/goro/core"
	"github.com/MagicalTux/goro/core/phpctx"
	"github.com/MagicalTux/goro/core/phpobj"
	"github.com/MagicalTux/goro/core/phpv"
)

func init() {
	phpctx.RegisterExt(&phpctx.Ext{
		Name:    "sockets",
		Version: core.VERSION,
		Classes: []*phpobj.ZClass{
			SocketClass,
		},
		Functions: map[string]*phpctx.ExtFunction{
			"socket_create":      {Func: fncSocketCreate, Args: []*phpctx.ExtFunctionArg{}},
			"socket_bind":        {Func: fncSocketBind, Args: []*phpctx.ExtFunctionArg{}},
			"socket_listen":      {Func: fncSocketListen, Args: []*phpctx.ExtFunctionArg{}},
			"socket_accept":      {Func: fncSocketAccept, Args: []*phpctx.ExtFunctionArg{}},
			"socket_connect":     {Func: fncSocketConnect, Args: []*phpctx.ExtFunctionArg{}},
			"socket_read":        {Func: fncSocketRead, Args: []*phpctx.ExtFunctionArg{}},
			"socket_write":       {Func: fncSocketWrite, Args: []*phpctx.ExtFunctionArg{}},
			"socket_close":       {Func: fncSocketClose, Args: []*phpctx.ExtFunctionArg{}},
			"socket_last_error":  {Func: fncSocketLastError, Args: []*phpctx.ExtFunctionArg{}},
			"socket_strerror":    {Func: fncSocketStrerror, Args: []*phpctx.ExtFunctionArg{}},
			"socket_clear_error": {Func: fncSocketClearError, Args: []*phpctx.ExtFunctionArg{}},
			"socket_set_option":  {Func: fncSocketSetOption, Args: []*phpctx.ExtFunctionArg{}},
			"socket_get_option":  {Func: fncSocketGetOption, Args: []*phpctx.ExtFunctionArg{}},
			"socket_getopt":      {Func: fncSocketGetOption, Args: []*phpctx.ExtFunctionArg{}},
			"socket_setopt":      {Func: fncSocketSetOption, Args: []*phpctx.ExtFunctionArg{}},
			"socket_set_nonblock": {Func: fncSocketSetNonblock, Args: []*phpctx.ExtFunctionArg{}},
			"socket_set_block":   {Func: fncSocketSetBlock, Args: []*phpctx.ExtFunctionArg{}},
			"socket_getpeername": {Func: fncSocketGetpeername, Args: []*phpctx.ExtFunctionArg{}},
			"socket_getsockname": {Func: fncSocketGetsockname, Args: []*phpctx.ExtFunctionArg{}},
			"socket_shutdown":    {Func: fncSocketShutdown, Args: []*phpctx.ExtFunctionArg{}},
			"socket_select":      {Func: fncSocketSelect, Args: []*phpctx.ExtFunctionArg{}},
			"socket_send":        {Func: fncSocketSend, Args: []*phpctx.ExtFunctionArg{}},
			"socket_recv":        {Func: fncSocketRecv, Args: []*phpctx.ExtFunctionArg{}},
			"socket_sendto":      {Func: fncSocketSendto, Args: []*phpctx.ExtFunctionArg{}},
			"socket_recvfrom":    {Func: fncSocketRecvfrom, Args: []*phpctx.ExtFunctionArg{}},
		},
		Constants: map[phpv.ZString]phpv.Val{
			"AF_INET":           phpv.ZInt(AF_INET),
			"AF_INET6":          phpv.ZInt(AF_INET6),
			"AF_UNIX":           phpv.ZInt(AF_UNIX),
			"SOCK_STREAM":       phpv.ZInt(SOCK_STREAM),
			"SOCK_DGRAM":        phpv.ZInt(SOCK_DGRAM),
			"SOCK_RAW":          phpv.ZInt(SOCK_RAW),
			"SOL_SOCKET":        phpv.ZInt(SOL_SOCKET),
			"SOL_TCP":           phpv.ZInt(SOL_TCP),
			"SOL_UDP":           phpv.ZInt(SOL_UDP),
			"IPPROTO_IP":        phpv.ZInt(0),
			"IPPROTO_TCP":       phpv.ZInt(SOL_TCP),
			"IPPROTO_UDP":       phpv.ZInt(SOL_UDP),
			"SO_REUSEADDR":      phpv.ZInt(SO_REUSEADDR),
			"SO_REUSEPORT":      phpv.ZInt(SO_REUSEPORT),
			"SO_KEEPALIVE":      phpv.ZInt(SO_KEEPALIVE),
			"SO_RCVTIMEO":       phpv.ZInt(SO_RCVTIMEO),
			"SO_SNDTIMEO":       phpv.ZInt(SO_SNDTIMEO),
			"SO_SNDBUF":         phpv.ZInt(SO_SNDBUF),
			"SO_RCVBUF":         phpv.ZInt(SO_RCVBUF),
			"SO_LINGER":         phpv.ZInt(SO_LINGER),
			"SO_BROADCAST":      phpv.ZInt(SO_BROADCAST),
			"SOMAXCONN":         phpv.ZInt(SOMAXCONN),
			"PHP_BINARY_READ":   phpv.ZInt(PHP_BINARY_READ),
			"PHP_NORMAL_READ":   phpv.ZInt(PHP_NORMAL_READ),
			"MSG_PEEK":          phpv.ZInt(MSG_PEEK),
			"MSG_DONTWAIT":      phpv.ZInt(MSG_DONTWAIT),
			"MSG_WAITALL":       phpv.ZInt(MSG_WAITALL),
			"MSG_OOB":           phpv.ZInt(MSG_OOB),
			"MSG_EOR":           phpv.ZInt(MSG_EOR),
			"TCP_NODELAY":       phpv.ZInt(TCP_NODELAY),
			"SOCKET_EACCES":     phpv.ZInt(13),
			"SOCKET_EADDRINUSE": phpv.ZInt(98),
			"SOCKET_ECONNREFUSED": phpv.ZInt(111),
			"SOCKET_ETIMEDOUT":  phpv.ZInt(110),
		},
	})
}
