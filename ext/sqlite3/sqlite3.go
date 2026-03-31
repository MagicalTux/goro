package sqlite3

import (
	"context"
	"database/sql"
	"strings"

	"github.com/MagicalTux/goro/core"
	"github.com/MagicalTux/goro/core/phpobj"
	"github.com/MagicalTux/goro/core/phpv"
	_ "github.com/glebarez/go-sqlite"
)

// SQLite3Class is the PHP SQLite3 class.
var SQLite3Class *phpobj.ZClass

// sqlite3Data stores the connection state for a SQLite3 object.
type sqlite3Data struct {
	db              *sql.DB
	lastErrCode     int
	lastErrMsg      string
	lastInsertRowID int64
	changes         int64
	useExceptions   bool
}

func initSQLite3Class() {
	SQLite3Class = &phpobj.ZClass{
		Name: "SQLite3",
		Methods: map[phpv.ZString]*phpv.ZClassMethod{
			"__construct":      {Name: "__construct", Method: phpobj.NativeMethod(sqlite3Construct)},
			"open":             {Name: "open", Method: phpobj.NativeMethod(sqlite3Open)},
			"close":            {Name: "close", Method: phpobj.NativeMethod(sqlite3Close)},
			"exec":             {Name: "exec", Method: phpobj.NativeMethod(sqlite3Exec)},
			"query":            {Name: "query", Method: phpobj.NativeMethod(sqlite3Query)},
			"prepare":          {Name: "prepare", Method: phpobj.NativeMethod(sqlite3Prepare)},
			"querysingle":      {Name: "querySingle", Method: phpobj.NativeMethod(sqlite3QuerySingle)},
			"lastinsertrowid":  {Name: "lastInsertRowID", Method: phpobj.NativeMethod(sqlite3LastInsertRowID)},
			"lasterrorcode":    {Name: "lastErrorCode", Method: phpobj.NativeMethod(sqlite3LastErrorCode)},
			"lasterrormsg":     {Name: "lastErrorMsg", Method: phpobj.NativeMethod(sqlite3LastErrorMsg)},
			"changes":          {Name: "changes", Method: phpobj.NativeMethod(sqlite3Changes)},
			"escapestring":     {Name: "escapeString", Modifiers: phpv.ZAttrPublic | phpv.ZAttrStatic, Method: phpobj.NativeStaticMethod(sqlite3EscapeString)},
			"version":          {Name: "version", Modifiers: phpv.ZAttrPublic | phpv.ZAttrStatic, Method: phpobj.NativeStaticMethod(sqlite3Version)},
			"busytimeout":      {Name: "busyTimeout", Method: phpobj.NativeMethod(sqlite3BusyTimeout)},
			"enableexceptions": {Name: "enableExceptions", Method: phpobj.NativeMethod(sqlite3EnableExceptions)},
			"createfunction":   {Name: "createFunction", Method: phpobj.NativeMethod(sqlite3CreateFunction)},
		},
	}
}

func getSQLite3Data(o *phpobj.ZObject) *sqlite3Data {
	if d, ok := o.GetOpaque(SQLite3Class).(*sqlite3Data); ok {
		return d
	}
	return nil
}

func sqlite3Construct(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var filename phpv.ZString
	var flags core.Optional[phpv.ZInt]
	var encKey core.Optional[phpv.ZString]

	_, err := core.Expand(ctx, args, &filename, &flags, &encKey)
	if err != nil {
		return nil, err
	}

	d := &sqlite3Data{}
	o.SetOpaque(SQLite3Class, d)

	return sqlite3OpenDB(ctx, d, string(filename), int(flags.GetOrDefault(phpv.ZInt(SQLITE3_OPEN_READWRITE|SQLITE3_OPEN_CREATE))))
}

func sqlite3Open(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	d := getSQLite3Data(o)
	if d == nil {
		d = &sqlite3Data{}
		o.SetOpaque(SQLite3Class, d)
	}

	var filename phpv.ZString
	var flags core.Optional[phpv.ZInt]
	var encKey core.Optional[phpv.ZString]

	_, err := core.Expand(ctx, args, &filename, &flags, &encKey)
	if err != nil {
		return nil, err
	}

	// Close existing connection if any
	if d.db != nil {
		d.db.Close()
		d.db = nil
	}

	return sqlite3OpenDB(ctx, d, string(filename), int(flags.GetOrDefault(phpv.ZInt(SQLITE3_OPEN_READWRITE|SQLITE3_OPEN_CREATE))))
}

