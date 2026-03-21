package date

import (
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/MagicalTux/goro/core"
	"github.com/MagicalTux/goro/core/phpobj"
	"github.com/MagicalTux/goro/core/phpv"
)

func degreesToRadians(d float64) float64 { return d * math.Pi / 180 }
func radiansToDegrees(r float64) float64 { return r * 180 / math.Pi }

const solarZenithSunrise = 90.833

func calculateSunTime(timestamp int64, latitude, longitude, zenith float64, isSunrise bool) float64 {
	t := time.Unix(timestamp, 0).UTC()
	dayOfYear := float64(t.YearDay())
	lngHour := longitude / 15.0
	var tApprox float64
	if isSunrise {
		tApprox = dayOfYear + (6-lngHour)/24
	} else {
		tApprox = dayOfYear + (18-lngHour)/24
	}
	M := 0.9856*tApprox - 3.289
	L := M + 1.916*math.Sin(degreesToRadians(M)) + 0.020*math.Sin(degreesToRadians(2*M)) + 282.634
	for L < 0 {
		L += 360
	}
	for L >= 360 {
		L -= 360
	}
	RA := radiansToDegrees(math.Atan(0.91764 * math.Tan(degreesToRadians(L))))
	for RA < 0 {
		RA += 360
	}
	for RA >= 360 {
		RA -= 360
	}
	Lquadrant := math.Floor(L/90) * 90
	RAquadrant := math.Floor(RA/90) * 90
	RA = RA + (Lquadrant - RAquadrant)
	RA = RA / 15
	sinDec := 0.39782 * math.Sin(degreesToRadians(L))
	cosDec := math.Cos(math.Asin(sinDec))
	cosH := (math.Cos(degreesToRadians(zenith)) - sinDec*math.Sin(degreesToRadians(latitude))) / (cosDec * math.Cos(degreesToRadians(latitude)))
	if cosH > 1 || cosH < -1 {
		return math.NaN()
	}
	var H float64
	if isSunrise {
		H = 360 - radiansToDegrees(math.Acos(cosH))
	} else {
		H = radiansToDegrees(math.Acos(cosH))
	}
	H = H / 15
	T := H + RA - 0.06571*tApprox - 6.622
	UT := T - lngHour
	for UT < 0 {
		UT += 24
	}
	for UT >= 24 {
		UT -= 24
	}
	return UT
}

