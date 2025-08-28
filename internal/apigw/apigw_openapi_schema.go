package apigw

import (
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"

	"github.com/fatih/camelcase"
	pgs "github.com/lyft/protoc-gen-star"
	dm_base "github.com/pb33f/libopenapi/datamodel/high/base"
	"github.com/pb33f/libopenapi/orderedmap"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
	"gopkg.in/yaml.v3"

	apigw_v1 "github.com/ductone/protoc-gen-apigw/apigw/v1"
)

var (
	caser = cases.Title(language.AmericanEnglish)
	space = regexp.MustCompile(`\s+`)
)

const FieldMaskWKT pgs.WellKnownType = "FieldMask"

func newSchemaContainer() *schemaContainer {
	return &schemaContainer{
		schemas: orderedmap.New[string, *dm_base.SchemaProxy](),
	}
}

type schemaContainer struct {
	schemas *orderedmap.Map[string, *dm_base.SchemaProxy]
}

func IsWellKnown(m pgs.Message) bool {
	if m.IsWellKnown() {
		return true
	}

	if string(m.Name()) == string(FieldMaskWKT) {
		return true
	}

	return false
}
func WellKnownType(m pgs.Message) pgs.WellKnownType {
	if m.IsWellKnown() {
		return m.WellKnownType()
	}

	if string(m.Name()) == string(FieldMaskWKT) {
		return FieldMaskWKT
	}

	return pgs.UnknownWKT
}

func (sc *schemaContainer) messageDocName(title string, m pgs.Message) string {
	if title != "" {
		return title
	}

	return transformName(m.Name())
}

func transformName(name pgs.Name) string {
	ret := name.Transform(caser.String, caser.String, " ").String()
	ret = strings.Join(camelcase.Split(ret), " ")
	ret = space.ReplaceAllString(ret, " ")
	return ret
}

func yamlString(in string) *yaml.Node {
	rv := &yaml.Node{}
	rv.SetString(in)
	return rv
}

func yamlBool(in bool) *yaml.Node {
	return &yaml.Node{
		Kind:  yaml.ScalarNode,
		Tag:   "!!bool",
		Value: strconv.FormatBool(in),
	}
}

func yamlStringSlice(in []string) *yaml.Node {
	inner := make([]*yaml.Node, 0, len(in))
	for _, s := range in {
		inner = append(inner, yamlString(s))
	}
	rv := &yaml.Node{
		Kind:    yaml.SequenceNode,
		Content: inner,
	}
	return rv
}

func (sc *schemaContainer) Message(m pgs.Message, filter []string, nullable *bool, readOnly bool, forced bool) *dm_base.SchemaProxy {
	if IsWellKnown(m) {
		// TODO(pquera): we may want to customize this some day,
		// but right now WKTs are just rendered inline, and not a Ref.
		s := sc.schemaForWKT(WellKnownType(m))
		s.ReadOnly = &readOnly
		s.Nullable = nullable
		return dm_base.CreateSchemaProxy(s)
	}

	terraformEntityName := ""
	terraformEntityJSON := false
	mopt := getMessageOptions(m)
	title := ""
	if mopt != nil {
		terraformEntity := mopt.GetTerraformEntity()
		if terraformEntity != nil {
			terraformEntityName = terraformEntity.Name
			terraformEntityJSON = terraformEntity.Json
		}
		title = mopt.GetTitle()
	}

	fqn := nicerFQN(m)
	if len(filter) != 0 {
		fqn += "Input"
	}

	if sc.schemas.Value(fqn) != nil {
		return dm_base.CreateSchemaProxyRef(SchemaProxyRefPrefix + fqn)
	}
	// This prevents infinite recursion when a message references itself.
	// This should be overwritten at the bottom by the object schema once it's resolved
	sc.schemas.Set(fqn, dm_base.CreateSchemaProxyRef(SchemaProxyRefPrefix+fqn))

	description := &strings.Builder{}
	comments := strings.TrimSpace(m.SourceCodeInfo().LeadingComments())
	if comments == "" {
		_, _ = fmt.Fprintf(description, "The %s message.", m.Name().String())
	} else {
		_, _ = description.WriteString(comments)
	}

	deprecated := oasBool(m.Descriptor().GetOptions().GetDeprecated())
	extensions := orderedmap.New[string, *yaml.Node]()
	extensions.Set("x-speakeasy-name-override", yamlString(m.Name().UpperCamelCase().String()))
	obj := &dm_base.Schema{
		Type:       []string{"object"},
		Properties: orderedmap.New[string, *dm_base.SchemaProxy](),
		Deprecated: deprecated,
		Title:      sc.messageDocName(title, m),

		Extensions: extensions,
	}
	if terraformEntityName != "" {
		extensions.Set("x-speakeasy-entity", yamlString(terraformEntityName))
	}
	if forced {
		extensions.Set("x-speakeasy-include", yamlBool(true))
	}
	if terraformEntityJSON {
		extensions.Set("x-speakeasy-type-override", yamlString("any"))
	}
	required := make([]string, 0)
	for _, f := range m.NonOneOfFields() {
		jn := jsonName(f)
		if len(filter) != 0 {
			if contains(f.Name().String(), filter) {
				continue
			}
			if contains(jn, filter) {
				continue
			}
		}

		if getRequiredSpec(f) {
			required = append(required, jn)
		}

		obj.Properties.Set(jn, sc.Field(f))
	}
	if len(required) > 0 {
		obj.Required = required
	}

	for _, of := range m.OneOfs() {
		_, _ = fmt.Fprintf(description,
			"\n\nThis message contains a oneof named %s. "+
				"Only a single field of the following list may be set at a time:\n",
			of.Name().String(),
		)

		for _, f := range of.Fields() {
			jn := jsonName(f)
			obj.Properties.Set(jn, sc.Field(f))
			_, _ = fmt.Fprintf(description, "  - %s\n", jn)
		}
	}
	obj.Description = description.String()
	rv := dm_base.CreateSchemaProxy(obj)
	sc.schemas.Set(fqn, rv)
	return dm_base.CreateSchemaProxyRef(SchemaProxyRefPrefix + fqn)
}

