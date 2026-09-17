package apigw

import (
	"bytes"
	"fmt"
	"go/format"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	pgs "github.com/lyft/protoc-gen-star"
	"github.com/pb33f/libopenapi"
	validator "github.com/pb33f/libopenapi-validator"
	v3 "github.com/pb33f/libopenapi/datamodel/high/v3"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protodesc"
	"google.golang.org/protobuf/reflect/protoregistry"
	"google.golang.org/protobuf/types/pluginpb"
	"gopkg.in/yaml.v3"

	apigw_v1 "github.com/ductone/protoc-gen-apigw/apigw/v1"
	// Blank imports register the example files in the global descriptor
	// registry, which TestDescriptorFixtureIsCurrent compares the committed
	// fixture against.
	_ "github.com/ductone/protoc-gen-apigw/example/bookstore/v1"
	_ "github.com/ductone/protoc-gen-apigw/example/tfcustomize/v1"
	"github.com/ductone/protoc-gen-apigw/tfoverlay"
)

const (
	bookstoreProto   = "bookstore/v1/bookstore.proto"
	tfcustomizeProto = "tfcustomize/v1/tfcustomize.proto"
)

// generatorFailure carries a generator-level failure out of pgs, whose default
// debugger exits the process. The module reports a bad annotation with
// ModuleBase.Fail, which in production terminates with a clean message; in a
// test it has to be catchable.
type generatorFailure struct{ msg string }

func (e generatorFailure) Error() string { return e.msg }

type captureDebugger struct {
	pgs.Debugger
}

func (d captureDebugger) Fail(v ...interface{}) { panic(generatorFailure{fmt.Sprint(v...)}) }

func (d captureDebugger) Failf(format string, v ...interface{}) {
	panic(generatorFailure{fmt.Sprintf(format, v...)})
}

func (d captureDebugger) CheckErr(err error, v ...interface{}) {
	if err != nil {
		panic(generatorFailure{fmt.Sprint(v...) + ": " + err.Error()})
	}
}

func (d captureDebugger) Assert(expr bool, v ...interface{}) {
	if !expr {
		panic(generatorFailure{fmt.Sprint(v...)})
	}
}

// renderProtoFile runs the real plugin over one of the committed example
// protos and returns the generated artifacts by name. It is the whole
// proto -> apigw -> artifact path, in process, with no toolchain required.
func renderProtoFile(t *testing.T, target string) map[string]string {
	t.Helper()
	artifacts, err := tryRenderProtoFile(t, target)
	if err != nil {
		t.Fatalf("generating %s: %v", target, err)
	}
	return artifacts
}

func tryRenderProtoFile(t *testing.T, target string) (map[string]string, error) {
	artifacts, err := renderOrPanic(t, target)
	if err != nil {
		return nil, err
	}
	return artifacts, nil
}

// renderOrPanic runs the generator and turns a generatorFailure panic (the
// module reports a bad annotation with ModuleBase.Fail, which exits the process
// in production) into an error.
func renderOrPanic(t *testing.T, target string) (map[string]string, error) {
	var (
		artifacts map[string]string
		err       error
	)
	func() {
		defer func() {
			r := recover()
			if r == nil {
				return
			}
			failure, ok := r.(generatorFailure)
			if !ok {
				panic(r)
			}
			artifacts, err = nil, failure
		}()
		artifacts, err = renderArtifacts(t, target)
	}()
	if err != nil {
		return nil, err
	}
	return artifacts, nil
}

