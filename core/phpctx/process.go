package phpctx

import (
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
func (p *Process) CommandLine(args []string) error {
	if args == nil {
		args = os.Args
	}

	// initially planned to use golang "flag" package, but not exactly suitable for PHP arguments...
	// TODO

	return nil
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
