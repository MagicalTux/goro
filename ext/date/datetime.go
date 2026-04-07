package date

import (
	"fmt"
	"math"
	"regexp"
	"strconv"
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
	className := obj.Class.GetName()
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

// checkDateTimeInterfaceInitialized is like checkDateTimeInitialized but uses "DateTimeInterface"
// as the type name in the error message, as expected when validating DatePeriod constructor args.
func checkDateTimeInterfaceInitialized(ctx phpv.Context, obj *phpobj.ZObject) error {
	if isDateTimeInitialized(obj) {
		return nil
	}
	return phpobj.ThrowError(ctx, DateObjectError,
		"Object of type DateTimeInterface has not been correctly initialized by calling parent::__construct() in its constructor")
}

// isDateIntervalInitialized checks if a DateInterval object has been properly initialized.
func isDateIntervalInitialized(obj *phpobj.ZObject) bool {
	_, ok := obj.Opaque[DateInterval]
	return ok
}

// checkDateIntervalInitialized throws DateObjectError if the DateInterval object is not initialized.
func checkDateIntervalInitialized(ctx phpv.Context, obj *phpobj.ZObject) error {
	// A DateInterval is initialized if its constructor was called (opaque key set).
	if isDateIntervalInitialized(obj) {
		return nil
	}
	className := obj.Class.GetName()
	if string(className) == "DateInterval" {
		return phpobj.ThrowError(ctx, DateObjectError,
			fmt.Sprintf("Object of type DateInterval has not been correctly initialized by calling parent::__construct() in its constructor"))
	}
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
	className := obj.Class.GetName()
	if string(className) == "DatePeriod" {
		return phpobj.ThrowError(ctx, DateObjectError,
			fmt.Sprintf("Object of type DatePeriod has not been correctly initialized by calling parent::__construct() in its constructor"))
	}
	return phpobj.ThrowError(ctx, DateObjectError,
		fmt.Sprintf("Object of type %s (inheriting DatePeriod) has not been correctly initialized by calling parent::__construct() in its constructor", className))
}

// getCalledClassForStatic gets the late-static-binding class from context, or returns the fallback.
func getCalledClassForStatic(ctx phpv.Context, fallback *phpobj.ZClass) *phpobj.ZClass {
	// Try the current context first (for static method calls, the class is set on callCtx)
	if cc, ok := ctx.(interface{ CalledClass() phpv.ZClass }); ok {
		if called := cc.CalledClass(); called != nil {
			if zc, ok := called.(*phpobj.ZClass); ok {
				return zc
			}
		}
	}
	// Then try the parent context
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
			if tzLoc, tzOk := getTimezoneLoc(tzObj); tzOk {
				loc = tzLoc
			} else if tzObj.Class.InstanceOf(DateTimeZone) {
				// DateTimeZone object but not initialized - throw Error
				return time.Time{}, phpobj.ThrowError(ctx, phpobj.Error,
					"The DateTimeZone object has not been correctly initialized by its constructor")
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

		base := time.Now().In(loc)
		// Use strtotime library for all date/time parsing
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
			locName = "+00:00"
			tzType = 1
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
		return nil, phpobj.ThrowError(ctx, phpobj.ArgumentCountError, "DateTime::format() expects exactly 1 argument, 0 given")
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
		return nil, phpobj.ThrowError(ctx, phpobj.ArgumentCountError, "DateTime::modify() expects exactly 1 argument, 0 given")
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
	// Handle @timestamp: set to Unix timestamp in UTC (timezone type 1)
	if len(modifier) > 1 && modifier[0] == '@' {
		if ts, err := strconv.ParseInt(strings.TrimSpace(string(modifier[1:])), 10, 64); err == nil {
			setTimeVal(this, time.Unix(ts, 0).UTC())
			return this.ZVal(), nil
		}
	}
	newT, stErr := strtotime.StrToTime(string(modifier), strtotime.InTZ(t.Location()), strtotime.Rel(t))
	if stErr != nil {
		return nil, phpobj.ThrowError(ctx, DateMalformedStringException, fmt.Sprintf("DateTime::modify(): Failed to parse time string (%s) at position 0 (%s): Unexpected character", modifier, string(modifier[0:1])))
	}
	setTimeVal(this, newT)
	return this.ZVal(), nil
}

// modifyImmutableMethod implements DateTimeImmutable::modify() - returns new instance
func modifyImmutableMethod(ctx phpv.Context, this *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	if len(args) < 1 {
		return nil, phpobj.ThrowError(ctx, phpobj.ArgumentCountError, "DateTimeImmutable::modify() expects exactly 1 argument, 0 given")
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
	// Handle @timestamp: set to Unix timestamp in UTC (timezone type 1)
	if len(modifier) > 1 && modifier[0] == '@' {
		if ts, err := strconv.ParseInt(strings.TrimSpace(string(modifier[1:])), 10, 64); err == nil {
			newObj, err := phpobj.NewZObject(ctx, this.Class)
			if err != nil {
				return nil, err
			}
			setTimeVal(newObj, time.Unix(ts, 0).UTC())
			return newObj.ZVal(), nil
		}
	}
	newT, stErr := strtotime.StrToTime(string(modifier), strtotime.InTZ(t.Location()), strtotime.Rel(t))
	if stErr != nil {
		return nil, phpobj.ThrowError(ctx, DateMalformedStringException, fmt.Sprintf("DateTimeImmutable::modify(): Failed to parse time string (%s) at position 0 (%s): Unexpected character", modifier, string(modifier[0:1])))
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
		return nil, phpobj.ThrowError(ctx, phpobj.ArgumentCountError, "DateTime::setDate() expects exactly 3 arguments")
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
		return nil, phpobj.ThrowError(ctx, phpobj.ArgumentCountError, "DateTimeImmutable::setDate() expects exactly 3 arguments")
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
		return nil, phpobj.ThrowError(ctx, phpobj.ArgumentCountError, "DateTime::setTime() expects at least 2 arguments")
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
		return nil, phpobj.ThrowError(ctx, phpobj.ArgumentCountError, "DateTimeImmutable::setTime() expects at least 2 arguments")
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

// getMicrosecondMethod implements DateTime::getMicrosecond(): int
func getMicrosecondMethod(ctx phpv.Context, this *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	if err := checkDateTimeInitialized(ctx, this); err != nil {
		return nil, err
	}
	t, ok := getTime(this)
	if !ok {
		return phpv.ZBool(false).ZVal(), nil
	}
	return phpv.ZInt(t.Nanosecond() / 1000).ZVal(), nil
}

// setMicrosecondMethod implements DateTime::setMicrosecond(int $microsecond): static
func setMicrosecondMethod(ctx phpv.Context, this *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	if len(args) < 1 {
		return nil, phpobj.ThrowError(ctx, phpobj.ArgumentCountError, "DateTime::setMicrosecond() expects exactly 1 argument")
	}
	if err := checkDateTimeInitialized(ctx, this); err != nil {
		return nil, err
	}
	micro := int(args[0].AsInt(ctx))
	if micro < 0 || micro > 999999 {
		return nil, phpobj.ThrowError(ctx, DateRangeError, fmt.Sprintf("DateTime::setMicrosecond(): Argument #1 ($microsecond) must be between 0 and 999999, %d given", micro))
	}
	t, ok := getTime(this)
	if !ok {
		return phpv.ZBool(false).ZVal(), nil
	}
	y, mo, d := t.Date()
	newT := time.Date(y, mo, d, t.Hour(), t.Minute(), t.Second(), micro*1000, t.Location())
	setTimeVal(this, newT)
	return this.ZVal(), nil
}

// setMicrosecondImmutableMethod implements DateTimeImmutable::setMicrosecond(int $microsecond): static
func setMicrosecondImmutableMethod(ctx phpv.Context, this *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	if len(args) < 1 {
		return nil, phpobj.ThrowError(ctx, phpobj.ArgumentCountError, "DateTimeImmutable::setMicrosecond() expects exactly 1 argument")
	}
	if err := checkDateTimeInitialized(ctx, this); err != nil {
		return nil, err
	}
	micro := int(args[0].AsInt(ctx))
	if micro < 0 || micro > 999999 {
		return nil, phpobj.ThrowError(ctx, DateRangeError, fmt.Sprintf("DateTimeImmutable::setMicrosecond(): Argument #1 ($microsecond) must be between 0 and 999999, %d given", micro))
	}
	t, ok := getTime(this)
	if !ok {
		return phpv.ZBool(false).ZVal(), nil
	}
	y, mo, d := t.Date()
	newT := time.Date(y, mo, d, t.Hour(), t.Minute(), t.Second(), micro*1000, t.Location())
	actualClass := this.Class
	newObj, err := phpobj.NewZObject(ctx, actualClass)
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
		return nil, phpobj.ThrowError(ctx, phpobj.ArgumentCountError, "DateTime::setTimezone() expects exactly 1 argument, 0 given")
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
		return nil, phpobj.ThrowError(ctx, phpobj.ArgumentCountError, "DateTime::setTimezone() expects parameter 1 to be DateTimeZone")
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
		return nil, phpobj.ThrowError(ctx, phpobj.ArgumentCountError, "DateTimeImmutable::setTimezone() expects exactly 1 argument, 0 given")
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
		return nil, phpobj.ThrowError(ctx, phpobj.ArgumentCountError, "DateTimeImmutable::setTimezone() expects parameter 1 to be DateTimeZone")
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
	locStr := t.Location().String()
	if locStr == "" {
		// When strtotime parses a numeric offset like "+05:00", Go returns
		// time.FixedZone("", offset) — the location name is empty.
		// Reconstruct the offset string from the actual zone offset.
		_, offset := t.Zone()
		sign := "+"
		if offset < 0 {
			sign = "-"
			offset = -offset
		}
		hours := offset / 3600
		mins := (offset % 3600) / 60
		locStr = fmt.Sprintf("%s%02d:%02d", sign, hours, mins)
	}
	tzObj, err := phpobj.NewZObject(ctx, DateTimeZone, phpv.ZString(locStr).ZVal())
	if err != nil {
		return nil, err
	}
	return tzObj.ZVal(), nil
}

// setTimestampMethod implements DateTime::setTimestamp(int $timestamp): DateTime
func setTimestampMethod(ctx phpv.Context, this *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	if len(args) < 1 {
		return nil, phpobj.ThrowError(ctx, phpobj.ArgumentCountError, "DateTime::setTimestamp() expects exactly 1 argument, 0 given")
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
		return nil, phpobj.ThrowError(ctx, phpobj.ArgumentCountError, "DateTimeImmutable::setTimestamp() expects exactly 1 argument, 0 given")
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
			return nil, phpobj.ThrowError(ctx, phpobj.ArgumentCountError, fmt.Sprintf("%s::createFromFormat() expects at least 2 arguments", targetClass.Name))
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

		actualClass := getCalledClassForStatic(ctx, targetClass)
		obj, err := phpobj.NewZObject(ctx, actualClass)
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
		return nil, phpobj.ThrowError(ctx, phpobj.ArgumentCountError, "DateTime::diff() expects at least 1 parameter, 0 given")
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
		return nil, phpobj.ThrowError(ctx, phpobj.TypeError, fmt.Sprintf("DateTime::diff() expects parameter 1 to be DateTimeInterface, %s given", args[0].GetType()))
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

	// Compute remaining hours/minutes/seconds.
	// PHP uses a hybrid approach:
	// - When there are calendar date differences (days/months/years > 0), use
	//   wall-clock time-of-day arithmetic. This ensures "1 day" means one
	//   calendar day regardless of DST transitions.
	// - When dates are the same (days/months/years == 0), use UTC elapsed time.
	//   This correctly handles DST fold periods where the same wall-clock time
	//   occurs twice.
	var remainSec int
	if days > 0 || months > 0 || years > 0 {
		// Multi-day: wall-clock time-of-day difference.
		// When from and to are in different timezone objects (type 1/2: fixed offsets
		// like EDT and EST), PHP includes the UTC offset difference in the time calc.
		// When they share the same IANA timezone (type 3: America/New_York), the
		// wall-clock time already reflects the DST transition, so no adjustment.
		fromSod := from.Hour()*3600 + from.Minute()*60 + from.Second()
		toSod := to.Hour()*3600 + to.Minute()*60 + to.Second()
		dstAdj := 0
		if from.Location().String() != to.Location().String() {
			_, fromOff := from.Zone()
			_, toOff := to.Zone()
			dstAdj = fromOff - toOff
		}
		remainSec = toSod - fromSod + dstAdj
		if remainSec < 0 {
			// Borrow one day
			remainSec += 86400
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
			// After borrowing, if all calendar parts are zero, fall back to
			// UTC elapsed time to handle DST fold correctly
			if days == 0 && months == 0 && years == 0 {
				remainSec = int(to.Unix() - from.Unix())
			}
		}
	} else {
		// Same calendar date: use UTC elapsed time
		remainSec = int(to.Unix() - from.Unix())
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

	// Calculate microsecond fraction
	fromUsec := from.Nanosecond() / 1000
	toUsec := to.Nanosecond() / 1000
	usecDiff := toUsec - fromUsec
	if usecDiff < 0 {
		usecDiff += 1000000
		seconds--
		if seconds < 0 {
			seconds += 60
			minutes--
			if minutes < 0 {
				minutes += 60
				hours--
			}
		}
	}
	fraction := float64(usecDiff) / 1000000.0

	intervalObj.ObjectSet(ctx, phpv.ZString("y"), phpv.ZInt(years).ZVal())
	intervalObj.ObjectSet(ctx, phpv.ZString("m"), phpv.ZInt(months).ZVal())
	intervalObj.ObjectSet(ctx, phpv.ZString("d"), phpv.ZInt(days).ZVal())
	intervalObj.ObjectSet(ctx, phpv.ZString("h"), phpv.ZInt(hours).ZVal())
	intervalObj.ObjectSet(ctx, phpv.ZString("i"), phpv.ZInt(minutes).ZVal())
	intervalObj.ObjectSet(ctx, phpv.ZString("s"), phpv.ZInt(seconds).ZVal())
	intervalObj.ObjectSet(ctx, phpv.ZString("f"), phpv.ZFloat(fraction).ZVal())
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

	// Handle from_string mode (DateInterval::createFromDateString)
	fromStrVal := ht.GetString("from_string")
	if fromStrVal != nil && bool(fromStrVal.AsBool(ctx)) {
		dateStrVal := ht.GetString("date_string")
		if dateStrVal != nil && !dateStrVal.IsNull() {
			dateStr := string(dateStrVal.AsString(ctx))
			// Try strtotime library first, then fall back to custom parser
			newT, stErr := strtotime.StrToTime(dateStr, strtotime.InTZ(t.Location()), strtotime.Rel(t))
			if stErr == nil {
				return newT
			}
		}
	}

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
	if years != 0 || months != 0 || days != 0 {
		origH, origM, origS := t.Clock()
		t = t.AddDate(years, months, days)

		// Check if AddDate landed in a DST gap (spring forward).
		// Go normalizes backward to the pre-gap time; PHP normalizes forward.
		// Detect by checking if the wall-clock time changed unexpectedly.
		newH, newM, newS := t.Clock()
		if newH != origH || newM != origM || newS != origS {
			_, curOff := t.Zone()
			// Probe a time known to be past any potential gap
			probe := time.Date(t.Year(), t.Month(), t.Day(), origH+2, 0, 0, 0, t.Location())
			_, probeOff := probe.Zone()
			if probeOff != curOff {
				// DST gap: shift forward by the gap size
				t = t.Add(time.Duration(probeOff-curOff) * time.Second)
			}
		}
	}

	// Then apply time part (uses real seconds, correctly handles DST)
	t = t.Add(time.Duration(hours)*time.Hour +
		time.Duration(minutes)*time.Minute +
		time.Duration(seconds)*time.Second +
		time.Duration(microseconds)*time.Microsecond)
	return t
}

// addMethod implements DateTime::add(DateInterval $interval): DateTime
func addMethod(ctx phpv.Context, this *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	if len(args) < 1 {
		return nil, phpobj.ThrowError(ctx, phpobj.ArgumentCountError, "DateTime::add() expects exactly 1 argument, 0 given")
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
		return nil, phpobj.ThrowError(ctx, phpobj.ArgumentCountError, "DateTime::add() expects parameter 1 to be DateInterval")
	}
	newT := addIntervalToTime(ctx, t, intervalObj, false)
	setTimeVal(this, newT)
	return this.ZVal(), nil
}

// addImmutableMethod implements DateTimeImmutable::add() - returns new instance
func addImmutableMethod(ctx phpv.Context, this *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	if len(args) < 1 {
		return nil, phpobj.ThrowError(ctx, phpobj.ArgumentCountError, "DateTimeImmutable::add() expects exactly 1 argument, 0 given")
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
		return nil, phpobj.ThrowError(ctx, phpobj.ArgumentCountError, "DateTimeImmutable::add() expects parameter 1 to be DateInterval")
	}
	newT := addIntervalToTime(ctx, t, intervalObj, false)
	newObj, err := phpobj.NewZObject(ctx, DateTimeImmutable)
	if err != nil {
		return nil, err
	}
	setTimeVal(newObj, newT)
	return newObj.ZVal(), nil
}

// checkSubInterval checks if a DateInterval can be used with sub(). Intervals created with
// createFromDateString (from_string=true) cannot be subtracted from dates.
func checkSubInterval(ctx phpv.Context, intervalObj *phpobj.ZObject, className string) error {
	fromStrVal := intervalObj.HashTable().GetString("from_string")
	if fromStrVal != nil && bool(fromStrVal.AsBool(ctx)) {
		return phpobj.ThrowError(ctx, DateInvalidOperationException,
			fmt.Sprintf("%s::sub(): Only non-special relative time specifications are supported for subtraction", className))
	}
	return nil
}

// subMethod implements DateTime::sub(DateInterval $interval): DateTime
func subMethod(ctx phpv.Context, this *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	if len(args) < 1 {
		return nil, phpobj.ThrowError(ctx, phpobj.ArgumentCountError, "DateTime::sub() expects exactly 1 argument, 0 given")
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
		return nil, phpobj.ThrowError(ctx, phpobj.ArgumentCountError, "DateTime::sub() expects parameter 1 to be DateInterval")
	}
	if err := checkSubInterval(ctx, intervalObj, "DateTime"); err != nil {
		return nil, err
	}
	newT := addIntervalToTime(ctx, t, intervalObj, true)
	setTimeVal(this, newT)
	return this.ZVal(), nil
}

// subImmutableMethod implements DateTimeImmutable::sub() - returns new instance
func subImmutableMethod(ctx phpv.Context, this *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	if len(args) < 1 {
		return nil, phpobj.ThrowError(ctx, phpobj.ArgumentCountError, "DateTimeImmutable::sub() expects exactly 1 argument, 0 given")
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
		return nil, phpobj.ThrowError(ctx, phpobj.ArgumentCountError, "DateTimeImmutable::sub() expects parameter 1 to be DateInterval")
	}
	if err := checkSubInterval(ctx, intervalObj, "DateTimeImmutable"); err != nil {
		return nil, err
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
		return nil, phpobj.ThrowError(ctx, phpobj.ArgumentCountError, "DateTime::setISODate() expects at least 2 arguments")
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
		return nil, phpobj.ThrowError(ctx, phpobj.ArgumentCountError, "DateTimeImmutable::setISODate() expects at least 2 arguments")
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
		// Use a fixed zone "+00:00" (type 1) not Go's UTC (type 3) per PHP behavior
		utcOffset := time.FixedZone("+00:00", 0)
		if val.GetType() == phpv.ZtFloat {
			f := float64(val.AsFloat(ctx))
			if math.IsNaN(f) || math.IsInf(f, 0) {
				return nil, phpobj.ThrowError(ctx, phpobj.ValueError, fmt.Sprintf("%s::createFromTimestamp(): Argument #1 ($timestamp) must be a finite number, %s given", targetClass.Name, val.AsString(ctx)))
			}
			// PHP rounds to microsecond precision (6 decimal places)
			usec := math.Round(f * 1e6)
			sec := int64(usec / 1e6)
			usecRemainder := int64(usec) - sec*1e6
			if usecRemainder < 0 {
				sec--
				usecRemainder += 1e6
			}
			nsec := usecRemainder * 1000
			t = time.Unix(sec, nsec).In(utcOffset)
		} else {
			ts := val.AsInt(ctx)
			t = time.Unix(int64(ts), 0).In(utcOffset)
		}

		actualClass := getCalledClassForStatic(ctx, targetClass)
		obj, err := phpobj.NewZObject(ctx, actualClass)
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

	// Include user-defined properties from subclasses (after standard props)
	appendSubclassProps(ctx, this, arr, map[string]bool{"date": true, "timezone_type": true, "timezone": true})

	return arr.ZVal(), nil
}

// resolveTimezoneForSerialization resolves a timezone string from serialized data.
// It handles named zones (Europe/London), abbreviations (CEST, BST), and offsets (+01:30, +0130).
func resolveTimezoneForSerialization(tzStr string) *time.Location {
	// Try as named location first
	if l, err := time.LoadLocation(tzStr); err == nil {
		return l
	}
	// Try as offset (+01:30, -05:00, +0130)
	if len(tzStr) >= 2 && (tzStr[0] == '+' || tzStr[0] == '-') {
		if offset, ok := parseTZOffset(tzStr); ok {
			// Normalize the name to +HH:MM format
			sign := "+"
			absOffset := offset
			if offset < 0 {
				sign = "-"
				absOffset = -offset
			}
			hours := absOffset / 3600
			mins := (absOffset % 3600) / 60
			name := fmt.Sprintf("%s%02d:%02d", sign, hours, mins)
			return time.FixedZone(name, offset)
		}
	}
	// Try as timezone abbreviation (CEST, BST, etc.)
	upper := strings.ToUpper(tzStr)
	if offset, ok := timezoneAbbreviationOffsets[upper]; ok {
		return time.FixedZone(tzStr, offset)
	}
	return time.UTC
}

// unserializeMethod implements DateTime::__unserialize(array $data)
func unserializeMethod(ctx phpv.Context, this *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	if len(args) < 1 || args[0].GetType() != phpv.ZtArray {
		return nil, phpobj.ThrowError(ctx, phpobj.Error, "Invalid serialization data for DateTime object")
	}
	arr := args[0].Value().(*phpv.ZArray)
	dateVal, _ := arr.OffsetGet(ctx, phpv.ZString("date").ZVal())
	if dateVal == nil || dateVal.IsNull() || dateVal.GetType() != phpv.ZtString || string(dateVal.AsString(ctx)) == "" {
		return nil, phpobj.ThrowError(ctx, phpobj.Error, "Invalid serialization data for DateTime object")
	}
	dateStr := string(dateVal.AsString(ctx))

	// Validate timezone_type: must be integer 1, 2, or 3
	tzTypeVal, _ := arr.OffsetGet(ctx, phpv.ZString("timezone_type").ZVal())
	if tzTypeVal != nil && !tzTypeVal.IsNull() {
		if tzTypeVal.GetType() != phpv.ZtInt {
			return nil, phpobj.ThrowError(ctx, phpobj.Error, "Invalid serialization data for DateTime object")
		}
		tzType := int(tzTypeVal.AsInt(ctx))
		if tzType < 1 || tzType > 3 {
			return nil, phpobj.ThrowError(ctx, phpobj.Error, "Invalid serialization data for DateTime object")
		}
	}

	tzVal, _ := arr.OffsetGet(ctx, phpv.ZString("timezone").ZVal())
	loc := time.UTC
	if tzVal != nil && !tzVal.IsNull() {
		tzStr := string(tzVal.AsString(ctx))
		loaded := resolveTimezoneForSerialization(tzStr)
		if loaded == time.UTC && tzStr != "UTC" && tzStr != "GMT" {
			if _, err := time.LoadLocation(tzStr); err != nil {
				if _, ok := timezoneAbbreviationOffsets[strings.ToUpper(tzStr)]; !ok {
					if len(tzStr) < 2 || (tzStr[0] != '+' && tzStr[0] != '-') {
						return nil, phpobj.ThrowError(ctx, phpobj.Error, "Invalid serialization data for DateTime object")
					}
				}
			}
		}
		loc = loaded
	}

	parsed, err := time.ParseInLocation("2006-01-02 15:04:05.000000", dateStr, loc)
	if err != nil {
		parsed, err = time.ParseInLocation("2006-01-02 15:04:05", dateStr, loc)
		if err != nil {
			return nil, phpobj.ThrowError(ctx, phpobj.Error, "Invalid serialization data for DateTime object")
		}
	}
	setTimeVal(this, parsed)

	// Restore custom properties from subclasses
	restoreSubclassProps(ctx, this, arr, map[string]bool{"date": true, "timezone_type": true, "timezone": true})

	return nil, nil
}

// wakeupMethod implements DateTime::__wakeup() - validates serialization
func wakeupMethod(ctx phpv.Context, this *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	dateVal := this.HashTable().GetString("date")
	if dateVal == nil || dateVal.IsNull() || dateVal.GetType() != phpv.ZtString || string(dateVal.AsString(ctx)) == "" {
		return nil, phpobj.ThrowError(ctx, phpobj.Error, "Invalid serialization data for DateTime object")
	}

	dateStr := string(dateVal.AsString(ctx))

	// Validate timezone_type: must be integer 1, 2, or 3
	tzTypeVal := this.HashTable().GetString("timezone_type")
	if tzTypeVal != nil && !tzTypeVal.IsNull() {
		if tzTypeVal.GetType() != phpv.ZtInt {
			return nil, phpobj.ThrowError(ctx, phpobj.Error, "Invalid serialization data for DateTime object")
		}
		tzType := int(tzTypeVal.AsInt(ctx))
		if tzType < 1 || tzType > 3 {
			return nil, phpobj.ThrowError(ctx, phpobj.Error, "Invalid serialization data for DateTime object")
		}
	}

	tzVal := this.HashTable().GetString("timezone")
	loc := time.UTC
	if tzVal != nil && !tzVal.IsNull() {
		tzStr := string(tzVal.AsString(ctx))
		// Validate: timezone must be loadable
		loaded := resolveTimezoneForSerialization(tzStr)
		// resolveTimezoneForSerialization returns UTC on failure; check if it actually loaded something valid
		// by verifying the timezone name or offset matches
		if loaded == time.UTC && tzStr != "UTC" && tzStr != "GMT" {
			// Try explicit load to see if the timezone is valid
			if _, err := time.LoadLocation(tzStr); err != nil {
				// Not a valid named timezone - check if it's a valid abbreviation
				if _, ok := timezoneAbbreviationOffsets[strings.ToUpper(tzStr)]; !ok {
					// Not a valid offset either - check if it starts with +/- for fixed offset
					if len(tzStr) < 2 || (tzStr[0] != '+' && tzStr[0] != '-') {
						return nil, phpobj.ThrowError(ctx, phpobj.Error, "Invalid serialization data for DateTime object")
					}
				}
			}
		}
		loc = loaded
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
				} else if l, err := parseTzName(tzName); err == nil {
					// Fall back to abbreviation lookup (e.g. PST, CET)
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
				// Don't return immediately - continue parsing the remaining format
				// to handle subsequent format chars like .u (microseconds)
				tFromU := time.Unix(ts, 0).In(loc)
				year = tFromU.Year()
				month = int(tFromU.Month())
				day = tFromU.Day()
				hour = tFromU.Hour()
				minute = tFromU.Minute()
				second = tFromU.Second()
				yearSet = true
				monthSet = true
				daySet = true
				hourSet = true
				minuteSet = true
				secondSet = true
				usedLoc = tFromU.Location()
				di = end
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

	// Include user-defined properties using IterProps for correct ordering and
	// PHP-standard mangling of private/protected property names. Skip the
	// standard DateTime properties (date, timezone_type, timezone) which are
	// added below from the opaque time value.
	standardProps := map[string]bool{"date": true, "timezone_type": true, "timezone": true}
	for prop := range this.IterProps(ctx) {
		if standardProps[string(prop.VarName)] {
			continue
		}
		var key phpv.ZString
		if prop.Modifiers.IsPrivate() {
			className := string(this.GetDeclClassName(prop))
			key = phpv.ZString("\x00" + className + "\x00" + string(prop.VarName))
		} else if prop.Modifiers.IsProtected() {
			key = phpv.ZString("\x00*\x00" + string(prop.VarName))
		} else {
			key = prop.VarName
		}
		v := this.GetPropValue(prop)
		if v != nil {
			arr.OffsetSet(ctx, key, v)
		}
	}
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

// restoreSubclassProps restores user-defined (non-standard) properties from a
// serialized array into an object. It handles conversion from PHP's NUL-byte
// mangling to internal property storage. This is used by __unserialize methods.
func restoreSubclassProps(ctx phpv.Context, obj *phpobj.ZObject, arr *phpv.ZArray, standardProps map[string]bool) {
	it := arr.NewIterator()
	for it.Valid(ctx) {
		k, _ := it.Key(ctx)
		v, _ := it.Current(ctx)
		// Skip integer keys - only string keys are property names
		if k.GetType() == phpv.ZtInt {
			it.Next(ctx)
			continue
		}
		ks := string(k.AsString(ctx))
		if standardProps[ks] {
			it.Next(ctx)
			continue
		}
		// Parse PHP-mangled property names: \0ClassName\0prop (private), \0*\0prop (protected)
		if len(ks) > 0 && ks[0] == '\x00' {
			secondNull := strings.IndexByte(ks[1:], '\x00')
			if secondNull >= 0 {
				classOrStar := ks[1 : secondNull+1]
				propName := phpv.ZString(ks[secondNull+2:])
				if classOrStar == "*" {
					// Protected: store under the bare name (or \0*\0name for undeclared)
					if _, found := obj.GetClass().GetProp(propName); found {
						obj.HashTable().ForceSetString(propName, v)
					} else {
						obj.HashTable().ForceSetString(phpv.ZString(ks), v)
					}
				} else {
					// Private: store under internal mangled name *ClassName:propName
					// Find the actual declaring class in the hierarchy
					var internalKey phpv.ZString
					if zclass, ok := obj.GetClass().(*phpobj.ZClass); ok {
						found := false
						for cl := zclass; cl != nil; cl = func() *phpobj.ZClass {
							if p := cl.GetParent(); p != nil {
								c, _ := p.(*phpobj.ZClass)
								return c
							}
							return nil
						}() {
							if string(cl.GetName()) == classOrStar {
								internalKey = phpobj.GetPrivatePropNameExt(cl, propName)
								found = true
								break
							}
						}
						if !found {
							// Class not found in hierarchy; use the object's own class
							internalKey = phpobj.GetPrivatePropNameExt(zclass, propName)
						}
					} else {
						// Fallback: use raw format
						internalKey = phpv.ZString(fmt.Sprintf("*%s:%s", classOrStar, propName))
					}
					obj.HashTable().ForceSetString(internalKey, v)
				}
				it.Next(ctx)
				continue
			}
		}
		// Non-mangled key: regular property
		obj.ObjectSet(ctx, phpv.ZString(ks), v)
		it.Next(ctx)
	}
}

// appendSubclassProps appends user-defined (non-standard) properties from a date
// object to the given array using PHP's NUL-byte mangling for private/protected
// properties. This is used by __serialize methods to include subclass properties
// in the serialized form.
func appendSubclassProps(ctx phpv.Context, obj *phpobj.ZObject, arr *phpv.ZArray, standardProps map[string]bool) {
	for prop := range obj.IterProps(ctx) {
		if standardProps[string(prop.VarName)] {
			continue
		}
		var key phpv.ZString
		if prop.Modifiers.IsPrivate() {
			className := string(obj.GetDeclClassName(prop))
			key = phpv.ZString("\x00" + className + "\x00" + string(prop.VarName))
		} else if prop.Modifiers.IsProtected() {
			key = phpv.ZString("\x00*\x00" + string(prop.VarName))
		} else {
			key = prop.VarName
		}
		v := obj.GetPropValue(prop)
		if v != nil {
			arr.OffsetSet(ctx, key, v)
		}
	}
}

// getTimezoneType returns the PHP timezone type for a location.
func getTimezoneType(loc *time.Location) int {
	name := loc.String()
	if name == "UTC" || name == "Local" {
		return 3
	}
	if len(name) > 0 && (name[0] == '+' || name[0] == '-') {
		return 1
	}
	if strings.Contains(name, "/") {
		return 3
	}
	if len(name) <= 6 {
		return 2
	}
	return 3
}

// dateTimeZoneCompare handles comparison of DateTimeZone objects.
func dateTimeZoneCompare(ctx phpv.Context, a, b phpv.ZObject) (int, error) {
	aObj, aOk := a.(*phpobj.ZObject)
	bObj, bOk := b.(*phpobj.ZObject)
	if !aOk || !bOk {
		return phpv.CompareUncomparable, nil
	}
	_, aInit := aObj.Opaque[DateTimeZone]
	_, bInit := bObj.Opaque[DateTimeZone]
	if !aInit || !bInit {
		return 0, phpobj.ThrowError(ctx, DateObjectError, "Trying to compare uninitialized DateTimeZone objects")
	}
	aLoc := aObj.Opaque[DateTimeZone].(*time.Location)
	bLoc := bObj.Opaque[DateTimeZone].(*time.Location)
	aType := getTimezoneType(aLoc)
	bType := getTimezoneType(bLoc)
	if aType != bType {
		return 0, phpobj.ThrowError(ctx, DateException, "Cannot compare two different kinds of DateTimeZone objects")
	}
	if aLoc.String() == bLoc.String() {
		return 0, nil
	}
	// Different timezone zones of the same type are not orderable (PHP returns 1 for !=,
	// but since < and > both return false for unequal timezones, use CompareUncomparable).
	return phpv.CompareUncomparable, nil
}

// dateTimeCompare handles comparison of DateTime and DateTimeImmutable objects.
// PHP compares by Unix timestamp (including sub-second precision via nanoseconds).
func dateTimeCompare(ctx phpv.Context, a, b phpv.ZObject) (int, error) {
	aTime, aOk := getTimeFromObj(a)
	bTime, bOk := getTimeFromObj(b)
	if !aOk || !bOk {
		return 0, phpobj.ThrowError(ctx, DateObjectError, "Trying to compare an incomplete DateTime or DateTimeImmutable object")
	}
	if aTime.Equal(bTime) {
		return 0, nil
	}
	if aTime.Before(bTime) {
		return -1, nil
	}
	return 1, nil
}

func init() {
	// DateTimeZone class
	DateTimeZone = &phpobj.ZClass{
		Name:  "DateTimeZone",
		Props: []*phpv.ZClassProp{},
		Const: map[phpv.ZString]*phpv.ZClassConst{
			"AFRICA":       {Value: phpv.ZInt(1)},
			"AMERICA":      {Value: phpv.ZInt(2)},
			"ANTARCTICA":   {Value: phpv.ZInt(4)},
			"ARCTIC":       {Value: phpv.ZInt(8)},
			"ASIA":         {Value: phpv.ZInt(16)},
			"ATLANTIC":     {Value: phpv.ZInt(32)},
			"AUSTRALIA":    {Value: phpv.ZInt(64)},
			"EUROPE":       {Value: phpv.ZInt(128)},
			"INDIAN":       {Value: phpv.ZInt(256)},
			"PACIFIC":      {Value: phpv.ZInt(512)},
			"UTC":          {Value: phpv.ZInt(1024)},
			"ALL":          {Value: phpv.ZInt(2047)},
			"ALL_WITH_BC":  {Value: phpv.ZInt(4095)},
			"PER_COUNTRY":  {Value: phpv.ZInt(4096)},
		},
		ConstOrder: []phpv.ZString{
			"AFRICA", "AMERICA", "ANTARCTICA", "ARCTIC", "ASIA", "ATLANTIC",
			"AUSTRALIA", "EUROPE", "INDIAN", "PACIFIC", "UTC", "ALL",
			"ALL_WITH_BC", "PER_COUNTRY",
		},
		H: &phpv.ZClassHandlers{HandleCompare: dateTimeZoneCompare},
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
					// Delegate to fncTimezoneTransitionsGet with $this prepended
					fullArgs := append([]*phpv.ZVal{this.ZVal()}, args...)
					return fncTimezoneTransitionsGet(ctx, fullArgs)
				}),
			},
			"getlocation": {
				Name:      "getLocation",
				Modifiers: phpv.ZAttrPublic,
				Method: phpobj.NativeMethod(func(ctx phpv.Context, this *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
					if err := checkDateTimeZoneInitialized(ctx, this); err != nil {
						return nil, err
					}
					return getTimezoneLocation(ctx, this)
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
			"__set_state": {
				Name:      "__set_state",
				Modifiers: phpv.ZAttrPublic | phpv.ZAttrStatic,
			},
			"__debuginfo": {
				Name:      "__debugInfo",
				Modifiers: phpv.ZAttrPublic,
				Method: phpobj.NativeMethod(func(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
					arr := phpv.NewZArray()
					// Include user-defined properties first (for subclasses)
					// Use IterProps for correct ordering and GetPropValue for
					// proper private property lookup. Keys use PHP-standard
					// NUL-byte mangling for private/protected visibility.
					for prop := range o.IterProps(ctx) {
						pName := prop.VarName
						if pName == "timezone_type" || pName == "timezone" {
							continue
						}
						var key phpv.ZString
						if prop.Modifiers.IsPrivate() {
							className := string(o.GetDeclClassName(prop))
							key = phpv.ZString("\x00" + className + "\x00" + string(pName))
						} else if prop.Modifiers.IsProtected() {
							key = phpv.ZString("\x00*\x00" + string(pName))
						} else {
							key = pName
						}
						v := o.GetPropValue(prop)
						if v != nil {
							arr.OffsetSet(ctx, key, v)
						}
					}
					// Then include timezone info from opaque data
					opaque := o.GetOpaque(DateTimeZone)
					if loc, ok := opaque.(*time.Location); ok && loc != nil {
						name := loc.String()
						// Determine timezone_type: 3=identifier, 2=abbreviation, 1=offset
						tzType := 3
						if len(name) > 0 && (name[0] == '+' || name[0] == '-') {
							tzType = 1
						} else if len(name) <= 5 && name != "Local" && name != "UTC" && !strings.Contains(name, "/") {
							tzType = 2
						}
						arr.OffsetSet(ctx, phpv.ZString("timezone_type"), phpv.ZInt(tzType).ZVal())
						arr.OffsetSet(ctx, phpv.ZString("timezone"), phpv.ZString(name).ZVal())
					}
					return arr.ZVal(), nil
				}),
			},
			"__serialize": {
				Name:      "__serialize",
				Modifiers: phpv.ZAttrPublic,
				Method: phpobj.NativeMethod(func(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
					// Throw if not initialized (constructor not called)
					if _, ok := o.Opaque[DateTimeZone]; !ok {
						className := o.Class.GetName()
						baseClass := "DateTimeZone"
						if string(className) == baseClass {
							return nil, phpobj.ThrowError(ctx, DateObjectError,
								fmt.Sprintf("Object of type %s has not been correctly initialized by calling parent::__construct() in its constructor", className))
						}
						return nil, phpobj.ThrowError(ctx, DateObjectError,
							fmt.Sprintf("Object of type %s (inheriting %s) has not been correctly initialized by calling parent::__construct() in its constructor", className, baseClass))
					}
					loc := o.Opaque[DateTimeZone].(*time.Location)
					name := loc.String()
					tzType := 3
					if len(name) > 0 && (name[0] == '+' || name[0] == '-') {
						tzType = 1
					} else if len(name) <= 5 && name != "Local" && name != "UTC" && !strings.Contains(name, "/") {
						tzType = 2
					}
					arr := phpv.NewZArray()
					arr.OffsetSet(ctx, phpv.ZString("timezone_type"), phpv.ZInt(tzType).ZVal())
					arr.OffsetSet(ctx, phpv.ZString("timezone"), phpv.ZString(name).ZVal())
					// Include user-defined properties from subclasses (after standard props)
					appendSubclassProps(ctx, o, arr, map[string]bool{"timezone_type": true, "timezone": true})
					return arr.ZVal(), nil
				}),
			},
			"__unserialize": {
				Name:      "__unserialize",
				Modifiers: phpv.ZAttrPublic,
				Method: phpobj.NativeMethod(func(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
					if len(args) < 1 || args[0].GetType() != phpv.ZtArray {
						return nil, nil
					}
					arr := args[0].Value().(*phpv.ZArray)
					tzVal, _ := arr.OffsetGet(ctx, phpv.ZString("timezone").ZVal())
					if tzVal == nil || tzVal.IsNull() {
						return nil, phpobj.ThrowError(ctx, phpobj.Error, "Invalid serialization data for DateTimeZone object")
					}
					tzName := tzVal.AsString(ctx)
					loc, err := parseTzName(string(tzName))
					if err != nil {
						return nil, phpobj.ThrowError(ctx, phpobj.Error, "Invalid serialization data for DateTimeZone object")
					}
					// Normalize offset timezone names to +HH:MM format (same as constructor)
					tzNameNorm := string(tzName)
					if len(tzNameNorm) >= 2 && (tzNameNorm[0] == '+' || tzNameNorm[0] == '-') {
						if offset, ok := parseTZOffset(tzNameNorm); ok {
							sign := "+"
							absOffset := offset
							if offset < 0 {
								sign = "-"
								absOffset = -offset
							}
							hours := absOffset / 3600
							mins := (absOffset % 3600) / 60
							tzNameNorm = fmt.Sprintf("%s%02d:%02d", sign, hours, mins)
							loc = time.FixedZone(tzNameNorm, offset)
						}
					}
					o.Opaque[DateTimeZone] = loc
					// Set hash table props for var_dump/var_export
					tzTypeVal, _ := arr.OffsetGet(ctx, phpv.ZString("timezone_type").ZVal())
					if tzTypeVal != nil && !tzTypeVal.IsNull() {
						o.HashTable().SetString("timezone_type", tzTypeVal)
					}
					o.HashTable().SetString("timezone", phpv.ZString(tzNameNorm).ZVal())
					// Restore any user-defined subclass properties
					restoreSubclassProps(ctx, o, arr, map[string]bool{"timezone": true, "timezone_type": true})
					return nil, nil
				}),
			},
		},
	}
	// Wire up DateTimeZone::__set_state which needs the class reference
	DateTimeZone.Methods["__set_state"].Method = phpobj.NativeStaticMethod(func(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
		if len(args) < 1 || args[0].GetType() != phpv.ZtArray {
			return nil, phpobj.ThrowError(ctx, phpobj.Error, "Invalid serialization data for DateTimeZone object")
		}
		arr := args[0].Value().(*phpv.ZArray)
		// timezone_type must be an integer (not float)
		tzTypeVal, _ := arr.OffsetGet(ctx, phpv.ZString("timezone_type").ZVal())
		if tzTypeVal == nil || tzTypeVal.GetType() != phpv.ZtInt {
			return nil, phpobj.ThrowError(ctx, phpobj.Error, "Invalid serialization data for DateTimeZone object")
		}
		tzType := int(tzTypeVal.Value().(phpv.ZInt))
		if tzType < 1 || tzType > 3 {
			return nil, phpobj.ThrowError(ctx, phpobj.Error, "Invalid serialization data for DateTimeZone object")
		}
		// timezone must be a string
		tzNameVal, _ := arr.OffsetGet(ctx, phpv.ZString("timezone").ZVal())
		if tzNameVal == nil || tzNameVal.GetType() != phpv.ZtString {
			return nil, phpobj.ThrowError(ctx, phpobj.Error, "Invalid serialization data for DateTimeZone object")
		}
		tzName := string(tzNameVal.Value().(phpv.ZString))
		// Must not contain null bytes
		if strings.Contains(tzName, "\x00") {
			return nil, phpobj.ThrowError(ctx, phpobj.Error, "Invalid serialization data for DateTimeZone object")
		}
		loc, err := parseTzName(tzName)
		if err != nil {
			return nil, phpobj.ThrowError(ctx, phpobj.Error, "Invalid serialization data for DateTimeZone object")
		}
		newObj, objErr := phpobj.NewZObject(ctx, DateTimeZone)
		if objErr != nil {
			return nil, objErr
		}
		newObj.Opaque[DateTimeZone] = loc
		newObj.HashTable().SetString("timezone_type", phpv.ZInt(tzType).ZVal())
		newObj.HashTable().SetString("timezone", phpv.ZString(tzName).ZVal())
		return newObj.ZVal(), nil
	})

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
		H: &phpv.ZClassHandlers{
			HandleCompare: func(ctx phpv.Context, a, b phpv.ZObject) (int, error) {
				ctx.Warn("Cannot compare DateInterval objects")
				return phpv.CompareUncomparable, nil
			},
			HandlePropGet: func(ctx phpv.Context, o phpv.ZObject, key phpv.ZString) (*phpv.ZVal, error) {
				// In from_string mode, y/m/d/h/i/s/f/invert/days are removed from
				// the hash table but should still be accessible with their default values.
				switch key {
				case "y", "m", "d", "h", "i", "s", "invert":
					return phpv.ZInt(0).ZVal(), nil
				case "f":
					return phpv.ZFloat(0).ZVal(), nil
				case "days":
					return phpv.ZBool(false).ZVal(), nil
				}
				return nil, nil
			},
		},
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
			"__debuginfo": {
				Name:      "__debugInfo",
				Modifiers: phpv.ZAttrPublic,
				Method: phpobj.NativeMethod(func(ctx phpv.Context, this *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
					arr := phpv.NewZArray()
					// Include user-defined properties first (for subclasses)
					standardProps := map[string]bool{
						"y": true, "m": true, "d": true, "h": true, "i": true, "s": true,
						"f": true, "invert": true, "days": true, "from_string": true, "date_string": true,
					}
					for prop := range this.IterProps(ctx) {
						if standardProps[string(prop.VarName)] {
							continue
						}
						var key phpv.ZString
						if prop.Modifiers.IsPrivate() {
							className := string(this.GetDeclClassName(prop))
							key = phpv.ZString("\x00" + className + "\x00" + string(prop.VarName))
						} else if prop.Modifiers.IsProtected() {
							key = phpv.ZString("\x00*\x00" + string(prop.VarName))
						} else {
							key = prop.VarName
						}
						v := this.GetPropValue(prop)
						if v != nil {
							arr.OffsetSet(ctx, key, v)
						}
					}
					// Then include standard DateInterval properties
					fromStr := this.HashTable().GetString("from_string")
					if fromStr != nil && bool(fromStr.AsBool(ctx)) {
						arr.OffsetSet(ctx, phpv.ZString("from_string"), phpv.ZBool(true).ZVal())
						ds := this.HashTable().GetString("date_string")
						if ds != nil {
							arr.OffsetSet(ctx, phpv.ZString("date_string"), ds)
						}
					} else {
						for _, key := range []string{"y", "m", "d", "h", "i", "s", "f", "invert", "days", "from_string"} {
							v := this.HashTable().GetString(phpv.ZString(key))
							if v != nil {
								arr.OffsetSet(ctx, phpv.ZString(key), v)
							}
						}
					}
					return arr.ZVal(), nil
				}),
			},
			"__unserialize": {
				Name:      "__unserialize",
				Modifiers: phpv.ZAttrPublic,
				Method: phpobj.NativeMethod(func(ctx phpv.Context, this *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
					if len(args) < 1 || args[0].GetType() != phpv.ZtArray {
						return nil, nil
					}
					arr := args[0].Value().(*phpv.ZArray)
					// Check if from_string mode (either from_string=true or date_string present without from_string)
					fromStr, _ := arr.OffsetGet(ctx, phpv.ZString("from_string").ZVal())
					ds, _ := arr.OffsetGet(ctx, phpv.ZString("date_string").ZVal())
					isFromString := (fromStr != nil && bool(fromStr.AsBool(ctx)))
					hasDatString := ds != nil && !ds.IsNull()

					// If date_string is present, validate it before modifying the object
					if hasDatString {
						dateStr := string(ds.AsString(ctx))
						_, stErr := strtotime.StrToTime(dateStr, strtotime.InTZ(getTimezone(ctx)), strtotime.Rel(time.Now().In(getTimezone(ctx))))
						if stErr != nil {
							ch := ""
							if len(dateStr) > 0 {
								ch = string([]rune(dateStr)[0:1])
							}
							return nil, phpobj.ThrowError(ctx, phpobj.Error,
								fmt.Sprintf("Unknown or bad format (%s) at position 0 (%s) while unserializing: The timezone could not be found in the database",
									dateStr, ch))
						}
					}

					// Mark as initialized (only after validation passes)
					this.SetOpaque(DateInterval, true)

					if isFromString {
						this.HashTable().SetString("from_string", phpv.ZBool(true).ZVal())
						if hasDatString {
							this.HashTable().SetString("date_string", ds)
						}
						// Remove non-from_string properties from hash table
						for _, key := range []string{"y", "m", "d", "h", "i", "s", "f", "invert", "days"} {
							this.HashTable().UnsetString(phpv.ZString(key))
						}
					} else if hasDatString {
						// date_string present but from_string not explicitly true
						this.HashTable().SetString("from_string", phpv.ZBool(true).ZVal())
						this.HashTable().SetString("date_string", ds)
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
					// Also handle any extra properties (subclass properties)
					restoreSubclassProps(ctx, this, arr, map[string]bool{
						"y": true, "m": true, "d": true, "h": true, "i": true, "s": true,
						"f": true, "invert": true, "days": true, "from_string": true, "date_string": true,
					})
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
					// Include custom properties from subclasses with proper PHP mangling
					appendSubclassProps(ctx, this, result, map[string]bool{
						"y": true, "m": true, "d": true, "h": true, "i": true, "s": true,
						"f": true, "invert": true, "days": true, "from_string": true, "date_string": true,
					})
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

					// Check if from_string is true or date_string is provided
					fromStr, _ := arr.OffsetGet(ctx, phpv.ZString("from_string").ZVal())
					ds, _ := arr.OffsetGet(ctx, phpv.ZString("date_string").ZVal())
					isFromString := (fromStr != nil && bool(fromStr.AsBool(ctx))) || (ds != nil && !ds.IsNull() && (fromStr == nil || fromStr.IsNull()))
					if isFromString {
						if ds != nil && !ds.IsNull() {
							dateStr := string(ds.AsString(ctx))
							// Validate the date string by attempting to parse it
							_, stErr := strtotime.StrToTime(dateStr, strtotime.InTZ(getTimezone(ctx)), strtotime.Rel(time.Now().In(getTimezone(ctx))))
							if stErr != nil {
								{
									// Find position and character of the error
									pos := 0
									ch := ""
									for i, c := range dateStr {
										if c < '0' || c > '9' {
											// Skip dashes at expected positions in YYYY-MM-DD format
											if c == '-' && (i == 4 || i == 7) {
												continue
											}
											// Skip spaces and colons in datetime parts
											if c == ' ' && i == 10 {
												continue
											}
											if c == ':' && (i == 13 || i == 16) {
												continue
											}
											pos = i
											ch = string(c)
											break
										}
									}
									return nil, phpobj.ThrowError(ctx, phpobj.Error,
										fmt.Sprintf("Unknown or bad format (%s) at position %d (%s) while unserializing: Unexpected character", dateStr, pos, ch))
								}
							}
							// Build DateInterval directly (don't use createDateIntervalFromString
							// which enforces relative-only - __set_state allows absolute datetimes)
							obj, objErr := phpobj.NewZObject(ctx, DateInterval)
							if objErr != nil {
								return nil, objErr
							}
							obj.SetOpaque(DateInterval, true)
							obj.HashTable().SetString("from_string", phpv.ZBool(true).ZVal())
							obj.HashTable().SetString("date_string", phpv.ZString(dateStr).ZVal())
							// Remove non-from_string properties
							for _, key := range []string{"y", "m", "d", "h", "i", "s", "f", "invert", "days"} {
								obj.HashTable().UnsetString(phpv.ZString(key))
							}
							return obj.ZVal(), nil
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
						return nil, phpobj.ThrowError(ctx, phpobj.ArgumentCountError, "DateInterval::createFromDateString() expects exactly 1 argument")
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
		Const:           map[phpv.ZString]*phpv.ZClassConst{},
		H:               &phpv.ZClassHandlers{HandleCompare: dateTimeCompare},
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
			"getmicrosecond": {
				Name:      "getMicrosecond",
				Modifiers: phpv.ZAttrPublic,
				Method:    phpobj.NativeMethod(getMicrosecondMethod),
			},
			"setmicrosecond": {
				Name:      "setMicrosecond",
				Modifiers: phpv.ZAttrPublic,
				Method:    phpobj.NativeMethod(setMicrosecondMethod),
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
		// Check that source object is a DateTimeImmutable (not just any DateTime)
		if zo, ok := srcObj.(*phpobj.ZObject); ok {
			if !zo.GetClass().InstanceOf(DateTimeImmutable) {
				return nil, phpobj.ThrowError(ctx, phpobj.TypeError, fmt.Sprintf("DateTime::createFromImmutable(): Argument #1 ($object) must be of type DateTimeImmutable, %s given", zo.GetClass().GetName()))
			}
			if err := checkDateTimeInitialized(ctx, zo); err != nil {
				return nil, err
			}
		}
		srcT, ok := getTimeFromObj(srcObj)
		if !ok {
			return nil, phpobj.ThrowError(ctx, phpobj.TypeError, "DateTime::createFromImmutable(): Argument #1 ($object) must be of type DateTimeImmutable")
		}
		// Use late static binding
		actualClass := getCalledClassForStatic(ctx, DateTime)
		newObj, err := phpobj.NewZObject(ctx, actualClass)
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
		H:               &phpv.ZClassHandlers{HandleCompare: dateTimeCompare},
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
			"getmicrosecond": {
				Name:      "getMicrosecond",
				Modifiers: phpv.ZAttrPublic,
				Method:    phpobj.NativeMethod(getMicrosecondMethod),
			},
			"setmicrosecond": {
				Name:      "setMicrosecond",
				Modifiers: phpv.ZAttrPublic,
				Method:    phpobj.NativeMethod(setMicrosecondImmutableMethod),
			},
			"settimestamp": {
				Name:      "setTimestamp",
				Modifiers: phpv.ZAttrPublic,
				Method:    phpobj.NativeMethod(setTimestampImmutableMethod),
				Attributes: []*phpv.ZAttribute{
					{
						ClassName: "NoDiscard",
						Args: []*phpv.ZVal{
							phpv.ZString("as DateTimeImmutable::setTimestamp() does not modify the object itself").ZVal(),
						},
					},
				},
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
			return nil, phpobj.ThrowError(ctx, phpobj.ArgumentCountError, "DateTimeImmutable::createFromMutable() expects exactly 1 argument, 0 given")
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
		H: &phpv.ZClassHandlers{
			HandlePropUnset: func(ctx phpv.Context, o phpv.ZObject, key phpv.ZString) (bool, error) {
				// DatePeriod's built-in properties cannot be unset
				builtinProps := map[phpv.ZString]bool{
					"start": true, "current": true, "end": true, "interval": true,
					"recurrences": true, "include_start_date": true, "include_end_date": true,
				}
				if builtinProps[key] {
					return true, phpobj.ThrowError(ctx, phpobj.Error,
						fmt.Sprintf("Cannot unset %s::$%s", o.GetClass().GetName(), key))
				}
				return false, nil
			},
		},
		Props: []*phpv.ZClassProp{
			{VarName: "start", Modifiers: phpv.ZAttrPublic | phpv.ZAttrReadonly, TypeHint: phpv.ParseTypeHint("?DateTimeInterface")},
			{VarName: "current", Modifiers: phpv.ZAttrPublic | phpv.ZAttrReadonly, TypeHint: phpv.ParseTypeHint("?DateTimeInterface")},
			{VarName: "end", Modifiers: phpv.ZAttrPublic | phpv.ZAttrReadonly, TypeHint: phpv.ParseTypeHint("?DateTimeInterface")},
			{VarName: "interval", Modifiers: phpv.ZAttrPublic | phpv.ZAttrReadonly, TypeHint: phpv.ParseTypeHint("?DateInterval")},
			{VarName: "recurrences", Modifiers: phpv.ZAttrPublic | phpv.ZAttrReadonly, TypeHint: phpv.ParseTypeHint("int")},
			{VarName: "include_start_date", Modifiers: phpv.ZAttrPublic | phpv.ZAttrReadonly, TypeHint: phpv.ParseTypeHint("bool")},
			{VarName: "include_end_date", Modifiers: phpv.ZAttrPublic | phpv.ZAttrReadonly, TypeHint: phpv.ParseTypeHint("bool")},
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
					if err := checkDatePeriodInitialized(ctx, this); err != nil {
						return nil, err
					}
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
					// getEndDate and getRecurrences are allowed to return NULL on uninitialized objects
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
					if err := checkDatePeriodInitialized(ctx, this); err != nil {
						return nil, err
					}
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
					opaque := this.GetOpaque(DatePeriod)
					if opaque != nil {
						if explicit, ok := opaque.(bool); ok && !explicit {
							return phpv.ZNULL.ZVal(), nil
						}
					}
					v, _ := this.ObjectGet(ctx, phpv.ZString("recurrences"))
					if v == nil || v.IsNull() {
						return phpv.ZNULL.ZVal(), nil
					}
					// Stored recurrences = original + (1 if include_start) + (1 if include_end)
					// Return the original count to the caller
					storedRec := int(v.AsInt(ctx))
					includeStartV, _ := this.ObjectGet(ctx, phpv.ZString("include_start_date"))
					if includeStartV != nil && !includeStartV.IsNull() && bool(includeStartV.AsBool(ctx)) {
						if storedRec > 0 {
							storedRec--
						}
					}
					includeEndV, _ := this.ObjectGet(ctx, phpv.ZString("include_end_date"))
					if includeEndV != nil && !includeEndV.IsNull() && bool(includeEndV.AsBool(ctx)) {
						if storedRec > 0 {
							storedRec--
						}
					}
					return phpv.ZInt(storedRec).ZVal(), nil
				}),
			},
			"getiterator": {
				Name:      "getIterator",
				Modifiers: phpv.ZAttrPublic,
				Method:    phpobj.NativeMethod(datePeriodGetIterator),
			},
			"__serialize": {
				Name:      "__serialize",
				Modifiers: phpv.ZAttrPublic,
				Method: phpobj.NativeMethod(func(ctx phpv.Context, this *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
					if err := checkDatePeriodInitialized(ctx, this); err != nil {
						return nil, err
					}
					result := phpv.NewZArray()
					builtinKeys := map[string]bool{"start": true, "current": true, "end": true, "interval": true, "recurrences": true, "include_start_date": true, "include_end_date": true}
					for _, key := range []string{"start", "current", "end", "interval", "recurrences", "include_start_date", "include_end_date"} {
						v, _ := this.ObjectGet(ctx, phpv.ZString(key))
						if v != nil {
							result.OffsetSet(ctx, phpv.ZString(key), v)
						} else {
							result.OffsetSet(ctx, phpv.ZString(key), phpv.ZNULL.ZVal())
						}
					}
					// Include subclass properties with proper PHP mangling
					appendSubclassProps(ctx, this, result, builtinKeys)
					return result.ZVal(), nil
				}),
			},
			"__unserialize": {
				Name:      "__unserialize",
				Modifiers: phpv.ZAttrPublic,
				Method: phpobj.NativeMethod(func(ctx phpv.Context, this *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
					if len(args) < 1 || args[0].GetType() != phpv.ZtArray {
						return nil, phpobj.ThrowError(ctx, phpobj.Error, "Invalid serialization data for DatePeriod object")
					}
					arr := args[0].Value().(*phpv.ZArray)
					// Must have at least a start key (even if null)
					if arr.Count(ctx) == 0 {
						return nil, phpobj.ThrowError(ctx, phpobj.Error, "Invalid serialization data for DatePeriod object")
					}
					// Validate start
					startV, _ := arr.OffsetGet(ctx, phpv.ZString("start").ZVal())
					if startV != nil && !startV.IsNull() {
						startObj, ok := startV.Value().(*phpobj.ZObject)
						if !ok || !startObj.Class.InstanceOf(DateTimeInterface) {
							return nil, phpobj.ThrowError(ctx, phpobj.Error, "Invalid serialization data for DatePeriod object")
						}
						if !isDateTimeInitialized(startObj) {
							return nil, phpobj.ThrowError(ctx, phpobj.Error, "Invalid serialization data for DatePeriod object")
						}
					}
					// Validate end
					endV, _ := arr.OffsetGet(ctx, phpv.ZString("end").ZVal())
					if endV != nil && !endV.IsNull() {
						endObj, ok := endV.Value().(*phpobj.ZObject)
						if !ok || !endObj.Class.InstanceOf(DateTimeInterface) {
							return nil, phpobj.ThrowError(ctx, phpobj.Error, "Invalid serialization data for DatePeriod object")
						}
						if !isDateTimeInitialized(endObj) {
							return nil, phpobj.ThrowError(ctx, phpobj.Error, "Invalid serialization data for DatePeriod object")
						}
					}
					// Validate current
					currentV, _ := arr.OffsetGet(ctx, phpv.ZString("current").ZVal())
					if currentV != nil && !currentV.IsNull() {
						curObj, ok := currentV.Value().(*phpobj.ZObject)
						if !ok || !curObj.Class.InstanceOf(DateTimeInterface) {
							return nil, phpobj.ThrowError(ctx, phpobj.Error, "Invalid serialization data for DatePeriod object")
						}
						if !isDateTimeInitialized(curObj) {
							return nil, phpobj.ThrowError(ctx, phpobj.Error, "Invalid serialization data for DatePeriod object")
						}
					}
					// Validate interval
					intervalV, _ := arr.OffsetGet(ctx, phpv.ZString("interval").ZVal())
					if intervalV != nil && !intervalV.IsNull() {
						intObj, ok := intervalV.Value().(*phpobj.ZObject)
						if !ok || !intObj.Class.InstanceOf(DateInterval) {
							return nil, phpobj.ThrowError(ctx, phpobj.Error, "Invalid serialization data for DatePeriod object")
						}
						if !isDateIntervalInitialized(intObj) {
							return nil, phpobj.ThrowError(ctx, phpobj.Error, "Invalid serialization data for DatePeriod object")
						}
					}
					// Set properties
					for _, key := range []string{"start", "current", "end", "interval", "recurrences", "include_start_date", "include_end_date"} {
						v, _ := arr.OffsetGet(ctx, phpv.ZString(key).ZVal())
						if v != nil {
							this.ObjectSet(ctx, phpv.ZString(key), v)
						}
					}
					// Set extra (subclass) properties with proper mangling
					restoreSubclassProps(ctx, this, arr, map[string]bool{
						"start": true, "current": true, "end": true, "interval": true,
						"recurrences": true, "include_start_date": true, "include_end_date": true,
					})
					// Ensure nullable properties are set to NULL if not provided
					if !this.HashTable().HasString("current") {
						this.HashTable().SetString("current", phpv.ZNULL.ZVal())
					}
					if !this.HashTable().HasString("end") {
						this.HashTable().SetString("end", phpv.ZNULL.ZVal())
					}
					// Mark all readonly properties as initialized
					for _, propName := range []phpv.ZString{"start", "current", "end", "interval", "recurrences", "include_start_date", "include_end_date"} {
						this.MarkReadonlyInit(propName)
					}
					// Mark as initialized
					this.SetOpaque(DatePeriod, true)
					return nil, nil
				}),
			},
		},
	}
	// Wire up methods that need DatePeriod reference
	DatePeriod.Methods["__set_state"].Method = phpobj.NativeStaticMethod(datePeriodSetState)
	DatePeriod.Methods["createfromiso8601string"].Method = phpobj.NativeStaticMethod(datePeriodCreateFromISO8601String)
}

// errISOMissingInterval is returned when an ISO 8601 period string is valid but missing the interval part
var errISOMissingInterval = fmt.Errorf("ISO interval must contain an interval")

// errISOMissingStartDate is returned when an ISO 8601 period starts with R but has no start date
var errISOMissingStartDate = fmt.Errorf("ISO interval must contain a start date")

// errISOZeroRecurrences is returned when an ISO 8601 period has start/interval but no recurrence count
var errISOZeroRecurrences = fmt.Errorf("ISO interval has zero recurrences")

// parseISO8601Period parses ISO 8601 repeating interval strings like "R2/2012-07-01T00:00:00Z/P7D"
// Returns recurrences, start time, interval duration spec, and end time (if any)
// Returns a special errISOMissingInterval if the string is missing the interval part.
func parseISO8601Period(ctx phpv.Context, isoStr string) (recurrences int, start time.Time, intervalStr string, hasEnd bool, end time.Time, err error) {
	recurrences = -1 // -1 means not specified

	// Format: R[n]/start/interval or R[n]/start/end or R[n]/interval/end
	parts := strings.SplitN(isoStr, "/", 3)
	if len(parts) < 3 {
		if len(parts) == 1 {
			// Single part: if starts with R, it's missing a start date
			rPart1 := parts[0]
			if len(rPart1) > 0 && (rPart1[0] == 'R' || rPart1[0] == 'r') {
				err = errISOMissingStartDate
				return
			}
		} else if len(parts) == 2 {
			rPart2 := parts[0]
			if len(rPart2) > 0 && (rPart2[0] == 'R' || rPart2[0] == 'r') {
				// Two parts starting with R: R/start format is missing interval
				_, parseErr := parseISO8601DateTime(parts[1])
				if parseErr == nil {
					err = errISOMissingInterval
					return
				}
			} else {
				// Two parts not starting with R: could be start/interval (no recurrence)
				_, dateParseErr := parseISO8601DateTime(parts[0])
				if dateParseErr == nil {
					p1 := parts[1]
					if len(p1) > 0 && (p1[0] == 'P' || p1[0] == 'p') {
						err = errISOZeroRecurrences
						return
					}
				}
			}
		}
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
		// It's a duration - return as string, caller will create the object
		intervalStr = thirdPart
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
	// hasTimezone checks if the string contains a timezone indicator (Z or +/-HH:MM)
	hasTimezone := len(s) > 0 && (s[len(s)-1] == 'Z' ||
		(len(s) > 6 && (s[len(s)-6] == '+' || s[len(s)-6] == '-')))

	// Use RFC3339 first: "2006-01-02T15:04:05Z07:00" handles both Z and +HH:MM offsets
	// RFC3339 parses Z as a zero offset (+00:00), giving timezone_type 1 (offset) in PHP
	for _, layout := range []string{
		time.RFC3339,
		"2006-01-02T15:04:05-07:00",
		"2006-01-02T15:04:05",
		"2006-01-02",
	} {
		if t, err := time.Parse(layout, s); err == nil {
			// If string has a timezone indicator, normalize to a fixed offset zone
			// This ensures timezone_type 1 (+00:00) instead of type 3 (UTC name)
			if hasTimezone && (t.Location().String() == "" || t.Location() == time.UTC) {
				_, offset := t.Zone()
				t = t.In(makeFixedZone(offset))
			} else if t.Location().String() == "" {
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

	// Check if first arg is a string or null (ISO 8601 period string - deprecated)
	if args[0].GetType() == phpv.ZtString || args[0].IsNull() {
		// String form only accepts 1 or 2 arguments (string [, int])
		if len(args) > 2 {
			return nil, phpobj.ThrowError(ctx, phpobj.TypeError, "DatePeriod::__construct() accepts (DateTimeInterface, DateInterval, int [, int]), or (DateTimeInterface, DateInterval, DateTime [, int]), or (string [, int]) as arguments")
		}

		if args[0].IsNull() {
			// Deprecation for passing null
			if err := ctx.Deprecated("DatePeriod::__construct(): Passing null to parameter #1 ($start) of type string is deprecated", logopt.NoFuncName(true)); err != nil {
				return nil, err
			}
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

	// Validate start is a DateTime object
	if startObj, ok := args[0].Value().(*phpobj.ZObject); ok {
		if err := checkDateTimeInterfaceInitialized(ctx, startObj); err != nil {
			return nil, err
		}
	}

	this.ObjectSet(ctx, phpv.ZString("start"), args[0])
	this.ObjectSet(ctx, phpv.ZString("interval"), args[1])

	options := 0

	if len(args) >= 4 {
		options = int(args[3].AsInt(ctx))
	}

	includeStart := options&1 == 0 // EXCLUDE_START_DATE = 1
	includeEnd := options&2 != 0   // INCLUDE_END_DATE = 2

	// Third arg could be end DateTime or recurrence count
	if args[2].GetType() == phpv.ZtInt {
		recCount := int(args[2].AsInt(ctx))
		if recCount < 1 {
			return nil, phpobj.ThrowError(ctx, DateException,
				fmt.Sprintf("DatePeriod::__construct(): Recurrence count must be greater or equal to 1 and lower than %d", int(^uint(0)>>1)))
		}
		// Store recurrences: PHP stores recurrences + (1 if includeStart) + (1 if includeEnd)
		storedRec := recCount
		if includeStart {
			storedRec++
		}
		if includeEnd {
			storedRec++
		}
		this.ObjectSet(ctx, phpv.ZString("recurrences"), phpv.ZInt(storedRec).ZVal())
		// Opaque true = recurrences were explicitly given
		this.SetOpaque(DatePeriod, true)
	} else {
		// Validate end is a DateTime object if provided
		if endObj, ok := args[2].Value().(*phpobj.ZObject); ok {
			if err := checkDateTimeInterfaceInitialized(ctx, endObj); err != nil {
				return nil, err
			}
		}
		this.ObjectSet(ctx, phpv.ZString("end"), args[2])
		// When end date is given, recurrences = 1 if including start, 0 if excluding
		endRecurrences := 1
		if !includeStart {
			endRecurrences = 0
		}
		this.ObjectSet(ctx, phpv.ZString("recurrences"), phpv.ZInt(endRecurrences).ZVal())
		this.SetOpaque(DatePeriod, false)
	}

	this.ObjectSet(ctx, phpv.ZString("include_start_date"), phpv.ZBool(includeStart).ZVal())
	this.ObjectSet(ctx, phpv.ZString("include_end_date"), phpv.ZBool(includeEnd).ZVal())

	// Explicitly set uninitialized nullable properties to NULL so they show as
	// NULL in var_dump rather than uninitialized.
	if !this.HashTable().HasString("current") {
		this.HashTable().SetString("current", phpv.ZNULL.ZVal())
	}
	if !this.HashTable().HasString("end") {
		this.HashTable().SetString("end", phpv.ZNULL.ZVal())
	}

	// Mark all readonly properties as initialized so that external code gets
	// "Cannot modify readonly property" instead of "protected(set) readonly" errors.
	for _, propName := range []phpv.ZString{"start", "current", "end", "interval", "recurrences", "include_start_date", "include_end_date"} {
		this.MarkReadonlyInit(propName)
	}

	return nil, nil
}

// datePeriodInitFromISO initializes a DatePeriod from an ISO 8601 string
// datePeriodInitFromISO initializes a DatePeriod from an ISO 8601 string
// callerName is used in error messages (e.g., "DatePeriod::__construct()" or "DatePeriod::createFromISO8601String()")
func datePeriodInitFromISO(ctx phpv.Context, this *phpobj.ZObject, isoStr string, options int) (*phpv.ZVal, error) {
	return datePeriodInitFromISOCaller(ctx, this, isoStr, options, "DatePeriod::__construct()")
}

func datePeriodInitFromISOCaller(ctx phpv.Context, this *phpobj.ZObject, isoStr string, options int, callerName string) (*phpv.ZVal, error) {
	recurrences, start, intervalStr, hasEnd, end, err := parseISO8601Period(ctx, isoStr)
	if err != nil {
		if err == errISOMissingInterval {
			return nil, phpobj.ThrowError(ctx, DateMalformedPeriodStringException,
				fmt.Sprintf("%s: ISO interval must contain an interval, %q given", callerName, isoStr))
		}
		if err == errISOMissingStartDate {
			return nil, phpobj.ThrowError(ctx, DateMalformedPeriodStringException,
				fmt.Sprintf("%s: ISO interval must contain a start date, %q given", callerName, isoStr))
		}
		if err == errISOZeroRecurrences {
			return nil, phpobj.ThrowError(ctx, DateMalformedPeriodStringException,
				fmt.Sprintf("%s: Recurrence count must be greater or equal to 1 and lower than %d", callerName, int(^uint(0)>>1)))
		}
		return nil, phpobj.ThrowError(ctx, DateMalformedPeriodStringException, fmt.Sprintf("Unknown or bad format (%s)", isoStr))
	}

	// Create start DateTimeImmutable first (to get correct object ID ordering)
	startObj, err := phpobj.NewZObject(ctx, DateTimeImmutable)
	if err != nil {
		return nil, err
	}
	setTimeVal(startObj, start)
	this.ObjectSet(ctx, phpv.ZString("start"), startObj.ZVal())

	// Create interval object after start (to maintain correct object ID ordering)
	if intervalStr != "" {
		intervalObj, err := phpobj.NewZObject(ctx, DateInterval, phpv.ZString(intervalStr).ZVal())
		if err != nil {
			return nil, err
		}
		this.ObjectSet(ctx, phpv.ZString("interval"), intervalObj.ZVal())
	}

	includeStart := options&1 == 0
	includeEnd := options&2 != 0

	if hasEnd {
		endObj, err := phpobj.NewZObject(ctx, DateTimeImmutable)
		if err != nil {
			return nil, err
		}
		setTimeVal(endObj, end)
		this.ObjectSet(ctx, phpv.ZString("end"), endObj.ZVal())
		// With end date: recurrences = 1 if including start, 0 if excluding
		endRecurrences := 1
		if !includeStart {
			endRecurrences = 0
		}
		this.ObjectSet(ctx, phpv.ZString("recurrences"), phpv.ZInt(endRecurrences).ZVal())
		this.SetOpaque(DatePeriod, false)
	} else if recurrences >= 0 {
		// R4 means 4 repetitions; store as recurrences+1 when including start, +1 more if includeEnd
		storedRec := recurrences
		if includeStart {
			storedRec++
		}
		if includeEnd {
			storedRec++
		}
		this.ObjectSet(ctx, phpv.ZString("recurrences"), phpv.ZInt(storedRec).ZVal())
		this.SetOpaque(DatePeriod, true)
	}

	this.ObjectSet(ctx, phpv.ZString("include_start_date"), phpv.ZBool(includeStart).ZVal())
	this.ObjectSet(ctx, phpv.ZString("include_end_date"), phpv.ZBool(includeEnd).ZVal())

	// Explicitly set uninitialized nullable properties to NULL so they show as
	// NULL in var_dump rather than uninitialized.
	if !this.HashTable().HasString("current") {
		this.HashTable().SetString("current", phpv.ZNULL.ZVal())
	}
	if !this.HashTable().HasString("end") {
		this.HashTable().SetString("end", phpv.ZNULL.ZVal())
	}

	// Mark all readonly properties as initialized so external writes give the
	// correct "Cannot modify readonly property" error.
	for _, propName := range []phpv.ZString{"start", "current", "end", "interval", "recurrences", "include_start_date", "include_end_date"} {
		this.MarkReadonlyInit(propName)
	}

	return nil, nil
}

// datePeriodIteratorData holds the state of a DatePeriod iterator
type datePeriodIteratorData struct {
	dates      []*phpv.ZVal
	pos        int
	nextCurrent *phpv.ZVal // the "current" value (one step past last date) to set on the parent
	parent     *phpobj.ZObject
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
					// Return a fresh clone so mutating the returned object doesn't affect iteration
					origVal := d.dates[d.pos]
					if origVal == nil || origVal.GetType() != phpv.ZtObject {
						return origVal, nil
					}
					origObj, ok := origVal.Value().(*phpobj.ZObject)
					if !ok {
						return origVal, nil
					}
					origTime, ok2 := getTime(origObj)
					if !ok2 {
						return origVal, nil
					}
					cloned, clErr := phpobj.NewZObject(ctx, origObj.Class)
					if clErr != nil {
						return origVal, nil
					}
					setTimeVal(cloned, origTime)
					return cloned.ZVal(), nil
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
						// When advancing past the last date, update parent's current property
						if d.pos >= len(d.dates) && d.nextCurrent != nil && d.parent != nil {
							d.parent.ObjectSet(ctx, phpv.ZString("current"), d.nextCurrent)
						}
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
						// When iteration ends, update parent's current property
						if d != nil && d.nextCurrent != nil && d.parent != nil {
							d.parent.ObjectSet(ctx, phpv.ZString("current"), d.nextCurrent)
						}
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
	// Use DatePeriod class name for this error specifically (matching PHP behavior)
	if _, ok := this.Opaque[DatePeriod]; !ok {
		startVal := this.HashTable().GetString("start")
		if startVal == nil || startVal.IsNull() {
			return nil, phpobj.ThrowError(ctx, DateObjectError,
				"Object of type DatePeriod has not been correctly initialized by calling parent::__construct() in its constructor")
		}
	}
	dates, nextCurrent, err := datePeriodGenerateDates(ctx, this)
	if err != nil {
		return nil, err
	}

	iterObj, err := phpobj.NewZObject(ctx, DatePeriodIterator)
	if err != nil {
		return nil, err
	}
	iterObj.SetOpaque(DatePeriodIterator, &datePeriodIteratorData{dates: dates, pos: 0, nextCurrent: nextCurrent, parent: this})
	return iterObj.ZVal(), nil
}

// datePeriodGenerateDates generates all dates in a DatePeriod
// Returns the dates slice and the "nextCurrent" (one step past the last date)
func datePeriodGenerateDates(ctx phpv.Context, this *phpobj.ZObject) ([]*phpv.ZVal, *phpv.ZVal, error) {
	startVal, _ := this.ObjectGet(ctx, phpv.ZString("start"))
	intervalVal, _ := this.ObjectGet(ctx, phpv.ZString("interval"))
	endVal, _ := this.ObjectGet(ctx, phpv.ZString("end"))
	recurrencesVal, _ := this.ObjectGet(ctx, phpv.ZString("recurrences"))
	includeStartVal, _ := this.ObjectGet(ctx, phpv.ZString("include_start_date"))
	includeEndVal, _ := this.ObjectGet(ctx, phpv.ZString("include_end_date"))
	currentVal, _ := this.ObjectGet(ctx, phpv.ZString("current"))

	if startVal == nil || startVal.IsNull() || intervalVal == nil || intervalVal.IsNull() {
		return nil, nil, nil
	}

	startObj, ok := startVal.Value().(*phpobj.ZObject)
	if !ok {
		return nil, nil, nil
	}
	startTime, ok := getTime(startObj)
	if !ok {
		return nil, nil, nil
	}

	// Determine the class to use for yielded dates (DateTime or DateTimeImmutable)
	// PHP preserves the type of the start date object
	yieldClass := DateTimeImmutable
	if startObj.Class.InstanceOf(DateTime) && !startObj.Class.InstanceOf(DateTimeImmutable) {
		yieldClass = DateTime
	}

	intervalObj, ok := intervalVal.Value().(*phpobj.ZObject)
	if !ok {
		return nil, nil, nil
	}

	includeStart := true
	if includeStartVal != nil && !includeStartVal.IsNull() {
		includeStart = bool(includeStartVal.AsBool(ctx))
	}

	includeEnd := false
	if includeEndVal != nil && !includeEndVal.IsNull() {
		includeEnd = bool(includeEndVal.AsBool(ctx))
	}

	// Determine the starting point for iteration.
	// If the DatePeriod has a "current" value set (from prior iteration or unserialization),
	// use that as the starting point instead of start.
	// The "current" property stores the next-to-yield value.
	var iterStart time.Time
	hasCurrentVal := currentVal != nil && !currentVal.IsNull()
	if hasCurrentVal {
		if curObj, ok2 := currentVal.Value().(*phpobj.ZObject); ok2 {
			if curTime, ok3 := getTime(curObj); ok3 {
				iterStart = curTime
				// When resuming from current, we don't include the start date again
				hasCurrentVal = !curTime.IsZero()
			}
		}
	}
	_ = hasCurrentVal // suppress unused warning

	var results []*phpv.ZVal
	current := startTime

	// Determine iteration limit
	hasEnd := endVal != nil && !endVal.IsNull()
	var endTime time.Time
	maxCount := -1 // -1 = unlimited (use end date)

	if hasEnd {
		if endObj, ok2 := endVal.Value().(*phpobj.ZObject); ok2 {
			endTime, _ = getTime(endObj)
		}
	}

	if recurrencesVal != nil && !recurrencesVal.IsNull() && recurrencesVal.GetType() == phpv.ZtInt {
		maxCount = int(recurrencesVal.AsInt(ctx))
	}

	// Safety limit for end-date based iteration
	maxIter := 10000
	if !hasEnd && maxCount >= 0 {
		// For recurrence-based: maxCount is the total number of dates to yield
		maxIter = maxCount
	}

	// If resuming from a serialized "current" position, start from there
	// Otherwise start from the start date
	if !iterStart.IsZero() {
		current = iterStart
		// Don't include start date since we're resuming
		// Generate up to remaining recurrences
		// (For now, just start from current and apply same logic)
	}

	count := 0
	for i := 0; i < maxIter+1; i++ {
		// Check if we've yielded enough
		if !hasEnd && maxCount >= 0 && count >= maxCount {
			break
		}

		// Check end condition for end-date based iteration
		if hasEnd {
			if current.After(endTime) {
				break
			}
			if current.Equal(endTime) {
				if !includeEnd {
					break
				}
				// includeEnd: yield this date and stop
				dateObj, err := phpobj.NewZObject(ctx, yieldClass)
				if err != nil {
					return nil, nil, err
				}
				setTimeVal(dateObj, current)
				results = append(results, dateObj.ZVal())
				// Advance current one more step for "nextCurrent"
				current = addIntervalToTime(ctx, current, intervalObj, false)
				break
			}
		}

		// For the first position: handle include_start_date
		if i == 0 && !includeStart {
			// Skip start date - advance and continue
			current = addIntervalToTime(ctx, current, intervalObj, false)
			continue
		}

		dateObj, err := phpobj.NewZObject(ctx, yieldClass)
		if err != nil {
			return nil, nil, err
		}
		setTimeVal(dateObj, current)
		results = append(results, dateObj.ZVal())
		count++

		// Advance to next position
		current = addIntervalToTime(ctx, current, intervalObj, false)
	}

	// Build the nextCurrent value (current position after iteration)
	var nextCurrent *phpv.ZVal
	if len(results) > 0 {
		// current is already one step past the last yielded date
		nextObj, err := phpobj.NewZObject(ctx, yieldClass)
		if err == nil {
			setTimeVal(nextObj, current)
			nextCurrent = nextObj.ZVal()
		}
	}

	return results, nextCurrent, nil
}

// datePeriodSetState implements DatePeriod::__set_state
func datePeriodSetState(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	if len(args) < 1 || args[0].GetType() != phpv.ZtArray {
		return nil, phpobj.ThrowError(ctx, phpobj.Error, "Invalid serialization data for DatePeriod object")
	}
	arr := args[0].Value().(*phpv.ZArray)

	// Validate and extract fields
	startVal, _ := arr.OffsetGet(ctx, phpv.ZString("start").ZVal())
	endVal, _ := arr.OffsetGet(ctx, phpv.ZString("end").ZVal())
	currentVal, _ := arr.OffsetGet(ctx, phpv.ZString("current").ZVal())
	intervalVal, _ := arr.OffsetGet(ctx, phpv.ZString("interval").ZVal())
	recurrencesVal, _ := arr.OffsetGet(ctx, phpv.ZString("recurrences").ZVal())
	includeStartVal, _ := arr.OffsetGet(ctx, phpv.ZString("include_start_date").ZVal())
	includeEndVal, _ := arr.OffsetGet(ctx, phpv.ZString("include_end_date").ZVal())

	// Validate start: must be null or DateTimeInterface
	if startVal != nil && !startVal.IsNull() {
		if _, ok := startVal.Value().(*phpobj.ZObject); !ok {
			return nil, phpobj.ThrowError(ctx, phpobj.Error, "Invalid serialization data for DatePeriod object")
		}
	}
	// Validate end: must be null or DateTimeInterface
	if endVal != nil && !endVal.IsNull() {
		if _, ok := endVal.Value().(*phpobj.ZObject); !ok {
			return nil, phpobj.ThrowError(ctx, phpobj.Error, "Invalid serialization data for DatePeriod object")
		}
	}
	// Validate current: must be null or DateTimeInterface
	if currentVal != nil && !currentVal.IsNull() {
		if _, ok := currentVal.Value().(*phpobj.ZObject); !ok {
			return nil, phpobj.ThrowError(ctx, phpobj.Error, "Invalid serialization data for DatePeriod object")
		}
	}
	// Validate interval: must be a non-null DateInterval (required)
	if intervalVal == nil || intervalVal.IsNull() {
		return nil, phpobj.ThrowError(ctx, phpobj.Error, "Invalid serialization data for DatePeriod object")
	}
	if intervalVal != nil && !intervalVal.IsNull() {
		obj, ok := intervalVal.Value().(*phpobj.ZObject)
		if !ok || !obj.Class.InstanceOf(DateInterval) {
			return nil, phpobj.ThrowError(ctx, phpobj.Error, "Invalid serialization data for DatePeriod object")
		}
	}
	// Validate recurrences: must be int >= 0
	if recurrencesVal != nil && !recurrencesVal.IsNull() {
		if recurrencesVal.GetType() != phpv.ZtInt {
			return nil, phpobj.ThrowError(ctx, phpobj.Error, "Invalid serialization data for DatePeriod object")
		}
		if recurrencesVal.AsInt(ctx) < 0 {
			return nil, phpobj.ThrowError(ctx, phpobj.Error, "Invalid serialization data for DatePeriod object")
		}
	}
	// Validate include_start_date: must be bool
	if includeStartVal != nil && !includeStartVal.IsNull() {
		if includeStartVal.GetType() != phpv.ZtBool {
			return nil, phpobj.ThrowError(ctx, phpobj.Error, "Invalid serialization data for DatePeriod object")
		}
	}
	// Validate include_end_date: must be bool
	if includeEndVal != nil && !includeEndVal.IsNull() {
		if includeEndVal.GetType() != phpv.ZtBool {
			return nil, phpobj.ThrowError(ctx, phpobj.Error, "Invalid serialization data for DatePeriod object")
		}
	}

	// Create a DatePeriod object without calling constructor
	actualClass := getCalledClassForStatic(ctx, DatePeriod)
	obj, err := phpobj.NewZObjectOpaque(ctx, actualClass, true)
	if err != nil {
		return nil, err
	}
	obj.SetOpaque(DatePeriod, true)

	// Set properties directly
	if startVal != nil {
		obj.ObjectSet(ctx, phpv.ZString("start"), startVal)
	}
	if endVal != nil {
		obj.ObjectSet(ctx, phpv.ZString("end"), endVal)
	}
	if currentVal != nil {
		obj.ObjectSet(ctx, phpv.ZString("current"), currentVal)
	}
	if intervalVal != nil {
		obj.ObjectSet(ctx, phpv.ZString("interval"), intervalVal)
	}
	if recurrencesVal != nil {
		obj.ObjectSet(ctx, phpv.ZString("recurrences"), recurrencesVal)
	}
	if includeStartVal != nil {
		obj.ObjectSet(ctx, phpv.ZString("include_start_date"), includeStartVal)
	} else {
		obj.ObjectSet(ctx, phpv.ZString("include_start_date"), phpv.ZBool(true).ZVal())
	}
	if includeEndVal != nil {
		obj.ObjectSet(ctx, phpv.ZString("include_end_date"), includeEndVal)
	} else {
		obj.ObjectSet(ctx, phpv.ZString("include_end_date"), phpv.ZBool(false).ZVal())
	}

	// Set any extra properties from subclasses
	builtinKeys := map[string]bool{"start": true, "end": true, "current": true, "interval": true, "recurrences": true, "include_start_date": true, "include_end_date": true}
	it := arr.NewIterator()
	for it.Valid(ctx) {
		k, _ := it.Key(ctx)
		v, _ := it.Current(ctx)
		ks := k.String()
		if !builtinKeys[ks] {
			obj.ObjectSet(ctx, phpv.ZString(ks), v)
		}
		it.Next(ctx)
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

	// Use late static binding for subclass support
	actualClass := getCalledClassForStatic(ctx, DatePeriod)

	// Check if the class is abstract (cannot instantiate abstract class)
	if actualClass.Attr&phpv.ZClassAttr(phpv.ZClassExplicitAbstract) != 0 {
		return nil, phpobj.ThrowError(ctx, phpobj.Error, fmt.Sprintf("Cannot instantiate abstract class %s", actualClass.GetName()))
	}

	// Create the DatePeriod without calling the constructor.
	obj, err := phpobj.NewZObjectOpaque(ctx, actualClass, true)
	if err != nil {
		return nil, err
	}
	// Ensure the DatePeriod opaque key is set for initialization checks
	obj.SetOpaque(DatePeriod, true)
	_, err = datePeriodInitFromISOCaller(ctx, obj, isoStr, options, "DatePeriod::createFromISO8601String()")
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
		// Not a proper ISO 8601 duration
		return nil, phpobj.ThrowError(ctx, DateMalformedIntervalStringException, "Failed to parse interval ("+spec+")")
	}
	if len(spec) < 2 {
		return nil, phpobj.ThrowError(ctx, DateMalformedIntervalStringException, "Unknown or bad format ("+spec+")")
	}
	// Simple parser for PnYnMnDTnHnMnS (supports negative values like P0Y3M-1D)
	inTime := false
	num := ""
	for i := 1; i < len(spec); i++ {
		c := spec[i]
		if c >= '0' && c <= '9' || (c == '-' && num == "") {
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
		case 'W':
			// P1W = 1 week = 7 days
			this.HashTable().SetString("d", phpv.ZInt(n*7).ZVal())
		default:
			// Unknown character in duration string - reject it
			return nil, phpobj.ThrowError(ctx, DateMalformedIntervalStringException, "Unknown or bad format ("+spec+")")
		}
	}
	return nil, nil
}

func dateIntervalFormat(ctx phpv.Context, this *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	if len(args) < 1 {
		return nil, phpobj.ThrowError(ctx, phpobj.ArgumentCountError, "DateInterval::format() expects exactly 1 argument")
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

// nonRelativePattern detects non-relative elements in a date string.
var nonRelativePattern = regexp.MustCompile(`(?i)(^\d{4}-\d{2}-\d{2}|\b\d{1,2}:\d{2}(:\d{2})?\b|\bnoon\b|\bmidnight\b|\bam\b|\bpm\b|` +
	`\b(january|february|march|april|may|june|july|august|september|october|november|december)\b.*\b\d{1,2}\b|` +
	`\b\d{1,2}\s+(january|february|march|april|may|june|july|august|september|october|november|december)\b|` +
	`\b(UTC|GMT|EST|CST|MST|PST|EDT|CDT|MDT|PDT|BST|CET|EET|IST|JST|AEST|NZST)\b)`)

// hasNonRelativeElements checks if a relative date string contains non-relative elements.
func hasNonRelativeElements(dateStr string) bool {
	return nonRelativePattern.MatchString(dateStr)
}

// createDateIntervalFromString creates a DateInterval from a relative date string.
// Throws DateMalformedIntervalStringException on unknown formats or non-relative elements.
func createDateIntervalFromString(ctx phpv.Context, dateStr string) (*phpv.ZVal, error) {
	result, warnMsg, err := createDateIntervalFromStringMsg(ctx, dateStr, true)
	if err != nil {
		return nil, err
	}
	if warnMsg != "" {
		return nil, phpobj.ThrowError(ctx, DateMalformedIntervalStringException, warnMsg)
	}
	return result, nil
}

// createDateIntervalFromStringMsg creates a DateInterval from a relative date string.
// Returns (result, warnMsg, err).
func createDateIntervalFromStringMsg(ctx phpv.Context, dateStr string, _ bool) (*phpv.ZVal, string, error) {
	obj, err := phpobj.NewZObject(ctx, DateInterval)
	if err != nil {
		return nil, "", err
	}

	// Store the from_string flag and original string
	obj.HashTable().SetString("from_string", phpv.ZBool(true).ZVal())
	obj.HashTable().SetString("date_string", phpv.ZString(dateStr).ZVal())

	trimmed := strings.TrimSpace(strings.ToLower(dateStr))
	parts := strings.Fields(trimmed)

	// Parse simple formats like "N unit" (e.g., "2 days", "1 month")
	parsed := false
	for i := 0; i < len(parts); i++ {
		num := 0
		if n, err2 := fmt.Sscanf(parts[i], "%d", &num); err2 == nil && n == 1 {
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

	if len(trimmed) == 0 {
		return nil, "Unknown or bad format () at position 0 ( ): Empty string", nil
	}

	_, stErr := strtotime.StrToTime(dateStr, strtotime.InTZ(getTimezone(ctx)), strtotime.Rel(time.Now().In(getTimezone(ctx))))
	if stErr != nil {
		if !parsed {
			firstChar := string([]rune(dateStr)[0:1])
			return nil, fmt.Sprintf("Unknown or bad format (%s) at position 0 (%s): The timezone could not be found in the database",
				dateStr, firstChar), nil
		}
	} else {
		// Check for non-relative elements
		if hasNonRelativeElements(dateStr) {
			return nil, fmt.Sprintf("String '%s' contains non-relative elements", dateStr), nil
		}
	}

	return obj.ZVal(), "", nil
}
