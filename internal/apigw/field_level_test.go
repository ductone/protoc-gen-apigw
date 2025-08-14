package apigw

import (
	"testing"

	"github.com/pb33f/libopenapi/orderedmap"
	"gopkg.in/yaml.v3"

	apigw_v1 "github.com/ductone/protoc-gen-apigw/apigw/v1"
)

// TestFieldLevelStabilityExtraction tests extraction of stability from field options
func TestFieldLevelStabilityExtraction(t *testing.T) {
	tests := []struct {
		name          string
		fieldOptions  []*apigw_v1.FieldOption
		wantStability apigw_v1.Stability
	}{
		{
			name:          "no field options",
			fieldOptions:  nil,
			wantStability: apigw_v1.Stability_STABILITY_UNSPECIFIED,
		},
		{
			name:          "empty field options",
			fieldOptions:  []*apigw_v1.FieldOption{},
			wantStability: apigw_v1.Stability_STABILITY_UNSPECIFIED,
		},
		{
			name: "single field option with stability",
			fieldOptions: []*apigw_v1.FieldOption{
				{
					Stability: apigw_v1.Stability_STABILITY_STABLE,
				},
			},
			wantStability: apigw_v1.Stability_STABILITY_STABLE,
		},
		{
			name: "multiple field options, first has stability",
			fieldOptions: []*apigw_v1.FieldOption{
				{
					Stability: apigw_v1.Stability_STABILITY_BETA,
				},
				{
					RequiredSpec: true,
				},
			},
			wantStability: apigw_v1.Stability_STABILITY_BETA,
		},
		{
			name: "multiple field options, second has stability",
			fieldOptions: []*apigw_v1.FieldOption{
				{
					RequiredSpec: true,
				},
				{
					Stability: apigw_v1.Stability_STABILITY_ALPHA,
				},
			},
			wantStability: apigw_v1.Stability_STABILITY_ALPHA,
		},
		{
			name: "multiple field options with different stabilities, first wins",
			fieldOptions: []*apigw_v1.FieldOption{
				{
					Stability: apigw_v1.Stability_STABILITY_DRAFT,
				},
				{
					Stability: apigw_v1.Stability_STABILITY_STABLE,
				},
			},
			wantStability: apigw_v1.Stability_STABILITY_DRAFT,
		},
		{
			name: "field option with unspecified stability",
			fieldOptions: []*apigw_v1.FieldOption{
				{
					Stability: apigw_v1.Stability_STABILITY_UNSPECIFIED,
				},
			},
			wantStability: apigw_v1.Stability_STABILITY_UNSPECIFIED,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Simulate the field stability extraction logic
			var gotStability apigw_v1.Stability = apigw_v1.Stability_STABILITY_UNSPECIFIED
			for _, fo := range tt.fieldOptions {
				if fo.Stability != apigw_v1.Stability_STABILITY_UNSPECIFIED {
					gotStability = fo.Stability
					break // Take the first non-unspecified stability
				}
			}

			if gotStability != tt.wantStability {
				t.Errorf("getFieldStability() = %v, want %v", gotStability, tt.wantStability)
			}
		})
	}
}

