package ginapi

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	apigw_v1 "github.com/ductone/protoc-gen-apigw/apigw/v1"
	"github.com/gin-gonic/gin"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/durationpb"
)

// serveWithHandler runs one GET request through Handler with a method whose
// implementation is fn.
func serveWithHandler(t *testing.T, fn func(ctx context.Context) (proto.Message, error)) *httptest.ResponseRecorder {
	t.Helper()
	gin.SetMode(gin.TestMode)
	method := &apigw_v1.MethodDesc{
		Name:   "/test.v1.Service/Method",
		Method: http.MethodGet,
		Route:  "/test",
		Handler: func(_ interface{}, ctx context.Context, _ func(proto.Message) error, _ grpc.UnaryServerInterceptor) (proto.Message, error) {
			return fn(ctx)
		},
	}
	engine := gin.New()
	engine.GET("/test", Handler(nil, method, nil))

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	engine.ServeHTTP(rec, req)
	return rec
}

func TestHandlerSendsSetHeaderOnError(t *testing.T) {
	rec := serveWithHandler(t, func(ctx context.Context) (proto.Message, error) {
		if err := grpc.SetHeader(ctx, metadata.Pairs("retry-after", "7", "x-ratelimit-limit", "100")); err != nil {
			t.Fatalf("SetHeader: %v", err)
		}
		return nil, status.Error(codes.ResourceExhausted, "Request denied: Rate limit exceeded.")
	})

	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusTooManyRequests)
	}
	if got := rec.Header().Get("Retry-After"); got != "7" {
		t.Errorf("Retry-After = %q, want %q", got, "7")
	}
	if got := rec.Header().Get("X-Ratelimit-Limit"); got != "100" {
		t.Errorf("X-RateLimit-Limit = %q, want %q", got, "100")
	}
	if got := rec.Header().Get("Content-Type"); got != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", got)
	}
}

func TestHandlerSendsSetHeaderOnSuccess(t *testing.T) {
	rec := serveWithHandler(t, func(ctx context.Context) (proto.Message, error) {
		if err := grpc.SetHeader(ctx, metadata.Pairs("x-ratelimit-remaining", "99")); err != nil {
			t.Fatalf("SetHeader: %v", err)
		}
		return durationpb.New(0), nil
	})

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if got := rec.Header().Values("X-Ratelimit-Remaining"); len(got) != 1 || got[0] != "99" {
		t.Errorf("X-RateLimit-Remaining = %q, want exactly [\"99\"]", got)
	}
}
