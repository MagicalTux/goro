// Package locale provides a pure-Go emulation of POSIX setlocale/localeconv/
// nl_langinfo for PHP. The previous implementation shelled out to libc via cgo,
// which made behavior depend on whichever locales happened to be generated on
// the host system (and was not thread-safe). This implementation ships its
// own locale data, so output is deterministic everywhere.
//
// The "C" and "POSIX" locales are always available and fully populated.
// A handful of common aliases (en_US, C.UTF-8, etc.) are mapped onto the C
// locale. Unknown locale names cause SetLocale to return false, matching
// PHP's behavior on a host that doesn't have the requested locale installed.
package locale

import (
	"os"
	"strings"
	"sync"

	"github.com/KarpelesLab/goro/core/phpv"
)

// Category codes passed to setlocale(). These are the PHP-visible values and
// are also used internally as array indices — keep them dense and in the
// listed order.
//
// > const
const (
	LC_CTYPE phpv.ZInt = iota
	LC_NUMERIC
	LC_TIME
	LC_COLLATE
	LC_MONETARY
	LC_MESSAGES
	LC_ALL
)

const lcCategoryCount = int(LC_ALL) // LC_ALL is a pseudo-category, not stored

// Item codes for nl_langinfo(). The integer values match GNU libc's
// langinfo.h on x86_64 Linux so C code that happened to see these constants
// via goro continues to work.
//
// > const
const (
	D_T_FMT    phpv.ZInt = 131112
	D_FMT      phpv.ZInt = 131113
	T_FMT      phpv.ZInt = 131114
	T_FMT_AMPM phpv.ZInt = 131115
	AM_STR     phpv.ZInt = 131110
	PM_STR     phpv.ZInt = 131111
	DAY_1      phpv.ZInt = 131079
	DAY_2      phpv.ZInt = 131080
	DAY_3      phpv.ZInt = 131081
	DAY_4      phpv.ZInt = 131082
	DAY_5      phpv.ZInt = 131083
	DAY_6      phpv.ZInt = 131084
	DAY_7      phpv.ZInt = 131085
	ABDAY_1    phpv.ZInt = 131072
	ABDAY_2    phpv.ZInt = 131073
	ABDAY_3    phpv.ZInt = 131074
	ABDAY_4    phpv.ZInt = 131075
	ABDAY_5    phpv.ZInt = 131076
	ABDAY_6    phpv.ZInt = 131077
	ABDAY_7    phpv.ZInt = 131078
	MON_1      phpv.ZInt = 131098
	MON_2      phpv.ZInt = 131099
	MON_3      phpv.ZInt = 131100
	MON_4      phpv.ZInt = 131101
	MON_5      phpv.ZInt = 131102
	MON_6      phpv.ZInt = 131103
	MON_7      phpv.ZInt = 131104
	MON_8      phpv.ZInt = 131105
	MON_9      phpv.ZInt = 131106
	MON_10     phpv.ZInt = 131107
	MON_11     phpv.ZInt = 131108
	MON_12     phpv.ZInt = 131109
	ABMON_1    phpv.ZInt = 131086
	ABMON_2    phpv.ZInt = 131087
	ABMON_3    phpv.ZInt = 131088
	ABMON_4    phpv.ZInt = 131089
	ABMON_5    phpv.ZInt = 131090
	ABMON_6    phpv.ZInt = 131091
	ABMON_7    phpv.ZInt = 131092
	ABMON_8    phpv.ZInt = 131093
	ABMON_9    phpv.ZInt = 131094
	ABMON_10   phpv.ZInt = 131095
	ABMON_11   phpv.ZInt = 131096
	ABMON_12   phpv.ZInt = 131097
	RADIXCHAR  phpv.ZInt = 65536
	THOUSEP    phpv.ZInt = 65537
	YESSTR     phpv.ZInt = 327682
	YESEXPR    phpv.ZInt = 327680
	NOSTR      phpv.ZInt = 327683
	NOEXPR     phpv.ZInt = 327681
	CRNCYSTR   phpv.ZInt = 262159
	CODESET    phpv.ZInt = 14
)

// LconvNumericFormat mirrors the subset of struct lconv used for either the
// local or international monetary format.
type LconvNumericFormat struct {
	CurrencySymbol string
	FragDigits     int8
	PCsPrecedes    int8
	NCsPrecedes    int8
	PSepBySpace    int8
	NSepBySpace    int8
	PSignPosN      int8
	NSignPosN      int8
}

