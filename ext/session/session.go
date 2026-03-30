package session

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/MagicalTux/goro/core"
	"github.com/MagicalTux/goro/core/phpv"
)

// Session status constants
const (
	PHP_SESSION_DISABLED = phpv.ZInt(0)
	PHP_SESSION_NONE     = phpv.ZInt(1)
	PHP_SESSION_ACTIVE   = phpv.ZInt(2)
)

// sessionState holds all per-request session state.
type sessionState struct {
	status       phpv.ZInt
	id           string
	name         string
	savePath     string
	started      bool
	origData     *phpv.ZArray // copy of session data as loaded (for reset/abort)
	cookieParams map[string]*phpv.ZVal
	cacheLimiter string
	cacheExpire  int
}

// sessionStore maps GlobalContext -> *sessionState.
// Each request has a unique Global, so this is safe.
var sessionStore sync.Map

// getOrCreateSessionState retrieves or initializes session state for this request.
func getOrCreateSessionState(ctx phpv.Context) *sessionState {
	g := ctx.Global()
	if v, ok := sessionStore.Load(g); ok {
		return v.(*sessionState)
	}
	st := newSessionState(ctx)
	sessionStore.Store(g, st)
	return st
}

func newSessionState(ctx phpv.Context) *sessionState {
	name := ctx.GetConfig("session.name", phpv.ZString("PHPSESSID").ZVal()).String()
	savePath := ctx.GetConfig("session.save_path", phpv.ZString("").ZVal()).String()
	if savePath == "" {
		savePath = os.TempDir()
	}
	cacheExpireVal := ctx.GetConfig("session.cache_expire", phpv.ZInt(180).ZVal())
	cacheExpire := int(cacheExpireVal.AsInt(ctx))
	cacheLimiter := ctx.GetConfig("session.cache_limiter", phpv.ZString("nocache").ZVal()).String()

	return &sessionState{
		status:       PHP_SESSION_NONE,
		name:         name,
		savePath:     savePath,
		cacheExpire:  cacheExpire,
		cacheLimiter: cacheLimiter,
		cookieParams: makeCookieParams(ctx),
	}
}

func makeCookieParams(ctx phpv.Context) map[string]*phpv.ZVal {
	return map[string]*phpv.ZVal{
		"lifetime": ctx.GetConfig("session.cookie_lifetime", phpv.ZInt(0).ZVal()),
		"path":     ctx.GetConfig("session.cookie_path", phpv.ZString("/").ZVal()),
		"domain":   ctx.GetConfig("session.cookie_domain", phpv.ZString("").ZVal()),
		"secure":   ctx.GetConfig("session.cookie_secure", phpv.ZFalse.ZVal()),
		"httponly": ctx.GetConfig("session.cookie_httponly", phpv.ZFalse.ZVal()),
		"samesite": ctx.GetConfig("session.cookie_samesite", phpv.ZString("").ZVal()),
	}
}

// generateID generates a 32-char hex session ID.
func generateID() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

// sessionFilePath returns the path for the session file.
func sessionFilePath(savePath, id string) string {
	return filepath.Join(savePath, "sess_"+id)
}

// sessionSerialize serializes the $_SESSION array using PHP session format:
// key|serialized_value key2|serialized_value2 ...
func sessionSerialize(ctx phpv.Context, arr *phpv.ZArray) (string, error) {
	var buf bytes.Buffer
	for k, v := range arr.IterateRaw(ctx) {
		key := k.String()
		serialized, err := phpSerialize(ctx, v)
		if err != nil {
			return "", err
		}
		buf.WriteString(key)
		buf.WriteByte('|')
		buf.WriteString(serialized)
	}
	return buf.String(), nil
}

