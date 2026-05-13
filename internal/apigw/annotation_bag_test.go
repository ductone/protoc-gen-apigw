package apigw

import (
	"strings"
	"testing"

	"github.com/pb33f/libopenapi/orderedmap"
	"gopkg.in/yaml.v3"

	apigw_v1 "github.com/ductone/protoc-gen-apigw/apigw/v1"
)

func TestGetAnnotationBagExtraction(t *testing.T) {
	tests := []struct {
		name         string
		fieldOptions []*apigw_v1.FieldOption
		want         bool
	}{
		{
			name:         "no field options",
			fieldOptions: nil,
			want:         false,
		},
		{
			name:         "empty field options",
			fieldOptions: []*apigw_v1.FieldOption{},
			want:         false,
		},
		{
			name: "single option with annotation_bag=true",
			fieldOptions: []*apigw_v1.FieldOption{
				{AnnotationBag: true},
			},
			want: true,
		},
		{
			name: "single option with annotation_bag=false",
			fieldOptions: []*apigw_v1.FieldOption{
				{AnnotationBag: false},
			},
			want: false,
		},
		{
			name: "annotation_bag set on second option",
			fieldOptions: []*apigw_v1.FieldOption{
				{RequiredSpec: true},
				{AnnotationBag: true},
			},
			want: true,
		},
		{
			name: "annotation_bag mixed with other options",
			fieldOptions: []*apigw_v1.FieldOption{
				{
					AnnotationBag: true,
					RequiredSpec:  true,
				},
			},
			want: true,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			var got bool
			for _, fo := range tt.fieldOptions {
				if fo.GetAnnotationBag() {
					got = true
					break
				}
			}
			if got != tt.want {
				t.Errorf("got %v, want %v", got, tt.want)
			}
		})
	}
}

func TestBuildAnnotationBagExtensionNode(t *testing.T) {
	node := buildAnnotationBagExtensionNode()
	if node.Kind != yaml.MappingNode {
		t.Fatalf("expected MappingNode, got Kind=%d", node.Kind)
	}

	var buf strings.Builder
	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(2)
	if err := enc.Encode(node); err != nil {
		t.Fatalf("yaml encode: %v", err)
	}
	_ = enc.Close()
	got := buf.String()

	want := []string{
		"imports:",
		annotationBagPlanModifierImport,
		"schemaDefinition:",
		annotationBagPlanModifierExpr,
	}
	for _, w := range want {
		if !strings.Contains(got, w) {
			t.Errorf("rendered YAML missing %q\n--- got ---\n%s", w, got)
		}
	}
}

func TestExtensionAccumulatorAcceptsAnnotationBag(t *testing.T) {
	exts := orderedmap.New[string, *yaml.Node]()
	exts.Set(annotationBagExtensionKey, buildAnnotationBagExtensionNode())

	v, ok := exts.Get(annotationBagExtensionKey)
	if !ok {
		t.Fatalf("expected key %q in extensions, not found", annotationBagExtensionKey)
	}
	if v.Kind != yaml.MappingNode {
		t.Fatalf("expected MappingNode, got Kind=%d", v.Kind)
	}
}

// Pin the public-facing constants — changing them is a coordinated cross-repo rename.
func TestAnnotationBagConstants(t *testing.T) {
	if annotationBagPlanModifierImport == "" {
		t.Error("annotationBagPlanModifierImport must not be empty")
	}
	if annotationBagPlanModifierExpr == "" {
		t.Error("annotationBagPlanModifierExpr must not be empty")
	}
	if annotationBagExtensionKey != "x-speakeasy-terraform-plan-modifier" {
		t.Errorf("annotationBagExtensionKey changed: got %q, want %q",
			annotationBagExtensionKey, "x-speakeasy-terraform-plan-modifier")
	}
	if !strings.HasPrefix(annotationBagPlanModifierImport, "github.com/") {
		t.Errorf("import path should start with github.com/: %q", annotationBagPlanModifierImport)
	}
	if !strings.Contains(annotationBagPlanModifierExpr, "(") || !strings.HasSuffix(annotationBagPlanModifierExpr, ")") {
		t.Errorf("expr should be a function call: %q", annotationBagPlanModifierExpr)
	}
}
