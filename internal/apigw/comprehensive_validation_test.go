package apigw

import (
	"strings"
	"testing"

	apigw_v1 "github.com/ductone/protoc-gen-apigw/apigw/v1"
)

// TestComprehensiveValidationScenarios tests all validation scenarios for stability and deprecation features.
func TestComprehensiveValidationScenarios(t *testing.T) {
	tests := []struct {
		name        string
		deprecation *apigw_v1.Deprecation
		wantErr     bool
		errContains string
	}{
		{
			name:        "nil deprecation should pass",
			deprecation: nil,
			wantErr:     false,
		},
		{
			name: "valid future sunset date should pass",
			deprecation: &apigw_v1.Deprecation{
				SunsetDate: "2030-12-31",
			},
			wantErr: false,
		},
		{
			name: "valid near future sunset date should pass",
			deprecation: &apigw_v1.Deprecation{
				SunsetDate: "2030-01-01",
			},
			wantErr: false,
		},
		{
			name: "empty sunset date should fail",
			deprecation: &apigw_v1.Deprecation{
				SunsetDate: "",
			},
			wantErr:     true,
			errContains: ErrMissingSunsetDate,
		},
		{
			name: "invalid date format - wrong separator should fail",
			deprecation: &apigw_v1.Deprecation{
				SunsetDate: "2025/12/31",
			},
			wantErr:     true,
			errContains: ErrInvalidDateFormat,
		},
		{
			name: "invalid date format - missing day should fail",
			deprecation: &apigw_v1.Deprecation{
				SunsetDate: "2025-12",
			},
			wantErr:     true,
			errContains: ErrInvalidDateFormat,
		},
		{
			name: "invalid date format - extra characters should fail",
			deprecation: &apigw_v1.Deprecation{
				SunsetDate: "2025-12-31T00:00:00Z",
			},
			wantErr:     true,
			errContains: ErrInvalidDateFormat,
		},
		{
			name: "invalid date - February 30th should fail",
			deprecation: &apigw_v1.Deprecation{
				SunsetDate: "2025-02-30",
			},
			wantErr:     true,
			errContains: ErrInvalidDateFormat,
		},
		{
			name: "invalid date - month 13 should fail",
			deprecation: &apigw_v1.Deprecation{
				SunsetDate: "2025-13-01",
			},
			wantErr:     true,
			errContains: ErrInvalidDateFormat,
		},
		{
			name: "invalid date - day 32 should fail",
			deprecation: &apigw_v1.Deprecation{
				SunsetDate: "2025-01-32",
			},
			wantErr:     true,
			errContains: ErrInvalidDateFormat,
		},
		{
			name: "past sunset date should now pass",
			deprecation: &apigw_v1.Deprecation{
				SunsetDate: "2020-01-01",
			},
			wantErr: false,
		},
		{
			name: "leap year date in leap year should pass",
			deprecation: &apigw_v1.Deprecation{
				SunsetDate: "2028-02-29", // 2028 is a leap year
			},
			wantErr: false,
		},
		{
			name: "leap year date in non-leap year should fail",
			deprecation: &apigw_v1.Deprecation{
				SunsetDate: "2029-02-29", // 2029 is not a leap year
			},
			wantErr:     true,
			errContains: ErrInvalidDateFormat,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateDeprecationInfo(tt.deprecation)

			if tt.wantErr {
				if err == nil {
					t.Errorf("Expected error but got none")
					return
				}
				if tt.errContains != "" && !strings.Contains(err.Error(), tt.errContains) {
					t.Errorf("Expected error containing %q but got: %v", tt.errContains, err)
				}
			} else if err != nil {
				t.Errorf("Expected no error but got: %v", err)
			}
		})
	}
}

