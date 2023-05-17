package apigw

import (
	"io"
	"path/filepath"

	pgs "github.com/lyft/protoc-gen-star"
	pgsgo "github.com/lyft/protoc-gen-star/lang/go"
)

type moduleTemplateContext struct {
	Version     string
	PackageName string
	SourceFile  string
}

func (module *Module) renderModule(ctx pgsgo.Context, w io.Writer, in pgs.File) error {
	c := &moduleTemplateContext{
		Version:     version,
		SourceFile:  filepath.Dir(in.Name().String()),
		PackageName: ctx.PackageName(in).String(),
	}

	return templates["module.tmpl"].Execute(w, c)
}
