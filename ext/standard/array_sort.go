package standard

import (
	"sort"
	"strings"

	"github.com/MagicalTux/goro/core"
	"github.com/MagicalTux/goro/core/phpv"
)

// > func bool sort ( array &$array [, int $sort_flags = SORT_REGULAR ] )
func fncArraySort(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var array core.Ref[*phpv.ZArray]
	var sortFlagsArg core.Optional[phpv.ZInt]
	_, err := core.Expand(ctx, args, &array, &sortFlagsArg)
	if err != nil {
		return nil, ctx.FuncError(err)
	}

	var entries []compareEntry
	for _, v := range array.Get().Iterate(ctx) {
		entries = append(entries, compareEntry{
			item: v,
		})
	}

	arraySort(ctx, entries, sortFlagsArg.GetOrDefault(SORT_REGULAR), false)

	if err = array.Get().Clear(ctx); err != nil {
		return nil, err
	}

	for _, v := range entries {
		array.Get().OffsetSet(ctx, nil, v.item)
	}

	return phpv.ZTrue.ZVal(), nil
}

// > func bool rsort ( array &$array [, int $sort_flags = SORT_REGULAR ] )
func fncArrayRSort(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var array core.Ref[*phpv.ZArray]
	var sortFlagsArg core.Optional[phpv.ZInt]
	_, err := core.Expand(ctx, args, &array, &sortFlagsArg)
	if err != nil {
		return nil, ctx.FuncError(err)
	}

	var entries []compareEntry
	for _, v := range (array).Get().Iterate(ctx) {
		entries = append(entries, compareEntry{
			item: v,
		})
	}

	arraySort(ctx, entries, sortFlagsArg.GetOrDefault(SORT_REGULAR), true)

	if err = array.Get().Clear(ctx); err != nil {
		return nil, err
	}

	for _, v := range core.IterateBackwards(entries) {
		array.Get().OffsetSet(ctx, nil, v.item)
	}

	return phpv.ZTrue.ZVal(), nil
}

// > func bool usort ( array &$array , callable $value_compare_func )
func fncArrayUSort(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var array core.Ref[*phpv.ZArray]
	var compareFunc phpv.Callable
	_, err := core.Expand(ctx, args, &array, &compareFunc)
	if err != nil {
		return nil, ctx.FuncError(err)
	}

	var entries []compareEntry
	for _, v := range (array).Get().Iterate(ctx) {
		entries = append(entries, compareEntry{
			item: v,
		})
	}

	err = arrayUSort(ctx, entries, compareFunc)
	if err != nil {
		return nil, ctx.FuncError(err)
	}

	if err = array.Get().Clear(ctx); err != nil {
		return nil, err
	}

	for _, v := range entries {
		array.Get().OffsetSet(ctx, nil, v.item)
	}

	return phpv.ZTrue.ZVal(), nil
}

// > func bool uasort ( array &$array , callable $value_compare_func )
func fncArrayUASort(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var array core.Ref[*phpv.ZArray]
	var compareFunc phpv.Callable
	_, err := core.Expand(ctx, args, &array, &compareFunc)
	if err != nil {
		return nil, ctx.FuncError(err)
	}

	var entries []compareEntry
	for k, v := range (array).Get().Iterate(ctx) {
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
		array.Get().OffsetUnset(ctx, k)
		array.Get().OffsetSet(ctx, k, v)
	}

	return phpv.ZTrue.ZVal(), nil
}

// > func bool uksort ( array &$array , callable $key_compare_func )
func fncArrayUKSort(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var array core.Ref[*phpv.ZArray]
	var compareFunc phpv.Callable
	_, err := core.Expand(ctx, args, &array, &compareFunc)
	if err != nil {
		return nil, ctx.FuncError(err)
	}

	var entries []compareEntry
	for k, v := range (array).Get().Iterate(ctx) {
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
		array.Get().OffsetUnset(ctx, k)
		array.Get().OffsetSet(ctx, k, v)
	}

	return phpv.ZTrue.ZVal(), nil
}

// > func bool ksort ( array &$array [, int $sort_flags = SORT_REGULAR ] )
func fncArrayKSort(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var array core.Ref[*phpv.ZArray]
	var sortFlagsArg core.Optional[phpv.ZInt]
	_, err := core.Expand(ctx, args, &array, &sortFlagsArg)
	if err != nil {
		return nil, ctx.FuncError(err)
	}

	var entries []compareEntry
	for k, v := range (array).Get().Iterate(ctx) {
		entries = append(entries, compareEntry{
			item: k,
			data: v,
		})
	}

	arraySort(ctx, entries, sortFlagsArg.GetOrDefault(SORT_REGULAR), false)

	for _, entry := range entries {
		k := entry.item
		v := entry.data
		array.Get().OffsetUnset(ctx, k)
		array.Get().OffsetSet(ctx, k, v)
	}

	return phpv.ZTrue.ZVal(), nil
}

// > func bool krsort ( array &$array [, int $sort_flags = SORT_REGULAR ] )
func fncArrayKRSort(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var array core.Ref[*phpv.ZArray]
	var sortFlagsArg core.Optional[phpv.ZInt]
	_, err := core.Expand(ctx, args, &array, &sortFlagsArg)
	if err != nil {
		return nil, ctx.FuncError(err)
	}

	var entries []compareEntry
	for k, v := range (array).Get().Iterate(ctx) {
		entries = append(entries, compareEntry{
			item: k,
			data: v,
		})
	}

	arraySort(ctx, entries, sortFlagsArg.GetOrDefault(SORT_REGULAR), true)

	for _, entry := range core.IterateBackwards(entries) {
		k := entry.item
		v := entry.data
		array.Get().OffsetUnset(ctx, k)
		array.Get().OffsetSet(ctx, k, v)
	}

	return phpv.ZTrue.ZVal(), nil
}

