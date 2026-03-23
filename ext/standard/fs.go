package standard

import (
	"errors"
	"io"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"regexp"
	"strings"
	"syscall"

	"github.com/MagicalTux/goro/core/phpobj"

	"github.com/MagicalTux/goro/core"
	"github.com/MagicalTux/goro/core/logopt"
	"github.com/MagicalTux/goro/core/phpv"
	"github.com/MagicalTux/goro/core/stream"
)

// > const
const (
	FILE_USE_INCLUDE_PATH   phpv.ZInt = 1
	FILE_IGNORE_NEW_LINES   phpv.ZInt = 2
	FILE_SKIP_EMPTY_LINES   phpv.ZInt = 4
	FILE_APPEND             phpv.ZInt = 8
	FILE_NO_DEFAULT_CONTEXT phpv.ZInt = 16
	FILE_BINARY             phpv.ZInt = 0
	FILE_TEXT               phpv.ZInt = 0
)

// > const
const (
	LOCK_SH phpv.ZInt = 1
	LOCK_EX phpv.ZInt = 2
	LOCK_NB phpv.ZInt = 4
	LOCK_UN phpv.ZInt = 8
)

// > func string dirname ( string $path [, int $levels = 1 ] )
func fncDirname(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var p string
	var lvl *phpv.ZInt
	_, err := core.Expand(ctx, args, &p, &lvl)
	if err != nil {
		return nil, err
	}

	for {
		if len(p) <= 1 {
			break
		}
		if p[len(p)-1] != '/' {
			break
		}
		p = p[:len(p)-1]
	}

	if lvl == nil {
		return phpv.ZString(path.Dir(p)).ZVal(), nil
	}

	levels := *lvl
	if levels < 1 {
		return nil, phpobj.ThrowError(ctx, phpobj.ValueError, "dirname(): Argument #2 ($levels) must be greater than or equal to 1")
	}

	for i := phpv.ZInt(0); i < levels; i++ {
		prev := p
		p = path.Dir(p)
		if p == prev {
			break // reached root, no point continuing
		}
	}
	return phpv.ZString(p).ZVal(), nil
}

// > func string basename ( string $path [, string $suffix] )
func fncBasename(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var path string
	var suffix core.Optional[phpv.ZString]
	_, err := core.Expand(ctx, args, &path, &suffix)
	if err != nil {
		return nil, err
	}

	// PHP's basename:
	// - empty string returns empty string
	// - only uses forward slash as separator on non-Windows
	// - strips trailing forward slashes before computing
	if path == "" {
		return phpv.ZString("").ZVal(), nil
	}

	// Strip trailing forward slashes (not backslashes on Linux)
	for len(path) > 1 && path[len(path)-1] == '/' {
		path = path[:len(path)-1]
	}

	// Find last forward slash
	idx := strings.LastIndex(path, "/")
	var result string
	if idx >= 0 {
		result = path[idx+1:]
	} else {
		result = path
	}

	if suffix.HasArg() && result != string(suffix.Get()) {
		result = strings.TrimSuffix(result, string(suffix.Get()))
	}

	return phpv.ZString(result).ZVal(), nil
}

// > func bool file_exists ( string $filename )
func fncFileExists(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var filename phpv.ZString
	_, err := core.Expand(ctx, args, &filename)
	if err != nil {
		return nil, err
	}

	// Empty string should return false
	if string(filename) == "" {
		return phpv.ZFalse.ZVal(), nil
	}

	// Check for path length exceeding system maximum
	p := string(filename)
	if !filepath.IsAbs(p) {
		p = filepath.Join(string(ctx.Global().Getwd()), p)
	}
	if len(p) > int(core.PHP_MAXPATHLEN) {
		ctx.Warn("file_exists(): File name is longer than the maximum allowed path length on this platform (%d): %s", core.PHP_MAXPATHLEN, p, logopt.NoFuncName(true))
		return phpv.ZFalse.ZVal(), nil
	}

	if err := ctx.Global().CheckOpenBasedir(ctx, string(filename), "file_exists"); err != nil {
		return phpv.ZFalse.ZVal(), nil
	}

	r, err := ctx.Global().Exists(filename)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return phpv.ZFalse.ZVal(), nil
		}
		return nil, err
	}

	return phpv.ZBool(r).ZVal(), nil
}

// > func bool is_dir ( string $filename )
func fncIsDir(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var filename phpv.ZString
	_, err := core.Expand(ctx, args, &filename)
	if err != nil {
		return nil, err
	}

	if string(filename) == "" {
		return phpv.ZFalse.ZVal(), nil
	}

	if err := ctx.Global().CheckOpenBasedir(ctx, string(filename), "is_dir"); err != nil {
		return phpv.ZFalse.ZVal(), nil
	}

	r, err := ctx.Global().Open(ctx, filename, "r", true)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return phpv.ZFalse.ZVal(), nil
		}
		return nil, err
	}
	stat, err := r.Stat()
	if err != nil {
		return nil, err
	}

	return phpv.ZBool(stat.IsDir()).ZVal(), nil
}

// > func bool is_file ( string $filename )
func fncIsFile(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var filename phpv.ZString
	_, err := core.Expand(ctx, args, &filename)
	if err != nil {
		return nil, err
	}

	if string(filename) == "" {
		return phpv.ZFalse.ZVal(), nil
	}

	if err := ctx.Global().CheckOpenBasedir(ctx, string(filename), "is_file"); err != nil {
		return phpv.ZFalse.ZVal(), nil
	}

	r, err := ctx.Global().Open(ctx, filename, "r", true)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return phpv.ZFalse.ZVal(), nil
		}
		return nil, err
	}
	stat, err := r.Stat()
	if err != nil {
		return nil, err
	}

	return phpv.ZBool(!stat.IsDir()).ZVal(), nil
}

// > func bool is_readable ( string $filename )
func fncIsReadable(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var filename phpv.ZString
	_, err := core.Expand(ctx, args, &filename)
	if err != nil {
		return nil, err
	}

	if err := ctx.Global().CheckOpenBasedir(ctx, string(filename), "is_readable"); err != nil {
		return phpv.ZFalse.ZVal(), nil
	}

	r, err := ctx.Global().Open(ctx, filename, "r", true)
	if err != nil {
		return phpv.ZFalse.ZVal(), nil
	}
	r.Close()
	return phpv.ZTrue.ZVal(), nil
}

