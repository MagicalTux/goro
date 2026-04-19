package date

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/KarpelesLab/gotz"
	"github.com/KarpelesLab/strtotime"
	"github.com/KarpelesLab/goro/core"
	"github.com/KarpelesLab/goro/core/phpobj"
	"github.com/KarpelesLab/goro/core/phpv"
)

// isDST returns true if the given time is in DST for its timezone.
// It uses gotz to get the authoritative IsDST flag from IANA data,
// which correctly handles historical DST rules and Southern Hemisphere timezones.
func isDST(t time.Time) bool {
	locName := t.Location().String()
	if locName == "UTC" || locName == "Local" {
		return false
	}
	// For fixed-offset timezones (e.g. "+05:00"), there is no DST
	if len(locName) > 0 && (locName[0] == '+' || locName[0] == '-') {
		return false
	}
	zone, err := gotz.Load(locName)
	if err != nil {
		// Fallback: compare with January offset
		_, offset := t.Zone()
		_, stdOffset := time.Date(t.Year(), time.January, 1, 0, 0, 0, 0, t.Location()).Zone()
		return offset != stdOffset
	}
	return zone.Lookup(t).IsDST
}

// getTimezone returns the timezone configured via date.timezone, falling back to UTC.
func getTimezone(ctx phpv.Context) *time.Location {
	tzName := ctx.GetConfig("date.timezone", phpv.ZString("").ZVal()).String()
	if tzName == "" {
		return time.UTC
	}
	loc, err := time.LoadLocation(tzName)
	if err != nil {
		return time.UTC
	}
	return loc
}

