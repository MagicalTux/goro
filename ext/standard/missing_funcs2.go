package standard

import (
	"fmt"
	"net"
	"os"
	"runtime"
	"strings"
	"syscall"

	"github.com/MagicalTux/goro/core"
	"github.com/MagicalTux/goro/core/phpctx"
	"github.com/MagicalTux/goro/core/phpobj"
	"github.com/MagicalTux/goro/core/phpv"
)

// > func bool is_countable ( mixed $value )
func fncIsCountable(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var z *phpv.ZVal
	_, err := core.Expand(ctx, args, &z)
	if err != nil {
		return nil, err
	}
	if z.GetType() == phpv.ZtArray {
		return phpv.ZTrue.ZVal(), nil
	}
	if z.GetType() == phpv.ZtObject {
		if _, ok := z.Value().(phpv.ZCountable); ok {
			return phpv.ZTrue.ZVal(), nil
		}
	}
	return phpv.ZFalse.ZVal(), nil
}

// > func bool is_iterable ( mixed $value )
func fncIsIterable(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var z *phpv.ZVal
	_, err := core.Expand(ctx, args, &z)
	if err != nil {
		return nil, err
	}
	if z.GetType() == phpv.ZtArray {
		return phpv.ZTrue.ZVal(), nil
	}
	if z.GetType() == phpv.ZtObject {
		obj, ok := z.Value().(phpv.ZObject)
		if ok && obj.GetClass().InstanceOf(phpobj.Traversable) {
			return phpv.ZTrue.ZVal(), nil
		}
	}
	return phpv.ZFalse.ZVal(), nil
}

// > func mixed forward_static_call ( callable $callback [, mixed $... ] )
func fncForwardStaticCall(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	if len(args) < 1 {
		return nil, phpobj.ThrowError(ctx, phpobj.ArgumentCountError,
			"forward_static_call() expects at least 1 argument, 0 given")
	}

	var callback phpv.Callable
	_, err := core.Expand(ctx, args, &callback)
	if err != nil {
		return nil, err
	}

	return ctx.CallZVal(ctx, callback, args[1:])
}

// > func mixed forward_static_call_array ( callable $callback , array $args )
func fncForwardStaticCallArray(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	if len(args) < 2 {
		return nil, phpobj.ThrowError(ctx, phpobj.ArgumentCountError,
			"forward_static_call_array() expects exactly 2 arguments")
	}

	var callback phpv.Callable
	var argsArray *phpv.ZArray
	_, err := core.Expand(ctx, args, &callback, &argsArray)
	if err != nil {
		return nil, err
	}

	var callArgs []*phpv.ZVal
	if argsArray != nil {
		for _, v := range argsArray.Iterate(ctx) {
			callArgs = append(callArgs, v)
		}
	}

	return ctx.CallZVal(ctx, callback, callArgs)
}

// > func int connection_aborted ( void )
func fncConnectionAborted(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	return phpv.ZInt(0).ZVal(), nil
}

// > func int connection_status ( void )
func fncConnectionStatus(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	// CONNECTION_NORMAL = 0
	return phpv.ZInt(0).ZVal(), nil
}

// > func int|bool ignore_user_abort ([ bool $enable ] )
func fncIgnoreUserAbort(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	// Always return 0 (previous setting was "don't ignore")
	return phpv.ZInt(0).ZVal(), nil
}

// > func array sys_getloadavg ( void )
func fncSysGetloadavg(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	result := phpv.NewZArray()
	// Return a reasonable approximation
	result.OffsetSet(ctx, nil, phpv.ZFloat(0.0).ZVal())
	result.OffsetSet(ctx, nil, phpv.ZFloat(0.0).ZVal())
	result.OffsetSet(ctx, nil, phpv.ZFloat(0.0).ZVal())
	return result.ZVal(), nil
}

