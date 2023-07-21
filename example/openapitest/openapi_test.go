package openapitest

import (
	"bytes"
	"io"
	"os"
	"testing"

	bookstore_v1 "github.com/ductone/protoc-gen-apigw/example/bookstore/v1"
	"github.com/pb33f/libopenapi"
	validator "github.com/pb33f/libopenapi-validator"
	dm_base "github.com/pb33f/libopenapi/datamodel/high/base"
	dm_v3 "github.com/pb33f/libopenapi/datamodel/high/v3"
	"github.com/stretchr/testify/require"
)

func TestOpenAPISpec(t *testing.T) {
	listShelvesResponseRef := dm_base.CreateSchemaProxyRef("#/components/schemas/ListShelvesResponse")
	shelfRef := dm_base.CreateSchemaProxyRef("#/components/schemas/Shelf")

	doc := dm_v3.Document{
		Version: "3.1.0",
		Info: &dm_base.Info{
			Title:   "Example API",
			Version: "1.0.0",
		},
		Paths: &dm_v3.Paths{
			PathItems: map[string]*dm_v3.PathItem{
				"/shelves": {
					Get: &dm_v3.Operation{
						OperationId: bookstore_v1.BookstoreService_ListShelves_FullMethodName,
						Parameters:  []*dm_v3.Parameter{},
						Responses: &dm_v3.Responses{
							Codes: map[string]*dm_v3.Response{
								"200": {
									Description: "OK",
									Content: map[string]*dm_v3.MediaType{
										"application/json": {
											Schema: listShelvesResponseRef,
										},
									},
								},
							},
						},
					},
				},
			},
		},
		Servers: []*dm_v3.Server{
			{
				URL: "https://api.example.com",
			},
		},
		Components: &dm_v3.Components{
			Schemas: map[string]*dm_base.SchemaProxy{
				"Shelf": dm_base.CreateSchemaProxy(&dm_base.Schema{
					Type: []string{"object"},
					Properties: map[string]*dm_base.SchemaProxy{
						"id": dm_base.CreateSchemaProxy(&dm_base.Schema{
							Type:   []string{"integer"},
							Format: "int64",
						}),
						"theme": dm_base.CreateSchemaProxy(&dm_base.Schema{
							Type: []string{"string"},
						}),
						`search[decoded]`: dm_base.CreateSchemaProxy(&dm_base.Schema{
							Type: []string{"string"},
						}),
						`search%5Bencoded%5D`: dm_base.CreateSchemaProxy(&dm_base.Schema{
							Type: []string{"string"},
						}),
					},
				}),
				"ListShelvesResponse": dm_base.CreateSchemaProxy(
					&dm_base.Schema{
						Type: []string{"object"},
						Properties: map[string]*dm_base.SchemaProxy{
							"shelves": dm_base.CreateSchemaProxy(
								&dm_base.Schema{
									Type:  []string{"array"},
									Items: &dm_base.DynamicValue[*dm_base.SchemaProxy, bool]{A: shelfRef},
								},
							),
						},
					},
				),
			},
		},
	}
	y, err := doc.Render()
	require.NoError(t, err)

	_, err = io.Copy(os.Stderr, bytes.NewReader(y))
	require.NoError(t, err)

	document, err := libopenapi.NewDocument(y)
	require.NoError(t, err)

	v, errs := validator.NewValidator(document)
	require.Nil(t, errs)
	ok, verrs := v.ValidateDocument()
	require.Nil(t, verrs)
	require.True(t, ok)
}
