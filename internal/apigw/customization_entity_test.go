package apigw

import (
	"strings"
	"testing"

	apigw_v1 "github.com/ductone/protoc-gen-apigw/apigw/v1"
	"github.com/ductone/protoc-gen-apigw/tfoverlay"
)

const (
	entityKindResource   = apigw_v1.OpenAPICustomizationEntityKind_OPEN_API_CUSTOMIZATION_ENTITY_KIND_RESOURCE
	entityKindDataSource = apigw_v1.OpenAPICustomizationEntityKind_OPEN_API_CUSTOMIZATION_ENTITY_KIND_DATA_SOURCE
	targetJSONPointer    = apigw_v1.OpenAPICustomizationTarget_OPEN_API_CUSTOMIZATION_TARGET_JSON_POINTER
	scopeShared          = apigw_v1.OpenAPICustomizationScope_OPEN_API_CUSTOMIZATION_SCOPE_SHARED
)

func entity(name string, kind apigw_v1.OpenAPICustomizationEntityKind) *apigw_v1.OpenAPICustomizationEntity {
	return &apigw_v1.OpenAPICustomizationEntity{Name: name, Kind: kind}
}

func TestEntitySelectorValidation(t *testing.T) {
	custom := func(mutate func(*apigw_v1.OpenAPICustomization)) *apigw_v1.OpenAPICustomization {
		c := &apigw_v1.OpenAPICustomization{
			Key:       "x-speakeasy-name-override",
			ValueJson: `"provisionPolicy"`,
			Entities:  []*apigw_v1.OpenAPICustomizationEntity{entity("Custom App Entitlement", entityKindResource)},
		}
		mutate(c)
		return c
	}
	tests := []struct {
		name    string
		cust    *apigw_v1.OpenAPICustomization
		wantErr string
	}{
		{"legal", custom(func(*apigw_v1.OpenAPICustomization) {}), ""},
		{"legal with both kinds of one name", custom(func(c *apigw_v1.OpenAPICustomization) {
			c.Entities = []*apigw_v1.OpenAPICustomizationEntity{entity("E", entityKindResource), entity("E", entityKindDataSource)}
		}), ""},
		{"legal json pointer into components", custom(func(c *apigw_v1.OpenAPICustomization) {
			c.Target, c.JsonPointer = targetJSONPointer, "/components/schemas/a.B/properties/c"
		}), ""},
		{"shared scope", custom(func(c *apigw_v1.OpenAPICustomization) { c.Scope = scopeShared }), "entities requires the Terraform scope"},
		{"parameter target", custom(func(c *apigw_v1.OpenAPICustomization) {
			c.Target = apigw_v1.OpenAPICustomizationTarget_OPEN_API_CUSTOMIZATION_TARGET_PARAMETER
		}), "cannot be entity-scoped"},
		{"json pointer outside components", custom(func(c *apigw_v1.OpenAPICustomization) {
			c.Target, c.JsonPointer = targetJSONPointer, "/paths/~1things/get"
		}), "under /components/schemas"},
		{"json pointer at the components root", custom(func(c *apigw_v1.OpenAPICustomization) {
			c.Target, c.JsonPointer = targetJSONPointer, "/components/schemas"
		}), "under /components/schemas"},
		{"empty name", custom(func(c *apigw_v1.OpenAPICustomization) {
			c.Entities = []*apigw_v1.OpenAPICustomizationEntity{entity("", entityKindResource)}
		}), "needs a name"},
		{"duplicate selector", custom(func(c *apigw_v1.OpenAPICustomization) {
			c.Entities = []*apigw_v1.OpenAPICustomizationEntity{entity("E", entityKindResource), entity("E", entityKindResource)}
		}), "duplicate entity selector"},
		{"unknown kind", custom(func(c *apigw_v1.OpenAPICustomization) {
			c.Entities = []*apigw_v1.OpenAPICustomizationEntity{entity("E", 99)}
		}), "unknown kind"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			collect := func(c *tfCollector) {
				c.declare(c.parseCustomization("field test.Field.f", tfOwnerField, "field", 0, tt.cust))
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

	t.Run("schema patches validate the same way", func(t *testing.T) {
		patch := func(mutate func(*apigw_v1.OpenAPISchemaPatch)) *apigw_v1.OpenAPISchemaPatch {
			p := &apigw_v1.OpenAPISchemaPatch{
				PatchJson: `{"default":1}`,
				Entities:  []*apigw_v1.OpenAPICustomizationEntity{entity("E", entityKindDataSource)},
			}
			mutate(p)
			return p
		}
		newCollector := newTFCollector()
		newCollector.declare(newCollector.parsePatch("field test.Field.f", tfOwnerField, "fieldp", 0, patch(func(*apigw_v1.OpenAPISchemaPatch) {})))
		for name, tc := range map[string]struct {
			patch   *apigw_v1.OpenAPISchemaPatch
			wantErr string
		}{
			"shared scope": {patch(func(p *apigw_v1.OpenAPISchemaPatch) { p.Scope = scopeShared }), "entities requires the Terraform scope"},
			"array items is fine": {patch(func(p *apigw_v1.OpenAPISchemaPatch) {
				p.Target = apigw_v1.OpenAPICustomizationTarget_OPEN_API_CUSTOMIZATION_TARGET_ARRAY_ITEMS
			}), ""},
			"empty name": {patch(func(p *apigw_v1.OpenAPISchemaPatch) {
				p.Entities = []*apigw_v1.OpenAPICustomizationEntity{entity("", 0)}
			}), "needs a name"},
		} {
			collect := func(c *tfCollector) {
				c.declare(c.parsePatch("field test.Field.f", tfOwnerField, "fieldp", 0, tc.patch))
			}
			if tc.wantErr == "" {
				collect(newTFCollector())
				continue
			}
			if got := expectDeclarationFailure(t, collect); !strings.Contains(got, tc.wantErr) {
				t.Errorf("%s: failure %q does not contain %q", name, got, tc.wantErr)
			}
		}
	})
}

func scopedCollector(t *testing.T, cust *apigw_v1.OpenAPICustomization) (*tfCollector, *tfDecl) {
	t.Helper()
	collector := newTFCollector()
	decl := collector.parseCustomization("field test.Field.f", tfOwnerField, "field", 0, cust)
	collector.declare(decl)
	return collector, decl
}

func TestEntityScopedAnnotationEmitsScopedOperations(t *testing.T) {
	collector, decl := scopedCollector(t, &apigw_v1.OpenAPICustomization{
		Key:       "x-speakeasy-name-override",
		ValueJson: `"provisionPolicy"`,
		Entities: []*apigw_v1.OpenAPICustomizationEntity{
			entity("Custom App Entitlement", entityKindResource),
			entity("Other", 0),
		},
	})
	pointer := "/components/schemas/c1.api.app.v1.AppEntitlement/properties/provisionerPolicy"
	collector.beginDoc("c1.api.app.v1.AppEntitlements", tfoverlay.KindService)
	collector.emit([]*tfDecl{decl}, map[apigw_v1.OpenAPICustomizationTarget]string{
		apigw_v1.OpenAPICustomizationTarget_OPEN_API_CUSTOMIZATION_TARGET_SCHEMA: pointer,
	}, nil)
	ov := collector.endDoc()
	if err := collector.verify(); err != nil {
		t.Fatalf("verify: %v", err)
	}

	got := ov.terraform.Operations
	if len(got) != 2 {
		t.Fatalf("operations = %d, want one per entity", len(got))
	}
	want := map[string]tfoverlay.EntityScope{
		"Custom App Entitlement": {Name: "Custom App Entitlement", Kind: tfoverlay.EntityKindResource},
		"Other":                  {Name: "Other", Kind: tfoverlay.EntityKindAny},
	}
	for _, op := range got {
		if op.Entity == nil || *op.Entity != want[op.Entity.Name] || op.Pointer != pointer {
			t.Errorf("operation = %+v (entity %v)", op, op.Entity)
		}
	}
	if !ov.shared.Empty() {
		t.Error("a scoped operation reached the shared document")
	}

	doc := []byte("components:\n  schemas:\n    c1.api.app.v1.AppEntitlement:\n      properties:\n        provisionerPolicy: {}\n")
	rendered, err := ov.terraform.Render(doc)
	if err != nil {
		t.Fatal(err)
	}
	if rendered.Global != nil || !strings.Contains(string(rendered.Entities), "part: entities") {
		t.Fatalf("rendered:\n%s\n---\n%s", rendered.Global, rendered.Entities)
	}
	parsed, err := tfoverlay.Parse(rendered.Entities)
	if err != nil || len(parsed.Operations) != 2 {
		t.Fatalf("artifact does not parse: %v", err)
	}
}

func TestEntityScopedAnnotationAtAParameterIsNotApplied(t *testing.T) {
	collector, decl := scopedCollector(t, &apigw_v1.OpenAPICustomization{
		Key:       "x-a",
		ValueJson: `1`,
		Entities:  []*apigw_v1.OpenAPICustomizationEntity{entity("E", 0)},
	})
	collector.beginDoc("svc", tfoverlay.KindService)
	collector.emit([]*tfDecl{decl}, map[apigw_v1.OpenAPICustomizationTarget]string{
		apigw_v1.OpenAPICustomizationTarget_OPEN_API_CUSTOMIZATION_TARGET_SCHEMA: "/paths/~1things/get/parameters/0/schema",
	}, nil)
	ov := collector.endDoc()
	if !ov.terraform.Empty() {
		t.Fatalf("operations were written at a parameter: %+v", ov.terraform.Operations)
	}
	err := collector.verify()
	if err == nil || !strings.Contains(err.Error(), "/components/schemas") {
		t.Fatalf("verify = %v, want an unapplied annotation with its reason", err)
	}
}

func TestEntityScopedAndSharedOperationsAtOneDestinationCoexist(t *testing.T) {
	// A shared-scoped override and an entity-scoped Terraform override at the same
	// destination are not a scope conflict: the entity's copy diverges on purpose.
	ov := newDocOverlay("svc", tfoverlay.KindService)
	pointer := "/components/schemas/X/properties/p"
	value, _ := tfoverlay.ParseJSONValue(`"shared"`)
	scopedValue, _ := tfoverlay.ParseJSONValue(`"scoped"`)
	shared := tfoverlay.Operation{Pointer: pointer, Channel: tfoverlay.ChannelExtension, Key: "x-a", Mode: tfoverlay.ModeSet, Value: value}
	scoped := shared
	scoped.Value = scopedValue
	scoped.Entity = &tfoverlay.EntityScope{Name: "E"}
	if err := ov.add(shared, scopeShared); err != nil {
		t.Fatal(err)
	}
	if err := ov.add(scoped, apigw_v1.OpenAPICustomizationScope_OPEN_API_CUSTOMIZATION_SCOPE_TERRAFORM); err != nil {
		t.Fatalf("scoped operation conflicted with a shared one: %v", err)
	}
}

func TestFinishDocumentChecksScopedDestinations(t *testing.T) {
	spec := []byte("openapi: 3.1.0\ncomponents:\n  schemas:\n    X:\n      properties:\n        p:\n          type: string\n")
	value, _ := tfoverlay.ParseJSONValue(`1`)
	op := func(pointer string) tfoverlay.Operation {
		return tfoverlay.Operation{
			Pointer: pointer, Channel: tfoverlay.ChannelExtension, Key: "x-a", Mode: tfoverlay.ModeSet, Value: value,
			Entity: &tfoverlay.EntityScope{Name: "E"},
		}
	}
	module := &Module{}

	ov := newDocOverlay("svc", tfoverlay.KindService)
	_ = ov.terraform.Add(op("/components/schemas/X/properties/p"))
	if out, err := module.finishDocument(spec, ov); err != nil || string(out) != string(spec) {
		t.Fatalf("a resolving scoped destination: err=%v, document changed=%v", err, string(out) != string(spec))
	}

	bad := newDocOverlay("svc", tfoverlay.KindService)
	_ = bad.terraform.Add(op("/components/schemas/Missing/properties/p"))
	if _, err := module.finishDocument(spec, bad); err == nil || !strings.Contains(err.Error(), "does not resolve") {
		t.Fatalf("an unresolved scoped destination: %v", err)
	}
}
