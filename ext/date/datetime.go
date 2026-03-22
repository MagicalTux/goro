package date

import (
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/KarpelesLab/strtotime"
	"github.com/MagicalTux/goro/core/logopt"
	"github.com/MagicalTux/goro/core/phpobj"
	"github.com/MagicalTux/goro/core/phpv"
)

var DateTimeInterface *phpobj.ZClass
var DateTime *phpobj.ZClass
var DateTimeImmutable *phpobj.ZClass
var DateInterval *phpobj.ZClass
var DateTimeZone *phpobj.ZClass
var DatePeriod *phpobj.ZClass

// isDateTimeInitialized checks if a DateTime/DateTimeImmutable object has been properly initialized.
// When a subclass overrides __construct() without calling parent::__construct(), the Opaque map
// won't contain the DateTimeInterface key.
func isDateTimeInitialized(obj *phpobj.ZObject) bool {
	_, ok := obj.Opaque[DateTimeInterface]
	return ok
}

// checkDateTimeInitialized throws DateObjectError if the object is not initialized.
// This is used for DateTime/DateTimeImmutable objects.
func checkDateTimeInitialized(ctx phpv.Context, obj *phpobj.ZObject) error {
	if isDateTimeInitialized(obj) {
		return nil
	}
	className := obj.GetClass().GetName()
	// Determine the base class (DateTime or DateTimeImmutable)
	baseClass := "DateTimeImmutable"
	if obj.Class.InstanceOf(DateTime) && !obj.Class.InstanceOf(DateTimeImmutable) {
		baseClass = "DateTime"
	}
	if string(className) == baseClass {
		return phpobj.ThrowError(ctx, DateObjectError,
			fmt.Sprintf("Object of type %s has not been correctly initialized by calling parent::__construct() in its constructor", className))
	}
	return phpobj.ThrowError(ctx, DateObjectError,
		fmt.Sprintf("Object of type %s (inheriting %s) has not been correctly initialized by calling parent::__construct() in its constructor", className, baseClass))
}

// checkDateIntervalInitialized throws DateObjectError if the DateInterval object is not initialized.
func checkDateIntervalInitialized(ctx phpv.Context, obj *phpobj.ZObject) error {
	// A DateInterval is initialized if its constructor was called.
	// We check if the class is a subclass and if the object's properties look uninitialized.
	// In PHP, DateInterval stores internal data; we check via a special opaque key.
	if _, ok := obj.Opaque[DateInterval]; ok {
		return nil
	}
	// If the class is exactly DateInterval (not a subclass), it's always initialized
	// because our constructor is always called for direct instantiation.
	if string(obj.GetClass().GetName()) == "DateInterval" {
		return nil
	}
	className := obj.GetClass().GetName()
	return phpobj.ThrowError(ctx, DateObjectError,
		fmt.Sprintf("Object of type %s (inheriting DateInterval) has not been correctly initialized by calling parent::__construct() in its constructor", className))
}

// checkDatePeriodInitialized throws DateObjectError if the DatePeriod object is not initialized.
func checkDatePeriodInitialized(ctx phpv.Context, obj *phpobj.ZObject) error {
	if _, ok := obj.Opaque[DatePeriod]; ok {
		return nil
	}
	// Check if start has been set (indicates constructor was called)
	startVal := obj.HashTable().GetString("start")
	if startVal != nil && !startVal.IsNull() {
		return nil
	}
	className := obj.GetClass().GetName()
	if string(className) == "DatePeriod" {
		return phpobj.ThrowError(ctx, DateObjectError,
			fmt.Sprintf("Object of type DatePeriod has not been correctly initialized by calling parent::__construct() in its constructor"))
	}
	return phpobj.ThrowError(ctx, DateObjectError,
		fmt.Sprintf("Object of type %s (inheriting DatePeriod) has not been correctly initialized by calling parent::__construct() in its constructor", className))
}

// checkDateTimeZoneInitialized checks if a DateTimeZone has been properly initialized.
func checkDateTimeZoneInitialized(ctx phpv.Context, obj *phpobj.ZObject) error {
	if _, ok := obj.Opaque[DateTimeZone]; ok {
		return nil
	}
	return phpobj.ThrowError(ctx, DateObjectError,
		"Trying to compare uninitialized DateTimeZone objects")
}

// getCalledClassForStatic gets the late-static-binding class from context, or returns the fallback.
func getCalledClassForStatic(ctx phpv.Context, fallback *phpobj.ZClass) *phpobj.ZClass {
	parent := ctx.Parent(1)
	if parent != nil {
		if cc, ok := parent.(interface{ CalledClass() phpv.ZClass }); ok {
			if called := cc.CalledClass(); called != nil {
				if zc, ok := called.(*phpobj.ZClass); ok {
					return zc
				}
			}
		}
	}
	return fallback
}

func parseDateTimeWithTz(ctx phpv.Context, args []*phpv.ZVal) (time.Time, error) {
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
			return time.Now().In(loc), nil
		}

		// Handle @timestamp - PHP always uses UTC (+00:00) for these
		s := strings.TrimSpace(string(dateStr))
		if len(s) > 0 && s[0] == '@' {
			ts, err := fmt.Sscanf(s[1:], "%d", new(int64))
			if err == nil && ts == 1 {
				var tsVal int64
				fmt.Sscanf(s[1:], "%d", &tsVal)
				return time.Unix(tsVal, 0).In(time.FixedZone("+00:00", 0)), nil
			}
		}

		// Try our custom parser first - it handles timezone abbreviations correctly
		// (the strtotime library converts abbreviations to full timezone names)
		base := time.Now().In(loc)
		if parsed, ok := strToTime(s, base); ok {
			return parsed, nil
		}
		// Use strtotime library for relative dates and complex formats
		if parsed, stErr := strtotime.StrToTime(s, strtotime.InTZ(loc), strtotime.Rel(base)); stErr == nil {
			// If the parsed time has a different location than the base,
			// the string contained a timezone - keep it.
			// Otherwise, apply the configured/requested timezone.
			if parsed.Location().String() != base.Location().String() {
				return parsed, nil
			}
			return parsed.In(loc), nil
		}
		// Last resort: try Go's built-in formats
		for _, layout := range []string{
			"2006-01-02 15:04:05 MST",
			"2006-01-02 15:04:05",
			"2006-01-02",
			time.RFC3339,
		} {
			if parsed, err := time.ParseInLocation(layout, s, loc); err == nil {
				t = parsed
				break
			}
		}
		if t.IsZero() {
			// Parse failed: throw DateMalformedStringException (PHP 8.3+)
			pos := 0
			ch := ""
			for i, c := range s {
				if c < '0' || c > '9' {
					if c != '-' || i > 4 {
						pos = i
						ch = string(c)
						break
					}
				}
			}
			if ch == "" && len(s) > 0 {
				pos = len(s) - 1
				ch = string(s[pos])
			}
			msg := fmt.Sprintf("Failed to parse time string (%s) at position %d (%s): Unexpected character", s, pos, ch)
			return t, phpobj.ThrowError(ctx, DateMalformedStringException, msg)
		}
		return t, nil
	}
	return time.Now().In(loc), nil
}

func getTime(this *phpobj.ZObject) (time.Time, bool) {
	if v, ok := this.Opaque[DateTimeInterface]; ok {
		return v.(time.Time), true
	}
	return time.Time{}, false
}

// getTimeFromObj extracts a time.Time from any ZObject that implements DateTimeInterface
func getTimeFromObj(obj phpv.ZObject) (time.Time, bool) {
	if zo, ok := obj.(*phpobj.ZObject); ok {
		return getTime(zo)
	}
	return time.Time{}, false
}

func setTimeVal(this *phpobj.ZObject, t time.Time) {
	this.Opaque[DateTimeInterface] = t
	// Update hash table properties for var_export/serialization
	dateStr := fmt.Sprintf("%04d-%02d-%02d %02d:%02d:%02d.%06d",
		t.Year(), int(t.Month()), t.Day(),
		t.Hour(), t.Minute(), t.Second(), t.Nanosecond()/1000)
	if t.Year() < 0 {
		dateStr = fmt.Sprintf("-%04d-%02d-%02d %02d:%02d:%02d.%06d",
			-t.Year(), int(t.Month()), t.Day(),
			t.Hour(), t.Minute(), t.Second(), t.Nanosecond()/1000)
	}
	this.HashTable().SetString("date", phpv.ZString(dateStr).ZVal())

	locName := t.Location().String()
	tzType := 3
	if locName == "" {
		// Empty location name from Go's time.Parse - derive from offset
		_, offset := t.Zone()
		if offset == 0 {
			locName = "UTC"
		} else {
			sign := "+"
			absOffset := offset
			if offset < 0 {
				sign = "-"
				absOffset = -offset
			}
			hours := absOffset / 3600
			mins := (absOffset % 3600) / 60
			locName = fmt.Sprintf("%s%02d:%02d", sign, hours, mins)
			tzType = 1
		}
	} else if locName == "UTC" {
		// UTC is type 3 identifier
	} else if len(locName) > 0 && (locName[0] == '+' || locName[0] == '-') {
		// Fixed offset timezone like "+05:00"
		tzType = 1
	} else if len(locName) <= 6 && !strings.Contains(locName, "/") {
		tzType = 2
	}
	this.HashTable().SetString("timezone_type", phpv.ZInt(tzType).ZVal())
	this.HashTable().SetString("timezone", phpv.ZString(locName).ZVal())
}

