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
	"github.com/MagicalTux/goro/core"
	"github.com/MagicalTux/goro/core/phperr"
	"github.com/MagicalTux/goro/core/phpobj"
	"github.com/MagicalTux/goro/core/phpv"
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

// calculateSunRiseSetTransit computes sunrise, sunset, and transit times
// as hours from UTC midnight. altit is the altitude in degrees (-0.8333 for standard rise/set).
// upperLimb: if true, adjusts for the sun's apparent radius.
func calculateSunRiseSetTransit(utcMidnightTS int64, lat, lon, altit float64, upperLimb bool) (rise, set, transit float64) {
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
		return
	}
	if cost <= -1.0 {
		// Sun never sets (midnight sun)
		rise = math.NaN()
		set = math.NaN()
		return
	}

	t := acosd(cost) / 15.0
	rise = tsouth - t
	set = tsouth + t
	return
}

const solarZenithSunrise = 90.5833

// calculateSunTime provides backward compatibility with callers that use the old zenith-based API.
// It wraps the Schlyter algorithm.
func calculateSunTime(timestamp int64, latitude, longitude, zenith float64, isSunrise bool) float64 {
	t := time.Unix(timestamp, 0).UTC()
	dayStart := time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC)
	altit := -(zenith - 90.0)
	// Use upper limb correction for standard sunrise/sunset zenith values
	upperLimb := zenith <= 91 && zenith >= 90
	rise, set, _ := calculateSunRiseSetTransit(dayStart.Unix(), latitude, longitude, altit, upperLimb)
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
	if loc, err := time.LoadLocation(tzName); err == nil {
		return loc, nil
	}
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
	// Try timezone abbreviations (PHP type 2)
	if offset, ok := timezoneAbbreviationOffsets[upper]; ok {
		return time.FixedZone(tzName, offset), nil
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
				return nil, phpobj.ThrowError(ctx, phpobj.Exception, "DateTimeZone::__construct(): Timezone offset is out of range ("+tzName+")")
			}
		}
	}

	loc, err := parseTzName(tzName)
	if err != nil {
		return nil, phpobj.ThrowError(ctx, phpobj.Exception, "DateTimeZone::__construct(): Unknown or bad timezone ("+tzName+")")
	}

	// Normalize offset timezone names to +HH:MM format
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
			formattedName := fmt.Sprintf("%s%02d:%02d", sign, hours, mins)
			loc = time.FixedZone(formattedName, offset)
		}
	}

	setTimezoneLoc(this, loc)
	// Also store timezone_type and timezone in the hash table so var_export works
	setTimezoneProps(this, loc)
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
	"j": 0,       // Juliet = local time
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

