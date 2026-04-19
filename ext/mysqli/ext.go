package mysqli

import (
	"github.com/KarpelesLab/goro/core"
	"github.com/KarpelesLab/goro/core/phpctx"
	"github.com/KarpelesLab/goro/core/phpobj"
	"github.com/KarpelesLab/goro/core/phpv"
)

// mysqli fetch mode constants
const (
	MYSQLI_ASSOC         = 1
	MYSQLI_NUM           = 2
	MYSQLI_BOTH          = 3
	MYSQLI_STORE_RESULT  = 0
	MYSQLI_USE_RESULT    = 1
	MYSQLI_CLIENT_COMPRESS = 32
	MYSQLI_CLIENT_SSL    = 2048
	MYSQLI_REPORT_OFF    = 0
	MYSQLI_REPORT_ERROR  = 1
	MYSQLI_REPORT_STRICT = 2
	MYSQLI_REPORT_ALL    = 255
)

// MYSQLI_TYPE_* constants (MySQL field type IDs)
const (
	MYSQLI_TYPE_DECIMAL     = 0
	MYSQLI_TYPE_TINY        = 1
	MYSQLI_TYPE_SHORT       = 2
	MYSQLI_TYPE_LONG        = 3
	MYSQLI_TYPE_FLOAT       = 4
	MYSQLI_TYPE_DOUBLE      = 5
	MYSQLI_TYPE_NULL        = 6
	MYSQLI_TYPE_TIMESTAMP   = 7
	MYSQLI_TYPE_LONGLONG    = 8
	MYSQLI_TYPE_INT24       = 9
	MYSQLI_TYPE_DATE        = 10
	MYSQLI_TYPE_TIME        = 11
	MYSQLI_TYPE_DATETIME    = 12
	MYSQLI_TYPE_YEAR        = 13
	MYSQLI_TYPE_NEWDATE     = 14
	MYSQLI_TYPE_VARCHAR     = 15
	MYSQLI_TYPE_BIT         = 16
	MYSQLI_TYPE_NEWDECIMAL  = 246
	MYSQLI_TYPE_ENUM        = 247
	MYSQLI_TYPE_SET         = 248
	MYSQLI_TYPE_TINY_BLOB   = 249
	MYSQLI_TYPE_MEDIUM_BLOB = 250
	MYSQLI_TYPE_LONG_BLOB   = 251
	MYSQLI_TYPE_BLOB        = 252
	MYSQLI_TYPE_VAR_STRING  = 253
	MYSQLI_TYPE_STRING      = 254
	MYSQLI_TYPE_GEOMETRY    = 255
)

