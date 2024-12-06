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
			"addcslashes":              {Func: fncStrAddCSlashes, Args: []*phpctx.ExtFunctionArg{}},
			"addslashes":               {Func: fncStrAddSlashes, Args: []*phpctx.ExtFunctionArg{}},
			"array_change_key_case":    {Func: fncArrayChangeKeyCase, Args: []*phpctx.ExtFunctionArg{}},
			"array_chunk":              {Func: fncArrayChunk, Args: []*phpctx.ExtFunctionArg{}},
			"array_column":             {Func: fncArrayColumn, Args: []*phpctx.ExtFunctionArg{}},
			"array_combine":            {Func: fncArrayCombine, Args: []*phpctx.ExtFunctionArg{}},
			"array_count_values":       {Func: fncArrayCountValues, Args: []*phpctx.ExtFunctionArg{}},
			"array_diff":               {Func: fncArrayDiff, Args: []*phpctx.ExtFunctionArg{}},
			"array_diff_assoc":         {Func: fncArrayDiffAssoc, Args: []*phpctx.ExtFunctionArg{}},
			"array_diff_key":           {Func: fncArrayDiffKey, Args: []*phpctx.ExtFunctionArg{}},
			"array_diff_uassoc":        {Func: fncArrayDiffUAssoc, Args: []*phpctx.ExtFunctionArg{}},
			"array_diff_ukey":          {Func: fncArrayDiffUKey, Args: []*phpctx.ExtFunctionArg{}},
			"array_fill":               {Func: fncArrayFill, Args: []*phpctx.ExtFunctionArg{}},
			"array_fill_keys":          {Func: fncArrayFillKeys, Args: []*phpctx.ExtFunctionArg{}},
			"array_filter":             {Func: fncArrayFilter, Args: []*phpctx.ExtFunctionArg{}},
			"array_flip":               {Func: fncArrayFlip, Args: []*phpctx.ExtFunctionArg{}},
			"array_intersect":          {Func: fncArrayIntersect, Args: []*phpctx.ExtFunctionArg{}},
			"array_intersect_assoc":    {Func: fncArrayIntersectAssoc, Args: []*phpctx.ExtFunctionArg{}},
			"array_intersect_key":      {Func: fncArrayIntersectKey, Args: []*phpctx.ExtFunctionArg{}},
			"array_intersect_uassoc":   {Func: fncArrayIntersectUAssoc, Args: []*phpctx.ExtFunctionArg{}},
			"array_intersect_ukey":     {Func: fncArrayIntersectUKey, Args: []*phpctx.ExtFunctionArg{}},
			"array_key_exists":         {Func: fncArrayKeyExists, Args: []*phpctx.ExtFunctionArg{}},
			"array_key_first":          {Func: fncArrayKeyFirst, Args: []*phpctx.ExtFunctionArg{}},
			"array_key_last":           {Func: fncArrayKeyLast, Args: []*phpctx.ExtFunctionArg{}},
			"array_keys":               {Func: fncArrayKeys, Args: []*phpctx.ExtFunctionArg{}},
			"array_map":                {Func: fncArrayMap, Args: []*phpctx.ExtFunctionArg{}},
			"array_merge":              {Func: fncArrayMerge, Args: []*phpctx.ExtFunctionArg{}},
			"array_merge_recursive":    {Func: fncArrayMergeRecursive, Args: []*phpctx.ExtFunctionArg{}},
			"array_pad":                {Func: fncArrayPad, Args: []*phpctx.ExtFunctionArg{}},
			"array_pop":                {Func: fncArrayPop, Args: []*phpctx.ExtFunctionArg{}},
			"array_product":            {Func: fncArrayProduct, Args: []*phpctx.ExtFunctionArg{}},
			"array_push":               {Func: fncArrayPush, Args: []*phpctx.ExtFunctionArg{}},
			"array_rand":               {Func: fncArrayRand, Args: []*phpctx.ExtFunctionArg{}},
			"array_reduce":             {Func: fncArrayReduce, Args: []*phpctx.ExtFunctionArg{}},
			"array_replace":            {Func: fncArrayReplace, Args: []*phpctx.ExtFunctionArg{}},
			"array_replace_recursive":  {Func: fncArrayReplaceRecursive, Args: []*phpctx.ExtFunctionArg{}},
			"array_reverse":            {Func: fncArrayReverse, Args: []*phpctx.ExtFunctionArg{}},
			"array_search":             {Func: fncArraySearch, Args: []*phpctx.ExtFunctionArg{}},
			"array_shift":              {Func: fncArrayShift, Args: []*phpctx.ExtFunctionArg{}},
			"array_slice":              {Func: fncArraySlice, Args: []*phpctx.ExtFunctionArg{}},
			"array_sum":                {Func: fncArraySum, Args: []*phpctx.ExtFunctionArg{}},
			"array_udiff":              {Func: fncArrayUDiff, Args: []*phpctx.ExtFunctionArg{}},
			"array_udiff_assoc":        {Func: fncArrayUDiffAssoc, Args: []*phpctx.ExtFunctionArg{}},
			"array_uintersect":         {Func: fncArrayUIntersect, Args: []*phpctx.ExtFunctionArg{}},
			"array_uintersect_assoc":   {Func: fncArrayUIntersectAssoc, Args: []*phpctx.ExtFunctionArg{}},
			"array_uintersect_uassoc":  {Func: fncArrayUIntersectUAssoc, Args: []*phpctx.ExtFunctionArg{}},
			"array_unique":             {Func: fncArrayUnique, Args: []*phpctx.ExtFunctionArg{}},
			"array_unshift":            {Func: fncArrayUnshift, Args: []*phpctx.ExtFunctionArg{}},
			"array_values":             {Func: fncArrayValues, Args: []*phpctx.ExtFunctionArg{}},
			"array_walk":               {Func: fncArrayWalk, Args: []*phpctx.ExtFunctionArg{}},
			"array_walk_recursive":     {Func: fncArrayWalkRecursive, Args: []*phpctx.ExtFunctionArg{}},
			"asin":                     {Func: mathAsin, Args: []*phpctx.ExtFunctionArg{}},
			"asinh":                    {Func: mathAsinh, Args: []*phpctx.ExtFunctionArg{}},
			"atan":                     {Func: mathAtan, Args: []*phpctx.ExtFunctionArg{}},
			"atan2":                    {Func: mathAtan2, Args: []*phpctx.ExtFunctionArg{}},
			"atanh":                    {Func: mathAtanh, Args: []*phpctx.ExtFunctionArg{}},
			"base64_decode":            {Func: fncBase64Decode, Args: []*phpctx.ExtFunctionArg{}},
			"base64_encode":            {Func: fncBase64Encode, Args: []*phpctx.ExtFunctionArg{}},
			"basename":                 {Func: fncBasename, Args: []*phpctx.ExtFunctionArg{}},
			"bin2hex":                  {Func: fncBin2hex, Args: []*phpctx.ExtFunctionArg{}},
			"boolval":                  {Func: fncBoolval, Args: []*phpctx.ExtFunctionArg{}},
			"call_user_func":           {Func: fncCallUserFunc, Args: []*phpctx.ExtFunctionArg{}},
			"call_user_func_array":     {Func: fncCallUserFuncArray, Args: []*phpctx.ExtFunctionArg{}},
			"chdir":                    {Func: fncChdir, Args: []*phpctx.ExtFunctionArg{}},
			"chop":                     {Func: fncStrRtrim, Args: []*phpctx.ExtFunctionArg{}}, // alias
			"chr":                      {Func: fncChr, Args: []*phpctx.ExtFunctionArg{}},
			"chunk_split":              {Func: fncStrChunkSplit, Args: []*phpctx.ExtFunctionArg{}},
			"compact":                  {Func: fncArrayCompact, Args: []*phpctx.ExtFunctionArg{}},
			"constant":                 {Func: constant, Args: []*phpctx.ExtFunctionArg{}},
			"cos":                      {Func: mathCos, Args: []*phpctx.ExtFunctionArg{}},
			"cosh":                     {Func: mathCosh, Args: []*phpctx.ExtFunctionArg{}},
			"count_chars":              {Func: fncStrCountChars, Args: []*phpctx.ExtFunctionArg{}},
			"current":                  {Func: fncArrayCurrent, Args: []*phpctx.ExtFunctionArg{}},
			"decbin":                   {Func: fncDecbin, Args: []*phpctx.ExtFunctionArg{}},
			"dechex":                   {Func: fncDechex, Args: []*phpctx.ExtFunctionArg{}},
			"decoct":                   {Func: fncDecoct, Args: []*phpctx.ExtFunctionArg{}},
			"deg2rad":                  {Func: mathDeg2rad, Args: []*phpctx.ExtFunctionArg{}},
			"die":                      {Func: die, Args: []*phpctx.ExtFunctionArg{}},
			"dirname":                  {Func: fncDirname, Args: []*phpctx.ExtFunctionArg{}},
			"dl":                       {Func: stdFuncDl, Args: []*phpctx.ExtFunctionArg{}},
			"doubleval":                {Func: fncDoubleval, Args: []*phpctx.ExtFunctionArg{}},
			"end":                      {Func: fncArrayEnd, Args: []*phpctx.ExtFunctionArg{}},
			"eval":                     {Func: stdFuncEval, Args: []*phpctx.ExtFunctionArg{}},
			"exit":                     {Func: exit, Args: []*phpctx.ExtFunctionArg{}},
			"exp":                      {Func: mathExp, Args: []*phpctx.ExtFunctionArg{}},
			"explode":                  {Func: fncStrExplode, Args: []*phpctx.ExtFunctionArg{}},
			"expm1":                    {Func: mathExpm1, Args: []*phpctx.ExtFunctionArg{}},
			"extension_loaded":         {Func: stdFunc, Args: []*phpctx.ExtFunctionArg{}},
			"extract":                  {Func: fncArrayExtract, Args: []*phpctx.ExtFunctionArg{}},
			"file_exists":              {Func: fncFileExists, Args: []*phpctx.ExtFunctionArg{}},
			"file_get_contents":        {Func: fncFileGetContents, Args: []*phpctx.ExtFunctionArg{}},
			"file_put_contents":        {Func: fncFilePutContents, Args: []*phpctx.ExtFunctionArg{}},
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
			"in_array":                 {Func: fncInArray, Args: []*phpctx.ExtFunctionArg{}},
			"intval":                   {Func: fncIntval, Args: []*phpctx.ExtFunctionArg{}},
			"is_array":                 {Func: fncIsArray, Args: []*phpctx.ExtFunctionArg{}},
			"is_bool":                  {Func: fncIsBool, Args: []*phpctx.ExtFunctionArg{}},
			"is_dir":                   {Func: fncIsDir, Args: []*phpctx.ExtFunctionArg{}},
			"is_double":                {Func: fncIsDouble, Args: []*phpctx.ExtFunctionArg{}},
			"is_file":                  {Func: fncIsFile, Args: []*phpctx.ExtFunctionArg{}},
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
			"join":                     {Func: fncStrImplode, Args: []*phpctx.ExtFunctionArg{}}, // alias
			"key":                      {Func: fncArrayKey, Args: []*phpctx.ExtFunctionArg{}},
			"ksort":                    {Func: fncArrayKSort, Args: []*phpctx.ExtFunctionArg{}},
			"lcfirst":                  {Func: fncStrLcFirst, Args: []*phpctx.ExtFunctionArg{}},
			"ltrim":                    {Func: fncStrLtrim, Args: []*phpctx.ExtFunctionArg{}},
			"max":                      {Func: mathMax, Args: []*phpctx.ExtFunctionArg{}},
			"md5":                      {Func: fncStrMd5, Args: []*phpctx.ExtFunctionArg{}},
			"md5_file":                 {Func: fncStrMd5File, Args: []*phpctx.ExtFunctionArg{}},
			"method_exists":            {Func: stdFuncMethodExists, Args: []*phpctx.ExtFunctionArg{}},
			"microtime":                {Func: fncMicrotime, Args: []*phpctx.ExtFunctionArg{}},
			"mktime":                   {Func: fncMkTime, Args: []*phpctx.ExtFunctionArg{}},
			"next":                     {Func: fncArrayNext, Args: []*phpctx.ExtFunctionArg{}},
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
			"ord":                      {Func: fncOrd, Args: []*phpctx.ExtFunctionArg{}},
			"parse_str":                {Func: fncStrParseStr, Args: []*phpctx.ExtFunctionArg{}},
			"php_sapi_name":            {Func: stdFuncSapiName, Args: []*phpctx.ExtFunctionArg{}},
			"php_uname":                {Func: fncUname, Args: []*phpctx.ExtFunctionArg{}},
			"pi":                       {Func: mathPi, Args: []*phpctx.ExtFunctionArg{}},
			"pos":                      {Func: fncArrayCurrent, Args: []*phpctx.ExtFunctionArg{}}, // alias
			"prev":                     {Func: fncArrayPrev, Args: []*phpctx.ExtFunctionArg{}},
			"print_r":                  {Func: fncPrintR, Args: []*phpctx.ExtFunctionArg{}},
			"printf":                   {Func: fncPrintf, Args: []*phpctx.ExtFunctionArg{}},
			"putenv":                   {Func: putenv, Args: []*phpctx.ExtFunctionArg{}},
			"quotemeta":                {Func: fncStrQuoteMeta, Args: []*phpctx.ExtFunctionArg{}},
			"range":                    {Func: fncRange, Args: []*phpctx.ExtFunctionArg{}},
			"rawurldecode":             {Func: fncRawurldecode, Args: []*phpctx.ExtFunctionArg{}},
			"rawurlencode":             {Func: fncRawurlencode, Args: []*phpctx.ExtFunctionArg{}},
			"realpath":                 {Func: fncRealPath, Args: []*phpctx.ExtFunctionArg{}},
			"reset":                    {Func: fncArrayReset, Args: []*phpctx.ExtFunctionArg{}},
			"rmdir":                    {Func: fncRmdir, Args: []*phpctx.ExtFunctionArg{}},
			"rtrim":                    {Func: fncStrRtrim, Args: []*phpctx.ExtFunctionArg{}},
			"set_time_limit":           {Func: fncSetTimeLimit, Args: []*phpctx.ExtFunctionArg{}},
			"sha1":                     {Func: fncStrSha1, Args: []*phpctx.ExtFunctionArg{}},
			"sha1_file":                {Func: fncStrSha1File, Args: []*phpctx.ExtFunctionArg{}},
			"shell_exec":               {Func: fncShellExec, Args: []*phpctx.ExtFunctionArg{}},
			"sleep":                    {Func: stdFuncSleep, Args: []*phpctx.ExtFunctionArg{}},
			"sort":                     {Func: fncArraySort, Args: []*phpctx.ExtFunctionArg{}},
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
			"strchr":                   {Func: fncStrStr, Args: []*phpctx.ExtFunctionArg{}}, // alias
			"strcmp":                   {Func: fncStrCmp, Args: []*phpctx.ExtFunctionArg{}},
			"strcoll":                  {Func: fncStrColl, Args: []*phpctx.ExtFunctionArg{}},
			"strcspn":                  {Func: fncStrCspn, Args: []*phpctx.ExtFunctionArg{}},
			"strip_tags":               {Func: fncStripTags, Args: []*phpctx.ExtFunctionArg{}},
			"stripcslashes":            {Func: fncStripCSlashes, Args: []*phpctx.ExtFunctionArg{}},
			"stripos":                  {Func: fncStrIPos, Args: []*phpctx.ExtFunctionArg{}},
			"stripslashes":             {Func: fncStripSlashes, Args: []*phpctx.ExtFunctionArg{}},
			"stristr":                  {Func: fncStrIStr, Args: []*phpctx.ExtFunctionArg{}},
			"strnatcasecmp":            {Func: fncStrNatCaseCmp, Args: []*phpctx.ExtFunctionArg{}},
			"strnatcmp":                {Func: fncStrNatCmp, Args: []*phpctx.ExtFunctionArg{}},
			"strncasecmp":              {Func: fncStrNCaseCmp, Args: []*phpctx.ExtFunctionArg{}},
			"strpbrk":                  {Func: fncStrPbrk, Args: []*phpctx.ExtFunctionArg{}},
			"strpos":                   {Func: fncStrPos, Args: []*phpctx.ExtFunctionArg{}},
			"strrchr":                  {Func: fncStrRChr, Args: []*phpctx.ExtFunctionArg{}},
			"strrev":                   {Func: fncStrRev, Args: []*phpctx.ExtFunctionArg{}},
			"strrpos":                  {Func: fncStrRPos, Args: []*phpctx.ExtFunctionArg{}},
			"strstr":                   {Func: fncStrStr, Args: []*phpctx.ExtFunctionArg{}},
			"strtok":                   {Func: fncStrtok, Args: []*phpctx.ExtFunctionArg{}},
			"strtolower":               {Func: fncStrToLower, Args: []*phpctx.ExtFunctionArg{}},
			"strtoupper":               {Func: fncStrToUpper, Args: []*phpctx.ExtFunctionArg{}},
			"strtr":                    {Func: fncStrTr, Args: []*phpctx.ExtFunctionArg{}},
			"strval":                   {Func: fncStrval, Args: []*phpctx.ExtFunctionArg{}},
			"substr":                   {Func: fncSubstr, Args: []*phpctx.ExtFunctionArg{}},
			"substr_compare":           {Func: fncSubstrCompare, Args: []*phpctx.ExtFunctionArg{}},
			"substr_cont":              {Func: fncSubstrCount, Args: []*phpctx.ExtFunctionArg{}},
			"substr_replace":           {Func: fncSubstrReplace, Args: []*phpctx.ExtFunctionArg{}},
			"time":                     {Func: fncTime, Args: []*phpctx.ExtFunctionArg{}},
			"trim":                     {Func: fncStrTrim, Args: []*phpctx.ExtFunctionArg{}},
			"ucfirst":                  {Func: fncUcFirst, Args: []*phpctx.ExtFunctionArg{}},
			"ucwords":                  {Func: fncUcWords, Args: []*phpctx.ExtFunctionArg{}},
			"uniqid":                   {Func: fncUniqId, Args: []*phpctx.ExtFunctionArg{}},
			"unlink":                   {Func: fncUnlink, Args: []*phpctx.ExtFunctionArg{}},
			"urldecode":                {Func: fncUrldecode, Args: []*phpctx.ExtFunctionArg{}},
			"urlencode":                {Func: fncUrlencode, Args: []*phpctx.ExtFunctionArg{}},
			"usleep":                   {Func: stdFuncUsleep, Args: []*phpctx.ExtFunctionArg{}},
			"var_dump":                 {Func: stdFuncVarDump, Args: []*phpctx.ExtFunctionArg{}},
			"var_export":               {Func: stdFuncVarExport, Args: []*phpctx.ExtFunctionArg{}},
			"vprintf":                  {Func: fncVPrintf, Args: []*phpctx.ExtFunctionArg{}},
			"vsprintf":                 {Func: fncVSprintf, Args: []*phpctx.ExtFunctionArg{}},
			"wordwrap":                 {Func: fncWordWrap, Args: []*phpctx.ExtFunctionArg{}},
		},
		Constants: map[phpv.ZString]phpv.Val{
			"ARRAY_FILTER_USE_BOTH":   ARRAY_FILTER_USE_BOTH,
			"ARRAY_FILTER_USE_KEY":    ARRAY_FILTER_USE_KEY,
			"CASE_LOWER":              CASE_LOWER,
			"CASE_UPPER":              CASE_UPPER,
			"EXTR_IF_EXISTS":          EXTR_IF_EXISTS,
			"EXTR_OVERWRITE":          EXTR_OVERWRITE,
			"EXTR_PREFIX_ALL":         EXTR_PREFIX_ALL,
			"EXTR_PREFIX_IF_EXISTS":   EXTR_PREFIX_IF_EXISTS,
			"EXTR_PREFIX_INVALID":     EXTR_PREFIX_INVALID,
			"EXTR_PREFIX_SAME":        EXTR_PREFIX_SAME,
			"EXTR_REFS":               EXTR_REFS,
			"EXTR_SKIP":               EXTR_SKIP,
			"FILE_APPEND":             FILE_APPEND,
			"FILE_IGNORE_NEW_LINES":   FILE_IGNORE_NEW_LINES,
			"FILE_NO_DEFAULT_CONTEXT": FILE_NO_DEFAULT_CONTEXT,
			"FILE_SKIP_EMPTY_LINES":   FILE_SKIP_EMPTY_LINES,
			"FILE_USE_INCLUDE_PATH":   FILE_USE_INCLUDE_PATH,
			"INF":                     INF,
			"LOCK_EX":                 LOCK_EX,
			"LOCK_NB":                 LOCK_NB,
			"LOCK_SH":                 LOCK_SH,
			"LOCK_UN":                 LOCK_UN,
			"M_1_PI":                  M_1_PI,
			"M_2_PI":                  M_2_PI,
			"M_2_SQRTPI":              M_2_SQRTPI,
			"M_E":                     M_E,
			"M_EULER":                 M_EULER,
			"M_LN2":                   M_LN2,
			"M_LNPI":                  M_LNPI,
			"M_LOG10E":                M_LOG10E,
			"M_LOG2E":                 M_LOG2E,
			"M_PHI":                   M_PHI,
			"M_PI":                    M_PI,
			"M_PI_2":                  M_PI_2,
			"M_PI_4":                  M_PI_4,
			"M_SQRT1_2":               M_SQRT1_2,
			"M_SQRT2":                 M_SQRT2,
			"M_SQRT3":                 M_SQRT3,
			"M_SQRTPI":                M_SQRTPI,
			"NAN":                     NAN,
			"PHP_ROUND_HALF_DOWN":     PHP_ROUND_HALF_DOWN,
			"PHP_ROUND_HALF_EVEN":     PHP_ROUND_HALF_EVEN,
			"PHP_ROUND_HALF_ODD":      PHP_ROUND_HALF_ODD,
			"PHP_ROUND_HALF_UP":       PHP_ROUND_HALF_UP,
			"SORT_ASC":                SORT_ASC,
			"SORT_DESC":               SORT_DESC,
			"SORT_FLAG_CASE":          SORT_FLAG_CASE,
			"SORT_LOCALE_STRING":      SORT_LOCALE_STRING,
			"SORT_NATURAL":            SORT_NATURAL,
			"SORT_NUMERIC":            SORT_NUMERIC,
			"SORT_REGULAR":            SORT_REGULAR,
			"SORT_STRING":             SORT_STRING,
			"STR_PAD_BOTH":            STR_PAD_BOTH,
			"STR_PAD_LEFT":            STR_PAD_LEFT,
			"STR_PAD_RIGHT":           STR_PAD_RIGHT,
		},
	})
}
