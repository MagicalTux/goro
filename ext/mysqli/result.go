package mysqli

import (
	"database/sql"
	"fmt"

	"github.com/MagicalTux/goro/core"
	"github.com/MagicalTux/goro/core/phpobj"
	"github.com/MagicalTux/goro/core/phpv"
)

// MysqliResultClass is the PHP mysqli_result class.
var MysqliResultClass *phpobj.ZClass

// mysqliResultData holds the state of a mysqli_result object.
type mysqliResultData struct {
	rows       *sql.Rows
	cols       []string
	colTypes   []*sql.ColumnType
	buffered   [][]interface{}
	pos        int
	freed      bool
	// for non-SELECT results
	affectedRows int64
	insertID     int64
}

func initMysqliResultClass() {
	MysqliResultClass = &phpobj.ZClass{
		Name: "mysqli_result",
		Methods: map[phpv.ZString]*phpv.ZClassMethod{
			"fetch_assoc":  {Name: "fetch_assoc", Method: phpobj.NativeMethod(mysqliFetchAssoc)},
			"fetch_array":  {Name: "fetch_array", Method: phpobj.NativeMethod(mysqliFetchArray)},
			"fetch_row":    {Name: "fetch_row", Method: phpobj.NativeMethod(mysqliFetchRow)},
			"fetch_object": {Name: "fetch_object", Method: phpobj.NativeMethod(mysqliFetchObject)},
			"fetch_all":    {Name: "fetch_all", Method: phpobj.NativeMethod(mysqliFetchAll)},
			"fetch_column": {Name: "fetch_column", Method: phpobj.NativeMethod(mysqliFetchColumn)},
			"fetch_field":  {Name: "fetch_field", Method: phpobj.NativeMethod(mysqliFetchField)},
			"fetch_fields": {Name: "fetch_fields", Method: phpobj.NativeMethod(mysqliFetchFields)},
			"data_seek":    {Name: "data_seek", Method: phpobj.NativeMethod(mysqliDataSeek)},
			"free":         {Name: "free", Method: phpobj.NativeMethod(mysqliFree)},
			"free_result":  {Name: "free_result", Method: phpobj.NativeMethod(mysqliFree)},
			"close":        {Name: "close", Method: phpobj.NativeMethod(mysqliFree)},
			"__debuginfo":  {Name: "__debugInfo", Method: phpobj.NativeMethod(mysqliResultDebugInfo)},
		},
		H: &phpv.ZClassHandlers{
			HandlePropGetEager: true,
			HandlePropGet: func(ctx phpv.Context, o phpv.ZObject, key phpv.ZString) (*phpv.ZVal, error) {
				zo, ok := o.(*phpobj.ZObject)
				if !ok {
					return nil, nil
				}
				d := getMysqliResultData(zo)
				if d == nil {
					return nil, nil
				}
				switch key {
				case "num_rows":
					if err := d.bufferAll(ctx); err != nil {
						return phpv.ZInt(0).ZVal(), nil
					}
					return phpv.ZInt(len(d.buffered)).ZVal(), nil
				case "field_count":
					return phpv.ZInt(len(d.cols)).ZVal(), nil
				case "current_field":
					return phpv.ZInt(0).ZVal(), nil
				case "lengths":
					return phpv.ZNULL.ZVal(), nil
				}
				return nil, nil
			},
		},
	}
}

func getMysqliResultData(o *phpobj.ZObject) *mysqliResultData {
	if d, ok := o.GetOpaque(MysqliResultClass).(*mysqliResultData); ok {
		return d
	}
	return nil
}

// getMysqliResultObj extracts a mysqli_result ZObject from the first arg.
func getMysqliResultObj(ctx phpv.Context, args []*phpv.ZVal) (*phpobj.ZObject, error) {
	if len(args) < 1 || args[0] == nil {
		return nil, phpobj.ThrowError(ctx, phpobj.TypeError,
			"Argument #1 must be of type mysqli_result")
	}
	if args[0].GetType() != phpv.ZtObject {
		return nil, phpobj.ThrowError(ctx, phpobj.TypeError,
			"Argument #1 must be of type mysqli_result")
	}
	zo, ok := args[0].Value().(*phpobj.ZObject)
	if !ok {
		return nil, phpobj.ThrowError(ctx, phpobj.TypeError,
			"Argument #1 must be of type mysqli_result")
	}
	if getMysqliResultData(zo) == nil {
		return nil, phpobj.ThrowError(ctx, phpobj.TypeError,
			"Argument #1 must be of type mysqli_result")
	}
	return zo, nil
}

