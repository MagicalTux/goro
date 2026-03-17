package date

import (
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/KarpelesLab/strtotime"
	"github.com/MagicalTux/goro/core/phpobj"
	"github.com/MagicalTux/goro/core/phpv"
)

var DateTimeInterface *phpobj.ZClass
var DateTime *phpobj.ZClass
var DateTimeImmutable *phpobj.ZClass
var DateInterval *phpobj.ZClass
var DateTimeZone *phpobj.ZClass

func parseDateTimeWithTz(ctx phpv.Context, args []*phpv.ZVal) time.Time {
	var t time.Time

	// Determine timezone: if second arg is a DateTimeZone, use it; otherwise use configured tz
	loc := getTimezone(ctx)
	if len(args) > 1 && args[1] != nil && !args[1].IsNull() {
		if tzObj, ok := args[1].Value().(*phpobj.ZObject); ok {
			if tzLoc, ok := getTimezoneLoc(tzObj); ok {
				loc = tzLoc
			}
		}
	}

	if len(args) > 0 && !args[0].IsNull() {
		dateStr := args[0].AsString(ctx)
		if string(dateStr) == "now" || string(dateStr) == "" {
			return time.Now().In(loc)
		}
		// Check if the string is just a timezone abbreviation/name
		if tzLoc, err := time.LoadLocation(string(dateStr)); err == nil {
			return time.Now().In(tzLoc)
		}
		// Use strtotime library first, fall back to custom parser
		base := time.Now().In(loc)
		if parsed, stErr := strtotime.StrToTime(string(dateStr), strtotime.InTZ(loc), strtotime.Rel(base)); stErr == nil {
			// If the parsed time has a different location than the base,
			// the string contained a timezone - keep it.
			// Otherwise, apply the configured/requested timezone.
			if parsed.Location().String() != base.Location().String() {
				return parsed
			}
			return parsed.In(loc)
		}
		// Fallback: try common formats
		for _, layout := range []string{
			"2006-01-02 15:04:05 MST",
			"2006-01-02 15:04:05",
			"2006-01-02",
			time.RFC3339,
		} {
			if parsed, err := time.ParseInLocation(layout, string(dateStr), loc); err == nil {
				t = parsed
				break
			}
		}
		if t.IsZero() {
			t = time.Now().In(loc)
		}
		return t
	}
	return time.Now().In(loc)
}

func getTime(this *phpobj.ZObject) (time.Time, bool) {
	if v, ok := this.Opaque[DateTimeInterface]; ok {
		return v.(time.Time), true
	}
	return time.Time{}, false
}

func setTimeVal(this *phpobj.ZObject, t time.Time) {
	this.Opaque[DateTimeInterface] = t
}

// formatMethod implements DateTime::format(string $format): string
func formatMethod(ctx phpv.Context, this *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	if len(args) < 1 {
		return nil, ctx.Errorf("DateTime::format() expects exactly 1 argument, 0 given")
	}
	t, ok := getTime(this)
	if !ok {
		return phpv.ZBool(false).ZVal(), nil
	}
	format := args[0].AsString(ctx)
	result := phpDateFormat(string(format), t)
	return phpv.ZString(result).ZVal(), nil
}

// getTimestampMethod implements DateTime::getTimestamp(): int
func getTimestampMethod(ctx phpv.Context, this *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	t, ok := getTime(this)
	if !ok {
		return phpv.ZBool(false).ZVal(), nil
	}
	return phpv.ZInt(t.Unix()).ZVal(), nil
}

// modifyMethod implements DateTime::modify(string $modifier): DateTime|false
func modifyMethod(ctx phpv.Context, this *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	if len(args) < 1 {
		return nil, ctx.Errorf("DateTime::modify() expects exactly 1 argument, 0 given")
	}
	t, ok := getTime(this)
	if !ok {
		return phpv.ZBool(false).ZVal(), nil
	}
	modifier := args[0].AsString(ctx)
	newT, stErr := strtotime.StrToTime(string(modifier), strtotime.InTZ(t.Location()), strtotime.Rel(t))
	if stErr != nil {
		// Fallback to custom parser
		var ok bool
		newT, ok = strToTime(string(modifier), t)
		if !ok {
			return phpv.ZBool(false).ZVal(), nil
		}
	}
	setTimeVal(this, newT)
	return this.ZVal(), nil
}

