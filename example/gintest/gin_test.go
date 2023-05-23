package gintest

import (
	"net/http"
	"net/http/httptest"
	"testing"

	bookstore_v1 "github.com/ductone/protoc-gen-apigw/example/bookstore/v1"
	"github.com/ductone/protoc-gen-apigw/routers/ginapi"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestGinRouter(t *testing.T) {
	engine := gin.New()
	registry := ginapi.NewRegistry(engine, nil)
	bs := &mockBookstore{}
	bookstore_v1.RegisterGatewayBookstoreServiceServer(registry, bs)

	w := httptest.NewRecorder()
	engine.ServeHTTP(w, httptest.NewRequest("GET", "/shelves", nil))

	require.Equal(t, http.StatusNotImplemented, w.Code)
}

type mockBookstore struct {
	bookstore_v1.UnimplementedBookstoreServiceServer
}

var _ bookstore_v1.BookstoreServiceServer = (*mockBookstore)(nil)
