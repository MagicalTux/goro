package locale

import (
	"testing"

	"github.com/KarpelesLab/goro/core/phpv"
)

func reset(t *testing.T) {
	t.Helper()
	mu.Lock()
	for i := range current {
		current[i] = cLocale
	}
	mu.Unlock()
}

func TestCLocaleBasics(t *testing.T) {
	reset(t)

	if got := GetLocale(LC_ALL); got != "C" {
		t.Errorf("LC_ALL = %q, want C", got)
	}
	if got := LangInfo(DAY_4); got != "Wednesday" {
		t.Errorf("DAY_4 = %q, want Wednesday", got)
	}
	if got := LangInfo(ABDAY_2); got != "Mon" {
		t.Errorf("ABDAY_2 = %q, want Mon", got)
	}
	if got := LangInfo(MON_4); got != "April" {
		t.Errorf("MON_4 = %q, want April", got)
	}
	if got := LangInfo(ABMON_7); got != "Jul" {
		t.Errorf("ABMON_7 = %q, want Jul", got)
	}
	if got := LangInfo(RADIXCHAR); got != "." {
		t.Errorf("RADIXCHAR = %q, want .", got)
	}
	if got := LangInfo(AM_STR); got != "AM" {
		t.Errorf("AM_STR = %q, want AM", got)
	}
}

func TestSetLocaleUnknown(t *testing.T) {
	reset(t)

	if _, ok := SetLocale(LC_ALL, "xyzzy_NOWHERE.garbage"); ok {
		t.Error("unknown locale should return ok=false")
	}
	// Nothing should have changed.
	if got := GetLocale(LC_ALL); got != "C" {
		t.Errorf("LC_ALL after failed set = %q, want C", got)
	}
}

func TestSetLocaleRoundTrip(t *testing.T) {
	reset(t)

	for _, name := range []phpv.ZString{"C", "POSIX", "en_US.UTF-8", "C.UTF-8"} {
		got, ok := SetLocale(LC_ALL, name)
		if !ok {
			t.Errorf("SetLocale(%q) returned ok=false", name)
			continue
		}
		if got != name {
			t.Errorf("SetLocale(%q) = %q, want echo of requested name", name, got)
		}
		if q := GetLocale(LC_ALL); q != name {
			t.Errorf("GetLocale after SetLocale(%q) = %q, want %q", name, q, name)
		}
	}
}

func TestUTF8Codeset(t *testing.T) {
	reset(t)

	if _, ok := SetLocale(LC_CTYPE, "en_US.UTF-8"); !ok {
		t.Fatal("en_US.UTF-8 should be supported")
	}
	if got := LangInfo(CODESET); got != "UTF-8" {
		t.Errorf("CODESET with UTF-8 locale = %q, want UTF-8", got)
	}

	if _, ok := SetLocale(LC_CTYPE, "C"); !ok {
		t.Fatal("C should be supported")
	}
	if got := LangInfo(CODESET); got != "ANSI_X3.4-1968" {
		t.Errorf("CODESET with C locale = %q, want ANSI_X3.4-1968", got)
	}
}

func TestLocaleconvCLocale(t *testing.T) {
	reset(t)

	conv := Localeconv()
	if conv.DecimalPoint != "." {
		t.Errorf("DecimalPoint = %q, want .", conv.DecimalPoint)
	}
	if conv.ThousandsSep != "" {
		t.Errorf("ThousandsSep = %q, want empty", conv.ThousandsSep)
	}
	if conv.LocalFormat.FragDigits != 127 {
		t.Errorf("FragDigits = %d, want 127 (CHAR_MAX)", conv.LocalFormat.FragDigits)
	}
}

func TestCompositeAllName(t *testing.T) {
	reset(t)

	if _, ok := SetLocale(LC_CTYPE, "en_US.UTF-8"); !ok {
		t.Fatal("SetLocale LC_CTYPE failed")
	}
	// Other categories are still "C", so LC_ALL should be the composite form.
	got := string(GetLocale(LC_ALL))
	want := "LC_CTYPE=en_US.UTF-8;LC_NUMERIC=C;LC_TIME=C;LC_COLLATE=C;LC_MONETARY=C;LC_MESSAGES=C"
	if got != want {
		t.Errorf("LC_ALL composite\n got: %s\nwant: %s", got, want)
	}
}

func TestSetLocaleFromEnv(t *testing.T) {
	reset(t)

	t.Setenv("LC_ALL", "")
	t.Setenv("LANG", "C.UTF-8")

	got, ok := SetLocale(LC_ALL, "")
	if !ok {
		t.Fatal("SetLocale with empty string should read env")
	}
	if got != "C.UTF-8" {
		t.Errorf("SetLocale from env = %q, want C.UTF-8", got)
	}
}

func TestQueryDoesNotMutate(t *testing.T) {
	reset(t)

	if _, ok := SetLocale(LC_ALL, "en_US.UTF-8"); !ok {
		t.Fatal("SetLocale failed")
	}
	before := GetLocale(LC_ALL)
	// Query again; should not change state.
	if after := GetLocale(LC_ALL); after != before {
		t.Errorf("GetLocale should be pure: first=%q, second=%q", before, after)
	}
}
