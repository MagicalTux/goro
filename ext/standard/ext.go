package standard

import (
	"github.com/MagicalTux/goro/core"
	"github.com/MagicalTux/goro/core/phpctx"
	"github.com/MagicalTux/goro/core/phpv"
)

// WARNING: This file is auto-generated. DO NOT EDIT

func init() {
	phpctx.RegisterExt(&phpctx.Ext{
		Name:    "standard",
		Version: core.VERSION,
		Classes: []phpv.ZClass{},
		Functions: map[string]*phpctx.ExtFunction{
			"abs":                      {Func: mathAbs, Args: []*phpctx.ExtFunctionArg{}},
			"acos":                     {Func: mathAcos, Args: []*phpctx.ExtFunctionArg{}},
			"acosh":                    {Func: mathACosh, Args: []*phpctx.ExtFunctionArg{}},
			"addslashes":               {Func: fncStrAddSlashes, Args: []*phpctx.ExtFunctionArg{}},
			"array_merge":              {Func: fncArrayMerge, Args: []*phpctx.ExtFunctionArg{}},
			"asin":                     {Func: mathAsin, Args: []*phpctx.ExtFunctionArg{}},
			"asinh":                    {Func: mathAsinh, Args: []*phpctx.ExtFunctionArg{}},
			"atan":                     {Func: mathAtan, Args: []*phpctx.ExtFunctionArg{}},
			"atan2":                    {Func: mathAtan2, Args: []*phpctx.ExtFunctionArg{}},
			"atanh":                    {Func: mathAtanh, Args: []*phpctx.ExtFunctionArg{}},
			"base64_decode":            {Func: fncBase64Decode, Args: []*phpctx.ExtFunctionArg{}},
			"base64_encode":            {Func: fncBase64Encode, Args: []*phpctx.ExtFunctionArg{}},
			"bin2hex":                  {Func: fncBin2hex, Args: []*phpctx.ExtFunctionArg{}},
			"boolval":                  {Func: fncBoolval, Args: []*phpctx.ExtFunctionArg{}},
			"chdir":                    {Func: fncChdir, Args: []*phpctx.ExtFunctionArg{}},
			"chr":                      {Func: fncStrChr, Args: []*phpctx.ExtFunctionArg{}},
			"chunk_split":              {Func: fncStrChunkSplit, Args: []*phpctx.ExtFunctionArg{}},
			"constant":                 {Func: constant, Args: []*phpctx.ExtFunctionArg{}},
			"cos":                      {Func: mathCos, Args: []*phpctx.ExtFunctionArg{}},
			"cosh":                     {Func: mathCosh, Args: []*phpctx.ExtFunctionArg{}},
			"count_chars":              {Func: fncStrCountChars, Args: []*phpctx.ExtFunctionArg{}},
			"decbin":                   {Func: fncDecbin, Args: []*phpctx.ExtFunctionArg{}},
			"dechex":                   {Func: fncDechex, Args: []*phpctx.ExtFunctionArg{}},
			"decoct":                   {Func: fncDecoct, Args: []*phpctx.ExtFunctionArg{}},
			"deg2rad":                  {Func: mathDeg2rad, Args: []*phpctx.ExtFunctionArg{}},
			"die":                      {Func: die, Args: []*phpctx.ExtFunctionArg{}},
			"dirname":                  {Func: fncDirname, Args: []*phpctx.ExtFunctionArg{}},
			"dl":                       {Func: stdFuncDl, Args: []*phpctx.ExtFunctionArg{}},
			"doubleval":                {Func: fncDoubleval, Args: []*phpctx.ExtFunctionArg{}},
			"eval":                     {Func: stdFuncEval, Args: []*phpctx.ExtFunctionArg{}},
			"exit":                     {Func: exit, Args: []*phpctx.ExtFunctionArg{}},
			"exp":                      {Func: mathExp, Args: []*phpctx.ExtFunctionArg{}},
			"explode":                  {Func: fncStrExplode, Args: []*phpctx.ExtFunctionArg{}},
			"expm1":                    {Func: mathExpm1, Args: []*phpctx.ExtFunctionArg{}},
			"extension_loaded":         {Func: stdFunc, Args: []*phpctx.ExtFunctionArg{}},
			"file_exists":              {Func: fncFileExists, Args: []*phpctx.ExtFunctionArg{}},
			"floatval":                 {Func: fncFloatval, Args: []*phpctx.ExtFunctionArg{}},
			"flush":                    {Func: fncFlush, Args: []*phpctx.ExtFunctionArg{}},
			"fmod":                     {Func: mathFmod, Args: []*phpctx.ExtFunctionArg{}},
			"function_exists":          {Func: stdFuncFuncExists, Args: []*phpctx.ExtFunctionArg{}},
			"get_cfg_var":              {Func: stdFuncGetCfgVar, Args: []*phpctx.ExtFunctionArg{}},
			"get_magic_quotes_gpc":     {Func: getMagicQuotesGpc, Args: []*phpctx.ExtFunctionArg{}},
			"get_magic_quotes_runtime": {Func: getMagicQuotesRuntime, Args: []*phpctx.ExtFunctionArg{}},
			"getcwd":                   {Func: fncGetcwd, Args: []*phpctx.ExtFunctionArg{}},
			"getenv":                   {Func: getenv, Args: []*phpctx.ExtFunctionArg{}},
			"gettype":                  {Func: fncGettype, Args: []*phpctx.ExtFunctionArg{}},
			"hrtime":                   {Func: stdFuncHrTime, Args: []*phpctx.ExtFunctionArg{}},
			"hypot":                    {Func: mathHypot, Args: []*phpctx.ExtFunctionArg{}},
			"implode":                  {Func: fncStrImplode, Args: []*phpctx.ExtFunctionArg{}},
			"intval":                   {Func: fncIntval, Args: []*phpctx.ExtFunctionArg{}},
			"is_array":                 {Func: fncIsArray, Args: []*phpctx.ExtFunctionArg{}},
			"is_bool":                  {Func: fncIsBool, Args: []*phpctx.ExtFunctionArg{}},
			"is_double":                {Func: fncIsDouble, Args: []*phpctx.ExtFunctionArg{}},
			"is_float":                 {Func: fncIsFloat, Args: []*phpctx.ExtFunctionArg{}},
			"is_int":                   {Func: fncIsInt, Args: []*phpctx.ExtFunctionArg{}},
			"is_integer":               {Func: fncIsInteger, Args: []*phpctx.ExtFunctionArg{}},
			"is_long":                  {Func: fncIsLong, Args: []*phpctx.ExtFunctionArg{}},
			"is_null":                  {Func: fncIsNull, Args: []*phpctx.ExtFunctionArg{}},
			"is_numeric":               {Func: fncIsNumeric, Args: []*phpctx.ExtFunctionArg{}},
			"is_object":                {Func: fncIsObject, Args: []*phpctx.ExtFunctionArg{}},
			"is_real":                  {Func: fncIsReal, Args: []*phpctx.ExtFunctionArg{}},
			"is_resource":              {Func: fncIsResource, Args: []*phpctx.ExtFunctionArg{}},
			"is_scalar":                {Func: fncIsScalar, Args: []*phpctx.ExtFunctionArg{}},
			"is_string":                {Func: fncIsString, Args: []*phpctx.ExtFunctionArg{}},
			"lcfirst":                  {Func: fncStrLcFirst, Args: []*phpctx.ExtFunctionArg{}},
			"ltrim":                    {Func: fncStrLtrim, Args: []*phpctx.ExtFunctionArg{}},
			"md5":                      {Func: fncStrMd5, Args: []*phpctx.ExtFunctionArg{}},
			"md5_file":                 {Func: fncStrMd5File, Args: []*phpctx.ExtFunctionArg{}},
			"microtime":                {Func: fncMicrotime, Args: []*phpctx.ExtFunctionArg{}},
			"nl2br":                    {Func: fncStrNewLine2Br, Args: []*phpctx.ExtFunctionArg{}},
			"number_format":            {Func: fncStrNumberFormat, Args: []*phpctx.ExtFunctionArg{}},
			"ob_clean":                 {Func: fncObClean, Args: []*phpctx.ExtFunctionArg{}},
			"ob_end_clean":             {Func: fncObEndClean, Args: []*phpctx.ExtFunctionArg{}},
			"ob_end_flush":             {Func: fncObEndFlush, Args: []*phpctx.ExtFunctionArg{}},
			"ob_flush":                 {Func: fncObFlush, Args: []*phpctx.ExtFunctionArg{}},
			"ob_get_clean":             {Func: fncObGetClean, Args: []*phpctx.ExtFunctionArg{}},
			"ob_get_contents":          {Func: fncObGetContents, Args: []*phpctx.ExtFunctionArg{}},
			"ob_get_flush":             {Func: fncObGetFlush, Args: []*phpctx.ExtFunctionArg{}},
			"ob_get_level":             {Func: fncObGetLevel, Args: []*phpctx.ExtFunctionArg{}},
			"ob_implicit_flush":        {Func: fncObImplicitFlush, Args: []*phpctx.ExtFunctionArg{}},
			"ob_start":                 {Func: fncObStart, Args: []*phpctx.ExtFunctionArg{}},
			"parse_str":                {Func: fncStrParseStr, Args: []*phpctx.ExtFunctionArg{}},
			"php_sapi_name":            {Func: stdFuncSapiName, Args: []*phpctx.ExtFunctionArg{}},
			"php_uname":                {Func: fncUname, Args: []*phpctx.ExtFunctionArg{}},
			"pi":                       {Func: mathPi, Args: []*phpctx.ExtFunctionArg{}},
			"print_r":                  {Func: fncPrintR, Args: []*phpctx.ExtFunctionArg{}},
			"putenv":                   {Func: putenv, Args: []*phpctx.ExtFunctionArg{}},
			"quotemeta":                {Func: fncStrQuoteMeta, Args: []*phpctx.ExtFunctionArg{}},
			"rawurlencode":             {Func: fncRawurlencode, Args: []*phpctx.ExtFunctionArg{}},
			"rtrim":                    {Func: fncStrRtrim, Args: []*phpctx.ExtFunctionArg{}},
			"set_time_limit":           {Func: fncSetTimeLimit, Args: []*phpctx.ExtFunctionArg{}},
			"sha1":                     {Func: fncStrSha1, Args: []*phpctx.ExtFunctionArg{}},
			"sha1_file":                {Func: fncStrSha1File, Args: []*phpctx.ExtFunctionArg{}},
			"sleep":                    {Func: stdFuncSleep, Args: []*phpctx.ExtFunctionArg{}},
			"sprintf":                  {Func: fncSprintf, Args: []*phpctx.ExtFunctionArg{}},
			"str_contains":             {Func: fncStrContains, Args: []*phpctx.ExtFunctionArg{}},
			"str_ends_with":            {Func: fncStrEndsWith, Args: []*phpctx.ExtFunctionArg{}},
			"str_getcsv":               {Func: fncStrGetCsv, Args: []*phpctx.ExtFunctionArg{}},
			"str_ireplace":             {Func: stdStrIReplace, Args: []*phpctx.ExtFunctionArg{}},
			"str_pad":                  {Func: fncStrPad, Args: []*phpctx.ExtFunctionArg{}},
			"str_repeat":               {Func: fncStrRepeat, Args: []*phpctx.ExtFunctionArg{}},
			"str_replace":              {Func: stdStrReplace, Args: []*phpctx.ExtFunctionArg{}},
			"str_rot13":                {Func: fncStrRot13, Args: []*phpctx.ExtFunctionArg{}},
			"str_shuffle":              {Func: fncStrShuffle, Args: []*phpctx.ExtFunctionArg{}},
			"str_split":                {Func: fncStrSplit, Args: []*phpctx.ExtFunctionArg{}},
			"str_starts_with":          {Func: fncStrStartsWith, Args: []*phpctx.ExtFunctionArg{}},
			"str_word_count":           {Func: fncStrWordCount, Args: []*phpctx.ExtFunctionArg{}},
			"strcasecmp":               {Func: fncStrCaseCmp, Args: []*phpctx.ExtFunctionArg{}},
			"strcmp":                   {Func: fncStrCmp, Args: []*phpctx.ExtFunctionArg{}},
			"strcoll":                  {Func: fncStrColl, Args: []*phpctx.ExtFunctionArg{}},
			"strcspn":                  {Func: fncStrCspn, Args: []*phpctx.ExtFunctionArg{}},
			"strip_tags":               {Func: fncStripTags, Args: []*phpctx.ExtFunctionArg{}},
			"stripcslashes":            {Func: fncStripCSlashes, Args: []*phpctx.ExtFunctionArg{}},
			"stripos":                  {Func: fncStrIPos, Args: []*phpctx.ExtFunctionArg{}},
			"stripslashes":             {Func: fncStripSlashes, Args: []*phpctx.ExtFunctionArg{}},
			"strnatcasecmp":            {Func: fncStrNatCaseCmp, Args: []*phpctx.ExtFunctionArg{}},
			"strnatcmp":                {Func: fncStrNatCmp, Args: []*phpctx.ExtFunctionArg{}},
			"strpbrk":                  {Func: fncStrPbrk, Args: []*phpctx.ExtFunctionArg{}},
			"strpos":                   {Func: fncStrPos, Args: []*phpctx.ExtFunctionArg{}},
			"strrev":                   {Func: fncStrRev, Args: []*phpctx.ExtFunctionArg{}},
			"strstr":                   {Func: fncStrStr, Args: []*phpctx.ExtFunctionArg{}},
			"strtok":                   {Func: fncStrtok, Args: []*phpctx.ExtFunctionArg{}},
			"strtolower":               {Func: fncStrToLower, Args: []*phpctx.ExtFunctionArg{}},
			"strtoupper":               {Func: fncStrToUpper, Args: []*phpctx.ExtFunctionArg{}},
			"strval":                   {Func: fncStrval, Args: []*phpctx.ExtFunctionArg{}},
			"substr":                   {Func: fncSubstr, Args: []*phpctx.ExtFunctionArg{}},
			"substr_compare":           {Func: fncSubstrCompare, Args: []*phpctx.ExtFunctionArg{}},
			"substr_cont":              {Func: fncSubstrCount, Args: []*phpctx.ExtFunctionArg{}},
			"substr_replace":           {Func: fncSubstrReplace, Args: []*phpctx.ExtFunctionArg{}},
			"time":                     {Func: fncTime, Args: []*phpctx.ExtFunctionArg{}},
			"trim":                     {Func: fncStrTrim, Args: []*phpctx.ExtFunctionArg{}},
			"urlencode":                {Func: fncUrlencode, Args: []*phpctx.ExtFunctionArg{}},
			"usleep":                   {Func: stdFuncUsleep, Args: []*phpctx.ExtFunctionArg{}},
			"var_dump":                 {Func: stdFuncVarDump, Args: []*phpctx.ExtFunctionArg{}},
			"wordwrap":                 {Func: fncWordWrap, Args: []*phpctx.ExtFunctionArg{}},
		},
		Constants: map[phpv.ZString]phpv.Val{
			"INF":                 INF,
			"M_1_PI":              M_1_PI,
			"M_2_PI":              M_2_PI,
			"M_2_SQRTPI":          M_2_SQRTPI,
			"M_E":                 M_E,
			"M_EULER":             M_EULER,
			"M_LN2":               M_LN2,
			"M_LNPI":              M_LNPI,
			"M_LOG10E":            M_LOG10E,
			"M_LOG2E":             M_LOG2E,
			"M_PHI":               M_PHI,
			"M_PI":                M_PI,
			"M_PI_2":              M_PI_2,
			"M_PI_4":              M_PI_4,
			"M_SQRT1_2":           M_SQRT1_2,
			"M_SQRT2":             M_SQRT2,
			"M_SQRT3":             M_SQRT3,
			"M_SQRTPI":            M_SQRTPI,
			"NAN":                 NAN,
			"PHP_ROUND_HALF_DOWN": PHP_ROUND_HALF_DOWN,
			"PHP_ROUND_HALF_EVEN": PHP_ROUND_HALF_EVEN,
			"PHP_ROUND_HALF_ODD":  PHP_ROUND_HALF_ODD,
			"PHP_ROUND_HALF_UP":   PHP_ROUND_HALF_UP,
			"STR_PAD_BOTH":        STR_PAD_BOTH,
			"STR_PAD_LEFT":        STR_PAD_LEFT,
			"STR_PAD_RIGHT":       STR_PAD_RIGHT,
		},
	})
}