// modifyImmutableMethod implements DateTimeImmutable::modify() - returns new instance
func modifyImmutableMethod(ctx phpv.Context, this *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	if len(args) < 1 {
		return nil, ctx.Errorf("DateTimeImmutable::modify() expects exactly 1 argument, 0 given")
	}
	t, ok := getTime(this)
	if !ok {
		return phpv.ZBool(false).ZVal(), nil
	}
	modifier := args[0].AsString(ctx)
	newT, ok := strToTime(string(modifier), t)
	if !ok {
		return phpv.ZBool(false).ZVal(), nil
	}
	newObj, err := phpobj.NewZObject(ctx, DateTimeImmutable)
	if err != nil {
		return nil, err
	}
	setTimeVal(newObj, newT)
	return newObj.ZVal(), nil
}

// setDateMethod implements DateTime::setDate(int $year, int $month, int $day): DateTime
func setDateMethod(ctx phpv.Context, this *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	if len(args) < 3 {
		return nil, ctx.Errorf("DateTime::setDate() expects exactly 3 arguments")
	}
	t, ok := getTime(this)
	if !ok {
		return phpv.ZBool(false).ZVal(), nil
	}
	year := args[0].AsInt(ctx)
	month := args[1].AsInt(ctx)
	day := args[2].AsInt(ctx)
	h, m, s := t.Clock()
	newT := time.Date(int(year), time.Month(int(month)), int(day), h, m, s, t.Nanosecond(), t.Location())
	setTimeVal(this, newT)
	return this.ZVal(), nil
}

// setDateImmutableMethod implements DateTimeImmutable::setDate() - returns new instance
func setDateImmutableMethod(ctx phpv.Context, this *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	if len(args) < 3 {
		return nil, ctx.Errorf("DateTimeImmutable::setDate() expects exactly 3 arguments")
	}
	t, ok := getTime(this)
	if !ok {
		return phpv.ZBool(false).ZVal(), nil
	}
	year := args[0].AsInt(ctx)
	month := args[1].AsInt(ctx)
	day := args[2].AsInt(ctx)
	h, m, s := t.Clock()
	newT := time.Date(int(year), time.Month(int(month)), int(day), h, m, s, t.Nanosecond(), t.Location())
	newObj, err := phpobj.NewZObject(ctx, DateTimeImmutable)
	if err != nil {
		return nil, err
	}
	setTimeVal(newObj, newT)
	return newObj.ZVal(), nil
}

// setTimeMethod implements DateTime::setTime(int $hour, int $minute, int $second = 0, int $microsecond = 0): DateTime
func setTimeMethod(ctx phpv.Context, this *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	if len(args) < 2 {
		return nil, ctx.Errorf("DateTime::setTime() expects at least 2 arguments")
	}
	t, ok := getTime(this)
	if !ok {
		return phpv.ZBool(false).ZVal(), nil
	}
	hour := args[0].AsInt(ctx)
	minute := args[1].AsInt(ctx)
	sec := phpv.ZInt(0)
	micro := phpv.ZInt(0)
	if len(args) > 2 {
		sec = args[2].AsInt(ctx)
	}
	if len(args) > 3 {
		micro = args[3].AsInt(ctx)
	}
	y, mo, d := t.Date()
	newT := time.Date(y, mo, d, int(hour), int(minute), int(sec), int(micro)*1000, t.Location())
	setTimeVal(this, newT)
	return this.ZVal(), nil
}

// setTimeImmutableMethod implements DateTimeImmutable::setTime() - returns new instance
func setTimeImmutableMethod(ctx phpv.Context, this *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	if len(args) < 2 {
		return nil, ctx.Errorf("DateTimeImmutable::setTime() expects at least 2 arguments")
	}
	t, ok := getTime(this)
	if !ok {
		return phpv.ZBool(false).ZVal(), nil
	}
	hour := args[0].AsInt(ctx)
	minute := args[1].AsInt(ctx)
	sec := phpv.ZInt(0)
	micro := phpv.ZInt(0)
	if len(args) > 2 {
		sec = args[2].AsInt(ctx)
	}
	if len(args) > 3 {
		micro = args[3].AsInt(ctx)
	}
	y, mo, d := t.Date()
	newT := time.Date(y, mo, d, int(hour), int(minute), int(sec), int(micro)*1000, t.Location())
	newObj, err := phpobj.NewZObject(ctx, DateTimeImmutable)
	if err != nil {
		return nil, err
	}
	setTimeVal(newObj, newT)
	return newObj.ZVal(), nil
}