// > func array|false getrusage ([ int $mode = 0 ] )
func fncGetrusage(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var mode *phpv.ZInt
	_, err := core.Expand(ctx, args, &mode)
	if err != nil {
		return nil, err
	}

	who := syscall.RUSAGE_SELF
	if mode != nil && *mode == 1 {
		who = syscall.RUSAGE_CHILDREN
	}

	var rusage syscall.Rusage
	if err := syscall.Getrusage(who, &rusage); err != nil {
		return phpv.ZFalse.ZVal(), nil
	}

	result := phpv.NewZArray()
	result.OffsetSet(ctx, phpv.ZString("ru_utime.tv_sec").ZVal(), phpv.ZInt(rusage.Utime.Sec).ZVal())
	result.OffsetSet(ctx, phpv.ZString("ru_utime.tv_usec").ZVal(), phpv.ZInt(rusage.Utime.Usec).ZVal())
	result.OffsetSet(ctx, phpv.ZString("ru_stime.tv_sec").ZVal(), phpv.ZInt(rusage.Stime.Sec).ZVal())
	result.OffsetSet(ctx, phpv.ZString("ru_stime.tv_usec").ZVal(), phpv.ZInt(rusage.Stime.Usec).ZVal())
	result.OffsetSet(ctx, phpv.ZString("ru_maxrss").ZVal(), phpv.ZInt(rusage.Maxrss).ZVal())
	result.OffsetSet(ctx, phpv.ZString("ru_ixrss").ZVal(), phpv.ZInt(rusage.Ixrss).ZVal())
	result.OffsetSet(ctx, phpv.ZString("ru_idrss").ZVal(), phpv.ZInt(rusage.Idrss).ZVal())
	result.OffsetSet(ctx, phpv.ZString("ru_isrss").ZVal(), phpv.ZInt(rusage.Isrss).ZVal())
	result.OffsetSet(ctx, phpv.ZString("ru_minflt").ZVal(), phpv.ZInt(rusage.Minflt).ZVal())
	result.OffsetSet(ctx, phpv.ZString("ru_majflt").ZVal(), phpv.ZInt(rusage.Majflt).ZVal())
	result.OffsetSet(ctx, phpv.ZString("ru_nswap").ZVal(), phpv.ZInt(rusage.Nswap).ZVal())
	result.OffsetSet(ctx, phpv.ZString("ru_inblock").ZVal(), phpv.ZInt(rusage.Inblock).ZVal())
	result.OffsetSet(ctx, phpv.ZString("ru_oublock").ZVal(), phpv.ZInt(rusage.Oublock).ZVal())
	result.OffsetSet(ctx, phpv.ZString("ru_msgsnd").ZVal(), phpv.ZInt(rusage.Msgsnd).ZVal())
	result.OffsetSet(ctx, phpv.ZString("ru_msgrcv").ZVal(), phpv.ZInt(rusage.Msgrcv).ZVal())
	result.OffsetSet(ctx, phpv.ZString("ru_nsignals").ZVal(), phpv.ZInt(rusage.Nsignals).ZVal())
	result.OffsetSet(ctx, phpv.ZString("ru_nvcsw").ZVal(), phpv.ZInt(rusage.Nvcsw).ZVal())
	result.OffsetSet(ctx, phpv.ZString("ru_nivcsw").ZVal(), phpv.ZInt(rusage.Nivcsw).ZVal())
	return result.ZVal(), nil
}

// > func int|false getservbyname ( string $service , string $protocol )
func fncGetservbyname(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var service, protocol phpv.ZString
	_, err := core.Expand(ctx, args, &service, &protocol)
	if err != nil {
		return nil, err
	}

	port, err := net.LookupPort(string(protocol), string(service))
	if err != nil {
		return phpv.ZFalse.ZVal(), nil
	}
	return phpv.ZInt(port).ZVal(), nil
}

