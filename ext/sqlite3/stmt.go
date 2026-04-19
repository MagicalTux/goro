package sqlite3

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/KarpelesLab/goro/core"
	"github.com/KarpelesLab/goro/core/phpobj"
	"github.com/KarpelesLab/goro/core/phpv"
)

// SQLite3StmtClass is the PHP SQLite3Stmt class.
var SQLite3StmtClass *phpobj.ZClass

// sqlite3StmtData holds the state of a SQLite3Stmt object.
type sqlite3StmtData struct {
	stmt     *sql.Stmt
	query    string
	db       *sqlite3Data // parent DB, to update lastInsertRowID/changes
	// bindings maps parameter index (1-based) or name to value
	bindings map[interface{}]interface{}
	closed   bool
}

func initSQLite3StmtClass() {
	SQLite3StmtClass = &phpobj.ZClass{
		Name: "SQLite3Stmt",
		Methods: map[phpv.ZString]*phpv.ZClassMethod{
			"bindparam":  {Name: "bindParam", Method: phpobj.NativeMethod(sqlite3StmtBindParam)},
			"bindvalue":  {Name: "bindValue", Method: phpobj.NativeMethod(sqlite3StmtBindValue)},
			"execute":    {Name: "execute", Method: phpobj.NativeMethod(sqlite3StmtExecute)},
			"reset":      {Name: "reset", Method: phpobj.NativeMethod(sqlite3StmtReset)},
			"clear":      {Name: "clear", Method: phpobj.NativeMethod(sqlite3StmtClear)},
			"close":      {Name: "close", Method: phpobj.NativeMethod(sqlite3StmtClose)},
			"paramcount": {Name: "paramCount", Method: phpobj.NativeMethod(sqlite3StmtParamCount)},
			"readonly":   {Name: "readOnly", Method: phpobj.NativeMethod(sqlite3StmtReadOnly)},
			"getsql":     {Name: "getSQL", Method: phpobj.NativeMethod(sqlite3StmtGetSQL)},
		},
	}
}

func getSQLite3StmtData(o *phpobj.ZObject) *sqlite3StmtData {
	if d, ok := o.GetOpaque(SQLite3StmtClass).(*sqlite3StmtData); ok {
		return d
	}
	return nil
}

func newSQLite3StmtObject(ctx phpv.Context, stmt *sql.Stmt, query string, db *sqlite3Data) (*phpobj.ZObject, error) {
	d := &sqlite3StmtData{
		stmt:     stmt,
		query:    query,
		db:       db,
		bindings: make(map[interface{}]interface{}),
	}
	obj, err := phpobj.NewZObjectOpaque(ctx, SQLite3StmtClass, d)
	if err != nil {
		return nil, err
	}
	return obj, nil
}