// Lconv mirrors the fields localeconv() returns to PHP.
type Lconv struct {
	DecimalPoint string
	ThousandsSep string
	Grouping     string

	MonDecimalPoint string
	MonThousandsSep string
	MonGrouping     string
	PositiveSign    string
	NegativeSign    string

	LocalFormat LconvNumericFormat
	IntlFormat  LconvNumericFormat
}

// data holds the full contents of a named locale. Fields are what the public
// API reads; everything else about the locale is derived from these.
type data struct {
	name string
	conv Lconv

	dtFmt, dFmt, tFmt, tFmtAmpm string
	amStr, pmStr                string

	// Sunday..Saturday (POSIX day index 1..7 → DAY_1..DAY_7 and ABDAY_*).
	days    [7]string
	abdays  [7]string
	months  [12]string
	abmonths [12]string

	yesStr  string
	yesExpr string
	noStr   string
	noExpr  string

	codeset  string
	crncystr string
}

// cLocale is the POSIX "C" locale. Values match what glibc returns for
// nl_langinfo/localeconv under LC_ALL=C.
var cLocale = &data{
	name: "C",
	conv: Lconv{
		DecimalPoint: ".",
		ThousandsSep: "",
		Grouping:     "",

		MonDecimalPoint: "",
		MonThousandsSep: "",
		MonGrouping:     "",
		PositiveSign:    "",
		NegativeSign:    "",

		// In the C locale every monetary field is empty / CHAR_MAX. PHP
		// surfaces CHAR_MAX as 127 in the int8 fields.
		LocalFormat: LconvNumericFormat{
			CurrencySymbol: "",
			FragDigits:     127,
			PCsPrecedes:    127,
			NCsPrecedes:    127,
			PSepBySpace:    127,
			NSepBySpace:    127,
			PSignPosN:      127,
			NSignPosN:      127,
		},
		IntlFormat: LconvNumericFormat{
			CurrencySymbol: "",
			FragDigits:     127,
			PCsPrecedes:    127,
			NCsPrecedes:    127,
			PSepBySpace:    127,
			NSepBySpace:    127,
			PSignPosN:      127,
			NSignPosN:      127,
		},
	},

	dtFmt:    "%a %b %e %H:%M:%S %Y",
	dFmt:     "%m/%d/%y",
	tFmt:     "%H:%M:%S",
	tFmtAmpm: "%I:%M:%S %p",
	amStr:    "AM",
	pmStr:    "PM",

	days:    [7]string{"Sunday", "Monday", "Tuesday", "Wednesday", "Thursday", "Friday", "Saturday"},
	abdays:  [7]string{"Sun", "Mon", "Tue", "Wed", "Thu", "Fri", "Sat"},
	months:  [12]string{"January", "February", "March", "April", "May", "June", "July", "August", "September", "October", "November", "December"},
	abmonths: [12]string{"Jan", "Feb", "Mar", "Apr", "May", "Jun", "Jul", "Aug", "Sep", "Oct", "Nov", "Dec"},

	yesStr:  "yes",
	yesExpr: "^[yY]",
	noStr:   "no",
	noExpr:  "^[nN]",

	codeset:  "ANSI_X3.4-1968",
	crncystr: "",
}

// registry maps normalized locale names to their backing data. Multiple names
// may point at the same data — for instance "en_US" and "C.UTF-8" both share
// the C locale because all the language content is identical; only the
// reported codeset differs, and we surface that via the name (see LangInfo).
var registry = map[string]*data{
	"c":     cLocale,
	"posix": cLocale,

	// UTF-8 variants: content identical to C, only the codeset changes. The
	// codeset is derived on the fly from the active locale name, so we can
	// share the data struct.
	"c.utf-8":     cLocale,
	"c.utf8":      cLocale,
	"posix.utf-8": cLocale,
	"posix.utf8":  cLocale,

	// English aliases use the same content as C (English day/month names,
	// "." decimal separator). Tests that ask for en_US-style locales on a
	// typical Linux box get essentially the same output.
	"en":             cLocale,
	"en_us":          cLocale,
	"en_us.utf-8":    cLocale,
	"en_us.utf8":     cLocale,
	"english":        cLocale,
	"english_us":     cLocale,
}

var (
	mu sync.Mutex

	// current is the active locale per category. LC_ALL is the pseudo-
	// category meaning "all of them", so it's not stored here directly —
	// queries for LC_ALL synthesize a composite name from the per-category
	// values.
	current [lcCategoryCount]*data
)

