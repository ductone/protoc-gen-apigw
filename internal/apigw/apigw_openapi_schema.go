package apigw

import (
	"fmt"
	"strings"

	apigw_v1 "github.com/ductone/protoc-gen-apigw/apigw/v1"
	pgs "github.com/lyft/protoc-gen-star"
	dm_base "github.com/pb33f/libopenapi/datamodel/high/base"
)

const FieldMaskWKT pgs.WellKnownType = "FieldMask"

func newSchemaContainer() *schemaContainer {
	return &schemaContainer{
		schemas: map[string]*dm_base.SchemaProxy{},
	}
}

type schemaContainer struct {
	schemas map[string]*dm_base.SchemaProxy
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

func (sc *schemaContainer) Message(m pgs.Message, filter []string, nullable *bool) *dm_base.SchemaProxy {
	if IsWellKnown(m) {
		// TODO(pquera): we may want to customize this some day,
		// but right now WKTs are just rendered inline, and not a Ref.
		return dm_base.CreateSchemaProxy(sc.schemaForWKT(WellKnownType(m)))
	}

	terraformEntityName := ""
	mopt := getMessageOptions(m)
	if mopt != nil {
		terraformEntity := mopt.GetTerraformEntity()
		if terraformEntity != nil {
			terraformEntityName = terraformEntity.Name
		}
	}

	fqn := nicerFQN(m)
	if len(filter) != 0 {
		fqn += "Input"
	}

	if sc.schemas[fqn] != nil {
		return dm_base.CreateSchemaProxyRef("#/components/schemas/" + fqn)
	}

	description := &strings.Builder{}
	comments := m.SourceCodeInfo().LeadingComments()
	if comments == "" {
		_, _ = fmt.Fprintf(description, "The %s message.", m.Name().String())
	} else {
		_, _ = description.WriteString(comments)
	}
	deprecated := oasBool(m.Descriptor().GetOptions().GetDeprecated())
	obj := &dm_base.Schema{
		Type:       []string{"object"},
		Properties: map[string]*dm_base.SchemaProxy{},
		Nullable:   nullable,
		Deprecated: deprecated,
		Extensions: map[string]any{
			"x-speakeasy-name-override": m.Name().UpperCamelCase().String(),
		},
	}
	if terraformEntityName != "" {
		obj.Extensions["x-speakeasy-entity"] = terraformEntityName
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

		fopt := getFieldOptions(f)
		if fopt != nil && fopt.GetRequiredSpec() {
			required = append(required, jn)
		}

		obj.Properties[jn] = sc.Field(f)
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
			obj.Properties[jn] = sc.Field(f)

			_, _ = fmt.Fprintf(description, "  - %s\n", jn)
		}
	}
	obj.Description = description.String()
	rv := dm_base.CreateSchemaProxy(obj)
	sc.schemas[fqn] = rv
	return dm_base.CreateSchemaProxyRef("#/components/schemas/" + fqn)
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
	enumValues := make([]any, 0, len(values))
	for _, v := range values {
		// TODO(pquerna): verify this is the right way to get the name
		// in the JSON format
		enumValues = append(enumValues, v.Name().String())
	}
	return &dm_base.Schema{
		Type: []string{"string"},
		Enum: enumValues,
	}
}

func (sc *schemaContainer) FieldTypeElem(fte pgs.FieldTypeElem) *dm_base.SchemaProxy {
	switch {
	case fte.IsEmbed():
		return sc.Message(fte.Embed(), nil, nil)
	case fte.IsEnum():
		return dm_base.CreateSchemaProxy(sc.Enum(fte.Enum()))
	default:
		return dm_base.CreateSchemaProxy(sc.schemaForScalar(fte.ProtoType()))
	}
}

func (sc *schemaContainer) Field(f pgs.Field) *dm_base.SchemaProxy {
	deprecated := oasBool(f.Descriptor().GetOptions().GetDeprecated())
	description := f.SourceCodeInfo().LeadingComments()
	if description == "" {
		jn := jsonName(f)
		description = "The " + jn + " field."
	}
	var nullable *bool
	if f.OneOf() != nil {
		nullable = oasTrue()
		description += "\nThis field is part of the `" + f.OneOf().Name().String() + "` oneof.\n" +
			"See the documentation for `" + nicerFQN(f.Message()) + "` for more details."
	}

	switch {
	case f.Type().IsRepeated():
		fteSchema := sc.FieldTypeElem(f.Type().Element())
		arraySchema := &dm_base.Schema{
			Type:        []string{"array"},
			Description: description,
			Nullable:    oasTrue(),
			Deprecated:  deprecated,
			Items:       &dm_base.DynamicValue[*dm_base.SchemaProxy, bool]{A: fteSchema},
		}
		return dm_base.CreateSchemaProxy(arraySchema)
	case f.Type().IsMap():
		fteSchema := sc.FieldTypeElem(f.Type().Element())
		mv := &dm_base.Schema{
			Type:                 []string{"object"},
			Deprecated:           deprecated,
			Description:          description,
			Nullable:             nullable,
			AdditionalProperties: fteSchema,
		}
		return dm_base.CreateSchemaProxy(mv)
	case f.Type().IsEnum():
		ev := sc.Enum(f.Type().Enum())
		ev.Deprecated = deprecated
		ev.Description = description
		mergeNullable(ev, nullable)
		return dm_base.CreateSchemaProxy(ev)
	case f.Type().IsEmbed():
		// todo: nested filters
		return sc.Message(f.Type().Embed(), nil, nullable)
	default:
		sv := sc.schemaForScalar(f.Type().ProtoType())
		mergeNullable(sv, nullable)
		sv.Deprecated = deprecated
		sv.Description = description
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

func getFieldOptions(m pgs.Field) *apigw_v1.FieldOption {
	fopt := &apigw_v1.FieldOptions{}
	_, err := m.Extension(apigw_v1.E_Field, fopt)
	if err != nil {
		return nil
	}
	if len(fopt.FieldOptions) > 0 {
		return fopt.FieldOptions[0]
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