// buildTZAbbrevMap builds the complete PHP timezone abbreviation map from all IANA zones.
// For each zone, it uses only the LAST occurrence of each (abbreviation, IsDST) pair,
// which corresponds to the "current" or most recent offset for that abbreviation.
// This matches PHP's behavior where only current (non-historical) types are included.
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

		// For each zone, keep only the LAST occurrence of each (abbr, isDST) pair
		// to get the current/most-recent offset for that abbreviation.
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
	// Check that the datetime argument is a proper DateTimeInterface object too
	if len(args) > 1 {
		if dtObj, ok2 := args[1].Value().(*phpobj.ZObject); ok2 {
			if err := checkDateTimeInitialized(ctx, dtObj); err != nil {
				return nil, err
			}
		}
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

	// Common timezone abbreviations mapping
	commonAbbrs := map[string]string{
		"CET":  "Europe/Berlin",
		"CEST": "Europe/Berlin",
		"EET":  "Europe/Helsinki",
		"EEST": "Europe/Helsinki",
		"WET":  "Europe/Lisbon",
		"WEST": "Europe/Lisbon",
		"GMT":  "UTC",
		"UTC":  "UTC",
		"EST":  "America/New_York",
		"EDT":  "America/New_York",
		"CST":  "America/Chicago",
		"CDT":  "America/Chicago",
		"MST":  "America/Denver",
		"MDT":  "America/Denver",
		"PST":  "America/Los_Angeles",
		"PDT": "America/Los_Angeles",
		"HST": "Pacific/Honolulu",
		"AKST": "America/Anchorage",
		"AKDT": "America/Anchorage",
		"AST":  "America/Puerto_Rico",
		"IST":  "Asia/Kolkata",
		"JST":  "Asia/Tokyo",
		"KST":  "Asia/Seoul",
		"CST6CDT": "America/Chicago",
		"EST5EDT": "America/New_York",
		"MST7MDT": "America/Denver",
		"PST8PDT": "America/Los_Angeles",
		"AEST": "Australia/Sydney",
		"AEDT": "Australia/Sydney",
		"ACST": "Australia/Adelaide",
		"ACDT": "Australia/Adelaide",
		"AWST": "Australia/Perth",
		"NZST": "Pacific/Auckland",
		"NZDT": "Pacific/Auckland",
		"BST":  "Europe/London",
		"MET":  "MET",
		"MSK":  "Europe/Moscow",
	}

	// Try direct abbreviation lookup
	if abbrStr != "" {
		if tz, ok := commonAbbrs[abbrStr]; ok {
			return phpv.ZString(tz).ZVal(), nil
		}
		// Try as a full timezone name
		if _, err := time.LoadLocation(abbrStr); err == nil {
			return phpv.ZString(abbrStr).ZVal(), nil
		}
	}

	// If abbr is empty and isDST is not explicitly provided, return false
	if abbrStr == "" && !isDST.HasArg() {
		return phpv.ZBool(false).ZVal(), nil
	}

	// If offset is provided and isDST is explicitly specified, search by offset
	if utcOffset.HasArg() && isDST.HasArg() {
		offset := int(utcOffset.Get())
		wantDST := int(isDST.Get()) == 1
		// Use a reference date in January (non-DST for northern hemisphere)
		// and July (DST for northern hemisphere) to check offsets
		refWinter := time.Date(2024, 1, 15, 12, 0, 0, 0, time.UTC)
		refSummer := time.Date(2024, 7, 15, 12, 0, 0, 0, time.UTC)
		// Go through known timezones to find one with matching offset and DST preference
		for _, tzName := range []string{
			"Europe/Paris", "Europe/London", "Europe/Berlin", "Europe/Helsinki",
			"Europe/Moscow", "Europe/Lisbon", "Europe/Rome", "Europe/Madrid",
			"America/New_York", "America/Chicago", "America/Denver",
			"America/Los_Angeles", "America/Anchorage", "Pacific/Honolulu",
			"Asia/Tokyo", "Asia/Shanghai", "Asia/Kolkata", "Asia/Seoul",
			"Australia/Sydney", "Australia/Adelaide", "Australia/Perth",
			"Pacific/Auckland", "UTC",
		} {
			loc, err := time.LoadLocation(tzName)
			if err != nil {
				continue
			}
			var ref time.Time
			if wantDST {
				ref = refSummer
			} else {
				ref = refWinter
			}
			_, tzOffset := ref.In(loc).Zone()
			if tzOffset == offset {
				return phpv.ZString(tzName).ZVal(), nil
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
		// Propagate thrown PHP exceptions (like Error for uninitialized DateTimeZone),
		// but return false for other failures (like invalid date strings).
		if _, isThrow := phpv.UnwrapError(err).(*phperr.PhpThrow); isThrow {
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
		if _, isThrow := phpv.UnwrapError(err).(*phperr.PhpThrow); isThrow {
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
		if err := ctx.Deprecated("date_interval_create_from_date_string(): Passing null to parameter #1 ($datetime) of type string is deprecated"); err != nil {
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
	return phpv.ZBool(false).ZVal(), nil
}

// dateParseHasDate returns true if the input string contains a date component (year-month-day).
var reDateParseHasDate = regexp.MustCompile(`\d{4}[-/]\d{1,2}[-/]\d{1,2}|\d{1,2}[-/]\d{1,2}[-/]\d{2,4}`)

// dateParseHasTime returns true if the input string contains a time component (HH:MM).
var reDateParseHasTime = regexp.MustCompile(`\d{1,2}:\d{2}`)

// dateParseHasFraction returns true if the input has a fractional seconds part (e.g. ":00.5").
var reDateParseHasFraction = regexp.MustCompile(`:\d{2}\.(\d+)`)

func fncDateParse(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	if len(args) < 1 {
		return nil, phpobj.ThrowError(ctx, phpobj.ArgumentCountError, "date_parse() expects exactly 1 argument, 0 given")
	}
	datetime := string(args[0].AsString(ctx))
	result := phpv.NewZArray()
	loc := getTimezone(ctx)
	base := time.Now().In(loc)
	t, stErr := strtotime.StrToTime(datetime, strtotime.InTZ(loc), strtotime.Rel(base))

	// Determine which components are present in the input string
	hasDate := reDateParseHasDate.MatchString(datetime)
	hasTime := reDateParseHasTime.MatchString(datetime)

	// Parse fraction from input
	var fraction phpv.ZVal
	if m := reDateParseHasFraction.FindStringSubmatch(datetime); m != nil {
		f, fErr := strconv.ParseFloat("0."+m[1], 64)
		if fErr == nil {
			fraction = *phpv.ZFloat(f).ZVal()
		} else {
			fraction = *phpv.ZBool(false).ZVal()
		}
	} else if hasTime {
		fraction = *phpv.ZFloat(0).ZVal()
	} else {
		fraction = *phpv.ZBool(false).ZVal()
	}

	if stErr != nil {
		result.OffsetSet(ctx, phpv.ZString("year"), phpv.ZBool(false).ZVal())
		result.OffsetSet(ctx, phpv.ZString("month"), phpv.ZBool(false).ZVal())
		result.OffsetSet(ctx, phpv.ZString("day"), phpv.ZBool(false).ZVal())
		result.OffsetSet(ctx, phpv.ZString("hour"), phpv.ZBool(false).ZVal())
		result.OffsetSet(ctx, phpv.ZString("minute"), phpv.ZBool(false).ZVal())
		result.OffsetSet(ctx, phpv.ZString("second"), phpv.ZBool(false).ZVal())
	} else {
		if hasDate {
			result.OffsetSet(ctx, phpv.ZString("year"), phpv.ZInt(t.Year()).ZVal())
			result.OffsetSet(ctx, phpv.ZString("month"), phpv.ZInt(int(t.Month())).ZVal())
			result.OffsetSet(ctx, phpv.ZString("day"), phpv.ZInt(t.Day()).ZVal())
		} else {
			result.OffsetSet(ctx, phpv.ZString("year"), phpv.ZBool(false).ZVal())
			result.OffsetSet(ctx, phpv.ZString("month"), phpv.ZBool(false).ZVal())
			result.OffsetSet(ctx, phpv.ZString("day"), phpv.ZBool(false).ZVal())
		}
		if hasTime {
			result.OffsetSet(ctx, phpv.ZString("hour"), phpv.ZInt(t.Hour()).ZVal())
			result.OffsetSet(ctx, phpv.ZString("minute"), phpv.ZInt(t.Minute()).ZVal())
			result.OffsetSet(ctx, phpv.ZString("second"), phpv.ZInt(t.Second()).ZVal())
		} else {
			result.OffsetSet(ctx, phpv.ZString("hour"), phpv.ZBool(false).ZVal())
			result.OffsetSet(ctx, phpv.ZString("minute"), phpv.ZBool(false).ZVal())
			result.OffsetSet(ctx, phpv.ZString("second"), phpv.ZBool(false).ZVal())
		}
	}
	result.OffsetSet(ctx, phpv.ZString("fraction"), fraction.ZVal())
	result.OffsetSet(ctx, phpv.ZString("warning_count"), phpv.ZInt(0).ZVal())
	result.OffsetSet(ctx, phpv.ZString("warnings"), phpv.NewZArray().ZVal())
	result.OffsetSet(ctx, phpv.ZString("error_count"), phpv.ZInt(0).ZVal())
	result.OffsetSet(ctx, phpv.ZString("errors"), phpv.NewZArray().ZVal())
	result.OffsetSet(ctx, phpv.ZString("is_localtime"), phpv.ZBool(false).ZVal())
	return result.ZVal(), nil
}

func fncDateParseFromFormat(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	if len(args) < 2 {
		return nil, phpobj.ThrowError(ctx, phpobj.ArgumentCountError, fmt.Sprintf("date_parse_from_format() expects exactly 2 arguments, %d given", len(args)))
	}
	format := string(args[0].AsString(ctx))
	datetime := string(args[1].AsString(ctx))
	result := phpv.NewZArray()
	t, ok := createFromFormatParsed(ctx, format, datetime, time.UTC)
	if !ok {
		result.OffsetSet(ctx, phpv.ZString("year"), phpv.ZBool(false).ZVal())
		result.OffsetSet(ctx, phpv.ZString("month"), phpv.ZBool(false).ZVal())
		result.OffsetSet(ctx, phpv.ZString("day"), phpv.ZBool(false).ZVal())
		result.OffsetSet(ctx, phpv.ZString("hour"), phpv.ZBool(false).ZVal())
		result.OffsetSet(ctx, phpv.ZString("minute"), phpv.ZBool(false).ZVal())
		result.OffsetSet(ctx, phpv.ZString("second"), phpv.ZBool(false).ZVal())
	} else {
		result.OffsetSet(ctx, phpv.ZString("year"), phpv.ZInt(t.Year()).ZVal())
		result.OffsetSet(ctx, phpv.ZString("month"), phpv.ZInt(int(t.Month())).ZVal())
		result.OffsetSet(ctx, phpv.ZString("day"), phpv.ZInt(t.Day()).ZVal())
		result.OffsetSet(ctx, phpv.ZString("hour"), phpv.ZInt(t.Hour()).ZVal())
		result.OffsetSet(ctx, phpv.ZString("minute"), phpv.ZInt(t.Minute()).ZVal())
		result.OffsetSet(ctx, phpv.ZString("second"), phpv.ZInt(t.Second()).ZVal())
	}
	result.OffsetSet(ctx, phpv.ZString("fraction"), phpv.ZFloat(0).ZVal())
	result.OffsetSet(ctx, phpv.ZString("warning_count"), phpv.ZInt(0).ZVal())
	result.OffsetSet(ctx, phpv.ZString("warnings"), phpv.NewZArray().ZVal())
	result.OffsetSet(ctx, phpv.ZString("error_count"), phpv.ZInt(0).ZVal())
	result.OffsetSet(ctx, phpv.ZString("errors"), phpv.NewZArray().ZVal())
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
	result := phpv.NewZArray()
	result.OffsetSet(ctx, phpv.ZString("sec"), phpv.ZInt(now.Unix()).ZVal())
	result.OffsetSet(ctx, phpv.ZString("usec"), phpv.ZInt(int64(now.Nanosecond()/1000)).ZVal())
	result.OffsetSet(ctx, phpv.ZString("minuteswest"), phpv.ZInt(0).ZVal())
	result.OffsetSet(ctx, phpv.ZString("dsttime"), phpv.ZInt(0).ZVal())
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
	result := phpv.NewZArray()
	result.OffsetSet(ctx, phpv.ZString("country_code"), phpv.ZString("??").ZVal())
	result.OffsetSet(ctx, phpv.ZString("latitude"), phpv.ZFloat(0).ZVal())
	result.OffsetSet(ctx, phpv.ZString("longitude"), phpv.ZFloat(0).ZVal())
	result.OffsetSet(ctx, phpv.ZString("comments"), phpv.ZString("").ZVal())
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
	t := time.Unix(timestamp, 0).UTC()
	dayStart := time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC)
	midnightTS := dayStart.Unix()

	result := phpv.NewZArray()

	// Sunrise/sunset: altit = -50/60 degrees, upperLimb = true
	rise, set, transit := calculateSunRiseSetTransit(midnightTS, latitude, longitude, -50.0/60.0, true)
	if math.IsNaN(rise) {
		result.OffsetSet(ctx, phpv.ZString("sunrise"), phpv.ZBool(false).ZVal())
	} else {
		result.OffsetSet(ctx, phpv.ZString("sunrise"), phpv.ZInt(midnightTS+int64(rise*3600)).ZVal())
	}
	if math.IsNaN(set) {
		result.OffsetSet(ctx, phpv.ZString("sunset"), phpv.ZBool(false).ZVal())
	} else {
		result.OffsetSet(ctx, phpv.ZString("sunset"), phpv.ZInt(midnightTS+int64(set*3600)).ZVal())
	}
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
		begin, end, _ := calculateSunRiseSetTransit(midnightTS, latitude, longitude, tw.altit, false)
		if math.IsNaN(begin) {
			result.OffsetSet(ctx, phpv.ZString(tw.beginKey), phpv.ZBool(false).ZVal())
		} else {
			result.OffsetSet(ctx, phpv.ZString(tw.beginKey), phpv.ZInt(midnightTS+int64(begin*3600)).ZVal())
		}
		if math.IsNaN(end) {
			result.OffsetSet(ctx, phpv.ZString(tw.endKey), phpv.ZBool(false).ZVal())
		} else {
			result.OffsetSet(ctx, phpv.ZString(tw.endKey), phpv.ZInt(midnightTS+int64(end*3600)).ZVal())
		}
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
	ctx.Deprecated(fmt.Sprintf("Function %s() is deprecated since 8.1", funcName))
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
	if len(args) > 2 {
		latitude = float64(args[2].AsFloat(ctx))
	}
	if len(args) > 3 {
		longitude = float64(args[3].AsFloat(ctx))
	}
	if len(args) > 4 {
		zenith = float64(args[4].AsFloat(ctx))
	}
	if len(args) > 5 {
		utcOffset = float64(args[5].AsFloat(ctx))
	}
	ut := calculateSunTime(timestamp, latitude, longitude, zenith, isSunrise)
	if math.IsNaN(ut) {
		return phpv.ZBool(false).ZVal(), nil
	}
	if !math.IsNaN(utcOffset) {
		ut += utcOffset
	}
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
	switch returnFormat {
	case 0:
		t := time.Unix(timestamp, 0).UTC()
		dayStart := time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC)
		return phpv.ZInt(dayStart.Unix() + int64(ut*3600)).ZVal(), nil
	case 1:
		hours := int(ut)
		minutes := int((ut - float64(hours)) * 60)
		return phpv.ZString(fmt.Sprintf("%02d:%02d", hours, minutes)).ZVal(), nil
	case 2:
		return phpv.ZFloat(ut).ZVal(), nil
	}
	return phpv.ZBool(false).ZVal(), nil
}
