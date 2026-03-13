package standard

import (
	"bufio"
	"bytes"
	"errors"
	"io"
	"os/exec"
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

	cmd = strings.TrimSpace(cmd)
	if cmd == "" {
		return phpv.ZNull{}.ZVal(), nil
	}

	// Use /bin/sh -c to match PHP's behavior (supports pipes, redirects, etc.)
	output, err := exec.Command("/bin/sh", "-c", cmd).Output()
	if err != nil {
		// PHP returns null when shell_exec fails
		return phpv.ZNull{}.ZVal(), nil
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

	cmdStr = strings.TrimSpace(cmdStr)
	if cmdStr == "" {
		return phpv.ZFalse.ZVal(), nil
	}

	// Use /bin/sh -c to match PHP's behavior
	var stderr bytes.Buffer
	var stdout bytes.Buffer
	command := exec.Command("/bin/sh", "-c", cmdStr)
	command.Stderr = &stderr
	command.Stdout = &stdout

	err = command.Run()
	if err != nil {
		errOutput, _ := io.ReadAll(&stderr)
		if len(errOutput) > 0 {
			ctx.Global().WriteErr(errOutput)
		}
	}
	if returnVar.HasArg() {
		exitCode := phpv.ZInt(command.ProcessState.ExitCode())
		returnVar.Set(ctx, exitCode)
	}

	// Print all output to stdout, return the last line
	var lastLine []byte
	buf := bufio.NewReader(&stdout)
	for {
		line, _, readErr := buf.ReadLine()
		if errors.Is(readErr, io.EOF) {
			break
		}
		if readErr != nil {
			return nil, ctx.Error(readErr)
		}
		lastLine = line
		ctx.Write(line)
		ctx.Write([]byte{'\n'})
	}

	return phpv.ZStr(string(lastLine)), nil
}
