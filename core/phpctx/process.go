package phpctx

import (
	"errors"
	"io"
	"net/http"
	"os"
	"strings"

	"github.com/MagicalTux/goro/core/phpv"
)

type Process struct {
	defaultConstants map[phpv.ZString]phpv.Val
	environ          *phpv.ZHashTable
	defaultOut       io.Writer
	defaultErr       io.Writer
}

type Options struct {
	RunCode    string
	NoIniFile  bool
	IniEntries map[string]string
}

// NewProcess instanciates a new instance of Process, which represents a
// running PHP process.
func NewProcess(sapi string) *Process {
	res := &Process{
		defaultConstants: make(map[phpv.ZString]phpv.Val),
		environ:          importEnv(os.Environ()),
		defaultOut:       os.Stdout,
		defaultErr:       os.Stderr,
	}
	res.populateConstants()
	res.SetConstant("PHP_SAPI", phpv.ZString(sapi))
	return res
}

// Hander returns a http.Handler object suitable for use with golang standard
// http servers and similar.
func (p *Process) Handler(docroot string) http.Handler {
	return &phpWebHandler{root: docroot, p: p}
}

func (p *Process) populateConstants() {
	for _, e := range globalExtMap {
		for k, v := range e.Constants {
			p.defaultConstants[k] = v
		}
	}

}

// SetConstant sets a global constant, typically used to set PHP_SAPI.
func (p *Process) SetConstant(name, value phpv.ZString) {
	p.defaultConstants[name] = value.ZVal()
}

// CommandLine will parse arguments from the command line and configure the
// process accordingly. If nil is passed, then the actual command line will be
// parsed. In case of error, messages will be sent to stderr by default.
func (p *Process) CommandLine(args []string) ([]string, *Options, error) {
	if args == nil {
		args = os.Args
	}

	processedArgs := []string{}

	options := &Options{
		IniEntries: map[string]string{},
	}

	args = expandArgs(args)
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if !strings.HasPrefix(arg, "-") {
			processedArgs = append(processedArgs, arg)
			continue
		}

		switch arg {
		case "-r", "--run":
			i++
			code := idx(args, i)
			if options.RunCode != "" {
				return nil, nil, errors.New("You can use -r only once")
			}
			options.RunCode = code
		case "-d", "--define":
			i++
			value := idx(args, i)
			options.IniEntries[arg] = value
		case "-n", "--no-php-ini":
			options.NoIniFile = true

			// TODO: add more flags
		}
	}

	return processedArgs, options, nil
}

// importEnv will copy an env type value (list of strings) as a hashtable
func importEnv(e []string) *phpv.ZHashTable {
	zt := phpv.NewHashTable()

	for _, s := range e {
		p := strings.IndexByte(s, '=')
		if p != -1 {
			zt.SetString(phpv.ZString(s[:p]), phpv.ZString(s[p+1:]).ZVal())
		}
	}

	return zt
}

// expands -foo=blah to -foo blah
// to make it easier to check
func expandArgs(args []string) []string {
	var result []string
	for _, arg := range args {
		if !strings.HasPrefix(arg, "-") {
			result = append(result, arg)
			continue
		}

		eq := strings.Index(arg, "=")
		if eq < 0 {
			result = append(result, arg)
			continue
		}

		result = append(result, arg[:eq])
		nextArg := arg[eq+1:]
		if nextArg != "" {
			result = append(result, nextArg)
		}

	}
	return result
}

// safe-index, returns default(T) if out of bounds
func idx[T any](xs []T, i int) T {
	var x T
	if i >= 0 && i < len(xs) {
		x = xs[i]
	}
	return x
}
