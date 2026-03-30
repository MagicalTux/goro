//go:build !linux

package sockets

import "syscall"

const (
	AF_INET  = syscall.AF_INET
	AF_INET6 = syscall.AF_INET6
	AF_UNIX  = syscall.AF_UNIX

	SOCK_STREAM = syscall.SOCK_STREAM
	SOCK_DGRAM  = syscall.SOCK_DGRAM
	SOCK_RAW    = syscall.SOCK_RAW

	SOL_SOCKET = syscall.SOL_SOCKET
	SOL_TCP    = 6
	SOL_UDP    = 17

	SO_REUSEADDR = syscall.SO_REUSEADDR
	SO_REUSEPORT = 0x200
	SO_KEEPALIVE = syscall.SO_KEEPALIVE
	SO_RCVTIMEO  = syscall.SO_RCVTIMEO
	SO_SNDTIMEO  = syscall.SO_SNDTIMEO
	SO_SNDBUF    = syscall.SO_SNDBUF
	SO_RCVBUF    = syscall.SO_RCVBUF
	SO_LINGER    = syscall.SO_LINGER
	SO_BROADCAST = syscall.SO_BROADCAST

	TCP_NODELAY = syscall.TCP_NODELAY

	SOMAXCONN = 128

	PHP_BINARY_READ = 2
	PHP_NORMAL_READ = 1

	MSG_PEEK     = syscall.MSG_PEEK
	MSG_DONTWAIT = 0x40
	MSG_WAITALL  = syscall.MSG_WAITALL
	MSG_OOB      = syscall.MSG_OOB
	MSG_EOR      = syscall.MSG_EOR
)
