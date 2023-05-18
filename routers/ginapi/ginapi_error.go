package ginapi

import (
	"bytes"
	"io"
	"net/http"

	apigw_v1 "github.com/ductone/protoc-gen-apigw/apigw/v1"
	"github.com/grpc-ecosystem/go-grpc-middleware/logging/zap/ctxzap"
	"go.uber.org/zap"

	"github.com/gin-gonic/gin"
	spb "google.golang.org/genproto/googleapis/rpc/status"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/encoding/protojson"
)

func ErrorResponse(c *gin.Context, err error) {
	var statusProto *spb.Status
	var httpStatus int
	l := ctxzap.Extract(c.Request.Context())
	// TODO(pquerna): make request id header configurable
	requestId := c.Writer.Header().Get("c1-trace-id")

	statusErr, ok := status.FromError(err)
	if ok {
		statusProto = statusErr.Proto()
		httpStatus = apigw_v1.HTTPStatusFromCode(statusErr.Code())
		l.Debug("ginapi: returning error",
			zap.Stringer("status_code", statusErr.Code()),
			zap.String("status_message", statusErr.Message()),
			zap.Int("http.status_code", httpStatus),
			zap.String("request_id", requestId),
		)
	} else {
		statusProto = &spb.Status{
			Code:    int32(codes.Internal),
			Message: "Internal Server Error",
		}
		httpStatus = http.StatusInternalServerError
		l.Error("ginapi: returning unknown error",
			zap.Error(err),
			zap.Int("http.status_code", httpStatus),
			zap.String("request_id", requestId),
		)
	}

	if requestId != "" {
		statusProto.Message += " (request-id: " + requestId + ")"
	}

	data, err := protojson.Marshal(statusProto)
	if err != nil {
		c.Header("Content-Type", "text/plain")
		c.Writer.WriteHeader(http.StatusInternalServerError)
		msg := "Internal Server Error generating status (protjson.Marshal)"
		if requestId != "" {
			msg += " (request-id: " + requestId + ")"
		}
		_, _ = c.Writer.Write([]byte(msg))
		return
	}
	c.Header("Content-Type", "application/json")
	c.Writer.WriteHeader(httpStatus)
	_, _ = io.Copy(c.Writer, bytes.NewReader(data))
}