func renderArtifacts(t *testing.T, target string) (map[string]string, error) {
	fixtures := loadDescriptorFixture(t)
	req := &pluginpb.CodeGeneratorRequest{
		FileToGenerate: []string{target},
		Parameter:      proto.String("paths=source_relative"),
	}
	seen := map[string]bool{}
	var walk func(name string) error
	walk = func(name string) error {
		if seen[name] {
			return nil
		}
		seen[name] = true
		if fdp, ok := fixtures[name]; ok {
			for _, dep := range fdp.GetDependency() {
				if err := walk(dep); err != nil {
					return err
				}
			}
			req.ProtoFile = append(req.ProtoFile, fdp)
			return nil
		}
		fd, err := protoregistry.GlobalFiles.FindFileByPath(name)
		if err != nil {
			return fmt.Errorf("resolving dependency %s: %w", name, err)
		}
		imports := fd.Imports()
		for i := 0; i < imports.Len(); i++ {
			if err := walk(imports.Get(i).FileDescriptor.Path()); err != nil {
				return err
			}
		}
		req.ProtoFile = append(req.ProtoFile, protodesc.ToFileDescriptorProto(fd))
		return nil
	}
	if err := walk(target); err != nil {
		return nil, err
	}

	payload, err := proto.Marshal(req)
	if err != nil {
		return nil, err
	}
	out := &bytes.Buffer{}
	g := pgs.Init(pgs.ProtocInput(bytes.NewReader(payload)), pgs.ProtocOutput(out))
	g.RegisterModule(New())
	g.Debugger = captureDebugger{Debugger: g.Debugger}
	g.Render()

	resp := &pluginpb.CodeGeneratorResponse{}
	if err := proto.Unmarshal(out.Bytes(), resp); err != nil {
		return nil, fmt.Errorf("decoding CodeGeneratorResponse: %w", err)
	}
	if e := resp.GetError(); e != "" {
		return nil, fmt.Errorf("generator reported: %s", e)
	}
	artifacts := map[string]string{}
	for _, f := range resp.GetFile() {
		artifacts[f.GetName()] = f.GetContent()
	}
	return artifacts, nil
}

func artifactEndingWith(t *testing.T, artifacts map[string]string, suffix string) (string, string) {
	t.Helper()
	names := make([]string, 0, len(artifacts))
	for name := range artifacts {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		if strings.HasSuffix(name, suffix) {
			return name, artifacts[name]
		}
	}
	t.Fatalf("no artifact ending with %q in %v", suffix, names)
	return "", ""
}

func byteIdentical(t *testing.T, want, got []byte, context string) {
	t.Helper()
	if !bytes.Equal(want, got) {
		t.Fatalf("%s is not byte-identical (want %d bytes, got %d bytes):\n--- want\n%s\n--- got\n%s",
			context, len(want), len(got), want, got)
	}
}

// ---------------------------------------------------------------------------
// annotation validation
// ---------------------------------------------------------------------------

func expectDeclarationFailure(t *testing.T, collect func(*tfCollector)) string {
	t.Helper()
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("expected a declaration failure, got panic %v", r)
		}
	}()
	var captured string
	func() {
		defer func() {
			if r := recover(); r != nil {
				if te, ok := r.(tfError); ok {
					captured = te.msg
					return
				}
				panic(r)
			}
		}()
		collect(newTFCollector())
	}()
	if captured == "" {
		t.Fatal("expected a declaration failure, got none")
	}
	return captured
}

func TestCustomizationDeclarationValidation(t *testing.T) {
	tests := []struct {
		name    string
		cust    *apigw_v1.OpenAPICustomization
		kind    tfOwnerKind
		wantErr string
	}{
		{
			name:    "key must be a vendor extension",
			cust:    &apigw_v1.OpenAPICustomization{Key: "default", ValueJson: `1`},
			kind:    tfOwnerField,
			wantErr: `must start with "x-"`,
		},
		{
			name:    "value is required for set",
			cust:    &apigw_v1.OpenAPICustomization{Key: "x-a"},
			kind:    tfOwnerField,
			wantErr: "value_json is required",
		},
		{
			name:    "malformed json is rejected",
			cust:    &apigw_v1.OpenAPICustomization{Key: "x-a", ValueJson: `{`},
			kind:    tfOwnerField,
			wantErr: "parsing JSON object",
		},
		{
			name:    "duplicate keys are rejected",
			cust:    &apigw_v1.OpenAPICustomization{Key: "x-a", ValueJson: `{"a":1,"a":2}`},
			kind:    tfOwnerField,
			wantErr: `duplicate object key "a"`,
		},
		{
			name:    "value must be empty for remove",
			cust:    &apigw_v1.OpenAPICustomization{Key: "x-a", ValueJson: `1`, Mode: apigw_v1.OpenAPICustomizationMode_OPEN_API_CUSTOMIZATION_MODE_REMOVE},
			kind:    tfOwnerField,
			wantErr: "must be empty when mode is REMOVE",
		},
		{
			name:    "json_pointer needs the pointer target",
			cust:    &apigw_v1.OpenAPICustomization{Key: "x-a", ValueJson: `1`, JsonPointer: "/a"},
			kind:    tfOwnerField,
			wantErr: "json_pointer is only valid with target JSON_POINTER",
		},
		{
			name:    "malformed json_pointer is rejected",
			cust:    &apigw_v1.OpenAPICustomization{Key: "x-a", ValueJson: `1`, Target: apigw_v1.OpenAPICustomizationTarget_OPEN_API_CUSTOMIZATION_TARGET_JSON_POINTER, JsonPointer: "a"},
			kind:    tfOwnerField,
			wantErr: "invalid JSON Pointer",
		},
		{
			name:    "target must be legal for the owner",
			cust:    &apigw_v1.OpenAPICustomization{Key: "x-a", ValueJson: `1`, Target: apigw_v1.OpenAPICustomizationTarget_OPEN_API_CUSTOMIZATION_TARGET_DOCUMENT_ROOT},
			kind:    tfOwnerField,
			wantErr: "is not valid on a field",
		},
		{
			name:    "operation target is illegal on a message",
			cust:    &apigw_v1.OpenAPICustomization{Key: "x-a", ValueJson: `1`, Target: apigw_v1.OpenAPICustomizationTarget_OPEN_API_CUSTOMIZATION_TARGET_OPERATION},
			kind:    tfOwnerMessage,
			wantErr: "is not valid on a message",
		},
		{
			name:    "array items target is legal on a field",
			cust:    &apigw_v1.OpenAPICustomization{Key: "x-a", ValueJson: `1`, Target: apigw_v1.OpenAPICustomizationTarget_OPEN_API_CUSTOMIZATION_TARGET_ARRAY_ITEMS},
			kind:    tfOwnerField,
			wantErr: "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			collect := func(c *tfCollector) {
				c.declare(c.parseCustomization("field test.Field.f", tt.kind, "field", 0, tt.cust))
			}
			if tt.wantErr == "" {
				collect(newTFCollector())
				return
			}
			got := expectDeclarationFailure(t, collect)
			if !strings.Contains(got, tt.wantErr) {
				t.Fatalf("failure %q does not contain %q", got, tt.wantErr)
			}
		})
	}
}

