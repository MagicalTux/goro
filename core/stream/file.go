package stream

import "path/filepath"

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

	fh := &fileHandler{
		root: root,
		cwd:  "/",
	}

	return fh
}