// getOffsetMethod implements DateTime::getOffset(): int
func getOffsetMethod(ctx phpv.Context, this *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	t, ok := getTime(this)
	if !ok {
		return phpv.ZBool(false).ZVal(), nil
	}
	_, offset := t.Zone()
	return phpv.ZInt(offset).ZVal(), nil
}

// setTimezoneMethod implements DateTime::setTimezone(DateTimeZone $timezone): DateTime
func setTimezoneMethod(ctx phpv.Context, this *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	if len(args) < 1 {
		return nil, ctx.Errorf("DateTime::setTimezone() expects exactly 1 argument, 0 given")
	}
	t, ok := getTime(this)
	if !ok {
		return phpv.ZBool(false).ZVal(), nil
	}
	tzObj, ok := args[0].Value().(*phpobj.ZObject)
	if !ok {
		return nil, ctx.Errorf("DateTime::setTimezone() expects parameter 1 to be DateTimeZone")
	}
	loc, ok := getTimezoneLoc(tzObj)
	if !ok {
		return phpv.ZBool(false).ZVal(), nil
	}
	setTimeVal(this, t.In(loc))
	return this.ZVal(), nil
}

// setTimezoneImmutableMethod implements DateTimeImmutable::setTimezone() - returns new instance
func setTimezoneImmutableMethod(ctx phpv.Context, this *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	if len(args) < 1 {
		return nil, ctx.Errorf("DateTimeImmutable::setTimezone() expects exactly 1 argument, 0 given")
	}
	t, ok := getTime(this)
	if !ok {
		return phpv.ZBool(false).ZVal(), nil
	}
	tzObj, ok := args[0].Value().(*phpobj.ZObject)
	if !ok {
		return nil, ctx.Errorf("DateTimeImmutable::setTimezone() expects parameter 1 to be DateTimeZone")
	}
	loc, ok := getTimezoneLoc(tzObj)
	if !ok {
		return phpv.ZBool(false).ZVal(), nil
	}
	newObj, err := phpobj.NewZObject(ctx, DateTimeImmutable)
	if err != nil {
		return nil, err
	}
	setTimeVal(newObj, t.In(loc))
	return newObj.ZVal(), nil
}

// getTimezoneMethod implements DateTime::getTimezone(): DateTimeZone|false
func getTimezoneMethod(ctx phpv.Context, this *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	t, ok := getTime(this)
	if !ok {
		return phpv.ZBool(false).ZVal(), nil
	}
	tzObj, err := phpobj.NewZObject(ctx, DateTimeZone, phpv.ZString(t.Location().String()).ZVal())
	if err != nil {
		return nil, err
	}
	return tzObj.ZVal(), nil
}

// setTimestampMethod implements DateTime::setTimestamp(int $timestamp): DateTime
func setTimestampMethod(ctx phpv.Context, this *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	if len(args) < 1 {
		return nil, ctx.Errorf("DateTime::setTimestamp() expects exactly 1 argument, 0 given")
	}
	t, ok := getTime(this)
	if !ok {
		return phpv.ZBool(false).ZVal(), nil
	}
	ts := args[0].AsInt(ctx)
	newT := time.Unix(int64(ts), 0).In(t.Location())
	setTimeVal(this, newT)
	return this.ZVal(), nil
}

// setTimestampImmutableMethod implements DateTimeImmutable::setTimestamp() - returns new instance
func setTimestampImmutableMethod(ctx phpv.Context, this *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	if len(args) < 1 {
		return nil, ctx.Errorf("DateTimeImmutable::setTimestamp() expects exactly 1 argument, 0 given")
	}
	t, ok := getTime(this)
	if !ok {
		return phpv.ZBool(false).ZVal(), nil
	}
	ts := args[0].AsInt(ctx)
	newT := time.Unix(int64(ts), 0).In(t.Location())
	newObj, err := phpobj.NewZObject(ctx, DateTimeImmutable)
	if err != nil {
		return nil, err
	}
	setTimeVal(newObj, newT)
	return newObj.ZVal(), nil
}

