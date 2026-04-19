package mysqli

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/KarpelesLab/goro/core"
	"github.com/KarpelesLab/goro/core/phpobj"
	"github.com/KarpelesLab/goro/core/phpv"
	_ "github.com/go-sql-driver/mysql"
)

// MysqliClass is the PHP mysqli class.
var MysqliClass *phpobj.ZClass

// mysqliData stores the connection state for a mysqli object.
type mysqliData struct {
	db           *sql.DB
	connectErrno int
	connectError string
	errno        int
	errMsg       string
	affectedRows int64
	insertID     int64
	info         string
	host         string
	user         string
}

func initMysqliClass() {
	MysqliClass = &phpobj.ZClass{
		Name: "mysqli",
		Methods: map[phpv.ZString]*phpv.ZClassMethod{
			"__construct":      {Name: "__construct", Method: phpobj.NativeMethod(mysqliConstruct)},
			"query":            {Name: "query", Method: phpobj.NativeMethod(mysqliQuery)},
			"prepare":          {Name: "prepare", Method: phpobj.NativeMethod(mysqliPrepare)},
			"real_escape_string": {Name: "real_escape_string", Method: phpobj.NativeMethod(mysqliRealEscapeString)},
			"escape_string":    {Name: "escape_string", Method: phpobj.NativeMethod(mysqliRealEscapeString)},
			"close":            {Name: "close", Method: phpobj.NativeMethod(mysqliClose)},
			"select_db":        {Name: "select_db", Method: phpobj.NativeMethod(mysqliSelectDb)},
			"set_charset":      {Name: "set_charset", Method: phpobj.NativeMethod(mysqliSetCharset)},
			"autocommit":       {Name: "autocommit", Method: phpobj.NativeMethod(mysqliAutocommit)},
			"begin_transaction": {Name: "begin_transaction", Method: phpobj.NativeMethod(mysqliBeginTransaction)},
			"commit":           {Name: "commit", Method: phpobj.NativeMethod(mysqliCommit)},
			"rollback":         {Name: "rollback", Method: phpobj.NativeMethod(mysqliRollback)},
			"ping":             {Name: "ping", Method: phpobj.NativeMethod(mysqliPing)},
			"__debuginfo":      {Name: "__debugInfo", Method: phpobj.NativeMethod(mysqliDebugInfo)},
		},
		H: &phpv.ZClassHandlers{
			HandlePropGetEager: true,
			HandlePropGet: func(ctx phpv.Context, o phpv.ZObject, key phpv.ZString) (*phpv.ZVal, error) {
				zo, ok := o.(*phpobj.ZObject)
				if !ok {
					return nil, nil
				}
				d := getMysqliData(zo)
				if d == nil {
					return nil, nil
				}
				return mysqliGetProp(ctx, d, key)
			},
		},
	}
}

func getMysqliData(o *phpobj.ZObject) *mysqliData {
	if d, ok := o.GetOpaque(MysqliClass).(*mysqliData); ok {
		return d
	}
	return nil
}

