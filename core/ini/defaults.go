package ini

type IniDirective struct {
	RawDefault string
	Mode       int
}

var Defaults = map[string]IniDirective{
	"allow_url_fopen":                       {`"1"`, INI_SYSTEM},
	"allow_url_include":                     {`"0"`, INI_SYSTEM},
	"arg_separator.input":                   {`"&"`, INI_PERDIR},
	"arg_separator.output":                  {`"&"`, INI_ALL},
	"assert.active":                         {`"1"`, INI_ALL},
	"assert.bail":                           {`"0"`, INI_ALL},
	"assert.callback":                       {`null`, INI_ALL},
	"assert.exception":                      {`"0"`, INI_ALL},
	"assert.quiet_eval":                     {`"0"`, INI_ALL},
	"assert.warning":                        {`"1"`, INI_ALL},
	"auto_append_file":                      {`null`, INI_PERDIR},
	"auto_detect_line_endings":              {`"0"`, INI_ALL},
	"auto_globals_jit":                      {`"1"`, INI_PERDIR},
	"auto_prepend_file":                     {`null`, INI_PERDIR},
	"bcmath.scale":                          {`"0"`, INI_ALL},
	"browscap":                              {`null`, INI_SYSTEM},
	"cgi.check_shebang_line":                {`"1"`, INI_SYSTEM},
	"cgi.discard_path":                      {`"0"`, INI_SYSTEM},
	"cgi.fix_pathinfo":                      {`"1"`, INI_SYSTEM},
	"cgi.force_redirect":                    {`"1"`, INI_SYSTEM},
	"cgi.nph":                               {`"0"`, INI_ALL},
	"cgi.redirect_status_env":               {`null`, INI_SYSTEM},
	"cgi.rfc2616_headers":                   {`"0"`, INI_ALL},
	"child_terminate":                       {`"0"`, INI_ALL},
	"cli.pager":                             {`""`, INI_ALL},
	"cli.prompt":                            {`"\\b \\> "`, INI_ALL},
	"cli_server.color":                      {`"0"`, INI_ALL},
	"com.allow_dcom":                        {`"0"`, INI_SYSTEM},
	"com.autoregister_typelib":              {`"0"`, INI_ALL},
	"com.autoregister_verbose":              {`"0"`, INI_ALL},
	"com.autoregister_casesensitive":        {`"1"`, INI_ALL},
	"com.code_page":                         {`""`, INI_ALL},
	"com.dotnet_version":                    {`""`, INI_SYSTEM},
	"com.typelib_file":                      {`""`, INI_SYSTEM},
	"curl.cainfo":                           {`NULL`, INI_SYSTEM},
	"date.default_latitude":                 {`"31.7667"`, INI_ALL},
	"date.default_longitude":                {`"35.2333"`, INI_ALL},
	"date.sunrise_zenith":                   {`"90.833333"`, INI_ALL},
	"date.sunset_zenith":                    {`"90.833333"`, INI_ALL},
	"date.timezone":                         {`"UTC"`, INI_ALL},
	"dba.default_handler":                   {`DBA_DEFAULT`, INI_ALL},
	"default_charset":                       {`"UTF-8"`, INI_ALL},
	"input_encoding":                        {`""`, INI_ALL},
	"output_encoding":                       {`""`, INI_ALL},
	"internal_encoding":                     {`""`, INI_ALL},
	"default_mimetype":                      {`"text/html"`, INI_ALL},
	"default_socket_timeout":                {`"60"`, INI_ALL},
	"disable_classes":                       {`""`, INI_PERDIR | INI_SYSTEM},
	"disable_functions":                     {`""`, INI_PERDIR | INI_SYSTEM},
	"display_errors":                        {`"1"`, INI_ALL},
	"display_startup_errors":                {`"1"`, INI_ALL},
	"docref_ext":                            {`""`, INI_ALL},
	"docref_root":                           {`""`, INI_ALL},
	"doc_root":                              {`null`, INI_SYSTEM},
	"enable_dl":                             {`"1"`, INI_SYSTEM},
	"enable_post_data_reading":              {`"On"`, INI_PERDIR},
	"engine":                                {`"1"`, INI_ALL},
	"error_append_string":                   {`null`, INI_ALL},
	"error_log":                             {`null`, INI_ALL},
	"error_log_mode":                        {`0644`, INI_ALL},
	"error_prepend_string":                  {`null`, INI_ALL},
	"error_reporting":                       {`E_ALL`, INI_ALL},
	"exif.encode_unicode":                   {`"ISO-8859-15"`, INI_ALL},
	"exif.decode_unicode_motorola":          {`"UCS-2BE"`, INI_ALL},
	"exif.decode_unicode_intel":             {`"UCS-2LE"`, INI_ALL},
	"exif.encode_jis":                       {`""`, INI_ALL},
	"exif.decode_jis_motorola":              {`"JIS"`, INI_ALL},
	"exif.decode_jis_intel":                 {`"JIS"`, INI_ALL},
	"exit_on_timeout":                       {`""`, INI_ALL},
	"expect.timeout":                        {`"10"`, INI_ALL},
	"expect.loguser":                        {`"1"`, INI_ALL},
	"expect.logfile":                        {`""`, INI_ALL},
	"expect.match_max":                      {`""`, INI_ALL},
	"expose_php":                            {`"1"`, INI_PERDIR | INI_SYSTEM},
	"extension":                             {`null`, INI_PERDIR | INI_SYSTEM},
	"extension_dir":                         {`"/path/to/php"`, INI_SYSTEM},
	"fastcgi.impersonate":                   {`"0"`, INI_SYSTEM},
	"fastcgi.logging":                       {`"1"`, INI_SYSTEM},
	"file_uploads":                          {`"1"`, INI_SYSTEM},
	"filter.default":                        {`"unsafe_raw"`, INI_PERDIR},
	"filter.default_flags":                  {`NULL`, INI_PERDIR},
	"from":                                  {`""`, INI_ALL},
	"gd.jpeg_ignore_warning":                {`"1"`, INI_ALL},
	"geoip.custom_directory":                {`""`, INI_ALL},
	"hard_timeout":                          {`"2"`, INI_SYSTEM},
	"highlight.comment":                     {`"#FF8000"`, INI_ALL},
	"highlight.default":                     {`"#0000BB"`, INI_ALL},
	"highlight.html":                        {`"#000000"`, INI_ALL},
	"highlight.keyword":                     {`"#007700"`, INI_ALL},
	"highlight.string":                      {`"#DD0000"`, INI_ALL},
	"html_errors":                           {`"1"`, INI_ALL},
	"ibase.allow_persistent":                {`"1"`, INI_SYSTEM},
	"ibase.max_persistent":                  {`"-1"`, INI_SYSTEM},
	"ibase.max_links":                       {`"-1"`, INI_SYSTEM},
	"ibase.default_db":                      {`NULL`, INI_SYSTEM},
	"ibase.default_user":                    {`NULL`, INI_ALL},
	"ibase.default_password":                {`NULL`, INI_ALL},
	"ibase.default_charset":                 {`NULL`, INI_ALL},
	"ibase.timestampformat":                 {`"%Y-%m-%d %H:%M:%S"`, INI_ALL},
	"ibase.dateformat":                      {`"%Y-%m-%d"`, INI_ALL},
	"ibase.timeformat":                      {`"%H:%M:%S"`, INI_ALL},
	"ibm_db2.binmode":                       {`"1"`, INI_ALL},
	"ibm_db2.i5_all_pconnect":               {`"0"`, INI_SYSTEM},
	"ibm_db2.i5_allow_commit":               {`"0"`, INI_SYSTEM},
	"ibm_db2.i5_blank_userid":               {`"0"`, INI_SYSTEM},
	"ibm_db2.i5_char_trim":                  {`"0"`, INI_SYSTEM},
	"ibm_db2.i5_dbcs_alloc":                 {`"0"`, INI_SYSTEM},
	"ibm_db2.i5_guard_profile":              {`"0"`, INI_SYSTEM},
	"ibm_db2.i5_ignore_userid":              {`"0"`, INI_SYSTEM},
	"ibm_db2.i5_job_sort":                   {`"0"`, INI_SYSTEM},
	"ibm_db2.i5_log_verbose":                {`"0"`, INI_SYSTEM},
	"ibm_db2.i5_max_pconnect":               {`"0"`, INI_SYSTEM},
	"ibm_db2.i5_override_ccsid":             {`"0"`, INI_SYSTEM},
	"ibm_db2.i5_servermode_subsystem":       {`NULL`, INI_SYSTEM},
	"ibm_db2.i5_sys_naming":                 {`"0"`, INI_SYSTEM},
	"ibm_db2.instance_name":                 {`NULL`, INI_SYSTEM},
	"iconv.input_encoding":                  {`""`, INI_ALL},
	"iconv.output_encoding":                 {`""`, INI_ALL},
	"iconv.internal_encoding":               {`""`, INI_ALL},
	"ignore_repeated_errors":                {`"0"`, INI_ALL},
	"ignore_repeated_source":                {`"0"`, INI_ALL},
	"ignore_user_abort":                     {`"0"`, INI_ALL},
	"implicit_flush":                        {`"0"`, INI_ALL},
	"include_path":                          {`".:/path/to/php/pear"`, INI_ALL},
	"intl.default_locale":                   {` `, INI_ALL},
	"intl.error_level":                      {`0`, INI_ALL},
	"intl.use_exceptions":                   {`0`, INI_ALL},
	"last_modified":                         {`"0"`, INI_ALL},
	"ldap.max_links":                        {`"-1"`, INI_SYSTEM},
	"log_errors":                            {`"0"`, INI_ALL},
	"log_errors_max_len":                    {`"1024"`, INI_ALL},
	"mail.add_x_header":                     {`"0"`, INI_PERDIR},
	"mail.force_extra_parameters":           {`null`, INI_PERDIR | INI_SYSTEM},
	"mail.log":                              {`""`, INI_PERDIR},
	"max_execution_time":                    {`"30"`, INI_ALL},
	"max_input_nesting_level":               {`"64"`, INI_PERDIR},
	"max_input_vars":                        {`1000`, INI_PERDIR},
	"max_input_time":                        {`"-1"`, INI_PERDIR},
	"mbstring.language":                     {`"neutral"`, INI_ALL},
	"mbstring.detect_order":                 {`NULL`, INI_ALL},
	"mbstring.http_input":                   {`"pass"`, INI_ALL},
	"mbstring.http_output":                  {`"pass"`, INI_ALL},
	"mbstring.internal_encoding":            {`NULL`, INI_ALL},
	"mbstring.substitute_character":         {`NULL`, INI_ALL},
	"mbstring.func_overload":                {`"0"`, INI_SYSTEM},
	"mbstring.encoding_translation":         {`"0"`, INI_PERDIR},
	"mbstring.http_output_conv_mimetypes":   {`"^(text/|application/xhtml\+xml)"`, INI_ALL},
	"mbstring.strict_detection":             {`"0"`, INI_ALL},
	"mbstring.regex_retry_limit":            {`"1000000"`, INI_ALL},
	"mbstring.regex_stack_limit":            {`"100000"`, INI_ALL},
	"mcrypt.algorithms_dir":                 {`null`, INI_ALL},
	"mcrypt.modes_dir":                      {`null`, INI_ALL},
	"memcache.allow_failover":               {`"1"`, INI_ALL},
	"memcache.max_failover_attempts":        {`"20"`, INI_ALL},
	"memcache.chunk_size":                   {`"8192"`, INI_ALL},
	"memcache.default_port":                 {`"11211"`, INI_ALL},
	"memcache.hash_strategy":                {`"standard"`, INI_ALL},
	"memcache.hash_function":                {`"crc32"`, INI_ALL},
	"memcache.protocol":                     {`ascii`, INI_ALL},
	"memcache.redundancy":                   {`1`, INI_ALL},
	"memcache.session_redundancy":           {`2`, INI_ALL},
	"memcache.compress_threshold":           {`20000`, INI_ALL},
	"memcache.lock_timeout":                 {`15`, INI_ALL},
	"memory_limit":                          {`"128M"`, INI_ALL},
	"mysql.allow_local_infile":              {`"1"`, INI_SYSTEM},
	"mysql.allow_persistent":                {`"1"`, INI_SYSTEM},
	"mysql.max_persistent":                  {`"-1"`, INI_SYSTEM},
	"mysql.max_links":                       {`"-1"`, INI_SYSTEM},
	"mysql.trace_mode":                      {`"0"`, INI_ALL},
	"mysql.default_port":                    {`NULL`, INI_ALL},
	"mysql.default_socket":                  {`NULL`, INI_ALL},
	"mysql.default_host":                    {`NULL`, INI_ALL},
	"mysql.default_user":                    {`NULL`, INI_ALL},
	"mysql.default_password":                {`NULL`, INI_ALL},
	"mysql.connect_timeout":                 {`"60"`, INI_ALL},
	"mysqli.allow_local_infile":             {`"0"`, INI_SYSTEM},
	"mysqli.local_infile_directory":         {` `, INI_SYSTEM},
	"mysqli.allow_persistent":               {`"1"`, INI_SYSTEM},
	"mysqli.max_persistent":                 {`"-1"`, INI_SYSTEM},
	"mysqli.max_links":                      {`"-1"`, INI_SYSTEM},
	"mysqli.default_port":                   {`"3306"`, INI_ALL},
	"mysqli.default_socket":                 {`NULL`, INI_ALL},
	"mysqli.default_host":                   {`NULL`, INI_ALL},
	"mysqli.default_user":                   {`NULL`, INI_ALL},
	"mysqli.default_pw":                     {`NULL`, INI_ALL},
	"mysqli.reconnect":                      {`"0"`, INI_SYSTEM},
	"mysqli.rollback_on_cached_plink":       {`"0"`, INI_SYSTEM},
	"mysqlnd.collect_statistics":            {`"1"`, INI_SYSTEM},
	"mysqlnd.collect_memory_statistics":     {`"0"`, INI_SYSTEM},
	"mysqlnd.debug":                         {`""`, INI_SYSTEM},
	"mysqlnd.log_mask":                      {`0`, INI_ALL},
	"mysqlnd.mempool_default_size":          {`16000`, INI_ALL},
	"mysqlnd.net_read_timeout":              {`"86400"`, INI_ALL},
	"mysqlnd.net_cmd_buffer_size":           {`"4096"`, INI_SYSTEM},
	"mysqlnd.net_read_buffer_size":          {`"32768"`, INI_SYSTEM},
	"mysqlnd.sha256_server_public_key":      {`""`, INI_PERDIR},
	"mysqlnd.trace_alloc":                   {`""`, INI_SYSTEM},
	"mysqlnd.fetch_data_copy":               {`0`, INI_ALL},
	"oci8.connection_class":                 {`""`, INI_ALL},
	"oci8.default_prefetch":                 {`"100"`, INI_SYSTEM},
	"oci8.events":                           {`Off`, INI_SYSTEM},
	"oci8.max_persistent":                   {`"-1"`, INI_SYSTEM},
	"oci8.old_oci_close_semantics":          {`Off`, INI_SYSTEM},
	"oci8.persistent_timeout":               {`"-1"`, INI_SYSTEM},
	"oci8.ping_interval":                    {`"60"`, INI_SYSTEM},
	"oci8.prefetch_lob_size":                {`"0"`, INI_SYSTEM},
	"oci8.privileged_connect":               {`Off`, INI_SYSTEM},
	"oci8.statement_cache_size":             {`"20"`, INI_SYSTEM},
	"odbc.allow_persistent":                 {`"1"`, INI_SYSTEM},
	"odbc.check_persistent":                 {`"1"`, INI_SYSTEM},
	"odbc.max_persistent":                   {`"-1"`, INI_SYSTEM},
	"odbc.max_links":                        {`"-1"`, INI_SYSTEM},
	"odbc.defaultlrl":                       {`"4096"`, INI_ALL},
	"odbc.defaultbinmode":                   {`"1"`, INI_ALL},
	"odbc.default_cursortype":               {`"3"`, INI_ALL},
	"opcache.enable":                        {`1`, INI_ALL},
	"opcache.enable_cli":                    {`0`, INI_SYSTEM},
	"opcache.memory_consumption":            {`128`, INI_SYSTEM},
	"opcache.interned_strings_buffer":       {`8`, INI_SYSTEM},
	"opcache.max_accelerated_files":         {`10000`, INI_SYSTEM},
	"opcache.max_wasted_percentage":         {`5`, INI_SYSTEM},
	"opcache.use_cwd":                       {`1`, INI_SYSTEM},
	"opcache.validate_timestamps":           {`1`, INI_ALL},
	"opcache.revalidate_freq":               {`2`, INI_ALL},
	"opcache.revalidate_path":               {`0`, INI_ALL},
	"opcache.save_comments":                 {`1`, INI_SYSTEM},
	"opcache.fast_shutdown":                 {`0`, INI_SYSTEM},
	"opcache.enable_file_override":          {`0`, INI_SYSTEM},
	"opcache.optimization_level":            {`0x7FFEBFFF`, INI_SYSTEM},
	"opcache.inherited_hack":                {`1`, INI_SYSTEM},
	"opcache.dups_fix":                      {`0`, INI_ALL},
	"opcache.blacklist_filename":            {`""`, INI_SYSTEM},
	"opcache.max_file_size":                 {`0`, INI_SYSTEM},
	"opcache.consistency_checks":            {`0`, INI_ALL},
	"opcache.force_restart_timeout":         {`180`, INI_SYSTEM},
	"opcache.error_log":                     {`""`, INI_SYSTEM},
	"opcache.log_verbosity_level":           {`1`, INI_SYSTEM},
	"opcache.record_warnings":               {`0`, INI_SYSTEM},
	"opcache.preferred_memory_model":        {`""`, INI_SYSTEM},
	"opcache.protect_memory":                {`0`, INI_SYSTEM},
	"opcache.mmap_base":                     {`null`, INI_SYSTEM},
	"opcache.restrict_api":                  {`""`, INI_SYSTEM},
	"opcache.file_update_protection":        {`2`, INI_ALL},
	"opcache.huge_code_pages":               {`0`, INI_SYSTEM},
	"opcache.lockfile_path":                 {`"/tmp"`, INI_SYSTEM},
	"opcache.opt_debug_level":               {`0`, INI_SYSTEM},
	"opcache.file_cache":                    {`null`, INI_SYSTEM},
	"opcache.file_cache_only":               {`0`, INI_SYSTEM},
	"opcache.file_cache_consistency_checks": {`1`, INI_SYSTEM},
	"opcache.file_cache_fallback":           {`1`, INI_SYSTEM},
	"opcache.validate_permission":           {`0`, INI_SYSTEM},
	"opcache.validate_root":                 {`0`, INI_SYSTEM},
	"opcache.preload":                       {`""`, INI_SYSTEM},
	"opcache.preload_user":                  {`""`, INI_SYSTEM},
	"opcache.cache_id":                      {`""`, INI_SYSTEM},
	"opcache.jit":                           {`"tracing"`, INI_ALL},
	"opcache.jit_buffer_size":               {`0`, INI_SYSTEM},
	"opcache.jit_debug":                     {`0`, INI_ALL},
	"opcache.jit_bisect_limit":              {`0`, INI_ALL},
	"opcache.jit_prof_threshold":            {`0.005`, INI_ALL},
	"opcache.jit_max_root_traces":           {`1024`, INI_SYSTEM},
	"opcache.jit_max_side_traces":           {`128`, INI_SYSTEM},
	"opcache.jit_max_exit_counters":         {`8192`, INI_SYSTEM},
	"opcache.jit_hot_loop":                  {`64`, INI_SYSTEM},
	"opcache.jit_hot_func":                  {`127`, INI_SYSTEM},
	"opcache.jit_hot_return":                {`8`, INI_SYSTEM},
	"opcache.jit_hot_side_exit":             {`8`, INI_SYSTEM},
	"opcache.jit_blacklist_root_trace":      {`16`, INI_ALL},
	"opcache.jit_blacklist_side_trace":      {`8`, INI_ALL},
	"opcache.jit_max_loop_unrolls":          {`8`, INI_ALL},
	"opcache.jit_max_recursive_calls":       {`2`, INI_ALL},
	"opcache.jit_max_recursive_returns":     {`2`, INI_ALL},
	"opcache.jit_max_polymorphic_calls":     {`2`, INI_ALL},
	"open_basedir":                          {`null`, INI_ALL},
	"output_buffering":                      {`"0"`, INI_PERDIR},
	"output_handler":                        {`null`, INI_PERDIR},
	"pcre.backtrack_limit":                  {`"1000000"`, INI_ALL},
	"pcre.recursion_limit":                  {`"100000"`, INI_ALL},
	"pcre.jit":                              {`"1"`, INI_ALL},
	"pdo.dsn.*":                             {` `, INI_PERDIR | INI_SYSTEM},
	"pdo_odbc.connection_pooling":           {`"strict"`, INI_ALL},
	"pdo_odbc.db2_instance_name":            {`NULL`, INI_SYSTEM},
	"pgsql.allow_persistent":                {`"1"`, INI_SYSTEM},
	"pgsql.max_persistent":                  {`"-1"`, INI_SYSTEM},
	"pgsql.max_links":                       {`"-1"`, INI_SYSTEM},
	"pgsql.auto_reset_persistent":           {`"0"`, INI_SYSTEM},
	"pgsql.ignore_notice":                   {`"0"`, INI_ALL},
	"pgsql.log_notice":                      {`"0"`, INI_ALL},
	"phar.readonly":                         {`"1"`, INI_ALL},
	"phar.require_hash":                     {`"1"`, INI_ALL},
	"phar.cache_list":                       {`""`, INI_SYSTEM},
	"post_max_size":                         {`"8M"`, INI_PERDIR},
	"precision":                             {`"14"`, INI_ALL},
	"realpath_cache_size":                   {`"16K"`, INI_SYSTEM},
	"realpath_cache_ttl":                    {`"120"`, INI_SYSTEM},
	"register_argc_argv":                    {`"1"`, INI_PERDIR},
	"report_memleaks":                       {`"1"`, INI_ALL},
	"report_zend_debug":                     {`"1"`, INI_ALL},
	"request_order":                         {`""`, INI_PERDIR},
	"runkit.superglobal":                    {`""`, INI_PERDIR},
	"runkit.internal_override":              {`"0"`, INI_SYSTEM},
	"sendmail_from":                         {`null`, INI_ALL},
	"sendmail_path":                         {`"/usr/sbin/sendmail -t -i"`, INI_SYSTEM},
	"serialize_precision":                   {`"-1"`, INI_ALL},
	"session.save_path":                     {`""`, INI_ALL},
	"session.name":                          {`"PHPSESSID"`, INI_ALL},
	"session.save_handler":                  {`"files"`, INI_ALL},
	"session.auto_start":                    {`"0"`, INI_PERDIR},
	"session.gc_probability":                {`"1"`, INI_ALL},
	"session.gc_divisor":                    {`"100"`, INI_ALL},
	"session.gc_maxlifetime":                {`"1440"`, INI_ALL},
	"session.serialize_handler":             {`"php"`, INI_ALL},
	"session.cookie_lifetime":               {`"0"`, INI_ALL},
	"session.cookie_path":                   {`"/"`, INI_ALL},
	"session.cookie_domain":                 {`""`, INI_ALL},
	"session.cookie_secure":                 {`"0"`, INI_ALL},
	"session.cookie_httponly":               {`"0"`, INI_ALL},
	"session.cookie_samesite":               {`""`, INI_ALL},
	"session.use_strict_mode":               {`"0"`, INI_ALL},
	"session.use_cookies":                   {`"1"`, INI_ALL},
	"session.use_only_cookies":              {`"1"`, INI_ALL},
	"session.referer_check":                 {`""`, INI_ALL},
	"session.cache_limiter":                 {`"nocache"`, INI_ALL},
	"session.cache_expire":                  {`"180"`, INI_ALL},
	"session.use_trans_sid":                 {`"0"`, INI_ALL},
	"session.trans_sid_tags":                {`"a=href,area=href,frame=src,form="`, INI_ALL},
	"session.trans_sid_hosts":               {`$_SERVER['HTTP_HOST']`, INI_ALL},
	"session.sid_length":                    {`"32"`, INI_ALL},
	"session.sid_bits_per_character":        {`"4"`, INI_ALL},
	"session.upload_progress.enabled":       {`"1"`, INI_PERDIR},
	"session.upload_progress.cleanup":       {`"1"`, INI_PERDIR},
	"session.upload_progress.prefix":        {`"upload_progress_"`, INI_PERDIR},
	"session.upload_progress.name":          {`"PHP_SESSION_UPLOAD_PROGRESS"`, INI_PERDIR},
	"session.upload_progress.freq":          {`"1%"`, INI_PERDIR},
	"session.upload_progress.min_freq":      {`"1"`, INI_PERDIR},
	"session.lazy_write":                    {`"1"`, INI_ALL},
	"session.hash_function":                 {`"0"`, INI_ALL},
	"session.hash_bits_per_character":       {`"4"`, INI_ALL},
	"session.entropy_file":                  {`""`, INI_ALL},
	"session.entropy_length":                {`"0"`, INI_ALL},
	"short_open_tag":                        {`"1"`, INI_PERDIR},
	"SMTP":                                  {`"localhost"`, INI_ALL},
	"smtp_port":                             {`"25"`, INI_ALL},
	"soap.wsdl_cache_enabled":               {`1`, INI_ALL},
	"soap.wsdl_cache_dir":                   {`"/tmp"`, INI_ALL},
	"soap.wsdl_cache_ttl":                   {`86400`, INI_ALL},
	"soap.wsdl_cache":                       {`1`, INI_ALL},
	"soap.wsdl_cache_limit":                 {`5`, INI_ALL},
	"sql.safe_mode":                         {`"0"`, INI_SYSTEM},
	"sqlite3.extension_dir":                 {`""`, INI_SYSTEM},
	"sqlite3.defensive":                     {`1`, INI_USER},
	"syslog.facility":                       {`"LOG_USER"`, INI_SYSTEM},
	"syslog.filter":                         {`"no-ctrl"`, INI_ALL},
	"syslog.ident":                          {`"php"`, INI_SYSTEM},
	"sys_temp_dir":                          {`""`, INI_SYSTEM},
	"sysvshm.init_mem":                      {`10000`, INI_SYSTEM},
	"tidy.default_config":                   {`""`, INI_SYSTEM},
	"tidy.clean_output":                     {`"0"`, INI_USER},
	"track_errors":                          {`"0"`, INI_ALL},
	"unserialize_callback_func":             {`null`, INI_ALL},
	"unserialize_max_depth":                 {`"4096"`, INI_ALL},
	"uploadprogress.file.filename_template": {`"/tmp/upt_%s.txt"`, INI_ALL},
	"upload_max_filesize":                   {`"2M"`, INI_PERDIR},
	"max_file_uploads":                      {`20`, INI_SYSTEM},
	"upload_tmp_dir":                        {`null`, INI_SYSTEM},
	"url_rewriter.hosts":                    {`""`, INI_ALL},
	"url_rewriter.tags":                     {`"form="`, INI_ALL},
	"user_agent":                            {`null`, INI_ALL},
	"user_dir":                              {`null`, INI_SYSTEM},
	"user_ini.cache_ttl":                    {`"300"`, INI_SYSTEM},
	"user_ini.filename":                     {`".user.ini"`, INI_SYSTEM},
	"uopz.disable":                          {`"0"`, INI_SYSTEM},
	"uopz.exit":                             {`"0"`, INI_SYSTEM},
	"uopz.overloads":                        {`"1"`, INI_SYSTEM},
	"variables_order":                       {`"EGPCS"`, INI_PERDIR},
	"windows.show_crt_warning":              {`"0"`, INI_ALL},
	"xbithack":                              {`"0"`, INI_ALL},
	"xmlrpc_errors":                         {`"0"`, INI_SYSTEM},
	"xmlrpc_error_number":                   {`"0"`, INI_ALL},
	"yaz.keepalive":                         {`"120"`, INI_ALL},
	"yaz.log_mask":                          {`null`, INI_ALL},
	"zend.assertions":                       {`"1"`, INI_ALL},
	"zend.detect_unicode":                   {`"1"`, INI_ALL},
	"zend.enable_gc":                        {`"1"`, INI_ALL},
	"zend.multibyte":                        {`"0"`, INI_PERDIR},
	"zend.script_encoding":                  {`null`, INI_ALL},
	"zend.signal_check":                     {`"0"`, INI_SYSTEM},
	"zend_extension":                        {`null`, INI_PERDIR | INI_SYSTEM},
	"zlib.output_compression":               {`"0"`, INI_ALL},
	"zlib.output_compression_level":         {`"-1"`, INI_ALL},
	"zlib.output_handler":                   {`""`, INI_ALL},
}
