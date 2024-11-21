package standard

import (
	"os/exec"
	"regexp"
	"strings"

	"github.com/MagicalTux/goro/core"
	"github.com/MagicalTux/goro/core/phpv"
)

// > func string shell_exec ( string $cmd )
func fncShellExec(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var cmd string
	_, err := core.Expand(ctx, args, &cmd)
	if err != nil {
		return nil, err
	}

	fields := regexp.MustCompile(`\s+`).Split(strings.TrimSpace(cmd), -1)
	prog := fields[0]
	cliArgs := fields[1:]

	if prog == "" {
		return phpv.ZStr(""), nil
	}

	output, err := exec.Command(prog, cliArgs...).Output()
	if err != nil {
		return nil, err
	}

	return phpv.ZStr(string(output)), nil
}
