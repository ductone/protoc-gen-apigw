package apigw

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// TestEnumQueryGenerated compiles the real plugin output, rather than just
// checking template text. Like make example, it requires protoc and the Go
// protobuf and gRPC plugins on PATH. All generated files are temporary.
func TestEnumQueryGenerated(t *testing.T) {
	testEnumQueryGenerated(t, "enumtypes")
}

func TestEnumQueryImportCollisions(t *testing.T) {
	for _, name := range []string{"status", "codes", "context", "strconv"} {
		t.Run(name, func(t *testing.T) {
			testEnumQueryGenerated(t, name)
		})
	}
}

func testEnumQueryGenerated(t *testing.T, enumPackage string) {
	t.Helper()
	for _, tool := range []string{"go", "protoc", "protoc-gen-go", "protoc-gen-go-grpc"} {
		if _, err := exec.LookPath(tool); err != nil {
			t.Skipf("generated decoder test requires %s on PATH: %v", tool, err)
		}
	}
	root, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	// Stay inside the module so the compiled fixture uses its pinned vendor tree.
	dir, err := os.MkdirTemp(root, "enumqueryfixture")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(dir) })
	run := func(args ...string) {
		t.Helper()
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Dir = root
		cmd.Env = append(os.Environ(), "DEBUG_PROTOC_INPUT=false", "DEBUG_PROTOC_USE_FILE=")
		output, err := cmd.CombinedOutput()
		t.Logf("%v\n%s", args, output)
		if err != nil {
			t.Fatalf("command failed: %v", err)
		}
	}
	plugin := filepath.Join(dir, "protoc-gen-apigw")
	run("go", "build", "-mod=vendor", "-o", plugin, ".")
	fixture := filepath.Join(root, "internal/apigw/testdata/enumquery")
	pkg := "github.com/ductone/protoc-gen-apigw/" + filepath.Base(dir)
	inputDir := t.TempDir()
	// Put the temporary package path in the input descriptors too: the gateway
	// resolves imported enum names from go_package, not protoc's Go M mappings.
	for _, name := range []string{"query.proto", "types.proto"} {
		data, err := os.ReadFile(filepath.Join(fixture, name))
		if err != nil {
			t.Fatal(err)
		}
		data = []byte(strings.ReplaceAll(string(data), "github.com/ductone/protoc-gen-apigw/internal/apigw/enumquery", pkg))
		if name == "types.proto" {
			data = []byte(strings.Replace(string(data), ";enumtypes", ";"+enumPackage, 1))
		}
		if err := os.WriteFile(filepath.Join(inputDir, name), data, 0600); err != nil {
			t.Fatal(err)
		}
	}
	run("protoc", "-I"+inputDir, "-I"+filepath.Join(root, "protos"),
		"--go_out="+dir, "--go_opt=module="+pkg,
		"--go-grpc_out="+dir, "--go-grpc_opt=module="+pkg,
		"--plugin=protoc-gen-apigw="+plugin, "--apigw_out="+dir,
		"--apigw_opt=paths=source_relative", "query.proto", "types.proto")
	data, err := os.ReadFile(filepath.Join(fixture, "runtime_test.go"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "runtime_test.go"), data, 0600); err != nil {
		t.Fatal(err)
	}
	run("go", "test", "-mod=vendor", "-count=1", "-v", "./"+filepath.Base(dir))
	source, err := os.ReadFile(filepath.Join(fixture, "query.proto"))
	if err != nil {
		t.Fatal(err)
	}
	for _, tt := range []struct {
		name, old, replacement, want string
	}{
		{"repeated field", "ExampleFormat format = 1;", "repeated ExampleFormat format = 1;", "target field is repeated"},
		{"nested field path", `value: "format"`, `value: "child.format"`, "unexpected path"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			inputDir := t.TempDir()
			modified := strings.Replace(string(source), tt.old, tt.replacement, 1)
			if err := os.WriteFile(filepath.Join(inputDir, "query.proto"), []byte(modified), 0600); err != nil {
				t.Fatal(err)
			}
			cmd := exec.Command("protoc", "-I"+inputDir, "-I"+fixture, "-I"+filepath.Join(root, "protos"),
				"--plugin=protoc-gen-apigw="+plugin, "--apigw_out="+dir,
				"--apigw_opt=paths=source_relative", "query.proto")
			cmd.Env = append(os.Environ(), "DEBUG_PROTOC_INPUT=false", "DEBUG_PROTOC_USE_FILE=")
			output, err := cmd.CombinedOutput()
			if err == nil || !strings.Contains(string(output), tt.want) {
				t.Fatalf("expected unsupported query target (%s), got %v:\n%s", tt.want, err, output)
			}
		})
	}
}