func TestSchemaPatchDeclarationValidation(t *testing.T) {
	tests := []struct {
		name    string
		patch   *apigw_v1.OpenAPISchemaPatch
		wantErr string
	}{
		{
			name:    "empty patch",
			patch:   &apigw_v1.OpenAPISchemaPatch{},
			wantErr: "needs patch_json or remove_keys",
		},
		{
			name:    "vendor extension in the patch channel",
			patch:   &apigw_v1.OpenAPISchemaPatch{PatchJson: `{"x-speakeasy-name-override":"a"}`},
			wantErr: "use customizations instead",
		},
		{
			name:    "vendor extension in remove_keys",
			patch:   &apigw_v1.OpenAPISchemaPatch{RemoveKeys: []string{"x-speakeasy-name-override"}},
			wantErr: "use customizations with mode REMOVE instead",
		},
		{
			name:    "patch_json must be an object",
			patch:   &apigw_v1.OpenAPISchemaPatch{PatchJson: `[1]`},
			wantErr: "must be a JSON object",
		},
		{
			name:    "malformed patch_json",
			patch:   &apigw_v1.OpenAPISchemaPatch{PatchJson: `{`},
			wantErr: "parsing JSON object",
		},
		{
			name:    "parameter target is not a schema",
			patch:   &apigw_v1.OpenAPISchemaPatch{PatchJson: `{"default":1}`, Target: apigw_v1.OpenAPICustomizationTarget_OPEN_API_CUSTOMIZATION_TARGET_PARAMETER},
			wantErr: "not valid for a schema patch",
		},
		{
			name:    "empty keyword",
			patch:   &apigw_v1.OpenAPISchemaPatch{RemoveKeys: []string{""}},
			wantErr: "empty keyword",
		},
		{
			name:    "legal patch",
			patch:   &apigw_v1.OpenAPISchemaPatch{PatchJson: `{"default":[]}`, RemoveKeys: []string{"format"}},
			wantErr: "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			collect := func(c *tfCollector) {
				c.declare(c.parsePatch("field test.Field.f", tfOwnerField, "fieldp", 0, tt.patch))
			}
			if tt.wantErr == "" {
				collect(newTFCollector())
				return
			}
			got := expectDeclarationFailure(t, collect)
			if !strings.Contains(got, tt.wantErr) {
				t.Fatalf("failure %q does not contain %q", got, tt.wantErr)
			}
		})
	}
}

