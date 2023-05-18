package ginapi

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"

	apigw_v1 "github.com/ductone/protoc-gen-apigw/apigw/v1"

	"github.com/gin-gonic/gin"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/encoding/protojson"
)

type jsonError struct {
	Code    codes.Code `json:"code"`
	Message string     `json:"message"`
	Details []any      `json:"details,omitempty"`
}

func (je *jsonError) write(c *gin.Context) {
	body, err := json.Marshal(je)
	if err != nil {
		c.Writer.WriteHeader(http.StatusInternalServerError)
		c.Writer.Write([]byte("Internal Server Error generating ISE (json.Marshal)"))
		return
	}
	_, _ = io.Copy(c.Writer, bytes.NewReader(body))
}

func ErrorResponse(c *gin.Context, err error) {
	statusErr, ok := status.FromError(err)
	if !ok {
		je := &jsonError{
			Code:    codes.Internal,
			Message: "Internal Server Error",
		}
		je.write(c)
		return
	}
	data, err := protojson.Marshal(statusErr.Proto())
	if err != nil {
		c.Writer.WriteHeader(http.StatusInternalServerError)
		c.Writer.Write([]byte("Internal Server Error generating status (protjson.Marshal)"))
		return
	}
	httpCode := apigw_v1.HTTPStatusFromCode(statusErr.Code())
	c.Writer.WriteHeader(httpCode)
	_, _ = io.Copy(c.Writer, bytes.NewReader(data))
}