// newMysqliResultObject creates a new mysqli_result object wrapping sql.Rows.
func newMysqliResultObject(ctx phpv.Context, rows *sql.Rows) (*phpobj.ZObject, error) {
	cols, err := rows.Columns()
	if err != nil {
		return nil, err
	}
	colTypes, _ := rows.ColumnTypes()

	d := &mysqliResultData{
		rows:     rows,
		cols:     cols,
		colTypes: colTypes,
	}

	obj, err := phpobj.NewZObjectOpaque(ctx, MysqliResultClass, d)
	if err != nil {
		return nil, err
	}
	return obj, nil
}

// bufferAll reads all rows from the result set into memory.
func (d *mysqliResultData) bufferAll(ctx phpv.Context) error {
	if d.rows == nil {
		return nil
	}
	for d.rows.Next() {
		row := make([]interface{}, len(d.cols))
		ptrs := make([]interface{}, len(d.cols))
		for i := range row {
			ptrs[i] = &row[i]
		}
		if err := d.rows.Scan(ptrs...); err != nil {
			return err
		}
		d.buffered = append(d.buffered, row)
	}
	d.rows.Close()
	d.rows = nil
	return nil
}

// fetchNextRow returns the next buffered or live row as a slice of interface{}.
func (d *mysqliResultData) fetchNextRow(ctx phpv.Context) ([]interface{}, error) {
	if d.freed {
		return nil, nil
	}

	if d.rows != nil {
		// streaming mode: read next row
		if d.rows.Next() {
			row := make([]interface{}, len(d.cols))
			ptrs := make([]interface{}, len(d.cols))
			for i := range row {
				ptrs[i] = &row[i]
			}
			if err := d.rows.Scan(ptrs...); err != nil {
				return nil, err
			}
			d.pos++
			return row, nil
		}
		d.rows.Close()
		d.rows = nil
		return nil, nil
	}

	// buffered mode
	if d.pos >= len(d.buffered) {
		return nil, nil
	}
	row := d.buffered[d.pos]
	d.pos++
	return row, nil
}

// sqlValueToZVal converts a database/sql scanned value to a phpv.ZVal.
func sqlValueToZVal(v interface{}) *phpv.ZVal {
	if v == nil {
		return phpv.ZNULL.ZVal()
	}
	switch val := v.(type) {
	case int64:
		return phpv.ZInt(val).ZVal()
	case float64:
		return phpv.ZFloat(val).ZVal()
	case bool:
		return phpv.ZBool(val).ZVal()
	case []byte:
		return phpv.ZString(val).ZVal()
	case string:
		return phpv.ZString(val).ZVal()
	default:
		return phpv.ZString(fmt.Sprintf("%v", val)).ZVal()
	}
}

func mysqliFetchAssoc(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	d := getMysqliResultData(o)
	if d == nil {
		return phpv.ZNULL.ZVal(), nil
	}

	row, err := d.fetchNextRow(ctx)
	if err != nil {
		return phpv.ZNULL.ZVal(), nil
	}
	if row == nil {
		return phpv.ZNULL.ZVal(), nil
	}

	arr := phpv.NewZArray()
	for i, col := range d.cols {
		arr.OffsetSet(ctx, phpv.ZString(col), sqlValueToZVal(row[i]))
	}
	return arr.ZVal(), nil
}

func mysqliFetchArray(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	d := getMysqliResultData(o)
	if d == nil {
		return phpv.ZNULL.ZVal(), nil
	}

	var mode core.Optional[phpv.ZInt]
	core.Expand(ctx, args, &mode)
	fetchMode := int(mode.GetOrDefault(phpv.ZInt(MYSQLI_BOTH)))

	row, err := d.fetchNextRow(ctx)
	if err != nil {
		return phpv.ZNULL.ZVal(), nil
	}
	if row == nil {
		return phpv.ZNULL.ZVal(), nil
	}

	arr := phpv.NewZArray()
	for i, col := range d.cols {
		v := sqlValueToZVal(row[i])
		if fetchMode == MYSQLI_ASSOC || fetchMode == MYSQLI_BOTH {
			arr.OffsetSet(ctx, phpv.ZString(col), v)
		}
		if fetchMode == MYSQLI_NUM || fetchMode == MYSQLI_BOTH {
			arr.OffsetSet(ctx, phpv.ZInt(i), v)
		}
	}
	return arr.ZVal(), nil
}

