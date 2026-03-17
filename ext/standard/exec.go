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

// > func string escapeshellarg ( string $arg )
func fncEscapeshellarg(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var arg phpv.ZString
	_, err := core.Expand(ctx, args, &arg)
	if err != nil {
		return nil, err
	}

	// Escape single quotes and wrap in single quotes
	s := string(arg)
	s = strings.ReplaceAll(s, "'", "'\\''")
	return phpv.ZString("'" + s + "'").ZVal(), nil
}

// > func string escapeshellcmd ( string $command )
func fncEscapeshellcmd(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var cmd phpv.ZString
	_, err := core.Expand(ctx, args, &cmd)
	if err != nil {
		return nil, err
	}

	// Escape shell metacharacters
	s := string(cmd)
	result := strings.Builder{}
	for _, c := range s {
		switch c {
		case '&', ';', '`', '|', '*', '?', '~', '<', '>', '^', '(', ')', '[', ']', '{', '}', '$', '\\', '#':
			result.WriteByte('\\')
		}
		result.WriteRune(c)
	}
	return phpv.ZString(result.String()).ZVal(), nil
}

// > func string exec ( string $command [, array &$output [, int &$return_var ]] )
func fncExec(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var cmdStr string
	var outputArr core.OptionalRef[*phpv.ZArray]
	var returnVar core.OptionalRef[phpv.ZInt]
	_, err := core.Expand(ctx, args, &cmdStr, &outputArr, &returnVar)
	if err != nil {
		return nil, err
	}

	cmdStr = strings.TrimSpace(cmdStr)
	if cmdStr == "" {
		return phpv.ZString("").ZVal(), nil
	}

	command := exec.Command("/bin/sh", "-c", cmdStr)
	out, runErr := command.Output()

	exitCode := 0
	if runErr != nil {
		if exitErr, ok := runErr.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		}
	} else if command.ProcessState != nil {
		exitCode = command.ProcessState.ExitCode()
	}

	outStr := string(out)
	outStr = strings.TrimRight(outStr, "\n")
	var lines []string
	if outStr != "" {
		lines = strings.Split(outStr, "\n")
	}

	lastLine := ""
	if len(lines) > 0 {
		lastLine = lines[len(lines)-1]
	}

	if outputArr.HasArg() {
		arr := phpv.NewZArray()
		for _, line := range lines {
			arr.OffsetSet(ctx, nil, phpv.ZString(line).ZVal())
		}
		outputArr.Set(ctx, arr)
	}

	if returnVar.HasArg() {
		returnVar.Set(ctx, phpv.ZInt(exitCode))
	}

	return phpv.ZString(lastLine).ZVal(), nil
}

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