// > func bool is_writable ( string $filename )
func fncIsWritable(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var filename phpv.ZString
	_, err := core.Expand(ctx, args, &filename)
	if err != nil {
		return nil, err
	}

	if err := ctx.Global().CheckOpenBasedir(ctx, string(filename), "is_writable"); err != nil {
		return phpv.ZFalse.ZVal(), nil
	}

	r, err := ctx.Global().Open(ctx, filename, "r", true)
	if err != nil {
		return phpv.ZFalse.ZVal(), nil
	}
	stat, err := r.Stat()
	r.Close()
	if err != nil {
		return phpv.ZFalse.ZVal(), nil
	}

	mode := stat.Mode()
	if mode&0200 != 0 {
		return phpv.ZTrue.ZVal(), nil
	}
	return phpv.ZFalse.ZVal(), nil
}

// > func bool is_executable ( string $filename )
func fncIsExecutable(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var filename phpv.ZString
	_, err := core.Expand(ctx, args, &filename)
	if err != nil {
		return nil, err
	}

	if err := ctx.Global().CheckOpenBasedir(ctx, string(filename), "is_executable"); err != nil {
		return phpv.ZFalse.ZVal(), nil
	}

	r, err := ctx.Global().Open(ctx, filename, "r", true)
	if err != nil {
		return phpv.ZFalse.ZVal(), nil
	}
	stat, err := r.Stat()
	r.Close()
	if err != nil {
		return phpv.ZFalse.ZVal(), nil
	}

	mode := stat.Mode()
	if mode&0111 != 0 {
		return phpv.ZTrue.ZVal(), nil
	}
	return phpv.ZFalse.ZVal(), nil
}

// > func bool is_link ( string $filename )
func fncIsLink(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var filename phpv.ZString
	_, err := core.Expand(ctx, args, &filename)
	if err != nil {
		return nil, err
	}

	if err := ctx.Global().CheckOpenBasedir(ctx, string(filename), "is_link"); err != nil {
		return phpv.ZFalse.ZVal(), nil
	}

	p := string(filename)
	if !filepath.IsAbs(p) {
		p = filepath.Join(string(ctx.Global().Getwd()), p)
	}

	// Use Lstat to check if the path itself is a symlink (not following it)
	stat, err := os.Lstat(p)
	if err != nil {
		return phpv.ZFalse.ZVal(), nil
	}

	return phpv.ZBool(stat.Mode()&os.ModeSymlink != 0).ZVal(), nil
}

// > func string realpath ( string $filename )
func fncRealPath(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var filename string
	_, err := core.Expand(ctx, args, &filename)
	if err != nil {
		return nil, err
	}

	if err := ctx.Global().CheckOpenBasedir(ctx, filename, "realpath"); err != nil {
		return phpv.ZFalse.ZVal(), nil
	}

	if regexp.MustCompile(`$\w+:\/\/`).MatchString(filename) {
		return phpv.ZFalse.ZVal(), nil
	}

	filename = resolveFilePath(ctx, filename)

	_, err = os.Stat(filename)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return phpv.ZFalse.ZVal(), nil
		}
		return nil, err
	}

	filename, err = filepath.Abs(filename)
	if err != nil {
		return nil, err
	}

	return phpv.ZStr(filename), nil
}

// > func string unlink ( string $filename [, resource $context ] )
func fncUnlink(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var filename string
	var context **phpv.ZVal // optional context parameter
	_, err := core.Expand(ctx, args, &filename, &context)
	if err != nil {
		return nil, err
	}

	if err := ctx.Global().CheckOpenBasedir(ctx, filename, "unlink"); err != nil {
		return phpv.ZFalse.ZVal(), nil
	}

	p := filename
	if !filepath.IsAbs(p) {
		p = filepath.Join(string(ctx.Global().Getwd()), p)
	}

	// Use Lstat so broken symlinks can still be unlinked
	stat, err := os.Lstat(p)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			err = ctx.Warn("unlink(%s): No such file or directory", filename, logopt.NoFuncName(true))
		} else {
			err = ctx.Warn("unlink(%s): %s", filename, err.Error(), logopt.NoFuncName(true))
		}
		return phpv.ZFalse.ZVal(), err
	}
	if stat.Mode()&os.ModeSymlink == 0 && stat.IsDir() {
		return phpv.ZFalse.ZVal(), ctx.Warn("unlink(%s): Is a directory", filename, logopt.NoFuncName(true))
	}

	if err := os.Remove(p); err != nil {
		return phpv.ZFalse.ZVal(), ctx.Warn("unlink(%s): %s", filename, err.Error(), logopt.NoFuncName(true))
	}

	return phpv.ZTrue.ZVal(), nil
}

// > func bool mkdir ( string $pathname [, int $mode = 0777 [, bool $recursive = FALSE [, resource $context ]]] )
func fncMkdir(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var pathname string
	var modeArg *phpv.ZInt
	var recursiveArg *phpv.ZBool
	var context **phpv.ZVal
	_, err := core.Expand(ctx, args, &pathname, &modeArg, &recursiveArg, &context)
	if err != nil {
		return nil, err
	}

	if err := ctx.Global().CheckOpenBasedir(ctx, pathname, "mkdir"); err != nil {
		return phpv.ZFalse.ZVal(), nil
	}

	mode := core.Deref(modeArg, phpv.ZInt(0777))
	recursive := core.Deref(recursiveArg, phpv.ZBool(false))

	pathname = resolveFilePath(ctx, pathname)

	if recursive {
		err = os.MkdirAll(pathname, os.FileMode(mode))
	} else {
		err = os.Mkdir(pathname, os.FileMode(mode))
	}

	if err != nil {
		return phpv.ZFalse.ZVal(), ctx.Warn("%s", err.Error())
	}

	return phpv.ZTrue.ZVal(), nil
}

// > func string rmdir ( string $dirname [, resource $context ] )
func fncRmdir(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var dirname string
	var context **phpv.ZVal
	_, err := core.Expand(ctx, args, &dirname, &context)
	if err != nil {
		return nil, err
	}

	if context != nil {
		return nil, ctx.Errorf("context resource is not yet supported, must be NULL")
	}

	if err := ctx.Global().CheckOpenBasedir(ctx, dirname, "rmdir"); err != nil {
		return phpv.ZFalse.ZVal(), nil
	}

	dirname = resolveFilePath(ctx, dirname)

	stat, err := os.Stat(dirname)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			err = ctx.Warn("No such file or directory")
		} else {
			err = ctx.Warn(err.Error())
		}
		return phpv.ZFalse.ZVal(), err
	}
	if !stat.IsDir() {
		return phpv.ZFalse.ZVal(), ctx.Warn("Not a directory")
	}

	if err := os.Remove(dirname); err != nil {
		return phpv.ZFalse.ZVal(), ctx.Warn(err.Error())
	}

	return phpv.ZTrue.ZVal(), nil
}

