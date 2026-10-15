package apigw

import (
	"strings"
	"testing"

	apigw_v1 "github.com/ductone/protoc-gen-apigw/apigw/v1"
	"github.com/ductone/protoc-gen-apigw/tfoverlay"
)

func TestSelectorDeclarationValidation(t *testing.T) {
	remove := apigw_v1.OpenAPICustomizationMode_OPEN_API_CUSTOMIZATION_MODE_REMOVE
	tests := []struct {
		name    string
		sc      *apigw_v1.OpenAPISelectorCustomization
		wantErr string
	}{
		{"set by descendant name", &apigw_v1.OpenAPISelectorCustomization{
			Selector: &apigw_v1.OpenAPISelector{JsonPath: "$..deletedAt"}, Key: "x-speakeasy-soft-delete-property", ValueJson: `true`}, ""},
		{"set by filter", &apigw_v1.OpenAPISelectorCustomization{
			Selector: &apigw_v1.OpenAPISelector{JsonPath: "$..[?(@.additionalProperties == true)]"}, Key: "additionalProperties", ValueJson: `false`}, ""},
		{"remove", &apigw_v1.OpenAPISelectorCustomization{
			Selector: &apigw_v1.OpenAPISelector{JsonPath: "$..['@type']"}, Mode: remove}, ""},
		{"shared scope", &apigw_v1.OpenAPISelectorCustomization{
			Selector: &apigw_v1.OpenAPISelector{JsonPath: "$..k"}, Key: "x-a", ValueJson: `1`, Scope: scopeShared}, "Terraform-only"},
		{"no selector", &apigw_v1.OpenAPISelectorCustomization{Key: "x-a", ValueJson: `1`}, "needs a JSONPath"},
		{"not RFC 9535", &apigw_v1.OpenAPISelectorCustomization{
			Selector: &apigw_v1.OpenAPISelector{JsonPath: "deletedAt"}, Key: "x-a", ValueJson: `1`}, "RFC 9535"},
		{"set without value", &apigw_v1.OpenAPISelectorCustomization{
			Selector: &apigw_v1.OpenAPISelector{JsonPath: "$..k"}, Key: "x-a"}, "value_json is required"},
		{"set with an object value", &apigw_v1.OpenAPISelectorCustomization{
			Selector: &apigw_v1.OpenAPISelector{JsonPath: "$..k"}, Key: "x-a", ValueJson: `{"a":1}`}, "scalar"},
		{"remove with a value", &apigw_v1.OpenAPISelectorCustomization{
			Selector: &apigw_v1.OpenAPISelector{JsonPath: "$..k"}, Mode: remove, ValueJson: `1`}, "must be empty"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			collect := func(c *tfCollector) {
				c.declare(c.parseSelector("file test.proto", tfOwnerFile, "files", 0, tt.sc))
			}
			if tt.wantErr == "" {
				collect(newTFCollector())
				return
			}
			if got := expectDeclarationFailure(t, collect); !strings.Contains(got, tt.wantErr) {
				t.Fatalf("failure %q does not contain %q", got, tt.wantErr)
			}
		})
	}
}