// sessionDeserialize parses PHP session format (key|value...) into a ZArray.
func sessionDeserialize(ctx phpv.Context, data string) (*phpv.ZArray, error) {
	arr := phpv.NewZArray()
	i := 0
	for i < len(data) {
		// find '|' delimiter for key
		pipe := strings.IndexByte(data[i:], '|')
		if pipe < 0 {
			break
		}
		key := data[i : i+pipe]
		i += pipe + 1

		// parse the serialized value starting at i
		val, nextOffset, err := phpUnserialize(ctx, data, i)
		if err != nil {
			break
		}
		if err := arr.OffsetSet(ctx, phpv.ZString(key).ZVal(), val); err != nil {
			return arr, err
		}
		i = nextOffset
	}
	return arr, nil
}

// loadSession reads session data from storage and populates $_SESSION.
func loadSession(ctx phpv.Context, st *sessionState) error {
	arr := phpv.NewZArray()
	fpath := sessionFilePath(st.savePath, st.id)

	data, err := os.ReadFile(fpath)
	if err == nil && len(data) > 0 {
		loaded, lerr := sessionDeserialize(ctx, string(data))
		if lerr == nil {
			arr = loaded
		}
	}
	// Store a deep copy as original data for reset/abort
	st.origData = arr.Dup()
	return ctx.Global().OffsetSet(ctx, phpv.ZString("_SESSION"), arr.ZVal())
}

// writeSession writes $_SESSION to storage.
func writeSession(ctx phpv.Context, st *sessionState) error {
	sessionVal, err := ctx.Global().OffsetGet(ctx, phpv.ZString("_SESSION"))
	if err != nil || sessionVal == nil || sessionVal.GetType() == phpv.ZtNull {
		return nil
	}
	arr := sessionVal.AsArray(ctx)
	if arr == nil {
		return nil
	}
	data, err := sessionSerialize(ctx, arr)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(st.savePath, 0755); err != nil {
		return fmt.Errorf("session: cannot create save_path %q: %w", st.savePath, err)
	}
	fpath := sessionFilePath(st.savePath, st.id)
	return os.WriteFile(fpath, []byte(data), 0600)
}

// --- Session functions ---

// > func bool session_start(array $options = [])
func fncSessionStart(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var optionsArg core.Optional[*phpv.ZVal]
	_, err := core.Expand(ctx, args, &optionsArg)
	if err != nil {
		return nil, err
	}

	st := getOrCreateSessionState(ctx)

	if st.status == PHP_SESSION_ACTIVE {
		ctx.Warn("session_start(): Session already started")
		return phpv.ZTrue.ZVal(), nil
	}

	// Apply options if provided
	if optionsArg.HasArg() {
		opt := optionsArg.Get()
		if opt != nil && opt.GetType() == phpv.ZtArray {
			optArr := opt.AsArray(ctx)
			for k, v := range optArr.IterateRaw(ctx) {
				switch k.String() {
				case "name":
					st.name = v.String()
				case "save_path":
					st.savePath = v.String()
				case "cookie_lifetime":
					st.cookieParams["lifetime"] = v
				case "cookie_path":
					st.cookieParams["path"] = v
				case "cookie_domain":
					st.cookieParams["domain"] = v
				case "cookie_secure":
					st.cookieParams["secure"] = v
				case "cookie_httponly":
					st.cookieParams["httponly"] = v
				case "cookie_samesite":
					st.cookieParams["samesite"] = v
				case "cache_limiter":
					st.cacheLimiter = v.String()
				case "cache_expire":
					st.cacheExpire = int(v.AsInt(ctx))
				}
			}
		}
	}

	// Determine session ID
	if st.id == "" {
		// Check if session cookie was sent (via $_COOKIE superglobal)
		cookieArr, _ := ctx.Global().OffsetGet(ctx, phpv.ZString("_COOKIE"))
		if cookieArr != nil && cookieArr.GetType() == phpv.ZtArray {
			cookieVal, _ := cookieArr.AsArray(ctx).OffsetGet(ctx, phpv.ZString(st.name))
			if cookieVal != nil && cookieVal.GetType() != phpv.ZtNull && cookieVal.GetType() != phpv.ZtBool {
				st.id = cookieVal.String()
			}
		}
	}

	if st.id == "" {
		newID, err := generateID()
		if err != nil {
			return phpv.ZFalse.ZVal(), fmt.Errorf("session_start(): failed to generate session ID: %w", err)
		}
		st.id = newID
	}

	// Load session data from storage
	if err := loadSession(ctx, st); err != nil {
		return phpv.ZFalse.ZVal(), err
	}

	st.status = PHP_SESSION_ACTIVE
	st.started = true
	return phpv.ZTrue.ZVal(), nil
}

