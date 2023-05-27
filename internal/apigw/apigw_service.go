package apigw

import (
	"fmt"
	"io"
	"path/filepath"

	pgs "github.com/lyft/protoc-gen-star"
	pgsgo "github.com/lyft/protoc-gen-star/lang/go"
)

type serviceTemplateContext struct {
	ServerName         string
	FullyQualifiedName string
	OASFileName        string
	Methods            []*methodTemplateContext
}

func (module *Module) renderService(ctx pgsgo.Context, w io.Writer, f pgs.File, in pgs.Service, ix *importTracker, oasName string) error {
	ix.APIGWV1 = true
	ix.Embed = true
	c := &serviceTemplateContext{
		ServerName: ctx.ServerName(in).String(),
		FullyQualifiedName: fmt.Sprintf("%s.%s",
			in.Package().ProtoName().String(),
			in.Name().String(),
		),
		OASFileName: filepath.Base(oasName),
	}
	for _, method := range in.Methods() {
		methodCtx, err := module.methodContext(ctx, w, f, in, method, ix)
		if err != nil {
			return fmt.Errorf("method generation failed [%st: %w", method.FullyQualifiedName(), err)
		}
		if methodCtx == nil {
			continue
		}
		c.Methods = append(c.Methods, methodCtx)
	}

	return templates["service.tmpl"].Execute(w, c)
}