func (sc *schemaContainer) OneOf(of pgs.OneOf) []*dm_base.SchemaProxy {
	fields := of.Fields()
	rv := make([]*dm_base.SchemaProxy, 0, len(fields))
	for _, f := range fields {
		rv = append(rv, sc.Field(f))
	}
	return rv
}

func (sc *schemaContainer) Enum(e pgs.Enum) *dm_base.Schema {
	values := e.Values()
	enumValues := make([]*yaml.Node, 0, len(values))
	for _, v := range values {
		// TODO(pquerna): verify this is the right way to get the name
		// in the JSON format
		enumValues = append(enumValues, yamlString(v.Name().String()))
	}
	return &dm_base.Schema{
		Type: []string{"string"},
		Enum: enumValues,
	}
}

func (sc *schemaContainer) FieldTypeElem(fte pgs.FieldTypeElem, readOnly bool) *dm_base.SchemaProxy {
	switch {
	case fte.IsEmbed():
		return sc.Message(fte.Embed(), nil, nil, readOnly, false)
	case fte.IsEnum():
		ev := sc.Enum(fte.Enum())
		ev.Extensions = orderedmap.New[string, *yaml.Node]()
		ev.Extensions.Set("x-speakeasy-unknown-values", yamlString("allow"))
		return dm_base.CreateSchemaProxy(ev)
	default:
		return dm_base.CreateSchemaProxy(sc.schemaForScalar(fte.ProtoType()))
	}
}