// TestUnappliedAnnotationsFailGeneration pins the contract that an annotation
// which no emitted document applied is an error, not a silent no-op: without
// this a typo'd annotation looks like success and silently changes nothing.
func TestUnappliedAnnotationsFailGeneration(t *testing.T) {
	collector := newTFCollector()
	collector.declare(&tfDecl{
		id:         "field:test.Field.f#0",
		descriptor: "field test.Field.f",
		describe:   "field test.Field.f annotation \"x-a\"",
		ownerKind:  tfOwnerField,
		target:     apigw_v1.OpenAPICustomizationTarget_OPEN_API_CUSTOMIZATION_TARGET_SCHEMA,
		scope:      apigw_v1.OpenAPICustomizationScope_OPEN_API_CUSTOMIZATION_SCOPE_TERRAFORM,
		key:        "x-a",
		value:      &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!int", Value: "1"},
	})
	err := collector.verify()
	if err == nil {
		t.Fatal("verify() = nil, want an unapplied-annotation failure")
	}
	if !strings.Contains(err.Error(), "1 annotation(s) could not be applied") {
		t.Fatalf("unexpected failure: %v", err)
	}

	collector.decls["field:test.Field.f#0"].applied = true
	if err := collector.verify(); err != nil {
		t.Fatalf("verify() after applying = %v, want nil", err)
	}
}

func TestUnappliedAnnotationReportsReason(t *testing.T) {
	collector := newTFCollector()
	field := &tfDecl{
		id:         "field:test.Field.f#0",
		descriptor: "field test.Field.f",
		describe:   "field test.Field.f annotation \"x-a\"",
		ownerKind:  tfOwnerField,
		target:     apigw_v1.OpenAPICustomizationTarget_OPEN_API_CUSTOMIZATION_TARGET_MAP_VALUES,
		scope:      apigw_v1.OpenAPICustomizationScope_OPEN_API_CUSTOMIZATION_SCOPE_TERRAFORM,
		key:        "x-a",
		value:      &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!int", Value: "1"},
	}
	collector.declare(field)
	collector.blockReason(field.id, "field is not a map")
	err := collector.verify()
	if err == nil {
		t.Fatal("verify() = nil, want a failure")
	}
	if !strings.Contains(err.Error(), "field is not a map") {
		t.Fatalf("failure should carry the reason, got: %v", err)
	}
}

// ---------------------------------------------------------------------------
// end to end transport
// ---------------------------------------------------------------------------

