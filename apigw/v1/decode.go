package v1

import (
	"io"
	"net/url"
)

type DecoderInput interface {
	Body() io.Reader
	Query() url.Values
	RouteParam(name string) string
}
