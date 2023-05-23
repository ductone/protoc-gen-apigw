package apigw

import (
	"fmt"
	"io"
	"strings"

	apigw_v1 "github.com/ductone/protoc-gen-apigw/apigw/v1"
	pgs "github.com/lyft/protoc-gen-star"
	pgsgo "github.com/lyft/protoc-gen-star/lang/go"
	"google.golang.org/protobuf/testing/protopack"
)

type methodTemplateContext struct {
	Name               string
	Route              string
	FullMethodName     string
	MethodHandlerName  string
	DecoderHandlerName string
	HasBody            bool
	QueryParams        map[string]msgPackerContext
	RouteParams        []routeParamContext
	ServerName         string
	RequestType        string
	MethodName         string
}

type routeParamContext struct {
	ParamName           string
	Converter           string
	ConverterOutputName string
}

type msgPackerContext struct {
	FieldNumbers []int // field path
	WireType     protopack.Type
}

func jsonName(f pgs.Field) string {
	return *f.Descriptor().JsonName
}

func (module *Module) path2fieldNumbers(path []string, msg pgs.Message) ([]protopack.Number, pgs.Field) {
	var lasField pgs.Field
	if len(path) == 0 {
		return nil, nil
	}
	rv := make([]protopack.Number, 0, len(path))
	next := path[0]
	deeper := path[1:]
	for _, f := range msg.Fields() {
		if next == jsonName(f) {
			lasField = f
			rv = append(rv, protopack.Number(f.Descriptor().GetNumber()))
			if len(deeper) >= 1 {
				nestedMsg := f.Type().Embed()
				if nestedMsg == nil {
					panic("apigw: getFieldNumbers: unexpected path: " + strings.Join(path, ".") + " on " + msg.FullyQualifiedName())
				}
				nums, edgeField := module.path2fieldNumbers(deeper, nestedMsg)
				lasField = edgeField
				rv = append(rv, nums...)
			}
			break
		}
	}
	if len(rv) == 0 {
		panic("apigw: getFieldNumbers: unexpected path: " + strings.Join(path, ".") + " on " + msg.FullyQualifiedName())
	}
	return rv, lasField
}

func isInt(pt pgs.ProtoType) bool {
	switch pt {
	case pgs.Int64T, pgs.SFixed64, pgs.SInt64, pgs.Int32T, pgs.SFixed32, pgs.SInt32, pgs.EnumT:
		return true
	}

	return false
}

func isUint(pt pgs.ProtoType) bool {
	switch pt {
	case pgs.UInt64T, pgs.Fixed64T, pgs.UInt32T, pgs.Fixed32T:
		return true
	}

	return false
}