func sqlite3OpenDB(ctx phpv.Context, d *sqlite3Data, filename string, flags int) (*phpv.ZVal, error) {
	dsn := filename

	// Apply flags via query parameters
	params := []string{}
	if flags&SQLITE3_OPEN_READONLY != 0 {
		params = append(params, "_pragma=query_only(1)")
	}
	if len(params) > 0 {
		dsn = dsn + "?" + strings.Join(params, "&")
	}

	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		d.lastErrCode = 1
		d.lastErrMsg = err.Error()
		return phpv.ZBool(false).ZVal(), nil
	}

	if err := db.PingContext(context.Background()); err != nil {
		db.Close()
		d.lastErrCode = 1
		d.lastErrMsg = err.Error()
		return phpv.ZBool(false).ZVal(), nil
	}

	d.db = db
	return phpv.ZBool(true).ZVal(), nil
}

func sqlite3Close(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	d := getSQLite3Data(o)
	if d == nil {
		return phpv.ZBool(false).ZVal(), nil
	}
	if d.db != nil {
		d.db.Close()
		d.db = nil
	}
	return phpv.ZBool(true).ZVal(), nil
}

func sqlite3Exec(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	d := getSQLite3Data(o)
	if d == nil || d.db == nil {
		return phpv.ZBool(false).ZVal(), nil
	}

	var query phpv.ZString
	_, err := core.Expand(ctx, args, &query)
	if err != nil {
		return nil, err
	}

	result, err := d.db.ExecContext(context.Background(), string(query))
	if err != nil {
		d.lastErrCode = 1
		d.lastErrMsg = err.Error()
		return phpv.ZBool(false).ZVal(), nil
	}

	affected, _ := result.RowsAffected()
	insertID, _ := result.LastInsertId()
	d.changes = affected
	d.lastInsertRowID = insertID
	d.lastErrCode = 0
	d.lastErrMsg = ""

	return phpv.ZBool(true).ZVal(), nil
}

func sqlite3Query(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	d := getSQLite3Data(o)
	if d == nil || d.db == nil {
		return phpv.ZBool(false).ZVal(), nil
	}

	var query phpv.ZString
	_, err := core.Expand(ctx, args, &query)
	if err != nil {
		return nil, err
	}

	rows, err := d.db.QueryContext(context.Background(), string(query))
	if err != nil {
		d.lastErrCode = 1
		d.lastErrMsg = err.Error()
		return phpv.ZBool(false).ZVal(), nil
	}

	d.lastErrCode = 0
	d.lastErrMsg = ""

	resObj, err := newSQLite3ResultObject(ctx, rows)
	if err != nil {
		rows.Close()
		return phpv.ZBool(false).ZVal(), nil
	}
	return resObj.ZVal(), nil
}

func sqlite3Prepare(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	d := getSQLite3Data(o)
	if d == nil || d.db == nil {
		return phpv.ZBool(false).ZVal(), nil
	}

	var query phpv.ZString
	_, err := core.Expand(ctx, args, &query)
	if err != nil {
		return nil, err
	}

	stmt, err := d.db.PrepareContext(context.Background(), string(query))
	if err != nil {
		d.lastErrCode = 1
		d.lastErrMsg = err.Error()
		return phpv.ZBool(false).ZVal(), nil
	}

	d.lastErrCode = 0
	d.lastErrMsg = ""

	stmtObj, err := newSQLite3StmtObject(ctx, stmt, string(query), d)
	if err != nil {
		stmt.Close()
		return phpv.ZBool(false).ZVal(), nil
	}
	return stmtObj.ZVal(), nil
}

func sqlite3QuerySingle(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	d := getSQLite3Data(o)
	if d == nil || d.db == nil {
		return phpv.ZBool(false).ZVal(), nil
	}

	var query phpv.ZString
	var entireRow core.Optional[phpv.ZBool]
	_, err := core.Expand(ctx, args, &query, &entireRow)
	if err != nil {
		return nil, err
	}

	rows, err := d.db.QueryContext(context.Background(), string(query))
	if err != nil {
		d.lastErrCode = 1
		d.lastErrMsg = err.Error()
		return phpv.ZBool(false).ZVal(), nil
	}
	defer rows.Close()

	d.lastErrCode = 0
	d.lastErrMsg = ""

	cols, err := rows.Columns()
	if err != nil || len(cols) == 0 {
		return phpv.ZBool(false).ZVal(), nil
	}

	if !rows.Next() {
		return phpv.ZBool(false).ZVal(), nil
	}

	row := make([]interface{}, len(cols))
	ptrs := make([]interface{}, len(cols))
	for i := range row {
		ptrs[i] = &row[i]
	}
	if err := rows.Scan(ptrs...); err != nil {
		return phpv.ZBool(false).ZVal(), nil
	}

	if bool(entireRow.GetOrDefault(phpv.ZBool(false))) {
		arr := phpv.NewZArray()
		for i, col := range cols {
			arr.OffsetSet(ctx, phpv.ZString(col), sqliteValueToZVal(row[i]))
		}
		return arr.ZVal(), nil
	}

	return sqliteValueToZVal(row[0]), nil
}