// > func bool asort ( array &$array [, int $sort_flags = SORT_REGULAR ] )
func fncArrayASort(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var array core.Ref[*phpv.ZArray]
	var sortFlagsArg core.Optional[phpv.ZInt]
	_, err := core.Expand(ctx, args, &array, &sortFlagsArg)
	if err != nil {
		return nil, ctx.FuncError(err)
	}

	var entries []compareEntry
	for k, v := range (array).Get().Iterate(ctx) {
		entries = append(entries, compareEntry{
			item: v,
			data: k,
		})
	}

	arraySort(ctx, entries, sortFlagsArg.GetOrDefault(SORT_REGULAR), false)

	for _, entry := range entries {
		k := entry.data
		v := entry.item
		array.Get().OffsetUnset(ctx, k)
		array.Get().OffsetSet(ctx, k, v)
	}

	return phpv.ZTrue.ZVal(), nil
}

// > func bool arsort ( array &$array [, int $sort_flags = SORT_REGULAR ] )
func fncArrayARSort(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var array core.Ref[*phpv.ZArray]
	var sortFlagsArg core.Optional[phpv.ZInt]
	_, err := core.Expand(ctx, args, &array, &sortFlagsArg)
	if err != nil {
		return nil, ctx.FuncError(err)
	}

	var entries []compareEntry
	for k, v := range (array).Get().Iterate(ctx) {
		entries = append(entries, compareEntry{
			item: v,
			data: k,
		})
	}

	arraySort(ctx, entries, sortFlagsArg.GetOrDefault(SORT_REGULAR), true)

	for _, entry := range core.IterateBackwards(entries) {
		k := entry.data
		v := entry.item
		array.Get().OffsetUnset(ctx, k)
		array.Get().OffsetSet(ctx, k, v)
	}

	return phpv.ZTrue.ZVal(), nil
}

// > func bool natsort ( array &$array )
func fncArrayNatSort(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var array core.Ref[*phpv.ZArray]
	var sortFlagsArg core.Optional[phpv.ZInt]
	_, err := core.Expand(ctx, args, &array, &sortFlagsArg)
	if err != nil {
		return nil, ctx.FuncError(err)
	}

	var entries []compareEntry
	for k, v := range (array).Get().Iterate(ctx) {
		entries = append(entries, compareEntry{
			item: v,
			data: k,
		})
	}

	arraySort(ctx, entries, SORT_NATURAL, false)

	for _, entry := range entries {
		k := entry.data
		v := entry.item
		array.Get().OffsetUnset(ctx, k)
		array.Get().OffsetSet(ctx, k, v)
	}

	return phpv.ZTrue.ZVal(), nil
}

// > func bool natcasesort ( array &$array )
func fncArrayNatCaseSort(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var array core.Ref[*phpv.ZArray]
	var sortFlagsArg core.Optional[phpv.ZInt]
	_, err := core.Expand(ctx, args, &array, &sortFlagsArg)
	if err != nil {
		return nil, ctx.FuncError(err)
	}

	var entries []compareEntry
	for k, v := range (array).Get().Iterate(ctx) {
		entries = append(entries, compareEntry{
			item: v,
			data: k,
		})
	}

	sortFlags := SORT_NATURAL | SORT_FLAG_CASE
	arraySort(ctx, entries, sortFlags, false)

	for _, entry := range entries {
		k := entry.data
		v := entry.item
		array.Get().OffsetUnset(ctx, k)
		array.Get().OffsetSet(ctx, k, v)
	}

	return phpv.ZTrue.ZVal(), nil
}

func arraySort(ctx phpv.Context, entries []compareEntry, sortFlags phpv.ZInt, reversed bool) {
	caseInsensitive := sortFlags&SORT_FLAG_CASE != 0
	sortFlags &= ^SORT_FLAG_CASE

	sortBy := zSortComparer{ctx, entries, caseInsensitive, reversed}
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
	// item is the one to be compared
	// can be either the key or value
	item *phpv.ZVal

	// data is the supplementary info,
	// if item is the key, then data is the value,
	// if item is the value, then data is the key,
	// this is needed to maintain index association
	data *phpv.ZVal
}

type zSortComparer struct {
	ctx             phpv.Context
	values          []compareEntry
	caseInsensitive bool
	reversed        bool
}

func (zv zSortComparer) regular(i, j int) bool {
	cmp, _ := phpv.Compare(zv.ctx, zv.values[i].item, zv.values[j].item)
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
		// this is to handle cases where
		// ["Orange", "orange"] is backwards,
		// this fixes rsort, arsort and krsort
		a = strings.ToUpper(a)
		b = strings.ToUpper(b)
		if a == b {
			return zv.reversed
		}
	}
	return strings.Compare(a, b) < 0
}

func (zv zSortComparer) naturally(i, j int) bool {
	s1 := string(zv.values[i].item.AsString(zv.ctx))
	s2 := string(zv.values[j].item.AsString(zv.ctx))
	if zv.caseInsensitive {
		s1 = strings.ToUpper(s1)
		s2 = strings.ToUpper(s2)
		if s1 == s2 {
			return zv.reversed
		}
	}
	a := []byte(s1)
	b := []byte(s2)
	return natCmp(a, b, !zv.caseInsensitive) < 0
}
