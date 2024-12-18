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
	_, err := core.Expand(ctx, args, core.Ref(&array), &sortFlagsArg)
	if err != nil {
		return nil, ctx.FuncError(err)
	}

	var entries []compareEntry
	for _, v := range (array).Iterate(ctx) {
		entries = append(entries, compareEntry{
			item: v,
		})
	}

	arraySort(ctx, entries, sortFlagsArg)

	if err = array.Clear(ctx); err != nil {
		return nil, err
	}

	for _, v := range entries {
		array.OffsetSet(ctx, nil, v.item)
	}

	return phpv.ZTrue.ZVal(), nil
}

// > func bool rsort ( array &$array [, int $sort_flags = SORT_REGULAR ] )
func fncArrayRSort(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var array *phpv.ZArray
	var sortFlagsArg *phpv.ZInt
	_, err := core.Expand(ctx, args, core.Ref(&array), &sortFlagsArg)
	if err != nil {
		return nil, ctx.FuncError(err)
	}

	var entries []compareEntry
	for _, v := range (array).Iterate(ctx) {
		entries = append(entries, compareEntry{
			item: v,
		})
	}

	arraySort(ctx, entries, sortFlagsArg)

	if err = array.Clear(ctx); err != nil {
		return nil, err
	}

	for _, v := range core.IterateBackwards(entries) {
		array.OffsetSet(ctx, nil, v.item)
	}

	return phpv.ZTrue.ZVal(), nil
}

// > func bool usort ( array &$array [, int $sort_flags = SORT_REGULAR ] )
func fncArrayUSort(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var array *phpv.ZArray
	var compareFunc phpv.Callable
	_, err := core.Expand(ctx, args, core.Ref(&array), &compareFunc)
	if err != nil {
		return nil, ctx.FuncError(err)
	}

	var entries []compareEntry
	for _, v := range (array).Iterate(ctx) {
		entries = append(entries, compareEntry{
			item: v,
		})
	}

	err = arrayUSort(ctx, entries, compareFunc)
	if err != nil {
		return nil, ctx.FuncError(err)
	}

	if err = array.Clear(ctx); err != nil {
		return nil, err
	}

	for _, v := range entries {
		array.OffsetSet(ctx, nil, v.item)
	}

	return phpv.ZTrue.ZVal(), nil
}

// > func bool uasort ( array &$array [, int $sort_flags = SORT_REGULAR ] )
func fncArrayUASort(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var array *phpv.ZArray
	var compareFunc phpv.Callable
	_, err := core.Expand(ctx, args, core.Ref(&array), &compareFunc)
	if err != nil {
		return nil, ctx.FuncError(err)
	}

	var entries []compareEntry
	for k, v := range (array).Iterate(ctx) {
		entries = append(entries, compareEntry{
			item: v,
			data: k,
		})
	}

	err = arrayUSort(ctx, entries, compareFunc)
	if err != nil {
		return nil, ctx.FuncError(err)
	}

	for _, entry := range entries {
		k := entry.data
		v := entry.item
		array.OffsetUnset(ctx, k)
		array.OffsetSet(ctx, k, v)
	}

	return phpv.ZTrue.ZVal(), nil
}

// > func bool uksort ( array &$array [, int $sort_flags = SORT_REGULAR ] )
func fncArrayUKSort(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var array *phpv.ZArray
	var compareFunc phpv.Callable
	_, err := core.Expand(ctx, args, core.Ref(&array), &compareFunc)
	if err != nil {
		return nil, ctx.FuncError(err)
	}

	var entries []compareEntry
	for k, v := range (array).Iterate(ctx) {
		entries = append(entries, compareEntry{
			item: k,
			data: v,
		})
	}

	err = arrayUSort(ctx, entries, compareFunc)
	if err != nil {
		return nil, ctx.FuncError(err)
	}

	for _, entry := range entries {
		k := entry.item
		v := entry.data
		array.OffsetUnset(ctx, k)
		array.OffsetSet(ctx, k, v)
	}

	return phpv.ZTrue.ZVal(), nil
}

// > func bool ksort ( array &$array [, int $sort_flags = SORT_REGULAR ] )
func fncArrayKSort(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var array *phpv.ZArray
	var sortFlagsArg *phpv.ZInt
	_, err := core.Expand(ctx, args, core.Ref(&array), &sortFlagsArg)
	if err != nil {
		return nil, ctx.FuncError(err)
	}

	var entries []compareEntry
	for k, v := range (array).Iterate(ctx) {
		entries = append(entries, compareEntry{
			item: k,
			data: v,
		})
	}

	arraySort(ctx, entries, sortFlagsArg)

	for _, entry := range entries {
		k := entry.item
		v := entry.data
		array.OffsetUnset(ctx, k)
		array.OffsetSet(ctx, k, v)
	}

	return phpv.ZTrue.ZVal(), nil
}

// > func bool krsort ( array &$array [, int $sort_flags = SORT_REGULAR ] )
func fncArrayKRSort(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var array *phpv.ZArray
	var sortFlagsArg *phpv.ZInt
	_, err := core.Expand(ctx, args, core.Ref(&array), &sortFlagsArg)
	if err != nil {
		return nil, ctx.FuncError(err)
	}

	var entries []compareEntry
	for k, v := range (array).Iterate(ctx) {
		entries = append(entries, compareEntry{
			item: k,
			data: v,
		})
	}

	arraySort(ctx, entries, sortFlagsArg)

	for _, entry := range core.IterateBackwards(entries) {
		k := entry.item
		v := entry.data
		array.OffsetUnset(ctx, k)
		array.OffsetSet(ctx, k, v)
	}

	return phpv.ZTrue.ZVal(), nil
}

