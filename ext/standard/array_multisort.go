package standard

import (
	"bytes"
	"fmt"
	"sort"
	"strings"

	"github.com/MagicalTux/goro/core"
	"github.com/MagicalTux/goro/core/phpv"
)

// mainly used for the array_multisort implementation
type ztable struct {
	columns    []*phpv.ZArray
	sortFlags  []phpv.ZInt
	sortOrders []phpv.ZInt

	// an indexMap of [2, 1, 0, 3]
	// means swap values in the indices 0 and 2
	indexMap []int
}

func (t *ztable) CommitChanges(ctx phpv.Context) {
	for j := 0; j < t.CountColumns(ctx); j++ {
		numRows := t.CountRows(ctx)
		overWritten := make([]*phpv.ZVal, numRows)
		for i := 0; i < numRows; i++ {
			newVal := overWritten[t.indexMap[i]]
			if newVal == nil {
				_, newVal, _ = t.columns[j].OffsetAt(ctx, t.indexMap[i])
			} else {
				overWritten[t.indexMap[i]] = nil
			}
			_, oldVal, _ := t.columns[j].OffsetAt(ctx, i)
			overWritten[i] = oldVal
			t.SetValue(ctx, j, i, newVal)
		}
	}
	for i := range t.indexMap {
		t.indexMap[i] = i
	}
}

func (t *ztable) SwapRows(i, j int) {
	k := t.indexMap[i]
	t.indexMap[i] = t.indexMap[j]
	t.indexMap[j] = k
}

func (t *ztable) AddColumn(ctx phpv.Context, col *phpv.ZArray, flag, order phpv.ZInt) {
	t.columns = append(t.columns, col)
	t.sortFlags = append(t.sortFlags, flag)
	t.sortOrders = append(t.sortOrders, order)
	if t.indexMap == nil {
		for i := 0; i < int(col.Count(ctx)); i++ {
			t.indexMap = append(t.indexMap, i)
		}
	}
}

func (t *ztable) SetValue(ctx phpv.Context, col, row int, val *phpv.ZVal) {
	t.columns[col].OffsetSetAt(ctx, row, val)
}

func (t *ztable) GetValue(ctx phpv.Context, col, row int) *phpv.ZVal {
	i := t.indexMap[row]
	_, v, _ := t.columns[col].OffsetAt(ctx, i)
	return v
}

// TODO: must maintain key-value pair
// func (t *Table) GetKeyValue(ctx phpv.Context, col, row int) (*phpv.ZVal, *phpv.ZVal) {
//}

func (t *ztable) CountRows(ctx phpv.Context) int {
	if len(t.columns) > 0 {
		return int(t.columns[0].Count(ctx))
	}
	return 0
}

func (t *ztable) CountColumns(ctx phpv.Context) int {
	return len(t.columns)
}

func (t *ztable) String(ctx phpv.Context) string {
	var buf bytes.Buffer
	for j := 0; j < t.CountColumns(ctx); j++ {
		buf.WriteString(fmt.Sprintf("col %d:", j))
		for i := 0; i < t.CountRows(ctx); i++ {
			v := t.GetValue(ctx, j, i)
			buf.WriteString(fmt.Sprintf(" %4s", v.String()))
		}
		buf.WriteString("\n")
	}
	return buf.String()
}

func (t *ztable) StringTransposed(ctx phpv.Context) string {
	var buf bytes.Buffer
	for i := 0; i < t.CountRows(ctx); i++ {
		buf.WriteString(fmt.Sprintf("row %d:", i))
		for j := 0; j < t.CountColumns(ctx); j++ {
			v := t.GetValue(ctx, j, i)
			buf.WriteString(fmt.Sprintf(" %4s", v.String()))
		}
		buf.WriteString("\n")
	}
	return buf.String()
}

// > func bool array_multisort ( array &$array1 [, mixed $array1_sort_order = SORT_ASC [, mixed $array1_sort_flags = SORT_REGULAR [, mixed $... ]]] )
func fncArrayMultiSort(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	if len(args) == 0 {
		return phpv.ZFalse.ZVal(), ctx.FuncErrorf("Must be 1 length")
	}

	// TODO: PHP Warning:  array_multisort(): Array sizes are inconsistent

	table := &ztable{}

	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg.GetType() != phpv.ZtArray {
			return phpv.ZFalse.ZVal(), ctx.FuncErrorf("array expected")
		}

		arr := arg.AsArray(ctx)
		sortFlag := SORT_REGULAR
		sortOrder := SORT_ASC
		read := 0

	GetOptionalSortOrderFlags:
		for j := 1; j <= 2; j++ {
			next := core.Idx(args, i+j, phpv.ZNULL.ZVal())
			if next.GetType() != phpv.ZtInt {
				break GetOptionalSortOrderFlags
			}
			switch n := next.AsInt(ctx); n {
			case SORT_ASC, SORT_DESC:
				sortOrder = n
			default:
				sortFlag = n
			}
			read++
		}
		i += read

		table.AddColumn(ctx, arr, sortFlag, sortOrder)
	}

	// SliceStable will reorder table.indexMap entries
	// this is the same as doing table.SwapRows(a, b)
	sort.SliceStable(table.indexMap, func(i1, i2 int) bool {
		for j := range table.CountColumns(ctx) {
			a := table.GetValue(ctx, j, i1)
			b := table.GetValue(ctx, j, i2)
			reversed := table.sortOrders[j] == SORT_DESC
			// TODO: caseInsensitive

			cmp := 0
			switch table.sortFlags[j] {
			case SORT_NUMERIC:
				x := a.AsInt(ctx)
				y := b.AsInt(ctx)
				cmp = int(x - y)
			case SORT_STRING:
				x := string(a.AsString(ctx))
				y := string(b.AsString(ctx))
				cmp = strings.Compare(x, y)
			case SORT_NATURAL:
				// TODO:

			default:
				fallthrough
			case SORT_REGULAR:
				cmp, _ = phpv.Compare(ctx, a, b)
			}

			if cmp != 0 {
				if reversed {
					return cmp >= 0
				}
				return cmp < 0
			}
		}

		return false
	})

	table.CommitChanges(ctx)

	return nil, nil
}
