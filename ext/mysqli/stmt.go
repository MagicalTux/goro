package mysqli

import (
	"context"
	"database/sql"

	"github.com/MagicalTux/goro/core"
	"github.com/MagicalTux/goro/core/phpobj"
	"github.com/MagicalTux/goro/core/phpv"
)

// MysqliStmtClass is the PHP mysqli_stmt class.
var MysqliStmtClass *phpobj.ZClass

// mysqliStmtData holds the state of a mysqli_stmt object.
type mysqliStmtData struct {
	stmt         *sql.Stmt
	query        string
	params       []interface{}
	errno        int
	errMsg       string
	affectedRows int64
	insertID     int64
	numRows      int64
}

func initMysqliStmtClass() {
	MysqliStmtClass = &phpobj.ZClass{
		Name: "mysqli_stmt",
		Methods: map[phpv.ZString]*phpv.ZClassMethod{
			"bind_param":   {Name: "bind_param", Method: phpobj.NativeMethod(mysqliStmtBindParam)},
			"execute":      {Name: "execute", Method: phpobj.NativeMethod(mysqliStmtExecute)},
			"get_result":   {Name: "get_result", Method: phpobj.NativeMethod(mysqliStmtGetResult)},
			"fetch":        {Name: "fetch", Method: phpobj.NativeMethod(mysqliStmtFetch)},
			"bind_result":  {Name: "bind_result", Method: phpobj.NativeMethod(mysqliStmtBindResult)},
			"close":        {Name: "close", Method: phpobj.NativeMethod(mysqliStmtClose)},
			"__debuginfo":  {Name: "__debugInfo", Method: phpobj.NativeMethod(mysqliStmtDebugInfo)},
		},
		H: &phpv.ZClassHandlers{
			HandlePropGetEager: true,
			HandlePropGet: func(ctx phpv.Context, o phpv.ZObject, key phpv.ZString) (*phpv.ZVal, error) {
				zo, ok := o.(*phpobj.ZObject)
				if !ok {
					return nil, nil
				}
				d := getMysqliStmtData(zo)
				if d == nil {
					return nil, nil
				}
				switch key {
				case "affected_rows":
					return phpv.ZInt(d.affectedRows).ZVal(), nil
				case "insert_id":
					return phpv.ZInt(d.insertID).ZVal(), nil
				case "num_rows":
					return phpv.ZInt(d.numRows).ZVal(), nil
				case "param_count":
					return phpv.ZInt(countPlaceholders(d.query)).ZVal(), nil
				case "field_count":
					return phpv.ZInt(0).ZVal(), nil
				case "errno":
					return phpv.ZInt(d.errno).ZVal(), nil
				case "error":
					return phpv.ZString(d.errMsg).ZVal(), nil
				case "error_list":
					arr := phpv.NewZArray()
					if d.errMsg != "" {
						entry := phpv.NewZArray()
						entry.OffsetSet(ctx, phpv.ZString("errno"), phpv.ZInt(d.errno).ZVal())
						entry.OffsetSet(ctx, phpv.ZString("sqlstate"), phpv.ZString("HY000").ZVal())
						entry.OffsetSet(ctx, phpv.ZString("error"), phpv.ZString(d.errMsg).ZVal())
						arr.OffsetSet(ctx, nil, entry.ZVal())
					}
					return arr.ZVal(), nil
				case "id":
					return phpv.ZInt(0).ZVal(), nil
				}
				return nil, nil
			},
		},
	}
}

func getMysqliStmtData(o *phpobj.ZObject) *mysqliStmtData {
	if d, ok := o.GetOpaque(MysqliStmtClass).(*mysqliStmtData); ok {
		return d
	}
	return nil
}

