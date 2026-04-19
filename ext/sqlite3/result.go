package sqlite3

import (
	"database/sql"
	"fmt"

	"github.com/KarpelesLab/goro/core"
	"github.com/KarpelesLab/goro/core/phpobj"
	"github.com/KarpelesLab/goro/core/phpv"
)

// SQLite3ResultClass is the PHP SQLite3Result class.
var SQLite3ResultClass *phpobj.ZClass

// sqlite3ResultData holds the state of a SQLite3Result object.
type sqlite3ResultData struct {
	rows      *sql.Rows
	cols      []string
	colTypes  []*sql.ColumnType
	buffered  [][]interface{}
	pos       int
	finalized bool
}

func initSQLite3ResultClass() {
	SQLite3ResultClass = &phpobj.ZClass{
		Name: "SQLite3Result",
		Methods: map[phpv.ZString]*phpv.ZClassMethod{
			"fetcharray":  {Name: "fetchArray", Method: phpobj.NativeMethod(sqlite3ResultFetchArray)},
			"numcolumns":  {Name: "numColumns", Method: phpobj.NativeMethod(sqlite3ResultNumColumns)},
			"columnname":  {Name: "columnName", Method: phpobj.NativeMethod(sqlite3ResultColumnName)},
			"columntype":  {Name: "columnType", Method: phpobj.NativeMethod(sqlite3ResultColumnType)},
			"reset":       {Name: "reset", Method: phpobj.NativeMethod(sqlite3ResultReset)},
			"finalize":    {Name: "finalize", Method: phpobj.NativeMethod(sqlite3ResultFinalize)},
		},
	}
}

func getSQLite3ResultData(o *phpobj.ZObject) *sqlite3ResultData {
	if d, ok := o.GetOpaque(SQLite3ResultClass).(*sqlite3ResultData); ok {
		return d
	}
	return nil
}

func newSQLite3ResultObject(ctx phpv.Context, rows *sql.Rows) (*phpobj.ZObject, error) {
	cols, err := rows.Columns()
	if err != nil {
		return nil, err
	}
	colTypes, _ := rows.ColumnTypes()

	d := &sqlite3ResultData{
		rows:     rows,
		cols:     cols,
		colTypes: colTypes,
	}

	obj, err := phpobj.NewZObjectOpaque(ctx, SQLite3ResultClass, d)
	if err != nil {
		return nil, err
	}
	return obj, nil
}

// bufferAll reads all remaining rows into memory and closes the cursor.
func (d *sqlite3ResultData) bufferAll() error {
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

func (d *sqlite3ResultData) fetchNextRow(ctx phpv.Context) ([]interface{}, error) {
	if d.finalized {
		return nil, nil
	}

	if d.rows != nil {
		if d.rows.Next() {
			row := make([]interface{}, len(d.cols))
			ptrs := make([]interface{}, len(d.cols))
			for i := range row {
				ptrs[i] = &row[i]
			}
			if err := d.rows.Scan(ptrs...); err != nil {
				return nil, err
			}
			// Also store in buffer for reset support
			d.buffered = append(d.buffered, row)
			d.pos++
			return row, nil
		}
		d.rows.Close()
		d.rows = nil
		return nil, nil
	}

	if d.pos >= len(d.buffered) {
		return nil, nil
	}
	row := d.buffered[d.pos]
	d.pos++
	return row, nil
}

// sqliteValueToZVal converts a database/sql scanned value to a phpv.ZVal.
func sqliteValueToZVal(v interface{}) *phpv.ZVal {
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

func sqlite3ResultFetchArray(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	d := getSQLite3ResultData(o)
	if d == nil {
		return phpv.ZBool(false).ZVal(), nil
	}

	var mode core.Optional[phpv.ZInt]
	core.Expand(ctx, args, &mode)
	fetchMode := int(mode.GetOrDefault(phpv.ZInt(SQLITE3_BOTH)))

	row, err := d.fetchNextRow(ctx)
	if err != nil {
		return phpv.ZBool(false).ZVal(), nil
	}
	if row == nil {
		return phpv.ZBool(false).ZVal(), nil
	}

	arr := phpv.NewZArray()
	for i, col := range d.cols {
		v := sqliteValueToZVal(row[i])
		if fetchMode == SQLITE3_ASSOC || fetchMode == SQLITE3_BOTH {
			arr.OffsetSet(ctx, phpv.ZString(col), v)
		}
		if fetchMode == SQLITE3_NUM || fetchMode == SQLITE3_BOTH {
			arr.OffsetSet(ctx, phpv.ZInt(i), v)
		}
	}
	return arr.ZVal(), nil
}

func sqlite3ResultNumColumns(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	d := getSQLite3ResultData(o)
	if d == nil {
		return phpv.ZInt(0).ZVal(), nil
	}
	return phpv.ZInt(len(d.cols)).ZVal(), nil
}

func sqlite3ResultColumnName(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	d := getSQLite3ResultData(o)
	if d == nil {
		return phpv.ZBool(false).ZVal(), nil
	}

	var colIdx phpv.ZInt
	_, err := core.Expand(ctx, args, &colIdx)
	if err != nil {
		return nil, err
	}

	idx := int(colIdx)
	if idx < 0 || idx >= len(d.cols) {
		return phpv.ZBool(false).ZVal(), nil
	}
	return phpv.ZString(d.cols[idx]).ZVal(), nil
}

func sqlite3ResultColumnType(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	d := getSQLite3ResultData(o)
	if d == nil {
		return phpv.ZBool(false).ZVal(), nil
	}

	var colIdx phpv.ZInt
	_, err := core.Expand(ctx, args, &colIdx)
	if err != nil {
		return nil, err
	}

	idx := int(colIdx)
	if idx < 0 || idx >= len(d.cols) {
		return phpv.ZBool(false).ZVal(), nil
	}

	if d.colTypes != nil && idx < len(d.colTypes) {
		dbType := d.colTypes[idx].DatabaseTypeName()
		switch dbType {
		case "INTEGER", "INT", "TINYINT", "SMALLINT", "MEDIUMINT", "BIGINT", "INT2", "INT8":
			return phpv.ZInt(SQLITE3_INTEGER).ZVal(), nil
		case "REAL", "DOUBLE", "DOUBLE PRECISION", "FLOAT", "NUMERIC", "DECIMAL":
			return phpv.ZInt(SQLITE3_FLOAT).ZVal(), nil
		case "BLOB":
			return phpv.ZInt(SQLITE3_BLOB).ZVal(), nil
		case "NULL":
			return phpv.ZInt(SQLITE3_NULL).ZVal(), nil
		}
	}

	return phpv.ZInt(SQLITE3_TEXT).ZVal(), nil
}

func sqlite3ResultReset(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	d := getSQLite3ResultData(o)
	if d == nil {
		return phpv.ZBool(false).ZVal(), nil
	}

	// Buffer all remaining rows first
	if d.rows != nil {
		if err := d.bufferAll(); err != nil {
			return phpv.ZBool(false).ZVal(), nil
		}
	}

	d.pos = 0
	return phpv.ZBool(true).ZVal(), nil
}

func sqlite3ResultFinalize(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	d := getSQLite3ResultData(o)
	if d == nil {
		return phpv.ZBool(false).ZVal(), nil
	}
	if d.rows != nil {
		d.rows.Close()
		d.rows = nil
	}
	d.buffered = nil
	d.finalized = true
	return phpv.ZBool(true).ZVal(), nil
}