// phpDateFormat converts a PHP date format string to a Go time layout and formats the given time.
func phpDateFormat(format string, t time.Time) string {
	var buf strings.Builder
	runes := []rune(format)
	for i := 0; i < len(runes); i++ {
		c := runes[i]
		switch c {
		case '\\':
			// next character is literal
			if i+1 < len(runes) {
				i++
				buf.WriteRune(runes[i])
			}
		// Day
		case 'd': // Day of the month, 2 digits with leading zeros
			buf.WriteString(fmt.Sprintf("%02d", t.Day()))
		case 'D': // A textual representation of a day, three letters
			buf.WriteString(t.Weekday().String()[:3])
		case 'j': // Day of the month without leading zeros
			buf.WriteString(strconv.Itoa(t.Day()))
		case 'l': // A full textual representation of the day of the week
			buf.WriteString(t.Weekday().String())
		case 'N': // ISO 8601 numeric representation of the day of the week (1=Monday, 7=Sunday)
			wd := int(t.Weekday())
			if wd == 0 {
				wd = 7
			}
			buf.WriteString(strconv.Itoa(wd))
		case 'S': // English ordinal suffix for the day of the month
			day := t.Day()
			switch {
			case day == 11 || day == 12 || day == 13:
				buf.WriteString("th")
			case day%10 == 1:
				buf.WriteString("st")
			case day%10 == 2:
				buf.WriteString("nd")
			case day%10 == 3:
				buf.WriteString("rd")
			default:
				buf.WriteString("th")
			}
		case 'w': // Numeric representation of the day of the week (0=Sunday, 6=Saturday)
			buf.WriteString(strconv.Itoa(int(t.Weekday())))
		case 'z': // The day of the year (starting from 0)
			buf.WriteString(strconv.Itoa(t.YearDay() - 1))

		// Week
		case 'W': // ISO 8601 week number of year
			_, week := t.ISOWeek()
			buf.WriteString(fmt.Sprintf("%02d", week))

		// Month
		case 'F': // A full textual representation of a month
			buf.WriteString(t.Month().String())
		case 'm': // Numeric representation of a month, with leading zeros
			buf.WriteString(fmt.Sprintf("%02d", int(t.Month())))
		case 'M': // A short textual representation of a month, three letters
			buf.WriteString(t.Month().String()[:3])
		case 'n': // Numeric representation of a month, without leading zeros
			buf.WriteString(strconv.Itoa(int(t.Month())))
		case 't': // Number of days in the given month
			// Get the last day of the month
			lastDay := time.Date(t.Year(), t.Month()+1, 0, 0, 0, 0, 0, t.Location())
			buf.WriteString(strconv.Itoa(lastDay.Day()))

		// Year
		case 'L': // Whether it's a leap year (1 if leap, 0 otherwise)
			y := t.Year()
			if (y%4 == 0 && y%100 != 0) || y%400 == 0 {
				buf.WriteString("1")
			} else {
				buf.WriteString("0")
			}
		case 'o': // ISO 8601 week-numbering year
			year, _ := t.ISOWeek()
			buf.WriteString(strconv.Itoa(year))
		case 'Y': // A full numeric representation of a year, 4 digits
			y := t.Year()
			if y < 0 {
				buf.WriteString(fmt.Sprintf("-%04d", -y))
			} else {
				buf.WriteString(fmt.Sprintf("%04d", y))
			}
		case 'y': // A two digit representation of a year
			buf.WriteString(fmt.Sprintf("%02d", t.Year()%100))

		// Time
		case 'a': // Lowercase am/pm
			if t.Hour() < 12 {
				buf.WriteString("am")
			} else {
				buf.WriteString("pm")
			}
		case 'A': // Uppercase AM/PM
			if t.Hour() < 12 {
				buf.WriteString("AM")
			} else {
				buf.WriteString("PM")
			}
		case 'g': // 12-hour format without leading zeros
			h := t.Hour() % 12
			if h == 0 {
				h = 12
			}
			buf.WriteString(strconv.Itoa(h))
		case 'G': // 24-hour format without leading zeros
			buf.WriteString(strconv.Itoa(t.Hour()))
		case 'h': // 12-hour format with leading zeros
			h := t.Hour() % 12
			if h == 0 {
				h = 12
			}
			buf.WriteString(fmt.Sprintf("%02d", h))
		case 'H': // 24-hour format with leading zeros
			buf.WriteString(fmt.Sprintf("%02d", t.Hour()))
		case 'i': // Minutes with leading zeros
			buf.WriteString(fmt.Sprintf("%02d", t.Minute()))
		case 's': // Seconds with leading zeros
			buf.WriteString(fmt.Sprintf("%02d", t.Second()))
		case 'u': // Microseconds
			buf.WriteString(fmt.Sprintf("%06d", t.Nanosecond()/1000))
		case 'v': // Milliseconds
			buf.WriteString(fmt.Sprintf("%03d", t.Nanosecond()/1000000))

		// Timezone
		case 'e': // Timezone identifier
			locName := t.Location().String()
			if locName == "" {
				// Fixed-offset zone with no name - format as +HH:MM
				_, offset := t.Zone()
				sign := "+"
				if offset < 0 {
					sign = "-"
					offset = -offset
				}
				hours := offset / 3600
				mins := (offset % 3600) / 60
				locName = fmt.Sprintf("%s%02d:%02d", sign, hours, mins)
			} else if locName == "GMT" {
				// GMT is not a valid IANA identifier; the canonical name is UTC
				locName = "UTC"
			}
			buf.WriteString(locName)
		case 'I': // Whether daylight saving time (1 if DST, 0 otherwise)
			if isDST(t) {
				buf.WriteString("1")
			} else {
				buf.WriteString("0")
			}
		case 'O': // Difference to UTC in hours, e.g. +0200
			_, offset := t.Zone()
			sign := "+"
			if offset < 0 {
				sign = "-"
				offset = -offset
			}
			hours := offset / 3600
			mins := (offset % 3600) / 60
			buf.WriteString(fmt.Sprintf("%s%02d%02d", sign, hours, mins))
		case 'P': // Difference to UTC with colon, e.g. +02:00
			_, offset := t.Zone()
			sign := "+"
			if offset < 0 {
				sign = "-"
				offset = -offset
			}
			hours := offset / 3600
			mins := (offset % 3600) / 60
			buf.WriteString(fmt.Sprintf("%s%02d:%02d", sign, hours, mins))
		case 'T': // Timezone abbreviation
			name, offset := t.Zone()
			locName := t.Location().String()
			// Check if this is a fixed-offset timezone (location name starts with +/-).
			// Even for +00:00 (where Go might report name="UTC"), PHP uses GMT+0000.
			if len(locName) > 0 && (locName[0] == '+' || locName[0] == '-') {
				sign := "+"
				absOffset := offset
				if offset < 0 {
					sign = "-"
					absOffset = -offset
				}
				hours := absOffset / 3600
				mins := (absOffset % 3600) / 60
				buf.WriteString(fmt.Sprintf("GMT%s%02d%02d", sign, hours, mins))
			} else if locName == "UTC" || name == "UTC" {
				buf.WriteString("UTC")
			} else if strings.Contains(locName, "/") {
				// Named IANA timezone (type 3): output the abbreviation as-is.
				// The abbreviation may be alphabetic (e.g. "BST", "EST") or a
				// numeric offset (e.g. "-02", "+0530") -- PHP outputs it directly.
				buf.WriteString(name)
			} else if name != "" && name[0] != '+' && name[0] != '-' {
				// Abbreviation timezone (type 2) like "GMT", "CET"
				buf.WriteString(name)
			} else {
				// Fixed-offset timezone (type 1): format as GMT+HHMM or GMT-HHMM
				sign := "+"
				absOffset := offset
				if offset < 0 {
					sign = "-"
					absOffset = -offset
				}
				hours := absOffset / 3600
				mins := (absOffset % 3600) / 60
				buf.WriteString(fmt.Sprintf("GMT%s%02d%02d", sign, hours, mins))
			}
		case 'Z': // Timezone offset in seconds
			_, offset := t.Zone()
			buf.WriteString(strconv.Itoa(offset))

		// Full Date/Time
		case 'c': // ISO 8601 date
			buf.WriteString(phpDateFormat("Y-m-d\\TH:i:sP", t))
		case 'r': // RFC 2822 formatted date
			buf.WriteString(phpDateFormat("D, d M Y H:i:s O", t))
		case 'U': // Seconds since Unix Epoch
			buf.WriteString(strconv.FormatInt(t.Unix(), 10))

		case 'B': // Swatch Internet time (beats)
			utcH := t.UTC().Hour()
			utcM := t.UTC().Minute()
			utcS := t.UTC().Second()
			beats := ((utcH+1)*3600 + utcM*60 + utcS) * 1000 / 86400
			buf.WriteString(fmt.Sprintf("%03d", beats%1000))

		case 'p': // Timezone identifier like P but with Z for UTC
			locName := t.Location().String()
			_, offset := t.Zone()
			// "Z" only for UTC, fixed +00:00, or other fixed zero-offset timezones.
			// Named timezones like Europe/London with offset 0 get "+00:00".
			isFixedZero := offset == 0 && (locName == "UTC" || locName == "+00:00" || locName == "Z" || locName == "" ||
				(len(locName) > 0 && (locName[0] == '+' || locName[0] == '-')))
			if isFixedZero {
				buf.WriteString("Z")
			} else {
				buf.WriteString(phpDateFormat("P", t))
			}

		case 'X': // An expanded full numeric representation of a year, always with sign prefix
			y := t.Year()
			if y < 0 {
				buf.WriteString(fmt.Sprintf("-%04d", -y))
			} else {
				buf.WriteString(fmt.Sprintf("+%04d", y))
			}

		case 'x': // An expanded full numeric representation if needed, or a standard representation if possible
			y := t.Year()
			if y < 0 || y > 9999 {
				if y < 0 {
					buf.WriteString(fmt.Sprintf("-%04d", -y))
				} else {
					buf.WriteString(fmt.Sprintf("+%d", y))
				}
			} else {
				buf.WriteString(fmt.Sprintf("%04d", y))
			}

		default:
			buf.WriteRune(c)
		}
	}
	return buf.String()
}

// > func string date ( string $format [, int $timestamp = time() ] )
func fncDate(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var format phpv.ZString
	var ts *phpv.ZInt
	_, err := core.Expand(ctx, args, &format, &ts)
	if err != nil {
		return nil, err
	}

	loc := getTimezone(ctx)
	var t time.Time
	if ts != nil {
		t = time.Unix(int64(*ts), 0).In(loc)
	} else {
		t = time.Now().In(loc)
	}

	result := phpDateFormat(string(format), t)
	return phpv.ZString(result).ZVal(), nil
}

// > func string gmdate ( string $format [, int $timestamp = time() ] )
func fncGmdate(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var format phpv.ZString
	var ts *phpv.ZInt
	_, err := core.Expand(ctx, args, &format, &ts)
	if err != nil {
		return nil, err
	}

	// Use GMT (not UTC) so that the 'T' format outputs "GMT" matching PHP behavior
	gmtLoc := time.FixedZone("GMT", 0)
	var t time.Time
	if ts != nil {
		t = time.Unix(int64(*ts), 0).In(gmtLoc)
	} else {
		t = time.Now().In(gmtLoc)
	}

	result := phpDateFormat(string(format), t)
	return phpv.ZString(result).ZVal(), nil
}

