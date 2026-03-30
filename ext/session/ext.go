package session

import (
	"github.com/MagicalTux/goro/core"
	"github.com/MagicalTux/goro/core/phpctx"
	"github.com/MagicalTux/goro/core/phpv"
)

func init() {
	phpctx.RegisterExt(&phpctx.Ext{
		Name:    "session",
		Version: core.VERSION,
		Functions: map[string]*phpctx.ExtFunction{
			"session_start":              {Func: fncSessionStart, Args: []*phpctx.ExtFunctionArg{}},
			"session_destroy":            {Func: fncSessionDestroy, Args: []*phpctx.ExtFunctionArg{}},
			"session_id":                 {Func: fncSessionId, Args: []*phpctx.ExtFunctionArg{}},
			"session_name":               {Func: fncSessionName, Args: []*phpctx.ExtFunctionArg{}},
			"session_status":             {Func: fncSessionStatus, Args: []*phpctx.ExtFunctionArg{}},
			"session_write_close":        {Func: fncSessionWriteClose, Args: []*phpctx.ExtFunctionArg{}},
			"session_commit":             {Func: fncSessionCommit, Args: []*phpctx.ExtFunctionArg{}},
			"session_unset":              {Func: fncSessionUnset, Args: []*phpctx.ExtFunctionArg{}},
			"session_regenerate_id":      {Func: fncSessionRegenerateId, Args: []*phpctx.ExtFunctionArg{}},
			"session_save_path":          {Func: fncSessionSavePath, Args: []*phpctx.ExtFunctionArg{}},
			"session_encode":             {Func: fncSessionEncode, Args: []*phpctx.ExtFunctionArg{}},
			"session_decode":             {Func: fncSessionDecode, Args: []*phpctx.ExtFunctionArg{}},
			"session_abort":              {Func: fncSessionAbort, Args: []*phpctx.ExtFunctionArg{}},
			"session_reset":              {Func: fncSessionReset, Args: []*phpctx.ExtFunctionArg{}},
			"session_gc":                 {Func: fncSessionGc, Args: []*phpctx.ExtFunctionArg{}},
			"session_set_cookie_params":  {Func: fncSessionSetCookieParams, Args: []*phpctx.ExtFunctionArg{}},
			"session_get_cookie_params":  {Func: fncSessionGetCookieParams, Args: []*phpctx.ExtFunctionArg{}},
			"session_cache_limiter":      {Func: fncSessionCacheLimiter, Args: []*phpctx.ExtFunctionArg{}},
			"session_cache_expire":       {Func: fncSessionCacheExpire, Args: []*phpctx.ExtFunctionArg{}},
		},
		Constants: map[phpv.ZString]phpv.Val{
			"PHP_SESSION_DISABLED": PHP_SESSION_DISABLED,
			"PHP_SESSION_NONE":     PHP_SESSION_NONE,
			"PHP_SESSION_ACTIVE":   PHP_SESSION_ACTIVE,
		},
	})
}
