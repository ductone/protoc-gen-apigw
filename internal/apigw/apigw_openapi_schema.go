package apigw

import (
	pgs "github.com/lyft/protoc-gen-star"
	dm_base "github.com/pb33f/libopenapi/datamodel/high/base"
)

func newSchemaContainer() *schemaContainer {
	return &schemaContainer{
		schemas: map[string]*dm_base.SchemaProxy{},
	}
}

type schemaContainer struct {
	schemas map[string]*dm_base.SchemaProxy
}

func (sc *schemaContainer) Message(m pgs.Message, filter []string) *dm_base.SchemaProxy {
	if m.IsWellKnown() {
		// TODO(pquera): we may want to customize this some day,
		// but right now WKTs are just rendered inline, and not a Ref.
		return sc.schemaForWKT(m.WellKnownType())
	}

	fqn := m.FullyQualifiedName()
	if len(filter) != 0 {
		fqn += "Input"
	}

	if sc.schemas[fqn] != nil {
		return dm_base.CreateSchemaProxyRef("#/components/schemas/" + fqn)
	}

	obj := &dm_base.Schema{
		Type:       []string{"object"},
		Properties: map[string]*dm_base.SchemaProxy{},
	}
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
		obj.Properties[jn] = sc.Field(f)
	}

	for _, of := range m.OneOfs() {
		required := []*dm_base.SchemaProxy{}

		for _, f := range of.Fields() {
			jn := jsonName(f)
			obj.Properties[jn] = sc.Field(f)
			required = append(required, dm_base.CreateSchemaProxy(&dm_base.Schema{
				Required: []string{jn},
			}))
		}

		ao := &dm_base.Schema{
			OneOf: required,
		}

		obj.AnyOf = append(obj.AnyOf, dm_base.CreateSchemaProxy(ao))
	}
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
		return sc.Message(fte.Embed(), nil)
	case fte.IsEnum():
		return dm_base.CreateSchemaProxy(sc.Enum(fte.Enum()))
	default:
		return dm_base.CreateSchemaProxy(sc.schemaForScalar(fte.ProtoType()))
	}
}

func (sc *schemaContainer) Field(f pgs.Field) *dm_base.SchemaProxy {
	switch {
	case f.Type().IsRepeated():
		fteSchema := sc.FieldTypeElem(f.Type().Element())
		arraySchema := &dm_base.Schema{
			Type:     []string{"array"},
			Nullable: oapiTrue(),
			Items:    &dm_base.DynamicValue[*dm_base.SchemaProxy, bool]{A: fteSchema},
		}
		return dm_base.CreateSchemaProxy(arraySchema)
	case f.Type().IsMap():
		fteSchema := sc.FieldTypeElem(f.Type().Element())

		return dm_base.CreateSchemaProxy(&dm_base.Schema{
			Type:                 []string{"object"},
			AdditionalProperties: fteSchema,
		})
	case f.Type().IsEnum():
		return dm_base.CreateSchemaProxy(sc.Enum(f.Type().Enum()))
	case f.Type().IsEmbed():
		// todo: nested filters
		return sc.Message(f.Type().Embed(), nil)
	default:
		return dm_base.CreateSchemaProxy(sc.schemaForScalar(f.Type().ProtoType()))
	}
}
