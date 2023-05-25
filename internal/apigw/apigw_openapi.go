package apigw

import (
	"fmt"
	"io"
	"net/http"
	"strings"

	apigw_v1 "github.com/ductone/protoc-gen-apigw/apigw/v1"
	pgs "github.com/lyft/protoc-gen-star"
	pgsgo "github.com/lyft/protoc-gen-star/lang/go"
	dm_base "github.com/pb33f/libopenapi/datamodel/high/base"
	dm_v3 "github.com/pb33f/libopenapi/datamodel/high/v3"
)

type route struct {
	Route  string
	Method string
}

type msgTracker struct {
	messages map[string]pgs.Message
}

func (mt *msgTracker) Add(m pgs.Message) *dm_base.SchemaProxy {
	if mt.messages == nil {
		mt.messages = map[string]pgs.Message{}
	}
	fqn := m.FullyQualifiedName()
	mt.messages[fqn] = m
	return dm_base.CreateSchemaProxyRef("#/components/schemas/" + fqn)
}

func (module *Module) buildOpenAPI(ctx pgsgo.Context, in pgs.Service) (*dm_v3.Document, error) {
	doc := &dm_v3.Document{
		Version: "3.1.0",
		Paths: &dm_v3.Paths{
			PathItems: map[string]*dm_v3.PathItem{},
		},
		Components: &dm_v3.Components{Schemas: map[string]*dm_base.SchemaProxy{}},
	}
	mt := &msgTracker{}
	for _, m := range in.Methods() {
		route, op, err := module.buildOperation(ctx, m, mt)
		if err != nil {
			return nil, fmt.Errorf("opeapi.buildOperation failed for '%s': %w", m.FullyQualifiedName(), err)
		}
		if route == nil {
			continue
		}
		addOperation(doc, route, op)
	}
	return doc, nil
}

func (module *Module) buildOperation(ctx pgsgo.Context, method pgs.Method, mt *msgTracker) (*route, *dm_v3.Operation, error) {

	mext := &apigw_v1.MethodOptions{}
	_, err := method.Extension(apigw_v1.E_Method, mext)
	if err != nil {
		return nil, nil, fmt.Errorf("apigw: failed to extract Method extension from '%s': %w", method.FullyQualifiedName(), err)
	}
	if len(mext.Operations) == 0 {
		return nil, nil, nil
	}
	operation := mext.Operations[0]
	r := &route{
		Method: operation.Method,
		Route:  operation.Route,
	}

	outputRef := mt.Add(method.Output())

	op := &dm_v3.Operation{
		OperationId: method.FullyQualifiedName(),
		Parameters:  []*dm_v3.Parameter{},
		Responses: &dm_v3.Responses{

			Codes: map[string]*dm_v3.Response{
				"200": {
					Content: map[string]*dm_v3.MediaType{
						"application/json": {
							Schema: outputRef,
						},
					},
				},
			},
		},
	}

	return r, op, nil
}

func addOperation(doc *dm_v3.Document, r *route, op *dm_v3.Operation) {
	switch r.Method {
	case http.MethodGet:
		doc.Paths.PathItems["/shelves"].Get = op
	case http.MethodPost:
		doc.Paths.PathItems["/shelves"].Post = op
	case http.MethodPut:
		doc.Paths.PathItems["/shelves"].Put = op
	case http.MethodDelete:
		doc.Paths.PathItems["/shelves"].Delete = op
	case http.MethodPatch:
		doc.Paths.PathItems["/shelves"].Patch = op
	case http.MethodHead:
		doc.Paths.PathItems["/shelves"].Head = op
	default:
		panic("apigw_openapi: addOperation: unsupported method: " + method)
	}
}

type openAPIContext struct {
	ServerName string
	Spec       string
}

func (module *Module) renderOpenAPI(ctx pgsgo.Context, w io.Writer, in pgs.Service) error {
	doc, err := module.buildOpenAPI(ctx, in)
	if err != nil {
		return err
	}
	yamlData, err := doc.Render()
	if err != nil {
		return err
	}
	c := openAPIContext{
		ServerName: ctx.ServerName(in).String(),
	}
	// hack to escaple backticks in the yaml string
	c.Spec = strings.Replace("`", "` + "+`"`+"`"+`"`+" + `", string(yamlData), -1)
	return templates["openapi.tmpl"].Execute(w, c)
}
