package standard

import (
	"os"

	"github.com/MagicalTux/goro/core"
	"github.com/MagicalTux/goro/core/phpv"
)

// > const
const (
	SCANDIR_SORT_ASCENDING phpv.ZInt = iota
	SCANDIR_SORT_DESCENDING
	SCANDIR_SORT_NONE
)

// > func string getcwd ( void )
func fncGetcwd(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	cwd := ctx.Global().Getwd()
	if cwd == "" {
		return phpv.ZBool(false).ZVal(), nil
	}

	return cwd.ZVal(), nil
}

// > func bool chdir ( string $directory )
func fncChdir(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var p phpv.ZString
	_, err := core.Expand(ctx, args, &p)
	if err != nil {
		return nil, ctx.FuncError(err)
	}

	return nil, ctx.Global().Chdir(p)
}

// > func array scandir ( string $directory [, int $sorting_order = SCANDIR_SORT_ASCENDING [, resource $context ]] )
func fncScanDir(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var dir phpv.ZString
	var sortingOrderArg *phpv.ZInt
	var context **phpv.ZVal // TODO: use context arg
	_, err := core.Expand(ctx, args, &dir, &sortingOrderArg, &context)
	if err != nil {
		return nil, ctx.FuncError(err)
	}

	sortingOrder := core.Deref(sortingOrderArg, SCANDIR_SORT_ASCENDING)

	files, err := os.ReadDir(string(dir))
	if err != nil {
		return nil, ctx.FuncError(err)
	}

	result := phpv.NewZArray()

	switch sortingOrder {
	case SCANDIR_SORT_ASCENDING, SCANDIR_SORT_NONE:
		result.OffsetSet(ctx, nil, phpv.ZStr("."))
		result.OffsetSet(ctx, nil, phpv.ZStr(".."))
		for _, file := range files {
			result.OffsetSet(ctx, nil, phpv.ZStr(file.Name()))
		}
	case SCANDIR_SORT_DESCENDING:
		for _, file := range core.IterateBackwards(files) {
			result.OffsetSet(ctx, nil, phpv.ZStr(file.Name()))
		}
		result.OffsetSet(ctx, nil, phpv.ZStr(".."))
		result.OffsetSet(ctx, nil, phpv.ZStr("."))
	}

	return result.ZVal(), nil
}