// > func bool file_get_contents ( string $filename [, bool $use_include_path = FALSE [, resource $context [, int $offset = 0 [, int $maxlen ]]]] )
func fncFileGetContents(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var filename phpv.ZString
	var useIncludePathArg *phpv.ZBool
	var contextZval *phpv.ZVal
	var offsetArg core.Optional[phpv.ZInt]
	var maxlen core.Optional[phpv.ZInt]

	// Expand required param first
	var err error
	if err = core.ExpandAt(ctx, args, 0, &filename); err != nil {
		return nil, err
	}
	// Optional params
	core.ExpandAt(ctx, args, 1, &useIncludePathArg)
	// Context param: manually handle since *phpv.ZVal is not treated as optional by Expand
	if len(args) >= 3 {
		contextZval = args[2]
	}
	core.ExpandAt(ctx, args, 3, &offsetArg)
	core.ExpandAt(ctx, args, 4, &maxlen)

	// Empty path throws ValueError
	if string(filename) == "" {
		return nil, phpobj.ThrowError(ctx, phpobj.ValueError, "Path must not be empty")
	}

	if useIncludePathArg != nil && *useIncludePathArg {
		// TODO: handle use_include_path
		return nil, errors.New("use_include_path is not yet supported, set to false")
	}

	if err := ctx.Global().CheckOpenBasedir(ctx, string(filename), "file_get_contents"); err != nil {
		ctx.Warn("%s(%s): Failed to open stream: Operation not permitted", ctx.GetFuncName(), filename, logopt.NoFuncName(true))
		return phpv.ZFalse.ZVal(), nil
	}

	if maxlen.HasArg() && maxlen.Get() < 0 {
		return nil, phpobj.ThrowError(ctx, phpobj.ValueError, "file_get_contents(): Argument #5 ($length) must be greater than or equal to 0")
	}

	// Parse the context resource - accept NULL or a valid stream context
	var contextResource phpv.Resource
	if contextZval != nil && contextZval.GetType() != phpv.ZtNull {
		s, cerr := contextZval.As(ctx, phpv.ZtResource)
		if cerr != nil {
			return nil, cerr
		}
		res, ok := s.Value().(phpv.Resource)
		if !ok {
			return nil, ctx.FuncErrorf("$context must be a stream context")
		}
		if _, ok := res.(*stream.Context); !ok {
			return nil, ctx.FuncErrorf("$context must be a stream context")
		}
		contextResource = res
	}

	var f phpv.Stream
	if contextResource != nil {
		f, err = ctx.Global().Open(ctx, filename, "r", true, contextResource)
	} else {
		f, err = ctx.Global().Open(ctx, filename, "r", true)
	}
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return phpv.ZFalse.ZVal(), ctx.Warn("%s(%s): Failed to open stream: No such file or directory", ctx.GetFuncName(), filename, logopt.NoFuncName(true))
		}
		// Catch "is a directory" and permission errors gracefully
		return phpv.ZFalse.ZVal(), ctx.Warn("%s(%s): Failed to open stream: %s", ctx.GetFuncName(), filename, err, logopt.NoFuncName(true))
	}
	defer f.Close()

	if offsetArg.HasArg() {
		off := offsetArg.Get()
		if off < 0 {
			_, err = f.Seek(int64(offsetArg.Get()), io.SeekEnd)
		} else {
			_, err = f.Seek(int64(offsetArg.Get()), io.SeekStart)
		}
		if err != nil {
			return phpv.ZFalse.ZVal(), ctx.Warn("%s(%s): Failed to seek stream", ctx.GetFuncName(), filename, logopt.NoFuncName(true))
		}
	}

	if !maxlen.HasArg() {
		buf, err := io.ReadAll(f)
		if err != nil {
			return phpv.ZFalse.ZVal(), ctx.Warn("%s(%s): Failed to read stream: %s", ctx.GetFuncName(), filename, err, logopt.NoFuncName(true))
		}
		return phpv.ZStr(string(buf)), nil
	}

	buf := make([]byte, maxlen.Get())
	n, err := f.Read(buf)
	if err != nil && err != io.EOF {
		return nil, err
	}
	if n == 0 {
		// Nothing was read (e.g. offset past end of stream) — PHP returns false
		return phpv.ZFalse.ZVal(), nil
	}

	return phpv.ZStr(string(buf[:n])), nil
}

// > func int file_put_contents ( string $filename , mixed $data [, int $flags = 0 [, resource $context ]]
func fncFilePutContents(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var filename phpv.ZString
	var data *phpv.ZVal
	var flagsArg *phpv.ZInt
	_, err := core.Expand(ctx, args, &filename, &data, &flagsArg)
	if err != nil {
		return nil, err
	}

	// Empty path throws ValueError
	if string(filename) == "" {
		return nil, phpobj.ThrowError(ctx, phpobj.ValueError, "Path must not be empty")
	}

	if err := ctx.Global().CheckOpenBasedir(ctx, string(filename), "file_put_contents"); err != nil {
		ctx.Warn("%s(%s): Failed to open stream: Operation not permitted", ctx.GetFuncName(), filename, logopt.NoFuncName(true))
		return phpv.ZFalse.ZVal(), nil
	}

	openMode := phpv.ZString("w")
	if flagsArg != nil {
		if (*flagsArg & FILE_APPEND) != 0 {
			openMode = "a"
		}
	}

	fh, err := ctx.Global().Open(ctx, filename, openMode, false)
	if err != nil {
		return nil, err
	}
	defer fh.Close()
	ss, _ := fh.(*stream.Stream)
	if ss == nil {
		return nil, errors.New("file_put_contents: cannot get stream")
	}
	r := ss.UnderlyingFile()
	if r == nil {
		return nil, errors.New("file_put_contents: cannot get underlying file handle")
	}

	written := 0
	switch data.GetType() {
	case phpv.ZtResource:
		res, rok := data.Value().(phpv.Resource)
		if !rok {
			return nil, errors.New("data resource is not a valid resource")
		}
		stm, sok := res.(*stream.Stream)
		if !sok {
			return nil, errors.New("data resource is not a stream")
		}
		rbuf, rerr := io.ReadAll(stm)
		if rerr != nil {
			return nil, rerr
		}
		if written, err = r.Write(rbuf); err != nil {
			return nil, err
		}

	case phpv.ZtArray:
		arr, aok := data.Value().(*phpv.ZArray)
		if aok && arr != nil {
			it := arr.NewIterator()
			for ; it.Valid(ctx); it.Next(ctx) {
				val, ierr := it.Current(ctx)
				if ierr != nil {
					return nil, ierr
				}
				sv := val.String()
				wn, werr := r.Write([]byte(sv))
				if werr != nil {
					return nil, werr
				}
				written += wn
			}
		}
	default:
		str := data.String()
		if written, err = r.Write([]byte(str)); err != nil {
			return nil, err
		}

	}

	return phpv.ZInt(written).ZVal(), nil
}