// expectedFixtureOperations is the complete, reviewed operation list the
// tfcustomize fixture must produce. It is a golden: a change in what the
// generator emits for a documented destination has to be a deliberate edit
// here.
var expectedFixtureOperations = map[string]string{
	"|extension|x-speakeasy-globals":                                                                                      "{parameters=[{in=query,name=tenant}]}",
	"|extension|x-speakeasy-max-method-params":                                                                            "7",
	"|extension|x-speakeasy-model-namespace":                                                                              "tfcustomize",
	"/components/schemas/tfcustomize.v1.Thing|extension|x-future-decimal":                                                 "1.50",
	"/components/schemas/tfcustomize.v1.Thing|extension|x-future-empty-array":                                             "[]",
	"/components/schemas/tfcustomize.v1.Thing|extension|x-future-empty-object":                                            "{}",
	"/components/schemas/tfcustomize.v1.Thing|extension|x-future-empty-string":                                            "",
	"/components/schemas/tfcustomize.v1.Thing|extension|x-future-false":                                                   "false",
	"/components/schemas/tfcustomize.v1.Thing|extension|x-future-int":                                                     "9007199254740993",
	"/components/schemas/tfcustomize.v1.Thing|extension|x-future-multiline":                                               "line one\nline two",
	"/components/schemas/tfcustomize.v1.Thing|extension|x-future-null":                                                    "null",
	"/components/schemas/tfcustomize.v1.Thing|extension|x-future-thing":                                                   "{nested={list=[1,2.50,null,false,x]}}",
	"/components/schemas/tfcustomize.v1.Thing|extension|x-speakeasy-entity-description":                                   "A **thing** resource.",
	"/components/schemas/tfcustomize.v1.Thing|schema|required":                                                            "[id]",
	"/components/schemas/tfcustomize.v1.Thing|schema|title":                                                               "",
	"/components/schemas/tfcustomize.v1.Thing/properties/child|extension|x-speakeasy-name-override":                       "child_thing",
	"/components/schemas/tfcustomize.v1.Thing/properties/createdAt|extension|x-speakeasy-param-computed":                  "false",
	"/components/schemas/tfcustomize.v1.Thing/properties/createdAt|schema|format":                                         "",
	"/components/schemas/tfcustomize.v1.Thing/properties/escaped|extension|x-speakeasy-docs":                              "{go=Escaped pointer target.}",
	"/components/schemas/tfcustomize.v1.Thing/properties/escaped|extension|x-speakeasy-param-source":                      "{kind=config,path=[a/b,c~d]}",
	"/components/schemas/tfcustomize.v1.Thing/properties/id|extension|x-speakeasy-name-override":                          "thing_identifier",
	"/components/schemas/tfcustomize.v1.Thing/properties/id|extension|x-speakeasy-param-readonly":                         "true",
	"/components/schemas/tfcustomize.v1.Thing/properties/id|schema|default":                                               "unknown",
	"/components/schemas/tfcustomize.v1.Thing/properties/id|schema|minLength":                                             "1",
	"/components/schemas/tfcustomize.v1.Thing/properties/labels/additionalProperties|extension|x-speakeasy-type-override": "any",
	"/components/schemas/tfcustomize.v1.Thing/properties/manualPolicy|extension|x-speakeasy-param-force-new":              "false",
	"/components/schemas/tfcustomize.v1.Thing/properties/odd~1name|extension|x-speakeasy-deprecation-message":             "use id",
	"/components/schemas/tfcustomize.v1.Thing/properties/status|extension|x-speakeasy-unknown-values":                     "disallow",
	"/components/schemas/tfcustomize.v1.Thing/properties/tags|extension|x-speakeasy-terraform-ignore":                     "false",
	"/components/schemas/tfcustomize.v1.Thing/properties/tags|schema|default":                                             "[]",
	"/components/schemas/tfcustomize.v1.Thing/properties/tags/items|extension|x-speakeasy-response-filter":                "true",
	"/paths/~1things/get|extension|x-speakeasy-pagination":                                                                "{inputs=[],outputs={},type=cursor}",
	"/paths/~1things/post|extension|x-speakeasy-ignore":                                                                   "false",
	"/paths/~1things/post|extension|x-speakeasy-polling":                                                                  "[{intervalSeconds=10,limitCount=5,name=wait}]",
	"/paths/~1things~1{thing_id}/get|extension|x-speakeasy-entity-missing-codes":                                          "[403,410]",
	"/paths/~1things~1{thing_id}/get|extension|x-speakeasy-entity-operation":                                              "{terraform-datasource=null,terraform-resource=Thing#read}",
	"/paths/~1things~1{thing_id}/get/parameters/0|extension|x-speakeasy-match":                                            "id",
	"/paths/~1things~1{thing_id}/get/parameters/0/schema|extension|x-speakeasy-param-readonly":                            "false",
	"/paths/~1things~1{thing_id}/get/parameters/1|extension|x-speakeasy-match":                                            "flavor",
}

func renderOpValue(n *yaml.Node) string {
	switch n.Kind {
	case yaml.DocumentNode:
		if len(n.Content) == 0 {
			return ""
		}
		return renderOpValue(n.Content[0])
	case yaml.AliasNode:
		if n.Alias != nil {
			return renderOpValue(n.Alias)
		}
		return n.Value
	case yaml.ScalarNode:
		return n.Value
	case yaml.SequenceNode:
		parts := make([]string, 0, len(n.Content))
		for _, c := range n.Content {
			parts = append(parts, renderOpValue(c))
		}
		return "[" + strings.Join(parts, ",") + "]"
	case yaml.MappingNode:
		parts := make([]string, 0, len(n.Content)/2)
		for i := 0; i+1 < len(n.Content); i += 2 {
			parts = append(parts, n.Content[i].Value+"="+renderOpValue(n.Content[i+1]))
		}
		return "{" + strings.Join(parts, ",") + "}"
	}
	return ""
}

func operationSet(t *testing.T, overlay *tfoverlay.Overlay) map[string]string {
	t.Helper()
	set := map[string]string{}
	for _, op := range overlay.Operations {
		key := op.Pointer + "|" + string(op.Channel) + "|" + op.Key
		if _, dup := set[key]; dup {
			t.Fatalf("duplicate operation %q", key)
		}
		value := ""
		if op.Value != nil {
			value = renderOpValue(op.Value)
		}
		set[key] = value
	}
	return set
}

