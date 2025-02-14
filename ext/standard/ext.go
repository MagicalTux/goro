package standard

import (
	"github.com/MagicalTux/goro/core"
	"github.com/MagicalTux/goro/core/phpctx"
	"github.com/MagicalTux/goro/core/phpobj"
	"github.com/MagicalTux/goro/core/phpv"
)

// WARNING: This file is auto-generated. DO NOT EDIT

func init() {
	phpctx.RegisterExt(&phpctx.Ext{
		Name:    "standard",
		Version: core.VERSION,
		Classes: []*phpobj.ZClass{},
		// Note: ExtFunctionArg is currently unused
		Functions: map[string]*phpctx.ExtFunction{
			"abs":                        {Func: mathAbs, Args: []*phpctx.ExtFunctionArg{}},
			"acos":                       {Func: mathAcos, Args: []*phpctx.ExtFunctionArg{}},
			"acosh":                      {Func: mathACosh, Args: []*phpctx.ExtFunctionArg{}},
			"addcslashes":                {Func: fncStrAddCSlashes, Args: []*phpctx.ExtFunctionArg{}},
			"addslashes":                 {Func: fncStrAddSlashes, Args: []*phpctx.ExtFunctionArg{}},
			"array_change_key_case":      {Func: fncArrayChangeKeyCase, Args: []*phpctx.ExtFunctionArg{}},
			"array_chunk":                {Func: fncArrayChunk, Args: []*phpctx.ExtFunctionArg{}},
			"array_column":               {Func: fncArrayColumn, Args: []*phpctx.ExtFunctionArg{}},
			"array_combine":              {Func: fncArrayCombine, Args: []*phpctx.ExtFunctionArg{}},
			"array_count_values":         {Func: fncArrayCountValues, Args: []*phpctx.ExtFunctionArg{}},
			"array_diff":                 {Func: fncArrayDiff, Args: []*phpctx.ExtFunctionArg{}},
			"array_diff_assoc":           {Func: fncArrayDiffAssoc, Args: []*phpctx.ExtFunctionArg{}},
			"array_diff_key":             {Func: fncArrayDiffKey, Args: []*phpctx.ExtFunctionArg{}},
			"array_diff_uassoc":          {Func: fncArrayDiffUAssoc, Args: []*phpctx.ExtFunctionArg{}},
			"array_diff_ukey":            {Func: fncArrayDiffUKey, Args: []*phpctx.ExtFunctionArg{}},
			"array_fill":                 {Func: fncArrayFill, Args: []*phpctx.ExtFunctionArg{}},
			"array_fill_keys":            {Func: fncArrayFillKeys, Args: []*phpctx.ExtFunctionArg{}},
			"array_filter":               {Func: fncArrayFilter, Args: []*phpctx.ExtFunctionArg{}},
			"array_flip":                 {Func: fncArrayFlip, Args: []*phpctx.ExtFunctionArg{}},
			"array_intersect":            {Func: fncArrayIntersect, Args: []*phpctx.ExtFunctionArg{}},
			"array_intersect_assoc":      {Func: fncArrayIntersectAssoc, Args: []*phpctx.ExtFunctionArg{}},
			"array_intersect_key":        {Func: fncArrayIntersectKey, Args: []*phpctx.ExtFunctionArg{}},
			"array_intersect_uassoc":     {Func: fncArrayIntersectUAssoc, Args: []*phpctx.ExtFunctionArg{}},
			"array_intersect_ukey":       {Func: fncArrayIntersectUKey, Args: []*phpctx.ExtFunctionArg{}},
			"array_key_exists":           {Func: fncArrayKeyExists, Args: []*phpctx.ExtFunctionArg{}},
			"array_key_first":            {Func: fncArrayKeyFirst, Args: []*phpctx.ExtFunctionArg{}},
			"array_key_last":             {Func: fncArrayKeyLast, Args: []*phpctx.ExtFunctionArg{}},
			"array_keys":                 {Func: fncArrayKeys, Args: []*phpctx.ExtFunctionArg{}},
			"array_map":                  {Func: fncArrayMap, Args: []*phpctx.ExtFunctionArg{}},
			"array_merge":                {Func: fncArrayMerge, Args: []*phpctx.ExtFunctionArg{}},
			"array_merge_recursive":      {Func: fncArrayMergeRecursive, Args: []*phpctx.ExtFunctionArg{}},
			"array_multisort":            {Func: fncArrayMultiSort, Args: []*phpctx.ExtFunctionArg{}},
			"array_pad":                  {Func: fncArrayPad, Args: []*phpctx.ExtFunctionArg{}},
			"array_pop":                  {Func: fncArrayPop, Args: []*phpctx.ExtFunctionArg{}},
			"array_product":              {Func: fncArrayProduct, Args: []*phpctx.ExtFunctionArg{}},
			"array_push":                 {Func: fncArrayPush, Args: []*phpctx.ExtFunctionArg{}},
			"array_rand":                 {Func: fncArrayRand, Args: []*phpctx.ExtFunctionArg{}},
			"array_reduce":               {Func: fncArrayReduce, Args: []*phpctx.ExtFunctionArg{}},
			"array_replace":              {Func: fncArrayReplace, Args: []*phpctx.ExtFunctionArg{}},
			"array_replace_recursive":    {Func: fncArrayReplaceRecursive, Args: []*phpctx.ExtFunctionArg{}},
			"array_reverse":              {Func: fncArrayReverse, Args: []*phpctx.ExtFunctionArg{}},
			"array_search":               {Func: fncArraySearch, Args: []*phpctx.ExtFunctionArg{}},
			"array_shift":                {Func: fncArrayShift, Args: []*phpctx.ExtFunctionArg{}},
			"array_slice":                {Func: fncArraySlice, Args: []*phpctx.ExtFunctionArg{}},
			"array_splice":               {Func: fncArraySplice, Args: []*phpctx.ExtFunctionArg{}},
			"array_sum":                  {Func: fncArraySum, Args: []*phpctx.ExtFunctionArg{}},
			"array_udiff":                {Func: fncArrayUDiff, Args: []*phpctx.ExtFunctionArg{}},
			"array_udiff_assoc":          {Func: fncArrayUDiffAssoc, Args: []*phpctx.ExtFunctionArg{}},
			"array_udiff_uassoc":         {Func: fncArrayUDiffUAssoc, Args: []*phpctx.ExtFunctionArg{}},
			"array_uintersect":           {Func: fncArrayUIntersect, Args: []*phpctx.ExtFunctionArg{}},
			"array_uintersect_assoc":     {Func: fncArrayUIntersectAssoc, Args: []*phpctx.ExtFunctionArg{}},
			"array_uintersect_uassoc":    {Func: fncArrayUIntersectUAssoc, Args: []*phpctx.ExtFunctionArg{}},
			"array_unique":               {Func: fncArrayUnique, Args: []*phpctx.ExtFunctionArg{}},
			"array_unshift":              {Func: fncArrayUnshift, Args: []*phpctx.ExtFunctionArg{}},
			"array_values":               {Func: fncArrayValues, Args: []*phpctx.ExtFunctionArg{}},
			"array_walk":                 {Func: fncArrayWalk, Args: []*phpctx.ExtFunctionArg{}},
			"array_walk_recursive":       {Func: fncArrayWalkRecursive, Args: []*phpctx.ExtFunctionArg{}},
			"arsort":                     {Func: fncArrayARSort, Args: []*phpctx.ExtFunctionArg{}},
			"asin":                       {Func: mathAsin, Args: []*phpctx.ExtFunctionArg{}},
			"asinh":                      {Func: mathAsinh, Args: []*phpctx.ExtFunctionArg{}},
			"asort":                      {Func: fncArrayASort, Args: []*phpctx.ExtFunctionArg{}},
			"atan":                       {Func: mathAtan, Args: []*phpctx.ExtFunctionArg{}},
			"atan2":                      {Func: mathAtan2, Args: []*phpctx.ExtFunctionArg{}},
			"atanh":                      {Func: mathAtanh, Args: []*phpctx.ExtFunctionArg{}},
			"base64_decode":              {Func: fncBase64Decode, Args: []*phpctx.ExtFunctionArg{}},
			"base64_encode":              {Func: fncBase64Encode, Args: []*phpctx.ExtFunctionArg{}},
			"base_convert":               {Func: mathBaseConvert, Args: []*phpctx.ExtFunctionArg{}},
			"basename":                   {Func: fncBasename, Args: []*phpctx.ExtFunctionArg{}},
			"bin2hex":                    {Func: fncBin2hex, Args: []*phpctx.ExtFunctionArg{}},
			"bindec":                     {Func: mathBinDec, Args: []*phpctx.ExtFunctionArg{}},
			"boolval":                    {Func: fncBoolval, Args: []*phpctx.ExtFunctionArg{}},
			"call_user_func":             {Func: fncCallUserFunc, Args: []*phpctx.ExtFunctionArg{}},
			"call_user_func_array":       {Func: fncCallUserFuncArray, Args: []*phpctx.ExtFunctionArg{}},
			"ceil":                       {Func: mathCeil, Args: []*phpctx.ExtFunctionArg{}},
			"chdir":                      {Func: fncChdir, Args: []*phpctx.ExtFunctionArg{}},
			"chop":                       {Func: fncStrRtrim, Args: []*phpctx.ExtFunctionArg{}}, // alias
			"chr":                        {Func: fncChr, Args: []*phpctx.ExtFunctionArg{}},
			"chunk_split":                {Func: fncStrChunkSplit, Args: []*phpctx.ExtFunctionArg{}},
			"compact":                    {Func: fncArrayCompact, Args: []*phpctx.ExtFunctionArg{}},
			"constant":                   {Func: constant, Args: []*phpctx.ExtFunctionArg{}},
			"convert_cyr_string":         {Func: fncCyrConvertString, Args: []*phpctx.ExtFunctionArg{}},
			"convert_uudecode":           {Func: convertUuDecode, Args: []*phpctx.ExtFunctionArg{}},
			"convert_uuencode":           {Func: convertUuEncode, Args: []*phpctx.ExtFunctionArg{}},
			"cos":                        {Func: mathCos, Args: []*phpctx.ExtFunctionArg{}},
			"cosh":                       {Func: mathCosh, Args: []*phpctx.ExtFunctionArg{}},
			"count_chars":                {Func: fncStrCountChars, Args: []*phpctx.ExtFunctionArg{}},
			"crc32":                      {Func: fncStrCrc32, Args: []*phpctx.ExtFunctionArg{}},
			"current":                    {Func: fncArrayCurrent, Args: []*phpctx.ExtFunctionArg{}},
			"decbin":                     {Func: mathDecBin, Args: []*phpctx.ExtFunctionArg{}},
			"dechex":                     {Func: mathDecHex, Args: []*phpctx.ExtFunctionArg{}},
			"decoct":                     {Func: mathDecOct, Args: []*phpctx.ExtFunctionArg{}},
			"deg2rad":                    {Func: mathDeg2rad, Args: []*phpctx.ExtFunctionArg{}},
			"die":                        {Func: die, Args: []*phpctx.ExtFunctionArg{}},
			"dirname":                    {Func: fncDirname, Args: []*phpctx.ExtFunctionArg{}},
			"dl":                         {Func: stdFuncDl, Args: []*phpctx.ExtFunctionArg{}},
			"doubleval":                  {Func: fncDoubleval, Args: []*phpctx.ExtFunctionArg{}},
			"each":                       {Func: fncArrayEach, Args: []*phpctx.ExtFunctionArg{}},
			"end":                        {Func: fncArrayEnd, Args: []*phpctx.ExtFunctionArg{}},
			"eval":                       {Func: stdFuncEval, Args: []*phpctx.ExtFunctionArg{}},
			"exit":                       {Func: exit, Args: []*phpctx.ExtFunctionArg{}},
			"exp":                        {Func: mathExp, Args: []*phpctx.ExtFunctionArg{}},
			"explode":                    {Func: fncStrExplode, Args: []*phpctx.ExtFunctionArg{}},
			"expm1":                      {Func: mathExpm1, Args: []*phpctx.ExtFunctionArg{}},
			"extension_loaded":           {Func: stdFunc, Args: []*phpctx.ExtFunctionArg{}},
			"extract":                    {Func: fncArrayExtract, Args: []*phpctx.ExtFunctionArg{}},
			"fclose":                     {Func: fncFileClose, Args: []*phpctx.ExtFunctionArg{}},
			"file_exists":                {Func: fncFileExists, Args: []*phpctx.ExtFunctionArg{}},
			"file_get_contents":          {Func: fncFileGetContents, Args: []*phpctx.ExtFunctionArg{}},
			"file_put_contents":          {Func: fncFilePutContents, Args: []*phpctx.ExtFunctionArg{}},
			"floatval":                   {Func: fncFloatval, Args: []*phpctx.ExtFunctionArg{}},
			"floor":                      {Func: mathFloor, Args: []*phpctx.ExtFunctionArg{}},
			"flush":                      {Func: fncFlush, Args: []*phpctx.ExtFunctionArg{}},
			"fmod":                       {Func: mathFmod, Args: []*phpctx.ExtFunctionArg{}},
			"fopen":                      {Func: fncFileOpen, Args: []*phpctx.ExtFunctionArg{}},
			"fprintf":                    {Func: fncFPrintf, Args: []*phpctx.ExtFunctionArg{}},
			"ftruncate":                  {Func: fncFtruncate, Args: []*phpctx.ExtFunctionArg{}},
			"function_exists":            {Func: stdFuncFuncExists, Args: []*phpctx.ExtFunctionArg{}},
			"get_cfg_var":                {Func: stdFuncGetCfgVar, Args: []*phpctx.ExtFunctionArg{}},
			"get_class":                  {Func: stdGetClass, Args: []*phpctx.ExtFunctionArg{}},
			"get_html_translation_table": {Func: fncGetHtmlTranslationTable, Args: []*phpctx.ExtFunctionArg{}},
			"get_magic_quotes_gpc":       {Func: getMagicQuotesGpc, Args: []*phpctx.ExtFunctionArg{}},
			"get_magic_quotes_runtime":   {Func: getMagicQuotesRuntime, Args: []*phpctx.ExtFunctionArg{}},
			"get_resource_type":          {Func: fncGetResourceType, Args: []*phpctx.ExtFunctionArg{}},
			"getcwd":                     {Func: fncGetcwd, Args: []*phpctx.ExtFunctionArg{}},
			"getenv":                     {Func: getenv, Args: []*phpctx.ExtFunctionArg{}},
			"getopt":                     {Func: fncGetOpt, Args: []*phpctx.ExtFunctionArg{}},
			"getrandmax":                 {Func: mathMtGetRandMax, Args: []*phpctx.ExtFunctionArg{}}, // alias
			"gettype":                    {Func: fncGettype, Args: []*phpctx.ExtFunctionArg{}},
			"hebrev":                     {Func: fncHebrev, Args: []*phpctx.ExtFunctionArg{}},
			"hebrevc":                    {Func: fncHebrevc, Args: []*phpctx.ExtFunctionArg{}},
			"hex2bin":                    {Func: fncHex2Bin, Args: []*phpctx.ExtFunctionArg{}},
			"hexdec":                     {Func: mathHexDec, Args: []*phpctx.ExtFunctionArg{}},
			"highlight_file":             {Func: fncHighlightFile, Args: []*phpctx.ExtFunctionArg{}},
			"highlight_string":           {Func: fncHighlightString, Args: []*phpctx.ExtFunctionArg{}},
			"hrtime":                     {Func: stdFuncHrTime, Args: []*phpctx.ExtFunctionArg{}},
			"htmlspecialchars":           {Func: fncHtmlSpecialChars, Args: []*phpctx.ExtFunctionArg{}},
			"htmlspecialchars_decode":    {Func: fncHtmlSpecialCharsDecode, Args: []*phpctx.ExtFunctionArg{}},
			"hypot":                      {Func: mathHypot, Args: []*phpctx.ExtFunctionArg{}},
			"implode":                    {Func: fncStrImplode, Args: []*phpctx.ExtFunctionArg{}},
			"in_array":                   {Func: fncInArray, Args: []*phpctx.ExtFunctionArg{}},
			"inet_ntop":                  {Func: fncInetNtop, Args: []*phpctx.ExtFunctionArg{}},
			"inet_pton":                  {Func: fncInetPton, Args: []*phpctx.ExtFunctionArg{}},
			"intdiv":                     {Func: mathIntDiv, Args: []*phpctx.ExtFunctionArg{}},
			"intval":                     {Func: fncIntval, Args: []*phpctx.ExtFunctionArg{}},
			"is_array":                   {Func: fncIsArray, Args: []*phpctx.ExtFunctionArg{}},
			"is_bool":                    {Func: fncIsBool, Args: []*phpctx.ExtFunctionArg{}},
			"is_dir":                     {Func: fncIsDir, Args: []*phpctx.ExtFunctionArg{}},
			"is_double":                  {Func: fncIsDouble, Args: []*phpctx.ExtFunctionArg{}},
			"is_file":                    {Func: fncIsFile, Args: []*phpctx.ExtFunctionArg{}},
			"is_finite":                  {Func: mathIsFinite, Args: []*phpctx.ExtFunctionArg{}},
			"is_float":                   {Func: fncIsFloat, Args: []*phpctx.ExtFunctionArg{}},
			"is_infinite":                {Func: mathIsInfinite, Args: []*phpctx.ExtFunctionArg{}},
			"is_int":                     {Func: fncIsInt, Args: []*phpctx.ExtFunctionArg{}},
			"is_integer":                 {Func: fncIsInteger, Args: []*phpctx.ExtFunctionArg{}},
			"is_long":                    {Func: fncIsLong, Args: []*phpctx.ExtFunctionArg{}},
			"is_nan":                     {Func: mathIsNaN, Args: []*phpctx.ExtFunctionArg{}},
			"is_null":                    {Func: fncIsNull, Args: []*phpctx.ExtFunctionArg{}},
			"is_numeric":                 {Func: fncIsNumeric, Args: []*phpctx.ExtFunctionArg{}},
			"is_object":                  {Func: fncIsObject, Args: []*phpctx.ExtFunctionArg{}},
			"is_real":                    {Func: fncIsReal, Args: []*phpctx.ExtFunctionArg{}},
			"is_resource":                {Func: fncIsResource, Args: []*phpctx.ExtFunctionArg{}},
			"is_scalar":                  {Func: fncIsScalar, Args: []*phpctx.ExtFunctionArg{}},
			"is_string":                  {Func: fncIsString, Args: []*phpctx.ExtFunctionArg{}},
			"join":                       {Func: fncStrImplode, Args: []*phpctx.ExtFunctionArg{}}, // alias
			"key":                        {Func: fncArrayKey, Args: []*phpctx.ExtFunctionArg{}},
			"key_exists":                 {Func: fncArrayKeyExists, Args: []*phpctx.ExtFunctionArg{}}, // alias
			"krsort":                     {Func: fncArrayKRSort, Args: []*phpctx.ExtFunctionArg{}},
			"ksort":                      {Func: fncArrayKSort, Args: []*phpctx.ExtFunctionArg{}},
			"lcfirst":                    {Func: fncStrLcFirst, Args: []*phpctx.ExtFunctionArg{}},
			"lcg_value":                  {Func: fncLcgValue, Args: []*phpctx.ExtFunctionArg{}},
			"log":                        {Func: mathLog, Args: []*phpctx.ExtFunctionArg{}},
			"log10":                      {Func: mathLog10, Args: []*phpctx.ExtFunctionArg{}},
			"log1p":                      {Func: mathLog1P, Args: []*phpctx.ExtFunctionArg{}},
			"ltrim":                      {Func: fncStrLtrim, Args: []*phpctx.ExtFunctionArg{}},
			"max":                        {Func: mathMax, Args: []*phpctx.ExtFunctionArg{}},
			"md5":                        {Func: fncStrMd5, Args: []*phpctx.ExtFunctionArg{}},
			"md5_file":                   {Func: fncStrMd5File, Args: []*phpctx.ExtFunctionArg{}},
			"method_exists":              {Func: stdFuncMethodExists, Args: []*phpctx.ExtFunctionArg{}},
			"microtime":                  {Func: fncMicrotime, Args: []*phpctx.ExtFunctionArg{}},
			"min":                        {Func: mathMin, Args: []*phpctx.ExtFunctionArg{}},
			"mktime":                     {Func: fncMkTime, Args: []*phpctx.ExtFunctionArg{}},
			"mt_getrandmax":              {Func: mathMtGetRandMax, Args: []*phpctx.ExtFunctionArg{}},
			"mt_rand":                    {Func: mathMtRand, Args: []*phpctx.ExtFunctionArg{}},
			"mt_srand":                   {Func: mathMtSRand, Args: []*phpctx.ExtFunctionArg{}},
			"natcasesort":                {Func: fncArrayNatCaseSort, Args: []*phpctx.ExtFunctionArg{}},
			"natsort":                    {Func: fncArrayNatSort, Args: []*phpctx.ExtFunctionArg{}},
			"next":                       {Func: fncArrayNext, Args: []*phpctx.ExtFunctionArg{}},
			"nl2br":                      {Func: fncStrNewLine2Br, Args: []*phpctx.ExtFunctionArg{}},
			"number_format":              {Func: fncStrNumberFormat, Args: []*phpctx.ExtFunctionArg{}},
			"ob_clean":                   {Func: fncObClean, Args: []*phpctx.ExtFunctionArg{}},
			"ob_end_clean":               {Func: fncObEndClean, Args: []*phpctx.ExtFunctionArg{}},
			"ob_end_flush":               {Func: fncObEndFlush, Args: []*phpctx.ExtFunctionArg{}},
			"ob_flush":                   {Func: fncObFlush, Args: []*phpctx.ExtFunctionArg{}},
			"ob_get_clean":               {Func: fncObGetClean, Args: []*phpctx.ExtFunctionArg{}},
			"ob_get_contents":            {Func: fncObGetContents, Args: []*phpctx.ExtFunctionArg{}},
			"ob_get_flush":               {Func: fncObGetFlush, Args: []*phpctx.ExtFunctionArg{}},
			"ob_get_level":               {Func: fncObGetLevel, Args: []*phpctx.ExtFunctionArg{}},
			"ob_implicit_flush":          {Func: fncObImplicitFlush, Args: []*phpctx.ExtFunctionArg{}},
			"ob_start":                   {Func: fncObStart, Args: []*phpctx.ExtFunctionArg{}},
			"octdec":                     {Func: mathOctDec, Args: []*phpctx.ExtFunctionArg{}},
			"ord":                        {Func: fncOrd, Args: []*phpctx.ExtFunctionArg{}},
			"parse_str":                  {Func: fncStrParseStr, Args: []*phpctx.ExtFunctionArg{}},
			"php_sapi_name":              {Func: stdFuncSapiName, Args: []*phpctx.ExtFunctionArg{}},
			"php_uname":                  {Func: fncUname, Args: []*phpctx.ExtFunctionArg{}},
			"pi":                         {Func: mathPi, Args: []*phpctx.ExtFunctionArg{}},
			"pos":                        {Func: fncArrayCurrent, Args: []*phpctx.ExtFunctionArg{}}, // alias
			"pow":                        {Func: mathPow, Args: []*phpctx.ExtFunctionArg{}},
			"prev":                       {Func: fncArrayPrev, Args: []*phpctx.ExtFunctionArg{}},
			"print_r":                    {Func: fncPrintR, Args: []*phpctx.ExtFunctionArg{}},
			"printf":                     {Func: fncPrintf, Args: []*phpctx.ExtFunctionArg{}},
			"putenv":                     {Func: putenv, Args: []*phpctx.ExtFunctionArg{}},
			"quoted_printable_decode":    {Func: fncQuotedPrintableDecode, Args: []*phpctx.ExtFunctionArg{}},
			"quotemeta":                  {Func: fncStrQuoteMeta, Args: []*phpctx.ExtFunctionArg{}},
			"rad2deg":                    {Func: mathRad2Deg, Args: []*phpctx.ExtFunctionArg{}},
			"rand":                       {Func: mathMtRand, Args: []*phpctx.ExtFunctionArg{}}, // alias
			"range":                      {Func: fncRange, Args: []*phpctx.ExtFunctionArg{}},
			"rawurldecode":               {Func: fncRawurldecode, Args: []*phpctx.ExtFunctionArg{}},
			"rawurlencode":               {Func: fncRawurlencode, Args: []*phpctx.ExtFunctionArg{}},
			"realpath":                   {Func: fncRealPath, Args: []*phpctx.ExtFunctionArg{}},
			"register_shutdown_function": {Func: registerShutdownFunction, Args: []*phpctx.ExtFunctionArg{}},
			"rename":                     {Func: fncRename, Args: []*phpctx.ExtFunctionArg{}},
			"reset":                      {Func: fncArrayReset, Args: []*phpctx.ExtFunctionArg{}},
			"rewind":                     {Func: fncRewind, Args: []*phpctx.ExtFunctionArg{}},
			"rmdir":                      {Func: fncRmdir, Args: []*phpctx.ExtFunctionArg{}},
			"round":                      {Func: mathRound, Args: []*phpctx.ExtFunctionArg{}},
			"rsort":                      {Func: fncArrayRSort, Args: []*phpctx.ExtFunctionArg{}},
			"rtrim":                      {Func: fncStrRtrim, Args: []*phpctx.ExtFunctionArg{}},
			"scandir":                    {Func: fncScanDir, Args: []*phpctx.ExtFunctionArg{}},
			"set_time_limit":             {Func: fncSetTimeLimit, Args: []*phpctx.ExtFunctionArg{}},
			"sha1":                       {Func: fncStrSha1, Args: []*phpctx.ExtFunctionArg{}},
			"sha1_file":                  {Func: fncStrSha1File, Args: []*phpctx.ExtFunctionArg{}},
			"shell_exec":                 {Func: fncShellExec, Args: []*phpctx.ExtFunctionArg{}},
			"show_source":                {Func: fncHighlightFile, Args: []*phpctx.ExtFunctionArg{}}, // alias
			"shuffle":                    {Func: fncArrayShuffle, Args: []*phpctx.ExtFunctionArg{}},
			"similar_text":               {Func: fncSimilarText, Args: []*phpctx.ExtFunctionArg{}},
			"sleep":                      {Func: stdFuncSleep, Args: []*phpctx.ExtFunctionArg{}},
			"sort":                       {Func: fncArraySort, Args: []*phpctx.ExtFunctionArg{}},
			"soundex":                    {Func: fncSoundex, Args: []*phpctx.ExtFunctionArg{}},
			"sprintf":                    {Func: fncSprintf, Args: []*phpctx.ExtFunctionArg{}},
			"sqrt":                       {Func: mathSqrt, Args: []*phpctx.ExtFunctionArg{}},
			"srand":                      {Func: mathMtSRand, Args: []*phpctx.ExtFunctionArg{}}, // alias
			"sscanf":                     {Func: fncSscanf, Args: []*phpctx.ExtFunctionArg{}},
			"str_contains":               {Func: fncStrContains, Args: []*phpctx.ExtFunctionArg{}},
			"str_ends_with":              {Func: fncStrEndsWith, Args: []*phpctx.ExtFunctionArg{}},
			"str_getcsv":                 {Func: fncStrGetCsv, Args: []*phpctx.ExtFunctionArg{}},
			"str_ireplace":               {Func: stdStrIReplace, Args: []*phpctx.ExtFunctionArg{}},
			"str_pad":                    {Func: fncStrPad, Args: []*phpctx.ExtFunctionArg{}},
			"str_repeat":                 {Func: fncStrRepeat, Args: []*phpctx.ExtFunctionArg{}},
			"str_replace":                {Func: stdStrReplace, Args: []*phpctx.ExtFunctionArg{}},
			"str_rot13":                  {Func: fncStrRot13, Args: []*phpctx.ExtFunctionArg{}},
			"str_shuffle":                {Func: fncStrShuffle, Args: []*phpctx.ExtFunctionArg{}},
			"str_split":                  {Func: fncStrSplit, Args: []*phpctx.ExtFunctionArg{}},
			"str_starts_with":            {Func: fncStrStartsWith, Args: []*phpctx.ExtFunctionArg{}},
			"str_word_count":             {Func: fncStrWordCount, Args: []*phpctx.ExtFunctionArg{}},
			"strcasecmp":                 {Func: fncStrCaseCmp, Args: []*phpctx.ExtFunctionArg{}},
			"strchr":                     {Func: fncStrStr, Args: []*phpctx.ExtFunctionArg{}}, // alias
			"strcmp":                     {Func: fncStrCmp, Args: []*phpctx.ExtFunctionArg{}},
			"strcoll":                    {Func: fncStrColl, Args: []*phpctx.ExtFunctionArg{}},
			"strcspn":                    {Func: fncStrCspn, Args: []*phpctx.ExtFunctionArg{}},
			"stream_get_contents":        {Func: fncStreamGetContents, Args: []*phpctx.ExtFunctionArg{}},
			"strip_tags":                 {Func: fncStripTags, Args: []*phpctx.ExtFunctionArg{}},
			"stripcslashes":              {Func: fncStripCSlashes, Args: []*phpctx.ExtFunctionArg{}},
			"stripos":                    {Func: fncStrIPos, Args: []*phpctx.ExtFunctionArg{}},
			"stripslashes":               {Func: fncStripSlashes, Args: []*phpctx.ExtFunctionArg{}},
			"stristr":                    {Func: fncStrIStr, Args: []*phpctx.ExtFunctionArg{}},
			"strnatcasecmp":              {Func: fncStrNatCaseCmp, Args: []*phpctx.ExtFunctionArg{}},
			"strnatcmp":                  {Func: fncStrNatCmp, Args: []*phpctx.ExtFunctionArg{}},
			"strncasecmp":                {Func: fncStrNCaseCmp, Args: []*phpctx.ExtFunctionArg{}},
			"strncmp":                    {Func: fncStrNCmp, Args: []*phpctx.ExtFunctionArg{}},
			"strpbrk":                    {Func: fncStrPbrk, Args: []*phpctx.ExtFunctionArg{}},
			"strpos":                     {Func: fncStrPos, Args: []*phpctx.ExtFunctionArg{}},
			"strrchr":                    {Func: fncStrRChr, Args: []*phpctx.ExtFunctionArg{}},
			"strrev":                     {Func: fncStrRev, Args: []*phpctx.ExtFunctionArg{}},
			"strripos":                   {Func: fncStrIRPos, Args: []*phpctx.ExtFunctionArg{}},
			"strrpos":                    {Func: fncStrRPos, Args: []*phpctx.ExtFunctionArg{}},
			"strspn":                     {Func: fncStrSpn, Args: []*phpctx.ExtFunctionArg{}},
			"strstr":                     {Func: fncStrStr, Args: []*phpctx.ExtFunctionArg{}},
			"strtok":                     {Func: fncStrtok, Args: []*phpctx.ExtFunctionArg{}},
			"strtolower":                 {Func: fncStrToLower, Args: []*phpctx.ExtFunctionArg{}},
			"strtoupper":                 {Func: fncStrToUpper, Args: []*phpctx.ExtFunctionArg{}},
			"strtr":                      {Func: fncStrTr, Args: []*phpctx.ExtFunctionArg{}},
			"strval":                     {Func: fncStrval, Args: []*phpctx.ExtFunctionArg{}},
			"substr":                     {Func: fncSubstr, Args: []*phpctx.ExtFunctionArg{}},
			"substr_compare":             {Func: fncSubstrCompare, Args: []*phpctx.ExtFunctionArg{}},
			"substr_count":               {Func: fncSubstrCount, Args: []*phpctx.ExtFunctionArg{}},
			"substr_replace":             {Func: fncSubstrReplace, Args: []*phpctx.ExtFunctionArg{}},
			"time":                       {Func: fncTime, Args: []*phpctx.ExtFunctionArg{}},
			"trim":                       {Func: fncStrTrim, Args: []*phpctx.ExtFunctionArg{}},
			"uasort":                     {Func: fncArrayUASort, Args: []*phpctx.ExtFunctionArg{}},
			"ucfirst":                    {Func: fncUcFirst, Args: []*phpctx.ExtFunctionArg{}},
			"ucwords":                    {Func: fncUcWords, Args: []*phpctx.ExtFunctionArg{}},
			"uksort":                     {Func: fncArrayUKSort, Args: []*phpctx.ExtFunctionArg{}},
			"uniqid":                     {Func: fncUniqId, Args: []*phpctx.ExtFunctionArg{}},
			"unlink":                     {Func: fncUnlink, Args: []*phpctx.ExtFunctionArg{}},
			"urldecode":                  {Func: fncUrldecode, Args: []*phpctx.ExtFunctionArg{}},
			"urlencode":                  {Func: fncUrlencode, Args: []*phpctx.ExtFunctionArg{}},
			"usleep":                     {Func: stdFuncUsleep, Args: []*phpctx.ExtFunctionArg{}},
			"usort":                      {Func: fncArrayUSort, Args: []*phpctx.ExtFunctionArg{}},
			"var_dump":                   {Func: stdFuncVarDump, Args: []*phpctx.ExtFunctionArg{}},
			"var_export":                 {Func: stdFuncVarExport, Args: []*phpctx.ExtFunctionArg{}},
			"vfprintf":                   {Func: fncVFPrintf, Args: []*phpctx.ExtFunctionArg{}},
			"vprintf":                    {Func: fncVPrintf, Args: []*phpctx.ExtFunctionArg{}},
			"vsprintf":                   {Func: fncVSprintf, Args: []*phpctx.ExtFunctionArg{}},
			"wordwrap":                   {Func: fncWordWrap, Args: []*phpctx.ExtFunctionArg{}},
		},
		Constants: map[phpv.ZString]phpv.Val{
			"ARRAY_FILTER_USE_BOTH":                ARRAY_FILTER_USE_BOTH,
			"ARRAY_FILTER_USE_KEY":                 ARRAY_FILTER_USE_KEY,
			"CASE_LOWER":                           CASE_LOWER,
			"CASE_UPPER":                           CASE_UPPER,
			"ENT_COMPAT":                           ENT_COMPAT,
			"ENT_DISALLOWED":                       ENT_DISALLOWED,
			"ENT_HTML401":                          ENT_HTML401,
			"ENT_HTML5":                            ENT_HTML5,
			"ENT_HTML_DOC_HTML401":                 ENT_HTML_DOC_HTML401,
			"ENT_HTML_DOC_HTML5":                   ENT_HTML_DOC_HTML5,
			"ENT_HTML_DOC_TYPE_MASK":               ENT_HTML_DOC_TYPE_MASK,
			"ENT_HTML_DOC_XHTML":                   ENT_HTML_DOC_XHTML,
			"ENT_HTML_DOC_XML1":                    ENT_HTML_DOC_XML1,
			"ENT_HTML_IGNORE_ERRORS":               ENT_HTML_IGNORE_ERRORS,
			"ENT_HTML_QUOTE_DOUBLE":                ENT_HTML_QUOTE_DOUBLE,
			"ENT_HTML_QUOTE_NONE":                  ENT_HTML_QUOTE_NONE,
			"ENT_HTML_QUOTE_SINGLE":                ENT_HTML_QUOTE_SINGLE,
			"ENT_HTML_SUBSTITUTE_DISALLOWED_CHARS": ENT_HTML_SUBSTITUTE_DISALLOWED_CHARS,
			"ENT_HTML_SUBSTITUTE_ERRORS":           ENT_HTML_SUBSTITUTE_ERRORS,
			"ENT_IGNORE":                           ENT_IGNORE,
			"ENT_NOQUOTES":                         ENT_NOQUOTES,
			"ENT_QUOTES":                           ENT_QUOTES,
			"ENT_SUBSTITUTE":                       ENT_SUBSTITUTE,
			"ENT_XHTML":                            ENT_XHTML,
			"ENT_XML1":                             ENT_XML1,
			"EXTR_IF_EXISTS":                       EXTR_IF_EXISTS,
			"EXTR_OVERWRITE":                       EXTR_OVERWRITE,
			"EXTR_PREFIX_ALL":                      EXTR_PREFIX_ALL,
			"EXTR_PREFIX_IF_EXISTS":                EXTR_PREFIX_IF_EXISTS,
			"EXTR_PREFIX_INVALID":                  EXTR_PREFIX_INVALID,
			"EXTR_PREFIX_SAME":                     EXTR_PREFIX_SAME,
			"EXTR_REFS":                            EXTR_REFS,
			"EXTR_SKIP":                            EXTR_SKIP,
			"FILE_APPEND":                          FILE_APPEND,
			"FILE_IGNORE_NEW_LINES":                FILE_IGNORE_NEW_LINES,
			"FILE_NO_DEFAULT_CONTEXT":              FILE_NO_DEFAULT_CONTEXT,
			"FILE_SKIP_EMPTY_LINES":                FILE_SKIP_EMPTY_LINES,
			"FILE_USE_INCLUDE_PATH":                FILE_USE_INCLUDE_PATH,
			"HTML_ENTITIES":                        HTML_ENTITIES,
			"HTML_SPECIALCHARS":                    HTML_SPECIALCHARS,
			"INF":                                  INF,
			"LOCK_EX":                              LOCK_EX,
			"LOCK_NB":                              LOCK_NB,
			"LOCK_SH":                              LOCK_SH,
			"LOCK_UN":                              LOCK_UN,
			"M_1_PI":                               M_1_PI,
			"M_2_PI":                               M_2_PI,
			"M_2_SQRTPI":                           M_2_SQRTPI,
			"M_E":                                  M_E,
			"M_EULER":                              M_EULER,
			"M_LN2":                                M_LN2,
			"M_LNPI":                               M_LNPI,
			"M_LOG10E":                             M_LOG10E,
			"M_LOG2E":                              M_LOG2E,
			"M_PHI":                                M_PHI,
			"M_PI":                                 M_PI,
			"M_PI_2":                               M_PI_2,
			"M_PI_4":                               M_PI_4,
			"M_SQRT1_2":                            M_SQRT1_2,
			"M_SQRT2":                              M_SQRT2,
			"M_SQRT3":                              M_SQRT3,
			"M_SQRTPI":                             M_SQRTPI,
			"NAN":                                  NAN,
			"PHP_ROUND_HALF_DOWN":                  PHP_ROUND_HALF_DOWN,
			"PHP_ROUND_HALF_EVEN":                  PHP_ROUND_HALF_EVEN,
			"PHP_ROUND_HALF_ODD":                   PHP_ROUND_HALF_ODD,
			"PHP_ROUND_HALF_UP":                    PHP_ROUND_HALF_UP,
			"SCANDIR_SORT_ASCENDING":               SCANDIR_SORT_ASCENDING,
			"SCANDIR_SORT_DESCENDING":              SCANDIR_SORT_DESCENDING,
			"SCANDIR_SORT_NONE":                    SCANDIR_SORT_NONE,
			"SORT_ASC":                             SORT_ASC,
			"SORT_DESC":                            SORT_DESC,
			"SORT_FLAG_CASE":                       SORT_FLAG_CASE,
			"SORT_LOCALE_STRING":                   SORT_LOCALE_STRING,
			"SORT_NATURAL":                         SORT_NATURAL,
			"SORT_NUMERIC":                         SORT_NUMERIC,
			"SORT_REGULAR":                         SORT_REGULAR,
			"SORT_STRING":                          SORT_STRING,
			"STR_PAD_BOTH":                         STR_PAD_BOTH,
			"STR_PAD_LEFT":                         STR_PAD_LEFT,
			"STR_PAD_RIGHT":                        STR_PAD_RIGHT,
		},
	})
}
