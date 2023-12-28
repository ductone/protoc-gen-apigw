package apigw

import (
	pgs "github.com/lyft/protoc-gen-star"
	dm_base "github.com/pb33f/libopenapi/datamodel/high/base"
)

// https://protobuf.dev/programming-guides/proto3/#json
func (sc *schemaContainer) schemaForScalar(pt pgs.ProtoType) *dm_base.Schema {
	switch pt {
	case pgs.DoubleT, pgs.FloatT:
		return &dm_base.Schema{
			Type: []string{"number"},
		}
	case pgs.Int64T, pgs.SFixed64, pgs.SInt64:
		return &dm_base.Schema{
			Type:   []string{"string"},
			Format: "int64",
		}
	case pgs.Int32T, pgs.SFixed32, pgs.SInt32:
		return &dm_base.Schema{
			Type:   []string{"integer"},
			Format: "int32",
		}
	case pgs.UInt64T, pgs.Fixed64T:
		return &dm_base.Schema{
			Type:   []string{"string"},
			Format: "uint64",
		}
	case pgs.UInt32T, pgs.Fixed32T:
		return &dm_base.Schema{
			Type:   []string{"integer"},
			Format: "uint32",
		}
	case pgs.StringT:
		return &dm_base.Schema{
			Type: []string{"string"},
		}
	case pgs.BytesT:
		return &dm_base.Schema{
			Type:   []string{"string"},
			Format: "base64",
		}
	case pgs.BoolT:
		return &dm_base.Schema{
			Type: []string{"boolean"},
		}
	default:
		panic("not a scalar scalar type: " + pt.String())
	}
}
