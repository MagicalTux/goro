package date

import (
	"fmt"
	"math"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/KarpelesLab/gotz"
	"github.com/KarpelesLab/strtotime"
	"github.com/KarpelesLab/goro/core"
	"github.com/KarpelesLab/goro/core/logopt"
	"github.com/KarpelesLab/goro/core/phperr"
	"github.com/KarpelesLab/goro/core/phpobj"
	"github.com/KarpelesLab/goro/core/phpv"
)

// Schlyter (J2000-based) solar calculation algorithm, matching PHP's implementation.

const j2000UnixSeconds = int64(946728000) // 2000-01-01 12:00:00 UTC

func tsToJ2000(ts int64) float64 {
	return float64(ts-j2000UnixSeconds) / 86400.0
}

func astroRevolution(x float64) float64 {
	return x - 360.0*math.Floor(x/360.0)
}

func astroRev180(x float64) float64 {
	x = astroRevolution(x)
	if x >= 180 {
		x -= 360
	}
	return x
}

func sind(deg float64) float64  { return math.Sin(deg * math.Pi / 180.0) }
func cosd(deg float64) float64  { return math.Cos(deg * math.Pi / 180.0) }
func tand(deg float64) float64  { return math.Tan(deg * math.Pi / 180.0) }
func acosd(x float64) float64   { return math.Acos(x) * 180.0 / math.Pi }
func asind(x float64) float64   { return math.Asin(x) * 180.0 / math.Pi }
func atan2d(y, x float64) float64 { return math.Atan2(y, x) * 180.0 / math.Pi }

// astroSunpos computes the Sun's ecliptic longitude and distance
// for a given number of days since J2000.
func astroSunpos(d float64) (lon, r float64) {
	M := astroRevolution(356.0470 + 0.9856002585*d)
	w := 282.9404 + 4.70935e-5*d
	e := 0.016709 - 1.151e-9*d
	E := M + (180.0/math.Pi)*e*sind(M)*(1.0+e*cosd(M))
	x := cosd(E) - e
	y := math.Sqrt(1.0-e*e) * sind(E)
	r = math.Sqrt(x*x + y*y)
	v := atan2d(y, x)
	lon = astroRevolution(v + w)
	return
}

// astroSunRADec computes the Sun's right ascension, declination, and distance.
func astroSunRADec(d float64) (RA, dec, r float64) {
	slon, sr := astroSunpos(d)
	oblEcl := 23.4393 - 3.563e-7*d
	x := sr * cosd(slon)
	y := sr * cosd(oblEcl) * sind(slon)
	z := sr * sind(oblEcl) * sind(slon)
	RA = atan2d(y, x)
	dec = atan2d(z, math.Sqrt(x*x+y*y))
	r = sr
	return
}

// astroGMST0 returns the Greenwich Mean Sidereal Time at 0h UT.
func astroGMST0(d float64) float64 {
	return astroRevolution((180.0 + 356.0470 + 282.9404) + (0.9856002585+4.70935e-5)*d)
}

// sunRC encodes the return code from calculateSunRiseSetTransit:
//
//	0 = normal (sun rises and sets)
//	-1 = sun never rises (below horizon all day)
//	+1 = sun never sets (midnight sun)
type sunRC int

const (
	sunNormal     sunRC = 0
	sunNeverRises sunRC = -1
	sunNeverSets  sunRC = 1
)

// calculateSunRiseSetTransit computes sunrise, sunset, and transit times
// as hours from UTC midnight. altit is the altitude in degrees (-0.8333 for standard rise/set).
// upperLimb: if true, adjusts for the sun's apparent radius.
func calculateSunRiseSetTransit(utcMidnightTS int64, lat, lon, altit float64, upperLimb bool) (rise, set, transit float64, rc sunRC) {
	d := tsToJ2000(utcMidnightTS) + 2.0 - lon/360.0

	sidtime := astroRevolution(astroGMST0(d) + 180.0 + lon)
	sRA, sdec, sr := astroSunRADec(d)
	tsouth := 12.0 - astroRev180(sidtime-sRA)/15.0

	sradius := 0.2666 / sr
	if upperLimb {
		altit -= sradius
	}

	cost := (sind(altit) - sind(lat)*sind(sdec)) / (cosd(lat) * cosd(sdec))
	transit = tsouth

	if cost >= 1.0 {
		// Sun never rises
		rise = math.NaN()
		set = math.NaN()
		rc = sunNeverRises
		return
	}
	if cost <= -1.0 {
		// Sun never sets (midnight sun)
		rise = math.NaN()
		set = math.NaN()
		rc = sunNeverSets
		return
	}

	t := acosd(cost) / 15.0
	rise = tsouth - t
	set = tsouth + t
	rc = sunNormal
	return
}

const solarZenithSunrise = 90.833333

// calculateSunTime provides backward compatibility with callers that use the old zenith-based API.
// It wraps the Schlyter algorithm. The loc parameter determines the local timezone used to
// resolve the calendar date from the timestamp (PHP uses the default timezone's date, not UTC).
func calculateSunTime(timestamp int64, latitude, longitude, zenith float64, isSunrise bool, loc *time.Location) float64 {
	// Use the local timezone's calendar date (PHP behavior), then create UTC midnight for that date.
	t := time.Unix(timestamp, 0).In(loc)
	dayStart := time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC)
	altit := -(zenith - 90.0)
	// Use upper limb correction for standard sunrise/sunset zenith values
	upperLimb := zenith <= 91 && zenith >= 90
	rise, set, _, _ := calculateSunRiseSetTransit(dayStart.Unix(), latitude, longitude, altit, upperLimb)
	if isSunrise {
		return rise
	}
	return set
}

// > func bool date_default_timezone_set ( string $timezoneId )
func fncDateDefaultTimezoneSet(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var tz phpv.ZString
	_, err := core.Expand(ctx, args, &tz)
	if err != nil {
		return nil, err
	}

	// Validate the timezone identifier
	_, loadErr := time.LoadLocation(string(tz))
	if loadErr != nil {
		ctx.Notice("Timezone ID '%s' is invalid", string(tz))
		return phpv.ZBool(false).ZVal(), nil
	}

	ctx.Global().SetLocalConfig("date.timezone", tz.ZVal())
	return phpv.ZBool(true).ZVal(), nil
}

// > func string date_default_timezone_get ( void )
func fncDateDefaultTimezoneGet(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	tz := ctx.GetConfig("date.timezone", phpv.ZString("UTC").ZVal())
	// If date.timezone is empty, fall back to UTC
	if s := tz.String(); s == "" {
		return phpv.ZString("UTC").ZVal(), nil
	}
	return tz.As(ctx, phpv.ZtString)
}

// > func DateInterval date_diff ( DateTimeInterface $datetime1 , DateTimeInterface $datetime2 [, bool $absolute = false ] )
func fncDateDiff(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	if len(args) < 2 {
		return nil, phpobj.ThrowError(ctx, phpobj.ArgumentCountError, fmt.Sprintf("date_diff() expects at least 2 parameters, %d given", len(args)))
	}

	obj1, ok := args[0].Value().(phpv.ZObject)
	if !ok {
		return nil, phpobj.ThrowError(ctx, phpobj.TypeError,
			fmt.Sprintf("date_diff(): Argument #1 ($baseObject) must be of type DateTimeInterface, %s given", args[0].GetType()))
	}

	zobj1, ok := obj1.(*phpobj.ZObject)
	if !ok {
		return phpv.ZBool(false).ZVal(), nil
	}

	return diffMethod(ctx, zobj1, args[1:])
}

// getTimezoneLoc retrieves the *time.Location stored in a DateTimeZone object.
func getTimezoneLoc(obj *phpobj.ZObject) (*time.Location, bool) {
	if v, ok := obj.Opaque[DateTimeZone]; ok {
		return v.(*time.Location), true
	}
	return nil, false
}

func getTimezoneName(obj *phpobj.ZObject) string {
	loc, ok := getTimezoneLoc(obj)
	if !ok {
		return "UTC"
	}
	return loc.String()
}

// checkDateTimeZoneInitialized throws DateObjectError if the DateTimeZone object is not initialized.
func checkDateTimeZoneInitialized(ctx phpv.Context, obj *phpobj.ZObject) error {
	if _, ok := obj.Opaque[DateTimeZone]; ok {
		return nil
	}
	className := obj.Class.GetName()
	baseClass := "DateTimeZone"
	if string(className) == baseClass {
		return phpobj.ThrowError(ctx, DateObjectError,
			fmt.Sprintf("Object of type %s has not been correctly initialized by calling parent::__construct() in its constructor", className))
	}
	return phpobj.ThrowError(ctx, DateObjectError,
		fmt.Sprintf("Object of type %s (inheriting %s) has not been correctly initialized by calling parent::__construct() in its constructor", className, baseClass))
}

// setTimezoneLoc stores a *time.Location in a DateTimeZone object.
func setTimezoneLoc(obj *phpobj.ZObject, loc *time.Location) {
	obj.Opaque[DateTimeZone] = loc
}

// setTimezoneProps sets timezone_type and timezone in the object's hash table so
// var_export / __set_state roundtrip works correctly.
func setTimezoneProps(obj *phpobj.ZObject, loc *time.Location) {
	name := loc.String()
	tzType := 3
	if len(name) > 0 && (name[0] == '+' || name[0] == '-') {
		tzType = 1
	} else if len(name) <= 5 && name != "Local" && name != "UTC" && !strings.Contains(name, "/") {
		tzType = 2
	}
	obj.HashTable().SetString("timezone_type", phpv.ZInt(tzType).ZVal())
	obj.HashTable().SetString("timezone", phpv.ZString(name).ZVal())
}

// timezoneAbbreviationOffsets maps timezone abbreviations to their UTC offsets in seconds.
// These are treated as type 2 (abbreviation) timezones in PHP.
var timezoneAbbreviationOffsets = map[string]int{
	"ACDT":  37800,  // +10:30
	"ACST":  34200,  // +09:30
	"ADT":   -10800, // -03:00
	"AEDT":  39600,  // +11:00
	"AEST":  36000,  // +10:00
	"AKDT":  -28800, // -08:00
	"AKST":  -32400, // -09:00
	"AST":   -14400, // -04:00
	"AWST":  28800,  // +08:00
	"BST":   3600,   // +01:00
	"CAT":   7200,   // +02:00
	"CDT":   -18000, // -05:00
	"CEST":  7200,   // +02:00
	"CET":   3600,   // +01:00
	"CST":   -21600, // -06:00
	"EAT":   10800,  // +03:00
	"EDT":   -14400, // -04:00
	"EEST":  10800,  // +03:00
	"EET":   7200,   // +02:00
	"EST":   -18000, // -05:00
	"GMT":   0,
	"HDT":   -32400, // -09:00
	"HKT":   28800,  // +08:00
	"HST":   -36000, // -10:00
	"ICT":   25200,  // +07:00
	"IDT":   10800,  // +03:00
	"IST":   19800,  // +05:30
	"JST":   32400,  // +09:00
	"KST":   32400,  // +09:00
	"MDT":   -21600, // -06:00
	"MET":   3600,   // +01:00
	"MEST":  7200,   // +02:00
	"MSK":   10800,  // +03:00
	"MST":   -25200, // -07:00
	"NDT":   -9000,  // -02:30
	"NST":   -12600, // -03:30
	"NZDT":  46800,  // +13:00
	"NZST":  43200,  // +12:00
	"PDT":   -25200, // -07:00
	"PKT":   18000,  // +05:00
	"PST":   -28800, // -08:00
	"SGT":   28800,  // +08:00
	"UTC":   0,
	"WAT":   3600,   // +01:00
	"WAST":  7200,   // +02:00
	"WET":   0,
	"WEST":  3600,   // +01:00
	"WIB":   25200,  // +07:00
	"WIT":   32400,  // +09:00
	"WITA":  28800,  // +08:00
}

