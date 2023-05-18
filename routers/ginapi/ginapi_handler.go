package ginapi

import (
	"context"

	apigw_v1 "github.com/ductone/protoc-gen-apigw/apigw/v1"

	"github.com/gin-gonic/gin"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/peer"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
)

// TODO(pquerna): consider a server option to control this.
var jsonMarshaler = protojson.MarshalOptions{
	UseProtoNames:   true,
	EmitUnpopulated: true,
}

func Handler(srv interface{}, method *apigw_v1.MethodDesc, interceptor grpc.UnaryServerInterceptor) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()

		input := ContextAsDecoderInput(c)
		md := apigw_v1.MetadataForRequest(c.Request, method.Name)
		p := apigw_v1.PeerForRequest(c.Request)
		timeout, ok := apigw_v1.TimeoutForRequest(c.Request)

		var timeoutDone context.CancelFunc
		if ok {
			ctx, timeoutDone = context.WithTimeout(ctx, timeout)
			defer timeoutDone()
		}

		ctx = peer.NewContext(ctx, p)
		ctx = metadata.NewIncomingContext(ctx, md)
		ctx = grpc.NewContextWithServerTransportStream(ctx, &ginTransportStream{ctx: c})

		resp, err := method.Handler(
			srv,
			ctx,
			func(reqProto proto.Message) error {
				return method.Decoder(ctx, input, reqProto)
			},
			interceptor,
		)
		if err != nil {
			// TODO(pquerna): statuspb mapping
			c.Error(err)
			return
		}

		data, err := jsonMarshaler.Marshal(resp)
		if err != nil {
			// TODO(pquerna): statuspb mapping, hard error.
			c.Error(err)
			return
		}
		c.Header("Content-Type", "application/json")
		c.Writer.Write(data)
	}
}

type ginTransportStream struct {
	ctx    *gin.Context
	method *apigw_v1.MethodDesc
}

func (g *ginTransportStream) Method() string {
	return g.method.Name
}

func (g *ginTransportStream) SetHeader(md metadata.MD) error {
	return nil
}

func (g *ginTransportStream) SendHeader(md metadata.MD) error {
	return nil
}

func (g *ginTransportStream) SetTrailer(md metadata.MD) error {
	return nil

}
