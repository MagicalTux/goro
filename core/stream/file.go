package stream

import (
	"net/url"
	"os"
	"path"
	"path/filepath"
	"syscall"
)

type fileHandler struct {
	cwd  string
	root string
}

func NewFileHandler(root string) (Handler, error) {
	// make sure root is absolute
	root, err := filepath.Abs(root)
	if err != nil {
		return nil, err
	}
	root, err = filepath.EvalSymlinks(root)
	if err != nil {
		return nil, err
	}

	if root[len(root)-1] != filepath.Separator {
		root = root + string([]byte{filepath.Separator})
	}

	fh := &fileHandler{
		root: root,
		cwd:  "/",
	}

	// try to get current working directory if within root
	wd, err := os.Getwd()
	if err != nil {
		return fh, nil // ignore error
	}

	wd, err = filepath.EvalSymlinks(wd)
	if err != nil {
		return fh, nil // ignore error
	}

	if len(wd) >= len(root) && wd[:len(root)] == root {
		localwd := filepath.Join("/", filepath.ToSlash(wd[len(root):]))
		localwd = filepath.Clean(localwd)
		fh.cwd = localwd
	}

	return fh, nil
}

func (f *fileHandler) localPath(name string) (string, string, error) {
	if !path.IsAbs(name) {
		name = path.Join(f.cwd, name)
	}
	name = path.Clean(name)

	// go to fname
	fname := filepath.Join(f.root, filepath.FromSlash(name))

	// resolve symlinks
	fname, err := filepath.EvalSymlinks(fname)
	if err != nil {
		return "", "", err
	}

	// check if OK
	if fname[:len(f.root)] != f.root {
		// not ok
		return "", "", os.ErrNotExist
	}

	return fname, name, nil
}

func (f *fileHandler) Open(p *url.URL) (*Stream, error) {
	fname, name, err := f.localPath(p.Path)
	if err != nil {
		return nil, err
	}

	res, err := os.Open(fname)
	if err != nil {
		return nil, err
	}

	s := NewStream(res)
	s.SetAttr("wrapper_type", "plainfile")
	s.SetAttr("stream_type", "Go")
	s.SetAttr("mode", "r")
	s.SetAttr("seekable", true)
	s.SetAttr("uri", name)

	return s, nil
}

func (f *fileHandler) Chdir(p string) error {
	fname, name, err := f.localPath(p)
	if err != nil {
		return err
	}

	s, err := os.Lstat(fname)
	if err != nil {
		return err
	}

	if !s.IsDir() {
		return &os.PathError{"chdir", p, syscall.ENOTDIR}
	}

	f.cwd = name
	return nil
}

func (f *fileHandler) Getwd() string {
	return f.cwd
}