// > func resource fopen (  string $filename , string $mode [, bool $use_include_path = FALSE [, resource $context ]] )
func fncFileOpen(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var filename phpv.ZString
	var mode phpv.ZString
	var useIncludePathArg core.Optional[phpv.ZBool]
	var contextResource core.Optional[phpv.Resource]
	_, err := core.Expand(ctx, args, &filename, &mode, &useIncludePathArg, &contextResource)
	if err != nil {
		return nil, err
	}

	useIncludePath := useIncludePathArg.HasArg() && bool(useIncludePathArg.Get())

	// Empty path throws ValueError
	if string(filename) == "" {
		return nil, phpobj.ThrowError(ctx, phpobj.ValueError, "Path must not be empty")
	}

	if err := ctx.Global().CheckOpenBasedir(ctx, string(filename), "fopen"); err != nil {
		ctx.Warn("%s(%s): Failed to open stream: Operation not permitted", ctx.GetFuncName(), filename, logopt.NoFuncName(true))
		return phpv.ZFalse.ZVal(), nil
	}

	f, err := ctx.Global().Open(ctx, filename, mode, useIncludePath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return phpv.ZFalse.ZVal(), ctx.Warn("%s(%s): Failed to open stream: No such file or directory", ctx.GetFuncName(), filename, logopt.NoFuncName(true))
		}
		return nil, ctx.Error(err)
	}

	return f.ZVal(), nil
}

// > func bool fclose ( resource $handle)
func fncFileClose(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var handle phpv.Resource
	_, err := core.Expand(ctx, args, &handle)
	if err != nil {
		return nil, err
	}
	if handle == nil {
		return nil, nil
	}

	if f, ok := handle.(*stream.Stream); ok {
		err = f.Close()
		if err != nil {
			return phpv.ZFalse.ZVal(), nil
		}
		// Mark the resource as closed (type becomes "Unknown")
		f.ResourceType = phpv.ResourceUnknown
		return phpv.ZTrue.ZVal(), nil
	}

	return phpv.ZFalse.ZVal(), nil
}

// > func int fwrite ( resource $handle , string $string [, int $length ] )
// > alias fputs
func fncFwrite(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var handle phpv.Resource
	var data phpv.ZString
	var length *phpv.ZInt
	_, err := core.Expand(ctx, args, &handle, &data, &length)
	if err != nil {
		return nil, err
	}
	if handle == nil {
		return phpv.ZFalse.ZVal(), nil
	}

	var file phpv.Stream
	switch handle.GetResourceType() {
	case phpv.ResourceStream:
		if f, ok := handle.(*stream.Stream); ok {
			file = f
		}
	}
	if file == nil {
		return phpv.ZFalse.ZVal(), nil
	}

	b := []byte(data)
	if length != nil {
		l := int(*length)
		if l < 0 {
			l = 0
		}
		if l < len(b) {
			b = b[:l]
		}
	}
	n, err := file.Write(b)
	if err != nil {
		ctx.Notice("fwrite(): Write of %d bytes failed with errno=9 Bad file descriptor", len(b), logopt.NoFuncName(true))
		return phpv.ZFalse.ZVal(), nil
	}
	return phpv.ZInt(n).ZVal(), nil
}

// > func string fread ( resource $handle , int $length )
func fncFread(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var handle phpv.Resource
	var length phpv.ZInt
	_, err := core.Expand(ctx, args, &handle, &length)
	if err != nil {
		return nil, err
	}
	if handle == nil {
		return phpv.ZFalse.ZVal(), nil
	}

	var file phpv.Stream
	switch handle.GetResourceType() {
	case phpv.ResourceStream:
		if f, ok := handle.(*stream.Stream); ok {
			file = f
		}
	}
	if file == nil {
		return phpv.ZFalse.ZVal(), nil
	}

	if length <= 0 {
		return nil, phpobj.ThrowError(ctx, phpobj.ValueError,
			"fread(): Argument #2 ($length) must be greater than 0")
	}
	buf := make([]byte, int(length))
	n, err := file.Read(buf)
	if err != nil && n == 0 {
		if err == io.EOF {
			return phpv.ZString("").ZVal(), nil
		}
		ctx.Notice("fread(): Read of %d bytes failed with errno=9 Bad file descriptor", length, logopt.NoFuncName(true))
		return phpv.ZFalse.ZVal(), nil
	}
	return phpv.ZString(buf[:n]).ZVal(), nil
}

// > func bool feof ( resource $handle )
func fncFeof(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var handle phpv.Resource
	_, err := core.Expand(ctx, args, &handle)
	if err != nil {
		return nil, err
	}
	if handle == nil {
		return phpv.ZTrue.ZVal(), nil
	}
	if handle.GetResourceType() == phpv.ResourceUnknown {
		return nil, phpobj.ThrowError(ctx, phpobj.TypeError, "feof(): Argument #1 ($stream) must be an open stream resource")
	}

	var file *stream.Stream
	if handle.GetResourceType() == phpv.ResourceStream {
		file, _ = handle.(*stream.Stream)
	}
	if file == nil {
		return phpv.ZTrue.ZVal(), nil
	}

	return phpv.ZBool(file.Eof()).ZVal(), nil
}

// > func string|false fgetc ( resource $handle )
func fncFgetc(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var handle phpv.Resource
	_, err := core.Expand(ctx, args, &handle)
	if err != nil {
		return nil, err
	}
	if handle == nil {
		return phpv.ZFalse.ZVal(), nil
	}

	var file *stream.Stream
	if handle.GetResourceType() == phpv.ResourceStream {
		file, _ = handle.(*stream.Stream)
	}
	if file == nil {
		return phpv.ZFalse.ZVal(), nil
	}

	b, err := file.ReadByte()
	if err != nil {
		return phpv.ZFalse.ZVal(), nil
	}
	return phpv.ZString([]byte{b}).ZVal(), nil
}