// > func string|false getservbyport ( int $port , string $protocol )
func fncGetservbyport(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var port phpv.ZInt
	var protocol phpv.ZString
	_, err := core.Expand(ctx, args, &port, &protocol)
	if err != nil {
		return nil, err
	}

	// Go doesn't have a direct reverse lookup for service names by port
	// Use a hardcoded map for common services
	services := map[string]map[int]string{
		"tcp": {
			7: "echo", 9: "discard", 13: "daytime", 19: "chargen",
			20: "ftp-data", 21: "ftp", 22: "ssh", 23: "telnet",
			25: "smtp", 37: "time", 53: "domain", 79: "finger",
			80: "http", 110: "pop3", 111: "sunrpc", 119: "nntp",
			135: "loc-srv", 139: "netbios-ssn", 143: "imap",
			443: "https", 445: "microsoft-ds", 465: "smtps",
			587: "submission", 993: "imaps", 995: "pop3s",
		},
		"udp": {
			7: "echo", 9: "discard", 13: "daytime", 19: "chargen",
			37: "time", 53: "domain", 67: "bootps", 68: "bootpc",
			69: "tftp", 111: "sunrpc", 123: "ntp", 137: "netbios-ns",
			138: "netbios-dgm", 161: "snmp", 162: "snmptrap",
			514: "syslog",
		},
	}

	proto := strings.ToLower(string(protocol))
	if svcMap, ok := services[proto]; ok {
		if name, ok := svcMap[int(port)]; ok {
			return phpv.ZString(name).ZVal(), nil
		}
	}
	return phpv.ZFalse.ZVal(), nil
}

// > func array|false get_extension_funcs ( string $extension )
func fncGetExtensionFuncs(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var extName phpv.ZString
	_, err := core.Expand(ctx, args, &extName)
	if err != nil {
		return nil, err
	}

	ext := phpctx.GetExt(string(extName))
	if ext == nil {
		return phpv.ZFalse.ZVal(), nil
	}

	result := phpv.NewZArray()
	for name := range ext.Functions {
		result.OffsetSet(ctx, nil, phpv.ZString(name).ZVal())
	}
	return result.ZVal(), nil
}

// > func bool phpinfo ([ int $what = INFO_ALL ] )
func fncPhpinfo(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	// Output a basic phpinfo page
	fmt.Fprintf(ctx, "phpinfo()\nPHP Version => %s\n\nSystem => %s %s %s %s\n",
		core.VERSION, runtime.GOOS, "", runtime.GOARCH, runtime.Version())
	fmt.Fprintf(ctx, "Build Date => %s\nServer API => %s\n",
		"unknown", "goro")
	fmt.Fprintf(ctx, "PHP API => 20210902\n")
	return phpv.ZTrue.ZVal(), nil
}

// > func bool proc_nice ( int $priority )
func fncProcNice(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var priority phpv.ZInt
	_, err := core.Expand(ctx, args, &priority)
	if err != nil {
		return nil, err
	}

	err2 := syscall.Setpriority(syscall.PRIO_PROCESS, 0, int(priority))
	if err2 != nil {
		return phpv.ZFalse.ZVal(), ctx.Warn("proc_nice(): Permission denied")
	}
	return phpv.ZTrue.ZVal(), nil
}

// > func string get_debug_type ( mixed $value )
func fncGetDebugType(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var z *phpv.ZVal
	_, err := core.Expand(ctx, args, &z)
	if err != nil {
		return nil, err
	}

	switch z.GetType() {
	case phpv.ZtNull:
		return phpv.ZString("null").ZVal(), nil
	case phpv.ZtBool:
		return phpv.ZString("bool").ZVal(), nil
	case phpv.ZtInt:
		return phpv.ZString("int").ZVal(), nil
	case phpv.ZtFloat:
		return phpv.ZString("float").ZVal(), nil
	case phpv.ZtString:
		return phpv.ZString("string").ZVal(), nil
	case phpv.ZtArray:
		return phpv.ZString("array").ZVal(), nil
	case phpv.ZtResource:
		return phpv.ZString("resource").ZVal(), nil
	case phpv.ZtObject:
		obj, ok := z.Value().(phpv.ZObject)
		if ok {
			return phpv.ZString(obj.GetClass().GetName()).ZVal(), nil
		}
		return phpv.ZString("object").ZVal(), nil
	default:
		return phpv.ZString("unknown").ZVal(), nil
	}
}

// > func string|false php_sapi_name ( void )
func fncPhpSapiName(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	return phpv.ZString("cli").ZVal(), nil
}

// > func int getmygid ( void )
func fncGetmygid(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	return phpv.ZInt(os.Getgid()).ZVal(), nil
}