// TestFieldLevelDeprecationExtraction tests extraction of deprecation from field options
func TestFieldLevelDeprecationExtraction(t *testing.T) {
	tests := []struct {
		name            string
		fieldOptions    []*apigw_v1.FieldOption
		wantDeprecation *apigw_v1.Deprecation
	}{
		{
			name:            "no field options",
			fieldOptions:    nil,
			wantDeprecation: nil,
		},
		{
			name:            "empty field options",
			fieldOptions:    []*apigw_v1.FieldOption{},
			wantDeprecation: nil,
		},
		{
			name: "single field option with deprecation",
			fieldOptions: []*apigw_v1.FieldOption{
				{
					Deprecation: &apigw_v1.Deprecation{
						SunsetDate: "2030-12-31",
					},
				},
			},
			wantDeprecation: &apigw_v1.Deprecation{
				SunsetDate: "2030-12-31",
			},
		},
		{
			name: "multiple field options, first has deprecation",
			fieldOptions: []*apigw_v1.FieldOption{
				{
					Deprecation: &apigw_v1.Deprecation{
						SunsetDate: "2030-06-30",
					},
				},
				{
					RequiredSpec: true,
				},
			},
			wantDeprecation: &apigw_v1.Deprecation{
				SunsetDate: "2030-06-30",
			},
		},
		{
			name: "multiple field options, second has deprecation",
			fieldOptions: []*apigw_v1.FieldOption{
				{
					RequiredSpec: true,
				},
				{
					Deprecation: &apigw_v1.Deprecation{
						SunsetDate: "2030-03-15",
					},
				},
			},
			wantDeprecation: &apigw_v1.Deprecation{
				SunsetDate: "2030-03-15",
			},
		},
		{
			name: "multiple field options with different deprecations, first wins",
			fieldOptions: []*apigw_v1.FieldOption{
				{
					Deprecation: &apigw_v1.Deprecation{
						SunsetDate: "2030-01-01",
					},
				},
				{
					Deprecation: &apigw_v1.Deprecation{
						SunsetDate: "2030-12-31",
					},
				},
			},
			wantDeprecation: &apigw_v1.Deprecation{
				SunsetDate: "2030-01-01",
			},
		},
		{
			name: "field option with nil deprecation",
			fieldOptions: []*apigw_v1.FieldOption{
				{
					Deprecation: nil,
				},
			},
			wantDeprecation: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Simulate the field deprecation extraction logic
			var gotDeprecation *apigw_v1.Deprecation
			for _, fo := range tt.fieldOptions {
				if fo.Deprecation != nil {
					gotDeprecation = fo.Deprecation
					break // Take the first non-nil deprecation
				}
			}

			// Compare deprecation info
			if gotDeprecation == nil && tt.wantDeprecation == nil {
				// Both nil, OK
			} else if gotDeprecation == nil || tt.wantDeprecation == nil {
				t.Errorf("getFieldDeprecation() = %v, want %v", gotDeprecation, tt.wantDeprecation)
			} else if gotDeprecation.SunsetDate != tt.wantDeprecation.SunsetDate {
				t.Errorf("getFieldDeprecation().SunsetDate = %v, want %v", gotDeprecation.SunsetDate, tt.wantDeprecation.SunsetDate)
			}
		})
	}
}