// > func int idate ( string $format [, int $timestamp = time() ] )
func fncIdate(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var format phpv.ZString
	var ts *phpv.ZInt
	_, err := core.Expand(ctx, args, &format, &ts)
	if err != nil {
		return nil, err
	}

	if len(format) != 1 {
		ctx.Warn("idate format is one char")
		return phpv.ZFalse.ZVal(), nil
	}

	loc := getTimezone(ctx)
	var t time.Time
	if ts != nil {
		t = time.Unix(int64(*ts), 0).In(loc)
	} else {
		t = time.Now().In(loc)
	}

	var result int
	switch format[0] {
	case 'B': // Swatch Internet time
		utcH := t.UTC().Hour()
		utcM := t.UTC().Minute()
		utcS := t.UTC().Second()
		beats := ((utcH+1)*3600 + utcM*60 + utcS) * 1000 / 86400
		result = beats % 1000
	case 'd':
		result = t.Day()
	case 'g':
		h := t.Hour() % 12
		if h == 0 {
			h = 12
		}
		result = h
	case 'G':
		result = t.Hour()
	case 'h':
		h := t.Hour() % 12
		if h == 0 {
			h = 12
		}
		result = h
	case 'H':
		result = t.Hour()
	case 'i':
		result = t.Minute()
	case 'I':
		if isDST(t) {
			result = 1
		} else {
			result = 0
		}
	case 'j':
		result = t.Day()
	case 'L':
		y := t.Year()
		if (y%4 == 0 && y%100 != 0) || y%400 == 0 {
			result = 1
		}
	case 'm':
		result = int(t.Month())
	case 'n':
		result = int(t.Month())
	case 'N':
		wd := int(t.Weekday())
		if wd == 0 {
			wd = 7
		}
		result = wd
	case 'o':
		year, _ := t.ISOWeek()
		result = year
	case 's':
		result = t.Second()
	case 't':
		lastDay := time.Date(t.Year(), t.Month()+1, 0, 0, 0, 0, 0, t.Location())
		result = lastDay.Day()
	case 'U':
		result = int(t.Unix())
	case 'w':
		result = int(t.Weekday())
	case 'W':
		_, week := t.ISOWeek()
		result = week
	case 'y':
		result = t.Year() % 100
	case 'Y':
		result = t.Year()
	case 'z':
		result = t.YearDay() - 1
	case 'Z':
		_, offset := t.Zone()
		result = offset
	default:
		ctx.Warn("Unrecognized date format token")
		return phpv.ZFalse.ZVal(), nil
	}

	return phpv.ZInt(result).ZVal(), nil
}

// > func int mktime ( [ int $hour = date("H") [, int $minute = date("i") [, int $second = date("s") [, int $month = date("n") [, int $day = date("j") [, int $year = date("Y") ]]]]]]] )
func fncMktime(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	if len(args) == 0 {
		return nil, phpobj.ThrowError(ctx, phpobj.ArgumentCountError, "mktime() expects at least 1 argument, 0 given")
	}
	if len(args) > 6 {
		return nil, phpobj.ThrowError(ctx, phpobj.ArgumentCountError, fmt.Sprintf("mktime() expects at most 6 arguments, %d given", len(args)))
	}
	var hourArg, minArg, secArg, monthArg, dayArg, yearArg *int
	_, err := core.Expand(ctx, args, &hourArg, &minArg, &secArg, &monthArg, &dayArg, &yearArg)
	if err != nil {
		return nil, err
	}

	loc := getTimezone(ctx)
	now := time.Now().In(loc)

	hour := now.Hour()
	min := now.Minute()
	sec := now.Second()
	month := now.Month()
	day := now.Day()
	year := now.Year()

	if hourArg != nil {
		hour = *hourArg
	}
	if minArg != nil {
		min = *minArg
	}
	if secArg != nil {
		sec = *secArg
	}
	if monthArg != nil {
		month = time.Month(*monthArg)
	}
	if dayArg != nil {
		day = *dayArg
	}
	if yearArg != nil {
		rawYear := *yearArg
		year = rawYear
		// PHP year normalization: 0-69 -> 2000-2069, 70-100 -> 1970-2000
		if rawYear >= 0 && rawYear <= 69 {
			year = rawYear + 2000
		} else if rawYear >= 70 && rawYear <= 100 {
			year = rawYear + 1900
		}
	}

	date := time.Date(year, month, day, hour, min, sec, 0, loc)
	return phpv.ZInt(date.Unix()).ZVal(), nil
}

// > func int gmmktime ( [ int $hour = gmdate("H") [, int $minute = gmdate("i") [, int $second = gmdate("s") [, int $month = gmdate("n") [, int $day = gmdate("j") [, int $year = gmdate("Y") ]]]]]]] )
func fncGmmktime(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var hourArg, minArg, secArg, monthArg, dayArg, yearArg *int
	_, err := core.Expand(ctx, args, &hourArg, &minArg, &secArg, &monthArg, &dayArg, &yearArg)
	if err != nil {
		return nil, err
	}

	now := time.Now().UTC()
	hour := now.Hour()
	min := now.Minute()
	sec := now.Second()
	month := now.Month()
	day := now.Day()
	year := now.Year()

	if hourArg != nil {
		hour = *hourArg
	}
	if minArg != nil {
		min = *minArg
	}
	if secArg != nil {
		sec = *secArg
	}
	if monthArg != nil {
		month = time.Month(*monthArg)
	}
	if dayArg != nil {
		day = *dayArg
	}
	if yearArg != nil {
		rawYear := *yearArg
		year = rawYear
		// PHP year normalization: 0-69 -> 2000-2069, 70-100 -> 1970-2000
		if rawYear >= 0 && rawYear <= 69 {
			year = rawYear + 2000
		} else if rawYear >= 70 && rawYear <= 100 {
			year = rawYear + 1900
		}
	}

	date := time.Date(year, month, day, hour, min, sec, 0, time.UTC)
	return phpv.ZInt(date.Unix()).ZVal(), nil
}