func mysqliFetchRow(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	d := getMysqliResultData(o)
	if d == nil {
		return phpv.ZNULL.ZVal(), nil
	}

	row, err := d.fetchNextRow(ctx)
	if err != nil {
		return phpv.ZNULL.ZVal(), nil
	}
	if row == nil {
		return phpv.ZNULL.ZVal(), nil
	}

	arr := phpv.NewZArray()
	for i := range d.cols {
		arr.OffsetSet(ctx, phpv.ZInt(i), sqlValueToZVal(row[i]))
	}
	return arr.ZVal(), nil
}

func mysqliFetchObject(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	d := getMysqliResultData(o)
	if d == nil {
		return phpv.ZNULL.ZVal(), nil
	}

	row, err := d.fetchNextRow(ctx)
	if err != nil {
		return phpv.ZNULL.ZVal(), nil
	}
	if row == nil {
		return phpv.ZNULL.ZVal(), nil
	}

	// Create a stdClass with the properties set
	obj, err2 := phpv.NewStdClassFunc(ctx)
	if err2 != nil {
		return phpv.ZNULL.ZVal(), nil
	}
	for i, col := range d.cols {
		obj.ObjectSet(ctx, phpv.ZString(col), sqlValueToZVal(row[i]))
	}
	return obj.ZVal(), nil
}

func mysqliFetchAll(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	d := getMysqliResultData(o)
	if d == nil {
		return phpv.NewZArray().ZVal(), nil
	}

	var mode core.Optional[phpv.ZInt]
	core.Expand(ctx, args, &mode)
	fetchMode := int(mode.GetOrDefault(phpv.ZInt(MYSQLI_ASSOC)))

	// Buffer all remaining rows
	if err := d.bufferAll(ctx); err != nil {
		return phpv.NewZArray().ZVal(), nil
	}

	result := phpv.NewZArray()
	savedPos := d.pos
	d.pos = savedPos

	for d.pos < len(d.buffered) {
		row, err := d.fetchNextRow(ctx)
		if err != nil || row == nil {
			break
		}

		arr := phpv.NewZArray()
		for i, col := range d.cols {
			v := sqlValueToZVal(row[i])
			if fetchMode == MYSQLI_ASSOC || fetchMode == MYSQLI_BOTH {
				arr.OffsetSet(ctx, phpv.ZString(col), v)
			}
			if fetchMode == MYSQLI_NUM || fetchMode == MYSQLI_BOTH {
				arr.OffsetSet(ctx, phpv.ZInt(i), v)
			}
		}
		result.OffsetSet(ctx, nil, arr.ZVal())
	}

	return result.ZVal(), nil
}

func mysqliFetchColumn(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	d := getMysqliResultData(o)
	if d == nil {
		return phpv.ZBool(false).ZVal(), nil
	}

	var colIdx core.Optional[phpv.ZInt]
	core.Expand(ctx, args, &colIdx)
	col := int(colIdx.GetOrDefault(0))

	row, err := d.fetchNextRow(ctx)
	if err != nil {
		return phpv.ZBool(false).ZVal(), nil
	}
	if row == nil {
		return phpv.ZBool(false).ZVal(), nil
	}
	if col >= len(row) {
		return phpv.ZBool(false).ZVal(), nil
	}
	return sqlValueToZVal(row[col]), nil
}

func mysqliFetchField(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	d := getMysqliResultData(o)
	if d == nil || len(d.cols) == 0 {
		return phpv.ZBool(false).ZVal(), nil
	}

	// Return info about first column as a stdClass object
	obj, err := phpv.NewStdClassFunc(ctx)
	if err != nil {
		return phpv.ZBool(false).ZVal(), nil
	}
	obj.ObjectSet(ctx, phpv.ZString("name"), phpv.ZString(d.cols[0]).ZVal())
	obj.ObjectSet(ctx, phpv.ZString("orgname"), phpv.ZString(d.cols[0]).ZVal())
	obj.ObjectSet(ctx, phpv.ZString("table"), phpv.ZString("").ZVal())
	obj.ObjectSet(ctx, phpv.ZString("orgtable"), phpv.ZString("").ZVal())
	obj.ObjectSet(ctx, phpv.ZString("def"), phpv.ZString("").ZVal())
	obj.ObjectSet(ctx, phpv.ZString("db"), phpv.ZString("").ZVal())
	obj.ObjectSet(ctx, phpv.ZString("catalog"), phpv.ZString("").ZVal())
	obj.ObjectSet(ctx, phpv.ZString("max_length"), phpv.ZInt(0).ZVal())
	obj.ObjectSet(ctx, phpv.ZString("length"), phpv.ZInt(0).ZVal())
	obj.ObjectSet(ctx, phpv.ZString("charsetnr"), phpv.ZInt(0).ZVal())
	obj.ObjectSet(ctx, phpv.ZString("flags"), phpv.ZInt(0).ZVal())
	obj.ObjectSet(ctx, phpv.ZString("type"), phpv.ZInt(MYSQLI_TYPE_STRING).ZVal())
	obj.ObjectSet(ctx, phpv.ZString("decimals"), phpv.ZInt(0).ZVal())
	return obj.ZVal(), nil
}

