package apigw

import (
	"strings"
	"testing"

	apigw_v1 "github.com/ductone/protoc-gen-apigw/apigw/v1"
)

func TestValidateSunsetDateFormat(t *testing.T) {
	tests := []struct {
		name    string
		date    string
		wantErr bool
		errMsg  string
	}{
		{
			name:    "valid date format",
			date:    "2024-12-31",
			wantErr: false,
		},
		{
			name:    "valid leap year date",
			date:    "2024-02-29",
			wantErr: false,
		},
		{
			name:    "empty date",
			date:    "",
			wantErr: true,
			errMsg:  "sunset date cannot be empty",
		},
		{
			name:    "invalid format - missing day",
			date:    "2024-12",
			wantErr: true,
			errMsg:  ErrInvalidDateFormat,
		},
		{
			name:    "invalid format - wrong separator",
			date:    "2024/12/31",
			wantErr: true,
			errMsg:  ErrInvalidDateFormat,
		},
		{
			name:    "invalid format - extra characters",
			date:    "2024-12-31T00:00:00Z",
			wantErr: true,
			errMsg:  ErrInvalidDateFormat,
		},
		{
			name:    "invalid format - short year",
			date:    "24-12-31",
			wantErr: true,
			errMsg:  ErrInvalidDateFormat,
		},
		{
			name:    "invalid date - February 30th",
			date:    "2024-02-30",
			wantErr: true,
			errMsg:  ErrInvalidDateFormat,
		},
		{
			name:    "invalid date - month 13",
			date:    "2024-13-01",
			wantErr: true,
			errMsg:  ErrInvalidDateFormat,
		},
		{
			name:    "invalid date - day 32",
			date:    "2024-01-32",
			wantErr: true,
			errMsg:  ErrInvalidDateFormat,
		},
		{
			name:    "invalid leap year date",
			date:    "2023-02-29",
			wantErr: true,
			errMsg:  ErrInvalidDateFormat,
		},
		{
			name:    "valid date with leading zeros",
			date:    "2024-01-01",
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateSunsetDateFormat(tt.date)
			if tt.wantErr {
				if err == nil {
					t.Errorf("ValidateSunsetDateFormat() expected error but got none")
					return
				}
				if tt.errMsg != "" && !strings.Contains(err.Error(), tt.errMsg) {
					t.Errorf("ValidateSunsetDateFormat() error = %v, want error containing %v", err, tt.errMsg)
				}
			} else if err != nil {
				t.Errorf("ValidateSunsetDateFormat() unexpected error = %v", err)
			}
		})
	}
}

func TestValidateSunsetDate(t *testing.T) {
	tests := []struct {
		name    string
		date    string
		wantErr bool
		errMsg  string
	}{
		{
			name:    "valid future date",
			date:    "2030-12-31",
			wantErr: false,
		},
		{
			name:    "invalid format",
			date:    "invalid-date",
			wantErr: true,
			errMsg:  ErrInvalidDateFormat,
		},
		{
			name:    "past date is now allowed",
			date:    "2020-01-01",
			wantErr: false,
		},
		{
			name:    "empty date",
			date:    "",
			wantErr: true,
			errMsg:  "sunset date cannot be empty",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateSunsetDate(tt.date)
			if tt.wantErr {
				if err == nil {
					t.Errorf("ValidateSunsetDate() expected error but got none")
					return
				}
				if tt.errMsg != "" && !strings.Contains(err.Error(), tt.errMsg) {
					t.Errorf("ValidateSunsetDate() error = %v, want error containing %v", err, tt.errMsg)
				}
			} else if err != nil {
				t.Errorf("ValidateSunsetDate() unexpected error = %v", err)
			}
		})
	}
}

func TestValidateDeprecationInfo(t *testing.T) {
	tests := []struct {
		name        string
		deprecation *apigw_v1.Deprecation
		wantErr     bool
		errMsg      string
	}{
		{
			name:        "nil deprecation",
			deprecation: nil,
			wantErr:     false,
		},
		{
			name: "valid sunset date",
			deprecation: &apigw_v1.Deprecation{
				SunsetDate: "2030-12-31",
			},
			wantErr: false,
		},
		{
			name: "without sunset date",
			deprecation: &apigw_v1.Deprecation{
				SunsetDate: "",
			},
			wantErr: true,
			errMsg:  ErrMissingSunsetDate,
		},
		{
			name: "invalid date format",
			deprecation: &apigw_v1.Deprecation{
				SunsetDate: "invalid-date",
			},
			wantErr: true,
			errMsg:  ErrInvalidDateFormat,
		},
		{
			name: "past date is now allowed",
			deprecation: &apigw_v1.Deprecation{
				SunsetDate: "2020-01-01",
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateDeprecationInfo(tt.deprecation)
			if tt.wantErr {
				if err == nil {
					t.Errorf("ValidateDeprecationInfo() expected error but got none")
					return
				}
				if tt.errMsg != "" && !strings.Contains(err.Error(), tt.errMsg) {
					t.Errorf("ValidateDeprecationInfo() error = %v, want error containing %v", err, tt.errMsg)
				}
			} else if err != nil {
				t.Errorf("ValidateDeprecationInfo() unexpected error = %v", err)
			}
		})
	}
}