func init() {
	for i := range current {
		current[i] = cLocale
	}
}

// normalize turns a user-supplied locale string into the key used in the
// registry. This is case-insensitive and collapses the various ways UTF-8 is
// spelled (utf-8, utf8, UTF-8).
func normalize(name string) string {
	n := strings.ToLower(strings.TrimSpace(name))
	// Map the two common spellings onto a single form so we need fewer
	// registry entries.
	n = strings.ReplaceAll(n, "utf-8", "utf8")
	return n
}

// lookup returns the locale backing the given name, or nil if no registered
// locale matches. The empty string is handled by the caller (it means "read
// from the environment").
func lookup(name string) *data {
	if name == "" {
		return nil
	}
	if d, ok := registry[normalize(name)]; ok {
		return d
	}
	// PHP on glibc accepts lone language codes as locale names (e.g. just
	// "en"). We already cover the common ones in the registry, but also
	// strip a trailing @modifier (e.g. "de_DE@euro") so a modifier doesn't
	// defeat the lookup.
	if i := strings.IndexByte(name, '@'); i >= 0 {
		return lookup(name[:i])
	}
	return nil
}

// fromEnv returns the locale selected by environment variables for the given
// category, following the POSIX precedence: LC_ALL > LC_<category> > LANG.
// The returned name is not yet validated against the registry.
func fromEnv(category phpv.ZInt) string {
	if v := os.Getenv("LC_ALL"); v != "" {
		return v
	}
	var envName string
	switch category {
	case LC_CTYPE:
		envName = "LC_CTYPE"
	case LC_NUMERIC:
		envName = "LC_NUMERIC"
	case LC_TIME:
		envName = "LC_TIME"
	case LC_COLLATE:
		envName = "LC_COLLATE"
	case LC_MONETARY:
		envName = "LC_MONETARY"
	case LC_MESSAGES:
		envName = "LC_MESSAGES"
	}
	if envName != "" {
		if v := os.Getenv(envName); v != "" {
			return v
		}
	}
	if v := os.Getenv("LANG"); v != "" {
		return v
	}
	return "C"
}

// GetLocale returns the name of the active locale for the given category.
// For LC_ALL it returns either a single name (when all categories match) or
// the POSIX composite form "LC_CTYPE=...;LC_NUMERIC=...;..." otherwise.
func GetLocale(category phpv.ZInt) phpv.ZString {
	mu.Lock()
	defer mu.Unlock()

	if category == LC_ALL {
		return phpv.ZString(allName())
	}
	if int(category) < 0 || int(category) >= lcCategoryCount {
		return ""
	}
	return phpv.ZString(current[category].name)
}

// SetLocale activates the given locale for the specified category. If name is
// the empty string, the locale is taken from the environment. On success it
// returns the canonical name of the now-active locale and ok=true. If the
// requested locale is unknown, it returns ok=false and leaves state alone.
func SetLocale(category phpv.ZInt, name phpv.ZString) (phpv.ZString, bool) {
	mu.Lock()
	defer mu.Unlock()

	requested := string(name)
	if requested == "" {
		requested = fromEnv(category)
	}

	d := lookup(requested)
	if d == nil {
		return "", false
	}
	// Record the requested name rather than the canonical one for
	// specificity-preserving variants (e.g. "en_US.UTF-8" vs "C"). This
	// lets GetLocale/SetLocale round-trip the exact name the user asked
	// for, which PHP tests compare against.
	resolved := &data{}
	*resolved = *d
	resolved.name = canonicalName(requested, d.name)

	if category == LC_ALL {
		for i := range current {
			current[i] = resolved
		}
	} else if int(category) >= 0 && int(category) < lcCategoryCount {
		current[category] = resolved
	} else {
		return "", false
	}

	return phpv.ZString(resolved.name), true
}

// canonicalName picks which name to report back to callers. If the user asked
// for a recognized variant (e.g. "en_US.UTF-8") we echo their spelling;
// otherwise fall back to the underlying data's canonical name.
func canonicalName(requested, fallback string) string {
	t := strings.TrimSpace(requested)
	if t == "" {
		return fallback
	}
	return t
}