// parseTzName handles offset strings like "+05:30" or "-02:00" in addition to named zones
func parseTzName(tzName string) (*time.Location, error) {
	if len(tzName) >= 2 && (tzName[0] == '+' || tzName[0] == '-') {
		if offset, ok := parseTZOffset(tzName); ok {
			return time.FixedZone(tzName, offset), nil
		}
	}
	// Handle "GMT+HHMM" or "GMT-HHMM" or "UTC+HH" style
	upper := strings.ToUpper(tzName)
	if strings.HasPrefix(upper, "GMT") || strings.HasPrefix(upper, "UTC") {
		rest := tzName[3:]
		if len(rest) > 0 && (rest[0] == '+' || rest[0] == '-') {
			if offset, ok := parseTZOffset(rest); ok {
				return makeFixedZone(offset), nil
			}
		}
	}
	// Try timezone abbreviations (PHP type 2) BEFORE time.LoadLocation.
	// Go's time.LoadLocation("CET") returns a full IANA timezone with DST
	// transitions, but PHP treats "CET" as a fixed-offset abbreviation (3600s).
	// "UTC" is special: PHP treats it as type 3 (identifier), not type 2.
	if upper != "UTC" {
		if offset, ok := timezoneAbbreviationOffsets[upper]; ok {
			return time.FixedZone(tzName, offset), nil
		}
		// Try military timezone abbreviations (single letter, case-insensitive)
		if offset, ok := militaryTZAbbrevs[strings.ToLower(tzName)]; ok {
			return time.FixedZone(tzName, offset), nil
		}
	}
	if loc, err := time.LoadLocation(tzName); err == nil {
		return loc, nil
	}
	return nil, fmt.Errorf("unknown timezone: %s", tzName)
}

// datetimezoneConstruct implements DateTimeZone::__construct(string $timezone)
func datetimezoneConstruct(ctx phpv.Context, this *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	if len(args) < 1 {
		// Allow 0 args for internal use (e.g., __set_state creates the object without calling the constructor).
		// In PHP, DateTimeZone::__construct() actually requires 1 arg and would throw, but since our
		// NewZObject() always calls the constructor, we allow 0 args here and leave the object uninitialized
		// (no opaque data set). The object will throw on any method call via checkDateTimeZoneInitialized.
		return nil, nil
	}
	if len(args) > 1 {
		return nil, phpobj.ThrowError(ctx, phpobj.ArgumentCountError, fmt.Sprintf("DateTimeZone::__construct() expects exactly 1 argument, %d given", len(args)))
	}
	tzName := string(args[0].AsString(ctx))

	// Check for null bytes
	if strings.ContainsRune(tzName, 0) {
		return nil, phpobj.ThrowError(ctx, phpobj.ValueError, "DateTimeZone::__construct(): Argument #1 ($timezone) must not contain any null bytes")
	}

	// Validate timezone offset ranges - minutes must be < 60
	if len(tzName) >= 2 && (tzName[0] == '+' || tzName[0] == '-') {
		cleaned := tzName[1:]
		colonIdx := strings.IndexByte(cleaned, ':')
		var minsStr string
		if colonIdx >= 0 {
			minsStr = cleaned[colonIdx+1:]
		} else if len(cleaned) >= 3 {
			// Format like +HHMM or +HHHMM
			if len(cleaned) == 4 {
				minsStr = cleaned[2:]
			} else if len(cleaned) == 5 {
				minsStr = cleaned[3:]
			}
		}
		if minsStr != "" {
			mins, err := strconv.Atoi(minsStr)
			if err == nil && mins >= 60 {
				return nil, phpobj.ThrowError(ctx, DateInvalidTimeZoneException, "DateTimeZone::__construct(): Timezone offset is out of range ("+tzName+")")
			}
		}
	}

	loc, err := parseTzName(tzName)
	if err != nil {
		return nil, phpobj.ThrowError(ctx, DateInvalidTimeZoneException, "DateTimeZone::__construct(): Unknown or bad timezone ("+tzName+")")
	}

	// Normalize offset timezone names to +HH:MM or +HH:MM:SS format
	if len(tzName) >= 2 && (tzName[0] == '+' || tzName[0] == '-') {
		offset, ok := parseTZOffset(tzName)
		if ok {
			sign := "+"
			absOffset := offset
			if offset < 0 {
				sign = "-"
				absOffset = -offset
			}
			hours := absOffset / 3600
			mins := (absOffset % 3600) / 60
			secs := absOffset % 60
			var formattedName string
			if secs != 0 {
				formattedName = fmt.Sprintf("%s%02d:%02d:%02d", sign, hours, mins, secs)
			} else {
				formattedName = fmt.Sprintf("%s%02d:%02d", sign, hours, mins)
			}
			loc = time.FixedZone(formattedName, offset)
		}
	}

	setTimezoneLoc(this, loc)
	// Store timezone_type and timezone using the ORIGINAL user-supplied name
	// (not loc.String() which may normalize names like "Universal" -> "UTC")
	displayName := loc.String()
	tzType := 3
	if len(displayName) > 0 && (displayName[0] == '+' || displayName[0] == '-') {
		tzType = 1
	}
	// Determine if the name is an abbreviation (type 2) or an IANA identifier (type 3).
	// PHP treats short timezone abbreviation codes (EST, GMT, BST, etc.) as type 2,
	// while IANA database names (UTC, Cuba, America/New_York) are type 3.
	// "UTC" is special - it's always type 3 in PHP.
	if tzType != 1 {
		upperTz := strings.ToUpper(tzName)
		_, isAbbrev := timezoneAbbreviationOffsets[upperTz]
		if !isAbbrev {
			// Also check military timezone abbreviations
			_, isAbbrev = militaryTZAbbrevs[strings.ToLower(tzName)]
		}
		if isAbbrev && upperTz != "UTC" {
			tzType = 2
			// PHP stores abbreviations in uppercase
			displayName = upperTz
		} else {
			// Use the original user-supplied name for display (preserving casing and name)
			displayName = tzName
		}
	}
	this.HashTable().SetString("timezone_type", phpv.ZInt(tzType).ZVal())
	this.HashTable().SetString("timezone", phpv.ZString(displayName).ZVal())
	return nil, nil
}

// datetimezoneGetName implements DateTimeZone::getName(): string
func datetimezoneGetName(ctx phpv.Context, this *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	if err := checkDateTimeZoneInitialized(ctx, this); err != nil {
		return nil, err
	}
	loc, _ := getTimezoneLoc(this)
	return phpv.ZString(loc.String()).ZVal(), nil
}

// datetimezoneGetOffset implements DateTimeZone::getOffset(DateTimeInterface $datetime): int
func datetimezoneGetOffset(ctx phpv.Context, this *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	if len(args) < 1 {
		return nil, phpobj.ThrowError(ctx, phpobj.TypeError, "DateTimeZone::getOffset() expects exactly 1 argument, 0 given")
	}
	if err := checkDateTimeZoneInitialized(ctx, this); err != nil {
		return nil, err
	}
	loc, _ := getTimezoneLoc(this)
	dtObj, ok := args[0].Value().(*phpobj.ZObject)
	if !ok {
		return nil, phpobj.ThrowError(ctx, phpobj.TypeError, fmt.Sprintf("DateTimeZone::getOffset(): Argument #1 ($datetime) must be of type DateTimeInterface, %s given", args[0].GetType().TypeName()))
	}
	t, ok := getTime(dtObj)
	if !ok {
		if err := checkDateTimeInitialized(ctx, dtObj); err != nil {
			return nil, err
		}
		return phpv.ZBool(false).ZVal(), nil
	}
	// Get offset at the given datetime in this timezone
	_, offset := t.In(loc).Zone()
	return phpv.ZInt(offset).ZVal(), nil
}

// DateTimeZone region constants (mirrors the PHP class constants).
const (
	dtzAFRICA     = 1
	dtzAMERICA    = 2
	dtzANTARCTICA = 4
	dtzARCTIC     = 8
	dtzASIA       = 16
	dtzATLANTIC   = 32
	dtzAUSTRALIA  = 64
	dtzEUROPE     = 128
	dtzINDIAN     = 256
	dtzPACIFIC    = 512
	dtzUTC        = 1024
	dtzALL        = 2047
	dtzALL_WITH_BC = 4095
	dtzPER_COUNTRY = 4096
)

// tzPrefixForRegion maps a single-bit region constant to its IANA prefix string.
// A non-empty prefix means the zone name must start with that prefix.
// An empty prefix with a special marker is used for "UTC" (exact match).
var tzPrefixForRegion = []struct {
	bit    int
	prefix string
}{
	{dtzAFRICA, "Africa/"},
	{dtzAMERICA, "America/"},
	{dtzANTARCTICA, "Antarctica/"},
	{dtzARCTIC, "Arctic/"},
	{dtzASIA, "Asia/"},
	{dtzATLANTIC, "Atlantic/"},
	{dtzAUSTRALIA, "Australia/"},
	{dtzEUROPE, "Europe/"},
	{dtzINDIAN, "Indian/"},
	{dtzPACIFIC, "Pacific/"},
	{dtzUTC, "UTC"},
}

// zoneMatchesRegion reports whether a timezone name belongs to the given region bitmask.
func zoneMatchesRegion(name string, regionMask int) bool {
	// UTC exact match
	if name == "UTC" && (regionMask&dtzUTC) != 0 {
		return true
	}
	for _, r := range tzPrefixForRegion {
		if r.prefix == "UTC" {
			continue // handled above
		}
		if (regionMask&r.bit) != 0 && strings.HasPrefix(name, r.prefix) {
			return true
		}
	}
	return false
}

// datetimezoneListIdentifiers implements DateTimeZone::listIdentifiers(int $timezoneGroup, string $countryCode): array
// $timezoneGroup defaults to DateTimeZone::ALL (2047).
// $countryCode is only used when $timezoneGroup == DateTimeZone::PER_COUNTRY.
func datetimezoneListIdentifiers(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	// Parse optional arguments
	regionMask := dtzALL
	countryCode := ""
	if len(args) >= 1 && !args[0].IsNull() {
		regionMask = int(args[0].AsInt(ctx))
	}
	if len(args) >= 2 && !args[1].IsNull() {
		countryCode = strings.ToUpper(string(args[1].AsString(ctx)))
	}

	result := phpv.NewZArray()

	// When PER_COUNTRY is specified, require a valid 2-letter country code
	if regionMask == dtzPER_COUNTRY && (countryCode == "" || len(countryCode) != 2) {
		return nil, phpobj.ThrowError(ctx, phpobj.ValueError, "timezone_identifiers_list(): Argument #2 ($countryCode) must be a two-letter ISO 3166-1 compatible country code when argument #1 ($timezoneGroup) is DateTimeZone::PER_COUNTRY")
	}

	if regionMask == dtzPER_COUNTRY && countryCode != "" {
		// Filter by country code using gotz metadata
		for _, name := range allIANAZones {
			zone, err := gotz.Load(name)
			if err != nil {
				continue
			}
			meta := zone.Meta()
			if meta == nil {
				continue
			}
			for _, c := range meta.Countries {
				if strings.ToUpper(c.Code) == countryCode {
					result.OffsetSet(ctx, nil, phpv.ZString(name).ZVal())
					break
				}
			}
		}
		return result.ZVal(), nil
	}

	for _, name := range allIANAZones {
		switch {
		case regionMask == dtzALL_WITH_BC:
			// ALL_WITH_BC includes everything in allIANAZones
			result.OffsetSet(ctx, nil, phpv.ZString(name).ZVal())
		case regionMask == dtzALL:
			// ALL: only canonical prefixes (Africa/, America/, ..., Pacific/) + UTC
			if zoneMatchesRegion(name, dtzALL) {
				result.OffsetSet(ctx, nil, phpv.ZString(name).ZVal())
			}
		default:
			// Specific region bitmask
			if zoneMatchesRegion(name, regionMask) {
				result.OffsetSet(ctx, nil, phpv.ZString(name).ZVal())
			}
		}
	}

	return result.ZVal(), nil
}