// createFromFormatStatic implements DateTime::createFromFormat(string $format, string $datetime, ?DateTimeZone $timezone = null): DateTime|false
func createFromFormatStatic(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	if len(args) < 2 {
		return nil, ctx.Errorf("DateTime::createFromFormat() expects at least 2 arguments")
	}
	// For now, delegate to parseDateTime-style parsing ignoring the format
	// This is a simplified implementation
	obj, err := phpobj.NewZObject(ctx, DateTime)
	if err != nil {
		return nil, err
	}
	t := parseDateTimeWithTz(ctx, args[1:])
	setTimeVal(obj, t)
	return obj.ZVal(), nil
}

// createFromFormatImmutableStatic implements DateTimeImmutable::createFromFormat()
func createFromFormatImmutableStatic(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	if len(args) < 2 {
		return nil, ctx.Errorf("DateTimeImmutable::createFromFormat() expects at least 2 arguments")
	}
	obj, err := phpobj.NewZObject(ctx, DateTimeImmutable)
	if err != nil {
		return nil, err
	}
	t := parseDateTimeWithTz(ctx, args[1:])
	setTimeVal(obj, t)
	return obj.ZVal(), nil
}

// diffMethod computes the difference between two DateTime-like objects
func diffMethod(ctx phpv.Context, this *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	if len(args) < 1 {
		return nil, ctx.Errorf("DateTime::diff() expects at least 1 parameter, 0 given")
	}

	t1, ok := getTime(this)
	if !ok {
		return phpv.ZBool(false).ZVal(), nil
	}

	targetObj, ok := args[0].Value().(phpv.ZObject)
	if !ok {
		return nil, ctx.Errorf("DateTime::diff() expects parameter 1 to be DateTimeInterface, %s given", args[0].GetType())
	}

	targetZObj, ok := targetObj.(*phpobj.ZObject)
	if !ok {
		return phpv.ZBool(false).ZVal(), nil
	}

	t2, ok := getTime(targetZObj)
	if !ok {
		return phpv.ZBool(false).ZVal(), nil
	}

	// Create DateInterval object
	intervalObj, err := phpobj.NewZObject(ctx, DateInterval)
	if err != nil {
		return nil, err
	}

	// Calculate the difference
	invert := false
	from := t1
	to := t2
	if from.After(to) {
		from, to = to, from
		invert = true
	}

	// Calculate year/month/day differences matching PHP's behavior.
	// PHP computes calendar date diff, then derives hours/minutes/seconds
	// from actual elapsed time minus the calendar portion. This handles
	// DST transitions correctly.
	y1, m1, d1 := from.Date()
	y2, m2, d2 := to.Date()

	years := y2 - y1
	months := int(m2) - int(m1)
	days := d2 - d1

	// Normalize: borrow from months if days < 0
	if days < 0 {
		prevMonth := time.Date(y2, m2, 0, 0, 0, 0, 0, to.Location())
		days += prevMonth.Day()
		months--
	}
	if months < 0 {
		months += 12
		years--
	}

	// Compute remaining hours/minutes/seconds using actual elapsed time.
	// Build a reference point: same wall-clock as 'from' but on the
	// target date, in the 'to' timezone. This ensures DST transitions
	// are accounted for correctly.
	h1, min1, s1 := from.Clock()
	ref := time.Date(y1+years, m1+time.Month(months), d1+days, h1, min1, s1, 0, from.Location())
	remainSec := int(to.Unix() - ref.Unix())

	// If remainSec is negative, we over-counted by one day
	if remainSec < 0 {
		days--
		if days < 0 {
			prevMonth := time.Date(y2, m2, 0, 0, 0, 0, 0, to.Location())
			days += prevMonth.Day()
			months--
			if months < 0 {
				months += 12
				years--
			}
		}
		ref = time.Date(y1+years, m1+time.Month(months), d1+days, h1, min1, s1, 0, to.Location())
		remainSec = int(to.Unix() - ref.Unix())
	}

	hours := remainSec / 3600
	remainSec %= 3600
	minutes := remainSec / 60
	seconds := remainSec % 60

	// Check absolute parameter
	absolute := false
	if len(args) > 1 {
		absolute = bool(args[1].AsBool(ctx))
	}

	// Calculate total days for the 'days' property
	totalDays := int(math.Round(to.Sub(from).Hours() / 24))

	intervalObj.ObjectSet(ctx, phpv.ZString("y"), phpv.ZInt(years).ZVal())
	intervalObj.ObjectSet(ctx, phpv.ZString("m"), phpv.ZInt(months).ZVal())
	intervalObj.ObjectSet(ctx, phpv.ZString("d"), phpv.ZInt(days).ZVal())
	intervalObj.ObjectSet(ctx, phpv.ZString("h"), phpv.ZInt(hours).ZVal())
	intervalObj.ObjectSet(ctx, phpv.ZString("i"), phpv.ZInt(minutes).ZVal())
	intervalObj.ObjectSet(ctx, phpv.ZString("s"), phpv.ZInt(seconds).ZVal())
	intervalObj.ObjectSet(ctx, phpv.ZString("days"), phpv.ZInt(totalDays).ZVal())
	if invert && !absolute {
		intervalObj.ObjectSet(ctx, phpv.ZString("invert"), phpv.ZInt(1).ZVal())
	} else {
		intervalObj.ObjectSet(ctx, phpv.ZString("invert"), phpv.ZInt(0).ZVal())
	}

	return intervalObj.ZVal(), nil
}

