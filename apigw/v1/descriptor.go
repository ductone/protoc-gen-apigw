package v1

import (
	"context"

	"google.golang.org/grpc"
	"google.golang.org/protobuf/proto"
)

type ServiceDesc struct {
	Name string
	// The pointer to the service interface. Used to check whether the user
	// provided implementation satisfies the interface requirements.
	HandlerType interface{}
	Methods     []*MethodDesc
	Spec        *Service
}

type methodHandler func(srv interface{}, ctx context.Context, dec func(proto.Message) error, interceptor grpc.UnaryServerInterceptor) (proto.Message, error)
type decoderHandler func(ctx context.Context, input DecoderInput, out proto.Message) error

type MethodDesc struct {
	Name    string
	Route   string
	Handler methodHandler
	Decoder decoderHandler
	Spec    *Operation
}

type ServiceRegistrar interface {
	RegisterService(sd *ServiceDesc, ss interface{})
}