// > func array getdate ([ int $timestamp = time() ] )
func fncGetdate(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var ts *phpv.ZInt
	_, err := core.Expand(ctx, args, &ts)
	if err != nil {
		return nil, err
	}

	loc := getTimezone(ctx)
	var t time.Time
	if ts != nil {
		t = time.Unix(int64(*ts), 0).In(loc)
	} else {
		t = time.Now().In(loc)
	}

	result := phpv.NewZArray()
	result.OffsetSet(ctx, phpv.ZString("seconds"), phpv.ZInt(t.Second()).ZVal())
	result.OffsetSet(ctx, phpv.ZString("minutes"), phpv.ZInt(t.Minute()).ZVal())
	result.OffsetSet(ctx, phpv.ZString("hours"), phpv.ZInt(t.Hour()).ZVal())
	result.OffsetSet(ctx, phpv.ZString("mday"), phpv.ZInt(t.Day()).ZVal())
	result.OffsetSet(ctx, phpv.ZString("wday"), phpv.ZInt(int(t.Weekday())).ZVal())
	result.OffsetSet(ctx, phpv.ZString("mon"), phpv.ZInt(int(t.Month())).ZVal())
	result.OffsetSet(ctx, phpv.ZString("year"), phpv.ZInt(t.Year()).ZVal())
	result.OffsetSet(ctx, phpv.ZString("yday"), phpv.ZInt(t.YearDay()-1).ZVal())
	result.OffsetSet(ctx, phpv.ZString("weekday"), phpv.ZString(t.Weekday().String()).ZVal())
	result.OffsetSet(ctx, phpv.ZString("month"), phpv.ZString(t.Month().String()).ZVal())
	result.OffsetSet(ctx, phpv.ZInt(0), phpv.ZInt(t.Unix()).ZVal())

	return result.ZVal(), nil
}

// > func array localtime ([ int $timestamp = time() [, bool $isAssociative = false ]] )
func fncLocaltime(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var ts *phpv.ZInt
	var assoc *phpv.ZBool
	_, err := core.Expand(ctx, args, &ts, &assoc)
	if err != nil {
		return nil, err
	}

	loc := getTimezone(ctx)
	var t time.Time
	if ts != nil {
		t = time.Unix(int64(*ts), 0).In(loc)
	} else {
		t = time.Now().In(loc)
	}

	// Check if DST
	_, offset := t.Zone()
	_, stdOffset := time.Date(t.Year(), time.January, 1, 0, 0, 0, 0, t.Location()).Zone()
	isDST := 0
	if offset != stdOffset {
		isDST = 1
	}

	result := phpv.NewZArray()

	if assoc != nil && bool(*assoc) {
		result.OffsetSet(ctx, phpv.ZString("tm_sec"), phpv.ZInt(t.Second()).ZVal())
		result.OffsetSet(ctx, phpv.ZString("tm_min"), phpv.ZInt(t.Minute()).ZVal())
		result.OffsetSet(ctx, phpv.ZString("tm_hour"), phpv.ZInt(t.Hour()).ZVal())
		result.OffsetSet(ctx, phpv.ZString("tm_mday"), phpv.ZInt(t.Day()).ZVal())
		result.OffsetSet(ctx, phpv.ZString("tm_mon"), phpv.ZInt(int(t.Month())-1).ZVal())
		result.OffsetSet(ctx, phpv.ZString("tm_year"), phpv.ZInt(t.Year()-1900).ZVal())
		result.OffsetSet(ctx, phpv.ZString("tm_wday"), phpv.ZInt(int(t.Weekday())).ZVal())
		result.OffsetSet(ctx, phpv.ZString("tm_yday"), phpv.ZInt(t.YearDay()-1).ZVal())
		result.OffsetSet(ctx, phpv.ZString("tm_isdst"), phpv.ZInt(isDST).ZVal())
	} else {
		result.OffsetSet(ctx, nil, phpv.ZInt(t.Second()).ZVal())
		result.OffsetSet(ctx, nil, phpv.ZInt(t.Minute()).ZVal())
		result.OffsetSet(ctx, nil, phpv.ZInt(t.Hour()).ZVal())
		result.OffsetSet(ctx, nil, phpv.ZInt(t.Day()).ZVal())
		result.OffsetSet(ctx, nil, phpv.ZInt(int(t.Month())-1).ZVal())
		result.OffsetSet(ctx, nil, phpv.ZInt(t.Year()-1900).ZVal())
		result.OffsetSet(ctx, nil, phpv.ZInt(int(t.Weekday())).ZVal())
		result.OffsetSet(ctx, nil, phpv.ZInt(t.YearDay()-1).ZVal())
		result.OffsetSet(ctx, nil, phpv.ZInt(isDST).ZVal())
	}

	return result.ZVal(), nil
}

// > func bool checkdate ( int $month , int $day , int $year )
func fncCheckdate(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var month, day, year phpv.ZInt
	_, err := core.Expand(ctx, args, &month, &day, &year)
	if err != nil {
		return nil, err
	}

	m := int(month)
	d := int(day)
	y := int(year)

	if y < 1 || y > 32767 {
		return phpv.ZBool(false).ZVal(), nil
	}
	if m < 1 || m > 12 {
		return phpv.ZBool(false).ZVal(), nil
	}
	if d < 1 {
		return phpv.ZBool(false).ZVal(), nil
	}

	// Get the number of days in the given month
	lastDay := time.Date(y, time.Month(m+1), 0, 0, 0, 0, 0, time.UTC)
	if d > lastDay.Day() {
		return phpv.ZBool(false).ZVal(), nil
	}

	return phpv.ZBool(true).ZVal(), nil
}