// dateTimeDebugInfo implements __debugInfo for DateTime/DateTimeImmutable
func dateTimeDebugInfo(ctx phpv.Context, this *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	t, ok := getTime(this)
	if !ok {
		return phpv.NewZArray().ZVal(), nil
	}
	arr := phpv.NewZArray()
	// Format: "2006-12-12 00:00:00.000000"
	dateStr := fmt.Sprintf("%04d-%02d-%02d %02d:%02d:%02d.%06d",
		t.Year(), int(t.Month()), t.Day(),
		t.Hour(), t.Minute(), t.Second(), t.Nanosecond()/1000)
	if t.Year() < 0 {
		dateStr = fmt.Sprintf("-%04d-%02d-%02d %02d:%02d:%02d.%06d",
			-t.Year(), int(t.Month()), t.Day(),
			t.Hour(), t.Minute(), t.Second(), t.Nanosecond()/1000)
	}
	arr.OffsetSet(ctx, phpv.ZString("date"), phpv.ZString(dateStr).ZVal())

	// timezone_type: 1=offset, 2=abbreviation, 3=identifier
	locName := t.Location().String()
	tzType := 3
	if locName == "UTC" || locName == "" {
		tzType = 3
		locName = "UTC"
	} else if len(locName) <= 6 && !strings.Contains(locName, "/") {
		tzType = 2
	}
	arr.OffsetSet(ctx, phpv.ZString("timezone_type"), phpv.ZInt(tzType).ZVal())
	arr.OffsetSet(ctx, phpv.ZString("timezone"), phpv.ZString(locName).ZVal())

	return arr.ZVal(), nil
}

