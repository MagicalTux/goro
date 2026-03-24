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

	for _, v := range entries {
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

	err = arrayUSort(ctx, entries, compareFunc, "usort")
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

	err = arrayUSort(ctx, entries, compareFunc, "uasort")
	if err != nil {
		return nil, ctx.FuncError(err)
	}

	if err = array.Get().Clear(ctx); err != nil {
		return nil, err
	}
	for _, entry := range entries {
		array.Get().OffsetSet(ctx, entry.data, entry.item)
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

	err = arrayUSort(ctx, entries, compareFunc, "uksort")
	if err != nil {
		return nil, ctx.FuncError(err)
	}

	if err = array.Get().Clear(ctx); err != nil {
		return nil, err
	}
	for _, entry := range entries {
		array.Get().OffsetSet(ctx, entry.item, entry.data)
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

	if err = array.Get().Clear(ctx); err != nil {
		return nil, err
	}
	for _, entry := range entries {
		array.Get().OffsetSet(ctx, entry.item, entry.data)
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

	if err = array.Get().Clear(ctx); err != nil {
		return nil, err
	}
	for _, entry := range entries {
		array.Get().OffsetSet(ctx, entry.item, entry.data)
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

	if err = array.Get().Clear(ctx); err != nil {
		return nil, err
	}
	for _, entry := range entries {
		array.Get().OffsetSet(ctx, entry.data, entry.item)
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

	if err = array.Get().Clear(ctx); err != nil {
		return nil, err
	}
	for _, entry := range entries {
		array.Get().OffsetSet(ctx, entry.data, entry.item)
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

	if err = array.Get().Clear(ctx); err != nil {
		return nil, err
	}
	for _, entry := range entries {
		array.Get().OffsetSet(ctx, entry.data, entry.item)
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

	if err = array.Get().Clear(ctx); err != nil {
		return nil, err
	}
	for _, entry := range entries {
		array.Get().OffsetSet(ctx, entry.data, entry.item)
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

	sort.SliceStable(entries, sortFn)
}

func arrayUSort(ctx phpv.Context, entries []compareEntry, compare phpv.Callable, funcName ...string) error {
	var err error
	boolDeprecated := false
	fname := "usort"
	if len(funcName) > 0 {
		fname = funcName[0]
	}
	sort.SliceStable(entries, func(i, j int) bool {
		if err != nil {
			return false
		}
		a := entries[i].item
		b := entries[j].item

		var ret *phpv.ZVal
		ret, err = ctx.CallZValInternal(ctx, compare, []*phpv.ZVal{a, b})
		if err != nil {
			return false
		}
		// PHP 8.2+ deprecation: comparison functions should not return bool
		if ret != nil && ret.GetType() == phpv.ZtBool && !boolDeprecated {
			boolDeprecated = true
			_ = ctx.Deprecated(fname + "(): Returning bool from comparison function is deprecated, return an integer less than, equal to, or greater than zero")
		}
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

func (c *zSortComparer) regular(i, j int) bool {
	a := c.values[i].item
	b := c.values[j].item

	if c.caseInsensitive {
		a = phpv.ZStr(strings.ToLower(a.String()))
		b = phpv.ZStr(strings.ToLower(b.String()))
	}

	cmp, err := phpv.Compare(c.ctx, a, b)
	if err != nil {
		return false
	}
	if c.reversed {
		return cmp > 0
	}
	return cmp < 0
}

func (c *zSortComparer) numerically(i, j int) bool {
	a := c.values[i].item.AsFloat(c.ctx)
	b := c.values[j].item.AsFloat(c.ctx)

	if c.reversed {
		return a > b
	}
	return a < b
}

func (c *zSortComparer) stringly(i, j int) bool {
	a := c.values[i].item.AsString(c.ctx)
	b := c.values[j].item.AsString(c.ctx)

	if c.caseInsensitive {
		a = phpv.ZString(strings.ToLower(string(a)))
		b = phpv.ZString(strings.ToLower(string(b)))
	}

	if c.reversed {
		return a > b
	}
	return a < b
}

func (c *zSortComparer) naturally(i, j int) bool {
	a := string(c.values[i].item.AsString(c.ctx))
	b := string(c.values[j].item.AsString(c.ctx))

	cmp := natCmp([]byte(a), []byte(b), !c.caseInsensitive)
	if c.reversed {
		return cmp > 0
	}
	return cmp < 0
}
