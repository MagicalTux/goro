package sqlite3

import (
	"github.com/MagicalTux/goro/core"
	"github.com/MagicalTux/goro/core/phpctx"
	"github.com/MagicalTux/goro/core/phpobj"
	"github.com/MagicalTux/goro/core/phpv"
)

// SQLite3 fetch/type constants
const (
	SQLITE3_ASSOC = 1
	SQLITE3_NUM   = 2
	SQLITE3_BOTH  = 3

	SQLITE3_INTEGER = 1
	SQLITE3_FLOAT   = 2
	SQLITE3_TEXT    = 3
	SQLITE3_BLOB    = 4
	SQLITE3_NULL    = 5

	SQLITE3_OPEN_READONLY  = 1
	SQLITE3_OPEN_READWRITE = 2
	SQLITE3_OPEN_CREATE    = 4
)

func init() {
	initSQLite3Class()
	initSQLite3ResultClass()
	initSQLite3StmtClass()

	phpctx.RegisterExt(&phpctx.Ext{
		Name:    "sqlite3",
		Version: core.VERSION,
		Classes: []*phpobj.ZClass{
			SQLite3Class,
			SQLite3ResultClass,
			SQLite3StmtClass,
		},
		Constants: map[phpv.ZString]phpv.Val{
			"SQLITE3_ASSOC": phpv.ZInt(SQLITE3_ASSOC),
			"SQLITE3_NUM":   phpv.ZInt(SQLITE3_NUM),
			"SQLITE3_BOTH":  phpv.ZInt(SQLITE3_BOTH),

			"SQLITE3_INTEGER": phpv.ZInt(SQLITE3_INTEGER),
			"SQLITE3_FLOAT":   phpv.ZInt(SQLITE3_FLOAT),
			"SQLITE3_TEXT":    phpv.ZInt(SQLITE3_TEXT),
			"SQLITE3_BLOB":    phpv.ZInt(SQLITE3_BLOB),
			"SQLITE3_NULL":    phpv.ZInt(SQLITE3_NULL),

			"SQLITE3_OPEN_READONLY":  phpv.ZInt(SQLITE3_OPEN_READONLY),
			"SQLITE3_OPEN_READWRITE": phpv.ZInt(SQLITE3_OPEN_READWRITE),
			"SQLITE3_OPEN_CREATE":    phpv.ZInt(SQLITE3_OPEN_CREATE),
		},
	})
}