func mysqliGetProp(ctx phpv.Context, d *mysqliData, key phpv.ZString) (*phpv.ZVal, error) {
	switch key {
	case "connect_errno":
		return phpv.ZInt(d.connectErrno).ZVal(), nil
	case "connect_error":
		if d.connectError == "" {
			return phpv.ZNULL.ZVal(), nil
		}
		return phpv.ZString(d.connectError).ZVal(), nil
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
	case "affected_rows":
		if d.db == nil {
			return phpv.ZInt(-1).ZVal(), nil
		}
		return phpv.ZInt(d.affectedRows).ZVal(), nil
	case "insert_id":
		return phpv.ZInt(d.insertID).ZVal(), nil
	case "info":
		if d.info == "" {
			return phpv.ZNULL.ZVal(), nil
		}
		return phpv.ZString(d.info).ZVal(), nil
	case "warning_count":
		return phpv.ZInt(0).ZVal(), nil
	case "field_count":
		return phpv.ZInt(0).ZVal(), nil
	case "host_info":
		if d.db == nil {
			return phpv.ZString("").ZVal(), nil
		}
		return phpv.ZString(fmt.Sprintf("%s via TCP/IP", d.host)).ZVal(), nil
	case "server_info":
		if d.db == nil {
			return phpv.ZString("").ZVal(), nil
		}
		var version string
		row := d.db.QueryRowContext(context.Background(), "SELECT VERSION()")
		row.Scan(&version)
		return phpv.ZString(version).ZVal(), nil
	case "server_version":
		if d.db == nil {
			return phpv.ZInt(0).ZVal(), nil
		}
		var version string
		row := d.db.QueryRowContext(context.Background(), "SELECT VERSION()")
		row.Scan(&version)
		// convert "5.7.38" to integer 50738
		parts := strings.SplitN(version, ".", 3)
		var major, minor, patch int
		if len(parts) >= 1 {
			fmt.Sscanf(parts[0], "%d", &major)
		}
		if len(parts) >= 2 {
			fmt.Sscanf(parts[1], "%d", &minor)
		}
		if len(parts) >= 3 {
			fmt.Sscanf(parts[2], "%d", &patch)
		}
		return phpv.ZInt(major*10000 + minor*100 + patch).ZVal(), nil
	case "client_info":
		return phpv.ZString("mysqlnd goro/1.0").ZVal(), nil
	case "client_version":
		return phpv.ZInt(50700).ZVal(), nil
	case "stat":
		if d.db == nil {
			return phpv.ZBool(false).ZVal(), nil
		}
		var status string
		row := d.db.QueryRowContext(context.Background(), "SHOW STATUS LIKE 'Uptime'")
		var name string
		row.Scan(&name, &status)
		return phpv.ZString("Uptime: " + status).ZVal(), nil
	}
	return nil, nil
}

func mysqliConstruct(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var hostname core.Optional[phpv.ZString]
	var username core.Optional[phpv.ZString]
	var password core.Optional[phpv.ZString]
	var database core.Optional[phpv.ZString]
	var port core.Optional[phpv.ZInt]

	core.Expand(ctx, args, &hostname, &username, &password, &database, &port)

	d := &mysqliData{}
	o.SetOpaque(MysqliClass, d)

	host := string(hostname.GetOrDefault("localhost"))
	user := string(username.GetOrDefault(""))
	pass := string(password.GetOrDefault(""))
	db := string(database.GetOrDefault(""))
	portNum := int(port.GetOrDefault(3306))

	// Handle null hostname as "not connecting"
	if len(args) > 0 && args[0] != nil && args[0].GetType() == phpv.ZtNull {
		return nil, nil
	}

	d.host = host
	d.user = user

	dsn := fmt.Sprintf("%s:%s@tcp(%s:%d)/%s", user, pass, host, portNum, db)

	conn, err := sql.Open("mysql", dsn)
	if err != nil {
		d.connectErrno = 2002
		d.connectError = err.Error()
		return nil, nil
	}

	// Ping to verify connection
	if err := conn.PingContext(context.Background()); err != nil {
		conn.Close()
		d.connectErrno = 2002
		d.connectError = err.Error()
		return nil, nil
	}

	d.db = conn
	return nil, nil
}