func TestPropertyPatchDeclarationValidation(t *testing.T) {
	remove := apigw_v1.OpenAPICustomizationMode_OPEN_API_CUSTOMIZATION_MODE_REMOVE
	tests := []struct {
		name    string
		pp      *apigw_v1.OpenAPIPropertyPatch
		wantErr string
	}{
		{"add", &apigw_v1.OpenAPIPropertyPatch{Name: "p", SchemaJson: `{"type":"string"}`}, ""},
		{"remove", &apigw_v1.OpenAPIPropertyPatch{Name: "p", Mode: remove}, ""},
		{"pointer target", &apigw_v1.OpenAPIPropertyPatch{Name: "@type", SchemaJson: `{}`,
			Target: targetJSONPointer, JsonPointer: "/components/schemas/a.B/properties/config"}, ""},
		{"entity scoped", &apigw_v1.OpenAPIPropertyPatch{Name: "p", SchemaJson: `{}`,
			Entities: []*apigw_v1.OpenAPICustomizationEntity{entity("E", 0)}}, ""},
		{"needs a name", &apigw_v1.OpenAPIPropertyPatch{SchemaJson: `{}`}, "needs a name"},
		{"add needs a schema", &apigw_v1.OpenAPIPropertyPatch{Name: "p"}, "schema_json is required"},
		{"schema must be an object", &apigw_v1.OpenAPIPropertyPatch{Name: "p", SchemaJson: `"x"`}, "JSON object"},
		{"remove takes no schema", &apigw_v1.OpenAPIPropertyPatch{Name: "p", Mode: remove, SchemaJson: `{}`}, "must be empty"},
		{"shared scope", &apigw_v1.OpenAPIPropertyPatch{Name: "p", SchemaJson: `{}`, Scope: scopeShared}, "Terraform-only"},
		{"pointer outside components", &apigw_v1.OpenAPIPropertyPatch{Name: "p", SchemaJson: `{}`,
			Target: targetJSONPointer, JsonPointer: "/paths/~1x"}, "under /components/schemas"},
		{"parameter target", &apigw_v1.OpenAPIPropertyPatch{Name: "p", SchemaJson: `{}`,
			Target: apigw_v1.OpenAPICustomizationTarget_OPEN_API_CUSTOMIZATION_TARGET_PARAMETER}, "not valid for a property patch"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			collect := func(c *tfCollector) {
				c.declare(c.parsePropertyPatch("message test.M", tfOwnerMessage, "messagepp", 0, tt.pp))
			}
			if tt.wantErr == "" {
				collect(newTFCollector())
				return
			}
			if got := expectDeclarationFailure(t, collect); !strings.Contains(got, tt.wantErr) {
				t.Fatalf("failure %q does not contain %q", got, tt.wantErr)
			}
		})
	}
}

func TestSelectorAndPropertyAnnotationsEmit(t *testing.T) {
	collector := newTFCollector()
	sel := collector.parseSelector("file test.proto", tfOwnerFile, "files", 0, &apigw_v1.OpenAPISelectorCustomization{
		Selector: &apigw_v1.OpenAPISelector{JsonPath: "$..deletedAt"}, Key: "x-speakeasy-soft-delete-property", ValueJson: `true`})
	prop := collector.parsePropertyPatch("message test.M", tfOwnerMessage, "messagepp", 0, &apigw_v1.OpenAPIPropertyPatch{
		Name: "webhookHmacSecret", SchemaJson: `{"type":"string"}`})
	collector.declare(sel)
	collector.declare(prop)

	collector.beginDoc("svc", tfoverlay.KindService)
	collector.emit([]*tfDecl{sel}, documentRootPlaces(), nil)
	collector.emit([]*tfDecl{prop}, map[apigw_v1.OpenAPICustomizationTarget]string{
		apigw_v1.OpenAPICustomizationTarget_OPEN_API_CUSTOMIZATION_TARGET_SCHEMA: "/components/schemas/test.M",
	}, nil)
	ov := collector.endDoc()
	if err := collector.verify(); err != nil {
		t.Fatal(err)
	}
	if len(ov.terraform.Selectors) != 1 || ov.terraform.Selectors[0].Selector.Path != "$..deletedAt" {
		t.Fatalf("selectors = %+v", ov.terraform.Selectors)
	}
	if len(ov.terraform.Operations) != 1 {
		t.Fatalf("operations = %+v", ov.terraform.Operations)
	}
	op := ov.terraform.Operations[0]
	if op.Channel != tfoverlay.ChannelProperty || op.Pointer != "/components/schemas/test.M/properties" || op.Key != "webhookHmacSecret" {
		t.Fatalf("property operation = %+v", op)
	}
	rendered, err := ov.terraform.Render([]byte("components:\n  schemas:\n    test.M:\n      properties: {}\n"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(rendered.Global), "- target: $..deletedAt\n") ||
		!strings.Contains(string(rendered.Global), "x-apigw-property: true") {
		t.Fatalf("rendered:\n%s", rendered.Global)
	}
	if _, err := tfoverlay.Parse(rendered.Global); err != nil {
		t.Fatalf("artifact does not parse: %v", err)
	}
}