// Regex patterns for strtotime parsing
var (
	reRelativeUnit = regexp.MustCompile(`(?i)([+-]?\s*\d+)\s+(year|month|week|day|hour|minute|second|min|sec)s?`)
	reNextLast     = regexp.MustCompile(`(?i)(next|last)\s+(sunday|monday|tuesday|wednesday|thursday|friday|saturday)`)
	reMDY          = regexp.MustCompile(`^\d{1,2}/\d{1,2}/\d{2,4}$`)
	reDMY          = regexp.MustCompile(`^\d{1,2}-\d{1,2}-\d{2,4}$`)
	reMonthDY      = regexp.MustCompile(`(?i)^(january|february|march|april|may|june|july|august|september|october|november|december)\s+(\d{1,2}),?\s+(\d{4})$`)
	reDMonY        = regexp.MustCompile(`(?i)^(\d{1,2})\s+(january|february|march|april|may|june|july|august|september|october|november|december)\s+(\d{4})$`)
	reAgo          = regexp.MustCompile(`(?i)(\d+)\s*(year|month|week|day|hour|minute|second|min|sec)s?\s+ago`)
	reNow          = regexp.MustCompile(`(?i)^now$`)
	reToday        = regexp.MustCompile(`(?i)^today$`)
	reTomorrow     = regexp.MustCompile(`(?i)^tomorrow$`)
	reYesterday    = regexp.MustCompile(`(?i)^yesterday$`)
	reMidnight     = regexp.MustCompile(`(?i)^midnight$`)
	reNoon         = regexp.MustCompile(`(?i)^noon$`)

	// ISO 8601 week date: YYYYWwwD or YYYYWww (with optional T time and tz)
	reISOWeek = regexp.MustCompile(`^(\d{4})W(\d{2})(\d)?(?:T(\d{2}):?(\d{2})?:?(\d{2})?)?(.*)?$`)

	// "Mon DD HH:MM:SS YYYY" e.g. "Sep 04 16:39:45 2001"
	reMonDTimeY = regexp.MustCompile(`(?i)^(jan(?:uary)?|feb(?:ruary)?|mar(?:ch)?|apr(?:il)?|may|jun(?:e)?|jul(?:y)?|aug(?:ust)?|sep(?:tember)?|oct(?:ober)?|nov(?:ember)?|dec(?:ember)?)\s+(\d{1,2})\s+(\d{1,2}):(\d{2}):(\d{2})\s+(\d{4})$`)

	// "Mon DD, YYYY HH:MM:SS TZ" e.g. "Nov 19, 2003 16:20:42 -0500"
	reMonDYTimeTZ = regexp.MustCompile(`(?i)^(jan(?:uary)?|feb(?:ruary)?|mar(?:ch)?|apr(?:il)?|may|jun(?:e)?|jul(?:y)?|aug(?:ust)?|sep(?:tember)?|oct(?:ober)?|nov(?:ember)?|dec(?:ember)?)\s+(\d{1,2}),?\s+(\d{4})\s+(\d{1,2}):(\d{2}):?(\d{2})?\s*([+-]\d{2}:?\d{2})?$`)

	// YYYY-MM format
	reYearMonthOnly = regexp.MustCompile(`^(\d{4})-(\d{2})$`)

	// AM/PM times: "3am", "12pm", "1pm", "1a.m.", "12p.m."
	reAmPm = regexp.MustCompile(`(?i)^(\d{1,2})\s*(am|pm|a\.m\.|p\.m\.)$`)

	// Day-name before date: "Mon 2005-11-14" or "Fri Nov 19 2003"
	// Day name followed by content that contains at least one digit (to avoid matching "Saturday" alone)
	reDayNameDate = regexp.MustCompile(`(?i)^(?:sunday|monday|tuesday|wednesday|thursday|friday|saturday|sun|mon|tue|wed|thu|fri|sat),?\s+(.*\d.*)$`)

	// RFC2822 with trailing timezone name: "Sun, 21 Dec 2003 20:38:33 +0000 GMT"
	reRFC2822Extra = regexp.MustCompile(`(?i)^[a-z]+,\s+(\d{1,2})\s+(jan(?:uary)?|feb(?:ruary)?|mar(?:ch)?|apr(?:il)?|may|jun(?:e)?|jul(?:y)?|aug(?:ust)?|sep(?:tember)?|oct(?:ober)?|nov(?:ember)?|dec(?:ember)?)\s+(\d{4})\s+(\d{2}):(\d{2}):(\d{2})\s+([+-]\d{4})\s+\(?[a-z]+\)?$`)

	// Timestamp with timezone comment: "Thu Nov 10 21:09:30 EST 2005" (like "Thu Nov 10 21:09:30 2005")
	// or  "Mon, 14 Nov 2005 09:05:00 +0000 (PST)" - offset with timezone comment in parens
	reOffsetWithComment = regexp.MustCompile(`(?i)^(.+[+-]\d{4})\s+\([A-Z]+\)$`)

	// "DD.MM.YYYY HH:MM" format
	reDotDate = regexp.MustCompile(`^(\d{1,2})\.(\d{1,2})\.(\d{4})(?:\s+(\d{1,2}):(\d{2})(?::(\d{2}))?)?$`)

	// Ordinal day: "November 26th" or "26th November 2005" (only when digits precede the suffix)
	reOrdinal = regexp.MustCompile(`\b(\d{1,2})(?:st|nd|rd|th)\b`)
)

var monthNames = map[string]time.Month{
	"january":   time.January,
	"february":  time.February,
	"march":     time.March,
	"april":     time.April,
	"may":       time.May,
	"june":      time.June,
	"july":      time.July,
	"august":    time.August,
	"september": time.September,
	"october":   time.October,
	"november":  time.November,
	"december":  time.December,
	"jan":       time.January,
	"feb":       time.February,
	"mar":       time.March,
	"apr":       time.April,
	"jun":       time.June,
	"jul":       time.July,
	"aug":       time.August,
	"sep":       time.September,
	"oct":       time.October,
	"nov":       time.November,
	"dec":       time.December,
}

var dayNames = map[string]time.Weekday{
	"sunday":    time.Sunday,
	"monday":    time.Monday,
	"tuesday":   time.Tuesday,
	"wednesday": time.Wednesday,
	"thursday":  time.Thursday,
	"friday":    time.Friday,
	"saturday":  time.Saturday,
}

func parseMonth(name string) (time.Month, bool) {
	m, ok := monthNames[strings.ToLower(name)]
	return m, ok
}

func parseWeekday(name string) (time.Weekday, bool) {
	d, ok := dayNames[strings.ToLower(name)]
	return d, ok
}

// parseTZOffset parses a timezone offset like "+02", "-0213", "+02:13", "+0", "-07:00", "-0700" etc.
// Returns offset in seconds and whether it was successfully parsed.
func parseTZOffset(s string) (int, bool) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, false
	}
	sign := 1
	if s[0] == '+' {
		s = s[1:]
	} else if s[0] == '-' {
		sign = -1
		s = s[1:]
	} else {
		return 0, false
	}

	// Check for seconds (HH:MM:SS format) first
	secs := 0
	parts := strings.Split(s, ":")
	if len(parts) == 3 {
		// HH:MM:SS format
		var err error
		hours, err := strconv.Atoi(parts[0])
		if err != nil {
			return 0, false
		}
		mins, err := strconv.Atoi(parts[1])
		if err != nil {
			return 0, false
		}
		secs, err = strconv.Atoi(parts[2])
		if err != nil {
			return 0, false
		}
		return sign * (hours*3600 + mins*60 + secs), true
	}

	// Remove colon if present (e.g. "02:13" -> "0213")
	s = strings.Replace(s, ":", "", 1)

	hours := 0
	mins := 0
	var err error
	switch len(s) {
	case 1: // e.g. "2" -> 2 hours
		hours, err = strconv.Atoi(s)
		if err != nil {
			return 0, false
		}
	case 2: // e.g. "02" -> 2 hours
		hours, err = strconv.Atoi(s)
		if err != nil {
			return 0, false
		}
	case 3: // e.g. "500" (from "5:00") -> 5 hours 0 minutes
		hours, err = strconv.Atoi(s[:1])
		if err != nil {
			return 0, false
		}
		mins, err = strconv.Atoi(s[1:3])
		if err != nil {
			return 0, false
		}
	case 4: // e.g. "0213" -> 2 hours 13 minutes
		hours, err = strconv.Atoi(s[:2])
		if err != nil {
			return 0, false
		}
		mins, err = strconv.Atoi(s[2:4])
		if err != nil {
			return 0, false
		}
	default:
		return 0, false
	}

	return sign * (hours*3600 + mins*60 + secs), true
}