// countSQLitePlaceholders counts ? placeholders in a SQLite query.
func countSQLitePlaceholders(query string) int {
	count := 0
	inString := false
	var stringChar byte
	for i := 0; i < len(query); i++ {
		c := query[i]
		if inString {
			if c == '\\' {
				i++
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

// buildArgs builds the positional args slice from bindings map.
// Supports both integer (1-based) and named (:name / @name / $name) params.
func (d *sqlite3StmtData) buildArgs() []interface{} {
	n := countSQLitePlaceholders(d.query)
	if n == 0 {
		return nil
	}
	args := make([]interface{}, n)
	for k, v := range d.bindings {
		switch idx := k.(type) {
		case int:
			if idx >= 1 && idx <= n {
				args[idx-1] = v
			}
		}
	}
	return args
}

func sqlite3ValueFromZVal(ctx phpv.Context, v *phpv.ZVal, sqlType int) interface{} {
	if v == nil || v.GetType() == phpv.ZtNull {
		return nil
	}
	switch sqlType {
	case SQLITE3_INTEGER:
		iv, _ := v.As(ctx, phpv.ZtInt)
		if iv != nil {
			return int64(iv.Value().(phpv.ZInt))
		}
		return int64(0)
	case SQLITE3_FLOAT:
		fv, _ := v.As(ctx, phpv.ZtFloat)
		if fv != nil {
			return float64(fv.Value().(phpv.ZFloat))
		}
		return float64(0)
	case SQLITE3_BLOB:
		sv, _ := v.As(ctx, phpv.ZtString)
		if sv != nil {
			return []byte(sv.Value().(phpv.ZString))
		}
		return []byte{}
	case SQLITE3_NULL:
		return nil
	default: // SQLITE3_TEXT
		sv, _ := v.As(ctx, phpv.ZtString)
		if sv != nil {
			return string(sv.Value().(phpv.ZString))
		}
		return ""
	}
}

func sqlite3StmtBindParam(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	d := getSQLite3StmtData(o)
	if d == nil || d.closed {
		return phpv.ZBool(false).ZVal(), nil
	}
	if len(args) < 2 {
		return phpv.ZBool(false).ZVal(), nil
	}

	var sqlType core.Optional[phpv.ZInt]
	core.Expand(ctx, []*phpv.ZVal{args[len(args)-1]}, &sqlType)
	typ := int(sqlType.GetOrDefault(phpv.ZInt(SQLITE3_TEXT)))

	// Determine parameter key
	paramVal := args[0]
	var key interface{}
	if paramVal.GetType() == phpv.ZtInt {
		key = int(paramVal.Value().(phpv.ZInt))
	} else {
		sv, _ := paramVal.As(ctx, phpv.ZtString)
		if sv != nil {
			key = string(sv.Value().(phpv.ZString))
		} else {
			key = 1
		}
	}

	var val interface{}
	if len(args) >= 2 {
		val = sqlite3ValueFromZVal(ctx, args[1], typ)
	}

	d.bindings[key] = val
	return phpv.ZBool(true).ZVal(), nil
}

func sqlite3StmtBindValue(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	d := getSQLite3StmtData(o)
	if d == nil || d.closed {
		return phpv.ZBool(false).ZVal(), nil
	}
	if len(args) < 2 {
		return phpv.ZBool(false).ZVal(), nil
	}

	var sqlType core.Optional[phpv.ZInt]
	if len(args) >= 3 {
		core.Expand(ctx, args[2:], &sqlType)
	}
	typ := int(sqlType.GetOrDefault(phpv.ZInt(SQLITE3_TEXT)))

	paramVal := args[0]
	var key interface{}
	if paramVal.GetType() == phpv.ZtInt {
		key = int(paramVal.Value().(phpv.ZInt))
	} else {
		sv, _ := paramVal.As(ctx, phpv.ZtString)
		if sv != nil {
			key = string(sv.Value().(phpv.ZString))
		} else {
			key = 1
		}
	}

	val := sqlite3ValueFromZVal(ctx, args[1], typ)
	d.bindings[key] = val
	return phpv.ZBool(true).ZVal(), nil
}

func sqlite3StmtExecute(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	d := getSQLite3StmtData(o)
	if d == nil || d.closed || d.stmt == nil {
		return phpv.ZBool(false).ZVal(), nil
	}

	execArgs := d.buildArgs()

	ifaces := make([]interface{}, len(execArgs))
	copy(ifaces, execArgs)

	// Determine if this is a SELECT-like query
	upper := strings.TrimSpace(strings.ToUpper(d.query))
	isSelect := strings.HasPrefix(upper, "SELECT") ||
		strings.HasPrefix(upper, "EXPLAIN") ||
		strings.HasPrefix(upper, "WITH")

	if isSelect {
		rows, err := d.stmt.QueryContext(context.Background(), ifaces...)
		if err != nil {
			return phpv.ZBool(false).ZVal(), nil
		}
		resObj, err := newSQLite3ResultObject(ctx, rows)
		if err != nil {
			rows.Close()
			return phpv.ZBool(false).ZVal(), nil
		}
		return resObj.ZVal(), nil
	}

	// For INSERT/UPDATE/DELETE/CREATE/DROP etc., use ExecContext (single execution).
	result, err := d.stmt.ExecContext(context.Background(), ifaces...)
	if err != nil {
		return phpv.ZBool(false).ZVal(), nil
	}

	affected, _ := result.RowsAffected()
	insertID, _ := result.LastInsertId()

	// Update parent DB state if available
	if d.db != nil {
		d.db.changes = affected
		d.db.lastInsertRowID = insertID
		d.db.lastErrCode = 0
		d.db.lastErrMsg = ""
	}

	// PHP's SQLite3Stmt::execute() returns SQLite3Result (with 0 cols) or false.
	// Return true for DML since we don't have a result set.
	return phpv.ZBool(true).ZVal(), nil
}

func sqlite3StmtReset(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	d := getSQLite3StmtData(o)
	if d == nil || d.closed {
		return phpv.ZBool(false).ZVal(), nil
	}
	// Reset just clears current execution state (bindings remain)
	return phpv.ZBool(true).ZVal(), nil
}

func sqlite3StmtClear(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	d := getSQLite3StmtData(o)
	if d == nil || d.closed {
		return phpv.ZBool(false).ZVal(), nil
	}
	d.bindings = make(map[interface{}]interface{})
	return phpv.ZBool(true).ZVal(), nil
}

func sqlite3StmtClose(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	d := getSQLite3StmtData(o)
	if d == nil {
		return phpv.ZBool(false).ZVal(), nil
	}
	if d.stmt != nil {
		d.stmt.Close()
		d.stmt = nil
	}
	d.closed = true
	return phpv.ZBool(true).ZVal(), nil
}

func sqlite3StmtParamCount(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	d := getSQLite3StmtData(o)
	if d == nil {
		return phpv.ZInt(0).ZVal(), nil
	}
	return phpv.ZInt(countSQLitePlaceholders(d.query)).ZVal(), nil
}

func sqlite3StmtReadOnly(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	d := getSQLite3StmtData(o)
	if d == nil {
		return phpv.ZBool(false).ZVal(), nil
	}
	upper := strings.TrimSpace(strings.ToUpper(d.query))
	isReadOnly := strings.HasPrefix(upper, "SELECT") ||
		strings.HasPrefix(upper, "EXPLAIN") ||
		strings.HasPrefix(upper, "PRAGMA")
	return phpv.ZBool(isReadOnly).ZVal(), nil
}

func sqlite3StmtGetSQL(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	d := getSQLite3StmtData(o)
	if d == nil {
		return phpv.ZBool(false).ZVal(), nil
	}

	var expanded core.Optional[phpv.ZBool]
	core.Expand(ctx, args, &expanded)

	if bool(expanded.GetOrDefault(phpv.ZBool(false))) {
		// Expanded: substitute bound values into the query
		query := d.query
		// Simple substitution for positional params
		result := expandQueryParams(query, d.bindings)
		return phpv.ZString(result).ZVal(), nil
	}

	return phpv.ZString(d.query).ZVal(), nil
}

// expandQueryParams replaces ? placeholders with their bound values.
func expandQueryParams(query string, bindings map[interface{}]interface{}) string {
	var sb strings.Builder
	paramIdx := 1
	inString := false
	var stringChar byte

	for i := 0; i < len(query); i++ {
		c := query[i]
		if inString {
			sb.WriteByte(c)
			if c == '\\' && i+1 < len(query) {
				i++
				sb.WriteByte(query[i])
			} else if c == stringChar {
				inString = false
			}
		} else if c == '\'' || c == '"' {
			inString = true
			stringChar = c
			sb.WriteByte(c)
		} else if c == '?' {
			if val, ok := bindings[paramIdx]; ok {
				sb.WriteString(formatSQLiteValue(val))
			} else {
				sb.WriteByte('?')
			}
			paramIdx++
		} else {
			sb.WriteByte(c)
		}
	}
	return sb.String()
}

func formatSQLiteValue(v interface{}) string {
	if v == nil {
		return "NULL"
	}
	switch val := v.(type) {
	case int64:
		return fmt.Sprintf("%d", val)
	case float64:
		return fmt.Sprintf("%g", val)
	case []byte:
		return "'" + strings.ReplaceAll(string(val), "'", "''") + "'"
	case string:
		return "'" + strings.ReplaceAll(val, "'", "''") + "'"
	default:
		return fmt.Sprintf("'%v'", val)
	}
}