func sqlite3LastInsertRowID(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	d := getSQLite3Data(o)
	if d == nil {
		return phpv.ZInt(0).ZVal(), nil
	}
	return phpv.ZInt(d.lastInsertRowID).ZVal(), nil
}

func sqlite3LastErrorCode(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	d := getSQLite3Data(o)
	if d == nil {
		return phpv.ZInt(0).ZVal(), nil
	}
	return phpv.ZInt(d.lastErrCode).ZVal(), nil
}

func sqlite3LastErrorMsg(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	d := getSQLite3Data(o)
	if d == nil {
		return phpv.ZString("").ZVal(), nil
	}
	return phpv.ZString(d.lastErrMsg).ZVal(), nil
}

func sqlite3Changes(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	d := getSQLite3Data(o)
	if d == nil {
		return phpv.ZInt(0).ZVal(), nil
	}
	return phpv.ZInt(d.changes).ZVal(), nil
}

func sqlite3EscapeString(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var s phpv.ZString
	_, err := core.Expand(ctx, args, &s)
	if err != nil {
		return nil, err
	}
	escaped := strings.ReplaceAll(string(s), "'", "''")
	return phpv.ZString(escaped).ZVal(), nil
}

func sqlite3Version(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		arr := phpv.NewZArray()
		arr.OffsetSet(ctx, phpv.ZString("versionString"), phpv.ZString("3.0.0").ZVal())
		arr.OffsetSet(ctx, phpv.ZString("versionNumber"), phpv.ZInt(3000000).ZVal())
		arr.OffsetSet(ctx, phpv.ZString("sourceID"), phpv.ZString("").ZVal())
		return arr.ZVal(), nil
	}
	defer db.Close()

	var versionStr string
	row := db.QueryRowContext(context.Background(), "SELECT sqlite_version()")
	row.Scan(&versionStr)

	versionNum := parseVersionNumber(versionStr)

	arr := phpv.NewZArray()
	arr.OffsetSet(ctx, phpv.ZString("versionString"), phpv.ZString(versionStr).ZVal())
	arr.OffsetSet(ctx, phpv.ZString("versionNumber"), phpv.ZInt(versionNum).ZVal())
	arr.OffsetSet(ctx, phpv.ZString("sourceID"), phpv.ZString("").ZVal())
	return arr.ZVal(), nil
}

func parseVersionNumber(version string) int {
	var major, minor, patch int
	parts := strings.SplitN(version, ".", 3)
	if len(parts) >= 1 {
		for _, c := range parts[0] {
			if c >= '0' && c <= '9' {
				major = major*10 + int(c-'0')
			}
		}
	}
	if len(parts) >= 2 {
		for _, c := range parts[1] {
			if c >= '0' && c <= '9' {
				minor = minor*10 + int(c-'0')
			}
		}
	}
	if len(parts) >= 3 {
		for _, c := range parts[2] {
			if c >= '0' && c <= '9' {
				patch = patch*10 + int(c-'0')
			}
		}
	}
	return major*1000000 + minor*1000 + patch
}

func sqlite3BusyTimeout(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	d := getSQLite3Data(o)
	if d == nil || d.db == nil {
		return phpv.ZBool(false).ZVal(), nil
	}

	var ms phpv.ZInt
	_, err := core.Expand(ctx, args, &ms)
	if err != nil {
		return nil, err
	}

	_, err = d.db.ExecContext(context.Background(), "PRAGMA busy_timeout = ?", int64(ms))
	if err != nil {
		return phpv.ZBool(false).ZVal(), nil
	}
	return phpv.ZBool(true).ZVal(), nil
}

func sqlite3EnableExceptions(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	d := getSQLite3Data(o)
	if d == nil {
		return phpv.ZBool(false).ZVal(), nil
	}

	var enable core.Optional[phpv.ZBool]
	core.Expand(ctx, args, &enable)

	old := d.useExceptions
	d.useExceptions = bool(enable.GetOrDefault(phpv.ZBool(false)))
	return phpv.ZBool(old).ZVal(), nil
}

func sqlite3CreateFunction(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	// createFunction requires CGO-level hooks in the SQLite driver;
	// with the pure-Go driver this is not fully supported.
	return phpv.ZBool(false).ZVal(), nil
}