// parseISOWeekDate parses ISO 8601 week date strings like "1997W011", "2004W101T05:00+0"
func parseISOWeekDate(input string, loc *time.Location) (time.Time, bool) {
	matches := reISOWeek.FindStringSubmatch(input)
	if matches == nil {
		return time.Time{}, false
	}

	year, _ := strconv.Atoi(matches[1])
	week, _ := strconv.Atoi(matches[2])
	day := 1 // default Monday
	if matches[3] != "" {
		day, _ = strconv.Atoi(matches[3])
	}

	hour, min, sec := 0, 0, 0
	if matches[4] != "" {
		hour, _ = strconv.Atoi(matches[4])
	}
	if matches[5] != "" {
		min, _ = strconv.Atoi(matches[5])
	}
	if matches[6] != "" {
		sec, _ = strconv.Atoi(matches[6])
	}

	// Calculate the date from ISO week
	// January 4 is always in week 1
	jan4 := time.Date(year, time.January, 4, 0, 0, 0, 0, loc)
	// Monday of week 1
	isoWeek1Monday := jan4.AddDate(0, 0, -int(jan4.Weekday()-time.Monday))
	if jan4.Weekday() == time.Sunday {
		isoWeek1Monday = jan4.AddDate(0, 0, -6)
	}
	// Add weeks and days
	t := isoWeek1Monday.AddDate(0, 0, (week-1)*7+(day-1))
	t = time.Date(t.Year(), t.Month(), t.Day(), hour, min, sec, 0, loc)

	// Handle timezone offset in remainder
	remainder := strings.TrimSpace(matches[7])
	if remainder != "" {
		if offset, ok := parseTZOffset(remainder); ok {
			t = time.Date(t.Year(), t.Month(), t.Day(), hour, min, sec, 0, makeFixedZone(offset))
		}
	}

	return t, true
}

// applyRelativeUnit applies a relative time unit to a time value.
func applyRelativeUnit(t time.Time, amount int, unit string) time.Time {
	unit = strings.ToLower(unit)
	switch unit {
	case "year", "years":
		return t.AddDate(amount, 0, 0)
	case "month", "months":
		return t.AddDate(0, amount, 0)
	case "week", "weeks":
		return t.AddDate(0, 0, amount*7)
	case "day", "days":
		return t.AddDate(0, 0, amount)
	case "hour", "hours":
		return t.Add(time.Duration(amount) * time.Hour)
	case "minute", "minutes", "min", "mins":
		return t.Add(time.Duration(amount) * time.Minute)
	case "second", "seconds", "sec", "secs":
		return t.Add(time.Duration(amount) * time.Second)
	}
	return t
}

// parsePHPCompoundModifier handles PHP-specific compound date modifiers like:
// "first day of next month", "last thursday of next month", "3 tuesday"
// Input should already be lowercased and trimmed.
func parsePHPCompoundModifier(input string, base time.Time) (time.Time, bool) {
	loc := base.Location()

	// Parse "N weekday" where N is an ordinal (1..5) - finds the Nth weekday in current month
	// e.g. "3 tuesday" = 3rd Tuesday of the current month
	reOrdinalWeekday := regexp.MustCompile(`^(\d+)\s+(sunday|monday|tuesday|wednesday|thursday|friday|saturday)$`)
	if m := reOrdinalWeekday.FindStringSubmatch(input); m != nil {
		n, _ := strconv.Atoi(m[1])
		wd, ok := parseWeekday(m[2])
		if ok && n >= 1 {
			y, mo, _ := base.Date()
			// Find the Nth occurrence of wd in month mo of year y
			// Start from day 1 of the month
			first := time.Date(y, mo, 1, base.Hour(), base.Minute(), base.Second(), base.Nanosecond(), loc)
			// Find first occurrence of the weekday
			diff := int(wd) - int(first.Weekday())
			if diff < 0 {
				diff += 7
			}
			result := first.AddDate(0, 0, diff+(n-1)*7)
			// Make sure we're still in the same month
			if result.Month() == mo {
				return result, true
			}
		}
	}

	// Handle "last weekday of [period]" e.g. "last thursday of next month"
	reLastWD := regexp.MustCompile(`^last\s+(sunday|monday|tuesday|wednesday|thursday|friday|saturday)\s+of\s+(.+)$`)
	if m := reLastWD.FindStringSubmatch(input); m != nil {
		wd, ok := parseWeekday(m[1])
		if ok {
			// Get the target month
			var targetMonth time.Time
			period := strings.TrimSpace(m[2])
			switch period {
			case "this month":
				y, mo, _ := base.Date()
				targetMonth = time.Date(y, mo, 1, 0, 0, 0, 0, loc)
			case "next month":
				y, mo, _ := base.Date()
				targetMonth = time.Date(y, mo+1, 1, 0, 0, 0, 0, loc)
			case "last month":
				y, mo, _ := base.Date()
				targetMonth = time.Date(y, mo-1, 1, 0, 0, 0, 0, loc)
			}
			if !targetMonth.IsZero() {
				// Find last day of targetMonth
				lastDay := time.Date(targetMonth.Year(), targetMonth.Month()+1, 0, 0, 0, 0, 0, loc)
				// Find last occurrence of wd
				diff := int(lastDay.Weekday()) - int(wd)
				if diff < 0 {
					diff += 7
				}
				result := lastDay.AddDate(0, 0, -diff)
				return result, true
			}
		}
	}

	// Handle "first/last day of [period]" e.g. "first day of next month"
	reFirstLastDay := regexp.MustCompile(`^(first|last)\s+day\s+of\s+(.+)$`)
	if m := reFirstLastDay.FindStringSubmatch(input); m != nil {
		isFirst := m[1] == "first"
		period := strings.TrimSpace(m[2])
		y, mo, _ := base.Date()
		switch period {
		case "this month":
			if isFirst {
				return time.Date(y, mo, 1, base.Hour(), base.Minute(), base.Second(), 0, loc), true
			}
			// last day of this month
			last := time.Date(y, mo+1, 0, base.Hour(), base.Minute(), base.Second(), 0, loc)
			return last, true
		case "next month":
			if isFirst {
				return time.Date(y, mo+1, 1, base.Hour(), base.Minute(), base.Second(), 0, loc), true
			}
			// last day of next month
			last := time.Date(y, mo+2, 0, base.Hour(), base.Minute(), base.Second(), 0, loc)
			return last, true
		case "last month":
			if isFirst {
				return time.Date(y, mo-1, 1, base.Hour(), base.Minute(), base.Second(), 0, loc), true
			}
			last := time.Date(y, mo, 0, base.Hour(), base.Minute(), base.Second(), 0, loc)
			return last, true
		case "january", "february", "march", "april", "may", "june",
			"july", "august", "september", "october", "november", "december":
			mo2, ok := parseMonth(period)
			if ok {
				if isFirst {
					return time.Date(y, mo2, 1, base.Hour(), base.Minute(), base.Second(), 0, loc), true
				}
				last := time.Date(y, mo2+1, 0, base.Hour(), base.Minute(), base.Second(), 0, loc)
				return last, true
			}
		}
	}

	return time.Time{}, false
}