// > func string|false fgets ( resource $handle [, int $length ] )
func fncFgets(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var handle phpv.Resource
	var length *phpv.ZInt
	_, err := core.Expand(ctx, args, &handle, &length)
	if err != nil {
		return nil, err
	}
	if handle == nil {
		return phpv.ZFalse.ZVal(), nil
	}

	var file *stream.Stream
	if handle.GetResourceType() == phpv.ResourceStream {
		file, _ = handle.(*stream.Stream)
	}
	if file == nil {
		return phpv.ZFalse.ZVal(), nil
	}

	maxLen := 1024
	if length != nil && int(*length) > 0 {
		maxLen = int(*length) - 1 // PHP's fgets includes the length-1 limit
	}

	var buf []byte
	for i := 0; i < maxLen; i++ {
		b, err := file.ReadByte()
		if err != nil {
			break
		}
		buf = append(buf, b)
		if b == '\n' {
			break
		}
	}

	if len(buf) == 0 {
		return phpv.ZFalse.ZVal(), nil
	}
	return phpv.ZString(buf).ZVal(), nil
}

// > func int fseek ( resource $handle , int $offset [, int $whence = SEEK_SET ] )
func fncFseek(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var handle phpv.Resource
	var offset phpv.ZInt
	var whence *phpv.ZInt
	_, err := core.Expand(ctx, args, &handle, &offset, &whence)
	if err != nil {
		return nil, err
	}
	if handle == nil {
		return phpv.ZInt(-1).ZVal(), nil
	}
	if handle.GetResourceType() == phpv.ResourceUnknown {
		return nil, phpobj.ThrowError(ctx, phpobj.TypeError, "fseek(): Argument #1 ($stream) must be an open stream resource")
	}

	var file *stream.Stream
	if handle.GetResourceType() == phpv.ResourceStream {
		file, _ = handle.(*stream.Stream)
	}
	if file == nil {
		return phpv.ZInt(-1).ZVal(), nil
	}

	w := io.SeekStart
	if whence != nil {
		switch int(*whence) {
		case 1:
			w = io.SeekCurrent
		case 2:
			w = io.SeekEnd
		}
	}

	_, err = file.Seek(int64(offset), w)
	if err != nil {
		return phpv.ZInt(-1).ZVal(), nil
	}
	return phpv.ZInt(0).ZVal(), nil
}

// > func int|false ftell ( resource $handle )
func fncFtell(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var handle phpv.Resource
	_, err := core.Expand(ctx, args, &handle)
	if err != nil {
		return nil, err
	}
	if handle == nil {
		return phpv.ZFalse.ZVal(), nil
	}
	if handle.GetResourceType() == phpv.ResourceUnknown {
		return nil, phpobj.ThrowError(ctx, phpobj.TypeError, "ftell(): Argument #1 ($stream) must be an open stream resource")
	}

	var file *stream.Stream
	if handle.GetResourceType() == phpv.ResourceStream {
		file, _ = handle.(*stream.Stream)
	}
	if file == nil {
		return phpv.ZFalse.ZVal(), nil
	}

	pos, err := file.Seek(0, io.SeekCurrent)
	if err != nil {
		return phpv.ZFalse.ZVal(), nil
	}
	return phpv.ZInt(pos).ZVal(), nil
}

// > func string get_resource_type ( resource $handle)
func fncGetResourceType(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	// TODO: move to another file
	var handle phpv.Resource
	_, err := core.Expand(ctx, args, &handle)
	if err != nil {
		return nil, err
	}

	rtype := handle.GetResourceType().String()
	return phpv.ZStr(rtype).ZVal(), nil
}

// > func int get_resource_id ( resource $resource )
func fncGetResourceId(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var handle phpv.Resource
	_, err := core.Expand(ctx, args, &handle)
	if err != nil {
		return nil, err
	}
	return phpv.ZInt(handle.GetResourceID()).ZVal(), nil
}

// > func bool ftruncate ( resource $handle , int $size )
func fncFtruncate(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var handle phpv.Resource
	var size phpv.ZInt
	_, err := core.Expand(ctx, args, &handle, &size)
	if err != nil {
		return nil, ctx.FuncError(err)
	}

	if handle.GetResourceType() == phpv.ResourceUnknown {
		return nil, phpobj.ThrowError(ctx, phpobj.TypeError, "ftruncate(): Argument #1 ($stream) must be an open stream resource")
	}

	if size < 0 {
		return nil, phpobj.ThrowError(ctx, phpobj.ValueError,
			"ftruncate(): Argument #2 ($size) must be greater than or equal to 0")
	}

	s, ok := handle.(*stream.Stream)
	if !ok {
		ctx.Warn("resource type not yet supported:" + handle.GetResourceType().String())
		return phpv.ZFalse.ZVal(), nil
	}
	if f := s.UnderlyingFile(); f != nil {
		err = f.Truncate(int64(size))
		if err != nil {
			return phpv.ZFalse.ZVal(), nil
		}
		return phpv.ZTrue.ZVal(), nil
	}
	var filename string
	if f, ok := s.Attr("uri").(string); ok {
		filename = f
	}
	if filename == "" {
		ctx.Warn("resource type not yet supported:" + handle.GetResourceType().String())
		return phpv.ZFalse.ZVal(), nil
	}
	err = os.Truncate(filename, int64(size))
	if err != nil {
		return phpv.ZFalse.ZVal(), nil
	}
	return phpv.ZTrue.ZVal(), nil
}

// > func bool fflush ( resource $handle )
func fncFflush(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var handle phpv.Resource
	_, err := core.Expand(ctx, args, &handle)
	if err != nil {
		return nil, ctx.FuncError(err)
	}

	if handle == nil {
		return phpv.ZFalse.ZVal(), nil
	}

	var file *stream.Stream
	if handle.GetResourceType() == phpv.ResourceStream {
		file, _ = handle.(*stream.Stream)
	}
	if file == nil {
		return phpv.ZFalse.ZVal(), nil
	}

	err = file.Flush()
	if err != nil {
		return phpv.ZFalse.ZVal(), nil
	}
	return phpv.ZTrue.ZVal(), nil
}