func init() {
	// DateTimeZone class
	DateTimeZone = &phpobj.ZClass{
		Name:  "DateTimeZone",
		Props: []*phpv.ZClassProp{},
		Methods: map[phpv.ZString]*phpv.ZClassMethod{
			"__construct": {
				Name:      "__construct",
				Modifiers: phpv.ZAttrPublic,
				Method:    phpobj.NativeMethod(datetimezoneConstruct),
			},
			"getname": {
				Name:      "getName",
				Modifiers: phpv.ZAttrPublic,
				Method:    phpobj.NativeMethod(datetimezoneGetName),
			},
			"getoffset": {
				Name:      "getOffset",
				Modifiers: phpv.ZAttrPublic,
				Method:    phpobj.NativeMethod(datetimezoneGetOffset),
			},
			"listidentifiers": {
				Name:      "listIdentifiers",
				Modifiers: phpv.ZAttrPublic | phpv.ZAttrStatic,
				Method:    phpobj.NativeStaticMethod(datetimezoneListIdentifiers),
			},
			"listabbreviations": {
				Name:      "listAbbreviations",
				Modifiers: phpv.ZAttrPublic | phpv.ZAttrStatic,
				Method:    phpobj.NativeStaticMethod(datetimezoneListAbbreviations),
			},
			"__debuginfo": {
				Name:      "__debugInfo",
				Modifiers: phpv.ZAttrPublic,
				Method: phpobj.NativeMethod(func(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
					arr := phpv.NewZArray()
					opaque := o.GetOpaque(DateTimeZone)
					if loc, ok := opaque.(*time.Location); ok && loc != nil {
						name := loc.String()
						// Determine timezone_type: 3=identifier, 2=abbreviation, 1=offset
						tzType := 3
						if len(name) <= 5 && name != "Local" {
							tzType = 2
						}
						arr.OffsetSet(ctx, phpv.ZString("timezone_type"), phpv.ZInt(tzType).ZVal())
						arr.OffsetSet(ctx, phpv.ZString("timezone"), phpv.ZString(name).ZVal())
					}
					return arr.ZVal(), nil
				}),
			},
		},
	}

	// DateTimeInterface - internal-only interface
	DateTimeInterface = &phpobj.ZClass{
		Name:         "DateTimeInterface",
		Type:         phpv.ZClassTypeInterface,
		InternalOnly: true,
	}

	// DateInterval class
	DateInterval = &phpobj.ZClass{
		Name: "DateInterval",
		Props: []*phpv.ZClassProp{
			{VarName: "y", Default: phpv.ZInt(0).ZVal(), Modifiers: phpv.ZAttrPublic},
			{VarName: "m", Default: phpv.ZInt(0).ZVal(), Modifiers: phpv.ZAttrPublic},
			{VarName: "d", Default: phpv.ZInt(0).ZVal(), Modifiers: phpv.ZAttrPublic},
			{VarName: "h", Default: phpv.ZInt(0).ZVal(), Modifiers: phpv.ZAttrPublic},
			{VarName: "i", Default: phpv.ZInt(0).ZVal(), Modifiers: phpv.ZAttrPublic},
			{VarName: "s", Default: phpv.ZInt(0).ZVal(), Modifiers: phpv.ZAttrPublic},
			{VarName: "f", Default: phpv.ZFloat(0).ZVal(), Modifiers: phpv.ZAttrPublic},
			{VarName: "invert", Default: phpv.ZInt(0).ZVal(), Modifiers: phpv.ZAttrPublic},
			{VarName: "days", Default: phpv.ZBool(false).ZVal(), Modifiers: phpv.ZAttrPublic},
		},
		Methods: map[phpv.ZString]*phpv.ZClassMethod{
			"__construct": {Name: "__construct", Method: phpobj.NativeMethod(dateIntervalConstruct)},
			"format":      {Name: "format", Method: phpobj.NativeMethod(dateIntervalFormat)},
		},
	}

	// DateTime class
	DateTime = &phpobj.ZClass{
		Name:            "DateTime",
		Implementations: []*phpobj.ZClass{DateTimeInterface},
		Props:           []*phpv.ZClassProp{},
		Const: map[phpv.ZString]*phpv.ZClassConst{
			"ATOM":              {Value: phpv.ZString("Y-m-d\\TH:i:sP")},
			"COOKIE":            {Value: phpv.ZString("l, d-M-Y H:i:s T")},
			"ISO8601":           {Value: phpv.ZString("Y-m-d\\TH:i:sO")},
			"ISO8601_EXPANDED":  {Value: phpv.ZString("X-m-d\\TH:i:sP")},
			"RFC822":            {Value: phpv.ZString("D, d M y H:i:s O")},
			"RFC850":            {Value: phpv.ZString("l, d-M-y H:i:s T")},
			"RFC1036":           {Value: phpv.ZString("D, d M y H:i:s O")},
			"RFC1123":           {Value: phpv.ZString("D, d M Y H:i:s O")},
			"RFC7231":           {Value: phpv.ZString("D, d M Y H:i:s \\G\\M\\T")},
			"RFC2822":           {Value: phpv.ZString("D, d M Y H:i:s O")},
			"RFC3339":           {Value: phpv.ZString("Y-m-d\\TH:i:sP")},
			"RFC3339_EXTENDED":  {Value: phpv.ZString("Y-m-d\\TH:i:s.vP")},
			"RSS":               {Value: phpv.ZString("D, d M Y H:i:s O")},
			"W3C":               {Value: phpv.ZString("Y-m-d\\TH:i:sP")},
		},
		Methods: map[phpv.ZString]*phpv.ZClassMethod{
			"__construct": {
				Name:      "__construct",
				Modifiers: phpv.ZAttrPublic,
				Method: phpobj.NativeMethod(func(ctx phpv.Context, this *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
					t := parseDateTimeWithTz(ctx, args)
					setTimeVal(this, t)
					return nil, nil
				}),
			},
			"format": {
				Name:      "format",
				Modifiers: phpv.ZAttrPublic,
				Method:    phpobj.NativeMethod(formatMethod),
			},
			"gettimestamp": {
				Name:      "getTimestamp",
				Modifiers: phpv.ZAttrPublic,
				Method:    phpobj.NativeMethod(getTimestampMethod),
			},
			"modify": {
				Name:      "modify",
				Modifiers: phpv.ZAttrPublic,
				Method:    phpobj.NativeMethod(modifyMethod),
			},
			"setdate": {
				Name:      "setDate",
				Modifiers: phpv.ZAttrPublic,
				Method:    phpobj.NativeMethod(setDateMethod),
			},
			"settime": {
				Name:      "setTime",
				Modifiers: phpv.ZAttrPublic,
				Method:    phpobj.NativeMethod(setTimeMethod),
			},
			"diff": {
				Name:      "diff",
				Modifiers: phpv.ZAttrPublic,
				Method:    phpobj.NativeMethod(diffMethod),
			},
			"getoffset": {
				Name:      "getOffset",
				Modifiers: phpv.ZAttrPublic,
				Method:    phpobj.NativeMethod(getOffsetMethod),
			},
			"settimezone": {
				Name:      "setTimezone",
				Modifiers: phpv.ZAttrPublic,
				Method:    phpobj.NativeMethod(setTimezoneMethod),
			},
			"gettimezone": {
				Name:      "getTimezone",
				Modifiers: phpv.ZAttrPublic,
				Method:    phpobj.NativeMethod(getTimezoneMethod),
			},
			"settimestamp": {
				Name:      "setTimestamp",
				Modifiers: phpv.ZAttrPublic,
				Method:    phpobj.NativeMethod(setTimestampMethod),
			},
			"createfromformat": {
				Name:      "createFromFormat",
				Modifiers: phpv.ZAttrPublic | phpv.ZAttrStatic,
				Method:    phpobj.NativeStaticMethod(createFromFormatStatic),
			},
			"__debuginfo": {
				Name:      "__debugInfo",
				Modifiers: phpv.ZAttrPublic,
				Method:    phpobj.NativeMethod(dateTimeDebugInfo),
			},
		},
	}

	// DateTimeImmutable class
	DateTimeImmutable = &phpobj.ZClass{
		Name:            "DateTimeImmutable",
		Implementations: []*phpobj.ZClass{DateTimeInterface},
		Props:           []*phpv.ZClassProp{},
		Methods: map[phpv.ZString]*phpv.ZClassMethod{
			"__construct": {
				Name:      "__construct",
				Modifiers: phpv.ZAttrPublic,
				Method: phpobj.NativeMethod(func(ctx phpv.Context, this *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
					t := parseDateTimeWithTz(ctx, args)
					setTimeVal(this, t)
					return nil, nil
				}),
			},
			"format": {
				Name:      "format",
				Modifiers: phpv.ZAttrPublic,
				Method:    phpobj.NativeMethod(formatMethod),
			},
			"gettimestamp": {
				Name:      "getTimestamp",
				Modifiers: phpv.ZAttrPublic,
				Method:    phpobj.NativeMethod(getTimestampMethod),
			},
			"modify": {
				Name:      "modify",
				Modifiers: phpv.ZAttrPublic,
				Method:    phpobj.NativeMethod(modifyImmutableMethod),
			},
			"setdate": {
				Name:      "setDate",
				Modifiers: phpv.ZAttrPublic,
				Method:    phpobj.NativeMethod(setDateImmutableMethod),
			},
			"settime": {
				Name:      "setTime",
				Modifiers: phpv.ZAttrPublic,
				Method:    phpobj.NativeMethod(setTimeImmutableMethod),
			},
			"diff": {
				Name:      "diff",
				Modifiers: phpv.ZAttrPublic,
				Method:    phpobj.NativeMethod(diffMethod),
			},
			"getoffset": {
				Name:      "getOffset",
				Modifiers: phpv.ZAttrPublic,
				Method:    phpobj.NativeMethod(getOffsetMethod),
			},
			"settimezone": {
				Name:      "setTimezone",
				Modifiers: phpv.ZAttrPublic,
				Method:    phpobj.NativeMethod(setTimezoneImmutableMethod),
			},
			"gettimezone": {
				Name:      "getTimezone",
				Modifiers: phpv.ZAttrPublic,
				Method:    phpobj.NativeMethod(getTimezoneMethod),
			},
			"settimestamp": {
				Name:      "setTimestamp",
				Modifiers: phpv.ZAttrPublic,
				Method:    phpobj.NativeMethod(setTimestampImmutableMethod),
			},
			"createfromformat": {
				Name:      "createFromFormat",
				Modifiers: phpv.ZAttrPublic | phpv.ZAttrStatic,
				Method:    phpobj.NativeStaticMethod(createFromFormatImmutableStatic),
			},
			"__debuginfo": {
				Name:      "__debugInfo",
				Modifiers: phpv.ZAttrPublic,
				Method:    phpobj.NativeMethod(dateTimeDebugInfo),
			},
		},
	}
}

