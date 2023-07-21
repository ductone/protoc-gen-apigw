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
	return &decoderInput{ctx: ctx}
}

type decoderInput struct {
	ctx *gin.Context
}

func (d *decoderInput) PathParam(name string) string {
	return d.ctx.Param(name)
}

func (d *decoderInput) Query() url.Values {
	return d.ctx.Request.URL.Query()
}

func (d *decoderInput) Body() io.Reader {
	return d.ctx.Request.Body
}