// getMysqliStmtObj extracts a mysqli_stmt ZObject from the first arg.
func getMysqliStmtObj(ctx phpv.Context, args []*phpv.ZVal) (*phpobj.ZObject, error) {
	if len(args) < 1 || args[0] == nil {
		return nil, phpobj.ThrowError(ctx, phpobj.TypeError,
			"Argument #1 must be of type mysqli_stmt")
	}
	if args[0].GetType() != phpv.ZtObject {
		return nil, phpobj.ThrowError(ctx, phpobj.TypeError,
			"Argument #1 must be of type mysqli_stmt")
	}
	zo, ok := args[0].Value().(*phpobj.ZObject)
	if !ok {
		return nil, phpobj.ThrowError(ctx, phpobj.TypeError,
			"Argument #1 must be of type mysqli_stmt")
	}
	if getMysqliStmtData(zo) == nil {
		return nil, phpobj.ThrowError(ctx, phpobj.TypeError,
			"Argument #1 must be of type mysqli_stmt")
	}
	return zo, nil
}

// newMysqliStmtObject creates a new mysqli_stmt object wrapping sql.Stmt.
func newMysqliStmtObject(ctx phpv.Context, stmt *sql.Stmt, query string) (*phpobj.ZObject, error) {
	d := &mysqliStmtData{
		stmt:  stmt,
		query: query,
	}
	obj, err := phpobj.NewZObjectOpaque(ctx, MysqliStmtClass, d)
	if err != nil {
		return nil, err
	}
	return obj, nil
}

// countPlaceholders counts the number of ? placeholders in a query.
func countPlaceholders(query string) int {
	count := 0
	inString := false
	var stringChar byte
	for i := 0; i < len(query); i++ {
		c := query[i]
		if inString {
			if c == '\\' {
				i++ // skip escaped char
			} else if c == stringChar {
				inString = false
			}
		} else if c == '\'' || c == '"' {
			inString = true
			stringChar = c
		} else if c == '?' {
			count++
		}
	}
	return count
}

func mysqliStmtBindParam(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	d := getMysqliStmtData(o)
	if d == nil {
		return phpv.ZBool(false).ZVal(), nil
	}

	if len(args) < 1 {
		return phpv.ZBool(false).ZVal(), nil
	}

	// First arg is types string
	typesVal, err2 := args[0].As(ctx, phpv.ZtString)
	if err2 != nil {
		return phpv.ZBool(false).ZVal(), nil
	}
	types := string(typesVal.Value().(phpv.ZString))

	params := make([]interface{}, len(types))
	for i, t := range types {
		if i+1 >= len(args) || args[i+1] == nil {
			params[i] = nil
			continue
		}
		v := args[i+1]
		switch t {
		case 'i':
			iv, _ := v.As(ctx, phpv.ZtInt)
			if iv != nil {
				params[i] = int64(iv.Value().(phpv.ZInt))
			} else {
				params[i] = int64(0)
			}
		case 'd':
			fv, _ := v.As(ctx, phpv.ZtFloat)
			if fv != nil {
				params[i] = float64(fv.Value().(phpv.ZFloat))
			} else {
				params[i] = float64(0)
			}
		case 'b':
			sv, _ := v.As(ctx, phpv.ZtString)
			if sv != nil {
				params[i] = []byte(sv.Value().(phpv.ZString))
			} else {
				params[i] = []byte{}
			}
		default: // 's' and everything else → string
			sv, _ := v.As(ctx, phpv.ZtString)
			if sv != nil {
				params[i] = string(sv.Value().(phpv.ZString))
			} else {
				params[i] = ""
			}
		}
	}

	d.params = params
	return phpv.ZBool(true).ZVal(), nil
}

