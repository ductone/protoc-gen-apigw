package apigw

import (
	"fmt"
	"regexp"
	"sort"

	pgs "github.com/lyft/protoc-gen-star"
	pgsgo "github.com/lyft/protoc-gen-star/lang/go"
)

type importTracker struct {
	ctx                       pgsgo.Context
	input                     pgs.File
	typeMapper                map[pgs.Name]pgs.FilePath
	APIGWV1                   bool
	Bytes                     bool
	Embed                     bool
	Fmt                       bool
	GRPC                      bool
	GRPCCodes                 bool
	GRPCStatus                bool
	Io                        bool
	NetHTTP                   bool
	Ogen                      bool
	ProtobufEncodingJSON      bool
	ProtobufProto             bool
	ProtobufProtoPack         bool
	ProtobufEncodingProtowire bool
	Strconv                   bool
	Strings                   bool
	Time                      bool
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

func (c *importTracker) Type(f pgs.Field) pgsgo.TypeName {
	ft := f.Type()

	var t pgsgo.TypeName
	switch {
	case ft.IsMap():
		key := scalarType(ft.Key().ProtoType())
		return pgsgo.TypeName(fmt.Sprintf("map[%s]%s", key, c.elType(ft)))
	case ft.IsRepeated():
		return pgsgo.TypeName(fmt.Sprintf("[]%s", c.elType(ft)))
	case ft.IsEmbed():
		return c.importableTypeName(f, ft.Embed()).Pointer()
	case ft.IsEnum():
		t = c.importableTypeName(f, ft.Enum())
	default:
		t = scalarType(ft.ProtoType())
	}

	if f.HasPresence() {
		return t.Pointer()
	}

	return t
}

func (ix *importTracker) importableTypeName(f pgs.Entity, containingEntity pgs.Entity) pgsgo.TypeName {
	t := pgsgo.TypeName(ix.ctx.Name(containingEntity))

	if ix.ctx.ImportPath(containingEntity) == ix.ctx.ImportPath(f) {
		return t
	}
	return pgsgo.TypeName(fmt.Sprintf("%s.%s", ix.importablePackageName(f, containingEntity), t))
}

func (ix *importTracker) importablePackageName(f pgs.Entity, containingEntity pgs.Entity) pgsgo.TypeName {
	ctx := ix.ctx
	if ctx.ImportPath(containingEntity) == ctx.ImportPath(f) {
		return ""
	}
	pkgName := ctx.PackageName(containingEntity)
	importName := ctx.ImportPath(containingEntity)
	matched, err := regexp.MatchString(`^v(\d)+$`, pkgName.String())
	if err != nil {
		panic(err)
	}

	if matched {
		niceName := importName.Dir().Base()
		pkgName = pgs.Name(fmt.Sprintf("%s_%s", niceName, pkgName.String()))
	}
	return pgsgo.TypeName(ix.allocatePackageName(pkgName, importName))
}

// Reserve every fixed header import even if its flag has not been enabled yet:
// another method in the same file can enable it after a proto alias is chosen.
func fixedImportName(name pgs.Name) bool {
	switch name {
	case "apigw_v1", "bytes", "codes", "context", "fmt", "grpc", "http", "io",
		"proto", "protojson", "protopack", "protowire", "status", "strconv", "strings", "time":
		return true
	default:
		return false
	}
}

func (ix *importTracker) allocatePackageName(pkgName pgs.Name, importName pgs.FilePath) pgs.Name {

	for {
		if fixedImportName(pkgName) {
			pkgName += "x"
			continue
		}
		to, ok := ix.typeMapper[pkgName]
		if !ok {
			ix.typeMapper[pkgName] = importName
			break
		}
		if to == importName {
			break
		}
		pkgName += "x"
	}
	return pkgName
}

func (c *importTracker) elType(ft pgs.FieldType) pgsgo.TypeName {
	el := ft.Element()
	switch {
	case el.IsEnum():
		return c.importableTypeName(ft.Field(), el.Enum())
	case el.IsEmbed():
		return c.importableTypeName(ft.Field(), el.Embed()).Pointer()
	default:
		return scalarType(el.ProtoType())
	}
}

func scalarType(t pgs.ProtoType) pgsgo.TypeName {
	switch t {
	case pgs.DoubleT:
		return "float64"
	case pgs.FloatT:
		return "float32"
	case pgs.Int64T, pgs.SFixed64, pgs.SInt64:
		return "int64"
	case pgs.UInt64T, pgs.Fixed64T:
		return "uint64"
	case pgs.Int32T, pgs.SFixed32, pgs.SInt32:
		return "int32"
	case pgs.UInt32T, pgs.Fixed32T:
		return "uint32"
	case pgs.BoolT:
		return "bool"
	case pgs.StringT:
		return "string"
	case pgs.BytesT:
		return "[]byte"
	default:
		panic("unreachable: invalid scalar type")
	}
}
