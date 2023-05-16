package apigw

import (
	"sort"

	pgs "github.com/lyft/protoc-gen-star"
	pgsgo "github.com/lyft/protoc-gen-star/lang/go"
)

type importTracker struct {
	ctx        pgsgo.Context
	input      pgs.File
	typeMapper map[pgs.Name]pgs.FilePath
	Ogen       bool
}

type ImportAlias struct {
	Import string
	Alias  string
}

func (ix *importTracker) ProtoImports() []ImportAlias {
	rv := make([]ImportAlias, 0, len(ix.typeMapper))
	for k, v := range ix.typeMapper {
		rv = append(rv, ImportAlias{
			Import: v.String(),
			Alias:  k.String(),
		})
	}
	sort.Slice(rv, func(i, j int) bool {
		return rv[i].Alias > rv[j].Alias
	})
	return rv
}