// tzAbbrevEntry is a single entry in the timezone abbreviation map.
type tzAbbrevEntry struct {
	dst      bool
	offset   int
	tzID     string
}

// tzAbbrevMap is a lazy-initialized cache of all timezone abbreviations.
var (
	tzAbbrevCache     map[string][]tzAbbrevEntry
	tzAbbrevCacheOnce sync.Once
)

// militaryTZAbbrevs maps single-letter NATO/military timezone abbreviations to their
// UTC offsets in seconds. These are included in PHP's timezone_abbreviations_list().
// J (Juliet) represents local time with offset 0.
var militaryTZAbbrevs = map[string]int{
	"a": 3600,    // Alpha = UTC+1
	"b": 7200,    // Bravo = UTC+2
	"c": 10800,   // Charlie = UTC+3
	"d": 14400,   // Delta = UTC+4
	"e": 18000,   // Echo = UTC+5
	"f": 21600,   // Foxtrot = UTC+6
	"g": 25200,   // Golf = UTC+7
	"h": 28800,   // Hotel = UTC+8
	"i": 32400,   // India = UTC+9
	// Note: "j" (Juliet) is NOT in PHP's timezonemap.h - it represents local time
	"k": 36000,   // Kilo = UTC+10
	"l": 39600,   // Lima = UTC+11
	"m": 43200,   // Mike = UTC+12
	"n": -3600,   // November = UTC-1
	"o": -7200,   // Oscar = UTC-2
	"p": -10800,  // Papa = UTC-3
	"q": -14400,  // Quebec = UTC-4
	"r": -18000,  // Romeo = UTC-5
	"s": -21600,  // Sierra = UTC-6
	"t": -25200,  // Tango = UTC-7
	"u": -28800,  // Uniform = UTC-8
	"v": -32400,  // Victor = UTC-9
	"w": -36000,  // Whiskey = UTC-10
	"x": -39600,  // X-ray = UTC-11
	"y": -43200,  // Yankee = UTC-12
	"z": 0,       // Zulu = UTC+0
}

// canonicalTZForAbbrev maps common timezone abbreviations to their canonical/primary
// IANA timezone identifier. This matches PHP's static timezonemap.h ordering where
// specific zones are listed first for each abbreviation.
// Without this, alphabetical zone ordering puts Africa/* before Europe/* for CET,
// America/Cancun before America/New_York for EDT, etc.
var canonicalTZForAbbrev = map[string]string{
	"acdt":  "Australia/Adelaide",
	"acst":  "Australia/Adelaide",
	"adt":   "America/Halifax",
	"aedt":  "Australia/Sydney",
	"aest":  "Australia/Sydney",
	"akdt":  "America/Anchorage",
	"akst":  "America/Anchorage",
	"ast":   "America/Halifax",
	"awst":  "Australia/Perth",
	"bst":   "Europe/London",
	"cat":   "Africa/Harare",
	"cdt":   "America/Chicago",
	"cest":  "Europe/Paris",
	"cet":   "Europe/Berlin",
	"cst":   "America/Chicago",
	"eat":   "Africa/Nairobi",
	"edt":   "America/New_York",
	"eest":  "Europe/Helsinki",
	"eet":   "Europe/Helsinki",
	"est":   "America/New_York",
	"gmt":   "UTC",
	"hdt":   "America/Adak",
	"hkt":   "Asia/Hong_Kong",
	"hst":   "Pacific/Honolulu",
	"ict":   "Asia/Bangkok",
	"idt":   "Asia/Jerusalem",
	"ist":   "Asia/Kolkata",
	"jst":   "Asia/Tokyo",
	"kst":   "Asia/Seoul",
	"mdt":   "America/Denver",
	"met":   "Europe/Paris",
	"mest":  "Europe/Paris",
	"msk":   "Europe/Moscow",
	"mst":   "America/Denver",
	"ndt":   "America/St_Johns",
	"nst":   "America/St_Johns",
	"nzdt":  "Pacific/Auckland",
	"nzst":  "Pacific/Auckland",
	"pdt":   "America/Los_Angeles",
	"pst":   "America/Los_Angeles",
	"sgt":   "Asia/Singapore",
	"utc":   "Etc/Universal",
	"wat":   "Africa/Lagos",
	"west":  "Europe/Lisbon",
	"wet":   "Europe/Lisbon",
}

// tzAbbrevFuture is used as the "to" time when scanning transitions for abbreviation lookups.
var tzAbbrevFuture = time.Date(2100, 1, 1, 0, 0, 0, 0, time.UTC)

// excludedAbbrevs lists timezone abbreviations that exist in IANA data but are
// NOT included in PHP's static timezonemap.h. PHP's map was built from a specific
// IANA snapshot and excludes certain very obscure historical abbreviations.
var excludedAbbrevs = map[string]bool{
	"zmt":  true, // Zomba Mean Time - historical Africa/Blantyre, not in PHP
	"admt": true, // not in PHP's timezonemap.h
	"east": true, // not in PHP's timezonemap.h
	"hkwt": true, // not in PHP's timezonemap.h
	"pmmt": true, // not in PHP's timezonemap.h
	"set":  true, // not in PHP's timezonemap.h
}

// timezoneFallbackMap mirrors PHP's timelib_timezone_fallbackmap from fallbackmap.h.
// It is used by timezone_name_from_abbr() when searching by offset/isDST without
// an abbreviation. The order matches PHP's static data exactly.
var timezoneFallbackMap = []tzAbbrevEntry{
	{dst: false, offset: -39600, tzID: "Pacific/Apia"},
	{dst: false, offset: -36000, tzID: "Pacific/Honolulu"},
	{dst: false, offset: -32400, tzID: "America/Anchorage"},
	{dst: true, offset: -28800, tzID: "America/Anchorage"},
	{dst: false, offset: -28800, tzID: "America/Los_Angeles"},
	{dst: true, offset: -25200, tzID: "America/Los_Angeles"},
	{dst: false, offset: -25200, tzID: "America/Denver"},
	{dst: true, offset: -21600, tzID: "America/Denver"},
	{dst: false, offset: -21600, tzID: "America/Chicago"},
	{dst: true, offset: -18000, tzID: "America/Chicago"},
	{dst: false, offset: -18000, tzID: "America/New_York"},
	{dst: false, offset: -16200, tzID: "America/Caracas"},
	{dst: true, offset: -14400, tzID: "America/New_York"},
	{dst: false, offset: -14400, tzID: "America/Halifax"},
	{dst: true, offset: -10800, tzID: "America/Halifax"},
	{dst: false, offset: -10800, tzID: "America/Sao_Paulo"},
	{dst: true, offset: -7200, tzID: "America/Sao_Paulo"},
	{dst: false, offset: -3600, tzID: "Atlantic/Azores"},
	{dst: true, offset: 0, tzID: "Atlantic/Azores"},
	{dst: false, offset: 0, tzID: "Europe/London"},
	{dst: true, offset: 3600, tzID: "Europe/London"},
	{dst: false, offset: 3600, tzID: "Europe/Paris"},
	{dst: true, offset: 7200, tzID: "Europe/Paris"},
	{dst: false, offset: 7200, tzID: "Europe/Helsinki"},
	{dst: true, offset: 10800, tzID: "Europe/Helsinki"},
	{dst: false, offset: 10800, tzID: "Europe/Moscow"},
	{dst: true, offset: 14400, tzID: "Europe/Moscow"},
	{dst: false, offset: 14400, tzID: "Asia/Dubai"},
	{dst: false, offset: 18000, tzID: "Asia/Karachi"},
	{dst: false, offset: 19800, tzID: "Asia/Kolkata"},
	{dst: false, offset: 20700, tzID: "Asia/Katmandu"},
	{dst: true, offset: 21600, tzID: "Asia/Yekaterinburg"},
	{dst: true, offset: 25200, tzID: "Asia/Novosibirsk"},
	{dst: false, offset: 25200, tzID: "Asia/Krasnoyarsk"},
	{dst: false, offset: 28800, tzID: "Asia/Shanghai"},
	{dst: true, offset: 28800, tzID: "Asia/Krasnoyarsk"},
	{dst: false, offset: 32400, tzID: "Asia/Tokyo"},
	{dst: false, offset: 36000, tzID: "Australia/Melbourne"},
	{dst: true, offset: 37800, tzID: "Australia/Adelaide"},
	{dst: true, offset: 39600, tzID: "Australia/Melbourne"},
	{dst: false, offset: 43200, tzID: "Pacific/Auckland"},
	{dst: true, offset: 46800, tzID: "Pacific/Auckland"},
}

// buildTZAbbrevMap builds the complete PHP timezone abbreviation map from all IANA zones.
// It includes all zone types (including historical ones) to match PHP's behavior,
// which uses a pre-compiled static map from IANA data.
// It also adds military timezone abbreviations (a-z) to match PHP's 144-entry count.
// The result is a map from lowercase abbreviation to []tzAbbrevEntry, de-duplicated
// across all zones by (abbr, offset, tzID).
func buildTZAbbrevMap() map[string][]tzAbbrevEntry {
	type globalKey struct {
		abbr   string
		offset int
		tzID   string
	}
	// globalSeen prevents duplicate (abbr, offset, tzID) entries across all zones.
	globalSeen := make(map[globalKey]bool)
	result := make(map[string][]tzAbbrevEntry)

	// Process all IANA zones plus UTC
	allZones := make([]string, len(allIANAZones)+1)
	copy(allZones, allIANAZones)
	allZones[len(allIANAZones)] = "UTC"

	for _, name := range allZones {
		// Skip Etc/UCT - PHP doesn't include it in timezone_abbreviations_list
		if name == "Etc/UCT" {
			continue
		}
		zone, err := gotz.Load(name)
		if err != nil {
			continue
		}
		types := zone.Types()

		// For each type, keep only the LAST occurrence of each (abbr, isDST) pair
		// to get the most-recent offset for that abbreviation in this zone.
		type localKey struct {
			abbr  string
			isDST bool
		}
		lastType := make(map[localKey]gotz.ZoneType)
		for _, zt := range types {
			abbr := strings.ToLower(zt.Abbrev)
			if abbr == "" || abbr == "zzz" || abbr == "lmt" {
				continue
			}
			// Skip numeric offset-style abbreviations like "+0020", "-0530", etc.
			if len(abbr) > 0 && (abbr[0] == '+' || abbr[0] == '-') {
				continue
			}
			// Skip abbreviations not in PHP's static map
			if excludedAbbrevs[abbr] {
				continue
			}
			// Each later type overwrites earlier ones (we want the most recent).
			lastType[localKey{abbr, zt.IsDST}] = zt
		}

		// Add each unique (abbr, isDST) type to the global result.
		for _, zt := range lastType {
			abbr := strings.ToLower(zt.Abbrev)
			gk := globalKey{abbr: abbr, offset: zt.Offset, tzID: name}
			if globalSeen[gk] {
				continue
			}
			globalSeen[gk] = true
			result[abbr] = append(result[abbr], tzAbbrevEntry{
				dst:    zt.IsDST,
				offset: zt.Offset,
				tzID:   name,
			})
		}
	}

	// Add military timezone abbreviations (a-z). PHP includes these in its
	// timezone_abbreviations_list() output with empty timezone_id values.
	for abbr, offset := range militaryTZAbbrevs {
		if _, exists := result[abbr]; !exists {
			result[abbr] = []tzAbbrevEntry{{dst: false, offset: offset, tzID: ""}}
		}
	}

	// phpCompatAbbrevs adds PHP-static-map entries for abbreviations that no longer
	// appear in modern IANA data (because countries like Brazil replaced named DST
	// abbreviations with numeric offsets like "-02"). PHP's timezonemap.h was built
	// from an older IANA snapshot and still maps these abbreviations.
	// Only add if not already present (real IANA data takes precedence).
	type phpCompatEntry struct {
		abbr   string
		dst    bool
		offset int
		tzID   string
	}
	phpCompatAbbrevs := []phpCompatEntry{
		// Canadian Double Daylight Time: historical, no longer in modern IANA data
		{"cddt", true, -14400, "America/Rankin_Inlet"},
		{"cddt", true, -14400, "America/Resolute"},
		// Eastern Double Daylight Time
		{"eddt", true, -10800, "America/Iqaluit"},
		// Mountain Double Daylight Time
		{"mddt", true, -18000, "America/Cambridge_Bay"},
		{"mddt", true, -18000, "America/Yellowknife"},
		// Middle European Summer Time
		{"mest", true, 7200, "MET"},
		// Middle European Time
		{"met", false, 3600, "MET"},
		// Pacific Double Daylight Time
		{"pddt", true, -21600, "America/Inuvik"},
		// UCT alias
		{"uct", false, 0, "Etc/UCT"},
	}

	// PHP's timezonemap.h has a duplicate "UTC" entry under "utc" abbreviation.
	// Add it explicitly since our dedup logic removes it.
	if entries, ok := result["utc"]; ok {
		// Check if UTC already appears and add a second one
		hasUTC := false
		for _, e := range entries {
			if e.tzID == "UTC" {
				hasUTC = true
				break
			}
		}
		if hasUTC {
			result["utc"] = append(result["utc"], tzAbbrevEntry{dst: false, offset: 0, tzID: "UTC"})
		}
	}
	for _, pc := range phpCompatAbbrevs {
		gk := globalKey{abbr: pc.abbr, offset: pc.offset, tzID: pc.tzID}
		if !globalSeen[gk] {
			globalSeen[gk] = true
			result[pc.abbr] = append(result[pc.abbr], tzAbbrevEntry{
				dst:    pc.dst,
				offset: pc.offset,
				tzID:   pc.tzID,
			})
		}
	}

	// Reorder entries so canonical/primary zones come first, matching PHP's
	// static timezonemap.h ordering. Without this, Africa/* zones precede
	// Europe/* and America/* alphabetically, giving wrong first entries for
	// abbreviations like CET (Africa/Algiers before Europe/Berlin).
	for abbr, canonical := range canonicalTZForAbbrev {
		entries := result[abbr]
		if len(entries) == 0 {
			continue
		}
		// Find the canonical entry and move it to the front.
		for i, e := range entries {
			if e.tzID == canonical {
				if i != 0 {
					entries[0], entries[i] = entries[i], entries[0]
					result[abbr] = entries
				}
				break
			}
		}
	}

	return result
}