// allName builds the name returned by GetLocale(LC_ALL). When every category
// shares the same locale we return it directly; otherwise we build the POSIX
// composite form so callers can still round-trip it back to SetLocale.
func allName() string {
	name := current[0].name
	same := true
	for i := 1; i < lcCategoryCount; i++ {
		if current[i].name != name {
			same = false
			break
		}
	}
	if same {
		return name
	}
	var b strings.Builder
	categoryNames := []string{"LC_CTYPE", "LC_NUMERIC", "LC_TIME", "LC_COLLATE", "LC_MONETARY", "LC_MESSAGES"}
	for i, cn := range categoryNames {
		if i > 0 {
			b.WriteByte(';')
		}
		b.WriteString(cn)
		b.WriteByte('=')
		b.WriteString(current[i].name)
	}
	return b.String()
}

// active returns the data struct active for the given category. For the
// pseudo-category LC_ALL we fall back to LC_CTYPE, which is how libc
// disambiguates langinfo/localeconv lookups.
func active(category phpv.ZInt) *data {
	if category == LC_ALL || int(category) < 0 || int(category) >= lcCategoryCount {
		return current[LC_CTYPE]
	}
	return current[category]
}

// Localeconv returns the localeconv(3) view of the currently-active numeric
// and monetary locales. LC_NUMERIC feeds the non-monetary fields and
// LC_MONETARY feeds the rest — matching what C's localeconv() does.
func Localeconv() *Lconv {
	mu.Lock()
	defer mu.Unlock()

	num := current[LC_NUMERIC].conv
	mon := current[LC_MONETARY].conv

	result := &Lconv{
		DecimalPoint: num.DecimalPoint,
		ThousandsSep: num.ThousandsSep,
		Grouping:     num.Grouping,

		MonDecimalPoint: mon.MonDecimalPoint,
		MonThousandsSep: mon.MonThousandsSep,
		MonGrouping:     mon.MonGrouping,
		PositiveSign:    mon.PositiveSign,
		NegativeSign:    mon.NegativeSign,

		LocalFormat: mon.LocalFormat,
		IntlFormat:  mon.IntlFormat,
	}
	return result
}

// LangInfo returns the value for a single nl_langinfo item from the currently
// active locale. Unknown item codes return the empty string, matching the
// behavior of nl_langinfo() on libc with an out-of-range item.
func LangInfo(item phpv.ZInt) phpv.ZString {
	mu.Lock()
	defer mu.Unlock()

	// Day / abbreviated day names.
	if item >= DAY_1 && item <= DAY_7 {
		return phpv.ZString(active(LC_TIME).days[item-DAY_1])
	}
	if item >= ABDAY_1 && item <= ABDAY_7 {
		return phpv.ZString(active(LC_TIME).abdays[item-ABDAY_1])
	}
	// Month / abbreviated month names.
	if item >= MON_1 && item <= MON_12 {
		return phpv.ZString(active(LC_TIME).months[item-MON_1])
	}
	if item >= ABMON_1 && item <= ABMON_12 {
		return phpv.ZString(active(LC_TIME).abmonths[item-ABMON_1])
	}

	switch item {
	case D_T_FMT:
		return phpv.ZString(active(LC_TIME).dtFmt)
	case D_FMT:
		return phpv.ZString(active(LC_TIME).dFmt)
	case T_FMT:
		return phpv.ZString(active(LC_TIME).tFmt)
	case T_FMT_AMPM:
		return phpv.ZString(active(LC_TIME).tFmtAmpm)
	case AM_STR:
		return phpv.ZString(active(LC_TIME).amStr)
	case PM_STR:
		return phpv.ZString(active(LC_TIME).pmStr)

	case RADIXCHAR:
		return phpv.ZString(active(LC_NUMERIC).conv.DecimalPoint)
	case THOUSEP:
		return phpv.ZString(active(LC_NUMERIC).conv.ThousandsSep)

	case YESSTR:
		return phpv.ZString(active(LC_MESSAGES).yesStr)
	case YESEXPR:
		return phpv.ZString(active(LC_MESSAGES).yesExpr)
	case NOSTR:
		return phpv.ZString(active(LC_MESSAGES).noStr)
	case NOEXPR:
		return phpv.ZString(active(LC_MESSAGES).noExpr)

	case CRNCYSTR:
		return phpv.ZString(active(LC_MONETARY).crncystr)

	case CODESET:
		// Derive codeset from the active CTYPE locale's name: if it ends
		// in .utf8 or .utf-8 we return UTF-8, otherwise the C fallback.
		n := strings.ToLower(active(LC_CTYPE).name)
		if strings.HasSuffix(n, ".utf-8") || strings.HasSuffix(n, ".utf8") {
			return "UTF-8"
		}
		return phpv.ZString(active(LC_CTYPE).codeset)
	}
	return ""
}
