package apigw

import (
	"io"

	pgs "github.com/lyft/protoc-gen-star"
	pgsgo "github.com/lyft/protoc-gen-star/lang/go"
)

type methodTemplateContext struct {
	Name  string
	Route string
}

func (module *Module) methodContext(ctx pgsgo.Context, w io.Writer, f pgs.File, service pgs.Service, method pgs.Method, ix *importTracker) (*methodTemplateContext, error) {
	rv := &methodTemplateContext{
		Name: method.FullyQualifiedName(),
	}
	return rv, nil
}
