package ginapi

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"path"

	apigw_v1 "github.com/ductone/protoc-gen-apigw/apigw/v1"

	"github.com/gin-gonic/gin"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/peer"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
)

// TODO(pquerna): consider a server option to control this.
var jsonMarshaler = protojson.MarshalOptions{
	UseProtoNames:   true,
	EmitUnpopulated: true,
}

const (
	// 4 megabytes.  This isn't science, since JSON reperesentation of a proto
	// could be much larger, but this is an order of mangitude we want to reject.
	// TODO(pquerna): consider a server option to control this.
	maxRequestBody = 4 * 1024 * 1024
)

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
		stream := newGinTransportStream(c, method)
		ctx = grpc.NewContextWithServerTransportStream(ctx, stream)

		bytesBody, err := io.ReadAll(http.MaxBytesReader(c.Writer, c.Request.Body, maxRequestBody))
		if err != nil {
			targetErr := &http.MaxBytesError{}
			if errors.Is(err, targetErr) {
				ErrorResponse(c, status.Error(codes.InvalidArgument, "request body too large"))
				return
			}
			ErrorResponse(c, err)
			return
		}

		c.Request.Body = io.NopCloser(bytes.NewBuffer(bytesBody))

		resp, err := method.Handler(
			srv,
			ctx,
			func(reqProto proto.Message) error {
				return method.Decoder(ctx, input, reqProto)
			},
			interceptor,
		)
		if err != nil {
			ErrorResponse(c, err)
			return
		}

		data, err := jsonMarshaler.Marshal(resp)
		if err != nil {
			ErrorResponse(c, err)
			return
		}
		c.Header("Content-Type", "application/json")

		err = stream.writeHeader()
		if err != nil {
			ErrorResponse(c, err)
			return
		}

		_, err = c.Writer.Write(data)
		if err != nil {
			ErrorResponse(c, err)
			return
		}

		if len(stream.trailers) > 0 {
			err = stream.writeTrailer()
			if err != nil {
				ErrorResponse(c, err)
				return
			}
		}
		// all done!
	}
}

type ginTransportStream struct {
	ctx      *gin.Context
	method   *apigw_v1.MethodDesc
	headers  metadata.MD
	trailers metadata.MD
}

func newGinTransportStream(ctx *gin.Context, method *apigw_v1.MethodDesc) *ginTransportStream {
	return &ginTransportStream{
		ctx:      ctx,
		method:   method,
		headers:  metadata.MD{},
		trailers: metadata.MD{},
	}
}

func (g *ginTransportStream) Method() string {
	return g.method.Name
}

func (g *ginTransportStream) SetHeader(md metadata.MD) error {
	for k, v := range md {
		g.headers.Set(k, v...)
	}
	return nil
}

func (g *ginTransportStream) SendHeader(md metadata.MD) error {
	for k, v := range md {
		g.headers.Set(k, v...)
	}
	return g.writeHeader()
}

func (g *ginTransportStream) writeHeader() error {
	for k, v := range g.headers {
		for _, vv := range v {
			g.ctx.Writer.Header().Add(k, vv)
		}
	}
	g.ctx.Writer.WriteHeader(http.StatusOK)
	return nil
}

func (g *ginTransportStream) SetTrailer(md metadata.MD) error {
	for k, v := range md {
		g.trailers.Set(http.TrailerPrefix+k, v...)
	}
	return nil
}

func (g *ginTransportStream) writeTrailer() error {
	for k, v := range g.trailers {
		for _, vv := range v {
			g.ctx.Writer.Header().Add(k, vv)
		}
	}
	return nil
}

func ConvertRoute(r string) (string, error) {
	rv := []string{"/"}
	tkn, err := apigw_v1.ParseRoute(r)
	if err != nil {
		return "", err
	}
	for _, t := range tkn {
		if t.IsParam {
			rv = append(rv, fmt.Sprintf(":%s", t.ParamName))
		} else {
			rv = append(rv, t.Value)
		}
	}
	return path.Join(rv...), nil
}
