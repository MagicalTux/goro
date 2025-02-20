package standard

import (
	"bufio"
	"bytes"
	"errors"
	"io"
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

// > func string system ( string $cmd [, int &$return_var ] )
func fncSystem(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var cmdStr string
	var returnVar core.OptionalRef[phpv.ZInt]
	_, err := core.Expand(ctx, args, &cmdStr, &returnVar)
	if err != nil {
		return nil, err
	}

	var lastLine []byte
	for _, c := range strings.Split(cmdStr, ";") {
		fields := regexp.MustCompile(`\s+`).Split(strings.TrimSpace(c), -1)
		prog := fields[0]
		cliArgs := fields[1:]

		if prog == "" {
			return phpv.ZStr(""), nil
		}

		var stderr bytes.Buffer
		var stdout bytes.Buffer
		command := exec.Command(prog, cliArgs...)
		command.Stderr = &stderr
		command.Stdout = &stdout

		err = command.Run()
		if err != nil {
			errOutput, err := io.ReadAll(&stderr)
			if err != nil {
				return nil, ctx.Error(err)
			}
			ctx.Global().WriteErr([]byte(errOutput))
		}
		if returnVar.HasArg() {
			exitCode := phpv.ZInt(command.ProcessState.ExitCode())
			returnVar.Set(ctx, exitCode)
		}

		// returns only the last line, but print
		// all output on stdout
		buf := bufio.NewReader(&stdout)
		for {
			line, _, err := buf.ReadLine()
			if errors.Is(err, io.EOF) {
				break
			}
			if err != nil {
				return nil, ctx.Error(err)
			}
			lastLine = line
			ctx.Write(line)
			ctx.Write([]byte{'\n'})
		}
	}

	return phpv.ZStr(string(lastLine)), nil
}
