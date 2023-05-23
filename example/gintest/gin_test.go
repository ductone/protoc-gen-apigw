package gintest

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	bookstore_v1 "github.com/ductone/protoc-gen-apigw/example/bookstore/v1"
	"github.com/ductone/protoc-gen-apigw/routers/ginapi"
	"github.com/gin-gonic/gin"
	"github.com/golang/protobuf/proto"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/encoding/protojson"
)

func TestGinRouter(t *testing.T) {
	engine := gin.New()
	registry := ginapi.NewRegistry(engine, nil)

	bs := &mockBookstore{t: t}
	bookstore_v1.RegisterGatewayBookstoreServiceServer(registry, bs)

	// TODO(pquerna): table tests for a bunch of different methods and behavoirs

	w := httptest.NewRecorder()
	engine.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/shelves", nil))
	require.Equal(t, http.StatusNotImplemented, w.Code)

	w = httptest.NewRecorder()
	engine.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/shelf", jsonify(t, map[string]interface{}{
		"shelf": map[string]interface{}{
			"id":                  123,
			"theme":               "test",
			"search[decoded]":     "sd",
			"search%5Bencoded%5D": "se",
		},
	})))
	require.Equal(t, http.StatusOK, w.Code)
	rb := &bookstore_v1.CreateShelfResponse{}
	err := protojson.Unmarshal(w.Body.Bytes(), rb)
	require.NoError(t, err)
	require.True(t, proto.Equal(&bookstore_v1.Shelf{Id: 123, Theme: "test", SearchDecoded: "sd", SearchEncoded: "se"}, rb.Shelf))

	w = httptest.NewRecorder()
	engine.ServeHTTP(w, httptest.NewRequest(http.MethodDelete, "/shelves/123", nil))
	require.Equal(t, http.StatusOK, w.Code)
	require.Equal(t, "{}", w.Body.String())
}

func jsonify(t *testing.T, obj map[string]interface{}) io.Reader {
	d, err := json.Marshal(obj)
	require.NoError(t, err)
	return bytes.NewReader(d)
}

type mockBookstore struct {
	bookstore_v1.UnimplementedBookstoreServiceServer
	t *testing.T
}

var _ bookstore_v1.BookstoreServiceServer = (*mockBookstore)(nil)

func (mb *mockBookstore) CreateShelf(ctx context.Context, req *bookstore_v1.CreateShelfRequest) (*bookstore_v1.CreateShelfResponse, error) {
	//	require.Equal(mb.t, &bookstore_v1.Shelf{Id: 123, Theme: "test", SearchDecoded: "sd", SearchEncoded: "se"}, req.Shelf)
	require.True(mb.t, proto.Equal(&bookstore_v1.Shelf{Id: 123, Theme: "test", SearchDecoded: "sd", SearchEncoded: "se"}, req.Shelf))
	return &bookstore_v1.CreateShelfResponse{
		Shelf: &bookstore_v1.Shelf{Id: 123, Theme: "test", SearchDecoded: "sd", SearchEncoded: "se"},
	}, nil
}

func (mb *mockBookstore) DeleteShelf(ctx context.Context, req *bookstore_v1.DeleteShelfRequest) (*bookstore_v1.DeleteShelfResponse, error) {
	require.Equal(mb.t, int64(123), req.Shelf)
	return &bookstore_v1.DeleteShelfResponse{}, nil
}