// > func bool session_destroy()
func fncSessionDestroy(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	st := getOrCreateSessionState(ctx)

	if st.status != PHP_SESSION_ACTIVE {
		ctx.Warn("session_destroy(): Trying to destroy uninitialized session")
		return phpv.ZFalse.ZVal(), nil
	}

	// Remove session file
	fpath := sessionFilePath(st.savePath, st.id)
	os.Remove(fpath) // ignore error

	st.status = PHP_SESSION_NONE
	st.id = ""

	// Clean up from store
	sessionStore.Delete(ctx.Global())
	return phpv.ZTrue.ZVal(), nil
}

// > func string session_id(?string $id = null)
func fncSessionId(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var idArg core.Optional[phpv.ZString]
	_, err := core.Expand(ctx, args, &idArg)
	if err != nil {
		return nil, err
	}

	st := getOrCreateSessionState(ctx)
	oldID := st.id

	if idArg.HasArg() {
		if st.status == PHP_SESSION_ACTIVE {
			ctx.Warn("session_id(): Cannot change session id when session is active")
		} else {
			st.id = string(idArg.Get())
		}
	}

	return phpv.ZString(oldID).ZVal(), nil
}

// > func string session_name(?string $name = null)
func fncSessionName(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var nameArg core.Optional[phpv.ZString]
	_, err := core.Expand(ctx, args, &nameArg)
	if err != nil {
		return nil, err
	}

	st := getOrCreateSessionState(ctx)
	oldName := st.name

	if nameArg.HasArg() {
		if st.status == PHP_SESSION_ACTIVE {
			ctx.Warn("session_name(): Cannot change session name when session is active")
		} else {
			st.name = string(nameArg.Get())
		}
	}

	return phpv.ZString(oldName).ZVal(), nil
}

// > func int session_status()
func fncSessionStatus(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	st := getOrCreateSessionState(ctx)
	return st.status.ZVal(), nil
}

// > func bool session_write_close()
func fncSessionWriteClose(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	st := getOrCreateSessionState(ctx)

	if st.status != PHP_SESSION_ACTIVE {
		return phpv.ZTrue.ZVal(), nil
	}

	if err := writeSession(ctx, st); err != nil {
		ctx.Warn("session_write_close(): Failed to write session data: %s", err.Error())
		st.status = PHP_SESSION_NONE
		sessionStore.Delete(ctx.Global())
		return phpv.ZFalse.ZVal(), nil
	}

	st.status = PHP_SESSION_NONE
	sessionStore.Delete(ctx.Global())
	return phpv.ZTrue.ZVal(), nil
}

// > func bool session_commit() — alias of session_write_close
func fncSessionCommit(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	return fncSessionWriteClose(ctx, args)
}

// > func bool session_unset()
func fncSessionUnset(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	st := getOrCreateSessionState(ctx)

	if st.status != PHP_SESSION_ACTIVE {
		return phpv.ZTrue.ZVal(), nil
	}

	emptyArr := phpv.NewZArray()
	ctx.Global().OffsetSet(ctx, phpv.ZString("_SESSION"), emptyArr.ZVal())
	return phpv.ZTrue.ZVal(), nil
}