// datetimezoneListAbbreviations implements DateTimeZone::listAbbreviations(): array
// Returns a map of lowercase abbreviation strings to arrays of [dst, offset, timezone_id] arrays.
func datetimezoneListAbbreviations(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	tzAbbrevCacheOnce.Do(func() {
		tzAbbrevCache = buildTZAbbrevMap()
	})

	result := phpv.NewZArray()
	// Collect and sort abbreviation keys for deterministic output
	keys := make([]string, 0, len(tzAbbrevCache))
	for k := range tzAbbrevCache {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	for _, abbr := range keys {
		entries := tzAbbrevCache[abbr]
		arr := phpv.NewZArray()
		for _, e := range entries {
			entry := phpv.NewZArray()
			entry.OffsetSet(ctx, phpv.ZString("dst"), phpv.ZBool(e.dst).ZVal())
			entry.OffsetSet(ctx, phpv.ZString("offset"), phpv.ZInt(e.offset).ZVal())
			entry.OffsetSet(ctx, phpv.ZString("timezone_id"), phpv.ZString(e.tzID).ZVal())
			arr.OffsetSet(ctx, nil, entry.ZVal())
		}
		result.OffsetSet(ctx, phpv.ZString(abbr), arr.ZVal())
	}

	return result.ZVal(), nil
}

// > func DateTimeZone timezone_open ( string $timezone )
func fncTimezoneOpen(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	if len(args) < 1 {
		return nil, phpobj.ThrowError(ctx, phpobj.ArgumentCountError, "timezone_open() expects exactly 1 argument, 0 given")
	}
	// Check for null bytes
	if strings.ContainsRune(string(args[0].AsString(ctx)), 0) {
		return nil, phpobj.ThrowError(ctx, phpobj.ValueError, "timezone_open(): Argument #1 ($timezone) must not contain any null bytes")
	}
	obj, err := phpobj.NewZObject(ctx, DateTimeZone, args[0])
	if err != nil {
		// timezone_open returns false on failure and emits a warning, unlike the constructor which throws
		tzName := args[0].AsString(ctx)
		ctx.Warn("Unknown or bad timezone (%s)", tzName)
		return phpv.ZBool(false).ZVal(), nil
	}
	return obj.ZVal(), nil
}

// > func string timezone_name_get ( DateTimeZone $object )
func fncTimezoneNameGet(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	if len(args) < 1 {
		return nil, phpobj.ThrowError(ctx, phpobj.ArgumentCountError, "timezone_name_get() expects exactly 1 argument, 0 given")
	}
	tzObj, ok := args[0].Value().(*phpobj.ZObject)
	if !ok {
		return phpv.ZBool(false).ZVal(), nil
	}
	if err := checkDateTimeZoneInitialized(ctx, tzObj); err != nil {
		return nil, err
	}
	loc, ok := getTimezoneLoc(tzObj)
	if !ok {
		return phpv.ZBool(false).ZVal(), nil
	}
	return phpv.ZString(loc.String()).ZVal(), nil
}

// > func array timezone_identifiers_list ()
func fncTimezoneIdentifiersList(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	return datetimezoneListIdentifiers(ctx, args)
}

// > func int timezone_offset_get ( DateTimeZone $object , DateTimeInterface $datetime )
func fncTimezoneOffsetGet(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	if len(args) < 2 {
		return nil, phpobj.ThrowError(ctx, phpobj.ArgumentCountError, "timezone_offset_get() expects exactly 2 argument")
	}
	tzObj, ok := args[0].Value().(*phpobj.ZObject)
	if !ok {
		return nil, phpobj.ThrowError(ctx, phpobj.TypeError, fmt.Sprintf("timezone_offset_get(): Argument #1 ($object) must be of type DateTimeZone, %s given", args[0].GetType().TypeName()))
	}
	// Verify it's actually a DateTimeZone
	if !tzObj.Class.InstanceOf(DateTimeZone) {
		className := tzObj.GetClass().GetName()
		return nil, phpobj.ThrowError(ctx, phpobj.TypeError, fmt.Sprintf("timezone_offset_get(): Argument #1 ($object) must be of type DateTimeZone, %s given", className))
	}
	// Validate the datetime argument: must be a DateTimeInterface object
	dtObj, dtOk := args[1].Value().(*phpobj.ZObject)
	if !dtOk {
		return nil, phpobj.ThrowError(ctx, phpobj.TypeError, fmt.Sprintf("timezone_offset_get(): Argument #2 ($datetime) must be of type DateTimeInterface, %s given", args[1].GetType().TypeName()))
	}
	if !dtObj.Class.InstanceOf(DateTimeInterface) {
		className := dtObj.GetClass().GetName()
		return nil, phpobj.ThrowError(ctx, phpobj.TypeError, fmt.Sprintf("timezone_offset_get(): Argument #2 ($datetime) must be of type DateTimeInterface, %s given", className))
	}
	if err := checkDateTimeInitialized(ctx, dtObj); err != nil {
		return nil, err
	}
	return datetimezoneGetOffset(ctx, tzObj, args[1:])
}

// > func array timezone_abbreviations_list ()
func fncTimezoneAbbreviationsList(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	return datetimezoneListAbbreviations(ctx, args)
}

// > func string|false timezone_name_from_abbr ( string $abbr [, int $utcOffset = -1 [, int $isDST = -1 ]] )
func fncTimezoneNameFromAbbr(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var abbr phpv.ZString
	var utcOffset core.Optional[phpv.ZInt]
	var isDST core.Optional[phpv.ZInt]
	_, err := core.Expand(ctx, args, &abbr, &utcOffset, &isDST)
	if err != nil {
		return nil, err
	}

	abbrStr := string(abbr)
	abbrUpper := strings.ToUpper(abbrStr)
	abbrLower := strings.ToLower(abbrStr)

	// Build the abbreviation map (cached)
	tzAbbrevCacheOnce.Do(func() {
		tzAbbrevCache = buildTZAbbrevMap()
	})

	// Step 1: If abbreviation is non-empty, look it up directly.
	// GMT and UTC are special-cased to always return "UTC" (PHP behavior).
	if abbrStr != "" {
		if abbrUpper == "GMT" || abbrUpper == "UTC" {
			return phpv.ZString("UTC").ZVal(), nil
		}
		if _, ok := tzAbbrevCache[abbrLower]; ok {
			// PHP returns the first entry from its static timezonemap.h.
			// Use the canonical mapping to ensure deterministic results.
			if canonical, hasCanonical := canonicalTZForAbbrev[abbrLower]; hasCanonical {
				return phpv.ZString(canonical).ZVal(), nil
			}
			// Fallback for abbreviations not in canonical map:
			// return the first entry with a non-empty timezone_id.
			for _, e := range tzAbbrevCache[abbrLower] {
				if e.tzID != "" {
					return phpv.ZString(e.tzID).ZVal(), nil
				}
			}
		}
	}

	// Step 2: Search by offset (and optionally DST). Used when abbr is empty
	// or not found in the abbreviation database.
	// PHP only does this search when isDST is explicitly provided (0 or 1).
	// When isDST is not provided (defaults to -1), offset-based search is skipped.
	offset := int64(-1)
	if utcOffset.HasArg() {
		offset = int64(utcOffset.Get())
	}
	wantDST := int64(-1)
	if isDST.HasArg() {
		wantDST = int64(isDST.Get())
	}

	if isDST.HasArg() && (offset != -1 || wantDST != -1) {
		// Search through the abbreviation database for matching offset
		// PHP iterates the abbreviation list and returns the first match
		// with a matching offset and (if isDST is specified) DST flag.

		entryMatches := func(e tzAbbrevEntry) bool {
			if e.tzID == "" {
				return false
			}
			if offset != -1 && e.offset != int(offset) {
				return false
			}
			if wantDST != -1 {
				eDST := 0
				if e.dst {
					eDST = 1
				}
				if int64(eDST) != wantDST {
					return false
				}
			}
			return true
		}

		// Search the PHP fallback map first (matching PHP's timelib_timezone_fallbackmap).
		// This is a curated list of well-known timezone mappings by offset+isDST
		// that PHP uses when no abbreviation match is found.
		for _, fb := range timezoneFallbackMap {
			if entryMatches(fb) {
				return phpv.ZString(fb.tzID).ZVal(), nil
			}
		}

		// If fallback map didn't match, search all abbreviation entries
		// in alphabetical order.
		keys := make([]string, 0, len(tzAbbrevCache))
		for k := range tzAbbrevCache {
			keys = append(keys, k)
		}
		sort.Strings(keys)

		for _, k := range keys {
			for _, e := range tzAbbrevCache[k] {
				if entryMatches(e) {
					return phpv.ZString(e.tzID).ZVal(), nil
				}
			}
		}
	}

	return phpv.ZBool(false).ZVal(), nil
}

// > func DateTime date_create ([ string $datetime = "now" [, DateTimeZone $timezone = null ]] )
func fncDateCreate(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	// Pass args to NewZObject so the constructor handles them
	obj, err := phpobj.NewZObject(ctx, DateTime, args...)
	if err != nil {
		// Procedural date_create() returns false on parse failure.
		// Propagate non-date exceptions (TypeError, ArgumentCountError, etc.)
		if pt, isThrow := phpv.UnwrapError(err).(*phperr.PhpThrow); isThrow {
			if exObj, ok := pt.Obj.(*phpobj.ZObject); ok && exObj.Class.InstanceOf(DateException) {
				return phpv.ZBool(false).ZVal(), nil
			}
			return nil, err
		}
		return phpv.ZBool(false).ZVal(), nil
	}
	return obj.ZVal(), nil
}

// > func DateTimeImmutable date_create_immutable ([ string $datetime = "now" [, DateTimeZone $timezone = null ]] )
func fncDateCreateImmutable(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	// Pass args to NewZObject so the constructor handles them
	obj, err := phpobj.NewZObject(ctx, DateTimeImmutable, args...)
	if err != nil {
		// Procedural date_create_immutable() returns false on parse failure.
		// Propagate non-date exceptions (TypeError, ArgumentCountError, etc.)
		if pt, isThrow := phpv.UnwrapError(err).(*phperr.PhpThrow); isThrow {
			if exObj, ok := pt.Obj.(*phpobj.ZObject); ok && exObj.Class.InstanceOf(DateException) {
				return phpv.ZBool(false).ZVal(), nil
			}
			return nil, err
		}
		return phpv.ZBool(false).ZVal(), nil
	}
	return obj.ZVal(), nil
}

// > func string date_format ( DateTimeInterface $object , string $format )
func fncDateFormat(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	if len(args) < 2 {
		return nil, phpobj.ThrowError(ctx, phpobj.ArgumentCountError, "date_format() expects exactly 2 argument")
	}
	dtObj, ok := args[0].Value().(*phpobj.ZObject)
	if !ok {
		return phpv.ZBool(false).ZVal(), nil
	}
	return formatMethod(ctx, dtObj, args[1:])
}

// > func int date_timestamp_get ( DateTimeInterface $object )
func fncDateTimestampGet(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	if len(args) < 1 {
		return nil, phpobj.ThrowError(ctx, phpobj.ArgumentCountError, "date_timestamp_get() expects exactly 1 argument")
	}
	dtObj, ok := args[0].Value().(*phpobj.ZObject)
	if !ok {
		return phpv.ZBool(false).ZVal(), nil
	}
	return getTimestampMethod(ctx, dtObj, nil)
}

// > func int date_offset_get ( DateTimeInterface $object )
func fncDateOffsetGet(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	if len(args) < 1 {
		return nil, phpobj.ThrowError(ctx, phpobj.ArgumentCountError, "date_offset_get() expects exactly 1 argument")
	}
	dtObj, ok := args[0].Value().(*phpobj.ZObject)
	if !ok {
		return phpv.ZBool(false).ZVal(), nil
	}
	return getOffsetMethod(ctx, dtObj, nil)
}

// > func DateTime date_modify ( DateTime $object , string $modifier )
func fncDateModify(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	if len(args) < 2 {
		return nil, phpobj.ThrowError(ctx, phpobj.ArgumentCountError, "date_modify() expects exactly 2 argument")
	}
	dtObj, ok := args[0].Value().(*phpobj.ZObject)
	if !ok {
		return phpv.ZBool(false).ZVal(), nil
	}
	// For the function form, an empty modifier emits a warning and returns false
	// (unlike the method form which throws DateMalformedStringException).
	modifier := args[1].AsString(ctx)
	if len(modifier) == 0 {
		ctx.Warn("Failed to parse time string () at position 0 ( ): Empty string")
		return phpv.ZBool(false).ZVal(), nil
	}
	return modifyMethod(ctx, dtObj, args[1:])
}

// > func DateTimeZone date_timezone_get ( DateTimeInterface $object )
func fncDateTimezoneGet(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	if len(args) < 1 {
		return nil, phpobj.ThrowError(ctx, phpobj.ArgumentCountError, "date_timezone_get() expects exactly 1 argument")
	}
	dtObj, ok := args[0].Value().(*phpobj.ZObject)
	if !ok {
		return phpv.ZBool(false).ZVal(), nil
	}
	return getTimezoneMethod(ctx, dtObj, nil)
}

// > func DateTime date_timezone_set ( DateTime $object , DateTimeZone $timezone )
func fncDateTimezoneSet(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	if len(args) < 2 {
		return nil, phpobj.ThrowError(ctx, phpobj.ArgumentCountError, "date_timezone_set() expects exactly 2 argument")
	}
	dtObj, ok := args[0].Value().(*phpobj.ZObject)
	if !ok {
		return phpv.ZBool(false).ZVal(), nil
	}
	return setTimezoneMethod(ctx, dtObj, args[1:])
}

// > func DateTime date_date_set ( DateTime $object , int $year , int $month , int $day )
func fncDateDateSet(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	if len(args) < 4 {
		return nil, phpobj.ThrowError(ctx, phpobj.ArgumentCountError, "date_date_set() expects exactly 4 argument")
	}
	dtObj, ok := args[0].Value().(*phpobj.ZObject)
	if !ok {
		return phpv.ZBool(false).ZVal(), nil
	}
	return setDateMethod(ctx, dtObj, args[1:])
}

// > func DateTime date_time_set ( DateTime $object , int $hour , int $minute [, int $second = 0 [, int $microsecond = 0 ]] )
func fncDateTimeSet(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	if len(args) < 3 {
		return nil, phpobj.ThrowError(ctx, phpobj.ArgumentCountError, "date_time_set() expects at least 3 argument")
	}
	dtObj, ok := args[0].Value().(*phpobj.ZObject)
	if !ok {
		return phpv.ZBool(false).ZVal(), nil
	}
	return setTimeMethod(ctx, dtObj, args[1:])
}

// > func DateTime date_timestamp_set ( DateTime $object , int $timestamp )
func fncDateTimestampSet(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	if len(args) < 2 {
		return nil, phpobj.ThrowError(ctx, phpobj.ArgumentCountError, "date_timestamp_set() expects exactly 2 argument")
	}
	// Check for null parameter deprecation
	if args[1].IsNull() {
		if err := ctx.Deprecated("Passing null to parameter #2 ($timestamp) of type int is deprecated"); err != nil {
			return nil, err
		}
	}
	dtObj, ok := args[0].Value().(*phpobj.ZObject)
	if !ok {
		return phpv.ZBool(false).ZVal(), nil
	}
	return setTimestampMethod(ctx, dtObj, args[1:])
}

func fncDateAdd(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	if len(args) < 2 {
		return nil, phpobj.ThrowError(ctx, phpobj.ArgumentCountError, "date_add() expects exactly 2 argument")
	}
	dtObj, ok := args[0].Value().(*phpobj.ZObject)
	if !ok {
		return phpv.ZBool(false).ZVal(), nil
	}
	return addMethod(ctx, dtObj, args[1:])
}

func fncDateSub(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	if len(args) < 2 {
		return nil, phpobj.ThrowError(ctx, phpobj.ArgumentCountError, "date_sub() expects exactly 2 argument")
	}
	dtObj, ok := args[0].Value().(*phpobj.ZObject)
	if !ok {
		return phpv.ZBool(false).ZVal(), nil
	}
	// Procedural date_sub() emits a Warning for special intervals instead of throwing.
	intervalObj, ok2 := args[1].Value().(*phpobj.ZObject)
	if ok2 {
		fromStrVal := intervalObj.HashTable().GetString("from_string")
		if fromStrVal != nil && bool(fromStrVal.AsBool(ctx)) {
			ctx.Warn("Only non-special relative time specifications are supported for subtraction")
			return dtObj.ZVal(), nil
		}
	}
	return subMethod(ctx, dtObj, args[1:])
}

func fncDateCreateFromFormat(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	return createFromFormatStaticFor(DateTime)(ctx, args)
}

func fncDateCreateImmutableFromFormat(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	return createFromFormatStaticFor(DateTimeImmutable)(ctx, args)
}

func fncDateIntervalCreateFromDateString(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	if len(args) < 1 {
		return nil, phpobj.ThrowError(ctx, phpobj.ArgumentCountError, "date_interval_create_from_date_string() expects exactly 1 argument")
	}
	if args[0].IsNull() {
		if err := ctx.Deprecated("Passing null to parameter #1 ($datetime) of type string is deprecated"); err != nil {
			return nil, err
		}
	}
	dateStr := string(args[0].AsString(ctx))
	// Procedural version: non-relative elements emit a Warning (not exception) but still return the object.
	// Unknown format emits Warning and returns false.
	result, warnMsg, err := createDateIntervalFromStringMsg(ctx, dateStr, false)
	if err != nil {
		return nil, err
	}
	if warnMsg != "" {
		if warnErr := ctx.Warn("%s", warnMsg); warnErr != nil {
			return nil, warnErr
		}
		if strings.Contains(warnMsg, "non-relative") {
			return result, nil
		}
		return phpv.ZBool(false).ZVal(), nil
	}
	return result, nil
}

func fncDateIntervalFormat(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	if len(args) < 2 {
		return nil, phpobj.ThrowError(ctx, phpobj.ArgumentCountError, "date_interval_format() expects exactly 2 argument")
	}
	intObj, ok := args[0].Value().(*phpobj.ZObject)
	if !ok {
		return phpv.ZBool(false).ZVal(), nil
	}
	return dateIntervalFormat(ctx, intObj, args[1:])
}

func fncDateISODateSet(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	if len(args) < 3 {
		return nil, phpobj.ThrowError(ctx, phpobj.ArgumentCountError, "date_isodate_set() expects at least 3 argument")
	}
	dtObj, ok := args[0].Value().(*phpobj.ZObject)
	if !ok {
		return phpv.ZBool(false).ZVal(), nil
	}
	return setISODateMethod(ctx, dtObj, args[1:])
}

func fncDateGetLastErrors(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	return getLastErrorsStatic(ctx, args)
}

// dateParseHasDate returns true if the input string contains a date component (year-month-day or day.month.year).
var reDateParseHasDate = regexp.MustCompile(`\d{4}[-/.]\d{1,2}[-/.]\d{1,2}|\d{1,2}[-/.]\d{1,2}[-/.]\d{2,4}`)

// dateParseHasTime returns true if the input string contains a time component (HH:MM).
var reDateParseHasTime = regexp.MustCompile(`\d{1,2}:\d{2}`)

// dateParseHasFraction returns true if the input has a fractional seconds part (e.g. ":00.5").
var reDateParseHasFraction = regexp.MustCompile(`:\d{2}\.(\d+)`)

// reDateParseExtractDate extracts year, month, day from a date pattern.
// Supports yyyy-mm-dd, yyyy/mm/dd, and dd.mm.yyyy formats.
var reDateParseExtractDate = regexp.MustCompile(`(\d{4})[-/.](\d{1,2})[-/.](\d{1,2})|(\d{1,2})\.(\d{1,2})\.(\d{4})`)

// extractDateYMD extracts year, month, day strings from a reDateParseExtractDate match.
// The regex has two alternatives: groups 1-3 for yyyy-mm-dd, groups 4-6 for dd.mm.yyyy.
func extractDateYMD(m []string) (year, month, day string) {
	if m[1] != "" {
		// yyyy-mm-dd format
		return m[1], m[2], m[3]
	}
	// dd.mm.yyyy format
	return m[6], m[5], m[4]
}

// reDateParseExtractTime extracts hour, minute, optional second from a time pattern
var reDateParseExtractTime = regexp.MustCompile(`(\d{1,2}):(\d{2})(?::(\d{2}))?`)

// relativeUnitRe matches patterns like "+3 months", "next year", "last day", etc.
var relativeUnitRe = regexp.MustCompile(`(?i)([+-]?\d+)\s*(year|month|day|hour|minute|second|week)s?`)
var relativeNextLastRe = regexp.MustCompile(`(?i)(next|last|previous)\s+(year|month|week|hour|minute|second)`)
var relativeFirstLastDayOfRe = regexp.MustCompile(`(?i)(first|last)\s+day\s+of`)

// relativeFirstLastDayRe matches "first day" or "last day" NOT followed by " of".
// This handles standalone "first day" / "last day" and compound "first day next month".
var relativeFirstLastDayRe = regexp.MustCompile(`(?i)(first|last)\s+day(?:\s+of\b)?`)

// parseRelativeComponents parses relative date components from a string
// and returns a "relative" array if any are found, or nil if none.
func parseRelativeComponents(ctx phpv.Context, datetime string) *phpv.ZArray {
	lower := strings.ToLower(strings.TrimSpace(datetime))
	relYear, relMonth, relDay, relHour, relMinute, relSecond := 0, 0, 0, 0, 0, 0
	firstDayOf := false
	lastDayOf := false
	hasRelative := false

	// Check for "first day of" / "last day of" qualifiers (sets day to 1st/last of month)
	if relativeFirstLastDayOfRe.MatchString(lower) {
		m := relativeFirstLastDayOfRe.FindStringSubmatch(lower)
		if m[1] == "first" {
			firstDayOf = true
		} else {
			lastDayOf = true
		}
		hasRelative = true
	} else if m := relativeFirstLastDayRe.FindStringSubmatch(lower); m != nil {
		// "first day" or "last day" without "of" - these are relative day modifiers
		if m[1] == "first" {
			relDay += 1
		} else {
			relDay -= 1
		}
		hasRelative = true
	}

	// Check for "next/last UNIT" patterns (not "day" - that's handled above)
	for _, m := range relativeNextLastRe.FindAllStringSubmatch(lower, -1) {
		dir := 1
		if m[1] == "last" || m[1] == "previous" {
			dir = -1
		}
		switch m[2] {
		case "year":
			relYear += dir
		case "month":
			relMonth += dir
		case "week":
			relDay += dir * 7
		case "hour":
			relHour += dir
		case "minute":
			relMinute += dir
		case "second":
			relSecond += dir
		}
		hasRelative = true
	}

	// Check for "+N unit" / "-N unit" patterns
	for _, m := range relativeUnitRe.FindAllStringSubmatch(lower, -1) {
		n, _ := strconv.Atoi(m[1])
		switch m[2] {
		case "year":
			relYear += n
		case "month":
			relMonth += n
		case "week":
			relDay += n * 7
		case "day":
			relDay += n
		case "hour":
			relHour += n
		case "minute":
			relMinute += n
		case "second":
			relSecond += n
		}
		hasRelative = true
	}

	// Also check for ordinal weekday patterns like "third tuesday of next month"
	if matched, _ := regexp.MatchString(`(?i)(first|second|third|fourth|fifth|sixth|seventh|eighth|ninth|tenth|eleventh|twelfth|last)\s+(mon|tue|wed|thu|fri|sat|sun)`, lower); matched {
		hasRelative = true
	}

	// Check for "tomorrow" / "yesterday"
	if strings.Contains(lower, "tomorrow") {
		relDay++
		hasRelative = true
	} else if strings.Contains(lower, "yesterday") {
		relDay--
		hasRelative = true
	}

	if !hasRelative {
		return nil
	}

	rel := phpv.NewZArray()
	rel.OffsetSet(ctx, phpv.ZString("year"), phpv.ZInt(relYear).ZVal())
	rel.OffsetSet(ctx, phpv.ZString("month"), phpv.ZInt(relMonth).ZVal())
	rel.OffsetSet(ctx, phpv.ZString("day"), phpv.ZInt(relDay).ZVal())
	rel.OffsetSet(ctx, phpv.ZString("hour"), phpv.ZInt(relHour).ZVal())
	rel.OffsetSet(ctx, phpv.ZString("minute"), phpv.ZInt(relMinute).ZVal())
	rel.OffsetSet(ctx, phpv.ZString("second"), phpv.ZInt(relSecond).ZVal())
	if firstDayOf {
		rel.OffsetSet(ctx, phpv.ZString("first_day_of_month"), phpv.ZBool(true).ZVal())
	}
	if lastDayOf {
		rel.OffsetSet(ctx, phpv.ZString("last_day_of_month"), phpv.ZBool(true).ZVal())
	}
	return rel
}

func fncDateParse(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	if len(args) < 1 {
		return nil, phpobj.ThrowError(ctx, phpobj.ArgumentCountError, "date_parse() expects exactly 1 argument, 0 given")
	}
	datetime := string(args[0].AsString(ctx))
	pd := strtotime.DateParse(datetime)
	return parsedDateToZArray(ctx, pd)
}

// parsedDateToZArray converts a strtotime.ParsedDate to a PHP array matching
// PHP's date_parse() output format.
func parsedDateToZArray(ctx phpv.Context, pd *strtotime.ParsedDate) (*phpv.ZVal, error) {
	result := phpv.NewZArray()

	// Year/month/day: set if present, false if not
	if pd.Year.Set {
		result.OffsetSet(ctx, phpv.ZString("year"), phpv.ZInt(pd.Year.V).ZVal())
	} else {
		result.OffsetSet(ctx, phpv.ZString("year"), phpv.ZBool(false).ZVal())
	}
	if pd.Month.Set {
		result.OffsetSet(ctx, phpv.ZString("month"), phpv.ZInt(pd.Month.V).ZVal())
	} else {
		result.OffsetSet(ctx, phpv.ZString("month"), phpv.ZBool(false).ZVal())
	}
	if pd.Day.Set {
		result.OffsetSet(ctx, phpv.ZString("day"), phpv.ZInt(pd.Day.V).ZVal())
	} else {
		result.OffsetSet(ctx, phpv.ZString("day"), phpv.ZBool(false).ZVal())
	}

	// Hour/minute/second
	if pd.Hour.Set {
		result.OffsetSet(ctx, phpv.ZString("hour"), phpv.ZInt(pd.Hour.V).ZVal())
	} else {
		result.OffsetSet(ctx, phpv.ZString("hour"), phpv.ZBool(false).ZVal())
	}
	if pd.Minute.Set {
		result.OffsetSet(ctx, phpv.ZString("minute"), phpv.ZInt(pd.Minute.V).ZVal())
	} else {
		result.OffsetSet(ctx, phpv.ZString("minute"), phpv.ZBool(false).ZVal())
	}
	if pd.Second.Set {
		result.OffsetSet(ctx, phpv.ZString("second"), phpv.ZInt(pd.Second.V).ZVal())
	} else {
		result.OffsetSet(ctx, phpv.ZString("second"), phpv.ZBool(false).ZVal())
	}

	// Fraction: float(0) when time components are present, false when no time parsed
	if pd.Fraction.Set {
		result.OffsetSet(ctx, phpv.ZString("fraction"), phpv.ZFloat(pd.Fraction.V).ZVal())
	} else if pd.Hour.Set || pd.Minute.Set || pd.Second.Set {
		result.OffsetSet(ctx, phpv.ZString("fraction"), phpv.ZFloat(0).ZVal())
	} else {
		result.OffsetSet(ctx, phpv.ZString("fraction"), phpv.ZBool(false).ZVal())
	}

	// Warnings
	result.OffsetSet(ctx, phpv.ZString("warning_count"), phpv.ZInt(pd.WarningCount).ZVal())
	warnArr := phpv.NewZArray()
	for pos, msg := range pd.Warnings {
		warnArr.OffsetSet(ctx, phpv.ZInt(pos), phpv.ZString(msg).ZVal())
	}
	result.OffsetSet(ctx, phpv.ZString("warnings"), warnArr.ZVal())

	// Errors
	result.OffsetSet(ctx, phpv.ZString("error_count"), phpv.ZInt(pd.ErrorCount).ZVal())
	errArr := phpv.NewZArray()
	for pos, msg := range pd.Errors {
		errArr.OffsetSet(ctx, phpv.ZInt(pos), phpv.ZString(msg).ZVal())
	}
	result.OffsetSet(ctx, phpv.ZString("errors"), errArr.ZVal())

	// Timezone info
	if pd.IsLocaltime {
		result.OffsetSet(ctx, phpv.ZString("is_localtime"), phpv.ZBool(true).ZVal())
		result.OffsetSet(ctx, phpv.ZString("zone_type"), phpv.ZInt(pd.ZoneType).ZVal())
		result.OffsetSet(ctx, phpv.ZString("zone"), phpv.ZInt(pd.Zone).ZVal())
		result.OffsetSet(ctx, phpv.ZString("is_dst"), phpv.ZBool(pd.IsDST).ZVal())
		if pd.ZoneType == 2 && pd.TzAbbr != "" {
			result.OffsetSet(ctx, phpv.ZString("tz_abbr"), phpv.ZString(pd.TzAbbr).ZVal())
		}
		if pd.ZoneType == 3 && pd.TzID != "" {
			result.OffsetSet(ctx, phpv.ZString("tz_id"), phpv.ZString(pd.TzID).ZVal())
		}
	} else {
		result.OffsetSet(ctx, phpv.ZString("is_localtime"), phpv.ZBool(false).ZVal())
	}

	// Relative components
	if pd.Relative != nil {
		rel := phpv.NewZArray()
		rel.OffsetSet(ctx, phpv.ZString("year"), phpv.ZInt(pd.Relative.Year).ZVal())
		rel.OffsetSet(ctx, phpv.ZString("month"), phpv.ZInt(pd.Relative.Month).ZVal())
		rel.OffsetSet(ctx, phpv.ZString("day"), phpv.ZInt(pd.Relative.Day).ZVal())
		rel.OffsetSet(ctx, phpv.ZString("hour"), phpv.ZInt(pd.Relative.Hour).ZVal())
		rel.OffsetSet(ctx, phpv.ZString("minute"), phpv.ZInt(pd.Relative.Minute).ZVal())
		rel.OffsetSet(ctx, phpv.ZString("second"), phpv.ZInt(pd.Relative.Second).ZVal())
		if pd.Relative.Weekday.Set {
			rel.OffsetSet(ctx, phpv.ZString("weekday"), phpv.ZInt(pd.Relative.Weekday.V).ZVal())
		}
		result.OffsetSet(ctx, phpv.ZString("relative"), rel.ZVal())
	}

	return result.ZVal(), nil
}

// Below: old fncDateParse body deleted. parsedDateToZArray above replaces it.
// --- dead code removed ---

// reDateParseHasTimezone remains for other functions.
func fncDateParseFromFormat(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	if len(args) < 2 {
		return nil, phpobj.ThrowError(ctx, phpobj.ArgumentCountError, fmt.Sprintf("date_parse_from_format() expects exactly 2 arguments, %d given", len(args)))
	}
	format := string(args[0].AsString(ctx))
	datetime := string(args[1].AsString(ctx))
	// Reject null bytes (PHP 8.3+: ValueError)
	if strings.ContainsRune(datetime, 0) {
		return nil, phpobj.ThrowError(ctx, phpobj.ValueError,
			"date_parse_from_format(): Argument #2 ($datetime) must not contain any null bytes")
	}
	result := phpv.NewZArray()
	parseErrors := newDateParseErrors()
	t, ok := createFromFormatParsed(ctx, format, datetime, time.UTC)

	// Determine which fields were set based on the format
	hasYear := false
	hasMonth := false
	hasDay := false
	hasHour := false
	hasMinute := false
	hasSecond := false
	hasFraction := false
	for i := 0; i < len(format); i++ {
		switch format[i] {
		case 'Y', 'y', 'X', 'x':
			hasYear = true
		case 'm', 'n', 'M', 'F':
			hasMonth = true
		case 'd', 'j', 'D', 'l':
			hasDay = true
		case 'z': // day of year sets month and day
			hasMonth = true
			hasDay = true
		case 'H', 'G', 'h', 'g':
			hasHour = true
		case 'i':
			hasMinute = true
		case 's':
			hasSecond = true
		case 'u', 'v': // microseconds/milliseconds
			hasFraction = true
		case 'U': // unix timestamp sets everything
			hasYear = true
			hasMonth = true
			hasDay = true
			hasHour = true
			hasMinute = true
			hasSecond = true
		case '!', '|': // reset all fields to Unix epoch defaults
			hasYear = true
			hasMonth = true
			hasDay = true
			hasHour = true
			hasMinute = true
			hasSecond = true
		case '\\':
			i++ // skip next char
		}
	}
	_ = hasFraction

	if !ok {
		result.OffsetSet(ctx, phpv.ZString("year"), phpv.ZBool(false).ZVal())
		result.OffsetSet(ctx, phpv.ZString("month"), phpv.ZBool(false).ZVal())
		result.OffsetSet(ctx, phpv.ZString("day"), phpv.ZBool(false).ZVal())
		result.OffsetSet(ctx, phpv.ZString("hour"), phpv.ZBool(false).ZVal())
		result.OffsetSet(ctx, phpv.ZString("minute"), phpv.ZBool(false).ZVal())
		result.OffsetSet(ctx, phpv.ZString("second"), phpv.ZBool(false).ZVal())
	} else {
		if hasYear {
			result.OffsetSet(ctx, phpv.ZString("year"), phpv.ZInt(t.Year()).ZVal())
		} else {
			result.OffsetSet(ctx, phpv.ZString("year"), phpv.ZBool(false).ZVal())
		}
		if hasMonth {
			result.OffsetSet(ctx, phpv.ZString("month"), phpv.ZInt(int(t.Month())).ZVal())
		} else {
			result.OffsetSet(ctx, phpv.ZString("month"), phpv.ZBool(false).ZVal())
		}
		if hasDay {
			result.OffsetSet(ctx, phpv.ZString("day"), phpv.ZInt(t.Day()).ZVal())
		} else {
			result.OffsetSet(ctx, phpv.ZString("day"), phpv.ZBool(false).ZVal())
		}
		if hasHour {
			result.OffsetSet(ctx, phpv.ZString("hour"), phpv.ZInt(t.Hour()).ZVal())
		} else {
			result.OffsetSet(ctx, phpv.ZString("hour"), phpv.ZBool(false).ZVal())
		}
		if hasMinute {
			result.OffsetSet(ctx, phpv.ZString("minute"), phpv.ZInt(t.Minute()).ZVal())
		} else {
			result.OffsetSet(ctx, phpv.ZString("minute"), phpv.ZBool(false).ZVal())
		}
		if hasSecond {
			result.OffsetSet(ctx, phpv.ZString("second"), phpv.ZInt(t.Second()).ZVal())
		} else {
			result.OffsetSet(ctx, phpv.ZString("second"), phpv.ZBool(false).ZVal())
		}
	}

	// Fraction is false when no time fields were in the format
	if hasHour || hasMinute || hasSecond || hasFraction {
		result.OffsetSet(ctx, phpv.ZString("fraction"), phpv.ZFloat(0).ZVal())
	} else {
		result.OffsetSet(ctx, phpv.ZString("fraction"), phpv.ZBool(false).ZVal())
	}

	// Add warnings and errors from parsing
	result.OffsetSet(ctx, phpv.ZString("warning_count"), phpv.ZInt(parseErrors.warningCount).ZVal())
	warnArr := phpv.NewZArray()
	for pos, msg := range parseErrors.warnings {
		warnArr.OffsetSet(ctx, phpv.ZInt(pos), phpv.ZString(msg).ZVal())
	}
	result.OffsetSet(ctx, phpv.ZString("warnings"), warnArr.ZVal())
	result.OffsetSet(ctx, phpv.ZString("error_count"), phpv.ZInt(parseErrors.errorCount).ZVal())
	errArr := phpv.NewZArray()
	for pos, msg := range parseErrors.errors {
		errArr.OffsetSet(ctx, phpv.ZInt(pos), phpv.ZString(msg).ZVal())
	}
	result.OffsetSet(ctx, phpv.ZString("errors"), errArr.ZVal())
	result.OffsetSet(ctx, phpv.ZString("is_localtime"), phpv.ZBool(false).ZVal())
	return result.ZVal(), nil
}

func fncGettimeofday(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var asFloat *phpv.ZBool
	_, err := core.Expand(ctx, args, &asFloat)
	if err != nil {
		return nil, err
	}
	now := time.Now()
	if asFloat != nil && bool(*asFloat) {
		return phpv.ZFloat(float64(now.UnixNano()) / 1e9).ZVal(), nil
	}
	loc := getTimezone(ctx)
	nowInLoc := now.In(loc)
	_, offsetSec := nowInLoc.Zone()
	minuteswest := -offsetSec / 60
	dsttime := 0
	if isDST(nowInLoc) {
		dsttime = 1
	}
	result := phpv.NewZArray()
	result.OffsetSet(ctx, phpv.ZString("sec"), phpv.ZInt(now.Unix()).ZVal())
	result.OffsetSet(ctx, phpv.ZString("usec"), phpv.ZInt(int64(now.Nanosecond()/1000)).ZVal())
	result.OffsetSet(ctx, phpv.ZString("minuteswest"), phpv.ZInt(minuteswest).ZVal())
	result.OffsetSet(ctx, phpv.ZString("dsttime"), phpv.ZInt(dsttime).ZVal())
	return result.ZVal(), nil
}

// tzdataVersion caches the system timezone database version string.
var tzdataVersion string

func getTzdataVersion() string {
	if tzdataVersion != "" {
		return tzdataVersion
	}
	// Try to read version from system tzdata.zi file
	data, err := os.ReadFile("/usr/share/zoneinfo/tzdata.zi")
	if err == nil {
		// File starts with "# version 2025b\n"
		lines := strings.Split(string(data), "\n")
		for _, line := range lines {
			if strings.HasPrefix(line, "# version ") {
				ver := strings.TrimPrefix(line, "# version ")
				ver = strings.TrimSpace(ver)
				// Convert e.g. "2025b" -> "2025.2" (a=1, b=2, c=3, ...)
				if len(ver) >= 5 {
					year := ver[:4]
					letter := ver[4:]
					if len(letter) > 0 && letter[0] >= 'a' && letter[0] <= 'z' {
						num := int(letter[0]-'a') + 1
						tzdataVersion = year + "." + strconv.Itoa(num)
						return tzdataVersion
					}
				}
				tzdataVersion = ver
				return tzdataVersion
			}
		}
	}
	tzdataVersion = "0.system"
	return tzdataVersion
}

func fncTimezoneVersionGet(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	return phpv.ZString(getTzdataVersion()).ZVal(), nil
}

func fncTimezoneLocationGet(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	if len(args) < 1 {
		return nil, phpobj.ThrowError(ctx, phpobj.ArgumentCountError, "timezone_location_get() expects exactly 1 argument")
	}
	tzObj, ok := args[0].Value().(*phpobj.ZObject)
	if !ok {
		return phpv.ZBool(false).ZVal(), nil
	}
	if err := checkDateTimeZoneInitialized(ctx, tzObj); err != nil {
		return nil, err
	}
	return getTimezoneLocation(ctx, tzObj)
}

// zoneTabEntry holds per-zone metadata from zone.tab (which includes link zones).
type zoneTabEntry struct {
	CountryCode string
	Lat, Lon    float64
	Comments    string
}

var (
	zoneTabOnce sync.Once
	zoneTabMap  map[string]*zoneTabEntry
)

// loadZoneTab parses the system zone.tab file which has entries for all timezones,
// including link zones. This supplements gotz's zone1970.tab which only has canonical names.
func loadZoneTab() {
	zoneTabOnce.Do(func() {
		zoneTabMap = make(map[string]*zoneTabEntry)
		paths := []string{
			"/usr/share/zoneinfo/zone.tab",
			"/usr/lib/zoneinfo/zone.tab",
			"/usr/share/lib/zoneinfo/zone.tab",
			"/etc/zoneinfo/zone.tab",
		}
		var data []byte
		for _, p := range paths {
			var err error
			data, err = os.ReadFile(p)
			if err == nil {
				break
			}
		}
		if data == nil {
			return
		}
		for _, line := range strings.Split(string(data), "\n") {
			if line == "" || line[0] == '#' {
				continue
			}
			fields := strings.Split(line, "\t")
			if len(fields) < 3 {
				continue
			}
			cc := fields[0]
			name := fields[2]
			var comments string
			if len(fields) >= 4 {
				comments = fields[3]
			}
			lat, lon := parseISO6709Coords(fields[1])
			zoneTabMap[name] = &zoneTabEntry{
				CountryCode: cc,
				Lat:         lat,
				Lon:         lon,
				Comments:    comments,
			}
		}
	})
}

// parseISO6709Coords parses ISO 6709 coordinates: ±DDMM±DDDMM or ±DDMMSS±DDDMMSS
func parseISO6709Coords(s string) (lat, lon float64) {
	lonStart := -1
	for i := 1; i < len(s); i++ {
		if s[i] == '+' || s[i] == '-' {
			lonStart = i
			break
		}
	}
	if lonStart < 0 {
		return 0, 0
	}
	lat = parseDMSCoord(s[:lonStart], 2)
	lon = parseDMSCoord(s[lonStart:], 3)
	return
}

// parseDMSCoord parses ±DD[D]MM[SS] into decimal degrees.
// No rounding is applied to match PHP's precision.
func parseDMSCoord(s string, degDigits int) float64 {
	if len(s) < 1+degDigits+2 {
		return 0
	}
	neg := s[0] == '-'
	s = s[1:]
	deg := atoiSimple(s[:degDigits])
	s = s[degDigits:]
	min := atoiSimple(s[:2])
	s = s[2:]
	var sec int
	if len(s) >= 2 {
		sec = atoiSimple(s[:2])
	}
	v := float64(deg) + float64(min)/60.0 + float64(sec)/3600.0
	if neg {
		v = -v
	}
	return v
}

func atoiSimple(s string) int {
	n := 0
	for _, c := range s {
		n = n*10 + int(c-'0')
	}
	return n
}

func getTimezoneLocation(ctx phpv.Context, tzObj *phpobj.ZObject) (*phpv.ZVal, error) {
	tzName := getTimezoneName(tzObj)
	zone, err := gotz.Load(tzName)
	if err != nil {
		// Fixed-offset or abbreviation timezones return false for getLocation
		return phpv.ZBool(false).ZVal(), nil
	}

	var countryCode, comments string
	var lat, lon float64

	meta := zone.Meta()
	if meta != nil {
		countryCode = "??"
		if len(meta.Countries) > 0 {
			countryCode = meta.Countries[0].Code
		}
		comments = meta.Commentary
		lat = meta.Lat
		lon = meta.Lon
	} else {
		// gotz's zone1970.tab may not have this zone (it's a link).
		// Fall back to system zone.tab which has entries for link zones too.
		loadZoneTab()
		if entry, ok := zoneTabMap[tzName]; ok {
			countryCode = entry.CountryCode
			lat = entry.Lat
			lon = entry.Lon
			comments = entry.Comments
		} else {
			return phpv.ZBool(false).ZVal(), nil
		}
	}

	result := phpv.NewZArray()
	result.OffsetSet(ctx, phpv.ZString("country_code"), phpv.ZString(countryCode).ZVal())
	result.OffsetSet(ctx, phpv.ZString("latitude"), phpv.ZFloat(lat).ZVal())
	result.OffsetSet(ctx, phpv.ZString("longitude"), phpv.ZFloat(lon).ZVal())
	result.OffsetSet(ctx, phpv.ZString("comments"), phpv.ZString(comments).ZVal())
	return result.ZVal(), nil
}

func fncTimezoneTransitionsGet(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	if len(args) < 1 {
		return nil, phpobj.ThrowError(ctx, phpobj.ArgumentCountError, "timezone_transitions_get() expects at least 1 argument")
	}
	tzObj, ok := args[0].Value().(*phpobj.ZObject)
	if !ok {
		return phpv.ZBool(false).ZVal(), nil
	}
	if err := checkDateTimeZoneInitialized(ctx, tzObj); err != nil {
		return nil, err
	}
	tzName := getTimezoneName(tzObj)

	// Optional timestamp range filters
	var tsBegin, tsEnd int64
	tsBegin = math.MinInt32
	tsEnd = math.MaxInt32
	if len(args) > 1 && args[1] != nil && !args[1].IsNull() {
		tsBegin = int64(args[1].AsInt(ctx))
	}
	if len(args) > 2 && args[2] != nil && !args[2].IsNull() {
		tsEnd = int64(args[2].AsInt(ctx))
	}

	result := phpv.NewZArray()

	// Try to load from gotz for full transition data
	zone, err := gotz.Load(tzName)
	if err != nil {
		// Fallback for fixed-offset timezones
		loc, ok := getTimezoneLoc(tzObj)
		if !ok {
			return phpv.ZBool(false).ZVal(), nil
		}
		now := time.Now().In(loc)
		name, offset := now.Zone()
		entry := phpv.NewZArray()
		entry.OffsetSet(ctx, phpv.ZString("ts"), phpv.ZInt(0).ZVal())
		entry.OffsetSet(ctx, phpv.ZString("time"), phpv.ZString("1970-01-01T00:00:00+00:00").ZVal())
		entry.OffsetSet(ctx, phpv.ZString("offset"), phpv.ZInt(offset).ZVal())
		entry.OffsetSet(ctx, phpv.ZString("isdst"), phpv.ZBool(false).ZVal())
		entry.OffsetSet(ctx, phpv.ZString("abbr"), phpv.ZString(name).ZVal())
		result.OffsetSet(ctx, nil, entry.ZVal())
		return result.ZVal(), nil
	}

	types := zone.Types()

	// Use TransitionsForRange to get transitions including POSIX-rule generated ones
	startTime := time.Unix(tsBegin, 0)
	endTime := time.Unix(tsEnd, 0)
	transitions := zone.TransitionsForRange(startTime, endTime)

	// PHP always includes the initial state before the first transition in the range.
	// Use Lookup to find what zone type is active at the start of the range.
	initType := zone.Lookup(startTime)
	entry := phpv.NewZArray()
	entry.OffsetSet(ctx, phpv.ZString("ts"), phpv.ZInt(tsBegin).ZVal())
	entry.OffsetSet(ctx, phpv.ZString("time"), phpv.ZString(startTime.UTC().Format("2006-01-02T15:04:05+00:00")).ZVal())
	entry.OffsetSet(ctx, phpv.ZString("offset"), phpv.ZInt(initType.Offset).ZVal())
	entry.OffsetSet(ctx, phpv.ZString("isdst"), phpv.ZBool(initType.IsDST).ZVal())
	entry.OffsetSet(ctx, phpv.ZString("abbr"), phpv.ZString(initType.Abbrev).ZVal())
	result.OffsetSet(ctx, nil, entry.ZVal())

	for _, tr := range transitions {
		if tr.Type < 0 || tr.Type >= len(types) {
			continue
		}
		// Skip transitions at or before the initial entry timestamp to avoid duplicates
		if tr.When <= tsBegin {
			continue
		}
		zt := types[tr.Type]
		entry := phpv.NewZArray()
		entry.OffsetSet(ctx, phpv.ZString("ts"), phpv.ZInt(tr.When).ZVal())
		t := time.Unix(tr.When, 0).UTC()
		entry.OffsetSet(ctx, phpv.ZString("time"), phpv.ZString(t.Format("2006-01-02T15:04:05+00:00")).ZVal())
		entry.OffsetSet(ctx, phpv.ZString("offset"), phpv.ZInt(zt.Offset).ZVal())
		entry.OffsetSet(ctx, phpv.ZString("isdst"), phpv.ZBool(zt.IsDST).ZVal())
		entry.OffsetSet(ctx, phpv.ZString("abbr"), phpv.ZString(zt.Abbrev).ZVal())
		result.OffsetSet(ctx, nil, entry.ZVal())
	}

	return result.ZVal(), nil
}

func fncDateSunInfo(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	if len(args) < 3 {
		return nil, phpobj.ThrowError(ctx, phpobj.ArgumentCountError, fmt.Sprintf("date_sun_info() expects exactly 3 arguments, %d given", len(args)))
	}
	timestamp := int64(args[0].AsInt(ctx))
	latitude := float64(args[1].AsFloat(ctx))
	longitude := float64(args[2].AsFloat(ctx))
	if math.IsNaN(latitude) || math.IsInf(latitude, 0) {
		return nil, phpobj.ThrowError(ctx, phpobj.ValueError, "date_sun_info(): Argument #2 ($latitude) must be finite")
	}
	if math.IsNaN(longitude) || math.IsInf(longitude, 0) {
		return nil, phpobj.ThrowError(ctx, phpobj.ValueError, "date_sun_info(): Argument #3 ($longitude) must be finite")
	}
	// Use the default timezone's local date (not UTC) to determine the day,
	// matching PHP's timelib_unixtime2local behavior in date_sun_info().
	loc := getTimezone(ctx)
	t := time.Unix(timestamp, 0).In(loc)
	dayStart := time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC)
	midnightTS := dayStart.Unix()

	result := phpv.NewZArray()

	// sunRiseSetVal returns the appropriate ZVal for a sun calculation result:
	// normal → timestamp, neverRises → false, neverSets → true
	sunRiseSetVal := func(hours float64, rc sunRC) *phpv.ZVal {
		switch rc {
		case sunNeverRises:
			return phpv.ZBool(false).ZVal()
		case sunNeverSets:
			return phpv.ZBool(true).ZVal()
		default:
			return phpv.ZInt(midnightTS + int64(hours*3600)).ZVal()
		}
	}

	// Sunrise/sunset: altit = -35/60 degrees (refraction), upperLimb = true
	// The upper limb correction subtracts the apparent sun radius (~0.2666/sr),
	// giving an effective altitude of approximately -50/60 degrees.
	rise, set, transit, rc := calculateSunRiseSetTransit(midnightTS, latitude, longitude, -35.0/60.0, true)
	result.OffsetSet(ctx, phpv.ZString("sunrise"), sunRiseSetVal(rise, rc))
	result.OffsetSet(ctx, phpv.ZString("sunset"), sunRiseSetVal(set, rc))
	result.OffsetSet(ctx, phpv.ZString("transit"), phpv.ZInt(midnightTS+int64(transit*3600)).ZVal())

	// Twilight calculations with different altitudes, no upper limb correction
	for _, tw := range []struct {
		altit    float64
		beginKey string
		endKey   string
	}{
		{-6.0, "civil_twilight_begin", "civil_twilight_end"},
		{-12.0, "nautical_twilight_begin", "nautical_twilight_end"},
		{-18.0, "astronomical_twilight_begin", "astronomical_twilight_end"},
	} {
		begin, end, _, twRC := calculateSunRiseSetTransit(midnightTS, latitude, longitude, tw.altit, false)
		result.OffsetSet(ctx, phpv.ZString(tw.beginKey), sunRiseSetVal(begin, twRC))
		result.OffsetSet(ctx, phpv.ZString(tw.endKey), sunRiseSetVal(end, twRC))
	}
	return result.ZVal(), nil
}