// TestAnnotationTransportOverlay is the proto -> generator -> overlay test for
// every destination class the contract enumerates.
func TestAnnotationTransportOverlay(t *testing.T) {
	artifacts := renderProtoFile(t, tfcustomizeProto)

	overlayName, overlayYAML := artifactEndingWith(t, artifacts, ".terraform_overlay.yaml")
	if !strings.HasSuffix(overlayName, "tfcustomize.pb.customize_service.terraform_overlay.yaml") {
		t.Fatalf("unexpected overlay artifact name %q", overlayName)
	}
	overlay, err := tfoverlay.Parse([]byte(overlayYAML))
	if err != nil {
		t.Fatalf("parsing the emitted overlay: %v", err)
	}
	if overlay.Source != "tfcustomize.v1.CustomizeService" || overlay.Kind != tfoverlay.KindService {
		t.Fatalf("overlay identity = %q/%q", overlay.Source, overlay.Kind)
	}

	got := operationSet(t, overlay)
	for key, want := range expectedFixtureOperations {
		value, ok := got[key]
		if !ok {
			t.Errorf("missing operation %q", key)
			continue
		}
		if value != want {
			t.Errorf("operation %q = %q, want %q", key, value, want)
		}
	}
	for key := range got {
		if _, want := expectedFixtureOperations[key]; !want {
			t.Errorf("unexpected operation %q = %q", key, got[key])
		}
	}
	if len(got) != len(expectedFixtureOperations) {
		t.Fatalf("got %d operations, want %d", len(got), len(expectedFixtureOperations))
	}

	t.Run("a fabricated future key travels with no catalog entry", func(t *testing.T) {
		want := "{nested={list=[1,2.50,null,false,x]}}"
		key := "/components/schemas/tfcustomize.v1.Thing|extension|x-future-thing"
		if got[key] != want {
			t.Fatalf("x-future-thing = %q, want %q", got[key], want)
		}
	})

	t.Run("shared scope is absent from the overlay", func(t *testing.T) {
		for key := range got {
			if strings.Contains(key, "x-speakeasy-enum-format") {
				t.Fatalf("shared-scoped operation leaked into the Terraform overlay: %q", key)
			}
		}
	})

	t.Run("the shared document carries the shared-scoped operation", func(t *testing.T) {
		_, oas := artifactEndingWith(t, artifacts, ".oas31.yaml")
		extensions := schemaPropertyExtensions(t, []byte(oas), "tfcustomize.v1.Thing", "status")
		if extensions["x-speakeasy-enum-format"] != "union" {
			t.Fatalf("shared extension = %q, want %q", extensions["x-speakeasy-enum-format"], "union")
		}
	})

	t.Run("terraform-scoped operations never touch the shared document", func(t *testing.T) {
		_, oas := artifactEndingWith(t, artifacts, ".oas31.yaml")
		extensions := schemaPropertyExtensions(t, []byte(oas), "tfcustomize.v1.Thing", "id")
		if _, present := extensions["x-speakeasy-name-override"]; present {
			t.Fatalf("terraform-scoped extension was written to the shared document: %v", extensions)
		}
	})

	t.Run("generation is deterministic", func(t *testing.T) {
		again := renderProtoFile(t, tfcustomizeProto)
		if len(again) != len(artifacts) {
			t.Fatalf("artifact count changed between runs: %d vs %d", len(artifacts), len(again))
		}
		for name, content := range artifacts {
			byteIdentical(t, []byte(content), []byte(again[name]), "repeat generation of "+name)
		}
	})
}