// > func bool rewind ( resource $handle)
func fncRewind(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var handle phpv.Resource
	_, err := core.Expand(ctx, args, &handle)
	if err != nil {
		return nil, ctx.FuncError(err)
	}

	s, ok := handle.(*stream.Stream)
	if !ok {
		t := handle.GetResourceType().String()
		return phpv.ZFalse.ZVal(), ctx.Warn("resource type not yet supported:" + t)
	}

	s.Seek(0, 0)
	return phpv.ZTrue.ZVal(), nil
}

// > func string stream_get_contents ( resource $handle [, int $maxlength = -1 [, int $offset = -1 ]] )
func fncStreamGetContents(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var handle phpv.Resource
	var maxLen core.Optional[phpv.ZInt]
	var offset core.Optional[phpv.ZInt]
	_, err := core.Expand(ctx, args, &handle, &maxLen, &offset)
	if err != nil {
		return nil, ctx.FuncError(err)
	}

	file, ok := handle.(*stream.Stream)
	if !ok {
		t := handle.GetResourceType().String()
		return phpv.ZFalse.ZVal(), ctx.Warn("resource type not yet supported:" + t)
	}

	if offset.HasArg() && offset.Get() >= 0 {
		file.Seek(int64(offset.Get()), 0)
	}

	var contents []byte
	if !maxLen.HasArg() || maxLen.Get() < 0 {
		contents, err = io.ReadAll(file)
	} else {
		contents = make([]byte, maxLen.Get())
		var n int
		n, err = io.ReadFull(file, contents)
		contents = contents[:n]
	}
	if err != nil && err != io.EOF && err != io.ErrUnexpectedEOF {
		return nil, ctx.FuncError(err)
	}

	return phpv.ZStr(string(contents)), nil
}

// > func bool rename ( string $oldname , string $newname [, resource $context ] )
func fncRename(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var oldNameArg, newNameArg phpv.ZString
	var contextResource core.Optional[phpv.Resource]
	_, err := core.Expand(ctx, args, &oldNameArg, &newNameArg, &contextResource)
	if err != nil {
		return nil, ctx.FuncError(err)
	}

	if err := ctx.Global().CheckOpenBasedir(ctx, string(oldNameArg), "rename"); err != nil {
		return phpv.ZFalse.ZVal(), nil
	}
	if err := ctx.Global().CheckOpenBasedir(ctx, string(newNameArg), "rename"); err != nil {
		return phpv.ZFalse.ZVal(), nil
	}

	oldName := resolveFilePath(ctx, string(oldNameArg))
	newName := resolveFilePath(ctx, string(newNameArg))

	oldStat, err := os.Stat(oldName)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, ctx.Warn("%s(%s,%s): No such file or directory",
				ctx.GetFuncName(), oldNameArg, newNameArg, logopt.NoFuncName(true))
		}
		return nil, ctx.FuncError(err)
	}

	newStat, err := os.Stat(newName)
	if err != nil && !os.IsNotExist(err) {
		return nil, ctx.FuncError(err)
	}

	if os.IsExist(err) {
		if oldStat.IsDir() && newStat.IsDir() {
			files, err := os.ReadDir(newName)
			if err != nil {
				return nil, ctx.FuncError(err)
			}
			if len(files) > 0 {
				return nil, ctx.Warn("%s(%s,%s): Directory not empty",
					ctx.GetFuncName(), oldNameArg, newNameArg, logopt.NoFuncName(true))
			}
		}
		if !oldStat.IsDir() && newStat.IsDir() {
			return nil, ctx.Warn("%s(%s,%s): Is a directory",
				ctx.GetFuncName(), oldNameArg, newNameArg, logopt.NoFuncName(true))
		}
		if oldStat.IsDir() && !newStat.IsDir() {
			return nil, ctx.Warn("%s(%s,%s): Not a directory",
				ctx.GetFuncName(), oldNameArg, newNameArg, logopt.NoFuncName(true))
		}
	}

	err = os.Rename(oldName, newName)
	if err != nil {
		return nil, ctx.FuncError(err)
	}

	return phpv.ZTrue.ZVal(), nil
}

// > func string sys_get_temp_dir ( void )
func fncSysGetTempDir(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	defaultDir := phpv.ZStr(os.TempDir())
	return ctx.GetConfig("sys_temp_dir", defaultDir.ZVal()), nil
}

// > func bool chmod ( string $filename , int $mode )
func fncChmod(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var filename phpv.ZString
	var mode phpv.ZInt
	_, err := core.Expand(ctx, args, &filename, &mode)
	if err != nil {
		return nil, err
	}

	if err := ctx.Global().CheckOpenBasedir(ctx, string(filename), "chmod"); err != nil {
		return phpv.ZFalse.ZVal(), nil
	}

	p := string(filename)
	if !filepath.IsAbs(p) {
		p = filepath.Join(string(ctx.Global().Getwd()), p)
	}

	err = os.Chmod(p, os.FileMode(mode))
	if err != nil {
		ctx.Warn("chmod(): %s", err)
		return phpv.ZFalse.ZVal(), nil
	}
	return phpv.ZTrue.ZVal(), nil
}

// > func bool copy ( string $source , string $dest )
func fncCopy(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var src, dst phpv.ZString
	_, err := core.Expand(ctx, args, &src, &dst)
	if err != nil {
		return nil, err
	}

	if err := ctx.Global().CheckOpenBasedir(ctx, string(src), "copy"); err != nil {
		return phpv.ZFalse.ZVal(), nil
	}
	if err := ctx.Global().CheckOpenBasedir(ctx, string(dst), "copy"); err != nil {
		ctx.Warn("copy(%s): Failed to open stream: Operation not permitted", dst, logopt.NoFuncName(true))
		return phpv.ZFalse.ZVal(), nil
	}

	srcPath := string(src)
	if !filepath.IsAbs(srcPath) {
		srcPath = filepath.Join(string(ctx.Global().Getwd()), srcPath)
	}
	dstPath := string(dst)
	if !filepath.IsAbs(dstPath) {
		dstPath = filepath.Join(string(ctx.Global().Getwd()), dstPath)
	}

	in, err := os.Open(srcPath)
	if err != nil {
		return phpv.ZFalse.ZVal(), ctx.Warn("copy(%s): Failed to open stream: %s", src, err)
	}
	defer in.Close()

	out, err := os.Create(dstPath)
	if err != nil {
		return phpv.ZFalse.ZVal(), ctx.Warn("copy(%s,%s): Failed to open stream: %s", src, dst, err)
	}
	defer out.Close()

	_, err = io.Copy(out, in)
	if err != nil {
		return phpv.ZFalse.ZVal(), nil
	}
	return phpv.ZTrue.ZVal(), nil
}