func dateIntervalConstruct(ctx phpv.Context, this *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	if len(args) < 1 {
		// When called internally (e.g. from diff()) with no args, just return with defaults
		return nil, nil
	}
	spec := string(args[0].AsString(ctx))
	// Parse ISO 8601 duration: P1Y2M3DT4H5M6S
	if len(spec) < 2 || spec[0] != 'P' {
		return nil, phpobj.ThrowError(ctx, phpobj.Error, "DateInterval::__construct(): Unknown or bad format ("+spec+")")
	}
	// Simple parser for PnYnMnDTnHnMnS
	inTime := false
	num := ""
	for i := 1; i < len(spec); i++ {
		c := spec[i]
		if c >= '0' && c <= '9' {
			num += string(c)
			continue
		}
		n := 0
		if num != "" {
			fmt.Sscanf(num, "%d", &n)
			num = ""
		}
		switch c {
		case 'Y':
			this.HashTable().SetString("y", phpv.ZInt(n).ZVal())
		case 'M':
			if inTime {
				this.HashTable().SetString("i", phpv.ZInt(n).ZVal())
			} else {
				this.HashTable().SetString("m", phpv.ZInt(n).ZVal())
			}
		case 'D':
			this.HashTable().SetString("d", phpv.ZInt(n).ZVal())
		case 'T':
			inTime = true
		case 'H':
			this.HashTable().SetString("h", phpv.ZInt(n).ZVal())
		case 'S':
			this.HashTable().SetString("s", phpv.ZInt(n).ZVal())
		}
	}
	return nil, nil
}

