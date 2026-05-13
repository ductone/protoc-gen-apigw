package apigw

import (
	"fmt"
	"os"

	pgs "github.com/lyft/protoc-gen-star"
	"github.com/pb33f/libopenapi/orderedmap"
	"gopkg.in/yaml.v3"
)

// Renaming the helper in the C1 TF provider requires changing these constants.
const (
	annotationBagPlanModifierImport = "github.com/conductorone/terraform-provider-conductorone/internal/annotations"
	annotationBagPlanModifierExpr   = "annotations.PlanModifier()"
	annotationBagExtensionKey       = "x-speakeasy-terraform-plan-modifier"
)

func getAnnotationBag(f pgs.Field) bool {
	for _, fo := range getFieldOptions(f) {
		if fo.GetAnnotationBag() {
			return true
		}
	}
	return false
}

func fieldIsStringStringMap(f pgs.Field) bool {
	ft := f.Type()
	if !ft.IsMap() {
		return false
	}
	if ft.Key().ProtoType() != pgs.StringT {
		return false
	}
	return ft.Element().ProtoType() == pgs.StringT
}

// Logs a warning and skips emission when the field is not map<string, string>.
func addAnnotationBagExtension(extensions *orderedmap.Map[string, *yaml.Node], f pgs.Field) {
	if !getAnnotationBag(f) {
		return
	}
	if !fieldIsStringStringMap(f) {
		_, _ = fmt.Fprintf(os.Stderr,
			"Warning: annotation_bag is only valid on map<string, string> fields; got '%s.%s' (skipping extension)\n",
			nicerFQN(f.Message()), f.Name().String())
		return
	}
	extensions.Set(annotationBagExtensionKey, buildAnnotationBagExtensionNode())
}

func buildAnnotationBagExtensionNode() *yaml.Node {
	importsSeq := &yaml.Node{
		Kind: yaml.SequenceNode,
		Tag:  seqTag,
		Content: []*yaml.Node{
			{Kind: yaml.ScalarNode, Tag: stringTag, Value: annotationBagPlanModifierImport},
		},
	}
	return &yaml.Node{
		Kind: yaml.MappingNode,
		Content: []*yaml.Node{
			{Kind: yaml.ScalarNode, Tag: stringTag, Value: "imports"},
			importsSeq,
			{Kind: yaml.ScalarNode, Tag: stringTag, Value: "schemaDefinition"},
			{Kind: yaml.ScalarNode, Tag: stringTag, Value: annotationBagPlanModifierExpr},
		},
	}
}