func init() {
	initMysqliClass()
	initMysqliResultClass()
	initMysqliStmtClass()

	phpctx.RegisterExt(&phpctx.Ext{
		Name:    "mysqli",
		Version: core.VERSION,
		Classes: []*phpobj.ZClass{
			MysqliClass,
			MysqliResultClass,
			MysqliStmtClass,
		},
		Functions: map[string]*phpctx.ExtFunction{
			"mysqli_connect":             {Func: fncMysqliConnect, Args: []*phpctx.ExtFunctionArg{}},
			"mysqli_query":               {Func: fncMysqliQuery, Args: []*phpctx.ExtFunctionArg{}},
			"mysqli_fetch_assoc":         {Func: fncMysqliFetchAssoc, Args: []*phpctx.ExtFunctionArg{}},
			"mysqli_fetch_array":         {Func: fncMysqliFetchArray, Args: []*phpctx.ExtFunctionArg{}},
			"mysqli_fetch_row":           {Func: fncMysqliFetchRow, Args: []*phpctx.ExtFunctionArg{}},
			"mysqli_fetch_all":           {Func: fncMysqliFetchAll, Args: []*phpctx.ExtFunctionArg{}},
			"mysqli_num_rows":            {Func: fncMysqliNumRows, Args: []*phpctx.ExtFunctionArg{}},
			"mysqli_affected_rows":       {Func: fncMysqliAffectedRows, Args: []*phpctx.ExtFunctionArg{}},
			"mysqli_insert_id":           {Func: fncMysqliInsertId, Args: []*phpctx.ExtFunctionArg{}},
			"mysqli_error":               {Func: fncMysqliError, Args: []*phpctx.ExtFunctionArg{}},
			"mysqli_errno":               {Func: fncMysqliErrno, Args: []*phpctx.ExtFunctionArg{}},
			"mysqli_close":               {Func: fncMysqliClose, Args: []*phpctx.ExtFunctionArg{}},
			"mysqli_real_escape_string":  {Func: fncMysqliRealEscapeString, Args: []*phpctx.ExtFunctionArg{}},
			"mysqli_prepare":             {Func: fncMysqliPrepare, Args: []*phpctx.ExtFunctionArg{}},
			"mysqli_stmt_execute":        {Func: fncMysqliStmtExecute, Args: []*phpctx.ExtFunctionArg{}},
			"mysqli_stmt_get_result":     {Func: fncMysqliStmtGetResult, Args: []*phpctx.ExtFunctionArg{}},
			"mysqli_stmt_close":          {Func: fncMysqliStmtClose, Args: []*phpctx.ExtFunctionArg{}},
			"mysqli_free_result":         {Func: fncMysqliFreeResult, Args: []*phpctx.ExtFunctionArg{}},
			"mysqli_select_db":           {Func: fncMysqliSelectDb, Args: []*phpctx.ExtFunctionArg{}},
			"mysqli_set_charset":         {Func: fncMysqliSetCharset, Args: []*phpctx.ExtFunctionArg{}},
			"mysqli_begin_transaction":   {Func: fncMysqliBeginTransaction, Args: []*phpctx.ExtFunctionArg{}},
			"mysqli_commit":              {Func: fncMysqliCommit, Args: []*phpctx.ExtFunctionArg{}},
			"mysqli_rollback":            {Func: fncMysqliRollback, Args: []*phpctx.ExtFunctionArg{}},
			"mysqli_autocommit":          {Func: fncMysqliAutocommit, Args: []*phpctx.ExtFunctionArg{}},
			"mysqli_ping":                {Func: fncMysqliPing, Args: []*phpctx.ExtFunctionArg{}},
		},
		Constants: map[phpv.ZString]phpv.Val{
			"MYSQLI_ASSOC":           phpv.ZInt(MYSQLI_ASSOC),
			"MYSQLI_NUM":             phpv.ZInt(MYSQLI_NUM),
			"MYSQLI_BOTH":            phpv.ZInt(MYSQLI_BOTH),
			"MYSQLI_STORE_RESULT":    phpv.ZInt(MYSQLI_STORE_RESULT),
			"MYSQLI_USE_RESULT":      phpv.ZInt(MYSQLI_USE_RESULT),
			"MYSQLI_CLIENT_COMPRESS": phpv.ZInt(MYSQLI_CLIENT_COMPRESS),
			"MYSQLI_CLIENT_SSL":      phpv.ZInt(MYSQLI_CLIENT_SSL),
			"MYSQLI_REPORT_OFF":      phpv.ZInt(MYSQLI_REPORT_OFF),
			"MYSQLI_REPORT_ERROR":    phpv.ZInt(MYSQLI_REPORT_ERROR),
			"MYSQLI_REPORT_STRICT":   phpv.ZInt(MYSQLI_REPORT_STRICT),
			"MYSQLI_REPORT_ALL":      phpv.ZInt(MYSQLI_REPORT_ALL),

			"MYSQLI_TYPE_DECIMAL":     phpv.ZInt(MYSQLI_TYPE_DECIMAL),
			"MYSQLI_TYPE_TINY":        phpv.ZInt(MYSQLI_TYPE_TINY),
			"MYSQLI_TYPE_SHORT":       phpv.ZInt(MYSQLI_TYPE_SHORT),
			"MYSQLI_TYPE_LONG":        phpv.ZInt(MYSQLI_TYPE_LONG),
			"MYSQLI_TYPE_FLOAT":       phpv.ZInt(MYSQLI_TYPE_FLOAT),
			"MYSQLI_TYPE_DOUBLE":      phpv.ZInt(MYSQLI_TYPE_DOUBLE),
			"MYSQLI_TYPE_NULL":        phpv.ZInt(MYSQLI_TYPE_NULL),
			"MYSQLI_TYPE_TIMESTAMP":   phpv.ZInt(MYSQLI_TYPE_TIMESTAMP),
			"MYSQLI_TYPE_LONGLONG":    phpv.ZInt(MYSQLI_TYPE_LONGLONG),
			"MYSQLI_TYPE_INT24":       phpv.ZInt(MYSQLI_TYPE_INT24),
			"MYSQLI_TYPE_DATE":        phpv.ZInt(MYSQLI_TYPE_DATE),
			"MYSQLI_TYPE_TIME":        phpv.ZInt(MYSQLI_TYPE_TIME),
			"MYSQLI_TYPE_DATETIME":    phpv.ZInt(MYSQLI_TYPE_DATETIME),
			"MYSQLI_TYPE_YEAR":        phpv.ZInt(MYSQLI_TYPE_YEAR),
			"MYSQLI_TYPE_NEWDATE":     phpv.ZInt(MYSQLI_TYPE_NEWDATE),
			"MYSQLI_TYPE_VARCHAR":     phpv.ZInt(MYSQLI_TYPE_VARCHAR),
			"MYSQLI_TYPE_BIT":         phpv.ZInt(MYSQLI_TYPE_BIT),
			"MYSQLI_TYPE_NEWDECIMAL":  phpv.ZInt(MYSQLI_TYPE_NEWDECIMAL),
			"MYSQLI_TYPE_ENUM":        phpv.ZInt(MYSQLI_TYPE_ENUM),
			"MYSQLI_TYPE_SET":         phpv.ZInt(MYSQLI_TYPE_SET),
			"MYSQLI_TYPE_TINY_BLOB":   phpv.ZInt(MYSQLI_TYPE_TINY_BLOB),
			"MYSQLI_TYPE_MEDIUM_BLOB": phpv.ZInt(MYSQLI_TYPE_MEDIUM_BLOB),
			"MYSQLI_TYPE_LONG_BLOB":   phpv.ZInt(MYSQLI_TYPE_LONG_BLOB),
			"MYSQLI_TYPE_BLOB":        phpv.ZInt(MYSQLI_TYPE_BLOB),
			"MYSQLI_TYPE_VAR_STRING":  phpv.ZInt(MYSQLI_TYPE_VAR_STRING),
			"MYSQLI_TYPE_STRING":      phpv.ZInt(MYSQLI_TYPE_STRING),
			"MYSQLI_TYPE_GEOMETRY":    phpv.ZInt(MYSQLI_TYPE_GEOMETRY),
		},
	})
}
