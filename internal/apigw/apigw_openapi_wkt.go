package apigw

import (
	pgs "github.com/lyft/protoc-gen-star"
	dm_base "github.com/pb33f/libopenapi/datamodel/high/base"
	"github.com/pb33f/libopenapi/orderedmap"
)

// based on the work here:
// https://github.com/google/gnostic/blob/main/cmd/protoc-gen-openapi/generator/wellknown/schemas.go
func (sc *schemaContainer) schemaForWKT(wkt pgs.WellKnownType) *dm_base.Schema {
	switch wkt {
	case pgs.AnyWKT:
		props := orderedmap.New[string, *dm_base.SchemaProxy]()
		props.Set("@type", dm_base.CreateSchemaProxy(&dm_base.Schema{
			Type:        []string{"string"},
			Description: "The type of the serialized message.",
		}))

		return &dm_base.Schema{
			Type:        []string{"object"},
			Description: "Contains an arbitrary serialized message along with a @type that describes the type of the serialized message.",
			Properties:  props,
			// TODO(pquerna): add a tag based annotation for possible Any values.
			AdditionalProperties: &dm_base.DynamicValue[*dm_base.SchemaProxy, bool]{B: true},
		}
	case pgs.DurationWKT:
		return &dm_base.Schema{
			Type:   []string{"string"},
			Format: "duration",
		}
	case pgs.EmptyWKT:
		return &dm_base.Schema{
			Type:     []string{"object"},
			Nullable: oasTrue(),
		}
	case pgs.StructWKT:
		// todo: pquerna: is this the right mapping for an arbitrary Struct?
		return &dm_base.Schema{
			Type:                 []string{"object"},
			AdditionalProperties: &dm_base.DynamicValue[*dm_base.SchemaProxy, bool]{B: true},
		}
	case pgs.TimestampWKT:
		return &dm_base.Schema{
			Type:   []string{"string"},
			Format: "date-time",
		}
	case pgs.ValueWKT:
		return &dm_base.Schema{
			Type:     []string{"string", "number", "object", "array", "boolean", "null"},
			Nullable: oasTrue(),
		}
	case pgs.ListValueWKT:
		return &dm_base.Schema{
			Type:     []string{"array"},
			Nullable: oasTrue(),
		}
	case pgs.DoubleValueWKT:
		return &dm_base.Schema{
			Type:     []string{"number"},
			Format:   "double",
			Nullable: oasTrue(),
		}
	case pgs.FloatValueWKT:
		return &dm_base.Schema{
			Type:     []string{"number"},
			Format:   "float",
			Nullable: oasTrue(),
		}
	case pgs.Int64ValueWKT:
		return &dm_base.Schema{
			Type:     []string{"string"},
			Format:   "int64",
			Nullable: oasTrue(),
		}
	case pgs.UInt64ValueWKT:
		return &dm_base.Schema{
			Type:     []string{"string"},
			Format:   "uint64",
			Nullable: oasTrue(),
		}
	case pgs.Int32ValueWKT:
		return &dm_base.Schema{
			Type:     []string{"integer"},
			Format:   "int32",
			Nullable: oasTrue(),
		}
	case pgs.UInt32ValueWKT:
		return &dm_base.Schema{
			Type:     []string{"integer"},
			Format:   "int64",
			Nullable: oasTrue(),
		}
	case pgs.BoolValueWKT:
		return &dm_base.Schema{
			Type:     []string{"boolean"},
			Nullable: oasTrue(),
		}
	case pgs.StringValueWKT:
		return &dm_base.Schema{
			Type:     []string{"string"},
			Nullable: oasTrue(),
		}
	case pgs.BytesValueWKT:
		return &dm_base.Schema{
			Type:     []string{"string"},
			Format:   "bytes",
			Nullable: oasTrue(),
		}
	case FieldMaskWKT:
		return &dm_base.Schema{
			Type:     []string{"string"},
			Nullable: oasTrue(),
		}
	case pgs.UnknownWKT:
		// TODO: handle these.. if any are really needed
		panic("UnknownWKT is not supported")
	default:
		panic("Unknown WKT")
	}
}