// > func bool symlink ( string $target , string $link )
func fncSymlink(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var target, link string
	_, err := core.Expand(ctx, args, &target, &link)
	if err != nil {
		return nil, err
	}

	// symlink() resolves paths before basedir check (PHP shows absolute paths in warnings)
	// Check link (dest) first, then target (source), matching PHP's order
	resolvedLink := resolveFilePath(ctx, link)
	resolvedTarget := resolveFilePath(ctx, target)

	if err := ctx.Global().CheckOpenBasedir(ctx, resolvedLink, "symlink"); err != nil {
		return phpv.ZFalse.ZVal(), nil
	}
	if err := ctx.Global().CheckOpenBasedir(ctx, resolvedTarget, "symlink"); err != nil {
		return phpv.ZFalse.ZVal(), nil
	}

	// Create symlink with original target (symlink targets are relative to symlink location)
	err = os.Symlink(target, resolvedLink)
	if err != nil {
		return phpv.ZFalse.ZVal(), ctx.Warn("%s(): %s", ctx.GetFuncName(), err.Error(), logopt.NoFuncName(true))
	}
	return phpv.ZTrue.ZVal(), nil
}

// > func string readlink ( string $path )
func fncReadlink(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var p string
	_, err := core.Expand(ctx, args, &p)
	if err != nil {
		return nil, err
	}

	if err := ctx.Global().CheckOpenBasedir(ctx, p, "readlink"); err != nil {
		return phpv.ZFalse.ZVal(), nil
	}

	p = resolveFilePath(ctx, p)
	target, err := os.Readlink(p)
	if err != nil {
		return phpv.ZFalse.ZVal(), ctx.Warn("%s(): %s", ctx.GetFuncName(), err.Error(), logopt.NoFuncName(true))
	}
	return phpv.ZString(target).ZVal(), nil
}

// > func int linkinfo ( string $path )
func fncLinkinfo(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var p string
	_, err := core.Expand(ctx, args, &p)
	if err != nil {
		return nil, err
	}

	if err := ctx.Global().CheckOpenBasedir(ctx, p, "linkinfo"); err != nil {
		return phpv.ZInt(-1).ZVal(), nil
	}

	p = resolveFilePath(ctx, p)
	fi, err := os.Lstat(p)
	if err != nil {
		return phpv.ZInt(-1).ZVal(), ctx.Warn("%s(): %s", ctx.GetFuncName(), err.Error(), logopt.NoFuncName(true))
	}

	return phpv.ZInt(int64(fi.Mode())).ZVal(), nil
}

// > func bool is_uploaded_file ( string $filename )
func fncIsUploadedFile(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var filename phpv.ZString
	_, err := core.Expand(ctx, args, &filename)
	if err != nil {
		return nil, err
	}

	if ctx.Global().IsUploadedFile(string(filename)) {
		return phpv.ZTrue.ZVal(), nil
	}
	return phpv.ZFalse.ZVal(), nil
}

// > func bool move_uploaded_file ( string $from, string $to )
func fncMoveUploadedFile(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var from, to phpv.ZString
	_, err := core.Expand(ctx, args, &from, &to)
	if err != nil {
		return nil, err
	}

	if !ctx.Global().IsUploadedFile(string(from)) {
		return phpv.ZFalse.ZVal(), nil
	}

	err = os.Rename(string(from), string(to))
	if err != nil {
		return phpv.ZFalse.ZVal(), nil
	}

	ctx.Global().UnregisterUploadedFile(string(from))
	return phpv.ZTrue.ZVal(), nil
}

// > func int umask ([ int $mask ] )
func fncUmask(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var maskArg *phpv.ZInt
	_, err := core.Expand(ctx, args, &maskArg)
	if err != nil {
		return nil, err
	}

	if maskArg == nil {
		// Query current umask: set to 0, read old value, restore
		old := syscall.Umask(0)
		syscall.Umask(old)
		return phpv.ZInt(old).ZVal(), nil
	}

	old := syscall.Umask(int(*maskArg))
	return phpv.ZInt(old).ZVal(), nil
}

