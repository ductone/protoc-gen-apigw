package apigw

import (
	"fmt"
	"io"
	"net/http"
	"reflect"
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
		route, op, components, err := module.buildOperation(ctx, m, mt)
		if err != nil {
			return nil, fmt.Errorf("opeapi.buildOperation failed for '%s': %w", m.FullyQualifiedName(), err)
		}
		if route == nil {
			continue
		}
		addOperation(doc, route, op, components)
	}
	return doc, nil
}

func (module *Module) buildOperation(ctx pgsgo.Context, method pgs.Method, mt *msgTracker) (*route, *dm_v3.Operation, *dm_v3.Components, error) {
	mext := &apigw_v1.MethodOptions{}
	_, err := method.Extension(apigw_v1.E_Method, mext)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("apigw: failed to extract Method extension from '%s': %w", method.FullyQualifiedName(), err)
	}
	if len(mext.Operations) == 0 {
		return nil, nil, nil, nil
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
			Default: &dm_v3.Response{
				Content: map[string]*dm_v3.MediaType{
					"application/json": {
						Schema: outputRef,
					},
				},
			},
		},
	}

	routeParts, err := apigw_v1.ParseRoute(r.Route)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("apigw: failed to parse route '%s': %w", r.Route, err)
	}

	inputFilter := []string{}

	sc := newSchemaContainer()

	for _, p := range routeParts {
		if !p.IsParam {
			continue
		}

		_, edgeField := module.path2fieldNumbers(strings.Split(p.ParamName, "."), method.Input())
		pp := &dm_v3.Parameter{
			Name:     p.ParamName,
			In:       "path",
			Required: true,
			Style:    "simple",
			Schema:   sc.Field(edgeField),
		}

		// TODO(pquerna): get docs from the field on the input object
		op.Parameters = append(op.Parameters, pp)
		inputFilter = append(inputFilter, p.ParamName)
	}
	for qp, fieldName := range operation.Query {
		_, edgeField := module.path2fieldNumbers(strings.Split(fieldName, "."), method.Input())
		// TODO(pquerna): get docs, types, and schema from the field on the input object
		op.Parameters = append(op.Parameters, &dm_v3.Parameter{
			Name:   qp,
			In:     "query",
			Style:  "simple",
			Schema: sc.Field(edgeField),
		})
		inputFilter = append(inputFilter, fieldName)
	}

	inputRef := mt.AddInput(method.Input(), inputFilter)
	if operation.Method != http.MethodGet && operation.Method != http.MethodHead {
		op.RequestBody = &dm_v3.RequestBody{
			Content: map[string]*dm_v3.MediaType{
				"application/json": {
					Schema: inputRef,
				},
			},
		}
	}
	for _, sd := range mt.messages {
		_ = sc.Message(sd.msg, sd.filter)
	}
	components := &dm_v3.Components{
		Schemas: sc.schemas,
	}
	return r, op, components, nil
}

func addOperation(doc *dm_v3.Document, r *route, op *dm_v3.Operation, comp *dm_v3.Components) {
	if doc.Paths.PathItems[r.Route] == nil {
		doc.Paths.PathItems[r.Route] = &dm_v3.PathItem{}
	}

	switch r.Method {
	case http.MethodGet:
		doc.Paths.PathItems[r.Route].Get = op
	case http.MethodPost:
		doc.Paths.PathItems[r.Route].Post = op
	case http.MethodPut:
		doc.Paths.PathItems[r.Route].Put = op
	case http.MethodDelete:
		doc.Paths.PathItems[r.Route].Delete = op
	case http.MethodPatch:
		doc.Paths.PathItems[r.Route].Patch = op
	case http.MethodHead:
		doc.Paths.PathItems[r.Route].Head = op
	default:
		panic("apigw_openapi: addOperation: unsupported method: " + r.Method + " " + r.Route)
	}
	// TODO(pquerna): currently we only use Schemas from Components.
	for k, v := range comp.Schemas {
		doc.Components.Schemas[k] = v
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
	c.Spec = strings.Replace(string(yamlData), "`", "` + "+`"`+"`"+`"`+" + `", -1)
	return templates["openapi.tmpl"].Execute(w, c)
}

type schemaData struct {
	path   string
	msg    pgs.Message
	filter []string
}

type msgTracker struct {
	messages map[string]*schemaData
}

func (mt *msgTracker) AddInput(m pgs.Message, filter []string) *dm_base.SchemaProxy {
	if len(filter) == 0 {
		return mt.Add(m)
	}
	if mt.messages == nil {
		mt.messages = map[string]*schemaData{}
	}
	// TODO(pquerna): methods must have unique Input() messages?
	fqn := m.FullyQualifiedName() + "Input"
	if sd, ok := mt.messages[fqn]; ok {
		if !reflect.DeepEqual(sd.filter, filter) {
			panic(fmt.Sprintf("apigw: AddInput: %s: filter must be identical for repeated inputs: %v != %v", fqn, sd.filter, filter))
		}
	} else {
		mt.messages[fqn] = &schemaData{path: fqn, msg: m, filter: filter}
	}

	return dm_base.CreateSchemaProxyRef("#/components/schemas/" + fqn)
}

func (mt *msgTracker) Add(m pgs.Message) *dm_base.SchemaProxy {
	if mt.messages == nil {
		mt.messages = map[string]*schemaData{}
	}
	fqn := m.FullyQualifiedName()
	mt.messages[fqn] = &schemaData{path: fqn, msg: m}
	return dm_base.CreateSchemaProxyRef("#/components/schemas/" + fqn)
}

func contains[T comparable](needle T, haystack []T) bool {
	for _, v := range haystack {
		if v == needle {
			return true
		}
	}
	return false
}
