package apigw

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"reflect"
	"sort"
	"strings"

	"github.com/fatih/camelcase"
	pgs "github.com/lyft/protoc-gen-star"
	pgsgo "github.com/lyft/protoc-gen-star/lang/go"
	dm_base "github.com/pb33f/libopenapi/datamodel/high/base"
	dm_v3 "github.com/pb33f/libopenapi/datamodel/high/v3"
	"github.com/pb33f/libopenapi/orderedmap"
	"github.com/stuart-warren/yamlfmt"
	"gopkg.in/yaml.v3"

	apigw_v1 "github.com/ductone/protoc-gen-apigw/apigw/v1"
)

const (
	stringTag = "!!str"
	nullTag   = "!!null"
)

type route struct {
	Route  string
	Method string
}

const SchemaProxyRefPrefix = "#/components/schemas/"

func (module *Module) buildOpenAPIService(ctx pgsgo.Context, in pgs.Service) (*dm_v3.Document, error) {
	doc := &dm_v3.Document{
		Version: "3.1.0",
		// NOTE: Info is required to be a valid OAS,
		// but we expect multiple services to Merge()
		// their OAS together, so we leave it minimally filled out.
		Info: &dm_base.Info{
			Title:       "API For " + nicerFQN(in),
			Version:     "0.0.1",
			Description: "This is an auto-generated API for " + nicerFQN(in) + ".\n",
		},
		Servers: []*dm_v3.Server{
			{
				URL:         "/",
				Description: "The server for " + nicerFQN(in) + ".",
			},
		},
		Paths: &dm_v3.Paths{
			PathItems: orderedmap.New[string, *dm_v3.PathItem](),
		},
		Components: &dm_v3.Components{
			Schemas: orderedmap.New[string, *dm_base.SchemaProxy](),
		},
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

func (module *Module) buildOpenAPIWithoutService(ctx pgsgo.Context, in pgs.File) (*dm_v3.Document, error) {
	doc := &dm_v3.Document{
		Version: "3.1.0",
		// NOTE: Info is required to be a valid OAS,
		// but we expect multiple services to Merge()
		// their OAS together, so we leave it minimally filled out.
		Info: &dm_base.Info{
			Title:       "Definitions For " + nicerFQN(in),
			Version:     "0.0.1",
			Description: "This is an auto-generated Definitions for " + nicerFQN(in) + ".\n",
		},
		Components: &dm_v3.Components{
			Schemas: orderedmap.New[string, *dm_base.SchemaProxy](),
		},
		Webhooks: orderedmap.New[string, *dm_v3.PathItem](),
	}
	found := false
	sc := newSchemaContainer()
	for _, m := range in.Messages() {
		opts := getMessageOptions(m)
		if opts == nil {
			continue
		}
		hasForceProp := opts.ForceExpose || opts.WebhookRequestName != ""
		if !hasForceProp {
			continue
		}
		found = true

		schemaProxy := sc.Message(m, nil, nil, false, true)
		if opts.WebhookRequestName != "" {
			_, exists := doc.Webhooks.Get(opts.WebhookRequestName)
			if !exists {
				truePtr := true
				content := orderedmap.New[string, *dm_v3.MediaType]()
				content.Set("application/json", &dm_v3.MediaType{
					Schema: schemaProxy,
				})
				doc.Webhooks.Set(opts.WebhookRequestName, &dm_v3.PathItem{
					Description: fmt.Sprintf("Schema for %s webhook", opts.WebhookRequestName),
					Post: &dm_v3.Operation{
						RequestBody: &dm_v3.RequestBody{
							Description: fmt.Sprintf("Schema for %s webhook request body", opts.WebhookRequestName),
							Content:     content,
							Required:    &truePtr,
						},
					},
				})
			}
		}
	}
	if !found {
		return nil, nil
	}
	components := &dm_v3.Components{
		Schemas: sc.schemas,
	}

	addOperation(doc, nil, nil, components)
	return doc, nil
}

func (module *Module) storeCanonicalRoute(route string, tokens []apigw_v1.RouteToken) *canonicalRoute {
	canonicalRouteStr := ""
	params := []string{}
	for _, token := range tokens {
		if token.IsParam {
			canonicalRouteStr += fmt.Sprintf("/{%d}", token.ParamIndex)
			params = append(params, toSnakeCase(token.ParamName))
		} else {
			canonicalRouteStr += "/" + token.Value
		}
	}

	routeData, ok := module.canonicalRouteMapper[canonicalRouteStr]
	if !ok {
		canon := &canonicalRoute{
			oasRoute: route,
			params:   params,
		}
		module.canonicalRouteMapper[canonicalRouteStr] = canon
		return canon
	}

	return routeData
}

func (module *Module) operationSummary(operation *apigw_v1.Operation, method pgs.Method) string {
	if operation.Summary != "" {
		return operation.Summary
	}

	return transformName(method.Name())
}

func (module *Module) getOpGroup(prefix string, operation *apigw_v1.Operation) string {
	if operation.Group != "" {
		return operation.Group
	}

	return strings.Join(camelcase.Split(prefix), " ")
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

	routeParts, err := apigw_v1.ParseRoute(operation.Route)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("apigw: failed to parse route '%s': %w", operation.Route, err)
	}

	canonicalRouteData := module.storeCanonicalRoute(operation.Route, routeParts)

	var snakeRoute strings.Builder
	for _, p := range routeParts {
		if _, err := snakeRoute.WriteString("/"); err != nil {
			return nil, nil, nil, err
		}
		if p.IsParam {
			paramName := canonicalRouteData.params[p.ParamIndex]
			if _, err := snakeRoute.WriteString(fmt.Sprintf("{%s}", paramName)); err != nil {
				return nil, nil, nil, err
			}
		} else {
			if _, err := snakeRoute.WriteString(p.Value); err != nil {
				return nil, nil, nil, err
			}
		}
	}
	r := &route{
		Method: operation.Method,
		Route:  snakeRoute.String(),
	}

	outObj := method.Output()
	outDescription := strings.TrimSpace(outObj.SourceCodeInfo().LeadingComments())
	if outDescription == "" {
		outDescription = "Successful response"
	}

	methodDescription := strings.TrimSpace(method.SourceCodeInfo().LeadingComments())
	if methodDescription == "" {
		methodDescription = "Invokes the " + nicerFQN(method) + " method."
	}

	fqn := strings.Split(method.FullyQualifiedName(), ".")
	extensions := orderedmap.New[string, *yaml.Node]()
	if len(fqn) > 2 {
		prefix := fqn[len(fqn)-2]
		methodName := fqn[len(fqn)-1]
		// Remove `Service` from method name
		prefix = strings.ReplaceAll(prefix, "Service", "")
		extensions.Set("tags",
			yamlStringSlice([]string{module.getOpGroup(prefix, operation)}),
		)
		extensions.Set("x-speakeasy-group", yamlString(prefix))
		extensions.Set("x-speakeasy-name-override", yamlString(methodName))
	}
	terraformEntity := getTerraformEntityOperationExtension(operation)

	if terraformEntity != nil {
		// Set the extension
		extensions.Set("x-speakeasy-entity-operation", terraformEntity)
	}

	outputRef := mt.Add(outObj)
	op := &dm_v3.Operation{
		OperationId: nicerFQN(method),
		Summary:     module.operationSummary(operation, method),
		Description: methodDescription,
		Deprecated:  oasBool(method.Descriptor().GetOptions().GetDeprecated()),
		Responses: &dm_v3.Responses{
			Codes: orderedmap.ToOrderedMap(map[string]*dm_v3.Response{
				"200": {
					Description: outDescription,
					Content: orderedmap.ToOrderedMap(map[string]*dm_v3.MediaType{
						"application/json": {
							Schema: outputRef,
						},
					}),
				},
			}),
		},
		Extensions: extensions,
	}

	inputFilter := []string{}

	sc := newSchemaContainer()
	for _, p := range routeParts {
		if !p.IsParam {
			continue
		}

		paramName := canonicalRouteData.params[p.ParamIndex]
		_, edgeField := module.path2fieldNumbers(strings.Split(p.ParamName, "."), method.Input())
		pp := &dm_v3.Parameter{
			Name:     paramName,
			In:       "path",
			Required: oasTrue(),
			Schema:   sc.Field(edgeField),
		}

		// TODO(pquerna): get docs from the field on the input object
		op.Parameters = append(op.Parameters, pp)
		inputFilter = append(inputFilter, p.ParamName)
	}

	paramsWithFieldNames := make([]queryWithParamName, 0, len(operation.Query))
	for qp, fieldName := range operation.Query {
		paramsWithFieldNames = append(paramsWithFieldNames, queryWithParamName{
			param: qp,
			field: fieldName,
		})
	}
	sort.Slice(paramsWithFieldNames, func(i, j int) bool {
		return paramsWithFieldNames[i].param < paramsWithFieldNames[j].param
	})

	for _, paramWithName := range paramsWithFieldNames {
		_, edgeField := module.path2fieldNumbers(strings.Split(paramWithName.field, "."), method.Input())
		// TODO(pquerna): get docs, types, and schema from the field on the input object
		op.Parameters = append(op.Parameters, &dm_v3.Parameter{
			Name:   paramWithName.param,
			In:     "query",
			Schema: sc.Field(edgeField),
		})
		inputFilter = append(inputFilter, paramWithName.field)
	}

	if operation.Method != http.MethodGet && operation.Method != http.MethodHead {
		inputRef := mt.AddInput(method.Input(), inputFilter)
		op.RequestBody = &dm_v3.RequestBody{
			Content: orderedmap.ToOrderedMap(map[string]*dm_v3.MediaType{
				"application/json": {
					Schema: inputRef,
				},
			}),
		}
	}
	for _, sd := range mt.messages {
		_ = sc.Message(sd.msg, sd.filter, nil, false, false)
	}
	components := &dm_v3.Components{
		Schemas: sc.schemas,
	}
	return r, op, components, nil
}

func toSnakeCase(s string) string {
	if !strings.Contains(s, ".") {
		return s
	}

	return strings.ReplaceAll(s, ".", "_")
}

func getTerraformEntityOperationExtension(operation *apigw_v1.Operation) *yaml.Node {
	terraformEntity := ""
	requiresDatasource := false
	if operation.TerraformEntity == nil {
		return nil
	}
	switch operation.TerraformEntity.Type {
	case apigw_v1.TerraformEntity_TERRAFORM_ENTITY_METHOD_TYPE_UNSPECIFIED:
		terraformEntity = ""
	case apigw_v1.TerraformEntity_TERRAFORM_ENTITY_METHOD_TYPE_CREATE:
		terraformEntity = fmt.Sprintf("%s#create", operation.TerraformEntity.Name)
	case apigw_v1.TerraformEntity_TERRAFORM_ENTITY_METHOD_TYPE_READ:
		requiresDatasource = true
		terraformEntity = fmt.Sprintf("%s#read", operation.TerraformEntity.Name)
	case apigw_v1.TerraformEntity_TERRAFORM_ENTITY_METHOD_TYPE_UPDATE:
		terraformEntity = fmt.Sprintf("%s#update", operation.TerraformEntity.Name)
	case apigw_v1.TerraformEntity_TERRAFORM_ENTITY_METHOD_TYPE_DELETE:
		terraformEntity = fmt.Sprintf("%s#delete", operation.TerraformEntity.Name)
	case apigw_v1.TerraformEntity_TERRAFORM_ENTITY_METHOD_TYPE_LIST:
		terraformEntity = fmt.Sprintf("%s#list", operation.TerraformEntity.Name)
	default:
		return nil
	}
	if operation.TerraformEntity.OperationNumber != 0 {
		terraformEntity = fmt.Sprintf("%s#%d", terraformEntity, operation.TerraformEntity.OperationNumber)
	}

	datasourceTag, resourceTag := stringTag, stringTag
	datasourceEntity, resourceEntity := terraformEntity, terraformEntity

	// Handle optional exclusions
	if te := operation.TerraformEntity; te != nil {
		switch te.OptionalExclusion {
		case apigw_v1.TerraformEntity_OPTIONAL_EXCLUSION_DATA_SOURCE_ONLY:
			resourceTag, resourceEntity = nullTag, ""
		case apigw_v1.TerraformEntity_OPTIONAL_EXCLUSION_RESOURCE_ONLY:
			datasourceTag, datasourceEntity = nullTag, ""
		case apigw_v1.TerraformEntity_OPTIONAL_EXCLUSION_UNSPECIFIED:
			// No special logic needed
		}
	}

	// Construct YAML node
	terraformEntityOperation := &yaml.Node{
		Kind: yaml.MappingNode,
		Content: []*yaml.Node{
			{Kind: yaml.ScalarNode, Tag: stringTag, Value: "terraform-resource"},
			{Kind: yaml.ScalarNode, Tag: resourceTag, Value: resourceEntity},
		},
	}

	dataSourceNodes := []*yaml.Node{
		{Kind: yaml.ScalarNode, Tag: stringTag, Value: "terraform-datasource"},
		{Kind: yaml.ScalarNode, Tag: datasourceTag, Value: datasourceEntity},
	}

	if requiresDatasource {
		terraformEntityOperation.Content = append(terraformEntityOperation.Content, dataSourceNodes...)
	}

	return terraformEntityOperation
}

func addOperation(doc *dm_v3.Document, r *route, op *dm_v3.Operation, comp *dm_v3.Components) {
	if r != nil {
		if doc.Paths.PathItems.Value(r.Route) == nil {
			doc.Paths.PathItems.Set(r.Route, &dm_v3.PathItem{})
		}

		switch r.Method {
		case http.MethodGet:
			doc.Paths.PathItems.Value(r.Route).Get = op
		case http.MethodPost:
			doc.Paths.PathItems.Value(r.Route).Post = op
		case http.MethodPut:
			doc.Paths.PathItems.Value(r.Route).Put = op
		case http.MethodDelete:
			doc.Paths.PathItems.Value(r.Route).Delete = op
		case http.MethodPatch:
			doc.Paths.PathItems.Value(r.Route).Patch = op
		case http.MethodHead:
			doc.Paths.PathItems.Value(r.Route).Head = op
		default:
			panic("apigw_openapi: addOperation: unsupported method: " + r.Method + " " + r.Route)
		}
	}

	// TODO(pquerna): currently we only use Schemas from Components.
	for pair := comp.Schemas.Oldest(); pair != nil; pair = pair.Next() {
		doc.Components.Schemas.Set(pair.Key, pair.Value)
	}
}

type openAPIContext struct {
	Name string
	Spec string
}

func (module *Module) renderOpenAPI(ctx pgsgo.Context, w io.Writer, in pgs.Service) error {
	doc, err := module.buildOpenAPIService(ctx, in)
	if err != nil {
		return err
	}
	yamlData, err := doc.Render()
	if err != nil {
		return err
	}
	c := openAPIContext{
		Name: ctx.ServerName(in).String(),
	}
	yamlData, err = yamlfmt.Format(bytes.NewReader(yamlData), true)
	if err != nil {
		return err
	}

	c.Spec = string(yamlData)
	return templates["openapi.tmpl"].Execute(w, c)
}
func (module *Module) renderOpenAPIWithoutService(ctx pgsgo.Context, w io.Writer, in pgs.File) (bool, error) {
	doc, err := module.buildOpenAPIWithoutService(ctx, in)
	if err != nil {
		return false, err
	}
	if doc == nil {
		return false, nil
	}
	yamlData, err := doc.Render()
	if err != nil {
		return false, err
	}
	c := openAPIContext{
		Name: ctx.PackageName(in).String(),
	}
	yamlData, err = yamlfmt.Format(bytes.NewReader(yamlData), true)
	if err != nil {
		return false, err
	}

	c.Spec = string(yamlData)
	return true, templates["openapi.tmpl"].Execute(w, c)
}

type schemaData struct {
	path   string
	msg    pgs.Message
	filter []string
}

type msgTracker struct {
	messages map[string]*schemaData
}

func nicerFQN(e pgs.Entity) string {
	return strings.TrimPrefix(e.FullyQualifiedName(), ".")
}

func (mt *msgTracker) AddInput(m pgs.Message, filter []string) *dm_base.SchemaProxy {
	if len(filter) == 0 {
		return mt.Add(m)
	}
	if mt.messages == nil {
		mt.messages = map[string]*schemaData{}
	}
	// TODO(pquerna): methods must have unique Input() messages?
	fqn := nicerFQN(m) + "Input"
	if sd, ok := mt.messages[fqn]; ok {
		if !reflect.DeepEqual(sd.filter, filter) {
			panic(fmt.Sprintf("apigw: AddInput: %s: filter must be identical for repeated inputs: %v != %v", fqn, sd.filter, filter))
		}
	} else {
		mt.messages[fqn] = &schemaData{path: fqn, msg: m, filter: filter}
	}

	return dm_base.CreateSchemaProxyRef(SchemaProxyRefPrefix + fqn)
}

func (mt *msgTracker) Add(m pgs.Message) *dm_base.SchemaProxy {
	if mt.messages == nil {
		mt.messages = map[string]*schemaData{}
	}
	fqn := nicerFQN(m)
	mt.messages[fqn] = &schemaData{path: fqn, msg: m}
	return dm_base.CreateSchemaProxyRef(SchemaProxyRefPrefix + fqn)
}

func contains[T comparable](needle T, haystack []T) bool {
	for _, v := range haystack {
		if v == needle {
			return true
		}
	}
	return false
}

func oasTrue() *bool {
	b := true
	return &b
}

func oasBool(v bool) *bool {
	b := v
	return &b
}