func (module *Module) methodContext(ctx pgsgo.Context, w io.Writer, f pgs.File, service pgs.Service, method pgs.Method, ix *importTracker) (*methodTemplateContext, error) {
	vn := &varNamer{prefix: "vn", offset: 0}

	mext := &apigw_v1.MethodOptions{}
	_, err := method.Extension(apigw_v1.E_Method, mext)
	if err != nil {
		return nil, fmt.Errorf("apigw: methodContext: failed to extract Method extension from '%s': %w", method.FullyQualifiedName(), err)
	}
	if len(mext.Operations) == 0 {
		return nil, nil
	}

	// TODO(pquerna): support multiple routes to the same handler
	if len(mext.Operations) != 1 {
		return nil, fmt.Errorf("apigw: methodContext: only single operation bindings are currently supported: %v", mext.Operations)
	}
	operation := mext.Operations[0]
	if operation.Route == "" {
		return nil, fmt.Errorf("apigw: methodContext: operation.Route missing on '%s'", method.FullyQualifiedName())
	}

	ix.ProtobufProto = true

	//TODO(pquerna): this is like the Service raw name, but translate to Go-safe letters.
	serviceShortName := strings.TrimSuffix(ctx.Name(service).String(), "Server")

	parts, err := apigw_v1.ParseRoute(operation.Route)
	if err != nil {
		return nil, fmt.Errorf("apigw: methodContext: operation.Route invalid '%s': %w", method.FullyQualifiedName(), err)
	}
	rpc := make([]routeParamContext, 0)
	for _, part := range parts {
		if !part.IsParam {
			continue
		}

		nesteFields := strings.Split(part.ParamName, ".")
		// TODO(pquerna): support nested fields
		if len(nesteFields) != 1 {
			module.Logf("apigw: methodContext: operation.Route invalid: target field is nested (unsupported right now) '%s' for route '%s'", method.FullyQualifiedName(), operation.Route)
			continue
		}

		nums, edgeField := module.path2fieldNumbers(nesteFields, method.Input())
		if len(nums) != 1 {
			return nil, fmt.Errorf("apigw: methodContext: operation.Route invalid: target numbers is nested (unsupported right now) '%s': %w", method.FullyQualifiedName(), err)
		}

		paramValueName := vn.String()
		vn = vn.Next()

		ix.ProtobufProtoPack = true
		routeGetter, err := templateExecToString("route_get_param.tmpl", &routeParseContext{
			ParamName:  part.ParamName,
			OutputName: paramValueName,
		})
		if err != nil {
			panic(err)
		}
		outName := vn.String()
		vn = vn.Next()

		switch {
		case edgeField.Type().IsRepeated():
			return nil, fmt.Errorf("apigw: methodContext: operation.Route invalid: target field is repeatd '%s': %w", method.FullyQualifiedName(), err)
		case edgeField.Type().IsMap():
			return nil, fmt.Errorf("apigw: methodContext: operation.Route invalid: target field is map '%s': %w", method.FullyQualifiedName(), err)
		case isInt(edgeField.Type().ProtoType()):
			ix.Strconv = true
			converter, err := templateExecToString("field_int.tmpl", &intFieldContext{
				FieldName:  jsonName(edgeField),
				Getter:     routeGetter,
				InputName:  paramValueName,
				OutputName: outName,
				Tag:        nums[0],
			})
			if err != nil {
				panic(err)
			}
			rpc = append(rpc, routeParamContext{
				ParamName:           part.ParamName,
				ConverterOutputName: outName,
				Converter:           converter,
			})
		case isUint(edgeField.Type().ProtoType()):
			ix.Strconv = true
			converter, err := templateExecToString("field_uint.tmpl", &uintFieldContext{
				FieldName:  jsonName(edgeField),
				Getter:     routeGetter,
				InputName:  paramValueName,
				OutputName: outName,
				Tag:        nums[0],
			})
			if err != nil {
				panic(err)
			}
			rpc = append(rpc, routeParamContext{
				ParamName:           part.ParamName,
				ConverterOutputName: outName,
				Converter:           converter,
			})
		case edgeField.Type().ProtoType() == pgs.StringT:
			converter, err := templateExecToString("field_string.tmpl", &stringFieldContext{
				FieldName:  jsonName(edgeField),
				Getter:     routeGetter,
				InputName:  paramValueName,
				OutputName: outName,
				Tag:        nums[0],
			})
			if err != nil {
				panic(err)
			}
			rpc = append(rpc, routeParamContext{
				ParamName:           part.ParamName,
				ConverterOutputName: outName,
				Converter:           converter,
			})
		case edgeField.Type().ProtoType() == pgs.BoolT:
			ix.Strconv = true
			converter, err := templateExecToString("field_bool.tmpl", &boolFieldContext{
				FieldName:  jsonName(edgeField),
				Getter:     routeGetter,
				InputName:  paramValueName,
				OutputName: outName,
				Tag:        nums[0],
			})
			if err != nil {
				panic(err)
			}
			rpc = append(rpc, routeParamContext{
				ParamName:           part.ParamName,
				ConverterOutputName: outName,
				Converter:           converter,
			})
		case edgeField.Type().ProtoType() == pgs.BytesT:
			return nil, fmt.Errorf("apigw: methodContext: operation.Route invalid: target field is bytes '%s': %w", method.FullyQualifiedName(), err)
		default:
			return nil, fmt.Errorf("apigw: methodContext: operation.Route invalid: target field is unknown '%s': %w", method.FullyQualifiedName(), err)
		}
	}

	rv := &methodTemplateContext{
		Name:  method.FullyQualifiedName(),
		Route: operation.Route,
		FullMethodName: fmt.Sprintf("%s_%s_FullMethodName",
			serviceShortName,
			ctx.Name(method).String(),
		),
		MethodHandlerName: fmt.Sprintf("_%s_%s_APIGW_Handler",
			serviceShortName,
			ctx.Name(method).String(),
		),
		DecoderHandlerName: fmt.Sprintf("_%s_%s_APIGW_Decoder",
			serviceShortName,
			ctx.Name(method).String(),
		),
		HasBody: operation.Method != "GET",

		ServerName:  ctx.ServerName(service).String(),
		RequestType: ctx.Name(method.Input()).String(),
		MethodName:  ctx.Name(method).String(),

		RouteParams: rpc,
	}
	if rv.HasBody {
		ix.GRPCCodes = true
		ix.GRPCStatus = true
		ix.ProtobufEncodingJSON = true
	}
	return rv, nil
}

type boolFieldContext struct {
	FieldName  string
	Getter     string
	OutputName string
	InputName  string
	Tag        protopack.Number
}

type stringFieldContext struct {
	FieldName  string
	Getter     string
	OutputName string
	InputName  string
	Tag        protopack.Number
}

type intFieldContext struct {
	FieldName  string
	Getter     string
	OutputName string
	InputName  string
	Tag        protopack.Number
}

type uintFieldContext struct {
	FieldName  string
	Getter     string
	OutputName string
	InputName  string
	Tag        protopack.Number
}

type routeParseContext struct {
	OutputName string
	ParamName  string
}
