package ginapi

import (
	"fmt"
	"reflect"

	apigw_v1 "github.com/ductone/protoc-gen-apigw/apigw/v1"
	"github.com/gin-gonic/gin"
	"google.golang.org/grpc"
)

func NewRegistry(router gin.IRouter,
	unaryInterceptor grpc.UnaryServerInterceptor,
) *Registry {
	return &Registry{
		router:           router,
		unaryInterceptor: unaryInterceptor,
	}
}

type Registry struct {
	router           gin.IRouter
	unaryInterceptor grpc.UnaryServerInterceptor
}

var _ apigw_v1.ServiceRegistrar = (*Registry)(nil)

func (r *Registry) RegisterService(sd *apigw_v1.ServiceDesc, impl interface{}) {
	// verify that the implentation implemnts the handler type
	if impl != nil {
		ht := reflect.TypeOf(sd.HandlerType).Elem()
		st := reflect.TypeOf(impl)
		if !st.Implements(ht) {
			panic(fmt.Sprintf("ginapi: RegisterService found the handler of type %v that does not satisfy %v", st, ht))
		}
	}

	for _, m := range sd.Methods {
		hnd := Handler(impl, m, r.unaryInterceptor)
		ginRoute, err := ConvertRoute(m.Route)
		if err != nil {
			panic(fmt.Sprintf("ginapi: RegisterService unable to convert route %q: %v", m.Route, err))
		}
		_ = r.router.Handle(m.Method, ginRoute, hnd)
	}
}
