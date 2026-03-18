package date

import (
	"time"

	"github.com/MagicalTux/goro/core"
	"github.com/MagicalTux/goro/core/phpobj"
	"github.com/MagicalTux/goro/core/phpv"
)

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

// datetimezoneConstruct implements DateTimeZone::__construct(string $timezone)
func datetimezoneConstruct(ctx phpv.Context, this *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	if len(args) < 1 {
		return nil, ctx.Errorf("DateTimeZone::__construct() expects exactly 1 argument, 0 given")
	}
	tzName := args[0].AsString(ctx)
	loc, err := time.LoadLocation(string(tzName))
	if err != nil {
		return nil, phpobj.ThrowError(ctx, phpobj.Exception, "DateTimeZone::__construct(): Unknown or bad timezone ("+string(tzName)+")")
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
		return nil, ctx.Errorf("DateTimeZone::getOffset() expects exactly 1 argument, 0 given")
	}
	loc, ok := getTimezoneLoc(this)
	if !ok {
		return phpv.ZBool(false).ZVal(), nil
	}
	dtObj, ok := args[0].Value().(*phpobj.ZObject)
	if !ok {
		return nil, ctx.Errorf("DateTimeZone::getOffset() expects parameter 1 to be DateTimeInterface")
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
	obj, err := phpobj.NewZObject(ctx, DateTime)
	if err != nil {
		return nil, err
	}
	t := parseDateTimeWithTz(ctx, args)
	setTimeVal(obj, t)
	return obj.ZVal(), nil
}

// > func DateTimeImmutable date_create_immutable ([ string $datetime = "now" [, DateTimeZone $timezone = null ]] )
func fncDateCreateImmutable(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	obj, err := phpobj.NewZObject(ctx, DateTimeImmutable)
	if err != nil {
		return nil, err
	}
	t := parseDateTimeWithTz(ctx, args)
	setTimeVal(obj, t)
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