// > func bool session_regenerate_id(bool $delete_old = false)
func fncSessionRegenerateId(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var deleteOldArg core.Optional[phpv.ZBool]
	_, err := core.Expand(ctx, args, &deleteOldArg)
	if err != nil {
		return nil, err
	}

	st := getOrCreateSessionState(ctx)

	if st.status != PHP_SESSION_ACTIVE {
		ctx.Warn("session_regenerate_id(): Cannot regenerate session id - session is not active")
		return phpv.ZFalse.ZVal(), nil
	}

	deleteOld := deleteOldArg.HasArg() && bool(deleteOldArg.Get())
	oldID := st.id

	newID, err := generateID()
	if err != nil {
		return phpv.ZFalse.ZVal(), err
	}

	st.id = newID
	if err := writeSession(ctx, st); err != nil {
		st.id = oldID
		return phpv.ZFalse.ZVal(), err
	}

	if deleteOld && oldID != "" {
		os.Remove(sessionFilePath(st.savePath, oldID))
	}

	return phpv.ZTrue.ZVal(), nil
}

// > func string session_save_path(?string $path = null)
func fncSessionSavePath(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var pathArg core.Optional[phpv.ZString]
	_, err := core.Expand(ctx, args, &pathArg)
	if err != nil {
		return nil, err
	}

	st := getOrCreateSessionState(ctx)
	oldPath := st.savePath

	if pathArg.HasArg() {
		if st.status == PHP_SESSION_ACTIVE {
			ctx.Warn("session_save_path(): Cannot change save path when session is active")
		} else {
			st.savePath = string(pathArg.Get())
		}
	}

	return phpv.ZString(oldPath).ZVal(), nil
}

// > func string|false session_encode()
func fncSessionEncode(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	st := getOrCreateSessionState(ctx)
	if st.status != PHP_SESSION_ACTIVE {
		ctx.Warn("session_encode(): Cannot encode non-started session")
		return phpv.ZFalse.ZVal(), nil
	}

	sessionVal, err := ctx.Global().OffsetGet(ctx, phpv.ZString("_SESSION"))
	if err != nil || sessionVal == nil || sessionVal.GetType() == phpv.ZtNull {
		return phpv.ZString("").ZVal(), nil
	}
	arr := sessionVal.AsArray(ctx)
	if arr == nil {
		return phpv.ZString("").ZVal(), nil
	}

	data, err := sessionSerialize(ctx, arr)
	if err != nil {
		return phpv.ZFalse.ZVal(), err
	}
	return phpv.ZString(data).ZVal(), nil
}

// > func bool session_decode(string $data)
func fncSessionDecode(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var data phpv.ZString
	_, err := core.Expand(ctx, args, &data)
	if err != nil {
		return nil, err
	}

	st := getOrCreateSessionState(ctx)
	if st.status != PHP_SESSION_ACTIVE {
		ctx.Warn("session_decode(): Cannot decode non-started session")
		return phpv.ZFalse.ZVal(), nil
	}

	arr, err := sessionDeserialize(ctx, string(data))
	if err != nil {
		return phpv.ZFalse.ZVal(), nil
	}
	ctx.Global().OffsetSet(ctx, phpv.ZString("_SESSION"), arr.ZVal())
	return phpv.ZTrue.ZVal(), nil
}

// > func bool session_abort()
func fncSessionAbort(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	st := getOrCreateSessionState(ctx)

	if st.status != PHP_SESSION_ACTIVE {
		return phpv.ZTrue.ZVal(), nil
	}

	// Restore original data (discard changes made during this request)
	if st.origData != nil {
		ctx.Global().OffsetSet(ctx, phpv.ZString("_SESSION"), st.origData.Dup().ZVal())
	}

	st.status = PHP_SESSION_NONE
	sessionStore.Delete(ctx.Global())
	return phpv.ZTrue.ZVal(), nil
}