func fncDateSunrise(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	return dateSunFunc(ctx, args, true)
}

func fncDateSunset(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	return dateSunFunc(ctx, args, false)
}

func dateSunFunc(ctx phpv.Context, args []*phpv.ZVal, isSunrise bool) (*phpv.ZVal, error) {
	funcName := "date_sunset"
	if isSunrise {
		funcName = "date_sunrise"
	}
	if err := ctx.Deprecated(fmt.Sprintf("Function %s() is deprecated since 8.1, use date_sun_info() instead", funcName), logopt.NoFuncName(true)); err != nil {
		return nil, err
	}
	if len(args) < 1 {
		return nil, phpobj.ThrowError(ctx, phpobj.ArgumentCountError, fmt.Sprintf("%s() expects at least 1 argument", funcName))
	}
	timestamp := int64(args[0].AsInt(ctx))
	returnFormat := 1
	latitude := 0.0
	longitude := 0.0
	zenith := solarZenithSunrise
	utcOffset := math.NaN()
	if len(args) > 1 {
		returnFormat = int(args[1].AsInt(ctx))
	}
	if returnFormat < 0 || returnFormat > 2 {
		return nil, phpobj.ThrowError(ctx, phpobj.ValueError, fmt.Sprintf("%s(): Argument #2 ($returnFormat) must be one of SUNFUNCS_RET_TIMESTAMP, SUNFUNCS_RET_STRING, or SUNFUNCS_RET_DOUBLE", funcName))
	}
	if len(args) > 2 {
		latitude = float64(args[2].AsFloat(ctx))
	}
	if len(args) > 3 {
		longitude = float64(args[3].AsFloat(ctx))
	}
	if len(args) > 4 {
		zenith = float64(args[4].AsFloat(ctx))
	}
	utcOffsetProvided := false
	if len(args) > 5 {
		utcOffset = float64(args[5].AsFloat(ctx))
		utcOffsetProvided = true
	}

	// If utcOffset was explicitly provided and is non-finite, return false
	if utcOffsetProvided && (math.IsNaN(utcOffset) || math.IsInf(utcOffset, 0)) {
		return phpv.ZBool(false).ZVal(), nil
	}

	// Use the default timezone's calendar date (not UTC) to determine the day
	loc := getTimezone(ctx)
	ut := calculateSunTime(timestamp, latitude, longitude, zenith, isSunrise, loc)
	if math.IsNaN(ut) {
		return phpv.ZBool(false).ZVal(), nil
	}

	// If utcOffset was not provided, derive it from the default timezone
	if !utcOffsetProvided {
		_, offsetSec := time.Unix(timestamp, 0).In(loc).Zone()
		utcOffset = float64(offsetSec) / 3600.0
	}

	// For SUNFUNCS_RET_TIMESTAMP: compute from UTC hours directly (no offset adjustment).
	// For STRING/DOUBLE: adjust to local time by adding utcOffset.
	switch returnFormat {
	case 0:
		// SUNFUNCS_RET_TIMESTAMP: return Unix timestamp = UTC midnight + UTC hours * 3600
		tLocal := time.Unix(timestamp, 0).In(loc)
		dayStart := time.Date(tLocal.Year(), tLocal.Month(), tLocal.Day(), 0, 0, 0, 0, time.UTC)
		return phpv.ZInt(dayStart.Unix() + int64(ut*3600)).ZVal(), nil
	case 1, 2:
		// SUNFUNCS_RET_STRING / SUNFUNCS_RET_DOUBLE: adjust to local time
		ut += utcOffset
		// If ut is infinite, NaN, or too extreme for the normalization loops, return false
		if math.IsInf(ut, 0) || math.IsNaN(ut) || ut > 1e15 || ut < -1e15 {
			return phpv.ZBool(false).ZVal(), nil
		}
		for ut < 0 {
			ut += 24
		}
		for ut >= 24 {
			ut -= 24
		}
		if returnFormat == 1 {
			hours := int(ut)
			minutes := int((ut - float64(hours)) * 60)
			return phpv.ZString(fmt.Sprintf("%02d:%02d", hours, minutes)).ZVal(), nil
		}
		return phpv.ZFloat(ut).ZVal(), nil
	}
	return phpv.ZBool(false).ZVal(), nil
}