func (sc *schemaContainer) Field(f pgs.Field) *dm_base.SchemaProxy {
	deprecated := oasBool(f.Descriptor().GetOptions().GetDeprecated())
	description := strings.TrimSpace(f.SourceCodeInfo().LeadingComments())
	readOnly := getReadOnlySpec(f)
	if description == "" {
		jn := jsonName(f)
		description = "The " + jn + " field."
	}
	var nullable *bool
	if f.OneOf() != nil {
		nullable = oasTrue()
		description += "\nThis field is part of the `" + f.OneOf().Name().String() + "` oneof.\n" +
			"See the documentation for `" + nicerFQN(f.Message()) + "` for more details."
	} else {
		// Use getNullableSpec to determine nullable based on required_spec
		nullable = getNullableSpec(f)
	}

	// Get field-level stability and deprecation info
	fieldStability := getFieldStability(f)
	fieldDeprecation := getFieldDeprecation(f)
	fieldTerraformField := getFieldTerraformField(f)

	// Validate field-level deprecation if present
	if fieldDeprecation != nil {
		if err := ValidateDeprecationInfo(fieldDeprecation); err != nil {
			// For now, we'll log the error but continue processing
			// In a production system, you might want to handle this differently
			// Log warning to stderr instead of stdout to avoid forbidden fmt.Printf
			_, _ = fmt.Fprintf(os.Stderr, "Warning: field deprecation validation failed for '%s.%s': %v\n",
				nicerFQN(f.Message()), f.Name().String(), err)
		}
		// If deprecation info is provided, the field is considered deprecated
		deprecated = oasTrue()
	}

	// Create extensions map for field-level annotations
	extensions := orderedmap.New[string, *yaml.Node]()
	// Add field-level stability extension
	addStabilityExtension(extensions, fieldStability)

	// Add field-level deprecation extension
	if fieldDeprecation != nil {
		addSunsetExtension(extensions, fieldDeprecation.SunsetDate)
	}

	// Add field-level terraform extension
	if fieldTerraformField != nil {
		addTerraformUpdateInPlaceExtension(extensions, fieldTerraformField.UpdateInPlace)
	}

	switch {
	case f.Type().IsRepeated():
		fteSchema := sc.FieldTypeElem(f.Type().Element(), readOnly)
		arraySchema := &dm_base.Schema{
			Type:        []string{"array"},
			Description: description,
			Nullable:    nullable,
			ReadOnly:    &readOnly,
			Deprecated:  deprecated,
			Items:       &dm_base.DynamicValue[*dm_base.SchemaProxy, bool]{A: fteSchema},
		}
		// Add extensions if any exist
		if extensions.Len() > 0 {
			arraySchema.Extensions = extensions
		}
		return dm_base.CreateSchemaProxy(arraySchema)
	case f.Type().IsMap():
		fteSchema := sc.FieldTypeElem(f.Type().Element(), readOnly)
		mv := &dm_base.Schema{
			Type:                 []string{"object"},
			Deprecated:           deprecated,
			Description:          description,
			Nullable:             nullable,
			ReadOnly:             &readOnly,
			AdditionalProperties: &dm_base.DynamicValue[*dm_base.SchemaProxy, bool]{A: fteSchema},
		}
		// Add extensions if any exist
		if extensions.Len() > 0 {
			mv.Extensions = extensions
		}
		return dm_base.CreateSchemaProxy(mv)
	case f.Type().IsEnum():
		ev := sc.Enum(f.Type().Enum())
		ev.Deprecated = deprecated
		ev.Description = description
		ev.ReadOnly = &readOnly
		if ev.Extensions == nil {
			ev.Extensions = orderedmap.New[string, *yaml.Node]()
		}
		// TODO: Make this optional instead of always true
		ev.Extensions.Set("x-speakeasy-unknown-values", yamlString("allow"))

		// Add field-level extensions
		for pair := extensions.Oldest(); pair != nil; pair = pair.Next() {
			ev.Extensions.Set(pair.Key, pair.Value)
		}

		mergeNullable(ev, nullable)
		return dm_base.CreateSchemaProxy(ev)
	case f.Type().IsEmbed():
		ref := sc.Message(f.Type().Embed(), nil, nil, readOnly, false)
		if nullable != nil && *nullable {
			// Emit oneOf with null type for nullable reference
			wrapper := &dm_base.Schema{
				OneOf: []*dm_base.SchemaProxy{
					ref,
					dm_base.CreateSchemaProxy(&dm_base.Schema{Type: []string{"null"}}),
				},
			}
			return dm_base.CreateSchemaProxy(wrapper)
		}
		return ref
	default:
		sv := sc.schemaForScalar(f.Type().ProtoType())
		sv.ReadOnly = &readOnly
		sv.Nullable = nullable
		sv.Deprecated = deprecated
		sv.Description = description
		// Add extensions if any exist
		if extensions.Len() > 0 {
			sv.Extensions = extensions
		}
		return dm_base.CreateSchemaProxy(sv)
	}
}

func getMessageOptions(m pgs.Message) *apigw_v1.MessageOption {
	mopt := &apigw_v1.MessageOptions{}
	_, err := m.Extension(apigw_v1.E_Message, mopt)
	if err != nil {
		return nil
	}
	if len(mopt.MessageOptions) > 0 {
		return mopt.MessageOptions[0]
	}
	return nil
}

func getFieldOptions(m pgs.Field) []*apigw_v1.FieldOption {
	fopt := &apigw_v1.FieldOptions{}
	_, err := m.Extension(apigw_v1.E_Field, fopt)
	if err != nil {
		return nil
	}
	return fopt.FieldOptions
}

func getRequiredSpec(f pgs.Field) bool {
	for _, fo := range getFieldOptions(f) {
		if fo.GetRequiredSpec() {
			return true
		}
	}
	return false
}

func getReadOnlySpec(f pgs.Field) bool {
	for _, fo := range getFieldOptions(f) {
		if fo.GetReadOnlySpec() {
			return true
		}
	}
	return false
}

func getNullableSpec(f pgs.Field) *bool {
	// If required_spec is false, then the field should be nullable
	// If required_spec is true, then the field should not be nullable (return nil to omit the field)
	required := getRequiredSpec(f)
	if !required {
		return oasTrue()
	}
	return nil
}

func getFieldStability(f pgs.Field) apigw_v1.Stability {
	for _, fo := range getFieldOptions(f) {
		if fo.Stability != apigw_v1.Stability_STABILITY_UNSPECIFIED {
			return fo.Stability
		}
	}
	return apigw_v1.Stability_STABILITY_UNSPECIFIED
}

func getFieldDeprecation(f pgs.Field) *apigw_v1.Deprecation {
	for _, fo := range getFieldOptions(f) {
		if fo.Deprecation != nil {
			return fo.Deprecation
		}
	}
	return nil
}

func getFieldTerraformField(f pgs.Field) *apigw_v1.TerraformField {
	for _, fo := range getFieldOptions(f) {
		if fo.TerraformField != nil {
			return fo.TerraformField
		}
	}
	return nil
}

func mergeNullable(s *dm_base.Schema, nullable *bool) {
	if nullable == nil || !*nullable {
		return
	}
	if *nullable {
		s.Nullable = oasTrue()
	}
}
