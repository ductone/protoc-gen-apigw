package apigw

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protodesc"
	"google.golang.org/protobuf/reflect/protoregistry"
	"google.golang.org/protobuf/types/descriptorpb"
)

// The generator reads proto comments through protoc-gen-star's SourceCodeInfo,
// which is present in a real CodeGeneratorRequest but absent from the
// descriptors compiled into Go packages. The tests therefore drive the plugin
// from a committed descriptor set that carries source info for the two example
// files under test.
//
// Regenerate after editing either example proto:
//
//	make testdata
const (
	descriptorFixturePath = "testdata/example.fds.bin"
	descriptorFixtureDir  = "testdata"
)

var fixtureTargets = []string{
	"bookstore/v1/bookstore.proto",
	"tfcustomize/v1/tfcustomize.proto",
}

var updateDescriptorFixture = flag.Bool(
	"update-descriptors", false,
	"regenerate "+descriptorFixturePath+" from the example protos with buf")

// TestRegenerateDescriptorFixture rewrites the fixture. It is skipped unless
// -update-descriptors is passed, so it never depends on buf during a normal
// test run.
func TestRegenerateDescriptorFixture(t *testing.T) {
	if !*updateDescriptorFixture {
		t.Skip("pass -update-descriptors to regenerate " + descriptorFixturePath)
	}
	set := &descriptorpb.FileDescriptorSet{}
	for _, target := range fixtureTargets {
		built, err := buildFileDescriptorSet(t, filepath.Dir("example/"+target))
		if err != nil {
			t.Fatalf("buf build for %s: %v", target, err)
		}
		found := false
		for _, f := range built.GetFile() {
			if f.GetName() != target {
				continue
			}
			found = true
			set.File = append(set.File, f)
		}
		if !found {
			t.Fatalf("buf build did not produce %s", target)
		}
	}
	data, err := proto.Marshal(set)
	if err != nil {
		t.Fatalf("marshaling fixture: %v", err)
	}
	if err := os.MkdirAll(descriptorFixtureDir, 0o755); err != nil {
		t.Fatalf("creating %s: %v", descriptorFixtureDir, err)
	}
	if err := os.WriteFile(descriptorFixturePath, data, 0o644); err != nil {
		t.Fatalf("writing %s: %v", descriptorFixturePath, err)
	}
	t.Logf("wrote %s (%d bytes, %d files)", descriptorFixturePath, len(data), len(set.GetFile()))
}

func buildFileDescriptorSet(t *testing.T, path string) (*descriptorpb.FileDescriptorSet, error) {
	t.Helper()
	cmd := exec.Command("buf", "build", "--as-file-descriptor-set", "--path", path, "-o", "-")
	// buf resolves the workspace from its working directory, and the tests run
	// inside internal/apigw.
	cmd.Dir = filepath.Join("..", "..")
	cmd.Stderr = os.Stderr
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("running buf: %w", err)
	}
	set := &descriptorpb.FileDescriptorSet{}
	if err := proto.Unmarshal(out, set); err != nil {
		return nil, fmt.Errorf("decoding buf output: %w", err)
	}
	return set, nil
}

func loadDescriptorFixture(t *testing.T) map[string]*descriptorpb.FileDescriptorProto {
	t.Helper()
	data, err := os.ReadFile(descriptorFixturePath)
	if err != nil {
		t.Fatalf("reading %s: %v (run `make testdata`)", descriptorFixturePath, err)
	}
	set := &descriptorpb.FileDescriptorSet{}
	if err := proto.Unmarshal(data, set); err != nil {
		t.Fatalf("decoding %s: %v", descriptorFixturePath, err)
	}
	files := map[string]*descriptorpb.FileDescriptorProto{}
	for _, f := range set.GetFile() {
		files[f.GetName()] = f
	}
	for _, target := range fixtureTargets {
		if _, ok := files[target]; !ok {
			t.Fatalf("%s does not contain %s (run `make testdata`)", descriptorFixturePath, target)
		}
	}
	return files
}

// TestDescriptorFixtureIsCurrent catches a proto edited without regenerating
// the fixture: the fixture must describe exactly the same file, ignoring the
// source info the Go registry does not carry.
func TestDescriptorFixtureIsCurrent(t *testing.T) {
	files := loadDescriptorFixture(t)
	for _, target := range fixtureTargets {
		t.Run(target, func(t *testing.T) {
			registered, err := protoregistry.GlobalFiles.FindFileByPath(target)
			if err != nil {
				t.Fatalf("finding %s in the global registry: %v", target, err)
			}
			want := protodesc.ToFileDescriptorProto(registered)
			got := proto.Clone(files[target]).(*descriptorpb.FileDescriptorProto)
			got.SourceCodeInfo = nil
			if !proto.Equal(want, got) {
				t.Fatalf("%s is stale (the proto changed): run `make testdata`", descriptorFixturePath)
			}
			if len(files[target].GetSourceCodeInfo().GetLocation()) == 0 {
				t.Fatalf("%s has no source info; the fixture cannot drive the generator", target)
			}
		})
	}
}