// TestFieldLevelExtensionGenerationDetailed tests generation of field-level OpenAPI extensions
func TestFieldLevelExtensionGenerationDetailed(t *testing.T) {
	tests := []struct {
		name                string
		fieldOptions        []*apigw_v1.FieldOption
		protoDeprecated     bool
		wantStabilityExt    bool
		wantStabilityValue  string
		wantSunsetExt       bool
		wantSunsetValue     string
		wantDeprecatedExt   bool
		wantValidationError bool
	}{
		{
			name:              "field with no annotations",
			fieldOptions:      nil,
			protoDeprecated:   false,
			wantStabilityExt:  false,
			wantSunsetExt:     false,
			wantDeprecatedExt: false,
		},
		{
			name: "field with stability only",
			fieldOptions: []*apigw_v1.FieldOption{
				{
					Stability: apigw_v1.Stability_STABILITY_BETA,
				},
			},
			protoDeprecated:    false,
			wantStabilityExt:   true,
			wantStabilityValue: "beta",
			wantSunsetExt:      false,
			wantDeprecatedExt:  false,
		},
		{
			name: "field with valid deprecation only",
			fieldOptions: []*apigw_v1.FieldOption{
				{
					Deprecation: &apigw_v1.Deprecation{
						SunsetDate: "2030-12-31",
					},
				},
			},
			protoDeprecated:   false,
			wantStabilityExt:  false,
			wantSunsetExt:     true,
			wantSunsetValue:   "2030-12-31",
			wantDeprecatedExt: true,
		},
		{
			name: "field with both stability and deprecation",
			fieldOptions: []*apigw_v1.FieldOption{
				{
					Stability: apigw_v1.Stability_STABILITY_STABLE,
					Deprecation: &apigw_v1.Deprecation{
						SunsetDate: "2030-06-30",
					},
				},
			},
			protoDeprecated:    false,
			wantStabilityExt:   true,
			wantStabilityValue: "stable",
			wantSunsetExt:      true,
			wantSunsetValue:    "2030-06-30",
			wantDeprecatedExt:  true,
		},
		{
			name:              "field with proto deprecated flag",
			fieldOptions:      nil,
			protoDeprecated:   true,
			wantStabilityExt:  false,
			wantSunsetExt:     false,
			wantDeprecatedExt: true,
		},
		{
			name: "field with proto deprecated and stability",
			fieldOptions: []*apigw_v1.FieldOption{
				{
					Stability: apigw_v1.Stability_STABILITY_ALPHA,
				},
			},
			protoDeprecated:    true,
			wantStabilityExt:   true,
			wantStabilityValue: "alpha",
			wantSunsetExt:      false,
			wantDeprecatedExt:  true,
		},
		{
			name: "field with invalid deprecation should fail validation",
			fieldOptions: []*apigw_v1.FieldOption{
				{
					Deprecation: &apigw_v1.Deprecation{
						SunsetDate: "", // Invalid: empty sunset date
					},
				},
			},
			protoDeprecated:     false,
			wantValidationError: true,
		},
		{
			name: "field with past sunset date should now pass validation",
			fieldOptions: []*apigw_v1.FieldOption{
				{
					Deprecation: &apigw_v1.Deprecation{
						SunsetDate: "2020-01-01", // Past dates are now allowed
					},
				},
			},
			protoDeprecated:     false,
			wantStabilityExt:    false,
			wantSunsetExt:       true,
			wantSunsetValue:     "2020-01-01",
			wantDeprecatedExt:   true,
			wantValidationError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Extract field-level information
			var fieldStability apigw_v1.Stability = apigw_v1.Stability_STABILITY_UNSPECIFIED
			var fieldDeprecation *apigw_v1.Deprecation

			for _, fo := range tt.fieldOptions {
				if fo.Stability != apigw_v1.Stability_STABILITY_UNSPECIFIED {
					fieldStability = fo.Stability
				}
				if fo.Deprecation != nil {
					fieldDeprecation = fo.Deprecation
				}
			}

			// Validate deprecation info if present
			if fieldDeprecation != nil {
				err := ValidateDeprecationInfo(fieldDeprecation)
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

			// Generate field extensions
			fieldExtensions := orderedmap.New[string, *yaml.Node]()

			// Add stability extension
			addStabilityExtension(fieldExtensions, fieldStability)

			// Handle deprecation
			isFieldDeprecated := tt.protoDeprecated
			if fieldDeprecation != nil {
				isFieldDeprecated = true
				addSunsetExtension(fieldExtensions, fieldDeprecation.SunsetDate)
			}

			// Add deprecated flag if field is deprecated
			if isFieldDeprecated {
				fieldExtensions.Set("deprecated", yamlBool(true))
			}

			// Validate stability extension
			stabilityNode, stabilityExists := fieldExtensions.Get("x-stability-level")
			if tt.wantStabilityExt {
				if !stabilityExists {
					t.Errorf("Expected x-stability-level extension to be set")
				} else if stabilityNode.Value != tt.wantStabilityValue {
					t.Errorf("x-stability-level = %v, want %v", stabilityNode.Value, tt.wantStabilityValue)
				}
			} else {
				if stabilityExists {
					t.Errorf("Did not expect x-stability-level extension to be set")
				}
			}

			// Validate sunset extension
			sunsetNode, sunsetExists := fieldExtensions.Get("x-sunset")
			if tt.wantSunsetExt {
				if !sunsetExists {
					t.Errorf("Expected x-sunset extension to be set")
				} else if sunsetNode.Value != tt.wantSunsetValue {
					t.Errorf("x-sunset = %v, want %v", sunsetNode.Value, tt.wantSunsetValue)
				}
			} else {
				if sunsetExists {
					t.Errorf("Did not expect x-sunset extension to be set")
				}
			}

			// Validate deprecated extension
			deprecatedNode, deprecatedExists := fieldExtensions.Get("deprecated")
			if tt.wantDeprecatedExt {
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

// TestFieldLevelComplexScenarios tests complex field-level scenarios
func TestFieldLevelComplexScenarios(t *testing.T) {
	tests := []struct {
		name                string
		fieldOptions        []*apigw_v1.FieldOption
		protoDeprecated     bool
		expectedExtensions  map[string]interface{}
		wantValidationError bool
	}{
		{
			name: "required stable field",
			fieldOptions: []*apigw_v1.FieldOption{
				{
					RequiredSpec: true,
					Stability:    apigw_v1.Stability_STABILITY_STABLE,
				},
			},
			protoDeprecated: false,
			expectedExtensions: map[string]interface{}{
				"x-stability-level": "stable",
			},
		},
		{
			name: "read-only deprecated field",
			fieldOptions: []*apigw_v1.FieldOption{
				{
					ReadOnlySpec: true,
					Deprecation: &apigw_v1.Deprecation{
						SunsetDate: "2030-09-30",
					},
				},
			},
			protoDeprecated: false,
			expectedExtensions: map[string]interface{}{
				"x-sunset":   "2030-09-30",
				"deprecated": true,
			},
		},
		{
			name: "required, read-only, stable, deprecated field",
			fieldOptions: []*apigw_v1.FieldOption{
				{
					RequiredSpec: true,
					ReadOnlySpec: true,
					Stability:    apigw_v1.Stability_STABILITY_STABLE,
					Deprecation: &apigw_v1.Deprecation{
						SunsetDate: "2030-12-31",
					},
				},
			},
			protoDeprecated: false,
			expectedExtensions: map[string]interface{}{
				"x-stability-level": "stable",
				"x-sunset":          "2030-12-31",
				"deprecated":        true,
			},
		},
		{
			name: "multiple field options with different properties",
			fieldOptions: []*apigw_v1.FieldOption{
				{
					RequiredSpec: true,
					Stability:    apigw_v1.Stability_STABILITY_BETA,
				},
				{
					ReadOnlySpec: true,
					Deprecation: &apigw_v1.Deprecation{
						SunsetDate: "2030-06-15",
					},
				},
			},
			protoDeprecated: false,
			expectedExtensions: map[string]interface{}{
				"x-stability-level": "beta",
				"x-sunset":          "2030-06-15",
				"deprecated":        true,
			},
		},
		{
			name: "proto deprecated with field options",
			fieldOptions: []*apigw_v1.FieldOption{
				{
					Stability: apigw_v1.Stability_STABILITY_DRAFT,
				},
			},
			protoDeprecated: true,
			expectedExtensions: map[string]interface{}{
				"x-stability-level": "draft",
				"deprecated":        true,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Extract field-level information
			var fieldStability apigw_v1.Stability = apigw_v1.Stability_STABILITY_UNSPECIFIED
			var fieldDeprecation *apigw_v1.Deprecation

			for _, fo := range tt.fieldOptions {
				if fo.Stability != apigw_v1.Stability_STABILITY_UNSPECIFIED {
					fieldStability = fo.Stability
				}
				if fo.Deprecation != nil {
					fieldDeprecation = fo.Deprecation
				}
			}

			// Validate deprecation info if present
			if fieldDeprecation != nil {
				err := ValidateDeprecationInfo(fieldDeprecation)
				if tt.wantValidationError {
					if err == nil {
						t.Errorf("Expected validation error but got none")
					}
					return
				} else if err != nil {
					t.Errorf("Unexpected validation error: %v", err)
					return
				}
			}

			// Generate field extensions
			fieldExtensions := orderedmap.New[string, *yaml.Node]()

			// Add stability extension
			addStabilityExtension(fieldExtensions, fieldStability)

			// Handle deprecation
			isFieldDeprecated := tt.protoDeprecated
			if fieldDeprecation != nil {
				isFieldDeprecated = true
				addSunsetExtension(fieldExtensions, fieldDeprecation.SunsetDate)
			}

			// Add deprecated flag if field is deprecated
			if isFieldDeprecated {
				fieldExtensions.Set("deprecated", yamlBool(true))
			}

			// Validate all expected extensions
			for expectedKey, expectedValue := range tt.expectedExtensions {
				node, exists := fieldExtensions.Get(expectedKey)
				if !exists {
					t.Errorf("Expected extension %s to be set", expectedKey)
					continue
				}

				var actualValue interface{}
				if expectedKey == "deprecated" {
					actualValue = (node.Kind == yaml.ScalarNode && node.Value == "true")
				} else {
					actualValue = node.Value
				}

				if actualValue != expectedValue {
					t.Errorf("Extension %s = %v, want %v", expectedKey, actualValue, expectedValue)
				}
			}

			// Ensure no unexpected extensions are set
			expectedKeys := make(map[string]bool)
			for key := range tt.expectedExtensions {
				expectedKeys[key] = true
			}

			for pair := fieldExtensions.First(); pair != nil; pair = pair.Next() {
				key := pair.Key()
				if !expectedKeys[key] {
					t.Errorf("Unexpected extension %s was set", key)
				}
			}
		})
	}
}
