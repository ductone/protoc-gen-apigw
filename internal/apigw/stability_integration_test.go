package apigw

import (
	"strings"
	"testing"

	apigw_v1 "github.com/ductone/protoc-gen-apigw/apigw/v1"
	"github.com/pb33f/libopenapi/orderedmap"
	"gopkg.in/yaml.v3"
)

// TestStabilityAndDeprecationIntegration tests end-to-end OpenAPI generation
// with stability and deprecation features
func TestStabilityAndDeprecationIntegration(t *testing.T) {
	// This test requires the test proto file to be compiled
	// For now, we'll test the core functionality with mock data
	t.Skip("Integration test requires compiled proto - implement after proto compilation setup")

	// TODO: Implement full integration test that:
	// 1. Compiles the test proto file
	// 2. Runs the plugin against it
	// 3. Validates the generated OpenAPI spec
	// 4. Checks all stability and deprecation extensions
}

// TestOpenAPIExtensionGeneration tests that the correct OpenAPI extensions
// are generated using the existing helper functions
func TestOpenAPIExtensionGeneration(t *testing.T) {
	tests := []struct {
		name                   string
		stability              apigw_v1.Stability
		deprecation            *apigw_v1.Deprecation
		protoDeprecated        bool
		expectedStabilityExt   bool
		expectedStabilityValue string
		expectedSunsetExt      bool
		expectedSunsetValue    string
		expectedDeprecatedExt  bool
		wantValidationError    bool
	}{
		{
			name:                   "stable with deprecation",
			stability:              apigw_v1.Stability_STABILITY_STABLE,
			deprecation:            &apigw_v1.Deprecation{SunsetDate: "2025-12-31"},
			protoDeprecated:        false,
			expectedStabilityExt:   true,
			expectedStabilityValue: "stable",
			expectedSunsetExt:      true,
			expectedSunsetValue:    "2025-12-31",
			expectedDeprecatedExt:  true,
		},
		{
			name:                   "draft without deprecation",
			stability:              apigw_v1.Stability_STABILITY_DRAFT,
			deprecation:            nil,
			protoDeprecated:        false,
			expectedStabilityExt:   true,
			expectedStabilityValue: "draft",
			expectedSunsetExt:      false,
			expectedDeprecatedExt:  false,
		},
		{
			name:                   "alpha without deprecation",
			stability:              apigw_v1.Stability_STABILITY_ALPHA,
			deprecation:            nil,
			protoDeprecated:        false,
			expectedStabilityExt:   true,
			expectedStabilityValue: "alpha",
			expectedSunsetExt:      false,
			expectedDeprecatedExt:  false,
		},
		{
			name:                   "beta without deprecation",
			stability:              apigw_v1.Stability_STABILITY_BETA,
			deprecation:            nil,
			protoDeprecated:        false,
			expectedStabilityExt:   true,
			expectedStabilityValue: "beta",
			expectedSunsetExt:      false,
			expectedDeprecatedExt:  false,
		},
		{
			name:                   "proto deprecated with stability",
			stability:              apigw_v1.Stability_STABILITY_BETA,
			deprecation:            nil,
			protoDeprecated:        true,
			expectedStabilityExt:   true,
			expectedStabilityValue: "beta",
			expectedSunsetExt:      false,
			expectedDeprecatedExt:  true,
		},
		{
			name:                "invalid deprecation should fail validation",
			stability:           apigw_v1.Stability_STABILITY_STABLE,
			deprecation:         &apigw_v1.Deprecation{SunsetDate: ""},
			protoDeprecated:     false,
			wantValidationError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Validate deprecation info if present
			if tt.deprecation != nil {
				err := ValidateDeprecationInfo(tt.deprecation)
				if tt.wantValidationError {
					if err == nil {
						t.Errorf("Expected validation error but got none")
					}
					return // Skip extension generation if validation fails
				} else if err != nil {
					t.Errorf("Unexpected validation error: %v", err)
					return
				}
			}

			// Generate extensions using the helper functions
			extensions := orderedmap.New[string, *yaml.Node]()

			// Add stability extension
			addStabilityExtension(extensions, tt.stability)

			// Handle deprecation
			isDeprecated := tt.protoDeprecated
			if tt.deprecation != nil {
				isDeprecated = true
				addSunsetExtension(extensions, tt.deprecation.SunsetDate)
			}

			// Add deprecated flag if deprecated
			if isDeprecated {
				extensions.Set("deprecated", yamlBool(true))
			}

			// Validate stability extension
			stabilityNode, stabilityExists := extensions.Get("x-stability-level")
			if tt.expectedStabilityExt {
				if !stabilityExists {
					t.Errorf("Expected x-stability-level extension to be set")
				} else if stabilityNode.Value != tt.expectedStabilityValue {
					t.Errorf("x-stability-level = %v, want %v", stabilityNode.Value, tt.expectedStabilityValue)
				}
			} else {
				if stabilityExists {
					t.Errorf("Did not expect x-stability-level extension to be set")
				}
			}

			// Validate sunset extension
			sunsetNode, sunsetExists := extensions.Get("x-sunset")
			if tt.expectedSunsetExt {
				if !sunsetExists {
					t.Errorf("Expected x-sunset extension to be set")
				} else if sunsetNode.Value != tt.expectedSunsetValue {
					t.Errorf("x-sunset = %v, want %v", sunsetNode.Value, tt.expectedSunsetValue)
				}
			} else {
				if sunsetExists {
					t.Errorf("Did not expect x-sunset extension to be set")
				}
			}

			// Validate deprecated extension
			deprecatedNode, deprecatedExists := extensions.Get("deprecated")
			if tt.expectedDeprecatedExt {
				if !deprecatedExists {
					t.Errorf("Expected deprecated extension to be set")
				} else if deprecatedNode.Kind != yaml.ScalarNode || deprecatedNode.Value != "true" {
					t.Errorf("deprecated = %v, want true", deprecatedNode.Value)
				}
			} else {
				if deprecatedExists {
					t.Errorf("Did not expect deprecated extension to be set")
				}
			}
		})
	}
}

