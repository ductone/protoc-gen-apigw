package ginapi

import (
	"io"
	"net/url"

	apigw_v1 "github.com/ductone/protoc-gen-apigw/apigw/v1"

	"github.com/gin-gonic/gin"
)

// Function ContextAsDecoderInput converts a gin.Context to a DecoderInput.
//
// This function is used in the generated code.
func ContextAsDecoderInput(ctx *gin.Context) apigw_v1.DecoderInput {
	return &decoderInput{
		ctx:   ctx,
		query: ctx.Request.URL.Query(),
	}
}

func newDecoderInput(ctx *gin.Context) (*decoderInput, error) {
	query, err := url.ParseQuery(ctx.Request.URL.RawQuery)
	if err != nil {
		return nil, err
	}
	return &decoderInput{
		ctx:   ctx,
		query: query,
	}, nil
}

type decoderInput struct {
	ctx   *gin.Context
	query url.Values
}

func (d *decoderInput) PathParam(name string) string {
	return d.ctx.Param(name)
}

func (d *decoderInput) Query() url.Values {
	return d.query
}

func (d *decoderInput) Body() io.Reader {
	return d.ctx.Request.Body
}
