package date

import (
	"math"
	"time"

	"github.com/MagicalTux/goro/core/phpobj"
	"github.com/MagicalTux/goro/core/phpv"
)

var DateTimeInterface *phpobj.ZClass
var DateTime *phpobj.ZClass
var DateTimeImmutable *phpobj.ZClass
var DateInterval *phpobj.ZClass

func parseDateTime(ctx phpv.Context, args []*phpv.ZVal) time.Time {
	var t time.Time
	if len(args) > 0 && !args[0].IsNull() {
		dateStr := args[0].AsString(ctx)
		// Try common formats
		for _, layout := range []string{
			"2006-01-02 15:04:05 MST",
			"2006-01-02 15:04:05",
			"2006-01-02",
			time.RFC3339,
		} {
			if parsed, err := time.Parse(layout, string(dateStr)); err == nil {
				t = parsed
				break
			}
		}
		if t.IsZero() {
			t = time.Now()
		}
	} else {
		t = time.Now()
	}
	return t
}

func getTime(this *phpobj.ZObject) (time.Time, bool) {
	if v, ok := this.Opaque[DateTimeInterface]; ok {
		return v.(time.Time), true
	}
	return time.Time{}, false
}

func setTime(this *phpobj.ZObject, t time.Time) {
	this.Opaque[DateTimeInterface] = t
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

	// Calculate year/month/day differences matching PHP's behavior
	y1, m1, d1 := from.Date()
	y2, m2, d2 := to.Date()
	h1, min1, s1 := from.Clock()
	h2, min2, s2 := to.Clock()

	years := y2 - y1
	months := int(m2) - int(m1)
	days := d2 - d1
	hours := h2 - h1
	minutes := min2 - min1
	seconds := s2 - s1

	if seconds < 0 {
		seconds += 60
		minutes--
	}
	if minutes < 0 {
		minutes += 60
		hours--
	}
	if hours < 0 {
		hours += 24
		days--
	}
	if days < 0 {
		// Get number of days in the previous month
		prevMonth := time.Date(y2, m2, 0, 0, 0, 0, 0, from.Location())
		days += prevMonth.Day()
		months--
	}
	if months < 0 {
		months += 12
		years--
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
	if invert {
		intervalObj.ObjectSet(ctx, phpv.ZString("invert"), phpv.ZInt(1).ZVal())
	} else {
		intervalObj.ObjectSet(ctx, phpv.ZString("invert"), phpv.ZInt(0).ZVal())
	}

	return intervalObj.ZVal(), nil
}

func init() {
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
		Methods: map[phpv.ZString]*phpv.ZClassMethod{},
	}

	// DateTime class
	DateTime = &phpobj.ZClass{
		Name: "DateTime",
		Implementations: []*phpobj.ZClass{DateTimeInterface},
		Props:           []*phpv.ZClassProp{},
		Methods: map[phpv.ZString]*phpv.ZClassMethod{
			"__construct": {
				Name:      "__construct",
				Modifiers: phpv.ZAttrPublic,
				Method: phpobj.NativeMethod(func(ctx phpv.Context, this *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
					t := parseDateTime(ctx, args)
					setTime(this, t)
					return nil, nil
				}),
			},
			"diff": {
				Name:      "diff",
				Modifiers: phpv.ZAttrPublic,
				Method:    phpobj.NativeMethod(diffMethod),
			},
		},
	}

	// DateTimeImmutable class
	DateTimeImmutable = &phpobj.ZClass{
		Name: "DateTimeImmutable",
		Implementations: []*phpobj.ZClass{DateTimeInterface},
		Props:           []*phpv.ZClassProp{},
		Methods: map[phpv.ZString]*phpv.ZClassMethod{
			"__construct": {
				Name:      "__construct",
				Modifiers: phpv.ZAttrPublic,
				Method: phpobj.NativeMethod(func(ctx phpv.Context, this *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
					t := parseDateTime(ctx, args)
					setTime(this, t)
					return nil, nil
				}),
			},
			"diff": {
				Name:      "diff",
				Modifiers: phpv.ZAttrPublic,
				Method:    phpobj.NativeMethod(diffMethod),
			},
		},
	}
}
