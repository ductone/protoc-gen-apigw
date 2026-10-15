package tfoverlay

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"

	"github.com/speakeasy-api/openapi/overlay"
	"gopkg.in/yaml.v3"
)

const (
	appEntitlement = "c1.api.app.v1.AppEntitlement"
	appView        = "c1.api.app.v1.AppEntitlementView"
	customEntity   = "Custom App Entitlement"
	cloneKey       = appEntitlement + ".tf.CustomAppEntitlement"
	viewCloneKey   = appView + ".tf.CustomAppEntitlement"
)

func readTestdata(t *testing.T, name string) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", "iga4347", name))
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func scopedOp(t *testing.T, pointer, key, valueJSON string, entity *EntityScope) Operation {
	t.Helper()
	value, err := ParseJSONValue(valueJSON)
	if err != nil {
		t.Fatal(err)
	}
	channel := ChannelExtension
	if !strings.HasPrefix(key, "x-") {
		channel = ChannelSchema
	}
	return Operation{Pointer: pointer, Channel: channel, Key: key, Mode: ModeSet, Value: value, Entity: entity}
}

func customResource() *EntityScope {
	return &EntityScope{Name: customEntity, Kind: EntityKindResource}
}

var policyPointer = "/components/schemas/" + appEntitlement + "/properties/provisionerPolicy"