// TestOverlayAppliesToTheEmittedDocument proves the artifact is applicable and
// that applying it produces a valid document with the intended effective
// values — precedence over a generated typed annotation included.
func TestOverlayAppliesToTheEmittedDocument(t *testing.T) {
	artifacts := renderProtoFile(t, tfcustomizeProto)
	_, oas := artifactEndingWith(t, artifacts, ".oas31.yaml")
	_, overlayYAML := artifactEndingWith(t, artifacts, ".terraform_overlay.yaml")
	overlay, err := tfoverlay.Parse([]byte(overlayYAML))
	if err != nil {
		t.Fatalf("parsing the emitted overlay: %v", err)
	}

	const getThing = "/things/{thing_id}"
	baseline := operationExtension(t, []byte(oas), getThing, "x-speakeasy-entity-missing-codes")
	if !strings.Contains(baseline, "404") {
		t.Fatalf("baseline x-speakeasy-entity-missing-codes = %q, want the generated value", baseline)
	}

	applied, err := tfoverlay.Apply([]byte(oas), overlay.Operations)
	if err != nil {
		t.Fatalf("applying the emitted overlay: %v", err)
	}

	t.Run("an explicit override replaces the generated value", func(t *testing.T) {
		got := operationExtension(t, applied, getThing, "x-speakeasy-entity-missing-codes")
		if got != "[403,410]" {
			t.Fatalf("effective x-speakeasy-entity-missing-codes = %q, want %q", got, "[403,410]")
		}
	})

	t.Run("the effective document is valid OpenAPI", func(t *testing.T) {
		document, err := libopenapi.NewDocument(applied)
		if err != nil {
			t.Fatalf("applied document does not parse: %v", err)
		}
		v, errs := validator.NewValidator(document)
		if len(errs) > 0 {
			t.Fatalf("validator construction: %v", errs)
		}
		ok, verrs := v.ValidateDocument()
		if !ok {
			t.Fatalf("applied document is not valid OpenAPI: %v", verrs)
		}
	})

	t.Run("value shapes survive application", func(t *testing.T) {
		v3Model, err := openapiV3(t, applied)
		if err != nil {
			t.Fatal(err)
		}
		thing, ok := v3Model.Model.Components.Schemas.Get("tfcustomize.v1.Thing")
		if !ok {
			t.Fatal("Thing component is missing from the applied document")
		}
		schema := thing.Schema()
		for key, want := range map[string]string{
			"x-future-int":          "9007199254740993",
			"x-future-decimal":      "1.50",
			"x-future-false":        "false",
			"x-future-null":         "null",
			"x-future-empty-string": "",
			"x-future-multiline":    "line one\nline two",
		} {
			node, ok := schema.Extensions.Get(key)
			if !ok {
				t.Errorf("extension %q is missing after application", key)
				continue
			}
			if node.Value != want {
				t.Errorf("extension %q = %q, want %q", key, node.Value, want)
			}
		}
		for _, key := range []string{"x-future-empty-array", "x-future-empty-object"} {
			node, ok := schema.Extensions.Get(key)
			if !ok {
				t.Errorf("extension %q is missing after application", key)
				continue
			}
			if node.Kind != yaml.SequenceNode && node.Kind != yaml.MappingNode {
				t.Errorf("extension %q has kind %v, want a sequence or mapping", key, node.Kind)
			}
			if len(node.Content) != 0 {
				t.Errorf("extension %q = %v, want an empty collection", key, node.Content)
			}
		}
	})

	t.Run("a schema keyword removal takes effect", func(t *testing.T) {
		v3Model, err := openapiV3(t, applied)
		if err != nil {
			t.Fatal(err)
		}
		thing, ok := v3Model.Model.Components.Schemas.Get("tfcustomize.v1.Thing")
		if !ok {
			t.Fatal("Thing component is missing from the applied document")
		}
		if thing.Schema().Title != "" {
			t.Fatalf("title survived removal: %q", thing.Schema().Title)
		}
	})
}

func schemaPropertyExtensions(t *testing.T, oas []byte, component, property string) map[string]string {
	t.Helper()
	v3, err := openapiV3(t, oas)
	if err != nil {
		t.Fatal(err)
	}
	schema, ok := v3.Model.Components.Schemas.Get(component)
	if !ok {
		t.Fatalf("component %q is missing", component)
	}
	prop, ok := schema.Schema().Properties.Get(property)
	if !ok {
		t.Fatalf("property %q is missing from %q", property, component)
	}
	extensions := map[string]string{}
	for pair := prop.Schema().Extensions.Oldest(); pair != nil; pair = pair.Next() {
		extensions[pair.Key] = pair.Value.Value
	}
	return extensions
}

func operationExtension(t *testing.T, oas []byte, route, key string) string {
	t.Helper()
	v3, err := openapiV3(t, oas)
	if err != nil {
		t.Fatal(err)
	}
	item, ok := v3.Model.Paths.PathItems.Get(route)
	if !ok {
		t.Fatalf("route %q is missing", route)
	}
	if item.Get == nil {
		t.Fatalf("route %q has no GET operation", route)
	}
	node, ok := item.Get.Extensions.Get(key)
	if !ok {
		return ""
	}
	return renderOpValue(node)
}

func openapiV3(t *testing.T, oas []byte) (*libopenapi.DocumentModel[v3.Document], error) {
	t.Helper()
	model, err := libopenapi.NewDocument(oas)
	if err != nil {
		return nil, fmt.Errorf("parsing document: %w", err)
	}
	v3Model, errs := model.BuildV3Model()
	if len(errs) > 0 {
		return nil, fmt.Errorf("building v3 model: %v", errs)
	}
	return v3Model, nil
}

