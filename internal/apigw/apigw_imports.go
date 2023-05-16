package apigw

import (
	pgs "github.com/lyft/protoc-gen-star"
	pgsgo "github.com/lyft/protoc-gen-star/lang/go"
)

type importTracker struct {
	ctx        pgsgo.Context
	input      pgs.File
	typeMapper map[pgs.Name]pgs.FilePath
}