func mysqliStmtExecute(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	d := getMysqliStmtData(o)
	if d == nil || d.stmt == nil {
		return phpv.ZBool(false).ZVal(), nil
	}

	d.errno = 0
	d.errMsg = ""

	// Check if params were passed directly (PHP 8.1 style)
	var execParams []interface{}
	if len(args) > 0 && args[0] != nil && args[0].GetType() == phpv.ZtArray {
		arr := args[0].Value().(*phpv.ZArray)
		for _, v := range arr.Iterate(ctx) {
			if v == nil {
				execParams = append(execParams, nil)
				continue
			}
			sv, _ := v.As(ctx, phpv.ZtString)
			if sv != nil {
				execParams = append(execParams, string(sv.Value().(phpv.ZString)))
			} else {
				execParams = append(execParams, nil)
			}
		}
	} else {
		execParams = d.params
	}

	// Determine if this is a SELECT by checking rows
	rows, err := d.stmt.QueryContext(context.Background(), execParams...)
	if err != nil {
		// Could be a non-SELECT statement; try Exec
		result, err2 := d.stmt.ExecContext(context.Background(), execParams...)
		if err2 != nil {
			d.errno = 1064
			d.errMsg = err2.Error()
			return phpv.ZBool(false).ZVal(), nil
		}
		affected, _ := result.RowsAffected()
		insertID, _ := result.LastInsertId()
		d.affectedRows = affected
		d.insertID = insertID
		return phpv.ZBool(true).ZVal(), nil
	}
	rows.Close()

	// For SELECT-returning statements, use ExecContext for metadata
	result, err2 := d.stmt.ExecContext(context.Background(), execParams...)
	if err2 == nil {
		affected, _ := result.RowsAffected()
		insertID, _ := result.LastInsertId()
		d.affectedRows = affected
		d.insertID = insertID
	}

	return phpv.ZBool(true).ZVal(), nil
}

func mysqliStmtGetResult(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	d := getMysqliStmtData(o)
	if d == nil || d.stmt == nil {
		return phpv.ZBool(false).ZVal(), nil
	}

	d.errno = 0
	d.errMsg = ""

	rows, err := d.stmt.QueryContext(context.Background(), d.params...)
	if err != nil {
		d.errno = 1064
		d.errMsg = err.Error()
		return phpv.ZBool(false).ZVal(), nil
	}

	resObj, err := newMysqliResultObject(ctx, rows)
	if err != nil {
		rows.Close()
		return phpv.ZBool(false).ZVal(), nil
	}
	return resObj.ZVal(), nil
}

func mysqliStmtFetch(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	// fetch() is for bound results via bind_result; return null as not fully supported
	return phpv.ZNULL.ZVal(), nil
}

func mysqliStmtBindResult(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	// bind_result is for the older fetch() API; mark as success
	return phpv.ZBool(true).ZVal(), nil
}

func mysqliStmtClose(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	d := getMysqliStmtData(o)
	if d == nil {
		return phpv.ZBool(false).ZVal(), nil
	}
	if d.stmt != nil {
		d.stmt.Close()
		d.stmt = nil
	}
	return phpv.ZBool(true).ZVal(), nil
}

func mysqliStmtDebugInfo(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	d := getMysqliStmtData(o)
	arr := phpv.NewZArray()
	if d != nil {
		arr.OffsetSet(ctx, phpv.ZString("affected_rows"), phpv.ZInt(d.affectedRows).ZVal())
		arr.OffsetSet(ctx, phpv.ZString("insert_id"), phpv.ZInt(d.insertID).ZVal())
		arr.OffsetSet(ctx, phpv.ZString("num_rows"), phpv.ZInt(d.numRows).ZVal())
		arr.OffsetSet(ctx, phpv.ZString("param_count"), phpv.ZInt(countPlaceholders(d.query)).ZVal())
		arr.OffsetSet(ctx, phpv.ZString("field_count"), phpv.ZInt(0).ZVal())
		arr.OffsetSet(ctx, phpv.ZString("errno"), phpv.ZInt(d.errno).ZVal())
		arr.OffsetSet(ctx, phpv.ZString("error"), phpv.ZString(d.errMsg).ZVal())
		arr.OffsetSet(ctx, phpv.ZString("error_list"), phpv.NewZArray().ZVal())
		arr.OffsetSet(ctx, phpv.ZString("sqlstate"), phpv.ZString("00000").ZVal())
		arr.OffsetSet(ctx, phpv.ZString("id"), phpv.ZInt(0).ZVal())
	}
	return arr.ZVal(), nil
}

// mysqliStmtExecuteOpt is used by the procedural wrapper when we need args.
func mysqliStmtExecuteOpt(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var optParams core.Optional[*phpv.ZArray]
	core.Expand(ctx, args, &optParams)
	if optParams.HasArg() {
		newArgs := []*phpv.ZVal{optParams.Get().ZVal()}
		return mysqliStmtExecute(ctx, o, newArgs)
	}
	return mysqliStmtExecute(ctx, o, nil)
}