// > func array|false fstat ( resource $stream )
func fncFstat(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var handle phpv.Resource
	_, err := core.Expand(ctx, args, &handle)
	if err != nil {
		return nil, err
	}
	if handle == nil {
		return phpv.ZFalse.ZVal(), nil
	}

	var file *stream.Stream
	if handle.GetResourceType() == phpv.ResourceStream {
		file, _ = handle.(*stream.Stream)
	}
	if file == nil {
		return phpv.ZFalse.ZVal(), nil
	}

	f := file.UnderlyingFile()
	if f == nil {
		return phpv.ZFalse.ZVal(), nil
	}

	info, statErr := f.Stat()
	if statErr != nil {
		return phpv.ZFalse.ZVal(), nil
	}

	// Build stat array (same format as stat())
	result := phpv.NewZArray()
	sys := info.Sys()
	if sysstat, ok := sys.(*syscall.Stat_t); ok {
		result.OffsetSet(ctx, phpv.ZInt(0).ZVal(), phpv.ZInt(sysstat.Dev).ZVal())
		result.OffsetSet(ctx, phpv.ZString("dev").ZVal(), phpv.ZInt(sysstat.Dev).ZVal())
		result.OffsetSet(ctx, phpv.ZInt(1).ZVal(), phpv.ZInt(sysstat.Ino).ZVal())
		result.OffsetSet(ctx, phpv.ZString("ino").ZVal(), phpv.ZInt(sysstat.Ino).ZVal())
		result.OffsetSet(ctx, phpv.ZInt(2).ZVal(), phpv.ZInt(sysstat.Mode).ZVal())
		result.OffsetSet(ctx, phpv.ZString("mode").ZVal(), phpv.ZInt(sysstat.Mode).ZVal())
		result.OffsetSet(ctx, phpv.ZInt(3).ZVal(), phpv.ZInt(sysstat.Nlink).ZVal())
		result.OffsetSet(ctx, phpv.ZString("nlink").ZVal(), phpv.ZInt(sysstat.Nlink).ZVal())
		result.OffsetSet(ctx, phpv.ZInt(4).ZVal(), phpv.ZInt(sysstat.Uid).ZVal())
		result.OffsetSet(ctx, phpv.ZString("uid").ZVal(), phpv.ZInt(sysstat.Uid).ZVal())
		result.OffsetSet(ctx, phpv.ZInt(5).ZVal(), phpv.ZInt(sysstat.Gid).ZVal())
		result.OffsetSet(ctx, phpv.ZString("gid").ZVal(), phpv.ZInt(sysstat.Gid).ZVal())
		result.OffsetSet(ctx, phpv.ZInt(6).ZVal(), phpv.ZInt(sysstat.Rdev).ZVal())
		result.OffsetSet(ctx, phpv.ZString("rdev").ZVal(), phpv.ZInt(sysstat.Rdev).ZVal())
		result.OffsetSet(ctx, phpv.ZInt(7).ZVal(), phpv.ZInt(sysstat.Size).ZVal())
		result.OffsetSet(ctx, phpv.ZString("size").ZVal(), phpv.ZInt(sysstat.Size).ZVal())
		result.OffsetSet(ctx, phpv.ZInt(8).ZVal(), phpv.ZInt(sysstat.Atim.Sec).ZVal())
		result.OffsetSet(ctx, phpv.ZString("atime").ZVal(), phpv.ZInt(sysstat.Atim.Sec).ZVal())
		result.OffsetSet(ctx, phpv.ZInt(9).ZVal(), phpv.ZInt(sysstat.Mtim.Sec).ZVal())
		result.OffsetSet(ctx, phpv.ZString("mtime").ZVal(), phpv.ZInt(sysstat.Mtim.Sec).ZVal())
		result.OffsetSet(ctx, phpv.ZInt(10).ZVal(), phpv.ZInt(sysstat.Ctim.Sec).ZVal())
		result.OffsetSet(ctx, phpv.ZString("ctime").ZVal(), phpv.ZInt(sysstat.Ctim.Sec).ZVal())
		result.OffsetSet(ctx, phpv.ZInt(11).ZVal(), phpv.ZInt(sysstat.Blksize).ZVal())
		result.OffsetSet(ctx, phpv.ZString("blksize").ZVal(), phpv.ZInt(sysstat.Blksize).ZVal())
		result.OffsetSet(ctx, phpv.ZInt(12).ZVal(), phpv.ZInt(sysstat.Blocks).ZVal())
		result.OffsetSet(ctx, phpv.ZString("blocks").ZVal(), phpv.ZInt(sysstat.Blocks).ZVal())
	}
	return result.ZVal(), nil
}

// > func bool chown ( string $filename , string|int $user )
func fncChown(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var filename phpv.ZString
	var user *phpv.ZVal
	_, err := core.Expand(ctx, args, &filename, &user)
	if err != nil {
		return nil, err
	}

	p := resolveFilePath(ctx, string(filename))
	uid := int(user.AsInt(ctx))

	if err := os.Chown(p, uid, -1); err != nil {
		ctx.Warn("Operation not permitted")
		return phpv.ZFalse.ZVal(), nil
	}
	return phpv.ZTrue.ZVal(), nil
}

// > func bool chgrp ( string $filename , string|int $group )
func fncChgrp(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var filename phpv.ZString
	var group *phpv.ZVal
	_, err := core.Expand(ctx, args, &filename, &group)
	if err != nil {
		return nil, err
	}

	p := resolveFilePath(ctx, string(filename))
	gid := int(group.AsInt(ctx))

	if err := os.Chown(p, -1, gid); err != nil {
		ctx.Warn("Operation not permitted")
		return phpv.ZFalse.ZVal(), nil
	}
	return phpv.ZTrue.ZVal(), nil
}

// > func int|false getlastmod ( void )
// Returns the last modification time of the main script
func fncGetlastmod(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	scriptFile := ctx.GetScriptFile()
	if scriptFile == "" {
		return phpv.ZFalse.ZVal(), nil
	}
	info, err := os.Stat(string(scriptFile))
	if err != nil {
		return phpv.ZFalse.ZVal(), nil
	}
	return phpv.ZInt(info.ModTime().Unix()).ZVal(), nil
}

// > func string get_current_user ( void )
func fncGetCurrentUser(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	username := os.Getenv("USER")
	if username == "" {
		username = os.Getenv("LOGNAME")
	}
	if username == "" {
		username = "nobody"
	}
	return phpv.ZString(username).ZVal(), nil
}

// > func void|false passthru ( string $command [, int &$result_code ] )
func fncPassthru(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var cmdStr string
	var returnVar core.OptionalRef[phpv.ZInt]
	_, err := core.Expand(ctx, args, &cmdStr, &returnVar)
	if err != nil {
		return nil, err
	}

	cmdStr = strings.TrimSpace(cmdStr)
	if cmdStr == "" {
		return nil, nil
	}

	cmd := exec.Command("/bin/sh", "-c", cmdStr)
	out, runErr := cmd.CombinedOutput()

	exitCode := 0
	if runErr != nil {
		if exitErr, ok := runErr.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		}
	} else if cmd.ProcessState != nil {
		exitCode = cmd.ProcessState.ExitCode()
	}

	// passthru writes output directly to stdout
	ctx.Write(out)

	if returnVar.HasArg() {
		returnVar.Set(ctx, phpv.ZInt(exitCode))
	}

	return nil, nil
}

// > func resource|false popen ( string $command , string $mode )
func fncPopen(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var cmdStr, mode string
	_, err := core.Expand(ctx, args, &cmdStr, &mode)
	if err != nil {
		return nil, err
	}

	if mode == "r" {
		cmd := exec.Command("/bin/sh", "-c", cmdStr)
		out, runErr := cmd.Output()
		if runErr != nil {
			if _, ok := runErr.(*exec.ExitError); !ok {
				return phpv.ZFalse.ZVal(), nil
			}
		}
		r := strings.NewReader(string(out))
		s := stream.NewStream(r)
		s.ResourceType = phpv.ResourceStream
		return s.ZVal(), nil
	}

	return phpv.ZFalse.ZVal(), nil
}

// > func int pclose ( resource $handle )
func fncPclose(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var handle phpv.Resource
	_, err := core.Expand(ctx, args, &handle)
	if err != nil {
		return nil, err
	}
	if handle == nil {
		return phpv.ZInt(-1).ZVal(), nil
	}

	s, ok := handle.(*stream.Stream)
	if ok {
		s.Close()
	}
	return phpv.ZInt(0).ZVal(), nil
}

