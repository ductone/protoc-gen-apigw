package v1

import (
	"context"
	"net/http"

	"google.golang.org/grpc"
)

type ServiceDesc struct {
	Name string
	// The pointer to the service interface. Used to check whether the user
	// provided implementation satisfies the interface requirements.
	HandlerType interface{}
	Methods     []MethodDesc
	Spec        *Service
}

type methodHandler func(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error)

type MethodDesc struct {
	Name    string
	Route   string
	Handler methodHandler
	Decoder func(req http.Request, out interface{}) error
	Spec    *Operation
}

type ServiceRegistrar interface {
	RegisterService(sd *ServiceDesc, ss interface{})
}