// formatMethod implements DateTime::format(string $format): string
func formatMethod(ctx phpv.Context, this *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	if len(args) < 1 {
		return nil, ctx.Errorf("DateTime::format() expects exactly 1 argument, 0 given")
	}
	if err := checkDateTimeInitialized(ctx, this); err != nil {
		return nil, err
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
	if err := checkDateTimeInitialized(ctx, this); err != nil {
		return nil, err
	}
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
	if err := checkDateTimeInitialized(ctx, this); err != nil {
		return nil, err
	}
	t, ok := getTime(this)
	if !ok {
		return phpv.ZBool(false).ZVal(), nil
	}
	modifier := args[0].AsString(ctx)
	if len(modifier) == 0 {
		return nil, phpobj.ThrowError(ctx, DateMalformedStringException, "DateTime::modify(): Failed to parse time string () at position 0 ( ): Empty string")
	}
	newT, stErr := strtotime.StrToTime(string(modifier), strtotime.InTZ(t.Location()), strtotime.Rel(t))
	if stErr != nil {
		// Fallback to custom parser
		var ok bool
		newT, ok = strToTime(string(modifier), t)
		if !ok {
			return nil, phpobj.ThrowError(ctx, DateMalformedStringException, fmt.Sprintf("DateTime::modify(): Failed to parse time string (%s) at position 0 (%s): Unexpected character", modifier, string(modifier[0:1])))
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
	if err := checkDateTimeInitialized(ctx, this); err != nil {
		return nil, err
	}
	t, ok := getTime(this)
	if !ok {
		return phpv.ZBool(false).ZVal(), nil
	}
	modifier := args[0].AsString(ctx)
	if len(modifier) == 0 {
		return nil, phpobj.ThrowError(ctx, DateMalformedStringException, "DateTimeImmutable::modify(): Failed to parse time string () at position 0 ( ): Empty string")
	}
	newT, stErr := strtotime.StrToTime(string(modifier), strtotime.InTZ(t.Location()), strtotime.Rel(t))
	if stErr != nil {
		var ok bool
		newT, ok = strToTime(string(modifier), t)
		if !ok {
			return nil, phpobj.ThrowError(ctx, DateMalformedStringException, fmt.Sprintf("DateTimeImmutable::modify(): Failed to parse time string (%s) at position 0 (%s): Unexpected character", modifier, string(modifier[0:1])))
		}
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
	if err := checkDateTimeInitialized(ctx, this); err != nil {
		return nil, err
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
	if err := checkDateTimeInitialized(ctx, this); err != nil {
		return nil, err
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
	if err := checkDateTimeInitialized(ctx, this); err != nil {
		return nil, err
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
	if err := checkDateTimeInitialized(ctx, this); err != nil {
		return nil, err
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
	if err := checkDateTimeInitialized(ctx, this); err != nil {
		return nil, err
	}
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
	if err := checkDateTimeInitialized(ctx, this); err != nil {
		return nil, err
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
	if err := checkDateTimeInitialized(ctx, this); err != nil {
		return nil, err
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
	if err := checkDateTimeInitialized(ctx, this); err != nil {
		return nil, err
	}
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
	if err := checkDateTimeInitialized(ctx, this); err != nil {
		return nil, err
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
	if err := checkDateTimeInitialized(ctx, this); err != nil {
		return nil, err
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

// createFromFormatStaticFor creates a createFromFormat static method for the given class
func createFromFormatStaticFor(targetClass *phpobj.ZClass) func(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	return func(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
		if len(args) < 2 {
			return nil, ctx.Errorf("%s::createFromFormat() expects at least 2 arguments", targetClass.Name)
		}

		format := string(args[0].AsString(ctx))
		datetime := string(args[1].AsString(ctx))

		// Reject null bytes in format or datetime (PHP behavior)
		for _, c := range format {
			if c == 0 {
				return phpv.ZBool(false).ZVal(), nil
			}
		}
		for _, c := range datetime {
			if c == 0 {
				return phpv.ZBool(false).ZVal(), nil
			}
		}

		// Determine timezone
		loc := getTimezone(ctx)
		if len(args) > 2 && args[2] != nil && !args[2].IsNull() {
			if tzObj, ok := args[2].Value().(*phpobj.ZObject); ok {
				if tzLoc, ok := getTimezoneLoc(tzObj); ok {
					loc = tzLoc
				}
			}
		}

		t, ok := createFromFormatParsed(ctx, format, datetime, loc)
		if !ok {
			return phpv.ZBool(false).ZVal(), nil
		}

		obj, err := phpobj.NewZObject(ctx, targetClass)
		if err != nil {
			return nil, err
		}
		setTimeVal(obj, t)
		return obj.ZVal(), nil
	}
}

// diffMethod computes the difference between two DateTime-like objects
func diffMethod(ctx phpv.Context, this *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	if len(args) < 1 {
		return nil, ctx.Errorf("DateTime::diff() expects at least 1 parameter, 0 given")
	}

	if err := checkDateTimeInitialized(ctx, this); err != nil {
		return nil, err
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

	if err := checkDateTimeInitialized(ctx, targetZObj); err != nil {
		return nil, err
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
	// PHP calculates this as the number of full days between the two dates.
	// Use Unix timestamps to handle massive date ranges (Duration is limited to ~292 years).
	fromUTC := time.Date(from.Year(), from.Month(), from.Day(), from.Hour(), from.Minute(), from.Second(), 0, time.UTC)
	toUTC := time.Date(to.Year(), to.Month(), to.Day(), to.Hour(), to.Minute(), to.Second(), 0, time.UTC)
	totalDays := int((toUTC.Unix() - fromUTC.Unix()) / 86400)

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

// addIntervalToTime adds a DateInterval to a time.Time and returns the result
func addIntervalToTime(ctx phpv.Context, t time.Time, intervalObj *phpobj.ZObject, subtract bool) time.Time {
	ht := intervalObj.HashTable()
	years := int(ht.GetString("y").AsInt(ctx))
	months := int(ht.GetString("m").AsInt(ctx))
	days := int(ht.GetString("d").AsInt(ctx))
	hours := int(ht.GetString("h").AsInt(ctx))
	minutes := int(ht.GetString("i").AsInt(ctx))
	seconds := int(ht.GetString("s").AsInt(ctx))
	invert := int(ht.GetString("invert").AsInt(ctx)) != 0

	// Get fractional seconds
	fVal := ht.GetString("f")
	var microseconds int
	if fVal != nil && !fVal.IsNull() {
		microseconds = int(fVal.AsFloat(ctx) * 1000000)
	}

	// If invert is set, flip the direction
	if invert {
		subtract = !subtract
	}

	if subtract {
		years = -years
		months = -months
		days = -days
		hours = -hours
		minutes = -minutes
		seconds = -seconds
		microseconds = -microseconds
	}

	// Apply date part first (years, months, days)
	t = t.AddDate(years, months, days)
	// Then apply time part
	t = t.Add(time.Duration(hours)*time.Hour +
		time.Duration(minutes)*time.Minute +
		time.Duration(seconds)*time.Second +
		time.Duration(microseconds)*time.Microsecond)
	return t
}

// addMethod implements DateTime::add(DateInterval $interval): DateTime
func addMethod(ctx phpv.Context, this *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	if len(args) < 1 {
		return nil, ctx.Errorf("DateTime::add() expects exactly 1 argument, 0 given")
	}
	if err := checkDateTimeInitialized(ctx, this); err != nil {
		return nil, err
	}
	t, ok := getTime(this)
	if !ok {
		return phpv.ZBool(false).ZVal(), nil
	}
	intervalObj, ok := args[0].Value().(*phpobj.ZObject)
	if !ok {
		return nil, ctx.Errorf("DateTime::add() expects parameter 1 to be DateInterval")
	}
	newT := addIntervalToTime(ctx, t, intervalObj, false)
	setTimeVal(this, newT)
	return this.ZVal(), nil
}

// addImmutableMethod implements DateTimeImmutable::add() - returns new instance
func addImmutableMethod(ctx phpv.Context, this *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	if len(args) < 1 {
		return nil, ctx.Errorf("DateTimeImmutable::add() expects exactly 1 argument, 0 given")
	}
	if err := checkDateTimeInitialized(ctx, this); err != nil {
		return nil, err
	}
	t, ok := getTime(this)
	if !ok {
		return phpv.ZBool(false).ZVal(), nil
	}
	intervalObj, ok := args[0].Value().(*phpobj.ZObject)
	if !ok {
		return nil, ctx.Errorf("DateTimeImmutable::add() expects parameter 1 to be DateInterval")
	}
	newT := addIntervalToTime(ctx, t, intervalObj, false)
	newObj, err := phpobj.NewZObject(ctx, DateTimeImmutable)
	if err != nil {
		return nil, err
	}
	setTimeVal(newObj, newT)
	return newObj.ZVal(), nil
}

// subMethod implements DateTime::sub(DateInterval $interval): DateTime
func subMethod(ctx phpv.Context, this *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	if len(args) < 1 {
		return nil, ctx.Errorf("DateTime::sub() expects exactly 1 argument, 0 given")
	}
	if err := checkDateTimeInitialized(ctx, this); err != nil {
		return nil, err
	}
	t, ok := getTime(this)
	if !ok {
		return phpv.ZBool(false).ZVal(), nil
	}
	intervalObj, ok := args[0].Value().(*phpobj.ZObject)
	if !ok {
		return nil, ctx.Errorf("DateTime::sub() expects parameter 1 to be DateInterval")
	}
	newT := addIntervalToTime(ctx, t, intervalObj, true)
	setTimeVal(this, newT)
	return this.ZVal(), nil
}

// subImmutableMethod implements DateTimeImmutable::sub() - returns new instance
func subImmutableMethod(ctx phpv.Context, this *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	if len(args) < 1 {
		return nil, ctx.Errorf("DateTimeImmutable::sub() expects exactly 1 argument, 0 given")
	}
	if err := checkDateTimeInitialized(ctx, this); err != nil {
		return nil, err
	}
	t, ok := getTime(this)
	if !ok {
		return phpv.ZBool(false).ZVal(), nil
	}
	intervalObj, ok := args[0].Value().(*phpobj.ZObject)
	if !ok {
		return nil, ctx.Errorf("DateTimeImmutable::sub() expects parameter 1 to be DateInterval")
	}
	newT := addIntervalToTime(ctx, t, intervalObj, true)
	newObj, err := phpobj.NewZObject(ctx, DateTimeImmutable)
	if err != nil {
		return nil, err
	}
	setTimeVal(newObj, newT)
	return newObj.ZVal(), nil
}

// setISODateMethod implements DateTime::setISODate(int $year, int $week, int $dayOfWeek = 1): DateTime
func setISODateMethod(ctx phpv.Context, this *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	if len(args) < 2 {
		return nil, ctx.Errorf("DateTime::setISODate() expects at least 2 arguments")
	}
	if err := checkDateTimeInitialized(ctx, this); err != nil {
		return nil, err
	}
	t, ok := getTime(this)
	if !ok {
		return phpv.ZBool(false).ZVal(), nil
	}
	year := int(args[0].AsInt(ctx))
	week := int(args[1].AsInt(ctx))
	dayOfWeek := 1
	if len(args) > 2 {
		dayOfWeek = int(args[2].AsInt(ctx))
	}

	// Calculate date from ISO year/week/day
	jan4 := time.Date(year, time.January, 4, 0, 0, 0, 0, t.Location())
	isoWeek1Monday := jan4.AddDate(0, 0, -int(jan4.Weekday()-time.Monday))
	if jan4.Weekday() == time.Sunday {
		isoWeek1Monday = jan4.AddDate(0, 0, -6)
	}
	target := isoWeek1Monday.AddDate(0, 0, (week-1)*7+(dayOfWeek-1))
	h, m, s := t.Clock()
	newT := time.Date(target.Year(), target.Month(), target.Day(), h, m, s, t.Nanosecond(), t.Location())
	setTimeVal(this, newT)
	return this.ZVal(), nil
}

// setISODateImmutableMethod implements DateTimeImmutable::setISODate() - returns new instance
func setISODateImmutableMethod(ctx phpv.Context, this *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	if len(args) < 2 {
		return nil, ctx.Errorf("DateTimeImmutable::setISODate() expects at least 2 arguments")
	}
	if err := checkDateTimeInitialized(ctx, this); err != nil {
		return nil, err
	}
	t, ok := getTime(this)
	if !ok {
		return phpv.ZBool(false).ZVal(), nil
	}
	year := int(args[0].AsInt(ctx))
	week := int(args[1].AsInt(ctx))
	dayOfWeek := 1
	if len(args) > 2 {
		dayOfWeek = int(args[2].AsInt(ctx))
	}

	jan4 := time.Date(year, time.January, 4, 0, 0, 0, 0, t.Location())
	isoWeek1Monday := jan4.AddDate(0, 0, -int(jan4.Weekday()-time.Monday))
	if jan4.Weekday() == time.Sunday {
		isoWeek1Monday = jan4.AddDate(0, 0, -6)
	}
	target := isoWeek1Monday.AddDate(0, 0, (week-1)*7+(dayOfWeek-1))
	h, m, s := t.Clock()
	newT := time.Date(target.Year(), target.Month(), target.Day(), h, m, s, t.Nanosecond(), t.Location())
	newObj, err := phpobj.NewZObject(ctx, DateTimeImmutable)
	if err != nil {
		return nil, err
	}
	setTimeVal(newObj, newT)
	return newObj.ZVal(), nil
}

// createFromTimestampStatic implements DateTime::createFromTimestamp(int|float $timestamp)
func createFromTimestampStatic(targetClass *phpobj.ZClass) func(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	return func(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
		if len(args) < 1 {
			return nil, phpobj.ThrowError(ctx, phpobj.TypeError, fmt.Sprintf("%s::createFromTimestamp() expects exactly 1 argument, 0 given", targetClass.Name))
		}
		val := args[0]

		var t time.Time
		if val.GetType() == phpv.ZtFloat {
			f := float64(val.AsFloat(ctx))
			if math.IsNaN(f) || math.IsInf(f, 0) {
				return nil, phpobj.ThrowError(ctx, phpobj.ValueError, fmt.Sprintf("%s::createFromTimestamp(): Argument #1 ($timestamp) must be a finite number, %s given", targetClass.Name, val.AsString(ctx)))
			}
			sec := int64(f)
			nsec := int64((f - float64(sec)) * 1e9)
			if nsec < 0 {
				sec--
				nsec += 1e9
			}
			t = time.Unix(sec, nsec).UTC()
		} else {
			ts := val.AsInt(ctx)
			t = time.Unix(int64(ts), 0).UTC()
		}

		obj, err := phpobj.NewZObject(ctx, targetClass)
		if err != nil {
			return nil, err
		}
		setTimeVal(obj, t)
		return obj.ZVal(), nil
	}
}

// createFromInterfaceStatic implements DateTime::createFromInterface(DateTimeInterface $object)
func createFromInterfaceStatic(targetClass *phpobj.ZClass) func(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	return func(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
		if len(args) < 1 {
			return nil, phpobj.ThrowError(ctx, phpobj.TypeError, fmt.Sprintf("%s::createFromInterface() expects exactly 1 argument, 0 given", targetClass.Name))
		}
		if args[0].GetType() != phpv.ZtObject {
			return nil, phpobj.ThrowError(ctx, phpobj.TypeError, fmt.Sprintf("%s::createFromInterface(): Argument #1 ($object) must be of type DateTimeInterface, %s given", targetClass.Name, args[0].GetType().TypeName()))
		}
		srcObj := args[0].Value().(phpv.ZObject)
		// Check if source object is properly initialized
		if zo, ok := srcObj.(*phpobj.ZObject); ok {
			if err := checkDateTimeInitialized(ctx, zo); err != nil {
				return nil, err
			}
		}
		srcT, ok := getTimeFromObj(srcObj)
		if !ok {
			return nil, phpobj.ThrowError(ctx, phpobj.TypeError, fmt.Sprintf("%s::createFromInterface(): Argument #1 ($object) must be of type DateTimeInterface", targetClass.Name))
		}
		// Use late static binding to get the called class
		actualClass := getCalledClassForStatic(ctx, targetClass)
		newObj, err := phpobj.NewZObject(ctx, actualClass)
		if err != nil {
			return nil, err
		}
		setTimeVal(newObj, srcT)
		return newObj.ZVal(), nil
	}
}

// serializeMethod implements DateTime::__serialize(): array
func serializeMethod(ctx phpv.Context, this *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	if err := checkDateTimeInitialized(ctx, this); err != nil {
		return nil, err
	}
	t, ok := getTime(this)
	if !ok {
		return phpv.NewZArray().ZVal(), nil
	}
	arr := phpv.NewZArray()
	dateStr := fmt.Sprintf("%04d-%02d-%02d %02d:%02d:%02d.%06d",
		t.Year(), int(t.Month()), t.Day(),
		t.Hour(), t.Minute(), t.Second(), t.Nanosecond()/1000)
	if t.Year() < 0 {
		dateStr = fmt.Sprintf("-%04d-%02d-%02d %02d:%02d:%02d.%06d",
			-t.Year(), int(t.Month()), t.Day(),
			t.Hour(), t.Minute(), t.Second(), t.Nanosecond()/1000)
	}
	arr.OffsetSet(ctx, phpv.ZString("date"), phpv.ZString(dateStr).ZVal())

	locName := t.Location().String()
	tzType := 3
	if locName == "" {
		_, offset := t.Zone()
		if offset == 0 {
			locName = "UTC"
		} else {
			sign := "+"
			absOffset := offset
			if offset < 0 {
				sign = "-"
				absOffset = -offset
			}
			hours := absOffset / 3600
			mins := (absOffset % 3600) / 60
			locName = fmt.Sprintf("%s%02d:%02d", sign, hours, mins)
			tzType = 1
		}
	} else if locName == "UTC" {
		tzType = 3
	} else if len(locName) > 0 && (locName[0] == '+' || locName[0] == '-') {
		tzType = 1
	} else if len(locName) <= 6 && !strings.Contains(locName, "/") {
		tzType = 2
	}
	arr.OffsetSet(ctx, phpv.ZString("timezone_type"), phpv.ZInt(tzType).ZVal())
	arr.OffsetSet(ctx, phpv.ZString("timezone"), phpv.ZString(locName).ZVal())

	return arr.ZVal(), nil
}

// unserializeMethod implements DateTime::__unserialize(array $data)
func unserializeMethod(ctx phpv.Context, this *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	if len(args) < 1 || args[0].GetType() != phpv.ZtArray {
		return nil, phpobj.ThrowError(ctx, phpobj.Error, "Invalid serialization data for DateTime object")
	}
	arr := args[0].Value().(*phpv.ZArray)
	dateVal, _ := arr.OffsetGet(ctx, phpv.ZString("date").ZVal())
	if dateVal == nil || dateVal.IsNull() || string(dateVal.AsString(ctx)) == "" {
		return nil, phpobj.ThrowError(ctx, phpobj.Error, "Invalid serialization data for DateTime object")
	}
	dateStr := string(dateVal.AsString(ctx))
	tzVal, _ := arr.OffsetGet(ctx, phpv.ZString("timezone").ZVal())
	var loc *time.Location = time.UTC
	if tzVal != nil && !tzVal.IsNull() {
		tzStr := string(tzVal.AsString(ctx))
		if l, err := time.LoadLocation(tzStr); err == nil {
			loc = l
		} else if offset, ok := parseTZOffset(tzStr); ok {
			loc = time.FixedZone(tzStr, offset)
		}
	}
	parsed, err := time.ParseInLocation("2006-01-02 15:04:05.000000", dateStr, loc)
	if err != nil {
		parsed, err = time.ParseInLocation("2006-01-02 15:04:05", dateStr, loc)
		if err != nil {
			return nil, phpobj.ThrowError(ctx, phpobj.Error, "Invalid serialization data for DateTime object")
		}
	}
	setTimeVal(this, parsed)
	return nil, nil
}

// wakeupMethod implements DateTime::__wakeup() - validates serialization
func wakeupMethod(ctx phpv.Context, this *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	dateVal := this.HashTable().GetString("date")
	if dateVal == nil || dateVal.IsNull() || string(dateVal.AsString(ctx)) == "" {
		return nil, phpobj.ThrowError(ctx, phpobj.Error, "Invalid serialization data for DateTime object")
	}

	dateStr := string(dateVal.AsString(ctx))
	tzVal := this.HashTable().GetString("timezone")
	var loc *time.Location = time.UTC
	if tzVal != nil && !tzVal.IsNull() {
		tzStr := string(tzVal.AsString(ctx))
		if l, err := time.LoadLocation(tzStr); err == nil {
			loc = l
		} else if offset, ok := parseTZOffset(tzStr); ok {
			loc = time.FixedZone(tzStr, offset)
		}
	}

	parsed, err := time.ParseInLocation("2006-01-02 15:04:05.000000", dateStr, loc)
	if err != nil {
		parsed, err = time.ParseInLocation("2006-01-02 15:04:05", dateStr, loc)
		if err != nil {
			return nil, phpobj.ThrowError(ctx, phpobj.Error, "Invalid serialization data for DateTime object")
		}
	}
	setTimeVal(this, parsed)
	return nil, nil
}

// getLastErrorsStatic implements DateTime::getLastErrors(): array|false
func getLastErrorsStatic(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	return phpv.ZBool(false).ZVal(), nil
}

// createFromFormatParsed does the actual format parsing for createFromFormat
func createFromFormatParsed(ctx phpv.Context, format string, datetime string, loc *time.Location) (time.Time, bool) {
	now := time.Now().In(loc)
	year := now.Year()
	month := int(now.Month())
	day := now.Day()
	hour := now.Hour()
	minute := now.Minute()
	second := now.Second()
	microsecond := 0
	yearSet := false
	monthSet := false
	daySet := false
	hourSet := false
	minuteSet := false
	secondSet := false
	resetTime := false
	usedLoc := loc

	di := 0 // datetime index
	for fi := 0; fi < len(format) && di <= len(datetime); fi++ {
		fc := format[fi]
		switch fc {
		case 'Y': // 4-digit year
			if di+4 > len(datetime) {
				return time.Time{}, false
			}
			n, err := fmt.Sscanf(datetime[di:di+4], "%d", &year)
			if err != nil || n != 1 {
				return time.Time{}, false
			}
			yearSet = true
			di += 4
		case 'y': // 2-digit year
			if di+2 > len(datetime) {
				return time.Time{}, false
			}
			var y2 int
			n, err := fmt.Sscanf(datetime[di:di+2], "%d", &y2)
			if err != nil || n != 1 {
				return time.Time{}, false
			}
			if y2 >= 70 {
				year = 1900 + y2
			} else {
				year = 2000 + y2
			}
			yearSet = true
			di += 2
		case 'm', 'n': // month with/without leading zero
			end := di
			for end < len(datetime) && end < di+2 && datetime[end] >= '0' && datetime[end] <= '9' {
				end++
			}
			if end == di {
				return time.Time{}, false
			}
			fmt.Sscanf(datetime[di:end], "%d", &month)
			monthSet = true
			di = end
		case 'd', 'j': // day with/without leading zero
			end := di
			for end < len(datetime) && end < di+2 && datetime[end] >= '0' && datetime[end] <= '9' {
				end++
			}
			if end == di {
				return time.Time{}, false
			}
			fmt.Sscanf(datetime[di:end], "%d", &day)
			daySet = true
			di = end
		case 'H', 'G': // 24-hour with/without leading zero
			end := di
			for end < len(datetime) && end < di+2 && datetime[end] >= '0' && datetime[end] <= '9' {
				end++
			}
			if end == di {
				return time.Time{}, false
			}
			fmt.Sscanf(datetime[di:end], "%d", &hour)
			hourSet = true
			di = end
		case 'h', 'g': // 12-hour format
			end := di
			for end < len(datetime) && end < di+2 && datetime[end] >= '0' && datetime[end] <= '9' {
				end++
			}
			if end == di {
				return time.Time{}, false
			}
			fmt.Sscanf(datetime[di:end], "%d", &hour)
			hourSet = true
			di = end
		case 'i': // minutes
			if di+2 > len(datetime) {
				return time.Time{}, false
			}
			fmt.Sscanf(datetime[di:di+2], "%d", &minute)
			minuteSet = true
			di += 2
		case 's': // seconds
			if di+2 > len(datetime) {
				return time.Time{}, false
			}
			fmt.Sscanf(datetime[di:di+2], "%d", &second)
			secondSet = true
			di += 2
		case 'u': // microseconds (up to 6 digits)
			end := di
			for end < len(datetime) && end < di+6 && datetime[end] >= '0' && datetime[end] <= '9' {
				end++
			}
			if end == di {
				return time.Time{}, false
			}
			s := datetime[di:end]
			for len(s) < 6 {
				s += "0"
			}
			fmt.Sscanf(s, "%d", &microsecond)
			di = end
		case 'v': // milliseconds (3 digits)
			end := di
			for end < len(datetime) && end < di+3 && datetime[end] >= '0' && datetime[end] <= '9' {
				end++
			}
			if end == di {
				return time.Time{}, false
			}
			s := datetime[di:end]
			for len(s) < 3 {
				s += "0"
			}
			var ms int
			fmt.Sscanf(s, "%d", &ms)
			microsecond = ms * 1000
			di = end
		case 'A', 'a': // AM/PM
			if di+2 > len(datetime) {
				return time.Time{}, false
			}
			ampm := strings.ToUpper(datetime[di : di+2])
			if ampm == "PM" && hour < 12 {
				hour += 12
			} else if ampm == "AM" && hour == 12 {
				hour = 0
			}
			di += 2
		case 'P', 'p', 'O': // timezone offset +02:00 or +0200 or Z
			if di < len(datetime) && datetime[di] == 'Z' {
				usedLoc = time.UTC
				di++
			} else if di < len(datetime) && (datetime[di] == '+' || datetime[di] == '-') {
				end := di + 1
				for end < len(datetime) && (datetime[end] >= '0' && datetime[end] <= '9' || datetime[end] == ':') {
					end++
				}
				if offset, ok := parseTZOffset(datetime[di:end]); ok {
					usedLoc = makeFixedZone(offset)
					di = end
				}
			}
		case 'T': // timezone abbreviation
			end := di
			for end < len(datetime) && ((datetime[end] >= 'A' && datetime[end] <= 'Z') || (datetime[end] >= 'a' && datetime[end] <= 'z')) {
				end++
			}
			if end > di {
				tzName := datetime[di:end]
				if l, err := time.LoadLocation(tzName); err == nil {
					usedLoc = l
				}
				di = end
			}
		case 'e': // timezone identifier
			end := di
			for end < len(datetime) && datetime[end] != ' ' && datetime[end] != '\t' {
				end++
			}
			if end > di {
				tzName := datetime[di:end]
				if l, err := time.LoadLocation(tzName); err == nil {
					usedLoc = l
				}
				di = end
			}
		case 'U': // unix timestamp
			end := di
			if end < len(datetime) && (datetime[end] == '+' || datetime[end] == '-') {
				end++
			}
			for end < len(datetime) && datetime[end] >= '0' && datetime[end] <= '9' {
				end++
			}
			if end > di {
				var ts int64
				fmt.Sscanf(datetime[di:end], "%d", &ts)
				return time.Unix(ts, 0).In(loc), true
			}
		case 'M', 'F': // month name (short or full)
			end := di
			for end < len(datetime) && ((datetime[end] >= 'A' && datetime[end] <= 'Z') || (datetime[end] >= 'a' && datetime[end] <= 'z')) {
				end++
			}
			if end > di {
				m, ok := parseMonth(datetime[di:end])
				if ok {
					month = int(m)
					monthSet = true
				}
				di = end
			}
		case 'D', 'l': // day name (skip)
			end := di
			for end < len(datetime) && ((datetime[end] >= 'A' && datetime[end] <= 'Z') || (datetime[end] >= 'a' && datetime[end] <= 'z')) {
				end++
			}
			di = end
		case '\\': // literal next char
			fi++
			if fi < len(format) && di < len(datetime) {
				di++
			}
		case '!': // reset all to unix epoch
			year = 1970
			month = 1
			day = 1
			hour = 0
			minute = 0
			second = 0
			microsecond = 0
			yearSet = true
			monthSet = true
			daySet = true
			hourSet = true
			minuteSet = true
			secondSet = true
		case '|': // reset unset fields to unix epoch
			resetTime = true
		case '+': // consume rest
			di = len(datetime)
		case '#': // separator: any of ;:/.,-
			if di < len(datetime) {
				di++
			}
		case '?': // any single character
			if di < len(datetime) {
				di++
			}
		case '*': // any number of characters until next digit
			for di < len(datetime) && !(datetime[di] >= '0' && datetime[di] <= '9') {
				di++
			}
		case ' ': // optional whitespace
			for di < len(datetime) && (datetime[di] == ' ' || datetime[di] == '\t') {
				di++
			}
		default:
			// Literal character match
			if di < len(datetime) && datetime[di] == fc {
				di++
			}
		}
	}

	// If '|' was used, reset unset fields to unix epoch values
	if resetTime {
		if !yearSet {
			year = 1970
		}
		if !monthSet {
			month = 1
		}
		if !daySet {
			day = 1
		}
		if !hourSet {
			hour = 0
		}
		if !minuteSet {
			minute = 0
		}
		if !secondSet {
			second = 0
		}
	}

	return time.Date(year, time.Month(month), day, hour, minute, second, microsecond*1000, usedLoc), true
}

// dateTimeDebugInfo implements __debugInfo for DateTime/DateTimeImmutable
func dateTimeDebugInfo(ctx phpv.Context, this *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	if !isDateTimeInitialized(this) {
		return phpv.NewZArray().ZVal(), nil
	}
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
	if locName == "" {
		_, offset := t.Zone()
		if offset == 0 {
			locName = "UTC"
		} else {
			sign := "+"
			absOffset := offset
			if offset < 0 {
				sign = "-"
				absOffset = -offset
			}
			hours := absOffset / 3600
			mins := (absOffset % 3600) / 60
			locName = fmt.Sprintf("%s%02d:%02d", sign, hours, mins)
			tzType = 1
		}
	} else if locName == "UTC" {
		tzType = 3
	} else if len(locName) > 0 && (locName[0] == '+' || locName[0] == '-') {
		tzType = 1
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
			"gettransitions": {
				Name:      "getTransitions",
				Modifiers: phpv.ZAttrPublic,
				Method: phpobj.NativeMethod(func(ctx phpv.Context, this *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
					loc, ok := getTimezoneLoc(this)
					if !ok {
						return phpv.ZBool(false).ZVal(), nil
					}
					// Return a basic transitions array with at least the current offset
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
				}),
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
		Const: map[phpv.ZString]*phpv.ZClassConst{
			"ATOM":              {Value: phpv.ZString("Y-m-d\\TH:i:sP")},
			"COOKIE":            {Value: phpv.ZString("l, d-M-Y H:i:s T")},
			"ISO8601":           {Value: phpv.ZString("Y-m-d\\TH:i:sO")},
			"ISO8601_EXPANDED":  {Value: phpv.ZString("X-m-d\\TH:i:sP")},
			"RFC822":            {Value: phpv.ZString("D, d M y H:i:s O")},
			"RFC850":            {Value: phpv.ZString("l, d-M-y H:i:s T")},
			"RFC1036":           {Value: phpv.ZString("D, d M y H:i:s O")},
			"RFC1123":           {Value: phpv.ZString("D, d M Y H:i:s O")},
			"RFC7231": {Value: phpv.ZString("D, d M Y H:i:s \\G\\M\\T"), Attributes: []*phpv.ZAttribute{{ClassName: "Deprecated", Args: []*phpv.ZVal{phpv.ZString("as this format ignores the associated timezone and always uses GMT").ZVal(), phpv.ZString("8.5").ZVal()}}}},
			"RFC2822":           {Value: phpv.ZString("D, d M Y H:i:s O")},
			"RFC3339":           {Value: phpv.ZString("Y-m-d\\TH:i:sP")},
			"RFC3339_EXTENDED":  {Value: phpv.ZString("Y-m-d\\TH:i:s.vP")},
			"RSS":               {Value: phpv.ZString("D, d M Y H:i:s O")},
			"W3C":               {Value: phpv.ZString("Y-m-d\\TH:i:sP")},
		},
		ConstOrder: []phpv.ZString{
			"ATOM", "COOKIE", "ISO8601", "ISO8601_EXPANDED",
			"RFC822", "RFC850", "RFC1036", "RFC1123", "RFC7231",
			"RFC2822", "RFC3339", "RFC3339_EXTENDED", "RSS", "W3C",
		},
		Methods: map[phpv.ZString]*phpv.ZClassMethod{
			"format":       {Name: "format", Modifiers: phpv.ZAttrPublic, Empty: true},
			"gettimestamp": {Name: "getTimestamp", Modifiers: phpv.ZAttrPublic, Empty: true},
			"getoffset":    {Name: "getOffset", Modifiers: phpv.ZAttrPublic, Empty: true},
			"gettimezone":  {Name: "getTimezone", Modifiers: phpv.ZAttrPublic, Empty: true},
			"diff":         {Name: "diff", Modifiers: phpv.ZAttrPublic, Empty: true},
			"__wakeup":     {Name: "__wakeup", Modifiers: phpv.ZAttrPublic, Empty: true},
		},
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
			{VarName: "from_string", Default: phpv.ZBool(false).ZVal(), Modifiers: phpv.ZAttrPublic},
		},
		Methods: map[phpv.ZString]*phpv.ZClassMethod{
			"__construct":        {Name: "__construct", Method: phpobj.NativeMethod(dateIntervalConstruct)},
			"format":             {Name: "format", Method: phpobj.NativeMethod(dateIntervalFormat)},
			"__unserialize": {
				Name:      "__unserialize",
				Modifiers: phpv.ZAttrPublic,
				Method: phpobj.NativeMethod(func(ctx phpv.Context, this *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
					if len(args) < 1 || args[0].GetType() != phpv.ZtArray {
						return nil, nil
					}
					arr := args[0].Value().(*phpv.ZArray)
					// Mark as initialized
					this.SetOpaque(DateInterval, true)
					// Check if from_string mode
					fromStr, _ := arr.OffsetGet(ctx, phpv.ZString("from_string").ZVal())
					if fromStr != nil && bool(fromStr.AsBool(ctx)) {
						this.HashTable().SetString("from_string", phpv.ZBool(true).ZVal())
						ds, _ := arr.OffsetGet(ctx, phpv.ZString("date_string").ZVal())
						if ds != nil && !ds.IsNull() {
							this.HashTable().SetString("date_string", ds)
						}
						// Remove non-from_string properties from hash table
						for _, key := range []string{"y", "m", "d", "h", "i", "s", "f", "invert", "days"} {
							this.HashTable().UnsetString(phpv.ZString(key))
						}
					} else {
						for _, key := range []string{"y", "m", "d", "h", "i", "s", "f", "invert", "days", "from_string", "date_string"} {
							v, _ := arr.OffsetGet(ctx, phpv.ZString(key).ZVal())
							if v != nil && !v.IsNull() {
								this.HashTable().SetString(phpv.ZString(key), v)
							}
						}
					}
					// Also handle any extra properties (subclass promoted properties)
					it := arr.NewIterator()
					for it.Valid(ctx) {
						k, _ := it.Key(ctx)
						v, _ := it.Current(ctx)
						ks := k.String()
						switch ks {
						case "y", "m", "d", "h", "i", "s", "f", "invert", "days", "from_string", "date_string":
							// Already handled above
						default:
							this.ObjectSet(ctx, phpv.ZString(ks), v)
						}
						it.Next(ctx)
					}
					return nil, nil
				}),
			},
			"__serialize": {
				Name:      "__serialize",
				Modifiers: phpv.ZAttrPublic,
				Method: phpobj.NativeMethod(func(ctx phpv.Context, this *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
					if err := checkDateIntervalInitialized(ctx, this); err != nil {
						return nil, err
					}
					result := phpv.NewZArray()
					// Check if from_string is set
					fromStr := this.HashTable().GetString("from_string")
					if fromStr != nil && bool(fromStr.AsBool(ctx)) {
						result.OffsetSet(ctx, phpv.ZString("from_string"), phpv.ZBool(true).ZVal())
						ds := this.HashTable().GetString("date_string")
						if ds != nil {
							result.OffsetSet(ctx, phpv.ZString("date_string"), ds)
						}
					} else {
						for _, key := range []string{"y", "m", "d", "h", "i", "s", "f", "invert", "days", "from_string"} {
							v := this.HashTable().GetString(phpv.ZString(key))
							if v != nil {
								result.OffsetSet(ctx, phpv.ZString(key), v)
							}
						}
					}
					return result.ZVal(), nil
				}),
			},
			"__set_state": {
				Name:      "__set_state",
				Modifiers: phpv.ZAttrPublic | phpv.ZAttrStatic,
				Method: phpobj.NativeStaticMethod(func(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
					if len(args) < 1 || args[0].GetType() != phpv.ZtArray {
						return nil, phpobj.ThrowError(ctx, phpobj.Error, "Invalid serialization data for DateInterval object")
					}
					arr := args[0].Value().(*phpv.ZArray)

					// Check if from_string is true
					fromStr, _ := arr.OffsetGet(ctx, phpv.ZString("from_string").ZVal())
					if fromStr != nil && bool(fromStr.AsBool(ctx)) {
						// date_string must be present and valid
						ds, _ := arr.OffsetGet(ctx, phpv.ZString("date_string").ZVal())
						if ds != nil && !ds.IsNull() {
							dateStr := string(ds.AsString(ctx))
							return createDateIntervalFromString(ctx, dateStr)
						}
						return nil, phpobj.ThrowError(ctx, phpobj.Error, "Invalid serialization data for DateInterval object")
					}

					// Create from individual fields
					obj, err := phpobj.NewZObject(ctx, DateInterval)
					if err != nil {
						return nil, err
					}
					for _, key := range []string{"y", "m", "d", "h", "i", "s", "f", "invert", "days", "from_string"} {
						v, _ := arr.OffsetGet(ctx, phpv.ZString(key).ZVal())
						if v != nil && !v.IsNull() {
							obj.HashTable().SetString(phpv.ZString(key), v)
						}
					}
					return obj.ZVal(), nil
				}),
			},
			"createfromdatestring": {
				Name:      "createFromDateString",
				Modifiers: phpv.ZAttrPublic | phpv.ZAttrStatic,
				Method: phpobj.NativeStaticMethod(func(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
					if len(args) < 1 {
						return nil, ctx.Errorf("DateInterval::createFromDateString() expects exactly 1 argument")
					}
					dateStr := string(args[0].AsString(ctx))
					return createDateIntervalFromString(ctx, dateStr)
				}),
			},
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
			"RFC7231": {Value: phpv.ZString("D, d M Y H:i:s \\G\\M\\T"), Attributes: []*phpv.ZAttribute{{ClassName: "Deprecated", Args: []*phpv.ZVal{phpv.ZString("as this format ignores the associated timezone and always uses GMT").ZVal(), phpv.ZString("8.5").ZVal()}}}},
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
					if len(args) > 2 {
						return nil, phpobj.ThrowError(ctx, phpobj.ArgumentCountError, fmt.Sprintf("DateTime::__construct() expects at most 2 arguments, %d given", len(args)))
					}
					t, err2 := parseDateTimeWithTz(ctx, args)
					if err2 != nil {
						return nil, err2
					}
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
			"add": {
				Name:      "add",
				Modifiers: phpv.ZAttrPublic,
				Method:    phpobj.NativeMethod(addMethod),
			},
			"sub": {
				Name:      "sub",
				Modifiers: phpv.ZAttrPublic,
				Method:    phpobj.NativeMethod(subMethod),
			},
			"setdate": {
				Name:      "setDate",
				Modifiers: phpv.ZAttrPublic,
				Method:    phpobj.NativeMethod(setDateMethod),
			},
			"setisodate": {
				Name:      "setISODate",
				Modifiers: phpv.ZAttrPublic,
				Method:    phpobj.NativeMethod(setISODateMethod),
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
			},
			"createfromtimestamp": {
				Name:      "createFromTimestamp",
				Modifiers: phpv.ZAttrPublic | phpv.ZAttrStatic,
			},
			"createfrominterface": {
				Name:      "createFromInterface",
				Modifiers: phpv.ZAttrPublic | phpv.ZAttrStatic,
			},
			"createfromimmutable": {
				Name:      "createFromImmutable",
				Modifiers: phpv.ZAttrPublic | phpv.ZAttrStatic,
			},
			"getlasterrors": {
				Name:      "getLastErrors",
				Modifiers: phpv.ZAttrPublic | phpv.ZAttrStatic,
				Method:    phpobj.NativeStaticMethod(getLastErrorsStatic),
			},
			"__debuginfo": {
				Name:      "__debugInfo",
				Modifiers: phpv.ZAttrPublic,
				Method:    phpobj.NativeMethod(dateTimeDebugInfo),
			},
			"__serialize": {
				Name:      "__serialize",
				Modifiers: phpv.ZAttrPublic,
				Method:    phpobj.NativeMethod(serializeMethod),
			},
			"__unserialize": {
				Name:      "__unserialize",
				Modifiers: phpv.ZAttrPublic,
				Method:    phpobj.NativeMethod(unserializeMethod),
			},
			"__wakeup": {
				Name:      "__wakeup",
				Modifiers: phpv.ZAttrPublic,
				Method:    phpobj.NativeMethod(wakeupMethod),
			},
			"__set_state": {
				Name:      "__set_state",
				Modifiers: phpv.ZAttrPublic | phpv.ZAttrStatic,
			},
		},
	}
	// Wire up methods that reference DateTime itself
	DateTime.Methods["__set_state"].Method = phpobj.NativeStaticMethod(dateTimeSetState(DateTime))
	DateTime.Methods["createfromformat"].Method = phpobj.NativeStaticMethod(createFromFormatStaticFor(DateTime))
	DateTime.Methods["createfromtimestamp"].Method = phpobj.NativeStaticMethod(createFromTimestampStatic(DateTime))
	DateTime.Methods["createfrominterface"].Method = phpobj.NativeStaticMethod(createFromInterfaceStatic(DateTime))
	DateTime.Methods["createfromimmutable"].Method = phpobj.NativeStaticMethod(func(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
		if len(args) < 1 {
			return nil, phpobj.ThrowError(ctx, phpobj.TypeError, "DateTime::createFromImmutable() expects exactly 1 argument, 0 given")
		}
		if args[0].GetType() != phpv.ZtObject {
			return nil, phpobj.ThrowError(ctx, phpobj.TypeError, "DateTime::createFromImmutable(): Argument #1 ($object) must be of type DateTimeImmutable, "+args[0].GetType().TypeName()+" given")
		}
		srcObj := args[0].Value().(phpv.ZObject)
		// Check if source object is properly initialized
		if zo, ok := srcObj.(*phpobj.ZObject); ok {
			if err := checkDateTimeInitialized(ctx, zo); err != nil {
				return nil, err
			}
		}
		srcT, ok := getTimeFromObj(srcObj)
		if !ok {
			return nil, phpobj.ThrowError(ctx, phpobj.TypeError, "DateTime::createFromImmutable(): Argument #1 ($object) must be of type DateTimeImmutable")
		}
		newObj, err := phpobj.NewZObject(ctx, DateTime)
		if err != nil {
			return nil, err
		}
		setTimeVal(newObj, srcT)
		return newObj.ZVal(), nil
	})

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
					if len(args) > 2 {
						return nil, phpobj.ThrowError(ctx, phpobj.ArgumentCountError, fmt.Sprintf("DateTimeImmutable::__construct() expects at most 2 arguments, %d given", len(args)))
					}
					t, err2 := parseDateTimeWithTz(ctx, args)
					if err2 != nil {
						return nil, err2
					}
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
			"add": {
				Name:      "add",
				Modifiers: phpv.ZAttrPublic,
				Method:    phpobj.NativeMethod(addImmutableMethod),
			},
			"sub": {
				Name:      "sub",
				Modifiers: phpv.ZAttrPublic,
				Method:    phpobj.NativeMethod(subImmutableMethod),
			},
			"setdate": {
				Name:      "setDate",
				Modifiers: phpv.ZAttrPublic,
				Method:    phpobj.NativeMethod(setDateImmutableMethod),
			},
			"setisodate": {
				Name:      "setISODate",
				Modifiers: phpv.ZAttrPublic,
				Method:    phpobj.NativeMethod(setISODateImmutableMethod),
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
				Method:    phpobj.NativeStaticMethod(createFromFormatStaticFor(DateTimeImmutable)),
			},
			"createfromtimestamp": {
				Name:      "createFromTimestamp",
				Modifiers: phpv.ZAttrPublic | phpv.ZAttrStatic,
			},
			"createfrominterface": {
				Name:      "createFromInterface",
				Modifiers: phpv.ZAttrPublic | phpv.ZAttrStatic,
			},
			"createfrommutable": {
				Name:      "createFromMutable",
				Modifiers: phpv.ZAttrPublic | phpv.ZAttrStatic,
			},
			"getlasterrors": {
				Name:      "getLastErrors",
				Modifiers: phpv.ZAttrPublic | phpv.ZAttrStatic,
				Method:    phpobj.NativeStaticMethod(getLastErrorsStatic),
			},
			"__debuginfo": {
				Name:      "__debugInfo",
				Modifiers: phpv.ZAttrPublic,
				Method:    phpobj.NativeMethod(dateTimeDebugInfo),
			},
			"__serialize": {
				Name:      "__serialize",
				Modifiers: phpv.ZAttrPublic,
				Method:    phpobj.NativeMethod(serializeMethod),
			},
			"__unserialize": {
				Name:      "__unserialize",
				Modifiers: phpv.ZAttrPublic,
				Method:    phpobj.NativeMethod(unserializeMethod),
			},
			"__wakeup": {
				Name:      "__wakeup",
				Modifiers: phpv.ZAttrPublic,
				Method:    phpobj.NativeMethod(wakeupMethod),
			},
			"__set_state": {
				Name:      "__set_state",
				Modifiers: phpv.ZAttrPublic | phpv.ZAttrStatic,
			},
		},
	}
	// Wire up methods that reference DateTimeImmutable itself
	DateTimeImmutable.Methods["__set_state"].Method = phpobj.NativeStaticMethod(dateTimeSetState(DateTimeImmutable))
	DateTimeImmutable.Methods["createfromtimestamp"].Method = phpobj.NativeStaticMethod(createFromTimestampStatic(DateTimeImmutable))
	DateTimeImmutable.Methods["createfrominterface"].Method = phpobj.NativeStaticMethod(createFromInterfaceStatic(DateTimeImmutable))
	DateTimeImmutable.Methods["createfrommutable"].Method = phpobj.NativeStaticMethod(func(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
		if len(args) < 1 {
			return nil, ctx.Errorf("DateTimeImmutable::createFromMutable() expects exactly 1 argument, 0 given")
		}
		if args[0].GetType() != phpv.ZtObject {
			return nil, phpobj.ThrowError(ctx, phpobj.TypeError, "DateTimeImmutable::createFromMutable(): Argument #1 ($object) must be of type DateTime, "+args[0].GetType().TypeName()+" given")
		}
		srcObj := args[0].Value().(phpv.ZObject)
		// Check that it's a DateTime, not a DateTimeImmutable
		if zo, ok := srcObj.(*phpobj.ZObject); ok {
			if zo.Class.InstanceOf(DateTimeImmutable) {
				return nil, phpobj.ThrowError(ctx, phpobj.TypeError, "DateTimeImmutable::createFromMutable(): Argument #1 ($object) must be of type DateTime, DateTimeImmutable given")
			}
			// Check if source object is properly initialized
			if err := checkDateTimeInitialized(ctx, zo); err != nil {
				return nil, err
			}
		}
		srcT, ok := getTimeFromObj(srcObj)
		if !ok {
			return nil, phpobj.ThrowError(ctx, phpobj.TypeError, "DateTimeImmutable::createFromMutable(): Argument #1 ($object) must be of type DateTime")
		}
		// Use late static binding
		actualClass := getCalledClassForStatic(ctx, DateTimeImmutable)
		newObj, err := phpobj.NewZObject(ctx, actualClass)
		if err != nil {
			return nil, err
		}
		setTimeVal(newObj, srcT)
		return newObj.ZVal(), nil
	})

	// DatePeriod class
	DatePeriod = &phpobj.ZClass{
		Name:            "DatePeriod",
		Implementations: []*phpobj.ZClass{phpobj.IteratorAggregate},
		Props: []*phpv.ZClassProp{
			{VarName: "start", Modifiers: phpv.ZAttrPublic | phpv.ZAttrReadonly},
			{VarName: "current", Modifiers: phpv.ZAttrPublic | phpv.ZAttrReadonly},
			{VarName: "end", Modifiers: phpv.ZAttrPublic | phpv.ZAttrReadonly},
			{VarName: "interval", Modifiers: phpv.ZAttrPublic | phpv.ZAttrReadonly},
			{VarName: "recurrences", Modifiers: phpv.ZAttrPublic | phpv.ZAttrReadonly},
			{VarName: "include_start_date", Modifiers: phpv.ZAttrPublic | phpv.ZAttrReadonly},
			{VarName: "include_end_date", Modifiers: phpv.ZAttrPublic | phpv.ZAttrReadonly},
		},
		Const: map[phpv.ZString]*phpv.ZClassConst{
			"EXCLUDE_START_DATE": {Value: phpv.ZInt(1)},
			"INCLUDE_END_DATE":   {Value: phpv.ZInt(2)},
		},
		Methods: map[phpv.ZString]*phpv.ZClassMethod{
			"__construct": {
				Name:      "__construct",
				Modifiers: phpv.ZAttrPublic,
				Method:    phpobj.NativeMethod(datePeriodConstruct),
			},
			"__set_state": {
				Name:      "__set_state",
				Modifiers: phpv.ZAttrPublic | phpv.ZAttrStatic,
			},
			"createfromiso8601string": {
				Name:      "createFromISO8601String",
				Modifiers: phpv.ZAttrPublic | phpv.ZAttrStatic,
			},
			"getstartdate": {
				Name:      "getStartDate",
				Modifiers: phpv.ZAttrPublic,
				Method: phpobj.NativeMethod(func(ctx phpv.Context, this *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
					v, _ := this.ObjectGet(ctx, phpv.ZString("start"))
					if v == nil {
						return phpv.ZNULL.ZVal(), nil
					}
					return v, nil
				}),
			},
			"getenddate": {
				Name:      "getEndDate",
				Modifiers: phpv.ZAttrPublic,
				Method: phpobj.NativeMethod(func(ctx phpv.Context, this *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
					v, _ := this.ObjectGet(ctx, phpv.ZString("end"))
					if v == nil {
						return phpv.ZNULL.ZVal(), nil
					}
					return v, nil
				}),
			},
			"getdateinterval": {
				Name:      "getDateInterval",
				Modifiers: phpv.ZAttrPublic,
				Method: phpobj.NativeMethod(func(ctx phpv.Context, this *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
					v, _ := this.ObjectGet(ctx, phpv.ZString("interval"))
					if v == nil {
						return phpv.ZNULL.ZVal(), nil
					}
					return v, nil
				}),
			},
			"getrecurrences": {
				Name:      "getRecurrences",
				Modifiers: phpv.ZAttrPublic,
				Method: phpobj.NativeMethod(func(ctx phpv.Context, this *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
					// getRecurrences() returns NULL when the period was created with an end date
					// (even though the property $recurrences is 1 in that case)
					if opaque := this.GetOpaque(DatePeriod); opaque != nil {
						if explicit, ok := opaque.(bool); ok && !explicit {
							return phpv.ZNULL.ZVal(), nil
						}
					}
					v, _ := this.ObjectGet(ctx, phpv.ZString("recurrences"))
					if v == nil || v.IsNull() {
						return phpv.ZNULL.ZVal(), nil
					}
					return v, nil
				}),
			},
			"getiterator": {
				Name:      "getIterator",
				Modifiers: phpv.ZAttrPublic,
				Method:    phpobj.NativeMethod(datePeriodGetIterator),
			},
		},
	}
	// Wire up methods that need DatePeriod reference
	DatePeriod.Methods["__set_state"].Method = phpobj.NativeStaticMethod(datePeriodSetState)
	DatePeriod.Methods["createfromiso8601string"].Method = phpobj.NativeStaticMethod(datePeriodCreateFromISO8601String)
}

// parseISO8601Period parses ISO 8601 repeating interval strings like "R2/2012-07-01T00:00:00Z/P7D"
// Returns recurrences, start time, interval duration spec, and end time (if any)
func parseISO8601Period(ctx phpv.Context, isoStr string) (recurrences int, start time.Time, intervalObj *phpobj.ZObject, hasEnd bool, end time.Time, err error) {
	recurrences = -1 // -1 means not specified

	// Format: R[n]/start/interval or R[n]/start/end or R[n]/interval/end
	parts := strings.SplitN(isoStr, "/", 3)
	if len(parts) < 3 {
		err = fmt.Errorf("invalid ISO 8601 period string: %s", isoStr)
		return
	}

	// Parse recurrences from R[n]
	rPart := parts[0]
	if len(rPart) == 0 || (rPart[0] != 'R' && rPart[0] != 'r') {
		err = fmt.Errorf("invalid ISO 8601 period string: expected R prefix")
		return
	}
	if len(rPart) > 1 {
		var n int
		_, scanErr := fmt.Sscanf(rPart[1:], "%d", &n)
		if scanErr == nil {
			recurrences = n
		}
	}

	// Parse start date/time
	startStr := parts[1]
	start, err = parseISO8601DateTime(startStr)
	if err != nil {
		return
	}

	// Parse interval or end
	thirdPart := parts[2]
	if len(thirdPart) > 0 && (thirdPart[0] == 'P' || thirdPart[0] == 'p') {
		// It's a duration
		intervalObj, err = phpobj.NewZObject(ctx, DateInterval, phpv.ZString(thirdPart).ZVal())
		if err != nil {
			return
		}
	} else {
		// It's an end date
		end, err = parseISO8601DateTime(thirdPart)
		if err != nil {
			return
		}
		hasEnd = true
	}

	return
}

// parseISO8601DateTime parses an ISO 8601 date/time string
func parseISO8601DateTime(s string) (time.Time, error) {
	// Try common formats
	for _, layout := range []string{
		"2006-01-02T15:04:05Z",
		"2006-01-02T15:04:05-07:00",
		"2006-01-02T15:04:05+07:00",
		time.RFC3339,
		"2006-01-02T15:04:05",
		"2006-01-02",
	} {
		if t, err := time.Parse(layout, s); err == nil {
			// Normalize: if the location is empty string from Parse, convert to proper fixed zone
			if t.Location().String() == "" {
				_, offset := t.Zone()
				t = t.In(makeFixedZone(offset))
			}
			return t, nil
		}
	}
	return time.Time{}, fmt.Errorf("cannot parse ISO 8601 date: %s", s)
}

// datePeriodConstruct implements DatePeriod::__construct
func datePeriodConstruct(ctx phpv.Context, this *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	if len(args) == 0 {
		return nil, phpobj.ThrowError(ctx, phpobj.TypeError, "DatePeriod::__construct() accepts (DateTimeInterface, DateInterval, int [, int]), or (DateTimeInterface, DateInterval, DateTime [, int]), or (string [, int]) as arguments")
	}

	// Check if first arg is a string (ISO 8601 period string)
	if args[0].GetType() == phpv.ZtString {
		// String form only accepts 1 or 2 arguments (string [, int])
		if len(args) > 2 {
			return nil, phpobj.ThrowError(ctx, phpobj.TypeError, "DatePeriod::__construct() accepts (DateTimeInterface, DateInterval, int [, int]), or (DateTimeInterface, DateInterval, DateTime [, int]), or (string [, int]) as arguments")
		}

		isoStr := string(args[0].AsString(ctx))

		// Deprecated: using string constructor
		ctx.Deprecated("Calling DatePeriod::__construct(string $isostr, int $options = 0) is deprecated, use DatePeriod::createFromISO8601String() instead", logopt.NoFuncName(true))

		// Parse options
		options := 0
		if len(args) >= 2 {
			options = int(args[1].AsInt(ctx))
		}

		return datePeriodInitFromISO(ctx, this, isoStr, options)
	}

	// Normal constructor: (DateTimeInterface $start, DateInterval $interval, int|DateTimeInterface $end [, int $options])
	if len(args) < 3 {
		return nil, phpobj.ThrowError(ctx, phpobj.TypeError, "DatePeriod::__construct() accepts (DateTimeInterface, DateInterval, int [, int]), or (DateTimeInterface, DateInterval, DateTime [, int]), or (string [, int]) as arguments")
	}

	this.ObjectSet(ctx, phpv.ZString("start"), args[0])
	this.ObjectSet(ctx, phpv.ZString("interval"), args[1])

	options := 0

	if len(args) >= 3 {
		// Third arg could be end DateTime or recurrence count
		if args[2].GetType() == phpv.ZtInt {
			recCount := int(args[2].AsInt(ctx))
			if recCount < 1 {
				return nil, phpobj.ThrowError(ctx, phpobj.ValueError,
					fmt.Sprintf("DatePeriod::__construct(): Recurrence count must be greater or equal to 1 and lower than %d", int(^uint(0)>>1)))
			}
			this.ObjectSet(ctx, phpv.ZString("recurrences"), args[2])
			// Opaque true = recurrences were explicitly given
			this.SetOpaque(DatePeriod, true)
			if len(args) >= 4 {
				options = int(args[3].AsInt(ctx))
			}
		} else {
			this.ObjectSet(ctx, phpv.ZString("end"), args[2])
			// When end date is given, recurrences property is 1 but getRecurrences() returns NULL
			this.ObjectSet(ctx, phpv.ZString("recurrences"), phpv.ZInt(1).ZVal())
			this.SetOpaque(DatePeriod, false)
			if len(args) >= 4 {
				options = int(args[3].AsInt(ctx))
			}
		}
	}

	includeStart := options&1 == 0 // EXCLUDE_START_DATE = 1
	includeEnd := options&2 != 0   // INCLUDE_END_DATE = 2
	this.ObjectSet(ctx, phpv.ZString("include_start_date"), phpv.ZBool(includeStart).ZVal())
	this.ObjectSet(ctx, phpv.ZString("include_end_date"), phpv.ZBool(includeEnd).ZVal())

	return nil, nil
}

// datePeriodInitFromISO initializes a DatePeriod from an ISO 8601 string
func datePeriodInitFromISO(ctx phpv.Context, this *phpobj.ZObject, isoStr string, options int) (*phpv.ZVal, error) {
	recurrences, start, intervalObj, hasEnd, end, err := parseISO8601Period(ctx, isoStr)
	if err != nil {
		return nil, phpobj.ThrowError(ctx, DateMalformedPeriodStringException, fmt.Sprintf("DatePeriod::__construct(): Unknown or bad format (%s)", isoStr))
	}

	// Create start DateTimeImmutable
	startObj, err := phpobj.NewZObject(ctx, DateTimeImmutable)
	if err != nil {
		return nil, err
	}
	setTimeVal(startObj, start)
	this.ObjectSet(ctx, phpv.ZString("start"), startObj.ZVal())

	if intervalObj != nil {
		this.ObjectSet(ctx, phpv.ZString("interval"), intervalObj.ZVal())
	}

	if hasEnd {
		endObj, err := phpobj.NewZObject(ctx, DateTimeImmutable)
		if err != nil {
			return nil, err
		}
		setTimeVal(endObj, end)
		this.ObjectSet(ctx, phpv.ZString("end"), endObj.ZVal())
	}

	if recurrences >= 0 {
		this.ObjectSet(ctx, phpv.ZString("recurrences"), phpv.ZInt(recurrences).ZVal())
	}

	includeStart := options&1 == 0
	includeEnd := options&2 != 0
	this.ObjectSet(ctx, phpv.ZString("include_start_date"), phpv.ZBool(includeStart).ZVal())
	this.ObjectSet(ctx, phpv.ZString("include_end_date"), phpv.ZBool(includeEnd).ZVal())

	return nil, nil
}

// datePeriodIteratorData holds the state of a DatePeriod iterator
type datePeriodIteratorData struct {
	dates []*phpv.ZVal
	pos   int
}

// DatePeriodIterator is an internal Iterator class for DatePeriod
var DatePeriodIterator *phpobj.ZClass

func init() {
	initDatePeriodIterator()
}

func initDatePeriodIterator() {
	DatePeriodIterator = &phpobj.ZClass{
		Name:            "DatePeriodIterator",
		Implementations: []*phpobj.ZClass{phpobj.Iterator},
		InternalOnly:    true,
		Methods: map[phpv.ZString]*phpv.ZClassMethod{
			"current": {
				Name:      "current",
				Modifiers: phpv.ZAttrPublic,
				Method: phpobj.NativeMethod(func(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
					d, _ := o.GetOpaque(DatePeriodIterator).(*datePeriodIteratorData)
					if d == nil || d.pos < 0 || d.pos >= len(d.dates) {
						return phpv.ZBool(false).ZVal(), nil
					}
					return d.dates[d.pos], nil
				}),
			},
			"key": {
				Name:      "key",
				Modifiers: phpv.ZAttrPublic,
				Method: phpobj.NativeMethod(func(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
					d, _ := o.GetOpaque(DatePeriodIterator).(*datePeriodIteratorData)
					if d == nil || d.pos < 0 || d.pos >= len(d.dates) {
						return phpv.ZNULL.ZVal(), nil
					}
					return phpv.ZInt(d.pos).ZVal(), nil
				}),
			},
			"next": {
				Name:      "next",
				Modifiers: phpv.ZAttrPublic,
				Method: phpobj.NativeMethod(func(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
					d, _ := o.GetOpaque(DatePeriodIterator).(*datePeriodIteratorData)
					if d != nil {
						d.pos++
					}
					return nil, nil
				}),
			},
			"rewind": {
				Name:      "rewind",
				Modifiers: phpv.ZAttrPublic,
				Method: phpobj.NativeMethod(func(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
					d, _ := o.GetOpaque(DatePeriodIterator).(*datePeriodIteratorData)
					if d != nil {
						d.pos = 0
					}
					return nil, nil
				}),
			},
			"valid": {
				Name:      "valid",
				Modifiers: phpv.ZAttrPublic,
				Method: phpobj.NativeMethod(func(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
					d, _ := o.GetOpaque(DatePeriodIterator).(*datePeriodIteratorData)
					if d == nil || d.pos < 0 || d.pos >= len(d.dates) {
						return phpv.ZBool(false).ZVal(), nil
					}
					return phpv.ZBool(true).ZVal(), nil
				}),
			},
		},
	}
}

// datePeriodGetIterator implements DatePeriod::getIterator()
func datePeriodGetIterator(ctx phpv.Context, this *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	dates, err := datePeriodGenerateDates(ctx, this)
	if err != nil {
		return nil, err
	}

	iterObj, err := phpobj.NewZObject(ctx, DatePeriodIterator)
	if err != nil {
		return nil, err
	}
	iterObj.SetOpaque(DatePeriodIterator, &datePeriodIteratorData{dates: dates, pos: 0})
	return iterObj.ZVal(), nil
}

// datePeriodGenerateDates generates all dates in a DatePeriod
func datePeriodGenerateDates(ctx phpv.Context, this *phpobj.ZObject) ([]*phpv.ZVal, error) {
	startVal, _ := this.ObjectGet(ctx, phpv.ZString("start"))
	intervalVal, _ := this.ObjectGet(ctx, phpv.ZString("interval"))
	endVal, _ := this.ObjectGet(ctx, phpv.ZString("end"))
	recurrencesVal, _ := this.ObjectGet(ctx, phpv.ZString("recurrences"))
	includeStartVal, _ := this.ObjectGet(ctx, phpv.ZString("include_start_date"))
	includeEndVal, _ := this.ObjectGet(ctx, phpv.ZString("include_end_date"))

	if startVal == nil || startVal.IsNull() || intervalVal == nil || intervalVal.IsNull() {
		return nil, nil
	}

	startObj, ok := startVal.Value().(*phpobj.ZObject)
	if !ok {
		return nil, nil
	}
	startTime, ok := getTime(startObj)
	if !ok {
		return nil, nil
	}

	intervalObj, ok := intervalVal.Value().(*phpobj.ZObject)
	if !ok {
		return nil, nil
	}

	includeStart := true
	if includeStartVal != nil && !includeStartVal.IsNull() {
		includeStart = bool(includeStartVal.AsBool(ctx))
	}

	includeEnd := false
	if includeEndVal != nil && !includeEndVal.IsNull() {
		includeEnd = bool(includeEndVal.AsBool(ctx))
	}

	var results []*phpv.ZVal
	current := startTime

	// Determine iteration limit
	hasEnd := endVal != nil && !endVal.IsNull()
	var endTime time.Time
	maxRecurrences := -1

	if hasEnd {
		endObj, ok := endVal.Value().(*phpobj.ZObject)
		if ok {
			endTime, _ = getTime(endObj)
		}
	}

	if recurrencesVal != nil && !recurrencesVal.IsNull() && recurrencesVal.GetType() == phpv.ZtInt {
		maxRecurrences = int(recurrencesVal.AsInt(ctx))
	}

	// Safety limit
	maxIter := 1000
	if maxRecurrences >= 0 && !hasEnd {
		maxIter = maxRecurrences + 1 // +1 because recurrences doesn't include start
	}

	count := 0
	for i := 0; i < maxIter; i++ {
		if i == 0 {
			// First iteration is the start date
			if includeStart {
				dateObj, err := phpobj.NewZObject(ctx, DateTimeImmutable)
				if err != nil {
					return nil, err
				}
				setTimeVal(dateObj, current)
				results = append(results, dateObj.ZVal())
				count++
			}
			// Advance to next
			current = addIntervalToTime(ctx, current, intervalObj, false)
			continue
		}

		// Check end condition
		if hasEnd {
			if current.After(endTime) || (!includeEnd && current.Equal(endTime)) {
				break
			}
			if includeEnd && current.Equal(endTime) {
				dateObj, err := phpobj.NewZObject(ctx, DateTimeImmutable)
				if err != nil {
					return nil, err
				}
				setTimeVal(dateObj, current)
				results = append(results, dateObj.ZVal())
				break
			}
		}

		if maxRecurrences >= 0 && !hasEnd && count > maxRecurrences {
			break
		}

		dateObj, err := phpobj.NewZObject(ctx, DateTimeImmutable)
		if err != nil {
			return nil, err
		}
		setTimeVal(dateObj, current)
		results = append(results, dateObj.ZVal())
		count++

		// Advance
		current = addIntervalToTime(ctx, current, intervalObj, false)
	}

	return results, nil
}

// datePeriodSetState implements DatePeriod::__set_state
func datePeriodSetState(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	if len(args) < 1 || args[0].GetType() != phpv.ZtArray {
		return nil, phpobj.ThrowError(ctx, phpobj.Error, "Invalid serialization data for DatePeriod object")
	}
	arr := args[0].Value().(*phpv.ZArray)
	start, _ := arr.OffsetGet(ctx, phpv.ZString("start").ZVal())
	interval, _ := arr.OffsetGet(ctx, phpv.ZString("interval").ZVal())
	end, _ := arr.OffsetGet(ctx, phpv.ZString("end").ZVal())
	recurrences, _ := arr.OffsetGet(ctx, phpv.ZString("recurrences").ZVal())

	var ctorArgs []*phpv.ZVal
	if start != nil && !start.IsNull() {
		ctorArgs = append(ctorArgs, start)
		if interval != nil && !interval.IsNull() {
			ctorArgs = append(ctorArgs, interval)
			if end != nil && !end.IsNull() {
				ctorArgs = append(ctorArgs, end)
			} else if recurrences != nil && !recurrences.IsNull() {
				ctorArgs = append(ctorArgs, recurrences)
			}
		}
	}
	if len(ctorArgs) == 0 {
		ctorArgs = []*phpv.ZVal{phpv.ZString("R1/2000-01-01T00:00:00Z/P1D").ZVal()}
	}
	obj, err := phpobj.NewZObject(ctx, DatePeriod, ctorArgs...)
	if err != nil {
		return nil, err
	}
	includeStart, _ := arr.OffsetGet(ctx, phpv.ZString("include_start_date").ZVal())
	if includeStart != nil {
		obj.ObjectSet(ctx, phpv.ZString("include_start_date"), includeStart)
	}
	includeEnd, _ := arr.OffsetGet(ctx, phpv.ZString("include_end_date").ZVal())
	if includeEnd != nil {
		obj.ObjectSet(ctx, phpv.ZString("include_end_date"), includeEnd)
	}
	return obj.ZVal(), nil
}

// datePeriodCreateFromISO8601String implements DatePeriod::createFromISO8601String
func datePeriodCreateFromISO8601String(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	if len(args) < 1 {
		return nil, phpobj.ThrowError(ctx, phpobj.TypeError, "DatePeriod::createFromISO8601String() expects exactly 1 argument, 0 given")
	}
	isoStr := string(args[0].AsString(ctx))
	options := 0
	if len(args) >= 2 {
		options = int(args[1].AsInt(ctx))
	}

	obj, err := phpobj.NewZObject(ctx, DatePeriod)
	if err != nil {
		return nil, err
	}
	_, err = datePeriodInitFromISO(ctx, obj, isoStr, options)
	if err != nil {
		return nil, err
	}
	return obj.ZVal(), nil
}

// dateTimeSetState implements DateTime::__set_state() and DateTimeImmutable::__set_state()
// It takes an array with "date", "timezone_type", and "timezone" keys.
func dateTimeSetState(targetClass *phpobj.ZClass) func(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	return func(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
		if len(args) < 1 {
			return nil, phpobj.ThrowError(ctx, phpobj.Error, fmt.Sprintf("%s::__set_state() expects exactly 1 argument, 0 given", targetClass.Name))
		}
		if args[0].GetType() != phpv.ZtArray {
			return nil, phpobj.ThrowError(ctx, phpobj.Error, fmt.Sprintf("Invalid serialization data for %s object", targetClass.Name))
		}
		arr := args[0].Value().(*phpv.ZArray)

		dateVal, _ := arr.OffsetGet(ctx, phpv.ZString("date").ZVal())
		if dateVal == nil || dateVal.IsNull() {
			return nil, phpobj.ThrowError(ctx, phpobj.Error, fmt.Sprintf("Invalid serialization data for %s object", targetClass.Name))
		}
		// date must be a string
		if dateVal.GetType() != phpv.ZtString {
			return nil, phpobj.ThrowError(ctx, phpobj.Error, fmt.Sprintf("Invalid serialization data for %s object", targetClass.Name))
		}
		dateStr := string(dateVal.AsString(ctx))

		// timezone_type must be present and be an integer (not float)
		tzTypeVal, _ := arr.OffsetGet(ctx, phpv.ZString("timezone_type").ZVal())
		if tzTypeVal == nil || tzTypeVal.IsNull() {
			return nil, phpobj.ThrowError(ctx, phpobj.Error, fmt.Sprintf("Invalid serialization data for %s object", targetClass.Name))
		}
		if tzTypeVal.GetType() == phpv.ZtFloat {
			return nil, phpobj.ThrowError(ctx, phpobj.Error, fmt.Sprintf("Invalid serialization data for %s object", targetClass.Name))
		}
		tzType := int(tzTypeVal.AsInt(ctx))

		// timezone must be a string
		tzVal, _ := arr.OffsetGet(ctx, phpv.ZString("timezone").ZVal())
		if tzVal == nil || tzVal.IsNull() {
			return nil, phpobj.ThrowError(ctx, phpobj.Error, fmt.Sprintf("Invalid serialization data for %s object", targetClass.Name))
		}
		if tzVal.GetType() != phpv.ZtString {
			return nil, phpobj.ThrowError(ctx, phpobj.Error, fmt.Sprintf("Invalid serialization data for %s object", targetClass.Name))
		}
		tzStr := string(tzVal.AsString(ctx))

		// Validate timezone_type and timezone
		var loc *time.Location
		switch tzType {
		case 1: // UTC offset
			offset, ok := parseTZOffset(tzStr)
			if !ok {
				return nil, phpobj.ThrowError(ctx, phpobj.Error, fmt.Sprintf("Invalid serialization data for %s object", targetClass.Name))
			}
			loc = time.FixedZone(tzStr, offset)
		case 2: // abbreviation
			if _, ok := timezoneAbbreviationOffsets[strings.ToUpper(tzStr)]; !ok {
				return nil, phpobj.ThrowError(ctx, phpobj.Error, fmt.Sprintf("Invalid serialization data for %s object", targetClass.Name))
			}
			loc = time.FixedZone(tzStr, timezoneAbbreviationOffsets[strings.ToUpper(tzStr)])
		case 3: // identifier
			var err error
			loc, err = time.LoadLocation(tzStr)
			if err != nil {
				return nil, phpobj.ThrowError(ctx, phpobj.Error, fmt.Sprintf("Invalid serialization data for %s object", targetClass.Name))
			}
		default:
			return nil, phpobj.ThrowError(ctx, phpobj.Error, fmt.Sprintf("Invalid serialization data for %s object", targetClass.Name))
		}

		// Parse date string - the format from __debugInfo is "2006-01-02 15:04:05.000000"
		parsed, err := time.ParseInLocation("2006-01-02 15:04:05.000000", dateStr, loc)
		if err != nil {
			// Try without microseconds
			parsed, err = time.ParseInLocation("2006-01-02 15:04:05", dateStr, loc)
			if err != nil {
				return nil, phpobj.ThrowError(ctx, phpobj.Error, fmt.Sprintf("Invalid serialization data for %s object", targetClass.Name))
			}
		}

		obj, err2 := phpobj.NewZObject(ctx, targetClass)
		if err2 != nil {
			return nil, err2
		}
		setTimeVal(obj, parsed)
		return obj.ZVal(), nil
	}
}

func dateIntervalConstruct(ctx phpv.Context, this *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	if len(args) < 1 {
		// When called internally (e.g. from diff()) with no args, just return with defaults
		// Mark as initialized
		this.SetOpaque(DateInterval, true)
		return nil, nil
	}
	// Mark as initialized
	this.SetOpaque(DateInterval, true)
	spec := string(args[0].AsString(ctx))
	// Parse ISO 8601 duration: P1Y2M3DT4H5M6S
	if len(spec) == 0 {
		return nil, phpobj.ThrowError(ctx, DateMalformedIntervalStringException, "Unknown or bad format ("+spec+")")
	}
	if spec[0] != 'P' {
		// Not a proper ISO 8601 duration - could be a datetime string
		return nil, phpobj.ThrowError(ctx, DateMalformedIntervalStringException, "Failed to parse interval ("+spec+")")
	}
	if len(spec) < 2 {
		return nil, phpobj.ThrowError(ctx, DateMalformedIntervalStringException, "Unknown or bad format ("+spec+")")
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
	if err := checkDateIntervalInitialized(ctx, this); err != nil {
		return nil, err
	}
	format := string(args[0].AsString(ctx))
	ht := this.HashTable()
	
	result := ""
	for i := 0; i < len(format); i++ {
		if format[i] == '%' {
			// A trailing '%' with no specifier is consumed silently (PHP returns "")
			if i+1 >= len(format) {
				break
			}
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
			case 'F':
				fVal := ht.GetString("f")
				if fVal != nil && !fVal.IsNull() {
					result += fmt.Sprintf("%06d", int(fVal.AsFloat(ctx)*1000000))
				} else {
					result += "000000"
				}
			case 'f':
				fVal := ht.GetString("f")
				if fVal != nil && !fVal.IsNull() {
					result += fmt.Sprintf("%d", int(fVal.AsFloat(ctx)*1000000))
				} else {
					result += "0"
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

// createDateIntervalFromString creates a DateInterval from a relative date string
// like "2 days", "1 month 3 days", "next thursday", etc.
func createDateIntervalFromString(ctx phpv.Context, dateStr string) (*phpv.ZVal, error) {
	obj, err := phpobj.NewZObject(ctx, DateInterval)
	if err != nil {
		return nil, err
	}

	// Store the from_string flag and original string
	obj.HashTable().SetString("from_string", phpv.ZBool(true).ZVal())
	obj.HashTable().SetString("date_string", phpv.ZString(dateStr).ZVal())

	// Parse relative date strings
	// Parse simple formats like "N unit" (e.g., "2 days", "1 month")
	trimmed := strings.TrimSpace(strings.ToLower(dateStr))
	parts := strings.Fields(trimmed)

	parsed := false
	for i := 0; i < len(parts); i++ {
		num := 0
		// Try to parse a number
		if n, err := fmt.Sscanf(parts[i], "%d", &num); err == nil && n == 1 {
			if i+1 < len(parts) {
				i++
				unit := parts[i]
				switch {
				case strings.HasPrefix(unit, "year"):
					obj.HashTable().SetString("y", phpv.ZInt(num).ZVal())
					parsed = true
				case strings.HasPrefix(unit, "month"):
					obj.HashTable().SetString("m", phpv.ZInt(num).ZVal())
					parsed = true
				case strings.HasPrefix(unit, "day"):
					obj.HashTable().SetString("d", phpv.ZInt(num).ZVal())
					parsed = true
				case strings.HasPrefix(unit, "hour"):
					obj.HashTable().SetString("h", phpv.ZInt(num).ZVal())
					parsed = true
				case strings.HasPrefix(unit, "minute") || strings.HasPrefix(unit, "min"):
					obj.HashTable().SetString("i", phpv.ZInt(num).ZVal())
					parsed = true
				case strings.HasPrefix(unit, "second") || strings.HasPrefix(unit, "sec"):
					obj.HashTable().SetString("s", phpv.ZInt(num).ZVal())
					parsed = true
				case strings.HasPrefix(unit, "week"):
					obj.HashTable().SetString("d", phpv.ZInt(num*7).ZVal())
					parsed = true
				}
			}
		}
	}

	if !parsed && len(trimmed) > 0 {
		// Try to use strtotime to parse relative expressions
		_, stErr := strtotime.StrToTime(dateStr)
		if stErr != nil {
			return nil, phpobj.ThrowError(ctx, DateMalformedIntervalStringException,
				fmt.Sprintf("Unknown or bad format (%s) at position 0 (%s): The timezone could not be found in the database",
					dateStr, string(dateStr[0:1])))
		}
	}

	return obj.ZVal(), nil
}