// TestStabilityLevelValidation tests validation of stability levels.
func TestStabilityLevelValidation(t *testing.T) {
	tests := []struct {
		name      string
		stability apigw_v1.Stability
		wantValid bool
	}{
		{
			name:      "unspecified stability is valid",
			stability: apigw_v1.Stability_STABILITY_UNSPECIFIED,
			wantValid: true,
		},
		{
			name:      "draft stability is valid",
			stability: apigw_v1.Stability_STABILITY_DRAFT,
			wantValid: true,
		},
		{
			name:      "alpha stability is valid",
			stability: apigw_v1.Stability_STABILITY_ALPHA,
			wantValid: true,
		},
		{
			name:      "beta stability is valid",
			stability: apigw_v1.Stability_STABILITY_BETA,
			wantValid: true,
		},
		{
			name:      "stable stability is valid",
			stability: apigw_v1.Stability_STABILITY_STABLE,
			wantValid: true,
		},
		{
			name:      "invalid stability value",
			stability: apigw_v1.Stability(999),
			wantValid: false, // Should map to empty string
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stabilityStr := stabilityToString(tt.stability)

			if tt.wantValid {
				// Valid stability levels should map to non-empty strings (except unspecified)
				if tt.stability == apigw_v1.Stability_STABILITY_UNSPECIFIED {
					if stabilityStr != "" {
						t.Errorf("Unspecified stability should map to empty string, got %q", stabilityStr)
					}
				} else {
					if stabilityStr == "" {
						t.Errorf("Valid stability %v should map to non-empty string", tt.stability)
					}
				}
			} else {
				// Invalid stability levels should map to empty string
				if stabilityStr != "" {
					t.Errorf("Invalid stability %v should map to empty string, got %q", tt.stability, stabilityStr)
				}
			}
		})
	}
}

// TestComplexValidationScenarios tests complex scenarios combining multiple validation rules.
func TestComplexValidationScenarios(t *testing.T) {
	tests := []struct {
		name                 string
		serviceStability     apigw_v1.Stability
		serviceDeprecation   *apigw_v1.Deprecation
		operationStability   apigw_v1.Stability
		operationDeprecation *apigw_v1.Deprecation
		fieldStability       apigw_v1.Stability
		fieldDeprecation     *apigw_v1.Deprecation
		wantServiceErr       bool
		wantOperationErr     bool
		wantFieldErr         bool
		serviceErrContains   string
		operationErrContains string
		fieldErrContains     string
	}{
		{
			name:             "all valid should pass",
			serviceStability: apigw_v1.Stability_STABILITY_STABLE,
			serviceDeprecation: &apigw_v1.Deprecation{
				SunsetDate: "2025-12-31",
			},
			operationStability: apigw_v1.Stability_STABILITY_BETA,
			operationDeprecation: &apigw_v1.Deprecation{
				SunsetDate: "2025-06-30",
			},
			fieldStability: apigw_v1.Stability_STABILITY_ALPHA,
			fieldDeprecation: &apigw_v1.Deprecation{
				SunsetDate: "2025-03-15",
			},
			wantServiceErr:   false,
			wantOperationErr: false,
			wantFieldErr:     false,
		},
		{
			name:             "service invalid, others valid",
			serviceStability: apigw_v1.Stability_STABILITY_STABLE,
			serviceDeprecation: &apigw_v1.Deprecation{
				SunsetDate: "", // Invalid: empty sunset date
			},
			operationStability: apigw_v1.Stability_STABILITY_BETA,
			operationDeprecation: &apigw_v1.Deprecation{
				SunsetDate: "2025-06-30",
			},
			fieldStability:     apigw_v1.Stability_STABILITY_ALPHA,
			wantServiceErr:     true,
			wantOperationErr:   false,
			wantFieldErr:       false,
			serviceErrContains: ErrMissingSunsetDate,
		},
		{
			name:               "operation invalid, others valid",
			serviceStability:   apigw_v1.Stability_STABILITY_STABLE,
			operationStability: apigw_v1.Stability_STABILITY_BETA,
			operationDeprecation: &apigw_v1.Deprecation{
				SunsetDate: "invalid-date", // Invalid: bad format
			},
			fieldStability:       apigw_v1.Stability_STABILITY_ALPHA,
			wantServiceErr:       false,
			wantOperationErr:     true,
			wantFieldErr:         false,
			operationErrContains: ErrInvalidDateFormat,
		},
		{
			name:               "field invalid, others valid",
			serviceStability:   apigw_v1.Stability_STABILITY_STABLE,
			operationStability: apigw_v1.Stability_STABILITY_BETA,
			fieldStability:     apigw_v1.Stability_STABILITY_ALPHA,
			fieldDeprecation: &apigw_v1.Deprecation{
				SunsetDate: "invalid-date", // Invalid: bad format
			},
			wantServiceErr:   false,
			wantOperationErr: false,
			wantFieldErr:     true,
			fieldErrContains: ErrInvalidDateFormat,
		},
		{
			name:             "multiple invalid",
			serviceStability: apigw_v1.Stability_STABILITY_STABLE,
			serviceDeprecation: &apigw_v1.Deprecation{
				SunsetDate: "", // Invalid: empty
			},
			operationStability: apigw_v1.Stability_STABILITY_BETA,
			operationDeprecation: &apigw_v1.Deprecation{
				SunsetDate: "", // Invalid: empty
			},
			fieldStability: apigw_v1.Stability_STABILITY_ALPHA,
			fieldDeprecation: &apigw_v1.Deprecation{
				SunsetDate: "invalid", // Invalid: format
			},
			wantServiceErr:       true,
			wantOperationErr:     true,
			wantFieldErr:         true,
			serviceErrContains:   ErrMissingSunsetDate,
			operationErrContains: ErrMissingSunsetDate,
			fieldErrContains:     ErrInvalidDateFormat,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test service validation
			serviceErr := ValidateDeprecationInfo(tt.serviceDeprecation)
			if tt.wantServiceErr {
				if serviceErr == nil {
					t.Errorf("Expected service validation error but got none")
				} else if tt.serviceErrContains != "" && !strings.Contains(serviceErr.Error(), tt.serviceErrContains) {
					t.Errorf("Expected service error containing %q but got: %v", tt.serviceErrContains, serviceErr)
				}
			} else {
				if serviceErr != nil {
					t.Errorf("Expected no service validation error but got: %v", serviceErr)
				}
			}

			// Test operation validation
			operationErr := ValidateDeprecationInfo(tt.operationDeprecation)
			if tt.wantOperationErr {
				if operationErr == nil {
					t.Errorf("Expected operation validation error but got none")
				} else if tt.operationErrContains != "" && !strings.Contains(operationErr.Error(), tt.operationErrContains) {
					t.Errorf("Expected operation error containing %q but got: %v", tt.operationErrContains, operationErr)
				}
			} else {
				if operationErr != nil {
					t.Errorf("Expected no operation validation error but got: %v", operationErr)
				}
			}

			// Test field validation
			fieldErr := ValidateDeprecationInfo(tt.fieldDeprecation)
			if tt.wantFieldErr {
				if fieldErr == nil {
					t.Errorf("Expected field validation error but got none")
				} else if tt.fieldErrContains != "" && !strings.Contains(fieldErr.Error(), tt.fieldErrContains) {
					t.Errorf("Expected field error containing %q but got: %v", tt.fieldErrContains, fieldErr)
				}
			} else {
				if fieldErr != nil {
					t.Errorf("Expected no field validation error but got: %v", fieldErr)
				}
			}
		})
	}
}

