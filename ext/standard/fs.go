package standard

import (
	"errors"
	"io"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"

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
		if len(p) == 1 {
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

	for i := phpv.ZInt(0); i < *lvl; i++ {
		p = path.Dir(p)
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

	result := filepath.Base(path)
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

	r, err := ctx.Global().Open(filename, "r", true)
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

	r, err := ctx.Global().Open(filename, "r", true)
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

// > func string realpath ( string $filename )
func fncRealPath(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var filename string
	_, err := core.Expand(ctx, args, &filename)
	if err != nil {
		return nil, err
	}

	if regexp.MustCompile(`$\w+:\/\/`).MatchString(filename) {
		return phpv.ZFalse.ZVal(), nil
	}

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
	var context **phpv.ZVal
	_, err := core.Expand(ctx, args, &filename, &context)
	if err != nil {
		return nil, err
	}

	if context != nil {
		return nil, ctx.Errorf("context resource is not yet supported, must be NULL")
	}

	stat, err := os.Stat(filename)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			err = ctx.Warn("No such file or directory")
		} else {
			err = ctx.Warn(err.Error())
		}
		return phpv.ZFalse.ZVal(), err
	}
	if stat.IsDir() {
		return phpv.ZFalse.ZVal(), ctx.Warn("Is a directory")
	}

	if err := os.Remove(filename); err != nil {
		return phpv.ZFalse.ZVal(), ctx.Warn(err.Error())
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
	var contextResource **phpv.ZVal
	var offsetArg *phpv.ZInt
	var maxlen *phpv.ZInt
	_, err := core.Expand(ctx, args, &filename, &useIncludePathArg, &contextResource, &offsetArg, &maxlen)
	if err != nil {
		return nil, err
	}

	if useIncludePathArg != nil && *useIncludePathArg {
		// TODO: handle use_include_path
		return nil, errors.New("use_include_path is not yet supported, set to false")
	}

	if contextResource != nil && !phpv.IsNull(*contextResource) {
		return nil, errors.New("context resource is not yet supported, set to NULL")
	}

	if maxlen != nil && *maxlen < 0 {
		return nil, errors.New("Argument #5 ($length) must be greater than or equal to 0")
	}

	f, err := ctx.Global().Open(filename, "r", true)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			// TODO: WARN Failed to open stream: No such file or directory
			return phpv.ZFalse.ZVal(), nil
		}
		return nil, err
	}
	defer f.Close()

	if offsetArg != nil {
		off := *offsetArg
		if off < 0 {
			_, err = f.Seek(int64(*offsetArg), io.SeekEnd)
		} else {
			_, err = f.Seek(int64(*offsetArg), io.SeekStart)
		}
		if err != nil {
			return nil, err
		}
	}

	if maxlen == nil {
		buf, err := io.ReadAll(f)
		if err != nil {
			return nil, err
		}
		return phpv.ZStr(string(buf)), nil
	}

	buf := make([]byte, *maxlen)
	_, err = f.Read(buf)
	if err != nil && err != io.EOF {
		return nil, err
	}

	return phpv.ZStr(string(buf)), nil
}

// > func int file_put_contents ( string $filename , mixed $data [, int $flags = 0 [, resource $context ]]
func fncFilePutContents(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var filename phpv.ZString
	var data *phpv.ZVal
	var flagsArg *phpv.ZInt
	var resource **phpv.ZVal
	_, err := core.Expand(ctx, args, &filename, &data, &flagsArg, &resource)
	if err != nil {
		return nil, err
	}

	if resource != nil {
		return nil, errors.New("context resource is not yet supported, set to NULL")
	}

	flags := os.O_CREATE | os.O_WRONLY | os.O_TRUNC
	if flagsArg != nil {
		// TODO: handle LOCK_EX and FILE_USE_INCLUDE_PATH flags
		if (*flagsArg & FILE_APPEND) != 0 {
			flags |= os.O_APPEND
			flags &= ^os.O_TRUNC
		}
	}

	// TODO: should use ctx.Global().Open()
	r, err := os.OpenFile(string(filename), flags, 0644)
	if err != nil {
		return nil, err
	}
	defer r.Close()

	written := 0
	switch data.GetType() {
	case phpv.ZtResource:
		return nil, errors.New("data resource is not yet supported")

	case phpv.ZtArray:
		array, err := data.As(ctx, phpv.ZtArray)
		if err != nil {
			return nil, err
		}
		output, err := fncStrImplode(ctx, []*phpv.ZVal{
			phpv.ZStr(","),
			array,
		})
		if err != nil {
			return nil, err
		}
		if written, err = r.Write([]byte(output.String())); err != nil {
			return nil, err
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
	var contextResource core.OptionalRef[*phpv.ZVal]
	_, err := core.Expand(ctx, args, &filename, &mode, &useIncludePathArg, &contextResource)
	if err != nil {
		return nil, err
	}

	if useIncludePathArg.HasArg() {
		// TODO: handle use_include_path
		return nil, ctx.FuncErrorf("use_include_path is not yet supported, set to false")
	}

	if contextResource.Get() != nil {
		return nil, ctx.FuncErrorf("context resource is not yet supported, set to NULL")
	}

	f, err := ctx.Global().Open(filename, mode, true)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return phpv.ZFalse.ZVal(), ctx.Warn("%s(%s): failed to open stream: No such file or directory", ctx.GetFuncName(), filename)
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

	var file phpv.Stream
	switch handle.GetResourceType() {
	case phpv.ResourceStream:
		if f, ok := handle.(*stream.Stream); ok {
			file = f
		}
	}
	if file == nil {
		return nil, ctx.Errorf("cannot close resource, not a file")
	}

	err = file.Close()
	if err != nil {
		return nil, ctx.Error(err)
	}

	return nil, nil
}

// TODO: fread, fwrite, fgets, fstat, fseek

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

// > func bool ftruncate ( resource $handle , int $size )
func fncFtruncate(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var handle phpv.Resource
	var size phpv.ZInt
	_, err := core.Expand(ctx, args, &handle, &size)
	if err != nil {
		return nil, ctx.FuncError(err)
	}

	var filename string
	if s, ok := handle.(*stream.Stream); ok {
		if f, ok := s.Attr("uri").(string); ok {
			filename = f
		}
	}

	if filename == "" {
		ctx.Warn("resource type not yet supported:" + handle.GetResourceType().String())
		return phpv.ZFalse.ZVal(), nil
	}

	os.Truncate(filename, int64(size))
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

	if offset.HasArg() && offset.Get() > 0 {
		file.Seek(int64(offset.Get()), 0)
	}

	var contents []byte
	if !maxLen.HasArg() {
		contents, err = io.ReadAll(file)
	} else {
		contents = make([]byte, maxLen.Get())
		_, err = io.ReadFull(file, contents)
	}
	if err != nil {
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

	oldName := string(oldNameArg)
	newName := string(newNameArg)

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