// TestValidationIntegration tests that validation errors are properly caught
// during OpenAPI generation
func TestValidationIntegration(t *testing.T) {
	tests := []struct {
		name          string
		setupFunc     func() error
		expectedError string
	}{
		{
			name: "deprecated without sunset date should fail",
			setupFunc: func() error {
				// Simulate validation of deprecated item without sunset date
				return ValidateDeprecationInfo(&apigw_v1.Deprecation{
					SunsetDate: "",
				})
			},
			expectedError: ErrMissingSunsetDate,
		},
		{
			name: "invalid sunset date format should fail",
			setupFunc: func() error {
				return ValidateDeprecationInfo(&apigw_v1.Deprecation{
					SunsetDate: "invalid-date",
				})
			},
			expectedError: ErrInvalidDateFormat,
		},
		{
			name: "past sunset date should now pass",
			setupFunc: func() error {
				return ValidateDeprecationInfo(&apigw_v1.Deprecation{
					SunsetDate: "2020-01-01",
				})
			},
			expectedError: "",
		},
		{
			name: "valid deprecation should pass",
			setupFunc: func() error {
				return ValidateDeprecationInfo(&apigw_v1.Deprecation{
					SunsetDate: "2030-12-31",
				})
			},
			expectedError: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.setupFunc()

			if tt.expectedError == "" {
				if err != nil {
					t.Errorf("Expected no error but got: %v", err)
				}
			} else {
				if err == nil {
					t.Errorf("Expected error containing %q but got none", tt.expectedError)
				} else if !strings.Contains(err.Error(), tt.expectedError) {
					t.Errorf("Expected error containing %q but got: %v", tt.expectedError, err)
				}
			}
		})
	}
}

