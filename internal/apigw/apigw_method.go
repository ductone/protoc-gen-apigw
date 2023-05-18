package apigw

import (
	"fmt"
	"io"
	"strings"

	pgs "github.com/lyft/protoc-gen-star"
	pgsgo "github.com/lyft/protoc-gen-star/lang/go"
)

type methodTemplateContext struct {
	Name               string
	Route              string
	MethodHandlerName  string
	DecoderHandlerName string
	HasBody            bool
	QueryParams        map[string]string
	RouteParams        map[string]string
}

func (module *Module) methodContext(ctx pgsgo.Context, w io.Writer, f pgs.File, service pgs.Service, method pgs.Method, ix *importTracker) (*methodTemplateContext, error) {
	ix.ProtobufProto = true

	//TODO(pquerna): this is like the Service raw name, but translate to Go-safe letters.
	serviceShortName := strings.TrimSuffix(ctx.Name(service).String(), "Server")

	rv := &methodTemplateContext{
		Name: method.FullyQualifiedName(),
		MethodHandlerName: fmt.Sprintf("_%s_%s_APIGW_Handler",
			//TODO(pquerna): this is like the Service raw name, but translate to Go-safe letters.
			serviceShortName,
			ctx.Name(method).String(),
		),
		DecoderHandlerName: fmt.Sprintf("_%s_%s_APIGW_Decoder",
			//TODO(pquerna): this is like the Service raw name, but translate to Go-safe letters.
			serviceShortName,
			ctx.Name(method).String(),
		),
	}
	return rv, nil
}