func mysqliQuery(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	d := getMysqliData(o)
	if d == nil || d.db == nil {
		return phpv.ZBool(false).ZVal(), nil
	}

	var query phpv.ZString
	_, err := core.Expand(ctx, args, &query)
	if err != nil {
		return nil, err
	}

	// Reset error state
	d.errno = 0
	d.errMsg = ""
	d.info = ""

	queryStr := string(query)

	// Determine if it's a SELECT-like query (returns rows)
	upperQ := strings.TrimSpace(strings.ToUpper(queryStr))
	isSelect := strings.HasPrefix(upperQ, "SELECT") ||
		strings.HasPrefix(upperQ, "SHOW") ||
		strings.HasPrefix(upperQ, "DESCRIBE") ||
		strings.HasPrefix(upperQ, "EXPLAIN") ||
		strings.HasPrefix(upperQ, "DESC") ||
		strings.HasPrefix(upperQ, "WITH")

	if isSelect {
		rows, err := d.db.QueryContext(context.Background(), queryStr)
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

	// DML query
	result, err := d.db.ExecContext(context.Background(), queryStr)
	if err != nil {
		d.errno = 1064
		d.errMsg = err.Error()
		return phpv.ZBool(false).ZVal(), nil
	}

	affected, _ := result.RowsAffected()
	insertID, _ := result.LastInsertId()
	d.affectedRows = affected
	d.insertID = insertID

	return phpv.ZBool(true).ZVal(), nil
}

func mysqliPrepare(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	d := getMysqliData(o)
	if d == nil || d.db == nil {
		return phpv.ZBool(false).ZVal(), nil
	}

	var query phpv.ZString
	_, err := core.Expand(ctx, args, &query)
	if err != nil {
		return nil, err
	}

	d.errno = 0
	d.errMsg = ""

	stmt, err := d.db.PrepareContext(context.Background(), string(query))
	if err != nil {
		d.errno = 1064
		d.errMsg = err.Error()
		return phpv.ZBool(false).ZVal(), nil
	}

	stmtObj, err := newMysqliStmtObject(ctx, stmt, string(query))
	if err != nil {
		stmt.Close()
		return phpv.ZBool(false).ZVal(), nil
	}
	return stmtObj.ZVal(), nil
}

func mysqliRealEscapeString(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var s phpv.ZString
	_, err := core.Expand(ctx, args, &s)
	if err != nil {
		return nil, err
	}
	return phpv.ZString(mysqlEscape(string(s))).ZVal(), nil
}

// mysqlEscape escapes special characters in a string for use in SQL.
func mysqlEscape(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch c {
		case 0:
			b.WriteString(`\0`)
		case '\n':
			b.WriteString(`\n`)
		case '\r':
			b.WriteString(`\r`)
		case '\\':
			b.WriteString(`\\`)
		case '\'':
			b.WriteString(`\'`)
		case '"':
			b.WriteString(`\"`)
		case 26: // \x1a (Ctrl+Z / SUB)
			b.WriteString(`\Z`)
		default:
			b.WriteByte(c)
		}
	}
	return b.String()
}

func mysqliClose(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	d := getMysqliData(o)
	if d == nil {
		return phpv.ZBool(false).ZVal(), nil
	}
	if d.db != nil {
		d.db.Close()
		d.db = nil
	}
	return phpv.ZBool(true).ZVal(), nil
}

func mysqliSelectDb(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	d := getMysqliData(o)
	if d == nil || d.db == nil {
		return phpv.ZBool(false).ZVal(), nil
	}

	var dbName phpv.ZString
	_, err := core.Expand(ctx, args, &dbName)
	if err != nil {
		return nil, err
	}

	_, err = d.db.ExecContext(context.Background(), "USE `"+string(dbName)+"`")
	if err != nil {
		d.errno = 1049
		d.errMsg = err.Error()
		return phpv.ZBool(false).ZVal(), nil
	}
	return phpv.ZBool(true).ZVal(), nil
}

func mysqliSetCharset(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	d := getMysqliData(o)
	if d == nil || d.db == nil {
		return phpv.ZBool(false).ZVal(), nil
	}

	var charset phpv.ZString
	_, err := core.Expand(ctx, args, &charset)
	if err != nil {
		return nil, err
	}

	_, err = d.db.ExecContext(context.Background(), "SET NAMES "+string(charset))
	if err != nil {
		d.errno = 2019
		d.errMsg = err.Error()
		return phpv.ZBool(false).ZVal(), nil
	}
	return phpv.ZBool(true).ZVal(), nil
}