var reDateTimeTZ = regexp.MustCompile(`^(-?\d{4})-(\d{2})-(\d{2})[T ](\d{2}):?(\d{2}):?(\d{2})?\s*([+-]\d{1,4}(?::?\d{2})?)?$`)
var reDateOnly = regexp.MustCompile(`^(-?\d{4})-(\d{2})-(\d{2})$`)
var reCompactDateTimeTZ = regexp.MustCompile(`^(\d{4})(\d{2})(\d{2})T(\d{2}):?(\d{2}):?(\d{2})?(.*)?$`)

// makeFixedZone creates a fixed zone with a properly formatted name like "+02:00" or "-08:00"
func makeFixedZone(offset int) *time.Location {
	sign := "+"
	absOffset := offset
	if offset < 0 {
		sign = "-"
		absOffset = -offset
	}
	hours := absOffset / 3600
	mins := (absOffset % 3600) / 60
	secs := absOffset % 60
	var name string
	if secs != 0 {
		name = fmt.Sprintf("%s%02d:%02d:%02d", sign, hours, mins, secs)
	} else {
		name = fmt.Sprintf("%s%02d:%02d", sign, hours, mins)
	}
	return time.FixedZone(name, offset)
}

func parseDateTimeWithOffset(input string, loc *time.Location) (time.Time, bool) {
	// Try standard date-time with optional timezone: YYYY-MM-DD[T ]HH:MM:SS[+-offset]
	if matches := reDateTimeTZ.FindStringSubmatch(input); matches != nil {
		year, _ := strconv.Atoi(matches[1])
		month, _ := strconv.Atoi(matches[2])
		day, _ := strconv.Atoi(matches[3])
		hour, _ := strconv.Atoi(matches[4])
		min, _ := strconv.Atoi(matches[5])
		sec := 0
		if matches[6] != "" {
			sec, _ = strconv.Atoi(matches[6])
		}

		tzPart := strings.TrimSpace(matches[7])
		if tzPart != "" {
			if offset, ok := parseTZOffset(tzPart); ok {
				return time.Date(year, time.Month(month), day, hour, min, sec, 0, makeFixedZone(offset)), true
			}
		}
		// No timezone offset, use provided location
		return time.Date(year, time.Month(month), day, hour, min, sec, 0, loc), true
	}

	// Try date-only with potentially negative year: YYYY-MM-DD
	if matches := reDateOnly.FindStringSubmatch(input); matches != nil {
		year, _ := strconv.Atoi(matches[1])
		month, _ := strconv.Atoi(matches[2])
		day, _ := strconv.Atoi(matches[3])
		// Validate that month/day are in valid range to avoid Go's date normalization
		// (e.g. 9999-11-33 would silently become 9999-12-03)
		if month < 1 || month > 12 || day < 1 {
			return time.Time{}, false
		}
		t := time.Date(year, time.Month(month), day, 0, 0, 0, 0, loc)
		if t.Year() != year || int(t.Month()) != month || t.Day() != day {
			return time.Time{}, false
		}
		return t, true
	}

	// Try compact date-time: YYYYMMDDTHHMMSS[+-offset]
	if matches := reCompactDateTimeTZ.FindStringSubmatch(input); matches != nil {
		year, _ := strconv.Atoi(matches[1])
		month, _ := strconv.Atoi(matches[2])
		day, _ := strconv.Atoi(matches[3])
		hour, _ := strconv.Atoi(matches[4])
		min, _ := strconv.Atoi(matches[5])
		sec := 0
		if matches[6] != "" {
			sec, _ = strconv.Atoi(matches[6])
		}

		tzPart := strings.TrimSpace(matches[7])
		if tzPart != "" {
			if offset, ok := parseTZOffset(tzPart); ok {
				return time.Date(year, time.Month(month), day, hour, min, sec, 0, makeFixedZone(offset)), true
			}
		}
		return time.Date(year, time.Month(month), day, hour, min, sec, 0, loc), true
	}

	return time.Time{}, false
}

