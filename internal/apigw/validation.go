package apigw

import (
	"errors"
	"fmt"
	"regexp"
	"time"

	apigw_v1 "github.com/ductone/protoc-gen-apigw/apigw/v1"
)

// Date format constants.
const (
	SunsetDateFormat = "2006-01-02" // YYYY-MM-DD format
)

// Error messages for validation.
const (
	ErrInvalidDateFormat = "sunset date must be in YYYY-MM-DD format"
	ErrMissingSunsetDate = "deprecated item must have a sunset date"
)

// dateFormatRegex validates YYYY-MM-DD format.
var dateFormatRegex = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}$`)

// ValidateSunsetDateFormat validates that the sunset date is in YYYY-MM-DD format
// and represents a valid date.
func ValidateSunsetDateFormat(date string) error {
	if date == "" {
		return fmt.Errorf("sunset date cannot be empty")
	}

	// Check basic format with regex
	if !dateFormatRegex.MatchString(date) {
		return errors.New(ErrInvalidDateFormat)
	}

	// Parse the date to ensure it's a valid date
	_, err := time.Parse(SunsetDateFormat, date)
	if err != nil {
		return fmt.Errorf("%s: %w", ErrInvalidDateFormat, err)
	}

	return nil
}

// ValidateSunsetDate validates the sunset date format only.
// Past dates are allowed and will not cause validation failures.
func ValidateSunsetDate(date string) error {
	return ValidateSunsetDateFormat(date)
}

// ValidateDeprecationInfo validates a Deprecation protobuf message.
// It validates that the sunset date is properly formatted. Past dates are allowed.
func ValidateDeprecationInfo(deprecation *apigw_v1.Deprecation) error {
	if deprecation == nil {
		return nil
	}
	if deprecation.SunsetDate == "" {
		return errors.New(ErrMissingSunsetDate)
	}
	return ValidateSunsetDate(deprecation.SunsetDate)
}