func mysqliAutocommit(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	d := getMysqliData(o)
	if d == nil || d.db == nil {
		return phpv.ZBool(false).ZVal(), nil
	}

	var enable phpv.ZBool
	_, err := core.Expand(ctx, args, &enable)
	if err != nil {
		return nil, err
	}

	val := "1"
	if !bool(enable) {
		val = "0"
	}
	_, err = d.db.ExecContext(context.Background(), "SET autocommit="+val)
	if err != nil {
		d.errno = 1
		d.errMsg = err.Error()
		return phpv.ZBool(false).ZVal(), nil
	}
	return phpv.ZBool(true).ZVal(), nil
}

func mysqliBeginTransaction(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	d := getMysqliData(o)
	if d == nil || d.db == nil {
		return phpv.ZBool(false).ZVal(), nil
	}

	_, err := d.db.ExecContext(context.Background(), "START TRANSACTION")
	if err != nil {
		d.errno = 1
		d.errMsg = err.Error()
		return phpv.ZBool(false).ZVal(), nil
	}
	return phpv.ZBool(true).ZVal(), nil
}

func mysqliCommit(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	d := getMysqliData(o)
	if d == nil || d.db == nil {
		return phpv.ZBool(false).ZVal(), nil
	}

	_, err := d.db.ExecContext(context.Background(), "COMMIT")
	if err != nil {
		d.errno = 1
		d.errMsg = err.Error()
		return phpv.ZBool(false).ZVal(), nil
	}
	return phpv.ZBool(true).ZVal(), nil
}

func mysqliRollback(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	d := getMysqliData(o)
	if d == nil || d.db == nil {
		return phpv.ZBool(false).ZVal(), nil
	}

	_, err := d.db.ExecContext(context.Background(), "ROLLBACK")
	if err != nil {
		d.errno = 1
		d.errMsg = err.Error()
		return phpv.ZBool(false).ZVal(), nil
	}
	return phpv.ZBool(true).ZVal(), nil
}

func mysqliPing(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	d := getMysqliData(o)
	if d == nil || d.db == nil {
		return phpv.ZBool(false).ZVal(), nil
	}

	err := d.db.PingContext(context.Background())
	if err != nil {
		// Try to reconnect (basic reconnect attempt)
		d.errno = 2006
		d.errMsg = err.Error()
		return phpv.ZBool(false).ZVal(), nil
	}
	return phpv.ZBool(true).ZVal(), nil
}

func mysqliDebugInfo(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	d := getMysqliData(o)
	arr := phpv.NewZArray()
	if d != nil {
		arr.OffsetSet(ctx, phpv.ZString("affected_rows"), phpv.ZInt(d.affectedRows).ZVal())
		arr.OffsetSet(ctx, phpv.ZString("client_info"), phpv.ZString("mysqlnd goro/1.0").ZVal())
		arr.OffsetSet(ctx, phpv.ZString("client_version"), phpv.ZInt(50700).ZVal())
		arr.OffsetSet(ctx, phpv.ZString("connect_errno"), phpv.ZInt(d.connectErrno).ZVal())
		if d.connectError == "" {
			arr.OffsetSet(ctx, phpv.ZString("connect_error"), phpv.ZNULL.ZVal())
		} else {
			arr.OffsetSet(ctx, phpv.ZString("connect_error"), phpv.ZString(d.connectError).ZVal())
		}
		arr.OffsetSet(ctx, phpv.ZString("errno"), phpv.ZInt(d.errno).ZVal())
		arr.OffsetSet(ctx, phpv.ZString("error"), phpv.ZString(d.errMsg).ZVal())
		arr.OffsetSet(ctx, phpv.ZString("field_count"), phpv.ZInt(0).ZVal())
		arr.OffsetSet(ctx, phpv.ZString("host_info"), phpv.ZString(fmt.Sprintf("%s via TCP/IP", d.host)).ZVal())
		arr.OffsetSet(ctx, phpv.ZString("info"), phpv.ZNULL.ZVal())
		arr.OffsetSet(ctx, phpv.ZString("insert_id"), phpv.ZInt(d.insertID).ZVal())
		arr.OffsetSet(ctx, phpv.ZString("server_info"), phpv.ZString("").ZVal())
		arr.OffsetSet(ctx, phpv.ZString("server_version"), phpv.ZInt(0).ZVal())
		arr.OffsetSet(ctx, phpv.ZString("stat"), phpv.ZBool(false).ZVal())
		arr.OffsetSet(ctx, phpv.ZString("sqlstate"), phpv.ZString("00000").ZVal())
		arr.OffsetSet(ctx, phpv.ZString("protocol_version"), phpv.ZInt(10).ZVal())
		arr.OffsetSet(ctx, phpv.ZString("thread_id"), phpv.ZInt(0).ZVal())
		arr.OffsetSet(ctx, phpv.ZString("warning_count"), phpv.ZInt(0).ZVal())
	}
	return arr.ZVal(), nil
}

