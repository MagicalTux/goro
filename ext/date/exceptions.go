package date

import (
	"github.com/MagicalTux/goro/core/phpobj"
)

// Date exception hierarchy (PHP 8.3+)

// > class DateException extends Exception
var DateException *phpobj.ZClass

// > class DateObjectError extends Error
var DateObjectError *phpobj.ZClass

// > class DateRangeError extends DateError (extends Error)
var DateError *phpobj.ZClass
var DateRangeError *phpobj.ZClass

// > class DateMalformedStringException extends DateException
var DateMalformedStringException *phpobj.ZClass

// > class DateMalformedIntervalStringException extends DateException
var DateMalformedIntervalStringException *phpobj.ZClass

// > class DateMalformedPeriodStringException extends DateException
var DateMalformedPeriodStringException *phpobj.ZClass

// > class DateInvalidTimeZoneException extends DateException
var DateInvalidTimeZoneException *phpobj.ZClass

// > class DateInvalidOperationException extends DateException
var DateInvalidOperationException *phpobj.ZClass

func init() {
	DateException = &phpobj.ZClass{
		Name:    "DateException",
		Extends: phpobj.Exception,
		Props:   phpobj.Exception.Props,
		Methods: phpobj.CopyMethods(phpobj.Exception.Methods),
	}

	DateObjectError = &phpobj.ZClass{
		Name:    "DateObjectError",
		Extends: phpobj.Error,
		Props:   phpobj.Error.Props,
		Methods: phpobj.CopyMethods(phpobj.Error.Methods),
	}

	DateError = &phpobj.ZClass{
		Name:    "DateError",
		Extends: phpobj.Error,
		Props:   phpobj.Error.Props,
		Methods: phpobj.CopyMethods(phpobj.Error.Methods),
	}

	DateRangeError = &phpobj.ZClass{
		Name:    "DateRangeError",
		Extends: DateError,
		Props:   DateError.Props,
		Methods: phpobj.CopyMethods(DateError.Methods),
	}

	DateMalformedStringException = &phpobj.ZClass{
		Name:    "DateMalformedStringException",
		Extends: DateException,
		Props:   DateException.Props,
		Methods: phpobj.CopyMethods(DateException.Methods),
	}

	DateMalformedIntervalStringException = &phpobj.ZClass{
		Name:    "DateMalformedIntervalStringException",
		Extends: DateException,
		Props:   DateException.Props,
		Methods: phpobj.CopyMethods(DateException.Methods),
	}

	DateMalformedPeriodStringException = &phpobj.ZClass{
		Name:    "DateMalformedPeriodStringException",
		Extends: DateException,
		Props:   DateException.Props,
		Methods: phpobj.CopyMethods(DateException.Methods),
	}

	DateInvalidTimeZoneException = &phpobj.ZClass{
		Name:    "DateInvalidTimeZoneException",
		Extends: DateException,
		Props:   DateException.Props,
		Methods: phpobj.CopyMethods(DateException.Methods),
	}

	DateInvalidOperationException = &phpobj.ZClass{
		Name:    "DateInvalidOperationException",
		Extends: DateException,
		Props:   DateException.Props,
		Methods: phpobj.CopyMethods(DateException.Methods),
	}

	// Assign to package-level phpv vars for ThrowError usage
	_ = DateException
}

func dateExceptionClasses() []*phpobj.ZClass {
	return []*phpobj.ZClass{
		DateException,
		DateObjectError,
		DateError,
		DateRangeError,
		DateMalformedStringException,
		DateMalformedIntervalStringException,
		DateMalformedPeriodStringException,
		DateInvalidTimeZoneException,
		DateInvalidOperationException,
	}
}
