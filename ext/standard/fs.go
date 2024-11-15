package standard

import (
	"errors"
	"io"
	"os"
	"path"

	"github.com/MagicalTux/goro/core"
	"github.com/MagicalTux/goro/core/phpv"
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

// > func bool file_exists ( string $filename )
func fncFileExists(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var filename phpv.ZString
	_, err := core.Expand(ctx, args, &filename)
	if err != nil {
		return nil, err
	}

	r, err := ctx.Global().Exists(filename)
	if err != nil {
		return nil, err
	}

	return phpv.ZBool(r).ZVal(), nil
}

// > func bool is_file ( string $filename )
func fncIsFile(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var filename phpv.ZString
	_, err := core.Expand(ctx, args, &filename)
	if err != nil {
		return nil, err
	}

	r, err := ctx.Global().Open(filename, true)
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

	f, err := ctx.Global().Open(filename, true)
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