func dateIntervalFormat(ctx phpv.Context, this *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	if len(args) < 1 {
		return nil, ctx.Errorf("DateInterval::format() expects exactly 1 argument")
	}
	format := string(args[0].AsString(ctx))
	ht := this.HashTable()
	
	result := ""
	for i := 0; i < len(format); i++ {
		if format[i] == '%' && i+1 < len(format) {
			i++
			switch format[i] {
			case 'Y':
				result += fmt.Sprintf("%02d", ht.GetString("y").AsInt(ctx))
			case 'y':
				result += fmt.Sprintf("%d", ht.GetString("y").AsInt(ctx))
			case 'M':
				result += fmt.Sprintf("%02d", ht.GetString("m").AsInt(ctx))
			case 'm':
				result += fmt.Sprintf("%d", ht.GetString("m").AsInt(ctx))
			case 'D':
				result += fmt.Sprintf("%02d", ht.GetString("d").AsInt(ctx))
			case 'd':
				result += fmt.Sprintf("%d", ht.GetString("d").AsInt(ctx))
			case 'H':
				result += fmt.Sprintf("%02d", ht.GetString("h").AsInt(ctx))
			case 'h':
				result += fmt.Sprintf("%d", ht.GetString("h").AsInt(ctx))
			case 'I':
				result += fmt.Sprintf("%02d", ht.GetString("i").AsInt(ctx))
			case 'i':
				result += fmt.Sprintf("%d", ht.GetString("i").AsInt(ctx))
			case 'S':
				result += fmt.Sprintf("%02d", ht.GetString("s").AsInt(ctx))
			case 's':
				result += fmt.Sprintf("%d", ht.GetString("s").AsInt(ctx))
			case 'R':
				if ht.GetString("invert").AsInt(ctx) != 0 {
					result += "-"
				} else {
					result += "+"
				}
			case 'r':
				if ht.GetString("invert").AsInt(ctx) != 0 {
					result += "-"
				}
			case 'a':
				days := ht.GetString("days")
				if days.GetType() == phpv.ZtBool {
					result += "(unknown)"
				} else {
					result += fmt.Sprintf("%d", days.AsInt(ctx))
				}
			case '%':
				result += "%"
			default:
				result += "%" + string(format[i])
			}
		} else {
			result += string(format[i])
		}
	}
	return phpv.ZStr(result), nil
}