func mysqliFetchFields(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	d := getMysqliResultData(o)
	if d == nil {
		return phpv.ZBool(false).ZVal(), nil
	}

	result := phpv.NewZArray()
	for _, col := range d.cols {
		obj, err := phpv.NewStdClassFunc(ctx)
		if err != nil {
			continue
		}
		obj.ObjectSet(ctx, phpv.ZString("name"), phpv.ZString(col).ZVal())
		obj.ObjectSet(ctx, phpv.ZString("orgname"), phpv.ZString(col).ZVal())
		obj.ObjectSet(ctx, phpv.ZString("table"), phpv.ZString("").ZVal())
		obj.ObjectSet(ctx, phpv.ZString("orgtable"), phpv.ZString("").ZVal())
		obj.ObjectSet(ctx, phpv.ZString("def"), phpv.ZString("").ZVal())
		obj.ObjectSet(ctx, phpv.ZString("db"), phpv.ZString("").ZVal())
		obj.ObjectSet(ctx, phpv.ZString("catalog"), phpv.ZString("").ZVal())
		obj.ObjectSet(ctx, phpv.ZString("max_length"), phpv.ZInt(0).ZVal())
		obj.ObjectSet(ctx, phpv.ZString("length"), phpv.ZInt(0).ZVal())
		obj.ObjectSet(ctx, phpv.ZString("charsetnr"), phpv.ZInt(0).ZVal())
		obj.ObjectSet(ctx, phpv.ZString("flags"), phpv.ZInt(0).ZVal())
		obj.ObjectSet(ctx, phpv.ZString("type"), phpv.ZInt(MYSQLI_TYPE_STRING).ZVal())
		obj.ObjectSet(ctx, phpv.ZString("decimals"), phpv.ZInt(0).ZVal())
		result.OffsetSet(ctx, nil, obj.ZVal())
	}
	return result.ZVal(), nil
}

func mysqliDataSeek(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	d := getMysqliResultData(o)
	if d == nil {
		return phpv.ZBool(false).ZVal(), nil
	}

	var offset phpv.ZInt
	_, err := core.Expand(ctx, args, &offset)
	if err != nil {
		return nil, err
	}

	// Ensure all rows are buffered to allow seeking
	if err := d.bufferAll(ctx); err != nil {
		return phpv.ZBool(false).ZVal(), nil
	}

	pos := int(offset)
	if pos < 0 || pos > len(d.buffered) {
		return phpv.ZBool(false).ZVal(), nil
	}

	d.pos = pos
	return phpv.ZBool(true).ZVal(), nil
}

func mysqliFree(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	d := getMysqliResultData(o)
	if d == nil {
		return phpv.ZNULL.ZVal(), nil
	}
	if d.rows != nil {
		d.rows.Close()
		d.rows = nil
	}
	d.buffered = nil
	d.freed = true
	return phpv.ZNULL.ZVal(), nil
}

func mysqliResultDebugInfo(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	d := getMysqliResultData(o)
	arr := phpv.NewZArray()
	if d != nil {
		if err := d.bufferAll(ctx); err == nil {
			arr.OffsetSet(ctx, phpv.ZString("num_rows"), phpv.ZInt(len(d.buffered)).ZVal())
		}
		arr.OffsetSet(ctx, phpv.ZString("field_count"), phpv.ZInt(len(d.cols)).ZVal())
		arr.OffsetSet(ctx, phpv.ZString("current_field"), phpv.ZInt(0).ZVal())
		arr.OffsetSet(ctx, phpv.ZString("lengths"), phpv.ZNULL.ZVal())
	}
	return arr.ZVal(), nil
}
