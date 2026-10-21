package apigw

import (
	"go/parser"
	"go/token"
	"path"
	"sort"
	"strconv"
	"testing"

	pgs "github.com/lyft/protoc-gen-star"
)

func TestImportAliasesReserveFixedHeaderNames(t *testing.T) {
	header, err := templateExecToString("header.tmpl", &headerTemplateContext{
		PackageName: "example",
		Imports: &importTracker{
			APIGWV1: true, Bytes: true, Embed: true, Fmt: true,
			GRPC: true, GRPCCodes: true, GRPCStatus: true, Io: true,
			NetHTTP: true, Ogen: true, ProtobufEncodingJSON: true,
			ProtobufProto: true, ProtobufProtoPack: true,
			ProtobufEncodingProtowire: true, Strconv: true, Strings: true, Time: true,
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	file, err := parser.ParseFile(token.NewFileSet(), "gateway.go", header, parser.ImportsOnly)
	if err != nil {
		t.Fatal(err)
	}
	names := map[string]bool{}
	for _, spec := range file.Imports {
		importPath, err := strconv.Unquote(spec.Path.Value)
		if err != nil {
			t.Fatal(err)
		}
		name := path.Base(importPath)
		if spec.Name != nil {
			name = spec.Name.Name
		}
		if name != "_" {
			names[name] = true
		}
	}
	ordered := make([]string, 0, len(names))
	for name := range names {
		ordered = append(ordered, name)
	}
	sort.Strings(ordered)
	for _, name := range ordered {
		t.Run(name, func(t *testing.T) {
			// Flags are intentionally unset: a later method can enable imports.
			ix := &importTracker{typeMapper: make(map[pgs.Name]pgs.FilePath)}
			firstPath := pgs.FilePath("example.com/first/" + name)
			first := ix.allocatePackageName(pgs.Name(name), firstPath)
			if first != pgs.Name(name+"x") {
				t.Fatalf("fixed identifier %q was not reserved: %q", name, first)
			}
			if again := ix.allocatePackageName(pgs.Name(name), firstPath); again != first {
				t.Fatalf("alias changed for the same import: %q != %q", again, first)
			}
			second := ix.allocatePackageName(pgs.Name(name), pgs.FilePath("example.com/second/"+name))
			if second != pgs.Name(name+"xx") {
				t.Fatalf("proto import collision was not disambiguated: %q", second)
			}
			imports := ix.ProtoImports()
			if len(imports) != 2 || imports[0].Alias != second.String() || imports[1].Alias != first.String() {
				t.Fatalf("emitted imports differ from allocated aliases: %v", imports)
			}
		})
	}
}

func TestImportAliasesPreserveOrdinaryNames(t *testing.T) {
	ix := &importTracker{typeMapper: make(map[pgs.Name]pgs.FilePath)}
	for _, name := range []string{"enumtypes", "statusx", "embed"} {
		got := ix.allocatePackageName(pgs.Name(name), pgs.FilePath("example.com/schema/"+name))
		if got != pgs.Name(name) {
			t.Fatalf("ordinary name %q changed to %q", name, got)
		}
	}
	got := ix.allocatePackageName("status", "example.com/schema/status")
	if got != "statusxx" {
		t.Fatalf("expected to skip both fixed and occupied names, got %q", got)
	}
}