// > func bool date_default_timezone_set ( string $timezoneId )
func fncDateDefaultTimezoneSet(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var tz phpv.ZString
	_, err := core.Expand(ctx, args, &tz)
	if err != nil {
		return nil, err
	}

	// Store timezone in global config (stub for now, actual timezone handling TODO)
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
		return nil, ctx.Errorf("date_diff() expects at least 2 parameters, %d given", len(args))
	}

	obj1, ok := args[0].Value().(phpv.ZObject)
	if !ok {
		return nil, ctx.Errorf("date_diff(): Argument #1 must be of type DateTimeInterface, %s given", args[0].GetType())
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

// setTimezoneLoc stores a *time.Location in a DateTimeZone object.
func setTimezoneLoc(obj *phpobj.ZObject, loc *time.Location) {
	obj.Opaque[DateTimeZone] = loc
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
	return nil, fmt.Errorf("unknown timezone: %s", tzName)
}

// datetimezoneConstruct implements DateTimeZone::__construct(string $timezone)
func datetimezoneConstruct(ctx phpv.Context, this *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	if len(args) < 1 {
		return nil, ctx.Errorf("DateTimeZone::__construct() expects exactly 1 argument, 0 given")
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
	return nil, nil
}

// datetimezoneGetName implements DateTimeZone::getName(): string
func datetimezoneGetName(ctx phpv.Context, this *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	loc, ok := getTimezoneLoc(this)
	if !ok {
		return phpv.ZBool(false).ZVal(), nil
	}
	return phpv.ZString(loc.String()).ZVal(), nil
}

// datetimezoneGetOffset implements DateTimeZone::getOffset(DateTimeInterface $datetime): int
func datetimezoneGetOffset(ctx phpv.Context, this *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	if len(args) < 1 {
		return nil, phpobj.ThrowError(ctx, phpobj.TypeError, "DateTimeZone::getOffset() expects exactly 1 argument, 0 given")
	}
	loc, ok := getTimezoneLoc(this)
	if !ok {
		return phpv.ZBool(false).ZVal(), nil
	}
	dtObj, ok := args[0].Value().(*phpobj.ZObject)
	if !ok {
		return nil, phpobj.ThrowError(ctx, phpobj.TypeError, fmt.Sprintf("DateTimeZone::getOffset(): Argument #1 ($datetime) must be of type DateTimeInterface, %s given", args[0].GetType().TypeName()))
	}
	t, ok := getTime(dtObj)
	if !ok {
		return phpv.ZBool(false).ZVal(), nil
	}
	// Get offset at the given datetime in this timezone
	_, offset := t.In(loc).Zone()
	return phpv.ZInt(offset).ZVal(), nil
}

// Common timezone identifiers for listIdentifiers
var commonTimezones = []string{
	"Africa/Abidjan", "Africa/Accra", "Africa/Addis_Ababa", "Africa/Algiers",
	"Africa/Cairo", "Africa/Casablanca", "Africa/Johannesburg", "Africa/Lagos",
	"Africa/Nairobi", "Africa/Tunis",
	"America/Anchorage", "America/Argentina/Buenos_Aires", "America/Bogota",
	"America/Chicago", "America/Denver", "America/Halifax", "America/Lima",
	"America/Los_Angeles", "America/Mexico_City", "America/New_York",
	"America/Phoenix", "America/Santiago", "America/Sao_Paulo", "America/Toronto",
	"America/Vancouver", "America/Winnipeg",
	"Asia/Almaty", "Asia/Baghdad", "Asia/Bangkok", "Asia/Colombo",
	"Asia/Dhaka", "Asia/Dubai", "Asia/Ho_Chi_Minh", "Asia/Hong_Kong",
	"Asia/Irkutsk", "Asia/Jakarta", "Asia/Jerusalem", "Asia/Karachi",
	"Asia/Kolkata", "Asia/Krasnoyarsk", "Asia/Kuala_Lumpur", "Asia/Kuwait",
	"Asia/Manila", "Asia/Novosibirsk", "Asia/Riyadh", "Asia/Seoul",
	"Asia/Shanghai", "Asia/Singapore", "Asia/Taipei", "Asia/Tehran",
	"Asia/Tokyo", "Asia/Vladivostok", "Asia/Yakutsk", "Asia/Yekaterinburg",
	"Atlantic/Azores", "Atlantic/Reykjavik",
	"Australia/Adelaide", "Australia/Brisbane", "Australia/Darwin",
	"Australia/Hobart", "Australia/Melbourne", "Australia/Perth", "Australia/Sydney",
	"Europe/Amsterdam", "Europe/Athens", "Europe/Belgrade", "Europe/Berlin",
	"Europe/Brussels", "Europe/Bucharest", "Europe/Budapest", "Europe/Copenhagen",
	"Europe/Dublin", "Europe/Helsinki", "Europe/Istanbul", "Europe/Kiev",
	"Europe/Lisbon", "Europe/London", "Europe/Madrid", "Europe/Moscow",
	"Europe/Oslo", "Europe/Paris", "Europe/Prague", "Europe/Rome",
	"Europe/Stockholm", "Europe/Vienna", "Europe/Vilnius", "Europe/Warsaw",
	"Europe/Zurich",
	"Indian/Maldives",
	"Pacific/Auckland", "Pacific/Fiji", "Pacific/Guam", "Pacific/Honolulu",
	"UTC",
}

// datetimezoneListIdentifiers implements DateTimeZone::listIdentifiers(): array
func datetimezoneListIdentifiers(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	result := phpv.NewZArray()
	for _, tz := range commonTimezones {
		result.OffsetSet(ctx, nil, phpv.ZString(tz).ZVal())
	}
	return result.ZVal(), nil
}

// datetimezoneListAbbreviations implements DateTimeZone::listAbbreviations(): array
func datetimezoneListAbbreviations(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	// Return a simplified version of timezone abbreviations
	result := phpv.NewZArray()

	abbrevs := map[string][]struct {
		offset int
		tzId   string
	}{
		"utc":  {{0, "UTC"}},
		"gmt":  {{0, "UTC"}},
		"est":  {{-18000, "America/New_York"}},
		"edt":  {{-14400, "America/New_York"}},
		"cst":  {{-21600, "America/Chicago"}},
		"cdt":  {{-18000, "America/Chicago"}},
		"mst":  {{-25200, "America/Denver"}},
		"mdt":  {{-21600, "America/Denver"}},
		"pst":  {{-28800, "America/Los_Angeles"}},
		"pdt":  {{-25200, "America/Los_Angeles"}},
		"cet":  {{3600, "Europe/Paris"}},
		"cest": {{7200, "Europe/Paris"}},
		"eet":  {{7200, "Europe/Athens"}},
		"eest": {{10800, "Europe/Athens"}},
		"jst":  {{32400, "Asia/Tokyo"}},
		"kst":  {{32400, "Asia/Seoul"}},
		"ist":  {{19800, "Asia/Kolkata"}},
		"cst (china)": {{28800, "Asia/Shanghai"}},
		"aest": {{36000, "Australia/Sydney"}},
		"aedt": {{39600, "Australia/Sydney"}},
		"nzst": {{43200, "Pacific/Auckland"}},
		"nzdt": {{46800, "Pacific/Auckland"}},
	}

	for abbr, entries := range abbrevs {
		arr := phpv.NewZArray()
		for _, e := range entries {
			entry := phpv.NewZArray()
			entry.OffsetSet(ctx, phpv.ZString("dst"), phpv.ZBool(false).ZVal())
			entry.OffsetSet(ctx, phpv.ZString("offset"), phpv.ZInt(e.offset).ZVal())
			entry.OffsetSet(ctx, phpv.ZString("timezone_id"), phpv.ZString(e.tzId).ZVal())
			arr.OffsetSet(ctx, nil, entry.ZVal())
		}
		result.OffsetSet(ctx, phpv.ZString(abbr), arr.ZVal())
	}

	return result.ZVal(), nil
}

// > func DateTimeZone timezone_open ( string $timezone )
func fncTimezoneOpen(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	if len(args) < 1 {
		return nil, ctx.Errorf("timezone_open() expects exactly 1 argument, 0 given")
	}
	obj, err := phpobj.NewZObject(ctx, DateTimeZone, args[0])
	if err != nil {
		// timezone_open returns false on failure, unlike the constructor which throws
		return phpv.ZBool(false).ZVal(), nil
	}
	return obj.ZVal(), nil
}

// > func string timezone_name_get ( DateTimeZone $object )
func fncTimezoneNameGet(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	if len(args) < 1 {
		return nil, ctx.Errorf("timezone_name_get() expects exactly 1 argument, 0 given")
	}
	tzObj, ok := args[0].Value().(*phpobj.ZObject)
	if !ok {
		return phpv.ZBool(false).ZVal(), nil
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
		return nil, ctx.Errorf("timezone_offset_get() expects exactly 2 arguments")
	}
	tzObj, ok := args[0].Value().(*phpobj.ZObject)
	if !ok {
		return phpv.ZBool(false).ZVal(), nil
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
		// date_create returns false on failure, unlike the constructor which throws
		return phpv.ZBool(false).ZVal(), nil
	}
	return obj.ZVal(), nil
}

// > func DateTimeImmutable date_create_immutable ([ string $datetime = "now" [, DateTimeZone $timezone = null ]] )
func fncDateCreateImmutable(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	// Pass args to NewZObject so the constructor handles them
	obj, err := phpobj.NewZObject(ctx, DateTimeImmutable, args...)
	if err != nil {
		return phpv.ZBool(false).ZVal(), nil
	}
	return obj.ZVal(), nil
}

// > func string date_format ( DateTimeInterface $object , string $format )
func fncDateFormat(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	if len(args) < 2 {
		return nil, ctx.Errorf("date_format() expects exactly 2 arguments")
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
		return nil, ctx.Errorf("date_timestamp_get() expects exactly 1 argument")
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
		return nil, ctx.Errorf("date_offset_get() expects exactly 1 argument")
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
		return nil, ctx.Errorf("date_modify() expects exactly 2 arguments")
	}
	dtObj, ok := args[0].Value().(*phpobj.ZObject)
	if !ok {
		return phpv.ZBool(false).ZVal(), nil
	}
	return modifyMethod(ctx, dtObj, args[1:])
}

// > func DateTimeZone date_timezone_get ( DateTimeInterface $object )
func fncDateTimezoneGet(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	if len(args) < 1 {
		return nil, ctx.Errorf("date_timezone_get() expects exactly 1 argument")
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
		return nil, ctx.Errorf("date_timezone_set() expects exactly 2 arguments")
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
		return nil, ctx.Errorf("date_date_set() expects exactly 4 arguments")
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
		return nil, ctx.Errorf("date_time_set() expects at least 3 arguments")
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
		return nil, ctx.Errorf("date_timestamp_set() expects exactly 2 arguments")
	}
	dtObj, ok := args[0].Value().(*phpobj.ZObject)
	if !ok {
		return phpv.ZBool(false).ZVal(), nil
	}
	return setTimestampMethod(ctx, dtObj, args[1:])
}

func fncDateAdd(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	if len(args) < 2 {
		return nil, ctx.Errorf("date_add() expects exactly 2 arguments")
	}
	dtObj, ok := args[0].Value().(*phpobj.ZObject)
	if !ok {
		return phpv.ZBool(false).ZVal(), nil
	}
	return addMethod(ctx, dtObj, args[1:])
}

func fncDateSub(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	if len(args) < 2 {
		return nil, ctx.Errorf("date_sub() expects exactly 2 arguments")
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
		return nil, ctx.Errorf("date_interval_create_from_date_string() expects exactly 1 argument")
	}
	return createDateIntervalFromString(ctx, string(args[0].AsString(ctx)))
}

func fncDateIntervalFormat(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	if len(args) < 2 {
		return nil, ctx.Errorf("date_interval_format() expects exactly 2 arguments")
	}
	intObj, ok := args[0].Value().(*phpobj.ZObject)
	if !ok {
		return phpv.ZBool(false).ZVal(), nil
	}
	return dateIntervalFormat(ctx, intObj, args[1:])
}

func fncDateISODateSet(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	if len(args) < 3 {
		return nil, ctx.Errorf("date_isodate_set() expects at least 3 arguments")
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

func fncDateParse(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	if len(args) < 1 {
		return nil, ctx.Errorf("date_parse() expects exactly 1 argument")
	}
	datetime := string(args[0].AsString(ctx))
	result := phpv.NewZArray()
	t, ok := strToTime(datetime, time.Now().UTC())
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

func fncDateParseFromFormat(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	if len(args) < 2 {
		return nil, ctx.Errorf("date_parse_from_format() expects exactly 2 arguments")
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

func fncTimezoneVersionGet(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	return phpv.ZString("0.system").ZVal(), nil
}

func fncTimezoneLocationGet(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	if len(args) < 1 {
		return nil, ctx.Errorf("timezone_location_get() expects exactly 1 argument")
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
		return nil, ctx.Errorf("timezone_transitions_get() expects at least 1 argument")
	}
	tzObj, ok := args[0].Value().(*phpobj.ZObject)
	if !ok {
		return phpv.ZBool(false).ZVal(), nil
	}
	loc, ok := getTimezoneLoc(tzObj)
	if !ok {
		return phpv.ZBool(false).ZVal(), nil
	}
	result := phpv.NewZArray()
	now := time.Now().In(loc)
	name, offset := now.Zone()
	entry := phpv.NewZArray()
	entry.OffsetSet(ctx, phpv.ZString("ts"), phpv.ZInt(0).ZVal())
	entry.OffsetSet(ctx, phpv.ZString("time"), phpv.ZString("1970-01-01T00:00:00+00:00").ZVal())
	entry.OffsetSet(ctx, phpv.ZString("offset"), phpv.ZInt(offset).ZVal())
	_, stdOffset := time.Date(now.Year(), time.January, 1, 0, 0, 0, 0, loc).Zone()
	entry.OffsetSet(ctx, phpv.ZString("isdst"), phpv.ZBool(offset != stdOffset).ZVal())
	entry.OffsetSet(ctx, phpv.ZString("abbr"), phpv.ZString(name).ZVal())
	result.OffsetSet(ctx, nil, entry.ZVal())
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
	result := phpv.NewZArray()
	sunriseUT := calculateSunTime(dayStart.Unix(), latitude, longitude, solarZenithSunrise, true)
	sunsetUT := calculateSunTime(dayStart.Unix(), latitude, longitude, solarZenithSunrise, false)
	if math.IsNaN(sunriseUT) {
		result.OffsetSet(ctx, phpv.ZString("sunrise"), phpv.ZBool(false).ZVal())
	} else {
		result.OffsetSet(ctx, phpv.ZString("sunrise"), phpv.ZInt(dayStart.Unix()+int64(sunriseUT*3600)).ZVal())
	}
	if math.IsNaN(sunsetUT) {
		result.OffsetSet(ctx, phpv.ZString("sunset"), phpv.ZBool(false).ZVal())
	} else {
		result.OffsetSet(ctx, phpv.ZString("sunset"), phpv.ZInt(dayStart.Unix()+int64(sunsetUT*3600)).ZVal())
	}
	transit := (sunriseUT + sunsetUT) / 2
	result.OffsetSet(ctx, phpv.ZString("transit"), phpv.ZInt(dayStart.Unix()+int64(transit*3600)).ZVal())
	for _, tw := range []struct {
		zenith   float64
		beginKey string
		endKey   string
	}{
		{96, "civil_twilight_begin", "civil_twilight_end"},
		{102, "nautical_twilight_begin", "nautical_twilight_end"},
		{108, "astronomical_twilight_begin", "astronomical_twilight_end"},
	} {
		begin := calculateSunTime(dayStart.Unix(), latitude, longitude, tw.zenith, true)
		end := calculateSunTime(dayStart.Unix(), latitude, longitude, tw.zenith, false)
		if math.IsNaN(begin) {
			result.OffsetSet(ctx, phpv.ZString(tw.beginKey), phpv.ZBool(false).ZVal())
		} else {
			result.OffsetSet(ctx, phpv.ZString(tw.beginKey), phpv.ZInt(dayStart.Unix()+int64(begin*3600)).ZVal())
		}
		if math.IsNaN(end) {
			result.OffsetSet(ctx, phpv.ZString(tw.endKey), phpv.ZBool(false).ZVal())
		} else {
			result.OffsetSet(ctx, phpv.ZString(tw.endKey), phpv.ZInt(dayStart.Unix()+int64(end*3600)).ZVal())
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
		return nil, ctx.Errorf("%s() expects at least 1 argument", funcName)
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