// > func bool asort ( array &$array [, int $sort_flags = SORT_REGULAR ] )
func fncArrayASort(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var array *phpv.ZArray
	var sortFlagsArg *phpv.ZInt
	_, err := core.Expand(ctx, args, core.Ref(&array), &sortFlagsArg)
	if err != nil {
		return nil, ctx.FuncError(err)
	}

	var entries []compareEntry
	for k, v := range (array).Iterate(ctx) {
		entries = append(entries, compareEntry{
			item: v,
			data: k,
		})
	}

	arraySort(ctx, entries, sortFlagsArg)

	for _, entry := range entries {
		k := entry.data
		v := entry.item
		array.OffsetUnset(ctx, k)
		array.OffsetSet(ctx, k, v)
	}

	return phpv.ZTrue.ZVal(), nil
}

// > func bool arsort ( array &$array [, int $sort_flags = SORT_REGULAR ] )
func fncArrayARSort(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var array *phpv.ZArray
	var sortFlagsArg *phpv.ZInt
	_, err := core.Expand(ctx, args, core.Ref(&array), &sortFlagsArg)
	if err != nil {
		return nil, ctx.FuncError(err)
	}

	var entries []compareEntry
	for k, v := range (array).Iterate(ctx) {
		entries = append(entries, compareEntry{
			item: v,
			data: k,
		})
	}

	arraySort(ctx, entries, sortFlagsArg)

	for _, entry := range core.IterateBackwards(entries) {
		k := entry.data
		v := entry.item
		array.OffsetUnset(ctx, k)
		array.OffsetSet(ctx, k, v)
	}

	return phpv.ZTrue.ZVal(), nil
}

// > func bool natsort ( array &$array [, int $sort_flags = SORT_REGULAR ] )
func fncArrayNatSort(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var array *phpv.ZArray
	var sortFlagsArg *phpv.ZInt
	_, err := core.Expand(ctx, args, core.Ref(&array), &sortFlagsArg)
	if err != nil {
		return nil, ctx.FuncError(err)
	}

	var entries []compareEntry
	for k, v := range (array).Iterate(ctx) {
		entries = append(entries, compareEntry{
			item: v,
			data: k,
		})
	}

	sortFlags := SORT_NATURAL
	arraySort(ctx, entries, &sortFlags)

	for _, entry := range entries {
		k := entry.data
		v := entry.item
		array.OffsetUnset(ctx, k)
		array.OffsetSet(ctx, k, v)
	}

	return phpv.ZTrue.ZVal(), nil
}

// > func bool natcasesort ( array &$array [, int $sort_flags = SORT_REGULAR ] )
func fncArrayNatCaseSort(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var array *phpv.ZArray
	var sortFlagsArg *phpv.ZInt
	_, err := core.Expand(ctx, args, core.Ref(&array), &sortFlagsArg)
	if err != nil {
		return nil, ctx.FuncError(err)
	}

	var entries []compareEntry
	for k, v := range (array).Iterate(ctx) {
		entries = append(entries, compareEntry{
			item: v,
			data: k,
		})
	}

	sortFlags := SORT_NATURAL | SORT_FLAG_CASE
	arraySort(ctx, entries, &sortFlags)

	for _, entry := range entries {
		k := entry.data
		v := entry.item
		array.OffsetUnset(ctx, k)
		array.OffsetSet(ctx, k, v)
	}

	return phpv.ZTrue.ZVal(), nil
}

func arraySort(ctx phpv.Context, entries []compareEntry, sortFlagsArg *phpv.ZInt) {
	caseInsensitive := false
	sortFlags := SORT_REGULAR

	if sortFlagsArg != nil {
		sortFlags = *sortFlagsArg
		caseInsensitive = sortFlags&SORT_FLAG_CASE != 0
		sortFlags &= ^SORT_FLAG_CASE
	}

	sortBy := zSortComparer{ctx, entries, caseInsensitive}
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

	sort.Slice(entries, sortFn)
}

func arrayUSort(ctx phpv.Context, entries []compareEntry, compare phpv.Callable) error {
	var err error
	sort.Slice(entries, func(i, j int) bool {
		if err != nil {
			return false
		}
		a := entries[i].item
		b := entries[j].item

		var ret *phpv.ZVal
		ret, err = compare.Call(ctx, []*phpv.ZVal{a, b})
		return ret.AsInt(ctx) < 0
	})

	return err
}

type compareEntry struct {
	item *phpv.ZVal
	data *phpv.ZVal
}
type zSortComparer struct {
	ctx             phpv.Context
	values          []compareEntry
	caseInsensitive bool
}

func (zv zSortComparer) regular(i, j int) bool {
	cmp, _ := core.Compare(zv.ctx, zv.values[i].item, zv.values[j].item)
	return cmp < 0
}

func (zv zSortComparer) numerically(i, j int) bool {
	a := zv.values[i].item.AsInt(zv.ctx)
	b := zv.values[j].item.AsInt(zv.ctx)
	return a < b
}

func (zv zSortComparer) stringly(i, j int) bool {
	a := string(zv.values[i].item.AsString(zv.ctx))
	b := string(zv.values[j].item.AsString(zv.ctx))
	if zv.caseInsensitive {
		a = strings.ToLower(a)
		b = strings.ToLower(b)
	}
	return strings.Compare(a, b) < 0
}

func (zv zSortComparer) naturally(i, j int) bool {
	a := []byte(zv.values[i].item.AsString(zv.ctx))
	b := []byte(zv.values[j].item.AsString(zv.ctx))
	return natCmp(a, b, !zv.caseInsensitive) < 0
}