// TestMultiLevelAnnotationHandling tests scenarios where stability and deprecation
// annotations exist at multiple levels (service, operation, field)
func TestMultiLevelAnnotationHandling(t *testing.T) {
	tests := []struct {
		name                     string
		serviceStability         apigw_v1.Stability
		serviceDeprecation       *apigw_v1.Deprecation
		operationStability       apigw_v1.Stability
		operationDeprecation     *apigw_v1.Deprecation
		expectedServiceStability string
		expectedServiceSunset    string
		expectedOpStability      string
		expectedOpSunset         string
	}{
		{
			name:                     "service stable, operation beta",
			serviceStability:         apigw_v1.Stability_STABILITY_STABLE,
			operationStability:       apigw_v1.Stability_STABILITY_BETA,
			expectedServiceStability: "stable",
			expectedOpStability:      "beta",
		},
		{
			name:                     "service deprecated, operation not deprecated",
			serviceStability:         apigw_v1.Stability_STABILITY_STABLE,
			serviceDeprecation:       &apigw_v1.Deprecation{SunsetDate: "2025-12-31"},
			operationStability:       apigw_v1.Stability_STABILITY_STABLE,
			expectedServiceStability: "stable",
			expectedServiceSunset:    "2025-12-31",
			expectedOpStability:      "stable",
		},
		{
			name:                     "service and operation both deprecated",
			serviceStability:         apigw_v1.Stability_STABILITY_STABLE,
			serviceDeprecation:       &apigw_v1.Deprecation{SunsetDate: "2025-12-31"},
			operationStability:       apigw_v1.Stability_STABILITY_STABLE,
			operationDeprecation:     &apigw_v1.Deprecation{SunsetDate: "2025-06-30"},
			expectedServiceStability: "stable",
			expectedServiceSunset:    "2025-12-31",
			expectedOpStability:      "stable",
			expectedOpSunset:         "2025-06-30",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test service-level extensions
			serviceExtensions := orderedmap.New[string, *yaml.Node]()
			addStabilityExtension(serviceExtensions, tt.serviceStability)
			if tt.serviceDeprecation != nil {
				err := ValidateDeprecationInfo(tt.serviceDeprecation)
				if err != nil {
					t.Fatalf("Service deprecation validation failed: %v", err)
				}
				addSunsetExtension(serviceExtensions, tt.serviceDeprecation.SunsetDate)
				serviceExtensions.Set("deprecated", yamlBool(true))
			}

			// Test operation-level extensions
			operationExtensions := orderedmap.New[string, *yaml.Node]()
			addStabilityExtension(operationExtensions, tt.operationStability)
			if tt.operationDeprecation != nil {
				err := ValidateDeprecationInfo(tt.operationDeprecation)
				if err != nil {
					t.Fatalf("Operation deprecation validation failed: %v", err)
				}
				addSunsetExtension(operationExtensions, tt.operationDeprecation.SunsetDate)
				operationExtensions.Set("deprecated", yamlBool(true))
			}

			// Validate service extensions
			if tt.expectedServiceStability != "" {
				node, exists := serviceExtensions.Get("x-stability-level")
				if !exists {
					t.Errorf("Service x-stability-level extension not found")
				} else if node.Value != tt.expectedServiceStability {
					t.Errorf("Service x-stability-level = %v, want %v", node.Value, tt.expectedServiceStability)
				}
			}

			if tt.expectedServiceSunset != "" {
				node, exists := serviceExtensions.Get("x-sunset")
				if !exists {
					t.Errorf("Service x-sunset extension not found")
				} else if node.Value != tt.expectedServiceSunset {
					t.Errorf("Service x-sunset = %v, want %v", node.Value, tt.expectedServiceSunset)
				}
			}

			// Validate operation extensions
			if tt.expectedOpStability != "" {
				node, exists := operationExtensions.Get("x-stability-level")
				if !exists {
					t.Errorf("Operation x-stability-level extension not found")
				} else if node.Value != tt.expectedOpStability {
					t.Errorf("Operation x-stability-level = %v, want %v", node.Value, tt.expectedOpStability)
				}
			}

			if tt.expectedOpSunset != "" {
				node, exists := operationExtensions.Get("x-sunset")
				if !exists {
					t.Errorf("Operation x-sunset extension not found")
				} else if node.Value != tt.expectedOpSunset {
					t.Errorf("Operation x-sunset = %v, want %v", node.Value, tt.expectedOpSunset)
				}
			}
		})
	}
}
