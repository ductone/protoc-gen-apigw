package apigw

import (
	"testing"

	"github.com/pb33f/libopenapi/orderedmap"
	"gopkg.in/yaml.v3"

	apigw_v1 "github.com/ductone/protoc-gen-apigw/apigw/v1"
)

func Test_toSnakeCase(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{
			input: "foo",
			want:  "foo",
		},
		{
			input: "foo.bar",
			want:  "foo_bar",
		},
		{
			input: "foo.bar.baz",
			want:  "foo_bar_baz",
		},
		{
			input: "foo_bar.baz",
			want:  "foo_bar_baz",
		},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			if got := toSnakeCase(tt.input); got != tt.want {
				t.Errorf("toSnakeCase() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_stabilityToString(t *testing.T) {
	tests := []struct {
		name      string
		stability apigw_v1.Stability
		want      string
	}{
		{
			name:      "draft stability",
			stability: apigw_v1.Stability_STABILITY_DRAFT,
			want:      "draft",
		},
		{
			name:      "alpha stability",
			stability: apigw_v1.Stability_STABILITY_ALPHA,
			want:      "alpha",
		},
		{
			name:      "beta stability",
			stability: apigw_v1.Stability_STABILITY_BETA,
			want:      "beta",
		},
		{
			name:      "stable stability",
			stability: apigw_v1.Stability_STABILITY_STABLE,
			want:      "stable",
		},
		{
			name:      "unspecified stability",
			stability: apigw_v1.Stability_STABILITY_UNSPECIFIED,
			want:      "",
		},
		{
			name:      "invalid stability value",
			stability: apigw_v1.Stability(999),
			want:      "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := stabilityToString(tt.stability); got != tt.want {
				t.Errorf("stabilityToString() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_addStabilityExtension(t *testing.T) {
	tests := []struct {
		name      string
		stability apigw_v1.Stability
		wantKey   string
		wantValue string
		wantSet   bool
	}{
		{
			name:      "draft stability",
			stability: apigw_v1.Stability_STABILITY_DRAFT,
			wantKey:   "x-stability-level",
			wantValue: "draft",
			wantSet:   true,
		},
		{
			name:      "alpha stability",
			stability: apigw_v1.Stability_STABILITY_ALPHA,
			wantKey:   "x-stability-level",
			wantValue: "alpha",
			wantSet:   true,
		},
		{
			name:      "beta stability",
			stability: apigw_v1.Stability_STABILITY_BETA,
			wantKey:   "x-stability-level",
			wantValue: "beta",
			wantSet:   true,
		},
		{
			name:      "stable stability",
			stability: apigw_v1.Stability_STABILITY_STABLE,
			wantKey:   "x-stability-level",
			wantValue: "stable",
			wantSet:   true,
		},
		{
			name:      "unspecified stability should not set extension",
			stability: apigw_v1.Stability_STABILITY_UNSPECIFIED,
			wantKey:   "x-stability-level",
			wantSet:   false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			extensions := orderedmap.New[string, *yaml.Node]()
			addStabilityExtension(extensions, tt.stability)

			node, exists := extensions.Get(tt.wantKey)
			if tt.wantSet {
				if !exists {
					t.Errorf("addStabilityExtension() did not set %s extension", tt.wantKey)
					return
				}
				if node.Value != tt.wantValue {
					t.Errorf("addStabilityExtension() set %s = %v, want %v", tt.wantKey, node.Value, tt.wantValue)
				}
			} else {
				if exists {
					t.Errorf("addStabilityExtension() should not set %s extension for unspecified stability", tt.wantKey)
				}
			}
		})
	}
}

func Test_addSunsetExtension(t *testing.T) {
	tests := []struct {
		name       string
		sunsetDate string
		wantKey    string
		wantValue  string
		wantSet    bool
	}{
		{
			name:       "valid sunset date",
			sunsetDate: "2024-12-31",
			wantKey:    "x-sunset",
			wantValue:  "2024-12-31",
			wantSet:    true,
		},
		{
			name:       "another valid sunset date",
			sunsetDate: "2025-06-15",
			wantKey:    "x-sunset",
			wantValue:  "2025-06-15",
			wantSet:    true,
		},
		{
			name:       "empty sunset date should not set extension",
			sunsetDate: "",
			wantKey:    "x-sunset",
			wantSet:    false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			extensions := orderedmap.New[string, *yaml.Node]()
			addSunsetExtension(extensions, tt.sunsetDate)

			node, exists := extensions.Get(tt.wantKey)
			if tt.wantSet {
				if !exists {
					t.Errorf("addSunsetExtension() did not set %s extension", tt.wantKey)
					return
				}
				if node.Value != tt.wantValue {
					t.Errorf("addSunsetExtension() set %s = %v, want %v", tt.wantKey, node.Value, tt.wantValue)
				}
			} else {
				if exists {
					t.Errorf("addSunsetExtension() should not set %s extension for empty sunset date", tt.wantKey)
				}
			}
		})
	}
}

// Test that service-level extensions are properly handled
func Test_serviceExtensionHandling(t *testing.T) {
	tests := []struct {
		name               string
		serviceStability   apigw_v1.Stability
		serviceDeprecation *apigw_v1.Deprecation
		protoDeprecated    bool
		wantStabilityKey   string
		wantStabilityValue string
		wantStabilitySet   bool
		wantSunsetKey      string
		wantSunsetValue    string
		wantSunsetSet      bool
		wantDeprecatedKey  string
		wantDeprecatedSet  bool
		wantError          bool
	}{
		{
			name:               "stable service without deprecation",
			serviceStability:   apigw_v1.Stability_STABILITY_STABLE,
			serviceDeprecation: nil,
			protoDeprecated:    false,
			wantStabilityKey:   "x-stability-level",
			wantStabilityValue: "stable",
			wantStabilitySet:   true,
			wantSunsetSet:      false,
			wantDeprecatedSet:  false,
			wantError:          false,
		},
		{
			name:             "deprecated service with sunset date",
			serviceStability: apigw_v1.Stability_STABILITY_STABLE,
			serviceDeprecation: &apigw_v1.Deprecation{
				SunsetDate: "2025-12-31",
			},
			protoDeprecated:    false,
			wantStabilityKey:   "x-stability-level",
			wantStabilityValue: "stable",
			wantStabilitySet:   true,
			wantSunsetKey:      "x-sunset",
			wantSunsetValue:    "2025-12-31",
			wantSunsetSet:      true,
			wantDeprecatedKey:  "deprecated",
			wantDeprecatedSet:  true,
			wantError:          false,
		},
		{
			name:               "service with proto deprecated flag",
			serviceStability:   apigw_v1.Stability_STABILITY_BETA,
			serviceDeprecation: nil,
			protoDeprecated:    true,
			wantStabilityKey:   "x-stability-level",
			wantStabilityValue: "beta",
			wantStabilitySet:   true,
			wantSunsetSet:      false,
			wantDeprecatedKey:  "deprecated",
			wantDeprecatedSet:  true,
			wantError:          false,
		},
		{
			name:             "deprecated service without sunset date should fail validation",
			serviceStability: apigw_v1.Stability_STABILITY_ALPHA,
			serviceDeprecation: &apigw_v1.Deprecation{
				SunsetDate: "",
			},
			protoDeprecated: false,
			wantError:       true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create service extensions like the buildOpenAPIService function does
			serviceExtensions := orderedmap.New[string, *yaml.Node]()

			// Add stability extension
			addStabilityExtension(serviceExtensions, tt.serviceStability)

			// Handle deprecation
			isServiceDeprecated := tt.protoDeprecated
			var validationErr error
			if tt.serviceDeprecation != nil {
				// Validate deprecation info
				validationErr = ValidateDeprecationInfo(tt.serviceDeprecation)
				if validationErr == nil {
					// If deprecation info is provided, the service is considered deprecated
					isServiceDeprecated = true

					// Add sunset extension
					addSunsetExtension(serviceExtensions, tt.serviceDeprecation.SunsetDate)
				}
			}

			// Add deprecated flag to service extensions if deprecated
			if isServiceDeprecated {
				serviceExtensions.Set("deprecated", yamlBool(true))
			}

			// Check for expected validation error
			if tt.wantError {
				if validationErr == nil {
					t.Errorf("Expected validation error but got none")
				}
				return
			}

			if validationErr != nil {
				t.Errorf("Unexpected validation error: %v", validationErr)
				return
			}

			// Check stability extension
			if tt.wantStabilitySet {
				node, exists := serviceExtensions.Get(tt.wantStabilityKey)
				if !exists {
					t.Errorf("Expected %s extension to be set", tt.wantStabilityKey)
					return
				}
				if node.Value != tt.wantStabilityValue {
					t.Errorf("Expected %s = %v, got %v", tt.wantStabilityKey, tt.wantStabilityValue, node.Value)
				}
			}

			// Check sunset extension
			if tt.wantSunsetSet {
				node, exists := serviceExtensions.Get(tt.wantSunsetKey)
				if !exists {
					t.Errorf("Expected %s extension to be set", tt.wantSunsetKey)
					return
				}
				if node.Value != tt.wantSunsetValue {
					t.Errorf("Expected %s = %v, got %v", tt.wantSunsetKey, tt.wantSunsetValue, node.Value)
				}
			} else {
				if _, exists := serviceExtensions.Get("x-sunset"); exists {
					t.Errorf("Did not expect x-sunset extension to be set")
				}
			}

			// Check deprecated extension
			if tt.wantDeprecatedSet {
				node, exists := serviceExtensions.Get(tt.wantDeprecatedKey)
				if !exists {
					t.Errorf("Expected %s extension to be set", tt.wantDeprecatedKey)
					return
				}
				if node.Kind != yaml.ScalarNode || node.Value != "true" {
					t.Errorf("Expected %s = true, got %v", tt.wantDeprecatedKey, node.Value)
				}
			} else {
				if _, exists := serviceExtensions.Get("deprecated"); exists {
					t.Errorf("Did not expect deprecated extension to be set")
				}
			}
		})
	}
}

// Test field-level stability and deprecation functionality
func Test_fieldLevelStabilityAndDeprecation(t *testing.T) {
	tests := []struct {
		name               string
		fieldOptions       []*apigw_v1.FieldOption
		wantStability      apigw_v1.Stability
		wantDeprecation    *apigw_v1.Deprecation
		wantStabilityExt   bool
		wantStabilityValue string
		wantSunsetExt      bool
		wantSunsetValue    string
	}{
		{
			name:             "field with no options",
			fieldOptions:     nil,
			wantStability:    apigw_v1.Stability_STABILITY_UNSPECIFIED,
			wantDeprecation:  nil,
			wantStabilityExt: false,
			wantSunsetExt:    false,
		},
		{
			name: "field with stability only",
			fieldOptions: []*apigw_v1.FieldOption{
				{
					Stability: apigw_v1.Stability_STABILITY_BETA,
				},
			},
			wantStability:      apigw_v1.Stability_STABILITY_BETA,
			wantDeprecation:    nil,
			wantStabilityExt:   true,
			wantStabilityValue: "beta",
			wantSunsetExt:      false,
		},
		{
			name: "field with deprecation only",
			fieldOptions: []*apigw_v1.FieldOption{
				{
					Deprecation: &apigw_v1.Deprecation{
						SunsetDate: "2025-12-31",
					},
				},
			},
			wantStability:    apigw_v1.Stability_STABILITY_UNSPECIFIED,
			wantDeprecation:  &apigw_v1.Deprecation{SunsetDate: "2025-12-31"},
			wantStabilityExt: false,
			wantSunsetExt:    true,
			wantSunsetValue:  "2025-12-31",
		},
		{
			name: "field with both stability and deprecation",
			fieldOptions: []*apigw_v1.FieldOption{
				{
					Stability: apigw_v1.Stability_STABILITY_STABLE,
					Deprecation: &apigw_v1.Deprecation{
						SunsetDate: "2024-06-30",
					},
				},
			},
			wantStability:      apigw_v1.Stability_STABILITY_STABLE,
			wantDeprecation:    &apigw_v1.Deprecation{SunsetDate: "2024-06-30"},
			wantStabilityExt:   true,
			wantStabilityValue: "stable",
			wantSunsetExt:      true,
			wantSunsetValue:    "2024-06-30",
		},
		{
			name: "field with multiple options, first has stability and deprecation",
			fieldOptions: []*apigw_v1.FieldOption{
				{
					Stability: apigw_v1.Stability_STABILITY_ALPHA,
					Deprecation: &apigw_v1.Deprecation{
						SunsetDate: "2025-03-15",
					},
				},
				{
					RequiredSpec: true,
				},
			},
			wantStability:      apigw_v1.Stability_STABILITY_ALPHA,
			wantDeprecation:    &apigw_v1.Deprecation{SunsetDate: "2025-03-15"},
			wantStabilityExt:   true,
			wantStabilityValue: "alpha",
			wantSunsetExt:      true,
			wantSunsetValue:    "2025-03-15",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test the helper functions directly with field options
			var gotStability apigw_v1.Stability = apigw_v1.Stability_STABILITY_UNSPECIFIED
			var gotDeprecation *apigw_v1.Deprecation

			// Simulate what getFieldStability and getFieldDeprecation do
			for _, fo := range tt.fieldOptions {
				if fo.Stability != apigw_v1.Stability_STABILITY_UNSPECIFIED {
					gotStability = fo.Stability
				}
				if fo.Deprecation != nil {
					gotDeprecation = fo.Deprecation
				}
			}

			// Test stability extraction
			if gotStability != tt.wantStability {
				t.Errorf("stability extraction = %v, want %v", gotStability, tt.wantStability)
			}

			// Test deprecation extraction
			if gotDeprecation == nil && tt.wantDeprecation == nil {
				// Both nil, OK
			} else if gotDeprecation == nil || tt.wantDeprecation == nil {
				t.Errorf("deprecation extraction = %v, want %v", gotDeprecation, tt.wantDeprecation)
			} else if gotDeprecation.SunsetDate != tt.wantDeprecation.SunsetDate {
				t.Errorf("deprecation extraction = %v, want %v", gotDeprecation, tt.wantDeprecation)
			}

			// Test extension generation
			extensions := orderedmap.New[string, *yaml.Node]()

			// Add stability extension
			addStabilityExtension(extensions, gotStability)

			// Add deprecation extension
			if gotDeprecation != nil {
				addSunsetExtension(extensions, gotDeprecation.SunsetDate)
			}

			// Check stability extension
			stabilityNode, stabilityExists := extensions.Get("x-stability-level")
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

			// Check sunset extension
			sunsetNode, sunsetExists := extensions.Get("x-sunset")
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
		})
	}
}