// ---- Procedural function wrappers ----

func fncMysqliConnect(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	obj, err := phpobj.NewZObject(ctx, MysqliClass)
	if err != nil {
		return phpv.ZBool(false).ZVal(), nil
	}
	d := &mysqliData{}
	obj.SetOpaque(MysqliClass, d)

	// Call constructor logic directly
	_, err = mysqliConstruct(ctx, obj, args)
	if err != nil {
		return phpv.ZBool(false).ZVal(), nil
	}

	return obj.ZVal(), nil
}

func getMysqliObj(ctx phpv.Context, args []*phpv.ZVal) (*phpobj.ZObject, *mysqliData, error) {
	if len(args) < 1 || args[0] == nil {
		return nil, nil, phpobj.ThrowError(ctx, phpobj.TypeError,
			"Argument #1 must be of type mysqli")
	}
	if args[0].GetType() != phpv.ZtObject {
		return nil, nil, phpobj.ThrowError(ctx, phpobj.TypeError,
			"Argument #1 must be of type mysqli")
	}
	zo, ok := args[0].Value().(*phpobj.ZObject)
	if !ok {
		return nil, nil, phpobj.ThrowError(ctx, phpobj.TypeError,
			"Argument #1 must be of type mysqli")
	}
	d := getMysqliData(zo)
	if d == nil {
		return nil, nil, phpobj.ThrowError(ctx, phpobj.TypeError,
			"Argument #1 must be of type mysqli")
	}
	return zo, d, nil
}

func fncMysqliQuery(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	zo, _, err := getMysqliObj(ctx, args)
	if err != nil {
		return nil, err
	}
	return mysqliQuery(ctx, zo, args[1:])
}

func fncMysqliFetchAssoc(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	zo, err := getMysqliResultObj(ctx, args)
	if err != nil {
		return nil, err
	}
	return mysqliFetchAssoc(ctx, zo, nil)
}

func fncMysqliFetchArray(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	zo, err := getMysqliResultObj(ctx, args)
	if err != nil {
		return nil, err
	}
	return mysqliFetchArray(ctx, zo, args[1:])
}

func fncMysqliFetchRow(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	zo, err := getMysqliResultObj(ctx, args)
	if err != nil {
		return nil, err
	}
	return mysqliFetchRow(ctx, zo, nil)
}

func fncMysqliFetchAll(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	zo, err := getMysqliResultObj(ctx, args)
	if err != nil {
		return nil, err
	}
	return mysqliFetchAll(ctx, zo, args[1:])
}

func fncMysqliNumRows(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	zo, err := getMysqliResultObj(ctx, args)
	if err != nil {
		return nil, err
	}
	d := getMysqliResultData(zo)
	if d == nil {
		return phpv.ZInt(0).ZVal(), nil
	}
	if err := d.bufferAll(ctx); err != nil {
		return phpv.ZInt(0).ZVal(), nil
	}
	return phpv.ZInt(len(d.buffered)).ZVal(), nil
}