// ---------------------------------------------------------------------------
// absence of annotations leaves output unchanged
// ---------------------------------------------------------------------------

// TestNoAnnotationsLeaveOutputUnchanged is the compatibility requirement of
// this change: a document with no customizations must be byte-identical to the
// committed artifact, and must not gain an overlay file.
func TestNoAnnotationsLeaveOutputUnchanged(t *testing.T) {
	artifacts := renderProtoFile(t, bookstoreProto)

	for _, name := range []string{
		"bookstore/v1/bookstore.pb.bookstore_service.oas31.yaml",
		"bookstore/v1/bookstore.pb.apigw.go",
	} {
		content, ok := artifacts[name]
		if !ok {
			t.Fatalf("artifact %q is missing from %v", name, artifactNames(artifacts))
		}
		committed, err := os.ReadFile(filepath.Join("..", "..", "example", name))
		if err != nil {
			t.Fatalf("reading committed %q: %v", name, err)
		}
		if strings.HasSuffix(name, ".go") {
			// The committed Go has been through buf's Go post-processing, which
			// formats the file and adds the imports the plugin's header
			// template does not emit (it references context.Context but never
			// lists "context"). Compare the emitted body instead of the
			// import block, which this change does not touch.
			content = goBody(t, content)
			committed = []byte(goBody(t, string(committed)))
		}
		byteIdentical(t, committed, []byte(content), "generated "+name)
	}

	for name := range artifacts {
		if strings.HasSuffix(name, tfOverlayArtifactSuffix) {
			t.Fatalf("a document with no annotations produced an overlay artifact: %q", name)
		}
	}

	t.Run("repeat generation is identical", func(t *testing.T) {
		again := renderProtoFile(t, bookstoreProto)
		for name, content := range artifacts {
			byteIdentical(t, []byte(content), []byte(again[name]), "repeat generation of "+name)
		}
	})
}

// goBody formats Go source and drops the import block.
func goBody(t *testing.T, src string) string {
	t.Helper()
	formatted, err := format.Source([]byte(src))
	if err != nil {
		t.Fatalf("formatting generated Go: %v", err)
	}
	out := string(formatted)
	start := strings.Index(out, "import (")
	if start < 0 {
		return out
	}
	end := strings.Index(out[start:], "\n)\n")
	if end < 0 {
		return out
	}
	return out[:start] + out[start+end:]
}

func artifactNames(artifacts map[string]string) []string {
	names := make([]string, 0, len(artifacts))
	for name := range artifacts {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// TestEmbeddedFieldExtensionsSurvive pins the fix for the metadata that used to
// be dropped when a field's schema was emitted through a message reference.
func TestEmbeddedFieldExtensionsSurvive(t *testing.T) {
	artifacts := renderProtoFile(t, tfcustomizeProto)
	_, oas := artifactEndingWith(t, artifacts, ".oas31.yaml")

	tests := []struct {
		name      string
		property  string
		extension string
		want      string
	}{
		{
			// Singular message field: emitted as a nullable $ref wrapper.
			name:      "nullable reference wrapper",
			property:  "child",
			extension: "x-stability-level",
			want:      "stable",
		},
		{
			// Well-known type: emitted inline rather than as a $ref.
			name:      "inline well-known type",
			property:  "createdAt",
			extension: "x-stability-level",
			want:      "stable",
		},
		{
			// Scalar: already worked, kept as the control.
			name:      "scalar control",
			property:  "id",
			extension: "x-stability-level",
			want:      "stable",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			extensions := schemaPropertyExtensions(t, []byte(oas), "tfcustomize.v1.Thing", tt.property)
			if got := extensions[tt.extension]; got != tt.want {
				t.Fatalf("%s on %s = %q, want %q", tt.extension, tt.property, got, tt.want)
			}
		})
	}

	t.Run("field metadata does not leak onto the referenced component", func(t *testing.T) {
		v3, err := openapiV3(t, []byte(oas))
		if err != nil {
			t.Fatal(err)
		}
		thing, ok := v3.Model.Components.Schemas.Get("tfcustomize.v1.Thing")
		if !ok {
			t.Fatal("Thing component is missing")
		}
		if _, present := thing.Schema().Extensions.Get("x-stability-level"); present {
			t.Fatal("the referenced component picked up the field's stability extension")
		}
	})
}