func iga4347Ops(t *testing.T) []Operation {
	t.Helper()
	ov, err := Parse(readTestdata(t, "scoped.terraform_entity_overlay.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	return ov.Operations
}

// decode returns a document as plain data, so that comparisons ignore key order
// and formatting.
func decode(t *testing.T, data []byte) map[string]interface{} {
	t.Helper()
	var out map[string]interface{}
	if err := yaml.Unmarshal(data, &out); err != nil {
		t.Fatal(err)
	}
	return out
}

// applyOpenAPIOverlay applies a standard OpenAPI Overlay document with the
// Speakeasy implementation, in strict mode.
func applyOpenAPIOverlay(t *testing.T, doc map[string]interface{}, data []byte) map[string]interface{} {
	t.Helper()
	reference, err := overlay.ParseReader(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	encoded, err := yaml.Marshal(doc)
	if err != nil {
		t.Fatal(err)
	}
	root := &yaml.Node{}
	if err := yaml.Unmarshal(encoded, root); err != nil {
		t.Fatal(err)
	}
	if _, err := reference.ApplyToStrict(root); err != nil {
		t.Fatal(err)
	}
	var out map[string]interface{}
	if err := root.Decode(&out); err != nil {
		t.Fatal(err)
	}
	return out
}

func schemaOf(doc map[string]interface{}, key string) map[string]interface{} {
	return doc["components"].(map[string]interface{})["schemas"].(map[string]interface{})[key].(map[string]interface{})
}

func TestApplyTerraformMatchesTheReferenceProjection(t *testing.T) {
	spec := readTestdata(t, "spec.yaml")
	out, report, err := ApplyTerraform(spec, iga4347Ops(t))
	if err != nil {
		t.Fatalf("ApplyTerraform: %v", err)
	}

	want := applyOpenAPIOverlay(t, decode(t, spec), readTestdata(t, "expected.overlay.yaml"))
	got := decode(t, out)
	if !reflect.DeepEqual(want, got) {
		t.Fatalf("projection differs from the hand-written reference\n--- want\n%v\n--- got\n%v", want, got)
	}

	t.Run("the projection is expressible as a standard overlay", func(t *testing.T) {
		generated, err := ProjectionOverlay("projection", spec, out)
		if err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(applyOpenAPIOverlay(t, decode(t, spec), generated), got) {
			t.Fatal("the generated projection overlay does not reproduce the projection")
		}
	})

	t.Run("report", func(t *testing.T) {
		if len(report.Entities) != 1 {
			t.Fatalf("entities = %d, want 1", len(report.Entities))
		}
		got := report.Entities[0]
		wantClones := []ComponentClone{
			{From: appEntitlement, To: cloneKey, NameOverride: "AppEntitlementCustomAppEntitlement"},
			{From: appView, To: viewCloneKey, NameOverride: "AppEntitlementViewCustomAppEntitlement"},
		}
		if !reflect.DeepEqual(got.Clones, wantClones) {
			t.Errorf("clones = %+v, want %+v", got.Clones, wantClones)
		}
		wantInPlace := []string{
			"c1.api.app.v1.CreateAppEntitlementResponse",
			"c1.api.app.v1.GetAppEntitlementResponse",
			"c1.api.app.v1.UpdateAppEntitlementRequestInput",
			"c1.api.app.v1.UpdateAppEntitlementResponse",
		}
		if !reflect.DeepEqual(got.InPlace, wantInPlace) {
			t.Errorf("in place = %v, want %v", got.InPlace, wantInPlace)
		}
		if got.Rebinding == nil || got.Rebinding.From != appEntitlement || got.Rebinding.To != cloneKey {
			t.Errorf("rebinding = %+v", got.Rebinding)
		}
		if len(got.Pointers) != 1 || got.Pointers[0].To != strings.Replace(policyPointer, appEntitlement, cloneKey, 1) {
			t.Errorf("pointers = %+v", got.Pointers)
		}
		if len(got.Operations) != 4 {
			t.Errorf("operations = %v, want the four Custom App Entitlement operations", got.Operations)
		}
	})

	t.Run("other entities are untouched", func(t *testing.T) {
		original := decode(t, spec)
		for _, key := range []string{
			"c1.api.app.v1.SearchAppEntitlementsResponse",
			"c1.api.requestcatalog.v1.RequestCatalog",
			"c1.api.policy.v1.ProvisionPolicy",
			"c1.api.app.v1.CreateAppEntitlementRequest",
		} {
			if !reflect.DeepEqual(schemaOf(original, key), schemaOf(got, key)) {
				t.Errorf("component %s changed", key)
			}
		}
		if !reflect.DeepEqual(original["paths"], got["paths"]) {
			t.Error("paths changed")
		}
		properties := schemaOf(got, appEntitlement)["properties"].(map[string]interface{})
		if _, leaked := properties["provisionerPolicy"].(map[string]interface{})["x-speakeasy-name-override"]; leaked {
			t.Error("the override leaked onto the shared component")
		}
		if schemaOf(got, appEntitlement)["x-speakeasy-entity"] != "App Entitlement" {
			t.Errorf("original entity binding = %v", schemaOf(got, appEntitlement)["x-speakeasy-entity"])
		}
	})
}

func TestApplyTerraformIsDeterministic(t *testing.T) {
	spec := readTestdata(t, "spec.yaml")
	ops := append(iga4347Ops(t),
		scopedOp(t, "/components/schemas/"+appEntitlement+"/properties/id", "x-speakeasy-param-computed", `true`, customResource()),
		scopedOp(t, "/components/schemas/"+appEntitlement, "title", `"Custom"`, customResource()),
		scopedOp(t, "/components/schemas/c1.api.app.v1.CreateAppEntitlementRequest", "x-global", `1`, nil),
	)
	first, _, err := ApplyTerraform(spec, ops)
	if err != nil {
		t.Fatal(err)
	}

	again, _, err := ApplyTerraform(spec, ops)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(first, again) {
		t.Fatal("two runs over the same input differ")
	}

	// Every ordering of the operation list yields the same bytes.
	permute(len(ops), func(order []int) {
		shuffled := make([]Operation, 0, len(ops))
		for _, i := range order {
			shuffled = append(shuffled, ops[i])
		}
		got, _, err := ApplyTerraform(spec, shuffled)
		if err != nil {
			t.Fatalf("order %v: %v", order, err)
		}
		if !bytes.Equal(first, got) {
			t.Fatalf("order %v changed the output", order)
		}
	})
}

func permute(n int, visit func([]int)) {
	order := make([]int, n)
	for i := range order {
		order[i] = i
	}
	var rec func(k int)
	rec = func(k int) {
		if k == n {
			visit(order)
			return
		}
		for i := k; i < n; i++ {
			order[k], order[i] = order[i], order[k]
			rec(k + 1)
			order[k], order[i] = order[i], order[k]
		}
	}
	rec(0)
}

func TestApplyTerraformWithoutScopedOperationsEqualsApply(t *testing.T) {
	spec := readTestdata(t, "spec.yaml")
	ops := []Operation{scopedOp(t, "/components/schemas/"+appEntitlement, "x-global", `"g"`, nil)}
	want, err := Apply(spec, ops)
	if err != nil {
		t.Fatal(err)
	}
	got, report, err := ApplyTerraform(spec, ops)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(want, got) || len(report.Entities) != 0 {
		t.Fatal("global-only ApplyTerraform must equal Apply and project nothing")
	}
}

func TestApplyTerraformReapplicationIsRejected(t *testing.T) {
	// The projection is built from the unprojected document. Applying the same
	// operations to its own output finds the destination unreachable from the
	// entity, which is reported instead of projecting twice.
	spec := readTestdata(t, "spec.yaml")
	ops := iga4347Ops(t)
	out, _, err := ApplyTerraform(spec, ops)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := ApplyTerraform(out, ops); err == nil || !strings.Contains(err.Error(), "not reachable") {
		t.Fatalf("re-application error = %v, want a not-reachable error", err)
	}
}

func TestApplyTerraformPrecedenceAndInheritance(t *testing.T) {
	spec := readTestdata(t, "spec.yaml")
	ops := []Operation{
		// Global at the same destination as the scoped operation: scoped wins on
		// the copy, global stays on the shared component.
		scopedOp(t, policyPointer, "x-speakeasy-name-override", `"globalName"`, nil),
		scopedOp(t, policyPointer, "x-speakeasy-name-override", `"provisionPolicy"`, customResource()),
		// Global elsewhere is inherited by the copy.
		scopedOp(t, "/components/schemas/"+appEntitlement+"/properties/id", "x-inherited", `true`, nil),
	}
	out, _, err := ApplyTerraform(spec, ops)
	if err != nil {
		t.Fatal(err)
	}
	doc := decode(t, out)
	prop := func(component string) map[string]interface{} {
		return schemaOf(doc, component)["properties"].(map[string]interface{})["provisionerPolicy"].(map[string]interface{})
	}
	if got := prop(appEntitlement)["x-speakeasy-name-override"]; got != "globalName" {
		t.Errorf("shared component override = %v, want globalName", got)
	}
	if got := prop(cloneKey)["x-speakeasy-name-override"]; got != "provisionPolicy" {
		t.Errorf("copy override = %v, want provisionPolicy", got)
	}
	id := schemaOf(doc, cloneKey)["properties"].(map[string]interface{})["id"].(map[string]interface{})
	if id["x-inherited"] != true {
		t.Errorf("the copy did not inherit the global operation: %v", id)
	}
}

func TestApplyTerraformScopedRemoveOfGlobalKey(t *testing.T) {
	spec := readTestdata(t, "spec.yaml")
	pointer := "/components/schemas/" + appEntitlement + "/properties/id"
	ops := []Operation{
		scopedOp(t, pointer, "x-temp", `1`, nil),
		{Pointer: pointer, Channel: ChannelExtension, Key: "x-temp", Mode: ModeRemove, Entity: customResource()},
	}
	out, _, err := ApplyTerraform(spec, ops)
	if err != nil {
		t.Fatal(err)
	}
	doc := decode(t, out)
	shared := schemaOf(doc, appEntitlement)["properties"].(map[string]interface{})["id"].(map[string]interface{})
	copied := schemaOf(doc, cloneKey)["properties"].(map[string]interface{})["id"].(map[string]interface{})
	if shared["x-temp"] != 1 {
		t.Errorf("shared id = %v", shared)
	}
	if _, present := copied["x-temp"]; present {
		t.Errorf("copy kept the removed key: %v", copied)
	}
}

const cycleSpec = `
openapi: 3.1.0
paths:
  /things/{id}:
    get:
      x-speakeasy-entity-operation: Thing#read
      responses:
        '200':
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/Thing'
  /nodes:
    get:
      responses:
        '200':
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/Node'
components:
  schemas:
    Thing:
      type: object
      x-speakeasy-entity: Thing
      properties:
        node:
          $ref: '#/components/schemas/Node'
    Node:
      type: object
      discriminator:
        propertyName: kind
        mapping:
          thing: '#/components/schemas/Thing'
      properties:
        label:
          type: string
        thing:
          $ref: '#/components/schemas/Thing'
`

func TestApplyTerraformClonesAWholeCycle(t *testing.T) {
	ops := []Operation{scopedOp(t, "/components/schemas/Node/properties/label", "x-speakeasy-name-override", `"title"`, &EntityScope{Name: "Thing"})}
	out, report, err := ApplyTerraform([]byte(cycleSpec), ops)
	if err != nil {
		t.Fatal(err)
	}
	doc := decode(t, out)
	if len(report.Entities[0].Clones) != 2 || len(report.Entities[0].InPlace) != 0 {
		t.Fatalf("report = %+v, want both cycle members cloned", report.Entities[0])
	}
	thing := schemaOf(doc, "Thing.tf.Thing")
	node := schemaOf(doc, "Node.tf.Thing")
	if thing["x-speakeasy-entity"] != "Thing" {
		t.Errorf("the copy of the root must carry the entity: %v", thing)
	}
	if _, bound := schemaOf(doc, "Thing")["x-speakeasy-entity"]; bound {
		t.Error("the original root must lose the entity binding")
	}
	if thing["properties"].(map[string]interface{})["node"].(map[string]interface{})["$ref"] != "#/components/schemas/Node.tf.Thing" {
		t.Errorf("thing -> node = %v", thing)
	}
	nodeProps := node["properties"].(map[string]interface{})
	if nodeProps["thing"].(map[string]interface{})["$ref"] != "#/components/schemas/Thing.tf.Thing" {
		t.Errorf("node -> thing = %v", nodeProps["thing"])
	}
	mapping := node["discriminator"].(map[string]interface{})["mapping"].(map[string]interface{})
	if mapping["thing"] != "#/components/schemas/Thing.tf.Thing" {
		t.Errorf("discriminator mapping = %v", mapping)
	}
	if nodeProps["label"].(map[string]interface{})["x-speakeasy-name-override"] != "title" {
		t.Errorf("scoped operation missing on the copy: %v", nodeProps["label"])
	}
	original := schemaOf(doc, "Node")
	if original["discriminator"].(map[string]interface{})["mapping"].(map[string]interface{})["thing"] != "#/components/schemas/Thing" {
		t.Error("the shared component's references changed")
	}
	// The operation outside the entity still returns the shared component.
	before := decode(t, []byte(cycleSpec))["paths"].(map[string]interface{})["/nodes"]
	if !reflect.DeepEqual(before, doc["paths"].(map[string]interface{})["/nodes"]) {
		t.Error("an operation outside the entity changed")
	}
}

const sharedKindSpec = `
openapi: 3.1.0
paths:
  /things:
    post:
      x-speakeasy-entity-operation: Thing#create
      requestBody:
        content:
          application/json:
            schema:
              $ref: '#/components/schemas/ThingInput'
      responses:
        '200':
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/Thing'
  /things/{id}:
    get:
      x-speakeasy-entity-operation: Thing#read
      responses:
        '200':
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/Thing'
components:
  schemas:
    ThingInput:
      type: object
    Thing:
      type: object
      x-speakeasy-entity: Thing
      properties:
        id:
          type: string
`

func TestApplyTerraformProjectionErrors(t *testing.T) {
	thing := func(kind EntityKind) *EntityScope { return &EntityScope{Name: "Thing", Kind: kind} }
	idPointer := "/components/schemas/Thing/properties/id"
	tests := []struct {
		name    string
		spec    string
		ops     []Operation
		wantErr string
	}{
		{
			name:    "unknown entity",
			spec:    sharedKindSpec,
			ops:     []Operation{scopedOp(t, idPointer, "x-a", `1`, &EntityScope{Name: "Nope"})},
			wantErr: "no operation of the entity",
		},
		{
			name:    "kind-restricted override of operations shared with the other kind",
			spec:    sharedKindSpec,
			ops:     []Operation{scopedOp(t, idPointer, "x-a", `1`, thing(EntityKindResource))},
			wantErr: "cannot split resource and data source that share operation",
		},
		{
			name: "differing resource and data source overrides",
			spec: sharedKindSpec,
			ops: []Operation{
				scopedOp(t, idPointer, "x-a", `1`, thing(EntityKindResource)),
				scopedOp(t, idPointer, "x-a", `2`, thing(EntityKindDataSource)),
			},
			wantErr: "differing scoped operations",
		},
		{
			name:    "destination outside the components",
			spec:    sharedKindSpec,
			ops:     []Operation{scopedOp(t, "/paths/~1things/post", "x-a", `1`, thing(EntityKindAny))},
			wantErr: "not inside a component schema",
		},
		{
			name:    "destination does not resolve",
			spec:    sharedKindSpec,
			ops:     []Operation{scopedOp(t, "/components/schemas/Missing/properties/id", "x-a", `1`, thing(EntityKindAny))},
			wantErr: "no component schema",
		},
		{
			name:    "destination is not reachable from the entity",
			spec:    cycleSpec,
			ops:     []Operation{scopedOp(t, "/components/schemas/Thing/properties/node", "x-a", `1`, &EntityScope{Name: "Other"})},
			wantErr: "no operation of the entity",
		},
		{
			name: "destination exists but the entity cannot reach it",
			spec: strings.Replace(sharedKindSpec, "components:\n  schemas:\n", "components:\n  schemas:\n    Island:\n      type: object\n      properties:\n        id:\n          type: string\n", 1),
			ops:  []Operation{scopedOp(t, "/components/schemas/Island/properties/id", "x-a", `1`, thing(EntityKindAny))},
			// Island is not referenced by Thing's operations.
			wantErr: "not reachable",
		},
		{
			name:    "the entity has no root schema",
			spec:    strings.Replace(sharedKindSpec, "      x-speakeasy-entity: Thing\n", "", 1),
			ops:     []Operation{scopedOp(t, idPointer, "x-a", `1`, thing(EntityKindAny))},
			wantErr: "expected exactly one schema",
		},
		{
			name:    "the entity has two root schemas",
			spec:    strings.Replace(sharedKindSpec, "    ThingInput:\n      type: object\n", "    ThingInput:\n      type: object\n      x-speakeasy-entity: Thing\n", 1),
			ops:     []Operation{scopedOp(t, idPointer, "x-a", `1`, thing(EntityKindAny))},
			wantErr: "expected exactly one schema",
		},
		{
			name:    "a removal that cannot apply",
			spec:    sharedKindSpec,
			ops:     []Operation{{Pointer: idPointer, Channel: ChannelExtension, Key: "x-absent", Mode: ModeRemove, Entity: thing(EntityKindAny)}},
			wantErr: "key is not present",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, _, err := ApplyTerraform([]byte(tt.spec), tt.ops)
			if err == nil {
				t.Fatalf("ApplyTerraform = nil, want an error containing %q", tt.wantErr)
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("error %q does not contain %q", err, tt.wantErr)
			}
		})
	}
}

func TestApplyTerraformCollidingCloneIsAnError(t *testing.T) {
	spec := strings.Replace(cycleSpec, "    Node:\n", "    Node.tf.Thing:\n      type: object\n    Node:\n", 1)
	ops := []Operation{scopedOp(t, "/components/schemas/Node/properties/label", "x-a", `1`, &EntityScope{Name: "Thing"})}
	if _, _, err := ApplyTerraform([]byte(spec), ops); err == nil || !strings.Contains(err.Error(), "already exists") {
		t.Fatalf("error = %v, want a collision error", err)
	}
}

func TestApplyTerraformAgreeingSelectorsApplyToTheWholeEntity(t *testing.T) {
	ops := []Operation{
		scopedOp(t, "/components/schemas/Thing/properties/id", "x-a", `1`, &EntityScope{Name: "Thing", Kind: EntityKindResource}),
		scopedOp(t, "/components/schemas/Thing/properties/id", "x-a", `1`, &EntityScope{Name: "Thing", Kind: EntityKindDataSource}),
	}
	if _, _, err := ApplyTerraform([]byte(sharedKindSpec), ops); err != nil {
		t.Fatalf("agreeing selectors: %v", err)
	}
}

func TestApplyAndValidateRejectScopedOperations(t *testing.T) {
	spec := readTestdata(t, "spec.yaml")
	ops := iga4347Ops(t)
	if _, err := Apply(spec, ops); err == nil || !strings.Contains(err.Error(), "use ApplyTerraform") {
		t.Fatalf("Apply error = %v", err)
	}
	if err := Validate(spec, ops); err == nil || !strings.Contains(err.Error(), "use ApplyTerraform") {
		t.Fatalf("Validate error = %v", err)
	}
	if err := ValidateTerraform(spec, ops); err != nil {
		t.Fatalf("ValidateTerraform: %v", err)
	}
	bad := []Operation{scopedOp(t, "/components/schemas/Missing", "x-a", `1`, customResource())}
	if err := ValidateTerraform(spec, bad); err == nil {
		t.Fatal("ValidateTerraform accepted an unresolved scoped destination")
	}
}

func TestEntityScopedConflicts(t *testing.T) {
	other := &EntityScope{Name: "Other App Entitlement", Kind: EntityKindResource}
	anyKind := &EntityScope{Name: customEntity}
	dataSource := &EntityScope{Name: customEntity, Kind: EntityKindDataSource}

	t.Run("different values for one entity conflict", func(t *testing.T) {
		ov := &Overlay{}
		if err := ov.Add(scopedOp(t, policyPointer, "x-a", `1`, customResource())); err != nil {
			t.Fatal(err)
		}
		err := ov.Add(scopedOp(t, policyPointer, "x-a", `2`, customResource()))
		var structured *Error
		if !errors.As(err, &structured) || structured.Entity == nil || structured.Entity.Name != customEntity {
			t.Fatalf("error = %v, want a structured error naming the entity", err)
		}
	})
	t.Run("identical values for one entity deduplicate", func(t *testing.T) {
		ov := &Overlay{}
		for i := 0; i < 2; i++ {
			if err := ov.Add(scopedOp(t, policyPointer, "x-a", `1`, customResource())); err != nil {
				t.Fatal(err)
			}
		}
		if len(ov.Operations) != 1 {
			t.Fatalf("operations = %d, want 1", len(ov.Operations))
		}
	})
	t.Run("different entities never conflict", func(t *testing.T) {
		ov := &Overlay{}
		for i, e := range []*EntityScope{customResource(), other} {
			if err := ov.Add(scopedOp(t, policyPointer, "x-a", strconv.Itoa(i), e)); err != nil {
				t.Fatal(err)
			}
		}
	})
	t.Run("global and scoped never conflict", func(t *testing.T) {
		ov := &Overlay{}
		if err := ov.Add(scopedOp(t, policyPointer, "x-a", `1`, nil)); err != nil {
			t.Fatal(err)
		}
		if err := ov.Add(scopedOp(t, policyPointer, "x-a", `2`, customResource())); err != nil {
			t.Fatal(err)
		}
	})
	t.Run("an any-kind selector conflicts with a kind of the same entity", func(t *testing.T) {
		ov := &Overlay{}
		if err := ov.Add(scopedOp(t, policyPointer, "x-a", `1`, anyKind)); err != nil {
			t.Fatal(err)
		}
		if err := ov.Add(scopedOp(t, policyPointer, "x-a", `2`, dataSource)); err == nil {
			t.Fatal("want a conflict")
		}
	})
	t.Run("overlapping destinations within one entity conflict", func(t *testing.T) {
		ov := &Overlay{}
		if err := ov.Add(scopedOp(t, "/components/schemas/X", "properties", `{}`, customResource())); err != nil {
			t.Fatal(err)
		}
		err := ov.Add(scopedOp(t, "/components/schemas/X/properties/p", "x-a", `1`, customResource()))
		if err == nil || !strings.Contains(err.Error(), "overlapping") {
			t.Fatalf("error = %v", err)
		}
	})
	t.Run("overlapping destinations across entities are fine", func(t *testing.T) {
		ov := &Overlay{}
		if err := ov.Add(scopedOp(t, "/components/schemas/X", "properties", `{}`, customResource())); err != nil {
			t.Fatal(err)
		}
		if err := ov.Add(scopedOp(t, "/components/schemas/X/properties/p", "x-a", `1`, other)); err != nil {
			t.Fatal(err)
		}
	})
	t.Run("merge keeps entities apart and rejects a differing duplicate", func(t *testing.T) {
		a := &Overlay{Source: "a"}
		b := &Overlay{Source: "b"}
		_ = a.Add(scopedOp(t, policyPointer, "x-a", `1`, customResource()))
		_ = b.Add(scopedOp(t, policyPointer, "x-a", `1`, customResource()))
		merged, err := Merge(a, b)
		if err != nil || len(merged.Operations) != 1 {
			t.Fatalf("merge identical: %v, %v", merged, err)
		}
		c := &Overlay{Source: "c"}
		_ = c.Add(scopedOp(t, policyPointer, "x-a", `2`, customResource()))
		if _, err := Merge(a, c); err == nil {
			t.Fatal("merge accepted a differing scoped duplicate")
		}
		d := &Overlay{Source: "d"}
		_ = d.Add(scopedOp(t, policyPointer, "x-a", `2`, other))
		if _, err := Merge(a, d); err != nil {
			t.Fatalf("merge of different entities: %v", err)
		}
	})
	t.Run("entity scope must name a known kind", func(t *testing.T) {
		ov := &Overlay{}
		bad := []*EntityScope{{Name: ""}, {Name: "X", Kind: "table"}}
		for _, e := range bad {
			if err := ov.Add(scopedOp(t, policyPointer, "x-a", `1`, e)); err == nil {
				t.Fatalf("accepted entity %+v", e)
			}
		}
	})
}

func TestEntityPartFormat(t *testing.T) {
	committed := readTestdata(t, "scoped.terraform_entity_overlay.yaml")
	ov, err := Parse(committed)
	if err != nil {
		t.Fatal(err)
	}
	if len(ov.Operations) != 1 || ov.Operations[0].Entity == nil ||
		*ov.Operations[0].Entity != (EntityScope{Name: customEntity, Kind: EntityKindResource}) {
		t.Fatalf("parsed = %+v", ov.Operations)
	}
	rendered, err := ov.Render(readTestdata(t, "spec.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if rendered.Global != nil {
		t.Fatalf("an overlay of entity-scoped operations has no global part:\n%s", rendered.Global)
	}
	if !bytes.Equal(rendered.Entities, committed) {
		t.Fatalf("entity part does not round-trip byte-for-byte:\n%s", rendered.Entities)
	}

	t.Run("global operations and entity operations go to separate parts", func(t *testing.T) {
		mixed := &Overlay{Source: "s", Kind: KindService}
		for _, op := range []Operation{
			scopedOp(t, policyPointer, "x-a", `1`, nil),
			scopedOp(t, policyPointer, "x-b", `1`, customResource()),
		} {
			if err := mixed.Add(op); err != nil {
				t.Fatal(err)
			}
		}
		out, err := mixed.Render(readTestdata(t, "spec.yaml"))
		if err != nil {
			t.Fatal(err)
		}
		if bytes.Contains(out.Global, []byte("x-apigw-entity")) || !bytes.Contains(out.Entities, []byte("x-apigw-entity")) {
			t.Fatalf("parts:\n%s\n---\n%s", out.Global, out.Entities)
		}
	})

	t.Run("committed example artifacts round-trip byte-for-byte", func(t *testing.T) {
		matches, err := filepath.Glob(filepath.Join("..", "example", "*", "v1", "*.terraform*overlay.yaml"))
		if err != nil || len(matches) == 0 {
			t.Fatalf("no committed overlay artifacts found: %v", err)
		}
		for _, path := range matches {
			committed, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			parsed, err := Parse(committed)
			if err != nil {
				t.Fatalf("%s: %v", path, err)
			}
			base, _, _ := strings.Cut(path, ".terraform")
			doc, err := os.ReadFile(base + ".oas31.yaml")
			if err != nil {
				t.Fatal(err)
			}
			again, err := parsed.Render(doc)
			if err != nil {
				t.Fatal(err)
			}
			part := again.Global
			if strings.HasSuffix(path, EntityArtifactSuffix) {
				part = again.Entities
			}
			if !bytes.Equal(committed, part) {
				t.Fatalf("%s is not byte-identical after parse and render", path)
			}
		}
	})

	t.Run("malformed entity parts are rejected", func(t *testing.T) {
		text := string(committed)
		for name, mutate := range map[string]func(string) string{
			"missing name": func(s string) string { return strings.Replace(s, "      name: Custom App Entitlement\n", "", 1) },
			"unknown field": func(s string) string {
				return strings.Replace(s, "kind: resource\n", "kind: resource\n      extra: x\n", 1)
			},
			"unknown kind": func(s string) string { return strings.Replace(s, "kind: resource", "kind: table", 1) },
			"empty kind":   func(s string) string { return strings.Replace(s, "kind: resource", "kind: ''", 1) },
			"not a mapping": func(s string) string {
				return strings.Replace(s, "x-apigw-entity:\n      kind: resource\n      name: Custom App Entitlement\n", "x-apigw-entity: Custom App Entitlement\n", 1)
			},
			"duplicate field": func(s string) string {
				return strings.Replace(s, "kind: resource\n", "kind: resource\n      kind: resource\n", 1)
			},
			"in the global part":          func(s string) string { return strings.Replace(s, "part: entities", "part: global", 1) },
			"wrong overlay version":       func(s string) string { return strings.Replace(s, "overlay: 1.1.0", "overlay: 1.0.0", 1) },
			"missing metadata":            func(s string) string { before, _, _ := strings.Cut(s, "x-apigw-terraform:"); return before },
			"unknown top-level extension": func(s string) string { return s + "x-other: 1\n" },
			"trailing document":           func(s string) string { return s + "---\n" + s },
		} {
			if _, err := Parse([]byte(mutate(text))); err == nil {
				t.Errorf("%s: Parse accepted the artifact", name)
			}
		}
	})
}