// TestBoundaryDateValidation tests boundary conditions for date validation.
func TestBoundaryDateValidation(t *testing.T) {
	tests := []struct {
		name    string
		date    string
		wantErr bool
		errMsg  string
	}{
		{
			name:    "minimum valid date",
			date:    "0001-01-01",
			wantErr: false, // Past dates are now allowed
		},
		{
			name:    "maximum valid date",
			date:    "9999-12-31",
			wantErr: false,
		},
		{
			name:    "year 2000 leap day",
			date:    "2000-02-29",
			wantErr: false, // Past dates are now allowed
		},
		{
			name:    "year 1900 not leap day",
			date:    "1900-02-29",
			wantErr: true,
			errMsg:  ErrInvalidDateFormat, // 1900 is not a leap year
		},
		{
			name:    "future leap day",
			date:    "2028-02-29",
			wantErr: false, // 2028 is a leap year
		},
		{
			name:    "future non-leap day",
			date:    "2029-02-29",
			wantErr: true,
			errMsg:  ErrInvalidDateFormat, // 2029 is not a leap year
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateSunsetDate(tt.date)

			if tt.wantErr {
				if err == nil {
					t.Errorf("Expected error but got none")
					return
				}
				if tt.errMsg != "" && !strings.Contains(err.Error(), tt.errMsg) {
					t.Errorf("Expected error containing %q but got: %v", tt.errMsg, err)
				}
			} else if err != nil {
				t.Errorf("Expected no error but got: %v", err)
			}
		})
	}
}
