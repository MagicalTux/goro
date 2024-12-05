package standard

import (
	"sort"
	"strings"

	"github.com/MagicalTux/goro/core"
	"github.com/MagicalTux/goro/core/phpv"
)

// > func bool sort ( array &$array [, int $sort_flags = SORT_REGULAR ] )
func fncArraySort(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var array *phpv.ZArray
	var sortFlagsArg *phpv.ZInt
	_, err := core.Expand(ctx, args, &array, &sortFlagsArg)
	if err != nil {
		return nil, ctx.FuncError(err)
	}

	caseInsensitive := false
	sortFlags := SORT_REGULAR

	if sortFlagsArg != nil {
		sortFlags = *sortFlagsArg
		caseInsensitive = sortFlags&SORT_FLAG_CASE != 0
		sortFlags &= ^SORT_FLAG_CASE
	}

	var values []*phpv.ZVal
	for _, v := range (array).Iterate(ctx) {
		values = append(values, v)
	}

	sortBy := zValuesSorter{ctx, values, caseInsensitive}
	sortFn := sortBy.regular

	switch sortFlags {
	case SORT_STRING, SORT_LOCALE_STRING:
		sortFn = sortBy.stringly
	case SORT_NATURAL:
		sortFn = sortBy.naturally
	case SORT_NUMERIC:
		sortFn = sortBy.numerically
	case SORT_REGULAR:
		sortFn = sortBy.regular
	}

	sort.Slice(values, sortFn)

	if err = array.Clear(ctx); err != nil {
		return nil, err
	}

	for _, v := range values {
		array.OffsetSet(ctx, nil, v)
	}

	return phpv.ZTrue.ZVal(), nil
}

// > func bool ksort ( array &$array [, int $sort_flags = SORT_REGULAR ] )
func fncArrayKSort(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var array *phpv.ZArray
	var sortFlagsArg *phpv.ZInt
	_, err := core.Expand(ctx, args, &array, &sortFlagsArg)
	if err != nil {
		return nil, ctx.FuncError(err)
	}

	caseInsensitive := false
	sortFlags := SORT_REGULAR

	if sortFlagsArg != nil {
		sortFlags = *sortFlagsArg
		caseInsensitive = sortFlags&SORT_FLAG_CASE != 0
		sortFlags &= ^SORT_FLAG_CASE
	}

	var keys []*phpv.ZVal
	for k := range (array).Iterate(ctx) {
		keys = append(keys, k)
	}

	sortBy := zValuesSorter{ctx, keys, caseInsensitive}
	sortFn := sortBy.regular

	switch sortFlags {
	case SORT_STRING, SORT_LOCALE_STRING:
		sortFn = sortBy.stringly
	case SORT_NATURAL:
		sortFn = sortBy.naturally
	case SORT_NUMERIC:
		sortFn = sortBy.numerically
	case SORT_REGULAR:
		sortFn = sortBy.regular
	}

	sort.Slice(keys, sortFn)

	for _, k := range keys {
		v, _ := array.OffsetGet(ctx, k)
		array.OffsetUnset(ctx, k)
		array.OffsetSet(ctx, k, v)
	}

	return phpv.ZTrue.ZVal(), nil
}

type zValuesSorter struct {
	ctx             phpv.Context
	values          []*phpv.ZVal
	caseInsensitive bool
}

func (zv zValuesSorter) regular(i, j int) bool {
	cmp, _ := core.Compare(zv.ctx, zv.values[i], zv.values[j])
	return cmp < 0
}

func (zv zValuesSorter) numerically(i, j int) bool {
	a := zv.values[i].AsInt(zv.ctx)
	b := zv.values[j].AsInt(zv.ctx)
	return a < b
}

func (zv zValuesSorter) stringly(i, j int) bool {
	a := string(zv.values[i].AsString(zv.ctx))
	b := string(zv.values[j].AsString(zv.ctx))
	if zv.caseInsensitive {
		a = strings.ToLower(a)
		b = strings.ToLower(b)
	}
	return strings.Compare(a, b) < 0
}

func (zv zValuesSorter) naturally(i, j int) bool {
	a := []byte(zv.values[i].AsString(zv.ctx))
	b := []byte(zv.values[j].AsString(zv.ctx))
	return natCmp(a, b, !zv.caseInsensitive) < 0
}