// reMonthNameFull matches "Mon DD YYYY HH:MM", "Mon DD YYYY", "Mon DD HH:MM"
var reMonthNameDateFull = regexp.MustCompile(`(?i)^(jan(?:uary)?|feb(?:ruary)?|mar(?:ch)?|apr(?:il)?|may|jun(?:e)?|jul(?:y)?|aug(?:ust)?|sep(?:tember)?|oct(?:ober)?|nov(?:ember)?|dec(?:ember)?)\s+(\d{1,2})\s+(\d{4})\s+(\d{1,2}):(\d{2})(?::(\d{2}))?$`)
var reMonthNameDateNoTime = regexp.MustCompile(`(?i)^(jan(?:uary)?|feb(?:ruary)?|mar(?:ch)?|apr(?:il)?|may|jun(?:e)?|jul(?:y)?|aug(?:ust)?|sep(?:tember)?|oct(?:ober)?|nov(?:ember)?|dec(?:ember)?)\s+(\d{1,2})\s+(\d{4})$`)
var reMonthNameTime = regexp.MustCompile(`(?i)^(jan(?:uary)?|feb(?:ruary)?|mar(?:ch)?|apr(?:il)?|may|jun(?:e)?|jul(?:y)?|aug(?:ust)?|sep(?:tember)?|oct(?:ober)?|nov(?:ember)?|dec(?:ember)?)\s+(\d{1,2})\s+(\d{1,2}):(\d{2})(?::(\d{2}))?$`)
var reMonthYear = regexp.MustCompile(`(?i)^(jan(?:uary)?|feb(?:ruary)?|mar(?:ch)?|apr(?:il)?|may|jun(?:e)?|jul(?:y)?|aug(?:ust)?|sep(?:tember)?|oct(?:ober)?|nov(?:ember)?|dec(?:ember)?)\s+(\d{4})$`)
var reYearMonth = regexp.MustCompile(`(?i)^(\d{4})\s+(jan(?:uary)?|feb(?:ruary)?|mar(?:ch)?|apr(?:il)?|may|jun(?:e)?|jul(?:y)?|aug(?:ust)?|sep(?:tember)?|oct(?:ober)?|nov(?:ember)?|dec(?:ember)?)$`)
var reMonthDay = regexp.MustCompile(`(?i)^(jan(?:uary)?|feb(?:ruary)?|mar(?:ch)?|apr(?:il)?|may|jun(?:e)?|jul(?:y)?|aug(?:ust)?|sep(?:tember)?|oct(?:ober)?|nov(?:ember)?|dec(?:ember)?)\s+(\d{1,2})$`)

func parseMonthNameDate(input string, base time.Time, loc *time.Location) (time.Time, bool) {
	// "Mon DD YYYY HH:MM[:SS]"
	if matches := reMonthNameDateFull.FindStringSubmatch(input); matches != nil {
		m, ok := parseMonth(matches[1])
		if !ok {
			return time.Time{}, false
		}
		d, _ := strconv.Atoi(matches[2])
		y, _ := strconv.Atoi(matches[3])
		hour, _ := strconv.Atoi(matches[4])
		min, _ := strconv.Atoi(matches[5])
		sec := 0
		if matches[6] != "" {
			sec, _ = strconv.Atoi(matches[6])
		}
		return time.Date(y, m, d, hour, min, sec, 0, loc), true
	}

	// "Mon DD YYYY"
	if matches := reMonthNameDateNoTime.FindStringSubmatch(input); matches != nil {
		m, ok := parseMonth(matches[1])
		if !ok {
			return time.Time{}, false
		}
		d, _ := strconv.Atoi(matches[2])
		y, _ := strconv.Atoi(matches[3])
		return time.Date(y, m, d, 0, 0, 0, 0, loc), true
	}

	// "Mon DD HH:MM[:SS]" - no year, use base year
	if matches := reMonthNameTime.FindStringSubmatch(input); matches != nil {
		m, ok := parseMonth(matches[1])
		if !ok {
			return time.Time{}, false
		}
		d, _ := strconv.Atoi(matches[2])
		hour, _ := strconv.Atoi(matches[3])
		min, _ := strconv.Atoi(matches[4])
		sec := 0
		if matches[5] != "" {
			sec, _ = strconv.Atoi(matches[5])
		}
		return time.Date(base.Year(), m, d, hour, min, sec, 0, loc), true
	}

	// "Mon YYYY" - month year
	if matches := reMonthYear.FindStringSubmatch(input); matches != nil {
		m, ok := parseMonth(matches[1])
		if !ok {
			return time.Time{}, false
		}
		y, _ := strconv.Atoi(matches[2])
		return time.Date(y, m, 1, 0, 0, 0, 0, loc), true
	}

	// "YYYY Mon" - year month
	if matches := reYearMonth.FindStringSubmatch(input); matches != nil {
		y, _ := strconv.Atoi(matches[1])
		m, ok := parseMonth(matches[2])
		if !ok {
			return time.Time{}, false
		}
		return time.Date(y, m, 1, 0, 0, 0, 0, loc), true
	}

	// "Month Day" (without year) - uses base year, or next year if the date has passed
	if matches := reMonthDay.FindStringSubmatch(input); matches != nil {
		m, ok := parseMonth(matches[1])
		if ok {
			d, _ := strconv.Atoi(matches[2])
			y := base.Year()
			candidate := time.Date(y, m, d, 0, 0, 0, 0, loc)
			// If the date is in the past relative to the base, use next year
			if candidate.Before(base) {
				y++
			}
			return time.Date(y, m, d, 0, 0, 0, 0, loc), true
		}
	}

	return time.Time{}, false
}

// > func int strtotime ( string $datetime [, int $baseTimestamp = time() ] )
func fncStrtotime(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	// Validate second argument type before expansion - PHP throws TypeError for non-int
	if len(args) >= 2 && args[1] != nil && !args[1].IsNull() {
		t := args[1].GetType()
		if t != phpv.ZtInt && t != phpv.ZtFloat && t != phpv.ZtString && t != phpv.ZtBool && t != phpv.ZtNull {
			typeName := "unknown"
			if t == phpv.ZtObject {
				if obj, ok := args[1].Value().(phpv.ZObject); ok {
					typeName = string(obj.GetClass().GetName())
				}
			} else {
				typeName = t.TypeName()
			}
			return nil, phpobj.ThrowError(ctx, phpobj.TypeError, fmt.Sprintf("strtotime(): Argument #2 ($baseTimestamp) must be of type ?int, %s given", typeName))
		}
	}
	var datetime phpv.ZString
	var baseTs *phpv.ZInt
	_, err := core.Expand(ctx, args, &datetime, &baseTs)
	if err != nil {
		return nil, err
	}

	loc := getTimezone(ctx)
	var base time.Time
	if baseTs != nil {
		base = time.Unix(int64(*baseTs), 0).In(loc)
	} else {
		base = time.Now().In(loc)
	}

	opts := []strtotime.Option{strtotime.InTZ(loc), strtotime.Rel(base)}
	normalizedDT := normalizeRelativeDateStr(string(datetime))
	t, stErr := strtotime.StrToTime(normalizedDT, opts...)
	if stErr != nil {
		return phpv.ZBool(false).ZVal(), nil
	}
	return phpv.ZInt(t.Unix()).ZVal(), nil
}