// > func bool session_reset()
func fncSessionReset(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	st := getOrCreateSessionState(ctx)

	if st.status != PHP_SESSION_ACTIVE {
		return phpv.ZTrue.ZVal(), nil
	}

	// Re-read session data from storage
	if err := loadSession(ctx, st); err != nil {
		return phpv.ZFalse.ZVal(), err
	}

	return phpv.ZTrue.ZVal(), nil
}

// > func int|false session_gc()
func fncSessionGc(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	st := getOrCreateSessionState(ctx)

	maxLifetime := int64(ctx.GetConfig("session.gc_maxlifetime", phpv.ZInt(1440).ZVal()).AsInt(ctx))
	if maxLifetime <= 0 {
		maxLifetime = 1440
	}

	entries, err := os.ReadDir(st.savePath)
	if err != nil {
		return phpv.ZFalse.ZVal(), nil
	}

	now := time.Now().Unix()
	var deleted phpv.ZInt

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if !strings.HasPrefix(name, "sess_") {
			continue
		}
		fpath := filepath.Join(st.savePath, name)
		fi, err := os.Stat(fpath)
		if err != nil {
			continue
		}
		age := now - fi.ModTime().Unix()
		if age >= maxLifetime {
			if os.Remove(fpath) == nil {
				deleted++
			}
		}
	}

	return deleted.ZVal(), nil
}

// > func bool session_set_cookie_params(int|array $lifetime_or_options [, string $path [, string $domain [, bool $secure [, bool $httponly]]]])
func fncSessionSetCookieParams(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	if len(args) == 0 {
		return phpv.ZTrue.ZVal(), nil
	}

	st := getOrCreateSessionState(ctx)

	// First arg can be an array of options or an int (lifetime)
	if args[0].GetType() == phpv.ZtArray {
		optArr := args[0].AsArray(ctx)
		for k, v := range optArr.IterateRaw(ctx) {
			key := k.String()
			switch key {
			case "lifetime", "path", "domain", "secure", "httponly", "samesite":
				st.cookieParams[key] = v
			}
		}
	} else {
		// positional: lifetime [, path [, domain [, secure [, httponly]]]]
		st.cookieParams["lifetime"] = args[0]
		if len(args) > 1 {
			st.cookieParams["path"] = args[1]
		}
		if len(args) > 2 {
			st.cookieParams["domain"] = args[2]
		}
		if len(args) > 3 {
			st.cookieParams["secure"] = args[3]
		}
		if len(args) > 4 {
			st.cookieParams["httponly"] = args[4]
		}
	}

	return phpv.ZTrue.ZVal(), nil
}

// > func array session_get_cookie_params()
func fncSessionGetCookieParams(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	st := getOrCreateSessionState(ctx)

	result := phpv.NewZArray()
	keys := []string{"lifetime", "path", "domain", "secure", "httponly", "samesite"}
	for _, k := range keys {
		if v, ok := st.cookieParams[k]; ok {
			result.OffsetSet(ctx, phpv.ZString(k).ZVal(), v)
		}
	}
	return result.ZVal(), nil
}

// > func string session_cache_limiter(?string $value = null)
func fncSessionCacheLimiter(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var valueArg core.Optional[phpv.ZString]
	_, err := core.Expand(ctx, args, &valueArg)
	if err != nil {
		return nil, err
	}

	st := getOrCreateSessionState(ctx)
	oldVal := st.cacheLimiter

	if valueArg.HasArg() {
		st.cacheLimiter = string(valueArg.Get())
	}

	return phpv.ZString(oldVal).ZVal(), nil
}

// > func int session_cache_expire(?int $value = null)
func fncSessionCacheExpire(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var valueArg core.Optional[phpv.ZInt]
	_, err := core.Expand(ctx, args, &valueArg)
	if err != nil {
		return nil, err
	}

	st := getOrCreateSessionState(ctx)
	oldVal := phpv.ZInt(st.cacheExpire)

	if valueArg.HasArg() {
		st.cacheExpire = int(valueArg.Get())
	}

	return oldVal.ZVal(), nil
}
