package stream

import (
	"io"
	"os"
	"time"

	"github.com/MagicalTux/goro/core/phpobj"
	"github.com/MagicalTux/goro/core/phpv"
)

// Seek implements io.Seeker for user stream wrappers.
func (u *UserStream) Seek(offset int64, whence int) (int64, error) {
	if u.obj == nil {
		return 0, ErrNotSupported
	}
	if offset == 0 && whence == io.SeekCurrent {
		if _, ok := u.obj.Class.GetMethod("stream_tell"); ok {
			pos, err := u.obj.CallMethod(u.ctx, "stream_tell")
			if err == nil && pos != nil {
				return int64(pos.AsInt(u.ctx)), nil
			}
		}
		return 0, nil
	}
	if _, ok := u.obj.Class.GetMethod("stream_seek"); !ok {
		return 0, ErrNotSupported
	}
	result, err := u.obj.CallMethod(u.ctx, "stream_seek",
		phpv.ZInt(offset).ZVal(), phpv.ZInt(whence).ZVal())
	if err != nil {
		return 0, err
	}
	if result != nil && result.AsBool(u.ctx) {
		if _, ok := u.obj.Class.GetMethod("stream_tell"); ok {
			pos, err := u.obj.CallMethod(u.ctx, "stream_tell")
			if err == nil && pos != nil {
				return int64(pos.AsInt(u.ctx)), nil
			}
		}
		return 0, nil
	}
	return 0, ErrNotSupported
}

// Stat implements Stater for fstat().
func (u *UserStream) Stat() (os.FileInfo, error) {
	if u.obj == nil {
		return nil, ErrNotSupported
	}
	if _, ok := u.obj.Class.GetMethod("stream_stat"); !ok {
		return nil, ErrNotSupported
	}
	result, err := u.obj.CallMethod(u.ctx, "stream_stat")
	if err != nil {
		return nil, err
	}
	if result == nil || result.GetType() == phpv.ZtBool {
		return nil, ErrNotSupported
	}
	return UserStatToFileInfo(u.ctx, result), nil
}

// Truncate implements Truncater.
func (u *UserStream) Truncate(size int64) error {
	if u.obj == nil {
		return ErrNotSupported
	}
	if _, ok := u.obj.Class.GetMethod("stream_truncate"); !ok {
		return ErrNotSupported
	}
	result, err := u.obj.CallMethod(u.ctx, "stream_truncate", phpv.ZInt(size).ZVal())
	if err != nil {
		return err
	}
	if result != nil && result.AsBool(u.ctx) {
		return nil
	}
	return ErrNotSupported
}

// CreateObject creates a new instance of the wrapper class.
func (h *UserStreamHandler) CreateObject(ctx phpv.Context) (*phpobj.ZObject, error) {
	class, err := ctx.Global().GetClass(ctx, h.ClassName, true)
	if err != nil {
		return nil, err
	}
	return phpobj.NewZObject(ctx, class)
}

// UrlStat calls url_stat on the wrapper class.
func (h *UserStreamHandler) UrlStat(ctx phpv.Context, path string, flags int) (os.FileInfo, error) {
	obj, err := h.CreateObject(ctx)
	if err != nil {
		return nil, err
	}
	result, err := obj.CallMethod(ctx, "url_stat",
		phpv.ZString(path).ZVal(), phpv.ZInt(flags).ZVal())
	if err != nil {
		return nil, err
	}
	if result == nil || result.GetType() == phpv.ZtBool {
		return nil, os.ErrNotExist
	}
	return UserStatToFileInfo(ctx, result), nil
}

// Unlink calls the unlink method on the wrapper.
func (h *UserStreamHandler) Unlink(ctx phpv.Context, path string) error {
	obj, err := h.CreateObject(ctx)
	if err != nil {
		return err
	}
	_, err = obj.CallMethod(ctx, "unlink", phpv.ZString(path).ZVal())
	return err
}

// Rename calls the rename method on the wrapper.
func (h *UserStreamHandler) Rename(ctx phpv.Context, from, to string) error {
	obj, err := h.CreateObject(ctx)
	if err != nil {
		return err
	}
	_, err = obj.CallMethod(ctx, "rename",
		phpv.ZString(from).ZVal(), phpv.ZString(to).ZVal())
	return err
}

// Mkdir calls the mkdir method on the wrapper.
func (h *UserStreamHandler) Mkdir(ctx phpv.Context, path string, mode int, options int) error {
	obj, err := h.CreateObject(ctx)
	if err != nil {
		return err
	}
	_, err = obj.CallMethod(ctx, "mkdir",
		phpv.ZString(path).ZVal(), phpv.ZInt(mode).ZVal(), phpv.ZInt(options).ZVal())
	return err
}

// Rmdir calls the rmdir method on the wrapper.
func (h *UserStreamHandler) Rmdir(ctx phpv.Context, path string, options int) error {
	obj, err := h.CreateObject(ctx)
	if err != nil {
		return err
	}
	_, err = obj.CallMethod(ctx, "rmdir",
		phpv.ZString(path).ZVal(), phpv.ZInt(options).ZVal())
	return err
}

type userStatInfo struct {
	name  string
	size  int64
	mode  os.FileMode
	mtime time.Time
	isDir bool
}

func (si *userStatInfo) Name() string       { return si.name }
func (si *userStatInfo) Size() int64        { return si.size }
func (si *userStatInfo) Mode() os.FileMode  { return si.mode }
func (si *userStatInfo) ModTime() time.Time { return si.mtime }
func (si *userStatInfo) IsDir() bool        { return si.isDir }
func (si *userStatInfo) Sys() interface{}   { return nil }

// UserStatToFileInfo converts a PHP stat array to os.FileInfo.
func UserStatToFileInfo(ctx phpv.Context, result *phpv.ZVal) os.FileInfo {
	si := &userStatInfo{}
	if result.GetType() == phpv.ZtArray {
		arr := result.AsArray(ctx)
		if v, err := arr.OffsetGet(ctx, phpv.ZStr("size")); err == nil && v != nil && v.GetType() != phpv.ZtNull {
			si.size = int64(v.AsInt(ctx))
		}
		if v, err := arr.OffsetGet(ctx, phpv.ZStr("mode")); err == nil && v != nil && v.GetType() != phpv.ZtNull {
			si.mode = os.FileMode(v.AsInt(ctx))
			si.isDir = si.mode&os.ModeDir != 0
		}
		if v, err := arr.OffsetGet(ctx, phpv.ZStr("mtime")); err == nil && v != nil && v.GetType() != phpv.ZtNull {
			si.mtime = time.Unix(int64(v.AsInt(ctx)), 0)
		}
	}
	return si
}