func fncMysqliAffectedRows(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	_, d, err := getMysqliObj(ctx, args)
	if err != nil {
		return nil, err
	}
	if d.db == nil {
		return phpv.ZInt(-1).ZVal(), nil
	}
	return phpv.ZInt(d.affectedRows).ZVal(), nil
}

func fncMysqliInsertId(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	_, d, err := getMysqliObj(ctx, args)
	if err != nil {
		return nil, err
	}
	return phpv.ZInt(d.insertID).ZVal(), nil
}

func fncMysqliError(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	_, d, err := getMysqliObj(ctx, args)
	if err != nil {
		return nil, err
	}
	return phpv.ZString(d.errMsg).ZVal(), nil
}

func fncMysqliErrno(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	_, d, err := getMysqliObj(ctx, args)
	if err != nil {
		return nil, err
	}
	return phpv.ZInt(d.errno).ZVal(), nil
}

func fncMysqliClose(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	zo, _, err := getMysqliObj(ctx, args)
	if err != nil {
		return nil, err
	}
	return mysqliClose(ctx, zo, nil)
}

func fncMysqliRealEscapeString(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	_, _, err := getMysqliObj(ctx, args)
	if err != nil {
		return nil, err
	}
	if len(args) < 2 || args[1] == nil {
		return phpv.ZBool(false).ZVal(), nil
	}
	sv, err2 := args[1].As(ctx, phpv.ZtString)
	if err2 != nil {
		return phpv.ZBool(false).ZVal(), nil
	}
	s := string(sv.Value().(phpv.ZString))
	return phpv.ZString(mysqlEscape(s)).ZVal(), nil
}

func fncMysqliPrepare(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	zo, _, err := getMysqliObj(ctx, args)
	if err != nil {
		return nil, err
	}
	return mysqliPrepare(ctx, zo, args[1:])
}

func fncMysqliStmtExecute(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	zo, err := getMysqliStmtObj(ctx, args)
	if err != nil {
		return nil, err
	}
	return mysqliStmtExecute(ctx, zo, nil)
}

func fncMysqliStmtGetResult(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	zo, err := getMysqliStmtObj(ctx, args)
	if err != nil {
		return nil, err
	}
	return mysqliStmtGetResult(ctx, zo, nil)
}

func fncMysqliStmtClose(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	zo, err := getMysqliStmtObj(ctx, args)
	if err != nil {
		return nil, err
	}
	return mysqliStmtClose(ctx, zo, nil)
}

func fncMysqliFreeResult(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	zo, err := getMysqliResultObj(ctx, args)
	if err != nil {
		return nil, err
	}
	return mysqliFree(ctx, zo, nil)
}

func fncMysqliSelectDb(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	zo, _, err := getMysqliObj(ctx, args)
	if err != nil {
		return nil, err
	}
	return mysqliSelectDb(ctx, zo, args[1:])
}

func fncMysqliSetCharset(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	zo, _, err := getMysqliObj(ctx, args)
	if err != nil {
		return nil, err
	}
	return mysqliSetCharset(ctx, zo, args[1:])
}

func fncMysqliBeginTransaction(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	zo, _, err := getMysqliObj(ctx, args)
	if err != nil {
		return nil, err
	}
	return mysqliBeginTransaction(ctx, zo, nil)
}

func fncMysqliCommit(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	zo, _, err := getMysqliObj(ctx, args)
	if err != nil {
		return nil, err
	}
	return mysqliCommit(ctx, zo, nil)
}

func fncMysqliRollback(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	zo, _, err := getMysqliObj(ctx, args)
	if err != nil {
		return nil, err
	}
	return mysqliRollback(ctx, zo, nil)
}

func fncMysqliAutocommit(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	zo, _, err := getMysqliObj(ctx, args)
	if err != nil {
		return nil, err
	}
	return mysqliAutocommit(ctx, zo, args[1:])
}

func fncMysqliPing(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	zo, _, err := getMysqliObj(ctx, args)
	if err != nil {
		return nil, err
	}
	return mysqliPing(ctx, zo, nil)
}
