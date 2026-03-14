package date

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/MagicalTux/goro/core"
	"github.com/MagicalTux/goro/core/phpv"
)

// getTimezone returns the timezone configured via date.timezone, falling back to UTC.
func getTimezone(ctx phpv.Context) *time.Location {
	tzName := ctx.GetConfig("date.timezone", phpv.ZString("UTC").ZVal()).String()
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
			buf.WriteString(fmt.Sprintf("%04d", t.Year()))
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
			buf.WriteString(t.Location().String())
		case 'I': // Whether daylight saving time (1 if DST, 0 otherwise)
			_, offset := t.Zone()
			_, stdOffset := time.Date(t.Year(), time.January, 1, 0, 0, 0, 0, t.Location()).Zone()
			if offset != stdOffset {
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
			name, _ := t.Zone()
			buf.WriteString(name)
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

	var t time.Time
	if ts != nil {
		t = time.Unix(int64(*ts), 0).UTC()
	} else {
		t = time.Now().UTC()
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
		return nil, ctx.Errorf("idate(): idate format is one char")
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
		_, offset := t.Zone()
		_, stdOffset := time.Date(t.Year(), time.January, 1, 0, 0, 0, 0, t.Location()).Zone()
		if offset != stdOffset {
			result = 1
		} else {
			result = 0
		}
	case 'L':
		y := t.Year()
		if (y%4 == 0 && y%100 != 0) || y%400 == 0 {
			result = 1
		}
	case 'm':
		result = int(t.Month())
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
		return nil, ctx.Errorf("idate(): Unrecognized date format token")
	}

	return phpv.ZInt(result).ZVal(), nil
}

// > func int mktime ( [ int $hour = date("H") [, int $minute = date("i") [, int $second = date("s") [, int $month = date("n") [, int $day = date("j") [, int $year = date("Y") ]]]]]]] )
func fncMktime(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
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
		year = *yearArg
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
		year = *yearArg
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
	reRelativeUnit = regexp.MustCompile(`(?i)([+-]?\d+)\s*(year|month|week|day|hour|minute|second|min|sec)s?`)
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

// strToTime parses a date/time string relative to a base time.
func strToTime(input string, base time.Time) (time.Time, bool) {
	input = strings.TrimSpace(input)
	if input == "" {
		return base, false
	}

	// Simple keywords
	switch {
	case reNow.MatchString(input):
		return base, true
	case reToday.MatchString(input):
		y, m, d := base.Date()
		return time.Date(y, m, d, 0, 0, 0, 0, base.Location()), true
	case reTomorrow.MatchString(input):
		y, m, d := base.Date()
		return time.Date(y, m, d+1, 0, 0, 0, 0, base.Location()), true
	case reYesterday.MatchString(input):
		y, m, d := base.Date()
		return time.Date(y, m, d-1, 0, 0, 0, 0, base.Location()), true
	case reMidnight.MatchString(input):
		y, m, d := base.Date()
		return time.Date(y, m, d, 0, 0, 0, 0, base.Location()), true
	case reNoon.MatchString(input):
		y, m, d := base.Date()
		return time.Date(y, m, d, 12, 0, 0, 0, base.Location()), true
	}

	// "N ago" format
	if matches := reAgo.FindStringSubmatch(input); matches != nil {
		amount, _ := strconv.Atoi(matches[1])
		return applyRelativeUnit(base, -amount, matches[2]), true
	}

	// "next/last weekday"
	if matches := reNextLast.FindStringSubmatch(input); matches != nil {
		direction := strings.ToLower(matches[1])
		target, ok := parseWeekday(matches[2])
		if ok {
			current := base.Weekday()
			var diff int
			if direction == "next" {
				diff = int(target) - int(current)
				if diff <= 0 {
					diff += 7
				}
			} else {
				diff = int(current) - int(target)
				if diff <= 0 {
					diff += 7
				}
				diff = -diff
			}
			return base.AddDate(0, 0, diff), true
		}
	}

	// Relative expressions: "+1 day", "-2 hours", "1 month", etc.
	if matches := reRelativeUnit.FindAllStringSubmatch(input, -1); len(matches) > 0 {
		t := base
		for _, match := range matches {
			amount, _ := strconv.Atoi(match[1])
			t = applyRelativeUnit(t, amount, match[2])
		}
		return t, true
	}

	// Try various absolute date formats
	loc := base.Location()

	// ISO 8601 with timezone
	for _, layout := range []string{
		time.RFC3339,
		"2006-01-02T15:04:05-07:00",
		"2006-01-02T15:04:05Z",
		"2006-01-02T15:04:05",
		"2006-01-02 15:04:05-07:00",
		"2006-01-02 15:04:05",
		"2006-01-02 15:04",
		"2006-01-02",
		"2006/01/02",
		"01/02/2006",
		"1/2/2006",
		"02-Jan-2006",
		"02-Jan-2006 15:04:05",
		"Mon, 02 Jan 2006 15:04:05 -0700",
		time.RFC1123Z,
		time.RFC1123,
		time.RFC822Z,
		time.RFC822,
		time.ANSIC,
		time.UnixDate,
		"Jan 2, 2006",
		"January 2, 2006",
		"Jan 2 2006",
		"January 2 2006",
		"2 Jan 2006",
		"2 January 2006",
		"20060102",
		"20060102150405",
		"15:04:05",
		"15:04",
	} {
		if t, err := time.ParseInLocation(layout, input, loc); err == nil {
			// If we only parsed a time (no date), apply it to the base date
			if layout == "15:04:05" || layout == "15:04" {
				y, m, d := base.Date()
				return time.Date(y, m, d, t.Hour(), t.Minute(), t.Second(), 0, loc), true
			}
			return t, true
		}
	}

	// "Month day, year" format
	if matches := reMonthDY.FindStringSubmatch(input); matches != nil {
		m, ok := parseMonth(matches[1])
		if ok {
			d, _ := strconv.Atoi(matches[2])
			y, _ := strconv.Atoi(matches[3])
			return time.Date(y, m, d, 0, 0, 0, 0, loc), true
		}
	}

	// "day Month year" format
	if matches := reDMonY.FindStringSubmatch(input); matches != nil {
		d, _ := strconv.Atoi(matches[1])
		m, ok := parseMonth(matches[2])
		if ok {
			y, _ := strconv.Atoi(matches[3])
			return time.Date(y, m, d, 0, 0, 0, 0, loc), true
		}
	}

	// M/D/Y format
	if reMDY.MatchString(input) {
		parts := strings.Split(input, "/")
		m, _ := strconv.Atoi(parts[0])
		d, _ := strconv.Atoi(parts[1])
		y, _ := strconv.Atoi(parts[2])
		if y < 100 {
			if y >= 70 {
				y += 1900
			} else {
				y += 2000
			}
		}
		return time.Date(y, time.Month(m), d, 0, 0, 0, 0, loc), true
	}

	// D-M-Y format (European)
	if reDMY.MatchString(input) {
		parts := strings.Split(input, "-")
		if len(parts) == 3 {
			// Check if middle part is numeric
			d, err1 := strconv.Atoi(parts[0])
			m, err2 := strconv.Atoi(parts[1])
			y, err3 := strconv.Atoi(parts[2])
			if err1 == nil && err2 == nil && err3 == nil {
				if y < 100 {
					if y >= 70 {
						y += 1900
					} else {
						y += 2000
					}
				}
				return time.Date(y, time.Month(m), d, 0, 0, 0, 0, loc), true
			}
		}
	}

	// Handle composite relative expressions like "next Monday +2 hours"
	// or "tomorrow 14:00" or "+1 day 12:00:00"
	words := strings.Fields(input)
	if len(words) >= 2 {
		// Try splitting and parsing parts
		// First try the first word(s) as a keyword/relative, then apply rest
		for split := 1; split < len(words); split++ {
			firstPart := strings.Join(words[:split], " ")
			secondPart := strings.Join(words[split:], " ")

			if t1, ok1 := strToTime(firstPart, base); ok1 {
				if t2, ok2 := strToTime(secondPart, t1); ok2 {
					return t2, true
				}
			}
		}
	}

	// Try Go's built-in relaxed parse with the @ prefix for unix timestamps
	if len(input) > 0 && input[0] == '@' {
		ts, err := strconv.ParseInt(input[1:], 10, 64)
		if err == nil {
			return time.Unix(ts, 0).In(loc), true
		}
	}

	return base, false
}

// > func int strtotime ( string $datetime [, int $baseTimestamp = time() ] )
func fncStrtotime(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
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

	t, ok := strToTime(string(datetime), base)
	if !ok {
		return phpv.ZBool(false).ZVal(), nil
	}

	return phpv.ZInt(t.Unix()).ZVal(), nil
}
